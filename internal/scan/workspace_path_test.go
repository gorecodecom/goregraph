package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspacePathFindsRouteBetweenFrontendAndBackend(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := WorkspaceGraphRecord{
		Nodes: []WorkspaceGraphNodeRecord{
			{ID: "project:frontend/app", Kind: "project", Label: "frontend/app"},
			{ID: "contract:get-users", Kind: "contract", Label: "GET /users/{userId}"},
			{ID: "route:ms-user:get:/users/{userid}", Kind: "route", Label: "UserController.get", Symbol: "UserController.get"},
		},
		Edges: []WorkspaceGraphEdgeRecord{
			{ID: "edge:1", From: "project:frontend/app", To: "contract:get-users", Kind: "declares_contract"},
			{ID: "edge:2", From: "contract:get-users", To: "route:ms-user:get:/users/{userid}", Kind: "resolved_by"},
		},
	}
	if err := writeJSON(filepath.Join(out, "workspace-graph.json"), graph); err != nil {
		t.Fatal(err)
	}

	nodes, edges, err := WorkspacePath(out, "frontend/app", "UserController.get")
	if err != nil {
		t.Fatal(err)
	}
	report := RenderWorkspacePath(nodes, edges)
	for _, want := range []string{"frontend/app", "GET /users/{userId}", "UserController.get"} {
		if !strings.Contains(report, want) {
			t.Fatalf("path report missing %q:\n%s", want, report)
		}
	}
}

func TestWorkspacePathPrefersExactFileNodeOverFlowContainingFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := WorkspaceGraphRecord{
		Nodes: []WorkspaceGraphNodeRecord{
			{ID: "flow:users", Kind: "flow", Label: "GET /users/{userId}", File: "src/api/users.ts"},
			{ID: "file:frontend/app:src/api/users.ts", Kind: "file", Label: "src/api/users.ts", File: "src/api/users.ts"},
			{ID: "route:users", Kind: "route", Label: "UserController.get", Symbol: "UserController.get"},
		},
		Edges: []WorkspaceGraphEdgeRecord{
			{ID: "edge:1", From: "flow:users", To: "file:frontend/app:src/api/users.ts", Kind: "starts_in"},
			{ID: "edge:2", From: "flow:users", To: "route:users", Kind: "handled_by"},
		},
	}
	if err := writeJSON(filepath.Join(out, "workspace-graph.json"), graph); err != nil {
		t.Fatal(err)
	}

	nodes, _, err := WorkspacePath(out, "src/api/users.ts", "UserController.get")
	if err != nil {
		t.Fatal(err)
	}
	if nodes[0].Kind != "file" {
		t.Fatalf("path should start at exact file node, got %#v", nodes[0])
	}
}

func TestWorkspacePathPrefersHandlerNodeOverFeatureWithSameSymbol(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := WorkspaceGraphRecord{
		Nodes: []WorkspaceGraphNodeRecord{
			{ID: "feature:users", Kind: "feature", Label: "GET /users/{userId}", Symbol: "UserController.get"},
			{ID: "symbol:users", Kind: "backend_handler", Label: "UserController.get", Symbol: "UserController.get"},
		},
	}
	if err := writeJSON(filepath.Join(out, "workspace-graph.json"), graph); err != nil {
		t.Fatal(err)
	}

	nodes, _, err := WorkspacePath(out, "GET /users/{userId}", "UserController.get")
	if err == nil && len(nodes) > 0 && nodes[len(nodes)-1].Kind != "backend_handler" {
		t.Fatalf("target should prefer handler node, got %#v", nodes[len(nodes)-1])
	}
	if err == nil {
		t.Fatalf("expected no path in disconnected graph, got %#v", nodes)
	}
	target := matchGraphNode(graph.Nodes, "UserController.get")
	if target.Kind != "backend_handler" {
		t.Fatalf("target match should prefer backend handler, got %#v", target)
	}
}
