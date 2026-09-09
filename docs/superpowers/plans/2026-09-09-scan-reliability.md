# Scan reliability and performance implementation plan

> **For agentic workers:** Use `superpowers:executing-plans` task by task. Use `superpowers:subagent-driven-development` only when parallel agent execution is explicitly authorized. Steps use checkboxes for tracking.

**Goal:** Make file selection correct, slow work visible and cancellable, script analysis efficient, and update selection trustworthy.

**Architecture:** Share file enumeration between scan and snapshot operations. Introduce context-aware build events and explicit build identity while retaining existing entry-point wrappers. Optimize lexical analysis without replacing the conservative module resolver.

**Tech Stack:** Go 1.23-compatible standard library, Go tests/benchmarks, existing three-platform CI.

**Spec:** [Improvement design](../specs/2026-09-09-goregraph-improvement-design.md). Read its evidence, constraints and acceptance gates before implementation.

## Global constraints

- Preserve the Go 1.23 language floor unless a separately justified compatibility decision changes it.
- Preserve Windows, Linux, and macOS support; the repository already tests all three in CI.
- Keep scans local, deterministic, offline, and free of execution of scanned project code.
- Do not add a runtime dependency without a documented gap that cannot reasonably be handled by the existing implementation or standard library.
- Keep source confidentiality, path confinement, configuration-value redaction, and bounded agent payloads intact.
- Keep private WEKA data outside committed fixtures and public CI.
- Do not modify agent configuration, Git configuration, or user source automatically.

## Task A0: Establish the baseline and classify existing failures

**Files:** create `docs/SCAN-BENCHMARKING.md`, `internal/scan/scan_benchmark_test.go`, `internal/scan/symbol_script_benchmark_test.go`; extend `.github/workflows/ci.yml` only where evidence retention requires it. Investigate existing `internal/agent/context_source_test.go`, `internal/cli/git_update_test.go` and `internal/cli/generality_acceptance_test.go`; no speculative fixes.

**Consumes:** checkout `081b405`, existing test fixtures, existing public `ExtractScriptSymbolFacts`.
**Produces:** frozen synthetic workloads, baseline test report, five-run timing/allocation measurements, and classified failure reproductions. B and C use the same workload identities.

- [ ] Capture `go version`, `goregraph version`, OS, commit, CPU/RAM, source snapshot ID and exact command arguments into an external evidence directory. Keep test output out of committed fixtures.
- [ ] Run `go test ./... -json -timeout 20m` once, retaining the complete output. Re-run only failing test names with `-count=1 -v`. Compare sandbox temporary paths with a normally owned Windows temporary workspace. Retain underlying `EvalSymlinks`, access and Git ownership errors; do not disable confinement or set global `safe.directory` to make tests green.
- [ ] Create synthetic TSX, imported calls, nested arrow/destructuring scopes and generated-bundle-shaped workloads at 16/32/64/128 KB and near the 512 KB input limit. Define `syntheticScriptSource(size int) string` in the benchmark file; generate repeated independent named functions and calls to an imported symbol, padding to the requested size with comments. No private source copies.

```go
func BenchmarkScriptSymbolFacts(b *testing.B) {
    for _, size := range []int{16384, 32768, 65536, 131072} {
        b.Run(strconv.Itoa(size), func(b *testing.B) {
            body := syntheticScriptSource(size)
            file := FileRecord{Path: "src/screen.tsx", Language: "typescript"}
            b.SetBytes(int64(len(body)))
            b.ReportAllocs()
            b.ResetTimer()
            for i := 0; i < b.N; i++ { ExtractScriptSymbolFacts(file, body) }
        })
    }
}
```

- [ ] Run `go test ./internal/scan -run '^$' -bench '^BenchmarkScriptSymbolFacts$' -benchmem -count=5`. Run CPU and allocation profiles separately from final timing runs. If `go tool pprof` is unavailable, record the profiling limitation and use a provisioned supported Go toolchain for controlled measurements; do not infer profile percentages from a stack sample.
- [ ] Document cold build, unchanged update, one-file update, output loading and context-command measurement protocols. Measure isolated processes sequentially, warm/cold separately, with fixed workloads and no concurrent test load. Record elapsed time, peak memory where available, files/bytes, facts, diagnostics and output bytes.
- [ ] Convert reproducible product defects from the existing failing tests into narrowly scoped fixes before their affected release gate. Classify environment failures separately with exact reproduction requirements. Commit baseline fixtures and methodology, not private evidence.

