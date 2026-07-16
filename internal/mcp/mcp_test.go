package mcp

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestExpertServeHandlesToolsListAndQueryTool(t *testing.T) {
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

	if err := ServeWithOptions(input, &output, Options{ExpertTools: true}); err != nil {
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

func TestExpertServeReadsGeneratedOutputTool(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_output","arguments":{"root":"` + filepath.ToSlash(root) + `","name":"graph-full"}}}` + "\n",
	)
	var output bytes.Buffer

	if err := ServeWithOptions(input, &output, Options{ExpertTools: true}); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "graph-full") && !strings.Contains(got, "StartServer") {
		t.Fatalf("get_output response missing rich graph content:\n%s", got)
	}
}

func TestExpertServeExposesLegacyAgentTools(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/main.go", "package main\nfunc StartServer() {}\n")
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"coverage","arguments":{"root":"` + filepath.ToSlash(root) + `","limit":2}}}` + "\n")
	var output bytes.Buffer
	if err := ServeWithOptions(input, &output, Options{ExpertTools: true}); err != nil {
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
	listed := tools(Options{ExpertTools: true})
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
	listed := tools(Options{ExpertTools: true})
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
	text, err := callTool(Options{ExpertTools: true}, "symbol_usages", map[string]any{
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

	next, err := callTool(Options{ExpertTools: true}, "symbol_usages", map[string]any{
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

func TestDefaultMCPListsOnlyTaskContext(t *testing.T) {
	listed := tools(Options{})
	if len(listed) != 1 || listed[0]["name"] != "task_context" {
		t.Fatalf("default tools = %#v", listed)
	}
	first, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(tools(Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("default tool list is not deterministic:\n%s\n%s", first, second)
	}
	for _, forbidden := range []string{"coverage", "query_code_map", "symbol_resolve"} {
		if bytes.Contains(first, []byte(forbidden)) {
			t.Fatalf("default tool list contains legacy tool %q: %s", forbidden, first)
		}
	}
}

func TestDefaultMCPTaskContextSchemaAndInstructions(t *testing.T) {
	listed := tools(Options{})
	want := map[string]any{
		"name":        "task_context",
		"description": "Return one compact, budgeted Context Pack for a coding task. If fallback_required is true, stop using GoreGraph and inspect source directly. Call at most twice per task.",
		"inputSchema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"query"},
			"properties": map[string]any{
				"root":          map[string]any{"type": "string"},
				"query":         map[string]any{"type": "string", "minLength": 1},
				"budget_tokens": map[string]any{"type": "integer", "minimum": 256, "maximum": 4000, "default": 1800},
				"max_files":     map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "default": 12},
			},
		},
	}
	if len(listed) != 1 || !reflect.DeepEqual(listed[0], want) {
		t.Fatalf("task_context schema = %#v, want %#v", listed, want)
	}
}

func TestExpertMCPRetainsExactlyTheLegacyToolSet(t *testing.T) {
	want := []string{
		"task_context",
		"query_code_map", "get_project_summary", "get_output", "get_file",
		"get_symbol", "get_related_files", "explain_file", "doctor",
		"workspace_summary", "workspace_delta", "service_context", "endpoint_search",
		"endpoint_trace", "symbol_trace", "trace_from", "data_flow", "impact_summary",
		"diagnostics", "coverage", "evidence", "tests", "change_context",
		"symbol_inventory", "symbol_resolve", "symbol_usages", "symbol_api_consumers",
		"symbol_explain",
	}
	listed := tools(Options{ExpertTools: true})
	got := toolNames(listed)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expert tools = %#v, want %#v", got, want)
	}
	seen := map[string]bool{}
	for _, name := range got {
		if seen[name] {
			t.Fatalf("expert tools contain duplicate %q: %#v", name, got)
		}
		seen[name] = true
	}
	if !reflect.DeepEqual(listed[0], tools(Options{})[0]) {
		t.Fatalf("expert task_context schema differs from default: %#v / %#v", listed[0], tools(Options{})[0])
	}
}

func TestDefaultMCPRejectsDirectLegacyCalls(t *testing.T) {
	names := toolNames(legacyTools())
	if len(names) != 27 {
		t.Fatalf("legacy tool count = %d, want 27: %#v", len(names), names)
	}
	for _, name := range names {
		if _, err := callTool(Options{}, name, map[string]any{}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
			t.Fatalf("default call %q error = %v", name, err)
		}
	}
}

func TestServeUsesDefaultOptionsForListAndCalls(t *testing.T) {
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"coverage","arguments":{}}}` + "\n",
	)
	var output bytes.Buffer
	if err := Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, `"name":"task_context"`) || strings.Contains(got, `"name":"coverage"`) {
		t.Fatalf("default tools/list response is not minimal:\n%s", got)
	}
	if !strings.Contains(got, "unknown tool: coverage") {
		t.Fatalf("default legacy call was not rejected:\n%s", got)
	}
}

