# Adaptive retrieval and fallback correction — 2026-09-10

The [installed comparison](AGENT-INSTALLED-COMPARISON-2026-09-10.md) took 15:14
with GoreGraph versus 10:00 without it, despite matching final coverage scores.
The adaptive run issued 110 commands versus 23. This correction targets concrete
selection defects and the fragmented fallback workflow observed in that run.

## Changes

- Adaptive source selection now allocates space to rendered evidence before
  filling optional file inventory. A new source file may displace unrendered
  supporting inventory; existing source sections, mandatory locations and the
  aggregate file limit remain protected.
- Configuration inventory cannot reintroduce domain-unrelated owners solely
  because they expose generic authentication or retry settings. Selected
  contract/resource bindings remain eligible.
- Related-model candidates are bounded after diversifying across files. Many
  routes from one controller can no longer consume the whole candidate allowance
  ahead of a service implementation or an internal provider. Explicit requests
  for internal interfaces prioritize those candidates.
- Being in a project containing a related model no longer admits a client for
  another parent domain. Generic authentication policy remains eligible within
  relevant projects. Plural route names can match singular words actually found
  in the candidate model declarations.
- Missing proof takes priority over additional exact file inventory in adaptive
  omissions. Verification requests exclude source ranges already supplied,
  retaining only bounded unseen segments and preserving project identity.
- The canonical adaptive instruction separates retrieval content from tool and
  output-format policy, while continuing to obey that policy. Independent bounded
  verification reads and caller-authorized fallback reads should be batched.

These changes do not infer new runtime call edges, expand the caller's authority,
raise response budgets or claim complete evidence from an incomplete pack.
Strict-v1 selection and instruction text remain compatible with the historical
controls. A preliminary control run exposed two changed strict outputs; explicit
protocol gating resolved them, and all three strict JSON controls match again.

## Verification and measurement

Six new regressions were observed failing before their fixes; an additional
integration check protects model bodies and the file limit. Independent review
caught the plural-route case and verified its correction. The final full suite (`go test ./... -timeout 20m`), `go vet ./...` and
`git diff --check` pass. All 15 unchanged controls and 93 integrity/budget/range
checks pass, including all three unchanged strict JSON outputs and no supplied
source rereads in adaptive verification requests.

The fresh adaptive CLI run uses the same frozen three-service workspace, base
task, `gpt-5.6-sol`, high reasoning and read-only permissions. The user requested
reusing the established no-GoreGraph baseline. The newly started baseline was
stopped (exit 143) and is excluded from the comparison; future rounds must not
repeat that baseline without a new explicit user request. The adaptive prompt uses the revised canonical workflow; this
measures the combined selection and workflow change, not selection in isolation.
No source/test changes or service builds are part of the benchmark.

## Completed adaptive attempts

| Metric | Stored baseline | Previous adaptive | Updated adaptive, attempt 1 | Updated adaptive, attempt 2 |
|---|---:|---:|---:|---:|
| Elapsed seconds | 600.30 | 914.05 | 888.70 | 903.10 |
| Uncached input plus output tokens | 165,839 | 133,299 | 145,306 | 131,049 |
| Input tokens including cache | 2,258,257 | 3,876,080 | 2,252,823 | 2,885,096 |
| Cached input tokens | 2,108,928 | 3,767,168 | 2,129,024 | 2,776,704 |
| Output tokens | 16,510 | 24,387 | 21,507 | 22,657 |
| Command invocations | 23 | 110 | 24 | 36 |
| First entrypoint identifier, seconds | 44.25 | 28.22 | 26.58 | 24.00 |
| Last command response, seconds | 370.80 | 685.85 | 428.02 | 635.15 |
| Coverage criteria | 12/12 | 12/12 | 12/12 | 12/12 |

Attempt 1 completed with exit 0. It used 12.4% fewer reported uncached input plus
output tokens than the stored baseline, but 9.0% more than the previous adaptive
attempt. Commands fell from 110 to 24. Reported elapsed time is still 48.0% above
the baseline; this does not establish that the latency problem is solved.

