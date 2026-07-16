package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestServeHandlesToolsListAndQueryTool(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query_code_map","arguments":{"root":"` + filepath.ToSlash(root) + `","term":"StartServer"}}}` + "\n",
	)
	var output bytes.Buffer

	if err := Serve(input, &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "query_code_map") {
		t.Fatalf("tools/list response missing query_code_map:\n%s", got)
	}
	if !strings.Contains(got, "StartServer") {
		t.Fatalf("tools/call response missing query result:\n%s", got)
	}
}

func TestServeReadsGeneratedOutputTool(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_output","arguments":{"root":"` + filepath.ToSlash(root) + `","name":"graph-full"}}}` + "\n",
	)
	var output bytes.Buffer

	if err := Serve(input, &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "graph-full") && !strings.Contains(got, "StartServer") {
		t.Fatalf("get_output response missing rich graph content:\n%s", got)
	}
}

func TestServeExposesCompactAgentTools(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"coverage","arguments":{"root":"` + filepath.ToSlash(root) + `","limit":2}}}` + "\n")
	var output bytes.Buffer
	if err := Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{"workspace_summary", "workspace_delta", "endpoint_search", "task_context", "diagnostics", "coverage", "evidence", "change_context", `\"truncated\": true`, `\"continuation\":`} {
		if !strings.Contains(got, want) {
			t.Fatalf("MCP output missing %q:\n%s", want, got)
		}
	}
}

func TestMCPMapsImpactSummaryTool(t *testing.T) {
	if task, ok := agentTaskForTool("impact_summary"); !ok || task != "impact-summary" {
		t.Fatalf("impact_summary mapping=%q ok=%v", task, ok)
	}
	listed := tools()
	found := false
	for _, item := range listed {
		if item["name"] == "impact_summary" {
			found = true
		}
	}
	if !found {
		t.Fatal("impact_summary tool is not listed")
	}
}

func TestMCPListsCanonicalSymbolTools(t *testing.T) {
	listed := tools()
	descriptions := map[string]string{}
	for _, item := range listed {
		name, _ := item["name"].(string)
		description, _ := item["description"].(string)
		descriptions[name] = description
	}
	for _, name := range []string{
		"symbol_inventory",
		"symbol_resolve",
		"symbol_usages",
		"symbol_api_consumers",
		"symbol_explain",
	} {
		if descriptions[name] == "" {
			t.Fatalf("%s tool is not listed", name)
		}
	}
	if !strings.Contains(descriptions["symbol_usages"], "exact direct references") ||
		!strings.Contains(descriptions["symbol_api_consumers"], "HTTP reachability") {
		t.Fatalf("symbol tool descriptions = %#v", descriptions)
	}
	for _, name := range []string{"symbol_usages", "symbol_api_consumers", "symbol_explain"} {
		if !strings.Contains(descriptions[name], "symbol_resolve") {
			t.Fatalf("%s description must direct human text through symbol_resolve: %q", name, descriptions[name])
		}
	}
}

func TestMCPSymbolUsageMatchesAgentAndPassesBounds(t *testing.T) {
	workspace := writeMCPSymbolProjectionFixture(t)
	serviceResult, err := (agent.Service{}).Run(agent.Request{
		Root: workspace, Task: "symbol-usages", Query: "symbol:mcp-01", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	text, err := callTool("symbol_usages", map[string]any{
		"root": workspace, "query": "symbol:mcp-01", "limit": float64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	var mcpResult agent.Result
	if err := json.Unmarshal([]byte(text), &mcpResult); err != nil {
		t.Fatal(err)
	}
	var normalizedAgent agent.Result
	agentBody, err := json.Marshal(serviceResult)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(agentBody, &normalizedAgent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mcpResult.Items[0].Data["usage"], normalizedAgent.Items[0].Data["usage"]) {
		t.Fatalf("MCP usage = %#v, agent usage = %#v", mcpResult.Items[0].Data["usage"], normalizedAgent.Items[0].Data["usage"])
	}
	if !mcpResult.Truncated || mcpResult.Continuation == "" {
		t.Fatalf("bounded MCP result = %#v", mcpResult)
	}
	usage := mcpResult.Items[0].Data["usage"].(map[string]any)
	agentUsage := normalizedAgent.Items[0].Data["usage"].(map[string]any)
	for _, field := range []string{
		"category",
		"resolution",
		"evidence_ids",
		"candidate_symbol_ids",
		"api_path",
	} {
		if !reflect.DeepEqual(usage[field], agentUsage[field]) {
			t.Fatalf("MCP usage %s = %#v, agent usage %s = %#v", field, usage[field], field, agentUsage[field])
		}
	}

	next, err := callTool("symbol_usages", map[string]any{
		"root": workspace, "query": "symbol:mcp-01", "limit": float64(1),
		"continuation": mcpResult.Continuation,
	})
	if err != nil {
		t.Fatal(err)
	}
	var nextResult agent.Result
	if err := json.Unmarshal([]byte(next), &nextResult); err != nil {
		t.Fatal(err)
	}
	if nextResult.Count != 1 || nextResult.Items[0].ID == mcpResult.Items[0].ID {
		t.Fatalf("continued MCP result = %#v", nextResult)
	}
}

func TestServeRejectsUnknownTool(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file","arguments":{}}}` + "\n")
	var output bytes.Buffer

	if err := Serve(input, &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	if !strings.Contains(output.String(), "unknown tool") {
		t.Fatalf("unknown tool response missing error:\n%s", output.String())
	}
}

func writeMCPSymbolProjectionFixture(t *testing.T) string {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "weka")
	writeFile(t, workspace, ".goregraph-workspace/index/symbol-index.json", `{
  "schema_version": 3,
  "symbols": [{
    "id": "symbol:mcp-01",
    "project": "microservices/ms-user",
    "language": "java",
    "kind": "class",
    "name": "UserService",
    "qualified_name": "com.weka.UserService",
    "declaration_file": "src/UserService.java",
    "declaration_line": 10,
    "analyzer": "java-source",
    "confidence": "EXACT",
    "coverage": "COMPLETE"
  }],
  "coverage": []
}`)
	writeFile(t, workspace, ".goregraph-workspace/index/symbol-usages.json", `{
  "schema_version": 3,
  "usages": [{
    "id": "usage:mcp-01",
    "provider_symbol_id": "symbol:mcp-01",
    "consumer_project": "microservices/ms-order",
    "category": "direct_reference",
    "language": "java",
    "relation_kind": "calls_method_owner",
    "source_file": "src/OrderService.java",
    "source_line": 20,
    "confidence": "EXACT",
    "resolution": "EXACT",
    "reason": "qualified Java reference",
    "analyzer": "workspace-symbols",
    "evidence_ids": ["microservices/ms-order#evidence:1"],
    "dependency_evidence": ["com.weka:ms-user"]
  }, {
    "id": "usage:mcp-02",
    "provider_symbol_id": "symbol:mcp-01",
    "consumer_project": "microservices/ms-billing",
    "category": "direct_reference",
    "language": "java",
    "relation_kind": "calls_method_owner",
    "source_file": "src/BillingService.java",
    "source_line": 12,
    "confidence": "EXACT",
    "resolution": "EXACT",
    "reason": "qualified Java reference",
    "analyzer": "workspace-symbols",
    "evidence_ids": ["microservices/ms-billing#evidence:2"],
    "candidate_symbol_ids": [],
    "api_path": []
  }],
  "coverage": []
}`)
	return workspace
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
