package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const header = "tool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tincluded_source_rereads\tunique_source_files"

type event struct {
	Type  string
	Item  json.RawMessage
	Usage json.RawMessage
}

type metrics struct {
	toolCalls, goregraphCalls, fullPacks, compactPacks, repeatedPacks int
	navigationCalls, sourceReadCalls, includedSourceRereads           int
	sourcePaths, completePackSourcePaths                              map[string]struct{}
}

type analysis struct {
	metrics metrics
	tokens  int64
}

type parsedContextPack struct {
	contextID      string
	duplicateOf    string
	sourceCoverage string
	sourcePaths    []string
}

func main() {
	mode, path, err := arguments(os.Args[1:])
	if err != nil {
		die(err)
	}
	if mode == "header" {
		fmt.Println(header)
		return
	}
	result, err := analyze(path)
	if err != nil {
		die(err)
	}
	if mode == "tokens" {
		fmt.Println(result.tokens)
		return
	}
	fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
		result.metrics.toolCalls, result.metrics.goregraphCalls, result.metrics.fullPacks,
		result.metrics.compactPacks, result.metrics.repeatedPacks, result.metrics.navigationCalls,
		result.metrics.sourceReadCalls, result.metrics.includedSourceRereads,
		len(result.metrics.sourcePaths))
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(2)
}

func arguments(args []string) (string, string, error) {
	mode := "metrics"
	if len(args) > 0 && (args[0] == "--header" || args[0] == "--tokens") {
		mode = strings.TrimPrefix(args[0], "--")
		args = args[1:]
	}
	if len(args) != 1 {
		return "", "", errors.New("usage: analyze-agent-context-log.go [--header|--tokens] /absolute/path/to/transcript.jsonl")
	}
	if !filepath.IsAbs(args[0]) {
		return "", "", fmt.Errorf("transcript must be an absolute path: %s", args[0])
	}
	info, err := os.Stat(args[0])
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("transcript must be a regular file: %s", args[0])
	}
	return mode, args[0], nil
}

func analyze(path string) (analysis, error) {
	file, err := os.Open(path)
	if err != nil {
		return analysis{}, err
	}
	defer file.Close()
	result := analysis{metrics: metrics{
		sourcePaths:             make(map[string]struct{}),
		completePackSourcePaths: make(map[string]struct{}),
	}}
	completed := make(map[string]string)
	fullIDs := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lineNumber := 0
	seenTool, seenUsage := false, false
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var outer event
		if err := json.Unmarshal(line, &outer); err != nil {
			return analysis{}, fmt.Errorf("invalid JSONL at line %d: %w", lineNumber, err)
		}
		switch outer.Type {
		case "item.completed":
			tool, err := processCompleted(outer.Item, completed, fullIDs, &result.metrics)
			if err != nil {
				return analysis{}, fmt.Errorf("invalid completed item at line %d: %w", lineNumber, err)
			}
			seenTool = seenTool || tool
		case "turn.completed":
			tokens, err := tokenUsage(outer.Usage)
			if err != nil {
				return analysis{}, fmt.Errorf("turn.completed at line %d: %w", lineNumber, err)
			}
			result.tokens, seenUsage = tokens, true
		}
	}
	if err := scanner.Err(); err != nil {
		return analysis{}, err
	}
	if !seenTool || result.metrics.toolCalls == 0 {
		return analysis{}, errors.New("transcript has no parseable terminal tool items")
	}
	if !seenUsage {
		return analysis{}, errors.New("transcript has no turn.completed usage")
	}
	return result, nil
}

func processCompleted(raw json.RawMessage, completed map[string]string, fullIDs map[string]struct{}, metrics *metrics) (bool, error) {
	if len(raw) == 0 {
		return false, errors.New("missing item payload")
	}
	var item map[string]json.RawMessage
	if err := json.Unmarshal(raw, &item); err != nil {
		return false, err
	}
	id, itemType := stringValue(item, "id"), stringValue(item, "type")
	if id == "" || itemType == "" {
		return false, errors.New("item payload is missing id or type")
	}
	canonical := string(raw)
	if previous, exists := completed[id]; exists {
		if previous != canonical {
			return false, fmt.Errorf("conflicting terminal item id: %s", id)
		}
		return isToolType(itemType), nil
	}
	completed[id] = canonical
	if isNonToolType(itemType) {
		return false, nil
	}
	if !isToolType(itemType) {
		return false, fmt.Errorf("unknown completed item type: %s", itemType)
	}
	metrics.toolCalls++
	contextCall := false
	includedReread := false
	switch itemType {
	case "command_execution":
		command, err := unwrapCommand(stringValue(item, "command"))
		if err != nil {
			return false, err
		}
		contextCall, includedReread = classifyCommand(command, metrics)
	case "mcp_tool_call":
		contextCall = stringValue(item, "tool") == "task_context" || stringValue(item, "name") == "task_context"
	case "file_change":
		recorded, included := recordSourcePath(stringValue(item, "path"), metrics)
		if recorded {
			metrics.navigationCalls++
			metrics.sourceReadCalls++
		}
		includedReread = included
	}
	if includedReread {
		metrics.includedSourceRereads++
	}
	if contextCall {
		metrics.goregraphCalls++
		if err := recordContextPack(item, fullIDs, metrics); err != nil {
			return false, err
		}
	}
	return true, nil
}

