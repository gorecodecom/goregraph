# Architecture Map Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the offline Architecture map for GitHub issue #23 so dense workspaces retain stable full-map context while exposing dynamic domain lanes, deterministic relationship trunks, complete selected-service neighborhoods, accessible domain focus, a persistent relationship summary, and inspectable static-call badges.

**Architecture:** Keep `workspace-service-map.json` as the only architecture data source and move DOM-free domain/layout/focus/count calculations into one embedded vanilla-JavaScript model shared by the renderer and executable Node tests. The SVG renderer always lays out the complete deterministic service set, applies service/domain/direction/risk state only as emphasis, draws background domain trunks before opaque cards, and fans selected direct edges to explicit ports. HTML controls and the inspector own summary, tooltip, filter, and detail interactions without changing SVG coordinates or viewport state.

**Tech Stack:** Go 1.23 standard library, Schema 2 additive records, standalone HTML/CSS, embedded vanilla JavaScript, Go tests, Node `--check` and DOM-free model execution, Playwright/browser acceptance, Git.

## Global Constraints

- Work directly on `main`; do not create or switch to a feature branch or worktree.
- Use strict TDD for every production behavior: RED, GREEN, REFACTOR, then focused tests before the task commit.
- Preserve the full architecture and stable service-card positions.
- Render dynamic domain lanes from service-map metadata rather than hardcoded workspace-specific labels or palettes.
- Highlight all direct incoming and outgoing relationships for a selected service simultaneously by default.
- Dim unrelated lanes, cards, and edges instead of hiding or re-laying them out.
- Direct references shown by the Architecture map are statically detected relationships, never runtime traffic or invocation frequency.
- Keep the inspector as the detailed relationship surface; the summary is an overview and filter surface only.
- The dashboard remains standalone, deterministic, and fully offline; do not add network assets or runtime dependencies.
- Do not execute scanned source code, build tools, dependency installers, or scanned applications.
- Keep Schema 2 and existing service-map fields compatible; issue #23 needs no schema-version increment.
- Keep the source target at unreleased `1.3.0`.
- Do not create a tag, GitHub Release, Homebrew publication, Scoop publication, or Winget publication.
- Preserve concurrent issue #25 Code Explorer and documentation changes; this plan changes only Architecture-map behavior and its documentation.
- All source comments, public labels, commit messages, and documentation added by this plan are in English.
- Add no Go or JavaScript dependency.
- Support and visually verify 1280×720, 1440×900, and 1920×1080.
- Color is never the only state cue; domain headers/chips, filters, badges, reset, and service cards require visible focus and keyboard activation.
- Respect `prefers-reduced-motion`; expose tooltips to pointer hover and keyboard focus.

---

## File Structure

- Create `internal/scan/workspace_dashboard_architecture.go`: DOM-free embedded JavaScript for domain grouping, deterministic layout, focus sets, counts, and bundle grouping.
- Create `internal/scan/workspace_dashboard_architecture_test.go`: dense generic fixture, Node harness for the production model, renderer contracts, syntax gate, and issue #23 acceptance.
- Modify `internal/scan/workspace_dashboard_template.go`: persistent Architecture controls/summary/tooltip outside SVG and the additional embedded model script.
- Modify `internal/scan/workspace_dashboard_script.go`: geometry, render orchestration, accessible event wiring, summary/badge/inspector behavior, and viewport-preserving state.
- Modify `internal/scan/workspace_dashboard_styles.go`: neutral lanes, dim/focus states, persistent ribbon, accessible badges/tooltips, responsive placement, and reduced motion.
- Modify `internal/scan/workspace_dashboard_test.go`: replace stale fixed-domain, explicit-isolation, and obsolete-copy assertions.
- Modify `docs/design-system.md`, `README.md`, `COMMANDS.md`, `docs/OUTPUTS.md`, `docs/RELEASE.md`, and `docs_test.go`: issue #23 public behavior and unreleased 1.3.0 contract.

### Task 1: Extract a Testable Dynamic Domain and Stable Layout Model

**Files:**
- Create: `internal/scan/workspace_dashboard_architecture.go`
- Create: `internal/scan/workspace_dashboard_architecture_test.go`
- Modify: `internal/scan/workspace_dashboard_template.go:79-93`
- Modify: `internal/scan/workspace_dashboard_script.go:25,36-37,65`
- Modify: `internal/scan/workspace_dashboard_styles.go:9-15`
- Modify: `internal/scan/workspace_dashboard_test.go:1003-1014`

**Interfaces:**
- Consumes: `workspacePayload.service_map.nodes[]`, especially `id`, `label`, `project`, `domain`, `incoming`, and `outgoing`.
- Produces: `const workspaceDashboardArchitectureModelScript string`, embedded before `workspaceDashboardScript` in the same offline `<script>`.
- Produces: JavaScript `architectureDomainKey(node) string`, `architectureDomainLabel(domain) string`, `architectureDomainColor(domain) {fill,stroke}`, `architectureDomains(nodes) Array<{id,label,color,nodes}>`, and `architectureLayout(nodes,width) {positions,domains,width,height,step,cardWidth,cardHeight,margin}`.
- Produces: `runArchitectureModel(t *testing.T, expression string, target any)` executing the exact production model with Node.

- [ ] **Step 1: Add the dense generic fixture and failing model test**

Create `internal/scan/workspace_dashboard_architecture_test.go`:

```go
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
	if err != nil { t.Skip("node is required for embedded dashboard model tests") }
	source := workspaceDashboardArchitectureModelScript + "\nprocess.stdout.write(JSON.stringify(" + expression + "));"
	cmd := exec.Command(node, "-e", source)
	output, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("architecture model failed: %v\n%s", err, output) }
	if err := json.Unmarshal(output, target); err != nil { t.Fatalf("decode architecture model result: %v\n%s", err, output) }
}

func TestArchitectureModelDerivesDynamicDomainsAndStablePositions(t *testing.T) {
	var result struct {
		Domains []string `json:"domains"`
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
	if strings.Join(result.Domains, ",") != strings.Join(want, ",") { t.Fatalf("domains = %v, want %v", result.Domains, want) }
	if len(result.Positions) != 4 { t.Fatalf("positions = %d, want all 4 services", len(result.Positions)) }
}
```

- [ ] **Step 2: Run the model test and confirm RED**

Run: `go test ./internal/scan -run TestArchitectureModelDerivesDynamicDomainsAndStablePositions -count=1`

Expected: compilation fails with `undefined: workspaceDashboardArchitectureModelScript`.

- [ ] **Step 3: Add the minimal DOM-free architecture model**

Create `internal/scan/workspace_dashboard_architecture.go`:

