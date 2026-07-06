# Frontend Route API Workspace Flows 0.8.5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend workspace feature flows so resolved frontend/backend flows start at the frontend route/component when GoreGraph can trace the route component to the API helper.

**Architecture:** Reuse existing scan outputs instead of adding a heavy parser. Load `flows.json` for indexed frontend projects, connect `CodeFlowRecord` route flows to `APIContractRecord` callers/files, then enrich `WorkspaceFeatureFlowRecord` with optional frontend route/component context and confidence reasons. Keep weak/unresolved frontend context explicit so reports remain useful without pretending precision.

**Tech Stack:** Go, existing GoreGraph scan/index JSON, `go test`, `go vet`.

---

## File Structure

- Modify `internal/scan/types.go`: add frontend route/component fields to `WorkspaceFeatureFlowRecord`.
- Modify `internal/scan/workspace_reconcile.go`: load `flows.json`, resolve frontend route context, render it in `workspace-feature-flows.md`.
- Modify `internal/scan/workspace_reconcile_test.go`: add RED/GREEN workspace tests for route/component-to-API linking and unresolved frontend context.
- Modify `internal/scan/scan.go`: keep generated output shape compatible if any placeholder records need new fields.
- Modify `internal/cli/cli_test.go`: bump expected local version to `0.8.5`.
- Modify `internal/version/version.go`: bump local development version to `0.8.5`.
- Modify `README.md`, `COMMANDS.md`, `docs/RELEASE.md`: document the route/component frontend context and version.

---

### Task 1: Route Component To API RED Test

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go`

- [ ] **Step 1: Write the failing test**

Add this test near `TestWorkspaceFeatureFlowsConnectFrontendBackendServiceAndTests`:

```go
func TestWorkspaceFeatureFlowsIncludeFrontendRouteComponentAndAPICall(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "apps/portal/src/routes.jsx", `import { Route } from "react-router-dom";
import { CadasterPage } from "./pages/CadasterPage";

export function Routes() {
  return <Route path="/cadasters/:cadasterId" component={CadasterPage} />;
}
`)
	writeFile(t, frontend, "apps/portal/src/pages/CadasterPage.jsx", `import { loadCadaster } from "../api/cadasterservice";

export function CadasterPage(props) {
  return loadCadaster(props.cadasterId);
}
`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", `export function loadCadaster(id) {
  return fetch(`/ + "`/cadasters/${id}`" + `);
}
`)
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get(@PathVariable String cadasterId) {
    return cadasterId;
  }
}
`)

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("Run cadaster returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	for _, want := range []string{
		"- Frontend route: `portal:/cadasters/:cadasterId` `/cadasters/:cadasterId` -> `CadasterPage`",
		"- Frontend API: `apps/portal/src/api/cadasterservice.js:2` `loadCadaster`",
		"route flow reaches API contract caller",
		"CadasterController.get",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("workspace feature flow report missing %q:\n%s", want, report)
		}
	}

	var flows []WorkspaceFeatureFlowRecord
	readJSON(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.json"), &flows)
	if len(flows) != 1 {
		t.Fatalf("feature flow count = %d, want 1: %#v", len(flows), flows)
	}
	if flows[0].FrontendRouteID != "portal:/cadasters/:cadasterId" {
		t.Fatalf("FrontendRouteID = %q, want portal route: %#v", flows[0].FrontendRouteID, flows[0])
	}
	if flows[0].FrontendComponent != "CadasterPage" {
		t.Fatalf("FrontendComponent = %q, want CadasterPage: %#v", flows[0].FrontendComponent, flows[0])
	}
}
```

- [ ] **Step 2: Run the test to verify RED**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsIncludeFrontendRouteComponentAndAPICall -count=1
```

Expected: FAIL because `WorkspaceFeatureFlowRecord` has no frontend route fields and the report does not render frontend route context.

---

