package gitignore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Matcher struct {
	patterns []pattern
}

type pattern struct {
	scope    string
	match    *regexp.Regexp
	negate   bool
	dirOnly  bool
	basename bool
}

func Parse(body string) Matcher {
	return (Matcher{}).WithFile("", body)
}

// WithFile returns a matcher extended by an ignore file in a root-relative directory.
func (m Matcher) WithFile(directory, body string) Matcher {
	patterns := append([]pattern(nil), m.patterns...)
	for _, line := range strings.Split(body, "\n") {
		text := trimIgnoreTrailingSpaces(strings.TrimSuffix(line, "\r"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		p := pattern{scope: strings.Trim(filepath.ToSlash(directory), "/")}
		if strings.HasPrefix(text, "!") {
			p.negate = true
			text = strings.TrimPrefix(text, "!")
		}
		anchored := strings.HasPrefix(text, "/")
		text = strings.TrimPrefix(text, "/")
		if strings.HasSuffix(text, "/") {
			p.dirOnly = true
			text = strings.TrimSuffix(text, "/")
		}
		p.basename = !anchored && !strings.Contains(text, "/")
		p.match = compileIgnorePattern(text)
		if text != "" && p.match != nil {
			patterns = append(patterns, p)
		}
	}
	return Matcher{patterns: patterns}
}

func Load(root string) Matcher {
	body, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return Matcher{}
	}
	return Parse(string(body))
}

func (m Matcher) Ignored(rel string, isDir bool) bool {
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))
	parts := strings.Split(rel, "/")
	for end := 1; end <= len(parts); end++ {
		candidate := strings.Join(parts[:end], "/")
		directory := end < len(parts) || isDir
		ignored := false
		for _, p := range m.patterns {
			if p.matches(candidate, directory) {
				ignored = !p.negate
			}
		}
		if ignored {
			return true
		}
	}
	return false
}

func (p pattern) matches(rel string, isDir bool) bool {
	if p.dirOnly && !isDir {
		return false
	}
	if p.scope != "" {
		var ok bool
		rel, ok = strings.CutPrefix(rel, p.scope+"/")
		if !ok {
			return false
		}
	}
	if p.basename {
		rel = filepath.Base(rel)
	}
	return p.match.MatchString(rel)
}

func trimIgnoreTrailingSpaces(value string) string {
	for strings.HasSuffix(value, " ") {
		backslashes := 0
		for i := len(value) - 2; i >= 0 && value[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 == 1 {
			break
		}
		value = value[:len(value)-1]
	}
	return value
}

func compileIgnorePattern(pattern string) *regexp.Regexp {
	var expression strings.Builder
	expression.WriteByte('^')
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++
			if i == len(pattern) {
				return nil
			}
			expression.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		case '*':
			start := i
			for i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
			}
			if i > start && (start == 0 || pattern[start-1] == '/') && (i+1 == len(pattern) || pattern[i+1] == '/') {
				if i+1 < len(pattern) {
					expression.WriteString("(?:.*/)?")
					i++
				} else {
					expression.WriteString(".*")
				}
			} else {
				expression.WriteString("[^/]*")
			}
		case '?':
			expression.WriteString("[^/]")
		case '[':
			end := i + 1
			if end < len(pattern) && (pattern[end] == '!' || pattern[end] == '^') {
				end++
			}
			if end < len(pattern) && pattern[end] == ']' {
				end++
			}
			for end < len(pattern) && pattern[end] != ']' {
				if strings.HasPrefix(pattern[end:], "[:") {
					if offset := strings.Index(pattern[end+2:], ":]"); offset >= 0 {
						end += offset + 4
						continue
					}
				}
				if pattern[end] == '\\' && end+1 < len(pattern) {
					end++
				}
				end++
			}
			if end == len(pattern) {
				return nil
			}
			class := pattern[i+1 : end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			if strings.HasPrefix(class, "^") {
				class = "^/" + class[1:]
			}
			expression.WriteString("[" + class + "]")
			i = end
		default:
			expression.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
	}
	expression.WriteByte('$')
	compiled, err := regexp.Compile(expression.String())
	if err != nil {
		return nil
	}
	return compiled
}

func EnsureOutputIgnored(root, outputDir string) (bool, error) {
	entry := strings.TrimSuffix(outputDir, "/") + "/"
	return ensureEntryIgnored(root, entry, "# GoreGraph local scan output")
}

func EnsureWorkspaceIgnored(root string) (bool, error) {
	return ensureEntryIgnored(root, ".goregraph-workspace/", "# GoreGraph local workspace output")
}

func ensureEntryIgnored(root, entry, comment string) (bool, error) {
	path := filepath.Join(root, ".gitignore")
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	text := string(body)
	entries := []string{entry, ".goregraph-lock-*", ".goregraph-journal-*", ".goregraph-stage-*", ".goregraph-backup-*"}
	present := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		present[strings.TrimSpace(line)] = true
	}
	missing := []string{}
	for _, candidate := range entries {
		if !present[candidate] {
			missing = append(missing, candidate)
		}
	}
	if len(missing) == 0 {
		return false, nil
	}

	var b strings.Builder
	b.WriteString(text)
	if text != "" && !strings.HasSuffix(text, "\n") {
		b.WriteString("\n")
	}
	if text != "" && !strings.HasSuffix(text, "\n\n") {
		b.WriteString("\n")
	}
	b.WriteString(comment)
	b.WriteString("\n")
	b.WriteString(strings.Join(missing, "\n"))
	b.WriteString("\n")
	return true, os.WriteFile(path, []byte(b.String()), 0o644)
}
