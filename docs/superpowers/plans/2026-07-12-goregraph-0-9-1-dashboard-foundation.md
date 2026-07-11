# GoreGraph 0.9.1 Dashboard Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Architecture the stable default dashboard, integrate explicit service-neighborhood isolation, merge endpoint inventory and trace navigation, rename Open Issues to explained Diagnostics, implement correct viewport controls, and document the dashboard for `0.9.1`.

**Architecture:** Preserve the existing service-map and endpoint-trace payloads while decomposing the generated dashboard into focused Go-owned template, style, and script assets. Keep Architecture selection non-destructive and spatially stable; implement isolation and centering only through explicit actions. Delay the canonical Evidence, Coverage, and directed data-flow models to their approved later releases.

**Tech Stack:** Go 1.23+, Go standard library, generated standalone HTML/CSS/JavaScript, Go unit tests, offline browser verification, real `~/projects/weka/` workspace acceptance.

## Global Constraints

- Version target is exactly `0.9.1`.
- Preserve Schema 1 and all existing generated JSON fields.
- Do not add a frontend build system, runtime dependency, CDN, remote font, or network asset.
- Architecture is the first navigation control and initial mode.
- Normal service selection keeps every Architecture node at the same position and highlights direct neighbors.
- Neighborhood isolation is explicit and reversible; it must not replace ordinary Architecture selection.
- Open Issues becomes Diagnostics; technical issue codes remain searchable but explanatory copy is primary.
- Endpoint Paths and Focused Service are removed as top-level views only after their useful behavior exists in Architecture and Endpoints.
- `Fit` must calculate a fit; it must not clear search, selection, or filters.
- Selection must never automatically center or reset the viewport.
- Keep the dashboard local, deterministic, offline, keyboard-operable, and reduced-motion aware.
- No Weka-specific labels or domains may be introduced.
- Do not push, tag, or publish.
- Final acceptance uses the freshly installed `0.9.1` binary and a fresh scan of `~/projects/weka/`.

---

## File Structure

- Create: `docs/design-system.md` — dashboard design tokens, interaction rules, and accessibility constraints.
- Modify: `internal/scan/workspace_dashboard.go` — payload marshaling and composition of the decomposed document only.
- Create: `internal/scan/workspace_dashboard_template.go` — semantic HTML shell and navigation markup.
- Create: `internal/scan/workspace_dashboard_styles.go` — offline CSS token and component stylesheet constant.
- Create: `internal/scan/workspace_dashboard_script.go` — dashboard state, filtering, rendering, details, and interaction JavaScript constant.
- Modify: `internal/scan/workspace_dashboard_test.go` — structural, copy, default-mode, focus, endpoint, diagnostics, and viewport regression tests.
- Modify: `README.md` — human task guide for Architecture, Endpoints, and Diagnostics plus dashboard controls.
- Modify: `docs/OUTPUTS.md` — update `workspace-map.html` behavior without changing output compatibility.
- Modify: `docs/RELEASE.md` — add `0.9.1` checks and local acceptance instructions.
- Modify: `internal/version/version.go` — set development version to `0.9.1`.
- Modify: `internal/cli/cli_test.go` — expect `0.9.1` version output.

## Task 1: Establish Dashboard Design Tokens And Responsibilities

**Files:**
- Create: `docs/design-system.md`
- Test: `docs_test.go`

**Interfaces:**
- Produces: documented token names consumed by `workspaceDashboardStyles` in Task 3.
- Produces: behavior rules used as acceptance criteria by later tasks.

- [ ] **Step 1: Add a failing documentation-contract test**

Append to `docs_test.go`:

```go
func TestDashboardDesignSystemDocumentsRequiredTokensAndBehavior(t *testing.T) {
	content, err := os.ReadFile("docs/design-system.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"--color-background",
		"--color-surface",
		"--color-text",
		"--color-muted",
		"--color-accent",
		"--color-focus",
		"--space-1",
		"--radius-control",
		"prefers-reduced-motion",
		"Selection does not relayout the Architecture view",
		"Fit preserves search, filters, and selection",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("design system missing %q", want)
		}
	}
}
```

