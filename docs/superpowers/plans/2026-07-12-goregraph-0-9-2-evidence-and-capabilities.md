# GoreGraph 0.9.2 Evidence And Capabilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add stable evidence, separated status dimensions, honest capability coverage, machine-readable outputs, Doctor validation, and a Coverage dashboard view without changing existing Schema 1 meanings.

**Architecture:** New focused records in `internal/scan/evidence.go` and `internal/scan/capabilities.go` normalize existing analyzer facts into additive `evidence.json`, `capabilities.json`, and `coverage.json`. Existing records remain readable; later diagnostics, Query/MCP, and directed traces consume stable evidence IDs and enums rather than legacy overloaded confidence strings.

**Tech Stack:** Go 1.23+, standard library, JSON/Markdown, generated offline HTML/CSS/JavaScript, Go tests.

## Global Constraints

- Preserve Schema 1 meanings; all 0.9.2 fields and files are additive.
- Evidence IDs are deterministic and contain no absolute paths or timestamps.
- Confidence values: `EXACT`, `RESOLVED`, `NORMALIZED`, `INFERRED`, `WEAK`, `UNKNOWN`.
- Resolution values: `MATCHED`, `PARTIAL`, `UNRESOLVED`, `OUT_OF_SCOPE`.
- Severity values: `INFO`, `WARNING`, `ERROR`.
- Coverage values: `COMPLETE`, `PARTIAL`, `UNAVAILABLE`, `FAILED`.
- Capability IDs: `symbols`, `relations`, `calls`, `routes`, `api_clients`, `tests`, `persistence`, `messaging`, `data_flow`.
- No dependencies, network access, telemetry, project-code execution, WEKA-specific rules, push, tag, or public release.
- Every production change starts with a failing focused test.

---

### Task 0: Endpoint Debugging Filters

**Files:**
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Adds `state.endpointFilters` with `methods Set<string>`, `callers Set<string>`, `providers Set<string>`, and `statuses Set<string>`.
- Produces filtered endpoint inventory rows without changing trace records or viewport semantics.

- [ ] **Step 1: Write failing dashboard tests requiring multi-select HTTP methods, separate caller/provider service selects, status selection, active-filter summary, result count, and `Clear filters`.**
- [ ] **Step 2: Add a failing state-transition assertion proving filters survive endpoint selection and `returnToEndpointInventory()`, while `Clear filters` preserves the endpoint viewport.**
- [ ] **Step 3: Run `GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestDashboardEndpointFilters' -count=1`; expect missing controls and state.**
- [ ] **Step 4: Implement method toggle buttons for GET/POST/PUT/PATCH/DELETE/other, caller/provider `<select multiple>` controls populated from visible endpoint rows, and status toggles for resolved/unresolved/mismatch/out-of-scope. Empty sets mean all.**
- [ ] **Step 5: Apply filters in `endpointRowsForService`, combine them with free-text search, update the visible row count and active-filter summary, and keep filter state independent from `setMode`, trace selection, Fit, and viewport restoration.**
- [ ] **Step 6: Add accessible labels, `aria-pressed`, keyboard operation, narrow-screen wrapping, and an honest no-results message that distinguishes filtering from missing analysis.**
- [ ] **Step 7: Run focused tests, all dashboard tests, embedded JavaScript syntax validation, and `git diff --check`; expect PASS.**
- [ ] **Step 8: Commit `feat: add endpoint debugging filters`.**

### Task 1: Canonical Status And Evidence Types

**Files:**
- Create: `internal/scan/evidence.go`
- Create: `internal/scan/evidence_test.go`

**Interfaces:**
- Produces: `Confidence`, `Resolution`, `Severity`, `Coverage`, `EvidenceRecord`, `EvidenceLocation`, `StableEvidenceID(EvidenceRecord) string`, and validation methods.

- [ ] **Step 1: Write failing enum and stable-ID tests**

```go
func TestStableEvidenceIDIgnoresAbsoluteRoot(t *testing.T) {
 a := EvidenceRecord{Project:"app", File:"src/a.go", Start:EvidenceLocation{Line:7}, Analyzer:"go", Method:"syntax", Reason:"call expression", SourceHash:"abc"}
 b := a
 if StableEvidenceID(a) != StableEvidenceID(b) { t.Fatal("evidence ID changed") }
 if !strings.HasPrefix(StableEvidenceID(a), "evidence:") { t.Fatal("missing stable prefix") }
}
func TestEvidenceEnumsRejectUnknownValues(t *testing.T) {
 if err := Coverage("BROKEN").Validate(); err == nil { t.Fatal("invalid coverage accepted") }
}
```

