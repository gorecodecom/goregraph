package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agent"
)

type sourceReadFileWire struct {
	agent.SourceReadFileRequest
	StartLine json.RawMessage `json:"start_line"`
	EndLine   json.RawMessage `json:"end_line"`
}

func decodeSourceReadRequest(body string) (agent.ReadSourceRequest, error) {
	var wire struct {
		Files []sourceReadFileWire `json:"files"`
	}
	decode := func(value string) error {
		wire.Files = nil
		decoder := json.NewDecoder(strings.NewReader(value))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&wire); err != nil {
			return err
		}
		remainder := strings.TrimSpace(value[decoder.InputOffset():])
		if remainder != "" && remainder != "]}" {
			return fmt.Errorf("expected exactly one JSON request")
		}
		return nil
	}
	if err := decode(body); err != nil {
		repaired := repairInvalidJSONEscapes(body)
		if repaired == body {
			return agent.ReadSourceRequest{}, err
		}
		if repairErr := decode(repaired); repairErr != nil {
			return agent.ReadSourceRequest{}, repairErr
		}
	}
	request := agent.ReadSourceRequest{Files: make([]agent.SourceReadFileRequest, len(wire.Files))}
	for i, file := range wire.Files {
		normalized, err := normalizeSourceReadFile(file)
		if err != nil {
			return agent.ReadSourceRequest{}, fmt.Errorf("files[%d]: %w", i, err)
		}
		request.Files[i] = normalized
	}
	return request, nil
}

func repairInvalidJSONEscapes(body string) string {
	var result strings.Builder
	result.Grow(len(body))
	inString := false
	repaired := false
	for index := 0; index < len(body); index++ {
		char := body[index]
		if !inString {
			result.WriteByte(char)
			if char == '"' {
				inString = true
			}
			continue
		}
		if char == '"' {
			inString = false
			result.WriteByte(char)
			continue
		}
		if char != '\\' || index+1 >= len(body) {
			result.WriteByte(char)
			continue
		}
		next := body[index+1]
		if !strings.ContainsRune(`"\\/bfnrtu`, rune(next)) {
			result.WriteByte('\\')
			repaired = true
		}
		result.WriteByte(char)
		result.WriteByte(next)
		index++
	}
	if !repaired {
		return body
	}
	return result.String()
}

func normalizeSourceReadFile(wire sourceReadFileWire) (agent.SourceReadFileRequest, error) {
	file := wire.SourceReadFileRequest
	if wire.StartLine == nil && wire.EndLine == nil {
		return file, nil
	}
	const example = `use "ranges":[[10,30]] or "find":{"pattern":"handle","start_line":10}; CLI shorthand requires positive integer start_line/end_line, or start_line with find and no other cursor`
	invalid := func(reason string) (agent.SourceReadFileRequest, error) {
		return agent.SourceReadFileRequest{}, fmt.Errorf("%s; %s", reason, example)
	}
	var start, end int
	if err := json.Unmarshal(wire.StartLine, &start); err != nil || start < 1 {
		return invalid("start_line must be a positive JSON integer")
	}
	if len(file.Ranges) != 0 {
		return invalid("start_line/end_line cannot accompany nonempty ranges")
	}
	if file.Find != nil {
		if wire.EndLine != nil || file.Find.StartLine != 0 {
			return invalid("find cannot accompany end_line or cursors at both file and find levels")
		}
		find := *file.Find
		find.StartLine = start
		file.Find = &find
		return file, nil
	}
	if err := json.Unmarshal(wire.EndLine, &end); err != nil || end < 1 {
		return invalid("end_line must be a positive JSON integer when find is absent")
	}
	file.Ranges = []agent.SourceReadRange{{start, end}}
	return file, nil
}
