# Implementation ledger — plan: docs/superpowers/plans/2026-09-09-goregraph-improvement.md

Baseline: `081b405`; execution authorized directly on `main` on 2026-09-09.

## Rulings

- Work on the existing `main` checkout: explicit user instruction overrides the skill's worktree default.
- Preserve the existing private WEKA indexes during implementation. User subsequently authorized installing the completed local build as version 1.4.1 for personal testing. Do not push, create a remote tag or publish a release.
- Preserve the historical approximately 86% token-saving case and its existing gates; the broader 25% target does not replace that baseline. Do not claim fresh end-to-end benchmark results without running them.
- Use independent subagents as required by the executing-plans/subagent-driven-development skills; restrict file ownership and integrate shared files centrally.
- Keep raw baseline logs and diagnostic scratch in the system temporary directory; only synthetic fixtures and aggregate non-private evidence belong in Git.
- Treat proposed performance gates as targets, not achieved results. External paid evaluation remains a separately budgeted action.

## Preflight interface review

| Tasks | Shared contract | Decision |
|---|---|---|
| A1/A2/A4 | enumeration, context, ignore digest | Root integrates; no concurrent edits to scan.go/workspace_update.go. |
| A2/A3 | cancellable script extraction | A3 owns script files and produces context-aware API; root consumes it. |
| A4/B1/B2 | manifests, projection identity, freshness | Integrate sequentially; do not advertise transaction guarantees early. |
| A3/B3 | pure extraction facts | Cache only completed extraction; resolution stays outside cache. |
| B2/C1/C2 | health, protocol and verification metadata | Wire fields remain additive and budgeted. |
| B4/C3 | context loader/ranker performance | Profile before introducing shared caches or maps. |
| C0/C4 | task contracts and outcomes | Independent fixture/evaluation package; no paid runs or hidden answer leakage. |
| C1/C5 | strict protocol replay/default switch | Preserve strict instruction unchanged; adaptive default requires outcome evidence. |

All task tests/targets reviewed against the design. Complex publication, scope analysis and external evaluation require retained evidence; no blanket waivers for baseline failures.

## Task status

- A0: baseline evidence retained; sandbox-only source-path/Git failures reproduced as passing in normal-user tests. The interrupted baseline is not a full-suite pass.
- A1: scoped ignore matcher, Git parity checks, shared scan/update/Doctor inventory implemented. Generated transaction bookkeeping is ignored on mutating commands, before update snapshots; read-only commands do not edit ignore files.
- A2: context, progress, file budgets, project snapshot budgets and cancellation checkpoints in script/workspace resolution implemented. Legacy synchronous analyzer/builders remain cooperatively cancellable between operations; no hard OS-level interruption is promised.
- A3: indexed lexical scopes retain syntax/fact compatibility. Synthetic extraction improved at least 27x under concurrent load; this is not an end-to-end/token result. Fuzz regression and 16,548-input rerun passed.
- A4: explicit build identities, pre-publication source rechecks and workspace no-op detection implemented. Effective-config read scope and the source-change-after-planning race are covered by passing regressions.
- B1: project and multi-output workspace transactions and stable built-in read boundaries integrated. Three independent recovery findings were fixed and independently re-reviewed; Windows failure/restart suite and Linux compilation passed. Discovery refreshes project index status after locking, rejects relocated output roots, and has a passing regression.
- B2: separate integrity/freshness/coverage metadata integrated into adaptive context, Doctor and dashboard. Interrupted explicit-provider analysis remains unresolved instead of claiming a missing implementation. Doctor review fixes and stale workspace/custom-policy regressions passed.
- B3/B4: synthetic full-build profile attributes only 3.34% cumulatively to script extraction after A3. No persistent cache, database or speculative lookup rewrite is justified; see SCAN-BENCHMARKING.md. Representative 50% end-to-end/update gates remain unmeasured.
- B5: investigation navigation preserves query/filter/focus, with health and uncertainty visible. Two real Playwright/Chromium tests passed, including offline asset lifecycle; synthetic screenshots reviewed. See DASHBOARD-ACCEPTANCE.md.
- C0: twelve concrete bilingual development tasks implemented. Six change fixtures have executable visible checks that pass initially and evaluator-owned behavioral acceptance checks that fail on their intended initial bugs; six investigations use full source and blinded rubrics. Acceptance checks stay outside scanned project roots. The initial sketch/discovery deficiencies were corrected before final evaluation.
- C1/C2: opt-in adaptive-v2, bounded exact verification requests and stable fallback reasons implemented. Strict-v1 remains the unchanged default; extra metadata is budgeted.
- C3: retrieval evaluation completed, but the broader gate is NOT met: all 48 original bilingual strict/adaptive requests fell back without source evidence. Compact ordinary-function projection and broad entrypoint selection remain limitations. No speculative ranking rewrite or efficacy claim was made; broader relevance work is unfinished. See AGENT-DEVELOPMENT-ACCEPTANCE.md.
- C4: offline manifest/outcome/usage validation and paired metrics implemented and tested. Nine-attempt pilot and 144-attempt held-out evaluation remain external, separately budgeted work.
- C5: source and documentation identify the local 1.4.1 candidate. Full Windows Go suite passed all 20 packages (2,653 pass events, 64 platform/optional skips); vet, documentation checks and two separate real-browser journeys passed. Native Linux/macOS/race and external paid outcome trials remain unrun. Local installation/rollback is described in LOCAL-1.4.1.md; the user authorized committing and pushing the candidate to `main` for backup while testing. No release tag or release publication is authorized.

