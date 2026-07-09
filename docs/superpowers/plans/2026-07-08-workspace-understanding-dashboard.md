# Workspace Understanding Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a deterministic local GoreGraph workspace understanding layer inspired by Graphify and Understand-Anything: clickable workspace map, explain/path/impact commands, stable IDs, and feature-centric outputs that work outside the current WEKA layout.

**Architecture:** Keep the scanner deterministic and local-first. Reuse existing workspace outputs (`registry.json`, `contract-matches.json`, `feature-flows.json`, `feature-dossiers.json`) to build a normalized graph model, then generate static HTML and CLI views from that graph without adding external runtime dependencies.

**Tech Stack:** Go stdlib CLI, JSON/Markdown/HTML outputs, existing GoreGraph scan package, `go test ./...`, no network dependency, no new third-party package unless a later task proves it is necessary.

## Global Constraints

- Version target is `0.9.0`.
- Do not push unless explicitly requested.
- No WEKA hardcoding: all grouping, labels, and graph edges must derive from detected project metadata and existing GoreGraph records.
- Local-first and deterministic: no LLM/API call is required for scan, dashboard, explain, path, or impact.
- Add dependencies only when genuinely necessary; this plan expects none.
- Generated HTML must be static, offline, and safe to open from disk.
- TDD: every behavior task starts with a failing test.
- Keep existing output files backward-compatible; add new fields/files instead of breaking existing JSON shapes.

---

## File Structure

- Create: `internal/scan/workspace_graph.go`
  - Builds normalized workspace graph nodes and edges from existing workspace records.
  - Owns stable ID generation and graph stats.
- Create: `internal/scan/workspace_graph_test.go`
  - Unit tests for graph construction, edge types, stable IDs, and generic project layout behavior.
- Create: `internal/scan/workspace_dashboard.go`
  - Renders a standalone `workspace-map.html` with embedded graph JSON, search, filters, details panel, and feature routes.
- Create: `internal/scan/workspace_dashboard_test.go`
  - Tests dashboard generation without browser dependency.
- Create: `internal/scan/workspace_explain.go`
  - Reads generated workspace outputs and explains a route, file, service/project, contract ID, or feature dossier ID.
- Create: `internal/scan/workspace_explain_test.go`
  - Tests explain output and matching behavior.
- Create: `internal/scan/workspace_path.go`
  - Computes shortest paths over `workspace-graph.json`.
- Create: `internal/scan/workspace_path_test.go`
  - Tests route-to-controller and frontend-to-backend graph paths.
- Create: `internal/scan/workspace_impact.go`
  - Maps changed files/routes/contracts to affected features, consumers, tests, auth, DTO, persistence, and risk records.
- Create: `internal/scan/workspace_impact_test.go`
  - Tests impact summaries from explicit changed-file input.
- Modify: `internal/scan/workspace_reconcile.go`
  - Writes `workspace-graph.json`, `workspace-map.html`, `workspace-impact.md`, and project-local graph/dashboard pointers.
- Modify: `internal/scan/types.go`
  - Adds `WorkspaceGraphRecord`, `WorkspaceGraphNodeRecord`, `WorkspaceGraphEdgeRecord`, `WorkspaceExplainRecord`, and `WorkspaceImpactRecord`.
- Modify: `internal/cli/cli.go`
  - Adds `goregraph workspace dashboard`, `goregraph workspace explain`, `goregraph workspace path`, and `goregraph workspace impact`.
- Modify: `internal/cli/cli_test.go`
  - CLI coverage for new commands and help text.
- Modify: `docs/OUTPUTS.md`
  - Documents new output files, stable IDs, graph node/edge semantics, confidence levels, and HTML safety constraints.
- Modify: `docs/RELEASE.md`
  - Adds the local understanding dashboard capability to the 0.9.0 release notes.

---

### Task 1: Workspace Graph Model

**Files:**
- Modify: `internal/scan/types.go`
- Create: `internal/scan/workspace_graph.go`
- Create: `internal/scan/workspace_graph_test.go`
- Modify: `internal/scan/workspace_reconcile.go`

**Interfaces:**
- Consumes: `WorkspaceRegistryRecord`, `WorkspaceContractMatchRecord`, `WorkspaceFeatureFlowRecord`, `FeatureDossierRecord`.
- Produces:
  - `func BuildWorkspaceGraph(registry WorkspaceRegistryRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, dossiers []FeatureDossierRecord) WorkspaceGraphRecord`
  - `func StableWorkspaceID(kind string, parts ...string) string`
  - `workspace-graph.json`

- [ ] **Step 1: Write failing stable-ID and graph tests**

Add this test to `internal/scan/workspace_graph_test.go`:

```go
package scan

import "testing"

func TestBuildWorkspaceGraphCreatesStableWorkspaceNodesAndEdges(t *testing.T) {
	registry := WorkspaceRegistryRecord{
		Root: "/workspace",
		Projects: []WorkspaceProjectRecord{
			{Path: "frontend/app", Name: "app", Kind: "frontend"},
			{Path: "services/ms-user", Name: "ms-user", Kind: "backend"},
		},
	}
	matches := []WorkspaceContractMatchRecord{{
		ContractID:      "contract:get-users",
		FrontendProject: "frontend/app",
		BackendProject:  "services/ms-user",
		Method:          "GET",
		Path:            "/users/{userId}",
		Confidence:      "RESOLVED",
		Issue:           "matched",
		FrontendFile:    "src/api/users.ts",
		BackendFile:     "src/main/java/UserController.java",
		BackendSymbol:   "UserController.get",
	}}
	flows := []WorkspaceFeatureFlowRecord{{
		ID:              "flow:get-users",
		ContractID:      "contract:get-users",
		FrontendProject: "frontend/app",
		BackendProject:  "services/ms-user",
		HTTPMethod:      "GET",
		HTTPPath:        "/users/{userId}",
		Steps: []WorkspaceFeatureFlowStepRecord{
			{Kind: "frontend_file", Project: "frontend/app", File: "src/api/users.ts"},
			{Kind: "backend_controller", Project: "services/ms-user", File: "src/main/java/UserController.java", Symbol: "UserController.get"},
		},
	}}
	dossiers := []FeatureDossierRecord{{
		ID:         "feature:get-users",
		Title:      "GET /users/{userId}",
		ContractID: "contract:get-users",
	}}

	graph := BuildWorkspaceGraph(registry, matches, flows, dossiers)

	requireGraphNode(t, graph, "project:frontend/app", "project")
	requireGraphNode(t, graph, "project:services/ms-user", "project")
	requireGraphNode(t, graph, "contract:get-users", "contract")
	requireGraphNode(t, graph, "route:services/ms-user:get:/users/{userId}", "route")
	requireGraphNode(t, graph, "feature:get-users", "feature")
	requireGraphEdge(t, graph, "project:frontend/app", "contract:get-users", "declares_contract")
	requireGraphEdge(t, graph, "contract:get-users", "route:services/ms-user:get:/users/{userId}", "resolved_by")
	requireGraphEdge(t, graph, "feature:get-users", "contract:get-users", "contains")
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
	t.Fatalf("missing node %s", id)
}

func requireGraphEdge(t *testing.T, graph WorkspaceGraphRecord, from, to, kind string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return
		}
	}
	t.Fatalf("missing edge %s -[%s]-> %s", from, kind, to)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestBuildWorkspaceGraphCreatesStableWorkspaceNodesAndEdges -count=1
```

