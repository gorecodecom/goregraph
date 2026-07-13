# GoreGraph 1.2 Workspace Intelligence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver Issues 9 through 22 as a locally installed GoreGraph 1.2.0 with a readable six-view dashboard and one canonical evidence-backed feature-flow model.

**Architecture:** Extend Schema 2 additively: keep every legacy feature-flow field and add canonical nodes, edges, test links, commands, coverage, and impact projections built from existing scan facts. Feed the standalone dashboard, Markdown, Query, and MCP from those Go records so UI and agent consumers cannot diverge. Implement one issue per test-first task and one local commit per issue.

**Tech Stack:** Go 1.25 standard library, standalone HTML/CSS/vanilla JavaScript embedded by Go, Go testing, Node syntax validation, Playwright/browser acceptance, Git.

## Global Constraints

- Set the final local product version to exactly `1.2.0`; keep `scan.SchemaVersion` exactly `2`.
- Preserve existing JSON fields, CLI commands, Query tasks, MCP tools, confidence meanings, and Markdown contracts.
- Add no external dependency.
- Treat missing coverage as uncertainty, never as proof that code, a relationship, or a test does not exist.
- Escape untrusted text at the final HTML rendering boundary.
- Keep Architecture selection stable; focus, isolation, Fit, and source opening are explicit actions.
- Use semantic HTML at normal browser scale for inventory-like views.
- Write comments and public documentation in English.
- Commit each GitHub issue separately and do not push.

---

### Task 1: Bundle Architecture Relationships By Domain — Issue 9

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Consumes: `WorkspaceServiceNodeRecord.Group`, `WorkspaceServiceEdgeRecord`, `architectureLayout`.
- Produces: `architectureBundles(edges, nodeByID)`, bundled background paths, and selected-service direct paths.

- [ ] **Step 1: Write the failing renderer test**

```go
func TestWorkspaceDashboardBundlesBackgroundArchitectureEdges(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{"function architectureBundles", "bundle-count", "data-architecture-bundle", "Unrelated relationships remain grouped"} {
		if !strings.Contains(html, want) { t.Fatalf("dashboard missing %q", want) }
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/scan -run TestWorkspaceDashboardBundlesBackgroundArchitectureEdges -count=1`

Expected: FAIL because the dashboard still emits every background edge separately.

- [ ] **Step 3: Implement deterministic bundles**

```javascript
function architectureBundles(edges,nodeByID){
  const bundles=new Map();
  edges.forEach(function(edge){
    const from=nodeByID.get(edge.from),to=nodeByID.get(edge.to);
    if(!from||!to)return;
    const key=(from.group||"Other")+"\u0000"+(to.group||"Other")+"\u0000"+(edge.risk||"resolved");
    const bundle=bundles.get(key)||{id:"bundle:"+key,from:from.group||"Other",to:to.group||"Other",risk:edge.risk,total:0,edges:[]};
    bundle.total+=edge.total||1;bundle.edges.push(edge);bundles.set(key,bundle);
  });
  return Array.from(bundles.values()).sort(function(a,b){return a.id.localeCompare(b.id);});
}
```

Render one muted path and count label per background bundle. When a service is selected, render its direct relationships above bundles with existing ports and IN/OUT badges. Add `.architecture-bundle`, `.bundle-count`, and selected/background opacity rules using existing design tokens.

- [ ] **Step 4: Run focused and dashboard tests**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboard(BundlesBackgroundArchitectureEdges|Architecture)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 9**

```powershell
git add internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: bundle architecture relationships by domain" -m "- Group background edges into deterministic domain bundles" -m "- Expand direct relationships only for the selected service" -m "- Cover dense architecture rendering and selection stability"
```

### Task 2: Add Explicit Focused-Service Architecture Mode — Issue 10

**Files:**
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Consumes: `serviceFocus`, per-mode viewport state, Architecture selection.
- Produces: `state.architectureFocused`, `enterArchitectureFocus()`, `leaveArchitectureFocus()`.

- [ ] **Step 1: Write a failing focus-state test**

```go
func TestWorkspaceDashboardUsesExplicitArchitectureFocus(t *testing.T) {
	html := renderDashboardFixture(t)
	for _, want := range []string{"Focus selected", "Back to full architecture", "architectureFocused", "savedFullArchitectureViewport"} {
		if !strings.Contains(html, want) { t.Fatalf("dashboard missing %q", want) }
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run TestWorkspaceDashboardUsesExplicitArchitectureFocus -count=1`

