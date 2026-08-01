# GoreGraph Agent Context Benchmark

This benchmark is the release gate for the bounded GoreGraph Context Pack. It
measures end-to-end Codex token use and evidence quality with a matched prompt;
it is not a benchmark of the dashboard.

## Matched-prompt protocol

Prepare all benchmark inputs outside the repository. Proprietary source,
prompts, transcripts, and completed score sheets must not be committed.

Every baseline and assisted run must use:

- the identical immutable workspace snapshot;
- the identical neutral base prompt, containing no statement that requires or
  forbids GoreGraph;
- the identical Codex model and reasoning setting;
- the identical sandbox and approval mode;
- the identical workspace and all other `CODEX_BENCHMARK_ARGS`;
- the identical skill availability and invocation settings;
- the same restrictions on network access, Git history, builds, tests, and
  writes whenever the neutral prompt forbids those actions.

The only treatment difference is the instruction appended to the neutral base
prompt. Do not add, remove, paraphrase, or reorder any other text.
Configure identical plugin and skill states outside the treatment prompt. Do
not add “do not use skills” or equivalent control text to either treatment
prompt. `--ignore-user-config` is not a skill-isolation guarantee. The harness
records plugin state but never mutates it.

`external_skill_read_calls` is plugin-agnostic transcript evidence: it counts
read or search targets outside the benchmark workspace that resolve to a skill
bundle. Both controlled variants require zero external skill reads across the
complete transcript. Normal GoreGraph use remains compatible with
Brainstorming, TDD, debugging, and review skills.

Set `CODEX_BENCHMARK_ARGS` as one literal argument per line. The harness rejects
space-split or executable shell text and never evaluates this value:

```bash
export CODEX_BENCHMARK_ARGS=$'-a\nnever\nexec\n--sandbox\nread-only\n--skip-git-repo-check\n--ephemeral\n--ignore-user-config\n--ignore-rules\n--color\nnever\n-m\n<model>\n-c\nmodel_reasoning_effort="high"\n-c\nskills.config=[{path="/absolute/path/to/always-on-bootstrap/SKILL.md",enabled=false}]'
```

The vector must contain exactly one `exec`, explicit model and reasoning
settings, approval mode `never`, sandbox `read-only`, `--ephemeral`,
`--skip-git-repo-check`, `--ignore-user-config`, `--ignore-rules`, and color
mode `never`. The harness owns the workspace and prompt arguments. It rejects
web search, extra directories, JSON mode, danger flags, and duplicate
controlled settings.

`--ignore-user-config` also discards per-skill switches stored by the Codex UI
or in `config.toml`. To reproduce an intentional benchmark skill state without
changing either treatment prompt, the harness permits at most one additional
`skills.config=[...]` override. Use absolute skill paths, pass the identical
override to both variants through `CODEX_BENCHMARK_ARGS`, and retain the exact
effective vector in `codex-args.txt`. This controlled override is not a general
recommendation to disable complementary Brainstorming, TDD, debugging, or
review skills during normal GoreGraph use.

The baseline instruction is exactly this one line:

```text
Do not use the goregraph CLI, MCP tools, goregraph-out, or .goregraph-workspace files.
```

The assisted instruction is exactly these twelve lines:

<!-- goregraph:generated agent-instruction start -->
```text
Call goregraph context . --query "<focused query>" exactly once before reading indexed source; put the caller's problem statement and requested evidence scope in the query.
Preserve the caller's domain language, identifiers, and requested evidence; exclude workspace setup, tool policy, safety constraints, and output-format instructions. Do not translate or add inferred repository or component responsibilities.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
A missing future call, route, or symbol required by the requested fix is evidence of the current gap, not a source-fallback trigger; assess entrypoint reliability from the existing production path.
For change plans, enumerate exact existing production and test paths supplied by the Context Pack or bounded omission reads, do not invent future filenames, and keep future route, authentication, status, lookup implementation, and cross-service transaction ordering as unknown design decisions unless rendered source proves them.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.
```
<!-- goregraph:generated agent-instruction end -->

Reject the benchmark before running if an input is absent, either instruction
differs from the text above, the base prompt is not neutral, or any execution
setting differs between variants.

## Three-by-three execution

Run three independent baseline executions and three independent assisted
executions, alternating which variant runs first in each numbered pair:

```text
baseline 1
assisted 1
assisted 2
baseline 2
baseline 3
assisted 3
```