```go
package scan

const workspaceDashboardArchitectureModelScript = `
const architectureLanePalette=[
  {fill:"#e8eef1",stroke:"#bdcbd2"},{fill:"#edf0eb",stroke:"#c4cec0"},
  {fill:"#f1eee8",stroke:"#d2c8b9"},{fill:"#ecebf1",stroke:"#c8c4d1"},
  {fill:"#eaf0ef",stroke:"#bfd0cc"},{fill:"#f0ebea",stroke:"#d2c2bf"}
];
function architectureDomainKey(node){const value=String(node&&node.domain||"").trim().toLowerCase();return value||"unassigned";}
function architectureDomainLabel(domain){return String(domain||"unassigned").split(/[._\-/]+/).filter(Boolean).map(function(word){return word.charAt(0).toUpperCase()+word.slice(1);}).join(" ")||"Unassigned";}
function architectureDomainColor(domain){let hash=2166136261;String(domain||"unassigned").split("").forEach(function(character){hash^=character.charCodeAt(0);hash=Math.imul(hash,16777619);});return architectureLanePalette[(hash>>>0)%architectureLanePalette.length];}
function architectureDomains(nodes){
  const groups=new Map();
  (nodes||[]).forEach(function(node){const id=architectureDomainKey(node);if(!groups.has(id))groups.set(id,[]);groups.get(id).push(node);});
  return Array.from(groups.entries()).map(function(entry){const id=entry[0],domainNodes=entry[1].slice().sort(function(a,b){return String(a.label||a.project||a.id).localeCompare(String(b.label||b.project||b.id))||String(a.id).localeCompare(String(b.id));});return {id:id,label:architectureDomainLabel(id),color:architectureDomainColor(id),nodes:domainNodes};}).sort(function(a,b){return a.label.localeCompare(b.label)||a.id.localeCompare(b.id);});
}
function architectureLayout(nodes,width){
  const domains=architectureDomains(nodes),layoutWidth=Math.max(width||0,Math.max(1040,domains.length*300+84)),margin=42,cardWidth=224,cardHeight=74;
  const step=domains.length>1?(layoutWidth-margin*2-cardWidth)/(domains.length-1):0,positions=new Map();let maxLength=0;
  domains.forEach(function(domain,lane){maxLength=Math.max(maxLength,domain.nodes.length);domain.nodes.forEach(function(node,index){positions.set(node.id,{x:margin+lane*step,y:190+index*90,lane:lane,w:cardWidth,h:cardHeight,domain:domain.id});});});
  return {positions:positions,width:layoutWidth,height:Math.max(760,290+maxLength*90),domains:domains,step:step,cardWidth:cardWidth,cardHeight:cardHeight,margin:margin};
}
`
```

In `renderWorkspaceDashboardDocument`, embed the model before the existing script:

```go
b.WriteString(";\n")
b.WriteString(workspaceDashboardArchitectureModelScript)
b.WriteString("\n")
b.WriteString(workspaceDashboardScript)
```

Replace the dashboard-local fallback and delete the old fixed `architectureLayout`:

```javascript
function serviceDomain(node){return architectureDomainKey(node);}
```

- [ ] **Step 4: Render lane backgrounds and accessible metadata-derived headers**

Add:

```javascript
function architectureLaneHTML(domain,index,layout,active,dim){
  const x=layout.margin+index*layout.step-18,y=118,width=layout.cardWidth+36,height=layout.height-150;
  return '<g class="architecture-domain-lane'+(active?' selected':'')+(dim?' dim':'')+'" data-architecture-domain="'+escapeAttr(domain.id)+'" tabindex="0" role="button" aria-pressed="'+String(active)+'" aria-label="Focus '+escapeAttr(domain.label)+' domain"><rect x="'+x+'" y="'+y+'" width="'+width+'" height="'+height+'" style="fill:'+domain.color.fill+';stroke:'+domain.color.stroke+'"></rect><text class="domain-title" x="'+(x+width/2)+'" y="150" text-anchor="middle">'+escapeHtml(domain.label)+'</text></g>';
}
```

Build `<g id="architecture-lane-layer">` before edge/node/label layers and add:

```css
.architecture-domain-lane rect{stroke-width:1;rx:14;opacity:.72}
.architecture-domain-lane.selected rect{stroke-width:2.5;opacity:1}
.architecture-domain-lane.dim{opacity:.28}
.architecture-domain-lane:focus-visible{outline:none}
.architecture-domain-lane:focus-visible rect{stroke:var(--color-focus);stroke-width:3}
```

- [ ] **Step 5: Update fixed-domain assertions and verify GREEN**

Require `architecture-lane-layer`, `architectureDomains`, `architectureDomainColor`, and `layout.domains`. Reject:

```go
for _, unwanted := range []string{
	`const domains=["frontend","document","cadaster","identity","platform"]`,
	`frontend:"Frontend clients"`, `document:"Documents / WPO"`,
} {
	if strings.Contains(html, unwanted) { t.Fatalf("architecture remains workspace-specific: %q", unwanted) }
}
```

Run:

```bash
gofmt -w internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_template.go
go test ./internal/scan -run 'TestArchitectureModelDerivesDynamicDomainsAndStablePositions|TestWorkspaceDashboardUsesMockupArchitectureViews' -count=1
```

Expected: PASS; every metadata domain and service is represented deterministically.

- [ ] **Step 6: Commit the dynamic-domain foundation**

```bash
git add internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "Derive architecture lanes from service metadata" -m "- Build deterministic domain groups and stable service positions" -m "- Render neutral accessible lanes without workspace-specific labels" -m "- Execute the production architecture model in focused Node tests"
```

### Task 2: Add Domain Focus Without Re-layout

**Files:**
- Modify: `internal/scan/workspace_dashboard_architecture.go`
- Modify: `internal/scan/workspace_dashboard_architecture_test.go`
- Modify: `internal/scan/workspace_dashboard_template.go:52-76`
- Modify: `internal/scan/workspace_dashboard_script.go:25,76,175-186,201-209`
- Modify: `internal/scan/workspace_dashboard_styles.go:9-15,24-35`
- Modify: `internal/scan/workspace_dashboard_test.go:96-118,808-887,1055-1075`

**Interfaces:**
- Consumes: Task 1 `architectureDomains`, `architectureLayout`, and metadata-only `serviceDomain`.
- Produces: `architectureFocusModel(nodes,edges,options) {nodeIDs:Set<string>,edgeIDs:Set<string>}` for `{selected,domain,direction,riskOnly}`.
- Produces: state fields `architectureDomain`, `architectureDirection`, `architectureRiskOnly`, `savedArchitectureDomainViewport`, and `savedArchitectureServiceViewport`.
- Produces: `setArchitectureDomain`, `setArchitectureDirection`, `resetArchitectureFocus`, `wireArchitectureDomainControls`, `renderArchitectureDomainControls`, and `fitArchitectureNeighborhoodIfNeeded(nodeIDs)`.

- [ ] **Step 1: Write failing executable focus-model tests**

```go
func TestArchitectureFocusModelKeepsDomainNeighborsAndAllServiceRelations(t *testing.T) {
	var result struct { DomainNodes, DomainEdges, ServiceNodes, ServiceEdges []string }
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"web",domain:"experience"},{id:"orders",domain:"commerce"},{id:"billing",domain:"commerce"},{id:"audit",domain:"observability"}];
		const edges=[{id:"web-orders",from:"web",to:"orders"},{id:"orders-billing",from:"orders",to:"billing"},{id:"orders-audit",from:"orders",to:"audit"},{id:"audit-billing",from:"audit",to:"billing"}];
		const domain=architectureFocusModel(nodes,edges,{domain:"commerce",direction:"outgoing",riskOnly:false});
		const service=architectureFocusModel(nodes,edges,{selected:"orders",direction:"both",riskOnly:false});
		return {DomainNodes:Array.from(domain.nodeIDs).sort(),DomainEdges:Array.from(domain.edgeIDs).sort(),ServiceNodes:Array.from(service.nodeIDs).sort(),ServiceEdges:Array.from(service.edgeIDs).sort()};
	})()`, &result)
	if strings.Join(result.DomainNodes, ",") != "audit,billing,orders" || strings.Join(result.DomainEdges, ",") != "orders-audit" { t.Fatalf("domain focus = %#v", result) }
	if strings.Join(result.ServiceEdges, ",") != "orders-audit,orders-billing,web-orders" || strings.Join(result.ServiceNodes, ",") != "audit,billing,orders,web" { t.Fatalf("service focus = %#v", result) }
}

