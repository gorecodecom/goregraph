package scan

import (
	"encoding/json"
	"os/exec"
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

func TestWorkspaceDashboardEmbedsCanonicalSymbolProjection(t *testing.T) {
	symbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Symbols: []CanonicalSymbolRecord{{
			ID:              "symbol:user-service",
			Project:         "services/users",
			Language:        "java",
			Kind:            "class",
			Name:            "UserService",
			QualifiedName:   "com.example.users.UserService",
			DeclarationFile: "src/main/java/com/example/users/UserService.java",
			DeclarationLine: 27,
			Analyzer:        "java",
			Confidence:      ConfidenceExact,
			Coverage:        CoverageComplete,
		}},
	}
	usages := WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: SchemaVersion,
		Usages: []CanonicalSymbolUsageRecord{{
			ID:                  "usage:user-controller",
			ProviderSymbolID:    "symbol:user-service",
			ConsumerProject:     "services/users",
			Category:            SymbolUsageDirectReference,
			Language:            "java",
			RelationKind:        "calls",
			TargetQualifiedName: "com.example.users.UserService",
			SourceFile:          "src/main/java/com/example/users/UserController.java",
			SourceLine:          41,
			Confidence:          ConfidenceExact,
			Resolution:          SymbolResolutionExact,
			Reason:              "resolved import and member call",
			Analyzer:            "java",
		}},
	}

	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		symbols,
		usages,
	)

	for _, want := range []string{
		`"symbol_index"`,
		`"symbol_usages"`,
		`"symbol:user-service"`,
		`"usage:user-controller"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing canonical symbol projection %q", want)
		}
	}
}

func TestWorkspaceDashboardOffersCodeExplorerForSelectedService(t *testing.T) {
	serviceMap := WorkspaceServiceMapRecord{
		SchemaVersion: SchemaVersion,
		Nodes: []WorkspaceServiceNodeRecord{
			{ID: "service:users", Label: "Users", Project: "services/users", Indexed: true},
			{ID: "service:docs", Label: "Docs", Project: "services/docs", Indexed: true},
		},
	}
	symbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Symbols: []CanonicalSymbolRecord{{
			ID:              "symbol:user-service",
			Project:         "services/users",
			Language:        "java",
			Kind:            "class",
			Name:            "UserService",
			QualifiedName:   "com.example.users.UserService",
			DeclarationFile: "src/main/java/com/example/users/UserService.java",
			DeclarationLine: 27,
			Analyzer:        "java",
			Confidence:      ConfidenceExact,
			Coverage:        CoverageComplete,
		}},
		Coverage: []SymbolCoverageRecord{{
			Project:    "services/docs",
			Language:   "markdown",
			Capability: "declarations",
			Coverage:   CoverageUnavailable,
			Reason:     "language has no supported symbol analyzer",
		}},
	}

	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		serviceMap,
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		symbols,
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)

	for _, want := range []string{
		"Explore classes &amp; symbols",
		`data-open-code-explorer`,
		`function codeExplorerAvailability(project)`,
		"No supported symbol inventory is available for this project.",
		"language has no supported symbol analyzer",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing selected-service Code Explorer contract %q", want)
		}
	}
	if strings.Contains(html, `data-view-mode="code-explorer"`) {
		t.Fatal("Code Explorer must only be entered from a selected service, not as a global empty view")
	}
}

func TestWorkspaceDashboardCodeExplorerRendersSemanticInventorySearchAndCounts(t *testing.T) {
	symbols := WorkspaceSymbolIndexRecord{
		SchemaVersion: SchemaVersion,
		Symbols: []CanonicalSymbolRecord{
			{
				ID:               "symbol:user-service",
				Project:          "services/users",
				Module:           "users-core",
				Package:          "com.example.users",
				WorkspacePackage: "workspace/users",
				Language:         "java",
				Kind:             "class",
				Name:             "UserService",
				QualifiedName:    "com.example.users.UserService",
				DeclarationFile:  "src/main/java/com/example/users/UserService.java",
				DeclarationLine:  27,
				Analyzer:         "java",
				Confidence:       ConfidenceExact,
				Coverage:         CoverageComplete,
			},
			{ID: "symbol:other-1", Project: "services/orders", Language: "java", Kind: "class", Name: "UserService", QualifiedName: "orders.UserService", DeclarationFile: "orders/UserService.java", Analyzer: "java", Confidence: ConfidenceExact, Coverage: CoverageComplete},
			{ID: "symbol:other-2", Project: "services/admin", Language: "java", Kind: "class", Name: "UserService", QualifiedName: "admin.UserService", DeclarationFile: "admin/UserService.java", Analyzer: "java", Confidence: ConfidenceExact, Coverage: CoverageComplete},
			{ID: "symbol:other-3", Project: "frontend/app", Language: "typescript", Kind: "class", Name: "UserService", QualifiedName: "app.UserService", DeclarationFile: "src/UserService.ts", Analyzer: "typescript", Confidence: ConfidenceExact, Coverage: CoverageComplete},
		},
		Coverage: []SymbolCoverageRecord{{
			Project:    "services/users",
			Language:   "java",
			Capability: "declarations",
			Coverage:   CoverageComplete,
			Reason:     "canonical declarations indexed",
		}},
	}
	usages := WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: SchemaVersion,
		Usages: []CanonicalSymbolUsageRecord{
			{ID: "usage:direct", ProviderSymbolID: "symbol:user-service", ConsumerProject: "services/users", Category: SymbolUsageDirectReference, Language: "java", RelationKind: "calls", SourceFile: "src/UserController.java", Confidence: ConfidenceExact, Resolution: SymbolResolutionExact, Reason: "direct call", Analyzer: "java"},
			{ID: "usage:api", ProviderSymbolID: "symbol:user-service", ConsumerProject: "frontend/app", Category: SymbolUsageReachedThroughAPI, Language: "typescript", RelationKind: "http_call", SourceFile: "src/api/users.ts", Confidence: ConfidenceResolved, Resolution: SymbolResolutionExact, Reason: "API path", Analyzer: "typescript"},
		},
	}

	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Nodes: []WorkspaceServiceNodeRecord{{ID: "service:users", Label: "Users", Project: "services/users", Indexed: true}}},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		symbols,
		usages,
	)

	for _, want := range []string{
		`<section class="code-explorer"`,
		`<aside class="code-inventory"`,
		`<h2 class="code-symbol-group-title"`,
		`data-code-symbol`,
		`aria-pressed="`,
		`id="code-search"`,
		`symbol.name,symbol.qualified_name,symbol.export_name,symbol.package,symbol.module,symbol.workspace_package,symbol.declaration_file`,
		`symbol.language`,
		`symbol.kind`,
		`symbol.qualified_name||symbol.export_name`,
		`sourceActions(symbol.project,symbol.declaration_file,symbol.declaration_line)`,
		`directCount`,
		`apiCount`,
		`symbol.confidence`,
		`symbol.coverage`,
		`unrelated symbols share the name`,
		`and were excluded.`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing Code Explorer inventory contract %q", want)
		}
	}
}