Use a fresh Codex process for every execution. Do not reuse conversation state
or a previous run's answer. Preserve every complete raw transcript outside the
repository together with the exact prompts, workspace snapshot identifier,
model, reasoning setting, sandbox, approval mode, `CODEX_BENCHMARK_ARGS`, and
run order.

The workspace snapshot identifier hashes benchmark source inputs while
excluding `.git`, `.goregraph-workspace`, and project `goregraph-out`
directories. Those directories contain VCS state or generated treatment
artifacts and must not make an otherwise identical source snapshot appear
different after a fresh scan.

Run the harness with absolute paths:

```bash
scripts/benchmark-agent-context.sh \
  --workspace /absolute/path/to/prepared-workspace \
  --prompt /absolute/path/to/base-prompt.txt \
  --baseline-instruction /absolute/path/to/baseline-instruction.txt \
  --assisted-instruction /absolute/path/to/context-instruction.txt \
  --runs 3 \
  --output /absolute/path/to/results
```

The harness invokes `codex exec --json` itself, records the resulting raw JSONL
stdout log, separate stderr log, and a colocated analyzer result outside the
workspace. Its `summary.tsv` has this schema:

<!-- goregraph:generated agent-benchmark-metrics start -->
The standard release summary schema is:

```text
variant	run	effective_tokens	input_tokens	cached_input_tokens	uncached_input_tokens	output_tokens	reasoning_output_tokens	total_tokens	external_skill_read_calls	tool_calls	goregraph_calls	full_context_packs	compact_duplicate_packs	repeated_full_packs	raw_navigation_calls	source_read_calls	bounded_omission_read_calls	unauthorized_source_read_calls	included_source_rereads	unique_source_files	log
```

The monotonic Golden-versus-candidate summary schema is:

```text
case	query	build	run	attempt	effective_tokens	input_tokens	cached_input_tokens	uncached_input_tokens	output_tokens	reasoning_output_tokens	total_tokens	external_skill_read_calls	tool_calls	context_calls	repeated_full_packs	broad_navigation_calls	source_read_calls	bounded_omission_read_calls	unauthorized_source_read_calls	included_source_rereads	context_millis	log
```

`effective_tokens` is `input_tokens - cached_input_tokens + output_tokens` and is the prospective comparison metric. It represents uncached input plus output. `total_tokens` is `input_tokens + output_tokens`. `reasoning_output_tokens` is recorded separately, and reasoning output is already part of output, so it is not added again.

`external_skill_read_calls` counts transcript-observed read or search targets outside the benchmark workspace that resolve to a skill bundle. Controlled baseline and assisted release runs require zero; normal GoreGraph workflows may continue to use task-scoped skills.

`source_read_calls` remains the total number of direct source-read terminal calls. `bounded_omission_read_calls` counts exact ranged reads wholly authorized by an earlier full Context Pack. `unauthorized_source_read_calls` counts every other source read, search, or inventory terminal call. A compound call is bounded only when every source target is bounded, and included-source overlap is never bounded. Bounded reads remain part of tool and token totals and cannot exceed the case contract's `max_source_omissions`. The monotonic gate compares unauthorized reads; the matched release gate continues to compare total `source_read_calls`.
<!-- goregraph:generated agent-benchmark-metrics end -->

Release evaluation uses the integer median of the three end-to-end
effective-token, tool-call, raw-navigation, and source-read totals for each
variant. The analyzer
deduplicates source paths and retains counts only; it does not retain source
content. It counts only unique terminal tool items from the Codex JSONL event
lifecycle, including unsuccessful tools. `included_source_rereads` counts a
terminal tool item once when it reads or searches source already supplied by an
earlier Context Pack. Complete packs protect every included source path. Partial
packs protect included line ranges, so only overlapping or whole-file reads
count; reads of other ranges and reads before the pack do not count.

## Token gate

Both raw and effective counters are retained. `effective_tokens` is
`input_tokens - cached_input_tokens + output_tokens`, or uncached input plus
output; `total_tokens` is `input_tokens + output_tokens`. Reasoning output is
recorded separately but is already part of output, so reasoning output is not
double-counted.

Both token conditions must pass:

1. The 80% matched threshold uses effective tokens: the assisted median must be
   at most 80% of the matched baseline median.
2. The 116,560 absolute cap uses effective tokens: the assisted median must be
   at most 116,560 effective tokens.

