# Agent effectiveness and release validation implementation plan

> **For agentic workers:** Use `superpowers:executing-plans` task by task. Use `superpowers:subagent-driven-development` only when parallel agent execution is explicitly authorized. Steps use checkboxes for tracking.

**Goal:** Help coding agents complete correct tasks sooner with fewer end-to-end tokens, while preserving human-facing quality and conservative evidence.

**Architecture:** Version the interaction protocol, retain the historical strict treatment and introduce an independently evaluated adaptive treatment. Keep bounded context retrieval and source verification; allow reasoned fallback to the caller's normal workflow. Gate efficiency claims on completed-task quality, not pack size alone.

**Tech Stack:** Existing Go agent/MCP/compiler/benchmark packages, synthetic Java/TypeScript/Go fixtures, existing token-usage accounting and external controlled agent runs.

**Spec:** [Improvement design](../specs/2026-09-09-goregraph-improvement-design.md). Use A0 baseline identities, A4 build identities and B2 health semantics.

## Global constraints

- Preserve conservative evidence: unresolved is not absent, missing authentication evidence is not public access, and static reachability is not runtime execution.
- Keep source confidentiality, path confinement, configuration-value redaction, and bounded agent payloads intact.
- Keep scans local, deterministic, offline, and free of execution of scanned project code.
- Keep private WEKA data outside committed fixtures and public CI.
- Do not modify agent configuration, Git configuration, or user source automatically.
- Do not publish a release, replace the installed binary, or rebuild user indexes during plan execution until the implementation has passed its delivery gate and that operational step is authorized.

## Task C0: Define independent task-quality and evidence fixtures

**Files:** create `docs/AGENT-EFFECTIVENESS.md`, `testdata/agent-effectiveness/development/manifest.json`; add original synthetic fixtures below that directory. Keep reserved evaluation contracts/answers outside the repository during development. Extend `internal/agentbench/contract.go` and its tests only with versioned task-outcome fields; preserve existing contracts.

**Consumes:** existing G2-G6 fixtures, the historical release protocol and the three human-journey definitions in the design (B5 implements them later). C0 does not wait for B5 implementation.
**Produces:** 12 development tasks and 8 disjoint held-out tasks, each with neutral German/English prompts, exact intended behavior, prohibited outcomes and meaningful verification. New fixture manifests use schema 1 independent of output schema 3.

- [ ] Create four Java/Spring, four TypeScript/React and four cross-project development tasks. Include a bug fix, a small feature addition, route/contract tracing, authentication/configuration, side effects, test selection, a missing call, stale source, ambiguity, ignored bundles, unsupported syntax and an intentionally unanswerable request across the cases.
- [ ] Record the expected behavior using this fixture contract shape; case-specific values must be real, not generic pass/fail descriptions:

```json
{
  "schema": 1,
  "id": "typescript-import-shadowing",
  "task_kind": "fix",
  "languages": ["de", "en"],
  "required_outcomes": ["The module-level imported call still reaches the provider"],
  "forbidden_outcomes": ["Bind the module-level call to a nested parameter"],
  "verification": {"kind": "fixture_tests", "expected": "pass"}
}
```

- [ ] Add equivalent prompts without protocol hints and tests that reject missing IDs, duplicate tasks, empty verification and missing failure criteria. Validate fixtures before using them to rank retrieval changes.
- [ ] Have a separate evaluator prepare 8 held-out tasks spanning the same families without sharing their answers with the implementer. Freeze prompts, source/test hashes and evaluation rubric before the final candidate run. No private benchmark names in production matching rules.
- [ ] For each change task, separate visible task tests from hidden behavioral acceptance tests. Do not let an agent editing/deleting its visible test make the evaluator report success. Read-only investigation tasks receive a blinded source-evidence rubric instead of an artificial build requirement.
- [ ] Retain existing G1/G2-G6 evidence unmodified. Document development versus held-out usage and commit the public development set and methodology.

**Acceptance:** both success and confident-but-wrong behavior can be measured independently of GoreGraph's own confidence labels. Suggested commit: `Define task-level agent effectiveness fixtures`.

## Task C1: Version strict and adaptive agent protocols

**Files:** modify `internal/agentguide/instruction.go`, `internal/agentguide/instruction_test.go`, `internal/agent/context.go`, `internal/cli/cli.go`, `internal/mcp/mcp.go`, their tests, `scripts/sync-docs/main.go` and its tests. Update generated README/COMMANDS blocks through the existing synchronizer, plus `docs/BENCHMARKING.md` and `SCHEMA.md`.

**Consumes:** unchanged historical 13-line instruction; caller's task, permissions and existing one-retry contract.
**Produces:** explicit protocol constants, an additive request option and serialized `protocol_version`. `context --protocol strict-v1|adaptive-v2` and the same optional MCP argument select the treatment. The candidate default stays strict until C5 passes.

