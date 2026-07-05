# GoreGraph Universal Language Intelligence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Make GoreGraph substantially more useful across Java, Go, PHP, JavaScript, TypeScript, React, Python, and Shell by adding deterministic route, call, flow, test, and navigation outputs.

**Architecture:** Keep the scanner fully local and deterministic. Add a language-neutral code intelligence layer that is fed during the existing file scan, then merge Java/Spring facts with generic language facts into shared outputs such as `callgraph.json`, `routes.json`, `flows.json`, `test-map.json`, and human-readable Markdown reports.

**Tech Stack:** Go standard library only, existing GoreGraph scanner, `go test`, `go vet`, `gofmt`, GitHub Actions, GoReleaser.

---

## File Structure

- Modify `internal/scan/types.go`: add language-neutral function, route, flow, and code intelligence records.
- Create `internal/scan/code_intelligence.go`: extract generic functions, methods, calls, routes, and tests from Go, PHP, JavaScript, TypeScript, React, Python, and Shell source.
- Create `internal/scan/code_flows.go`: resolve generic callgraph edges, route flows, and method-level test mappings.
- Create `internal/scan/code_reports.go`: render `routes.md`, `flows.md`, and `navigation.md`.
- Modify `internal/scan/scan.go`: collect code intelligence during scan and write new outputs.
- Modify `internal/scan/analyzers.go`: mark PHP, JS/TS, React, Go, Python, and Shell as call/test/route capable where implemented.
- Modify `internal/query/query.go`, `internal/cli/cli.go`, `internal/mcp/mcp.go`, and `internal/doctor/doctor.go`: expose and validate new outputs.
- Modify `internal/scan/scan_test.go`, `internal/query/query_test.go`, `internal/cli/cli_test.go`, and `internal/mcp/mcp_test.go`: add RED tests before implementation.
- Modify `README.md`, `COMMANDS.md`, `SCHEMA.md`, `ROADMAP.md`, `docs/RELEASE.md`, and `AI_INTEGRATION_PLAN.md`: document the new version and outputs.
- Modify `internal/version/version.go`: bump version to `0.5.0`.

## Task 1: RED Tests For Universal Code Intelligence

**Files:**
- Modify: `internal/scan/scan_test.go`

- [x] Add a failing test with a mixed project containing:
  - Go `http.HandleFunc` and handler-to-service call.
  - PHP Laravel-style `Route::get` and controller-to-service call.
  - TypeScript/React Router route and component-to-service call.
  - Express route with handler call.
  - Python FastAPI route and function call.
  - Shell entry script sourcing helper and calling helper.
  - Matching tests for Go, PHP, TS, and Python.

- [x] Assert generated files exist:
  - `routes.json`
  - `routes.md`
  - `flows.json`
  - `flows.md`
  - `navigation.md`

- [x] Assert `callgraph.json` contains non-Java call edges.
- [x] Assert `test-map.json` contains non-Java method-level mappings.
- [x] Assert `routes.md`, `flows.md`, and `navigation.md` contain human-useful entries.
- [x] Run `go test ./internal/scan -run TestRunExtractsUniversalLanguageIntelligence -count=1`.
- [x] Confirm RED because the new outputs and types do not exist yet.

