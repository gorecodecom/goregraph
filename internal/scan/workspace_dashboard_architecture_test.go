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