func TestWorkspaceDashboardCodeExplorerSeparatesUsageTabsFiltersAndEvidence(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)

	for _, want := range []string{
		"Direct references",
		"Reached through API",
		">All<",
		"Ambiguous / unresolved",
		`data-code-tab="direct"`,
		`data-code-tab="api"`,
		`data-code-tab="all"`,
		`data-code-tab="uncertainty"`,
		`id="code-filter-consumer"`,
		`id="code-filter-category"`,
		`id="code-filter-relation-kind"`,
		`id="code-filter-language"`,
		`id="code-filter-confidence"`,
		`usage.category==="direct_reference"`,
		`usage.category==="reached_through_api"`,
		`["ambiguous","unresolved"].includes(usage.category)`,
		"Canonical provider",
		"Canonical consumer",
		"Reason",
		"Evidence",
		"Dependency / artifact evidence",
		"Ordered API steps",
		"Limitations",
		`sourceActions(usage.consumer_project,usage.source_file,usage.source_line)`,
		"No projected usage evidence exists for this symbol in indexed coverage; this is not proof that the symbol is unused.",
		`symbolUsages.coverage`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing Code Explorer usage contract %q", want)
		}
	}
}

func TestWorkspaceDashboardCodeExplorerIsAccessibleResponsiveAndRestoresArchitecture(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)

	for _, want := range []string{
		`state.architectureReturn = {
    selected: state.selected,
    domain: state.domainFocus,
    direction: state.directionFocus,
    risk: state.riskFocus,
    zoom: state.zoom,
    panX: state.panX,
    panY: state.panY
  };`,
		`state.selected=saved.selected`,
		`state.architectureDomain=saved.domain`,
		`state.architectureDirection=saved.direction`,
		`state.architectureRiskOnly=saved.risk`,
		`state.zoom=saved.zoom`,
		`state.panX=saved.panX`,
		`state.panY=saved.panY`,
		`focusReturnedArchitectureService`,
		`event.key==="Enter"||event.key===" "`,
		`aria-pressed="`,
		`aria-describedby="code-explorer-help"`,
		`.code-explorer button:focus-visible`,
		`@media (min-width:1241px) and (max-width:1439px)`,
		`@media (min-width:1440px) and (max-width:1679px)`,
		`@media (min-width:1680px)`,
		`@media (max-width:1679px)`,
		`grid-template-columns:minmax(320px,.8fr) minmax(520px,1.4fr)`,
		`overflow-y:auto`,
		`min-height:44px`,
		`@media (prefers-reduced-motion:reduce)`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing Code Explorer accessibility contract %q", want)
		}
	}
}

func TestWorkspaceDashboardCodeExplorerReviewRegressions(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)

	for _, want := range []string{
		`const symbolUsagesByConsumer=new Map()`,
		`usage.consumer_symbol_id`,
		`.concat(symbolUsagesByConsumer.get(symbolID)||[])`,
		`function wireCodeExplorerChrome(workbench)`,
		`wireCodeExplorerChrome(workbench);`,
		`No projected usage evidence exists for this symbol in indexed coverage; this is not proof that the symbol is unused.`,
		`No direct references match this view. Reached-through-API evidence is available in its own tab.`,
		`No usage evidence matches the current tab and filters.`,
		`main.classList.contains("workbench-view")`,
		`#workspace-workbench [data-select-id]`,
		`#graph-layer [data-select-id]`,
		`@media (max-width:1679px){.code-explorer-grid{grid-template-columns:1fr}`,
		`@media (min-width:1680px){.code-explorer{max-width:1480px}.code-explorer-grid{grid-template-columns:minmax(320px,.8fr) minmax(520px,1.4fr)`,
		`data-code-tab="direct" aria-pressed="`,
		`data-code-symbol="`,
		`data-code-usage="`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing Code Explorer review fix %q", want)
		}
	}

	renderStart := strings.Index(html, "function renderCodeExplorer()")
	renderEnd := strings.Index(html[renderStart:], "function setElementHidden")
	if renderStart < 0 || renderEnd < 0 {
		t.Fatal("dashboard missing Code Explorer render boundaries")
	}
	renderCodeExplorer := html[renderStart : renderStart+renderEnd]
	wireIndex := strings.Index(renderCodeExplorer, "wireCodeExplorerChrome(workbench);")
	emptyIndex := strings.Index(renderCodeExplorer, "if(!symbol)")
	if wireIndex < 0 || emptyIndex < 0 || wireIndex > emptyIndex {
		t.Fatal("Code Explorer must wire search and back controls before the no-symbol return")
	}

	for _, forbidden := range []string{
		`role="tablist"`,
		`role="tab"`,
		`role="listbox"`,
		`role="option"`,
	} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("Code Explorer must not advertise incomplete composite widget semantics %q", forbidden)
		}
	}
}

func TestWorkspaceDashboardCodeExplorerPreservesUsageDirectionAndMatrixFocus(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)

	for _, want := range []string{
		`symbolUsageRecords.forEach(function(usage){if(usage.transport!=="http"||!["reached_through_api","ambiguous","unresolved"].includes(usage.category))return;const consumer=usage.consumer_symbol_id`,
		`architectureSelectionOrigin:"graph"`,
		`architectureReturnFocus:null`,
		`state.architectureSelectionOrigin="matrix-service"`,
		`state.architectureSelectionOrigin="matrix-provider"`,
		`state.architectureReturnFocus=state.architectureSelectionOrigin`,
		`function focusReturnedArchitectureService(origin)`,
		`#workspace-workbench [data-matrix-provider]`,
		`const focusOrigin=state.architectureReturnFocus`,
		`focusReturnedArchitectureService(focusOrigin)`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing Code Explorer usage direction or matrix focus contract %q", want)
		}
	}

	consumerIndexStart := strings.Index(html, `symbolUsageRecords.forEach(function(usage){if(usage.transport!=="http"||!["reached_through_api","ambiguous","unresolved"].includes(usage.category))return;const consumer=usage.consumer_symbol_id`)
	if consumerIndexStart < 0 {
		t.Fatal("dashboard must index consumer-side usages only after the outbound HTTP category guard")
	}
	consumerIndexEnd := strings.Index(html[consumerIndexStart:], `});`)
	if consumerIndexEnd < 0 {
		t.Fatal("dashboard must index consumer-side usages only after the outbound HTTP category guard")
	}
	consumerIndex := html[consumerIndexStart : consumerIndexStart+consumerIndexEnd]
	if !strings.Contains(consumerIndex, `records.push(usage)`) {
		t.Fatal("dashboard must retain outbound HTTP usages for the consumer symbol")
	}
}

