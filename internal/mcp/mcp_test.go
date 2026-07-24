package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	for _, forbidden := range []string{`"name":"coverage"`, `"name":"query_code_map"`, `"name":"symbol_resolve"`} {
		if bytes.Contains(first, []byte(forbidden)) {
			t.Fatalf("default tool list contains legacy tool %q: %s", forbidden, first)
		}
	}
}

func TestDefaultMCPTaskContextSchemaAndInstructions(t *testing.T) {
	listed := tools(Options{})
	want := map[string]any{
		"name":        "task_context",
		"description": "Return one evidence-backed Context Pack with current, line-numbered source for the central coding path. Call it once with the complete task before reading indexed source. Treat source_sections as already read: for complete coverage, run no source-reading commands on indexed project files; answer only from source_sections and mark absent details as unknown. For partial or missing coverage, inspect only exact project/path entries in source_omissions; do not inspect other files or widen ranges, and report pathless omissions as uncertainty. Retry only when retry_allowed is true, with one retry_anchor and previous_context_id.",
		"inputSchema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"query"},
			"properties": map[string]any{
				"root":                map[string]any{"type": "string"},
				"query":               map[string]any{"type": "string", "minLength": 1},
				"budget_tokens":       map[string]any{"type": "integer", "minimum": agent.MinContextBudgetTokens, "maximum": agent.MaxContextBudgetTokens, "default": agent.DefaultContextBudgetTokens},
				"max_files":           map[string]any{"type": "integer", "minimum": agent.MinContextMaxFiles, "maximum": agent.MaxContextMaxFiles, "default": agent.DefaultContextMaxFiles},
				"previous_context_id": map[string]any{"type": "string", "minLength": 24, "maxLength": 24, "pattern": "^[0-9a-f]{24}$"},
			},
		},
	}
	if len(listed) != 1 || !reflect.DeepEqual(listed[0], want) {
		t.Fatalf("task_context schema = %#v, want %#v", listed, want)
	}
}

func TestTaskContextSchemaSharesAgentBounds(t *testing.T) {
	source, err := os.ReadFile("mcp.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"agent.MinContextBudgetTokens",
		"agent.MaxContextBudgetTokens",
		"agent.DefaultContextBudgetTokens",
		"agent.MinContextMaxFiles",
		"agent.MaxContextMaxFiles",
		"agent.DefaultContextMaxFiles",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("task_context does not share agent bound %q", want)
		}
	}
}

func TestInitializeInstructsAgentsToReuseIncludedSource(t *testing.T) {
	result := handle(request{JSONRPC: "2.0", ID: 1, Method: "initialize"}, Options{})
	body, err := json.Marshal(result.Result)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Instructions string `json:"instructions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	const assistedInstruction = `Call goregraph context once with the complete task before reading indexed source.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path entries listed in source_omissions; do not inspect other files or widen ranges. Report pathless omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.`
	if count := strings.Count(payload.Instructions, assistedInstruction); count != 1 {
		t.Fatalf(
			"initialize instructions contain the exact assisted instruction %d times, want 1: %s",
			count,
			payload.Instructions,
		)
	}
}

func TestMCPTaskContextPassesPreviousContextID(t *testing.T) {
	root := writeMCPContextFixture(t)
	firstText, err := callTool(Options{}, "task_context", map[string]any{"root": root, "query": "DELETE /users/{id}"})
	if err != nil {
		t.Fatal(err)
	}
	var first agent.ContextPack
	if err := json.Unmarshal([]byte(firstText), &first); err != nil {
		t.Fatal(err)
	}
	secondText, err := callTool(Options{}, "task_context", map[string]any{
		"root": root, "query": "DELETE /users/{id}", "previous_context_id": first.ContextID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var second agent.ContextPack
	if err := json.Unmarshal([]byte(secondText), &second); err != nil {
		t.Fatal(err)
	}
	if second.DuplicateOf != first.ContextID || second.EstimatedTokens > 200 || second.RetryAllowed {
		t.Fatalf("MCP duplicate response = %#v", second)
	}
	if got := toolNames(tools(Options{})); !reflect.DeepEqual(got, []string{"task_context"}) {
		t.Fatalf("default MCP tools = %#v", got)
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
	if len(pack.SourceSections) == 0 || !strings.Contains(pack.SourceSections[0].Content, "deleteUser") {
		t.Fatalf("MCP source is not useful: %#v", pack.SourceSections)
	}
	if section := pack.SourceSections[0]; section.StartLine != 20 || section.EndLine != 22 ||
		section.RenderMode != "declaration_body" || !strings.Contains(section.Content, "service.removeUser();") {
		t.Fatalf("MCP enriched source = %#v, want declaration body range 20-22", section)
	}
	if pack.SourceCoverage != "complete" || pack.SourceUnrepresented != 0 {
		t.Fatalf("MCP source coverage = %q / %d", pack.SourceCoverage, pack.SourceUnrepresented)
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
	if strings.Contains(text, "```") {
		t.Fatalf("compact JSON contains Markdown fences: %s", text)
	}
	again, err := callTool(Options{ExpertTools: true}, "task_context", args)
	if err != nil {
		t.Fatal(err)
	}
	if again != text {
		t.Fatalf("standard/expert context differs:\n%s\n%s", text, again)
	}
}