If not already imported, add `strings` to the existing import block.

- [ ] **Step 2: Run the focused test and verify failure**

Run:

```bash
GOCACHE=/private/tmp/goregraph-gocache go test . -run TestDashboardDesignSystemDocumentsRequiredTokensAndBehavior -count=1
```

Expected: FAIL because `docs/design-system.md` does not exist.

- [ ] **Step 3: Create the design-system document**

Create `docs/design-system.md` with these concrete sections and values:

```markdown
# GoreGraph Dashboard Design System

## Product Character

Technical, calm, precise, dense enough for code navigation, and visually restrained. Decoration never competes with graph meaning.

## Tokens

- `--color-background: #f3f6f7`
- `--color-surface: #ffffff`
- `--color-canvas: #eef4f6`
- `--color-text: #17212b`
- `--color-muted: #5f6f7e`
- `--color-border: #d3dde4`
- `--color-accent: #0b6b79`
- `--color-focus: #0b6b79`
- `--color-success: #287a4b`
- `--color-warning: #a56a00`
- `--color-danger: #a33131`
- `--space-1: 4px`, `--space-2: 8px`, `--space-3: 12px`, `--space-4: 16px`, `--space-5: 20px`, `--space-6: 24px`
- `--radius-control: 6px`, `--radius-panel: 6px`
- no decorative shadows; inset selection indicators are functional

## Typography

Use the existing local-first Avenir Next / Segoe UI / Helvetica / Arial stack. Dashboard labels prioritize density and platform availability over brand display typography.

## Interaction

- Selection does not relayout the Architecture view.
- Selection does not center automatically.
- Isolation is explicit and reversible.
- Fit preserves search, filters, and selection.
- Focus indicators are visible on every interactive element.
- `prefers-reduced-motion` disables non-essential transitions.
```

- [ ] **Step 4: Run the documentation test**

Run the focused command from Step 2.

Expected: PASS.

- [ ] **Step 5: Commit the design-system contract**

```bash
git add docs/design-system.md docs_test.go
git commit -m "docs: define dashboard design system"
```

## Task 2: Lock The New Navigation And Architecture Default With Failing Tests

**Files:**
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: required view IDs `architecture`, `endpoints`, and `diagnostics`.
- Produces: initial state contract `mode:"architecture"`.
- Produces: removed top-level IDs `services`, `raw`, and `issues`.

- [ ] **Step 1: Replace the old navigation assertions**

In `TestRenderWorkspaceDashboardHTMLContainsInteractiveGraphData`, replace assertions for the five old modes with:

```go
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
```

Remove the assertion that requires `mode:"issues"` and the old functions `renderServiceMap`, `renderEndpointPaths`, and `renderIssueWorkbench` as public mode renderers.

- [ ] **Step 2: Add explicit copy and purpose assertions**

Add:

```go
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
```

- [ ] **Step 3: Run the focused dashboard test and verify failure**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestRenderWorkspaceDashboardHTMLContainsInteractiveGraphData -count=1
```

Expected: FAIL because the generated dashboard still starts in Issues and exposes the old five modes.

- [ ] **Step 4: Commit the red test contract**

```bash
git add internal/scan/workspace_dashboard_test.go
git commit -m "test: define architecture-first dashboard contract"
```

## Task 3: Decompose The Dashboard Generator Without Changing Its Payload

**Files:**
- Modify: `internal/scan/workspace_dashboard.go`
- Create: `internal/scan/workspace_dashboard_template.go`
- Create: `internal/scan/workspace_dashboard_styles.go`
- Create: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Keeps: `RenderWorkspaceDashboardHTML(graph WorkspaceGraphRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string`.
- Keeps: `RenderWorkspaceDashboardHTMLWithModels(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string`.
- Produces: `renderWorkspaceDashboardDocument(title string, payload []byte) string`.
- Produces: constants `workspaceDashboardStyles` and `workspaceDashboardScript`.

