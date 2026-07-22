# Dashboard Navigation and Monorepo Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make workspace dashboard navigation collapsible and persistent, expose the secure Architecture editor, and keep multi-package repositories grouped as one service while excluding `.worktrees` content.

**Architecture:** Preserve the generated self-contained dashboard and existing loopback editor. Add small browser-state helpers around the existing shell, enrich transient namespace evidence with its package unit so grouping can recognize repository-level monorepos, and route the concise top-level edit command into the existing authenticated editor implementation.

**Tech Stack:** Go 1.25, Go standard library, embedded HTML/CSS/vanilla JavaScript, existing Node/Playwright dashboard tests.

## Global Constraints

- Keep the source version at unreleased `1.3.0`.
- Do not add a runtime, frontend framework, Python requirement, or third-party dependency.
- Keep generated dashboards self-contained and offline-readable.
- Keep writable editing loopback-only, token-authenticated, revision-checked, and atomic.
- Keep repository/project roots as service boundaries on Windows, macOS, and Linux.
- Preserve the existing uncommitted resolver work in `internal/cli/cli.go`, `internal/cli/cli_test.go`, `README.md`, and `COMMANDS.md`.
- Do not create a tag, GitHub Release, package publication, or other release artifact.

---

### Task 1: Exclude nested worktree directories from every project scan

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/scan/filter.go`
- Modify: `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: `config.Defaults()`, `shouldSkipPath(rel string, isDir bool, cfg config.Config, matcher gitignore.Matcher)`.
- Produces: `.worktrees/` as a recursive default exclusion honored at any depth.

- [ ] **Step 1: Write failing default-config and nested-scan tests**

Add `.worktrees/` to the expected defaults in `TestDefaults` and add a scan fixture that writes `src/app.ts` plus `.worktrees/branch/src/copied.ts`, runs `Run`, and asserts that only `src/app.ts` appears in `result.Index.Files`.

