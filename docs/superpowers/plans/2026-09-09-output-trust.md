# Output trust and dashboard usefulness implementation plan

> **For agentic workers:** Use `superpowers:executing-plans` task by task. Use `superpowers:subagent-driven-development` only when parallel agent execution is explicitly authorized. Steps use checkboxes for tracking.

**Goal:** Preserve trustworthy output through failures, expose freshness honestly, reduce repeated work, and make three developer journeys effective.

**Architecture:** Add a shared transactional output boundary for built-in readers and writers, retain the existing public output paths, and use additive generation/input metadata. Apply pure extraction reuse and measured in-memory indexing before considering storage replacement. Improve existing dashboard views and their navigation.

**Tech Stack:** Go 1.23-compatible standard library, existing offline HTML/CSS/JavaScript dashboard, Go tests and browser flow verification.

**Spec:** [Improvement design](../specs/2026-09-09-goregraph-improvement-design.md); prerequisite contracts are in [slice A](2026-09-09-scan-reliability.md).

## Global constraints

- Preserve Windows, Linux, and macOS support; the repository already tests all three in CI.
- Keep scans local, deterministic, offline, and free of execution of scanned project code.
- Keep existing CLI aliases and agent/dashboard/all targets working; document additive fields and any deliberate protocol change.
- Separate output structural integrity, analysis coverage, and freshness. A usable output can have explicitly partial analysis; it must never silently claim full coverage.
- Keep source confidentiality, path confinement, configuration-value redaction, and bounded agent payloads intact.
- Do not modify agent configuration, Git configuration, or user source automatically.
- Keep private WEKA data outside committed fixtures and public CI.

## Task B1: Stage, validate and recover output publication

**Files:** create `internal/outputstore/store.go`, `internal/outputstore/store_test.go`, `internal/outputstore/lock_windows.go`, `internal/outputstore/lock_unix.go`; modify `internal/scan/scan.go`, `internal/scan/workspace_reconcile.go`, `internal/scan/output_layout.go`, `internal/scan/workspace_dashboard.go`, `internal/scan/workspace_dashboard_script.go`, `internal/agent/context_load.go`, `internal/agent/service.go`, `internal/query/query.go`, `internal/doctor/doctor.go`, `internal/dashboardeditor/server.go` and remaining output readers found by the inventory below. Add failure cases to existing output-layout and workspace reconciliation tests; update `SCHEMA.md` and `docs/OUTPUTS.md`.

**Consumes:** A4 identities and A2 context/events. `outputstore` must not import `scan`, `agent` or `doctor`.
**Produces:** these interfaces and additive manifest `generation_id` plus per-projection input identities:

```go
type UpdateRequest struct {
    Root string
    Write func(stage string) error
    Validate func(stage string) error
}
func Update(ctx context.Context, request UpdateRequest) error
func UpdateMany(ctx context.Context, requests []UpdateRequest) error
func WithRead(ctx context.Context, root string, read func(committed string) error) error
func Recover(ctx context.Context, root string) error
```

- [ ] Inventory `os.ReadFile`, JSON loaders, `RemoveAll`, `Rename`, manifest writers and dashboard source readers in `internal`. Record every output consumer that must use `WithRead`; source-code reads remain under their existing confinement rules. Include sibling project overlays written by workspace reconciliation.
- [ ] Add the basic last-good-output regression before implementing the store:

```go
func TestFailedWritePreservesPreviousOutput(t *testing.T) {
    root := filepath.Join(t.TempDir(), "out")
    if err := os.MkdirAll(root, 0755); err != nil { t.Fatal(err) }
    original := filepath.Join(root, "sentinel")
    if err := os.WriteFile(original, []byte("previous"), 0644); err != nil { t.Fatal(err) }
    err := Update(context.Background(), UpdateRequest{Root: root,
        Write: func(string) error { return errors.New("injected write failure") },
        Validate: func(string) error { return nil },
    })
    if err == nil { t.Fatal("expected write failure") }
    got, readErr := os.ReadFile(original)
    if readErr != nil || string(got) != "previous" { t.Fatalf("lost previous output: %q %v", got, readErr) }
}
```