func TestMCPTaskContextEndpointSerialization(t *testing.T) {
	root := writeMCPEndpointContextFixture(t)
	query := "GET /orders authentication"
	expected, err := agent.BuildContext(agent.ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	text, err := callTool(Options{}, "task_context", map[string]any{"root": root, "query": query})
	if err != nil {
		t.Fatal(err)
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(text), &pack); err != nil {
		t.Fatal(err)
	}
	assertMCPEndpointContextWireKeys(t, []byte(text))
	if !reflect.DeepEqual(pack.Endpoints, expected.Endpoints) {
		t.Fatalf("MCP endpoints = %#v, want %#v", pack.Endpoints, expected.Endpoints)
	}
	if len(pack.Endpoints) != 1 {
		t.Fatalf("MCP endpoints = %#v", pack.Endpoints)
	}
	endpoint := pack.Endpoints[0]
	if endpoint.Provider != "services/orders" || endpoint.HTTPMethod != "GET" || endpoint.Path != "/orders/{id}" ||
		endpoint.Security != "bearer" || len(endpoint.Consumers) != 8 || endpoint.OmittedConsumers != 5 {
		t.Fatalf("MCP endpoint details = %#v", endpoint)
	}
	if pack.BudgetTokens != agent.DefaultContextBudgetTokens || pack.FallbackRequired {
		t.Fatalf("MCP context bounds/fallback = %#v", pack)
	}
	if got := toolNames(tools(Options{})); !reflect.DeepEqual(got, []string{"task_context"}) {
		t.Fatalf("default MCP tools = %#v, want only task_context", got)
	}
	for _, forbidden := range []string{"api-catalog.json", ".goregraph-dashboard.json", "/invoices"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("MCP context contains %q: %s", forbidden, text)
		}
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
		{name: "high budget", mutate: func(args map[string]any) { args["budget_tokens"] = float64(6001) }},
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
	root := writeMCPBoundsFixture(t)
	for _, bounds := range []struct {
		budget float64
		files  float64
	}{
		{budget: 256, files: float64(agent.MinContextMaxFiles)},
		{budget: 6000, files: 20},
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
			Qualified: "UserController.deleteUser", HTTPMethod: "DELETE", Path: "/users/{id}", File: "UserController.java",
			Line: 20, EndLine: 22, Confidence: "EXACT",
		}, {
			ID: "service", Project: "api", Kind: "symbol", Name: "removeUser",
			Qualified: "UserService.removeUser", File: "UserService.java",
			Line: 2, EndLine: 4, Confidence: "EXACT",
		}},
	}
	index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
		ID: "edge-service", FromFactID: "route", ToFactID: "service",
		FromLabel: "route", ToLabel: "service", Kind: "call", Confidence: "EXACT",
	})
	for number := 0; number < 8; number++ {
		id := "neighbor-" + string(rune('a'+number))
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Project: "api", Kind: "symbol", Name: id,
			Confidence: "EXACT",
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
	writeFile(t, root, "UserController.java", strings.Repeat("// padding\n", 19)+"public void deleteUser() {\n\tservice.removeUser();\n}\n")
	writeFile(t, root, "UserService.java", "public class UserService {\n\tpublic void removeUser() {\n\t\trepository.delete();\n\t}\n}\n")
	return root
}

func writeMCPBoundsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Kind: "route", Name: "delete user",
			HTTPMethod: "DELETE", Path: "/users/{id}", Confidence: "EXACT",
		}},
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "goregraph-out/agent/context-index.json", string(body))
	return root
}

func writeMCPEndpointContextFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "endpoint:orders", Project: "services/orders", Kind: "api_endpoint", Name: "GET /orders/{id}",
				Qualified: "OrderController.get", HTTPMethod: "GET", Path: "/orders/{id}",
				File: "services/orders/src/OrderController.java", Line: 20,
				Summary: "provider orders; security bearer; request OrderRequest; response OrderResponse; 3 consumer call sites omitted",
				Search:  "GET /orders/{id} authentication bearer services orders", Confidence: "EXACT",
			},
			{
				ID: "endpoint:billing", Project: "services/billing", Kind: "api_endpoint", Name: "GET /invoices",
				HTTPMethod: "GET", Path: "/invoices", File: "services/billing/src/InvoiceController.java", Line: 12,
				Summary: "provider billing; security public", Search: "GET /invoices billing public", Confidence: "EXACT",
			},
		},
	}
	for number := 0; number < 10; number++ {
		id := fmt.Sprintf("consumer:%02d", number)
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Project: fmt.Sprintf("frontend/web-%02d", number), Kind: "api_consumer", Name: id,
			File: fmt.Sprintf("frontend/web-%02d/src/api.ts", number), Line: 7,
			Summary: "consumer service web; auth bearer", Confidence: "RESOLVED",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + id, FromFactID: id, ToFactID: "endpoint:orders",
			Kind: "consumes_endpoint", Reason: "catalog consumer auth bearer", Confidence: "RESOLVED",
		})
	}
	for _, metadata := range []struct {
		id   string
		file string
	}{
		{id: "consumer:catalog", file: ".goregraph-workspace/agent/api-catalog.json"},
		{id: "consumer:dashboard", file: ".goregraph-dashboard.json"},
	} {
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: metadata.id, Project: "frontend/metadata", Kind: "api_consumer", Name: metadata.id,
			File: metadata.file, Line: 1, Summary: "consumer service metadata; auth unknown",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge:" + metadata.id, FromFactID: metadata.id, ToFactID: "endpoint:orders", Kind: "consumes_endpoint",
		})
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "goregraph-out/agent/context-index.json", string(body))
	writeFile(t, root, "services/orders/src/OrderController.java",
		strings.Repeat("// padding\n", 19)+"public OrderResponse get() {\n\treturn service.getOrder();\n}\n")
	return root
}

func assertMCPEndpointContextWireKeys(t *testing.T, body []byte) {
	t.Helper()
	var contextObject map[string]json.RawMessage
	if err := json.Unmarshal(body, &contextObject); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"endpoints", "budget_tokens", "fallback_required", "retry_allowed"} {
		if _, ok := contextObject[key]; !ok {
			t.Fatalf("MCP context JSON missing public key %q: %s", key, body)
		}
	}
	for _, alias := range []string{"endpoint", "Endpoints", "budgetTokens", "BudgetTokens", "fallbackRequired", "FallbackRequired"} {
		if _, ok := contextObject[alias]; ok {
			t.Fatalf("MCP context JSON contains alias key %q: %s", alias, body)
		}
	}

	var endpoints []map[string]json.RawMessage
	if err := json.Unmarshal(contextObject["endpoints"], &endpoints); err != nil || len(endpoints) != 1 {
		t.Fatalf("MCP endpoints JSON = %s, error = %v", contextObject["endpoints"], err)
	}
	endpoint := endpoints[0]
	for _, key := range []string{"http_method", "omitted_consumers"} {
		if _, ok := endpoint[key]; !ok {
			t.Fatalf("MCP endpoint JSON missing public key %q: %s", key, contextObject["endpoints"])
		}
	}
	for _, alias := range []string{"method", "httpMethod", "HTTPMethod", "omittedConsumers", "OmittedConsumers"} {
		if _, ok := endpoint[alias]; ok {
			t.Fatalf("MCP endpoint JSON contains alias key %q: %s", alias, contextObject["endpoints"])
		}
	}

	var consumers []map[string]json.RawMessage
	if err := json.Unmarshal(endpoint["consumers"], &consumers); err != nil || len(consumers) == 0 {
		t.Fatalf("MCP consumers JSON = %s, error = %v", endpoint["consumers"], err)
	}
	if _, ok := consumers[0]["authentication"]; !ok {
		t.Fatalf("MCP consumer JSON missing public key %q: %s", "authentication", endpoint["consumers"])
	}
	if _, ok := consumers[0]["auth"]; ok {
		t.Fatalf("MCP consumer JSON contains alias key %q: %s", "auth", endpoint["consumers"])
	}
	if _, ok := consumers[0]["Authentication"]; ok {
		t.Fatalf("MCP consumer JSON contains alias key %q: %s", "Authentication", endpoint["consumers"])
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
