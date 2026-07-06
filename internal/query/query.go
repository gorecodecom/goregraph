package query

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func Search(root, term string) (string, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return "", fmt.Errorf("query term is required")
	}
	if name, ok := outputAliases[term]; ok {
		return ReadOutput(root, name)
	}
	files, symbols, relations, err := loadIndex(root)
	if err != nil {
		return "", err
	}
	lower := strings.ToLower(term)
	var lines []string
	lines = append(lines, fmt.Sprintf("# GoreGraph Query: %s", term), "")
	for _, file := range files {
		if strings.Contains(strings.ToLower(file.Path), lower) || strings.Contains(strings.ToLower(file.Language), lower) {
			lines = append(lines, fmt.Sprintf("- file `%s` (%s)", file.Path, file.Language))
		}
	}
	for _, symbol := range symbols {
		if strings.Contains(strings.ToLower(symbol.Name), lower) || strings.Contains(strings.ToLower(symbol.File), lower) {
			lines = append(lines, fmt.Sprintf("- symbol `%s` (%s) in `%s:%d`", symbol.Name, symbol.Kind, symbol.File, symbol.Line))
		}
	}
	for _, relation := range relations {
		if strings.Contains(strings.ToLower(relation.From), lower) || strings.Contains(strings.ToLower(relation.To), lower) {
			lines = append(lines, fmt.Sprintf("- relation `%s` --%s--> `%s`", relation.From, relation.Type, relation.To))
		}
	}
	if len(lines) == 2 {
		lines = append(lines, "No matches.")
	}
	return strings.Join(lines, "\n") + "\n", nil
}

var outputAliases = map[string]string{
	"files":                 "files.json",
	"symbols":               "symbols.json",
	"symbols-full":          "symbols-full.json",
	"relations":             "relations.json",
	"relations-full":        "relations-full.json",
	"graph":                 "graph.json",
	"graph-full":            "graph-full.json",
	"callgraph":             "callgraph.json",
	"callgraph-md":          "callgraph.md",
	"report":                "report.md",
	"modules":               "modules.md",
	"entrypoints":           "entrypoints.md",
	"tests":                 "test-map.md",
	"test-map":              "test-map.md",
	"test-map-json":         "test-map.json",
	"audit":                 "audit.json",
	"spring":                "spring.json",
	"routes":                "routes.md",
	"routes-json":           "routes.json",
	"flows":                 "flows.md",
	"flows-json":            "flows.json",
	"api-contracts":         "api-contracts.md",
	"api-contracts-json":    "api-contracts.json",
	"contract-matches":      "contract-matches.md",
	"contracts":             "contract-matches.md",
	"contract-matches-json": "contract-matches.json",
	"broken-contracts":      "potentially-broken-contracts.md",
	"diagnostics":           "diagnostics.md",
	"diagnostics-json":      "diagnostics.json",
	"package-graph":         "package-graph.md",
	"package-graph-json":    "package-graph.json",
	"maven-graph":           "maven-graph.md",
	"maven-graph-json":      "maven-graph.json",
	"navigation":            "navigation.md",
	"endpoints":             "endpoints.md",
	"endpoint-flows":        "endpoint-flows.md",
	"endpoint-flows-json":   "endpoint-flows.json",
	"dependencies":          "dependencies.md",
	"workspace":             "workspace.md",
	"workspace-context":     "workspace-context.md",
	"workspace-contracts":   "workspace-contract-matches.md",
	"frontend-consumers":    "frontend-consumers.md",
	"analyzers":             "analyzers.md",
	"analyzers-json":        "analyzers.json",
	"affected":              "affected.md",
}

var workspaceOutputFallbacks = map[string]string{
	"workspace-context.md":          "workspace-context.md",
	"workspace-contract-matches.md": "contract-matches.md",
}

