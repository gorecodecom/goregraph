package query

import (
	"encoding/json"
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

func TestRunTaskReturnsBoundedJSONEnvelope(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	result, err := RunTask(TaskOptions{Root: root, Task: "coverage", Format: "json", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"task": "coverage"`, `"truncated": true`, `"continuation":`} {
		if !strings.Contains(result, want) {
			t.Fatalf("task result missing %q:\n%s", want, result)
		}
	}
}

func TestRunTaskMarkdownShowsArtifactFreshness(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	result, err := RunTask(TaskOptions{Root: root, Task: "task-context", Query: "main.go", Format: "markdown", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Freshness: goregraph") || !strings.Contains(result, "source fingerprint") {
		t.Fatalf("task markdown missing freshness provenance:\n%s", result)
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

	frontendUsage, err := Search(root, "frontend-usage")
	if err != nil {
		t.Fatalf("Search frontend-usage returned error: %v", err)
	}
	if !strings.Contains(frontendUsage, "# GoreGraph Frontend Usage") {
		t.Fatalf("frontend-usage alias returned unexpected output:\n%s", frontendUsage)
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

	nextActions, err := Search(workspace, "workspace-next-actions")
	if err != nil {
		t.Fatalf("Search workspace-next-actions at workspace root returned error: %v", err)
	}
	if !strings.Contains(nextActions, "# GoreGraph Workspace Next Actions") {
		t.Fatalf("workspace root next-actions alias returned unexpected output:\n%s", nextActions)
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

func TestRunTaskUsesCanonicalSymbolUsageCategories(t *testing.T) {
	workspace, symbolID, directID, apiID := writeQuerySymbolProjectionFixture(t)

	direct, err := RunTask(TaskOptions{
		Root: workspace, Task: "symbol-usages", Query: symbolID, Format: "json", Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(direct, directID) || strings.Contains(direct, apiID) ||
		!strings.Contains(direct, `"category": "direct_reference"`) {
		t.Fatalf("direct usage output:\n%s", direct)
	}

	api, err := RunTask(TaskOptions{
		Root: workspace, Task: "symbol-api-consumers", Query: symbolID, Format: "json", Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(api, apiID) || strings.Contains(api, directID) ||
		!strings.Contains(api, `"category": "reached_through_api"`) {
		t.Fatalf("API usage output:\n%s", api)
	}
}

func TestRunTaskTextFormatsIncludeStableSymbolItemIDs(t *testing.T) {
	workspace, symbolID, directID, apiID := writeQuerySymbolProjectionFixture(t)
	tests := []struct {
		task  string
		query string
		id    string
	}{
		{task: "symbol-inventory", query: "microservices/ms-user", id: symbolID},
		{task: "symbol-resolve", query: "com.weka.UserService", id: symbolID},
		{task: "symbol-usages", query: symbolID, id: directID},
		{task: "symbol-api-consumers", query: symbolID, id: apiID},
		{task: "symbol-explain", query: directID, id: directID},
	}
	for _, format := range []string{"markdown", "text"} {
		for _, test := range tests {
			result, err := RunTask(TaskOptions{
				Root: workspace, Task: test.task, Query: test.query,
				Format: format, Detail: "full", Limit: 20,
			})
			if err != nil {
				t.Fatalf("%s %s: %v", format, test.task, err)
			}
			if !strings.Contains(result, "`"+test.id+"`") {
				t.Fatalf("%s %s output missing stable ID %q:\n%s", format, test.task, test.id, result)
			}
		}
	}
}

func TestExplainStableSymbolAndUsageIDsUsesCanonicalProjection(t *testing.T) {
	workspace, symbolID, directID, _ := writeQuerySymbolProjectionFixture(t)

	symbol, err := Explain(workspace, symbolID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(symbol, "# GoreGraph symbol-explain") ||
		!strings.Contains(symbol, "com.weka.UserService") {
		t.Fatalf("symbol explanation:\n%s", symbol)
	}

	usage, err := Explain(workspace, directID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# GoreGraph symbol-explain",
		"qualified Java reference",
		"direct_reference",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage explanation missing %q:\n%s", want, usage)
		}
	}
}

func TestSearchReadsCanonicalSymbolProjectionAliases(t *testing.T) {
	workspace, symbolID, directID, _ := writeQuerySymbolProjectionFixture(t)

	symbols, err := Search(workspace, "symbol-index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(symbols, symbolID) {
		t.Fatalf("symbol-index alias output:\n%s", symbols)
	}

	usages, err := Search(workspace, "symbol-usages-json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(usages, directID) {
		t.Fatalf("symbol-usages-json alias output:\n%s", usages)
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

func writeQuerySymbolProjectionFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "weka")
	out := filepath.Join(workspace, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	const symbolID = "symbol:query-01"
	const directID = "usage:query-direct"
	const apiID = "usage:query-api"
	symbols := scan.WorkspaceSymbolIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Root:          filepath.ToSlash(workspace),
		Symbols: []scan.CanonicalSymbolRecord{{
			ID: symbolID, Project: "microservices/ms-user", Language: "java", Kind: "class",
			Name: "UserService", QualifiedName: "com.weka.UserService",
			DeclarationFile: "src/UserService.java", DeclarationLine: 10,
			Analyzer: "java-source", Confidence: scan.ConfidenceExact, Coverage: scan.CoverageComplete,
		}},
	}
	usages := scan.WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Root:          filepath.ToSlash(workspace),
		Usages: []scan.CanonicalSymbolUsageRecord{
			{
				ID: directID, ProviderSymbolID: symbolID, ConsumerProject: "microservices/ms-order",
				Category: scan.SymbolUsageDirectReference, Language: "java", RelationKind: "calls_method_owner",
				SourceFile: "src/OrderService.java", SourceLine: 20, Confidence: scan.ConfidenceExact,
				Resolution: scan.SymbolResolutionExact, Reason: "qualified Java reference",
				Analyzer: "workspace-symbols", EvidenceIDs: []string{"microservices/ms-order#evidence:direct"},
				DependencyEvidence: []string{"com.weka:ms-user"},
			},
			{
				ID: apiID, ProviderSymbolID: symbolID, ConsumerProject: "frontend/app",
				Category: scan.SymbolUsageReachedThroughAPI, Language: "typescript", RelationKind: "http_reachability",
				SourceFile: "src/UserPage.tsx", SourceLine: 7, Confidence: scan.ConfidenceResolved,
				Resolution: scan.SymbolResolutionExact, Reason: "resolved HTTP contract and implementation",
				Analyzer: "workspace-symbol-api", EvidenceIDs: []string{"frontend/app#evidence:api"},
				Transport: "http", APIPath: []scan.SymbolAPIPathStepRecord{{
					Position: 0, Kind: "selected_symbol", Project: "microservices/ms-user",
					SymbolID: symbolID, Label: "UserService",
				}},
			},
		},
	}
	writeQueryJSON(t, filepath.Join(out, "symbol-index.json"), symbols)
	writeQueryJSON(t, filepath.Join(out, "symbol-usages.json"), usages)
	return workspace, symbolID, directID, apiID
}

func writeQueryJSON(t *testing.T, path string, value any) {
	t.Helper()
	path = queryTestOutputPath(path)
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := queryTestOutputPath(filepath.Join(root, filepath.FromSlash(rel)))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func queryTestOutputPath(path string) string {
	dir, name := filepath.Dir(path), filepath.Base(path)
	if name == "manifest.json" {
		return path
	}
	parent := filepath.Base(dir)
	if parent != "goregraph-out" && parent != ".goregraph-workspace" {
		return path
	}
	if strings.HasSuffix(name, ".json") {
		return filepath.Join(dir, "index", name)
	}
	return filepath.Join(dir, "dashboard", name)
}
