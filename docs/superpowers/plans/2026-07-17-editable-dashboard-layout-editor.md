# Editable Dashboard Layout Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace workspace-specific architecture grouping with persisted package/module grouping, add a secure local layout editor, and fix Code Explorer source-path layout.

**Architecture:** Workspace reconciliation derives deterministic group proposals from indexed production namespaces, then merges `.goregraph-dashboard.json` overrides into the canonical service map. A foreground loopback-only editor server is the sole write path; exported dashboard HTML stays offline and read-only.

**Tech Stack:** Go 1.23 standard library, generated semantic HTML/CSS/vanilla JavaScript, Go tests, Node syntax checks, optional Playwright browser checks.

## Global Constraints

- Work directly on `main`; do not create a feature branch or worktree.
- Use failing-first TDD for every behavior change.
- Keep the source target at unreleased `1.3.0`; do not tag, publish, or create a release.
- Do not add dependencies; use the Go standard library and existing dashboard runtime.
- Never hardcode organization names such as VD, WPO, Hekate, Portal, or RDBV.
- Static generated dashboards remain offline and read-only.
- `.goregraph-dashboard.json` contains presentation choices only and is never projected into agent context.
- Preserve existing user changes and commit each task independently with an English imperative commit message and explanatory bullets.

## File Structure

- Create `internal/scan/workspace_dashboard_config.go`: strict schema, validation, revision, atomic load/save.
- Create `internal/scan/workspace_dashboard_config_test.go`: configuration and persistence tests.
- Create `internal/scan/workspace_grouping.go`: production namespace facts, automatic grouping, override merge.
- Create `internal/scan/workspace_grouping_test.go`: neutral Java and JS/TS grouping fixtures and ordering tests.
- Modify `internal/scan/types.go`: architecture group and node metadata records.
- Modify `internal/scan/workspace_service_map.go`: consume canonical layout instead of keyword domains.
- Modify `internal/scan/workspace_reconcile.go`: load config, derive namespaces, apply layout, preserve prior dashboard on invalid config.
- Modify `internal/doctor/doctor.go`: validate config and report stale overrides.
- Create `internal/doctor/dashboard_config_test.go`: Doctor config diagnostics.
- Create `internal/dashboardeditor/server.go`: loopback editor lifecycle and allowlisted JSON API.
- Create `internal/dashboardeditor/server_test.go`: server security, conflict, and shutdown tests.
- Modify `internal/cli/cli.go` and `internal/cli/cli_test.go`: `workspace dashboard edit` command and help.
- Modify `internal/scan/workspace_dashboard.go`: include edit metadata in the dashboard payload.
- Modify `internal/scan/workspace_dashboard_template.go`: edit-mode controls and semantic structure.
- Modify `internal/scan/workspace_dashboard_script.go`: drag-and-drop, keyboard moves, save/discard/reset, conflict states.
- Modify `internal/scan/workspace_dashboard_architecture.go`: order layout by canonical groups and service order.
- Modify `internal/scan/workspace_dashboard_styles.go`: editor and source-path layout styles.
- Modify `internal/scan/workspace_dashboard_test.go` and `internal/scan/workspace_dashboard_architecture_test.go`: DOM, JS, persistence, responsive, and accessibility contracts.

---

### Task 1: Strict Workspace Dashboard Configuration

**Files:**
- Create: `internal/scan/workspace_dashboard_config.go`
- Create: `internal/scan/workspace_dashboard_config_test.go`

**Interfaces:**
- Produces: `WorkspaceDashboardConfig`, `DashboardArchitectureConfig`, `DashboardGroupConfig`, `DashboardServiceConfig`.
- Produces: `LoadWorkspaceDashboardConfig(root string) (WorkspaceDashboardConfig, string, error)`.
- Produces: `SaveWorkspaceDashboardConfig(root, expectedRevision string, config WorkspaceDashboardConfig) (string, error)`.
- Produces: `ValidateWorkspaceDashboardConfig(config WorkspaceDashboardConfig) error`.

