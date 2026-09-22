package scan

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// AgentAuditReference is a statically identified, project-relative source link.
type AgentAuditReference struct {
	File string `json:"file"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
}

// AgentAuditSource records tooling source identity without copying source values.
type AgentAuditSource struct {
	Project       string                `json:"project,omitempty"`
	File          string                `json:"file"`
	Kind          string                `json:"kind"`
	Lines         int                   `json:"lines"`
	Hash          string                `json:"hash"`
	Topics        []string              `json:"topics,omitempty"`
	References    []AgentAuditReference `json:"references,omitempty"`
	Unknown       []string              `json:"unknown,omitempty"`
	rawReferences []AgentAuditReference
}

var auditCommandPath = regexp.MustCompile(`[A-Za-z0-9_./@-]+\.(?:[cm]?[jt]sx?|ya?ml)\b`)
var auditLocalInclude = regexp.MustCompile(`^\s*(?:-\s*)?local\s*:\s*(.+)$`)

func extractAgentAuditSource(file FileRecord, body string) (AgentAuditSource, bool) {
	p := contextPathKey(file.Path)
	ext := strings.ToLower(path.Ext(p))
	kind := ""
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		kind = "code"
	case ".md", ".mdx":
		kind = "documentation"
	case ".yml", ".yaml":
		kind = "ci"
	case ".json":
		if path.Base(p) == "package.json" {
			kind = "package"
		}
	}
	if kind == "" {
		return AgentAuditSource{}, false
	}
	if IsStorybookStorySource(p) {
		kind = "story"
	}
	if IsStorybookConfigurationSource(p) {
		kind = "configuration"
	}
	lower := strings.ToLower(p + "\n" + body)
	record := AgentAuditSource{File: p, Kind: kind, Hash: file.Hash, Lines: strings.Count(strings.TrimSuffix(strings.ReplaceAll(body, "\r\n", "\n"), "\n"), "\n") + 1}
	for _, topic := range []string{"storybook", "playwright", "vitest"} {
		if strings.Contains(lower, topic) {
			record.Topics = append(record.Topics, topic)
		}
	}
	if kind == "story" {
		record.Topics = appendUniqueContextID(record.Topics, "storybook")
	}
	if kind == "ci" {
		record.rawReferences, record.Unknown = auditCIIncludes(body)
	} else if kind == "package" {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal([]byte(body), &pkg) == nil {
			names := make([]string, 0, len(pkg.Scripts))
			for name := range pkg.Scripts {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				command := pkg.Scripts[name]
				encoded, _ := json.Marshal(command)
				line := 1
				if offset := strings.Index(body, string(encoded)); offset >= 0 {
					line += strings.Count(body[:offset], "\n")
				}
				for _, value := range auditCommandPath.FindAllString(command, -1) {
					record.rawReferences = append(record.rawReferences, AgentAuditReference{File: value, Kind: "script", Line: line})
				}
			}
		} else {
			record.Unknown = append(record.Unknown, "package manifest could not be parsed")
		}
	} else if kind != "documentation" {
		record.rawReferences = auditScriptPaths(body)
		if len(record.rawReferences) >= 256 {
			record.Unknown = append(record.Unknown, "literal reference limit reached")
		}
	}
	return record, true
}

// auditScriptPaths ignores comments and reads only literal path candidates. It
// never executes JavaScript or treats dynamic expressions as resolved paths.
func auditScriptPaths(body string) []AgentAuditReference {
	var refs []AgentAuditReference
	line := 1
	for i := 0; i < len(body); {
		if body[i] == '\n' {
			line++
			i++
			continue
		}
		if i+1 < len(body) && body[i:i+2] == "//" {
			for i < len(body) && body[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(body) && body[i:i+2] == "/*" {
			i += 2
			for i < len(body) {
				if i+1 < len(body) && body[i:i+2] == "*/" {
					i += 2
					break
				}
				if body[i] == '\n' {
					line++
				}
				i++
			}
			continue
		}
		quote := body[i]
		if quote != '\'' && quote != '"' && quote != '`' {
			i++
			continue
		}
		startLine := line
		i++
		start := i
		escaped := false
		for i < len(body) && body[i] != quote {
			if body[i] == '\\' {
				escaped = true
				i++
				if i >= len(body) {
					break
				}
			}
			if body[i] == '\n' {
				line++
			}
			i++
		}
		value := body[start:i]
		closed := i < len(body)
		if i < len(body) {
			i++
		}
		if closed && !escaped && !strings.ContainsAny(value, "${}\n\r") && (strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || auditCommandPath.MatchString(value) || strings.Contains(value, "*")) {
			if len(refs) < 256 {
				refs = append(refs, AgentAuditReference{File: value, Kind: "reference", Line: startLine})
			}
		}
	}
	return refs
}