- [ ] **Step 1: Add a decomposition regression test**

Add to `workspace_dashboard_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to establish the passing compatibility baseline**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestRenderWorkspaceDashboardHTMLKeepsPayloadOfflineAfterDecomposition -count=1
```

Expected: PASS before refactoring.

- [ ] **Step 3: Move CSS into `workspace_dashboard_styles.go`**

Create the file with `package scan` and a raw-string constant named `workspaceDashboardStyles`. Mechanically move every byte currently between the generated `<style>` and `</style>` tags into that constant before making style changes; the rendered compatibility test from Step 1 must remain green after the move. Rename the existing color variables to the role names from `docs/design-system.md` and add these exact rules at the end of the constant:

```css
button:focus-visible,input:focus-visible,[data-select-id]:focus-visible{outline:3px solid var(--color-focus);outline-offset:2px}
@media (prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition-duration:0.01ms!important;animation-duration:0.01ms!important}}
```

- [ ] **Step 4: Move JavaScript into `workspace_dashboard_script.go`**

Create the file with `package scan` and a raw-string constant named `workspaceDashboardScript`. Mechanically move the existing script beginning with `const workspaceGraph = workspacePayload.graph` and ending with `renderList();renderCanvas();` into that constant. Leave `const workspacePayload = <marshaled JSON>;` in the template so the payload is inserted exactly once. Run the compatibility test immediately after the move; it must remain green before behavior changes begin.

- [ ] **Step 5: Move the semantic shell into `workspace_dashboard_template.go`**

Implement:

```go
package scan

import (
	"html"
	"strings"
)

func renderWorkspaceDashboardDocument(title string, payload []byte) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title>\n<style>")
	b.WriteString(workspaceDashboardStyles)
	b.WriteString("</style>\n</head>\n<body>\n")
	b.WriteString(workspaceDashboardShell)
	b.WriteString("\n<script>\nconst workspacePayload = ")
	b.Write(payload)
	b.WriteString(";\n")
	b.WriteString(workspaceDashboardScript)
	b.WriteString("\n</script>\n</body>\n</html>")
	return b.String()
}
```

Define `workspaceDashboardShell` in the same file with the complete semantic body markup.

- [ ] **Step 6: Reduce `workspace_dashboard.go` to payload composition**

Keep the two exported render functions and `marshalDashboardPayload`. After computing `title` and `payload`, return:

```go
return renderWorkspaceDashboardDocument(title, payload)
```

Remove imports that are now owned by the template file.

- [ ] **Step 7: Run dashboard and full scan-package tests**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestRenderWorkspaceDashboard' -count=1
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -count=1
```

Expected: the compatibility test remains PASS; the new Architecture-first contract remains FAIL until Task 4.

- [ ] **Step 8: Commit the focused decomposition**

```bash
git add internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "refactor: split workspace dashboard generator"
```

## Task 4: Implement Architecture-First Navigation And Stable Service Focus

**Files:**
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- View IDs: `architecture`, `endpoints`, `diagnostics`.
- State fields: `mode`, `selected`, `isolation`, `viewports`.
- Produces: `setArchitectureIsolation(enabled bool)` and `serviceFocus(id string)` JavaScript behavior.

- [ ] **Step 1: Add a regression test for stable selection and explicit isolation**

Add:

```go
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
```

- [ ] **Step 2: Run the test and verify failure**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestDashboardArchitectureSelectionKeepsFullLayoutUntilExplicitIsolation -count=1
```

Expected: FAIL because isolation state and controls do not exist.

- [ ] **Step 3: Replace the navigation shell**

Use this order and copy:

```html
<button data-view-mode="architecture" class="active">Architecture</button>
<button data-view-mode="endpoints">Endpoints</button>
<button data-view-mode="diagnostics">Diagnostics</button>
```

The initial help text is:

```text
See how projects and services communicate across the workspace. Select a service to highlight direct incoming and outgoing relationships without changing the layout.
```

