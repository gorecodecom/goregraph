package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderWorkspaceDashboardHTMLEscapesInlineScriptPayload(t *testing.T) {
	injected := `</script><script>globalThis.dashboardInjected=true</script>`
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: injected},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	if strings.Contains(html, injected) {
		t.Fatal("dashboard payload must not contain a literal script-closing injection sequence")
	}
	const prefix = "const workspacePayload = "
	start := strings.Index(html, prefix)
	end := strings.Index(html[start+len(prefix):], ";\n")
	if start < 0 || end < 0 {
		t.Fatal("dashboard payload boundaries not found")
	}
	payload := html[start+len(prefix) : start+len(prefix)+end]
	var decoded struct {
		Graph WorkspaceGraphRecord `json:"graph"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("escaped dashboard payload is not valid JSON: %v", err)
	}
	if decoded.Graph.Root != injected {
		t.Fatalf("escaped payload lost source data: got %q", decoded.Graph.Root)
	}
}

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

func TestWorkspaceDashboardShowsCanonicalContractSummary(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, ContractSummary: WorkspaceContractSummaryRecord{Total: 5, Resolved: 2, MissingRoute: 1, MethodMismatch: 1, DynamicUnresolved: 1}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{`id="contract-count"`, "contract_summary", "contractSummary.resolved"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing contract summary marker %q", want)
		}
	}
}

func TestDashboardGridAvoidsHorizontalOverflowAtNarrowDesktopWidths(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`grid-template-columns:minmax(320px,380px) minmax(560px,1fr) minmax(320px,420px)`,
		`@media (max-width:1240px){.shell{grid-template-columns:1fr;grid-template-areas:"side" "details" "main"}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing responsive grid rule %q", want)
		}
	}
	if strings.Contains(html, `grid-template-columns:420px minmax(760px,1fr) 480px`) {
		t.Fatal("dashboard must not require a 1660px-wide three-column layout")
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
		"focused&&!focused.has(node.id)",
		"focusedMode=state.isolation||state.architectureFocused",
		"nodes=focusedMode?allNodes.filter",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing stable focus behavior %q", want)
		}
	}
	if strings.Contains(html, "state.selected?serviceFocus(state.selected):null;const nodes=focused?allNodes.filter") {
		t.Fatal("ordinary selection must not filter Architecture nodes")
	}
}

func TestDashboardArchitectureShowsDirectionAndExplicitCardPorts(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		`id="arrow-outgoing"`,
		`id="arrow-incoming"`,
		`function architectureDirection(edge,selected)`,
		`function architecturePortOffset(edges,edge,nodeId)`,
		`const span=56`,
		`return incident.length===1?0:-span/2+index*span/(incident.length-1)`,
		`function edgePortPoints(from,to,fromOffset,toOffset)`,
		`class="edge-port source`,
		`class="edge-port target`,
		`class="direction-badge '+direction`,
		`label=direction==="outgoing"?"OUT":"IN"`,
		`marker-end="url(#arrow-`,
		`.edge.incoming{`,
		`stroke-dasharray:7 5`,
		`.service-node rect{fill:var(--color-surface)`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing clear Architecture direction/port contract %q", want)
		}
	}
}

func TestDashboardViewportControlsPreserveUserContext(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"function saveViewport(mode)",
		"function restoreViewport(mode)",
		"function fitVisibleContent()",
		"getBBox()",
		"function zoomAtPoint(factor,x,y)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing viewport behavior %q", want)
		}
	}
	if strings.Contains(html, `state.query="";state.selected=null`) {
		t.Fatal("Fit must not clear query or selection")
	}
}