Expected: FAIL because `WorkspaceGraphRecord` and `BuildWorkspaceGraph` do not exist.

- [ ] **Step 3: Add graph types**

Add to `internal/scan/types.go`:

```go
type WorkspaceGraphRecord struct {
	SchemaVersion int                        `json:"schema_version"`
	Generated     string                     `json:"generated"`
	Root          string                     `json:"root"`
	Nodes         []WorkspaceGraphNodeRecord `json:"nodes"`
	Edges         []WorkspaceGraphEdgeRecord `json:"edges"`
	Stats         map[string]int             `json:"stats,omitempty"`
}

type WorkspaceGraphNodeRecord struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Label      string            `json:"label"`
	Project    string            `json:"project,omitempty"`
	File       string            `json:"file,omitempty"`
	Symbol     string            `json:"symbol,omitempty"`
	Method     string            `json:"method,omitempty"`
	Path       string            `json:"path,omitempty"`
	Confidence string            `json:"confidence,omitempty"`
	Risk       string            `json:"risk,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

type WorkspaceGraphEdgeRecord struct {
	ID         string            `json:"id"`
	From       string            `json:"from"`
	To         string            `json:"to"`
	Kind       string            `json:"kind"`
	Confidence string            `json:"confidence,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}
```

- [ ] **Step 4: Implement graph builder**

Create `internal/scan/workspace_graph.go` with:

```go
package scan

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

func StableWorkspaceID(kind string, parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(filepathSlash(part))
		if value != "" {
			clean = append(clean, strings.ToLower(value))
		}
	}
	raw := kind + ":" + strings.Join(clean, ":")
	if len(raw) <= 160 && !strings.Contains(raw, "\n") {
		return raw
	}
	sum := sha1.Sum([]byte(raw))
	return kind + ":" + hex.EncodeToString(sum[:8])
}

func BuildWorkspaceGraph(registry WorkspaceRegistryRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, dossiers []FeatureDossierRecord) WorkspaceGraphRecord {
	builder := workspaceGraphBuilder{
		nodes: map[string]WorkspaceGraphNodeRecord{},
		edges: map[string]WorkspaceGraphEdgeRecord{},
		stats: map[string]int{},
	}
	for _, project := range registry.Projects {
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:      StableWorkspaceID("project", project.Path),
			Kind:    "project",
			Label:   firstNonEmpty(project.Name, project.Path),
			Project: project.Path,
			Meta: map[string]string{
				"kind": project.Kind,
				"path": project.Path,
			},
		})
	}
	for _, match := range matches {
		contractID := firstNonEmpty(match.ContractID, StableWorkspaceID("contract", match.FrontendProject, match.Method, match.Path, match.FrontendFile))
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:         contractID,
			Kind:       "contract",
			Label:      strings.TrimSpace(match.Method + " " + match.Path),
			Project:    match.FrontendProject,
			File:       match.FrontendFile,
			Method:     match.Method,
			Path:       match.Path,
			Confidence: match.Confidence,
			Risk:       match.Issue,
		})
		if match.FrontendProject != "" {
			builder.addEdge(StableWorkspaceID("project", match.FrontendProject), contractID, "declares_contract", match.Confidence, nil)
		}
		if match.BackendProject != "" && match.BackendFile != "" {
			routeID := StableWorkspaceID("route", match.BackendProject, match.Method, match.BackendPath)
			builder.addNode(WorkspaceGraphNodeRecord{
				ID:         routeID,
				Kind:       "route",
				Label:      strings.TrimSpace(match.Method + " " + firstNonEmpty(match.BackendPath, match.Path)),
				Project:    match.BackendProject,
				File:       match.BackendFile,
				Symbol:     match.BackendSymbol,
				Method:     match.Method,
				Path:       firstNonEmpty(match.BackendPath, match.Path),
				Confidence: match.Confidence,
			})
			builder.addEdge(contractID, routeID, "resolved_by", match.Confidence, map[string]string{"issue": match.Issue})
			builder.addEdge(StableWorkspaceID("project", match.BackendProject), routeID, "owns_route", match.Confidence, nil)
		}
	}
	for _, flow := range flows {
		flowID := firstNonEmpty(flow.ID, StableWorkspaceID("flow", flow.FrontendProject, flow.HTTPMethod, flow.HTTPPath))
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:         flowID,
			Kind:       "flow",
			Label:      strings.TrimSpace(flow.HTTPMethod + " " + flow.HTTPPath),
			Project:    firstNonEmpty(flow.FrontendProject, flow.BackendProject),
			Method:     flow.HTTPMethod,
			Path:       flow.HTTPPath,
			Confidence: flow.Confidence,
		})
		if flow.ContractID != "" {
			builder.addEdge(flowID, flow.ContractID, "uses_contract", flow.Confidence, nil)
		}
		for _, step := range flow.Steps {
			stepID := StableWorkspaceID("symbol", step.Project, step.File, step.Symbol, step.Kind)
			builder.addNode(WorkspaceGraphNodeRecord{
				ID:      stepID,
				Kind:    firstNonEmpty(step.Kind, "symbol"),
				Label:   firstNonEmpty(step.Symbol, step.File, step.Kind),
				Project: step.Project,
				File:    step.File,
				Symbol:  step.Symbol,
			})
			builder.addEdge(flowID, stepID, "has_step", flow.Confidence, map[string]string{"step_kind": step.Kind})
		}
	}
	for _, dossier := range dossiers {
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:    dossier.ID,
			Kind:  "feature",
			Label: firstNonEmpty(dossier.Title, dossier.ID),
			Risk:  dossier.RiskLevel,
		})
		if dossier.ContractID != "" {
			builder.addEdge(dossier.ID, dossier.ContractID, "contains", "", nil)
		}
	}
	return builder.record(registry.Root)
}

type workspaceGraphBuilder struct {
	nodes map[string]WorkspaceGraphNodeRecord
	edges map[string]WorkspaceGraphEdgeRecord
	stats map[string]int
}

func (b *workspaceGraphBuilder) addNode(node WorkspaceGraphNodeRecord) {
	if node.ID == "" {
		return
	}
	if _, exists := b.nodes[node.ID]; !exists {
		b.nodes[node.ID] = node
		b.stats["nodes_"+node.Kind]++
	}
}

func (b *workspaceGraphBuilder) addEdge(from, to, kind, confidence string, meta map[string]string) {
	if from == "" || to == "" || kind == "" {
		return
	}
	id := StableWorkspaceID("edge", from, kind, to)
	if _, exists := b.edges[id]; exists {
		return
	}
	b.edges[id] = WorkspaceGraphEdgeRecord{ID: id, From: from, To: to, Kind: kind, Confidence: confidence, Meta: meta}
	b.stats["edges_"+kind]++
}

func (b *workspaceGraphBuilder) record(root string) WorkspaceGraphRecord {
	nodes := make([]WorkspaceGraphNodeRecord, 0, len(b.nodes))
	for _, node := range b.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	edges := make([]WorkspaceGraphEdgeRecord, 0, len(b.edges))
	for _, edge := range b.edges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	b.stats["nodes_total"] = len(nodes)
	b.stats["edges_total"] = len(edges)
	return WorkspaceGraphRecord{
		SchemaVersion: SchemaVersion,
		Generated:     time.Now().UTC().Format(time.RFC3339),
		Root:          root,
		Nodes:         nodes,
		Edges:         edges,
		Stats:         b.stats,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
```

Use an existing slash-normalization helper if one already exists; otherwise add:

```go
func filepathSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}
```

- [ ] **Step 5: Wire graph output into workspace reconcile**

In `internal/scan/workspace_reconcile.go`, after feature dossiers are built:

```go
workspaceGraph := BuildWorkspaceGraph(registry, matches, featureFlows, featureDossiers)
```

Write it after `feature-dossiers.json`:

```go
if err := writeJSON(filepath.Join(workspaceOut, "workspace-graph.json"), workspaceGraph); err != nil {
	return nil, err
}
```

- [ ] **Step 6: Run scan tests**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestBuildWorkspaceGraph|TestWorkspace' -count=1
```

Expected: PASS.

---

### Task 2: Static Clickable Workspace Dashboard

**Files:**
- Create: `internal/scan/workspace_dashboard.go`
- Create: `internal/scan/workspace_dashboard_test.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `docs/OUTPUTS.md`

**Interfaces:**
- Consumes: `WorkspaceGraphRecord`, `[]FeatureDossierRecord`, `[]WorkspaceContractMatchRecord`.
- Produces:
  - `func RenderWorkspaceDashboardHTML(graph WorkspaceGraphRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string`
  - `.goregraph-workspace/workspace-map.html`

- [ ] **Step 1: Write failing HTML test**

Create `internal/scan/workspace_dashboard_test.go`:

```go
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
			{ID: "route:services/ms-user:get:/users/{userId}", Kind: "route", Label: "GET /users/{userId}", Project: "services/ms-user"},
		},
		Edges: []WorkspaceGraphEdgeRecord{
			{ID: "edge:1", From: "project:frontend/app", To: "route:services/ms-user:get:/users/{userId}", Kind: "depends_on"},
		},
	}
	html := RenderWorkspaceDashboardHTML(graph, nil, nil)

	for _, want := range []string{
		`<!doctype html>`,
		`id="workspace-search"`,
		`data-kind-filter`,
		`project:frontend/app`,
		`GET /users/{userId}`,
		`const workspaceGraph =`,
		`function renderGraph()`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard html missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, "https://") || strings.Contains(html, "http://") {
		t.Fatalf("dashboard must not load remote assets")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestRenderWorkspaceDashboardHTMLContainsInteractiveGraphData -count=1
```

Expected: FAIL because renderer is missing.

- [ ] **Step 3: Implement standalone HTML renderer**

Create `internal/scan/workspace_dashboard.go` with:

```go
package scan

import (
	"encoding/json"
	"html"
	"strings"
)

func RenderWorkspaceDashboardHTML(graph WorkspaceGraphRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string {
	payload, _ := json.Marshal(struct {
		Graph    WorkspaceGraphRecord          `json:"graph"`
		Matches  []WorkspaceContractMatchRecord `json:"matches"`
		Dossiers []FeatureDossierRecord        `json:"dossiers"`
	}{Graph: graph, Matches: matches, Dossiers: dossiers})
	title := "GoreGraph Workspace Map"
	if graph.Root != "" {
		title = "GoreGraph Workspace Map - " + graph.Root
	}
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>
:root{color-scheme:light;background:#f7f8f8;color:#111827;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
body{margin:0}
.shell{display:grid;grid-template-columns:320px minmax(0,1fr) 360px;min-height:100vh}
aside,.details{background:#fff;border-color:#d8dee4}
aside{border-right:1px solid #d8dee4;padding:20px;overflow:auto}
.details{border-left:1px solid #d8dee4;padding:20px;overflow:auto}
main{position:relative;overflow:hidden}
h1{font-size:20px;line-height:1.2;margin:0 0 16px}
h2{font-size:14px;margin:22px 0 8px;text-transform:uppercase;color:#586069}
input{box-sizing:border-box;width:100%;height:40px;border:1px solid #c9d1d9;border-radius:6px;padding:0 12px;font:inherit}
button{border:1px solid #c9d1d9;background:#fff;border-radius:6px;height:34px;padding:0 10px;font:inherit;cursor:pointer}
button.active{background:#0f172a;color:#fff;border-color:#0f172a}
.filters{display:flex;flex-wrap:wrap;gap:8px}
.node-list{display:grid;gap:8px;margin-top:16px}
.node-row{border:1px solid #d8dee4;border-radius:6px;background:#fff;padding:10px;text-align:left}
.node-row strong{display:block;font-size:13px}
.node-row span{display:block;font-size:12px;color:#586069;margin-top:4px;overflow:hidden;text-overflow:ellipsis}
svg{width:100%;height:100vh;display:block}
.edge{stroke:#aeb8c2;stroke-width:1.4}
.node circle{stroke:#fff;stroke-width:2}
.node text{font-size:11px;fill:#111827;pointer-events:none}
.kind-project{fill:#2563eb}.kind-contract{fill:#7c3aed}.kind-route{fill:#059669}.kind-feature{fill:#f59e0b}.kind-flow{fill:#dc2626}.kind-symbol{fill:#64748b}
.empty{color:#586069;font-size:13px}
@media (max-width: 980px){.shell{grid-template-columns:1fr}.details,aside{border:0;border-bottom:1px solid #d8dee4}svg{height:64vh}}
</style>
</head>
<body>
<div class="shell">
<aside>
<h1>GoreGraph</h1>
<input id="workspace-search" aria-label="Search workspace graph" placeholder="Search projects, routes, files">
<h2>Filters</h2>
<div class="filters">
<button data-kind-filter="all" class="active">All</button>
<button data-kind-filter="project">Projects</button>
<button data-kind-filter="contract">Contracts</button>
<button data-kind-filter="route">Routes</button>
<button data-kind-filter="feature">Features</button>
</div>
<h2>Results</h2>
<div id="node-list" class="node-list"></div>
</aside>
<main><svg id="workspace-graph" role="img" aria-label="Workspace dependency graph"></svg></main>
<section class="details" id="details"><p class="empty">Select a node to inspect its project, files, contracts, and connected routes.</p></section>
</div>
<script>
const workspaceGraph = ` + string(payload) + `;
const state = {query:"", kind:"all", selected:null};
function visibleNodes(){
 const q=state.query.toLowerCase();
 return workspaceGraph.graph.nodes.filter(n => (state.kind==="all" || n.kind===state.kind) && (!q || JSON.stringify(n).toLowerCase().includes(q)));
}
function renderList(){
 const list=document.getElementById("node-list");
 const nodes=visibleNodes().slice(0,80);
 list.innerHTML=nodes.map(n=>` + "`" + `<button class="node-row" data-node-id="${escapeAttr(n.id)}"><strong>${escapeHtml(n.label||n.id)}</strong><span>${escapeHtml(n.kind)} · ${escapeHtml(n.project||"workspace")}</span></button>` + "`" + `).join("") || "<p class='empty'>No matching nodes.</p>";
 list.querySelectorAll("[data-node-id]").forEach(el=>el.addEventListener("click",()=>selectNode(el.dataset.nodeId)));
}
function renderGraph(){
 const svg=document.getElementById("workspace-graph");
 const nodes=visibleNodes().slice(0,160);
 const nodeIDs=new Set(nodes.map(n=>n.id));
 const edges=workspaceGraph.graph.edges.filter(e=>nodeIDs.has(e.from)&&nodeIDs.has(e.to));
 const width=svg.clientWidth||900, height=svg.clientHeight||700;
 const cx=width/2, cy=height/2, radius=Math.max(160, Math.min(width,height)*0.36);
 const pos=new Map();
 nodes.forEach((n,i)=>{const a=(Math.PI*2*i)/Math.max(nodes.length,1);pos.set(n.id,{x:cx+Math.cos(a)*radius,y:cy+Math.sin(a)*radius});});
 svg.innerHTML=edges.map(e=>{const a=pos.get(e.from),b=pos.get(e.to);return a&&b?` + "`" + `<line class="edge" x1="${a.x}" y1="${a.y}" x2="${b.x}" y2="${b.y}"></line>` + "`" + `:""}).join("")+
 nodes.map(n=>{const p=pos.get(n.id);return ` + "`" + `<g class="node" data-node-id="${escapeAttr(n.id)}"><circle class="kind-${escapeAttr(n.kind)}" cx="${p.x}" cy="${p.y}" r="8"></circle><text x="${p.x+12}" y="${p.y+4}">${escapeHtml(shortLabel(n.label||n.id))}</text></g>` + "`" + `}).join("");
 svg.querySelectorAll("[data-node-id]").forEach(el=>el.addEventListener("click",()=>selectNode(el.dataset.nodeId)));
}
function selectNode(id){
 state.selected=id;
 const node=workspaceGraph.graph.nodes.find(n=>n.id===id);
 const links=workspaceGraph.graph.edges.filter(e=>e.from===id||e.to===id);
 document.getElementById("details").innerHTML=node?` + "`" + `<h1>${escapeHtml(node.label||node.id)}</h1><p><strong>${escapeHtml(node.kind)}</strong></p><p>${escapeHtml(node.project||"")}</p><p>${escapeHtml(node.file||"")}</p><h2>Connections</h2>${links.map(e=>`<p>${escapeHtml(e.kind)}<br><small>${escapeHtml(e.from)} → ${escapeHtml(e.to)}</small></p>`).join("")||"<p class='empty'>No direct edges.</p>"}` + "`" + `:"<p class='empty'>Node not found.</p>";
}
function escapeHtml(v){return String(v||"").replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c]));}
function escapeAttr(v){return escapeHtml(v).replace(/` + "`" + `/g,"&#96;");}
function shortLabel(v){v=String(v||"");return v.length>42?v.slice(0,39)+"...":v;}
document.getElementById("workspace-search").addEventListener("input",e=>{state.query=e.target.value;renderList();renderGraph();});
document.querySelectorAll("[data-kind-filter]").forEach(btn=>btn.addEventListener("click",()=>{state.kind=btn.dataset.kindFilter;document.querySelectorAll("[data-kind-filter]").forEach(b=>b.classList.toggle("active",b===btn));renderList();renderGraph();}));
window.addEventListener("resize",renderGraph);
renderList();renderGraph();
</script>
</body>
</html>`
}
```

If Go string escaping becomes unwieldy, split the template into small `strings.Builder` sections. Do not add a frontend build chain.

- [ ] **Step 4: Wire dashboard output**

In `internal/scan/workspace_reconcile.go`, after `workspace-graph.json`:

```go
if err := os.WriteFile(filepath.Join(workspaceOut, "workspace-map.html"), []byte(RenderWorkspaceDashboardHTML(workspaceGraph, matches, featureDossiers)), 0o644); err != nil {
	return nil, err
}
```

- [ ] **Step 5: Test dashboard output**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestRenderWorkspaceDashboardHTML|TestBuildWorkspaceGraph' -count=1
```

