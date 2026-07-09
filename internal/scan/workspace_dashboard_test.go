package scan

import (
	"strings"
	"testing"
)

func TestRenderWorkspaceDashboardHTMLContainsInteractiveGraphData(t *testing.T) {
	graph := WorkspaceGraphRecord{
		SchemaVersion: SchemaVersion,
		Root:          "/workspace",
		Nodes: []WorkspaceGraphNodeRecord{
			{ID: "project:frontend/app", Kind: "project", Label: "app", Project: "frontend/app"},
			{ID: "route:services/ms-user:get:/users/{userid}", Kind: "route", Label: "GET /users/{userId}", Project: "services/ms-user"},
		},
		Edges: []WorkspaceGraphEdgeRecord{
			{ID: "edge:1", From: "project:frontend/app", To: "route:services/ms-user:get:/users/{userid}", Kind: "depends_on"},
		},
	}
	matches := []WorkspaceContractMatchRecord{
		{
			ID:                "contract:get-user",
			APIProject:        "frontend/app",
			APIHTTPMethod:     "GET",
			APIPath:           "/users/{userId}",
			APIFile:           "src/api/users.ts",
			APICaller:         "getUser",
			BackendProject:    "services/ms-user",
			BackendService:    "ms-user",
			BackendHTTPMethod: "GET",
			BackendPath:       "/users/{userId}",
			BackendHandler:    "UserController.get",
			Confidence:        "RESOLVED",
		},
	}
	html := RenderWorkspaceDashboardHTML(graph, matches, nil)

	for _, want := range []string{
		"<!doctype html>",
		`id="workspace-search"`,
		"data-kind-filter",
		`data-view-mode="architecture"`,
		`data-view-mode="services"`,
		`data-view-mode="endpoint"`,
		`data-view-mode="raw"`,
		`data-view-mode="issues"`,
		`id="clear-selection"`,
		`id="zoom-in"`,
		`id="zoom-out"`,
		`id="reset-view"`,
		`id="toggle-labels"`,
		`id="graph-layer"`,
		"Service Map",
		"Architecture Map",
		"Endpoint Trace",
		"Open Issues",
		"Issue Workbench",
		"function buildIssueGroups",
		"function renderIssueWorkbench",
		"function sourceHref",
		"function fileLink",
		"Incoming",
		"Outgoing",
		"Frontend clients",
		"Backend services",
		"Status glossary",
		"RESOLVED",
		"MISMATCH",
		"UNRESOLVED",
		"OUT_OF_SCOPE",
		"function renderServiceMap",
		"function renderArchitectureMap",
		"function renderEndpointTrace",
		"Endpoint Paths",
		"function renderEndpointPaths",
		"function endpointPathRowsForService",
		"function clearSelection",
		"function serviceRole",
		"function serviceDomain",
		"function focusGraphItem",
		"function focusTraceStep",
		"function wrapSvgText",
		"function selectOrToggleItem",
		"const serviceMap =",
		"const endpointTraces =",
		"frontend/app -> services/ms-user",
		"function zoomBy",
		"function panBy",
		"user-select:none",
		"project:frontend/app",
		"GET /users/{userId}",
		"const workspaceGraph =",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, "Connections</h3><p class=\"connection-help\">Connections show why this node exists") {
		t.Fatalf("dashboard should not default to raw connection ID detail blocks")
	}
	if strings.Contains(html, "Shared / Internal") {
		t.Fatalf("dashboard should not label unrelated frontend projects as shared/internal")
	}
	if !strings.Contains(html, `const state={mode:"issues"`) {
		t.Fatalf("dashboard should open in issues mode when generated")
	}
	for _, want := range []string{`data-step-id`, `data-focus-id`, `trace-card`, `path-card`, "centerOnPosition", "truncateWord"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing usability hook %q", want)
		}
	}
	if strings.Contains(html, `renderCanvas();focusGraphItem(id);`) {
		t.Fatalf("dashboard must not auto-center every selection")
	}
	if strings.Contains(html, `e.target&&e.target.id==="workspace-graph")clearSelection()`) {
		t.Fatalf("dashboard must not clear selection on empty canvas clicks")
	}
	if strings.Contains(html, `"dossiers"`) || strings.Contains(html, `"matches"`) {
		t.Fatalf("dashboard should not embed unused raw matches or dossiers payload")
	}
	if strings.Contains(html, "https://") || strings.Contains(html, "http://") {
		t.Fatalf("dashboard must not load remote assets")
	}
}