The CLI recorded a backend stream disconnection and automatic retry at
`2026-09-10T14:31:19.439297Z` while preparing the final answer. Elapsed time includes
that interruption; no guessed duration is subtracted. CLI-reported usage is kept
as reported, but accounting for a disconnected generation is not independently
verified. The identical adaptive-only repeat also completed with exit 0 and
recorded a stream disconnection at `2026-09-10T14:47:54.236389Z`. Its reported token
reduction is 21.0% against the stored baseline, with 50.4% greater elapsed time.
No further repeat was started. Both attempts remain recorded; neither establishes
a reliable speed advantage. Even before its final response, attempt 2 had spent
635.15 seconds on command responses, exceeding the baseline's entire 600.30
seconds. Backend interruptions therefore do not explain all remaining overhead.

Attempt 2 made one successful context call and one rejected retry using an
invented `--retry-anchor` option (exit 2). It then used authorized source fallback.
Its 40 inventory paths and tabular source ranges are valid; the same executing
reviewer assessed 12/12 coverage, with the same transaction-design limitations.
Both runs have zero file-change and web-search items. Token totals are CLI-reported
input minus cached input plus output; reasoning tokens are already part of output.
This is not a monetary-cost calculation or a statistical efficacy claim.

The executing reviewer assessed the same 12 coverage criteria, checked all 36
inventory paths and tabular source ranges, and inspected the core mutation,
client and task side effects against source. This is coverage assessment, not
blinded evaluation or approval of the proposed cross-service transaction design.
The answer identifies the absence of atomicity, the inverse failure state after
remote cleanup and concurrent creation as unresolved design questions.

The initial 3,976-token pack provides seven source sections and three bounded
verification requests. Two requests concern unrelated cachet/protocol models.
The final answer rejects those as direct regulation-cleanup targets, but reaching
12/12 coverage still requires ordinary authorized source fallback. Complete-task
efficacy and fully relevant evidence selection remain open.

Raw evidence is outside Git in the existing Mac benchmark artifact directory:
`latency-fix-stable` for the installed candidate and controls,
`latency-adaptive-cli` for attempt 1 and `latency-adaptive-retry-cli` for its repeat, `local-update-baseline-cli` for the reused
reference, and `latency-baseline-cli` for the excluded cancelled attempt. Intermediate
selection experiments remain separately labeled and are not completed-task
benchmark results.


## Retry instruction follow-up

Attempt 2 exposed an ambiguous canonical instruction. The adaptive guide now
explicitly supplies the executable CLI form: use exactly one supplied
`retry_anchors` value as `--query`, plus `--previous-context-id <context_id>`.
The corresponding MCP arguments are `query` and `previous_context_id`. The
historical strict guide remains unchanged. A separate local syntax control passes.
The instruction/context tests in `internal/agentguide` and `internal/cli`, plus
root documentation tests and `internal/mcp`, pass after this small follow-up.

The two completed CLI attempts used the earlier frozen prompt and executable;
they do not measure the effect of this final instruction clarification. The
post-run 80 checks passed before replacement. Final installation checks confirm
unchanged workspace sources/indices, the expected binaries, and identical output
from the valid retry command. Source hashes confirm that only the canonical guide
changed after the benchmark build. No new full CLI run was started for this text
clarification.

## Local installation

The benchmark used 1.4.1 development build `d67d1f4ab3c3-dirty`, built
`2026-09-10T14:11:18Z`, at `/Users/gorecode/go/bin/goregraph` and
`/opt/homebrew/bin/goregraph-local`, preserving the Homebrew symlink and backing
up both prior binaries. SHA-256:
`f33366fe8aaa01b29005e8cf6000daa8a043bc9bf5e1db509ec6f2c401206e92`.
The final instruction follow-up is now installed at both paths, with fresh
backups in `retry-guide-final`. It is still 1.4.1 `d67d1f4ab3c3-dirty`, built
`2026-09-10T14:50:38Z`, SHA-256
`360edcebf72eaffc594499b4822f8e8eee20e7a84ffdafcf336af18aa34ca27c`.
No release was created. The workspace update dry-run skipped all three unchanged
services; their 374 source files and existing indices still retain their hashes.
No rescan is required.
