// Package languageexport provides opt-in compiler exporters without executing them.
package languageexport

import (
	"embed"
	"fmt"
)

//go:embed scripts/*
var scripts embed.FS

// Files returns a standalone exporter bundle for an explicitly selected language.
func Files(language string) (map[string][]byte, error) {
	names := []string{}
	switch language {
	case "csharp":
		names = []string{"Program.cs", "Exporter.csproj", "NuGet.Config"}
	case "swift":
		names = []string{"sourcekit.py"}
	default:
		return nil, fmt.Errorf("unsupported language exporter %q", language)
	}
	result := map[string][]byte{}
	for _, name := range names {
		body, err := scripts.ReadFile("scripts/" + name)
		if err != nil {
			return nil, err
		}
		result[name] = body
	}
	return result, nil
}
