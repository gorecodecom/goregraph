package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainWorkspaceTargetFindsRouteAndConnections(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := WorkspaceGraphRecord{
		Nodes: []WorkspaceGraphNodeRecord{
			{ID: "contract:get-users", Kind: "contract", Label: "GET /users/{userId}", Project: "frontend/app", File: "src/api/users.ts"},
			{ID: "route:ms-user:get:/users/{userid}", Kind: "route", Label: "GET /users/{userId}", Project: "services/ms-user", File: "UserController.java", Symbol: "UserController.get"},
		},
		Edges: []WorkspaceGraphEdgeRecord{
			{ID: "edge:1", From: "contract:get-users", To: "route:ms-user:get:/users/{userid}", Kind: "resolved_by", Confidence: "RESOLVED"},
		},
	}
	if err := writeJSON(filepath.Join(out, "workspace-graph.json"), graph); err != nil {
		t.Fatal(err)
	}

	explain, err := ExplainWorkspaceTarget(out, "GET /users/{userId}")
	if err != nil {
		t.Fatal(err)
	}
	report := RenderWorkspaceExplain(explain)
	for _, want := range []string{"GET /users/{userId}", "UserController.get", "src/api/users.ts"} {
		if !strings.Contains(report, want) {
			t.Fatalf("explain report missing %q:\n%s", want, report)
		}
	}
}
