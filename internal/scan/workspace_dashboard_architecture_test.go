package scan

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func denseArchitectureFixture() WorkspaceServiceMapRecord {
	nodes := []WorkspaceServiceNodeRecord{
		{ID: "service:web", Label: "Customer Web", Project: "apps/web", Domain: "experience", Incoming: 1, Outgoing: 7},
		{ID: "service:admin", Label: "Admin UI", Project: "apps/admin", Domain: "experience", Outgoing: 4},
		{ID: "service:orders", Label: "Order Service", Project: "services/orders", Domain: "commerce", Incoming: 6, Outgoing: 3},
		{ID: "service:billing", Label: "Billing Service", Project: "services/billing", Domain: "commerce", Incoming: 4, Outgoing: 1},
		{ID: "service:audit", Label: "Audit Service", Project: "services/audit", Domain: "observability", Incoming: 4},
		{ID: "service:worker", Label: "Invoice Worker", Project: "workers/invoice", Domain: "operations", Incoming: 1, Outgoing: 2},
	}
	edges := []WorkspaceServiceEdgeRecord{
		{ID: "edge:web-orders", From: "service:web", To: "service:orders", Total: 4, Resolved: 3, Unresolved: 1, Risk: "has_unresolved", Endpoints: []string{"GET /orders", "POST /orders"}},
		{ID: "edge:admin-orders", From: "service:admin", To: "service:orders", Total: 2, Resolved: 2},
		{ID: "edge:admin-billing", From: "service:admin", To: "service:billing", Total: 2, Resolved: 1, Mismatched: 1, Risk: "has_mismatches"},
		{ID: "edge:orders-billing", From: "service:orders", To: "service:billing", Total: 2, Resolved: 2},
		{ID: "edge:orders-audit", From: "service:orders", To: "service:audit", Total: 1, Resolved: 1},
		{ID: "edge:billing-audit", From: "service:billing", To: "service:audit", Total: 3, Resolved: 3},
		{ID: "edge:worker-orders", From: "service:worker", To: "service:orders", Total: 1, Resolved: 1},
	}
	return WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Nodes: nodes, Edges: edges}
}

func architectureGeometryStressFixture() WorkspaceServiceMapRecord {
	nodes := make([]WorkspaceServiceNodeRecord, 0, 6*13)
	for domainIndex := range 6 {
		domain := "d" + formatDashboardFixtureIndex(domainIndex)
		for nodeIndex := range 13 {
			id := "service:geometry-" + formatDashboardFixtureIndex(domainIndex) + "-" + formatDashboardFixtureIndex(nodeIndex)
			nodes = append(nodes, WorkspaceServiceNodeRecord{ID: id, Label: id, Project: id, Domain: domain})
		}
	}
	edges := []WorkspaceServiceEdgeRecord{{
		ID: "edge:geometry", From: nodes[0].ID, To: nodes[13].ID, Total: 1, Resolved: 1,
	}}
	return WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Nodes: nodes, Edges: edges}
}

func TestWorkspaceDashboardCoversArchitectureMapIssue23Acceptance(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(), WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{
		"architecture-lane-layer", "architecture-bundle-trunk", "architecture-bundle-branch",
		"architecture-relationship-summary", "architecture-relationship-tooltip",
		"architectureFocusModel", "architectureRelationshipSummary",
		"data-architecture-domain", "data-architecture-direction", "data-architecture-edge",
		"not runtime request frequency", "prefers-reduced-motion:reduce",
		"@media (max-width:1240px)", "aria-pressed", "tabindex=\"0\"",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("issue #23 acceptance missing %q", want)
		}
	}
	for _, obsolete := range []string{
		`const domains=["frontend","document","cadaster","identity","platform"]`,
		`nodes=focusedMode?allNodes.filter`,
	} {
		if strings.Contains(html, obsolete) {
			t.Fatalf("issue #23 obsolete behavior remains: %q", obsolete)
		}
	}
	if obsolete, found := issue23ObsoleteArchitectureMarker(html); !found {
		t.Fatal("issue #23 acceptance could not isolate the Architecture renderer")
	} else if obsolete != "" {
		t.Fatalf("issue #23 obsolete Architecture behavior remains: %q", obsolete)
	}
}

func issue23ObsoleteArchitectureMarker(html string) (string, bool) {
	start := strings.Index(html, "function renderArchitectureMap()")
	if start < 0 {
		return "", false
	}
	end := strings.Index(html[start:], "function architectureEdgeID(edge)")
	if end < 0 {
		return "", false
	}
	architecture := html[start : start+end]
	for _, obsolete := range []string{`>OUT<`, `>Caller<`, `>Called<`} {
		if strings.Contains(architecture, obsolete) {
			return obsolete, true
		}
	}
	return "", true
}

func TestIssue23ObsoleteRoleMarkersAreScopedToArchitectureRenderer(t *testing.T) {
	endpointCaller := `<span>Caller</span><script>function renderArchitectureMap(){return "current"}function architectureEdgeID(edge){}</script>`
	if obsolete, found := issue23ObsoleteArchitectureMarker(endpointCaller); !found || obsolete != "" {
		t.Fatalf("Endpoint Caller copy was treated as obsolete Architecture markup: marker=%q found=%v", obsolete, found)
	}
	architectureCaller := `<span>Caller</span><script>function renderArchitectureMap(){return "<text>Caller</text>"}function architectureEdgeID(edge){}</script>`
	if obsolete, found := issue23ObsoleteArchitectureMarker(architectureCaller); !found || obsolete != `>Caller<` {
		t.Fatalf("obsolete Architecture Caller marker = %q, found=%v", obsolete, found)
	}
}

func TestWorkspaceDashboardPreservesEndpointCallerCopy(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(), WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{`<span>Caller</span>`, `endpointInventoryCell("Caller",row.from,row.kind)`} {
		if !strings.Contains(html, want) {
			t.Fatalf("Endpoint inventory missing established Caller copy %q", want)
		}
	}
}

func TestWorkspaceDashboardEmbeddedJavaScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for embedded dashboard syntax validation")
	}
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(), WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	start, end := strings.Index(html, "<script>\n"), strings.LastIndex(html, "\n</script>")
	if start < 0 || end <= start {
		t.Fatal("embedded dashboard script boundaries not found")
	}
	cmd := exec.Command(node, "--check", "-")
	cmd.Stdin = strings.NewReader(html[start+len("<script>\n") : end])
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("embedded dashboard JavaScript is invalid: %v\n%s", err, output)
	}
}

