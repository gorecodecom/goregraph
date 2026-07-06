# Diagnostics Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GoreGraph scan output more useful for diagnosis by adding service-aware contract classification, a compact diagnostics report, better affected-file ranking, and local `0.8.1` install documentation.

**Architecture:** Reuse existing scan facts instead of adding new parsers. Derive diagnostics from routes, flows, API contracts, contract matches, endpoint flows, test mappings, and the rich graph during `writeOutputs`.

**Tech Stack:** Go CLI, existing `internal/scan` report builders, Go tests with `go test`.

---

### Task 1: Service-aware contract matching

**Files:**
- Modify: `internal/scan/contract_matches.go`
- Modify: `internal/scan/types.go`
- Test: `internal/scan/scan_test.go`

- [ ] Add a failing scan test where frontend calls both `/cadasters/{id}` and `/tasks/{id}` while only a Spring cadaster route is scanned.
- [ ] Verify the test fails because `/tasks/{id}` is currently reported as `missing_backend_route`.
- [ ] Add `unscanned_service` classification when a service candidate is known but not present in scanned backend routes.
- [ ] Preserve `missing_backend_route` for the scanned backend service.
- [ ] Run the focused test and then the scan package tests.

### Task 2: Diagnostics report

**Files:**
- Create: `internal/scan/diagnostics.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/query/query.go`
- Modify: `internal/doctor/doctor.go`
- Test: `internal/scan/scan_test.go`
- Test: `internal/query/query_test.go`

- [ ] Add a failing scan test that expects `diagnostics.json` and `diagnostics.md`.
- [ ] Include top entrypoints, risky contracts, unscanned services, endpoints without tests, weak flows, and likely tests.
- [ ] Add query aliases for `diagnostics` and `diagnostics-json`.
- [ ] Add doctor validation for `diagnostics.json`.
- [ ] Run focused scan/query tests.

### Task 3: Better affected report

**Files:**
- Modify: `internal/scan/spring_report.go`
- Test: `internal/scan/scan_test.go`

- [ ] Add a failing test showing external package labels such as `react` are not top affected entries.
- [ ] Filter dependency/package labels from `affected.md`.
- [ ] Weight local route, API, flow, and test context over generic imports.
- [ ] Run the focused affected report test.

### Task 4: Docs and version

**Files:**
- Modify: `internal/version/version.go`
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `docs/RELEASE.md`
- Test: `docs_test.go`
- Test: `release_files_test.go`

- [ ] Set development version to `0.8.1`.
- [ ] Document diagnostics outputs, `unscanned_service`, improved affected semantics, and local install commands.
- [ ] Run docs-related tests.

### Task 5: Verification and local commit

**Files:**
- All changed files.

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./...`.
- [ ] Build a local binary.
- [ ] Verify the local binary reports `0.8.1`.
- [ ] Commit locally without pushing or creating a release.
