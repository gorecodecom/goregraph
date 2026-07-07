# Frontend Usage Graph Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make workspace feature flows trace a frontend route through component usage, effects/events/actions, API helpers, backend endpoint flow, and linked tests with explicit confidence.

**Architecture:** Add a narrow frontend usage layer on top of the existing deterministic JS/TS extraction. Keep the current workspace contract matching and backend flow pipeline, but improve the frontend side so `workspace-feature-flows.md` can distinguish route-proven API usage from app-scope guesses.

**Tech Stack:** Go stdlib, existing GoreGraph scan pipeline, existing workspace reconciliation, TDD with `go test`.

## Global Constraints

- Keep the implementation deterministic and dependency-free.
- Do not add a JS/TS parser dependency in this slice.
- Keep local validation on version `0.8.6`; do not bump the version unless explicitly requested.
- Do not push and do not create a release tag during local validation.
- Prefer `unknown` or `WEAK_MATCH` over invented frontend edges.
- Every new confidence upgrade must include a concrete reason string.

---

## File Structure

- Modify `internal/scan/workspace_reconcile_test.go`
  - Golden workspace tests for route -> effect/event/action -> API -> backend feature flows.
- Modify `internal/scan/code_intelligence.go`
  - Extract narrow frontend usage calls from React components and helper functions.
- Modify `internal/scan/code_flows.go`
  - Preserve frontend usage edges as route flow steps with clear step kinds.
- Modify `internal/scan/workspace_reconcile.go`
  - Score frontend route context using the new usage steps and emit precise confidence/reasons.
- Modify `internal/scan/types.go`
  - Only if a new field is required for stable JSON output; prefer reusing `CodeFlowStep.Kind`, `Name`, `Owner`, `File`, and `Line`.
- Modify `README.md`, `COMMANDS.md`, `docs/RELEASE.md`
  - Document what the new frontend usage graph does and what it deliberately does not prove.

---

### Task 1: Golden Fixture For Route To Effect To API

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: `Run(root, config.Defaults())`, `WorkspaceFeatureFlowRecord`, existing workspace reconciliation.
- Produces: `TestWorkspaceFeatureFlowsResolveEffectToAPICaller`.

- [ ] **Step 1: Write the failing test**

Add a test fixture with:

```text
weka/
  frontend/frontend-monorepo/
    package.json
    apps/portal/src/routes.jsx
    apps/portal/src/pages/CadasterPage.jsx
    apps/portal/src/api/cadasterservice.js
    apps/portal/src/utils/requestHelper.js
  microservices/ms-cadaster/
    src/main/java/com/example/CadasterController.java
```

Use this frontend shape:

```jsx
// apps/portal/src/routes.jsx
import { Route } from "react-router-dom";
import { CadasterPage } from "./pages/CadasterPage";

export const routes = <Route path="/cadasters/:cadasterId" element={<CadasterPage />} />;
```

```jsx
// apps/portal/src/pages/CadasterPage.jsx
import { useEffect } from "react";
import { loadCadaster } from "../api/cadasterservice";

export function CadasterPage({ cadasterId }) {
  useEffect(() => {
    loadCadaster(cadasterId);
  }, [cadasterId]);
  return <main />;
}
```

```js
// apps/portal/src/api/cadasterservice.js
import { GetHelper } from "../utils/requestHelper";

export function loadCadaster(id) {
  return GetHelper(null, `/cadasters/${id}`);
}
```

Assert after scanning frontend and backend:

```go
report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
if !strings.Contains(report, "Frontend route: portal:/cadasters/:cadasterId") {
	t.Fatalf("feature flow should include frontend route:\n%s", report)
}
if !strings.Contains(report, "Frontend confidence: RESOLVED") {
	t.Fatalf("feature flow should be resolved through effect usage:\n%s", report)
}
if !strings.Contains(report, "route flow reaches API contract caller through effect") {
	t.Fatalf("feature flow should explain effect-based resolution:\n%s", report)
}
```

