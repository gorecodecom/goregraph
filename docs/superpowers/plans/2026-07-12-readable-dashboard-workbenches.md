# Readable Dashboard Workbenches Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Endpoint Inventory and Data Flow readable at normal 100% scale and make Architecture relationship direction and card attachment unambiguous.

**Architecture:** Add one HTML workbench layer beside the existing SVG layer. Inventory-like views render semantic HTML at browser scale; Architecture and implementation traces retain SVG pan/zoom. Focused Architecture edges use calculated card-boundary ports, redundant direction cues, and opaque node masking.

**Tech Stack:** Go-generated standalone HTML, CSS, vanilla JavaScript, SVG, Go tests.

## Global Constraints

- Keep `internal/version.Version` exactly `1.0.0` and `scan.SchemaVersion` exactly `2`.
- Do not add dependencies or change JSON/Markdown/Query/MCP contracts.
- Preserve all existing Endpoint filters, trace navigation, Architecture layout positions, search, selection, and Data Flow records.
- Do not open a browser; validate generated HTML statically and report residual visual risk.
- Follow `docs/design-system.md` and the approved UI specification.

---

### Task 1: Shared Workbench Layer And Toolbar State

**Files:**
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `#workspace-workbench`, `setCanvasPresentation(kind)`, and CSS states `main.graph-view` / `main.workbench-view`.
- Consumes: existing `#workspace-graph`, `.canvas-tools`, `renderCanvas()`.

- [ ] Add a failing renderer test requiring a hidden semantic workbench container and a presentation function that hides graph controls for inventory/Data Flow and restores them for Architecture/traces.
- [ ] Run `GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run TestDashboardSwitchesBetweenGraphAndReadableWorkbench -count=1`; expect failure for missing workbench contract.
- [ ] Add `<section id="workspace-workbench" ...>` next to the SVG, scoped workbench styles, and `setCanvasPresentation` without changing the shell grid.
- [ ] Call presentation switching from `renderCanvas()` based on mode and whether an endpoint trace is open.
- [ ] Re-run the focused test; expect pass, then commit `feat: add readable dashboard workbench layer`.

### Task 2: Readable Endpoint Inventory

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Consumes: `endpointRowsForService(serviceId)`, `endpointRowMatchesFilters(row)`, `selectItem(id)`, existing filter state.
- Produces: `renderEndpointInventoryWorkbench()`, `.endpoint-inventory`, `.endpoint-inventory-row`, saved `endpointInventoryScrollTop`.

- [ ] Replace the obsolete SVG-size assertion with failing tests requiring semantic Caller/Endpoint/Provider columns, button rows, wrapping, saved scroll position, and absence of SVG row construction in inventory rendering.
- [ ] Run the focused Endpoint tests and confirm they fail because the SVG inventory remains.
- [ ] Render filtered rows into the workbench as keyboard-native `<button>` elements with three labelled cells, status metadata, `aria-pressed`, and long-text wrapping.
- [ ] Save workbench scroll before opening a trace and restore it on return; keep filters and selected service unchanged.
- [ ] Ensure opening a trace switches back to SVG and graph controls, while returning switches to the HTML inventory.
- [ ] Run Endpoint dashboard tests; expect pass, then commit `feat: render readable endpoint inventory`.

### Task 3: Data Flow Master-Detail Workbench

**Files:**
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: `state.selectedDataFlow`, `renderDataFlowList()`, `renderDataFlowWorkbench()`, `showDataFlowNodeDetails(flow,node)`.
- Consumes: `dataFlows`, `state.query`, common confidence and evidence fields.

- [ ] Add failing tests requiring initial selection guidance, a sidebar button per flow, exactly one selected-flow chain in the workbench, explicit gap blocks, node detail actions, and responsive horizontal/vertical CSS.
- [ ] Run the focused Data Flow tests; expect failure for the existing all-flow SVG renderer.
- [ ] Add selected-flow state and render the sidebar as accessible buttons with selected semantics.
- [ ] Render no-selection guidance or one ordered flow chain with semantic nodes, connectors, confidence, source summary, and explicit gap blocks.
- [ ] Wire node selection to the details panel without changing zoom/pan or other view state.
- [ ] Add narrow-layout vertical flow CSS and run focused tests; expect pass, then commit `feat: add data flow master detail workbench`.

### Task 4: Architecture Direction And Attachment Ports

**Files:**
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Test: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Produces: SVG markers `arrow-outgoing` / `arrow-incoming`, `edgePortPoints(from,to)`, `architectureDirection(edge,selected)`, `.edge.outgoing`, `.edge.incoming`, `.edge-port`, and node `IN`/`OUT` badges.
- Consumes: stable `architectureLayout`, `state.positions`, `filteredServiceEdges()`, selected service ID.

- [ ] Add failing tests for distinct incoming/outgoing styles and markers, calculated card-boundary ports, opaque card layer ordering, distributed port offsets, and textual direction badges.
- [ ] Run focused Architecture tests; expect failure for the current shared focused class and center-to-center curves.
- [ ] Add larger teal outgoing and amber incoming markers plus non-color stroke differentiation.
- [ ] Calculate source/target points at card borders with short terminal segments and deterministic per-node port offsets.
- [ ] Render background edges first, focused edges second, opaque node cards above edges, then visible port dots and `IN`/`OUT` badges.
- [ ] Preserve all node positions and selection/isolation behavior; run focused tests and commit `feat: clarify architecture relationship direction`.

### Task 5: Regression, Documentation, Installation, And WEKA Acceptance

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/design-system.md`
- Test: all Go packages

**Interfaces:**
- Verifies: version `1.0.0`, schema `2`, standalone offline dashboard output.

- [ ] Document readable workbench semantics, graph-only toolbar rules, Data Flow selection, and Architecture port/direction cues without changing version.
- [ ] Run `gofmt` on changed Go files, `GOCACHE=/private/tmp/goregraph-gocache go test ./... -count=1`, `GOCACHE=/private/tmp/goregraph-gocache go vet ./...`, and `git diff --check`; expect clean success.
- [ ] Run the frontend design review against generated HTML contracts and fix all blocker/should-fix findings that can be verified without a browser.
- [ ] Install with `go install ./cmd/goregraph` and replace `/opt/homebrew/bin/goregraph-local`; verify `goregraph 1.0.0`, schema 2.
- [ ] In `~/projects/weka`, preview `goregraph workspace clean .`, execute only generated outputs, and run `goregraph workspace scan-all .`; expect 44 projects.
- [ ] Verify Doctor, Query, MCP, 44 Schema 2 manifests, workbench/toolbar/port HTML contracts, and dashboard output presence.
- [ ] Repeat `workspace scan-all`, compare deterministic JSON hashes, and confirm no referenced services are missing.
- [ ] Commit `docs: document readable dashboard workbenches`; do not push, tag, or publish.