```go
func TestRunExcludesNestedWorktreeDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name":"app"}`)
	writeFile(t, root, "src/app.ts", "export const app = true\n")
	writeFile(t, root, ".worktrees/branch/src/copied.ts", "export const copied = true\n")

	result, err := Run(root, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range result.Index.Files {
		if strings.Contains(filepath.ToSlash(file.Path), ".worktrees/") {
			t.Fatalf("worktree file was indexed: %#v", file)
		}
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/config ./internal/scan -run 'TestDefaults|TestRunExcludesNestedWorktreeDirectories' -count=1`

Expected: FAIL because `.worktrees/` is not a default recursive exclusion and the copied TypeScript file is indexed.

- [ ] **Step 3: Add the recursive exclusion**

Add `".worktrees/"` to `config.Defaults().Exclude` and `".worktrees"` to `isRecursiveDefaultDirectoryExclude`.

```go
case ".git", ".worktrees", "node_modules", "vendor", "target", "build", "dist", "coverage", ".idea", ".vscode", "goregraph-out", ".goregraph-workspace":
	return true
```

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run: `go test ./internal/config ./internal/scan -run 'TestDefaults|TestRunExcludesNestedWorktreeDirectories' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the scan exclusion**

```text
Exclude worktree directories from project scans

- Treat .worktrees as a recursive default exclusion
- Verify nested worktree sources never enter project indexes
```

---

### Task 2: Treat multi-package repositories as repository-boundary services

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/scan/workspace_grouping.go`
- Modify: `internal/scan/workspace_grouping_test.go`

**Interfaces:**
- Consumes: `RichSymbolRecord.Package`, `RichSymbolRecord.WorkspacePackage`, `RichSymbolRecord.Artifact`, existing manual dashboard config.
- Produces: `WorkspaceProjectNamespaceRecord.PackageUnit string`, `workspaceMultiPackageProjects([]WorkspaceProjectNamespaceRecord) map[string]bool`, repository-path fallback for projects with more than one non-empty package unit.

- [ ] **Step 1: Write failing namespace provenance tests**

Extend `TestWorkspaceProjectNamespacesUsesProductionSymbols` so TypeScript records retain their workspace package and Java records retain their artifact as `PackageUnit`. Add two distinct frontend workspace packages.

```go
want := []WorkspaceProjectNamespaceRecord{
	{Project: "services/orders", Namespace: "@example/commerce/orders", PackageUnit: "@example/commerce/orders", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
	{Project: "services/orders", Namespace: "@example/shared/ui", PackageUnit: "@example/shared/ui", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
	{Project: "services/orders", Namespace: "org.example.commerce.orders", PackageUnit: "org.example:orders", Language: "java", Source: "production_package", Confidence: "EXTRACTED"},
}
```

- [ ] **Step 2: Write failing grouping tests**

Add one test with `frontend/frontend-monorepo` and two `PackageUnit` values that expects `frontend:frontend`, plus regressions showing that coherent Java packages with one artifact still resolve to their package group and manual config still wins.

```go
func TestBuildWorkspaceArchitectureLayoutUsesRepositoryBoundaryForMultiPackageProject(t *testing.T) {
	project := WorkspaceProjectRecord{Path: "frontend/frontend-monorepo", Name: "frontend-monorepo", Kind: "frontend"}
	namespaces := []WorkspaceProjectNamespaceRecord{
		{Project: project.Path, Namespace: "@example/designsystem", PackageUnit: "@example/designsystem", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
		{Project: project.Path, Namespace: "@example/portal", PackageUnit: "@example/portal", Language: "typescript", Source: "production_package", Confidence: "EXTRACTED"},
	}
	layout := BuildWorkspaceArchitectureLayout(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{project}}, namespaces, WorkspaceDashboardConfig{Schema: 1})
	service := layout.Service(project.Path)
	if service.GroupID != "frontend:frontend" || service.Source != "workspace_path" {
		t.Fatalf("multi-package layout = %#v", service)
	}
}
```

- [ ] **Step 3: Run grouping tests and verify RED**

Run: `go test ./internal/scan -run 'TestWorkspaceProjectNamespacesUsesProductionSymbols|TestBuildWorkspaceArchitectureLayoutUsesRepositoryBoundaryForMultiPackageProject' -count=1`

Expected: FAIL because package-unit provenance and multi-package fallback do not exist.

- [ ] **Step 4: Add package-unit provenance**

Add the field and derive it only from the unit that owns the selected namespace.

```go
type WorkspaceProjectNamespaceRecord struct {
	Project     string `json:"project"`
	Namespace   string `json:"namespace"`
	PackageUnit string `json:"package_unit,omitempty"`
	Language    string `json:"language"`
	Source      string `json:"source"`
	Confidence  string `json:"confidence"`
}

func workspaceSymbolNamespace(symbol RichSymbolRecord) (string, string) {
	if namespace := strings.TrimSpace(symbol.Package); namespace != "" {
		return namespace, strings.TrimSpace(symbol.Artifact)
	}
	if namespace := strings.TrimSpace(symbol.WorkspacePackage); namespace != "" {
		return namespace, namespace
	}
	return strings.TrimSpace(symbol.Module), ""
}
```

Include `PackageUnit` in deduplication and deterministic sort keys.

- [ ] **Step 5: Add deterministic multi-package fallback**

```go
func workspaceMultiPackageProjects(namespaces []WorkspaceProjectNamespaceRecord) map[string]bool {
	units := map[string]map[string]bool{}
	for _, namespace := range namespaces {
		unit := strings.TrimSpace(namespace.PackageUnit)
		if unit == "" || !strings.EqualFold(strings.TrimSpace(namespace.Source), "production_package") {
			continue
		}
		if units[namespace.Project] == nil {
			units[namespace.Project] = map[string]bool{}
		}
		units[namespace.Project][unit] = true
	}
	result := map[string]bool{}
	for project, projectUnits := range units {
		result[project] = len(projectUnits) > 1
	}
	return result
}
```

Skip package-derived candidates for projects marked by this helper so `workspaceFallbackServiceLayout` remains authoritative. Manual configuration continues to run before inference.

- [ ] **Step 6: Run grouping and reconciliation tests**

Run: `go test ./internal/scan -run 'TestWorkspaceProjectNamespaces|TestBuildWorkspaceArchitectureLayout|TestBuildWorkspaceServiceMapWithLayout' -count=1`

Expected: PASS, including existing deterministic and manual-override fixtures.

- [ ] **Step 7: Commit monorepo grouping**

```text
Keep multi-package repositories in workspace groups

- Track the owning package or artifact in namespace evidence
- Fall back to the repository boundary when multiple units share one service
- Preserve single-package and manual architecture grouping
```

---

### Task 3: Add independent persistent dashboard panel controls

**Files:**
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Create: `internal/scan/workspace_dashboard_chrome_test.go`

**Interfaces:**
- Consumes: `workspacePayload.graph.root`, current `.shell`, `.side`, `main`, and `#details` elements.
- Produces: `loadDashboardPanelState(storage, key)`, `saveDashboardPanelState(storage, key, value)`, `setDashboardPanelVisibility(side, visible)`, and two always-reachable toggle buttons.

- [ ] **Step 1: Write failing HTML and Playwright behavior tests**

Assert that the shell has `id="workspace-shell"`, panels have stable IDs, both buttons have `aria-controls` and `aria-expanded`, and the script contains guarded local-storage helpers. In Playwright, hide each panel, reload the same HTML, and assert the saved state and expanded main width are restored independently.

```js
await page.click("#toggle-left-panel");
await page.reload({waitUntil: "load"});
const state = await page.evaluate(() => ({
  leftHidden: document.getElementById("workspace-sidebar").hidden,
  rightHidden: document.getElementById("details").hidden,
  leftExpanded: document.getElementById("toggle-left-panel").getAttribute("aria-expanded"),
  rightExpanded: document.getElementById("toggle-right-panel").getAttribute("aria-expanded")
}));
```

- [ ] **Step 2: Run panel tests and verify RED**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboardPanel' -count=1`

Expected: FAIL because panel controls and storage helpers are absent.

- [ ] **Step 3: Add shell controls and guarded persistence**

Use a versioned key scoped by the generated root and schema. Both helpers must catch storage exceptions.

```js
function loadDashboardPanelState(storage,key){
  try{const value=JSON.parse(storage.getItem(key)||"null");return value&&typeof value.leftVisible==="boolean"&&typeof value.rightVisible==="boolean"?value:{leftVisible:true,rightVisible:true};}
  catch(error){return {leftVisible:true,rightVisible:true};}
}
function saveDashboardPanelState(storage,key,value){try{storage.setItem(key,JSON.stringify(value));return true;}catch(error){return false;}}
```

Update `hidden`, `aria-expanded`, `aria-label`, and `.left-panel-hidden` / `.right-panel-hidden` shell classes together. Keep the buttons inside `main` at its left and right edges.

- [ ] **Step 4: Add responsive layout rules**

Add zero-width desktop grid columns when a panel is hidden, `display:none` for hidden panels, focus-visible styling, and mobile rules that preserve the existing stacked layout while keeping both controls reachable.

- [ ] **Step 5: Run panel and existing dashboard tests**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboardPanel|TestWorkspaceDashboardCodeExplorerSourceLayoutGeometry|TestWorkspaceDashboardArchitecture' -count=1`

Expected: PASS with no horizontal overflow at 1280, 1440, or 1920 pixels.

- [ ] **Step 6: Commit panel controls**

```text
Add persistent dashboard panel controls

- Toggle navigation and details panels independently
- Restore panel visibility per workspace with guarded browser storage
- Expand graph and workbench layouts into available space
```

---

### Task 4: Make Code Explorer package groups collapsible

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_chrome_test.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Consumes: `codeSymbolGroup(symbol)`, `state.codeSymbol`, `state.codeQuery`, and current Code Explorer re-rendering.
- Produces: `state.codeExpandedGroups Set`, disclosure buttons with `data-code-group`, symbol counts, and selection/search-driven expansion.

- [ ] **Step 1: Write failing disclosure tests**

Add contract assertions for `aria-expanded`, `aria-controls`, count text, and hidden group bodies. Add a Node or Playwright behavior test covering default collapse, pointer and keyboard activation, selected-group expansion, and search expansion.

```js
const result = await page.evaluate(() => ({
  groups: Array.from(document.querySelectorAll("[data-code-group]")).map(button => ({
    name: button.dataset.codeGroup,
    expanded: button.getAttribute("aria-expanded"),
    hidden: document.getElementById(button.getAttribute("aria-controls")).hidden
  }))
}));
```

- [ ] **Step 2: Run disclosure tests and verify RED**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboardCodeExplorerGroup' -count=1`

Expected: FAIL because groups are static headings.

- [ ] **Step 3: Render accessible disclosure groups**

Add `codeExpandedGroups:new Set()` to state. Determine open state from explicit session state, selected symbol group, or non-empty search. Render a stable body ID derived from the group index and a native button.

```js
function codeGroupIsOpen(group,records){
  if(records.some(function(symbol){return symbol.id===state.codeSymbol;}))return true;
  if(state.codeQuery.trim())return true;
  return state.codeExpandedGroups.has(group);
}
```

The button text contains the group label and `records.length + " symbols"`; its body uses the standard `hidden` property.

- [ ] **Step 4: Wire disclosure without losing search focus**

Toggle membership in `state.codeExpandedGroups`, re-render Code Explorer, and restore focus to the same `data-code-group` button. Keep existing symbol and usage activation unchanged.

- [ ] **Step 5: Run Code Explorer tests**

Run: `go test ./internal/scan -run 'TestWorkspaceDashboardCodeExplorer' -count=1`

Expected: PASS, including long-path geometry and existing direct/API separation.

- [ ] **Step 6: Commit Code Explorer disclosure**

```text
Make Code Explorer groups collapsible

- Collapse package inventories by default with accessible controls
- Keep selected and searched symbol groups visible
- Preserve explicit disclosure choices during the browser session
```

---

### Task 5: Expose Architecture provenance and the secure edit workflow

**Files:**
- Modify: `internal/scan/workspace_dashboard_architecture.go`
- Modify: `internal/scan/workspace_dashboard_architecture_test.go`
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_chrome_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`

**Interfaces:**
- Consumes: existing `runWorkspaceDashboard`, `serveDashboardEditor`, `architectureDraftFromConfigData`, and uncommitted top-level dashboard resolver work.
- Produces: always-visible `Edit layout`, static command dialog, provenance labels, and `goregraph dashboard edit <path>`.

- [ ] **Step 1: Write failing Architecture draft and UI tests**

Assert that group and service drafts retain `source` and `confidence`, that the editor renders `Repository boundary`, `Package evidence`, or `Manual`, and that static mode opens a dialog containing the exact command while editor mode directly calls `enterArchitectureEditMode`.

```js
function architectureEvidenceLabel(source,confidence,manual){
  if(manual)return "Manual";
  const label=source==="production_package"?"Package evidence":source==="workspace_path"?"Repository boundary":"Detected";
  return confidence?label+" · "+confidence:label;
}
```

- [ ] **Step 2: Write failing top-level edit command tests**

Reuse the existing editor test hooks to call `Run([]string{"dashboard", "edit", workspace}, &stdout, &stderr)` and assert that it supplies the resolved workspace root and `workspace-map.html` to `serveDashboardEditor`. Extend dashboard help assertions with `edit` and the foreground lifecycle.

- [ ] **Step 3: Run Architecture and CLI tests and verify RED**

Run: `go test ./internal/scan ./internal/cli -run 'TestWorkspaceDashboardArchitecture.*Evidence|TestWorkspaceDashboardStaticEdit|TestRunDashboardEdit|TestDashboardHelp' -count=1`

Expected: FAIL because provenance, static help, and the top-level alias are absent.

- [ ] **Step 4: Preserve provenance in editor drafts**

Copy `source` and `confidence` from architecture group records and service nodes into detected draft entries. Preserve those fields through clone, move, merge, and render operations; manual state remains authoritative for saved entries.

- [ ] **Step 5: Add the static editor command dialog**

Render an always-enabled `Edit layout` button. In static mode open a native dialog with a read-only command field, `Copy command`, and `Close`; in editor mode enter the existing editor. Clipboard errors keep the command selected and update an `aria-live` status.

```js
function openArchitectureEdit(){
  if(editorEnabled){enterArchitectureEditMode();return;}
  const dialog=document.getElementById("architecture-edit-help");
  dialog.showModal();
  document.getElementById("architecture-edit-command").select();
}
```

- [ ] **Step 6: Delegate the top-level alias to the established editor**

Teach `runDashboard` to accept `edit`, resolve its path as a workspace, and call the same editor path as `runWorkspaceDashboard`. Do not duplicate server security or persistence code. Reject `edit` when only a project dashboard exists and report the workspace build command.

- [ ] **Step 7: Update help and documentation**

Document these exact workflows consistently:

```text
goregraph dashboard .
goregraph dashboard open .
goregraph dashboard edit .
```

Explain that static viewing is offline/read-only, editing runs in the foreground on authenticated loopback, and the result is stored in `.goregraph-dashboard.json`. Preserve the already-written project-first/workspace-fallback resolver documentation.

- [ ] **Step 8: Run focused and package tests**

Run: `go test ./internal/scan ./internal/cli ./internal/dashboardeditor -count=1`

Expected: PASS.

- [ ] **Step 9: Commit the editor workflow and existing resolver changes**

```text
Expose writable dashboard editing

- Keep dashboard path resolution project-first with workspace fallback
- Show a truthful static editing prompt and architecture provenance
- Route the concise edit command through the secure workspace editor
- Document static, open, and editable dashboard workflows
```

---

### Task 6: Complete regression, visual, Weka, installation, and push verification

**Files:**
- Modify only files required by concrete failures found in this task.

**Interfaces:**
- Consumes: all preceding task outputs.
- Produces: formatted, reviewed, locally installed, Weka-verified, committed, and pushed unreleased `1.3.0` source.

- [ ] **Step 1: Format and inspect the complete diff**

Run: `gofmt -w internal/config/config.go internal/config/config_test.go internal/scan/filter.go internal/scan/scan_test.go internal/scan/types.go internal/scan/workspace_reconcile.go internal/scan/workspace_reconcile_test.go internal/scan/workspace_grouping.go internal/scan/workspace_grouping_test.go internal/scan/workspace_dashboard_template.go internal/scan/workspace_dashboard_script.go internal/scan/workspace_dashboard_styles.go internal/scan/workspace_dashboard_architecture.go internal/scan/workspace_dashboard_architecture_test.go internal/scan/workspace_dashboard_chrome_test.go internal/cli/cli.go internal/cli/cli_test.go`

Run: `git diff --check`

Expected: no formatting or whitespace errors.

- [ ] **Step 2: Run complete affected tests and vet**

Run: `go test ./internal/config ./internal/scan ./internal/cli ./internal/dashboardeditor -count=1`

Run: `go vet ./internal/config ./internal/scan ./internal/cli ./internal/dashboardeditor`

Expected: PASS.

- [ ] **Step 3: Perform the required frontend design review**

Use the `frontend-design-review` skill on the generated dashboard at 1280×720, 1440×900, and 1920×1080. Verify both panel combinations, Code Explorer disclosure states, static edit dialog, editor layout, focus visibility, and absence of horizontal overflow. Fix every P0/P1 finding and any P2 regression caused by this change, then rerun focused tests.

- [ ] **Step 4: Install the unreleased binary locally**

Run: `go install ./cmd/goregraph`

Run: `C:\Users\goretzkh\go\bin\goregraph.exe version`

Expected: `goregraph 1.3.0` with schema 3 and no release tag requirement.

- [ ] **Step 5: Rebuild and verify the Weka workspace**

Run: `C:\Users\goretzkh\go\bin\goregraph.exe workspace build dashboard C:\Users\goretzkh\projects\weka`

Verify from `.goregraph-workspace/index/workspace-service-map.json` that the four frontend repositories remain four services under `frontend`, and from `symbol-index.json` that no declaration or module path contains `.worktrees/`. Open the generated dashboard and exercise both panels, Code Explorer groups, the static edit prompt, and `goregraph dashboard edit .`.

- [ ] **Step 6: Run final repository checks**

Run: `git status --short --branch`

Run: `git diff --check HEAD`

Run: `git log --oneline --decorate -8`

Expected: all intended changes committed, no accidental generated Weka output in the GoreGraph repository, and no tag or release action.

- [ ] **Step 7: Push the current branch only**

Run: `git push origin fix/workspace-project-eligibility`

Expected: the branch advances on GitHub. Do not run `git tag`, `gh release`, package publication, or any release workflow.
