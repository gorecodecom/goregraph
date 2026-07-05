package scan

import (
	"fmt"
	"sort"
	"strings"
)

func buildAnalyzerInventory(files []FileRecord, workspace WorkspaceIndex) []AnalyzerRecord {
	seenLanguages := map[string]bool{}
	for _, file := range files {
		seenLanguages[file.Language] = true
	}
	if len(workspace.MavenPackages) > 0 {
		seenLanguages["maven"] = true
	}
	if len(workspace.NodePackages) > 0 {
		seenLanguages["node"] = true
	}

	capabilities := analyzerCapabilities()
	var records []AnalyzerRecord
	for language := range seenLanguages {
		if language == "" || language == "unknown" {
			continue
		}
		record, ok := capabilities[language]
		if !ok {
			record = AnalyzerRecord{Language: language, Scope: "file", Symbols: true, Relations: false}
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Language < records[j].Language })
	return records
}

func analyzerCapabilities() map[string]AnalyzerRecord {
	return map[string]AnalyzerRecord{
		"go":         {Language: "go", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: true, Outputs: []string{"symbols.json", "relations.json", "graph-full.json"}},
		"java":       {Language: "java", Scope: "language+spring", Symbols: true, Relations: true, Calls: true, Endpoints: true, Tests: true, Outputs: []string{"symbols-full.json", "relations-full.json", "spring.json", "callgraph.json", "endpoint-flows.json", "test-map.json"}},
		"javascript": {Language: "javascript", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"typescript": {Language: "typescript", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"python":     {Language: "python", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"php":        {Language: "php", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"shell":      {Language: "shell", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"markdown":   {Language: "markdown", Scope: "document", Symbols: false, Relations: false, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"files.json", "report.md"}},
		"json":       {Language: "json", Scope: "metadata", Symbols: true, Relations: false, Calls: false, Endpoints: false, Tests: false, Workspace: true, Outputs: []string{"workspace.md"}},
		"yaml":       {Language: "yaml", Scope: "metadata", Symbols: false, Relations: false, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"files.json"}},
		"maven":      {Language: "maven", Scope: "workspace", Symbols: false, Relations: false, Calls: false, Endpoints: false, Tests: false, Workspace: true, Outputs: []string{"workspace.md"}},
		"node":       {Language: "node", Scope: "workspace", Symbols: true, Relations: false, Calls: false, Endpoints: false, Tests: false, Workspace: true, Outputs: []string{"workspace.md", "entrypoints.md"}},
		"composer":   {Language: "composer", Scope: "workspace", Symbols: false, Relations: false, Calls: false, Endpoints: false, Tests: false, Workspace: true, Outputs: []string{"workspace.md"}},
	}
}

func renderAnalyzersReport(records []AnalyzerRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Analyzers\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		var capabilities []string
		if record.Symbols {
			capabilities = append(capabilities, "symbols")
		}
		if record.Relations {
			capabilities = append(capabilities, "relations")
		}
		if record.Calls {
			capabilities = append(capabilities, "calls")
		}
		if record.Endpoints {
			capabilities = append(capabilities, "endpoints")
		}
		if record.Tests {
			capabilities = append(capabilities, "tests")
		}
		if record.Workspace {
			capabilities = append(capabilities, "workspace")
		}
		if len(capabilities) == 0 {
			capabilities = append(capabilities, "inventory")
		}
		b.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", record.Language, record.Scope, strings.Join(capabilities, ", ")))
	}
	return b.String()
}