func stringValue(values map[string]json.RawMessage, name string) string {
	raw, ok := values[name]
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

func isToolType(itemType string) bool {
	switch itemType {
	case "command_execution", "file_change", "mcp_tool_call", "collab_tool_call", "web_search":
		return true
	default:
		return false
	}
}

func isNonToolType(itemType string) bool {
	switch itemType {
	case "agent_message", "reasoning", "plan", "todo_list", "error", "user_message", "system_message":
		return true
	default:
		return false
	}
}

func tokenUsage(raw json.RawMessage) (int64, error) {
	var usage map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &usage) != nil {
		return 0, errors.New("usage is missing or invalid")
	}
	if total, ok := intValue(usage, "total_tokens"); ok && total > 0 {
		return total, nil
	}
	input, hasInput := intValue(usage, "input_tokens")
	output, hasOutput := intValue(usage, "output_tokens")
	if hasInput && hasOutput && input+output > 0 {
		return input + output, nil
	}
	return 0, errors.New("usage has no positive total_tokens or input/output token total")
}

func intValue(values map[string]json.RawMessage, name string) (int64, bool) {
	raw, ok := values[name]
	if !ok {
		return 0, false
	}
	var value int64
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	return value, true
}

func unwrapCommand(command string) (string, error) {
	words, err := shellWords(command)
	if err != nil {
		return "", err
	}
	if len(words) == 3 && isShell(words[0]) && words[1] == "-lc" {
		return words[2], nil
	}
	return command, nil
}

func isShell(name string) bool {
	switch name {
	case "/bin/sh", "/bin/bash", "/bin/zsh", "sh", "bash", "zsh":
		return true
	default:
		return false
	}
}

func shellWords(command string) ([]string, error) {
	var words []string
	var current strings.Builder
	appendCurrent := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	var quote rune
	escaped := false
	for _, char := range command {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && char == '\\' {
				escaped = true
			} else if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
			continue
		}
		switch {
		case char == '\'' || char == '"':
			quote = char
		case char == '\\':
			escaped = true
		case char == ';' || char == '|' || char == '&':
			appendCurrent()
			words = append(words, string(char))
		case unicode.IsSpace(char):
			appendCurrent()
		default:
			current.WriteRune(char)
		}
	}
	if escaped || quote != 0 {
		return nil, errors.New("unterminated shell command quoting")
	}
	appendCurrent()
	return words, nil
}

func classifyCommand(command string, metrics *metrics) (bool, bool) {
	words, err := shellWords(command)
	if err != nil || len(words) == 0 {
		return false, false
	}
	contextCall, navigation, sourceRead, includedReread := false, false, false, false
	for _, segment := range shellSegments(words) {
		context, navigates, reads, included := classifySimpleCommand(segment, metrics)
		contextCall = contextCall || context
		navigation = navigation || navigates
		sourceRead = sourceRead || reads
		includedReread = includedReread || included
	}
	if navigation {
		metrics.navigationCalls++
	}
	if sourceRead {
		metrics.sourceReadCalls++
	}
	return contextCall, includedReread
}

func shellSegments(words []string) [][]string {
	segments := make([][]string, 0, 1)
	current := make([]string, 0, len(words))
	for _, word := range words {
		if word == ";" || word == "|" || word == "&" {
			if len(current) > 0 {
				segments = append(segments, current)
				current = make([]string, 0, len(words))
			}
			continue
		}
		current = append(current, word)
	}
	if len(current) > 0 {
		segments = append(segments, current)
	}
	return segments
}

func classifySimpleCommand(words []string, metrics *metrics) (bool, bool, bool, bool) {
	if len(words) == 0 {
		return false, false, false, false
	}
	switch words[0] {
	case "goregraph":
		return len(words) > 1 && words[1] == "context", false, false, false
	case "rg", "grep":
		recorded, included := recordSearchTargets(words[1:], metrics)
		return false, recorded, false, included
	case "find":
		recorded, included := recordFindTargets(words[1:], metrics)
		return false, recorded, false, included
	case "sed", "nl", "cat", "head", "tail":
		reads, included := recordReadTargets(words[0], words[1:], metrics)
		return false, reads, reads, included
	}
	return false, false, false, false
}

