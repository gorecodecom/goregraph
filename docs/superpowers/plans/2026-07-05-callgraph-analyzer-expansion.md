# GoreGraph Call Graph And Analyzer Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Make GoreGraph a stronger Graphify-like local code graph by adding endpoint hardening, method-level Java/Spring call flows, method-aware test mapping, and a language-neutral analyzer inventory.

**Architecture:** Keep all changes local, deterministic, and additive. Preserve existing outputs while enriching `graph-full.json`, `relations-full.json`, `spring.json`, and Markdown reports with normalized `type`, call, endpoint-flow, test, and analyzer metadata.

**Tech Stack:** Go standard library, existing GoreGraph scanner, `go test`, `go vet`, `gofmt`.

---

## Scope

This plan implements the combined release scope previously discussed as `0.2.2`, `0.3.0`, and `0.4.0`:

- Endpoint hardening: path normalization, multipart/request-part support, correct handler detection.
- Schema hardening: `graph-full.json` edges expose `type` consistently while keeping `relation` for compatibility.
- Java/Spring method graph: method call records, service/repository flow hints, endpoint-to-method flow output.
- Test mapping: method-level Java test relations and endpoint hints for MockMvc-style tests.
- Analyzer structure: language-neutral analyzer inventory for Java, Go, JavaScript, TypeScript, Python, PHP, Shell, Markdown, JSON, YAML, Maven, Composer, and Node package metadata.

## Files

- Modify `internal/scan/types.go`: add call graph, endpoint flow, analyzer, multipart, and test mapping types.
- Modify `internal/scan/extract_java.go`: improve method body tracking, annotation parsing, parameter annotation parsing, method calls, and static call handling.
- Modify `internal/scan/spring_extract.go`: fix path parsing, multipart detection, endpoint request metadata, and build endpoint flows.
- Modify `internal/scan/rich_build.go`: add `type` to rich edges and include call/test relation classes.
- Modify `internal/scan/scan.go`: wire new outputs.
- Modify `internal/scan/report.go` and `internal/scan/spring_report.go`: render method-aware test maps, call flows, and analyzer inventory.
- Create `internal/scan/java_callgraph.go`: resolve Java method calls and method-level test mappings.
- Create `internal/scan/analyzers.go`: emit language-neutral analyzer capability records.
- Modify `internal/scan/rich_graph_test.go`: add regression tests for endpoints, multipart, calls, flows, schema consistency, and analyzer inventory.
- Modify `internal/query/query.go`, `internal/cli/cli.go`, `internal/mcp/mcp.go`: expose new outputs if needed.
- Update `README.md`, `COMMANDS.md`, `SCHEMA.md`, `ROADMAP.md`, `AI_INTEGRATION_PLAN.md`, `docs/RELEASE.md`.
- Update `internal/version/version.go` and version tests to `0.4.0`.

## Tasks

### Task 1: Endpoint And Rich Schema Hardening

- [x] Add failing tests for:
  - `{cadasterId}` path variables keeping their closing brace.
  - `@PostMapping(path = "/import", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)` mapping to the real method name.
  - `@RequestPart MultipartFile file` and `@RequestParam String source` request metadata.
  - `graph-full.json` edges exposing `type` and still exposing `relation`.
- [x] Run focused scan tests and verify they fail.
- [x] Fix annotation attribute splitting, Spring path splitting, parameter annotation parsing, multipart request detection, and rich edge `type`.
- [x] Run focused scan tests and verify they pass.

### Task 2: Java Method Call Graph

- [x] Add failing tests for Controller -> Service -> Repository method calls.
- [x] Add `CallGraphRecord`, `CallGraphEdgeRecord`, and `MethodRefRecord` types.
- [x] Resolve field receiver types and direct same-class calls.
- [x] Emit `callgraph.json` and call edges in `relations-full.json` / `graph-full.json`.
- [x] Add `callgraph.md` report.
- [x] Run focused tests and verify they pass.

### Task 3: Endpoint Flows

- [x] Add failing tests for endpoint flow output showing endpoint -> controller method -> service method -> repository method.
- [x] Add `SpringEndpointFlowRecord`.
- [x] Build best-effort flows from resolved Java call graph and Spring endpoints.
- [x] Emit `endpoint-flows.json` and `endpoint-flows.md`.
- [x] Run focused tests and verify they pass.

### Task 4: Method-Level Test Mapping

- [x] Add failing tests for:
  - direct test method calls to production methods.
  - MockMvc HTTP path matching to Spring endpoints.
  - confidence metadata for exact vs inferred mappings.
- [x] Add `TestMapRecord`.
- [x] Emit `test-map.json` and enrich `test-map.md`.
- [x] Run focused tests and verify they pass.

### Task 5: Analyzer Inventory

- [x] Add failing tests for `analyzers.json` and `analyzers.md`.
- [x] Implement deterministic analyzer capability records for all currently supported languages and workspace metadata scanners.
- [x] Emit analyzer reports and expose them through query/MCP aliases.
- [x] Run focused tests and verify they pass.

### Task 6: Docs, Version, Full Verification

- [x] Update docs for new outputs, query aliases, schema compatibility, and install/update notes.
- [x] Set version to `0.4.0`.
- [x] Run `gofmt -l .`, `go test -count=1 ./...`, `go vet ./...`, and `go build`.
- [x] Run a real scan against `/Users/gorecode/projects/weka/microservices/ms-cadaster` and inspect endpoint/call/test outputs.
- [x] Commit and push only if verification is green and the user expects repository state to be updated.
