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
		"go":         {Language: "go", Scope: "language+routes", Symbols: true, Relations: true, Calls: true, Endpoints: true, Tests: true, Outputs: []string{"symbols.json", "relations.json", "callgraph.json", "routes.json", "flows.json", "test-map.json", "graph-full.json"}},
		"java":       {Language: "java", Scope: "language+spring", Symbols: true, Relations: true, Calls: true, Endpoints: true, Tests: true, Outputs: []string{"symbols-full.json", "relations-full.json", "spring.json", "callgraph.json", "endpoint-flows.json", "test-map.json"}},
		"javascript": {Language: "javascript", Scope: "language+routes+api", Symbols: true, Relations: true, Calls: true, Endpoints: true, Tests: true, Outputs: []string{"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json", "flows.json", "api-contracts.json", "test-map.json", "graph-full.json"}},
		"typescript": {Language: "typescript", Scope: "language+react+routes+api", Symbols: true, Relations: true, Calls: true, Endpoints: true, Tests: true, Outputs: []string{"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json", "flows.json", "api-contracts.json", "test-map.json", "graph-full.json"}},
		"python":     {Language: "python", Scope: "language+routes", Symbols: true, Relations: true, Calls: true, Endpoints: true, Tests: true, Outputs: []string{"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json", "flows.json", "test-map.json", "graph-full.json"}},
		"php":        {Language: "php", Scope: "language+routes", Symbols: true, Relations: true, Calls: true, Endpoints: true, Tests: true, Outputs: []string{"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json", "flows.json", "test-map.json", "graph-full.json"}},
		"shell":      {Language: "shell", Scope: "language", Symbols: true, Relations: true, Calls: true, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "callgraph.json", "flows.json", "graph-full.json"}},
		"rust":       {Language: "rust", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"kotlin":     {Language: "kotlin", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"scala":      {Language: "scala", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"swift":      {Language: "swift", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"ruby":       {Language: "ruby", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"c":          {Language: "c", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"cpp":        {Language: "cpp", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"csharp":     {Language: "csharp", Scope: "language", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"symbols-full.json", "relations-full.json", "graph-full.json"}},
		"markdown":   {Language: "markdown", Scope: "document", Symbols: false, Relations: false, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"files.json", "report.md"}},
		"json":       {Language: "json", Scope: "metadata", Symbols: true, Relations: false, Calls: false, Endpoints: false, Tests: false, Workspace: true, Outputs: []string{"workspace.md"}},
		"yaml":       {Language: "yaml", Scope: "metadata", Symbols: false, Relations: false, Calls: false, Endpoints: false, Tests: false, Outputs: []string{"files.json"}},
		"maven":      {Language: "maven", Scope: "workspace+dependencies", Symbols: false, Relations: true, Calls: false, Endpoints: false, Tests: false, Workspace: true, Outputs: []string{"workspace.md", "maven-graph.json", "maven-graph.md"}},
		"node":       {Language: "node", Scope: "workspace", Symbols: true, Relations: true, Calls: false, Endpoints: false, Tests: false, Workspace: true, Outputs: []string{"workspace.md", "package-graph.json", "package-graph.md", "entrypoints.md"}},
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