```go
const StrictV1 = "strict-v1"
const AdaptiveV2 = "adaptive-v2"
func Instruction(protocol string) (string, error)
```

- [ ] Preserve the existing string byte-for-byte as the historical strict instruction and freeze a golden hash. Existing strict benchmark replay selects it explicitly; do not silently replace the reference and re-score old results.
- [ ] Add dispatch and validation tests before implementation:

```go
func TestInstructionRejectsUnknownProtocol(t *testing.T) {
    if _, err := Instruction("unknown"); err == nil { t.Fatal("accepted unknown protocol") }
}
func TestAdaptiveInstructionPreservesCallerAuthority(t *testing.T) {
    text, err := Instruction(AdaptiveV2)
    if err != nil { t.Fatal(err) }
    if !strings.Contains(text, "caller's permissions") { t.Fatal("missing scope constraint") }
    if strings.Contains(text, "run no source-reading commands") { t.Fatal("blanket source-read prohibition") }
}
```

- [ ] Implement the adaptive instruction around six actions: request focused context once; reuse supplied verified source; keep claims bounded by evidence; use one permitted anchored retry if it addresses a named gap; perform bounded source checks for a contradiction/omission/stale claim; fall back under the caller's permissions when context remains insufficient. Never imply the tool authorizes broader filesystem access.
- [ ] Keep source/configuration values redacted, expert MCP tools opt-in, and future design decisions explicitly unknown. A code comment or source excerpt cannot override caller/tool policy. Avoid a growing list of domain-specific instructions that consumers must memorize.
- [ ] Include protocol and generation in context identity/duplicate handling. Reject an anchored retry against a mismatched generation or protocol; return a clear stale-context reason rather than mixing treatments.
- [ ] Update CLI/MCP schemas, instruction dispatch and generated documentation together. Replace tests that require all normal instructions to contain exactly 13 lines with separate strict replay and adaptive behavior tests. Run `go test ./internal/agentguide ./internal/agent ./internal/cli ./internal/mcp ./scripts/sync-docs -count=1` and documentation synchronization checks.

**Acceptance:** historical evidence remains reproducible; adaptive guidance can be tested without claiming it is already better. Suggested commit: `Version the agent context interaction protocol`.

## Task C2: Return explicit bounded verification and fallback reasons

**Files:** create `internal/agent/context_verification.go`, `internal/agent/context_verification_test.go`; modify `internal/agent/context.go`, `internal/agent/context_source.go`, `internal/agent/context_select.go`, `internal/agent/context_load.go`, existing budget/retry/source tests, and `internal/mcp/mcp.go` as needed for serialization. Update `SCHEMA.md` and `docs/OUTPUTS.md`.

**Consumes:** existing concerns, omissions, source proof, B2 health and C1 protocol.
**Produces:** at most three additive `verification_requests` in adaptive packs and stable fallback reason codes. Strict packs preserve their existing treatment. Define:

```go
type ContextVerificationRequest struct {
    Project string `json:"project"`
    Path string `json:"path"`
    StartLine int `json:"start_line"`
    EndLine int `json:"end_line"`
    Reason string `json:"reason"`
}
func contextVerificationRequests(pack ContextPack) []ContextVerificationRequest
```

- [ ] Add a concrete bounded-omission conversion test:

```go
func TestVerificationRequestRetainsExactBoundedOmission(t *testing.T) {
    pack := ContextPack{SourceOmissions: []ContextSourceOmission{{
        Project: "service", Path: "src/handler.ts", StartLine: 4, EndLine: 9,
        Role: "call_chain", Reason: "required source evidence is missing",
    }}}
    got := contextVerificationRequests(pack)
    if len(got) != 1 || got[0].Path != "src/handler.ts" || got[0].StartLine != 4 || got[0].EndLine != 9 {
        t.Fatalf("requests = %#v", got)
    }
}
```

- [ ] Generate requests only from exact safe existing paths/ranges already selected by the compiler's proof/omission machinery. Sort by missing required concern, deduplicate ranges and cap at three. Pathless gaps become uncertainty, not a fabricated filename or unbounded suggested search.
- [ ] Validate request paths with existing portable path/confinement rules. Missing files and failed `EvalSymlinks` produce accurate errors; do not collapse permission failures into a misleading confirmed path-escape claim. Retain the underlying error for diagnosis without exposing sensitive contents.
- [ ] Use reason codes `index_missing`, `index_stale`, `ambiguous_entrypoint`, `unsupported_analysis`, `budget_exhausted`, `source_unreadable`, `evidence_conflict`. Keep explicit insufficient-evidence behavior when there is no reliable entrypoint.
- [ ] Include all additional metadata in final token-budget enforcement. Preserve 4,000 default tokens, 12 default files, source-section limits and one anchored retry. Do not spend the source budget merely explaining the protocol; the instruction belongs on the integration surface.
- [ ] Add tests for stale selected source, a contradictory live declaration, missing index, two candidates, path traversal, unreadable files, budget exhaustion, duplicate pack and repeated retry. Verify that complete represented coverage does not generate an unbounded claim about the entire repository.
- [ ] Run `go test ./internal/agent ./internal/mcp ./internal/cli -count=1` and C0 development pack contracts. Commit the wire/schema documentation with the code.