func TestArchitectureRiskFocusChangesEmphasisNotLayout(t *testing.T) {
	var result struct { PositionCount int; FocusedEdges []string }
	runArchitectureModel(t, `(()=>{const nodes=[{id:"web",domain:"experience"},{id:"orders",domain:"commerce"},{id:"audit",domain:"observability"}],edges=[{id:"web-orders",from:"web",to:"orders",resolved:4},{id:"orders-audit",from:"orders",to:"audit",unresolved:1,risk:"has_unresolved"}],layout=architectureLayout(nodes,1440),focus=architectureFocusModel(nodes,edges,{selected:"orders",direction:"both",riskOnly:true});return {PositionCount:layout.positions.size,FocusedEdges:Array.from(focus.edgeIDs).sort()};})()`, &result)
	if result.PositionCount != 3 || strings.Join(result.FocusedEdges, ",") != "orders-audit" { t.Fatalf("risk focus = %#v", result) }
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `go test ./internal/scan -run 'TestArchitectureFocusModel|TestArchitectureRiskFocus' -count=1`

Expected: Node reports `ReferenceError: architectureFocusModel is not defined`.

- [ ] **Step 3: Implement the pure focus model**

Append inside the model raw string:

```javascript
function architectureEdgeRisk(edge){return !!(edge&&(edge.mismatched||edge.unresolved||String(edge.risk||"").match(/risk|mismatch|unresolved|missing/i)));}
function architectureDirectionMatches(edge,selected,domain,direction,nodeByID){
  if(selected){if(direction==="incoming")return edge.to===selected;if(direction==="outgoing")return edge.from===selected;return edge.from===selected||edge.to===selected;}
  if(!domain)return true;
  const from=nodeByID.get(edge.from),to=nodeByID.get(edge.to),fromDomain=from&&architectureDomainKey(from),toDomain=to&&architectureDomainKey(to);
  if(direction==="incoming")return toDomain===domain&&fromDomain!==domain;
  if(direction==="both")return (fromDomain===domain)!==(toDomain===domain);
  return fromDomain===domain&&toDomain!==domain;
}
function architectureFocusModel(nodes,edges,options){
  options=options||{};const nodeByID=new Map((nodes||[]).map(function(node){return [node.id,node];})),nodeIDs=new Set(),edgeIDs=new Set();
  if(options.selected)nodeIDs.add(options.selected);
  if(options.domain)(nodes||[]).forEach(function(node){if(architectureDomainKey(node)===options.domain)nodeIDs.add(node.id);});
  (edges||[]).forEach(function(edge){if(!architectureDirectionMatches(edge,options.selected,options.domain,options.direction||"both",nodeByID))return;if(options.riskOnly&&!architectureEdgeRisk(edge))return;edgeIDs.add(edge.id);nodeIDs.add(edge.from);nodeIDs.add(edge.to);});
  return {nodeIDs:nodeIDs,edgeIDs:edgeIDs};
}
```

- [ ] **Step 4: Add the persistent domain control shell and state transitions**

Insert before the SVG:

```html
<section id="architecture-focus-panel" class="architecture-focus-panel" aria-label="Architecture focus" aria-live="polite">
<div class="architecture-domain-controls"><strong>Domains</strong><div id="architecture-domain-chips" class="architecture-domain-chips"></div></div>
<div class="architecture-direction-controls" role="group" aria-label="Relationship direction">
<button type="button" data-architecture-direction="outgoing" aria-pressed="false">Outgoing</button>
<button type="button" data-architecture-direction="incoming" aria-pressed="false">Incoming</button>
<button type="button" data-architecture-direction="both" aria-pressed="true">Both</button>
</div>
<button type="button" id="architecture-risk-toggle" aria-pressed="false">Risk</button>
<button type="button" id="reset-architecture-focus">Reset focus</button>
</section>
```

Extend state with `architectureDomain:null,architectureDirection:"both",architectureRiskOnly:false,savedArchitectureDomainViewport:null,savedArchitectureServiceViewport:null`. Implement:

```javascript
function renderArchitectureDomainControls(domains){const active=state.architectureDomain;document.getElementById("architecture-domain-chips").innerHTML='<button type="button" data-architecture-domain="all" aria-pressed="'+String(!active)+'" class="'+(!active?'active':'')+'">All</button>'+domains.map(function(domain){const selected=domain.id===active;return '<button type="button" data-architecture-domain="'+escapeAttr(domain.id)+'" aria-pressed="'+String(selected)+'" class="'+(selected?'active':'')+'">'+escapeHtml(domain.label)+'</button>';}).join("");}
function setArchitectureDomain(domain){
  const next=domain&&domain!=="all"?domain:null;domain=next===state.architectureDomain?null:next;
  if(state.savedArchitectureServiceViewport){applyViewport(state.savedArchitectureServiceViewport);state.savedArchitectureServiceViewport=null;}
  if(domain&&!state.architectureDomain)state.savedArchitectureDomainViewport={zoom:state.zoom,panX:state.panX,panY:state.panY};
  state.architectureDomain=domain;state.architectureDirection=domain?"outgoing":"both";state.architectureRiskOnly=false;state.selected=null;state.selections.architecture=null;
  if(!domain&&state.savedArchitectureDomainViewport){applyViewport(state.savedArchitectureDomainViewport);state.savedArchitectureDomainViewport=null;}
  renderList();renderCanvas();
}
function setArchitectureDirection(direction){state.architectureDirection=["incoming","outgoing","both"].includes(direction)?direction:"both";renderCanvas();}
function restoreArchitectureServiceViewport(){if(!state.savedArchitectureServiceViewport)return;applyViewport(state.savedArchitectureServiceViewport);state.savedArchitectureServiceViewport=null;}
function resetArchitectureFocus(){const savedDomain=state.savedArchitectureDomainViewport;restoreArchitectureServiceViewport();state.selected=null;state.selections.architecture=null;state.selectedArchitectureEdge=null;state.architectureDomain=null;state.architectureDirection="both";state.architectureRiskOnly=false;state.savedArchitectureDomainViewport=null;if(savedDomain)applyViewport(savedDomain);clearDetailsForMode("architecture");renderList();renderCanvas();}
function wireArchitectureDomainControls(){document.querySelectorAll("[data-architecture-domain]").forEach(function(element){const activate=function(event){event.preventDefault();event.stopPropagation();setArchitectureDomain(element.dataset.architectureDomain);};element.addEventListener("pointerdown",function(event){event.stopPropagation();});element.addEventListener("click",activate);element.addEventListener("keydown",function(event){if(event.key==="Enter"||event.key===" ")activate(event);});});}
```

Wire direction/risk/reset buttons once. Header and chip both call `setArchitectureDomain`, ensuring identical state.

- [ ] **Step 5: Apply focus only as dimming and preserve service refinement**

At the start of `renderArchitectureMap`, use:

```javascript
const allNodes=serviceNodes.slice(),layout=architectureLayout(allNodes,width),allEdges=filteredServiceEdges().filter(function(edge){return layout.positions.has(edge.from)&&layout.positions.has(edge.to);});
const focus=architectureFocusModel(allNodes,allEdges,{selected:state.selected,domain:state.architectureDomain,direction:state.architectureDirection,riskOnly:state.architectureRiskOnly});
const focused=!!(state.selected||state.architectureDomain||state.architectureRiskOnly);
```

Render every node/lane/edge, adding `dim` outside the focus sets. `selectItem` sets direction to `both` for a selected Architecture service. `clearSelection` clears only service/edge selection and retains `architectureDomain`, returning to domain context. Call `renderArchitectureDomainControls(layout.domains)` and `wireArchitectureDomainControls()` after installing lane HTML.

After the first render of a newly selected service, fit only when its complete direct neighborhood lies outside the current SVG viewport; do not change `layout.positions`:

```javascript
function fitArchitectureNeighborhoodIfNeeded(nodeIDs){
  const svg=document.getElementById("workspace-graph"),viewBox=svg.viewBox.baseVal,positions=Array.from(nodeIDs||[]).map(function(id){return state.positions.get(id);}).filter(Boolean);
  if(!positions.length)return;
  const left=Math.min.apply(null,positions.map(function(p){return p.x;})),top=Math.min.apply(null,positions.map(function(p){return p.y;})),right=Math.max.apply(null,positions.map(function(p){return p.x+p.w;})),bottom=Math.max.apply(null,positions.map(function(p){return p.y+p.h;})),margin=56;
  const visible={left:(viewBox.x-state.panX)/state.zoom,top:(viewBox.y-state.panY)/state.zoom,right:(viewBox.x+viewBox.width-state.panX)/state.zoom,bottom:(viewBox.y+viewBox.height-state.panY)/state.zoom};
  if(left>=visible.left+margin&&top>=visible.top+margin&&right<=visible.right-margin&&bottom<=visible.bottom-margin)return;
  if(!state.savedArchitectureServiceViewport)state.savedArchitectureServiceViewport={zoom:state.zoom,panX:state.panX,panY:state.panY};
  const width=right-left+margin*2,height=bottom-top+margin*2;state.zoom=Math.max(.35,Math.min(3,Math.min(viewBox.width/width,viewBox.height/height)));state.panX=viewBox.x+viewBox.width/2-(left+right)/2*state.zoom;state.panY=viewBox.y+viewBox.height/2-(top+bottom)/2*state.zoom;applyTransform();
}
```

`clearSelection` calls `restoreArchitectureServiceViewport()` before clearing the service; `resetArchitectureFocus` already does so in the exact implementation above. This satisfies the complete-neighborhood fit requirement while preserving every service coordinate and the prior full-map viewport.

```css
.architecture-domain-chips,.architecture-direction-controls{display:flex;flex-wrap:wrap;gap:6px}
.architecture-domain-chips button.active,.architecture-direction-controls button.active{background:var(--color-accent);border-color:var(--color-accent);color:#fff;box-shadow:inset 0 -3px 0 rgba(0,0,0,.18)}
.architecture-domain-lane.selected .domain-title{font-weight:800;text-decoration:underline}
.edge.dim{opacity:.07}
```

- [ ] **Step 6: Verify focus, keyboard, and viewport restoration**

Update stale tests to require `architectureFocusModel`, both saved viewport fields, `fitArchitectureNeighborhoodIfNeeded`, shared `data-architecture-domain`, `aria-pressed`, Enter/Space, and `applyViewport(saved)`. Reject:

```go
if strings.Contains(html, `nodes=focusedMode?allNodes.filter`) { t.Fatal("architecture focus must dim instead of filtering nodes") }
```

Run:

```bash
gofmt -w internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_template.go
go test ./internal/scan -run 'TestArchitectureFocusModel|TestArchitectureRiskFocus|TestDashboardArchitectureSelection|TestDashboardGraphSelectionSupportsKeyboard|TestDashboardControlStateUsesARIA' -count=1
```

Expected: PASS; model tests prove direction, risk emphasis, complete adjacency, and unchanged position count.

- [ ] **Step 7: Commit accessible domain focus**

```bash
git add internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "Add stable architecture domain focus" -m "- Keep every service coordinate while dimming unrelated context" -m "- Share domain state between accessible headers and filter chips" -m "- Restore the exact full-map viewport when domain focus resets"
```

### Task 3: Route Background Trunks and Fan Every Selected Relationship to Card Ports

**Files:**
- Modify: `internal/scan/workspace_dashboard_architecture.go`
- Modify: `internal/scan/workspace_dashboard_architecture_test.go`
- Modify: `internal/scan/workspace_dashboard_script.go:73-91`
- Modify: `internal/scan/workspace_dashboard_styles.go:9-15`
- Modify: `internal/scan/workspace_dashboard_test.go:120-153,983-1040`

**Interfaces:**
- Consumes: Task 1 stable `architectureLayout` and Task 2 `architectureFocusModel`.
- Produces: `architectureBundles(edges,nodeByID) Array<{id,fromDomain,toDomain,risk,total,edges}>`, sorted by stable bundle ID.
- Produces: `architectureBundleGeometry(bundle,layout,index) {trunkPath,branches,badge}`; each branch contains `{edge,sourcePath,targetPath,target}`.
- Produces: existing `edgePortPoints`, `architecturePortOffset`, and `architecturePortPath` for unbundled selected direct relationships.

- [ ] **Step 1: Write failing grouping and selected-neighborhood tests**

```go
func TestArchitectureBundlesAreDeterministicAndRetainParallelRelationships(t *testing.T) {
	var result []struct { ID string; Total int; EdgeIDs []string }
	runArchitectureModel(t, `(()=>{
		const nodes=[{id:"web",domain:"experience"},{id:"admin",domain:"experience"},{id:"orders",domain:"commerce"},{id:"billing",domain:"commerce"}],byID=new Map(nodes.map(n=>[n.id,n]));
		const edges=[{id:"b",from:"admin",to:"billing",total:2,resolved:2},{id:"a",from:"web",to:"orders",total:4,resolved:3,unresolved:1,risk:"has_unresolved"},{id:"c",from:"admin",to:"orders",total:1,resolved:1}];
		return architectureBundles(edges,byID).map(bundle=>({ID:bundle.id,Total:bundle.total,EdgeIDs:bundle.edges.map(edge=>edge.id)}));
	})()`, &result)
	if len(result) != 2 { t.Fatalf("bundles = %#v, want resolved and unresolved groups", result) }
	if result[0].ID > result[1].ID { t.Fatalf("bundles not sorted: %#v", result) }
}

func TestWorkspaceDashboardKeepsAllSelectedServiceRelationsUnbundled(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(), WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{
		`const directEdges=allEdges.filter(function(edge){return focus.edgeIDs.has(edge.id)&&state.selected&&(edge.from===state.selected||edge.to===state.selected);})`,
		`const backgroundEdges=allEdges.filter(function(edge){return !directEdgeIDs.has(edge.id);})`,
		`architecturePortOffset(directEdges,edge,edge.from)`, `architecturePortOffset(directEdges,edge,edge.to)`, `marker-end="url(#arrow-`,
	} {
		if !strings.Contains(html, want) { t.Fatalf("dashboard missing direct relationship contract %q", want) }
	}
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `go test ./internal/scan -run 'TestArchitectureBundlesAreDeterministic|TestWorkspaceDashboardKeepsAllSelectedServiceRelationsUnbundled' -count=1`