func TestWorkspaceDashboardCodeExplorerIncludesOnlyOutboundHTTPConsumerUsages(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for embedded Code Explorer model tests")
	}

	usages := []CanonicalSymbolUsageRecord{
		{ID: "usage:http-exact", ProviderSymbolID: "symbol:backend", ConsumerProject: "frontend/app", ConsumerSymbolID: "symbol:frontend", Category: SymbolUsageReachedThroughAPI, Transport: "http"},
		{ID: "usage:http-ambiguous", ProviderSymbolID: "symbol:backend-a", ConsumerProject: "frontend/app", ConsumerSymbolID: "symbol:frontend", Category: SymbolUsageAmbiguous, CandidateSymbolIDs: []string{"symbol:backend-a", "symbol:backend-b"}, Transport: "http"},
		{ID: "usage:http-unresolved", ConsumerProject: "frontend/app", ConsumerSymbolID: "symbol:frontend", Category: SymbolUsageUnresolved, Transport: "http"},
		{ID: "usage:direct-call", ProviderSymbolID: "symbol:backend", ConsumerProject: "frontend/app", ConsumerSymbolID: "symbol:frontend", Category: SymbolUsageDirectReference},
		{ID: "usage:ambiguous-import", ProviderSymbolID: "symbol:backend-a", ConsumerProject: "frontend/app", ConsumerSymbolID: "symbol:frontend", Category: SymbolUsageAmbiguous, CandidateSymbolIDs: []string{"symbol:backend-a", "symbol:backend-b"}},
	}
	encodedUsages, err := json.Marshal(usages)
	if err != nil {
		t.Fatalf("encode Code Explorer usage fixture: %v", err)
	}
	section := func(start, end string) string {
		t.Helper()
		from := strings.Index(workspaceDashboardScript, start)
		if from < 0 {
			t.Fatalf("dashboard script missing section start %q", start)
		}
		to := strings.Index(workspaceDashboardScript[from:], end)
		if to < 0 {
			t.Fatalf("dashboard script missing section end %q", end)
		}
		return workspaceDashboardScript[from : from+to]
	}
	source := strings.Join([]string{
		`const symbolUsageRecords=` + string(encodedUsages) + `;`,
		section("const symbolUsagesByProvider=new Map();", "const state="),
		`const state={codeTab:"all"};`,
		section("function codeUsagesForSymbol(symbolID)", "function codeUsageCounts(symbolID)"),
		section("function codeUsageCounts(symbolID)", "function codeSameNameNote(symbol)"),
		section("function codeTabMatches(usage,tab)", "function codeUsageMatchesFilters(usage)"),
		section("function codeTabsHTML(usages)", "function codeUsageRowsHTML(usages)"),
		`const usages=codeUsagesForSymbol("symbol:frontend");`,
		`const ids=function(tab){return usages.filter(function(usage){return codeTabMatches(usage,tab);}).map(function(usage){return usage.id;}).sort();};`,
		`process.stdout.write(JSON.stringify({all:ids("all"),direct:ids("direct"),api:ids("api"),uncertainty:ids("uncertainty"),counts:codeUsageCounts("symbol:frontend"),hasUncertaintyTab:codeTabsHTML(usages).includes('data-code-tab="uncertainty"')}));`,
	}, "\n")
	output, err := exec.Command(node, "-e", source).CombinedOutput()
	if err != nil {
		t.Fatalf("Code Explorer consumer usage model failed: %v\n%s", err, output)
	}
	var result struct {
		All         []string `json:"all"`
		Direct      []string `json:"direct"`
		API         []string `json:"api"`
		Uncertainty []string `json:"uncertainty"`
		Counts      struct {
			Direct int `json:"directCount"`
			API    int `json:"apiCount"`
		} `json:"counts"`
		HasUncertaintyTab bool `json:"hasUncertaintyTab"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode Code Explorer consumer usage result: %v\n%s", err, output)
	}
	if got := strings.Join(result.All, ","); got != "usage:http-ambiguous,usage:http-exact,usage:http-unresolved" {
		t.Fatalf("all consumer usages = %q, want only exact, ambiguous, and unresolved HTTP evidence", got)
	}
	if len(result.Direct) != 0 || result.Counts.Direct != 0 {
		t.Fatalf("consumer-side direct import/call leaked into incoming direct references: %#v", result)
	}
	if got := strings.Join(result.API, ","); got != "usage:http-exact" || result.Counts.API != 1 {
		t.Fatalf("exact API tab/count = %q/%d, want one proven HTTP reachability usage", got, result.Counts.API)
	}
	if got := strings.Join(result.Uncertainty, ","); got != "usage:http-ambiguous,usage:http-unresolved" || !result.HasUncertaintyTab {
		t.Fatalf("HTTP uncertainty tab = %q, visible=%v", got, result.HasUncertaintyTab)
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
		`height:100vh;min-height:0;overflow:hidden`,
		`@media (max-width:1240px){.shell{grid-template-columns:1fr;grid-template-areas:"side" "main" "details";height:auto;min-height:100vh;overflow:visible}`,
		`height:auto;min-height:100vh;overflow:visible`,
		`.side,.details{max-height:55vh`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing responsive grid rule %q", want)
		}
	}
	if strings.Contains(html, `grid-template-columns:420px minmax(760px,1fr) 480px`) {
		t.Fatal("dashboard must not require a 1660px-wide three-column layout")
	}
}

func TestDashboardArchitectureSelectionKeepsCanonicalLayoutAcrossFocusStates(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"architectureFocusModel",
		`architectureDirection:"both"`,
		`architectureRiskOnly:false`,
		"allNodes=serviceNodes.slice()",
		"layout=architectureLayout(allNodes,width)",
		"allEdges=canonicalEdges",
		"emphasizedEdges=filteredServiceEdges().filter",
		"focus=architectureFocusModel(allNodes,allEdges",
		"focused=!!(state.selected||state.architectureDomain||state.architectureRiskOnly)",
		"state.positions=layout.positions",
		"!focus.nodeIDs.has(node.id)",
		"!focus.edgeIDs.has(edge.id)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing stable focus behavior %q", want)
		}
	}
	if strings.Contains(html, `nodes=focusedMode?allNodes.filter`) {
		t.Fatal("architecture focus must dim instead of filtering nodes")
	}
}

func TestDashboardArchitectureKeepsCanonicalEdgeAndBundleMembership(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"canonicalEdges=serviceEdges.filter",
		"allEdges=canonicalEdges",
		"emphasizedEdges=filteredServiceEdges().filter",
		"emphasizedEdgeIds=new Set(emphasizedEdges.map",
		"bundleModels=architectureBundles(backgroundEdges,nodeByID)",
		"directEdges=allEdges.filter",
		"backgroundEdges=allEdges.filter",
		"function architectureEdgeDimmed(edge,emphasizedEdgeIds,focus,focused)",
		`(dim?' dim':'')`,
		`.edge.architecture-bundle.dim{`,
		`.bundle-count.dim,.architecture-call-pill.dim{`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing canonical architecture edge behavior %q", want)
		}
	}
	for _, unstable := range []string{
		"visibleEdges=filteredServiceEdges()",
		"architectureBundles(canonicalEdges)",
	} {
		if strings.Contains(html, unstable) {
			t.Fatalf("architecture edge membership still changes with emphasis %q", unstable)
		}
	}
}

func TestDashboardArchitectureDomainLanesSupportPointerAndKeyboardFocus(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`architectureDomain:null`,
		`architectureDirection:"both"`,
		`architectureRiskOnly:false`,
		`savedArchitectureDomainViewport:null`,
		`savedArchitectureServiceViewport:null`,
		"function setArchitectureDomain(domain)",
		"function setArchitectureDirection(direction)",
		"function resetArchitectureFocus()",
		"function renderArchitectureDomainControls(domains)",
		"function wireArchitectureDomainControls()",
		`document.querySelectorAll("[data-architecture-domain]")`,
		`element.addEventListener("pointerdown"`,
		`element.addEventListener("click"`,
		`event.key==="Enter"||event.key===" "`,
		"event.preventDefault()",
		"setArchitectureDomain(element.dataset.architectureDomain)",
		"wireArchitectureDomainControls();",
		"state.architectureDomain===domain.id",
		"architectureStringCompare",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing interactive architecture domain behavior %q", want)
		}
	}
	for _, localeSensitive := range []string{
		"Array.from(bundles.values()).sort(function(a,b){return a.id.localeCompare(b.id);})",
		"architectureDomainLabel(serviceDomain(a)).localeCompare",
	} {
		if strings.Contains(html, localeSensitive) {
			t.Fatalf("architecture ordering remains locale-sensitive: %q", localeSensitive)
		}
	}
}

func TestDashboardArchitectureMatrixUsesSharedServiceSelectionState(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/workspace"},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"function setArchitectureServiceSelection(id)",
		`state.architectureDirection="both"`,
		"setArchitectureServiceSelection(edge.from)",
		"setArchitectureServiceSelection(button.dataset.matrixProvider)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing shared Architecture selection state %q", want)
		}
	}
	matrixStart := strings.Index(html, "function renderArchitectureMatrix()")
	if matrixStart < 0 {
		t.Fatal("dashboard missing Architecture matrix function boundaries")
	}
	matrixEnd := strings.Index(html[matrixStart:], "function serviceFocus(id)")
	if matrixEnd < 0 {
		t.Fatal("dashboard missing Architecture matrix function boundaries")
	}
	matrix := html[matrixStart : matrixStart+matrixEnd]
	for _, direct := range []string{"state.selected=edge.from", "state.selected=button.dataset.matrixProvider", "state.selections.architecture=state.selected"} {
		if strings.Contains(matrix, direct) {
			t.Fatalf("Architecture matrix bypasses shared selection state with %q", direct)
		}
	}
}

func TestDashboardSearchAndKindFiltersUseSharedDeselectionState(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"function clearSelectedItemState()",
		`applyViewport(saved);state.savedArchitectureServiceViewport=null`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing shared deselection state %q", want)
		}
	}
	clearStart := strings.Index(html, "function clearSelectedItemState()")
	clearEnd := strings.Index(html[clearStart:], "function clearSelection()")
	if clearStart < 0 || clearEnd < 0 {
		t.Fatal("dashboard missing shared deselection function boundaries")
	}
	clearState := html[clearStart : clearStart+clearEnd]
	for _, want := range []string{
		`if(state.mode==="architecture"){clearArchitectureServiceState();return;}`,
	} {
		if !strings.Contains(clearState, want) {
			t.Fatalf("shared deselection state missing %q", want)
		}
	}
	if !strings.Contains(html, "function hideArchitectureSelectionActions()") {
		t.Fatal("dashboard missing shared Architecture selection-action cleanup")
	}
	searchStart := strings.Index(html, `document.getElementById("workspace-search").addEventListener`)
	kindStart := strings.Index(html, `document.querySelectorAll("[data-kind-filter]").forEach(function(btn){btn.addEventListener`)
	if searchStart < 0 || kindStart < 0 {
		t.Fatal("dashboard missing search or kind-filter handler boundaries")
	}
	searchEnd := strings.Index(html[searchStart:], `document.querySelectorAll("[data-view-mode]")`)
	kindEnd := strings.Index(html[kindStart:], `document.querySelectorAll("[data-endpoint-method]")`)
	if searchEnd < 0 || kindEnd < 0 {
		t.Fatal("dashboard missing search or kind-filter handler boundaries")
	}
	for name, handler := range map[string]string{
		"search":      html[searchStart : searchStart+searchEnd],
		"kind filter": html[kindStart : kindStart+kindEnd],
	} {
		if !strings.Contains(handler, "clearSelectedItemState();") {
			t.Fatalf("%s does not use shared deselection state", name)
		}
		for _, direct := range []string{"state.selected=null", "state.selections[state.mode]=null", "state.isolation=false"} {
			if strings.Contains(handler, direct) {
				t.Fatalf("%s bypasses shared deselection state with %q", name, direct)
			}
		}
	}
}

func TestDashboardArchitectureIsolationChangesPresentationOnly(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`presentationMode=state.architectureFocused?"focused":state.isolation?"isolated":"context"`,
		`architecture-presentation '+presentationMode`,
		`state.isolation?'Isolated neighborhood'`,
		`.architecture-presentation.isolated .edge.dim,.architecture-presentation.focused .edge.dim{opacity:.02}`,
		`.architecture-presentation.isolated .service-node.dim,.architecture-presentation.focused .service-node.dim{opacity:.12}`,
		"layout=architectureLayout(allNodes,width)",
		"canonicalEdges=serviceEdges.filter",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing presentation-only Architecture isolation %q", want)
		}
	}
}

func TestDashboardDomainAndResetUseSharedArchitectureServiceCleanup(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"function clearArchitectureServiceState()",
		"const restoredServiceViewport=restoreArchitectureServiceViewport()",
		"if(!restoredServiceViewport&&state.architectureFocused&&state.savedFullArchitectureViewport)applyViewport(state.savedFullArchitectureViewport)",
		"state.isolation=false;state.architectureFocused=false;state.savedFullArchitectureViewport=null",
		"hideArchitectureSelectionActions()",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing shared Architecture cleanup %q", want)
		}
	}
	functionBody := func(start, end string) string {
		t.Helper()
		from := strings.Index(html, start)
		if from < 0 {
			t.Fatalf("dashboard missing function start %q", start)
		}
		to := strings.Index(html[from:], end)
		if to < 0 {
			t.Fatalf("dashboard missing function end %q", end)
		}
		return html[from : from+to]
	}
	domain := functionBody("function setArchitectureDomain(domain)", "function setArchitectureDirection(direction)")
	reset := functionBody("function resetArchitectureFocus()", "function setArchitectureServiceSelection(id)")
	for name, body := range map[string]string{"domain": domain, "reset": reset} {
		if !strings.Contains(body, "clearArchitectureServiceState();") {
			t.Fatalf("%s transition bypasses shared Architecture cleanup", name)
		}
		for _, direct := range []string{"state.selected=null", "state.selections.architecture=null", "state.isolation=false", "state.architectureFocused=false", "state.savedFullArchitectureViewport=null"} {
			if strings.Contains(body, direct) {
				t.Fatalf("%s transition directly clears service state with %q", name, direct)
			}
		}
	}
	if cleanup, saveDomain := strings.Index(domain, "clearArchitectureServiceState();"), strings.Index(domain, "if(domain&&!state.architectureDomain)state.savedArchitectureDomainViewport="); cleanup < 0 || saveDomain < 0 || cleanup > saveDomain {
		t.Fatal("domain transition must restore service/focused viewport before saving the domain viewport")
	}
	if saveDomain, cleanup, restoreDomain := strings.Index(reset, "const savedDomain=state.savedArchitectureDomainViewport"), strings.Index(reset, "clearArchitectureServiceState();"), strings.Index(reset, "if(savedDomain)applyViewport(savedDomain)"); saveDomain < 0 || cleanup < saveDomain || restoreDomain < cleanup {
		t.Fatal("reset must save the domain viewport, clear service focus, then restore the domain viewport")
	}
}

func TestDashboardArchitectureAutoFitUsesCanonicalDirectNeighborhood(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		"function architectureDirectNeighborhood(edges,selected)",
		"neighborhoodNodeIDs=architectureDirectNeighborhood(allEdges,state.selected)",
		"fitArchitectureNeighborhoodIfNeeded(neighborhoodNodeIDs)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing canonical direct-neighborhood auto-fit %q", want)
		}
	}
	if strings.Contains(html, "fitArchitectureNeighborhoodIfNeeded(focus.nodeIDs)") {
		t.Fatal("Architecture auto-fit must not reuse direction/risk/domain emphasis nodes")
	}
}

func TestDashboardArchitectureAutoFitRunsOnceForNewServiceSelection(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`pendingArchitectureServiceFit:null`,
		`const changed=state.selected!==id`,
		`if(changed)state.pendingArchitectureServiceFit=id`,
		`state.pendingArchitectureServiceFit=null`,
		`if(state.pendingArchitectureServiceFit===state.selected){state.pendingArchitectureServiceFit=null;fitArchitectureNeighborhoodIfNeeded(neighborhoodNodeIDs);}`,
		`if(!state.architectureFocused&&!state.savedArchitectureServiceViewport)state.savedArchitectureServiceViewport=`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing one-shot Architecture auto-fit lifecycle %q", want)
		}
	}
	functionBody := func(start, end string) string {
		t.Helper()
		from := strings.Index(html, start)
		if from < 0 {
			t.Fatalf("dashboard missing function start %q", start)
		}
		to := strings.Index(html[from:], end)
		if to < 0 {
			t.Fatalf("dashboard missing function end %q", end)
		}
		return html[from : from+to]
	}
	selection := functionBody("function setArchitectureServiceSelection(id)", "function restoreArchitectureDomainFocus(domain,elementName)")
	if strings.Contains(selection, `state.pendingArchitectureServiceFit=id;state.selected=id`) {
		t.Fatal("Architecture auto-fit must not be armed before confirming the service selection changed")
	}
	cleanup := functionBody("function clearArchitectureServiceState()", "function resetArchitectureFocus()")
	if !strings.Contains(cleanup, `state.pendingArchitectureServiceFit=null`) {
		t.Fatal("Architecture service cleanup must clear pending auto-fit state")
	}
	render := functionBody("function renderArchitectureMap()", "function architectureEdgeID(edge)")
	if strings.Contains(render, `if(state.selected){showServiceDetails(state.selected,false);fitArchitectureNeighborhoodIfNeeded(neighborhoodNodeIDs);}`) {
		t.Fatal("Architecture render must not auto-fit every time a service remains selected")
	}
	for _, rerender := range []string{
		`function setArchitectureDirection(direction){`,
		`document.getElementById("architecture-risk-toggle").addEventListener`,
		`document.getElementById("toggle-labels").addEventListener`,
		`window.addEventListener("resize",renderCanvas)`,
	} {
		start := strings.Index(html, rerender)
		if start < 0 {
			t.Fatalf("dashboard missing rerender path %q", rerender)
		}
		lineEnd := strings.Index(html[start:], "\n")
		if lineEnd < 0 {
			lineEnd = len(html) - start
		}
		if strings.Contains(html[start:start+lineEnd], "pendingArchitectureServiceFit") {
			t.Fatalf("rerender path %q must not arm Architecture auto-fit", rerender)
		}
	}
}