- [ ] Implement same-filesystem staging, validation before publication, OS-backed locks and a journal with prepared/publishing/committed states. Journal entries contain normalized target, stage, backup and generation identities; validate all paths against the intended output parent. Flush journal changes before destructive transitions. Never execute a recursive deletion based on an unchecked journal path.
- [ ] Implement rollback and restart recovery for interrupted directory moves. Retain the previous complete result until the new commit marker is durable. Multi-output publication stages all targets, locks in canonical absolute-path order, verifies expected generation identities and journals rollback information before any promotion. Include rollback for a failure after the first sibling overlay has been promoted.
- [ ] Make readers acquire a stable read boundary and reject an unresolved journal with a specific error. Read-only query/Doctor calls do not repair files. Build/update calls recover their own prior interrupted publication before beginning a new mutation. Shared locks must release before nested independent loads, or use one scoped read of all required output roots in canonical order.
- [ ] Preserve current output layout and use generation-qualified immutable lazy dashboard assets. Publish the new HTML only after all of its assets exist; retain assets used by the previous complete HTML. Document that arbitrary third-party direct JSON reads are outside built-in lock guarantees.
- [ ] Inject failures at stage creation, write, validation, journal sync, backup rename, promotion, manifest commit and cleanup. Add Windows open-file/sharing failures, concurrent reader/writer, process interruption and repeated recovery. Verify no mixed generation reaches `WithRead` and no source/config file is moved or deleted.
- [ ] Run `go test ./internal/outputstore ./internal/scan ./internal/agent ./internal/query ./internal/doctor ./internal/dashboardeditor -count=1`; run lock/race tests on supported CI configurations. Commit only after the Windows transaction tests pass.

**Acceptance:** errors and cancellation leave a recoverable last-good result; built-in clients never serve a mixed snapshot as current. If stable paths cannot meet this contract, stop and document a schema/layout migration before changing them. Suggested commit: `Publish generated outputs through recoverable transactions`.

## Task B2: Make freshness, coverage and diagnostics coherent

**Files:** modify `internal/scan/freshness.go`, `internal/scan/coverage_report.go`, `internal/scan/workspace_coverage.go`, `internal/scan/canonical_diagnostics.go`, `internal/scan/workspace_dashboard_template.go`, `internal/scan/workspace_dashboard_script.go`, `internal/agent/context.go`, `internal/agent/context_source.go`, `internal/doctor/doctor.go`; extend their corresponding tests. Create `internal/scan/projection_health.go` and `internal/scan/projection_health_test.go`. Update `SCHEMA.md`, `docs/OUTPUTS.md`, `COMMANDS.md`.

**Consumes:** generation/input identities, partial-file diagnostics, stable output reads.
**Produces:** shared projection-health model; each axis has its own field:

```go
type ProjectionHealth struct {
    GenerationID string
    Integrity string // valid, invalid, unavailable
    Freshness string // current, stale, unknown
    Coverage string // complete, partial, unsupported, unknown
    Reasons []string
}
```

- [ ] Add a regression fixture that builds both targets, modifies one source and rebuilds only the agent target. Assert the previous dashboard remains structurally valid but is stale; its generation timestamp alone must not imply currentness.
- [ ] Add cases for unsupported language, file timeout, unreadable input, stale analyzer revision, missing optional projection and unresolved contract. Verify `UNRESOLVED` does not turn into a missing provider claim when provider coverage is incomplete.
- [ ] Derive structural health from committed manifests/validation, freshness from verified input identities and coverage from analysis outcomes. If live inputs were not checked, report `unknown`; expose the last successful generation date separately. Context requests can verify selected source files without claiming the entire workspace was revalidated.
- [ ] Give diagnostics stable reason codes and direct actions: missing index -> build command; changed identity -> update; timed-out file -> file path and budget; ambiguous route -> candidate evidence; unsupported construct -> analyzer limitation. Redact configuration values and never label unknown authorization as public.
- [ ] Update Doctor, CLI/context metadata and dashboard state labels together. Add cross-surface fixtures proving they agree for the same output. Keep schema-3 outputs readable with unknown identity until the next rebuild.
- [ ] Run `go test ./internal/scan ./internal/agent ./internal/doctor ./internal/cli -count=1`; inspect stale/partial/unknown states in the browser. Commit after terminology and evidence are consistent.

