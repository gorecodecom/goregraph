package scan

import (
	"path/filepath"
	"strings"
)

func detectLanguage(rel string) string {
	if strings.HasSuffix(rel, ".goregraph-blender.json") {
		return "blender"
	}
	if strings.HasSuffix(rel, ".goregraph-unity.json") {
		return "unity"
	}
	switch strings.ToLower(filepath.Base(rel)) {
	case "go.mod":
		return "go"
	case "pubspec.lock":
		return "yaml"
	case "package.json":
		return "json"
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".rs":
		return "rust"
	case ".html", ".htm":
		return "html"
	case ".css", ".uss":
		return "css"
	case ".cmd", ".bat":
		return "batch"
	case ".m", ".mm":
		return "objectivec"
	case ".dart":
		return "dart"
	case ".swift":
		return "swift"
	case ".rb":
		return "ruby"
	case ".unity", ".prefab", ".asset", ".meta", ".mat", ".controller", ".overridecontroller", ".anim", ".playable", ".asmdef", ".asmref":
		return "unity"
	case ".blend":
		return "blender"
	case ".cs":
		return "csharp"
	case ".c", ".h":
		return "c"
	case ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".py":
		return "python"
	case ".php":
		return "php"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".csproj", ".props", ".targets":
		return "xml"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".sh", ".bash", ".zsh", ".command":
		return "shell"
	default:
		return "text"
	}
}

// IsSupportedSourceFile reports whether the scanner recognizes a file type.
func IsSupportedSourceFile(rel string) bool {
	return detectLanguage(rel) != "text"
}

func detectKind(rel string) string {
	base := strings.ToLower(filepath.Base(rel))
	switch base {
	case "pubspec.yaml", "go.mod", "package.json", "composer.json", "pom.xml", "build.gradle", "settings.gradle":
		return "build"
	case "readme.md":
		return "documentation"
	default:
		return "source"
	}
}

func detectSourceLanguage(file, body string) string {
	language := detectLanguage(file)
	if strings.EqualFold(filepath.Ext(file), ".h") {
		tokens, _ := cFamilyTokens(body)
		for at, token := range tokens {
			if token.text == "@" && at+1 < len(tokens) {
				switch tokens[at+1].text {
				case "interface", "implementation", "protocol", "class":
					return "objectivec"
				}
			}
		}
	}
	return language
}