func TestWorkspaceDashboardArchitectureMatrixKeepsEveryDiscoveredService(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for embedded dashboard completeness tests")
	}

	nodes := make([]WorkspaceServiceNodeRecord, 0, 261)
	edges := make([]WorkspaceServiceEdgeRecord, 0, 260)
	for index := range 260 {
		id := "service:consumer-" + formatDashboardFixtureIndex(index)
		nodes = append(nodes, WorkspaceServiceNodeRecord{ID: id, Label: id, Project: id, Domain: "test"})
		edges = append(edges, WorkspaceServiceEdgeRecord{
			ID:    "edge:" + id,
			From:  id,
			To:    "service:provider",
			Total: 1,
		})
	}
	nodes = append(nodes, WorkspaceServiceNodeRecord{ID: "service:provider", Label: "Provider", Project: "service:provider", Domain: "test"})

	encodedNodes, err := json.Marshal(nodes)
	if err != nil {
		t.Fatalf("encode service nodes: %v", err)
	}
	encodedEdges, err := json.Marshal(edges)
	if err != nil {
		t.Fatalf("encode service edges: %v", err)
	}
	function := func(start, end string) string {
		t.Helper()
		from := strings.Index(workspaceDashboardScript, start)
		if from < 0 {
			t.Fatalf("dashboard script missing function start %q", start)
		}
		to := strings.Index(workspaceDashboardScript[from:], end)
		if to < 0 {
			t.Fatalf("dashboard script missing function end %q", end)
		}
		return workspaceDashboardScript[from : from+to]
	}
	source := strings.Join([]string{
		`const serviceNodes=` + string(encodedNodes) + `;`,
		`const serviceEdges=` + string(encodedEdges) + `;`,
		`const serviceById=new Map(serviceNodes.map(function(node){return [node.id,node];}));`,
		`const state={filter:"all",mode:"architecture",query:"",selected:null,selectedArchitectureEdge:null};`,
		`function filteredServiceEdges(){return serviceEdges;}`,
		`function serviceRole(node){return node.role||"";}`,
		`function includesText(){return true;}`,
		`function architectureProviderOrder(a,b){return a.id<b.id?-1:a.id>b.id?1:0;}`,
		`function architectureEdgeID(edge){return edge.from+"|"+edge.to;}`,
		`function architectureDomains(){return [{id:"test",label:"Test"}];}`,
		`function architectureDomainLabel(){return "Test";}`,
		`function serviceDomain(){return "test";}`,
		`function routeStatusClass(){return "ok";}`,
		`function escapeAttr(value){return String(value);}`,
		`function escapeHtml(value){return String(value);}`,
		`function architectureMatrixDetail(){return "";}`,
		`function selectItem(){}`,
		`function setArchitectureServiceSelection(){}`,
		`function showServiceDetails(){}`,
		`function renderList(){}`,
		`function syncArchitectureViewControls(){}`,
		`const workbench={innerHTML:"",querySelectorAll:function(){return [];}};`,
		`const document={getElementById:function(){return workbench;}};`,
		function("function visibleServices()", "function visibleTraces()"),
		function("function renderArchitectureMatrix()", "function serviceFocus(id)"),
		`const visible=visibleServices();`,
		`renderArchitectureMatrix();`,
		`const cards=(workbench.innerHTML.match(/data-select-id=/g)||[]).length;`,
		`const providers=(workbench.innerHTML.match(/data-matrix-provider=/g)||[]).length;`,
		`process.stdout.write(JSON.stringify({visible:visible.length,cards:cards,providers:providers,hasLastConsumer:workbench.innerHTML.includes("service:consumer-259"),hasProvider:workbench.innerHTML.includes("service:provider")}));`,
	}, "\n")
	output, err := exec.Command(node, "-e", source).CombinedOutput()
	if err != nil {
		t.Fatalf("dashboard completeness model failed: %v\n%s", err, output)
	}
	var result struct {
		Visible         int  `json:"visible"`
		Cards           int  `json:"cards"`
		Providers       int  `json:"providers"`
		HasLastConsumer bool `json:"hasLastConsumer"`
		HasProvider     bool `json:"hasProvider"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode dashboard completeness result: %v\n%s", err, output)
	}
	if result.Visible != 261 || result.Cards != 260 || result.Providers != 1 || !result.HasLastConsumer || !result.HasProvider {
		t.Fatalf("dashboard omitted discovered services: %#v", result)
	}
}

func formatDashboardFixtureIndex(index int) string {
	const digits = "0123456789"
	return string([]byte{digits[index/100], digits[index/10%10], digits[index%10]})
}

func runArchitectureModel(t *testing.T, expression string, target any) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for embedded dashboard model tests")
	}
	source := workspaceDashboardArchitectureModelScript + "\nprocess.stdout.write(JSON.stringify(" + expression + "));"
	cmd := exec.Command(node, "-e", source)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("architecture model failed: %v\n%s", err, output)
	}
	if err := json.Unmarshal(output, target); err != nil {
		t.Fatalf("decode architecture model result: %v\n%s", err, output)
	}
}

func runRequiredArchitectureModel(t *testing.T, expression string, target any) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("node is required for architecture editor regression tests: %v", err)
	}
	source := workspaceDashboardArchitectureModelScript + "\nprocess.stdout.write(JSON.stringify(" + expression + "));"
	output, err := exec.Command(node, "-e", source).CombinedOutput()
	if err != nil {
		t.Fatalf("architecture model failed: %v\n%s", err, output)
	}
	if err := json.Unmarshal(output, target); err != nil {
		t.Fatalf("decode architecture model result: %v\n%s", err, output)
	}
}

func TestArchitectureRelationshipSummaryCountsRelationsServicesAndRiskBuckets(t *testing.T) {
	var result struct {
		IncomingRelationships int
		IncomingServices      int
		OutgoingRelationships int
		OutgoingServices      int
		Resolved              int
		Unresolved            int
		Mismatched            int
	}
	runArchitectureModel(t, `architectureRelationshipSummary("orders",[
		{id:"web-orders",from:"web",to:"orders",total:4,resolved:3,unresolved:1},
		{id:"worker-orders",from:"worker",to:"orders",total:1,resolved:1},
		{id:"orders-billing",from:"orders",to:"billing",total:2,resolved:1,mismatched:1},
		{id:"orders-audit",from:"orders",to:"audit",total:1,resolved:1}
	])`, &result)
	if result.IncomingRelationships != 5 || result.IncomingServices != 2 || result.OutgoingRelationships != 3 || result.OutgoingServices != 2 {
		t.Fatalf("counts = %#v", result)
	}
	if result.Resolved != 6 || result.Unresolved != 1 || result.Mismatched != 1 {
		t.Fatalf("buckets = %#v", result)
	}
}

func TestArchitectureTooltipPositionClampsAndFlipsWithinViewport(t *testing.T) {
	var result struct {
		Centered struct {
			Left      int
			Top       int
			Placement string
		}
		LeftEdge struct {
			Left int
		}
		RightEdge struct {
			Left int
		}
		BottomEdge struct {
			Left      int
			Top       int
			Placement string
		}
	}
	runArchitectureModel(t, `(()=>{
		const tooltip={width:240,height:60},viewport={width:800,height:600};
		return {
			Centered:architectureTooltipPosition({left:300,width:40,top:100,bottom:120},tooltip,viewport),
			LeftEdge:architectureTooltipPosition({left:-20,width:20,top:100,bottom:120},tooltip,viewport),
			RightEdge:architectureTooltipPosition({left:790,width:20,top:100,bottom:120},tooltip,viewport),
			BottomEdge:architectureTooltipPosition({left:300,width:40,top:550,bottom:580},tooltip,viewport)
		};
	})()`, &result)
	if result.Centered.Left != 320 || result.Centered.Top != 128 || result.Centered.Placement != "below" {
		t.Fatalf("centered tooltip = %#v", result.Centered)
	}
	if result.LeftEdge.Left != 128 || result.RightEdge.Left != 672 {
		t.Fatalf("horizontal clamps = left %#v, right %#v", result.LeftEdge, result.RightEdge)
	}
	if result.BottomEdge.Left != 320 || result.BottomEdge.Top != 482 || result.BottomEdge.Placement != "above" {
		t.Fatalf("bottom tooltip = %#v", result.BottomEdge)
	}
}

func TestArchitectureModelDerivesDynamicDomainsAndStablePositions(t *testing.T) {
	nodes, err := json.Marshal(denseArchitectureFixture().Nodes)
	if err != nil {
		t.Fatalf("encode dense architecture fixture: %v", err)
	}
	type position struct {
		X      float64 `json:"x"`
		Y      int     `json:"y"`
		Lane   int     `json:"lane"`
		Width  int     `json:"w"`
		Height int     `json:"h"`
		Domain string  `json:"domain"`
	}
	var result struct {
		Domains   []string            `json:"domains"`
		First     map[string]position `json:"first"`
		Repeated  map[string]position `json:"repeated"`
		Reordered map[string]position `json:"reordered"`
	}
	runArchitectureModel(t, `(()=>{
		const nodes=`+string(nodes)+`;
		const localeCompare=String.prototype.localeCompare;
		String.prototype.localeCompare=function(){throw new Error("architecture ordering must be locale independent");};
		try {
			const first=architectureLayout(nodes,1280);
			const repeated=architectureLayout(nodes,1280);
			const reordered=architectureLayout(nodes.slice().reverse(),1280);
			return {
				domains:first.domains.map(function(domain){return domain.id;}),
				first:Object.fromEntries(first.positions),
				repeated:Object.fromEntries(repeated.positions),
				reordered:Object.fromEntries(reordered.positions)
			};
		} finally {
			String.prototype.localeCompare=localeCompare;
		}
	})()`, &result)
	want := []string{"commerce", "experience", "observability", "operations"}
	if !reflect.DeepEqual(result.Domains, want) {
		t.Fatalf("domains = %v, want %v", result.Domains, want)
	}
	wantPositions := map[string]position{
		"service:billing": {X: 42, Y: 190, Lane: 0, Width: 224, Height: 74, Domain: "commerce"},
		"service:orders":  {X: 42, Y: 280, Lane: 0, Width: 224, Height: 74, Domain: "commerce"},
		"service:admin":   {X: 367.3333333333333, Y: 190, Lane: 1, Width: 224, Height: 74, Domain: "experience"},
		"service:web":     {X: 367.3333333333333, Y: 280, Lane: 1, Width: 224, Height: 74, Domain: "experience"},
		"service:audit":   {X: 692.6666666666666, Y: 190, Lane: 2, Width: 224, Height: 74, Domain: "observability"},
		"service:worker":  {X: 1018, Y: 190, Lane: 3, Width: 224, Height: 74, Domain: "operations"},
	}
	if !reflect.DeepEqual(result.First, wantPositions) {
		t.Fatalf("positions = %#v, want %#v", result.First, wantPositions)
	}
	if !reflect.DeepEqual(result.Repeated, result.First) {
		t.Fatalf("repeated positions changed: first=%#v repeated=%#v", result.First, result.Repeated)
	}
	if !reflect.DeepEqual(result.Reordered, result.First) {
		t.Fatalf("input order changed positions: first=%#v reordered=%#v", result.First, result.Reordered)
	}
	if strings.Contains(workspaceDashboardArchitectureModelScript, "localeCompare") {
		t.Fatal("architecture model ordering must not depend on localeCompare")
	}
}

func TestArchitectureLayoutUsesCanonicalConfiguredOrder(t *testing.T) {
	var result struct {
		Domains  []string
		Services map[string][]string
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[
			{id:"service:alpha-z",label:"Zulu",project:"services/zulu",domain:"alpha",architecture_order:20},
			{id:"service:alpha-a",label:"Alpha",project:"services/alpha",domain:"alpha",architecture_order:10},
			{id:"service:beta-b",label:"Beta",project:"services/beta",domain:"beta",architecture_order:0},
			{id:"service:beta-a",label:"Beta",project:"services/able",domain:"beta",architecture_order:0}
		];
		const groups=[{id:"alpha",label:"Alpha",order:1},{id:"beta",label:"Beta",order:0}];
		const domains=architectureDomains(nodes,groups);
		return {Domains:domains.map(group=>group.id),Services:Object.fromEntries(domains.map(group=>[group.id,group.nodes.map(node=>node.id)]))};
	})()`, &result)
	if !reflect.DeepEqual(result.Domains, []string{"beta", "alpha"}) {
		t.Fatalf("domains = %v, want configured group order", result.Domains)
	}
	if !reflect.DeepEqual(result.Services["alpha"], []string{"service:alpha-a", "service:alpha-z"}) {
		t.Fatalf("alpha services = %v, want architecture order", result.Services["alpha"])
	}
	if !reflect.DeepEqual(result.Services["beta"], []string{"service:beta-a", "service:beta-b"}) {
		t.Fatalf("beta services = %v, want stable label/project/id fallback", result.Services["beta"])
	}
}