**Acceptance:** users and agents can distinguish old, partial, unsupported and broken data; neither stale nor partial data silently becomes definitive. Suggested commit: `Separate projection integrity freshness and analysis coverage`.

## Task B3: Reuse pure extraction without stale relationships

**Files:** create `internal/scan/script_fact_cache.go`, `internal/scan/script_fact_cache_test.go`; modify `internal/scan/scan.go`, `internal/scan/workspace_update.go`, `internal/scan/audit.go`, existing scan/update benchmarks and artifact-cleanup logic; update `docs/SCAN-BENCHMARKING.md`, `docs/OUTPUTS.md`.

**Consumes:** A3 pure script facts and A4 extractor identity; B1 safe metadata writes.
**Produces:** file-level cache methods with no cross-project-resolution results:

```go
type ScriptFactCacheKey struct { Path, ContentHash, ExtractorRevision string }
type ScriptFactCache struct { Root string }
func (c ScriptFactCache) Load(key ScriptFactCacheKey) (ProjectSymbolFacts, bool)
func (c ScriptFactCache) Store(key ScriptFactCacheKey, facts ProjectSymbolFacts) error
```

- [ ] Use A0/A3 profiles to confirm repeated extraction still contributes materially to one-file-update time. If it does not, record the evidence and retain no persistent cache; satisfy the update gate through the actually measured bottleneck instead of implementing unused infrastructure.
- [ ] Add table tests for hit, content change, path rename, extractor revision change, corrupt/truncated entry and an unknown cache schema. A miss must never return partial cached facts. Verify identical size/mtime with changed bytes still misses.
- [ ] Implement keys from relative path, SHA-256 content hash and extractor revision. Store under excluded generated output, atomically replace individual entries, validate schema/identity on read, and treat a corrupt entry as a miss. Cache no raw source bodies or environment/configuration values. Prune deleted/old-revision entries after a successful build; retain at most current and previous extractor revision.
- [ ] Re-run resolution for affected projects after any import/export/package/alias change. Include a test where a provider export changes while the caller body is unchanged; the cached caller facts must not preserve the former resolved target. Reconciliation still uses current project generations.
- [ ] Measure cold/warm complete scans and one-file updates separately. Require semantic output equality against an uncached build after normalized generation metadata, plus the spec's update-time target. If cache disk/I/O overhead exceeds its gain, remove it and retain the measurements.
- [ ] Run `go test ./internal/scan -count=1`, update cleanup/Doctor tests, and commit the justified optimization independently.

**Acceptance:** no cache hit can fabricate freshness, and relationships update when providers change. Suggested commit: `Reuse content-addressed script extraction facts`.

## Task B4: Remove measured projection and loading overhead

**Files:** investigate `internal/scan/workspace_symbols.go`, `internal/scan/workspace_reconcile.go`, `internal/scan/workspace_dashboard.go`, `internal/scan/workspace_dashboard_script.go`, `internal/agent/context_load.go`, `internal/agent/context_paths.go`, `internal/agent/context_rank.go`; extend existing benchmarks/tests in the changed component. Create only a focused lookup/helper file in that component if the profile identifies a repeated lookup.

**Consumes:** A0 fixed workloads; existing dashboard usage shards; C0 query fixtures.
**Produces:** a before/after cost breakdown and only profile-justified changes.

