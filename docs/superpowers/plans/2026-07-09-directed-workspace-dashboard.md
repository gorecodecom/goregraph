# Directed Workspace Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the raw default workspace graph with clear, directed Service Map and Endpoint Trace views that explain what calls what and why.

**Architecture:** Keep `workspace-graph.json` as the raw normalized graph, but add precomputed service relationship and endpoint trace JSON files. The standalone dashboard consumes those higher-level artifacts first, and exposes the raw graph only as an advanced fallback.

**Tech Stack:** Go stdlib, generated static HTML/CSS/JS, `go test`, deterministic JSON outputs, no new runtime dependencies.

## Global Constraints

- Version stays `0.9.0`; do not commit or push unless explicitly requested.
- Keep the dashboard local/offline; no remote assets, no external JS/CSS.
- TDD: every behavior change starts with a failing Go test.
- Use stable IDs derived from project, route, file, and symbol data.
- The UI must be generic across workspace layouts, not WEKA-specific.
- The default dashboard must answer: service relationships, endpoint trace, direction, owner, evidence, and next useful action.

---

### Task 1: Service Relationship Model

**Files:**
- Modify: `internal/scan/types.go`
- Create: `internal/scan/workspace_service_map.go`
- Create: `internal/scan/workspace_service_map_test.go`

**Interfaces:**
- Produces: `BuildWorkspaceServiceMap(registry WorkspaceRegistryRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord) WorkspaceServiceMapRecord`
- Produces: `WorkspaceServiceMapRecord`, `WorkspaceServiceNodeRecord`, `WorkspaceServiceEdgeRecord`

- [ ] Write a failing test that a frontend project calling a backend route creates one directed edge from API project to backend project.
- [ ] Verify the test fails because the service map types/function do not exist.
- [ ] Add the service map types and builder.
- [ ] Verify the service map test passes.

### Task 2: Endpoint Trace Model

**Files:**
- Modify: `internal/scan/types.go`
- Create: `internal/scan/workspace_endpoint_trace.go`
- Create: `internal/scan/workspace_endpoint_trace_test.go`

**Interfaces:**
- Produces: `BuildWorkspaceEndpointTraces(matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, dossiers []FeatureDossierRecord) WorkspaceEndpointTraceIndexRecord`
- Produces: `WorkspaceEndpointTraceRecord`, `WorkspaceEndpointTraceStepRecord`, `WorkspaceEndpointTraceEdgeRecord`

- [ ] Write a failing test that a resolved frontend contract produces a left-to-right trace: consumer -> contract -> backend route -> backend handler.
- [ ] Verify the test fails because the trace types/function do not exist.
- [ ] Add the trace types and builder.
- [ ] Verify the endpoint trace test passes.

### Task 3: Workspace Reconcile Outputs

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/scan.go`
- Modify: `docs/OUTPUTS.md`
- Modify: `docs/RELEASE.md`

**Interfaces:**
- Writes: `.goregraph-workspace/workspace-service-map.json`
- Writes: `.goregraph-workspace/workspace-endpoint-traces.json`

- [ ] Write a failing reconcile test that checks both new workspace JSON outputs exist.
- [ ] Verify the test fails.
- [ ] Wire builders into `ReconcileWorkspace`.
- [ ] Add generated-file placeholders for project scans where needed.
- [ ] Update output/release docs.
- [ ] Verify targeted reconcile tests pass.

### Task 4: Dashboard UX

**Files:**
- Modify: `internal/scan/workspace_dashboard.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Changes: `RenderWorkspaceDashboardHTML(graph, matches, dossiers)` computes service map and traces internally for embedded dashboard payload.

- [ ] Write failing dashboard tests for `Service Map`, `Endpoint Trace`, directional arrows, incoming/outgoing sections, and absence of raw connection ID dumps in the default details.
- [ ] Verify the tests fail against the current dashboard.
- [ ] Replace the default raw-lane render with a service relationship view.
- [ ] Add endpoint trace mode with readable left-to-right steps.
- [ ] Keep search, filters, zoom, pan, fit, and labels.
- [ ] Keep raw graph as an advanced mode.
- [ ] Verify dashboard tests pass.

### Task 5: Workspace Refresh Command

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Adds: `goregraph workspace refresh [path] [--workspace <path>]`

- [ ] Write a failing CLI test that `workspace refresh` rewrites `.goregraph-workspace` from existing project outputs without scanning sources.
- [ ] Verify the test fails.
- [ ] Add the CLI command that calls `scan.ReconcileWorkspace`.
- [ ] Update CLI help text.
- [ ] Verify CLI tests pass.

### Task 6: Ten Iteration Verification Loop

**Files:**
- Use generated WEKA outputs under `/Users/gorecode/projects/weka`

**Interfaces:**
- Uses: `goregraph workspace clean . --execute`
- Uses: `goregraph workspace scan-all .`
- Uses: `goregraph workspace refresh .`

- [ ] Build and install local `goregraph 0.9.0`.
- [ ] Clean WEKA workspace outputs.
- [ ] Run `goregraph workspace scan-all .` from `/Users/gorecode/projects/weka`.
- [ ] Inspect `.goregraph-workspace/workspace-map.html`, service map JSON, endpoint traces JSON, and textual reports.
- [ ] Repeat up to 10 review/fix/verify passes, stopping early only if additional passes show no meaningful dashboard clarity gain.
- [ ] Run full `go test ./...`, `go vet ./...`, `go build ./cmd/goregraph`.
- [ ] Reinstall local binary after the final code change.

## Design Brief

- Product world: local code intelligence for humans and coding agents working across multi-repo workspaces.
- Audience: engineers who do not know the workspace yet and need evidence before editing.
- Primary job: show directed service and endpoint relationships without requiring graph-theory decoding.
- Target perception after 5 seconds: "I can see which service calls which service, and I can trace this endpoint end to end."
- Style direction: dense utility workbench, restrained, evidence-first, readable paths.
- Anti-references: raw hairball graph, anonymous circles, hover-only meaning, decorative dashboards.
- Quality bar: solid production utility.

## Visual Contract

- Default view: service-to-service map, not raw graph.
- Service selection: incoming and outgoing relationships with arrow direction, route count, confidence, and endpoint examples.
- Endpoint selection: horizontal trace lanes: Consumer -> API Contract -> Backend Route -> Handler -> Backend Steps -> Tests/Risks.
- Detail panel: human labels and grouped evidence, no raw ID wall.
- Raw graph: available as advanced/debug mode only.

## Self-Review

- Spec coverage: covers directed services, endpoint direction, zoom/readability, GitNexus-style precomputed relational intelligence, local install, clean/scan/review loop.
- Placeholder scan: no TODO/TBD placeholders.
- Type consistency: service map and endpoint trace types are named consistently and consumed by reconcile/dashboard.