- [ ] **Step 2: Run RED**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsResolveEffectToAPICaller -count=1
```

Expected: FAIL because current route flow does not give effects/events/actions a distinct evidence trail.

---

### Task 2: Golden Fixture For Route To Event Handler To API

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: same workspace fixture style as Task 1.
- Produces: `TestWorkspaceFeatureFlowsResolveEventHandlerToAPICaller`.

- [ ] **Step 1: Write the failing test**

Add a second frontend page:

```jsx
import { loadCadaster } from "../api/cadasterservice";

export function CadasterPage({ cadasterId }) {
  function handleRefresh() {
    loadCadaster(cadasterId);
  }
  return <button onClick={handleRefresh}>Refresh</button>;
}
```

Assert:

```go
report := readText(t, filepath.Join(frontend, "goregraph-out", "workspace-feature-flows.md"))
if !strings.Contains(report, "route flow reaches API contract caller through event handler") {
	t.Fatalf("feature flow should explain event-handler-based resolution:\n%s", report)
}
```

- [ ] **Step 2: Run RED**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsResolveEventHandlerToAPICaller -count=1
```

Expected: FAIL until event-handler evidence is extracted and scored.

---

### Task 3: Extract Narrow Frontend Usage Calls

**Files:**
- Modify: `internal/scan/code_intelligence.go`
- Modify: `internal/scan/code_flows.go`

**Interfaces:**
- Consumes: existing `CodeCallRecord` extraction and `CodeFlowStep`.
- Produces: route flow steps whose `Kind` distinguishes at least:
  - `effect_call`
  - `event_handler`
  - `api_call`

- [ ] **Step 1: Extend call extraction for effects**

Detect calls inside common React effect callbacks:

```go
// Pattern target:
// useEffect(() => {
//   loadCadaster(id)
// }, [id])
```

The extracted step should preserve:

```go
CodeFlowStep{
	Kind: "effect_call",
	Name: "loadCadaster",
	File: "apps/portal/src/pages/CadasterPage.jsx",
	Line: <call line>,
}
```

- [ ] **Step 2: Extend call extraction for local event handlers**

Detect a component-local handler passed to JSX event props:

```jsx
function handleRefresh() {
  loadCadaster(cadasterId);
}
return <button onClick={handleRefresh}>Refresh</button>;
```

The route flow should include both the handler and the API helper call:

```text
route_handler CadasterPage
event_handler handleRefresh
api_call loadCadaster
```

- [ ] **Step 3: Keep non-evidence weak**

Do not mark a route as `RESOLVED` only because:

```text
same app
same folder
same imported API module without a route-flow call path
```

Those remain `WEAK_MATCH` with the existing app-scope reason.

- [ ] **Step 4: Run GREEN for Tasks 1 and 2**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsResolveEffectToAPICaller -count=1
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsResolveEventHandlerToAPICaller -count=1
```

Expected: PASS.

---

### Task 4: Score Frontend Evidence In Workspace Feature Flows

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: `resolveWorkspaceFrontendContext`, `scoreWorkspaceFrontendFlow`, `WorkspaceContractMatchRecord`.
- Produces: stable frontend confidence and reason output.

- [ ] **Step 1: Update scoring rules**

Use this confidence ladder:

```text
0.95 RESOLVED route flow reaches API contract caller directly
0.92 RESOLVED route flow reaches API contract caller through effect
0.90 RESOLVED route flow reaches API contract caller through event handler
0.82 RESOLVED route flow reaches API contract file but not caller
0.35 WEAK_MATCH frontend route shares app with API contract but no route-flow step reached the API caller
```

- [ ] **Step 2: Assert reason strings**

Each resolved case must emit one of:

```text
route flow reaches API contract caller
route flow reaches API contract caller through effect
route flow reaches API contract caller through event handler
route flow reaches API contract file
```

- [ ] **Step 3: Run workspace regression suite**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspace -count=1
```

Expected: PASS.