Add initially hidden buttons with IDs `isolate-neighborhood` and `show-full-architecture`.

- [ ] **Step 4: Change the initial state**

Use:

```js
const state={mode:"architecture",query:"",filter:"all",selected:null,isolation:false,zoom:1,panX:0,panY:0,labels:false,drag:null,dragMoved:false,positions:new Map(),viewports:new Map()};
```

- [ ] **Step 5: Preserve every Architecture node during ordinary selection**

In `renderArchitectureMap`, derive:

```js
const allNodes=visibleServices();
const focused=state.selected?serviceFocus(state.selected):null;
const nodes=state.isolation&&focused?allNodes.filter(function(n){return focused.has(n.id);}):allNodes;
```

Continue dimming nodes with `focused&&!focused.has(n.id)`. Do not change `architectureLayout` based on selection unless isolation is true.

- [ ] **Step 6: Implement explicit isolation controls**

Add:

```js
function setArchitectureIsolation(enabled){
  if(enabled&&!state.selected)return;
  state.isolation=enabled;
  document.getElementById("isolate-neighborhood").hidden=enabled||!state.selected;
  document.getElementById("show-full-architecture").hidden=!enabled;
  renderCanvas();
}
```

Wire both buttons. Clearing selection sets `state.isolation=false`. Normal selection shows `Isolate neighborhood`; it must not enable isolation automatically.

- [ ] **Step 7: Update mode handling and purpose copy**

`setMode` must save the current viewport, set the new mode, restore its viewport, reset isolation, update active navigation, and use the approved one-sentence purpose for Architecture, Endpoints, or Diagnostics. Do not use `resetView()` on every mode change.

- [ ] **Step 8: Run the stable-focus and navigation tests**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestDashboardArchitectureSelection|TestRenderWorkspaceDashboardHTMLContainsInteractiveGraphData' -count=1
```

Expected: stable-focus assertions PASS. The full contract may still fail on Endpoints, Diagnostics, or viewport helpers implemented in later tasks.

- [ ] **Step 9: Commit Architecture behavior**

```bash
git add internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: make architecture the dashboard default"
```

## Task 5: Merge Endpoint Inventory And Trace Into Endpoints

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `renderEndpoints()` and `endpointRowsForService(serviceID)`.
- Keeps: existing `WorkspaceEndpointTraceRecord` payload and detail source links.
- Removes as modes: `renderEndpointPaths()` and raw mode selection.

- [ ] **Step 1: Replace the old Endpoint Paths test with the Endpoints contract**

Rename `TestRenderWorkspaceDashboardHTMLEndpointPathsUseServiceRows` to `TestRenderWorkspaceDashboardHTMLEndpointsCombineInventoryAndTrace`. Assert:

```go
for _, want := range []string{
	"function renderEndpoints()",
	"function endpointRowsForService(serviceId)",
	"Endpoint inventory",
	"Implementation trace",
	"Caller",
	"Endpoint",
	"Provider",
	"trace:get-user",
} {
	if !strings.Contains(html, want) {
		t.Fatalf("dashboard html missing combined endpoint behavior %q", want)
	}
}
```

Assert that `function renderEndpointPaths()` and the phrase `This replaces the low-level raw node cloud` are absent.

- [ ] **Step 2: Run the focused test and verify failure**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestRenderWorkspaceDashboardHTMLEndpointsCombineInventoryAndTrace -count=1
```

Expected: FAIL because the dashboard still has separate Endpoint Trace and Endpoint Paths renderers.

- [ ] **Step 3: Rename and retain the inventory model**

Rename `endpointPathRowsForService` to `endpointRowsForService`. Keep its deduplication of trace and service-edge rows, but use UI labels `Caller`, `Endpoint`, and `Provider` without exposing the old raw-node terminology.

- [ ] **Step 4: Implement `renderEndpoints`**

Behavior:

```js
function renderEndpoints(){
  const selectedTrace=state.selected&&traceById.get(state.selected);
  if(selectedTrace){renderEndpointTrace(selectedTrace);return;}
  renderEndpointInventory();
}
```