### Task 2: Data Model And Workspace Index Loading

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`

- [ ] **Step 1: Add frontend context fields**

Extend `WorkspaceFeatureFlowRecord`:

```go
FrontendRouteID         string         `json:"frontend_route_id,omitempty"`
FrontendRoutePath       string         `json:"frontend_route_path,omitempty"`
FrontendRouteFile       string         `json:"frontend_route_file,omitempty"`
FrontendRouteLine       int            `json:"frontend_route_line,omitempty"`
FrontendComponent       string         `json:"frontend_component,omitempty"`
FrontendComponentFile   string         `json:"frontend_component_file,omitempty"`
FrontendComponentLine   int            `json:"frontend_component_line,omitempty"`
FrontendSteps           []CodeFlowStep `json:"frontend_steps,omitempty"`
FrontendContextConfidence string       `json:"frontend_context_confidence,omitempty"`
FrontendContextReason   string         `json:"frontend_context_reason,omitempty"`
```

Keep existing `FrontendProject`, `FrontendFile`, and `FrontendLine`; they still identify the API contract site.

- [ ] **Step 2: Load frontend route flows in workspace indexes**

Extend `workspaceIndexProject`:

```go
codeFlows []CodeFlowRecord
```

Update `loadWorkspaceIndexes`:

```go
if err := readWorkspaceJSON(filepath.Join(out, "flows.json"), &loaded.codeFlows); err != nil && !os.IsNotExist(err) {
	return nil, err
}
```

- [ ] **Step 3: Run compile check**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsIncludeFrontendRouteComponentAndAPICall -count=1
```

Expected: still FAIL because the fields are not populated or rendered yet.

---

### Task 3: Resolve Frontend Route Context

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`

- [ ] **Step 1: Add resolver helper**

Add helper types/functions near `buildWorkspaceFeatureFlows`:

```go
type workspaceFrontendContext struct {
	routeID    string
	routePath  string
	routeFile  string
	routeLine  int
	component  string
	componentFile string
	componentLine int
	steps      []CodeFlowStep
	confidence string
	reason     string
	score      float64
}

func resolveWorkspaceFrontendContext(project workspaceIndexProject, match WorkspaceContractMatchRecord) workspaceFrontendContext {
	var best workspaceFrontendContext
	for _, flow := range project.codeFlows {
		if flow.Kind != "frontend" {
			continue
		}
		if match.APIFile != "" && flow.App != "" && codeFileApp(match.APIFile) != "" && flow.App != codeFileApp(match.APIFile) {
			continue
		}
		candidate := scoreWorkspaceFrontendFlow(flow, match)
		if candidate.score > best.score {
			best = candidate
		}
	}
	return best
}

func scoreWorkspaceFrontendFlow(flow CodeFlowRecord, match WorkspaceContractMatchRecord) workspaceFrontendContext {
	context := workspaceFrontendContext{
		routeID:    flow.RouteID,
		routePath:  flow.Path,
		routeFile:  flow.File,
		routeLine:  flow.Line,
		component:  flow.Handler,
		steps:      flow.Steps,
		confidence: "WEAK_MATCH",
		reason:     "frontend route shares app with API contract but no route-flow step reached the API caller",
		score:      0.35,
	}
	for _, step := range flow.Steps {
		if step.Kind == "route_handler" && step.File != "" {
			context.componentFile = step.File
			context.componentLine = step.Line
			if step.Name != "" {
				context.component = step.Name
			}
		}
		if step.File == match.APIFile && (match.APIFile != "" || match.APILine > 0) {
			context.confidence = "RESOLVED"
			context.reason = "route flow reaches API contract file"
			context.score = 0.82
		}
		if step.File == match.APIFile && match.APILine > 0 && step.Line > 0 && sameNearbyLine(step.Line, match.APILine) {
			context.confidence = "RESOLVED"
			context.reason = "route flow reaches API contract caller"
			context.score = 0.92
		}
	}
	return context
}

func sameNearbyLine(left, right int) bool {
	if left > right {
		return left-right <= 3
	}
	return right-left <= 3
}
```

- [ ] **Step 2: Populate fields in `buildWorkspaceFeatureFlows`**

Inside the matched-flow block, look up the frontend project:

```go
frontendProject, hasFrontendProject := byProject[match.APIProject]
if hasFrontendProject {
	frontend := resolveWorkspaceFrontendContext(frontendProject, match)
	record.FrontendRouteID = frontend.routeID
	record.FrontendRoutePath = frontend.routePath
	record.FrontendRouteFile = frontend.routeFile
	record.FrontendRouteLine = frontend.routeLine
	record.FrontendComponent = frontend.component
	record.FrontendComponentFile = frontend.componentFile
	record.FrontendComponentLine = frontend.componentLine
	record.FrontendSteps = frontend.steps
	record.FrontendContextConfidence = frontend.confidence
	record.FrontendContextReason = frontend.reason
}
```

- [ ] **Step 3: Run the RED test**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsIncludeFrontendRouteComponentAndAPICall -count=1
```