func recordSearchTargets(words []string, metrics *metrics) (bool, bool) {
	patternSeen, optionValue, optionIsPattern, endOptions := false, false, false, false
	found, includedReread := false, false
	for _, word := range words {
		if optionValue {
			if optionIsPattern {
				patternSeen = true
			}
			optionValue, optionIsPattern = false, false
			continue
		}
		if !endOptions {
			switch word {
			case "--":
				endOptions = true
				continue
			case "-e", "--regexp", "-f", "--file":
				optionValue, optionIsPattern = true, true
				continue
			case "-g", "--glob", "--type", "--type-not":
				optionValue = true
				continue
			}
			if (strings.HasPrefix(word, "-e") || strings.HasPrefix(word, "-f")) && len(word) > 2 {
				patternSeen = true
				continue
			}
			if strings.HasPrefix(word, "--regexp=") || strings.HasPrefix(word, "--file=") {
				patternSeen = true
				continue
			}
			if strings.HasPrefix(word, "--glob=") || strings.HasPrefix(word, "--type=") || strings.HasPrefix(word, "--type-not=") || strings.HasPrefix(word, "-") {
				continue
			}
		}
		if !patternSeen {
			patternSeen = true
			continue
		}
		recorded, included := recordSourcePath(word, metrics)
		found = recorded || found
		includedReread = includedReread || included
	}
	return found, includedReread
}

func recordFindTargets(words []string, metrics *metrics) (bool, bool) {
	found, includedReread := false, false
	for _, word := range words {
		switch word {
		case "-name", "-iname", "-path", "-ipath", "-type", "-exec", "-execdir", "-ok", "-okdir", "-print", "-print0", "-delete", "-quit":
			return found, includedReread
		}
		recorded, included := recordSourcePath(word, metrics)
		found = recorded || found
		includedReread = includedReread || included
	}
	return found, includedReread
}

func recordReadTargets(command string, words []string, metrics *metrics) (bool, bool) {
	scriptRequired, scriptSeen, optionValue, endOptions, found := command == "sed", false, false, false, false
	includedReread := false
	for _, word := range words {
		if optionValue {
			optionValue = false
			if command == "sed" {
				scriptSeen = true
			}
			continue
		}
		if !endOptions {
			if word == "--" {
				endOptions = true
				continue
			}
			if optionTakesValue(command, word) {
				optionValue = true
				continue
			}
			if command == "sed" && (strings.HasPrefix(word, "-e") || strings.HasPrefix(word, "-f")) && len(word) > 2 {
				scriptSeen = true
				continue
			}
			if strings.HasPrefix(word, "-") {
				continue
			}
		}
		if scriptRequired && !scriptSeen {
			scriptSeen = true
			continue
		}
		recorded, included := recordSourcePath(word, metrics)
		found = recorded || found
		includedReread = includedReread || included
	}
	return found, includedReread
}

func optionTakesValue(command, option string) bool {
	switch command {
	case "sed":
		return option == "-e" || option == "-f"
	case "nl":
		switch option {
		case "-b", "-d", "-f", "-h", "-i", "-l", "-n", "-p", "-s", "-v", "-w":
			return true
		}
	case "head", "tail":
		return option == "-n" || option == "-c"
	}
	return false
}

func recordSourcePath(path string, metrics *metrics) (bool, bool) {
	path = normalizeRecordedSourcePath(path)
	if path == "" || strings.ContainsAny(path, "*?[") {
		return false, false
	}
	if !isSourcePath(path) {
		return false, false
	}
	metrics.sourcePaths[path] = struct{}{}
	for includedPath := range metrics.completePackSourcePaths {
		if sameSourcePath(path, includedPath) {
			return true, true
		}
	}
	return true, false
}

func normalizeRecordedSourcePath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(strings.Trim(path, "\"'(),;:")))
	for strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, "./")
	}
	return path
}

func sameSourcePath(readPath, includedPath string) bool {
	readPath = normalizeRecordedSourcePath(readPath)
	includedPath = normalizeRecordedSourcePath(includedPath)
	return readPath == includedPath ||
		strings.HasSuffix(readPath, "/"+includedPath)
}

func isSourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".asm", ".bash", ".c", ".cc", ".clj", ".cpp", ".cs", ".css", ".cxx", ".dart", ".elm", ".ex", ".exs", ".fs", ".fsi", ".go", ".groovy", ".gvy", ".h", ".hpp", ".hrl", ".hs", ".html", ".java", ".jl", ".js", ".jsx", ".kt", ".kts", ".lua", ".m", ".mjs", ".mm", ".php", ".pl", ".pm", ".py", ".r", ".rb", ".rs", ".scala", ".scss", ".sh", ".sol", ".sql", ".swift", ".ts", ".tsx", ".vue", ".zig", ".zsh":
		return true
	default:
		return false
	}
}