`renderEndpointInventory` shows rows for the selected service or the first visible service. Selecting an endpoint trace ID renders its implementation trace in the same view. Selecting a service switches the inventory without changing modes.

- [ ] **Step 5: Add explicit return navigation**

Trace details include a `Back to endpoint inventory` button. It clears only the selected trace and preserves the Endpoints viewport and search/filter state.

- [ ] **Step 6: Update selection routing**

`selectItem` must distinguish service IDs from trace IDs in Endpoints. It must not fall through to raw graph details. Remove `visibleRawNodes`, raw-mode list rendering, and raw-mode dispatch only when no remaining dashboard path references them.

- [ ] **Step 7: Run endpoint and existing trace tests**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestRenderWorkspaceDashboardHTML(Endpoints|WithModelsUsesFullEndpointTraces)' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit the unified Endpoints view**

```bash
git add internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: combine endpoint inventory and traces"
```

## Task 6: Rename And Explain Diagnostics Without Changing Schema 1

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `buildDiagnosticGroups()` and `renderDiagnostics()` over existing trace records.
- Keeps: existing technical risk codes and trace payload.
- Defers: canonical Go diagnostic records to `0.9.3`.

- [ ] **Step 1: Rename the issue-group test and strengthen its semantics**

Rename `TestRenderWorkspaceDashboardHTMLGroupsOpenIssuesAndAddsFileLinks` to `TestRenderWorkspaceDashboardHTMLExplainsDiagnosticGroupsAndAddsFileLinks`. Assert:

```go
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
```

- [ ] **Step 2: Run the test and verify failure**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestRenderWorkspaceDashboardHTMLExplainsDiagnosticGroupsAndAddsFileLinks -count=1
```

Expected: FAIL because the current view exposes Open Issues and raw risk codes.

- [ ] **Step 3: Add presentation mapping**

Implement `diagnosticPresentation(trace)` with these mappings:

```js
const diagnosticCopy={
  method_mismatch:{classification:"Likely code defect",title:"Frontend and backend use different HTTP methods",next:"Compare the client call with the available backend route and check for a stale contract."},
  indexed_backend_route_missing:{classification:"Missing scan coverage",title:"No matching route was found in the indexed backend",next:"Inspect nearby backend routes and confirm that gateway prefixes and the owning service were scanned."},
  dynamic_endpoint_unresolved:{classification:"Dynamic or statically ambiguous",title:"The endpoint is built dynamically",next:"Inspect the source expression or configure the project-specific route pattern."},
  frontend_internal_api:{classification:"Expected behavior",title:"The call targets a frontend-internal API",next:"No backend action is required unless this route should leave the frontend application."}
};
```

Unknown codes use classification `Information`, a title derived from the code, and a next step to inspect evidence.

- [ ] **Step 4: Rename issue functions and UI copy**

Rename `buildIssueGroups` to `buildDiagnosticGroups`, `renderIssueWorkbench` to `renderDiagnostics`, `showIssueDetails` to `showDiagnosticDetails`, and visible labels from Open Issues/Issue Workbench/Problem family to Diagnostics/Diagnostic workbench/Diagnostic group.

Keep technical risk codes in secondary metadata and search values.

- [ ] **Step 5: Explain the selected diagnostic**

The detail panel must render `Classification`, `Why GoreGraph shows this`, `Possible impact`, `Evidence`, and `What to check next`. Existing source file links remain clickable.

- [ ] **Step 6: Run diagnostic tests**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestRenderWorkspaceDashboardHTML(ExplainsDiagnostic|ContainsInteractiveGraphData)' -count=1
```

Expected: diagnostic assertions PASS; `frontend_internal_api` appears as expected behavior rather than a likely defect.

- [ ] **Step 7: Commit Diagnostics behavior**

```bash
git add internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: explain dashboard diagnostics"
```

