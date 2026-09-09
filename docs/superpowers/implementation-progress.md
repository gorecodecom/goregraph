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