- [ ] **Step 1: Write failing schema and round-trip tests**

```go
func TestWorkspaceDashboardConfigRoundTripAndConflict(t *testing.T) {
	root := t.TempDir()
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		GroupOrder: []string{"org.example.alpha"},
		Groups: map[string]DashboardGroupConfig{"org.example.alpha": {Label: "Alpha"}},
		Services: map[string]DashboardServiceConfig{"services/api": {Group: "org.example.alpha", Order: 20}},
	}}
	revision, err := SaveWorkspaceDashboardConfig(root, "missing", config)
	if err != nil { t.Fatal(err) }
	loaded, loadedRevision, err := LoadWorkspaceDashboardConfig(root)
	if err != nil { t.Fatal(err) }
	if revision != loadedRevision || !reflect.DeepEqual(config, loaded) {
		t.Fatalf("round trip mismatch: revision=%q loaded=%#v", loadedRevision, loaded)
	}
	if _, err := SaveWorkspaceDashboardConfig(root, "missing", config); !errors.Is(err, ErrDashboardConfigConflict) {
		t.Fatalf("stale save error = %v", err)
	}
}

func TestWorkspaceDashboardConfigRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, WorkspaceDashboardConfigName)
	if err := os.WriteFile(path, []byte(`{"schema":1,"architecture":{},"secret":"no"}`), 0o644); err != nil { t.Fatal(err) }
	if _, _, err := LoadWorkspaceDashboardConfig(root); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboardConfig' -count=1`

Expected: FAIL because the configuration types and functions do not exist.

- [ ] **Step 3: Implement strict validation, revisions, and atomic writes**

Add these public contracts and keep filesystem helpers private:

```go
const WorkspaceDashboardConfigName = ".goregraph-dashboard.json"
const missingDashboardConfigRevision = "missing"

var ErrDashboardConfigConflict = errors.New("workspace dashboard configuration changed")

type WorkspaceDashboardConfig struct {
	Schema       int                         `json:"schema"`
	Architecture DashboardArchitectureConfig `json:"architecture"`
}

type DashboardArchitectureConfig struct {
	GroupOrder []string                          `json:"groupOrder,omitempty"`
	Groups     map[string]DashboardGroupConfig   `json:"groups,omitempty"`
	Services   map[string]DashboardServiceConfig `json:"services,omitempty"`
}

type DashboardGroupConfig struct { Label string `json:"label"` }
type DashboardServiceConfig struct {
	Group string `json:"group"`
	Order int    `json:"order,omitempty"`
}
```

`LoadWorkspaceDashboardConfig` must return an empty schema-1 config plus revision `missing` when the file does not exist. Decode with `json.Decoder.DisallowUnknownFields`, require EOF after the first object, reject schemas other than `1`, empty/absolute/traversing service keys, blank configured group IDs, blank labels, service group references absent from `Groups`, and duplicate `GroupOrder` values. Compute revisions as lowercase SHA-256 of exact file bytes. Save with `os.CreateTemp(root, ".goregraph-dashboard-*.tmp")`, mode `0644`, `Sync`, `Close`, and `os.Rename`; remove the temp file on every error.

- [ ] **Step 4: Run focused and package tests and verify GREEN**

Run: `gofmt -w internal/scan/workspace_dashboard_config.go internal/scan/workspace_dashboard_config_test.go && go test ./internal/scan -run 'TestWorkspaceDashboardConfig' -count=1`

Expected: PASS, including exact conflict and strict-decoder assertions.

- [ ] **Step 5: Commit the configuration boundary**

```bash
git add internal/scan/workspace_dashboard_config.go internal/scan/workspace_dashboard_config_test.go
git commit -m "Add workspace dashboard configuration" -m "- Validate the schema and workspace-relative service identities\n- Persist revisions through conflict-safe atomic writes"
```