## Task 7: Implement Correct Fit, Pointer-Centered Zoom, And Per-View Viewports

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `saveViewport(mode)`, `restoreViewport(mode)`, `fitVisibleContent()`, and `zoomAtPoint(factor, x, y)`.
- Keeps: `resetView()` as a zoom/pan-only reset.

- [ ] **Step 1: Add exact viewport behavior assertions**

Add:

```go
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
```

- [ ] **Step 2: Run the test and verify failure**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestDashboardViewportControlsPreserveUserContext -count=1
```

Expected: FAIL because Fit currently clears query and selection.

- [ ] **Step 3: Implement viewport persistence**

Add:

```js
function saveViewport(mode){state.viewports.set(mode,{zoom:state.zoom,panX:state.panX,panY:state.panY});}
function restoreViewport(mode){const saved=state.viewports.get(mode)||{zoom:1,panX:0,panY:0};state.zoom=saved.zoom;state.panX=saved.panX;state.panY=saved.panY;applyTransform();}
```

Call `saveViewport(state.mode)` before changing mode and `restoreViewport(newMode)` after rendering the new view.

- [ ] **Step 4: Implement real Fit**

Use:

```js
function fitVisibleContent(){
  const svg=document.getElementById("workspace-graph"),layer=document.getElementById("graph-layer");
  const box=layer.getBBox(),width=svg.clientWidth||1100,height=svg.clientHeight||720,padding=56;
  if(!box.width||!box.height)return resetView();
  state.zoom=Math.max(.35,Math.min(3,Math.min((width-padding*2)/box.width,(height-padding*2)/box.height)));
  state.panX=width/2-(box.x+box.width/2)*state.zoom;
  state.panY=height/2-(box.y+box.height/2)*state.zoom;
  applyTransform();
  saveViewport(state.mode);
}
```

Wire the Fit button directly to this function. It must not alter query, filters, selection, or isolation.

- [ ] **Step 5: Implement pointer-centered zoom**

Use:

```js
function zoomAtPoint(factor,x,y){
  const previous=state.zoom,next=Math.max(.35,Math.min(3,previous*factor));
  const graphX=(x-state.panX)/previous,graphY=(y-state.panY)/previous;
  state.zoom=next;state.panX=x-graphX*next;state.panY=y-graphY*next;
  applyTransform();saveViewport(state.mode);
}
```

Wheel handling derives `x` and `y` from the SVG bounding rectangle. Zoom buttons use the SVG center. Remove `focusTraceStep` calls to `centerOnPosition`; keep an explicit `Center selection` action if centering is still offered.

- [ ] **Step 6: Preserve viewport after pan and reset**

Save the viewport after pointer-up and after `resetView`. Reset must change only `zoom`, `panX`, and `panY`.

- [ ] **Step 7: Run viewport and dashboard tests**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestDashboardViewport|TestRenderWorkspaceDashboard' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit viewport behavior**

```bash
git add internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "fix: stabilize dashboard viewport controls"
```

## Task 8: Update README, Output Contract, Release Notes, And Version

**Files:**
- Modify: `README.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`
- Modify: `internal/version/version.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `release_files_test.go`

**Interfaces:**
- Produces: development version `0.9.1`.
- Documents: Architecture, Endpoints, Diagnostics, isolation, Fit, confidence limitations, and fresh-scan workflow.

- [ ] **Step 1: Change the version expectation test first**

In `TestRunVersionPrintsBuildMetadata`, change:

```go
"goregraph 0.9.0",
```

to:

```go
"goregraph 0.9.1",
```

Update any release-file assertion that expects `0.9.0` as the current development version to `0.9.1`.