func TestArchitectureDraftReorderHelpersArePureAndMoveBetweenGroups(t *testing.T) {
	var result struct {
		OriginalGroups []string
		MovedGroups    []string
		OriginalAlpha  []string
		MovedAlpha     []string
		MovedBeta      []string
	}
	runArchitectureModel(t, `(()=>{
		const draft={groups:[
			{id:"alpha",services:[{project:"services/a"},{project:"services/b"}]},
			{id:"beta",services:[{project:"services/c"}]}
		]};
		const movedGroups=architectureMoveItem(draft.groups,0,1);
		const movedServices=architectureMoveServiceDraft(draft,"services/a","alpha","beta",1);
		return {
			OriginalGroups:draft.groups.map(group=>group.id),
			MovedGroups:movedGroups.map(group=>group.id),
			OriginalAlpha:draft.groups[0].services.map(service=>service.project),
			MovedAlpha:movedServices.groups.find(group=>group.id==="alpha").services.map(service=>service.project),
			MovedBeta:movedServices.groups.find(group=>group.id==="beta").services.map(service=>service.project)
		};
	})()`, &result)
	if !reflect.DeepEqual(result.OriginalGroups, []string{"alpha", "beta"}) || !reflect.DeepEqual(result.MovedGroups, []string{"beta", "alpha"}) {
		t.Fatalf("group move mutated input or returned wrong order: %#v", result)
	}
	if !reflect.DeepEqual(result.OriginalAlpha, []string{"services/a", "services/b"}) ||
		!reflect.DeepEqual(result.MovedAlpha, []string{"services/b"}) ||
		!reflect.DeepEqual(result.MovedBeta, []string{"services/c", "services/a"}) {
		t.Fatalf("service move mutated input or returned wrong groups: %#v", result)
	}
}