func TestDashboardViewportUsesVisibleContentAndSVGCoordinates(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`data-viewport-background="true"`,
		"function visibleContentBounds(layer)",
		`querySelectorAll(":scope > :not([data-viewport-background])")`,
		"function screenToSVGPoint(clientX,clientY)",
		"createSVGPoint()",
		"getScreenCTM().inverse()",
		"svg.viewBox.baseVal",
		"const point=screenToSVGPoint(e.clientX,e.clientY)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing reviewed viewport behavior %q", want)
		}
	}
	fitStart := strings.Index(html, "function fitVisibleContent()")
	if fitStart < 0 {
		t.Fatal("dashboard missing Fit function")
	}
	fitEnd := strings.Index(html[fitStart:], "function saveEndpointInventoryScroll()")
	if fitEnd < 0 {
		t.Fatal("dashboard missing fit function boundaries")
	}
	fit := html[fitStart : fitStart+fitEnd]
	if strings.Contains(fit, "clientWidth") || strings.Contains(fit, "clientHeight") {
		t.Fatal("Fit must calculate in SVG user units, not CSS client pixels")
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
		`frontend/app -\u003e services/ms-user`,
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
	for _, want := range []string{`data-step-id`, `data-focus-id`, `trace-card`, `endpoint-inventory-row`, "centerOnPosition", "truncateWord"} {
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

func TestRenderWorkspaceDashboardUsesCanonicalDiagnosticFamilies(t *testing.T) {
	serviceMap := WorkspaceServiceMapRecord{
		SchemaVersion: SchemaVersion,
		DiagnosticFamilies: []DiagnosticFamilyRecord{{
			FamilyID: "diagnostic-family:tree", Code: "indexed_backend_route_missing", Service: "services/tree",
			RoutePattern: "/tree/{variant}", RootCause: "No indexed route matches.", AffectedCount: 2,
			DiagnosticIDs: []string{"diagnostic:a", "diagnostic:b"}, SuggestedCheck: "Check the backend route.",
		}},
	}
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, serviceMap, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{"diagnostic-family:tree", "canonicalDiagnosticFamilies", "canonicalDiagnosticFamilies.length"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard does not consume canonical diagnostic families: missing %q", want)
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

func TestRenderWorkspaceDashboardHTMLEndpointTraceRestoresInventoryScroll(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)

	for _, want := range []string{
		"endpointInventoryScrollTop:0",
		"function saveEndpointInventoryScroll()",
		"state.endpointInventoryScrollTop=workbench.scrollTop",
		"saveEndpointInventoryScroll();resetTraceViewport();state.selected=id",
		"state.selected=state.endpointService;state.focusStep=null;renderList();renderCanvas()",
		"workbench.scrollTop=state.endpointInventoryScrollTop",
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

func TestDashboardEndpointInventoryScrollDoesNotResetFilters(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`endpointFilters:{methods:new Set(),callers:new Set(),providers:new Set(),statuses:new Set()}`,
		`function saveEndpointInventoryScroll()`,
		`function returnToEndpointInventory(){if(!traceById.has(state.selected))return;state.selected=state.endpointService;state.focusStep=null;renderList();renderCanvas();}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing one-shot endpoint viewport transition %q", want)
		}
	}
}

func TestDashboardServiceRelationRowsRemainVisibleButStatic(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`function endpointRowNode(row,cls,id,x,y,w,h,title,meta,selected)`,
		`if(row.kind==="endpoint_trace")return boxNode`,
		`role="presentation"`,
		`if(row.kind==="endpoint_trace"){html+='<button class="relation-row" data-endpoint-id="'`,
		`else{html+='<div class="relation-row static"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing static service relation behavior %q", want)
		}
	}
}

func TestDashboardEndpointCardsReserveSpaceForWrappedTitles(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{`.endpoint-inventory-cell strong{font-size:14px;line-height:1.35;overflow-wrap:anywhere}`, `.endpoint-inventory-row{grid-template-columns:1fr}`, `.relation-row strong{display:block;font-size:14px;overflow-wrap:anywhere;word-break:break-word}`} {
		if !strings.Contains(html, want) {
			t.Fatalf("endpoint rows do not preserve readable wrapped titles: missing %q", want)
		}
	}
}

func TestDashboardEndpointInventoryUsesReadableRowsAtBrowserScale(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		`function renderEndpointInventoryWorkbench()`,
		`class="endpoint-inventory"`,
		`class="endpoint-inventory-header"`,
		`class="endpoint-inventory-row`,
		`data-endpoint-id="`,
		`aria-label="Open endpoint trace`,
		`endpointInventoryCell("Caller",row.from,row.kind)`,
		`endpointInventoryCell("Endpoint",row.route`,
		`endpointInventoryCell("Provider",row.to,row.kind)`,
		`endpointInventoryScrollTop:0`,
		`workbench.scrollTop=state.endpointInventoryScrollTop`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing readable endpoint inventory contract %q", want)
		}
	}
	if strings.Contains(html, `function renderEndpointInventory(){const svg=`) {
		t.Fatal("endpoint inventory still renders its rows into the scaled SVG")
	}
}

func TestDashboardEndpointFiltersSupportDebuggingAndSurviveTraceNavigation(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`id="endpoint-filters"`,
		`data-endpoint-method="DELETE"`,
		`data-endpoint-method="PUT"`,
		`id="endpoint-caller-filter" multiple`,
		`id="endpoint-provider-filter" multiple`,
		`data-endpoint-status="unresolved"`,
		`id="clear-endpoint-filters"`,
		`id="endpoint-filter-summary" aria-live="polite"`,
		`endpointFilters:{methods:new Set(),callers:new Set(),providers:new Set(),statuses:new Set()}`,
		`function endpointRowMatchesFilters(row)`,
		`function clearEndpointFilters()`,
		`returnToEndpointInventory(){if(!traceById.has(state.selected))return;`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing endpoint debugging filter contract %q", want)
		}
	}
}

func TestDashboardCoverageViewExplainsCapabilityMatrix(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Capabilities: []CapabilityRecord{{ID: CapabilitySymbols, Language: "go", Coverage: CoverageComplete, Reason: "Go symbols extracted", FilesSeen: 2}}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		`data-view-mode="coverage" aria-pressed="false">Coverage</button>`,
		`const capabilities=serviceMap.capabilities||[]`,
		`function renderCoverage()`,
		`function coverageGroups()`,
		`function renderCoverageWorkbench()`,
		`class="coverage-summary"`,
		`class="coverage-table"`,
		`Analyzed project/language groups`,
		`Project/language analyzer gaps`,
		`Capability coverage`,
		`COMPLETE`, `PARTIAL`, `UNAVAILABLE`, `FAILED`,
		`Coverage describes analyzer support, not whether source behavior exists.`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing coverage contract %q", want)
		}
	}
	if strings.Contains(html, `function renderCoverage(){const svg=`) {
		t.Fatal("Coverage still renders all capability records into one scaled SVG")
	}
}