- [ ] **Step 2: Run `GOCACHE=/private/tmp/goregraph-gocache go test ./internal/scan -run 'TestStableEvidence|TestEvidenceEnums' -count=1` and confirm compile failure.**
- [ ] **Step 3: Implement string enums, records, SHA-256 ID over canonical project/file/location/analyzer/adapter/method/reason/source-hash fields, and `Validate` switches.**
- [ ] **Step 4: Run focused tests and `gofmt -w internal/scan/evidence.go internal/scan/evidence_test.go`; expect PASS.**
- [ ] **Step 5: Commit `feat: add canonical evidence model`.**

### Task 2: Focused Capability And Coverage Model

**Files:**
- Create: `internal/scan/capabilities.go`
- Create: `internal/scan/capabilities_test.go`
- Modify: `internal/scan/analyzers.go`

**Interfaces:**
- Consumes: `Coverage`.
- Produces: `CapabilityID`, `CapabilityRecord`, `CoverageRecord`, `BuildCapabilityInventory([]FileRecord, WorkspaceIndex) []CapabilityRecord`, `BuildCoverage([]FileRecord, []CapabilityRecord) CoverageRecord`.

- [ ] **Step 1: Write failing tests proving TypeScript can report complete symbols but partial persistence, Rust unavailable routes, and unknown languages index-only rather than complete.**
- [ ] **Step 2: Run `go test ./internal/scan -run 'TestBuildCapability|TestBuildCoverage' -count=1`; expect undefined symbols.**
- [ ] **Step 3: Implement records with `id`, `language`, `adapter`, `coverage`, `reason`, `files_seen`, `evidence_ids`, and optional `failure`; derive records from the existing analyzer table without changing `AnalyzerRecord`.**
- [ ] **Step 4: Sort by language, adapter, capability ID; run focused tests and `gofmt`; expect PASS.**
- [ ] **Step 5: Commit `feat: add capability coverage model`.**

### Task 3: Generate Evidence, Capabilities, Coverage, And Markdown

**Files:**
- Modify: `internal/scan/scan.go`
- Modify: `internal/scan/scan_test.go`
- Create: `internal/scan/evidence_build.go`
- Create: `internal/scan/evidence_build_test.go`
- Create: `internal/scan/coverage_report.go`

**Interfaces:**
- Produces: `BuildEvidence(Index, []RichRelationRecord, CallGraphRecord, []CodeRouteRecord, []CodeFlowRecord) []EvidenceRecord`, `RenderCoverageReport(CoverageRecord) string`.

- [ ] **Step 1: Add a scan fixture test requiring valid deterministic `evidence.json`, `capabilities.json`, `coverage.json`, and `coverage.md`, plus manifest entries.**
- [ ] **Step 2: Run `go test ./internal/scan -run TestRunWritesEvidenceAndCoverage -count=1`; expect missing files.**
- [ ] **Step 3: Build evidence from source-backed relations, calls, routes, and flows; deduplicate by stable ID and sort. Do not copy source excerpts.**
- [ ] **Step 4: Add the four files to `GeneratedFiles`, JSON writes, report writes, README-style coverage explanations, and deterministic golden assertions.**
- [ ] **Step 5: Run focused and full scan tests; expect PASS.**
- [ ] **Step 6: Commit `feat: generate evidence and coverage outputs`.**

### Task 4: Add Evidence References Without Breaking Legacy Fields

**Files:**
- Modify: `internal/scan/types.go`
- Modify: `internal/scan/rich_build.go`
- Modify: `internal/scan/code_flows.go`
- Modify: `internal/scan/contract_matches.go`
- Create: `internal/scan/evidence_refs_test.go`

**Interfaces:**
- Adds only `EvidenceIDs []string \`json:"evidence_ids,omitempty"\`` and normalized optional dimensions to public edge/flow/match records.

- [ ] **Step 1: Write failing tests requiring every publicly emitted source-backed relation, call, route-flow edge, and contract match to reference an existing evidence ID.**
- [ ] **Step 2: Run focused tests; expect missing references.**
- [ ] **Step 3: Populate references through one shared evidence-key helper; retain existing `confidence`, `confidence_score`, `issue`, and `reason` values unchanged.**
- [ ] **Step 4: Add JSON compatibility test that unmarshals a pre-0.9.2 record and asserts zero-value new fields.**
- [ ] **Step 5: Run `go test ./internal/scan -count=1`; expect PASS.**
- [ ] **Step 6: Commit `feat: link generated facts to evidence`.**

