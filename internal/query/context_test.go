package query

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestRenderContextMarkdownIsCompactAndActionable(t *testing.T) {
	pack := completeContextPackFixture()
	pack.FallbackRequired = false
	pack.FallbackReason = ""

	body := RenderContextMarkdown(pack)
	for _, want := range []string{
		"# GoreGraph Context",
		"Query: delete user",
		"Confidence: EXACT",
		"Fallback required: no",
		"`api/UserController.java:20-28`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("markdown missing %q:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{
		"WARNING:", "Suggested next:", "maven /", "text /", "yaml /",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("markdown contains %q:\n%s", forbidden, body)
		}
	}
	if !strings.HasSuffix(body, "\n") || strings.HasSuffix(body, "\n\n") {
		t.Fatalf("markdown must end in exactly one newline: %q", body)
	}
}

func TestRenderContextMarkdownOrdersAndOmitsSections(t *testing.T) {
	body := RenderContextMarkdown(completeContextPackFixture())
	last := -1
	for _, heading := range []string{
		"## Entrypoints",
		"## Call chain",
		"## Contracts",
		"## Persistence",
		"## Tests",
		"## Files to inspect",
		"## Uncertainties",
		"## Fallback",
	} {
		index := strings.Index(body, heading)
		if index <= last {
			t.Fatalf("section %q is absent or out of order:\n%s", heading, body)
		}
		last = index
	}

	empty := RenderContextMarkdown(agent.ContextPack{
		Query: "inspect route", Confidence: "LOW", BudgetTokens: 1800,
	})
	for _, heading := range []string{
		"## Entrypoints", "## Call chain", "## Contracts", "## Persistence",
		"## Tests", "## Files to inspect", "## Uncertainties", "## Fallback",
	} {
		if strings.Contains(empty, heading) {
			t.Fatalf("empty section %q was rendered:\n%s", heading, empty)
		}
	}
}

