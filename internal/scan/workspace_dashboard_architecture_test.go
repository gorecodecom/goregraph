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