### Task 2: Generic Production Namespace Grouping

**Files:**
- Create: `internal/scan/workspace_grouping.go`
- Create: `internal/scan/workspace_grouping_test.go`
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_service_map.go`

**Interfaces:**
- Consumes: `WorkspaceDashboardConfig` from Task 1.
- Produces: `WorkspaceProjectNamespaceRecord` and `WorkspaceArchitectureLayoutRecord`.
- Produces: `BuildWorkspaceArchitectureLayout(registry WorkspaceRegistryRecord, namespaces []WorkspaceProjectNamespaceRecord, config WorkspaceDashboardConfig) WorkspaceArchitectureLayoutRecord`.
- Produces: `BuildWorkspaceServiceMapWithLayout(registry WorkspaceRegistryRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, dependencies []WorkspaceServiceDependencyRecord, layout WorkspaceArchitectureLayoutRecord) WorkspaceServiceMapRecord`.

- [ ] **Step 1: Write failing neutral grouping and override tests**

```go
func TestBuildWorkspaceArchitectureLayoutUsesFirstDifferentiatingProductionNamespace(t *testing.T) {
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "services/orders", Name: "orders", Indexed: true},
		{Path: "services/billing", Name: "billing", Indexed: true},
	}}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: "services/orders", Namespace: "org.example.commerce.orders", Language: "java", Source: "production_package"},
		{Project: "services/billing", Namespace: "org.example.finance.billing", Language: "java", Source: "production_package"},
	}
	layout := BuildWorkspaceArchitectureLayout(registry, namespaces, WorkspaceDashboardConfig{Schema: 1})
	if got := layout.Service("services/orders").GroupID; got != "org.example.commerce" { t.Fatalf("orders group = %q", got) }
	if got := layout.Service("services/billing").GroupID; got != "org.example.finance" { t.Fatalf("billing group = %q", got) }
}

func TestBuildWorkspaceArchitectureLayoutKeepsManualOrderAndPlacesNewServices(t *testing.T) {
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		GroupOrder: []string{"custom"}, Groups: map[string]DashboardGroupConfig{"custom": {Label: "Core"}},
		Services: map[string]DashboardServiceConfig{"services/orders": {Group: "custom", Order: 10}},
	}}
	registry := WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "services/orders"}, {Path: "services/new"}}}
	layout := BuildWorkspaceArchitectureLayout(registry, nil, config)
	if !layout.Service("services/orders").Manual || layout.Groups[0].Label != "Core" { t.Fatalf("manual layout lost: %#v", layout) }
	if layout.Service("services/new").GroupID == "" { t.Fatalf("new service was not auto-placed: %#v", layout) }
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/scan -run 'TestBuildWorkspaceArchitectureLayout' -count=1`

Expected: FAIL because namespace/layout records and builder do not exist.

- [ ] **Step 3: Implement deterministic inference and service-map metadata**

Add records with these exact JSON fields:

```go
type WorkspaceArchitectureGroupRecord struct {
	ID string `json:"id"`; Label string `json:"label"`; Order int `json:"order"`
	Source string `json:"source,omitempty"`; Confidence string `json:"confidence,omitempty"`; Manual bool `json:"manual,omitempty"`
}
type WorkspaceProjectNamespaceRecord struct {
	Project string; Namespace string; Language string; Source string; Confidence string
}
type WorkspaceArchitectureServiceLayoutRecord struct {
	Project string; GroupID string; Order int; Source string; Confidence string; Manual bool
}
type WorkspaceArchitectureLayoutRecord struct {
	Groups []WorkspaceArchitectureGroupRecord
	Services []WorkspaceArchitectureServiceLayoutRecord
	StaleServices []string
}
```

Extend `WorkspaceServiceMapRecord` with `ArchitectureGroups []WorkspaceArchitectureGroupRecord` and `WorkspaceServiceNodeRecord` with `ArchitectureOrder`, `DomainSource`, `DomainConfidence`, and `DomainManual`. Infer namespaces only from paths classified as production; normalize Java dots and JS/TS package slashes into segments; remove shared prefixes; select the shortest prefix that meaningfully separates projects; fall back to role plus workspace parent path with `PARTIAL` confidence. Sort groups by configured order then ID, and services by configured order then label/project/ID. Remove `workspaceServiceDomain` keyword matching entirely.

- [ ] **Step 4: Run grouping and service-map regressions**

Run: `gofmt -w internal/scan/workspace_grouping.go internal/scan/workspace_grouping_test.go internal/scan/types.go internal/scan/workspace_service_map.go && go test ./internal/scan -run 'WorkspaceArchitectureLayout|WorkspaceServiceMap' -count=1`

Expected: PASS with no organization-specific domain strings in production code.

- [ ] **Step 5: Commit generic grouping**

```bash
git add internal/scan/workspace_grouping.go internal/scan/workspace_grouping_test.go internal/scan/types.go internal/scan/workspace_service_map.go
git commit -m "Derive architecture groups from source namespaces" -m "- Replace keyword domains with deterministic production package evidence\n- Merge stable manual group and service ordering metadata"
```

### Task 3: Reconciliation and Doctor Integration

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/doctor/doctor.go`
- Create: `internal/doctor/dashboard_config_test.go`