func TestRenderContextMarkdownNormalizesInlineValuesAndRanges(t *testing.T) {
	pack := agent.ContextPack{
		Query: "delete\n## Injected", Confidence: "EXACT", BudgetTokens: 1800,
		Entrypoints: []agent.ContextLocation{
			{Kind: "route", Label: "GET\n## Bad", Project: "api`one", File: "route`file.go", Line: 7},
		},
		Files: []agent.ContextFile{
			{Path: "no-line.go", Role: "entrypoint", Reason: "exact\nroute"},
			{Path: "single.go", StartLine: 20, EndLine: 20, Role: "entrypoint", Reason: "exact"},
			{Path: "range.go", StartLine: 20, EndLine: 28, Role: "entrypoint", Reason: "exact"},
		},
	}

	body := RenderContextMarkdown(pack)
	for _, want := range []string{
		"Query: delete ## Injected",
		"GET ## Bad",
		"`api'one/route'file.go:7`",
		"`no-line.go`",
		"`single.go:20`",
		"`range.go:20-28`",
		"exact route",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("normalized markdown missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "\n## Injected") || strings.Contains(body, "\n## Bad") {
		t.Fatalf("inline value injected a section:\n%s", body)
	}
}

func TestRunContextReturnsBarePrettyJSONWithSeparateByteGates(t *testing.T) {
	root := writeQueryContextIndex(t, simpleContextIndex())

	body, err := RunContext(ContextOptions{
		Root: root, Query: "DELETE /users/{id}", Format: "json",
		BudgetTokens: 900, MaxFiles: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(body, "\n") || strings.HasSuffix(body, "\n\n") {
		t.Fatalf("JSON must end in exactly one newline: %q", body)
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(body), &pack); err != nil {
		t.Fatalf("decode context pack: %v\n%s", err, body)
	}
	if pack.BudgetTokens != 900 || pack.Query != "DELETE /users/{id}" || pack.FallbackRequired {
		t.Fatalf("unexpected context pack: %#v", pack)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["task"]; ok {
		t.Fatalf("direct context JSON returned an agent.Result envelope: %s", body)
	}
	compact, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if len(compact) > 3600 {
		t.Fatalf("compact JSON gate exceeded: %d bytes", len(compact))
	}
	if len(body) > 5400 {
		t.Fatalf("pretty JSON gate exceeded: %d bytes", len(body))
	}
	if len(body) <= len(compact) {
		t.Fatalf("pretty and compact byte gates were conflated: pretty=%d compact=%d", len(body), len(compact))
	}
}

func TestRunContextDefaultsToDeterministicMarkdown(t *testing.T) {
	root := writeQueryContextIndex(t, simpleContextIndex())
	options := ContextOptions{Root: root, Query: "DELETE /users/{id}"}

	first, err := RunContext(options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunContext(options)
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := RunContext(ContextOptions{
		Root: root, Query: options.Query, Format: "markdown",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first != explicit {
		t.Fatalf("markdown is not deterministic/default-equivalent:\nfirst=%s\nsecond=%s\nexplicit=%s", first, second, explicit)
	}
	if !strings.Contains(first, "Budget tokens: ") || !strings.Contains(first, " / 1800") {
		t.Fatalf("default budget missing:\n%s", first)
	}
}

func TestRunContextRejectsUnknownFormat(t *testing.T) {
	root := writeQueryContextIndex(t, simpleContextIndex())
	if _, err := RunContext(ContextOptions{
		Root: root, Query: "DELETE /users/{id}", Format: "text",
	}); err == nil || err.Error() != "context format must be json or markdown" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTaskForwardsContextBudgetsAndMapsOnlyExplicitLimit(t *testing.T) {
	root := writeQueryContextIndex(t, denseContextIndex())
	tests := []struct {
		name     string
		options  TaskOptions
		wantMax  int
		wantPack int
	}{
		{name: "defaults", options: TaskOptions{}, wantMax: agent.DefaultContextMaxFiles, wantPack: agent.DefaultContextBudgetTokens},
		{name: "explicit limit", options: TaskOptions{Limit: 5}, wantMax: 5, wantPack: agent.DefaultContextBudgetTokens},
		{name: "capped limit", options: TaskOptions{Limit: 25}, wantMax: 20, wantPack: agent.DefaultContextBudgetTokens},
		{name: "max files wins", options: TaskOptions{Limit: 20, MaxFiles: 6, BudgetTokens: 900}, wantMax: 6, wantPack: 900},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := test.options
			options.Root = root
			options.Task = "task-context"
			options.Query = "GET /users"
			options.Format = "json"
			body, err := RunTask(options)
			if err != nil {
				t.Fatal(err)
			}
			pack := decodeLegacyContextPack(t, body)
			if len(pack.Files) != test.wantMax || pack.BudgetTokens != test.wantPack {
				t.Fatalf("files/budget = %d/%d, want %d/%d: %#v", len(pack.Files), pack.BudgetTokens, test.wantMax, test.wantPack, pack)
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal([]byte(body), &envelope); err != nil {
				t.Fatal(err)
			}
			if _, ok := envelope["task"]; !ok {
				t.Fatalf("legacy JSON lost agent.Result envelope: %s", body)
			}
		})
	}
}

func completeContextPackFixture() agent.ContextPack {
	return agent.ContextPack{
		Schema: 2, Query: "delete user", Freshness: "generated", Confidence: "EXACT",
		Entrypoints: []agent.ContextLocation{{
			ID: "route", Project: "api", Kind: "route", Label: "DELETE /users/{id}",
			File: "UserController.java", Line: 20, EndLine: 28, Reason: "exact route",
			Confidence: "EXACT", EvidenceIDs: []string{"route:1"},
		}},
		CallChain: []agent.ContextRelationship{{
			From: "DELETE /users/{id}", To: "UserService.delete", Kind: "calls",
			Reason: "direct call", Confidence: "EXACT",
		}},
		Contracts:   []agent.ContextLocation{{Kind: "api_contract", Label: "DELETE /users/{id}", Confidence: "EXACT"}},
		Persistence: []agent.ContextLocation{{Kind: "persistence", Label: "UserRepository.delete", Confidence: "RESOLVED"}},
		Tests:       []agent.ContextLocation{{Kind: "test", Label: "deletes user", File: "UserTest.java", Line: 10}},
		Files: []agent.ContextFile{{
			Project: "api", Path: "UserController.java", StartLine: 20, EndLine: 28,
			Role: "entrypoint", Reason: "exact route match", Confidence: "EXACT",
		}},
		Uncertainties:    []agent.ContextUncertainty{{Scope: "database", Reason: "dynamic SQL is unresolved"}},
		FallbackRequired: true,
		FallbackReason:   "inspect generated source",
		EstimatedTokens:  120,
		BudgetTokens:     1800,
	}
}

func simpleContextIndex() scan.AgentContextIndexRecord {
	return scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Project: "api", Kind: "route", Name: "delete user",
			HTTPMethod: "DELETE", Path: "/users/{id}", File: "UserController.java",
			Line: 20, EndLine: 28, Confidence: "EXACT",
		}},
	}
}

func denseContextIndex() scan.AgentContextIndexRecord {
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "generated",
		Facts: []scan.AgentContextFactRecord{{
			ID: "route", Kind: "route", Name: "GET /users", HTTPMethod: "GET",
			Path: "/users", File: "route.go", Confidence: "EXACT",
		}},
	}
	for number := 0; number < 19; number++ {
		id := "neighbor-" + string(rune('a'+number))
		index.Facts = append(index.Facts, scan.AgentContextFactRecord{
			ID: id, Kind: "symbol", Name: id, File: id + ".go", Confidence: "EXACT",
		})
		index.Edges = append(index.Edges, scan.AgentContextEdgeRecord{
			ID: "edge-" + id, FromFactID: "route", ToFactID: id,
			FromLabel: "route", ToLabel: id, Kind: "call", Confidence: "EXACT",
		})
	}
	return index
}

func writeQueryContextIndex(t *testing.T, index scan.AgentContextIndexRecord) string {
	t.Helper()
	root := t.TempDir()
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "goregraph-out", "agent", "context-index.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func decodeLegacyContextPack(t *testing.T, body string) agent.ContextPack {
	t.Helper()
	var result struct {
		Items []struct {
			Data struct {
				Context agent.ContextPack `json:"context"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode legacy result: %v\n%s", err, body)
	}
	if len(result.Items) != 1 {
		t.Fatalf("legacy item count = %d, want 1: %s", len(result.Items), body)
	}
	return result.Items[0].Data.Context
}