Expected: still FAIL if the report renderer is not updated yet.

---

### Task 4: Render Frontend Route Context

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`

- [ ] **Step 1: Update `renderWorkspaceFeatureFlowsReport`**

Render route context before the existing `Frontend` API line:

```go
if record.FrontendRouteID != "" || record.FrontendRoutePath != "" || record.FrontendComponent != "" {
	b.WriteString(fmt.Sprintf("- Frontend route: `%s` `%s` -> `%s`",
		emptyAsNone(record.FrontendRouteID),
		emptyAsNone(record.FrontendRoutePath),
		emptyAsNone(record.FrontendComponent),
	))
	if record.FrontendRouteFile != "" {
		b.WriteString(fmt.Sprintf(" in `%s:%d`", record.FrontendRouteFile, record.FrontendRouteLine))
	}
	if record.FrontendContextConfidence != "" {
		b.WriteString(fmt.Sprintf(" (%s", record.FrontendContextConfidence))
		if record.FrontendContextReason != "" {
			b.WriteString(fmt.Sprintf(", %s", record.FrontendContextReason))
		}
		b.WriteString(")")
	}
	b.WriteString("\n")
} else {
	b.WriteString("- Frontend route: none resolved (no route flow reached this API contract)\n")
}
b.WriteString(fmt.Sprintf("- Frontend API: `%s:%d` `%s`\n", record.FrontendFile, record.FrontendLine, emptyAsNone(record.FrontendCaller)))
```

If `FrontendCaller` does not exist yet, add `FrontendCaller string json:"frontend_caller,omitempty"` to `WorkspaceFeatureFlowRecord` and populate it from `match` after Task 5 adds the field.

- [ ] **Step 2: Run the RED test**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsIncludeFrontendRouteComponentAndAPICall -count=1
```

Expected: PASS after the route context is rendered.

---

### Task 5: Preserve API Caller In Workspace Matches

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_reconcile_test.go`

- [ ] **Step 1: Add caller fields**

Add to `WorkspaceContractMatchRecord`:

```go
APICaller string `json:"api_caller,omitempty"`
```

Add to `WorkspaceFeatureFlowRecord`:

```go
FrontendCaller string `json:"frontend_caller,omitempty"`
```

Populate `APICaller` in `workspaceContractMatch`:

```go
APICaller: contract.Caller,
```

Populate `FrontendCaller` in `buildWorkspaceFeatureFlows`:

```go
FrontendCaller: match.APICaller,
```

- [ ] **Step 2: Tighten resolver scoring**

In `scoreWorkspaceFrontendFlow`, upgrade to `RESOLVED` when a flow step name matches the API caller:

```go
if match.APICaller != "" && step.Name == match.APICaller {
	context.confidence = "RESOLVED"
	context.reason = "route flow reaches API contract caller"
	context.score = 0.95
}
```

- [ ] **Step 3: Run tests**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run "TestWorkspaceFeatureFlowsIncludeFrontendRouteComponentAndAPICall|TestLaterBackendScanRefreshesExistingFrontendWorkspaceOverlay|TestWorkspaceFeatureFlowsConnectFrontendBackendServiceAndTests" -count=1
```

Expected: PASS. If Windows `cmd` quote handling breaks the regex, run the three tests individually.

---

