# Workspace Reconcile Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make normal `goregraph scan .` reconcile cross-project workspace overlays so later service scans update earlier frontend scan outputs without rescanning those projects.

**Architecture:** Keep local project scan facts immutable per scan and add a separate workspace overlay layer. A scan writes the current project output, discovers a workspace root, registers sibling projects, loads existing `goregraph-out` indexes, computes cross-project contract matches, writes central `.goregraph-workspace` files, and refreshes `workspace-*` overlay files in each indexed project.

**Tech Stack:** Go CLI, existing `internal/scan` package, JSON/Markdown files, Go tests with `go test`.

---

### Task 1: Workspace Model And Discovery

**Files:**
- Create: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/types.go`
- Test: `internal/scan/workspace_reconcile_test.go`

- [ ] Write a failing test that creates `weka/frontend/frontend-monorepo` and `weka/microservices/ms-cadaster`, scans the frontend, and expects `.goregraph-workspace/registry.json`.
- [ ] The expected registry lists `frontend/frontend-monorepo` as `current,indexed`, `microservices/ms-cadaster` as `not_indexed`, and `microservices/ms-task` as `not_indexed` when those directories exist.
- [ ] Implement conservative workspace discovery from parent directories with grouped project folders such as `frontend`, `microservices`, `services`, `apps`, or `packages`.
- [ ] Implement project discovery using project markers and existing `goregraph-out/manifest.json`.

### Task 2: Workspace Reconcile

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/scan.go`
- Test: `internal/scan/workspace_reconcile_test.go`

- [ ] Write a failing test where frontend is scanned first and backend second.
- [ ] After the backend scan, assert the frontend output now contains `workspace-contract-matches.md` with a matched backend route.
- [ ] Assert the backend output contains `frontend-consumers.md` naming the frontend API call.
- [ ] Implement index loading from existing project `goregraph-out` folders only.
- [ ] Compute cross-project matches by combining frontend `api-contracts.json` with backend `routes.json`.
- [ ] Write central `.goregraph-workspace/contract-matches.md` and per-project overlay files.

### Task 3: CLI Controls

**Files:**
- Modify: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`

- [ ] Write a failing test that `goregraph scan . --no-workspace` creates local output but does not create `.goregraph-workspace`.
- [ ] Add `--no-workspace` to scan option parsing.
- [ ] Add `--workspace <path>` as an explicit override.
- [ ] Add `workspace status` command that prints indexed and not-indexed projects from the discovered registry.

### Task 4: Documentation And Version

**Files:**
- Modify: `internal/version/version.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `docs/RELEASE.md`

- [ ] Bump local development version to `0.8.2`.
- [ ] Document zero-config workspace reconciliation and overlay files.
- [ ] Document that scans do not automatically rescan sibling projects.

### Task 5: Verification And Commit

**Files:**
- All changed files.

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Install locally with `go install ./cmd/goregraph`.
- [ ] Verify `goregraph version` prints `0.8.2`.
- [ ] Commit locally without pushing.
