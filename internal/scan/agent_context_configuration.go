package scan

import (
	"path/filepath"
	"sort"
	"strings"
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

func agentContextConfigurationKeyGroups(path string, lines []string) map[string]agentContextConfigurationRange {
	groups := map[string]agentContextConfigurationRange{}
	yamlParents := map[int]string{}
	for index, line := range lines {
		lineNumber := index + 1
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
			group.end = lineNumber
			groups[key] = group
		} else {
			groups[key] = agentContextConfigurationRange{start: lineNumber, end: lineNumber}
		}
	}
	return groups
}

func agentContextConfigurationKey(path, line string) (string, int, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
		return "", 0, false
	}
	if isAgentContextConfigurationYAML(path) {
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
	delimiter := agentContextPropertiesDelimiter(trimmed)
	if delimiter <= 0 {
		return "", 0, false
	}
	key := agentContextPropertiesKeyGroup(trimmed[:delimiter])
	if key == "" {
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

func agentContextPropertiesKeyGroup(rawKey string) string {
	var group strings.Builder
	escaped := false
	for _, character := range rawKey {
		if escaped {
			group.WriteRune(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '.' {
			break
		}
		group.WriteRune(character)
	}
	if escaped {
		group.WriteByte('\\')
	}
	return group.String()
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