func TestArchitectureEditorLifecyclePreservesDirtyAndGuardsBusy(t *testing.T) {
	var result struct {
		ConflictDirty      bool
		ValidationDirty    bool
		OfflineDirty       bool
		CloseNeedsConfirm  bool
		CanMutateBusy      bool
		RequestVersion     int
		DuplicateVersion   int
		StaleResponseBusy  bool
		ResetEditing       bool
		ResetDirty         bool
		ResetDraft         any
		ResetRequiresBuild bool
		RebaseRevision     string
		RebaseDraftGroup   string
	}
	runArchitectureModel(t, `(()=>{
		const dirty={architectureEditing:true,architectureDirty:true,architectureBusy:true,architectureRequestVersion:7,architectureDraft:{groups:[{id:"alpha"}]}};
		const conflict=architectureEditorLifecycle(dirty,{type:"failure",requestVersion:7,reason:"conflict"});
		const validation=architectureEditorLifecycle(dirty,{type:"failure",requestVersion:7,reason:"validation"});
		const offline=architectureEditorLifecycle(dirty,{type:"failure",requestVersion:7,reason:"offline"});
		const begun=architectureEditorLifecycle({architectureEditing:true,architectureDirty:true,architectureBusy:false,architectureRequestVersion:2,architectureDraft:{}},{type:"begin"});
		const duplicate=architectureEditorLifecycle(begun,{type:"begin"});
		const stale=architectureEditorLifecycle(begun,{type:"failure",requestVersion:2,reason:"offline"});
		const reset=architectureEditorLifecycle(dirty,{type:"reset",requestVersion:7});
		const rebased=architectureEditorRebase({revision:"revision-9",config:{schema:1,architecture:{groups:{latest:{label:"Latest"}}}}},dirty.architectureDraft);
		return {
			ConflictDirty:conflict.architectureDirty,
			ValidationDirty:validation.architectureDirty,
			OfflineDirty:offline.architectureDirty,
			CloseNeedsConfirm:architectureEditorNeedsDiscardConfirmation(conflict),
			CanMutateBusy:architectureEditorCanMutate(dirty),
			RequestVersion:begun.architectureRequestVersion,
			DuplicateVersion:duplicate.architectureRequestVersion,
			StaleResponseBusy:stale.architectureBusy,
			ResetEditing:reset.architectureEditing,
			ResetDirty:reset.architectureDirty,
			ResetDraft:reset.architectureDraft,
			ResetRequiresBuild:reset.architectureResetRequiresRebuild,
			RebaseRevision:rebased.architectureRevision,
			RebaseDraftGroup:rebased.architectureDraft.groups[0].id
		};
	})()`, &result)
	if !result.ConflictDirty || !result.ValidationDirty || !result.OfflineDirty || !result.CloseNeedsConfirm {
		t.Fatalf("failed editor requests lost dirty state: %#v", result)
	}
	if result.CanMutateBusy || result.RequestVersion != 3 || result.DuplicateVersion != 3 || !result.StaleResponseBusy {
		t.Fatalf("busy/request ordering guard = %#v", result)
	}
	if result.ResetEditing || result.ResetDirty || result.ResetDraft != nil || !result.ResetRequiresBuild {
		t.Fatalf("reset lifecycle can resurrect stale draft: %#v", result)
	}
	if result.RebaseRevision != "revision-9" || result.RebaseDraftGroup != "alpha" {
		t.Fatalf("conflict rebase did not retain and reapply draft: %#v", result)
	}
}