## Evidence boundaries

Historical strict benchmark package tests passed during integration; the approximately 86% historical token saving remains unchanged historical evidence. The new adaptive workflow has not passed a new paid end-to-end outcome trial and therefore does not become the default. This local candidate is for user acceptance, not a claim that all program performance/effectiveness gates are complete.

Local installation completed: Scoop current selects the separately built 1.4.1 (HEAD-dirty), with 1.4.0 preserved. The installed executable passed a synthetic all-projection build, nested-ignore exclusion and adaptive named-symbol Context Pack smoke test. No private indexes were rebuilt and nothing was pushed.

## Local scan-all follow-up

The user-reported Windows stage promotion failure stopped the third of 43 WEKA
projects. A real open-file regression reproduced the access-denied rename.
Windows renames now retry access/sharing/lock violations for up to two seconds;
persistent errors preserve the previous output. Workspace build/scan-all now
attempts remaining projects after individual failures, reports all failures and
skips reconciliation on a failed run. Cancellation still stops the loop.
Both affected packages passed in full, along with scoped vet and documentation
checks. The first real follow-up attempted all 43 projects: 42 succeeded, while
frontends reported changing source inputs. A subsequent read-only comparison
found matching contents and ignore rules for all 1,461 frontend files.

The remaining publication failure was traced with Windows handle inspection to
the separate frontend project's `@wbp/local-dev` Node watcher (PID 29392 during
diagnosis), which opens all stage subdirectories. A synthetic output reproduced
the same permanent block. Its current generated-build ignore predicate lacks
GoreGraph paths. The tested scan-all/retry improvement is installed locally as
1.4.1; the remaining frontend watcher exclusion and server restart require work
in that separate running project. No watcher was stopped or source there edited.

## Context selection follow-up — 2026-09-10

A synthetic regression reproduced a source-planning loss: keeping only the
highest-ranked fact per file discarded a second deletion method containing the
requested mail side effect, before its source could be verified. The context
planner now fills unused slots with additional distinct declarations after
selecting representatives from different files. The eight-candidate limit per
concern, final response budgets, source verification and protocol defaults are
unchanged. Duplicate facts at the same project/path/declaration line do not
consume additional slots.

The new regression verifies both the rendered mail evidence and its delivery
in a context pack with a primary entrypoint in another project. A second test
covers file diversity, the candidate limit, duplicate declarations and stable
ordering. Both tests fail against the previous implementation and pass with the
fix on macOS arm64 / Go 1.26.5.

`go vet ./...` passed. The full `go test ./... -timeout 20m` run passed all
packages except `internal/dashboardeditor` (sandbox denial of loopback binds)
and `internal/outputstore` (test fault-injection path comparisons under the
symlinked macOS temporary directory). The dashboard package passed when rerun
outside the sandbox. The output-store package passed with
`TMPDIR=/private/tmp GOCACHE=/tmp/goregraph-go-build go test ./internal/outputstore -count=1 -timeout 5m`.
No dashboard or output-store code was changed for these environment adjustments.

This is a bounded correction to evidence selection after candidate discovery.
It does not establish that the private benchmark's missing client/provider
chain is recovered, fix broad natural-language entrypoint selection or ordinary
TypeScript-function projection, or satisfy the C3 gate. The private workspace
and benchmark artifacts were not transferred with the repository; no new paid
agent trial, installation or release was performed.