func TestRenderWorkspaceDashboardHTMLGroupsOpenIssuesAndAddsFileLinks(t *testing.T) {
	graph := WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"}
	serviceMap := WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion}
	traces := WorkspaceEndpointTraceIndexRecord{
		SchemaVersion: SchemaVersion,
		Traces: []WorkspaceEndpointTraceRecord{
			{
				ID:          "trace:frontend-tree",
				Route:       "GET /tree/regulationtopics",
				Method:      "GET",
				Path:        "/tree/regulationtopics",
				FromProject: "frontend/frontend",
				ToProject:   "microservices/ms-regulationtree",
				Status:      "UNRESOLVED",
				Risk:        "indexed_backend_route_missing",
				Steps: []WorkspaceEndpointTraceStepRecord{
					{ID: "step:rdbv", Kind: "api_contract", Label: "loadTree", Project: "frontend/frontend", File: "src/api/tree.ts", Line: 42},
				},
			},
			{
				ID:          "trace:frontends-tree",
				Route:       "GET /tree/regulationtopics",
				Method:      "GET",
				Path:        "/tree/regulationtopics",
				FromProject: "frontend/frontends",
				ToProject:   "microservices/ms-regulationtree",
				Status:      "UNRESOLVED",
				Risk:        "indexed_backend_route_missing",
			},
			{
				ID:          "trace:method",
				Route:       "PUT /productservice/users/{userId}/services/{serviceCode}",
				Method:      "PUT",
				Path:        "/productservice/users/{userId}/services/{serviceCode}",
				FromProject: "frontend/frontend-monorepo",
				ToProject:   "microservices/ms-productservice",
				Status:      "MISMATCH",
				Risk:        "method_mismatch",
			},
		},
	}

	html := RenderWorkspaceDashboardHTMLWithModels(graph, serviceMap, traces, nil, nil)

	for _, want := range []string{
		`data-view-mode="issues"`,
		"Open Issues",
		"Issue Workbench",
		"function issueGroupKey",
		"function issueGroupTitle",
		"RDBV /tree/*",
		"file://",
		"src/api/tree.ts:42",
		"method_mismatch",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing issue workbench value %q\n%s", want, html)
		}
	}
}

func TestRenderWorkspaceDashboardHTMLEndpointPathsUseServiceRows(t *testing.T) {
	serviceMap := WorkspaceServiceMapRecord{
		SchemaVersion: SchemaVersion,
		Nodes: []WorkspaceServiceNodeRecord{
			{ID: "service:frontend/app", Label: "app", Project: "frontend/app", Kind: "frontend", Role: "frontend", Domain: "frontend", Indexed: true, Outgoing: 1},
			{ID: "service:services/ms-user", Label: "ms-user", Project: "services/ms-user", Kind: "backend", Role: "backend", Domain: "identity", Indexed: true, Incoming: 1},
		},
		Edges: []WorkspaceServiceEdgeRecord{
			{
				ID: "edge:app-user", From: "service:frontend/app", To: "service:services/ms-user",
				FromProject: "frontend/app", ToProject: "services/ms-user", Direction: "frontend/app -> services/ms-user",
				Total: 1, Resolved: 1, Risk: "resolved", Endpoints: []string{"GET /users/{userId}"},
			},
		},
	}
	traces := WorkspaceEndpointTraceIndexRecord{
		SchemaVersion: SchemaVersion,
		Traces: []WorkspaceEndpointTraceRecord{
			{ID: "trace:get-user", Route: "GET /users/{userId}", FromProject: "frontend/app", ToProject: "services/ms-user", Status: "RESOLVED"},
		},
	}

	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		serviceMap,
		traces,
		nil,
		nil,
	)

	for _, want := range []string{
		"Endpoint Paths",
		"function endpointPathRowsForService",
		"function renderEndpointPaths",
		"Caller",
		"Endpoint / relation",
		"Provider / next hop",
		"trace:get-user",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing endpoint path value %q\n%s", want, html)
		}
	}
}