func TestDashboardDirectedTraceSupportsTraceFromSelection(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Directed: []DirectedTraceRecord{{ID: "trace", Nodes: []DirectedTraceNodeRecord{{ID: "handler", Role: TraceRoleController, Label: "UserController.delete"}}}}},
		nil, nil,
	)
	for _, want := range []string{`const directedTraces=endpointTraces.directed||[]`, `function traceFromHere(id)`, `Trace from here`, `Controller / handler`, `Evidence`, `Selection does not move or relayout the trace.`} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing directed trace behavior %q", want)
		}
	}
}

func TestDashboardLongDirectedTraceStartsAtReadableScale(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Directed: []DirectedTraceRecord{{
			ID: "long-trace",
			Nodes: []DirectedTraceNodeRecord{
				{ID: "controller", Role: TraceRoleController, Label: "Controller.handle"},
				{ID: "service", Role: TraceRoleFunction, Label: "Service.execute"},
				{ID: "repository", Role: TraceRoleRepository, Label: "Repository.save"},
			},
			MainPath: []string{"controller", "service", "repository"},
		}}},
		nil,
		nil,
	)

	for _, want := range []string{
		`function resetTraceViewport(){state.zoom=1;state.panX=0;state.panY=0;}`,
		`setViewBox(width,760)`,
		`resetTraceViewport();state.selected=id`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("long directed traces do not start at readable browser scale: missing %q", want)
		}
	}
	if strings.Contains(html, `setViewBox(maxX,maxY)`) {
		t.Fatal("directed trace still shrinks the complete path into the initial viewport")
	}
}

func TestDashboardDataFlowShowsMappingsAndExplicitGaps(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, DataFlows: []DataFlowRecord{{ID: "flow", Route: "POST /users", Gaps: []DataFlowGapRecord{{Reason: "Unknown transformation", Confidence: ConfidenceUnknown}}}}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{`data-view-mode="data-flow"`, `const dataFlows=serviceMap.data_flows||[]`, `selectedDataFlow:null`, `function renderDataFlowList()`, `function renderDataFlowWorkbench()`, `Select a data flow`, `data-flow-chain`, `data-flow-node`, `data-flow-gap`, `Unknown transformation`, `aria-pressed="`, `showDataFlowNodeDetails(flow,node)`, `.workbench-kicker{`, `@media (max-width:900px){.data-flow-chain`} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing data-flow contract %q", want)
		}
	}
	if strings.Contains(html, `function renderDataFlow(){const svg=`) {
		t.Fatal("Data Flow still renders every flow into a scaled SVG")
	}
}

