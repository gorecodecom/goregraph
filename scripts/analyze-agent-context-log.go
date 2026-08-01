package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/gorecodecom/goregraph/internal/agentmetrics"
)

const header = agentmetrics.AnalyzerHeader

type event struct {
	Type  string
	Item  json.RawMessage
	Usage json.RawMessage
}

type metrics struct {
	toolCalls, goregraphCalls, fullPacks, compactPacks, repeatedPacks int
	navigationCalls, sourceReadCalls, includedSourceRereads           int
	boundedOmissionReadCalls, unauthorizedSourceReads                 int
	sourcePaths                                                       map[string]struct{}
	includedSourceRanges, authorizedOmissionRanges                    []sourceRange
	currentSourceTargets                                              []sourceRange
	workspace, commandDirectory                                       string
	eventOrder                                                        int
	currentItemID, currentCommand                                     string
	skillReads                                                        []skillReadEvidence
	skillReadEvents                                                   map[int]struct{}
	currentSkillTargets                                               map[string]struct{}
}

type analysis struct {
	metrics      metrics
	legacyTokens int64
	usage        agentmetrics.TokenUsage
	usageErr     error
}

type analyzerConfig struct {
	mode      string
	workspace string
	path      string
}

type skillReadEvidence struct {
	EventOrder int    `json:"event_order"`
	ItemID     string `json:"item_id"`
	Command    string `json:"command"`
	Target     string `json:"target"`
}

type parsedContextPack struct {
	contextID      string
	duplicateOf    string
	sourceCoverage string
	sourceRanges   []sourceRange
	omissionRanges []sourceRange
}

type sourceRange struct {
	path       string
	startLine  int
	endLine    int
	allContent bool
}

func main() {
	config, err := arguments(os.Args[1:])
	if err != nil {
		die(err)
	}
	if config.mode == "header" {
		fmt.Println(header)
		return
	}
	result, err := analyze(config.path, config.workspace)
	if err != nil {
		die(err)
	}
	switch config.mode {
	case "tokens":
		fmt.Println(result.legacyTokens)
		return
	case "usage":
		if result.usageErr != nil {
			die(result.usageErr)
		}
		fmt.Println(result.usage.TSV())
		return
	case "skill-reads":
		if result.metrics.workspace == "" {
			die(errors.New("--skill-reads requires --workspace"))
		}
		if err := json.NewEncoder(os.Stdout).Encode(result.metrics.skillReads); err != nil {
			die(err)
		}
		return
	}
	if result.metrics.toolCalls == 0 {
		die(errors.New("transcript has no parseable terminal tool items"))
	}
	fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
		result.metrics.toolCalls, result.metrics.goregraphCalls, result.metrics.fullPacks,
		result.metrics.compactPacks, result.metrics.repeatedPacks, result.metrics.navigationCalls,
		result.metrics.sourceReadCalls, result.metrics.boundedOmissionReadCalls,
		result.metrics.unauthorizedSourceReads, result.metrics.includedSourceRereads,
		len(result.metrics.sourcePaths), len(result.metrics.skillReadEvents))
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(2)
}

func arguments(args []string) (analyzerConfig, error) {
	config := analyzerConfig{mode: "metrics"}
	for len(args) > 0 {
		argument := args[0]
		args = args[1:]
		switch argument {
		case "--header", "--tokens", "--usage", "--skill-reads":
			if config.mode != "metrics" {
				return analyzerConfig{}, errors.New("only one analyzer output mode may be specified")
			}
			config.mode = strings.TrimPrefix(argument, "--")
		case "--workspace":
			if config.workspace != "" || len(args) == 0 {
				return analyzerConfig{}, errors.New("--workspace requires one absolute path")
			}
			config.workspace = args[0]
			args = args[1:]
			if !filepath.IsAbs(config.workspace) {
				return analyzerConfig{}, fmt.Errorf("workspace must be an absolute path: %s", config.workspace)
			}
		default:
			if config.path != "" {
				return analyzerConfig{}, errors.New("usage: analyze-agent-context-log.go [--header|--tokens|--usage|--skill-reads] [--workspace /absolute/path] /absolute/path/to/transcript.jsonl")
			}
			config.path = argument
		}
	}
	if config.path == "" {
		return analyzerConfig{}, errors.New("usage: analyze-agent-context-log.go [--header|--tokens|--usage|--skill-reads] [--workspace /absolute/path] /absolute/path/to/transcript.jsonl")
	}
	if !filepath.IsAbs(config.path) {
		return analyzerConfig{}, fmt.Errorf("transcript must be an absolute path: %s", config.path)
	}
	info, err := os.Stat(config.path)
	if err != nil {
		return analyzerConfig{}, err
	}
	if !info.Mode().IsRegular() {
		return analyzerConfig{}, fmt.Errorf("transcript must be a regular file: %s", config.path)
	}
	return config, nil
}