func TestDashboardArchitectureShowsDirectionalArrowsAndExplicitCardPorts(t *testing.T) {
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
		`marker-end="url(#arrow-`,
		`.edge.incoming{`,
		`stroke-dasharray:7 5`,
		`.service-node rect{fill:var(--color-surface)`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing clear Architecture direction/port contract %q", want)
		}
	}
	for _, obsolete := range []string{
		`class="direction-badge '+direction`,
		`label=direction==="outgoing"?"OUT":"IN"`,
	} {
		if strings.Contains(html, obsolete) {
			t.Fatalf("dashboard still renders obsolete Architecture direction badge %q", obsolete)
		}
	}
}

func TestWorkspaceDashboardKeepsAllSelectedServiceRelationsUnbundled(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`const directEdges=allEdges.filter(function(edge){return focus.edgeIDs.has(edge.id)&&state.selected&&(edge.from===state.selected||edge.to===state.selected);})`,
		`const directEdgeIDs=new Set(directEdges.map(function(edge){return edge.id;}));`,
		`const backgroundEdges=allEdges.filter(function(edge){return !directEdgeIDs.has(edge.id);})`,
		`const nodeByID=new Map(allNodes.map(function(node){return [node.id,node];}));`,
		`architecturePortOffset(directEdges,edge,edge.from)`,
		`architecturePortOffset(directEdges,edge,edge.to)`,
		`marker-end="url(#arrow-`,
		`backgroundSourceLayer+backgroundTrunkLayer+backgroundTargetLayer+directEdgeLayer`,
		`portLayer+labelLayer`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing direct relationship contract %q", want)
		}
	}
	renderStart := strings.Index(html, "function renderArchitectureMap()")
	if renderStart < 0 {
		t.Fatal("dashboard missing Architecture renderer start")
	}
	renderEnd := strings.Index(html[renderStart:], "function architectureEdgeID(edge)")
	if renderEnd < 0 {
		t.Fatal("dashboard missing Architecture renderer boundaries")
	}
	render := html[renderStart : renderStart+renderEnd]
	for _, obsolete := range []string{"Caller", "Called", "OUT", "direction-badge", "architectureBundles(allEdges)"} {
		if strings.Contains(render, obsolete) {
			t.Fatalf("Architecture renderer retains obsolete or duplicate selected-edge behavior %q", obsolete)
		}
	}
}