The retained JSONL transcripts and `summary.tsv` are authoritative for this
gate. Context Pack `estimated_tokens` remains unrelated to end-to-end usage; it
is an approximate local size estimate used only to enforce the pack budget.

Each assisted transcript must show the source-backed workflow above: one initial
Context Pack call, at most one narrower retry, and no specialist GoreGraph query
or expert MCP tool.

## Structural gates

After both token conditions pass, all structural conditions must pass:

1. The assisted tool-call median must be at most 70% of the matched baseline
   median.
2. The assisted source-read median must be at most 50% of the matched baseline
   median. A zero baseline source-read median is invalid benchmark input because
   it cannot measure source-replacement savings.
3. No assisted transcript may contain a repeated full Context Pack.
4. The sum of `included_source_rereads` across assisted transcripts must be
   zero.

Content quality is enforced by deterministic Context Pack regressions rather
than by benchmark-specific filenames in the transcript analyzer. Those
regressions require requested `domain_model` evidence, prefer informative
declaration bodies with stable domain identity, and allow up to two distinct
domain-model and persistence families per project. The analyzer's efficiency
gates and schema remain environment-neutral.

`compact_duplicate_packs` records responses with `duplicate_of`. These compact
responses are expected diagnostic evidence and do not fail the benchmark. A
later full payload that reuses a previously full `context_id` is instead counted
as `repeated_full_packs` and fails the gate. This deliberately replaces the
earlier ambiguous single duplicate-pack column.

## Latest diagnostic evidence

<!-- goregraph:generated release-evidence-status start -->
The latest controlled three-by-three release benchmark did not pass: its raw total-token medians were 2551495 baseline and 147212 assisted, so the assisted result exceeded the legacy 116560 absolute cap. The retained result remains failed and is not rescored. A prospective offline calculation produced effective-token medians of 164295 and 39180, but both variants also contained external skill reads. Publication remains blocked until a fresh prospectively calibrated matrix has zero external skill reads and receives the required signed 12-point quality review.
<!-- goregraph:generated release-evidence-status end -->

The previous controlled three-by-three result remains failed and is not
rescored. Its offline effective-token calculation is diagnostic only. A fresh,
prospectively calibrated matrix must satisfy the complete-transcript zero-skill
rule; prompt text must not be used to disable skills for either variant.

## Twelve-point quality rubric

Quality is scored manually from retained transcripts against source evidence.
Award one point only when the answer correctly and specifically evidences the
item. Award zero for an incorrect, unsupported, missing, or materially
incomplete answer.

1. Public endpoint.
2. Current call chain.
3. Root cause.
4. Required cross-repository call chain.
5. Task variants.
6. Lookup attributes.
7. Internal API contract.
8. Authentication/configuration.
9. Persistence operations.
10. Business side effects.
11. Production/test files.
12. Error, retry, and test strategy.

Apply the same rubric and reference evidence to all six transcripts. Record each
run's score out of 12 and calculate the integer median for each variant. The
assisted quality score must be greater than or equal to the baseline quality
score.

The harness does not score quality. An independent reviewer must complete and
sign the rubric outside the repository, recording at minimum:

```text
Workspace snapshot:
Base prompt digest:
Model and reasoning:
Sandbox and approval mode:
Baseline scores (runs 1-3) and median:
Assisted scores (runs 1-3) and median:
Evidence notes for rubric items 1-12:
Reviewer name:
Reviewer signature:
Review date:
```

Retain this signed rubric with the raw transcripts and `summary.tsv` outside the
repository as release evidence.

## Monotonic regression benchmark

The release benchmark compares no GoreGraph with one accepted GoreGraph build.
The monotonic regression benchmark compares the frozen Golden GoreGraph build
from commit `1bc4408` with one candidate build. Passing either benchmark does
not imply passing the other.

### Phases and case matrix

Smoke uses one run per build only to validate mechanics and artifacts. A smoke
result is not gateable. Full uses exactly three paired runs per build, and only
those three-run results may reach the monotonic gate.

The matrix contains six cases. G1 is external and operator-owned; its materials
remain outside the repository. G2-G6 are committed generic cases. Every Golden
and candidate run requires an independent, evidence-backed review against
source evidence. Do not reuse a review between runs.