func recordContextPack(item map[string]json.RawMessage, fullIDs map[string]struct{}, metrics *metrics) error {
	texts, err := contextTexts(item)
	if err != nil {
		return err
	}
	for _, text := range texts {
		pack := parseContextPack(text)
		if pack.contextID == "" {
			continue
		}
		if pack.sourceCoverage == "complete" {
			for _, path := range pack.sourcePaths {
				path = normalizeRecordedSourcePath(path)
				if path != "" {
					metrics.completePackSourcePaths[path] = struct{}{}
				}
			}
		}
		if pack.duplicateOf != "" {
			metrics.compactPacks++
		} else if _, exists := fullIDs[pack.contextID]; exists {
			metrics.repeatedPacks++
		} else {
			metrics.fullPacks++
			fullIDs[pack.contextID] = struct{}{}
		}
		break
	}
	return nil
}

func contextTexts(item map[string]json.RawMessage) ([]string, error) {
	texts := make([]string, 0, 2)
	if output := stringValue(item, "aggregated_output"); output != "" {
		texts = append(texts, output)
	}
	raw, ok := item["result"]
	if !ok || string(raw) == "null" {
		return texts, nil
	}
	var direct string
	if json.Unmarshal(raw, &direct) == nil {
		return append(texts, direct), nil
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("invalid context result: %w", err)
	}
	var content []map[string]json.RawMessage
	if rawContent, ok := result["content"]; ok && json.Unmarshal(rawContent, &content) == nil {
		for _, block := range content {
			if stringValue(block, "type") == "text" {
				texts = append(texts, stringValue(block, "text"))
			}
		}
	}
	return texts, nil
}

func parseContextPack(text string) parsedContextPack {
	var jsonPack struct {
		ContextID      string `json:"context_id"`
		DuplicateOf    string `json:"duplicate_of"`
		SourceCoverage string `json:"source_coverage"`
		SourceSections []struct {
			Project string `json:"project"`
			Path    string `json:"path"`
		} `json:"source_sections"`
	}
	if json.Unmarshal([]byte(text), &jsonPack) == nil && jsonPack.ContextID != "" {
		pack := parsedContextPack{
			contextID:      jsonPack.ContextID,
			duplicateOf:    jsonPack.DuplicateOf,
			sourceCoverage: jsonPack.SourceCoverage,
			sourcePaths:    make([]string, 0, len(jsonPack.SourceSections)),
		}
		for _, section := range jsonPack.SourceSections {
			path := section.Path
			if section.Project != "" {
				path = strings.TrimSuffix(section.Project, "/") + "/" + strings.TrimPrefix(path, "/")
			}
			pack.sourcePaths = append(pack.sourcePaths, path)
		}
		return pack
	}

	pack := parsedContextPack{}
	inSourceSections := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Context ID:") {
			pack.contextID = strings.TrimSpace(strings.TrimPrefix(line, "Context ID:"))
		}
		if strings.HasPrefix(line, "Duplicate of:") {
			pack.duplicateOf = strings.TrimSpace(strings.TrimPrefix(line, "Duplicate of:"))
		}
		if strings.HasPrefix(line, "Source coverage:") {
			pack.sourceCoverage = strings.TrimSpace(strings.TrimPrefix(line, "Source coverage:"))
		}
		if line == "## Source sections" {
			inSourceSections = true
			continue
		}
		if inSourceSections && strings.HasPrefix(line, "## ") {
			inSourceSections = false
			continue
		}
		if !inSourceSections || !strings.HasPrefix(line, "### ") {
			continue
		}
		if path := markdownSourcePath(line); path != "" {
			pack.sourcePaths = append(pack.sourcePaths, path)
		}
	}
	return pack
}

func markdownSourcePath(line string) string {
	start := strings.IndexByte(line, '`')
	if start < 0 {
		return ""
	}
	rest := line[start+1:]
	end := strings.IndexByte(rest, '`')
	if end < 0 {
		return ""
	}
	return stripSourceLineRange(rest[:end])
}

func stripSourceLineRange(path string) string {
	colon := strings.LastIndexByte(path, ':')
	if colon < 0 {
		return path
	}
	suffix := path[colon+1:]
	parts := strings.Split(suffix, "-")
	if len(parts) > 2 {
		return path
	}
	for _, part := range parts {
		if part == "" || strings.IndexFunc(part, func(current rune) bool {
			return current < '0' || current > '9'
		}) >= 0 {
			return path
		}
	}
	return path[:colon]
}