func analyze(path, workspace string) (analysis, error) {
	file, err := os.Open(path)
	if err != nil {
		return analysis{}, err
	}
	defer file.Close()
	result := analysis{metrics: metrics{
		sourcePaths:      make(map[string]struct{}),
		workspace:        workspace,
		commandDirectory: workspace,
		skillReadEvents:  make(map[int]struct{}),
	}}
	completed := make(map[string]string)
	fullIDs := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lineNumber := 0
	seenUsage := false
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
			_, err := processCompleted(outer.Item, completed, fullIDs, &result.metrics)
			if err != nil {
				return analysis{}, fmt.Errorf("invalid completed item at line %d: %w", lineNumber, err)
			}
		case "turn.completed":
			legacyTokens, err := legacyTokenUsage(outer.Usage)
			if err != nil {
				return analysis{}, fmt.Errorf("turn.completed at line %d: %w", lineNumber, err)
			}
			usage, usageErr := agentmetrics.ParseTokenUsage(outer.Usage)
			result.legacyTokens = legacyTokens
			result.usage = usage
			result.usageErr = usageErr
			seenUsage = true
		}
	}
	if err := scanner.Err(); err != nil {
		return analysis{}, err
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
	metrics.eventOrder++
	if isNonToolType(itemType) {
		return false, nil
	}
	if !isToolType(itemType) {
		return false, fmt.Errorf("unknown completed item type: %s", itemType)
	}
	metrics.toolCalls++
	metrics.commandDirectory = metrics.workspace
	metrics.currentItemID = id
	metrics.currentCommand = ""
	metrics.currentSkillTargets = make(map[string]struct{})
	contextCall := false
	includedReread := false
	switch itemType {
	case "command_execution":
		command, err := unwrapCommand(stringValue(item, "command"))
		if err != nil {
			return false, err
		}
		metrics.currentCommand = command
		contextCall, includedReread = classifyCommand(command, metrics)
	case "mcp_tool_call":
		contextCall = stringValue(item, "tool") == "task_context" || stringValue(item, "name") == "task_context"
	case "file_change":
		metrics.currentSourceTargets = nil
		recorded, included := recordSourcePath(stringValue(item, "path"), 0, 0, metrics)
		if recorded {
			metrics.navigationCalls++
			metrics.sourceReadCalls++
			metrics.unauthorizedSourceReads++
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

func legacyTokenUsage(raw json.RawMessage) (int64, error) {
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

func executableName(value string) string {
	windows := strings.Contains(value, `\`) ||
		(len(value) >= 2 && unicode.IsLetter(rune(value[0])) && value[1] == ':')
	name := pathpkg.Base(strings.ReplaceAll(value, `\`, "/"))
	if windows {
		name = strings.ToLower(name)
		name = strings.TrimSuffix(name, ".exe")
	}
	return name
}

func isShell(name string) bool {
	switch executableName(name) {
	case "sh", "bash", "zsh":
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
	metrics.currentSourceTargets = nil
	metrics.commandDirectory = metrics.workspace
	contextCall, navigation, sourceRead, includedReread := false, false, false, false
	searchOrInventory := false
	for _, segment := range shellSegments(words) {
		if len(segment) == 2 && executableName(segment[0]) == "cd" {
			if directory, ok := resolveCommandDirectory(metrics.commandDirectory, segment[1]); ok {
				metrics.commandDirectory = directory
			}
			continue
		}
		context, navigates, reads, included, searches := classifySimpleCommand(segment, metrics)
		contextCall = contextCall || context
		navigation = navigation || navigates
		sourceRead = sourceRead || reads
		includedReread = includedReread || included
		searchOrInventory = searchOrInventory || searches
	}
	if navigation {
		metrics.navigationCalls++
	}
	if sourceRead {
		metrics.sourceReadCalls++
	}
	if navigation {
		if sourceRead && !searchOrInventory && !includedReread &&
			sourceTargetsFitOmissions(metrics.currentSourceTargets, metrics.authorizedOmissionRanges) {
			metrics.boundedOmissionReadCalls++
		} else {
			metrics.unauthorizedSourceReads++
		}
	}
	return contextCall, includedReread
}

func resolveCommandDirectory(current, target string) (string, bool) {
	current = strings.TrimSpace(strings.ReplaceAll(current, `\`, "/"))
	target = strings.TrimSpace(strings.ReplaceAll(target, `\`, "/"))
	if target == "" || !isAbsoluteCommandPath(current) {
		return "", false
	}
	if isAbsoluteCommandPath(target) {
		return pathpkg.Clean(target), true
	}
	return pathpkg.Join(current, target), true
}

func isAbsoluteCommandPath(value string) bool {
	if strings.HasPrefix(value, "/") {
		return true
	}
	return len(value) >= 3 && unicode.IsLetter(rune(value[0])) &&
		value[1] == ':' && value[2] == '/'
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

func classifySimpleCommand(words []string, metrics *metrics) (bool, bool, bool, bool, bool) {
	if len(words) == 0 {
		return false, false, false, false, false
	}
	command := executableName(words[0])
	switch command {
	case "goregraph":
		return len(words) > 1 && words[1] == "context", false, false, false, false
	case "rg", "grep":
		_, included := recordSearchTargets(command, words[1:], metrics)
		return false, true, false, included, true
	case "find":
		_, included := recordFindTargets(words[1:], metrics)
		return false, true, false, included, true
	case "sed", "nl", "cat", "head", "tail":
		reads, included := recordReadTargets(command, words[1:], metrics)
		return false, reads, reads, included, false
	}
	return false, false, false, false, false
}

func recordSearchTargets(command string, words []string, metrics *metrics) (bool, bool) {
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
			case "--files":
				if command == "rg" {
					patternSeen = true
				}
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
		recordCommandTarget(word, metrics)
		recorded, included := recordSourcePath(word, 0, 0, metrics)
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
		recordCommandTarget(word, metrics)
		recorded, included := recordSourcePath(word, 0, 0, metrics)
		found = recorded || found
		includedReread = includedReread || included
	}
	return found, includedReread
}

func recordReadTargets(command string, words []string, metrics *metrics) (bool, bool) {
	scriptRequired, scriptSeen, optionValue, endOptions, found := command == "sed", false, false, false, false
	includedReread := false
	startLine, endLine := 0, 0
	for _, word := range words {
		if optionValue {
			optionValue = false
			if command == "sed" {
				scriptSeen = true
				startLine, endLine = sedSourceRange(word)
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
			if command == "sed" && strings.HasPrefix(word, "-e") && len(word) > 2 {
				scriptSeen = true
				startLine, endLine = sedSourceRange(word[2:])
				continue
			}
			if command == "sed" && strings.HasPrefix(word, "-f") && len(word) > 2 {
				scriptSeen = true
				continue
			}
			if strings.HasPrefix(word, "-") {
				continue
			}
		}
		if scriptRequired && !scriptSeen {
			scriptSeen = true
			startLine, endLine = sedSourceRange(word)
			continue
		}
		recordCommandTarget(word, metrics)
		recorded, included := recordSourcePath(word, startLine, endLine, metrics)
		found = recorded || found
		includedReread = includedReread || included
	}
	return found, includedReread
}

func recordCommandTarget(target string, metrics *metrics) {
	normalized, ok := agentmetrics.ClassifyExternalSkillTarget(
		metrics.workspace,
		metrics.commandDirectory,
		target,
	)
	if !ok {
		return
	}
	if _, exists := metrics.currentSkillTargets[normalized]; exists {
		return
	}
	metrics.currentSkillTargets[normalized] = struct{}{}
	metrics.skillReads = append(metrics.skillReads, skillReadEvidence{
		EventOrder: metrics.eventOrder,
		ItemID:     metrics.currentItemID,
		Command:    metrics.currentCommand,
		Target:     normalized,
	})
	metrics.skillReadEvents[metrics.eventOrder] = struct{}{}
}

func sedSourceRange(script string) (int, int) {
	script = strings.TrimSpace(script)
	address := ""
	if strings.HasSuffix(script, "{=;p;}") {
		address = strings.TrimSpace(strings.TrimSuffix(script, "{=;p;}"))
	} else {
		if len(script) == 0 || script[len(script)-1] != 'p' {
			return 0, 0
		}
		address = strings.TrimSpace(strings.TrimSuffix(script, "p"))
	}
	parts := strings.Split(address, ",")
	if len(parts) > 2 {
		return 0, 0
	}
	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || start <= 0 {
		return 0, 0
	}
	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || end < start {
			return 0, 0
		}
	}
	return start, end
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

func recordSourcePath(path string, startLine, endLine int, metrics *metrics) (bool, bool) {
	path = normalizeRecordedSourcePath(path)
	if path == "" || strings.ContainsAny(path, "*?[") {
		return false, false
	}
	if !isSourcePath(path) {
		return false, false
	}
	metrics.sourcePaths[path] = struct{}{}
	metrics.currentSourceTargets = append(metrics.currentSourceTargets, sourceRange{
		path: path, startLine: startLine, endLine: endLine,
	})
	for _, included := range metrics.includedSourceRanges {
		if sameSourcePath(path, included.path) &&
			sourceRangesOverlap(startLine, endLine, included) {
			return true, true
		}
	}
	return true, false
}

func sourceTargetsFitOmissions(targets, omissions []sourceRange) bool {
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		if target.startLine <= 0 || target.endLine < target.startLine {
			return false
		}
		contained := false
		for _, omission := range omissions {
			if sameSourcePath(target.path, omission.path) &&
				omission.startLine > 0 &&
				omission.endLine >= omission.startLine &&
				target.startLine >= omission.startLine &&
				target.endLine <= omission.endLine {
				contained = true
				break
			}
		}
		if !contained {
			return false
		}
	}
	return true
}

func sourceRangesOverlap(readStart, readEnd int, included sourceRange) bool {
	if included.allContent || readStart <= 0 || readEnd <= 0 ||
		included.startLine <= 0 || included.endLine <= 0 {
		return true
	}
	return readStart <= included.endLine && included.startLine <= readEnd
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
	case ".asm", ".bash", ".c", ".cc", ".clj", ".cpp", ".cs", ".css", ".cxx", ".dart", ".elm", ".ex", ".exs", ".fs", ".fsi", ".go", ".groovy", ".gvy", ".h", ".hpp", ".hrl", ".hs", ".html", ".java", ".jl", ".js", ".jsx", ".kt", ".kts", ".lua", ".m", ".mjs", ".mm", ".php", ".pl", ".pm", ".properties", ".py", ".r", ".rb", ".rs", ".scala", ".scss", ".sh", ".sol", ".sql", ".swift", ".ts", ".tsx", ".vue", ".yaml", ".yml", ".zig", ".zsh":
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
		for _, included := range pack.sourceRanges {
			included.path = normalizeRecordedSourcePath(included.path)
			included.allContent = pack.sourceCoverage == "complete"
			if included.path != "" {
				metrics.includedSourceRanges = append(metrics.includedSourceRanges, included)
			}
		}
		if pack.duplicateOf == "" {
			for _, omission := range pack.omissionRanges {
				omission.path = normalizeRecordedSourcePath(omission.path)
				if omission.path != "" && omission.startLine > 0 &&
					omission.endLine >= omission.startLine {
					metrics.authorizedOmissionRanges = append(
						metrics.authorizedOmissionRanges,
						omission,
					)
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
			Project   string `json:"project"`
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		} `json:"source_sections"`
		SourceOmissions []struct {
			Project   string `json:"project"`
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		} `json:"source_omissions"`
	}
	if json.Unmarshal([]byte(text), &jsonPack) == nil && jsonPack.ContextID != "" {
		pack := parsedContextPack{
			contextID:      jsonPack.ContextID,
			duplicateOf:    jsonPack.DuplicateOf,
			sourceCoverage: jsonPack.SourceCoverage,
			sourceRanges:   make([]sourceRange, 0, len(jsonPack.SourceSections)),
			omissionRanges: make([]sourceRange, 0, len(jsonPack.SourceOmissions)),
		}
		for _, section := range jsonPack.SourceSections {
			path := section.Path
			if section.Project != "" {
				path = strings.TrimSuffix(section.Project, "/") + "/" + strings.TrimPrefix(path, "/")
			}
			pack.sourceRanges = append(pack.sourceRanges, sourceRange{
				path: path, startLine: section.StartLine, endLine: section.EndLine,
			})
		}
		for _, omission := range jsonPack.SourceOmissions {
			if strings.TrimSpace(omission.Project) == "" ||
				strings.TrimSpace(omission.Path) == "" {
				continue
			}
			path := strings.TrimSuffix(omission.Project, "/") + "/" +
				strings.TrimPrefix(omission.Path, "/")
			pack.omissionRanges = append(pack.omissionRanges, sourceRange{
				path: path, startLine: omission.StartLine, endLine: omission.EndLine,
			})
		}
		return pack
	}

	pack := parsedContextPack{}
	inSourceSections, inSourceOmissions := false, false
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
			inSourceOmissions = false
			continue
		}
		if line == "## Source omissions" {
			inSourceSections = false
			inSourceOmissions = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inSourceSections = false
			inSourceOmissions = false
			continue
		}
		if inSourceSections && strings.HasPrefix(line, "### ") {
			if section := markdownSourceRange(line); section.path != "" {
				pack.sourceRanges = append(pack.sourceRanges, section)
			}
		}
		if inSourceOmissions && strings.HasPrefix(line, "- ") {
			if omission := markdownSourceRange(line); omission.path != "" {
				pack.omissionRanges = append(pack.omissionRanges, omission)
			}
		}
	}
	return pack
}

func markdownSourceRange(line string) sourceRange {
	start := strings.IndexByte(line, '`')
	if start < 0 {
		return sourceRange{}
	}
	rest := line[start+1:]
	end := strings.IndexByte(rest, '`')
	if end < 0 {
		return sourceRange{}
	}
	return parseSourceLineRange(rest[:end])
}

func parseSourceLineRange(path string) sourceRange {
	colon := strings.LastIndexByte(path, ':')
	if colon < 0 {
		return sourceRange{path: path}
	}
	suffix := path[colon+1:]
	parts := strings.Split(suffix, "-")
	if len(parts) > 2 {
		return sourceRange{path: path}
	}
	for _, part := range parts {
		if part == "" || strings.IndexFunc(part, func(current rune) bool {
			return current < '0' || current > '9'
		}) >= 0 {
			return sourceRange{path: path}
		}
	}
	startLine, err := strconv.Atoi(parts[0])
	if err != nil || startLine <= 0 {
		return sourceRange{path: path}
	}
	endLine := startLine
	if len(parts) == 2 {
		endLine, err = strconv.Atoi(parts[1])
		if err != nil || endLine < startLine {
			return sourceRange{path: path}
		}
	}
	return sourceRange{path: path[:colon], startLine: startLine, endLine: endLine}
}