**Interfaces:**
- Consumes: Task 1 configuration loader and Task 2 layout builder.
- Produces: `workspaceProjectNamespaces(indexed []workspaceIndexProject) []WorkspaceProjectNamespaceRecord`.
- Produces: Doctor failures for invalid config and warnings for stale services.

- [ ] **Step 1: Write failing reconciliation and Doctor tests**

```go
func TestWorkspaceReconcileAppliesDashboardConfigAcrossRefresh(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetDashboard)
	project := projects[0]
	config := WorkspaceDashboardConfig{Schema: 1, Architecture: DashboardArchitectureConfig{
		GroupOrder: []string{"custom"}, Groups: map[string]DashboardGroupConfig{"custom": {Label: "Custom"}},
		Services: map[string]DashboardServiceConfig{workspaceRel(workspace, project): {Group: "custom", Order: 7}},
	}}
	if _, err := SaveWorkspaceDashboardConfig(workspace, "missing", config); err != nil { t.Fatal(err) }
	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	if _, err := ReconcileWorkspaceTarget(project, cfg, BuildTargetDashboard); err != nil { t.Fatal(err) }
	var serviceMap WorkspaceServiceMapRecord
	path := NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Index("workspace-service-map.json")
	if err := readWorkspaceJSON(path, &serviceMap); err != nil { t.Fatal(err) }
	if serviceMap.Nodes[0].Domain != "custom" || !serviceMap.Nodes[0].DomainManual { t.Fatalf("config not applied: %#v", serviceMap.Nodes[0]) }
}
```