Expected: Node reports that shared-model `architectureBundles` is undefined; the renderer lacks the new `directEdges` partition.

- [ ] **Step 3: Move deterministic grouping into the shared model**

Append inside `workspaceDashboardArchitectureModelScript` and remove the old global-dependent `architectureBundles`:

```javascript
function architectureBundleRisk(edge){return edge.mismatched?"mismatch":edge.unresolved?"unresolved":"resolved";}
function architectureBundles(edges,nodeByID){
  const bundles=new Map();
  (edges||[]).slice().sort(function(a,b){return String(a.id).localeCompare(String(b.id));}).forEach(function(edge){
    const from=nodeByID.get(edge.from),to=nodeByID.get(edge.to);if(!from||!to)return;
    const fromDomain=architectureDomainKey(from),toDomain=architectureDomainKey(to),risk=architectureBundleRisk(edge),key=[fromDomain,toDomain,risk].join("|");
    if(!bundles.has(key))bundles.set(key,{id:"bundle:"+key,fromDomain:fromDomain,toDomain:toDomain,risk:risk,total:0,edges:[]});
    const bundle=bundles.get(key);bundle.total+=edge.total||1;bundle.edges.push(edge);
  });
  return Array.from(bundles.values()).sort(function(a,b){return a.id.localeCompare(b.id);});
}
```