func TestDashboardDataFlowUsesSpecificHelpAndClearSelection(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		`function modeHelpText(mode)`,
		`case "data-flow":return "Choose one endpoint to inspect how request data reaches validation, persistence, messages, and the response."`,
		`if(state.mode==="data-flow"){state.selectedDataFlow=null;state.selectedDataFlowNode=null;}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing Data Flow help/clear behavior %q", want)
		}
	}
}

func TestDashboardSwitchesBetweenGraphAndReadableWorkbench(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		`id="workspace-workbench" class="workspace-workbench" hidden`,
		`function setCanvasPresentation(kind,mode)`,
		`function setElementHidden(element,hidden)`,
		`if(hidden)element.setAttribute("hidden","");else element.removeAttribute("hidden")`,
		`main.classList.toggle("workbench-view",workbench)`,
		`main.dataset.activeView=mode`,
		`setElementHidden(document.getElementById("workspace-graph"),workbench)`,
		`setElementHidden(document.getElementById("workspace-workbench"),!workbench)`,
		`setElementHidden(document.querySelector(".canvas-tools"),workbench)`,
		`workbenchModes=new Set(["endpoints","feature-flow","data-flow","diagnostics","coverage"])`,
		`setCanvasPresentation(workbench?"workbench":"graph",state.mode)`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing graph/workbench presentation contract %q", want)
		}
	}
}

func TestDashboardGraphSelectionSupportsKeyboardAndAccessibleNames(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`tabindex="0" role="button" aria-label="`,
		`el.addEventListener("keydown",function(e){if(e.key!=="Enter"&&e.key!==" ")return;e.preventDefault();e.stopPropagation();activateGraphItem(el.dataset.selectId);});`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing accessible graph selection %q", want)
		}
	}
}

func TestDashboardGraphSelectionDispatchesTraceStepsToTraceFocus(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`function isSelectedTraceStep(id){const trace=traceById.get(state.selected);return !!trace&&(trace.steps||[]).some(function(step){return step.id===id;});}`,
		`function activateGraphItem(id){if(isSelectedTraceStep(id)){focusTraceStep(id);return;}selectOrToggleItem(id);}`,
		`activateGraphItem(el.dataset.selectId)`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing trace-step graph dispatch %q", want)
		}
	}
}

func TestDashboardInteractiveSVGExposesFocusableDescendants(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	if !strings.Contains(html, `id="workspace-graph" role="group" aria-label="Directed workspace relationship map"`) {
		t.Fatal("interactive workspace SVG must expose its focusable descendants as a labelled group")
	}
	if strings.Contains(html, `id="workspace-graph" role="img"`) {
		t.Fatal("interactive workspace SVG must not hide descendant buttons behind an image role")
	}
}

