# GoreGraph 1.2 Workspace Intelligence Design

## Goal

Deliver GitHub Issues 9 through 22 as GoreGraph 1.2.0: make dense workspace
architecture readable, make every dashboard workbench actionable at normal
browser scale, and expose one evidence-backed feature-flow model to humans and
agents. The release remains additive on Schema 2 and is validated locally
against the WEKA workspace without pushing Git state.

## Product Decisions

The release uses one canonical-data approach rather than adding independent UI
and agent models. Existing `WorkspaceFeatureFlowRecord` fields remain valid for
Schema 2 consumers. New typed nodes, edges, test links, verification commands,
and impact summaries are additive projections of facts already gathered during
scan and workspace reconciliation.

The alternatives rejected for this release are:

- replacing the existing feature-flow array with a new top-level document,
  because that would break Schema 2 consumers and require Schema 3;
- generating a dashboard-only graph beside the JSON reports, because separate
  truth sources caused the caller and confidence inconsistencies this release
  is intended to prevent;
- introducing a universal graph abstraction for every GoreGraph artifact,
  because Issues 9 through 22 only require a canonical feature-flow boundary.

## Compatibility And Versioning

- `internal/version.Version` becomes `1.2.0`; `scan.SchemaVersion` remains `2`.
- Existing JSON fields, CLI commands, Query tasks, MCP tools, Markdown reports,
  and status meanings remain valid.
- `workspace-feature-flows.json` remains an array of feature-flow records. Each
  record gains additive canonical fields while retaining its legacy flat fields.
- Existing confidence values are preserved. New presentations normalize their
  descriptions without silently reclassifying stored evidence.
- Deterministic IDs identify the same extracted source identity across unchanged
  rescans. GoreGraph does not promise that IDs survive a source refactoring that
  changes the identifying project, file, symbol, signature, or source range.
- Missing coverage remains uncertainty, never proof that code, a relationship,
  or a test does not exist.

## Canonical Feature-Flow Model

Each `WorkspaceFeatureFlowRecord` gains a `model_version` and ordered `nodes`
and `edges` collections. Legacy fields continue to be populated from the same
source facts during the compatibility period.

A canonical node contains:

- deterministic `id` and `kind`;
- project and service ownership;
- display symbol, qualified name, and signature when extracted;
- source file plus start and end line when available;
- confidence, reason, and evidence IDs;
- role-specific metadata only when it is needed by a projection.

Supported node kinds are route, component, API call, endpoint, service method,
repository method, persistence entity, message, and test. IDs use the existing
stable-ID helper over kind, project, service, file, source range, qualified name,
and signature. Empty optional fields do not create competing identities.

A canonical edge contains deterministic `id`, `from_node_id`, `to_node_id`,
`edge_type`, confidence, reason, evidence IDs, and source analyzer. Initial edge
types are renders, calls, invokes API, handles endpoint, delegates, persists,
publishes, consumes, and tested by. Every weak or inferred edge has a concrete
reason; an unknown relation is represented as a gap rather than an invented
edge.

Canonical validation rejects dangling node references and duplicate IDs with
different facts. It reports contradictory ownership or source fields through
Doctor and tests. Node and edge ordering is deterministic, and semantic
fingerprints exclude generated timestamps.

## Dashboard Information Architecture

The dashboard has six top-level questions:

1. Architecture: how do services communicate?
2. Endpoints: where is an API implemented and called?
3. Feature Flow: which code chain implements a user-facing feature?
4. Data Flow: how does request and response data move and transform?
5. Diagnostics: what could not be confirmed and what should be checked?
6. Coverage: what was analyzed, what remains uncertain, and which scan helps
   most?

Each top-level view owns its selection and detail state. Switching views clears
incompatible details. A view restores prior state only where the view explicitly
documents restoration, such as returning from focused architecture to the full
architecture map.

### Architecture Full View — Mockup 1

The default Architecture view keeps stable domain lanes and stable service card
positions. Background relationships between domains are bundled and labeled
with relationship or contract counts. Selecting a service expands only the
relationships that touch that service; unrelated bundles remain visible but
strongly muted. Direct outgoing and incoming edges retain redundant direction
cues, visible card ports, and risk styling. Selection never automatically pans,
zooms, or relayouts the map.

### Focused-Service View — Mockup 2

`Focus selected` is an explicit action. It opens a readable neighborhood with
the selected service, direct callers, and direct providers grouped by domain.
Direction and endpoint counts remain visible at 100% scale. `Back to full
architecture` restores the prior selection, zoom, pan, and layout. Ordinary
selection never enters focus mode.

### Endpoints

Inventory rows identify caller and provider services. Trace details separate
service, project, controller or class, symbol, file, and line. Source values are
rendered as safe links rather than escaped HTML. Copy-path and open-source
actions provide explicit fallbacks. An optional `editor_url_template` creates
editor-specific URLs without hard-coding an IDE.

### Feature Flow