Expected: PASS.

---

### Task 3: Workspace Explain Command

**Files:**
- Create: `internal/scan/workspace_explain.go`
- Create: `internal/scan/workspace_explain_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `.goregraph-workspace/workspace-graph.json`, `contract-matches.json`, `feature-dossiers.json`.
- Produces:
  - `func ExplainWorkspaceTarget(workspaceOut string, target string) (WorkspaceExplainRecord, error)`
  - `func RenderWorkspaceExplain(record WorkspaceExplainRecord) string`
  - CLI: `goregraph workspace explain <target> [--workspace <path>]`

- [ ] **Step 1: Write failing explain test**

Create `internal/scan/workspace_explain_test.go`:

```go
package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainWorkspaceTargetFindsRouteAndConnections(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := WorkspaceGraphRecord{
		Nodes: []WorkspaceGraphNodeRecord{
			{ID: "contract:get-users", Kind: "contract", Label: "GET /users/{userId}", Project: "frontend/app", File: "src/api/users.ts"},
			{ID: "route:ms-user:get:/users/{userId}", Kind: "route", Label: "GET /users/{userId}", Project: "services/ms-user", File: "UserController.java", Symbol: "UserController.get"},
		},
		Edges: []WorkspaceGraphEdgeRecord{
			{ID: "edge:1", From: "contract:get-users", To: "route:ms-user:get:/users/{userId}", Kind: "resolved_by", Confidence: "RESOLVED"},
		},
	}
	if err := writeJSON(filepath.Join(out, "workspace-graph.json"), graph); err != nil {
		t.Fatal(err)
	}

	explain, err := ExplainWorkspaceTarget(out, "GET /users/{userId}")
	if err != nil {
		t.Fatal(err)
	}
	report := RenderWorkspaceExplain(explain)
	if !strings.Contains(report, "GET /users/{userId}") || !strings.Contains(report, "UserController.get") || !strings.Contains(report, "src/api/users.ts") {
		t.Fatalf("unexpected explain report:\n%s", report)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestExplainWorkspaceTargetFindsRouteAndConnections -count=1
```

Expected: FAIL because explain functions are missing.

- [ ] **Step 3: Add explain types**

Add to `internal/scan/types.go`:

```go
type WorkspaceExplainRecord struct {
	Target      string                     `json:"target"`
	MatchedNode WorkspaceGraphNodeRecord   `json:"matched_node"`
	Neighbors   []WorkspaceGraphNodeRecord `json:"neighbors,omitempty"`
	Edges       []WorkspaceGraphEdgeRecord `json:"edges,omitempty"`
}
```

- [ ] **Step 4: Implement explain**

Create `internal/scan/workspace_explain.go`:

```go
package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ExplainWorkspaceTarget(workspaceOut string, target string) (WorkspaceExplainRecord, error) {
	graph, err := readWorkspaceGraph(filepath.Join(workspaceOut, "workspace-graph.json"))
	if err != nil {
		return WorkspaceExplainRecord{}, err
	}
	targetLower := strings.ToLower(strings.TrimSpace(target))
	var matched WorkspaceGraphNodeRecord
	for _, node := range graph.Nodes {
		haystack := strings.ToLower(strings.Join([]string{node.ID, node.Label, node.Project, node.File, node.Symbol, node.Method + " " + node.Path}, " "))
		if strings.Contains(haystack, targetLower) {
			matched = node
			break
		}
	}
	if matched.ID == "" {
		return WorkspaceExplainRecord{}, fmt.Errorf("no workspace graph node matches %q", target)
	}
	byID := map[string]WorkspaceGraphNodeRecord{}
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	record := WorkspaceExplainRecord{Target: target, MatchedNode: matched}
	for _, edge := range graph.Edges {
		if edge.From != matched.ID && edge.To != matched.ID {
			continue
		}
		record.Edges = append(record.Edges, edge)
		if edge.From == matched.ID {
			record.Neighbors = append(record.Neighbors, byID[edge.To])
		} else {
			record.Neighbors = append(record.Neighbors, byID[edge.From])
		}
	}
	return record, nil
}

func RenderWorkspaceExplain(record WorkspaceExplainRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GoreGraph Explain\n\n")
	fmt.Fprintf(&b, "- Target: `%s`\n", record.Target)
	fmt.Fprintf(&b, "- Node: `%s`\n", record.MatchedNode.ID)
	fmt.Fprintf(&b, "- Kind: `%s`\n", record.MatchedNode.Kind)
	if record.MatchedNode.Project != "" {
		fmt.Fprintf(&b, "- Project: `%s`\n", record.MatchedNode.Project)
	}
	if record.MatchedNode.File != "" {
		fmt.Fprintf(&b, "- File: `%s`\n", record.MatchedNode.File)
	}
	if record.MatchedNode.Symbol != "" {
		fmt.Fprintf(&b, "- Symbol: `%s`\n", record.MatchedNode.Symbol)
	}
	fmt.Fprintf(&b, "\n## Connections\n\n")
	if len(record.Edges) == 0 {
		fmt.Fprintf(&b, "none\n")
		return b.String()
	}
	for i, edge := range record.Edges {
		neighbor := WorkspaceGraphNodeRecord{}
		if i < len(record.Neighbors) {
			neighbor = record.Neighbors[i]
		}
		fmt.Fprintf(&b, "- `%s` -> `%s` (`%s`, %s)\n", edge.From, edge.To, edge.Kind, edge.Confidence)
		if neighbor.ID != "" {
			fmt.Fprintf(&b, "  - %s `%s` %s\n", neighbor.Kind, neighbor.ID, neighbor.Label)
		}
	}
	return b.String()
}

func readWorkspaceGraph(path string) (WorkspaceGraphRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceGraphRecord{}, err
	}
	var graph WorkspaceGraphRecord
	if err := json.Unmarshal(data, &graph); err != nil {
		return WorkspaceGraphRecord{}, err
	}
	return graph, nil
}
```

- [ ] **Step 5: Add CLI command**

In `internal/cli/cli.go`, add `explain` under `workspace`:

```go
case "explain":
	return runWorkspaceExplain(args[1:], stdout, stderr)
```

Add:

```go
func runWorkspaceExplain(args []string, stdout, stderr io.Writer) int {
	cfg := config.Defaults()
	root := "."
	target := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			cfg.WorkspaceRoot = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace explain <target> [--workspace <path>]\n\nExplains a workspace graph node, route, file, symbol, or contract using generated outputs.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			if target == "" {
				target = arg
			} else {
				target += " " + arg
			}
		}
	}
	if target == "" {
		fmt.Fprint(stderr, "error: workspace explain requires a target\n")
		return 2
	}
	workspaceRoot, ok, err := scan.WorkspaceRoot(root, cfg)
	if err != nil || !ok {
		fmt.Fprintf(stderr, "error: workspace root not found: %v\n", err)
		return 1
	}
	record, err := scan.ExplainWorkspaceTarget(filepath.Join(workspaceRoot, ".goregraph-workspace"), target)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace explain failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(scan.RenderWorkspaceExplain(record)))
	return 0
}
```

- [ ] **Step 6: Add CLI test and run**

Add a CLI test in `internal/cli/cli_test.go` that invokes:

```go
code := Run([]string{"workspace", "explain", "GET /users/{userId}", "--workspace", root}, &stdout, &stderr)
```

Expected: exit code `0`, stdout contains `GoreGraph Explain`.

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/cli ./internal/scan -run 'TestExplain|TestRunWorkspaceExplain' -count=1
```