Bundle badges store their exact member IDs in `data-architecture-edge-ids`, sorted by edge ID. This avoids re-resolving a bundle from labels or counts.

- [ ] **Step 4: Replace average curves with shared trunks and card-safe branches**

Replace `architectureBundleModel` with:

```javascript
function architectureBundleGeometry(bundle,layout,index){
  const records=bundle.edges.map(function(edge){return {edge:edge,from:layout.positions.get(edge.from),to:layout.positions.get(edge.to)};}).filter(function(record){return record.from&&record.to;});
  if(!records.length)return null;
  const forward=records.reduce(function(total,record){return total+(record.to.x-record.from.x);},0)>=0,sourceLane=records[0].from.lane,targetLane=records[0].to.lane,gutter=Math.max(72,layout.step-layout.cardWidth);
  if(sourceLane===targetLane){
    const railX=layout.margin+sourceLane*layout.step+layout.cardWidth+gutter*.34,branches=records.map(function(record){const source={x:record.from.x+record.from.w,y:record.from.y+record.from.h/2},target={x:record.to.x+record.to.w,y:record.to.y+record.to.h/2};return {edge:record.edge,sourcePath:"M"+source.x+" "+source.y+" C"+railX+" "+source.y+" "+railX+" "+source.y+" "+railX+" "+source.y,targetPath:"M"+railX+" "+target.y+" C"+railX+" "+target.y+" "+target.x+" "+target.y+" "+target.x+" "+target.y,target:target};}),branchYs=branches.flatMap(function(branch){return [layout.positions.get(branch.edge.from).y+layout.cardHeight/2,layout.positions.get(branch.edge.to).y+layout.cardHeight/2];}).sort(function(a,b){return a-b;});
    return {trunkPath:"M"+railX+" "+branchYs[0]+" L"+railX+" "+branchYs[branchYs.length-1],branches:branches,badge:{x:railX,y:branchYs[Math.floor(branchYs.length/2)]}};
  }
  const sourceTrunkX=forward?layout.margin+sourceLane*layout.step+layout.cardWidth+gutter*.34:layout.margin+sourceLane*layout.step-gutter*.34;
  const targetTrunkX=forward?layout.margin+targetLane*layout.step-gutter*.34:layout.margin+targetLane*layout.step+layout.cardWidth+gutter*.34;
  const ys=records.flatMap(function(record){return [record.from.y+record.from.h/2,record.to.y+record.to.h/2];}).sort(function(a,b){return a-b;}),trunkY=ys[Math.floor(ys.length/2)]+index%3*10;
  const branches=records.map(function(record){const source={x:forward?record.from.x+record.from.w:record.from.x,y:record.from.y+record.from.h/2},target={x:forward?record.to.x:record.to.x+record.to.w,y:record.to.y+record.to.h/2};return {edge:record.edge,sourcePath:"M"+source.x+" "+source.y+" C"+sourceTrunkX+" "+source.y+" "+sourceTrunkX+" "+trunkY+" "+sourceTrunkX+" "+trunkY,targetPath:"M"+targetTrunkX+" "+trunkY+" C"+targetTrunkX+" "+target.y+" "+target.x+" "+target.y+" "+target.x+" "+target.y,target:target};});
  return {trunkPath:"M"+sourceTrunkX+" "+trunkY+" L"+targetTrunkX+" "+trunkY,branches:branches,badge:{x:(sourceTrunkX+targetTrunkX)/2,y:trunkY}};
}
```

Render in this order: lane layer; background source branches/shared trunks/target branches; selected direct edges; opaque cards; ports and badges. Only target branches carry `marker-end="url(#arrow)"`. Selected direct edges carry incoming/outgoing markers and are excluded from background bundles.

- [ ] **Step 5: Partition direct and background edges without filtering nodes**

Use exactly:

```javascript
const directEdges=allEdges.filter(function(edge){return focus.edgeIDs.has(edge.id)&&state.selected&&(edge.from===state.selected||edge.to===state.selected);});
const directEdgeIDs=new Set(directEdges.map(function(edge){return edge.id;}));
const backgroundEdges=allEdges.filter(function(edge){return !directEdgeIDs.has(edge.id);});
const nodeByID=new Map(allNodes.map(function(node){return [node.id,node];}));
const bundleModels=architectureBundles(backgroundEdges,nodeByID).map(function(bundle,index){return {bundle:bundle,geometry:architectureBundleGeometry(bundle,layout,index)};}).filter(function(item){return !!item.geometry;});
```

Add `dim` to a background bundle only when none of its edges is in `focus.edgeIDs`; add `dim` to a card only when focus is active and its ID is outside `focus.nodeIDs`. Never call `centerOnPosition` after selection.

- [ ] **Step 6: Verify arrows, ports, layer order, and obsolete-copy removal**

Run:

```bash
gofmt -w internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go
go test ./internal/scan -run 'TestArchitectureBundles|TestWorkspaceDashboardKeepsAllSelectedServiceRelationsUnbundled|TestDashboardArchitectureShowsDirectionalArrowsAndExplicitCardPorts|TestWorkspaceDashboardBundlesBackgroundArchitectureEdges|TestWorkspaceDashboardKeepsArchitectureExplanationVisible' -count=1
```

Expected: PASS. Tests reject `Caller`, `Called`, `OUT`, `direction-badge`, selected-service node filtering, and duplicate selected-edge rendering.

- [ ] **Step 7: Commit deterministic trunk routing**

```bash
git add internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "Bundle architecture relationships through shared trunks" -m "- Route background branches through deterministic inter-domain gutters" -m "- Fan every selected incoming and outgoing edge to explicit card ports" -m "- Preserve stable card positions and directional arrowheads"
```

### Task 4: Add the Relationship Summary, Static-Call Badges, Tooltip, and Inspector Details

**Files:**
- Modify: `internal/scan/workspace_dashboard_architecture.go`
- Modify: `internal/scan/workspace_dashboard_architecture_test.go`
- Modify: `internal/scan/workspace_dashboard_template.go:52-76`
- Modify: `internal/scan/workspace_dashboard_script.go:78-91,164-183,201-209`
- Modify: `internal/scan/workspace_dashboard_styles.go:4-15,24-35`
- Modify: `internal/scan/workspace_dashboard_test.go:584-604,983-1040`