func TestDashboardControlStateUsesARIA(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`data-view-mode="architecture" class="active" aria-pressed="true"`,
		`data-kind-filter="all" class="active" aria-pressed="true"`,
		`id="toggle-labels" title="Toggle labels" aria-label="Toggle relationship labels" aria-pressed="false"`,
		`id="zoom-readout" class="readout" aria-live="polite"`,
		`id="result-note" class="result-note" aria-live="polite"`,
		`btn.setAttribute("aria-pressed",String(btn.dataset.viewMode===mode))`,
		`b.setAttribute("aria-pressed",String(b===btn))`,
		`this.setAttribute("aria-pressed",String(state.labels))`,
		`aria-label="Zoom out"`,
		`aria-label="Zoom in"`,
		`aria-label="Reset zoom and pan"`,
		`aria-label="Fit visible graph"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing ARIA state contract %q", want)
		}
	}
}

func TestDashboardMobileGridOrdersDetailsBeforeCanvasAndEnlargesControls(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`grid-template-areas:"side main details"`,
		`.side{grid-area:side`,
		`main{grid-area:main`,
		`.details{grid-area:details`,
		`grid-template-areas:"side" "details" "main"`,
		`.filters button,.modes button,.canvas-tools button{min-height:44px}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing responsive details/control contract %q", want)
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

func TestWorkspaceDashboardBundlesBackgroundArchitectureEdges(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"function architectureBundles",
		"bundle-count",
		"data-architecture-bundle",
		"Unrelated relationships remain grouped",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestWorkspaceDashboardUsesExplicitArchitectureFocus(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"Focus selected",
		"Back to full architecture",
		"architectureFocused",
		"savedFullArchitectureViewport",
		"function enterArchitectureFocus",
		"function leaveArchitectureFocus",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestWorkspaceDashboardShowsCompleteEndpointSourceContext(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: `C:\workspace`},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, EditorURLTemplate: "vscode://file/{file}:{line}"},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Directed: []DirectedTraceRecord{{
			ID:    "trace:users",
			Nodes: []DirectedTraceNodeRecord{{ID: "handler", Role: TraceRoleController, Label: "UserController.get", Symbol: "UserController.get", Project: "services/users", File: `src\UserController.java`, Line: 42}},
		}}},
		nil,
		nil,
	)
	for _, want := range []string{
		"function sourceActions",
		"Copy path",
		"Open source",
		"editor_url_template",
		`detailField("Service"`,
		`detailField("Class / controller"`,
		`detailField("File"`,
		`detailField("Line"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestWorkspaceDashboardExplainsDataFlowPurposeAndEvidence(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		"Endpoints show the call path",
		"Data Flow shows the data path",
		"Exact evidence",
		"Inferred evidence",
		"Weak evidence",
		"Missing evidence",
		"Open related Data Flow",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestWorkspaceDashboardRendersDiagnosticsAsHTMLWorkbench(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{"function renderDiagnosticsWorkbench", "diagnostic-row", "diagnostic-workbench", "normal vertical scrolling"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
	if strings.Contains(html, "function renderDiagnostics(){const svg=") {
		t.Fatal("Diagnostics still uses a fitted SVG")
	}
}

func TestWorkspaceDashboardUsesCanonicalDiagnosticAccounting(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, DiagnosticFamilies: []DiagnosticFamilyRecord{{
			FamilyID: "diagnostic-family:dynamic", Code: "dynamic_endpoint_unresolved", RoutePattern: "/documentdownload/{variant}",
			RootCause: "A dynamic route segment prevents exhaustive static resolution.", ObservedCount: 4, ResolvedCount: 2,
			UnresolvedCount: 2, LikelyOwner: "frontend/app", AffectedProjects: []string{"frontend/app"}, NextChecks: []string{"Inspect variant."},
		}}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{"observed_count", "resolved_count", "unresolved_count", "out_of_scope_count", "likely_owner", "affected_projects", "next_checks"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing canonical diagnostic field %q", want)
		}
	}
}

func TestWorkspaceDashboardExplainsAndCollapsesExpectedCoverage(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Capabilities: []CapabilityRecord{{
			ID: CapabilitySymbols, Project: "app", Language: "markdown", Coverage: CoveragePartial,
			StatusReason: "Generic indexing is best effort.", SourceClass: "documentation",
		}}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{"status_reason", "expected_unavailable", "source_class", "coverage-source-group", "Expected analyzer gaps", "Analyzer execution failed"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing coverage explanation %q", want)
		}
	}
}

func TestWorkspaceDashboardOwnsSelectionPerView(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{"selections:{architecture", "clearDetailsForMode", "Feature Flow context", "Coverage context"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestWorkspaceDashboardRendersCanonicalFeatureFlow(t *testing.T) {
	flow := BuildCanonicalFeatureFlow(WorkspaceFeatureFlowRecord{ID: "flow:users", FrontendProject: "web", FrontendCaller: "loadUsers", FrontendFile: "api.ts", FrontendLine: 8, HTTPMethod: "GET", Path: "/users", BackendProject: "services/users", BackendService: "users", BackendController: "UserController", BackendMethod: "list", BackendFile: "UserController.java", BackendLine: 20, Confidence: "RESOLVED", Reason: "matched contract"})
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, FeatureFlows: []WorkspaceFeatureFlowRecord{flow}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{`data-view-mode="feature-flow"`, "Feature Flow", "renderFeatureFlowWorkbench", "feature-flow-stage", "sourceActions", "Open related Endpoint", "Open related Data Flow", "Resolved evidence", "Missing stage"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing Feature Flow behavior %q", want)
		}
	}
}

func TestWorkspaceDashboardShowsWorkspaceCoverageAndNextScans(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, WorkspaceCoverage: WorkspaceCoverageSummaryRecord{KnownProjects: 4, IndexedProjects: 2, ReferencedServices: 3, IndexedReferencedServices: 1, ContractSummary: WorkspaceContractSummaryRecord{Total: 10, Resolved: 6}, NextScans: []NextScanRecord{{Service: "users", Project: "services/users", AffectedContracts: 4, Command: "goregraph scan services/users"}}}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{"workspace_coverage", "Workspace coverage", "Most useful next scans", "indexed projects", "indexed referenced services", "resolved contracts", "goregraph scan services/users", "Missing coverage is uncertainty"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing workspace coverage behavior %q", want)
		}
	}
}