func TestWorkspaceDashboardExposesPersistentSummaryAndInspectableStaticCallBadges(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`id="architecture-relationship-summary"`,
		`aria-live="polite"`,
		`id="architecture-relationship-tooltip"`,
		`role="tooltip"`,
		`data-architecture-edge`,
		`data-architecture-bundle`,
		`tabindex="0"`,
		`not runtime request frequency`,
		`function renderArchitectureSummary`,
		`function architectureRelationshipBadge`,
		`function architectureBundleBadge`,
		`function wireArchitectureRelationshipBadges`,
		`function showArchitectureRelationshipDetails`,
		`function showArchitectureTooltip`,
		`function hideArchitectureTooltip`,
		`detailField("Direction"`,
		`detailField("Endpoints"`,
		`detailField("Problems"`,
		`detailField("Evidence"`,
		`Reset focus`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing summary contract %q", want)
		}
	}
	start := strings.Index(html, "function wireArchitectureRelationshipBadges()")
	end := strings.Index(html, "function architectureDirection(")
	if start < 0 || end <= start {
		t.Fatal("dashboard missing inspectable badge handler boundaries")
	}
	handler := html[start:end]
	for _, forbidden := range []string{"architectureLayout", "setViewBox", "fitArchitecture", "zoomAtPoint", "panBy", "setArchitectureServiceSelection", "renderCanvas"} {
		if strings.Contains(handler, forbidden) {
			t.Fatalf("badge inspection changes architecture layout or viewport through %q", forbidden)
		}
	}
	if strings.Contains(html, "runtime calls") {
		t.Fatal("dashboard labels static relationships as runtime calls")
	}
}