### Task 6: Unresolved Frontend Context Test

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go`

- [ ] **Step 1: Write the failing negative test**

Add a test where the API helper exists but no route/component reaches it:

```go
func TestWorkspaceFeatureFlowsExplainMissingFrontendRouteContext(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	frontend := filepath.Join(workspace, "frontend", "frontend-monorepo")
	cadaster := filepath.Join(workspace, "microservices", "ms-cadaster")
	writeFile(t, frontend, "package.json", `{"name":"frontend-monorepo"}`)
	writeFile(t, frontend, "apps/portal/src/api/cadasterservice.js", `export function loadCadaster(id) {
  return fetch(`/ + "`/cadasters/${id}`" + `);
}
`)
	writeFile(t, cadaster, "src/main/java/com/example/CadasterController.java", `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/cadasters")
class CadasterController {
  @GetMapping("/{cadasterId}")
  String get(@PathVariable String cadasterId) {
    return cadasterId;
  }
}
`)

	if _, err := Run(frontend, config.Defaults()); err != nil {
		t.Fatalf("Run frontend returned error: %v", err)
	}
	if _, err := Run(cadaster, config.Defaults()); err != nil {
		t.Fatalf("Run cadaster returned error: %v", err)
	}

	report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
	if !strings.Contains(report, "Frontend route: none resolved") {
		t.Fatalf("feature flow report should explain missing frontend route context:\n%s", report)
	}
}
```

- [ ] **Step 2: Run the negative test**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsExplainMissingFrontendRouteContext -count=1
```

Expected: PASS after Task 4; otherwise implement the explicit fallback line.

---

### Task 7: Documentation And Version 0.8.5

**Files:**
- Modify: `internal/version/version.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/RELEASE.md`

- [ ] **Step 1: Bump version**

Change:

```go
Version = "0.8.5"
```

Update `TestRunVersionPrintsBuildMetadata` to expect:

```go
"goregraph 0.8.5",
```

- [ ] **Step 2: Update docs**

Add release-note bullets:

```markdown
`v0.8.5` local feature checks:

- `workspace-feature-flows.md` includes frontend route/component context when a route flow reaches the API contract caller.
- unresolved frontend route context is explicit instead of silent.
- workspace feature flow JSON includes route ID, route path, component, frontend steps, confidence, and reason.
- `goregraph version` reports `0.8.5`.
```

Update README/COMMANDS descriptions for `workspace-feature-flows.md` from “frontend API call to backend endpoint flow” to “frontend route/component/API call to backend endpoint flow”.

- [ ] **Step 3: Run version test**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/cli -run TestRunVersionPrintsBuildMetadata -count=1
```

Expected: PASS.

---

### Task 8: Full Verification And Local Install

**Files:**
- No source edits expected.

- [ ] **Step 1: Format Go code**

Run:

```bash
gofmt -w internal\scan\types.go internal\scan\workspace_reconcile.go internal\scan\workspace_reconcile_test.go internal\cli\cli_test.go internal\version\version.go
```

- [ ] **Step 2: Run full tests**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./...
```

Expected: all packages pass.

- [ ] **Step 3: Run vet**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go vet ./...
```

Expected: exit 0.

- [ ] **Step 4: Install local binary**

Run:

```bash
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-install&& go install ./cmd/goregraph
goregraph version
```

Expected:

```text
goregraph 0.8.5
```

- [ ] **Step 5: Clean local caches**

Run:

```bash
rmdir /s /q .gocache-test
rmdir /s /q .gocache-install
```

If Windows keeps a cache file locked, leave it untracked and do not stage it.

---

### Task 9: Local Commit Only

**Files:**
- Stage only source and documentation files changed by this plan.

- [ ] **Step 1: Check status**

Run:

```bash
git status --short
git diff --check
git diff --stat
```

Expected: only planned source/doc changes; no cache directories staged.

- [ ] **Step 2: Commit locally**

Use a commit message file to avoid Windows quote parsing problems:

```text
feat: add frontend route context to workspace flows

- connect frontend route flows to workspace API contracts
- render route/component context in workspace feature flows
- explain unresolved frontend route context explicitly
- bump local development version to 0.8.5
```

Run:

```bash
git commit -F commit-message.tmp
del commit-message.tmp
git status --short
```

Expected: clean worktree. Do not push and do not create a release tag.

---

## Self-Review

- Spec coverage: Covers the next big value gap identified in the latest evaluation: route/component context before API calls.
- Scope control: Does not try to solve indirect test mapping or scan more services; those remain separate follow-up work.
- TDD: First task writes a failing behavior test before model/renderer changes.
- Risk: Existing JS/TS callgraph quality determines how often route context resolves. The plan keeps unresolved context explicit and confidence-scored to avoid false precision.