**Interfaces:**
- Consumes: Task 2 focus state/model and Task 3 geometry.
- Produces: `architectureRelationshipSummary(selected,edges) {incomingRelationships,incomingServices,outgoingRelationships,outgoingServices,resolved,unresolved,mismatched}`.
- Produces: `renderArchitectureSummary`, `architectureRelationshipBadge`, `architectureBundleBadge`, `wireArchitectureRelationshipBadges`, `showArchitectureRelationshipDetails`, `showArchitectureTooltip`, and `hideArchitectureTooltip`.
- Produces: persistent `#architecture-relationship-summary` and `#architecture-relationship-tooltip` outside `#graph-layer`.

- [ ] **Step 1: Write failing exact-count and renderer-contract tests**

```go
func TestArchitectureRelationshipSummaryCountsRelationsServicesAndRiskBuckets(t *testing.T) {
	var result struct { IncomingRelationships, IncomingServices, OutgoingRelationships, OutgoingServices, Resolved, Unresolved, Mismatched int }
	runArchitectureModel(t, `architectureRelationshipSummary("orders",[
		{id:"web-orders",from:"web",to:"orders",total:4,resolved:3,unresolved:1},
		{id:"worker-orders",from:"worker",to:"orders",total:1,resolved:1},
		{id:"orders-billing",from:"orders",to:"billing",total:2,resolved:1,mismatched:1},
		{id:"orders-audit",from:"orders",to:"audit",total:1,resolved:1}
	])`, &result)
	if result.IncomingRelationships != 5 || result.IncomingServices != 2 || result.OutgoingRelationships != 3 || result.OutgoingServices != 2 { t.Fatalf("counts = %#v", result) }
	if result.Resolved != 6 || result.Unresolved != 1 || result.Mismatched != 1 { t.Fatalf("buckets = %#v", result) }
}

func TestWorkspaceDashboardExposesPersistentSummaryAndInspectableStaticCallBadges(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(), WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	for _, want := range []string{`id="architecture-relationship-summary"`, `aria-live="polite"`, `id="architecture-relationship-tooltip"`, `role="tooltip"`, `data-architecture-edge`, `data-architecture-bundle`, `tabindex="0"`, `not runtime request frequency`, `showArchitectureRelationshipDetails`, `Reset focus`} {
		if !strings.Contains(html, want) { t.Fatalf("dashboard missing summary contract %q", want) }
	}
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `go test ./internal/scan -run 'TestArchitectureRelationshipSummary|TestWorkspaceDashboardExposesPersistentSummary' -count=1`

Expected: model reports undefined `architectureRelationshipSummary`; HTML lacks summary and tooltip IDs.

- [ ] **Step 3: Implement selected-service counts**

Append inside the model:

```javascript
function architectureRelationshipSummary(selected,edges){
  const incoming=(edges||[]).filter(function(edge){return edge.to===selected;}),outgoing=(edges||[]).filter(function(edge){return edge.from===selected;}),all=incoming.concat(outgoing),sum=function(records,key){return records.reduce(function(total,edge){return total+(edge[key]||0);},0);};
  return {incomingRelationships:sum(incoming,"total"),incomingServices:new Set(incoming.map(function(edge){return edge.from;})).size,outgoingRelationships:sum(outgoing,"total"),outgoingServices:new Set(outgoing.map(function(edge){return edge.to;})).size,resolved:sum(all,"resolved"),unresolved:sum(all,"unresolved"),mismatched:sum(all,"mismatched")};
}
```

- [ ] **Step 4: Add persistent summary markup and rendering**

Inside `#architecture-focus-panel`, add:

```html
<div id="architecture-relationship-summary" class="architecture-relationship-summary" aria-live="polite" hidden>
<strong id="architecture-summary-service">No service selected</strong>
<span><b id="architecture-summary-incoming">0</b> incoming</span>
<span><b id="architecture-summary-outgoing">0</b> outgoing</span>
<span><b id="architecture-summary-resolution">0 resolved · 0 unresolved · 0 mismatch</b></span>
</div>
<div id="architecture-relationship-tooltip" class="architecture-relationship-tooltip" role="tooltip" hidden></div>
```

Implement:

```javascript
function syncArchitectureFocusControls(){document.querySelectorAll("[data-architecture-direction]").forEach(function(button){const active=button.dataset.architectureDirection===state.architectureDirection;button.classList.toggle("active",active);button.setAttribute("aria-pressed",String(active));});const risk=document.getElementById("architecture-risk-toggle");risk.classList.toggle("active",state.architectureRiskOnly);risk.setAttribute("aria-pressed",String(state.architectureRiskOnly));}
function renderArchitectureSummary(){
  const summary=document.getElementById("architecture-relationship-summary"),node=serviceById.get(state.selected);summary.hidden=!node;if(!node){syncArchitectureFocusControls();return;}
  const counts=architectureRelationshipSummary(node.id,serviceEdges);document.getElementById("architecture-summary-service").textContent=node.label||node.project;
  document.getElementById("architecture-summary-incoming").textContent=counts.incomingRelationships+" relationships · "+counts.incomingServices+" caller services";
  document.getElementById("architecture-summary-outgoing").textContent=counts.outgoingRelationships+" relationships · "+counts.outgoingServices+" target services";
  document.getElementById("architecture-summary-resolution").textContent=counts.resolved+" resolved · "+counts.unresolved+" unresolved · "+counts.mismatched+" mismatch";syncArchitectureFocusControls();
}
```

Call it after every Architecture render and state transition. It remains outside the SVG transform.

- [ ] **Step 5: Make every `N calls` badge keyboard-accessible and inspectable**

The direct badge must include:

```javascript
function architectureRelationshipBadge(edge,model,layout){
  const from=serviceById.get(edge.from),to=serviceById.get(edge.to),count=edge.total||1,fromLabel=from?(from.label||from.project):edge.from,toLabel=to?(to.label||to.project):edge.to;
  const tooltip=count+" statically detected call relationship"+(count===1?"":"s")+" between "+fromLabel+" and "+toLabel+"; this is not runtime request frequency.",position=architectureCallBadgePosition(edge,model,layout,state.selected),label=count+" call"+(count===1?"":"s");
  return '<g class="architecture-call-pill '+model.direction+' '+routeStatusClass(edge.risk)+'" transform="translate('+position.x+' '+position.y+')" data-architecture-edge="'+escapeAttr(edge.id)+'" data-tooltip="'+escapeAttr(tooltip)+'" aria-label="'+escapeAttr(label+". "+tooltip)+'" aria-describedby="architecture-relationship-tooltip" tabindex="0" role="button"><rect x="-36" y="-12" width="72" height="24"></rect><text x="0" y="4">'+escapeHtml(label)+'</text></g>';
}
```

Bundle badges receive `data-architecture-bundle` and identical static-analysis wording. Wire hover/focus and click/Enter/Space:

```javascript
function showArchitectureTooltip(element){const tooltip=document.getElementById("architecture-relationship-tooltip"),rect=element.getBoundingClientRect();tooltip.textContent=element.dataset.tooltip||"";tooltip.hidden=false;tooltip.style.left=Math.round(rect.left+rect.width/2)+"px";tooltip.style.top=Math.round(rect.bottom+8)+"px";}
function hideArchitectureTooltip(){document.getElementById("architecture-relationship-tooltip").hidden=true;}
function wireArchitectureRelationshipBadges(){document.querySelectorAll("[data-architecture-edge],[data-architecture-bundle]").forEach(function(element){element.addEventListener("pointerdown",function(event){event.stopPropagation();});element.addEventListener("pointerenter",function(){showArchitectureTooltip(element);});element.addEventListener("pointerleave",hideArchitectureTooltip);element.addEventListener("focus",function(){showArchitectureTooltip(element);});element.addEventListener("blur",hideArchitectureTooltip);const activate=function(event){event.preventDefault();event.stopPropagation();const ids=new Set(element.dataset.architectureEdge?[element.dataset.architectureEdge]:String(element.dataset.architectureEdgeIds||"").split(",").filter(Boolean)),edges=serviceEdges.filter(function(edge){return ids.has(edge.id);});showArchitectureRelationshipDetails(edges,element.textContent.trim());};element.addEventListener("click",activate);element.addEventListener("keydown",function(event){if(event.key==="Enter"||event.key===" ")activate(event);});});}
```

`showArchitectureRelationshipDetails` renders direction, totals, resolution buckets, endpoints, problems, and evidence in `#details`. It changes only `selectedArchitectureEdge`; it never changes zoom, pan, positions, or selected service.

- [ ] **Step 6: Style the persistent ribbon and accessible tooltip**

```css
.architecture-focus-panel{display:none;position:absolute;z-index:4;top:96px;left:12px;right:12px;align-items:center;gap:8px;min-height:44px;border:1px solid var(--color-border);border-radius:6px;background:rgba(255,255,255,.97);padding:7px 9px;color:var(--color-text)}
main[data-active-view="architecture"].graph-view .architecture-focus-panel{display:flex;flex-wrap:wrap}
.architecture-relationship-summary{display:flex;align-items:center;flex:1 1 520px;gap:12px;min-width:0;border-left:1px solid var(--color-border);padding-left:12px;font-size:12px}
.architecture-relationship-summary[hidden],.architecture-relationship-tooltip[hidden]{display:none}
.architecture-relationship-tooltip{position:fixed;z-index:20;max-width:320px;transform:translateX(-50%);border:1px solid var(--color-border);border-radius:6px;background:#17212b;color:#fff;padding:8px 10px;font-size:12px;line-height:1.35;pointer-events:none}
.architecture-call-pill[role="button"]{cursor:pointer}
.architecture-call-pill[role="button"]:focus-visible rect,.bundle-count[role="button"]:focus-visible rect{stroke:var(--color-focus);stroke-width:3}
```

The ribbon uses an absolute HTML overlay because Architecture navigation pans and zooms the SVG rather than scrolling the document. Remaining outside `#graph-layer` gives it the required sticky behavior: it stays fixed directly below the controls throughout canvas navigation.

- [ ] **Step 7: Verify counts, tooltip, inspector, and viewport preservation**

Run:

```bash
gofmt -w internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_template.go
go test ./internal/scan -run 'TestArchitectureRelationshipSummary|TestWorkspaceDashboardExposesPersistentSummary|TestDashboardServiceRelationRowsRemainVisibleButStatic|TestDashboardGraphSelectionSupportsKeyboard|TestDashboardControlStateUsesARIA' -count=1
```

Expected: PASS. Negative assertions reject `OUT`, `Caller`, `Called`, `runtime calls`, and badge handlers that call layout/viewport functions.

- [ ] **Step 8: Commit summary and relationship inspection**

```bash
git add internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "Add architecture relationship summary and details" -m "- Keep selected-service counts and direction filters visible above the canvas" -m "- Open static relationship evidence from keyboard-accessible call badges" -m "- Explain that badge counts are not runtime request frequency"
```

### Task 5: Lock Responsive, Accessibility, Documentation, and Issue #23 Acceptance

**Files:**
- Modify: `internal/scan/workspace_dashboard_architecture_test.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_test.go`
- Modify: `docs/design-system.md`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs_test.go`

**Interfaces:**
- Consumes: all issue #23 Architecture functions and UI contracts from Tasks 1-4.
- Produces: one dense renderer acceptance test, embedded-JavaScript syntax validation, a documentation contract, and a three-viewport browser evidence checklist.
- Produces: no CLI command, schema field, release artifact, tag, or package-manager publication.

- [ ] **Step 1: Add failing combined acceptance and syntax tests**

Append to `workspace_dashboard_architecture_test.go`:

```go
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
		if !strings.Contains(html, want) { t.Fatalf("issue #23 acceptance missing %q", want) }
	}
	for _, obsolete := range []string{
		`const domains=["frontend","document","cadaster","identity","platform"]`,
		`nodes=focusedMode?allNodes.filter`, `>OUT<`, `>Caller<`, `>Called<`,
	} {
		if strings.Contains(html, obsolete) { t.Fatalf("issue #23 obsolete behavior remains: %q", obsolete) }
	}
}

