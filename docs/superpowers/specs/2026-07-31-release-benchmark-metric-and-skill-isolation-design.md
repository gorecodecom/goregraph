# Deterministic Release Benchmark Metrics and Skill Isolation

Status: Approved design

## Context

The six-run G1 release matrix for candidate `c8d0afc629f5fa95bccdd9159fa995e2eb1ea00c` completed with three baseline and three GoreGraph-assisted runs. The existing release harness reported a formal failure because it compared raw total-token medians: 2,551,495 baseline tokens versus 147,212 assisted tokens, while the absolute assisted cap was 116,560.

The immutable logs also contain cached-input counters. A prospective offline calculation using uncached input plus output produces medians of 164,295 baseline effective tokens and 39,180 assisted effective tokens, a 23.85% assisted share and 76.15% savings. This is diagnostic evidence for changing future measurement only. It does not rescore the completed matrix or change its failed verdict.

The same logs show uncontrolled workflow-skill reads. Baseline runs contain 4, 2, and 0 Superpowers reads; assisted runs contain 2, 5, and 4, all before the first GoreGraph context request. `--ignore-user-config` did not isolate plugin-provided skills in the tested Codex environment. Despite that contamination, the assisted runs showed materially better structural behavior: median tool calls fell from 28 to 8 and median source reads from 20 to 3. Every assisted run used one full context pack with three bounded omissions and no broad project-source reads, unauthorized project-source reads, or rereads.

## Goals

1. Measure the non-cached token cost that a benchmark run actually contributes while retaining every raw counter needed for diagnosis.
2. Detect external skill reads without coupling the analyzer to Superpowers or any other plugin name.
3. Make controlled release matrices fail fast when either variant is contaminated, preserving the failed run as evidence and avoiding unnecessary external runs.
4. Keep normal GoreGraph usage compatible with Brainstorming, TDD, review, and other task-scoped skills.
5. Keep benchmark evidence reproducible across macOS, Linux, and Windows path formats.

## Non-goals

- Change context ranking, budgeting, rendering, or omission behavior.
- Add G1-, repository-, source-file-, or service-specific benchmark logic.
- Suppress skills through benchmark prompt wording.
- Mutate global Codex or plugin configuration.
- Claim that `--ignore-user-config` provides skill isolation.
- Rescore or overwrite existing benchmark results.
- Publish, tag, or release a GoreGraph version.
- Start another external benchmark without fresh, explicit authorization.

## Chosen architecture

### Shared token accounting

A new `internal/agentmetrics.TokenUsage` model is the single source of truth for analyzer and harness output. It records:

- input tokens;
- cached input tokens;
- uncached input tokens;
- output tokens;
- reasoning output tokens;
- raw total tokens; and
- effective tokens.

The derived values are:

```text
uncached_input_tokens = input_tokens - cached_input_tokens
total_tokens          = input_tokens + output_tokens
effective_tokens      = uncached_input_tokens + output_tokens
```

Reasoning tokens are a subset of output tokens and are never added a second time. Input and output counters are required non-negative integers. Cached-input and reasoning-output counters are optional non-negative integers that default to zero. Cached input must not exceed input, reasoning output must not exceed output, and every addition or subtraction must be checked for overflow or underflow. Missing required counters, malformed values, or violated relationships fail analysis rather than silently falling back to another metric.

The package owns the stable TSV header, row serialization, and row parsing. Release and regression summaries expose all seven counters with explicit names. Release and monotonic-regression gates use `effective_tokens`; raw totals remain diagnostic. The benchmark contract names and freezes the metric as `uncached_input_plus_output` so future formula changes require an intentional contract and documentation update.

The analyzer gains a structured `--usage` output for the full token row. The existing `--tokens` output remains a legacy raw-total interface during this change to avoid breaking independent consumers. New benchmark code must not use it for gates.

### Generic skill-read evidence

The analyzer adds `external_skill_read_calls`. A call is counted when transcript evidence shows a direct terminal read or search target that:

1. resolves outside the benchmark workspace; and
2. resolves to a skill bundle, identified by a `SKILL.md` target or a file beneath a normalized skill directory.

Classification is based on path structure and command targets, not plugin names, skill names, prose in prompts, or tool output content. The analyzer accepts the benchmark workspace as an optional absolute `--workspace` argument for this classification. Existing metrics invocations without that option remain compatible, but controlled release harnesses must provide it.

Path normalization handles `/` and `\\` separators, drive letters, redundant separators, `.` segments, and parent traversal before comparing workspace boundaries. Tests cover macOS, Linux, and Windows-shaped paths. Reads outside the workspace that are not confidently within a skill bundle are retained in the transcript but are not misclassified as skill reads.

