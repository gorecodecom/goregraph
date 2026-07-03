package scan

import (
	"path/filepath"
	"strings"
)

func detectLanguage(rel string) string {
	switch strings.ToLower(filepath.Base(rel)) {
	case "go.mod":
		return "go"
	case "package.json":
		return "json"
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".py":
		return "python"
	case ".php":
		return "php"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".sh", ".bash", ".zsh":
		return "shell"
	default:
		return "text"
	}
}

func detectKind(rel string) string {
	base := strings.ToLower(filepath.Base(rel))
	switch base {
	case "go.mod", "package.json", "composer.json", "pom.xml", "build.gradle", "settings.gradle":
		return "build"
	case "readme.md":
		return "documentation"
	default:
		return "source"
	}
}
