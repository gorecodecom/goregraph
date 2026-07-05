package query

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSearchFindsMatchingSymbolsAndFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Search(root, "StartServer")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if !strings.Contains(result, "StartServer") {
		t.Fatalf("Search result missing symbol:\n%s", result)
	}
	if !strings.Contains(result, "src/main.go") {
		t.Fatalf("Search result missing file:\n%s", result)
	}
}

func TestSearchReadsGeneratedOutputAliases(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Search(root, "graph-full")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if !strings.Contains(result, `"directed"`) || !strings.Contains(result, "StartServer") {
		t.Fatalf("graph-full alias returned unexpected output:\n%s", result)
	}

	audit, err := Search(root, "audit")
	if err != nil {
		t.Fatalf("Search audit returned error: %v", err)
	}
	if !strings.Contains(audit, `"network_used": false`) || !strings.Contains(audit, `"external_commands": false`) {
		t.Fatalf("audit alias missing safety fields:\n%s", audit)
	}

	analyzers, err := Search(root, "analyzers-json")
	if err != nil {
		t.Fatalf("Search analyzers-json returned error: %v", err)
	}
	if !strings.Contains(analyzers, `"language": "go"`) {
		t.Fatalf("analyzers-json alias missing Go analyzer:\n%s", analyzers)
	}

	routes, err := Search(root, "routes")
	if err != nil {
		t.Fatalf("Search routes returned error: %v", err)
	}
	if !strings.Contains(routes, "# GoreGraph Routes") {
		t.Fatalf("routes alias returned unexpected output:\n%s", routes)
	}

	flows, err := Search(root, "flows-json")
	if err != nil {
		t.Fatalf("Search flows-json returned error: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(flows), "[") {
		t.Fatalf("flows-json alias returned unexpected output:\n%s", flows)
	}

	apiContracts, err := Search(root, "api-contracts")
	if err != nil {
		t.Fatalf("Search api-contracts returned error: %v", err)
	}
	if !strings.Contains(apiContracts, "# GoreGraph API Contracts") {
		t.Fatalf("api-contracts alias returned unexpected output:\n%s", apiContracts)
	}

	packageGraph, err := Search(root, "package-graph-json")
	if err != nil {
		t.Fatalf("Search package-graph-json returned error: %v", err)
	}
	if !strings.Contains(packageGraph, `"nodes"`) {
		t.Fatalf("package-graph-json alias returned unexpected output:\n%s", packageGraph)
	}

	navigation, err := Search(root, "navigation")
	if err != nil {
		t.Fatalf("Search navigation returned error: %v", err)
	}
	if !strings.Contains(navigation, "# GoreGraph Navigation") {
		t.Fatalf("navigation alias returned unexpected output:\n%s", navigation)
	}
}

func TestExplainFileShowsSymbolsAndRelations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nimport \"fmt\"\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Explain(root, "src/main.go")
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}

	if !strings.Contains(result, "src/main.go") {
		t.Fatalf("Explain result missing file:\n%s", result)
	}
	if !strings.Contains(result, "StartServer") {
		t.Fatalf("Explain result missing symbol:\n%s", result)
	}
	if !strings.Contains(result, "fmt") {
		t.Fatalf("Explain result missing import relation:\n%s", result)
	}
}

func TestExplainFileShowsInboundRelationsAndLikelyTests(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "internal/service/service.go", "package service\nfunc StartServer() {}\n")
	writeFile(t, root, "internal/service/service_test.go", "package service\nfunc TestStartServer() {}\n")
	writeFile(t, root, "cmd/api/main.go", "package main\nimport \"example.test/demo/internal/service\"\nfunc main() { service.StartServer() }\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Explain(root, "internal/service/service.go")
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}

	if !strings.Contains(result, "## Inbound Relations") {
		t.Fatalf("Explain result missing inbound section:\n%s", result)
	}
	if !strings.Contains(result, "cmd/api/main.go") {
		t.Fatalf("Explain result missing importer:\n%s", result)
	}
	if !strings.Contains(result, "## Likely Tests") {
		t.Fatalf("Explain result missing likely tests section:\n%s", result)
	}
	if !strings.Contains(result, "internal/service/service_test.go") {
		t.Fatalf("Explain result missing likely test:\n%s", result)
	}
}

func TestSearchReportsNoMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	result, err := Search(root, "MissingThing")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if !strings.Contains(result, "No matches") {
		t.Fatalf("Search result missing no-match message:\n%s", result)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