Expected: PASS.

---

### Task 4: Workspace Path Command

**Files:**
- Create: `internal/scan/workspace_path.go`
- Create: `internal/scan/workspace_path_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `workspace-graph.json`.
- Produces:
  - `func WorkspacePath(workspaceOut, from, to string) ([]WorkspaceGraphNodeRecord, []WorkspaceGraphEdgeRecord, error)`
  - `func RenderWorkspacePath(nodes []WorkspaceGraphNodeRecord, edges []WorkspaceGraphEdgeRecord) string`
  - CLI: `goregraph workspace path --from <target> --to <target> [--workspace <path>]`

- [ ] **Step 1: Write failing path test**

Create `internal/scan/workspace_path_test.go`:

```go
package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspacePathFindsRouteBetweenFrontendAndBackend(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := WorkspaceGraphRecord{
		Nodes: []WorkspaceGraphNodeRecord{
			{ID: "project:frontend/app", Kind: "project", Label: "app"},
			{ID: "contract:get-users", Kind: "contract", Label: "GET /users/{userId}"},
			{ID: "route:ms-user:get:/users/{userId}", Kind: "route", Label: "UserController.get"},
		},
		Edges: []WorkspaceGraphEdgeRecord{
			{ID: "edge:1", From: "project:frontend/app", To: "contract:get-users", Kind: "declares_contract"},
			{ID: "edge:2", From: "contract:get-users", To: "route:ms-user:get:/users/{userId}", Kind: "resolved_by"},
		},
	}
	if err := writeJSON(filepath.Join(out, "workspace-graph.json"), graph); err != nil {
		t.Fatal(err)
	}

	nodes, edges, err := WorkspacePath(out, "frontend/app", "UserController.get")
	if err != nil {
		t.Fatal(err)
	}
	report := RenderWorkspacePath(nodes, edges)
	if !strings.Contains(report, "frontend/app") || !strings.Contains(report, "GET /users/{userId}") || !strings.Contains(report, "UserController.get") {
		t.Fatalf("unexpected path report:\n%s", report)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestWorkspacePathFindsRouteBetweenFrontendAndBackend -count=1
```

Expected: FAIL because `WorkspacePath` does not exist.

- [ ] **Step 3: Implement BFS over graph**

Create `internal/scan/workspace_path.go`:

```go
package scan

import (
	"fmt"
	"path/filepath"
	"strings"
)

func WorkspacePath(workspaceOut, from, to string) ([]WorkspaceGraphNodeRecord, []WorkspaceGraphEdgeRecord, error) {
	graph, err := readWorkspaceGraph(filepath.Join(workspaceOut, "workspace-graph.json"))
	if err != nil {
		return nil, nil, err
	}
	start := matchGraphNode(graph.Nodes, from)
	end := matchGraphNode(graph.Nodes, to)
	if start.ID == "" || end.ID == "" {
		return nil, nil, fmt.Errorf("path endpoints not found")
	}
	neighbors := map[string][]WorkspaceGraphEdgeRecord{}
	for _, edge := range graph.Edges {
		neighbors[edge.From] = append(neighbors[edge.From], edge)
		neighbors[edge.To] = append(neighbors[edge.To], WorkspaceGraphEdgeRecord{
			ID: edge.ID, From: edge.To, To: edge.From, Kind: edge.Kind, Confidence: edge.Confidence, Meta: edge.Meta,
		})
	}
	queue := []string{start.ID}
	seen := map[string]bool{start.ID: true}
	prevNode := map[string]string{}
	prevEdge := map[string]WorkspaceGraphEdgeRecord{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == end.ID {
			break
		}
		for _, edge := range neighbors[current] {
			if seen[edge.To] {
				continue
			}
			seen[edge.To] = true
			prevNode[edge.To] = current
			prevEdge[edge.To] = edge
			queue = append(queue, edge.To)
		}
	}
	if !seen[end.ID] {
		return nil, nil, fmt.Errorf("no path from %q to %q", from, to)
	}
	byID := map[string]WorkspaceGraphNodeRecord{}
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	var nodeIDs []string
	var edges []WorkspaceGraphEdgeRecord
	for current := end.ID; current != ""; current = prevNode[current] {
		nodeIDs = append(nodeIDs, current)
		if current == start.ID {
			break
		}
		edges = append(edges, prevEdge[current])
	}
	nodes := make([]WorkspaceGraphNodeRecord, 0, len(nodeIDs))
	for i := len(nodeIDs) - 1; i >= 0; i-- {
		nodes = append(nodes, byID[nodeIDs[i]])
	}
	for i, j := 0, len(edges)-1; i < j; i, j = i+1, j-1 {
		edges[i], edges[j] = edges[j], edges[i]
	}
	return nodes, edges, nil
}

func RenderWorkspacePath(nodes []WorkspaceGraphNodeRecord, edges []WorkspaceGraphEdgeRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GoreGraph Path\n\n")
	for i, node := range nodes {
		if i > 0 && i-1 < len(edges) {
			fmt.Fprintf(&b, "  -- `%s` -->\n", edges[i-1].Kind)
		}
		fmt.Fprintf(&b, "%d. `%s` %s\n", i+1, node.Kind, firstNonEmpty(node.Label, node.ID))
	}
	return b.String()
}

func matchGraphNode(nodes []WorkspaceGraphNodeRecord, target string) WorkspaceGraphNodeRecord {
	q := strings.ToLower(strings.TrimSpace(target))
	for _, node := range nodes {
		haystack := strings.ToLower(strings.Join([]string{node.ID, node.Label, node.Project, node.File, node.Symbol, node.Method + " " + node.Path}, " "))
		if strings.Contains(haystack, q) {
			return node
		}
	}
	return WorkspaceGraphNodeRecord{}
}
```

- [ ] **Step 4: Add CLI command and tests**

Add `workspace path` to `internal/cli/cli.go` with options `--from`, `--to`, and `--workspace`.

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/cli ./internal/scan -run 'TestWorkspacePath|TestRunWorkspacePath' -count=1
```

Expected: PASS.

---

### Task 5: Workspace Impact Command

**Files:**
- Create: `internal/scan/workspace_impact.go`
- Create: `internal/scan/workspace_impact_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/scan/workspace_reconcile.go`

**Interfaces:**
- Consumes: `feature-dossiers.json`, `feature-flows.json`, `contract-matches.json`, changed file list.
- Produces:
  - `func WorkspaceImpact(workspaceOut string, changedFiles []string) (WorkspaceImpactRecord, error)`
  - `func RenderWorkspaceImpact(record WorkspaceImpactRecord) string`
  - CLI: `goregraph workspace impact --changed-file <path> [--changed-file <path>] [--workspace <path>]`

- [ ] **Step 1: Write failing impact test**

Create `internal/scan/workspace_impact_test.go`:

```go
package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceImpactMapsChangedFileToFeatureDossier(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	dossiers := []FeatureDossierRecord{{
		ID:         "feature:get-users",
		Title:      "GET /users/{userId}",
		ContractID: "contract:get-users",
		Files:      []string{"frontend/app/src/api/users.ts", "services/ms-user/UserController.java"},
		RiskLevel:  "medium",
	}}
	if err := writeJSON(filepath.Join(out, "feature-dossiers.json"), dossiers); err != nil {
		t.Fatal(err)
	}

	impact, err := WorkspaceImpact(out, []string{"frontend/app/src/api/users.ts"})
	if err != nil {
		t.Fatal(err)
	}
	report := RenderWorkspaceImpact(impact)
	if !strings.Contains(report, "GET /users/{userId}") || !strings.Contains(report, "medium") {
		t.Fatalf("unexpected impact report:\n%s", report)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestWorkspaceImpactMapsChangedFileToFeatureDossier -count=1
```

Expected: FAIL because impact functions are missing.

- [ ] **Step 3: Add impact type**

Add to `internal/scan/types.go`:

```go
type WorkspaceImpactRecord struct {
	ChangedFiles     []string               `json:"changed_files"`
	AffectedFeatures []FeatureDossierRecord `json:"affected_features"`
	RiskSummary      map[string]int         `json:"risk_summary,omitempty"`
}
```

- [ ] **Step 4: Implement explicit-file impact**

Create `internal/scan/workspace_impact.go`:

```go
package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WorkspaceImpact(workspaceOut string, changedFiles []string) (WorkspaceImpactRecord, error) {
	data, err := os.ReadFile(filepath.Join(workspaceOut, "feature-dossiers.json"))
	if err != nil {
		return WorkspaceImpactRecord{}, err
	}
	var dossiers []FeatureDossierRecord
	if err := json.Unmarshal(data, &dossiers); err != nil {
		return WorkspaceImpactRecord{}, err
	}
	changed := map[string]bool{}
	for _, file := range changedFiles {
		changed[filepathSlash(file)] = true
	}
	record := WorkspaceImpactRecord{ChangedFiles: changedFiles, RiskSummary: map[string]int{}}
	seen := map[string]bool{}
	for _, dossier := range dossiers {
		for _, file := range dossier.Files {
			if changed[filepathSlash(file)] && !seen[dossier.ID] {
				record.AffectedFeatures = append(record.AffectedFeatures, dossier)
				record.RiskSummary[firstNonEmpty(dossier.RiskLevel, "unknown")]++
				seen[dossier.ID] = true
			}
		}
	}
	return record, nil
}

func RenderWorkspaceImpact(record WorkspaceImpactRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GoreGraph Impact\n\n")
	fmt.Fprintf(&b, "Changed files: %d\n\n", len(record.ChangedFiles))
	if len(record.AffectedFeatures) == 0 {
		fmt.Fprintf(&b, "No affected features found.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "## Affected Features\n\n")
	for _, feature := range record.AffectedFeatures {
		fmt.Fprintf(&b, "- `%s` %s", firstNonEmpty(feature.RiskLevel, "unknown"), feature.Title)
		if feature.ContractID != "" {
			fmt.Fprintf(&b, " (`%s`)", feature.ContractID)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}
```

- [ ] **Step 5: Add CLI command**

Add `workspace impact` with repeated `--changed-file` support. If no `--changed-file` is provided, print:

```text
error: workspace impact requires at least one --changed-file
```

Do not read git diff in this task; that is a later enhancement because it adds environment variance.

- [ ] **Step 6: Run tests**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/cli ./internal/scan -run 'TestWorkspaceImpact|TestRunWorkspaceImpact' -count=1
```

Expected: PASS.

---

### Task 6: Dashboard CLI and Discoverability

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `docs/OUTPUTS.md`

**Interfaces:**
- Consumes: `.goregraph-workspace/workspace-map.html`.
- Produces:
  - CLI: `goregraph workspace dashboard [path] [--workspace <path>]`
  - Prints absolute dashboard path and does not auto-open a browser.

- [ ] **Step 1: Write failing dashboard CLI test**

Add to `internal/cli/cli_test.go`:

```go
func TestRunWorkspaceDashboardPrintsDashboardPath(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "workspace-map.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "dashboard", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "workspace-map.html") {
		t.Fatalf("stdout missing dashboard path: %s", stdout.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/cli -run TestRunWorkspaceDashboardPrintsDashboardPath -count=1
```

Expected: FAIL because command is missing.

- [ ] **Step 3: Implement CLI**

Add `dashboard` to `runWorkspace`, then implement:

```go
func runWorkspaceDashboard(args []string, stdout, stderr io.Writer) int {
	cfg := config.Defaults()
	root := "."
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			cfg.WorkspaceRoot = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace dashboard [path] [--workspace <path>]\n\nPrints the generated workspace dashboard HTML path.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}
	workspaceRoot, ok, err := scan.WorkspaceRoot(root, cfg)
	if err != nil || !ok {
		fmt.Fprintf(stderr, "error: workspace root not found: %v\n", err)
		return 1
	}
	path := filepath.Join(workspaceRoot, ".goregraph-workspace", "workspace-map.html")
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(stderr, "error: dashboard not found; run goregraph workspace scan-all first: %v\n", err)
		return 1
	}
	abs, _ := filepath.Abs(path)
	fmt.Fprintf(stdout, "%s\n", abs)
	return 0
}
```

- [ ] **Step 4: Update help text**

Update workspace help to include:

```text
dashboard      Print generated workspace dashboard path
explain        Explain a route, file, symbol, contract, or feature
path           Show a graph path between two workspace targets
impact         Show affected features for changed files
```

- [ ] **Step 5: Run CLI tests**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/cli -run 'TestRunWorkspaceDashboard|TestRunWorkspaceHelp' -count=1
```

Expected: PASS.

---

### Task 7: Documentation and Output Contract

**Files:**
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Documents generated files and commands.

- [ ] **Step 1: Update output docs**

Add this section to `docs/OUTPUTS.md`:

```markdown
## Workspace Understanding Outputs

`goregraph workspace scan-all <workspace>` generates these additional workspace files:

- `.goregraph-workspace/workspace-graph.json`: normalized graph model with stable node and edge IDs.
- `.goregraph-workspace/workspace-map.html`: standalone offline dashboard for search, filtering, and clickable graph inspection.
- `.goregraph-workspace/feature-dossiers.json`: feature-centric route/UI/backend/test/risk summaries.
- `.goregraph-workspace/feature-dossiers.md`: human-readable feature dossiers.

Stable graph IDs are deterministic from kind plus normalized semantic parts, for example:

- `project:frontend/app`
- `contract:<existing-contract-id>`
- `route:<project>:<method>:<path>`
- `feature:<existing-feature-id>`

The dashboard does not load remote assets and does not execute scan logic. It only renders generated GoreGraph JSON.
```

- [ ] **Step 2: Update release notes**

Add to `docs/RELEASE.md` under `0.9.0`:

```markdown
- Added workspace understanding outputs: `workspace-graph.json` and standalone `workspace-map.html`.
- Added workspace navigation commands: `workspace dashboard`, `workspace explain`, `workspace path`, and `workspace impact`.
- Added stable graph IDs for projects, contracts, routes, flows, features, and symbols.
```

- [ ] **Step 3: Run documentation-adjacent tests**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./...
```

Expected: PASS.

---

### Task 8: Full Local Verification on WEKA Workspace

**Files:**
- No repo source changes expected.
- Generated outputs in `/Users/gorecode/projects/weka/.goregraph-workspace` are verification artifacts, not release source.

**Interfaces:**
- Validates general workspace behavior on the large WEKA workspace.

- [ ] **Step 1: Build local binary**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go build -o /private/tmp/goregraph ./cmd/goregraph
```

Expected: PASS.

- [ ] **Step 2: Install locally only if requested or needed for user testing**

Run with approval if installing to `/opt/homebrew/bin`:

```bash
cp /private/tmp/goregraph /opt/homebrew/bin/goregraph
```

Expected:

```text
goregraph version
```

prints `goregraph 0.9.0`.

- [ ] **Step 3: Clean WEKA outputs**

Run only when the user wants a fresh workspace scan:

```bash
cd /Users/gorecode/projects/weka
goregraph workspace clean . --execute
```

Expected: GoreGraph output directories are removed.

- [ ] **Step 4: Scan all**

Run:

```bash
cd /Users/gorecode/projects/weka
goregraph workspace scan-all . --no-update-gitignore
```

Expected: all workspace projects are scanned and `.goregraph-workspace/workspace-map.html` exists.

- [ ] **Step 5: Verify new commands**

Run:

```bash
cd /Users/gorecode/projects/weka
goregraph workspace dashboard .
goregraph workspace explain "GET /cadasters/{cadasterId}"
goregraph workspace path --from "apps/portal/src/api/vd/userService.ts" --to "CadasterController.get"
goregraph workspace impact --changed-file "microservices/ms-cadaster/src/main/java/de/weka/businessportal/cadaster/CadasterController.java"
```

Expected:

- `workspace dashboard` prints an absolute `workspace-map.html` path.
- `workspace explain` prints frontend consumers and backend route context.
- `workspace path` shows a project/contract/route/step chain.
- `workspace impact` lists affected feature dossiers and risk levels.

---

## Self-Review

**Spec coverage:** This plan covers the requested direction toward Understand-Anything style usefulness: clickable visualization, local dashboard, explain/path commands, impact view, stable IDs, and generated artifacts that are not WEKA-specific.

**Intentional exclusions:** LLM summaries, multi-agent background scanning, installer hooks, semantic embeddings, and chat UI are excluded from this version because they would make GoreGraph less deterministic and harder to trust. They can be optional modules later.

**Risk:** The dashboard renderer is intentionally simple. It will be useful immediately, but visual polish can be improved after the graph contract is stable.

**Verification gate:** The implementation is not complete until `GOCACHE=/private/tmp/goregraph-gocache go test ./...` passes and the WEKA scan produces `workspace-graph.json` plus `workspace-map.html`.