## Retrieval and source freshness follow-up — 2026-09-10

The next bounded corrections add exported ordinary script functions to the agent
projection, preserve current candidate evidence in adaptive low-relevance
fallbacks, distinguish insufficient relevance from unsupported analysis, resolve
literal script test registrations, and record project-scoped source snapshots.
Adaptive queries detect changed or missing selected/concern-expanded files before
duplicate suppression. Agent projection build revision 2 triggers rebuilding for
the new projection content. Strict-v1 remains the default.

An independent review found missing-file and concern-expanded-file freshness
gaps and an unreserved context-ID budget boundary; all three were corrected with
regressions. A follow-up review also caught context identities losing the
association between source paths and contents; a structured candidate identity
and a swapped-file regression correct that collision. The complete suite also exposed a named TypeScript test compatibility
regression, fixed by retaining declaration resolution ahead of title resolution.

The retrieval-only rerun of the original 48 development requests now returns
source sections in 1/24 strict and 17/24 adaptive packs, compared with 0/48 for
the baseline. Both protocols still return 23 fallbacks. These counts do not prove
complete task coverage, answer quality or token savings. Details and outstanding
limitations are in [the follow-up report](../AGENT-RETRIEVAL-2026-09-10.md).

Final verification passed for every package with
`TMPDIR=/private/tmp GOCACHE=/tmp/goregraph-go-build go test ./... -timeout 20m`
outside the sandbox, plus `go vet ./...` and `git diff --check`. The final 48-pack
rerun had no API errors and respected the default token budget and bounded
verification paths. Changes remain local on version 1.4.1; no release was created.

## 2026-09-10 — diagnostic correction round

The [correction report](../AGENT-FIX-ROUND-2026-09-10.md) records adaptive metadata
and verification reservation, UTF-8 byte accounting, German evidence vocabulary,
stricter source proof, bounded inferred model/source discovery and explicit
`insufficient_evidence` fallback. Inferred candidates do not enter the established
call-chain selection. Thirteen new regression test functions cover the defects
and additional independently reviewed counterexamples.

All 15 saved Mac controls complete without API errors, versus eight errors in
the diagnosed build. They respect public budgets and bounded source ranges; the
prepared source, indices and installed binaries remain unchanged. Cross-service
completeness is still an open acceptance item. A prepared Codex CLI follow-up has
not started: automatic approval review requires explicit consent to send private
workspace source to the Codex backend. No new completed-task savings are claimed.


## 2026-09-10 — primary mutation evidence

Adaptive source selection now prevents supporting file inventory from blocking
the verified entrypoint and first local call. Primary-path coverage requires
method bodies, and a missing body receives a bounded verification request before
additional inventory gaps. Independent review caught and verified a fix for
metadata-only path double counting. Three new regressions and the full suite pass.

The `fix-round-completeness-03` Mac controls pass all 15 requests and 80 independent
checks. Neutral 4,000/6,000-token packs include the previously omitted mutation
body; strict-v1 outputs and prepared sources/indices/installed binaries remain
unchanged. The [correction report](../AGENT-FIX-ROUND-2026-09-10.md) retains the
remaining cross-service completeness limits and the unstarted CLI consent block.


## 2026-09-10 — authorized local installation

Installed the current local 1.4.1 development build at both existing executable
paths, with backups and SHA-256 installation records. Workspace update dry-run
skips all three unchanged test services; no rescan was required. Automatic review
again blocked the private-source Codex CLI launch pending explicit consent to
transmit the workspace source/tests to the OpenAI Codex backend.

The user subsequently supplied the exact private-source/backend consent. Both
the adaptive CLI diagnostic and a contemporaneous baseline completed successfully
in sequential order. The installed binary also passes all 15 local
controls and 80 integrity/budget/range checks, with the authorized binary update
accounted for separately from unchanged Codex binaries and workspace data.


## 2026-09-10 — completed installed CLI comparison

The [installed comparison](../AGENT-INSTALLED-COMPARISON-2026-09-10.md) supersedes
the earlier unstarted CLI status. Adaptive plus authorized source fallback uses
133,299 uncached input plus output tokens in 914.05 seconds; the fresh baseline
uses 165,839 in 600.30 seconds. Both cover 12/12 existing rubric criteria. That
is 19.6% fewer tokens under the stated metric, but 52.3% more elapsed time in
one sequential pair. Cached tokens are reported separately; no general cost or
repeatable efficacy claim follows.