Expected: FAIL because isolation is the only neighborhood action.

- [ ] **Step 3: Implement explicit focus and restoration**

```javascript
function enterArchitectureFocus(){
  if(!state.selected)return;
  state.savedFullArchitectureViewport={zoom:state.zoom,panX:state.panX,panY:state.panY,selected:state.selected};
  state.architectureFocused=true;renderList();renderCanvas();
}
function leaveArchitectureFocus(){
  const saved=state.savedFullArchitectureViewport;
  state.architectureFocused=false;
  if(saved){state.zoom=saved.zoom;state.panX=saved.panX;state.panY=saved.panY;state.selected=saved.selected;}
  renderList();renderCanvas();
}
```

Focused layout contains selected service plus direct callers and providers grouped by domain. Ordinary selection leaves `architectureFocused` false. Wire explicit buttons and accessible pressed/hidden states.

- [ ] **Step 4: Run focus and viewport tests**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboard.*(Focus|Viewport|Architecture)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 10**

```powershell
git add internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: add explicit architecture focus mode" -m "- Show one service with direct callers and providers on demand" -m "- Restore the full-map selection and viewport on return" -m "- Keep ordinary architecture selection layout-stable"
```

### Task 3: Show Complete Endpoint Source Context — Issue 11

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_dashboard.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `Config.EditorURLTemplate string`, dashboard payload `editor_url_template`, and safe `sourceActions(project,file,line)`.

- [ ] **Step 1: Write failing configuration and renderer tests**

```go
func TestLoadEditorURLTemplate(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "goregraph.yml"), []byte("version: 1\neditor_url_template: vscode://file/{file}:{line}\n"), 0o644)
	cfg, err := Load(root)
	if err != nil || cfg.EditorURLTemplate != "vscode://file/{file}:{line}" { t.Fatalf("template=%q err=%v", cfg.EditorURLTemplate, err) }
}
```

Add a dashboard test requiring `source-actions`, `Copy path`, `Open source`, separate Service/Project/Class/Symbol/File/Line fields, and no escaped `<a class=` source markup.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/config ./internal/scan -run 'Test(LoadEditorURLTemplate|WorkspaceDashboard.*SourceContext)' -count=1`

Expected: FAIL for unsupported config and missing source actions.

- [ ] **Step 3: Implement safe source actions**

Add config parsing for `editor_url_template`. Carry it into `WorkspaceServiceMapRecord.EditorURLTemplate`. Implement URL expansion only for `{file}` and `{line}`, encode values with `encodeURIComponent`, and render links through escaped attributes. Use `navigator.clipboard.writeText` with a visible fallback containing the path and line.

```javascript
function editorURL(file,line){
  if(!editorURLTemplate)return "";
  return editorURLTemplate.replaceAll("{file}",encodeURIComponent(file)).replaceAll("{line}",String(line||1));
}
```

- [ ] **Step 4: Run config, endpoint, and Windows-path tests**

Run: `go test ./internal/config ./internal/scan -run 'Test(LoadEditorURLTemplate|WorkspaceDashboard.*(Endpoint|Source|Windows))' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 11**

```powershell
git add internal/config/config.go internal/config/config_test.go internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "fix: show complete endpoint source context" -m "- Separate service, class, symbol, file, and line details" -m "- Add safe copy and configurable editor source actions" -m "- Cover escaping, fallbacks, and Windows paths"
```

### Task 4: Clarify Data Flow Purpose And Evidence — Issue 12

**Files:**
- Modify: `internal/scan/data_flow.go`
- Modify: `internal/scan/data_flow_report.go`
- Test: `internal/scan/data_flow_test.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Consumes: `DataFlowRecord.Nodes`, `DataFlowRecord.Gaps`.
- Produces: ordered stage labels and gap `next_check` guidance.

- [ ] **Step 1: Write failing stage and copy tests**

```go
func TestDataFlowExplainsStagesAndGaps(t *testing.T) {
	records := BuildDataFlows([]WorkspaceFeatureFlowRecord{{ID:"flow", HTTPMethod:"POST", Path:"/users"}})
	if len(records) != 1 || len(records[0].Gaps) == 0 || records[0].Gaps[0].NextCheck == "" { t.Fatalf("flow=%#v", records) }
}
```

Require dashboard copy `Endpoints show the call path` and `Data Flow shows the data path`, a visible evidence legend, and an endpoint-to-data-flow action.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run 'Test(DataFlowExplainsStagesAndGaps|WorkspaceDashboard.*DataFlow)' -count=1`

