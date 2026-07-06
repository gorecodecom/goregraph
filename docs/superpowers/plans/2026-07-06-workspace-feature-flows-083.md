# Workspace Feature Flows 0.8.3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a compact end-to-end workspace feature report to local version `0.8.3`.

**Architecture:** Extend workspace reconciliation with endpoint-flow and test-map inputs from already scanned sibling indexes. Build resolved feature-flow records from frontend API contracts matched to backend endpoints, then render workspace-level and project-level JSON/Markdown overlays.

**Tech Stack:** Go CLI, existing GoreGraph static indexes, standard library JSON/filesystem, current scan/query tests.

---

### Task 1: RED test for end-to-end workspace feature flows

**Files:**
- Modify: `internal/scan/workspace_reconcile_test.go`
- Modify: `internal/query/query_test.go`

- [ ] Add a fixture with a frontend API call, a Spring backend endpoint, service/repository calls, and a backend endpoint test.
- [ ] Assert `workspace-feature-flows.md` contains frontend API file, backend controller, service step, repository step, and test file.
- [ ] Assert `workspace-feature-flows.json` contains a record with steps and tests.
- [ ] Assert `goregraph query . workspace-features` works from a scanned project and from the workspace root.
- [ ] Run focused tests and confirm they fail because the report does not exist yet.

### Task 2: Build feature-flow data

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/workspace_reconcile.go`

- [ ] Add `WorkspaceFeatureFlowRecord`.
- [ ] Load `endpoint-flows.json` and `test-map.json` for indexed workspace projects.
- [ ] Build feature flows from matched workspace contracts, backend endpoint flows, and matching tests.
- [ ] Keep this derived from existing scan outputs; do not rescan siblings.

### Task 3: Render and expose outputs

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/query/query.go`

- [ ] Write `.goregraph-workspace/feature-flows.json` and `.goregraph-workspace/feature-flows.md`.
- [ ] Write project-local `workspace-feature-flows.json` and `workspace-feature-flows.md`.
- [ ] Add no-workspace placeholders for normal scans.
- [ ] Add query aliases `workspace-features`, `workspace-feature-flows`, and `workspace-feature-flows-json`.

### Task 4: Docs, verification, install, commit

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `docs/RELEASE.md`

- [ ] Document the new report and query aliases.
- [ ] Keep version `0.8.3`.
- [ ] Run `go test ./...` and `go vet ./...`.
- [ ] Run `go install ./cmd/goregraph` and verify `goregraph version` still reports `0.8.3`.
- [ ] Commit locally without push or release tag.