### Task 5: Doctor Validation

**Files:**
- Modify: `internal/doctor/doctor.go`
- Modify: `internal/doctor/doctor_test.go`

**Interfaces:**
- Validates the three new JSON files, enum values, unique stable IDs, evidence-reference integrity, and explicit capability failure messages.

- [ ] **Step 1: Add failing Doctor tests for malformed enum, duplicate evidence ID, dangling evidence reference, and a valid 0.9.2 output.**
- [ ] **Step 2: Run `go test ./internal/doctor -count=1`; expect failures.**
- [ ] **Step 3: Implement read-only validation with actionable `goregraph scan <path>` guidance; capability `FAILED` is a warning when structurally valid, malformed data is a failure.**
- [ ] **Step 4: Run Doctor and CLI tests; expect PASS.**
- [ ] **Step 5: Commit `feat: validate evidence and coverage outputs`.**

### Task 6: Coverage Dashboard View

**Files:**
- Modify: `internal/scan/workspace_reconcile.go`
- Modify: `internal/scan/workspace_dashboard.go`
- Modify: `internal/scan/workspace_dashboard_template.go`
- Modify: `internal/scan/workspace_dashboard_script.go`
- Modify: `internal/scan/workspace_dashboard_styles.go`
- Modify: `internal/scan/workspace_dashboard_test.go`

**Interfaces:**
- Adds top-level view ID `coverage`; consumes aggregated project capability records; preserves all existing per-view viewport behavior.

- [ ] **Step 1: Write failing dashboard tests requiring `Coverage` after `Diagnostics`, a language-by-capability matrix, COMPLETE/PARTIAL/UNAVAILABLE/FAILED legend, honest empty states, and evidence-backed detail rows.**
- [ ] **Step 2: Run focused dashboard tests; expect missing view.**
- [ ] **Step 3: Pass coverage payload into the renderer and implement the matrix as a dense workbench table, not decorative cards; cells expose status, reason, files seen, and adapter.**
- [ ] **Step 4: Add keyboard focus, accessible names, horizontal table scrolling on narrow screens, and no automatic viewport movement.**
- [ ] **Step 5: Run dashboard tests and embedded JavaScript syntax check; expect PASS.**
- [ ] **Step 6: Commit `feat: add capability coverage dashboard`.**

### Task 7: Documentation, Version, And Acceptance

**Files:**
- Modify: `README.md`
- Modify: `COMMANDS.md`
- Modify: `docs/RELEASE.md`
- Modify: `internal/version/version.go`
- Modify: relevant version tests

- [ ] **Step 1: Add failing version and documentation assertions for `0.9.2`, evidence semantics, coverage meanings, static-analysis limits, and rescan requirements.**
- [ ] **Step 2: Update version and documentation without claiming full language parity.**
- [ ] **Step 3: Run `gofmt`, `go test ./... -count=1`, `go vet ./...`, JavaScript syntax check, and `git diff --check`; expect PASS.**
- [ ] **Step 4: Install with `go install ./cmd/goregraph`, update the existing local default target, and verify `command -v goregraph` plus `goregraph version` reports 0.9.2.**
- [ ] **Step 5: In `~/projects/weka`, run clean dry-run, reviewed `clean --execute`, `scan-all`, `status`, representative Doctor checks, and deterministic repeat hashes.**
- [ ] **Step 6: Verify Architecture, Endpoints, Diagnostics, and Coverage output without changing selection/viewport rules; inspect desktop and narrow layout only if browser use is authorized.**
- [ ] **Step 7: Commit `docs: prepare GoreGraph 0.9.2`. Do not push, tag, or publish.**

## Completion Gate

- [ ] Existing Schema 1 files and meanings remain compatible.
- [ ] Stable evidence IDs and references are deterministic and Doctor-validated.
- [ ] Confidence, resolution, severity, and coverage are separate dimensions.
- [ ] Capability absence is never reported as absence of source behavior.
- [ ] Coverage view explains language/framework analysis honestly.
- [ ] 0.9.2 is installed locally and the clean WEKA rescan passes.
- [ ] Gate B interfaces are ready for 0.9.3 Query/MCP and 0.9.4 directed traces.
