package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestServiceReturnsDirectedTraceAndTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	index := scan.DirectedTraceIndexRecord{SchemaVersion: 1, Traces: []scan.DirectedTraceRecord{{ID: "trace:1", Route: "GET /users", Nodes: []scan.DirectedTraceNodeRecord{{ID: "entry", Role: scan.TraceRoleAPIClient}, {ID: "repo", Role: scan.TraceRoleRepository}}, Edges: []scan.DirectedTraceEdgeRecord{{From: "entry", To: "repo"}}, EntryNodes: []string{"entry"}, ExitNodes: []string{"repo"}, MainPath: []string{"entry", "repo"}}}}
	body, _ := json.Marshal(index)
	if err := os.WriteFile(filepath.Join(root, "goregraph-out", "directed-traces.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	result, err := service.Run(Request{Root: root, Task: "endpoint-trace", Query: "GET /users"})
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("trace result: %#v %v", result, err)
	}
	traversed, err := service.Run(Request{Root: root, Task: "trace-from", Query: "trace:1#entry"})
	if err != nil || len(traversed.Items) != 1 {
		t.Fatalf("traversal result: %#v %v", traversed, err)
	}
}

func TestServiceBoundsResultsAndContinuesDeterministically(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	first, err := service.Run(Request{Root: root, Task: "coverage", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Schema != 1 || first.Task != "coverage" || len(first.Items) != 2 || !first.Truncated || first.Continuation == "" {
		t.Fatalf("unexpected first result: %#v", first)
	}
	second, err := service.Run(Request{Root: root, Task: "coverage", Limit: 2, Continuation: first.Continuation})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) == 0 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("continuation did not advance: %#v", second)
	}
}

func TestServiceRejectsUnsafeBoundsAndReportsCoverage(t *testing.T) {
	service := Service{}
	if _, err := service.Run(Request{Root: t.TempDir(), Task: "coverage", Limit: 101}); err == nil {
		t.Fatal("limit above maximum accepted")
	}
	if _, err := service.Run(Request{Root: t.TempDir(), Task: "coverage", Detail: "verbose"}); err == nil {
		t.Fatal("invalid detail accepted")
	}
}

func TestServiceExposesReferenceCapabilityEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "client.ts"), []byte("export const load = () => fetch('/users')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	coverage, err := service.Run(Request{Root: root, Task: "coverage", Query: "api_clients"})
	if err != nil || len(coverage.Items) != 1 || len(coverage.Items[0].EvidenceIDs) == 0 {
		t.Fatalf("coverage evidence: %#v %v", coverage, err)
	}
	evidence, err := service.Run(Request{Root: root, Task: "evidence", Query: coverage.Items[0].EvidenceIDs[0]})
	if err != nil || len(evidence.Items) != 1 || evidence.Items[0].File != "client.ts" {
		t.Fatalf("architecture evidence: %#v %v", evidence, err)
	}
}
