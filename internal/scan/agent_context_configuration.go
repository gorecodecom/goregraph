package scan

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const maxAgentContextConfigurationKeyGroups = 32

func extractAgentContextConfigurationFacts(file FileRecord, body string) []AgentContextFactRecord {
	if !isAgentContextConfigurationResource(file.Path) {
		return nil
	}

	path := contextPathKey(file.Path)
	normalizedBody := strings.ReplaceAll(body, "\r\n", "\n")
	normalizedBody = strings.ReplaceAll(normalizedBody, "\r", "\n")
	lines := strings.Split(normalizedBody, "\n")
	lineCount := len(lines)
	if strings.HasSuffix(normalizedBody, "\n") {
		lineCount--
	}
	facts := []AgentContextFactRecord{{
		ID:         stableID("agent-context-configuration", path, "file"),
		Kind:       "configuration",
		Name:       contextFileBase(path),
		Qualified:  path,
		File:       path,
		Line:       1,
		EndLine:    lineCount,
		Summary:    "Spring configuration resource",
		Confidence: string(ConfidenceExact),
		Search:     compactContextSearch("configuration", contextFileBase(path), path),
	}}

	groups := agentContextConfigurationKeyGroups(path, lines)
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > maxAgentContextConfigurationKeyGroups {
		keys = keys[:maxAgentContextConfigurationKeyGroups]
	}
	for _, key := range keys {
		group := groups[key]
		facts = append(facts, AgentContextFactRecord{
			ID:         stableID("agent-context-configuration", path, key),
			Kind:       "configuration",
			Name:       key,
			Qualified:  key,
			File:       path,
			Line:       group.start,
			EndLine:    group.end,
			Summary:    "Spring configuration key group",
			Confidence: string(ConfidenceExact),
			Search:     compactContextSearch("configuration", key, contextFileBase(path)),
		})
	}
	return facts
}

type agentContextConfigurationRange struct {
	start int
	end   int
}

type agentContextConfigurationLine struct {
	content string
	start   int
	end     int
}

func agentContextConfigurationKeyGroups(path string, lines []string) map[string]agentContextConfigurationRange {
	groups := map[string]agentContextConfigurationRange{}
	yamlParents := map[int]string{}
	for _, logicalLine := range agentContextConfigurationLogicalLines(path, lines) {
		line := logicalLine.content
		key, indent, ok := agentContextConfigurationKey(path, line)
		if !ok {
			continue
		}
		if isAgentContextConfigurationYAML(path) {
			for level := range yamlParents {
				if level > indent {
					delete(yamlParents, level)
				}
			}
			if indent > 0 {
				if parent, exists := yamlParents[largestConfigurationParentIndent(yamlParents, indent)]; exists {
					key = parent
				}
			}
			if strings.TrimSpace(line[strings.Index(line, ":")+1:]) == "" {
				yamlParents[indent] = key
			}
		}
		if group, exists := groups[key]; exists {
			group.end = logicalLine.end
			groups[key] = group
		} else {
			groups[key] = agentContextConfigurationRange{start: logicalLine.start, end: logicalLine.end}
		}
	}
	return groups
}

func agentContextConfigurationLogicalLines(path string, lines []string) []agentContextConfigurationLine {
	logicalLines := make([]agentContextConfigurationLine, 0, len(lines))
	if isAgentContextConfigurationYAML(path) {
		for index, line := range lines {
			lineNumber := index + 1
			logicalLines = append(logicalLines, agentContextConfigurationLine{
				content: line,
				start:   lineNumber,
				end:     lineNumber,
			})
		}
		return logicalLines
	}

	for index := 0; index < len(lines); index++ {
		logicalLine := agentContextConfigurationLine{
			content: lines[index],
			start:   index + 1,
			end:     index + 1,
		}
		if !agentContextPropertiesComment(logicalLine.content) &&
			agentContextPropertiesContinues(logicalLine.content) {
			content := append([]byte(nil), logicalLine.content[:len(logicalLine.content)-1]...)
			for agentContextPropertiesHasNextPhysicalLine(lines, index) {
				index++
				continuation := strings.TrimLeft(lines[index], " \t\f")
				continues := agentContextPropertiesContinues(continuation)
				if continues {
					continuation = continuation[:len(continuation)-1]
				}
				content = append(content, continuation...)
				logicalLine.end = index + 1
				if !continues {
					break
				}
			}
			logicalLine.content = string(content)
		}
		logicalLines = append(logicalLines, logicalLine)
	}
	return logicalLines
}

