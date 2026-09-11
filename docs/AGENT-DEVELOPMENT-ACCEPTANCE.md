# Development context retrieval acceptance

Follow-up: [2026-09-10 retrieval fixes and rerun](AGENT-RETRIEVAL-2026-09-10.md).
The results below describe the historical 2026-09-09 run; projection and fallback
limitations subsequently fixed are recorded in the follow-up. The broader C3
efficacy gate remains unmet.

**The C3 broader retrieval/efficacy gate is NOT met.** The 2026-09-09 evaluation did not establish sufficient retrieval for the 24 original German/English development prompts. All 48 strict/adaptive requests returned a low-confidence fallback, with no source sections, selected files, endpoints, or verification requests. No development task was performed by an agent, so completed-task effectiveness and token savings remain unmeasured.

The run used the 12 concrete cases in [the development manifest](../testdata/agent-effectiveness/development/manifest.json), the original prompts, `strict-v1` and `adaptive-v2`, and default context limits: 4,000 estimated tokens and 12 files. Each final project snapshot was indexed once: 16 project scans and four workspace reconciliations. The stale-controller case indexed the saved old source, restored the supplied live source, and queried without rebuilding. All final case-tree hashes were checked against the repository inputs after evaluation. No paid agents ran; no prompt, ranking, or production implementation was changed for this evaluation.

## Observed results

| Measure | strict-v1 | adaptive-v2 |
| --- | ---: | ---: |
| Requests | 24 | 24 |
| API errors | 0 | 0 |
| Fallbacks / low confidence | 24 | 24 |
| Packs containing source evidence | 0 | 0 |
| Estimated tokens, minimum–maximum | 101–130 | 192–221 |
| Estimated tokens, mean | 109.5 | 200.6 |
| Compact JSON bytes, minimum–maximum | 402–523 | 767–888 |

These are sizes of unsuccessful retrieval responses, not savings for successfully completed tasks. Token counts are the pack's estimate; bytes are the UTF-8 size of compact JSON for the pack alone.

Strict returned “no sufficiently relevant context fact found” for 22 prompts and “context confidence is low; inspect source directly” for the English audit and focused-test prompts. Adaptive returned `ambiguous_entrypoint` for 22 prompts and `unsupported_analysis` for those two English prompts. The latter is the current generic mapping for low-confidence fallback; it does not prove that the underlying source language was unsupported. Every adaptive pack reported valid output integrity, unknown live-source freshness, and partial static-analysis coverage. The final scans reported no budget-limited files.

No fabricated exact handler, verified runtime owner, unsafe verification range, or claim of current live-source freshness appeared. Because no source evidence was returned, safe fallback does not establish the source-citation or task-specific uncertainty criteria either.

## Evidence available and still missing

Token pairs below are **German / English**. Every row has zero returned source sections in all four requests. “Available” describes the supplied files or generated indexes, not evidence delivered in the ContextPack.

| Case | Available evidence and unresolved requirement | Strict tokens | Adaptive tokens |
| --- | --- | ---: | ---: |
| `java-missing-audit-call` | Account service and audit dependency have index facts; delete → audit → notification ordering and failure behavior were not retrieved. | 130 / 123 | 221 / 213 |
| `java-auth-header-configuration` | Client, properties, and YAML have index facts; configured header binding and nondisclosure evidence were not retrieved. | 112 / 103 | 204 / 194 |
| `java-stale-controller-source` | Old controller was indexed and live controller restored; the `/v1/orders` versus `/v2/orders` mismatch was not surfaced. Freshness remained unknown. | 112 / 107 | 204 / 198 |
| `java-ambiguous-payment-handler` | Both controller files have facts; neither candidate nor the missing discriminator was presented. | 114 / 104 | 205 / 196 |
| `typescript-import-shadowing` | Raw symbol index contains exact production functions; the compact agent index contains only a visible-test fact. Provider/shadowing evidence was not available through the pack. | 115 / 105 | 206 / 196 |
| `typescript-ignore-generated-bundle` | Ignore file, handwritten client, and generated bundle exist. Agent facts include the bundle but omit the ordinary client function; the required precise exclusion was not established. | 103 / 101 | 194 / 192 |
| `typescript-unsupported-decorator` | `RpcOrders.ts` has a fact; macro implementation/generated route evidence is intentionally absent. The pack supplied neither citation nor the specific macro-expansion gap. | 108 / 110 | 199 / 201 |
| `typescript-focused-test-selection` | Store and three test paths have facts; the two relevant tests were not selected and broader-CI limitations were not explained. | 110 / 106 | 201 / 196 |
| `cross-route-contract-trace` | Consumer, controller, and service have project facts and both projects are registered; the HTTP-boundary trace was not retrieved. | 104 / 105 | 195 / 197 |
| `cross-cancellation-side-effects` | Service and frontend have facts; post-commit event ordering and rollback behavior were not retrieved. | 109 / 107 | 201 / 198 |
| `cross-stale-client-contract` | Live Java provider is represented; the ordinary TypeScript client function is absent from its compact project index. Route migration and authorization preservation were not retrieved. | 103 / 104 | 194 / 195 |
| `cross-unanswerable-runtime-owner` | Both generated consumers have facts; provider source and deployment ownership are intentionally absent. Fallback avoided inventing an owner but did not provide the required consumer citations. | 117 / 115 | 208 / 206 |

Ordinary TypeScript functions are currently omitted from the compact agent projection unless they participate in supported routes, flows, tests, or contracts; exported types, components, and hooks have separate navigation coverage. The missing ordinary-function facts above are a current projection limitation, not evidence that the script scanner failed to extract them. Broad natural-language entrypoint selection is another limitation in this set. No exact-name control queries were substituted for the original prompts.