**Acceptance:** existing failures are named and classified; baseline measurements can be repeated; no new claim of a fully green suite without evidence. Suggested commit: `Establish scan performance and compatibility baselines`.

## Task A1: Correct ignore semantics and share enumeration

**Files:** modify `internal/gitignore/gitignore.go`, `internal/gitignore/gitignore_test.go`, `internal/scan/filter.go`, `internal/scan/scan.go`, `internal/scan/workspace_update.go`, `internal/doctor/doctor.go`; create `internal/scan/file_walk.go`, `internal/scan/file_walk_test.go`; update `COMMANDS.md` ignore semantics.

**Consumes:** `config.Config`, `FileRecord`, existing `gitignore.Matcher`.
**Produces:** `Matcher.WithFile(directory, body string) Matcher` with slash-relative scope, preserving `Parse` and `Load` compatibility. Define these scan types and use them in both extraction and snapshots:

```go
type WalkedFile struct { Path string; Size int64 }
type FileWalkReport struct {
    Visited int
    Skipped map[string]int
    IgnoreDigest string
}
func WalkProjectFiles(ctx context.Context, root string, cfg config.Config,
    visit func(WalkedFile) error) (FileWalkReport, error)
```

- [ ] Add a regression test that fails on the present implementation:

```go
func TestIgnoredDirectoryAtAnyDepth(t *testing.T) {
    m := Parse("dist-offline/\n")
    for _, path := range []string{"dist-offline", "apps/web/dist-offline"} {
        if !m.Ignored(path, true) { t.Fatalf("not ignored: %s", path) }
    }
    if m.Ignored("apps/web/dist-offline.ts", false) { t.Fatal("ignored source file") }
}
```

- [ ] Add table tests for `/dist-offline/` anchoring, directory versus file identity, nested rules, `**`, escaped prefixes, negation, ordering, trailing escaped spaces and excluded-parent behavior. Compare actual traversal results against isolated `git check-ignore --no-index` fixtures. Isolate system/global Git configuration in the fixture process, never in the user's global config.
- [ ] Preserve anchored and directory-only flags when parsing. Match unanchored slash-free names against path components; path-containing patterns against their scoped relative path. Load child `.gitignore` before traversing that directory. Maintain a per-directory inherited rule stack and hash normalized relative ignore paths plus exact rule contents.
- [ ] Implement the shared walk with context checks, default exclusions, explicit include behavior, size limits, symlink refusal and stable slash-relative paths. Always prune GoreGraph staging/backup/cache artifacts regardless of custom output directory. Return explicit I/O errors for incomplete input inventory; a failed read must not be counted as deletion by an update snapshot.
- [ ] Move scan/snapshot traversal to the shared walk, preserving binary-content handling after file reads. Make Doctor's source freshness comparison use the same file eligibility policy. Test identical scan/snapshot eligible-file sets, including nested ignores and changed rules.
- [ ] Run `go test ./internal/gitignore ./internal/scan ./internal/doctor -count=1`. Inspect a read-only current frontend enumeration: ignored bundles must be absent. Record the count, not private source. Commit after narrow tests and review.

**Acceptance:** runtime has no Git subprocess requirement, parity matrix passes, and both full scan and update exclude the observed nested build directory. Suggested commit: `Honor scoped Git ignore rules during all file scans`.

## Task A2: Add real progress, cancellation and resource outcomes

**Files:** create `internal/scan/build_options.go`, `internal/scan/build_options_test.go`; modify `internal/scan/scan.go`, `internal/scan/workspace_update.go`, `internal/scan/workspace_reconcile.go`, `internal/scan/types.go`, `internal/scan/audit.go`, `internal/cli/cli.go`, `internal/cli/workspace_progress.go`, `internal/cli/workspace_progress_test.go`; inspect and instrument long-running loops in `internal/scan/symbol_java.go`, `internal/scan/java_resolve.go`, `internal/scan/java_callgraph.go`, `internal/scan/symbol_facts.go` and `internal/scan/code_flows.go` where needed to meet cancellation; update `COMMANDS.md`.

**Consumes:** A1 enumerator and existing CLI streams.
**Produces:** the following API; wrappers `RunBuild`, `WorkspaceUpdatePlan` and `ReconcileWorkspaceTarget` use background context and default options. New CLI paths use the context-aware variants.

