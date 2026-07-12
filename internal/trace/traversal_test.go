package trace

import (
	"github.com/gorecodecom/goregraph/internal/scan"
	"testing"
)

func TestTraverseReturnsBoundedUpstreamDownstreamAndTargets(t *testing.T) {
	graph := scan.DirectedTraceRecord{ID: "t", Nodes: []scan.DirectedTraceNodeRecord{{ID: "entry", Role: scan.TraceRoleUIRoute}, {ID: "handler", Role: scan.TraceRoleController}, {ID: "repo", Role: scan.TraceRoleRepository}, {ID: "test", Role: scan.TraceRoleTest}}, Edges: []scan.DirectedTraceEdgeRecord{{From: "entry", To: "handler"}, {From: "handler", To: "repo"}, {From: "handler", To: "test"}}, EntryNodes: []string{"entry"}, ExitNodes: []string{"repo", "test"}}
	result, err := Traverse(graph, "handler", Options{MaxNodes: 3, MaxCycleVisits: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Upstream, "entry") || !contains(result.Downstream, "repo") || !contains(result.Downstream, "test") {
		t.Fatalf("bad traversal: %#v", result)
	}
	if len(result.PathsToPersistence) != 1 || result.PathsToPersistence[0][1] != "repo" {
		t.Fatalf("missing repository path: %#v", result.PathsToPersistence)
	}
	if len(result.PathsToTests) != 1 {
		t.Fatalf("missing test path: %#v", result.PathsToTests)
	}
}

func TestTraverseRejectsUnknownNodeAndReportsTruncation(t *testing.T) {
	graph := scan.DirectedTraceRecord{Nodes: []scan.DirectedTraceNodeRecord{{ID: "a"}, {ID: "b"}, {ID: "c"}}, Edges: []scan.DirectedTraceEdgeRecord{{From: "a", To: "b"}, {From: "b", To: "c"}}}
	if _, err := Traverse(graph, "missing", Options{}); err == nil {
		t.Fatal("unknown node accepted")
	}
	result, err := Traverse(graph, "a", Options{MaxNodes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatal("bounded traversal did not report truncation")
	}
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