func TestArchitectureLoadedConfigControlsManualFlagsAndExactPayload(t *testing.T) {
	var result struct {
		EmptyGroupManual    bool
		EmptyServiceManual  bool
		LoadedGroupManual   bool
		LoadedServiceManual bool
		Payload             map[string]any
		ResetPayload        map[string]any
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[
			{id:"service:a",label:"A",project:"services/a",domain:"alpha",architecture_order:0,domain_manual:true},
			{id:"service:b",label:"B",project:"services/b",domain:"alpha",architecture_order:1,domain_manual:true}
		],groups=[{id:"alpha",label:"Embedded stale label",order:0,manual:true}],empty={schema:1,architecture:{}},loaded={schema:1,architecture:{groupOrder:["alpha"],groups:{alpha:{label:"Loaded label"}},services:{"services/a":{group:"alpha",order:0}}}};
		const fromEmpty=architectureDraftFromConfigData(nodes,groups,empty),fromLoaded=architectureDraftFromConfigData(nodes,groups,loaded),moved=architectureMoveServiceDraft(fromEmpty,"services/a","alpha","alpha",1);
		return {
			EmptyGroupManual:fromEmpty.groups[0].manual,
			EmptyServiceManual:fromEmpty.groups[0].services[0].manual,
			LoadedGroupManual:fromLoaded.groups[0].manual,
			LoadedServiceManual:fromLoaded.groups[0].services.find(service=>service.project==="services/a").manual,
			Payload:architectureDraftConfigValue(moved),
			ResetPayload:architectureEmptyConfig()
		};
	})()`, &result)
	if result.EmptyGroupManual || result.EmptyServiceManual || !result.LoadedGroupManual || !result.LoadedServiceManual {
		t.Fatalf("loaded config is not authoritative for manual flags: %#v", result)
	}
	payload, err := json.Marshal(result.Payload)
	if err != nil {
		t.Fatalf("encode reorder payload: %v", err)
	}
	want := `{"architecture":{"groupOrder":["alpha"],"groups":{"alpha":{"label":"Alpha"}},"services":{"services/a":{"group":"alpha","order":1},"services/b":{"group":"alpha","order":0}}},"schema":1}`
	if string(payload) != want {
		t.Fatalf("reorder payload = %s, want %s", payload, want)
	}
	reset, err := json.Marshal(result.ResetPayload)
	if err != nil {
		t.Fatalf("encode reset payload: %v", err)
	}
	if string(reset) != `{"architecture":{"groupOrder":[],"groups":{},"services":{}},"schema":1}` {
		t.Fatalf("reset payload = %s", reset)
	}
}

func TestArchitectureEditorThreeWayRebasePreservesUnrelatedRemoteChanges(t *testing.T) {
	var result struct {
		GroupOrder        []string
		AlphaLabel        string
		BetaLabel         string
		AlphaServices     []string
		BetaServices      []string
		SerializedPayload map[string]any
		ConflictService   struct {
			Group string
			Index int
		}
	}
	runArchitectureModel(t, `(()=>{
		const service=(project,manual)=>({id:"service:"+project,project:project,label:project,manual:!!manual});
		const base={groups:[
			{id:"alpha",label:"Alpha",manual:false,services:[service("a"),service("b")]},
			{id:"beta",label:"Beta",manual:false,services:[service("c"),service("d")]}
		]};
		const local=architectureCloneDraft(base);
		local.groups[0].label="Local Alpha";local.groups[0].manual=true;
		local.groups=architectureMoveItem(local.groups,0,1);
		const moved=architectureMoveServiceDraft(local,"b","alpha","beta",1);local.groups=moved.groups;
		const latest={groups:[
			{id:"gamma",label:"Remote Gamma",manual:true,services:[]},
			{id:"alpha",label:"Alpha",manual:false,services:[service("d",true),service("a")]},
			{id:"beta",label:"Remote Beta",manual:true,services:[service("c"),service("e",true)]}
		]};
		const merged=architectureDraftThreeWayMerge(base,local,latest);
		const alpha=merged.groups.find(group=>group.id==="alpha"),beta=merged.groups.find(group=>group.id==="beta");
		const conflictBase=architectureCloneDraft(base),conflictLocal=architectureMoveServiceDraft(conflictBase,"b","alpha","beta",0),conflictLatest=architectureCloneDraft(base);
		conflictLatest.groups[0].services=architectureMoveItem(conflictLatest.groups[0].services,1,0);
		const conflict=architectureDraftThreeWayMerge(base,conflictLocal,conflictLatest),conflictGroup=conflict.groups.find(group=>group.services.some(item=>item.project==="b"));
		return {
			GroupOrder:merged.groups.map(group=>group.id),
			AlphaLabel:alpha.label,
			BetaLabel:beta.label,
			AlphaServices:alpha.services.map(item=>item.project),
			BetaServices:beta.services.map(item=>item.project),
			SerializedPayload:architectureDraftConfigValue(merged),
			ConflictService:{Group:conflictGroup.id,Index:conflictGroup.services.findIndex(item=>item.project==="b")}
		};
	})()`, &result)
	if !reflect.DeepEqual(result.GroupOrder, []string{"gamma", "beta", "alpha"}) {
		t.Fatalf("three-way group order = %v, want remote gamma plus local beta/alpha intent", result.GroupOrder)
	}
	if result.AlphaLabel != "Local Alpha" || result.BetaLabel != "Remote Beta" {
		t.Fatalf("three-way labels = alpha %q beta %q", result.AlphaLabel, result.BetaLabel)
	}
	if !reflect.DeepEqual(result.AlphaServices, []string{"d", "a"}) || !reflect.DeepEqual(result.BetaServices, []string{"c", "b", "e"}) {
		t.Fatalf("three-way services = alpha %v beta %v", result.AlphaServices, result.BetaServices)
	}
	payload, err := json.Marshal(result.SerializedPayload)
	if err != nil {
		t.Fatalf("encode three-way payload: %v", err)
	}
	want := `{"architecture":{"groupOrder":["gamma","beta","alpha"],"groups":{"alpha":{"label":"Local Alpha"},"beta":{"label":"Remote Beta"},"gamma":{"label":"Remote Gamma"}},"services":{"a":{"group":"alpha","order":1},"b":{"group":"beta","order":1},"c":{"group":"beta","order":0},"d":{"group":"alpha","order":0},"e":{"group":"beta","order":2}}},"schema":1}`
	if string(payload) != want {
		t.Fatalf("three-way serialized payload = %s, want %s", payload, want)
	}
	if result.ConflictService.Group != "beta" || result.ConflictService.Index != 0 {
		t.Fatalf("same-service conflict = %#v, want local pending position to win", result.ConflictService)
	}
}

func TestArchitectureEditorThreeWayRebasePersistsCompleteTouchedGroupOrder(t *testing.T) {
	var result struct {
		SourceServices         []string
		TargetServices         []string
		SourceAllManual        bool
		TargetAllManual        bool
		UntouchedGroupManual   bool
		UntouchedServiceManual bool
		SerializedPayload      map[string]any
	}
	runRequiredArchitectureModel(t, `(()=>{
		const service=(project,label)=>({id:"service:"+project,project:project,label:label,manual:false});
		const base={groups:[
			{id:"alpha",label:"Alpha",manual:false,services:[service("source-z","Zulu Source"),service("moved","Middle Moved"),service("source-a","Alpha Source")]},
			{id:"beta",label:"Beta",manual:false,services:[service("target-z","Zulu Target"),service("target-a","Alpha Target")]},
			{id:"gamma",label:"Gamma",manual:false,services:[service("untouched","Untouched")]}
		]};
		const local=architectureMoveServiceDraft(architectureCloneDraft(base),"moved","alpha","beta",1);
		const latest=architectureCloneDraft(base),beta=latest.groups.find(group=>group.id==="beta");
		beta.services.splice(1,0,service("remote-auto","Aardvark Remote"));
		const merged=architectureDraftThreeWayMerge(base,local,latest),source=merged.groups.find(group=>group.id==="alpha"),target=merged.groups.find(group=>group.id==="beta"),untouched=merged.groups.find(group=>group.id==="gamma");
		return {
			SourceServices:source.services.map(item=>item.project),
			TargetServices:target.services.map(item=>item.project),
			SourceAllManual:source.services.every(item=>item.manual),
			TargetAllManual:target.services.every(item=>item.manual),
			UntouchedGroupManual:untouched.manual,
			UntouchedServiceManual:untouched.services[0].manual,
			SerializedPayload:architectureDraftConfigValue(merged)
		};
	})()`, &result)
	if !reflect.DeepEqual(result.SourceServices, []string{"source-z", "source-a"}) {
		t.Fatalf("merged source services = %v", result.SourceServices)
	}
	if !reflect.DeepEqual(result.TargetServices, []string{"target-z", "remote-auto", "moved", "target-a"}) {
		t.Fatalf("merged target services = %v", result.TargetServices)
	}
	if !result.SourceAllManual || !result.TargetAllManual {
		t.Fatalf("touched groups were not fully persisted: source=%t target=%t", result.SourceAllManual, result.TargetAllManual)
	}
	if result.UntouchedGroupManual || result.UntouchedServiceManual {
		t.Fatalf("untouched group manual flags changed: group=%t service=%t", result.UntouchedGroupManual, result.UntouchedServiceManual)
	}
	payload, err := json.Marshal(result.SerializedPayload)
	if err != nil {
		t.Fatalf("encode touched-group payload: %v", err)
	}
	want := `{"architecture":{"groupOrder":["alpha","beta","gamma"],"groups":{"alpha":{"label":"Alpha"},"beta":{"label":"Beta"}},"services":{"moved":{"group":"beta","order":2},"remote-auto":{"group":"beta","order":1},"source-a":{"group":"alpha","order":1},"source-z":{"group":"alpha","order":0},"target-a":{"group":"beta","order":3},"target-z":{"group":"beta","order":0}}},"schema":1}`
	if string(payload) != want {
		t.Fatalf("touched-group serialized payload = %s, want %s", payload, want)
	}
}

func TestArchitectureCanvasGeometryInsetsCompactContentBelowStackedControls(t *testing.T) {
	type geometry struct {
		Compact         bool
		PresentationTop int
		LegendTop       int
		ToolsTop        int
		FocusTop        int
		FocusBottom     int
		ContentInset    int
	}
	var result struct {
		Compact1280  geometry
		Compact1440  geometry
		Wide1920     geometry
		WideSelected geometry
		FirstCardY   int
		LaneTop      int
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"web",domain:"experience"},{id:"orders",domain:"commerce"}];
		const layout=architectureLayout(nodes,1220);
		return {
			Compact1280:architectureCanvasGeometry(580,84),
			Compact1440:architectureCanvasGeometry(740,84),
			Wide1920:architectureCanvasGeometry(1220,46),
			WideSelected:architectureCanvasGeometry(1220,70),
			FirstCardY:layout.positions.get("orders").y,
			LaneTop:layout.laneTop
		};
	})()`, &result)
	for name, geometry := range map[string]geometry{
		"1280px": result.Compact1280,
		"1440px": result.Compact1440,
	} {
		if !geometry.Compact || geometry.PresentationTop != 12 || geometry.LegendTop != 56 || geometry.ToolsTop != 100 || geometry.FocusTop != 144 || geometry.FocusBottom != 228 {
			t.Fatalf("%s compact header geometry = %#v", name, geometry)
		}
		if geometry.ContentInset < geometry.FocusBottom+24 {
			t.Fatalf("%s content inset %d does not clear focus bottom %d", name, geometry.ContentInset, geometry.FocusBottom)
		}
	}
	if result.Wide1920.Compact || result.Wide1920.PresentationTop != 12 || result.Wide1920.LegendTop != 12 || result.Wide1920.ToolsTop != 12 || result.Wide1920.FocusTop != 96 || result.Wide1920.ContentInset != 0 {
		t.Fatalf("1920px geometry changed unexpectedly: %#v", result.Wide1920)
	}
	if result.WideSelected.Compact || result.WideSelected.FocusBottom != 166 || result.WideSelected.ContentInset != 40 {
		t.Fatalf("1920px selected geometry does not clear the expanded focus panel: %#v", result.WideSelected)
	}
	if result.FirstCardY != 190 || result.LaneTop != 118 {
		t.Fatalf("wide Architecture coordinates changed: first card y=%d lane top=%d", result.FirstCardY, result.LaneTop)
	}
}