func agentContextPropertiesContinues(line string) bool {
	backslashes := 0
	for index := len(line) - 1; index >= 0 && line[index] == '\\'; index-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func agentContextPropertiesHasNextPhysicalLine(lines []string, index int) bool {
	next := index + 1
	return next < len(lines) && !(next == len(lines)-1 && lines[next] == "")
}

func agentContextPropertiesComment(line string) bool {
	trimmed := strings.TrimLeft(line, " \t\f")
	return strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!")
}

func agentContextConfigurationKey(path, line string) (string, int, bool) {
	if isAgentContextConfigurationYAML(path) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			return "", 0, false
		}
		colon := strings.Index(trimmed, ":")
		if colon <= 0 {
			return "", 0, false
		}
		key := strings.TrimSpace(strings.TrimPrefix(trimmed[:colon], "-"))
		if key == "" {
			return "", 0, false
		}
		return strings.Split(key, ".")[0], len(line) - len(strings.TrimLeft(line, " \t")), true
	}
	trimmed := strings.TrimLeft(line, " \t\f")
	if trimmed == "" || agentContextPropertiesComment(line) {
		return "", 0, false
	}
	delimiter := agentContextPropertiesDelimiter(trimmed)
	if delimiter == 0 {
		return "", 0, false
	}
	rawKey := trimmed
	if delimiter > 0 {
		rawKey = trimmed[:delimiter]
	}
	key, valid := agentContextPropertiesKeyGroup(rawKey)
	if !valid || key == "" {
		return "", 0, false
	}
	return key, 0, true
}

func agentContextPropertiesDelimiter(line string) int {
	escaped := false
	for index, character := range line {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '=' || character == ':' || character == ' ' || character == '\t' || character == '\f' {
			return index
		}
	}
	return -1
}

func agentContextPropertiesKeyGroup(rawKey string) (string, bool) {
	var group strings.Builder
	inGroup := true
	for index := 0; index < len(rawKey); {
		character, size := utf8.DecodeRuneInString(rawKey[index:])
		if character == utf8.RuneError && size == 1 {
			return "", false
		}
		index += size
		if character != '\\' {
			if inGroup {
				if character == '.' {
					inGroup = false
				} else {
					group.WriteRune(character)
				}
			}
			continue
		}

		if index == len(rawKey) {
			return "", false
		}
		escapedCharacter, escapedSize := utf8.DecodeRuneInString(rawKey[index:])
		if escapedCharacter == utf8.RuneError && escapedSize == 1 {
			return "", false
		}
		index += escapedSize
		switch escapedCharacter {
		case 't':
			escapedCharacter = '\t'
		case 'n':
			escapedCharacter = '\n'
		case 'r':
			escapedCharacter = '\r'
		case 'f':
			escapedCharacter = '\f'
		case 'u':
			var valid bool
			escapedCharacter, index, valid = agentContextPropertiesUnicodeEscape(rawKey, index)
			if !valid {
				return "", false
			}
		}
		if inGroup {
			group.WriteRune(escapedCharacter)
		}
	}
	return group.String(), true
}

func agentContextPropertiesUnicodeEscape(value string, index int) (rune, int, bool) {
	first, next, valid := agentContextPropertiesHexCodeUnit(value, index)
	if !valid {
		return 0, index, false
	}
	if first >= 0xd800 && first <= 0xdbff {
		if next+2 > len(value) || value[next] != '\\' || value[next+1] != 'u' {
			return 0, index, false
		}
		second, end, secondValid := agentContextPropertiesHexCodeUnit(value, next+2)
		if !secondValid || second < 0xdc00 || second > 0xdfff {
			return 0, index, false
		}
		return utf16.DecodeRune(rune(first), rune(second)), end, true
	}
	if first >= 0xdc00 && first <= 0xdfff {
		return 0, index, false
	}
	return rune(first), next, true
}

func agentContextPropertiesHexCodeUnit(value string, index int) (uint16, int, bool) {
	if index+4 > len(value) {
		return 0, index, false
	}
	var codeUnit uint16
	for _, digit := range []byte(value[index : index+4]) {
		codeUnit <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			codeUnit += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			codeUnit += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			codeUnit += uint16(digit-'A') + 10
		default:
			return 0, index, false
		}
	}
	return codeUnit, index + 4, true
}

func largestConfigurationParentIndent(parents map[int]string, indent int) int {
	parentIndent := -1
	for level := range parents {
		if level < indent && level > parentIndent {
			parentIndent = level
		}
	}
	return parentIndent
}

func appendAgentContextConfigurationFacts(index AgentContextIndexRecord, project string, facts []AgentContextFactRecord) AgentContextIndexRecord {
	for _, fact := range facts {
		fact.Project = project
		index.Facts = append(index.Facts, fact)
	}
	sortAgentContextFacts(index.Facts)
	return index
}

func isAgentContextConfigurationResource(value string) bool {
	base := filepath.Base(contextPathKey(value))
	extension := strings.ToLower(filepath.Ext(base))
	if extension != ".properties" && extension != ".yml" && extension != ".yaml" {
		return false
	}
	name := strings.TrimSuffix(base, extension)
	return name == "application" || name == "bootstrap" ||
		strings.HasPrefix(name, "application-") || strings.HasPrefix(name, "bootstrap-")
}

func isAgentContextConfigurationYAML(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".yml" || extension == ".yaml"
}
