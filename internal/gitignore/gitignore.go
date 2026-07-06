package gitignore

import (
	"os"
	"path/filepath"
	"strings"
)

type Matcher struct {
	patterns []pattern
}

type pattern struct {
	raw     string
	negate  bool
	dirOnly bool
	glob    bool
}

func Parse(body string) Matcher {
	var patterns []pattern
	for _, line := range strings.Split(body, "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		p := pattern{}
		if strings.HasPrefix(text, "!") {
			p.negate = true
			text = strings.TrimSpace(strings.TrimPrefix(text, "!"))
		}
		text = strings.TrimPrefix(text, "/")
		if strings.HasSuffix(text, "/") {
			p.dirOnly = true
			text = strings.TrimSuffix(text, "/")
		}
		p.raw = filepath.ToSlash(text)
		p.glob = strings.ContainsAny(p.raw, "*?[")
		if p.raw != "" {
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
	ignored := false
	for _, p := range m.patterns {
		if p.matches(rel, isDir) {
			ignored = !p.negate
		}
	}
	return ignored
}

func (p pattern) matches(rel string, isDir bool) bool {
	if p.dirOnly {
		return rel == p.raw || strings.HasPrefix(rel, p.raw+"/")
	}
	if p.glob {
		ok, _ := filepath.Match(p.raw, rel)
		if ok {
			return true
		}
		ok, _ = filepath.Match(p.raw, filepath.Base(rel))
		return ok
	}
	return rel == p.raw || filepath.Base(rel) == p.raw || strings.HasPrefix(rel, p.raw+"/")
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
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == entry {
			return false, nil
		}
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
	b.WriteString(entry)
	b.WriteString("\n")
	return true, os.WriteFile(path, []byte(b.String()), 0o644)
}