func TestWorkspaceDashboardRestoresDimCallBadgeOpacityOnKeyboardFocus(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	const opacityRule = `.architecture-presentation .bundle-count.dim:focus-visible,.architecture-presentation .architecture-call-pill.dim:focus-visible{opacity:1}`
	const strokeRule = `.architecture-call-pill[role="button"]:focus-visible rect,.bundle-count[role="button"]:focus-visible rect{stroke:var(--color-focus);stroke-width:3}`
	for _, want := range []string{opacityRule, strokeRule} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing visible dim-badge focus contract %q", want)
		}
	}
	dimRule := strings.Index(html, `.architecture-presentation.isolated .bundle-count.dim`)
	focusRule := strings.Index(html, opacityRule)
	if dimRule < 0 || focusRule <= dimRule {
		t.Fatalf("focus opacity override must follow dimming rules: dim=%d focus=%d", dimRule, focusRule)
	}
}

func TestWorkspaceDashboardUsesOnlySummaryAsArchitectureLiveRegion(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	start := strings.Index(html, `<section id="architecture-focus-panel"`)
	if start < 0 {
		t.Fatal("dashboard missing Architecture focus panel")
	}
	end := strings.Index(html[start:], `</section>`)
	if end < 0 {
		t.Fatal("dashboard missing Architecture focus panel boundary")
	}
	panel := html[start : start+end]
	if count := strings.Count(panel, `aria-live="polite"`); count != 1 {
		t.Fatalf("Architecture focus panel live-region count = %d, want 1", count)
	}
	if !strings.Contains(panel, `id="architecture-relationship-summary" class="architecture-relationship-summary" aria-live="polite"`) {
		t.Fatal("Architecture relationship summary must be the polite live region")
	}
	if strings.Contains(panel, `id="architecture-focus-panel" class="architecture-focus-panel" aria-label="Architecture focus" aria-live=`) {
		t.Fatal("Architecture focus panel must not nest a second live region")
	}
}