Post-run verification passes all 80 control/integrity checks. All 374 prepared
source files and existing indices remain unchanged, and both installed binaries
match the authorized build. No rescan was needed. Joint evidence selection and
reducing the many fallback reads remain open; full answer quality is not
established from the bounded context pack alone.


## 2026-09-10 — adaptive latency correction

The [latency correction](../AGENT-LATENCY-FIX-2026-09-10.md) prioritizes source
evidence before supporting inventory, diversifies related candidates by file,
filters unrelated parent-domain candidates, prioritizes missing proof, and avoids
verification rereads. The canonical adaptive workflow now batches independent
reads and excludes tool/output-format policy from the retrieval query.

The current local development build is installed with backups. The full suite,
vet, 15 controls and 93 integrity/budget/range checks pass; all three strict JSON
controls remain unchanged. No workspace rescan was needed.

The user explicitly requested reusing the saved no-GoreGraph baseline instead of
rerunning it for each fix. The redundant active baseline was stopped and excluded.
Use `local-update-baseline-cli` (600.30 seconds, 165,839 uncached input plus output
tokens, 12/12 coverage) unless a new baseline is explicitly requested.

The first updated adaptive attempt completed: 888.70 seconds, 145,306 uncached
input plus output tokens, 24 commands and 12/12 coverage. A backend stream
interruption affects elapsed time; its duration is not estimated or subtracted.
The identical adaptive-only repeat also completed with a backend interruption:
903.10 seconds, 131,049 tokens, 36 commands and 12/12 coverage. Both attempts are
retained. Neither establishes a reliable speed advantage; the second attempt's
635.15 seconds before its last command already exceeds the full stored baseline.

The repeat exposed ambiguous retry instructions: the agent invented an
unsupported `--retry-anchor` option. The canonical adaptive guide now gives the
actual CLI form (`--query` with one supplied anchor and `--previous-context-id`)
and corresponding MCP arguments. The syntax control, instruction/context tests,
MCP tests and documentation tests pass. This final guide-only follow-up is
installed locally with backups; it was not part of the completed CLI prompts.
Final integrity checks confirm unchanged sources/indices and no required rescan.
The latency acceptance gate and complete-task evidence selection remain open.


## 2026-09-10 — adaptive evidence follow-up

The [planned follow-up](plans/2026-09-10-adaptive-evidence-followup.md) is implemented
and locally installed. Adaptive selection now anchors inferred models to the
terminal resource, preserves generic and qualified variants, and avoids elevating
unrelated read-contract payloads when concrete models exist. It reserves useful
follow-up metadata, matches verification priority to omission roles, skips fully
supplied budget omissions before the cap, and caps final output only after fit
checks. Explicit requests, primary dependencies and strict-v1 remain protected.

Final review is clean; the complete Go suite, vet, 17 fixed local queries and 117
integrity/budget/range checks pass. Installed build 1.4.1 d67d1f4ab3c3-dirty from
2026-09-10T15:18:44Z has backups at both existing locations. All 374 service files
and scan indices remain unchanged. No rescan or release was needed.

The [evidence report](../AGENT-EVIDENCE-FOLLOWUP-2026-09-10.md) retains preliminary
candidates and limits. One adaptive-only CLI analysis completed in 754.95 seconds
with 105403 uncached input plus output tokens and 12/12 coverage criteria; all 33
inventory paths/ranges are valid. Against the saved 600.30-second/165839-token
reference, tokens decrease 36.4% but elapsed time increases 25.8%. No no-GoreGraph
baseline was repeated. No stream interruption is recorded in the new run.
Post-run integrity checks pass. Ordinary source fallback and repeated model reads
remain; 250.99 seconds follow the last command response. The planned fixes are
complete, but the complete-task latency acceptance gate remains open.


## Adaptive scope evidence continuation — 2026-09-10

Plan: `plans/2026-09-10-adaptive-scope-evidence.md`; detailed evidence:
`../AGENT-SCOPE-EVIDENCE-2026-09-10.md`.

