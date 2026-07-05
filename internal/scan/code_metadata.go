package scan

import (
	"path/filepath"
	"strings"
)

func codeFileApp(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) >= 2 && parts[0] == "apps" && parts[1] != "" {
		return parts[1]
	}
	return "workspace-root"
}

func codeFilePackage(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) >= 2 {
		switch parts[0] {
		case "apps", "packages":
			return parts[1]
		}
	}
	return ""
}

func codeRouteID(app, path string) string {
	if app == "" {
		app = "workspace-root"
	}
	return app + ":" + normalizeCodeRoutePath(path)
}

func isLowSignalCodeFile(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	segments := strings.Split(normalized, "/")
	for _, segment := range segments {
		switch {
		case segment == "node_modules", segment == "vendor", segment == "dist", segment == "build", segment == "coverage":
			return true
		case segment == "__generated__", segment == "generated":
			return true
		case strings.Contains(segment, "storybook") || strings.Contains(segment, "archive"):
			return true
		}
	}
	return strings.HasSuffix(normalized, ".d.ts") || strings.HasSuffix(normalized, ".min.js")
}

func isLowValueCallTarget(method string) bool {
	switch method {
	case "", "find", "text", "match", "push", "block", "simulate", "props", "state", "set", "get", "map", "filter", "reduce", "forEach", "then", "catch", "finally":
		return true
	default:
		return isCodeKeyword(method)
	}
}