---

### Task 5: Add Redux/Action Slice Only If Tests Stay Small

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/scan/code_intelligence.go`
- Modify: `internal/scan/code_flows.go`

**Interfaces:**
- Consumes: event/effect extraction from Tasks 3 and 4.
- Produces: optional support for one direct Redux-style action pattern.

- [ ] **Step 1: Add RED test for direct dispatch action**

Only include this task if Tasks 1-4 remain small. Test this pattern:

```jsx
import { fetchCadaster } from "../store/cadasterActions";

export function CadasterPage({ cadasterId, dispatch }) {
  function handleRefresh() {
    dispatch(fetchCadaster(cadasterId));
  }
  return <button onClick={handleRefresh}>Refresh</button>;
}
```

```js
import { loadCadaster } from "../api/cadasterservice";

export function fetchCadaster(id) {
  return loadCadaster(id);
}
```

Expected reason:

```text
route flow reaches API contract caller through action
```

- [ ] **Step 2: Implement only direct named action calls**

Support this:

```text
dispatch(fetchCadaster(id))
```

Do not support in this slice:

```text
sagas
observables
dynamic action maps
middleware side effects
barrel re-export chains
```

- [ ] **Step 3: Stop if it gets broad**

If this requires broad import resolver changes, defer Redux/action support to a separate plan and keep this plan focused on effects/events.

---

### Task 6: Report And Docs Update

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Consumes: implemented frontend usage graph behavior.
- Produces: user-facing explanation of what GoreGraph can and cannot infer.

- [ ] **Step 1: Update README**

Document:

```text
Workspace feature flows can resolve frontend route context through direct component calls, React effect calls, and local event handlers when those steps reach the API contract caller.
```

- [ ] **Step 2: Update COMMANDS**

Clarify `workspace-feature-flows.md`:

```text
Resolved frontend route confidence means the route flow contains a concrete step to the API contract file or caller. App-only matches remain WEAK_MATCH.
```

- [ ] **Step 3: Update RELEASE**

Under `v0.8.6` local feature checks, add:

```text
- frontend route flows can resolve API callers through React effect calls and local event handlers.
- workspace feature flow reasons distinguish direct, effect, event-handler, and app-scope matches.
```

Do not bump the version.

---

### Task 7: Full Verification And Local Install

**Files:**
- No source file changes unless verification exposes a defect.

**Interfaces:**
- Consumes: all previous tasks.
- Produces: locally installed `goregraph 0.8.6`.

- [ ] **Step 1: Run full tests**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./...
```

Expected: all packages PASS.

- [ ] **Step 2: Run vet**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go vet ./...
```

Expected: no output, exit code 0.

- [ ] **Step 3: Install locally**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-install&& go install ./cmd/goregraph
goregraph version
```

Expected:

```text
goregraph 0.8.6
```

- [ ] **Step 4: Clean generated caches**

Remove only repo-local generated cache directories:

```cmd
if exist .gocache-test rmdir /s /q .gocache-test
if exist .gocache-install rmdir /s /q .gocache-install
```

- [ ] **Step 5: Commit locally only**

Commit message:

```text
feat: trace frontend usage to API callers

- resolve workspace feature flows through effects and event handlers
- keep app-only frontend matches weak and explicit
- document frontend usage confidence rules
```

Do not push. Do not tag. Do not create a release.

---

## Acceptance Criteria

- `workspace-feature-flows.md` can show route-proven frontend paths for effect/event based API calls.
- App-scope-only matches stay `WEAK_MATCH`.
- Every frontend confidence upgrade has a reason that names the evidence type.
- Existing cross-repo workspace behavior still works when frontend and backend scans happen in either order.
- `goregraph version` remains `0.8.6`.

## Deferred Work

- Full Redux saga/thunk/middleware modeling.
- Barrel export resolution across multiple index files.
- TypeScript AST parser dependency.
- Frontend test coverage mapping.
- Public version bump, tag, push, and release.