func TestArchitectureFocusModelKeepsDomainNeighborsAndAllServiceRelations(t *testing.T) {
	var result struct {
		DomainNodes  []string
		DomainEdges  []string
		ServiceNodes []string
		ServiceEdges []string
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"web",domain:"experience"},{id:"orders",domain:"commerce"},{id:"billing",domain:"commerce"},{id:"audit",domain:"observability"}];
		const edges=[{id:"web-orders",from:"web",to:"orders"},{id:"orders-billing",from:"orders",to:"billing"},{id:"orders-audit",from:"orders",to:"audit"},{id:"audit-billing",from:"audit",to:"billing"}];
		const domain=architectureFocusModel(nodes,edges,{domain:"commerce",direction:"outgoing",riskOnly:false});
		const service=architectureFocusModel(nodes,edges,{selected:"orders",direction:"both",riskOnly:false});
		return {DomainNodes:Array.from(domain.nodeIDs).sort(),DomainEdges:Array.from(domain.edgeIDs).sort(),ServiceNodes:Array.from(service.nodeIDs).sort(),ServiceEdges:Array.from(service.edgeIDs).sort()};
	})()`, &result)
	if strings.Join(result.DomainNodes, ",") != "audit,billing,orders" || strings.Join(result.DomainEdges, ",") != "orders-audit" {
		t.Fatalf("domain focus = %#v", result)
	}
	if strings.Join(result.ServiceEdges, ",") != "orders-audit,orders-billing,web-orders" || strings.Join(result.ServiceNodes, ",") != "audit,billing,orders,web" {
		t.Fatalf("service focus = %#v", result)
	}
}

func TestArchitectureRiskFocusChangesEmphasisNotLayout(t *testing.T) {
	var result struct {
		PositionCount int
		FocusedEdges  []string
	}
	runArchitectureModel(t, `(()=>{const nodes=[{id:"web",domain:"experience"},{id:"orders",domain:"commerce"},{id:"audit",domain:"observability"}],edges=[{id:"web-orders",from:"web",to:"orders",resolved:4},{id:"orders-audit",from:"orders",to:"audit",unresolved:1,risk:"has_unresolved"}],layout=architectureLayout(nodes,1440),focus=architectureFocusModel(nodes,edges,{selected:"orders",direction:"both",riskOnly:true});return {PositionCount:layout.positions.size,FocusedEdges:Array.from(focus.edgeIDs).sort()};})()`, &result)
	if result.PositionCount != 3 || strings.Join(result.FocusedEdges, ",") != "orders-audit" {
		t.Fatalf("risk focus = %#v", result)
	}
}

func TestArchitectureDirectNeighborhoodIgnoresDirectionAndRiskEmphasis(t *testing.T) {
	var result struct {
		Focused      []string
		Neighborhood []string
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"web",domain:"experience"},{id:"orders",domain:"commerce"},{id:"billing",domain:"commerce"},{id:"audit",domain:"observability"}];
		const edges=[{id:"web-orders",from:"web",to:"orders",resolved:1},{id:"orders-billing",from:"orders",to:"billing",resolved:1},{id:"orders-audit",from:"orders",to:"audit",unresolved:1,risk:"has_unresolved"}];
		const focus=architectureFocusModel(nodes,edges,{selected:"orders",direction:"outgoing",riskOnly:true});
		const neighborhood=architectureDirectNeighborhood(edges,"orders");
		return {Focused:Array.from(focus.nodeIDs).sort(),Neighborhood:Array.from(neighborhood).sort()};
	})()`, &result)
	if strings.Join(result.Focused, ",") != "audit,orders" {
		t.Fatalf("focused nodes = %v", result.Focused)
	}
	if strings.Join(result.Neighborhood, ",") != "audit,billing,orders,web" {
		t.Fatalf("direct neighborhood = %v", result.Neighborhood)
	}
}

