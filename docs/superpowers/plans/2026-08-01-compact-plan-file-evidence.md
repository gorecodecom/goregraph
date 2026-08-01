# Compact Plan-File Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bounded metadata-only test and test-pattern paths to exact missing-transition change plans without weakening causal source evidence.

**Architecture:** Select up to four exact indexed test identities after metadata compilation, pay for their final bytes by dropping only repetitive file-selection reasons, and render them separately from source permissions. Keep all existing source budgets and read rules unchanged.

**Tech Stack:** Go 1.26, Go standard library, Schema 3 Context Packs, existing `internal/agent` selectors and `internal/query` renderer

## Global Constraints

- Keep the 4,000-token Context Pack limit.
- Keep the 12-source-file and 12-source-section limits.
- Keep the three bounded source-omission limit.
- Emit at most four metadata-only plan files.
- Accept only exact-confidence indexed test-source facts with normalized relative paths.
- Do not add dependencies, prompt exceptions, retries, fallbacks, private identifiers, or invented future paths.

---

### Task 1: Define the plan-file contract with failing tests

**Files:**
- Create: `internal/agent/context_plan_files_test.go`
- Modify: `internal/agent/context_change_analysis_test.go`

**Interfaces:**
- Produces: `ContextPlanFile{Project string, Path string, Use string}`.
- Produces: `ContextPack.PlanFiles []ContextPlanFile` serialized as `plan_files`.

- [ ] **Step 1: Write the unit regression**

Create a generic pack and index containing a caller `InventoryClientMock`/`InventoryClientRetryableTest` pair, a provider `JobManagementControllerTest`, and a provider `JobServiceTest`. Require four sorted `ContextPlanFile` entries and reject an unmatched mock, a production source, a partial-confidence test, and a test from an unrelated project.

- [ ] **Step 2: Write the integration regression**

Extend a dedicated copy of the runtime-shaped release fixture with the paired caller pattern. Require `BuildContext` at 4,000 tokens and 12 files to keep all previously asserted production evidence and publish the caller pair without increasing `contextSourceFileCount`, source sections, or omissions.

- [ ] **Step 3: Verify RED**

Run:

```bash
GOCACHE=/private/tmp/goregraph-go-cache go test ./internal/agent -run 'TestContextPlanFiles|TestBuildContextKeepsCompactPlanFileEvidence' -count=1
```

Expected: compilation fails because `ContextPlanFile`, `PlanFiles`, and `contextPlanFiles` do not exist.

### Task 2: Select exact bounded plan-file identities

**Files:**
- Create: `internal/agent/context_plan_files.go`
- Modify: `internal/agent/context.go`
- Modify: `internal/agent/context_rank.go`
- Test: `internal/agent/context_plan_files_test.go`

**Interfaces:**
- Produces: `contextPlanFiles(ContextPack, scan.AgentContextIndexRecord) []ContextPlanFile`.
- Produces: `maximumContextPlanFiles = 4`.
- Consumes: `contextQueryRequestsExactEvidenceInventory`, `contextQueryPlansMissingTransition`, `contextQueryRequestsTests`, normalized projects/paths, and represented source identities.

- [ ] **Step 1: Implement eligibility and path deduplication**

Accept exact-confidence symbol/test facts below `src/test`, reject configuration resources, and exclude paths represented by `files`, `source_sections`, or `source_omissions`. Derive the entrypoint project and requested non-entrypoint projects exclusively from the pack.

- [ ] **Step 2: Implement provider test selection**

Select at most one internal controller test and one service test from requested provider projects. Score with existing query/domain helpers, then sort by score, project, path, line, and fact ID.

- [ ] **Step 3: Implement paired caller pattern selection**

Normalize a caller test stem by removing only `Mock` and `RetryableTest`. Emit `mock_pattern` and `retry_pattern` only when both exact test-source facts share a non-empty stem and project. Choose one deterministic best pair.

- [ ] **Step 4: Integrate finalization, cloning, and monotonic budgeting**

Add `PlanFiles` to `ContextPack`, clone it in `cloneContextPack`, and clear and recompute it in `finalizeContextSourceDecision`. When entries exist, clear repetitive `files.reason` text while retaining paths, ranges, roles, confidence, and source evidence. Exclude only the plan-file delta and matching reason compaction from `contextFinalDecisionBudgetReserve`; keep existing uncertainty reserves and the final hard-budget reduction loop. Do not add plan files to `contextSourceFileCount`, retry anchors, source coverage, or source omissions.

- [ ] **Step 5: Verify GREEN**

Run:

```bash
GOCACHE=/private/tmp/goregraph-go-cache go test ./internal/agent -run 'TestContextPlanFiles|TestBuildContextKeepsCompactPlanFileEvidence|TestContextMissingTransitionOrderingGapRequiresCrossProjectPlan|TestBuildContextBudgetsFinalDecisionMetadata' -count=1
```

Expected: PASS.

### Task 3: Render and document metadata-only semantics

**Files:**
- Modify: `internal/query/context.go`
- Modify: `internal/query/context_test.go`
- Modify: `scripts/sync-docs/main.go`
- Generated by sync: `README.md`
- Generated by sync: `COMMANDS.md`
- Modify: `docs/OUTPUTS.md`
- Modify: `SCHEMA.md`

**Interfaces:**
- Consumes: `ContextPack.PlanFiles`.
- Produces: Markdown section `## Plan file identities (metadata only; do not read)`.

- [ ] **Step 1: Write the failing renderer test**

Require sorted entries in `project/path — use` form, one section only, no blank paths, and no source-read wording.

- [ ] **Step 2: Render the compact section**

Append it after `Files to inspect` and before source sections. Sanitize project, path, and use through existing inline/code-reference helpers.

- [ ] **Step 3: Update generated instructions and reference docs**

State that `plan_files` may be named as existing identities or patterns but never read unless the same range appears in `source_omissions`; preserve every existing read prohibition and hard source limit.

- [ ] **Step 4: Synchronize docs and verify**

Run:

```bash
GOCACHE=/private/tmp/goregraph-go-cache go run ./scripts/sync-docs --write
GOCACHE=/private/tmp/goregraph-go-cache go test ./internal/query ./scripts/sync-docs -count=1
GOCACHE=/private/tmp/goregraph-go-cache go run ./scripts/sync-docs --check
```

Expected: PASS with no documentation drift.

### Task 4: Run complete local and historical acceptance

**Files:**
- Verify: entire repository
- Verify: historical G1 workspace

**Interfaces:**
- Consumes: installed committed candidate and unchanged G1 source snapshot.
- Produces: deterministic bounded Context Pack and fresh workspace projections.

- [ ] **Step 1: Run all repository gates**

```bash
GOCACHE=/private/tmp/goregraph-go-cache go test ./... -count=1
GOCACHE=/private/tmp/goregraph-go-cache go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/calibrate-agent-context-tokens_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
GOCACHE=/private/tmp/goregraph-go-cache go run ./scripts/sync-docs --check
git diff --check
```

- [ ] **Step 2: Commit, push, and install exact metadata**

Create separate English commits for design and implementation, push `fix/release-benchmark-metrics`, build with exact commit/UTC build ldflags, and verify `goregraph version` plus SHA-256.

- [ ] **Step 3: Clean and scan G1**

Preview and execute `workspace clean`, run `workspace build all` with explicit `--workspace`, require three indexed projects, all Doctor checks, and unchanged source digest `549e6f2af8a030fb62353b9a4a8f2f7954861a731b656578089b1f8952e6148f`.

- [ ] **Step 4: Verify the real Context Pack**

Require two byte-identical packs, `estimated_tokens <= 4000`, unchanged source-file/section/omission ceilings, the `cross_service_ordering` uncertainty, and the qualifying plan-file identities without private selection rules.

### Task 5: Run the prospective release matrix

**Files:**
- Write evidence only below a new `/private/tmp/goregraph-release-3x3-<commit>-20260801-m6` directory.

**Interfaces:**
- Consumes: frozen prompt, workspace digest, Codex model/reasoning/sandbox settings, and installed exact candidate.
- Produces: six retained JSONL transcripts, metrics, skill-read evidence, manual reviews, and matrix classification.

- [ ] **Step 1: Run three baseline and three assisted executions**

Use identical prompt, `gpt-5.6-sol`, high reasoning, read-only sandbox, approval `never`, ephemeral mode, ignored user/rule configuration, and the same two disabled Superpowers paths. Fail fast on infrastructure, source mutation, unauthorized reads, repeated Context Packs, or external skill reads.

- [ ] **Step 2: Apply automatic gates**

Require exactly one full Context Pack per assisted run, no repeated packs or included-source rereads, only bounded omission reads, zero external skill reads, assisted effective-token median at or below both the matched baseline threshold and absolute cap, and unchanged source identity.

- [ ] **Step 3: Review items 1-12**

Require assisted median quality at least baseline median. Item 11 passes only when the answer distinguishes exact existing production/test targets, existing plan-file patterns, and unknown future filenames.

- [ ] **Step 4: Use the four-run reserve only after classification**

If the matrix fails, retain all evidence, classify one root cause, make one narrow TDD correction, reinstall/rescan, and spend reserve runs only on the corrected candidate. If the matrix passes, keep the reserve unused and report release readiness without tagging, merging, or releasing.