- [ ] **Step 2: Run version tests and verify failure**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/cli -run TestRunVersionPrintsBuildMetadata -count=1
GOCACHE=/private/tmp/goregraph-gocache go test . -run TestRelease -count=1
```

Expected: FAIL because `internal/version.Version` remains `0.9.0`.

- [ ] **Step 3: Set the development version**

Change `internal/version/version.go` to:

```go
Version = "0.9.1"
```

- [ ] **Step 4: Rewrite the README dashboard section**

Replace the old five-view description with sections answering:

- Architecture: understand service relationships; normal focus preserves the map; isolation is explicit.
- Endpoints: inventory and implementation trace in one view.
- Diagnostics: unresolved or conflicting relationships with meaning and next checks.
- Controls: 100% resets viewport only; Fit fits visible content; selection does not auto-center.
- Static-analysis limits: absence of a relationship is not runtime proof.

Update the development release target to `v0.9.1`.

- [ ] **Step 5: Update `docs/OUTPUTS.md`**

Document that the existing `workspace-map.html` output remains Schema 1 compatible while its top-level UI becomes Architecture, Endpoints, and Diagnostics. State that Focused Service behavior is available through explicit Architecture isolation and Endpoint Paths behavior is available through endpoint inventory.

- [ ] **Step 6: Add `v0.9.1` release checks**

Add a `v0.9.1` section to `docs/RELEASE.md` with the exact dashboard acceptance behaviors and version output. Do not create a tag or public release instruction specific to an already completed release.

- [ ] **Step 7: Run documentation, version, and release tests**

```bash
GOCACHE=/private/tmp/goregraph-gocache go test . -count=1
GOCACHE=/private/tmp/goregraph-gocache go test ./internal/cli -run TestRunVersionPrintsBuildMetadata -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit documentation and version**

```bash
git add README.md docs/OUTPUTS.md docs/RELEASE.md internal/version/version.go internal/cli/cli_test.go release_files_test.go
git commit -m "docs: prepare GoreGraph 0.9.1"
```

## Task 9: Full Automated Verification And Local Installation

**Files:**
- Verify only; modify production files only if a failing test exposes a defect and add a regression test first.

**Interfaces:**
- Consumes: all `0.9.1` changes.
- Produces: locally installed `goregraph 0.9.1` used by real-workspace acceptance.

- [ ] **Step 1: Format and inspect the diff**

```bash
gofmt -w internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go internal/version/version.go internal/cli/cli_test.go docs_test.go release_files_test.go
git diff --check
```

Expected: no output from `git diff --check`.

- [ ] **Step 2: Run static and full tests**

```bash
GOCACHE=/private/tmp/goregraph-gocache go vet ./...
GOCACHE=/private/tmp/goregraph-gocache go test ./... -count=1
```

Expected: PASS for every package.

- [ ] **Step 3: Build and inspect the local binary before installation**

```bash
GOCACHE=/private/tmp/goregraph-gocache go build -o /private/tmp/goregraph-0.9.1 ./cmd/goregraph
/private/tmp/goregraph-0.9.1 version
```

Expected first line: `goregraph 0.9.1`.

- [ ] **Step 4: Install through the repository's documented local path**

```bash
GOCACHE=/private/tmp/goregraph-gocache go install ./cmd/goregraph
command -v goregraph
goregraph version
```

Expected: the resolved command is the Go-installed binary on PATH and the first version line is `goregraph 0.9.1`. If PATH resolves another installation first, stop and correct PATH or invoke the verified Go bin path; do not test an older Homebrew binary.

- [ ] **Step 5: Commit formatting-only changes if any**

Only when `gofmt` changed tracked files:

```bash
git add internal docs_test.go release_files_test.go
git commit -m "chore: format GoreGraph 0.9.1 changes"
```

## Task 10: Clean WEKA Rescan, Output Inspection, And Browser Acceptance

**Files:**
- Generated outside this repository: `~/projects/weka/.goregraph-workspace/` and project-local GoreGraph output directories.
- Do not commit generated WEKA outputs.

**Interfaces:**
- Consumes: installed `goregraph 0.9.1`.
- Produces: fresh real-workspace acceptance evidence.

- [ ] **Step 1: Confirm workspace and installed binary**

```bash
cd ~/projects/weka
pwd
command -v goregraph
goregraph version
```

Expected: `pwd` ends in `/projects/weka`; version starts with `goregraph 0.9.1`.