Add Doctor fixtures asserting an invalid group reference is a failure containing `.goregraph-dashboard.json`, while a removed service override is a warning containing the relative service path.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./internal/scan ./internal/doctor -run 'DashboardConfig|AppliesDashboardConfig' -count=1`

Expected: FAIL because reconciliation and Doctor do not load the sidecar.

- [ ] **Step 3: Wire namespaces, config, failure preservation, and diagnostics**

Build namespace records from `workspaceIndexProject.symbols`: use `Package`, then `WorkspacePackage`, then `Module`, and exclude paths recognized as tests, generated output, vendored source, or build output. In `ReconcileWorkspaceTarget`, load and validate config before writing an incomplete manifest or removing dashboard output. Build layout and call `BuildWorkspaceServiceMapWithLayout`. Return errors prefixed with `.goregraph-dashboard.json:` so the last complete dashboard remains untouched.

In Doctor, call the same loader and validator at workspace scope. Invalid JSON/schema is a failure. Compare configured service keys with registry projects and emit one warning per stale key. Do not mutate or delete the config.

- [ ] **Step 4: Run focused and reconciliation suites**

Run: `gofmt -w internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/doctor/doctor.go internal/doctor/dashboard_config_test.go && go test ./internal/scan ./internal/doctor -run 'DashboardConfig|WorkspaceReconcile' -count=1`

Expected: PASS; corrupt configuration leaves the previous dashboard manifest and HTML unchanged.

- [ ] **Step 5: Commit reconciliation integration**

```bash
git add internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/doctor/doctor.go internal/doctor/dashboard_config_test.go
git commit -m "Apply dashboard layout during reconciliation" -m "- Derive production namespace evidence from indexed projects\n- Report invalid and stale layout configuration without destructive fallback"
```

### Task 4: Secure Loopback Editor Server

**Files:**
- Create: `internal/dashboardeditor/server.go`
- Create: `internal/dashboardeditor/server_test.go`

**Interfaces:**
- Consumes: `scan.LoadWorkspaceDashboardConfig` and `scan.SaveWorkspaceDashboardConfig`.
- Produces: `Options{WorkspaceRoot, DashboardPath string; OpenURL func(string) error; OnReady func(string)}`.
- Produces: `Serve(ctx context.Context, options Options) error`.
- HTTP: `GET /api/config` and `PUT /api/config`; known dashboard assets only.

- [ ] **Step 1: Write failing loopback, authorization, traversal, and conflict tests**

```go
func TestServerRejectsUnauthorizedWritesAndRevisionConflicts(t *testing.T) {
	server := newTestServer(t)
	request, _ := http.NewRequest(http.MethodPut, server.URL+"/api/config", strings.NewReader(`{"revision":"missing","config":{"schema":1,"architecture":{}}}`))
	response, err := http.DefaultClient.Do(request)
	if err != nil { t.Fatal(err) }
	if response.StatusCode != http.StatusUnauthorized { t.Fatalf("status = %d", response.StatusCode) }
	authorized := server.request(t, http.MethodPut, "/api/config", `{"revision":"stale","config":{"schema":1,"architecture":{}}}`)
	if authorized.StatusCode != http.StatusConflict { t.Fatalf("conflict status = %d", authorized.StatusCode) }
}

func TestServerRejectsTraversalAndNonLoopbackHost(t *testing.T) {
	server := newTestServer(t)
	if status := server.request(t, http.MethodGet, "/../../README.md", "").StatusCode; status != http.StatusNotFound { t.Fatalf("traversal status = %d", status) }
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	request.Host = "attacker.example"
	response, _ := http.DefaultClient.Do(request)
	if response.StatusCode != http.StatusForbidden { t.Fatalf("host status = %d", response.StatusCode) }
}

type testEditorServer struct {
	t *testing.T
	URL string
	Token string
	cancel context.CancelFunc
	done chan error
}

func newTestServer(t *testing.T) *testEditorServer {
	t.Helper()
	root := t.TempDir()
	dashboardPath := filepath.Join(root, ".goregraph-workspace", "dashboard", "workspace-map.html")
	if err := os.MkdirAll(filepath.Dir(dashboardPath), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(dashboardPath, []byte("<!doctype html><title>fixture</title>"), 0o644); err != nil { t.Fatal(err) }
	ready := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, Options{WorkspaceRoot: root, DashboardPath: dashboardPath, OnReady: func(value string) { ready <- value }})
	}()
	rawURL := <-ready
	parsed, err := url.Parse(rawURL)
	if err != nil { t.Fatal(err) }
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil { t.Fatal(err) }
	testServer := &testEditorServer{t: t, URL: "http://"+parsed.Host, Token: fragment.Get("token"), cancel: cancel, done: done}
	t.Cleanup(func() { cancel(); if err := <-done; err != nil { t.Errorf("Serve()=%v", err) } })
	return testServer
}