func TestMCPTaskContextPassesBudgetsToCompilerAsBareCompactJSON(t *testing.T) {
	root := writeMCPContextFixture(t)
	args := map[string]any{
		"root": root, "query": "DELETE /users/{id}",
		"budget_tokens": float64(700), "max_files": float64(5),
	}
	text, err := callTool(Options{}, "task_context", args)
	if err != nil {
		t.Fatal(err)
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(text), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.BudgetTokens != 700 || len(pack.Files) > 5 || pack.EstimatedTokens > 700 {
		t.Fatalf("pack = %#v", pack)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"task", "items", "coverage_warnings", "suggested_next"} {
		if _, ok := envelope[forbidden]; ok {
			t.Fatalf("bare pack contains agent.Result field %q: %s", forbidden, text)
		}
	}
	if !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") || strings.Contains(text, "\n  ") {
		t.Fatalf("context text is not compact JSON with one newline: %q", text)
	}
	again, err := callTool(Options{ExpertTools: true}, "task_context", args)
	if err != nil {
		t.Fatal(err)
	}
	if again != text {
		t.Fatalf("standard/expert context differs:\n%s\n%s", text, again)
	}
}

func TestMCPTaskContextUsesCompilerDefaultsAndReturnsFallbackData(t *testing.T) {
	root := writeMCPContextFixture(t)
	text, err := callTool(Options{}, "task_context", map[string]any{
		"root": root, "query": "no relevant generated fact",
	})
	if err != nil {
		t.Fatal(err)
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(text), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.BudgetTokens != agent.DefaultContextBudgetTokens || !pack.FallbackRequired || pack.FallbackReason == "" {
		t.Fatalf("fallback/default pack = %#v", pack)
	}
}

func TestMCPTaskContextValidatesArgumentsStrictly(t *testing.T) {
	root := writeMCPContextFixture(t)
	valid := map[string]any{"root": root, "query": "DELETE /users/{id}"}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "missing query", mutate: func(args map[string]any) { delete(args, "query") }},
		{name: "blank query", mutate: func(args map[string]any) { args["query"] = " \t " }},
		{name: "wrong query type", mutate: func(args map[string]any) { args["query"] = true }},
		{name: "wrong root type", mutate: func(args map[string]any) { args["root"] = false }},
		{name: "unknown property", mutate: func(args map[string]any) { args["detail"] = "full" }},
		{name: "zero budget", mutate: func(args map[string]any) { args["budget_tokens"] = float64(0) }},
		{name: "low budget", mutate: func(args map[string]any) { args["budget_tokens"] = float64(255) }},
		{name: "high budget", mutate: func(args map[string]any) { args["budget_tokens"] = float64(4001) }},
		{name: "fractional budget", mutate: func(args map[string]any) { args["budget_tokens"] = 700.5 }},
		{name: "string budget", mutate: func(args map[string]any) { args["budget_tokens"] = "700" }},
		{name: "nan budget", mutate: func(args map[string]any) { args["budget_tokens"] = math.NaN() }},
		{name: "infinite budget", mutate: func(args map[string]any) { args["budget_tokens"] = math.Inf(1) }},
		{name: "zero max files", mutate: func(args map[string]any) { args["max_files"] = float64(0) }},
		{name: "negative max files", mutate: func(args map[string]any) { args["max_files"] = float64(-1) }},
		{name: "high max files", mutate: func(args map[string]any) { args["max_files"] = float64(21) }},
		{name: "fractional max files", mutate: func(args map[string]any) { args["max_files"] = 5.5 }},
		{name: "boolean max files", mutate: func(args map[string]any) { args["max_files"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := map[string]any{}
			for key, value := range valid {
				args[key] = value
			}
			test.mutate(args)
			if _, err := callTool(Options{}, "task_context", args); err == nil {
				t.Fatalf("invalid arguments were accepted: %#v", args)
			}
		})
	}
}

func TestMCPTaskContextAcceptsExactBounds(t *testing.T) {
	root := writeMCPContextFixture(t)
	for _, bounds := range []struct {
		budget float64
		files  float64
	}{
		{budget: 256, files: 1},
		{budget: 4000, files: 20},
	} {
		text, err := callTool(Options{}, "task_context", map[string]any{
			"root": root, "query": "DELETE /users/{id}",
			"budget_tokens": bounds.budget, "max_files": bounds.files,
		})
		if err != nil {
			t.Fatalf("bounds %#v: %v", bounds, err)
		}
		var pack agent.ContextPack
		if err := json.Unmarshal([]byte(text), &pack); err != nil {
			t.Fatal(err)
		}
		if pack.BudgetTokens != int(bounds.budget) || len(pack.Files) > int(bounds.files) {
			t.Fatalf("bounds %#v produced %#v", bounds, pack)
		}
	}
}

func toolNames(listed []map[string]any) []string {
	names := make([]string, 0, len(listed))
	for _, item := range listed {
		name, _ := item["name"].(string)
		names = append(names, name)
	}
	return names
}

func writeMCPContextFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Project: "api", Kind: "route", Name: "delete user",
			HTTPMethod: "DELETE", Path: "/users/{id}", File: "UserController.java",
			Line: 20, EndLine: 28, Confidence: "EXACT",
		}},
	}
	for number := 0; number < 8; number++ {
		id := "neighbor-" + string(rune('a'+number))
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Project: "api", Kind: "symbol", Name: id,
			File: id + ".go", Confidence: "EXACT",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge-" + id, FromFactID: "route", ToFactID: id,
			FromLabel: "route", ToLabel: id, Kind: "call", Confidence: "EXACT",
		})
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "goregraph-out/agent/context-index.json", string(body))
	return root
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
