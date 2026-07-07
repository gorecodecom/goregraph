# Frontend Component Flow 0.8.6 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve workspace feature flows so frontend route context can resolve through rendered React components before reaching API helper functions.

**Architecture:** Extend existing lightweight JS/TS call extraction instead of adding a parser dependency. Treat JSX component tags inside function bodies as component-call edges, then reuse the existing callgraph and workspace frontend scoring.

**Tech Stack:** Go stdlib, existing GoreGraph scan/query/report pipeline, TDD with `go test`.

## Global Constraints

- Keep the implementation deterministic and dependency-free.
- Keep weak matches explicit; only mark route context `RESOLVED` when the route flow reaches the API file/caller.
- Update version, docs, schema, and release notes for local development version `0.8.6`.
- Commit locally only. Do not push and do not create a release tag.

---

### Task 1: RED Test For Route Component To Child Component To API

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go`

**Interfaces:**
- Consumes: existing `Run`, workspace reconciliation, and `WorkspaceFeatureFlowRecord`.
- Produces: a failing regression test requiring `workspace-feature-flows` to mark frontend route context `RESOLVED` when a route component renders a child component that calls the API helper.

- [ ] **Step 1: Write failing test**

Add a workspace test with:
- route `/cadaster/:cadasterId` -> `Home`
- `Home` returns `<CadasterPanel />`
- `CadasterPanel` imports and calls `loadCadaster`
- `loadCadaster` calls `GetHelper(..., "/cadasters/${id}")`
- backend exposes `GET /cadasters/{cadasterId}`

- [ ] **Step 2: Verify RED**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./internal/scan -run TestWorkspaceFeatureFlowsResolveRenderedComponentToAPICaller -count=1
```

Expected: FAIL because the route flow does not currently include the rendered child component and cannot reach the API caller.

### Task 2: Extract JSX Component Calls

**Files:**
- Modify: `internal/scan/code_intelligence.go`

**Interfaces:**
- Consumes: existing `extractCallsForFunction` and `CodeCallRecord`.
- Produces: JSX component tags as `CodeCallRecord{Method: "<Component>", ...}` entries when they occur inside a script function body.

- [ ] **Step 1: Add JSX component call extraction**

Inside `extractCallsForFunction`, for JavaScript/TypeScript lines, detect `<ComponentName` tags using `codeJSXComponentOpenRE`. Ignore `Fragment` and self-calls.

- [ ] **Step 2: Verify GREEN**

Run the RED test again and confirm it passes.

### Task 3: Docs And Version 0.8.6

**Files:**
- Modify: `internal/version/version.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Consumes: existing version/report documentation.
- Produces: local development version `0.8.6` and docs describing component-aware frontend feature flows.

- [ ] **Step 1: Bump version and test expectation**
- [ ] **Step 2: Update docs for JSX component route-flow resolution**
- [ ] **Step 3: Run full verification**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go test ./...
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-test&& go vet ./...
```

### Task 4: Local Install And Commit

**Files:**
- Stage all changed source, test, docs, and plan files.

- [ ] **Step 1: Install locally**

Run:

```cmd
set GOCACHE=C:\Users\goretzkh\projects\gorecode\goregraph\.gocache-install&& go install ./cmd/goregraph
goregraph version
```

Expected: `goregraph 0.8.6`.

- [ ] **Step 2: Clean local caches**
- [ ] **Step 3: Commit locally**

Commit message:

```text
feat: resolve frontend component flows to API callers

- trace JSX component calls inside frontend route flows
- upgrade workspace feature flow confidence when routes reach API callers
- bump local development version to 0.8.6
```