func (server *testEditorServer) request(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil { t.Fatal(err) }
	request.Header.Set("X-GoreGraph-Editor-Token", server.Token)
	request.Header.Set("Origin", server.URL)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil { t.Fatal(err) }
	return response
}
```

- [ ] **Step 2: Run server tests and verify RED**

Run: `go test ./internal/dashboardeditor -count=1`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the foreground server boundary**

Use `net.Listen("tcp", "127.0.0.1:0")`, `crypto/rand` for a 32-byte hex token, an `http.Server` with read-header timeout, explicit routes, 1 MiB `http.MaxBytesReader`, JSON content-type checks, constant-time token comparison, exact loopback Host/Origin validation, and no CORS response. Serve `/` from the selected `workspace-map.html` and only its sibling generated assets after `filepath.Rel` plus traversal rejection. Publish the URL as `http://127.0.0.1:<port>/#token=<hex>` through `OnReady`, then invoke `OpenURL` when non-nil. The config API response is:

```json
{"revision":"<sha256-or-missing>","config":{"schema":1,"architecture":{}}}
```

Require `X-GoreGraph-Editor-Token` for API calls, pass the token in the editor URL fragment so it is not sent as a referrer, and inject only an `editor_enabled` boolean plus API base into the served HTML response. On `context.Context` cancellation, call `Shutdown` with a five-second timeout. Map validation to 400, missing token to 401, host/origin to 403, revision conflict to 409, oversized body to 413, and unexpected filesystem errors to 500 without disclosing unrelated absolute paths.

- [ ] **Step 4: Run race-sensitive server tests**

Run: `gofmt -w internal/dashboardeditor/server.go internal/dashboardeditor/server_test.go && go test -race ./internal/dashboardeditor -count=1`

Expected: PASS with the listener address starting with `127.0.0.1:` and clean shutdown.

- [ ] **Step 5: Commit the editor server**

```bash
git add internal/dashboardeditor/server.go internal/dashboardeditor/server_test.go
git commit -m "Add secure local dashboard editor server" -m "- Restrict editing to an authenticated loopback session\n- Validate revisions and persist configuration atomically"
```

### Task 5: CLI Edit Command and Help

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `dashboardeditor.Serve` from Task 4.
- Produces: `goregraph workspace dashboard edit [path] [--workspace <path>]`.

- [ ] **Step 1: Add failing CLI parsing and help tests**

```go
func TestWorkspaceDashboardHelpExplainsStaticAndEditableModes(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if code := Run([]string{"workspace", "dashboard", "help"}, stdout, stderr); code != 0 { t.Fatalf("code=%d stderr=%s", code, stderr) }
	for _, want := range []string{"path|open|edit", "read-only", "loopback", ".goregraph-dashboard.json", "Ctrl-C"} {
		if !strings.Contains(stdout.String(), want) { t.Fatalf("help missing %q:\n%s", want, stdout) }
	}
}
```

Add an injected `serveDashboardEditor` function variable in tests so `edit` can assert resolved workspace/dashboard paths without starting a real blocking process.

- [ ] **Step 2: Run CLI tests and verify RED**

Run: `go test ./internal/cli -run 'WorkspaceDashboard' -count=1`

Expected: FAIL because `edit` is treated as a path.

- [ ] **Step 3: Wire the edit action**

Recognize exactly `path`, `open`, and `edit`. Resolve the workspace with current marker and `--workspace` behavior. Require an existing complete workspace dashboard. For `edit`, create a signal-aware context, print `Dashboard editor: <url>` from the server start callback, invoke the existing platform opener, remain foreground, and return exit code 1 with `error: dashboard editor failed:` on server errors. Update global workspace help and examples without changing compatibility `workspace dashboard .` behavior.

- [ ] **Step 4: Run CLI and help regressions**