func ReadOutput(root, name string) (string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, name))
	if err != nil {
		if os.IsNotExist(err) {
			if fallbackName, ok := workspaceOutputFallbacks[name]; ok {
				if body, fallbackErr := os.ReadFile(filepath.Join(root, ".goregraph-workspace", fallbackName)); fallbackErr == nil {
					return string(body), nil
				}
			}
			return "", fmt.Errorf("output %s is missing; run `goregraph scan <path>` first", name)
		}
		return "", err
	}
	return string(body), nil
}

func Explain(root, target string) (string, error) {
	target = strings.TrimSpace(filepath.ToSlash(target))
	if target == "" {
		return "", fmt.Errorf("explain target is required")
	}
	files, symbols, relations, err := loadIndex(root)
	if err != nil {
		return "", err
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("# GoreGraph Explain: %s", target), "")
	for _, file := range files {
		if file.Path == target {
			lines = append(lines, fmt.Sprintf("- file `%s` (%s, %d bytes)", file.Path, file.Language, file.Size))
		}
	}
	lines = append(lines, "", "## Symbols")
	count := 0
	for _, symbol := range symbols {
		if symbol.File == target || strings.EqualFold(symbol.Name, target) {
			lines = append(lines, fmt.Sprintf("- `%s` (%s) in `%s:%d`", symbol.Name, symbol.Kind, symbol.File, symbol.Line))
			count++
		}
	}
	if count == 0 {
		lines = append(lines, "- none")
	}
	lines = append(lines, "", "## Outbound Relations")
	count = 0
	for _, relation := range relations {
		if relation.From == target {
			lines = append(lines, fmt.Sprintf("- `%s` --%s--> `%s`", relation.From, relation.Type, relation.To))
			count++
		}
	}
	if count == 0 {
		lines = append(lines, "- none")
	}
	lines = append(lines, "", "## Inbound Relations")
	count = 0
	for _, relation := range relations {
		if relation.From != target && relationTargetsFile(relation, target) {
			lines = append(lines, fmt.Sprintf("- `%s` --%s--> `%s`", relation.From, relation.Type, relation.To))
			count++
		}
	}
	if count == 0 {
		lines = append(lines, "- none")
	}
	lines = append(lines, "", "## Likely Tests")
	count = 0
	for _, relation := range relations {
		if relation.Type == "tests" && relation.To == target {
			lines = append(lines, fmt.Sprintf("- `%s`", relation.From))
			count++
		}
	}
	if count == 0 {
		lines = append(lines, "- none")
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func loadIndex(root string) ([]scan.FileRecord, []scan.SymbolRecord, []scan.RelationRecord, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	out := filepath.Join(root, cfg.OutputDir)
	var files []scan.FileRecord
	var symbols []scan.SymbolRecord
	var relations []scan.RelationRecord
	if err := readJSON(filepath.Join(out, "files.json"), &files); err != nil {
		return nil, nil, nil, err
	}
	if err := readJSON(filepath.Join(out, "symbols.json"), &symbols); err != nil {
		return nil, nil, nil, err
	}
	if err := readJSON(filepath.Join(out, "relations.json"), &relations); err != nil {
		return nil, nil, nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].File != symbols[j].File {
			return symbols[i].File < symbols[j].File
		}
		if symbols[i].Line != symbols[j].Line {
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].Name < symbols[j].Name
	})
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].From != relations[j].From {
			return relations[i].From < relations[j].From
		}
		return relations[i].To < relations[j].To
	})
	return files, symbols, relations, nil
}

func readJSON(path string, dest any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("index is missing; run `goregraph scan <path>` first")
		}
		return err
	}
	return json.Unmarshal(body, dest)
}

func relationTargetsFile(relation scan.RelationRecord, target string) bool {
	if relation.To == target {
		return true
	}
	if relation.Type != "imports" {
		return false
	}
	targetDir := filepath.ToSlash(filepath.Dir(target))
	if targetDir == "." || targetDir == "" {
		return false
	}
	return strings.HasSuffix(relation.To, targetDir)
}
