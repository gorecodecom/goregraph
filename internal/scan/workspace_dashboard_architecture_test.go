package scan

import (
	"encoding/json"
	"os/exec"
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
	var result struct {
		Domains   []string       `json:"domains"`
		Positions map[string]any `json:"positions"`
	}
	runArchitectureModel(t, `(()=>{
		const nodes=[
			{id:"web",label:"Customer Web",project:"apps/web",domain:"experience"},
			{id:"orders",label:"Order Service",project:"services/orders",domain:"commerce"},
			{id:"audit",label:"Audit Service",project:"services/audit",domain:"observability"},
			{id:"worker",label:"Invoice Worker",project:"workers/invoice",domain:"operations"}
		];
		const layout=architectureLayout(nodes,1280);
		return {domains:layout.domains.map(d=>d.id),positions:Object.fromEntries(layout.positions)};
	})()`, &result)
	want := []string{"commerce", "experience", "observability", "operations"}
	if strings.Join(result.Domains, ",") != strings.Join(want, ",") {
		t.Fatalf("domains = %v, want %v", result.Domains, want)
	}
	if len(result.Positions) != 4 {
		t.Fatalf("positions = %d, want all 4 services", len(result.Positions))
	}
}
