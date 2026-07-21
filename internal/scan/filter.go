package scan

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

func shouldSkipPath(rel string, isDir bool, cfg config.Config, matcher gitignore.Matcher) bool {
	if len(cfg.Include) > 0 && !includedByPatterns(rel, isDir, cfg.Include) {
		return true
	}
	for _, pattern := range cfg.Exclude {
		p := strings.TrimSuffix(filepath.ToSlash(pattern), "/")
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
		if isLiteralPathSegment(p) && containsPathSegment(rel, p) {
			return true
		}
	}
	return matcher.Ignored(rel, isDir)
}

func isLiteralPathSegment(pattern string) bool {
	return pattern != "" &&
		!strings.Contains(pattern, "/") &&
		!strings.ContainsAny(pattern, "*?[")
}

func containsPathSegment(rel, segment string) bool {
	for _, candidate := range strings.Split(filepath.ToSlash(rel), "/") {
		if candidate == segment {
			return true
		}
	}
	return false
}

func includedByPatterns(rel string, isDir bool, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			if isDir && strings.HasPrefix(prefix, rel+"/") {
				return true
			}
			continue
		}
		if strings.HasSuffix(pattern, "/") {
			prefix := strings.TrimSuffix(pattern, "/")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			if isDir && strings.HasPrefix(prefix, rel+"/") {
				return true
			}
			continue
		}
		if matched, _ := filepath.Match(pattern, rel); matched {
			return true
		}
		if isDir && strings.HasPrefix(pattern, rel+"/") {
			return true
		}
	}
	return false
}

func isBinary(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	if strings.Contains(string(body), "\x00") {
		return true
	}
	return !utf8.Valid(body)
}
