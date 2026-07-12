package scan

import "testing"

func TestBuildDirectedTraceProjectsLegacyMainPath(t *testing.T) {
	legacy := WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Traces: []WorkspaceEndpointTraceRecord{{ID: "trace:1", Route: "DELETE /users/{id}", Steps: []WorkspaceEndpointTraceStepRecord{{ID: "ui", Kind: "frontend_step", Label: "Click", File: "ui.tsx", Line: 4}, {ID: "api", Kind: "api_contract", Label: "deleteUser", File: "api.ts", Line: 8}, {ID: "handler", Kind: "backend_handler", Label: "UserController.delete", File: "UserController.java", Line: 20}, {ID: "repo", Kind: "backend_step", Label: "UserRepository.delete", File: "UserRepository.java", Line: 12}}, Edges: []WorkspaceEndpointTraceEdgeRecord{{From: "ui", To: "api"}, {From: "api", To: "handler"}, {From: "handler", To: "repo"}}}}}
	index := BuildDirectedTraceIndex(legacy)
	if len(index.Traces) != 1 {
		t.Fatalf("len=%d", len(index.Traces))
	}
	trace := index.Traces[0]
	if len(trace.EntryNodes) != 1 || trace.EntryNodes[0] != "ui" || len(trace.ExitNodes) != 1 || trace.ExitNodes[0] != "repo" {
		t.Fatalf("bad entry/exit: %#v", trace)
	}
	if len(trace.MainPath) != 4 || trace.MainPath[2] != "handler" {
		t.Fatalf("bad main path: %#v", trace.MainPath)
	}
	roles := map[string]TraceNodeRole{}
	for _, node := range trace.Nodes {
		roles[node.ID] = node.Role
		if node.StableID == "" {
			t.Fatalf("node lacks stable ID: %#v", node)
		}
	}
	if roles["api"] != TraceRoleAPIClient || roles["handler"] != TraceRoleController || roles["repo"] != TraceRoleRepository {
		t.Fatalf("bad roles: %#v", roles)
	}
	projected := trace.LegacyProjection()
	if len(projected.Steps) != 4 || projected.Steps[3].Label != "UserRepository.delete" {
		t.Fatalf("legacy projection changed: %#v", projected)
	}
}