A separate installed-binary smoke check queried the known Java type `Greeting` and returned adaptive `EXACT` confidence with `fallback_required=false` and a generation identifier. Its local evidence is `installed-smoke-context.json`. This confirms that a bounded known-type query can work; it is not one of these development fixtures, is excluded from all counts above, and does not satisfy the broader efficacy gate.

## Input and run audit

The initial fixture versions contained source sketches and prose test descriptions, so evaluation was paused until concrete files/checks were supplied. One early audit-case run was superseded after its fixture changed. An initial cross-project attempt exposed missing project-discovery markers; the author added actual `goregraph.yml` files to the fixtures, then the four cross cases were rescanned from those final inputs. No disposable-only registration changes are included in the accepted results. The unchanged single-project cases were retained only after their hashes were rechecked.

The final manifest SHA-256 is `2b5b36d8869f3efce0fe982850d55e4632646705983920d31bd9e82f51067948`. Full case-tree hashes and 48 packs are recorded in the local implementation scratch artifact `c3-final-results/accepted-results.json`, with single-project details under `c3-final-results/` and final cross-project details under `c3-cross-final-results/`. Earlier cross-project records under `c3-final-results/` are superseded diagnostics. The temporary runner was removed from the repository; its source was retained with those scratch artifacts. These local diagnostics are not a published benchmark dataset.

The default remains `strict-v1`; adaptive behavior is explicitly selected. The historical effectiveness protocols and benchmark were left unchanged. Their previously reported approximately 86% token reduction is not validated, replaced, or extended by this retrieval-only run. General development-task efficacy is not achieved by the evidence reported here.


## Separate Mac three-cycle follow-up — 2026-09-10

The [three-cycle report](AGENT-THREE-CYCLE-2026-09-10.md) records three further
adaptive-only static analysis runs on the frozen Mac test workspace. Core12/12
passes in each; final time485.95seconds and108888effective tokens improve over
the saved no-GoreGraph reference, but known test-file completeness is5/7 and
repeated reads remain. This does not replace the broader development-fixture
acceptance evidence above or establish general efficacy/release readiness.

## Separate Mac inventory/reuse follow-up — 2026-09-11

The [inventory/reuse report](AGENT-INVENTORY-REUSE-2026-09-11.md) records the completed
follow-up. The same final candidate reaches core12/12 and held-out7/7 in both CLI
runs, using123863/109660 effective tokens and769.79/605.25 seconds. Compared with
the saved600.30-second/165839-token reference, effective tokens improve25.31%/33.88%
but elapsed time is28.23%/0.82% higher. No new baseline or service test execution.
The reader deduplicates carried ranges; separate search/read commands still emit
repeated source. Source labeling and EOF-convention qualifications remain visible.
This closes the measured test-inventory gap in those two runs, but does not change
the broader development-fixture gate, default protocol, or release readiness.


## Separate Mac combined search/read follow-up — 2026-09-11

The [combined-reader report](AGENT-COMBINED-READ-2026-09-11.md) records two completed
same-candidate static runs: core 12/12 and held-out inventory 7/7 in both,
544.16/590.03 seconds and 125283/95784 effective tokens. Both improve on the saved
600.30-second/165839-token reference, with a small second-run time margin. Verified
source outputs have zero repeated rows at the documented parser coverage; caller
size/schema errors, shallow test references, citation gaps and a run-2 HTTP
idempotence terminology error remain. No baseline rerun or service test execution.
This completes the bounded reader improvement and evaluation, without changing
the broader development acceptance gate or establishing general release readiness.

## Separate Mac reader-precision follow-up — 2026-09-11

The [reader-precision report](AGENT-READER-PRECISION-2026-09-11.md) records the
completed CLI normalization/evidence-guidance update and two identical-candidate
runs: 582.256123s/103649 and 680.44152s/110728 effective tokens. Core scores are
12/12 and 11/12: run 2 fails the complete-path criterion despite 7/7 semantic test
identities. Exact test paths are 6/7 and 1/7. Citation delivery improves after
explicit parser adjudication; suggested-test ambiguity and repeated output remain.
Full Go tests/vet, independent code review and local regressions pass. No baseline
rerun, scan or service test execution. This mixed result does not change the broader
development-fixture acceptance gate, default protocol or release readiness.

## Separate Mac answer-check and batch-reader follow-up — 2026-09-11

The [answer-check report](AGENT-ANSWER-GUARD-2026-09-11.md) records completed
multi-selector reads, bounded Markdown evidence validation and adaptive guidance.
After a retained failing diagnostic, the final installed development candidate
passes mechanical validation unchanged and independent static review: core12/12,
seven exact test references, no material semantic error found in this scoped audit.
Final workflow627.602616s/93047effective tokens saves43.89%tokens but takes4.55%more
time than the reused baseline. One oversized read and22 repeated verified lines
remain. No baseline rerun, rescan or service execution. Distinct diagnostic and
confirmation candidates are recorded; this does not alter the broader acceptance
gate or establish general release readiness.

## Separate Mac reader auto-paging follow-up — 2026-09-11

The [reader auto-paging report](AGENT-READER-AUTOPAGE-2026-09-11.md) records three
completed adaptive-only diagnose/fix cycles. The third run reaches core12/12 and
all seven required test identities with zero failed GoreGraph commands, zero
repeated verified source rows, 505.816309 seconds, and 79,464 effective tokens.
This is 15.74% faster and 52.08% lower in effective tokens than the saved baseline.

The retained third-run answer passes the final coverage-aware answer checker with
50 references, 52 ranges, 12 safe repairs, and no findings. Full Go tests, vet and
integrity checks pass. No baseline rerun, rescan, service execution or release was
performed. This successful frozen-workspace run does not by itself establish the
broader multi-fixture acceptance gate or general release readiness.