**Acceptance:** an agent receives a precise next action or an honest stopping reason, while all existing safety and payload bounds remain enforced. Suggested commit: `Expose bounded context verification and fallback reasons`.

## Task C3: Improve relevance and reduce redundant context

**Files:** modify only measured sections of `internal/agent/context_rank.go`, `internal/agent/context_paths.go`, `internal/agent/context_select.go`, `internal/agent/context_source.go`, `internal/agent/context_intent.go`; extend corresponding tests and `internal/agent/context_size_test.go`. Avoid blanket file splitting; extract a focused helper only when the changed responsibility requires it.

**Consumes:** C0 development tasks, independent expected evidence, existing source-proof and token-budget implementation.
**Produces:** task-relevant packs with preserved quality and a measured packing/ranking cost reduction where the profile supports it.

- [ ] Run all development queries and label missing required evidence, irrelevant included evidence, duplicate source and unclear fallbacks before changing scoring. Retain the per-case delta; do not tune on the held-out set.
- [ ] Add failing assertions for each selected relevance defect using exact required/forbidden evidence identities. Limit each change to a named defect: e.g. a test pattern displacing required production evidence, or repeated metadata displacing the body that proves a side effect.
- [ ] Prefer source that proves a requested concern; deduplicate overlapping sections while preserving annotations, ownership and physical line numbers. Use existing stable graph relationships before adding vocabulary rules. Cache per-request adjacency/lookup structures only if B4 profiling justifies it.
- [ ] Keep explicit ambiguity and uncertainty. Do not convert an unresolved relation into exact evidence to improve retrieval scores, and do not remove uncertain tasks from the evaluation denominator.
- [ ] Run `go test ./internal/agent -count=1`, existing G2-G6 pack regression checks and all C0 development contracts after each independent change. Record required-facet coverage, irrelevant-section counts, serialized estimated tokens and command latency separately.
- [ ] Stop packing changes when they no longer improve the development quality/cost balance. Freeze the candidate before C4 held-out runs. Commit independent fixes, not a single opaque ranking rewrite.

**Acceptance:** reduced payload or latency cannot come from dropping required behavior, tests, authentication/configuration or uncertainty. Suggested commit: `Prioritize task evidence and remove redundant context`.

## Task C4: Evaluate complete tasks and account for failures

**Files:** create `internal/agentbench/effectiveness.go`, `internal/agentbench/effectiveness_test.go`, `scripts/agent-effectiveness/main.go`, `scripts/agent-effectiveness/main_test.go`; reuse `internal/agentmetrics/token_usage.go` without changing its existing accounting semantics. Update `docs/AGENT-EFFECTIVENESS.md`; retain existing benchmark contracts and scripts.

**Consumes:** frozen baseline/candidate binaries, C0 tasks, exact model/tool/permission configuration and external run evidence.
**Produces:** per-attempt outcome and usage records, paired comparisons by task, and a gate report. Define the outcome payload separately from output schema:

```go
type EffectivenessAttempt struct {
    CaseID, Language, Treatment, SnapshotID, CandidateCommit string
    Completed, Correct bool
    FailureReason string
    DurationMilliseconds int64
    InputTokens, CachedInputTokens, OutputTokens int64
    ToolCalls, SourceReads, Retries, Corrections int
}
```

