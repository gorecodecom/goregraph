package scan

import (
	"fmt"
	"sort"
	"strings"
)

func renderRoutesReport(routes []CodeRouteRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Routes\n\n")
	if len(routes) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, route := range routes {
		rendered := ""
		if len(route.RenderedComponents) > 0 {
			rendered = fmt.Sprintf(", renders `%s`", strings.Join(route.RenderedComponents, "`, `"))
		}
		b.WriteString(fmt.Sprintf("- `%s` %s `%s` -> `%s` (%s, %s, route `%s`, `%s:%d`, %s%s)\n",
			route.Kind,
			route.HTTPMethod,
			route.Path,
			emptyAsNone(route.Handler),
			route.Framework,
			route.Language,
			route.RouteID,
			route.File,
			route.Line,
			route.Confidence,
			rendered,
		))
	}
	return b.String()
}

func renderCodeFlowsReport(flows []CodeFlowRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Flows\n\n")
	if len(flows) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, flow := range flows {
		b.WriteString(fmt.Sprintf("## %s `%s`\n\n", flow.HTTPMethod, flow.Path))
		if flow.RouteID != "" {
			b.WriteString(fmt.Sprintf("- Route ID: `%s`\n", flow.RouteID))
		}
		b.WriteString(fmt.Sprintf("- Framework: %s\n", flow.Framework))
		b.WriteString(fmt.Sprintf("- Language: %s\n", flow.Language))
		b.WriteString(fmt.Sprintf("- Entry: `%s:%d`\n", flow.File, flow.Line))
		b.WriteString("\n")
		for _, step := range flow.Steps {
			name := step.Name
			if step.Owner != "" && !strings.Contains(name, ".") {
				name = step.Owner + "." + name
			}
			location := ""
			if step.File != "" {
				location = fmt.Sprintf(" - `%s:%d`", step.File, step.Line)
			}
			b.WriteString(fmt.Sprintf("- `%s` (%s, %s)%s\n", name, emptyAsNone(step.Kind), step.Confidence, location))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderNavigationReport(files []FileRecord, symbols []SymbolRecord, relations []RelationRecord, routes []CodeRouteRecord, flows []CodeFlowRecord, tests []TestMapRecord, analyzers []AnalyzerRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Navigation\n\n")
	b.WriteString("## Where To Start\n\n")
	if len(routes) == 0 {
		b.WriteString("- No routes detected. Start with `entrypoints.md`, `modules.md`, and `report.md`.\n")
	} else {
		limit := minInt(len(routes), 20)
		for _, route := range routes[:limit] {
			b.WriteString(fmt.Sprintf("- %s `%s` -> `%s` in `%s:%d` (%s)\n", route.HTTPMethod, route.Path, emptyAsNone(route.Handler), route.File, route.Line, route.Framework))
		}
	}
	b.WriteString("\n## Most Connected Files\n\n")
	connected := mostConnectedFiles(files, relations, routes, flows)
	if len(connected) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, item := range connected {
			b.WriteString(fmt.Sprintf("- `%s` - %d links\n", item.path, item.count))
		}
	}
	b.WriteString("\n## Important Symbols\n\n")
	important := importantSymbols(symbols)
	if len(important) == 0 {
		b.WriteString("- none detected\n")
	} else {
		for _, symbol := range important {
			b.WriteString(fmt.Sprintf("- `%s` (%s) in `%s:%d`\n", symbol.Name, symbol.Kind, symbol.File, symbol.Line))
		}
	}
	b.WriteString("\n## Test Orientation\n\n")
	if len(tests) == 0 {
		b.WriteString("- none detected\n")
	} else {
		limit := minInt(len(tests), 20)
		for _, test := range tests[:limit] {
			b.WriteString(fmt.Sprintf("- `%s` checks `%s` (%s)\n", test.TestMethod, emptyAsNone(test.TargetMethod), test.Confidence))
		}
	}
	b.WriteString("\n## Analyzer Coverage\n\n")
	for _, analyzer := range analyzers {
		b.WriteString(fmt.Sprintf("- `%s`: %s\n", analyzer.Language, analyzerSummary(analyzer)))
	}
	return b.String()
}

type connectedFile struct {
	path  string
	count int
}

func mostConnectedFiles(files []FileRecord, relations []RelationRecord, routes []CodeRouteRecord, flows []CodeFlowRecord) []connectedFile {
	counts := map[string]int{}
	fileSet := map[string]bool{}
	for _, file := range files {
		fileSet[file.Path] = true
	}
	for _, relation := range relations {
		if fileSet[relation.From] {
			counts[relation.From]++
		}
		if fileSet[relation.To] {
			counts[relation.To]++
		}
	}
	for _, route := range routes {
		counts[route.File] += 2
	}
	for _, flow := range flows {
		counts[flow.File]++
		for _, step := range flow.Steps {
			if step.File != "" {
				counts[step.File]++
			}
		}
	}
	var connected []connectedFile
	for path, count := range counts {
		if count > 0 {
			connected = append(connected, connectedFile{path: path, count: count})
		}
	}
	sort.Slice(connected, func(i, j int) bool {
		if connected[i].count != connected[j].count {
			return connected[i].count > connected[j].count
		}
		return connected[i].path < connected[j].path
	})
	if len(connected) > 20 {
		return connected[:20]
	}
	return connected
}

func importantSymbols(symbols []SymbolRecord) []SymbolRecord {
	var result []SymbolRecord
	for _, symbol := range symbols {
		switch symbol.Kind {
		case "entrypoint", "class", "interface", "trait", "component", "function", "method", "test":
			result = append(result, symbol)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].File != result[j].File {
			return result[i].File < result[j].File
		}
		return result[i].Line < result[j].Line
	})
	if len(result) > 30 {
		return result[:30]
	}
	return result
}

func analyzerSummary(record AnalyzerRecord) string {
	var parts []string
	if record.Symbols {
		parts = append(parts, "symbols")
	}
	if record.Relations {
		parts = append(parts, "relations")
	}
	if record.Calls {
		parts = append(parts, "calls")
	}
	if record.Endpoints {
		parts = append(parts, "routes")
	}
	if record.Tests {
		parts = append(parts, "tests")
	}
	if record.Workspace {
		parts = append(parts, "workspace")
	}
	if len(parts) == 0 {
		return "inventory"
	}
	return strings.Join(parts, ", ")
}

func emptyAsNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