```go
type BuildEvent struct {
    Phase, Project, File, Outcome string
    Completed, Total int
    Elapsed time.Duration
}
type BuildOptions struct {
    FileTimeout, ProjectTimeout time.Duration
    Observer func(BuildEvent)
}
func DefaultBuildOptions() BuildOptions // FileTimeout: 5*time.Second; ProjectTimeout: 0.
func RunBuildWithOptions(ctx context.Context, root string, cfg config.Config,
    target BuildTarget, options BuildOptions) (Result, error)
func WorkspaceUpdatePlanWithOptions(ctx context.Context, root string,
    cfg config.Config, target BuildTarget, options BuildOptions) (WorkspaceUpdatePlanRecord, error)
func ReconcileWorkspaceWithOptions(ctx context.Context, root string,
    cfg config.Config, target BuildTarget, options BuildOptions) (*WorkspaceRegistryRecord, error)
```

- [ ] Add cancellation before any work as a failing regression:

```go
func TestCancelledBuildDoesNotPublish(t *testing.T) {
    root := t.TempDir()
    ctx, cancel := context.WithCancel(context.Background()); cancel()
    _, err := RunBuildWithOptions(ctx, root, config.Defaults(), BuildTargetAll, BuildOptions{})
    if !errors.Is(err, context.Canceled) { t.Fatalf("error = %v", err) }
    if _, err := os.Stat(filepath.Join(root, "goregraph-out")); !os.IsNotExist(err) {
        t.Fatalf("cancelled build created output: %v", err)
    }
}
```

- [ ] Define phases `discover`, `snapshot`, `extract`, `resolve`, `project`, `reconcile`, `validate`, `publish`; outcomes `started`, `completed`, `skipped`, `partial`, `failed`, `cancelled`. Emit file-start before expensive work and completion after it; no synthetic successful progress during a stalled phase.
- [ ] Add shared signal cancellation to build/update aliases. Validate timeout flags (nonnegative durations); default file budget 5 seconds and project budget 0. Explicit zero disables a budget; wrappers obtain defaults through `DefaultBuildOptions`, not by silently replacing zero values. Render progress to stderr, including JSON progress events, without corrupting stdout JSON results. Preserve machine-readable errors and exit 130 on Ctrl+C.
- [ ] Thread context through extraction, resolution and serialization loops. A file timeout discards that file's unfinished facts and produces a diagnostic with explicit partial coverage. A project timeout aborts publication. Do not start a timeout goroutine that leaves actual work running. Complete script-loop cancellation in A3 before declaring the stress gate passed.
- [ ] Extend fake-clock tests for known/unknown totals, phase changes, repeated heartbeat, file failure, update-plan enumeration, stderr-only progress and all four progress modes. Check throttling at one line/second and heartbeat within five seconds without wall-clock sleeps in unit tests.
- [ ] Record actual `version.Version`, invoked command, phase durations and slowest relative file identities in audit output. Limit slow-file records to 20 and avoid source contents. Run `go test ./internal/scan ./internal/cli -count=1`; commit validated behavior and documentation.

**Acceptance:** slow work is identifiable; no abandoned workers; partial coverage is visible; cancellation never publishes a success. Suggested commit: `Expose scan phases and propagate cancellation`.

## Task A3: Index script scopes once per file

**Files:** create `internal/scan/script_lexical_index.go`, `internal/scan/script_lexical_index_test.go`; modify `internal/scan/symbol_script.go`, `internal/scan/symbol_script_test.go`, A0 benchmarks and A2 cancellation integration.

**Consumes:** masked script text, file identity and build context.
**Produces:** `ExtractScriptSymbolFactsContext(context.Context, FileRecord, string) (ProjectSymbolFacts, error)`; the existing extraction function remains a background-context wrapper. A private lexical index owns delimiter pairs, line offsets, scope parentage, declaration spans and binding tables.

- [ ] Freeze existing fact output on syntax cases involving shadowing, typed arrows, destructuring, namespaces, imports, JSX, comments, regexes and malformed delimiters. Add this independent semantic regression:

```go
func TestImportedCallSurvivesUnrelatedShadowScope(t *testing.T) {
    file := FileRecord{Path: "src/view.ts", Language: "typescript"}
    facts := ExtractScriptSymbolFacts(file, "import { run } from './api';\nfunction nested(run: () => void) { run(); }\nrun();\n")
    found := false
    for _, ref := range facts.References {
        if ref.Type == "calls_export" && ref.TargetExport == "run" && ref.Line == 3 { found = true }
    }
    if !found { t.Fatal("lost imported call outside the shadowing scope") }
}
```