func TestWorkspaceDashboardUsesViewportSafeArchitectureTooltipPosition(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`function architectureTooltipPosition`,
		`tooltipRect=tooltip.getBoundingClientRect()`,
		`width:window.innerWidth,height:window.innerHeight`,
		`tooltip.dataset.placement=position.placement`,
		`not runtime request frequency`,
		`pointer-events:none`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing viewport-safe tooltip contract %q", want)
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
		"function fitArchitectureNeighborhoodIfNeeded(nodeIDs)",
		"function restoreArchitectureServiceViewport()",
		`savedArchitectureDomainViewport:null`,
		`savedArchitectureServiceViewport:null`,
		"applyViewport(saved)",
		"if(savedDomain)applyViewport(savedDomain)",
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
		`id="architecture-focus-panel" class="architecture-focus-panel" aria-label="Architecture focus"`,
		`data-architecture-direction="outgoing" aria-pressed="false"`,
		`data-architecture-direction="incoming" aria-pressed="false"`,
		`data-architecture-direction="both" aria-pressed="true"`,
		`id="architecture-risk-toggle" aria-pressed="false"`,
		`id="reset-architecture-focus"`,
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

func TestDashboardMobileGridOrdersCanvasBeforeDetailsAndEnlargesControls(t *testing.T) {
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
		`grid-template-areas:"side" "main" "details"`,
		`.filters button,.modes button,.canvas-tools button{min-height:44px}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing responsive details/control contract %q", want)
		}
	}
}

func TestDashboardArchitectureCompactGeometryIsWiredToProduction(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`main{grid-area:main;container-type:inline-size`,
		`@container (max-width:1000px)`,
		`main[data-active-view="architecture"].graph-view .architecture-focus-panel{top:144px}`,
		`architectureCanvasGeometry(width,focusPanel.getBoundingClientRect().height)`,
		`svg.style.paddingTop=canvasGeometry.contentInset?canvasGeometry.contentInset+"px":""`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard compact Architecture geometry is not wired to production: missing %q", want)
		}
	}
}

func TestDashboardArchitectureRenderedGeometryAtRequiredViewports(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)

	geometries := renderedDashboardHeaderGeometries(t, html)
	if len(geometries) != 6 {
		t.Fatalf("Architecture header geometry count = %d, want 6 viewport and selection scenarios", len(geometries))
	}

	for _, geometry := range geometries {
		if geometry.ScrollWidth > float64(geometry.Viewport) {
			t.Fatalf("%dpx dashboard overflows horizontally: scroll width %.2f", geometry.Viewport, geometry.ScrollWidth)
		}
		for _, prefix := range []string{"domain-header-", "service-card-", "relationship-badge-"} {
			found := false
			for _, content := range geometry.Content {
				if strings.HasPrefix(content.Name, prefix) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%dpx dashboard did not render required Architecture content %q", geometry.Viewport, prefix)
			}
		}
		for _, element := range geometry.Headers {
			if element.Left < geometry.Main.Left || element.Right > geometry.Main.Right || element.Top < geometry.Main.Top || element.Bottom > geometry.Main.Bottom {
				t.Fatalf("%dpx %s is outside main pane: element=%#v main=%#v", geometry.Viewport, element.Name, element, geometry.Main)
			}
		}
		for _, content := range geometry.Content {
			if content.Top < geometry.Main.Top || content.Bottom > geometry.Main.Bottom {
				t.Fatalf("%dpx %s is vertically outside main pane: content=%#v main=%#v", geometry.Viewport, content.Name, content, geometry.Main)
			}
		}
		for left := 0; left < len(geometry.Headers); left++ {
			for right := left + 1; right < len(geometry.Headers); right++ {
				if geometry.Headers[left].intersects(geometry.Headers[right]) {
					t.Fatalf("%dpx Architecture header overlap: %s=%#v intersects %s=%#v", geometry.Viewport, geometry.Headers[left].Name, geometry.Headers[left], geometry.Headers[right].Name, geometry.Headers[right])
				}
			}
			for _, content := range geometry.Content {
				if geometry.Headers[left].intersects(content) {
					t.Fatalf("%dpx Architecture content overlap: %s=%#v intersects %s=%#v", geometry.Viewport, geometry.Headers[left].Name, geometry.Headers[left], content.Name, content)
				}
			}
		}
	}

	for _, wide := range geometries {
		if wide.Viewport == 1920 && wide.Scenario == "unselected" {
			if wide.Headers[0].Top != 12 || wide.Headers[1].Top != 12 || wide.Headers[2].Top != 12 || wide.Headers[3].Top != 96 {
				t.Fatalf("1920px Architecture header layout changed unexpectedly: %#v", wide)
			}
			return
		}
	}
	t.Fatal("1920px unselected Architecture geometry was not measured")
}

type dashboardHeaderRect struct {
	Name   string  `json:"name"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
}

func (rect dashboardHeaderRect) intersects(other dashboardHeaderRect) bool {
	return rect.Left < other.Right && rect.Right > other.Left && rect.Top < other.Bottom && rect.Bottom > other.Top
}

type dashboardHeaderGeometry struct {
	Viewport    int                   `json:"viewport"`
	Scenario    string                `json:"scenario"`
	Main        dashboardHeaderRect   `json:"main"`
	Headers     []dashboardHeaderRect `json:"headers"`
	Content     []dashboardHeaderRect `json:"content"`
	ScrollWidth float64               `json:"scrollWidth"`
}

func renderedDashboardHeaderGeometries(t *testing.T, html string) []dashboardHeaderGeometry {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for rendered dashboard geometry tests")
	}
	if output, err := exec.Command(node, "-e", `require.resolve("playwright")`).CombinedOutput(); err != nil {
		t.Skipf("Playwright is not installed for rendered dashboard geometry tests: %s", strings.TrimSpace(string(output)))
	}
	encodedHTML, err := json.Marshal(html)
	if err != nil {
		t.Fatalf("encode dashboard HTML for rendered geometry test: %v", err)
	}
	source := strings.Join([]string{
		`const {chromium}=require("playwright");`,
		`const html=` + string(encodedHTML) + `;`,
		`const headerSelectors=[["presentation",'[aria-label="Architecture presentation"]'],["legend",'[aria-label="Architecture map legend"]'],["tools",".canvas-tools"],["focus",'[aria-label="Architecture focus"]']];`,
		`const contentSelectors=[["domain-header","#architecture-lane-layer .domain-title"],["service-card","#architecture-node-layer .service-node"],["relationship-badge","#architecture-label-layer .bundle-count, #architecture-label-layer .architecture-call-pill"]];`,
		`(async()=>{const browser=await chromium.launch({headless:true}),geometries=[];try{for(const viewport of [{width:1280,height:720},{width:1440,height:900},{width:1920,height:1080}]){const page=await browser.newPage({viewport:viewport});await page.setContent(html,{waitUntil:"load"});await page.waitForFunction(()=>document.querySelectorAll("#architecture-node-layer .service-node").length>0);const measure=async scenario=>geometries.push(await page.evaluate(({headerSelectors,contentSelectors,scenario})=>{const rect=(name,element)=>{const value=element.getBoundingClientRect();return {name:name,left:value.left,right:value.right,top:value.top,bottom:value.bottom};},one=(name,selector)=>rect(name,document.querySelector(selector)),many=(name,selector)=>Array.from(document.querySelectorAll(selector)).map((element,index)=>rect(name+"-"+index,element));return {viewport:innerWidth,scenario:scenario,main:one("main","main"),headers:headerSelectors.map(item=>one(item[0],item[1])),content:contentSelectors.flatMap(item=>many(item[0],item[1])),scrollWidth:document.documentElement.scrollWidth};},{headerSelectors,contentSelectors,scenario}));await measure("unselected");await page.evaluate(()=>document.querySelector("#architecture-node-layer .service-node").dispatchEvent(new MouseEvent("click",{bubbles:true})));await page.waitForFunction(()=>!document.getElementById("architecture-relationship-summary").hidden);await measure("selected");await page.close();}}finally{await browser.close();}process.stdout.write(JSON.stringify(geometries));})().catch(error=>{console.error(error);process.exit(1);});`,
	}, "\n")
	output, err := exec.Command(node, "-e", source).CombinedOutput()
	if err != nil {
		t.Fatalf("rendered dashboard geometry failed: %v\n%s", err, output)
	}
	var result []dashboardHeaderGeometry
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode rendered dashboard geometry: %v\n%s", err, output)
	}
	return result
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
		"function architectureBundleGeometry",
		"bundle-count",
		"data-architecture-bundle",
		"data-architecture-edge-ids",
		"architecture-bundle-branch source",
		"architecture-bundle-branch target",
		`backgroundTargetLayer+='<path class="'+cls+' architecture-bundle-branch target"'+attributes+' marker-end="url(#arrow)"`,
		`backgroundTrunkLayer+='<path class="'+cls+' architecture-bundle-trunk"`,
		"Unrelated relationships remain grouped",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestWorkspaceDashboardUsesMockupArchitectureViews(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{`data-architecture-view="flow"`, `data-architecture-view="matrix"`, `data-architecture-view="selected"`, `main[data-active-view="architecture"].graph-view .canvas-tools{top:12px;left:350px}`, "architecture-lane-layer", "architecture-edge-layer", "architecture-node-layer", "architecture-label-layer", "architectureDomains", "architectureDomainColor", "layout.domains", "architecture-call-pill", "architecture-legend", "setViewBox(layout.width,layout.height)", "otherPosition.x-gutter/2", "otherPosition.y+otherPosition.h/2", "y:190+index*90", "architectureBundleGeometry", "svg{height:100vh}"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing mockup architecture contract %q", want)
		}
	}
	for _, unwanted := range []string{
		`const domains=["frontend","document","cadaster","identity","platform"]`,
		`frontend:"Frontend clients"`, `document:"Documents / WPO"`,
	} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("architecture remains workspace-specific: %q", unwanted)
		}
	}
}