Feature Flow is a semantic HTML master-detail workbench, not a fitted SVG. One
selected flow shows route, component chain, API call, backend endpoint, service
or repository chain, persistence, and linked tests. Exact, inferred, weak, and
missing stages are distinct and explain their evidence. Users can move to the
related Endpoint or Data Flow and return without losing the Feature Flow
selection. Search and filters cover route, project, service, file, confidence,
and test-link status.

### Data Flow

Copy explicitly distinguishes the endpoint call path from the field-level data
path. Stages are request, validation, transformation, persistence, messaging,
and response. Gap cards name the missing capability or evidence and a concrete
next check. Low-evidence flows explain their limitations rather than appearing
broken.

### Diagnostics

Diagnostics becomes a semantic HTML workbench with normal vertical scrolling;
graph zoom controls are not shown. Every diagnostic family explains its plain-
language meaning, affected counts, observed and unresolved evidence, likely
owner, and next check. `unsafe_dynamic` identifies the dynamic segment and the
variable or configuration that prevents exhaustive validation. Unscanned
service, missing route, method mismatch, and frontend-internal families use the
same explanation contract across dashboard, Markdown, Query, and MCP.

### Coverage And Next Scans

Coverage separates analyzer capability from workspace completeness. Capability
rows explain complete, partial, unavailable, and failed; documentation and
configuration languages are collapsed by default. Workspace coverage shows
indexed projects, known projects, indexed referenced services, all referenced
services, and balanced contract categories. `Most useful next scans` is ordered
by affected contract count with deterministic tie-breaking and includes a scan
command only when GoreGraph knows the project path.

## Test Linkage And Verification Commands

Test relationships distinguish direct, indirect, inferred, candidate, and not
detected. Every linked test includes confidence, reason, source location, and
evidence. The wording `no linked test detected` is used when coverage cannot
prove absence.

Verification commands are structured records with tool, working directory,
argument list, display form, confidence, and reason. Maven, Gradle, Jest, Vitest,
and Playwright commands are derived only from detected project metadata and
supported invocation patterns. GoreGraph returns the missing prerequisite
instead of inventing a command when module, package, runner, or test identity is
unknown. The same records feed JSON, Markdown, task context, Query, MCP, and the
dashboard.

## Impact And Blast Radius

An impact summary is available for routes, API calls, endpoints, services,
symbols, and feature flows. It separates direct consumers, bounded indirect
consumers, dependent tests, packages or applications, and public API surface.
Each item carries relationship type, confidence, reason, source context, and
evidence IDs. Risk levels use documented deterministic inputs: relationship
depth, public surface, consumer count, test linkage, confidence, and coverage
gaps. They are not opaque scores.

Query and MCP results accept traversal depth, item limit, and continuation.
Cycles are bounded and reported. The dashboard renders `Changing this may
affect` with links to consumers, tests, and source context. Coverage gaps and
unscanned services are always shown as reasons the reported radius may be
incomplete.

## Error Handling And Trust

- Unsafe HTML, source paths, symbols, and reasons are escaped at their final
  rendering boundary.
- File URLs and editor URLs are generated only through the source-link policy;
  missing configuration falls back to copyable path and line text.
- Canonical validation fails generated-output health checks for structural
  corruption but records unsupported analysis as coverage uncertainty.
- Unsupported test runners or incomplete command metadata produce explanatory
  gaps, not executable guesses.
- Impact traversal enforces depth and result limits and reports truncation and
  continuation.
- Dashboard empty states explain what input or scan is required next.

## Testing And Delivery

Implementation follows dependency order and test-first red-green cycles. Each
GitHub issue receives its own local commit after focused tests and diff checks:

1. Issues 9 and 10 establish bundled and focused Architecture behavior.
2. Issues 11 through 16 repair and clarify existing workbenches and state.
3. Issue 18 establishes the canonical feature-flow foundation.
4. Issues 19 through 22 add Feature Flow, workspace coverage, test commands,
   and impact projections.
5. Issue 17 completes deterministic visual, responsive, keyboard, and
   cross-view acceptance for the combined dashboard.
6. A final release commit updates version and release documentation to 1.2.0.

Focused Go tests verify every builder and renderer. JavaScript syntax checks and
rendered-browser tests cover desktop and narrow viewports, 100% browser scale,
keyboard focus, empty states, long labels, selection restoration, and all
top-level transitions. Compatibility tests deserialize legacy Schema 2 feature
flows and verify that old fields retain their meanings.

Final verification runs formatting checks, `go test ./... -count=1`,
`go vet ./...`, JavaScript syntax checks, build, and `git diff --check`. The
resulting local 1.2.0 binary is installed. From
`C:\Users\goretzkh\projects\weka`, acceptance runs:

```powershell
goregraph workspace clean . --execute
goregraph workspace scan-all .
```

The regenerated workspace outputs are validated for canonical references,
balanced contract counts, coverage, test commands, and impact bounds. The
standalone dashboard is manually inspected in the browser across all six views,
including Mockup 1 and Mockup 2 behavior. GitHub Issues are closed only after
their automated and visual acceptance passes. No branch, commit, or tag is
pushed.
