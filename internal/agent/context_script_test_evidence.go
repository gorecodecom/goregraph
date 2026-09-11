package agent

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var scriptTestRegistration = regexp.MustCompile(`\b(?:test|it)(?:\.(?:only|skip))?\s*\(`)

func scriptTestSourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func renderScriptTestSource(candidate sourceCandidate, file sourceFile) (ContextSourceSection, error) {
	raw := strings.Join(file.Lines, "\n")
	code := strings.Join(sourceCodeMask(file.Lines), "\n")
	var matches []ContextSourceSection
	for _, match := range scriptTestRegistration.FindAllStringIndex(code, -1) {
		open := match[1] - 1
		start := open + 1
		for start < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[start])) {
			start++
		}
		if start >= len(raw) || raw[start] != '"' && raw[start] != '\'' {
			continue
		}
		end := start + 1
		for end < len(raw) && raw[end] != raw[start] {
			if raw[end] == '\\' {
				end++
			}
			end++
		}
		if end >= len(raw) {
			continue
		}
		title := raw[start+1 : end]
		if raw[start] == '"' {
			var err error
			title, err = strconv.Unquote(raw[start : end+1])
			if err != nil {
				continue
			}
		} else if strings.Contains(title, "\\") {
			continue
		}
		if title != candidate.Name {
			continue
		}
		depth, close := 1, open+1
		for ; close < len(code); close++ {
			// The shared mask does not parse JavaScript regular expressions.
			// Refuse an uncertain boundary instead of returning a truncated test.
			if code[close] == '/' {
				break
			} else if code[close] == '(' {
				depth++
			} else if code[close] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if depth != 0 {
			continue
		}
		first := strings.Count(code[:match[0]], "\n") + 1
		last := strings.Count(code[:close+1], "\n") + 1
		if last-first+1 > 120 {
			continue
		}
		state := "relocated_current"
		if first == candidate.StartLine {
			state = "indexed_range_current"
		}
		matches = append(matches, ContextSourceSection{
			Project: candidate.Project, Path: candidate.Path, StartLine: first, EndLine: last,
			Role: candidate.Role, RenderMode: "declaration_body", SourceState: state,
			Content: renderNumberedSource(file.Lines, first, last),
		})
	}
	var anchored []ContextSourceSection
	for _, match := range matches {
		if match.StartLine == candidate.StartLine {
			anchored = append(anchored, match)
		}
	}
	if len(anchored) == 1 {
		return anchored[0], nil
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return ContextSourceSection{}, fmt.Errorf("indexed test registration is absent or ambiguous in current source")
}
