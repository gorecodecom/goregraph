package scan

import (
	"strings"
	"testing"
)

func TestRenderWorkspaceDashboardHTMLKeepsPayloadOfflineAfterDecomposition(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{"<!doctype html>", "const workspacePayload =", `id="workspace-graph"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("decomposed dashboard missing %q", want)
		}
	}
	if strings.Contains(html, "https://") || strings.Contains(html, "http://") {
		t.Fatal("dashboard must remain offline")
	}
}

func TestDashboardArchitectureSelectionKeepsFullLayoutUntilExplicitIsolation(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`isolation:false`,
		"function setArchitectureIsolation(enabled)",
		"focused&&!focused.has(n.id)",
		"state.isolation?allNodes.filter",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing stable focus behavior %q", want)
		}
	}
	if strings.Contains(html, "state.selected?serviceFocus(state.selected):null;const nodes=focused?allNodes.filter") {
		t.Fatal("ordinary selection must not filter Architecture nodes")
	}
}

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
		`id="clear-selection"`,
		`id="zoom-in"`,
		`id="zoom-out"`,
		`id="reset-view"`,
		`id="toggle-labels"`,
		`id="graph-layer"`,
		"function buildDiagnosticGroups",
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
		"function renderArchitectureMap",
		"function renderEndpointTrace",
		"function endpointRowsForService",
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
	for _, want := range []string{
		`data-view-mode="architecture"`,
		`data-view-mode="endpoints"`,
		`data-view-mode="diagnostics"`,
		`const state={mode:"architecture"`,
		"Architecture",
		"Endpoints",
		"Diagnostics",
		"Isolate neighborhood",
		"Show full architecture",
		"How was this determined?",
		"function fitVisibleContent",
		"function zoomAtPoint",
		"viewports:new Map()",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing 0.9.1 behavior %q", want)
		}
	}

	for _, unwanted := range []string{
		`data-view-mode="services"`,
		`data-view-mode="raw"`,
		`data-view-mode="issues"`,
		">Focused Service<",
		">Endpoint Paths<",
		">Open Issues<",
	} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("dashboard html retains removed top-level navigation %q", unwanted)
		}
	}

	for _, want := range []string{
		"See how projects and services communicate across the workspace.",
		"Find an endpoint, inspect its consumers, and follow its implementation trace.",
		"Review relationships GoreGraph could not safely confirm and learn what to check next.",
		"Likely code defect",
		"Missing scan coverage",
		"Expected behavior",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing explanatory copy %q", want)
		}
	}
	if strings.Contains(html, "Connections</h3><p class=\"connection-help\">Connections show why this node exists") {
		t.Fatalf("dashboard should not default to raw connection ID detail blocks")
	}
	if strings.Contains(html, "Shared / Internal") {
		t.Fatalf("dashboard should not label unrelated frontend projects as shared/internal")
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

func TestRenderWorkspaceDashboardHTMLExplainsDiagnosticGroupsAndAddsFileLinks(t *testing.T) {
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
		"Diagnostics",
		"function buildDiagnosticGroups",
		"function diagnosticPresentation",
		"Likely code defect",
		"Missing scan coverage",
		"Dynamic or statically ambiguous",
		"Expected behavior",
		"What to check next",
		"file://",
		"src/api/tree.ts:42",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing diagnostic value %q", want)
		}
	}
}

func TestRenderWorkspaceDashboardHTMLSeparatesTreeDiagnosticsByCode(t *testing.T) {
	traces := WorkspaceEndpointTraceIndexRecord{
		SchemaVersion: SchemaVersion,
		Traces: []WorkspaceEndpointTraceRecord{
			{
				ID: "trace:tree-missing", Route: "GET /tree/topics", Path: "/tree/topics",
				ToProject: "microservices/ms-regulationtree", Status: "UNRESOLVED", Risk: "indexed_backend_route_missing",
			},
			{
				ID: "trace:tree-internal", Route: "GET /tree/search", Path: "/tree/search",
				ToProject: "microservices/ms-regulationtree", Status: "RESOLVED", Risk: "frontend_internal_api",
			},
		},
	}

	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		traces,
		nil,
		nil,
	)

	for _, want := range []string{
		"function diagnosticCode(trace)",
		`return "tree-prefix|"+diagnosticCode(t)+"|"+(t.to_project||"unresolved")`,
		"presentation:diagnosticPresentation(t)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html does not separate /tree diagnostics by code: missing %q", want)
		}
	}
}

func TestRenderWorkspaceDashboardHTMLEndpointsCombineInventoryAndTrace(t *testing.T) {
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
		"function renderEndpoints()",
		"function endpointRowsForService(serviceId)",
		"Endpoint inventory",
		"Implementation trace",
		"Back to endpoint inventory",
		"Caller",
		"Endpoint",
		"Provider",
		"trace:get-user",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing combined endpoint behavior %q", want)
		}
	}
	for _, unwanted := range []string{
		"function renderEndpointPaths()",
		"This replaces the low-level raw node cloud",
	} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("dashboard html retains removed endpoint behavior %q", unwanted)
		}
	}
}

func TestRenderWorkspaceDashboardHTMLEndpointTraceRestoresInventoryViewport(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)

	for _, want := range []string{
		"endpointInventoryViewport:null",
		"function saveEndpointInventoryViewport()",
		"state.endpointInventoryViewport={zoom:state.zoom,panX:state.panX,panY:state.panY}",
		"function restoreEndpointInventoryViewport()",
		"state.zoom=viewport.zoom;state.panX=viewport.panX;state.panY=viewport.panY",
		"saveEndpointInventoryViewport();state.selected=id",
		"restoreEndpointInventoryViewport();renderList();renderCanvas()",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing endpoint inventory viewport behavior %q", want)
		}
	}

	returnStart := strings.Index(html, "function returnToEndpointInventory()")
	if returnStart < 0 {
		t.Fatal("dashboard html missing endpoint inventory return function")
	}
	returnEnd := strings.Index(html[returnStart:], "function selectOrToggleItem")
	if returnEnd < 0 {
		t.Fatal("dashboard html missing endpoint inventory return function boundary")
	}
	returnBody := html[returnStart : returnStart+returnEnd]
	if strings.Contains(returnBody, "state.query=") || strings.Contains(returnBody, "state.filter=") || strings.Contains(returnBody, "state.endpointService=") {
		t.Fatalf("endpoint inventory return mutates preserved context: %s", returnBody)
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