- Adaptive German evidence recognition and selected-project configuration navigation implemented, including raw-query/project collision protection and optional metadata budget trimming. Strict controls preserved.
- Full Go suite, vet, focused regressions and 18 local queries / 127 checks pass. Reviewed candidate 99cb8737 installed as 1.4.1 development build; frozen sources/indices unchanged, no rescan.
- Completed adaptive-only run: 895.69 seconds, 126234 uncached-input-plus-output tokens, 36 commands, 12/12 core rubric with inventory/wording caveats. Regression against prior adaptive 754.95 seconds / 105403 tokens. Saved no-GoreGraph 600.30 seconds / 165839 tokens reused; never rerun.
- Trace identifies malformed generated AWK printing complete files and large unscoped search output. Bounded-reader instruction-only follow-up installed and measured: 872.11 seconds / 105833 effective tokens / 21 calls / 200776 output bytes. Reader bug absent in this run, but at least 37 repeated lines and test inventory reduced to 3 files. Latency gate still failed. No commit, merge or release.


## Three-cycle completion — 2026-09-10

All three requested adaptive-only diagnose/fix/CLI cycles completed; no saved
no-GoreGraph baseline was rerun. See `../AGENT-THREE-CYCLE-2026-09-10.md`.
Round results: 414.11s/88443effective tokens, 442.45s/88411, 485.95s/108888.
Core12/12 in each; known test inventories5/7,4/7,5/7. The last result remains
19.0% faster and34.3% lower effective-token usage than the saved reference,
but it is worse than the first two adaptive measurements and still repeats source.

Adaptive correction-plan/provider navigation, guide refinement and primary-delete
test ranking are implemented; strict bytes/behavior and source/proof scopes remain
protected. Final full Go suite/vet,21controls/152assertions and whole-change review
pass. Final build1.4.1 d67d1f4ab3c3-dirty,2026-09-10T20:58:40Z,
SHAc9625c6a9873e78a1a1beebc5c570ecdaebad189aa6de0dea9799f0e125a0546 installed
with backups. All374service files/indices and15baseline files unchanged; no rescan,
service mutation/build/tests, commit, merge or release. Mail-test completeness,
source reuse and stable best-case efficiency remain open. The three requested
rounds are complete; the broader acceptance gate is not.

## Inventory and source reuse follow-up — 2026-09-11

Completed the approved five-step follow-up; see [full report](../AGENT-INVENTORY-REUSE-2026-09-11.md).
The final local 1.4.1 development candidate (2026-09-11T08:36:28Z, SHA256
`14d3c9701f6768f0e226a7dc35469a58c1ef39485129ee8f168d3be40d397cfd`)
passes full Go tests/vet, scoped reviews, 180 assertions across23 local queries
and receipt reuse for115 source sections. Both unchanged final CLI runs reach
core12/12 and held-out7/7. Elapsed/effective tokens:769.79143s/123863 and
605.247773s/109660. Token usage improves over the historical no-GoreGraph baseline;
stable elapsed-time improvement does not. Search outputs followed by source reads
remain the dominant duplicated output; caller-carried receipts cannot intercept
shell reads. Sources, indexes and all15 baseline files are unchanged. No baseline
rerun, rescan, service edit, release or commit. Existing dirty work remains local.

Final reports preserve EOF logical/physical line-count distinctions, run1's
incorrect unread labels, failed local candidates, and all scope/budget rulings.
This completes the implementation and evaluation plan, not a general efficacy gate.


## Combined source search/read follow-up — 2026-09-11

Completed the approved bounded reader extension, installation and two unchanged
CLI runs; see [report](../AGENT-COMBINED-READ-2026-09-11.md). Per-file redacted RE2
search returns bounded excerpts plus existing receipts and explicit match/paging
metadata. Strict behavior, source authority and all reader caps remain unchanged.
Full Go tests/vet, independent code review, 180 local controls, all 23 prior
context outputs, 115 receipt-reuse sections and 55 real-source find calls pass.
Installed 1.4.1 development candidate: 2026-09-11T10:17:13Z,
SHA `a67cac6a955fb9944acae0948e7c86b7d6c064ea929930876ec7d57ed3d0f92e`.

Both runs score core 12/12 and inventory 7/7; 544.162188s/125283 and
590.028448s/95784 effective tokens. Both are faster than the saved baseline,
although the second margin is small. No duplicated verified source output;
coverage and remaining caller/citation/terminology errors are documented.
All frozen sources, indexes, build sources and baseline files remain unchanged.
No rescan, baseline rerun, service mutation/tests, commit, merge or release.
The broader efficacy gate remains separate.

## CLI reader precision follow-up — 2026-09-11