func TestArchitectureBundlesAreDeterministicAndRetainParallelRelationships(t *testing.T) {
	var result []struct {
		ID      string
		Total   int
		EdgeIDs []string
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"web",domain:"experience"},{id:"admin",domain:"experience"},{id:"orders",domain:"commerce"},{id:"billing",domain:"commerce"}],byID=new Map(nodes.map(n=>[n.id,n]));
		const edges=[{id:"b",from:"admin",to:"billing",total:2,resolved:2},{id:"a",from:"web",to:"orders",total:4,resolved:3,unresolved:1,risk:"has_unresolved"},{id:"c",from:"admin",to:"orders",total:1,resolved:1}];
		return architectureBundles(edges,byID).map(bundle=>({ID:bundle.id,Total:bundle.total,EdgeIDs:bundle.edges.map(edge=>edge.id)}));
	})()`, &result)
	if len(result) != 2 {
		t.Fatalf("bundles = %#v, want resolved and unresolved groups", result)
	}
	if result[0].ID > result[1].ID {
		t.Fatalf("bundles not sorted: %#v", result)
	}
	if result[0].ID != "bundle:experience|commerce|resolved" || result[0].Total != 3 || strings.Join(result[0].EdgeIDs, ",") != "b,c" {
		t.Fatalf("resolved bundle = %#v, want sorted parallel edges b,c with total 3", result[0])
	}
	if result[1].ID != "bundle:experience|commerce|unresolved" || result[1].Total != 4 || strings.Join(result[1].EdgeIDs, ",") != "a" {
		t.Fatalf("unresolved bundle = %#v, want edge a with total 4", result[1])
	}
}

func TestArchitectureBundlesFanEveryRelationshipThroughSharedGeometry(t *testing.T) {
	var result struct {
		TrunkPath  string
		BranchIDs  []string
		SourcePath []string
		TargetPath []string
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"web",domain:"experience"},{id:"admin",domain:"experience"},{id:"orders",domain:"commerce"},{id:"billing",domain:"commerce"}],layout=architectureLayout(nodes,1040),byID=new Map(nodes.map(node=>[node.id,node]));
		const edges=[{id:"web-orders",from:"web",to:"orders",total:4},{id:"admin-billing",from:"admin",to:"billing",total:2}];
		const geometry=architectureBundleGeometry(architectureBundles(edges,byID)[0],layout,0);
		return {TrunkPath:geometry.trunkPath,BranchIDs:geometry.branches.map(branch=>branch.edge.id),SourcePath:geometry.branches.map(branch=>branch.sourcePath),TargetPath:geometry.branches.map(branch=>branch.targetPath)};
	})()`, &result)
	if result.TrunkPath == "" {
		t.Fatal("bundle geometry is missing a shared trunk")
	}
	if !reflect.DeepEqual(result.BranchIDs, []string{"admin-billing", "web-orders"}) {
		t.Fatalf("branch IDs = %v, want every sorted parallel relationship", result.BranchIDs)
	}
	if len(result.SourcePath) != 2 || len(result.TargetPath) != 2 || result.SourcePath[0] == result.SourcePath[1] || result.TargetPath[0] == result.TargetPath[1] {
		t.Fatalf("bundle branches do not fan to distinct card endpoints: %#v", result)
	}
}

