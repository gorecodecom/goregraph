package scan

import (
	"encoding/json"
	"sort"
	"strings"
)

func extractPackageScripts(file FileRecord, body string) []SymbolRecord {
	var parsed struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil
	}
	var names []string
	for name := range parsed.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	symbols := make([]SymbolRecord, 0, len(names))
	for _, name := range names {
		symbols = append(symbols, SymbolRecord{Name: name, Kind: "script", File: file.Path, Line: 1})
	}
	return symbols
}

func extractComposerAutoloads(file FileRecord, body string) []SymbolRecord {
	var parsed struct {
		Autoload struct {
			PSR4 map[string]string `json:"psr-4"`
		} `json:"autoload"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil
	}
	var namespaces []string
	for namespace := range parsed.Autoload.PSR4 {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	symbols := make([]SymbolRecord, 0, len(namespaces))
	for _, namespace := range namespaces {
		symbols = append(symbols, SymbolRecord{Name: strings.TrimSuffix(namespace, `\`), Kind: "autoload", File: file.Path, Line: 1})
	}
	return symbols
}