Completed scoped normalization, adaptive evidence guidance, local installation
and two unchanged CLI runs; see [report](../AGENT-READER-PRECISION-2026-09-11.md).
Both historical caller-schema failures now produce exactly the old canonical
JSON and receipts. Canonical API, strict guide and read limits remain unchanged.
Full Go suite (146.97s), vet, independent spec/quality review, 180 controls across
23 unchanged context outputs, 115 reused sections and 55 find checks pass.

Installed 1.4.1 development candidate: 2026-09-11T11:05:25Z,
SHA a2938d66112ed00f66dd365ee97e2548c856100191d194f8df7c03b5a18bdc34.
CLI runs: 582.256123s/103649 and 680.44152s/110728 effective tokens.
Independent static core scores 12/12 and 11/12; semantic test inventory7/7 both,
but literal full test paths6/7 and1/7. Run2 fails criterion11 due32 abbreviated
table paths. All actual Java citation spans are delivered after unambiguous path
resolution;14 configuration spans show redacted keys only. Raw parser artifacts
remain intact. Verified repeated output0/64; caller errors and test-proposal
ambiguity remain. This is a mixed result, not general efficacy acceptance.

Rulings: preserve the dirty checkout and before snapshot; normalize only
unambiguous CLI shorthands into the unchanged API; keep general evidence guidance
free of private expected answers. Scope rollback would require the saved delta.
Both runs use identical build/guide/task/arguments. Frozen sources, indexes and
15 baseline files remain unchanged. No baseline rerun, rescan, service changes or
tests, commit, merge or release. Broader acceptance remains open.

## Answer validation and independent find batches — 2026-09-11

Completed implementation, independent review, local installation and diagnostic
plus confirmation; see [report](../AGENT-ANSWER-GUARD-2026-09-11.md).
Duplicate find selectors retain independent pagination and merge source once.
The new answer-check command validates caller-ledger identities/ranges and repairs
only unique paths. Semantic assessment remains explicitly separate.

Full Go tests/vet, targeted post-review regressions/race checks,180 controls,
115 receipt sections,55 find calls and three actual duplicate-request replays pass.
After diagnostic parser/extractor corrections,11 generic extractor tests and final
package tests pass. Retrieval/reader sources remain unchanged from the full-control
candidate; two final context checks are byte-identical.

Final installed1.4.1 development build2026-09-11T12:57:46Z,
SHA fc815c8e387031ecccbebd4a426bd414e704be324389a8c2255df177db6734e6.
Confirmation:627.602616s workflow/93047effective tokens,12/12core,7/7exact test
references, no material semantic error found. Mechanical validation has0findings,
0repairs; raw/final answer identical. One oversized request,22 repeated verified
lines and4.55%time overhead vs saved baseline remain; token saving43.89%.

Rulings: preserve dirty checkout with before/delta snapshots; keep mechanical and
semantic acceptance separate; reuse immutable baseline and measure finalization
explicitly. Rollback needs the scoped delta, semantic review remains necessary,
and changed diagnostic/final guide/checker versions are not an identical-run pair.
All374sources, indexes, build/installation and15baseline artifacts pass final
integrity. No scan, service mutation/tests, baseline rerun, commit or release.

## Reader auto-paging and confirmed answer check — 2026-09-11

Completed three adaptive CLI diagnose/fix cycles; see
[the report](../AGENT-READER-AUTOPAGE-2026-09-11.md). Automatic find-page reduction,
whole-file result deferral, narrow CLI JSON interoperability, and explicit local
multi-family rollback-test guidance are implemented. The final answer checker
uniquely resolves same-basename citations only when one candidate covers every
cited range.

Run1/2/3 raw results are730.252901s/105900,603.905148s/91191 and
505.816309s/79464 effective tokens. Run3 has14 commands, zero failures, no repeated
verified source rows, core12/12 and required test identities7/7. Against the saved
baseline it is15.74% faster with52.08% fewer effective tokens. The retained answer
passes the final checker with50 references,52 ranges,12 repairs and zero findings.

Full Go tests, vet, diff and integrity checks pass. Installed local1.4.1 candidate
2026-09-11T15:09:29Z, SHA0723dfdd2bb6a7704fffebb0da3e58745aa902a788f18ddbefe36043b2a8644f.
Frozen sources, indexes and baseline are unchanged; no rescan, baseline rerun,
service execution, commit or release.