func TestArchitectureSameDomainBundleGeometryChoosesAnAvailableRailSide(t *testing.T) {
	type geometry struct {
		Width          float64
		CardLeft       float64
		CardRight      float64
		TrunkX         float64
		BadgeLeft      float64
		BadgeRight     float64
		SourceStartXs  []float64
		RepeatedTrunkX float64
	}
	var result struct {
		Leftmost  geometry
		Middle    geometry
		Rightmost geometry
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"alpha-a",domain:"alpha"},{id:"alpha-b",domain:"alpha"},{id:"omega-a",domain:"omega"},{id:"omega-b",domain:"omega"}],layout=architectureLayout(nodes,1040),byID=new Map(nodes.map(node=>[node.id,node])),middleNodes=nodes.concat([{id:"middle-a",domain:"middle"},{id:"middle-b",domain:"middle"}]),middleLayout=architectureLayout(middleNodes,1040),middleByID=new Map(middleNodes.map(node=>[node.id,node]));
		function inspect(layout,byID,edges){
			const bundle=architectureBundles(edges,byID)[0],geometry=architectureBundleGeometry(bundle,layout,0),repeated=architectureBundleGeometry(bundle,layout,0),card=layout.positions.get(edges[0].from),label=bundle.total+" call"+(bundle.total===1?"":"s"),badgeHalfWidth=Math.max(58,label.length*7+18)/2;
			return {Width:layout.width,CardLeft:card.x,CardRight:card.x+card.w,TrunkX:Number(geometry.trunkPath.slice(1).split(" ")[0]),BadgeLeft:geometry.badge.x-badgeHalfWidth,BadgeRight:geometry.badge.x+badgeHalfWidth,SourceStartXs:geometry.branches.map(branch=>Number(branch.sourcePath.slice(1).split(" ")[0])),RepeatedTrunkX:Number(repeated.trunkPath.slice(1).split(" ")[0])};
		}
		return {Leftmost:inspect(layout,byID,[{id:"alpha-edge",from:"alpha-a",to:"alpha-b",total:2}]),Middle:inspect(middleLayout,middleByID,[{id:"middle-edge",from:"middle-a",to:"middle-b",total:2}]),Rightmost:inspect(layout,byID,[{id:"omega-edge-a",from:"omega-a",to:"omega-b",total:2},{id:"omega-edge-b",from:"omega-b",to:"omega-a",total:1}])};
	})()`, &result)
	const trunkHalfStroke = 1.2
	for name, geometry := range map[string]geometry{"leftmost": result.Leftmost, "middle": result.Middle, "rightmost": result.Rightmost} {
		if geometry.TrunkX-trunkHalfStroke < 0 || geometry.TrunkX+trunkHalfStroke > geometry.Width {
			t.Fatalf("%s trunk x = %.2f exceeds layout width %.2f with stroke", name, geometry.TrunkX, geometry.Width)
		}
		if geometry.BadgeLeft < 0 || geometry.BadgeRight > geometry.Width {
			t.Fatalf("%s badge bounds = [%.2f, %.2f], want within [0, %.2f]", name, geometry.BadgeLeft, geometry.BadgeRight, geometry.Width)
		}
		if geometry.TrunkX != geometry.RepeatedTrunkX {
			t.Fatalf("%s trunk changed across repeated geometry: %.2f != %.2f", name, geometry.TrunkX, geometry.RepeatedTrunkX)
		}
	}
	if result.Leftmost.TrunkX <= result.Leftmost.CardRight {
		t.Fatalf("leftmost rail x = %.2f, want right of card edge %.2f", result.Leftmost.TrunkX, result.Leftmost.CardRight)
	}
	if result.Middle.TrunkX <= result.Middle.CardRight {
		t.Fatalf("middle rail x = %.2f, want right of card edge %.2f", result.Middle.TrunkX, result.Middle.CardRight)
	}
	if result.Rightmost.TrunkX >= result.Rightmost.CardLeft {
		t.Fatalf("rightmost rail x = %.2f, want left of card edge %.2f", result.Rightmost.TrunkX, result.Rightmost.CardLeft)
	}
	for _, sourceX := range result.Rightmost.SourceStartXs {
		if sourceX != result.Rightmost.CardLeft {
			t.Fatalf("rightmost source branch starts at %.2f, want left card edge %.2f", sourceX, result.Rightmost.CardLeft)
		}
	}
}

func TestArchitectureDeferredAutoFitRestoresOriginalMatrixViewport(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for embedded dashboard state-sequence tests")
	}
	function := func(start, end string) string {
		t.Helper()
		from := strings.Index(workspaceDashboardScript, start)
		if from < 0 {
			t.Fatalf("dashboard script missing function start %q", start)
		}
		to := strings.Index(workspaceDashboardScript[from:], end)
		if to < 0 {
			t.Fatalf("dashboard script missing function end %q", end)
		}
		return workspaceDashboardScript[from : from+to]
	}
	source := strings.Join([]string{
		`const original={zoom:2.25,panX:137,panY:-59};`,
		`const controls=new Map();`,
		`const graph={viewBox:{baseVal:{x:0,y:0,width:1000,height:700}}};`,
		`const document={getElementById:function(id){if(id==="workspace-graph")return graph;if(!controls.has(id))controls.set(id,{hidden:false});return controls.get(id);}};`,
		`let state;let fitCount=0;`,
		`function applyTransform(){}`,
		`function renderList(){}`,
		`function renderCanvas(){if(state.selected&&state.pendingArchitectureServiceFit===state.selected){state.pendingArchitectureServiceFit=null;fitCount++;fitArchitectureNeighborhoodIfNeeded(new Set([state.selected]));}}`,
		function("function applyViewport(viewport)", "function fitArchitectureNeighborhoodIfNeeded(nodeIDs)"),
		function("function fitArchitectureNeighborhoodIfNeeded(nodeIDs)", "function visibleContentBounds(layer)"),
		function("function restoreArchitectureServiceViewport()", "function clearArchitectureServiceState()"),
		function("function clearArchitectureServiceState()", "function resetArchitectureFocus()"),
		function("function setArchitectureServiceSelection(id)", "function restoreArchitectureDomainFocus(domain,elementName)"),
		function("function enterArchitectureFocus()", "function leaveArchitectureFocus()"),
		function("function leaveArchitectureFocus()", "function hideArchitectureSelectionActions()"),
		function("function hideArchitectureSelectionActions()", "function clearSelectedItemState()"),
		`function freshState(){return {mode:"architecture",architectureView:"matrix",architectureDirection:"both",savedArchitectureServiceViewport:null,pendingArchitectureServiceFit:null,selectedArchitectureEdge:null,selected:null,selections:{architecture:null},focusStep:null,isolation:false,architectureFocused:false,savedFullArchitectureViewport:null,zoom:original.zoom,panX:original.panX,panY:original.panY,positions:new Map([["service:orders",{x:1600,y:900,w:224,h:74}]])};}`,
		`function viewport(){return {zoom:state.zoom,panX:state.panX,panY:state.panY};}`,
		`function enterFromMatrix(){state=freshState();const before=fitCount;setArchitectureServiceSelection("service:orders");enterArchitectureFocus();return {fitCount:fitCount-before,afterFit:viewport(),savedService:state.savedArchitectureServiceViewport,pending:state.pendingArchitectureServiceFit};}`,
		`const cleared=enterFromMatrix();clearArchitectureServiceState();cleared.afterExit=viewport();cleared.savedServiceAfterExit=state.savedArchitectureServiceViewport;`,
		`const left=enterFromMatrix();leaveArchitectureFocus();left.afterExit=viewport();left.savedServiceAfterExit=state.savedArchitectureServiceViewport;clearArchitectureServiceState();left.afterClear=viewport();`,
		`state=freshState();state.architectureView="flow";const beforeFlowFit=fitCount;setArchitectureServiceSelection("service:orders");renderCanvas();const flow={fitCount:fitCount-beforeFlowFit,afterFit:viewport(),savedService:state.savedArchitectureServiceViewport,pending:state.pendingArchitectureServiceFit};clearArchitectureServiceState();flow.afterExit=viewport();`,
		`process.stdout.write(JSON.stringify({original:original,cleared:cleared,left:left,flow:flow}));`,
	}, "\n")
	output, err := exec.Command(node, "-e", source).CombinedOutput()
	if err != nil {
		t.Fatalf("dashboard state sequence failed: %v\n%s", err, output)
	}
	type viewport struct {
		Zoom float64 `json:"zoom"`
		PanX float64 `json:"panX"`
		PanY float64 `json:"panY"`
	}
	type sequence struct {
		FitCount              int       `json:"fitCount"`
		AfterFit              viewport  `json:"afterFit"`
		AfterExit             viewport  `json:"afterExit"`
		AfterClear            viewport  `json:"afterClear"`
		SavedService          *viewport `json:"savedService"`
		SavedServiceAfterExit *viewport `json:"savedServiceAfterExit"`
		Pending               *string   `json:"pending"`
	}
	var result struct {
		Original viewport `json:"original"`
		Cleared  sequence `json:"cleared"`
		Left     sequence `json:"left"`
		Flow     sequence `json:"flow"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode dashboard state sequence: %v\n%s", err, output)
	}
	for name, sequence := range map[string]sequence{"clear": result.Cleared, "leave": result.Left} {
		if sequence.FitCount != 1 {
			t.Fatalf("%s sequence fit count = %d, want one deferred fit", name, sequence.FitCount)
		}
		if reflect.DeepEqual(sequence.AfterFit, result.Original) {
			t.Fatalf("%s sequence did not perform an actual viewport fit", name)
		}
		if sequence.SavedService != nil {
			t.Fatalf("%s sequence captured reset Selected viewport as service viewport: %#v", name, sequence.SavedService)
		}
		if sequence.Pending != nil {
			t.Fatalf("%s sequence did not consume pending auto-fit: %q", name, *sequence.Pending)
		}
		if !reflect.DeepEqual(sequence.AfterExit, result.Original) {
			t.Fatalf("%s viewport after exit = %#v, want original %#v", name, sequence.AfterExit, result.Original)
		}
	}
	if result.Left.SavedServiceAfterExit != nil {
		t.Fatalf("leave sequence retained stale service viewport: %#v", result.Left.SavedServiceAfterExit)
	}
	if !reflect.DeepEqual(result.Left.AfterClear, result.Original) {
		t.Fatalf("viewport after leave then clear = %#v, want original %#v", result.Left.AfterClear, result.Original)
	}
	if result.Flow.FitCount != 1 || reflect.DeepEqual(result.Flow.AfterFit, result.Original) {
		t.Fatalf("flow selection did not perform one actual fit: %#v", result.Flow)
	}
	if result.Flow.SavedService == nil || !reflect.DeepEqual(*result.Flow.SavedService, result.Original) {
		t.Fatalf("flow selection service viewport = %#v, want original %#v", result.Flow.SavedService, result.Original)
	}
	if !reflect.DeepEqual(result.Flow.AfterExit, result.Original) {
		t.Fatalf("flow viewport after clear = %#v, want original %#v", result.Flow.AfterExit, result.Original)
	}
}