func TestRenderWorkspaceDashboardHTMLShowsUnconnectedFrontendClients(t *testing.T) {
	serviceMap := WorkspaceServiceMapRecord{
		SchemaVersion: SchemaVersion,
		Nodes: []WorkspaceServiceNodeRecord{
			{ID: "service:frontend/frontend", Label: "frontend", Project: "frontend/frontend", Kind: "frontend", Role: "frontend", Domain: "frontend", Indexed: true},
			{ID: "service:frontend/frontend-monorepo", Label: "frontend-monorepo", Project: "frontend/frontend-monorepo", Kind: "frontend", Role: "frontend", Domain: "frontend", Indexed: true, Outgoing: 1},
			{ID: "service:frontend/frontends", Label: "frontends", Project: "frontend/frontends", Kind: "frontend", Role: "frontend", Domain: "frontend", Indexed: true},
			{ID: "service:frontend/playwright", Label: "playwright", Project: "frontend/playwright", Kind: "frontend", Role: "frontend", Domain: "frontend", Indexed: true},
			{ID: "service:frontend/shop-frontend-2024", Label: "shop-frontend-2024", Project: "frontend/shop-frontend-2024", Kind: "frontend", Role: "frontend", Domain: "frontend", Indexed: true},
			{ID: "service:microservices/ms-user", Label: "ms-user", Project: "microservices/ms-user", Kind: "backend", Role: "backend", Domain: "identity", Indexed: true, Incoming: 1},
		},
		Edges: []WorkspaceServiceEdgeRecord{
			{
				ID: "edge:frontend-user", From: "service:frontend/frontend-monorepo", To: "service:microservices/ms-user",
				FromProject: "frontend/frontend-monorepo", ToProject: "microservices/ms-user", Direction: "frontend/frontend-monorepo -> microservices/ms-user",
				Total: 1, Resolved: 1, Risk: "resolved", Endpoints: []string{"GET /users/{userId}"},
			},
		},
	}

	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		serviceMap,
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)

	for _, want := range []string{
		"frontend/frontend",
		"frontend/frontend-monorepo",
		"frontend/frontends",
		"frontend/playwright",
		"frontend/shop-frontend-2024",
		"Scanned, no outgoing API calls detected",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing frontend visibility value %q\n%s", want, html)
		}
	}
}

func TestRenderWorkspaceDashboardHTMLWithModelsUsesFullEndpointTraces(t *testing.T) {
	graph := WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"}
	serviceMap := WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion}
	traces := WorkspaceEndpointTraceIndexRecord{
		SchemaVersion: SchemaVersion,
		Traces: []WorkspaceEndpointTraceRecord{
			{
				ID:          "trace:get-user",
				Route:       "GET /users/{userId}",
				FromProject: "frontend/app",
				ToProject:   "services/ms-user",
				Status:      "RESOLVED",
				Steps: []WorkspaceEndpointTraceStepRecord{
					{ID: "step:component", Kind: "frontend_step", Label: "UserPage.load", File: "src/UserPage.tsx"},
					{ID: "step:contract", Kind: "api_contract", Label: "getUser", File: "src/api/users.ts"},
					{ID: "step:handler", Kind: "backend_handler", Label: "UserController.get", File: "UserController.java"},
				},
			},
		},
	}

	html := RenderWorkspaceDashboardHTMLWithModels(graph, serviceMap, traces, nil, nil)

	for _, want := range []string{"UserPage.load", "frontend_step", "UserController.get"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing full trace value %q\n%s", want, html)
		}
	}
}