func TestWorkspaceDashboardEmbeddedJavaScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil { t.Skip("node is required for embedded dashboard syntax validation") }
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(), WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil)
	start,end := strings.Index(html, "<script>\n"),strings.LastIndex(html, "\n</script>")
	if start < 0 || end <= start { t.Fatal("embedded dashboard script boundaries not found") }
	cmd := exec.Command(node, "--check", "-")
	cmd.Stdin = strings.NewReader(html[start+len("<script>\n") : end])
	if output, err := cmd.CombinedOutput(); err != nil { t.Fatalf("embedded dashboard JavaScript is invalid: %v\n%s", err, output) }
}
```

Extend `docs_test.go`:

```go
func TestArchitectureMapDocumentationMatchesIssue23(t *testing.T) {
	files := []string{"README.md", "COMMANDS.md", "docs/OUTPUTS.md", "docs/RELEASE.md", "docs/design-system.md"}
	var combined strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil { t.Fatal(err) }
		combined.Write(content)
		combined.WriteByte('\n')
	}
	text := strings.ToLower(combined.String())
	for _, want := range []string{"dynamic domain lanes", "incoming and outgoing", "statically detected", "not runtime", "1.3.0"} {
		if !strings.Contains(text, strings.ToLower(want)) { t.Fatalf("architecture documentation missing %q", want) }
	}
}
```

- [ ] **Step 2: Run combined acceptance and confirm documentation RED**

Run:

```bash
go test ./internal/scan -run 'TestWorkspaceDashboardCoversArchitectureMapIssue23Acceptance|TestWorkspaceDashboardEmbeddedJavaScriptParses' -count=1
go test . -run TestArchitectureMapDocumentationMatchesIssue23 -count=1
```

Expected: renderer and syntax tests pass after Tasks 1-4; the documentation test fails until every public file uses the new Architecture contract.

- [ ] **Step 3: Finalize responsive and accessibility styles**

Ensure `workspace_dashboard_styles.go` contains:

```css
button:focus-visible,input:focus-visible,.source-link:focus-visible,[data-select-id]:focus-visible,[data-architecture-domain]:focus-visible,[data-architecture-edge]:focus-visible,[data-architecture-bundle]:focus-visible{outline:3px solid var(--color-focus);outline-offset:2px}
@media (max-width:1240px){.architecture-focus-panel{position:absolute;top:96px;left:12px;right:12px}.architecture-focus-panel button{min-height:44px}.architecture-relationship-summary{flex-basis:100%;border-left:0;border-top:1px solid var(--color-border);padding:7px 0 0}}
@media (prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition-duration:0.01ms!important;animation-duration:0.01ms!important}}
```

At 1280×720 the ribbon occupies only the canvas header: lanes start at SVG y=118, domain labels at y=150, cards at y=190, and badge coordinates stay in gutters rather than card rectangles.

- [ ] **Step 4: Update documentation with exact issue #23 behavior**

Use this Architecture bullet in `README.md`, preserving all issue #25 table and Code Explorer edits already present:

```markdown
- **Architecture:** understand how projects and services communicate without losing the full workspace layout. Dynamic domain lanes come from service-map metadata. Selecting a service keeps every card at its stable position, highlights all direct incoming and outgoing relationships, and dims unrelated context. Background relationships share bundled trunks; selected relationships fan out to explicit card ports. The persistent summary shows relationship, neighboring-service, resolved, unresolved, and mismatch counts and filters by direction or risk. `N calls` means statically detected relationships, not runtime request frequency.
```

Update `COMMANDS.md` so the section says `The 1.3.0 dashboard is organized around six views` and uses the same Architecture semantics. Remove stale `OUT`, explicit-isolation-first, five-view, and 1.0.0 wording.

Add to the `workspace-map.html` paragraph in `docs/OUTPUTS.md`:

```markdown
Architecture derives dynamic domain lanes from `workspace-service-map.json`, keeps stable card coordinates during service/domain/direction/risk focus, groups background relationships through shared trunks, fans selected direct relationships to card ports, and keeps a persistent count/filter summary outside the SVG transform. Call badges describe statically detected relationships and are not runtime traffic metrics.
```

Add to the unreleased 1.3.0 list in `docs/RELEASE.md` without changing publication status:

```markdown
- dynamic metadata-derived Architecture domain lanes with stable service-card positions;
- simultaneous incoming/outgoing selected-service focus with unrelated context dimmed;
- deterministic background relationship trunks and direct card-port fan-out;
- a persistent relationship summary with domain, direction, risk, and reset controls;
- keyboard-accessible static-call badges whose details remain separate from runtime traffic claims.
```

Add to `docs/design-system.md`:

```markdown
- Architecture uses dynamic domain lanes from service-map metadata, never a workspace-specific label or palette table.
- Service, domain, direction, and risk focus preserve every card coordinate and dim unrelated context.
- Background relations share domain/service trunks; selected direct relations fan out near opaque white cards and terminate at visible ports.
- The persistent relationship summary stays outside the SVG transform and complements, but never replaces, the inspector.
- `N calls` always means statically detected relationships, never runtime frequency; its tooltip is available on focus and hover.
```

- [ ] **Step 5: Run focused, full, format, vet, syntax, and diff gates**

Run exactly:

```bash
gofmt -w internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_template.go docs_test.go
gofmt -l .
go test ./internal/scan -run 'TestArchitecture|TestDashboardArchitecture|TestWorkspaceDashboard' -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Expected: `gofmt -l .` prints nothing; tests and vet exit 0; the JavaScript syntax test executes rather than skips on the development machine; `git diff --check` prints nothing.

- [ ] **Step 6: Perform browser interaction and visual acceptance at all viewports**

Use a freshly generated dense dashboard. The final combined issue #23/#25 delivery cleans and rescans `~/projects/weka/` first, then opens:

```text
~/projects/weka/.goregraph-workspace/workspace-map.html
```

At 1280×720, 1440×900, and 1920×1080, automate this exact sequence:

1. Record every service card SVG `x` and `y`.
2. Activate a domain header with Enter; its chip has `aria-pressed="true"`, Outgoing is active, domain services plus outgoing external neighbors are undimmed, and coordinates are unchanged.
3. Activate Incoming and Both; direction-specific neighbors change only through dimming and coordinates remain unchanged.
4. Activate the matching chip; it produces the same state as the header.
5. Select a service inside the domain; every direct incoming and outgoing edge is simultaneously visible with arrowhead and ports.
6. Activate Risk; non-risk direct edges dim without movement.
7. Focus and hover an `N calls` badge; the visible tooltip says statically detected and not runtime request frequency.
8. Activate the badge with Space; the inspector exposes endpoint/problem/evidence rows and zoom/pan are unchanged.
9. Verify the persistent summary's service, relationship/service, resolved, unresolved, and mismatch totals against JSON.
10. Activate Reset focus; the saved full-map zoom/pan and unchanged coordinates return.
11. Tab through domain chips/headers, filters, reset, cards, and badges; each has a visible logical focus state.
12. Emulate `prefers-reduced-motion: reduce`; state remains understandable without animation.
13. Confirm no path, badge, label, lane, or ribbon covers service-card content.

Save temporary, uncommitted evidence:

```text
/private/tmp/goregraph-issue-23-1280x720.png
/private/tmp/goregraph-issue-23-1440x900.png
/private/tmp/goregraph-issue-23-1920x1080.png
```

Expected: all thirteen checks pass at all three viewports.

- [ ] **Step 7: Commit documentation and acceptance coverage**

```bash
git add internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go docs/design-system.md README.md COMMANDS.md docs/OUTPUTS.md docs/RELEASE.md docs_test.go
git commit -m "Document and verify the architecture map redesign" -m "- Cover dynamic lanes, stable focus, summaries, badges, and responsive behavior" -m "- Align dashboard, command, output, design-system, and release documentation" -m "- Keep the 1.3.0 target explicitly unreleased"
```

- [ ] **Step 8: Gate issue closure on combined delivery, installation, and Weka rescan**

Do not close issue #23 from this task in isolation. The parent delivery first:

1. completes and independently reviews issues #23 and #25 with no unresolved Critical or Important finding;
2. verifies `main` is pushed and equals `origin/main`;
3. installs current source and verifies `goregraph version` reports `1.3.0`;
4. previews and executes `goregraph workspace clean ~/projects/weka/ --execute`;
5. runs `goregraph workspace scan-all ~/projects/weka/` with the new binary;
6. runs Doctor/integrity validation on fresh outputs;
7. repeats the three-viewport browser checks against the new Weka dashboard;
8. verifies no tag, release, or package-manager publication exists.

Only after all eight conditions pass may the parent close issue #23 with a comment summarizing implementation and verification evidence.

---

## Self-Review Checklist

- [x] Spec coverage: dynamic domains, neutral lanes, default Outgoing domain focus, Incoming/Both, service refinement, simultaneous direct incoming/outgoing focus, stable coordinates, dimming, trunks, ports, arrowheads, summary, risk, reset, badge details, static-not-runtime tooltip, inspector continuity, keyboard, focus, reduced motion, and all viewports each map to a task and test. No gap found.
- [x] Issue #23 coverage: legend remains near controls; obsolete `OUT`/Caller/Called is rejected; badges stay outside cards; reset restores viewport; dense generic fixture and fresh Weka dashboard both receive visual verification. No gap found.
- [x] Scope isolation: no Code Explorer, symbol projection, Query, Explain, MCP, Doctor-symbol, extractor, or schema work is introduced; issue #25 edits are preserved.
- [x] Completeness scan: every step contains concrete code, a command, an expected result, or an exact acceptance action; no deferred implementation markers remain.
- [x] Type consistency: `architectureDomainKey`, `architectureDomains`, `architectureLayout`, `architectureFocusModel`, `architectureBundles`, and `architectureRelationshipSummary` keep identical signatures throughout.
- [x] Delivery safety: closure follows combined remote verification, local 1.3.0 installation, clean Weka rescan, integrity checks, and browser acceptance; publication remains forbidden.
