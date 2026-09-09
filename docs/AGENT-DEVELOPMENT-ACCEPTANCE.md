# Development context retrieval acceptance

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