Expected: FAIL because gaps lack concrete next checks and the distinction is implicit.

- [ ] **Step 3: Add explicit stage and evidence presentation**

Extend `DataFlowGapRecord` with additive `Capability` and `NextCheck` fields. Keep stage order request, validation, transformation, persistence, messaging, response. Render exact, inferred, weak, and missing legend entries and a concrete low-evidence empty state.

- [ ] **Step 4: Run Data Flow tests**

Run: `go test ./internal/scan -run 'Test.*DataFlow' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 12**

```powershell
git add internal/scan/data_flow.go internal/scan/data_flow_report.go internal/scan/data_flow_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: clarify data flow purpose and evidence" -m "- Distinguish endpoint call paths from field-level data paths" -m "- Add ordered stages, evidence legend, and actionable gaps" -m "- Connect endpoint selection to the related data flow"
```

### Task 5: Make Diagnostics Readable At 100 Percent — Issue 13

**Files:**
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `renderDiagnosticsSidebar()` and `renderDiagnosticsWorkbench()` using semantic HTML.

- [ ] **Step 1: Write the failing semantic-workbench test**

```go
func TestWorkspaceDashboardRendersDiagnosticsAsHTMLWorkbench(t *testing.T) {
	html := renderDashboardFixture(t)
	for _, want := range []string{"function renderDiagnosticsWorkbench", "diagnostic-row", "diagnostic-workbench", "normal vertical scrolling"} {
		if !strings.Contains(html, want) { t.Fatalf("dashboard missing %q", want) }
	}
	if strings.Contains(html, "function renderDiagnostics(){const svg=") { t.Fatal("Diagnostics still uses a fitted SVG") }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run TestWorkspaceDashboardRendersDiagnosticsAsHTMLWorkbench -count=1`

Expected: FAIL because Diagnostics renders all groups in one SVG.

- [ ] **Step 3: Implement the master-detail workbench**

Render diagnostic rows as buttons in the sidebar and one selected diagnostic in `workspace-workbench`. Hide graph controls in Diagnostics, preserve list scroll, and synchronize selected row and details without changing layout.

- [ ] **Step 4: Run Diagnostics renderer tests**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboard.*Diagnostic' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 13**

```powershell
git add internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "fix: make diagnostics readable at normal scale" -m "- Replace the fitted diagnostic SVG with an HTML workbench" -m "- Preserve selected rows and scroll position without relayout" -m "- Hide graph-only zoom controls in Diagnostics"
```

### Task 6: Provide Actionable Diagnostic Explanations — Issue 14

**Files:**
- Modify: `internal/scan/canonical_diagnostics.go`
- Test: `internal/scan/canonical_diagnostics_test.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/agent/service.go`
- Test: `internal/agent/service_test.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces additive diagnostic-family fields `ObservedCount`, `ResolvedCount`, `UnresolvedCount`, `OutOfScopeCount`, `LikelyOwner`, `AffectedProjects`, `NextChecks`.

- [ ] **Step 1: Write failing family-accounting tests**

```go
func TestDiagnosticFamilyExplainsUnsafeDynamicEvidence(t *testing.T) {
	families := BuildDiagnosticFamilies("frontend/app", []CanonicalDiagnosticRecord{{ID:"d1", Code:"dynamic_endpoint_unresolved", Resolution:ResolutionPartial, AffectedArtifacts:[]string{"GET /documentdownload/{variant}"}, NextChecks:[]string{"Inspect variant."}}})
	if len(families)!=1 || families[0].UnresolvedCount!=1 || !strings.Contains(families[0].RootCause,"dynamic") || len(families[0].NextChecks)==0 { t.Fatalf("families=%#v",families) }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan ./internal/agent -run 'Test.*DiagnosticFamily' -count=1`

Expected: FAIL because family evidence is not accounted or shared across presentations.

- [ ] **Step 3: Centralize diagnostic explanations in Go**

Populate family counts and guidance from canonical diagnostics. Add specific dynamic-segment copy, unscanned service, method mismatch, missing route, and frontend-internal next checks. Render the same fields in Markdown, agent Query/MCP items, and dashboard payload rather than rebuilding explanations in JavaScript.

- [ ] **Step 4: Run diagnostic tests**

Run: `go test ./internal/scan ./internal/agent -run 'Test.*Diagnostic' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 14**

```powershell
git add internal/scan/canonical_diagnostics.go internal/scan/canonical_diagnostics_test.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/agent/service.go internal/agent/service_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: make diagnostic families actionable" -m "- Account for observed, resolved, unresolved, and out-of-scope evidence" -m "- Explain dynamic segments and concrete next checks" -m "- Share one explanation model across reports and dashboard"
```

### Task 7: Explain Partial And Unavailable Coverage — Issue 15

**Files:**
- Modify: `internal/scan/capabilities.go`
- Test: `internal/scan/capabilities_test.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `CapabilityRecord.StatusReason`, `ExpectedUnavailable`, `SourceClass`.

- [ ] **Step 1: Write failing status-reason tests**

```go
func TestGenericLanguagesExplainExpectedCoverage(t *testing.T) {
	records := BuildCapabilityInventory([]FileRecord{{Language:"markdown"},{Language:"yaml"},{Language:"text"}}, WorkspaceIndex{})
	for _, record := range records {
		if record.StatusReason=="" || record.SourceClass=="" { t.Fatalf("record=%#v",record) }
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run 'Test(GenericLanguagesExplainExpectedCoverage|WorkspaceDashboard.*Coverage)' -count=1`

Expected: FAIL because the new reasons and source classes do not exist.

- [ ] **Step 3: Add honest capability semantics**

Classify source as code, build, documentation, configuration, or text. Explain COMPLETE as analyzer support, PARTIAL as best effort, UNAVAILABLE as no registered capability, and FAILED as execution failure. Collapse documentation/configuration/text groups by default and keep failures expanded.

- [ ] **Step 4: Run capability and coverage tests**

Run: `go test ./internal/scan -run 'Test.*(Capability|Coverage)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 15**

```powershell
git add internal/scan/capabilities.go internal/scan/capabilities_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: explain capability coverage states" -m "- Distinguish analyzer support, expected gaps, and failures" -m "- Collapse documentation and configuration noise by default" -m "- Cover representative code and generic source classes"
```

### Task 8: Reset Stale Details Across Views — Issue 16

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: per-view `state.selections`, `clearDetailsForMode(mode)`, explicit transition restoration.

- [ ] **Step 1: Write the failing transition test**

```go
func TestWorkspaceDashboardOwnsSelectionPerView(t *testing.T) {
	html := renderDashboardFixture(t)
	for _, want := range []string{"selections:{architecture", "clearDetailsForMode", "Feature Flow context", "Coverage context"} {
		if !strings.Contains(html,want) { t.Fatalf("dashboard missing %q",want) }
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run TestWorkspaceDashboardOwnsSelectionPerView -count=1`

Expected: FAIL because a shared `selected` value owns incompatible details.

- [ ] **Step 3: Implement explicit view-state ownership**

Store Architecture, Endpoints, Feature Flow, Data Flow, Diagnostics, and Coverage selection separately. On transition, clear details immediately, restore only documented view state, and render each view's own empty state.

- [ ] **Step 4: Run all transition tests**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboard.*(Selection|Transition|Details|Mode)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 16**

```powershell
git add internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "fix: isolate dashboard selection by view" -m "- Clear incompatible details during top-level transitions" -m "- Restore state only for explicitly persistent views" -m "- Cover every dashboard mode transition"
```

### Task 9: Add Canonical Feature-Flow Nodes And Edges — Issue 18

**Files:**
- Create: `internal/scan/canonical_feature_flow.go`
- Create: `internal/scan/canonical_feature_flow_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/doctor/doctor.go`
- Test: `internal/doctor/doctor_test.go`
- Modify: `SCHEMA.md`

**Interfaces:**
- Produces: `CanonicalFlowNodeRecord`, `CanonicalFlowEdgeRecord`, `BuildCanonicalFeatureFlow(flow)`, `ValidateCanonicalFeatureFlow(flow)`.

- [ ] **Step 1: Write failing canonical-model tests**

```go
func TestBuildCanonicalFeatureFlowUsesStableReferences(t *testing.T) {
	flow := WorkspaceFeatureFlowRecord{ID:"flow:users", FrontendProject:"web", FrontendFile:"src/users.ts", FrontendLine:8, FrontendCaller:"loadUsers", HTTPMethod:"GET", Path:"/users", BackendProject:"services/users", BackendService:"users", BackendController:"UserController", BackendMethod:"list", BackendFile:"UserController.java", BackendLine:20}
	got := BuildCanonicalFeatureFlow(flow)
	if got.ModelVersion!=1 || len(got.Nodes)<2 || len(got.Edges)==0 { t.Fatalf("flow=%#v",got) }
	if err:=ValidateCanonicalFeatureFlow(got); err!=nil { t.Fatal(err) }
}
```

Add tests for deterministic IDs, dangling edges, conflicting duplicate nodes, legacy JSON unmarshalling, and stable semantic output.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan ./internal/doctor -run 'Test.*CanonicalFeatureFlow' -count=1`

Expected: FAIL because the canonical types and builders do not exist.

- [ ] **Step 3: Implement the additive canonical projection**

```go
type CanonicalFlowNodeRecord struct {
	ID string `json:"id"`; Kind string `json:"kind"`; Project string `json:"project,omitempty"`; Service string `json:"service,omitempty"`
	Symbol string `json:"symbol,omitempty"`; QualifiedName string `json:"qualified_name,omitempty"`; Signature string `json:"signature,omitempty"`
	File string `json:"file,omitempty"`; LineStart int `json:"line_start,omitempty"`; LineEnd int `json:"line_end,omitempty"`
	Confidence string `json:"confidence,omitempty"`; Reason string `json:"reason,omitempty"`; EvidenceIDs []string `json:"evidence_ids,omitempty"`
}
type CanonicalFlowEdgeRecord struct {
	ID string `json:"id"`; FromNodeID string `json:"from_node_id"`; ToNodeID string `json:"to_node_id"`; EdgeType string `json:"edge_type"`
	Confidence string `json:"confidence"`; Reason string `json:"reason"`; EvidenceIDs []string `json:"evidence_ids,omitempty"`; SourceAnalyzer string `json:"source_analyzer,omitempty"`
}
```

Add `ModelVersion`, `Nodes`, and `Edges` to `WorkspaceFeatureFlowRecord`. Populate them after legacy fields are complete. Validate generated records in Doctor without treating unsupported analysis as corruption.

- [ ] **Step 4: Run canonical, reconciliation, Doctor, and compatibility tests**

Run: `go test ./internal/scan ./internal/doctor -run 'Test.*(CanonicalFeatureFlow|WorkspaceFeatureFlow|Doctor)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 18**

```powershell
git add internal/scan/canonical_feature_flow.go internal/scan/canonical_feature_flow_test.go internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/doctor/doctor.go internal/doctor/doctor_test.go SCHEMA.md
git commit -m "feat: add canonical workspace feature flows" -m "- Add deterministic typed nodes and evidence-backed edges" -m "- Preserve legacy Schema 2 feature-flow fields" -m "- Validate references, identities, and deterministic ordering"
```

### Task 10: Add The Feature Flow Dashboard — Issue 19

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_service_map.go`
- Test: `internal/scan/workspace_service_map_test.go`
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Consumes: `WorkspaceFeatureFlowRecord.Nodes`, `Edges`, legacy fields for compatibility.
- Produces: `WorkspaceServiceMapRecord.FeatureFlows` and dashboard mode `feature-flow`.

- [ ] **Step 1: Write failing payload and UI tests**

```go
func TestWorkspaceServiceMapCarriesCanonicalFeatureFlows(t *testing.T) {
	flow:=BuildCanonicalFeatureFlow(WorkspaceFeatureFlowRecord{ID:"flow",FrontendProject:"web",FrontendFile:"api.ts",HTTPMethod:"GET",Path:"/users",BackendProject:"users"})
	got:=BuildWorkspaceServiceMap(WorkspaceRegistryRecord{},nil,[]WorkspaceFeatureFlowRecord{flow},nil)
	if len(got.FeatureFlows)!=1 { t.Fatalf("map=%#v",got) }
}
```

Require top-level `Feature Flow`, `renderFeatureFlowWorkbench`, source actions, confidence filters, and Endpoint/Data Flow handoff.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run 'TestWorkspace(ServiceMapCarriesCanonicalFeatureFlows|Dashboard.*FeatureFlow)' -count=1`

Expected: FAIL because feature flows are not embedded or rendered.

- [ ] **Step 3: Implement the semantic Feature Flow workbench**

Add the sixth view button. Render a searchable sidebar and one selected flow as ordered semantic cards derived from canonical nodes and edges. Show explicit gap cards for missing route, backend, persistence, or tests. Preserve selection when opening related Endpoint/Data Flow and returning.

- [ ] **Step 4: Run Feature Flow and dashboard tests**

Run: `go test ./internal/scan -run 'Test.*(FeatureFlow|Dashboard)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 19**

```powershell
git add internal/scan/types.go internal/scan/workspace_service_map.go internal/scan/workspace_service_map_test.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: add the Feature Flow dashboard" -m "- Present route-to-persistence implementation chains at normal scale" -m "- Explain confidence and missing stages without inventing links" -m "- Connect Feature Flow with Endpoints and Data Flow"
```

### Task 11: Add Prioritized Workspace Coverage — Issue 20

**Files:**
- Create: `internal/scan/workspace_coverage.go`
- Create: `internal/scan/workspace_coverage_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_service_map.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `WorkspaceCoverageSummaryRecord`, `NextScanRecord`, `BuildWorkspaceCoverage(context, summary)`.

- [ ] **Step 1: Write the failing prioritization test**

```go
func TestWorkspaceCoveragePrioritizesUsefulScans(t *testing.T) {
	context:=WorkspaceContextRecord{Projects:[]WorkspaceProjectRecord{{Path:"services/a",Service:"a",Indexed:false},{Path:"services/b",Service:"b",Indexed:false}},MissingServiceDetails:[]WorkspaceMissingServiceRecord{{Service:"b",Contracts:2,Project:"services/b"},{Service:"a",Contracts:7,Project:"services/a"}}}
	got:=BuildWorkspaceCoverage(context,WorkspaceContractSummaryRecord{Total:9,MissingRoute:9})
	if len(got.NextScans)!=2 || got.NextScans[0].Service!="a" { t.Fatalf("coverage=%#v",got) }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run TestWorkspaceCoveragePrioritizesUsefulScans -count=1`

Expected: FAIL because the summary builder does not exist.

- [ ] **Step 3: Build and render workspace completeness**

Compute known/indexed project and referenced-service counts plus canonical contract categories. Sort next scans by descending affected contracts, then project and service. Generate `goregraph scan <path>` only for known project paths. Embed the record in service-map payload and show it separately from analyzer capabilities.

- [ ] **Step 4: Run coverage, reconciliation, and dashboard tests**

Run: `go test ./internal/scan -run 'Test.*WorkspaceCoverage' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 20**

```powershell
git add internal/scan/workspace_coverage.go internal/scan/workspace_coverage_test.go internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/workspace_service_map.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: prioritize workspace coverage gaps" -m "- Separate workspace completeness from analyzer capability" -m "- Rank next scans by affected contracts with deterministic ties" -m "- Reuse canonical contract categories across presentations"
```

### Task 12: Improve Test Linkage And Verification Commands — Issue 21

**Files:**
- Create: `internal/scan/test_verification.go`
- Create: `internal/scan/test_verification_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/canonical_feature_flow.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/agent/service.go`
- Test: `internal/agent/service_test.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `TestLinkRecord`, `VerificationCommandRecord`, `BuildTestLinks(flow)`, `BuildVerificationCommands(project, tests)`.

- [ ] **Step 1: Write failing linkage and command tests**

```go
func TestBuildVerificationCommandsUsesDetectedMavenTest(t *testing.T) {
	project:=WorkspaceProjectRecord{Path:"services/users",Kind:"maven"}
	tests:=[]TestMapRecord{{TestFile:"src/test/java/UserControllerTest.java",TestMethod:"deletesUser",Confidence:"EXACT"}}
	got:=BuildVerificationCommands(project,tests)
	if len(got)!=1 || got[0].Tool!="maven" || got[0].WorkingDirectory!="services/users" || len(got[0].Args)==0 { t.Fatalf("commands=%#v",got) }
}
```

Add cases for Gradle, Jest, Vitest, Playwright, unsupported runners, candidates, and shell-safe Windows/POSIX paths.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan ./internal/agent -run 'Test.*(TestLink|VerificationCommand)' -count=1`

Expected: FAIL because the records and builders do not exist.

- [ ] **Step 3: Implement evidence-backed links and structured commands**

```go
type VerificationCommandRecord struct {
	Tool string `json:"tool"`; WorkingDirectory string `json:"working_directory"`; Args []string `json:"args"`; Display string `json:"display"`
	Confidence string `json:"confidence"`; Reason string `json:"reason"`; MissingPrerequisite string `json:"missing_prerequisite,omitempty"`
}
```

Derive commands only from project metadata and supported test records. Store direct, indirect, inferred, candidate, or not-detected relation in `TestLinkRecord`. Project the same data through canonical flows, task context, dashboard, and Markdown.

- [ ] **Step 4: Run test-link, agent, reconciliation, and dashboard tests**

Run: `go test ./internal/scan ./internal/agent -run 'Test.*(TestLink|Verification|TaskContext)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 21**

```powershell
git add internal/scan/test_verification.go internal/scan/test_verification_test.go internal/scan/types.go internal/scan/canonical_feature_flow.go internal/scan/workspace_reconcile.go internal/agent/service.go internal/agent/service_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: add test linkage and verification commands" -m "- Distinguish confirmed, inferred, candidate, and undetected tests" -m "- Derive structured commands only from supported project metadata" -m "- Share test evidence across flows, agents, and dashboard"
```

### Task 13: Add Evidence-Backed Impact Summaries — Issue 22

**Files:**
- Create: `internal/scan/impact_summary.go`
- Create: `internal/scan/impact_summary_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_service_map.go`
- Modify: `internal/agent/service.go`
- Test: `internal/agent/service_test.go`
- Modify: `internal/mcp/mcp.go`
- Test: `internal/mcp/mcp_test.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `ImpactSummaryRecord`, `ImpactItemRecord`, `BuildImpactSummaries(flows, serviceMap, coverage, maxDepth)` and agent task `impact-summary`/MCP `impact_summary`.

- [ ] **Step 1: Write failing bounded-impact tests**

```go
func TestBuildImpactSummariesSeparatesDirectAndIndirectConsumers(t *testing.T) {
	flows:=[]WorkspaceFeatureFlowRecord{{ID:"a",Nodes:[]CanonicalFlowNodeRecord{{ID:"api",Kind:"api_call"},{ID:"endpoint",Kind:"endpoint"}},Edges:[]CanonicalFlowEdgeRecord{{ID:"e",FromNodeID:"api",ToNodeID:"endpoint",EdgeType:"invokes_api",Confidence:"RESOLVED",Reason:"matched contract"}}}
	got:=BuildImpactSummaries(flows,WorkspaceServiceMapRecord{},WorkspaceCoverageSummaryRecord{},2)
	if len(got)==0 || len(got[0].DirectConsumers)==0 || got[0].RiskLevel=="" { t.Fatalf("impact=%#v",got) }
}
```

Add cycle, depth, truncation, continuation, mixed confidence, high fan-in/out, and missing-coverage cases.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan ./internal/agent ./internal/mcp -run 'Test.*Impact' -count=1`

Expected: FAIL because impact records and tasks do not exist.

- [ ] **Step 3: Implement deterministic bounded traversal**

Separate direct consumers, indirect consumers, tests, packages/apps, and public surface. Compute documented risk levels from depth, public surface, consumer count, test linkage, confidence, and coverage gaps. Add Query/agent/MCP projection with depth, limit, truncation, and continuation; render `Changing this may affect` in details.

- [ ] **Step 4: Run impact, agent, MCP, and dashboard tests**

Run: `go test ./internal/scan ./internal/agent ./internal/mcp -run 'Test.*(Impact|Agent|MCP)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 22**

```powershell
git add internal/scan/impact_summary.go internal/scan/impact_summary_test.go internal/scan/types.go internal/scan/workspace_service_map.go internal/agent/service.go internal/agent/service_test.go internal/mcp/mcp.go internal/mcp/mcp_test.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_test.go
git commit -m "feat: expose evidence-backed impact summaries" -m "- Separate direct, indirect, test, package, and public impact" -m "- Bound traversal with depth, limits, cycles, and continuation" -m "- Share deterministic risk explanations across agent and dashboard"
```

### Task 14: Add Visual And Responsive Acceptance — Issue 17

**Files:**
- Modify: `internal/scan/workspace_dashboard_test.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `docs/design-system.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `COMMANDS.md`
- Modify: `README.md`
- Test: `docs_test.go`

**Interfaces:**
- Consumes: all six dashboard views and their deterministic payload fixtures.
- Produces: representative dense fixture and acceptance assertions for desktop and narrow layouts.

- [ ] **Step 1: Add failing combined acceptance tests**

```go
func TestWorkspaceDashboardCoversSixViewAcceptance(t *testing.T) {
	html:=renderDenseWorkspaceDashboardFixture(t)
	for _, want:=range []string{"Architecture","Endpoints","Feature Flow","Data Flow","Diagnostics","Coverage","@media (max-width:","prefers-reduced-motion","Focus selected","Most useful next scans","Changing this may affect"} {
		if !strings.Contains(html,want) { t.Fatalf("dashboard missing %q",want) }
	}
}
```

Add assertions for visible focus, semantic buttons, long labels, empty states, no horizontal clipping rules, source actions, and view transitions.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/scan -run TestWorkspaceDashboardCoversSixViewAcceptance -count=1`

Expected: FAIL until every combined acceptance marker is present.

- [ ] **Step 3: Complete responsive styles and documentation**

Use the design-system spacing, typography, colors, radii, focus, and reduced-motion tokens. Switch chains to vertical before cards become cramped. Keep three-panel desktop layout and stack details below workbench at narrow width. Document six distinct questions, architecture bundle/focus behavior, normal-scale workbenches, source actions, and coverage uncertainty.

- [ ] **Step 4: Run complete renderer and documentation tests**

Run: `go test ./internal/scan ./... -run 'TestWorkspaceDashboard|TestDocs' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Issue 17**

```powershell
git add internal/scan/workspace_dashboard_test.go internal/scan/workspace_dashboard_styles.go docs/design-system.md docs/OUTPUTS.md COMMANDS.md README.md docs_test.go
git commit -m "test: add dashboard visual acceptance coverage" -m "- Cover all six views with dense deterministic fixtures" -m "- Verify responsive, keyboard, source, and transition behavior" -m "- Document the GoreGraph 1.2 dashboard experience"
```

### Task 15: Prepare And Validate Local GoreGraph 1.2.0

**Files:**
- Modify: `internal/version/version.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `docs/RELEASE.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `SCHEMA.md`
- Test: `release_files_test.go`

**Interfaces:**
- Produces: local version `1.2.0` and release documentation; no tag or push.

- [ ] **Step 1: Change version expectations first**

Update `TestRunVersionPrintsBuildMetadata` and release-file assertions to require `goregraph 1.2.0`, the six dashboard views, canonical flows, prioritized coverage, test commands, and impact summaries.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/cli . -run 'TestRunVersionPrintsBuildMetadata|TestReleaseFiles' -count=1`

Expected: FAIL because the local version remains 1.1.0 and release notes lack 1.2.0.

- [ ] **Step 3: Set version and release documentation**

```go
var (
	Version = "1.2.0"
	Commit = "unknown"
	Date = "unknown"
)
```

Document additive Schema 2 compatibility and all Issue 9–22 outcomes. Do not add tag or push instructions to executed work.

- [ ] **Step 4: Run full source verification**

```powershell
gofmt -w internal
go test ./... -count=1
go vet ./...
git diff --check
go build -o GoreGraph-1.2.0-local.exe ./cmd/goregraph
node --check .goregraph-dashboard-script.js
```

For JavaScript validation, extract the embedded dashboard script to `.goregraph-dashboard-script.js`, run `node --check`, then remove the temporary extraction after confirming exit code 0.

Expected: every command exits 0; formatting reports no changed Go files after the final run.

- [ ] **Step 5: Commit the release state**

```powershell
git add internal/version/version.go internal/cli/cli_test.go README.md docs/RELEASE.md docs/OUTPUTS.md SCHEMA.md release_files_test.go
git commit -m "chore: prepare GoreGraph 1.2.0" -m "- Record canonical workspace intelligence and six-view dashboard behavior" -m "- Preserve additive Schema 2 compatibility" -m "- Document local release acceptance and verification"
```

- [ ] **Step 6: Install and verify the local binary**

```powershell
go install ./cmd/goregraph
Get-Command goregraph | Select-Object -ExpandProperty Source
goregraph version
```

Expected: command resolves to `C:\Users\goretzkh\go\bin\goregraph.exe`; first version line is `goregraph 1.2.0`; schema is 2.

- [ ] **Step 7: Clean and rescan WEKA**

Run from `C:\Users\goretzkh\projects\weka`:

```powershell
goregraph workspace clean . --execute
goregraph workspace scan-all .
```

Expected: clean removes generated project/workspace outputs; scan-all completes every discovered project and regenerates `.goregraph-workspace\workspace-map.html`.

- [ ] **Step 8: Validate generated outputs and browser behavior**

Run Doctor and bounded Query/MCP checks for canonical flows, diagnostics, coverage, test commands, and impact. Inspect Architecture full bundles, explicit focus and return, endpoint source context, Feature Flow, Data Flow, Diagnostics at 100%, Coverage and next scans, and every view transition at desktop and narrow viewport. Record any defect with a failing regression test before fixing it.

- [ ] **Step 9: Close verified GitHub Issues**

Close Issues 9 through 22 only after their focused tests, full verification, installed-binary WEKA scan, and manual visual acceptance all pass. Include each local commit hash in its closure comment and state that no commit was pushed.