func auditCIIncludes(body string) ([]AgentAuditReference, []string) {
	var refs []AgentAuditReference
	var unknown []string
	inInclude := false
	for i, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, "include:") {
			inInclude = true
			value := strings.TrimSpace(strings.TrimPrefix(line, "include:"))
			if value != "" {
				if strings.HasPrefix(value, "'") || strings.HasPrefix(value, "\"") {
					refs = append(refs, AgentAuditReference{File: strings.Trim(value, "'\""), Kind: "include", Line: i + 1})
				} else {
					unknown = appendUniqueContextID(unknown, "inline or dynamic CI include requires verification")
				}
			}
			continue
		}
		if inInclude && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '-' {
			inInclude = false
		}
		if !inInclude {
			continue
		}
		if match := auditLocalInclude.FindStringSubmatch(line); len(match) > 0 {
			value := strings.TrimSpace(strings.SplitN(match[1], " #", 2)[0])
			value = strings.Trim(value, "'\"")
			if strings.ContainsAny(value, "${}") {
				unknown = appendUniqueContextID(unknown, "dynamic CI include requires verification")
			} else {
				refs = append(refs, AgentAuditReference{File: value, Kind: "include", Line: i + 1})
			}
		} else if strings.HasPrefix(trimmed, "- '") || strings.HasPrefix(trimmed, "- \"") {
			refs = append(refs, AgentAuditReference{File: strings.Trim(strings.TrimSpace(trimmed[1:]), "'\""), Kind: "include", Line: i + 1})
		} else if strings.Contains(trimmed, ":") {
			unknown = appendUniqueContextID(unknown, "external or conditional CI include remains unknown")
		}
	}
	return refs, unknown
}

func finalizeAgentAuditSources(sources []AgentAuditSource, project string) []AgentAuditSource {
	known := map[string]bool{}
	for _, source := range sources {
		known[source.File] = true
	}
	for i := range sources {
		source := &sources[i]
		source.Project = project
		if len(source.rawReferences) > 256 {
			source.rawReferences = source.rawReferences[:256]
			source.Unknown = appendUniqueContextID(source.Unknown, "literal reference limit reached")
		}
		for _, ref := range source.rawReferences {
			bases := []string{path.Join(path.Dir(source.File), ref.File), ref.File}
			if strings.HasPrefix(ref.File, "./") || strings.HasPrefix(ref.File, "../") {
				bases = bases[:1]
			}
			if ref.Kind == "include" {
				bases = []string{strings.TrimPrefix(ref.File, "/")}
			}
			matches := map[string]bool{}
			for _, base := range bases {
				if base == ".." || strings.HasPrefix(base, "../") || strings.Contains(base, ":") {
					continue
				}
				if known[base] {
					matches[base] = true
					continue
				}
				if strings.ContainsAny(base, "*?") {
					matcher := compileAuditGlob(base)
					for candidate := range known {
						if matcher.MatchString(candidate) {
							matches[candidate] = true
						}
					}
					continue
				}
				for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"} {
					for _, candidate := range []string{base + ext, path.Join(base, "index"+ext)} {
						if known[candidate] {
							matches[candidate] = true
						}
					}
				}
			}
			if len(matches) > 1 && !strings.ContainsAny(ref.File, "*?") {
				source.Unknown = appendUniqueContextID(source.Unknown, "ambiguous local source reference remains unknown")
				continue
			}
			if len(matches) == 0 && (ref.Kind == "include" || strings.HasPrefix(ref.File, "./") || strings.HasPrefix(ref.File, "../")) {
				source.Unknown = appendUniqueContextID(source.Unknown, "local source reference is missing or excluded from the index")
			}
			for target := range matches {
				if target != source.File {
					source.References = append(source.References, AgentAuditReference{File: target, Kind: ref.Kind, Line: ref.Line})
				}
			}
		}
		sort.Slice(source.References, func(a, b int) bool {
			l, r := source.References[a], source.References[b]
			if l.File != r.File {
				return l.File < r.File
			}
			return l.Line < r.Line
		})
		if len(source.References) > 256 {
			source.References = source.References[:256]
			source.Unknown = appendUniqueContextID(source.Unknown, "audit reference limit reached")
		}
		source.rawReferences = nil
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].File < sources[j].File })
	return sources
}

func compileAuditGlob(pattern string) *regexp.Regexp {
	var expression strings.Builder
	expression.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					expression.WriteString("(?:.*/)?")
				} else {
					expression.WriteString(".*")
				}
			} else {
				expression.WriteString("[^/]*")
			}
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
	}
	expression.WriteString("$")
	return regexp.MustCompile(expression.String())
}