func TestWorkspaceDashboardKeepsArchitectureExplanationVisible(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		`class="architecture-overlay-legend"`,
		`aria-label="Architecture map legend"`,
		"statically detected call relationship",
		"not runtime request frequency",
		"Grouped calls",
		"Direct calls",
		"Risk",
		"Selected",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing visible architecture explanation %q", want)
		}
	}
	for _, unwanted := range []string{"function architectureDirectionBadges", "function architectureNodeDirections", "architectureLegend(42,layout.height-58)"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("dashboard still renders obsolete architecture cue %q", unwanted)
		}
	}
}

func TestWorkspaceDashboardRendersArchitectureMatrixDetails(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{"function renderArchitectureMatrix", "function architectureProviderOrder", "architecture-matrix", "architecture-matrix-wrap{width:100%;min-width:0;overflow:auto}", `columns="190px repeat("`, `",96px)"`, "Consumer / provider", "data-architecture-edge", "architecture-matrix-detail", "View all relationships", "clamp(620px,calc(100vw - 1170px),920px)"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing architecture matrix contract %q", want)
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

func TestWorkspaceDashboardShowsTestLinksAndVerificationCommands(t *testing.T) {
	flow := BuildCanonicalFeatureFlow(WorkspaceFeatureFlowRecord{ID: "flow:test", HTTPMethod: "GET", Path: "/users", FrontendProject: "web", FrontendFile: "api.ts", BackendProject: "users", Tests: []TestMapRecord{{TestFile: "UserTest.java", TestMethod: "works", Confidence: "EXACT"}}, TestLinks: []TestLinkRecord{{ID: "test-link:1", Relation: "direct", TestFile: "UserTest.java", Confidence: "EXACT", Reason: "calls endpoint"}}, VerificationCommands: []VerificationCommandRecord{{Tool: "maven", WorkingDirectory: "users", Args: []string{"-Dtest=UserTest#works", "test"}, Display: "mvn -Dtest=UserTest#works test", Confidence: "EXACT", Reason: "detected Maven test"}}})
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, FeatureFlows: []WorkspaceFeatureFlowRecord{flow}}, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{"test_links", "verification_commands", "Linked tests", "Verification commands", "No linked test detected", "mvn -Dtest=UserTest#works test"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing test verification behavior %q", want)
		}
	}
}

func TestWorkspaceDashboardBoundsDenseFeatureFlowEvidence(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion}, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{"featureFlowPreviewLimit=12", "previewItems", "more not shown", "featureFlowOverflowCard"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing dense feature-flow preview marker %q", want)
		}
	}
}

func TestWorkspaceDashboardShowsEvidenceBackedImpact(t *testing.T) {
	serviceMap := WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, ImpactSummaries: []ImpactSummaryRecord{{
		ID: "impact:users", TargetID: "flow", TargetLabel: "GET /users", RiskLevel: "medium",
		RiskReasons:     []string{"Public endpoint with one direct consumer."},
		DirectConsumers: []ImpactItemRecord{{ID: "api", Relationship: "direct_consumer", Kind: "api_call", Project: "web", Confidence: "RESOLVED", Reason: "matched contract"}},
	}}}
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, serviceMap, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{"impact_summaries", "Changing this may affect", "Direct consumers", "Indirect consumers", "Dependent tests", "Coverage uncertainty"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing impact behavior %q", want)
		}
	}
}

func TestWorkspaceDashboardCoversSixViewAcceptance(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion}, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{"Architecture", "Endpoints", "Feature Flow", "Data Flow", "Diagnostics", "Coverage", "@media (max-width:", "prefers-reduced-motion", "Focus selected", "Most useful next scans", "Changing this may affect", `grid-template-areas:"side" "main" "details"`, ".impact-grid,.feature-verification,.next-scans article{grid-template-columns:1fr}"} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing visual acceptance marker %q", want)
		}
	}
}

func TestWorkspaceDashboardLocksIssue23ResponsiveAccessibilityStyles(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		nil,
		nil,
	)
	for _, want := range []string{
		`button:focus-visible,input:focus-visible,.source-link:focus-visible,[data-select-id]:focus-visible,[data-architecture-domain]:focus-visible,[data-architecture-edge]:focus-visible,[data-architecture-bundle]:focus-visible{outline:3px solid var(--color-focus);outline-offset:2px}`,
		`@media (max-width:1240px){`,
		`.architecture-focus-panel{position:absolute;top:96px;left:12px;right:12px}`,
		`.architecture-focus-panel button{min-height:44px}`,
		`.architecture-relationship-summary{flex-basis:100%;border-left:0;border-top:1px solid var(--color-border);padding:7px 0 0}`,
		`@media (prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition-duration:0.01ms!important;animation-duration:0.01ms!important}}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing issue #23 responsive/accessibility style %q", want)
		}
	}
}
