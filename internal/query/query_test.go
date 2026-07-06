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

	contractMatches, err := Search(root, "contract-matches")
	if err != nil {
		t.Fatalf("Search contract-matches returned error: %v", err)
	}
	if !strings.Contains(contractMatches, "# GoreGraph Contract Matches") {
		t.Fatalf("contract-matches alias returned unexpected output:\n%s", contractMatches)
	}

	brokenContracts, err := Search(root, "broken-contracts")
	if err != nil {
		t.Fatalf("Search broken-contracts returned error: %v", err)
	}
	if !strings.Contains(brokenContracts, "# GoreGraph Potentially Broken Contracts") {
		t.Fatalf("broken-contracts alias returned unexpected output:\n%s", brokenContracts)
	}

	packageGraph, err := Search(root, "package-graph-json")
	if err != nil {
		t.Fatalf("Search package-graph-json returned error: %v", err)
	}
	if !strings.Contains(packageGraph, `"nodes"`) {
		t.Fatalf("package-graph-json alias returned unexpected output:\n%s", packageGraph)
	}

	mavenGraph, err := Search(root, "maven-graph")
	if err != nil {
		t.Fatalf("Search maven-graph returned error: %v", err)
	}
	if !strings.Contains(mavenGraph, "# GoreGraph Maven Graph") {
		t.Fatalf("maven-graph alias returned unexpected output:\n%s", mavenGraph)
	}

	navigation, err := Search(root, "navigation")
	if err != nil {
		t.Fatalf("Search navigation returned error: %v", err)
	}
	if !strings.Contains(navigation, "# GoreGraph Navigation") {
		t.Fatalf("navigation alias returned unexpected output:\n%s", navigation)
	}

	diagnostics, err := Search(root, "diagnostics")
	if err != nil {
		t.Fatalf("Search diagnostics returned error: %v", err)
	}
	if !strings.Contains(diagnostics, "# GoreGraph Diagnostics") {
		t.Fatalf("diagnostics alias returned unexpected output:\n%s", diagnostics)
	}

	diagnosticsJSON, err := Search(root, "diagnostics-json")
	if err != nil {
		t.Fatalf("Search diagnostics-json returned error: %v", err)
	}
	if !strings.Contains(diagnosticsJSON, `"entrypoints"`) {
		t.Fatalf("diagnostics-json alias returned unexpected output:\n%s", diagnosticsJSON)
	}
}

func TestSearchReadsWorkspaceOverlayAliases(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "src/api/cadasterservice.js", "export function loadCadaster(id) {\n"+
		"  return fetch(`/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get(@PathVariable String cadasterId) {
    return cadasterId;
  }
}
`)
	if _, err := scan.Run(frontend, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(cadaster, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	context, err := Search(frontend, "workspace-context")
	if err != nil {
		t.Fatalf("Search workspace-context returned error: %v", err)
	}
	if !strings.Contains(context, "microservices/ms-cadaster") {
		t.Fatalf("workspace-context alias missing cadaster project:\n%s", context)
	}

	matches, err := Search(frontend, "workspace-contracts")
	if err != nil {
		t.Fatalf("Search workspace-contracts returned error: %v", err)
	}
	if !strings.Contains(matches, "ms-cadaster GET `/cadasters/{cadasterId}`") {
		t.Fatalf("workspace-contracts alias missing backend match:\n%s", matches)
	}

	consumers, err := Search(cadaster, "frontend-consumers")
	if err != nil {
		t.Fatalf("Search frontend-consumers returned error: %v", err)
	}
	if !strings.Contains(consumers, "frontend/frontend-monorepo") {
		t.Fatalf("frontend-consumers alias missing frontend project:\n%s", consumers)
	}
}

func TestSearchReadsWorkspaceRootOverlayAliases(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "src/api/cadasterservice.js", "export function loadCadaster(id) {\n"+
		"  return fetch(`/cadasters/${id}`);\n"+
		"}\n")
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get(@PathVariable String cadasterId) {
    return cadasterId;
  }
}
`)
	if _, err := scan.Run(frontend, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(cadaster, config.Defaults()); err != nil {
		t.Fatal(err)
	}

	matches, err := Search(workspace, "workspace-contracts")
	if err != nil {
		t.Fatalf("Search workspace-contracts at workspace root returned error: %v", err)
	}
	if !strings.Contains(matches, "frontend/frontend-monorepo") || !strings.Contains(matches, "ms-cadaster GET `/cadasters/{cadasterId}`") {
		t.Fatalf("workspace root alias returned unexpected output:\n%s", matches)
	}

	context, err := Search(workspace, "workspace-context")
	if err != nil {
		t.Fatalf("Search workspace-context at workspace root returned error: %v", err)
	}
	if !strings.Contains(context, "Loaded Indexes") || !strings.Contains(context, "microservices/ms-cadaster") {
		t.Fatalf("workspace root context alias returned unexpected output:\n%s", context)
	}

	features, err := Search(workspace, "workspace-features")
	if err != nil {
		t.Fatalf("Search workspace-features at workspace root returned error: %v", err)
	}
	if !strings.Contains(features, "# GoreGraph Workspace Feature Flows") {
		t.Fatalf("workspace root feature alias returned unexpected output:\n%s", features)
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
