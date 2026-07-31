package agentmetrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const TokenUsageHeader = "input_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\teffective_tokens"

type TokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	UncachedInputTokens   int64 `json:"uncached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
	EffectiveTokens       int64 `json:"effective_tokens"`
}

func ParseTokenUsage(raw []byte) (TokenUsage, error) {
	var values map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &values) != nil {
		return TokenUsage{}, errors.New("usage is missing or invalid")
	}
	input, err := requiredTokenField(values, "input_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	output, err := requiredTokenField(values, "output_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	cached, err := optionalTokenField(values, "cached_input_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	reasoning, err := optionalTokenField(values, "reasoning_output_tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	if cached > input {
		return TokenUsage{}, errors.New("cached_input_tokens exceeds input_tokens")
	}
	if reasoning > output {
		return TokenUsage{}, errors.New("reasoning_output_tokens exceeds output_tokens")
	}
	if input > math.MaxInt64-output {
		return TokenUsage{}, errors.New("input_tokens plus output_tokens overflows int64")
	}
	total := input + output
	if total == 0 {
		return TokenUsage{}, errors.New("token usage is empty")
	}
	uncached := input - cached
	return TokenUsage{
		InputTokens:           input,
		CachedInputTokens:     cached,
		UncachedInputTokens:   uncached,
		OutputTokens:          output,
		ReasoningOutputTokens: reasoning,
		TotalTokens:           total,
		EffectiveTokens:       uncached + output,
	}, nil
}

func (usage TokenUsage) TSV() string {
	return fmt.Sprintf(
		"%d\t%d\t%d\t%d\t%d\t%d\t%d",
		usage.InputTokens,
		usage.CachedInputTokens,
		usage.UncachedInputTokens,
		usage.OutputTokens,
		usage.ReasoningOutputTokens,
		usage.TotalTokens,
		usage.EffectiveTokens,
	)
}

func ParseTokenUsageRow(row string) (TokenUsage, error) {
	fields := strings.Split(strings.TrimSpace(row), "\t")
	if len(fields) != 7 {
		return TokenUsage{}, fmt.Errorf("token usage row has %d fields, want 7", len(fields))
	}
	values := make([]int64, len(fields))
	for index, field := range fields {
		value, err := strconv.ParseInt(field, 10, 64)
		if err != nil || value < 0 {
			return TokenUsage{}, fmt.Errorf("token usage field %d is invalid", index+1)
		}
		values[index] = value
	}
	usage := TokenUsage{
		InputTokens:           values[0],
		CachedInputTokens:     values[1],
		UncachedInputTokens:   values[2],
		OutputTokens:          values[3],
		ReasoningOutputTokens: values[4],
		TotalTokens:           values[5],
		EffectiveTokens:       values[6],
	}
	if usage.CachedInputTokens > usage.InputTokens ||
		usage.UncachedInputTokens != usage.InputTokens-usage.CachedInputTokens ||
		usage.ReasoningOutputTokens > usage.OutputTokens ||
		usage.TotalTokens != usage.InputTokens+usage.OutputTokens ||
		usage.EffectiveTokens != usage.UncachedInputTokens+usage.OutputTokens ||
		usage.TotalTokens == 0 {
		return TokenUsage{}, errors.New("token usage row is internally inconsistent")
	}
	return usage, nil
}

func requiredTokenField(
	values map[string]json.RawMessage,
	name string,
) (int64, error) {
	raw, ok := values[name]
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	return decodeTokenField(raw, name)
}

func optionalTokenField(
	values map[string]json.RawMessage,
	name string,
) (int64, error) {
	raw, ok := values[name]
	if !ok {
		return 0, nil
	}
	return decodeTokenField(raw, name)
}

func decodeTokenField(raw json.RawMessage, name string) (int64, error) {
	var value int64
	if json.Unmarshal(raw, &value) != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must not be negative", name)
	}
	return value, nil
}