- [ ] Build delimiter pairs and line positions in one bounded pass over masked text. Maintain a scope stack; index parameters, variables, declarations and ownership by offsets. Resolve a use through its enclosing scopes, never by rescanning every declaration/arrow in the file. Treat unsupported constructs as unresolved with a reason.
- [ ] Replace whole-file repeated work in `scriptShadowReason`, scope lookup and line-number lookup with indexed operations. Keep module resolution memoization and output sorting. Check cancellation between bounded input chunks and usage batches.
- [ ] Add adversarial size/depth fixtures and fuzz tests checking no panics, no escaped ranges and deterministic results. Add cancellation during a bundle-shaped fixture with a controlled cancellation trigger. Do not introduce timing-based assertions into ordinary semantic unit tests.
- [ ] Run all script tests, `go test ./internal/scan -count=1`, then the five-run A0 benchmark and semantic comparison. Meet the spec's scaling target; investigate any fact decrease before accepting performance gains. Commit the lexical change separately from caching.

**Acceptance:** old accepted facts remain, false exact claims do not increase, script work cooperatively cancels, and repeated whole-file scans are removed from the hot path. Suggested commit: `Reuse lexical scope indexes during script analysis`.

## Task A4: Invalidate updates by their actual inputs

**Files:** create `internal/scan/build_identity.go`, `internal/scan/build_identity_test.go`; modify `internal/scan/output_layout.go`, `internal/scan/freshness.go`, `internal/scan/workspace_update.go`, `internal/scan/workspace_update_test.go`, `internal/scan/workspace_reconcile.go`, `internal/doctor/doctor.go`; update `SCHEMA.md`, `docs/OUTPUTS.md`.

**Consumes:** A1 ignore digest, content hashes, normalized config and requested targets.
**Produces:** an additive manifest `BuildIdentity` and per-projection input identities:

```go
type BuildIdentity struct {
    ExtractorRevision, ResolverRevision string
    AgentRevision, DashboardRevision string
    ConfigDigest, AnalysisPolicyDigest, IgnoreDigest, SourceFingerprint string
}
func CurrentBuildIdentity(cfg config.Config, options BuildOptions, ignoreDigest, sourceFingerprint string) BuildIdentity
```

- [ ] Extend the existing unchanged-project fixture test by changing only `manifest.BuildIdentity.ExtractorRevision` to `"previous"`, rewriting its manifest and asserting the update action becomes `WorkspaceUpdateActionBuild` with reason `"extractor revision changed"`. First observe the regression failure.
- [ ] Define normalized config inputs explicitly: include/exclude order where meaningful, size limit, symlink policy and analyzer-affecting options. Store per-file timeout in the analysis-policy digest, so increasing it can retry formerly partial extraction. Output location, observer, progress rendering and the project deadline do not affect successful extracted facts. Hash ignore contents separately even when `.gitignore` is excluded from source files.
- [ ] Distinguish extraction, resolution, agent and dashboard changes. Missing identity requests a one-time rebuild. A documentation-only tool version change does not invalidate facts. Persist tool version/commit separately for auditability. On target-specific updates, mark any preserved projection with changed inputs as stale; do not erase it or call it current.
- [ ] Add table regressions for source add/modify/delete/rename, unchanged files, config changes, ignored-file edits, ignore changes, extractor/resolver/projector revisions, new/removed workspace projects and dashboard layout changes. Count extractor and reconciler invocations with test hooks. An unchanged update must leave both output bytes and timestamps unchanged.
- [ ] Recheck source/ignore/config fingerprints before publication and reject changed inputs. Test a source edit injected between extraction and publication; retain prior output. B1 makes this publication guarantee recoverable across write failures.
- [ ] Run `go test ./internal/scan ./internal/doctor ./internal/cli -count=1` and `go run ./scripts/sync-docs --check`. Document one-time rebuild behavior and commit.

**Acceptance:** no silent reuse after relevant changes; no costly work for a true no-op; output integrity and stale analysis are separate states. Suggested commit: `Track analyzer and input identities for workspace updates`.

## Slice A exit gate

- [ ] All A regressions pass on Windows/Linux/macOS; existing failing tests have verified resolution or a separately documented environment reproduction, never an unexplained waiver.
- [ ] Perform read-only WEKA file enumeration and isolated candidate measurements under an approved source snapshot, with no ignored bundles entering extraction.
- [ ] Record semantic and performance deltas against A0. Proceed to B/C using the exact event and identity interfaces above.
