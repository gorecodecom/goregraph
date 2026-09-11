# Installed GoreGraph comparison — 2026-09-10

The current local 1.4.1 development build is installed and both authorized Codex
CLI runs completed successfully. In this single pair, the adaptive GoreGraph
workflow used 19.6% fewer uncached input plus output tokens, but took 52.3% longer.
Both final answers cover the 12 established review criteria. The adaptive answer
requires ordinary source fallback; the initial context pack alone is incomplete.

## Installation and workspace

- Build: `1.4.1`, commit label `d67d1f4ab3c3-dirty`, built
  `2026-09-10T13:07:51Z`, Go 1.26.5, darwin/arm64. No release was created.
- SHA-256: `c64c7f3e644e70110ae1cd5f557eb34263e394dd7630bd435d7da2033b0adcb0`.
- Installed at `/Users/gorecode/go/bin/goregraph` and
  `/opt/homebrew/bin/goregraph-local`, with backups. The existing
  `/opt/homebrew/bin/goregraph` symlink is preserved.
- Workspace: `/Users/gorecode/Documents/Codex/2026-07-16/ka/work/goregraph-benchmark/0442483-pre-fix`.
  Update dry-run skips all three unchanged services. Source/index revisions are
  compatible; no rescan was needed.
- After both CLI runs, the integrity verifier passes all 80 checks, including
  unchanged indices, all 374 prepared source files, and the installed binaries.
  The installed build also completed all 15 local retrieval controls without API
  errors. These controls are separate from the two completed CLI tasks.

## Method

Both runs used the same base investigation task, frozen services, Codex model
`gpt-5.6-sol`, high reasoning, read-only sandbox and isolated configuration.
The adaptive run started at 15:11:00 CEST; the baseline followed at 15:26:54 CEST.
The user explicitly authorized transmission of these private sources and tests
to the OpenAI Codex backend after automatic review rejected the earlier launch.

Adaptive used one request to the installed GoreGraph executable, three bounded
verification reads, then authorized ordinary source investigation. Baseline
used ordinary source investigation without GoreGraph or generated index content.
Neither run built, tested or modified the services. Both processes exited 0.

Wall time comes from the external monotonic wrapper; usage comes from completed
CLI events. Tool counts are command invocations, which can contain several shell
operations. No cost estimate is inferred from these token counts.

## Measurements

| Metric | Without GoreGraph | With GoreGraph, adaptive + fallback |
|---|---:|---:|
| Wall time | 600.30 s (10:00) | 914.05 s (15:14) |
| Input tokens, including cached | 2,258,257 | 3,876,080 |
| Cached input tokens | 2,108,928 | 3,767,168 |
| Uncached input tokens | 149,329 | 108,912 |
| Output tokens | 16,510 | 24,387 |
| Uncached input + output | 165,839 | 133,299 |
| All input + output, including cached | 2,274,767 | 3,900,467 |
| Command invocations | 23 | 110 |
| First entrypoint identifier observed | 44.25 s | 28.22 s |
| Last command response | 370.80 s | 685.85 s |
| Time after last command response | 229.50 s | 228.20 s |
| Coverage rubric | 12/12 | 12/12 |
| Existing inventory files with valid numeric ranges | 34/34 | 37/37 |

The comparison metric is `input_tokens - cached_input_tokens + output_tokens`.
Reasoning output is already included in output and is not added again. This
metric falls by 32,540 tokens (19.62%). Total tokens including cache increase;
this is not a claim that all token processing or monetary cost decreased.

Elapsed time increases by 313.75 seconds (52.27%). The time after the last tool
response is almost identical; the observed difference lies in the investigation
phase. Adaptive makes many more small reads, while baseline batches larger source
reads. This identifies a workflow optimization candidate, not a proven causal
explanation from one pair. First identifier visibility does not establish the
time of the first correct diagnosis or complete proof.

Six adaptive commands and one baseline command returned nonzero. Five adaptive
exits and the baseline exit were status 1; no-match searches must not be counted
as API failures. One adaptive command returned status 2 for an invalid search
path. Both agents recovered and completed their answers.

## Quality and evidence limits

The executing Codex reviewer applied the same 12 criteria from
[the benchmark handoff](BENCHMARK-0442483-HANDOFF.md). Both answers identify the
entrypoint, persistence boundary, plausible cleanup gap, three project roles,
both data variants, exact deletion scope, client/provider extension, auth/config,
persistence constraints, side effects, real file targets and regression cases.
Missing database implementation and deployment evidence remain explicit unknowns.

All inventory files and numeric ranges were mechanically checked, alongside
targeted semantic review. Baseline correctly discusses single-task protocol/mail
side effects and read their source, but its final file inventory cites only the
later mail/list ranges for that file. Adaptive supplies a more precise final
reference for the deletion side effects. Neither answer's self-rated 85%
confidence is a measured probability.

The answers suggest different conditional orders for remote cleanup and local
database deletion. Both acknowledge the distributed failure window; the coverage
score does not validate either ordering as a complete implementation design.
This was not a blinded or independent human review and does not exclude every
subtle interpretation error.

The initial adaptive pack reports medium entrypoint confidence and
`insufficient_evidence`, uses 3,876 estimated tokens, and contains seven source
sections plus three verification requests. The primary mutation body is present,
but the full client/provider, data-variant, side-effect and test chain is not.
No explicit reread overlap with supplied source ranges was detected. Final 12/12
coverage is therefore an outcome of GoreGraph **plus fallback**, not pack-only
completeness.

## Remaining work

1. Improve joint evidence selection for client/provider, both data variants,
   deletion side effects and relevant tests within the existing budgets.
2. Investigate batching fallback reads and suppressing irrelevant verification
   targets; measure whether this reduces the many follow-up invocations without
   losing evidence quality.
3. After a concrete change, repeat matched tasks with alternating run order and
   multiple samples before claiming reliable time or token savings.

This single sequential pair is a diagnostic result, not the broader efficacy
acceptance gate. Earlier strict-mode trials and historical comparisons remain
separate. No new implementation change was made during this installation and
benchmark step.

## Local evidence

Artifacts are outside Git under
`/Users/gorecode/.codex/visualizations/2026/09/10/01a08a04-dfec-7502-a22a-af1ed717fd03/benchmark-0442483-macos`:

- `local-update-20260910`: installation record/backups, controls, post-run
  `verification.json`, `paired-results.json` and `comparison.json`.
- `fix-round-cli`: adaptive prompt, raw events, external timing, final answer,
  command timeline, citation/range checks and `quality.json`.
- `local-update-baseline-cli`: corresponding baseline evidence.
- `collect-local-update.py`: metrics extraction; source/index integrity is checked
  separately by `local-update-20260910/verify-fix-round.py`.