## Task 2: Language-Neutral Types And Scan Wiring

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/scan.go`
- Modify: `internal/doctor/doctor.go`

- [x] Add records:
  - `CodeIntelligenceRecord`
  - `CodeFunctionRecord`
  - `CodeCallRecord`
  - `CodeRouteRecord`
  - `CodeFlowRecord`
  - `CodeFlowStep`

- [x] Add `Code CodeIntelligenceRecord` to `Index`.
- [x] Add generated outputs:
  - `routes.json`
  - `flows.json`
  - `routes.md`
  - `flows.md`
  - `navigation.md`

- [x] Add doctor validation for `routes.json` and `flows.json`.
- [x] Run the RED scan test again and confirm it still fails at extraction expectations, not missing files.

## Task 3: Generic Extractors

**Files:**
- Create: `internal/scan/code_intelligence.go`

- [x] Implement `extractCodeIntelligence(file FileRecord, body string) CodeIntelligenceRecord`.
- [x] Extract function/method/test declarations for:
  - Go via lightweight text fallback over existing parser symbols.
  - PHP functions, methods, classes, and controller methods.
  - JavaScript/TypeScript functions, arrow functions, methods, exported functions, React components, and tests.
  - Python classes, functions, methods, FastAPI handlers, and tests.
  - Shell functions.

- [x] Extract route records for:
  - Go `http.HandleFunc`, `.GET/.POST/.PUT/.DELETE/.PATCH`, and `HandleFunc`.
  - PHP `Route::get/post/put/delete/patch`.
  - JS/TS Express/Fastify router calls.
  - React Router `<Route path=... element={<Component />}>` and `{ path: "...", element: <Component /> }`.
  - Python FastAPI/Flask decorators such as `@app.get("/path")`.

- [x] Extract call records inside function bodies with conservative keyword filtering.
- [x] Run the scan test and confirm missing resolution/report expectations remain RED.

## Task 4: Generic Callgraph, Route Flows, And Test Map

**Files:**
- Create: `internal/scan/code_flows.go`
- Modify: `internal/scan/scan.go`

- [x] Implement `buildGenericCallGraph`.
- [x] Merge generic edges into existing `CallGraphRecord`.
- [x] Implement `buildCodeFlows`.
- [x] Merge Spring endpoint flows into generic flow output.
- [x] Implement `buildGenericTestMap`.
- [x] Append generic test map records to the existing Java test map.
- [x] Add call relations to `relations.json`, `relations-full.json`, `graph.json`, and `graph-full.json`.
- [x] Run the scan test and confirm it passes.

## Task 5: Reports, Query, CLI, MCP, And Analyzer Inventory

**Files:**
- Create: `internal/scan/code_reports.go`
- Modify: `internal/query/query.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/mcp/mcp.go`
- Modify: `internal/scan/analyzers.go`
- Modify: tests for query, CLI, MCP as needed.

- [x] Render `routes.md` with route kind, framework, method, path, handler/component, file, and line.
- [x] Render `flows.md` with route-to-handler-to-call steps.
- [x] Render `navigation.md` with primary entrypoints, routes, central files, tests, and analyzer notes.
- [x] Add query aliases:
  - `routes`
  - `routes-json`
  - `flows`
  - `flows-json`
  - `navigation`

- [x] Update MCP `get_output` descriptions.
- [x] Mark implemented analyzers as call/route/test capable.
- [x] Run targeted tests for scan, query, CLI, and MCP.

## Task 6: Documentation And Version

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `SCHEMA.md`
- Modify: `ROADMAP.md`
- Modify: `docs/RELEASE.md`
- Modify: `AI_INTEGRATION_PLAN.md`
- Modify: `internal/version/version.go`

- [x] Bump version to `0.5.0`.
- [x] Document all new output files.
- [x] Document which language capabilities are extracted and which remain best-effort.
- [x] Document Homebrew upgrade behavior.
- [x] Document that outputs are deterministic and local.

## Task 7: Verification, Real Scans, Cleanup, Commit, Push, Release

**Files:**
- Inspect generated output under real project roots only.

- [x] Run `gofmt -w`.
- [x] Run `go test -count=1 ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go build -o /tmp/goregraph-dev ./cmd/goregraph`.
- [x] Run `/tmp/goregraph-dev scan /Users/gorecode/projects/weka/microservices/ms-cadaster --no-update-gitignore`.
- [x] Run `/tmp/goregraph-dev scan /Users/gorecode/projects/weka/frontend/frontend-monorepo --no-update-gitignore`.
- [x] Inspect `routes.md`, `flows.md`, `navigation.md`, `callgraph.md`, and `test-map.md` from both projects.
- [x] Update docs if the real scans expose wording gaps.
- [x] Commit all changes.
- [x] Push `main`.
- [x] Create and push tag `v0.5.0`.
- [x] Watch GitHub Actions release workflow.
- [x] Verify GitHub Release and Homebrew formula update.

## Self-Review

- [x] Spec coverage: Java, Go, PHP, JS/TS/React, Python, and Shell all get deterministic route/call/test/navigation handling.
- [x] Placeholder scan: no task contains TODO/TBD implementation placeholders.
- [x] Type consistency: route, flow, callgraph, test-map, query, doctor, and docs use the same file names and field names.