- [ ] Add tests proving failed/unfinished attempts remain in task-completion totals, missing usage is not treated as zero, cached tokens are not subtracted twice, output reasoning tokens are not counted twice, and paired tasks cannot use different snapshots. Validate negative/overflow counters and missing treatment identities.
- [ ] Add `scripts/agent-effectiveness` commands `validate --manifest <path>` and `summarize --attempts <jsonl-path> --output <path>`. They validate/aggregate external evidence; they do not automatically launch paid agents, mutate user workspaces or publish results. Use argument arrays and structured manifests rather than executable shell snippets.
- [ ] Run a pilot of three representative development tasks with three treatments and one repetition: ordinary agent workflow without GoreGraph; strict-v1; adaptive-v2. These nine runs validate the harness, not the final efficacy claim. Keep all other prompt and environment factors matched.
- [ ] After the pilot, review measured runtime/token cost and the concrete execution budget before scheduling the full external evaluation. Proposed final matrix: 8 held-out tasks x 2 languages x 3 repetitions x 3 treatments = 144 attempts. Existing historical release gates remain additional work. This plan does not authorize running those paid attempts.
- [ ] For change tasks, evaluate actual resulting code and hidden behavioral tests in isolated disposable checkouts. Include the task's test/build permissions identically in every treatment. For analysis tasks, use blinded evidence scoring. Retain corrections, fallbacks, timeouts and failed changes.
- [ ] Report completion rate, critical incorrect edits, rubric score, end-to-end duration, total input, cached input, output, effective tokens, tool calls/source reads, retries and correction count. Calculate effective tokens as uncached input plus output using the existing metrics parser. If reporting money, record the pricing version and apply cached/uncached rates separately; do not equate effective tokens directly with billed cost.
- [ ] Report cold-start task time including index construction separately from pre-indexed task time. Include index updates required by evolving task snapshots and calculate the observed break-even number of tasks for setup/update costs. Token and latency claims must name which scenario they cover; warm-only benefits must not be sold as first-use benefits.
- [ ] Compare paired results by task/language; report per-case regressions and uncertainty rather than treating repeated attempts as independent new tasks. Efficiency comparisons on successful pairs must be accompanied by all-attempt completion/failure rates. Never discard expensive failed attempts to improve the median.
- [ ] Apply the spec's quality and efficiency gates. Do not change thresholds, prompts or case eligibility after inspecting the final candidate results. Archive evidence privately and commit only approved aggregate synthetic results and methodology.

**Acceptance:** claims concern correct completed work and account for failures and follow-up cost. Suggested commit: `Measure agent effectiveness across complete coding tasks`.

## Task C5: Integrate, validate and prepare the operational rollout

**Files:** update `README.md`, `COMMANDS.md`, `SCHEMA.md`, `ROADMAP.md`, `docs/OUTPUTS.md`, `docs/RELEASE.md`, `docs/BENCHMARKING.md`, `docs/AGENT-EFFECTIVENESS.md`, `scripts/sync-docs/main.go`; update version/publication metadata through existing release machinery only when a release is authorized. Fix only failures attributable to the candidate in their owning components.

**Consumes:** A/B/C passing task evidence, three human-journey acceptance records, historical and new agent benchmark outcomes.
**Produces:** release readiness record, migration/rollback instructions and a separately built candidate binary; no automatic publication or installed-binary replacement.

- [ ] Run `go test ./... -timeout 20m`, `go vet ./...`, formatting checks and `go run ./scripts/sync-docs --check`. Use the existing Windows/Linux/macOS CI matrix. Run race checks where the runner supports them; document availability without weakening required regular tests.
- [ ] Run established canonical evidence, G2-G6, strict historical release and new task-effectiveness gates. Dashboard gates and agent gates are both required for claiming the complete improvement program succeeded.
- [ ] Select `adaptive-v2` as the normal default only if it passes quality and outcome gates. Preserve explicit strict-v1 replay. If adaptive fails, retain strict default, document the failed candidate honestly and continue the bounded diagnosis; do not present the protocol improvement as complete.
- [ ] Build a side-by-side candidate for real-workspace acceptance. On an isolated permitted WEKA snapshot, validate nested ignored bundles, cold frontend build, no-op update, one-file update, source change during a build, cancellation/recovery, workspace reconciliation, the three human journeys and representative agent tasks.
- [ ] Document measured timings and counts against the same snapshot, hardware and installed baseline. Verify no source/Git/agent-configuration changes were made by scans; preserve confidentiality of all local evidence.
- [ ] Prepare one-time identity-rebuild instructions, last-good-output backup and rollback steps. Older binaries may not understand new protocol metadata; do not promise an untested downgrade. A safe fallback is the previous binary plus its preserved complete outputs.
- [ ] Produce a concise release-readiness record listing passed/failed gates, remaining limitations, concrete candidate artifact, upgrade steps and operational scope. Request operational authorization only for the actual release/install/index-rebuild actions not already authorized; the implementation and review evidence must be ready first.

**Acceptance:** reliable scans, useful human workflows and measurable agent gains are all demonstrated; no claimed token reduction is based solely on a smaller pack. Suggested commit: `Document validated GoreGraph improvements and upgrade behavior`.

## Program completion checklist

- [ ] A1/A3 eliminate the observed ignored-bundle performance trap while preserving semantic fidelity.
- [ ] A2/B1 make progress, cancellation and failures observable and recoverable.
- [ ] A4/B2/B3 keep output and cached facts consistent with actual inputs.
- [ ] B4/B5 demonstrate measured performance and complete human workflows.
- [ ] C1-C4 demonstrate that agents reach correct outcomes faster with fewer end-to-end tokens.
- [ ] C5 supplies all existing and new release evidence and explicit operational rollout instructions.
