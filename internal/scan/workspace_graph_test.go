package scan

import "testing"

func TestBuildWorkspaceGraphCreatesStableWorkspaceNodesAndEdges(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Path: "frontend/app", Name: "app", Kind: "frontend", Indexed: true},
			{Path: "services/ms-user", Name: "ms-user", Kind: "backend", Indexed: true},
		},
	}
	matches := []WorkspaceContractMatchRecord{{
		ID:                "contract:get-users",
		APIProject:        "frontend/app",
		APIHTTPMethod:     "GET",
		APIPath:           "/users/{userId}",
		APIFile:           "src/api/users.ts",
		APICaller:         "loadUser",
		BackendProject:    "services/ms-user",
		BackendHTTPMethod: "GET",
		BackendPath:       "/users/{userId}",
		BackendHandler:    "UserController.get",
		BackendFile:       "src/main/java/UserController.java",
		Confidence:        "RESOLVED",
		Issue:             "matched",
	}}
	flows := []WorkspaceFeatureFlowRecord{{
		ID:                "flow:get-users",
		FrontendProject:   "frontend/app",
		FrontendFile:      "src/api/users.ts",
		FrontendCaller:    "loadUser",
		HTTPMethod:        "GET",
		Path:              "/users/{userId}",
		BackendProject:    "services/ms-user",
		BackendFile:       "src/main/java/UserController.java",
		BackendController: "UserController",
		BackendMethod:     "get",
		Confidence:        "MATCHED",
	}}
	dossiers := []FeatureDossierRecord{{
		ID:           "feature:get-users",
		Route:        "GET /users/{userId}",
		SourceFlowID: "flow:get-users",
	}}

	graph := BuildWorkspaceGraph(registry, matches, flows, dossiers)

	requireGraphNode(t, graph, "project:frontend/app", "project")
	requireGraphNode(t, graph, "project:services/ms-user", "project")
	requireGraphNode(t, graph, "contract:get-users", "contract")
	requireGraphNode(t, graph, "route:services/ms-user:get:/users/{userid}", "route")
	requireGraphNode(t, graph, "feature:get-users", "feature")
	requireGraphEdge(t, graph, "project:frontend/app", "contract:get-users", "declares_contract")
	requireGraphEdge(t, graph, "contract:get-users", "route:services/ms-user:get:/users/{userid}", "resolved_by")
	requireGraphEdge(t, graph, "feature:get-users", "flow:get-users", "summarizes_flow")
}

func requireGraphNode(t *testing.T, graph WorkspaceGraphRecord, id, kind string) {
	t.Helper()
	for _, node := range graph.Nodes {
		if node.ID == id {
			if node.Kind != kind {
				t.Fatalf("node %s kind = %s, want %s", id, node.Kind, kind)
			}
			return
		}
	}
	t.Fatalf("missing node %s in %#v", id, graph.Nodes)
}

func requireGraphEdge(t *testing.T, graph WorkspaceGraphRecord, from, to, kind string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return
		}
	}
	t.Fatalf("missing edge %s -[%s]-> %s in %#v", from, kind, to, graph.Edges)
}
