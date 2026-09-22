package scan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceExplorerProjectionPreservesEvidence(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for dashboard projection tests")
	}
	model := strings.Split(dashboardFile("adapter.js"), "const projectedDashboard=")[0]
	script := model + `
const assert=require('node:assert/strict');
const payload={graph:{root:'/example'},service_map:{nodes:[{id:'a',project:'a',label:'A',domain:'manual'},{id:'b',project:'b',label:'B',domain:'other'}],architecture_groups:[{id:'manual',label:'Custom',order:5}],edges:[{id:'java',from:'a',to:'b',total:1,resolved:1,endpoints:['java_client_import'],evidence:['src/main/A.java:42 imports com.example.B']},{id:'self',from:'b',to:'b',total:3}]},endpoint_traces:{traces:[]}};
const before=JSON.stringify(payload),result=dashboardProjection(payload);
assert.equal(JSON.stringify(payload),before);
assert.equal(result.architecture.nodes[0].domain,'manual');
assert.equal(result.architecture.groups[0].label,'Custom');
const java=result.architecture.edges[0].records[0];assert.equal(java.file,'src/main/A.java');assert.equal(java.line,42);assert.equal(java.scope,'production');
assert.equal(result.architecture.edges[1].records[0].count,3);
assert.equal(dashboardProjection({}).architecture.nodes.length,0);
assert.equal(dashboardSourceScope('src/test/A.java'),'test');assert.equal(dashboardSourceScope('src/main/A.java'),'production');assert.equal(dashboardSourceScope('unknown'),'unknown');
` + dashboardFile("evidence-model.js") + `
const projected=window.EvidenceModel.project(result.architecture.edges);assert.equal(projected.reduce((n,e)=>n+e.total,0),4);
const domains=window.EvidenceModel.domainPairs(projected,result.architecture.nodes);assert.equal(domains.length,2);assert.equal(domains.find(p=>p.from==='other').total,3);
`
	command := exec.Command(node)
	command.Stdin = strings.NewReader("global.window={};\n" + script)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("projection: %v\n%s", err, output)
	}
}

func explorerTestFixture(t *testing.T, root string) workspaceDashboardArtifacts {
	t.Helper()
	serviceMap := WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Root: root, Generated: "2026-09-21T00:00:00Z", Nodes: []WorkspaceServiceNodeRecord{
		{ID: "a", Label: "Orders", Project: "services/orders", Domain: "commerce"},
		{ID: "b", Label: "Storefront", Project: "web/store", Domain: "frontend"},
	}}
	symbols := WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion, Symbols: []CanonicalSymbolRecord{
		{ID: "order", Project: "services/orders", Name: "Order", QualifiedName: "example.Order", Kind: "class", DeclarationFile: "src/main/Order.java", DeclarationLine: 4},
		{ID: "product-card", Project: "web/store", Name: "ProductCard", QualifiedName: "ProductCard", Kind: "component", DeclarationFile: "src/ProductCard.tsx", DeclarationLine: 1},
		{ID: "consumer", Project: "web/store", Name: "OrderConsumer", QualifiedName: "example.OrderConsumer", Kind: "class", DeclarationFile: "src/main/Consumer.java", DeclarationLine: 8},
	}}
	usages := WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion, Usages: []CanonicalSymbolUsageRecord{
		{ID: "u1", ProviderSymbolID: "order", ConsumerSymbolID: "consumer", ConsumerProject: "web/store", Category: SymbolUsageDirectReference, RelationKind: "imports_type", SourceFile: "src/main/Consumer.java", SourceLine: 3, Resolution: "EXACT"},
		{ID: "u2", ProviderSymbolID: "order", ConsumerSymbolID: "consumer", ConsumerProject: "web/store", Category: SymbolUsageDirectReference, RelationKind: "field_type", SourceFile: "src/main/Consumer.java", SourceLine: 12, Resolution: "EXACT"},
	}}
	sources, notes := toolingFixtureSources(t)
	tooling := map[string]DashboardToolingRecord{"web/store": buildDashboardTooling(sources, notes), "services/orders": {Version: 1, Sources: []DashboardToolingSource{}}}
	return buildWorkspaceDashboardArtifacts(WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: root}, serviceMap, WorkspaceEndpointTraceIndexRecord{}, APICatalogRecord{}, symbols, usages, tooling)
}

func TestWorkspaceExplorerOfflineBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required")
	}
	if output, err := exec.Command(node, "-e", `require.resolve("playwright")`).CombinedOutput(); err != nil {
		t.Skipf("Playwright unavailable: %s", output)
	}
	root := t.TempDir()
	artifacts := explorerTestFixture(t, root)
	writeExplorerArtifacts(t, root, artifacts)
	empty := renderWorkspaceDashboardDocument("Empty workspace", []byte(`{}`))
	if err := os.WriteFile(filepath.Join(root, "empty.html"), []byte(empty), 0600); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("dashboard/workspace-explorer.browser.cjs")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(node, script, root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("offline browser: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func writeExplorerArtifacts(t *testing.T, root string, artifacts workspaceDashboardArtifacts) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "workspace-map.html"), []byte(artifacts.HTML), 0600); err != nil {
		t.Fatal(err)
	}
	for name, body := range artifacts.Assets {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Optional acceptance fixture: render a user's existing export without scanning or
// changing its project indexes. Only the explicitly supplied output is written.
func TestWorkspaceExplorerExistingExport(t *testing.T) {
	source, target := os.Getenv("GOREGRAPH_DASHBOARD_SOURCE"), os.Getenv("GOREGRAPH_DASHBOARD_OUTPUT")
	if source == "" || target == "" {
		t.Skip("no external acceptance export requested")
	}
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "const workspacePayload = "
	start := strings.Index(string(original), prefix)
	if start < 0 {
		t.Fatal("missing dashboard payload")
	}
	start += len(prefix)
	end := strings.Index(string(original[start:]), ";\n")
	if end < 0 {
		t.Fatal("missing payload end")
	}
	payload := original[start : start+end]
	var record workspaceDashboardPayload
	if err := json.Unmarshal(payload, &record); err != nil {
		t.Fatal(err)
	}
	artifacts := workspaceDashboardArtifacts{HTML: renderWorkspaceDashboardDocument("GoreGraph Workspace Explorer", payload), Assets: map[string][]byte{}}
	for _, asset := range record.CodeUsageAssets {
		if !strings.HasPrefix(asset, "workspace-map-assets/") || strings.Contains(asset, "..") || filepath.IsAbs(asset) {
			t.Fatal("unexpected asset path")
		}
		body, err := os.ReadFile(filepath.Join(filepath.Dir(source), filepath.FromSlash(asset)))
		if err != nil {
			t.Fatal(err)
		}
		artifacts.Assets[asset] = body
	}
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	writeExplorerArtifacts(t, target, artifacts)
	t.Logf("Rendered existing export: %d services, %d APIs, %d symbols, %d usage shards", len(record.ServiceMap.Nodes), len(record.APICatalog.Endpoints), len(record.SymbolIndex.Symbols), len(artifacts.Assets))
}
