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
- the same restrictions on network access, Git history, builds, tests, and
  writes whenever the neutral prompt forbids those actions.

The only treatment difference is the instruction appended to the neutral base
prompt. Do not add, remove, paraphrase, or reorder any other text.

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

The assisted instruction is exactly these four lines:

```text
Call goregraph context once with the task and its default budget.
Read only cited source needed for verification.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
At most one narrower retry may use the exact route or qualified symbol from that entrypoint; never use a call-chain value.
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

The harness records every raw log and writes `summary.tsv` with the variant,
run number, final `tokens used` value, and log path. Release evaluation uses the
integer median of the three end-to-end token totals for each variant.

## Token gate

Both token conditions must pass:

1. The assisted median must be at most 80% of the matched baseline median.
2. When compared directly with the recorded 145,700-token baseline, the
   assisted median must be at most 116,560 tokens.

The final Codex `tokens used` totals in the retained raw transcripts and
`summary.tsv` are authoritative for this gate. A Context Pack's
`estimated_tokens` value is an approximate local size estimate only; it is
useful for enforcing the pack budget but must not replace end-to-end Codex token
totals.

Each assisted transcript must also show no more than two Context Pack calls and
no specialist GoreGraph query or expert MCP fallback.

## Latest diagnostic evidence

A single matched-task diagnostic run recorded 169,913 baseline tokens and
148,657 assisted tokens. The assisted run used 21,256 fewer tokens (12.51%), or
87.49% of the baseline. It therefore missed the 80% gate: the maximum for that
baseline was 135,930 tokens, 12,727 fewer than the assisted result.

This pair is regression evidence, not release evidence. It has only one run per
variant, and the transcripts did not load identical agent skills. The pre-fix
Context Pack also selected three test methods as entrypoints, required an
unhelpful symbol retry, and did not reduce source navigation.

The resulting 1.3.0 ranking regression now requires one reliable production
entrypoint, ranks the first substantive problem statement ahead of later
analysis requirements, excludes test-source symbols from production seeds, and
follows a bounded two-step production chain. Against the same generated index,
the corrected local pack selects the public DELETE route and its operations
service in 534 estimated tokens instead of the incorrect 895-token pack. This
pack-size improvement does not replace a new three-by-three end-to-end run.

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

Release 1.3.0 only when both token conditions pass, assisted quality is at least
baseline quality, every assisted run follows the Context-call limits, and the
raw transcripts plus signed external rubric are retained.

If any gate fails, do not release 1.3.0. Keep the dashboard, remove the standard
MCP integration from release documentation, and explicitly decide whether to
ship a dashboard-only release or continue Context ranking work in a later
version.