- [ ] Measure JSON decode/validation, ranking/traversal, repeated relation joins, serialization, HTML payload and first interaction. Separate CLI cold process loading from repeated MCP requests. Record median, tail observations, allocation and output-byte counts.
- [ ] For the leading repeated lookup, write a semantic equality regression using the fixed fixture, then replace repeated linear scans with a per-operation map or adjacency index. Do not keep whole decoded workspaces across requests without checking generation identity. Avoid additional maps when the profile shows decoding or serialization dominates.
- [ ] Avoid building dashboard-only report strings during agent-only builds where the existing target contract does not need them. Keep canonical required index data intact. Verify requested projections and preserved stale projections explicitly.
- [ ] Reuse existing usage-asset lazy loading. If the initial payload is the bottleneck, move only the measured heavy inactive-view payload behind immutable generation assets; preserve offline `file://` support rather than assuming an HTTP server exists.
- [ ] Run the affected existing package tests and A0 workload measurements. Compare facts and visible evidence against the baseline. Record a no-change decision when there is no demonstrated improvement; a database or format rewrite is outside this task.

**Acceptance:** performance improvements have a measured cause and preserve evidence; no automatic rewrite follows from a large JSON file alone. Suggested commit, if changed: `Reduce repeated workspace projection work`.

## Task B5: Complete three human investigation journeys

**Files:** modify only relevant portions of `internal/scan/workspace_dashboard_template.go`, `internal/scan/workspace_dashboard_script.go`, `internal/scan/workspace_dashboard_styles.go`, `internal/scan/workspace_endpoint_trace.go`, `internal/scan/workspace_impact.go`, `internal/scan/test_verification.go`; extend existing dashboard/impact/trace tests; create `docs/DASHBOARD-ACCEPTANCE.md` with the fixed flow cases. UI implementation must use the applicable frontend testing and design-review skills.

**Consumes:** B2 health/diagnostics, existing API catalog, endpoint traces, symbols and test-map evidence.
**Produces:** navigable request tracing, bounded change impact and existing-test discovery within current views.

- [ ] Define synthetic journey fixtures with expected source identities before editing the UI: one resolved frontend/provider chain; one ambiguous provider pair; one changed symbol with linked tests; one missing/stale index. Record exact expected evidence and uncertainty, not just page titles.
- [ ] Add backend projection tests asserting the relevant IDs and links survive the journey. Keep multiple candidates explicit; a direct source link must not imply the graph has proved an otherwise unresolved edge.
- [ ] Implement forward/back navigation preserving query, selected project and filters. Connect endpoint evidence to implementation and tests. Provide actionable empty states for no provider, no test evidence and unsupported analysis. Reuse current views and source-link construction.
- [ ] Verify in a real browser: keyboard-only completion, visible focus, 200% zoom, narrow desktop viewport, offline open, delayed asset load, stale generation, missing asset and old open tab across a rebuild. Capture only synthetic screens for committed/public evidence.
- [ ] Run applicable Go tests and automated browser flow tests where a maintained runner already exists; otherwise document exact reproducible browser steps and outcomes rather than claiming DOM string tests prove usability. Perform the required frontend design review and fix observed accessibility/navigation defects.
- [ ] Compare each journey against its expected final evidence and complete the acceptance record. Commit UI and related evidence-link changes without unrelated redesign.

**Acceptance:** all three workflows reach the expected source and test evidence, with uncertainty visible and no lost navigation context. Suggested commit: `Connect dashboard investigation paths to evidence and tests`.

## Slice B exit gate

- [ ] Last-good-output recovery passes on all supported operating systems.
- [ ] Dashboard and agent loaders agree on the committed generation and freshness state.
- [ ] Cache/load changes preserve semantic evidence and have retained performance measurements.
- [ ] All three human workflows pass; C's agent workflow receives the same health and generation guarantees.