The analyzer retains event order so reports can show whether skill reads occurred before or after GoreGraph context. The controlled release rule is deliberately stronger: baseline and assisted runs must each have zero external skill reads anywhere in the transcript. This prevents both workflow setup and later task-scoped skill activity from becoming uncontrolled release-matrix variables. Compatibility smoke tests for normal skill-enabled operation remain a separate concern.

### Harness flow and fail-fast semantics

Before consuming an external run, the release harness records the Codex version, exact arguments, and `codex plugin list --json`. If this inventory cannot be captured, the harness aborts before starting the run. It records configuration for evidence only and never changes plugin state.

Each completed transcript then follows this flow:

```text
Codex JSONL
  -> analyzer token and command-path parsing
  -> full usage row plus structural and skill metrics
  -> immutable per-run artifacts
  -> external-skill-read gate
  -> aggregate effective-token gates after all clean runs
  -> qualitative review after mechanical gates pass
```

If `external_skill_read_calls` is non-zero, the harness marks the matrix failed immediately after analyzing that run, preserves stdout, stderr, transcript, metrics, Codex arguments, version, and plugin inventory, and does not launch or replace remaining runs. There is no automatic retry because replacement would hide variance and spend additional external runs without resolving the environmental cause.

After all runs pass contamination checks, release and regression thresholds operate on effective-token medians. Existing structural gates and qualitative review remain independent. A mechanical pass does not imply a release recommendation until the signed quality review also passes.

## Interfaces and artifacts

The additive analyzer and report surface includes:

- the seven explicit token fields;
- `token_metric=uncached_input_plus_output`;
- `external_skill_read_calls`;
- skill-read event order and normalized target evidence;
- benchmark workspace;
- Codex version and invocation arguments; and
- plugin inventory captured before the run.

Release summaries state which token field each threshold uses. Legacy `tokens` data may still be read for older artifacts, but it is never silently interpreted as effective tokens. Historical artifacts stay immutable and keep their original verdict.

## Error handling

- Invalid or incomplete usage counters fail the affected analysis.
- Arithmetic overflow, cached input above input, or reasoning output above output fails the affected analysis.
- Plugin inventory failure aborts before an external run.
- A confidently detected skill read fails the controlled matrix after preserving the run.
- Ambiguous external paths remain visible evidence but are not guessed to be skills.
- Contaminated runs are not retried, removed, or replaced automatically.
- Aggregate gates do not run against an incomplete or contaminated matrix.

## Testing strategy

Implementation follows test-driven development with each behavior introduced by a failing test first.

1. Unit tests cover valid token derivation, optional counters, relation failures, and overflow boundaries.
2. Analyzer tests cover cached-token arithmetic, legacy raw-total output, malformed usage events, generic skill paths, ordinary external paths, event order, and Unix- and Windows-shaped paths.
3. Release-harness tests use fake transcripts to prove that cache-sensitive gates consume effective tokens, complete clean matrices, stop after the first contaminated run, preserve its evidence, and do not invoke subsequent fake runs.
4. Regression-runner and contract tests prove that `uncached_input_plus_output` is the frozen gate metric and that old raw-total artifacts are not reinterpreted.
5. Documentation tests keep README, benchmark instructions, release policy, supported outputs, and generated metric blocks synchronized.
6. Repository verification runs focused Go tests, analyzer and harness shell tests, the full Go suite, `go vet`, formatting checks, and the existing documentation consistency checks.

No external Codex benchmark is part of implementation verification unless the user grants a new, bounded authorization.

## Documentation and release policy

README and benchmark documentation distinguish normal operation from controlled release qualification:

- In normal use, GoreGraph remains compatible with task-scoped skills such as Brainstorming, TDD, and review.
- In a controlled baseline-versus-assisted release matrix, both variants require zero transcript-observed external skill reads.
- `--ignore-user-config` is not documented as a skill-isolation guarantee.
- Effective and raw token metrics are named, defined, and reported separately.
- Published benchmark claims require a mechanically clean matrix plus the existing signed qualitative review.

The implementation does not claim that GoreGraph controls Codex plugin activation. It makes environmental contamination observable and turns it into a deterministic release gate.

## Acceptance criteria

The design is implemented when all of the following are true:

1. Analyzer, release harness, and regression runner share the strict token model and report all seven counters.
2. Every new token-efficiency and monotonicity gate uses effective tokens, while raw totals remain available.
3. Historical benchmark artifacts and verdicts are unchanged.
4. Controlled release runs generically detect skill reads in both variants and stop after the first contaminated run without replacement.
5. Normal skill-enabled GoreGraph workflows are not disabled or reconfigured.
6. Codex arguments, version, plugin inventory, workspace, and contamination evidence are preserved for each matrix.
7. Cross-platform path tests pass for macOS, Linux, and Windows-shaped skill locations.
8. Focused tests, the full Go suite, vet, formatting, shell harness tests, and documentation checks pass.
9. No release, tag, publication, or external benchmark run occurs as part of this implementation without separate authorization.