Run: `gofmt -w internal/cli/cli.go internal/cli/cli_test.go && go test ./internal/cli -count=1`

Expected: PASS; existing `path` and `open` tests remain unchanged.

- [ ] **Step 5: Commit CLI support**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "Expose workspace dashboard edit mode" -m "- Start the authenticated editor as a foreground command\n- Clarify static path, open, and editable dashboard behavior"
```

### Task 6: Architecture Edit Mode

**Files:**
- Modify: `internal/scan/workspace_dashboard.go`
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_architecture.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_architecture_test.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Consumes: `service_map.architecture_groups` and node layout metadata from Task 2.
- Consumes: Task 4 `GET/PUT /api/config` only when editor mode is enabled.
- Produces: `enterArchitectureEditMode`, `moveArchitectureGroup`, `moveArchitectureService`, `saveArchitectureLayout`, `discardArchitectureLayout`, and `resetArchitectureLayout` JavaScript functions.

- [ ] **Step 1: Add failing dashboard contract tests**

```go
func TestWorkspaceDashboardArchitectureEditorIsExplicitAndAccessible(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)
	for _, want := range []string{
		`id="architecture-edit-layout"`, `id="architecture-save-layout"`, `id="architecture-discard-layout"`,
		`id="architecture-reset-layout"`, `aria-live="polite"`, `function moveArchitectureGroup(`,
		`function moveArchitectureService(`, `X-GoreGraph-Editor-Token`, `data-layout-mode="manual"`,
	} {
		if !strings.Contains(html, want) { t.Fatalf("dashboard missing %q", want) }
	}
}
```

Add a Node DOM-independent test for pure reorder helpers and a Playwright test that enters edit mode, moves a service with keyboard controls, saves through a stubbed `fetch`, and verifies that normal Architecture selection never issues a request.

- [ ] **Step 2: Run dashboard tests and verify RED**

Run: `go test ./internal/scan -run 'ArchitectureEditor|ArchitectureLayout' -count=1`

Expected: FAIL because controls and edit state are absent.

- [ ] **Step 3: Implement editable layout without changing static mode**

Render `Edit layout` only when `editor_enabled` is true. On entry, clone group/service order into `state.architectureDraft`; normal graph selection is disabled only for draggable items. Add HTML buttons for move earlier/later, move to previous/next group, and group label editing so every pointer operation has a keyboard equivalent. Use native pointer drag events with one draft update on drop, not continuous persistence.

`saveArchitectureLayout` serializes only group labels/order and manual service group/order, sends the loaded revision, updates revision/config on 200, reports field errors on 400, preserves draft and prompts reload on 409, and never reloads the page implicitly. Discard restores loaded config. Reset sends schema 1 with empty architecture maps after confirmation. Static HTML has no enabled controls, no token, and no writable fetch path.

Update `architectureLayout` to sort groups by canonical `order` and services by `architecture_order`, then label/project/ID. Group headers show editable label plus `Auto`/`Manual` only in edit mode.

- [ ] **Step 4: Run JS parse, dashboard, and browser tests**

Run: `gofmt -w internal/scan/workspace_dashboard*.go && go test ./internal/scan -run 'WorkspaceDashboard.*Architecture|ArchitectureEditor|EmbeddedJavaScript' -count=1`

Expected: PASS; optional Playwright checks run when `require.resolve("playwright")` succeeds and otherwise skip with the existing convention.

- [ ] **Step 5: Commit architecture editing**

```bash
git add internal/scan/workspace_dashboard.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_test.go
git commit -m "Make architecture layout editable" -m "- Add accessible group and service reordering in explicit edit mode\n- Persist intentional layout changes without affecting offline dashboards"
```

### Task 7: Code Explorer Source Path Layout

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: consistent `.source-path` followed by `.source-actions` in inventory, selected-symbol summary, and details panel.

- [ ] **Step 1: Add failing source-layout regression test**

```go
func TestWorkspaceDashboardCodeExplorerSeparatesLongPathsFromActions(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion}, denseArchitectureFixture(),
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion, Symbols: []CanonicalSymbolRecord{{
			ID: "symbol:repository", Project: "services/catalog", Name: "CatalogRepository",
			Language: "java", Kind: "interface", QualifiedName: "org.example.catalog.persistence.CatalogRepository",
			DeclarationFile: "src/main/java/org/example/catalog/persistence/CatalogRepository.java", DeclarationLine: 10,
			Analyzer: "java", Confidence: ConfidenceExact, Coverage: CoverageComplete,
		}}}, WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)
	for _, want := range []string{`.code-symbol-summary .source-path`, `.details .source-actions`, `overflow-wrap:anywhere`, `min-width:0`} {
		if !strings.Contains(html, want) { t.Fatalf("source layout missing %q", want) }
	}
	if strings.Contains(html, `sourcePath+sourceActionButton`) { t.Fatal("path and action are still concatenated") }
}
```

Add Playwright geometry assertions at widths 1280, 1440, and 1920: source action top must be greater than path bottom minus one pixel, and `document.documentElement.scrollWidth <= innerWidth`.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./internal/scan -run 'CodeExplorerSeparatesLongPaths' -count=1`