Before either phase starts, freeze the contracts, thresholds, single
hypothesis, Golden and candidate binaries, and run order. Record their hashes
or digests with the external evidence. A later contract correction applies only
to a newly frozen run and never retroactively changes a retained review or gate
result.

### Pack contracts and answer reviews

Put deterministic Context Pack invariants in the contract's `pack` section.
For example, G3 requires its generic Go route in `ContextPack.Entrypoints` and
sets `max_endpoints` to `0`; the local matrix test verifies both conditions
without an external answer. Do not ask an answer reviewer to repeat internal
serialization or representation details.

The answer rubric covers only user-visible evidence quality, forbidden
outcomes, and disclosures that the answer must actually make. This keeps a
correct Pack representation from failing because the final answer omitted an
implementation detail that the user did not request.

### Monotonic gates

Quality and per-run safety conditions always remain hard:

1. Every previously passing required facet must pass in every candidate run.
2. Forbidden outcomes and required uncertainty disclosures must remain safe in
   every candidate run.
3. The single declared target facet must improve in at least two of the three
   candidate runs.
4. Candidate bounded omission reads may not exceed the case contract.

The runner's semantic `pack-diff.json` determines whether comparative
efficiency can be attributed to GoreGraph. When the Pack Diff contains a
semantic change, all comparative limits remain hard:

1. Median unauthorized source reads and total tool calls may not increase.
2. Median end-to-end tokens may increase by at most 5%.
3. Median direct Context latency may increase by at most 10%.
4. Unexplained paired direct Context latency above 2x fails.

When the semantic Pack Diff is unchanged, breaching one of those comparative
limits is retained in `observations` as model variance and does not become a
GoreGraph product regression. Answer-quality, review-integrity, run-validity,
metric-validity, and bounded-read failures are not downgraded.

The gate command requires the runner's exact case/query Pack Diff:

```text
scripts/benchmark-agent-context-regression.sh gate \
  --contract <absolute-contract.json> \
  --hypothesis <absolute-hypothesis.json> \
  --pack-diff <absolute-pack-diff.json> \
  --golden-runs <absolute-golden-runs.json> \
  --candidate-runs <absolute-candidate-runs.json> \
  --output <absolute-gate-report.json>
```

Passing the smoke phase does not satisfy these gates. The final monotonic gate
requires exactly three paired Golden and candidate runs for every evaluated
case.

### Run validity and retained evidence

A valid slow or inaccurate run is never replaced. An infrastructure-invalid run
retains its diagnostic artifacts, cannot pass, and cannot count as a gateable
run. Record the invalid reason and retained log rather than treating a quality
failure as infrastructure failure.

Source snapshots and prepared workspaces are temporary and are removed after
success, failure, or cancellation. Retained external evidence contains hashes,
semantic packs and diffs, transcripts, metrics, review templates and completed
reviews, summaries, and gate reports. Keep process diagnostics with that
evidence, including unchanged-Pack model-variance observations, but do not
retain source copies.

### External G1 evidence

`GOREGRAPH_G1_WORKSPACE` and
`GOREGRAPH_G1_REFERENCE_TRANSCRIPT` must be absolute, readable,
operator-supplied paths. Neither path nor its contents may be copied into
repository output; all G1 materials and benchmark evidence remain in
operator-controlled external directories.

Build the Golden binary from detached commit `1bc4408` and the candidate binary
from the implementation worktree. Install them into separate external evidence
directories and hash both binaries. Do not replace the operator's normal local
installation.

The G1 contract stays external and preserves its required facets, forbidden
outcomes, and explicit unknowns. For an accuracy candidate, derive the
candidate contract externally before execution, move only the single declared
target facet from explicit unknowns to required facets, and retain both
contract digests. Every other explicit unknown remains a mandatory uncertainty
disclosure.

After a failed gate, an external `failure-classification.json` is required
before a new hypothesis may begin. Allowed categories are `scanner_truth`,
`intent`, `ranking`, `budget`, `rendering`, or `agent_behavior`. The
classification is evidence only and cannot waive a failed gate.

## Release decision

Release 1.3.0 only when both token and structural conditions pass, assisted
quality is at least baseline quality, every assisted run follows the Context-call
limits, and the raw transcripts plus signed external rubric are retained.

If any gate fails, do not release 1.3.0. Keep the dashboard, remove the standard
MCP integration from release documentation, and explicitly decide whether to
ship a dashboard-only release or continue Context ranking work in a later
version.
