# Workspace Integration 0.8.3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make workspace overlays feel integrated with normal GoreGraph commands and reports for local version `0.8.3`.

**Architecture:** Keep the normal project scan deterministic, then let workspace reconciliation refresh additive project overlays. Query should fall back to workspace-level files when invoked at the workspace root. Diagnostics and endpoint reports should read workspace overlay facts when present, without rescanning sibling projects.

**Tech Stack:** Go CLI, standard library filesystem/JSON, existing GoreGraph scan/query tests.

---

### Task 1: Workspace-aware query aliases

**Files:**
- Modify: `internal/query/query.go`
- Test: `internal/query/query_test.go`

- [ ] **Step 1: Write failing test**
  Add a test that scans `frontend/frontend-monorepo` and `microservices/ms-cadaster`, then calls `Search(workspace, "workspace-contracts")` and expects the workspace root `.goregraph-workspace/contract-matches.md` content.

- [ ] **Step 2: Verify RED**
  Run `go test ./internal/query -run TestSearchReadsWorkspaceRootOverlayAliases -v`. Expected: fails because `ReadOutput` only reads `<root>/goregraph-out`.

- [ ] **Step 3: Implement minimal fallback**
  If project output is missing and the alias is workspace-level, read from `<root>/.goregraph-workspace/contract-matches.md` or `workspace-context.md`.

- [ ] **Step 4: Verify GREEN**
  Run the same focused test.

### Task 2: Per-project workspace context labels

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`

- [ ] **Step 1: Write failing test**
  After scanning frontend then backend, assert frontend `workspace-context.md` contains `This project: frontend/frontend-monorepo` and `Last refreshed by: microservices/ms-cadaster`.

- [ ] **Step 2: Verify RED**
  Run `go test ./internal/scan -run TestLaterBackendScanRefreshesExistingFrontendWorkspaceOverlay -v`. Expected: fails because all project overlays say `Current project: microservices/ms-cadaster`.

- [ ] **Step 3: Implement minimal rendering**
  Add project-specific rendering for project overlay files while keeping workspace-root context as current scan context.

- [ ] **Step 4: Verify GREEN**
  Run the focused scan test.

### Task 3: Workspace diagnostics integration

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`

- [ ] **Step 1: Write failing test**
  After scanning frontend first and backend second, assert frontend `diagnostics.md` contains a `Workspace Resolved Contracts` section with `ms-cadaster GET /cadasters/{cadasterId}` and no longer lists `ms-cadaster` under unscanned services when resolved by workspace data.

- [ ] **Step 2: Verify RED**
  Run `go test ./internal/scan -run TestLaterBackendScanRefreshesExistingFrontendWorkspaceOverlay -v`. Expected: fails because diagnostics remain local-only.

- [ ] **Step 3: Implement minimal overlay diagnostics rewrite**
  During workspace reconciliation, read each indexed project's `diagnostics.md`, append/replace a workspace section, and suppress resolved workspace services from the rendered unscanned service section.

- [ ] **Step 4: Verify GREEN**
  Run the focused scan test.

### Task 4: Manifest and audit overlay generated files

**Files:**
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/audit.go`
- Test: `internal/scan/workspace_reconcile_test.go`

- [ ] **Step 1: Write failing test**
  Assert frontend `manifest.json` and `audit.json` list `workspace-context.md`, `workspace-contract-matches.md`, and `frontend-consumers.md`.

- [ ] **Step 2: Verify RED**
  Run the focused scan test. Expected: generated files do not include overlays.

- [ ] **Step 3: Implement minimal generated list**
  Add overlay files to generated metadata for scans. They may be `none detected` when no workspace exists.

- [ ] **Step 4: Verify GREEN**
  Run the focused scan test.

### Task 5: Path specificity for method mismatch

**Files:**
- Modify: `internal/scan/contract_matches.go`
- Modify: `internal/scan/workspace_reconcile.go`
- Test: `internal/scan/workspace_reconcile_test.go`

- [ ] **Step 1: Write failing test**
  Add backend routes `/cadasters/{cadasterId}` and `/cadasters/cadastertopics`; assert `POST /cadasters/cadastertopics` does not method-mismatch against `/cadasters/{cadasterId}`.

- [ ] **Step 2: Verify RED**
  Run focused scan test. Expected: current wildcard compatibility picks the variable route too broadly.

- [ ] **Step 3: Implement minimal scoring**
  Prefer path-compatible routes with the highest static segment score, and do not treat a variable segment match as compatible when the contract segment is static and another same-path static route exists.

- [ ] **Step 4: Verify GREEN**
  Run focused tests for scan contract matching.

### Task 6: Docs, version, install, commit

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `docs/RELEASE.md`
- Modify: `internal/version/version.go`
- Test: `internal/cli/cli_test.go`

- [ ] **Step 1: Update version tests to `0.8.3`**
- [ ] **Step 2: Update command and schema docs for workspace-root query fallback and integrated diagnostics**
- [ ] **Step 3: Run `go test ./...` and `go vet ./...`**
- [ ] **Step 4: Run `go install ./cmd/goregraph` and verify `goregraph version` reports `0.8.3`**
- [ ] **Step 5: Commit locally, no push, no tag**