Expected: FAIL because selected summary and details still combine path/actions.

- [ ] **Step 3: Render source path and actions as sibling blocks**

Create a single JavaScript renderer returning:

```js
function sourceLocationMarkup(file,line){
  const path=sourceLocation(file,line);
  return '<div class="source-location"><div class="source-path">'+escapeHtml(path)+'</div><div class="source-actions">'+sourceActionButtons(file,line)+'</div></div>';
}
```

Use it for inventory rows, center selected-symbol header, and right details. Change the selected-symbol summary to one column so the full path sits under name and metadata. Apply `min-width:0`, `white-space:normal`, and `overflow-wrap:anywhere`; keep buttons in a flex-wrapping action row.

- [ ] **Step 4: Run focused and full dashboard tests**

Run: `gofmt -w internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go && go test ./internal/scan -run 'WorkspaceDashboard.*CodeExplorer|EmbeddedJavaScript' -count=1`

Expected: PASS with no horizontal page overflow in the browser geometry checks.

- [ ] **Step 5: Commit the Code Explorer correction**

```bash
git add internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_test.go
git commit -m "Fix Code Explorer source path layout" -m "- Place long paths below symbol metadata with wrapping\n- Keep source actions on a separate consistent row"
```

### Task 8: Layout Editor Integration Verification

**Files:**
- Modify only if a discovered regression requires a focused correction in files already owned by Tasks 1-7.

**Interfaces:**
- Verifies the complete layout-editor subsystem before API Catalog work begins.

- [ ] **Step 1: Run package tests with race detection where relevant**

Run: `go test ./internal/scan ./internal/doctor ./internal/cli && go test -race ./internal/dashboardeditor`

Expected: PASS.

- [ ] **Step 2: Run the complete Go regression suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Build and smoke-test the foreground editor**

Run: `go build ./cmd/goregraph`

Expected: PASS and a local `goregraph` binary. Against a temporary indexed workspace, start `./goregraph workspace dashboard edit <fixture>`, verify the printed URL uses `127.0.0.1`, save one rename, rebuild the dashboard, and verify the label persists.

- [ ] **Step 4: Perform responsive visual inspection**

Open the fixture editor at 1280×720, 1440×900, and 1920×1080. Record that Architecture normal mode, edit mode, drag affordances, keyboard controls, conflict/error status, and Code Explorer long paths have no overlap, clipping, or horizontal page scroll.

- [ ] **Step 5: Commit only if verification required a correction**

Use one focused commit naming the observed regression. If no correction was required, do not create an empty commit and record the passing commands in the implementation handoff.
