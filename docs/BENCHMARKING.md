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
Control skill isolation through the Codex invocation, not through either
treatment prompt. In particular, never add “do not use skills” to one prompt.

Set `CODEX_BENCHMARK_ARGS` as one literal argument per line. The harness rejects
space-split or executable shell text and never evaluates this value:

```bash
export CODEX_BENCHMARK_ARGS=$'-a\nnever\nexec\n--sandbox\nread-only\n--skip-git-repo-check\n--ephemeral\n--ignore-user-config\n--ignore-rules\n--color\nnever\n-m\n<model>\n-c\nmodel_reasoning_effort="high"'
```

The vector must contain exactly one `exec`, explicit model and reasoning
settings, approval mode `never`, sandbox `read-only`, `--ephemeral`,
`--skip-git-repo-check`, `--ignore-user-config`, `--ignore-rules`, and color
mode `never`. The harness owns the workspace and prompt arguments. It rejects
web search, extra directories, JSON mode, danger flags, and duplicate
controlled settings.

The baseline instruction is exactly this one line:

```text
Do not use the goregraph CLI, MCP tools, goregraph-out, or .goregraph-workspace files.
```

The assisted instruction is exactly these nine lines:

```text
Call goregraph context once with the complete task before reading indexed source.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.
```

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

```text
variant run tokens tool_calls goregraph_calls full_context_packs compact_duplicate_packs repeated_full_packs raw_navigation_calls source_read_calls included_source_rereads unique_source_files log
```

Release evaluation uses the integer median of the three end-to-end token,
tool-call, raw-navigation, and source-read totals for each variant. The analyzer
deduplicates source paths and retains counts only; it does not retain source
content. It counts only unique terminal tool items from the Codex JSONL event
lifecycle, including unsuccessful tools. `included_source_rereads` counts a
terminal tool item once when it reads or searches source already supplied by an
earlier Context Pack. Complete packs protect every included source path. Partial
packs protect included line ranges, so only overlapping or whole-file reads
count; reads of other ranges and reads before the pack do not count.

## Token gate

Both token conditions must pass:

1. The assisted median must be at most 80% of the matched baseline median.
2. When compared directly with the recorded 145,700-token baseline, the
   assisted median must be at most 116,560 tokens.

The complete-session `turn.completed` usage totals in the retained JSONL
transcripts and `summary.tsv` are authoritative for this gate. A Context Pack's
`estimated_tokens` value is an approximate local size estimate only; it is
useful for enforcing the pack budget but must not replace end-to-end Codex token
totals.

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

The latest diagnostic pair recorded 169,913 baseline tokens and 166,833
assisted tokens, a 3,080-token reduction (1.81%). The assisted run made 31 shell
executions versus 28 baseline executions, a 10.71% increase. This is diagnostic
evidence only, not controlled three-by-three release proof. A release run must
isolate skills in the invocation for both treatments; prompt text must not be
used to disable skills for only one variant.

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

## Release decision

Release 1.3.0 only when both token and structural conditions pass, assisted
quality is at least baseline quality, every assisted run follows the Context-call
limits, and the raw transcripts plus signed external rubric are retained.

If any gate fails, do not release 1.3.0. Keep the dashboard, remove the standard
MCP integration from release documentation, and explicitly decide whether to
ship a dashboard-only release or continue Context ranking work in a later
version.