- [ ] **Step 2: Review the non-destructive clean plan**

```bash
goregraph workspace clean .
```

Expected: `Dry run: true`; every listed path is either `.goregraph-workspace` or a configured GoreGraph output directory. Stop if any source or unrelated path appears.

- [ ] **Step 3: Execute the reviewed cleanup**

```bash
goregraph workspace clean . --execute
```

Expected: only reviewed generated paths are removed.

- [ ] **Step 4: Perform a full fresh scan**

```bash
goregraph workspace scan-all .
```

Expected: all discovered projects scan successfully and `.goregraph-workspace/workspace-map.html` is regenerated.

- [ ] **Step 5: Run health checks**

```bash
goregraph doctor .
goregraph workspace status .
goregraph workspace dashboard .
```

Expected: doctor succeeds, status reports the freshly indexed workspace, and dashboard prints the absolute HTML path.

- [ ] **Step 6: Inspect generated content without opening the UI**

```bash
test -f .goregraph-workspace/workspace-map.html
rg -n 'mode:"architecture"|>Architecture<|>Endpoints<|>Diagnostics<|Isolate neighborhood|fitVisibleContent' .goregraph-workspace/workspace-map.html
rg -n '>Focused Service<|>Endpoint Paths<|>Open Issues<' .goregraph-workspace/workspace-map.html
```

Expected: the first search finds all new behavior; the second search returns no matches.

- [ ] **Step 7: Open the generated dashboard in the approved browser environment**

Use the Codex in-app browser when available. Verify no console errors and capture desktop and narrow-viewport screenshots.

- [ ] **Step 8: Verify Architecture behavior**

Confirm:

- Architecture is active on initial load.
- All expected services remain visible.
- Selecting a service highlights direct edges and dims unrelated nodes without moving any node.
- `Isolate neighborhood` filters only after explicit activation.
- `Show full architecture` restores the same full-map positions.
- Incoming and Outgoing details retain endpoint examples and source evidence already available in Schema 1.

- [ ] **Step 9: Verify Endpoints and Diagnostics**

Confirm:

- selecting a service shows its endpoint inventory
- selecting an endpoint opens its existing implementation trace
- returning to inventory preserves search and filter context
- Diagnostics explains classification, reason, possible impact, evidence, and next check
- frontend-internal API is presented as expected behavior, not a likely defect

- [ ] **Step 10: Verify viewport and accessibility behavior**

Confirm:

- selecting items never auto-centers
- wheel zoom follows the pointer
- 100% changes only zoom and pan
- Fit fits visible content without clearing state
- each view restores its last viewport
- keyboard focus is visible
- Escape exits isolation or selection predictably
- narrow layout remains usable
- reduced-motion emulation introduces no required-motion interaction

- [ ] **Step 11: Repeat the scan for determinism-sensitive outputs**

Record hashes of deterministic dashboard inputs before and after a second `goregraph workspace scan-all .`. Exclude documented timestamp metadata. Expected: deterministic content is unchanged.

- [ ] **Step 12: Record regressions before fixing them**

For every failure, return to the repository, add the smallest failing Go or browser regression test, implement the minimal fix, reinstall `0.9.1`, and repeat Tasks 9 and 10 from the clean step.

## Final 0.9.1 Completion Gate

- [ ] Architecture is first and default.
- [ ] Ordinary service focus preserves the complete layout and all connections.
- [ ] Explicit isolation preserves and restores the Architecture context.
- [ ] Endpoints combines service inventory and endpoint traces.
- [ ] Diagnostics replaces Open Issues and explains meaning and next checks.
- [ ] Fit, zoom, reset, pan, mode viewports, keyboard focus, and reduced motion pass.
- [ ] README, design system, output contract, release notes, and version report `0.9.1` behavior accurately.
- [ ] Full Go verification passes.
- [ ] The installed binary reports `0.9.1`.
- [ ] A clean fresh scan of `~/projects/weka/` passes doctor, status, output, and browser acceptance.
- [ ] No push, tag, or public release was performed.
