# Three-Cycle Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute each scoped fix with review before its benchmark. Track steps with checkboxes.

**Goal:** Complete three successive diagnose/fix/adaptive-benchmark cycles, stopping early only when the scenario's quality, completeness, reuse, latency and token gates all pass.

**Architecture:** Keep adaptive evidence intent separate from raw domain identity and from proof of missing runtime transitions. Reuse already selected exact evidence for navigation. Each round consumes the prior measured trace, freezes a distinct candidate, and compares one unchanged-task CLI run with the preserved baseline.

**Tech Stack:** Go standard library, synthetic Go fixtures, existing 20-query local replay, Codex CLI.

**Spec:** `docs/AGENT-THREE-CYCLE-2026-09-10.md` and the user's explicit instruction to repeat diagnosis, fixes and testing three times.

## Global Constraints

- Continue `fix/adaptive-evidence-followup` in the existing dirty checkout. Preserve all previous changes; no commit, push, merge or release.
- Never repeat or rewrite `local-update-baseline-cli`. Its 15 files are hash-guarded.
- Keep the base CLI task, model, settings, private frozen sources and indices unchanged. The same consent covers local installation and private source/test analysis by the OpenAI Codex backend.
- Preserve strict-v1 guide and behavior, original public query, explicit project scopes, source-proof semantics, source/file/token/byte limits and the three-verification-range cap.
- No private-domain aliases, new dependencies, generated runtime edges or service edits.
- Keep failed candidates/attempts. A passing unit test or shorter output is not a passed end-to-end gate.

### Task 1: Adaptive correction-plan evidence inventories

**Files:**
- Modify `internal/agent/context_evidence_query.go` for bounded generic correction-plan and retry evidence vocabulary.
- Modify `internal/agent/context_plan_files.go` and `internal/agent/context_production_plan_files.go` for adaptive inventory eligibility and selected-provider fallback.
- Create `internal/agent/context_change_plan_inventory_test.go`; extend `context_evidence_query_test.go` only where shared intent behavior needs coverage.
- Add a small `internal/agent/context_change_plan_inventory.go` only if shared helpers keep the selectors clearer.

**Interfaces:** Keep public structs and JSON fields unchanged. Use `contextEvidenceQueryForProtocol(query, protocol)` only for evidence intent. Preserve `contextQueryPlansMissingTransition(rawQuery)` everywhere it controls proof/current-vs-future semantics. A new internal inventory eligibility helper may allow an explicit correction plan in adaptive-v2 without claiming that its missing transition is known. Provider helpers may accept the index to reuse `contextEvidenceProjectRoles(pack,index)` when their old concern-derived provider set is empty.

**Diagnosis:** Both selectors currently return before budget selection: the exact inventory recognizer misses correction-plan wording, the missing-transition gate requires stronger wording than a requested correction plan, and the provider set ignores already selected model evidence. The latest German query also loses the resilience concern for `Wiederholungsverhalten`.

- [x] Add synthetic regressions based on `runtimeShapeReleaseQualityMissingContractIndex` / `writeReleaseQualityMissingContractFixtureWithIndex` and existing plan-file tests. Use ordinary example-domain nouns, never private names. Exercise a German correction-plan query with production/configuration/test files and internal interfaces, an English correction/fix-plan equivalent, and `Wiederholungsverhalten`. Assert nonempty relevant provider/test inventory and resilience concern through actual planning/BuildContext, unchanged public query and raw missing-transition semantics.
- [x] Add negative controls: strict-v1 unchanged; ordinary diagnosis without a correction/missing-transition request does not acquire plan inventories; unrelated selected projects and explicit caller-only scope do not supply provider files; ambiguous entrypoint remains rejected. Use real facts/edges and output values, not mocks or text-only snapshots.
- [x] Run the focused tests first and retain the expected assertion failures. Existing applicable fixtures are in `context_plan_files_test.go`, `context_production_plan_files_test.go`, `context_configuration_navigation_test.go`, and `context_evidence_query_test.go`.
- [x] Implement bounded correction-plan recognition. For example, `Korrekturplan` or the phrase `fix plan` may add `change` to the private evidence query so exact inventory intent is recognized; `Wiederholungsverhalten` may add `retry`. Do not append `missing` or `new call chain`, because requesting a fix does not prove a missing edge.
- [x] In the two inventory selectors, preserve their old eligibility first; adaptive-v2 may additionally accept an explicit correction plan with existing exact inventory/test requirements. Keep raw project/seed/domain selection unchanged.
- [x] For an empty concern-derived provider set, adaptive-v2 may use already selected exact contract/model projects from `contextEvidenceProjectRoles`, excluding the entrypoint and respecting raw explicit scope. Do not scan all index projects or include projects merely because they have config/call-chain evidence. Selected EXACT management contract-shaped source facts also qualify when the compact pack omits Contracts; unselected/nonexact facts do not. Retain existing path-confidence, relevance, represented-path and count filters.
- [x] Test deterministic bounded output under a saturated budget and preservation of primary mutation evidence. Reuse the final budget fitter; inspect isolated replay time before accepting any additional source-selection loop. Resolve a measured regression within this round before installation.
- [x] Run focused Go tests, gofmt/diff checks and review this round's source delta against its before snapshot. Do not change canonical guide in this task.
- [x] Freeze candidate, run full Go checks and 20 isolated local controls, install with backups, verify frozen sources/indices/baseline hashes, and run one adaptive-only CLI benchmark. Score core12 separately from inventory completeness, cited ranges and repeated reads. Record round1 outcome.

### Task 2: Second measured feedback cycle

Concrete scoped implementation: adaptive-only inventory reporting and identity/concern-bounded fallback after discovery. Requirements and measured trigger: `.superpowers/sdd/2026-09-10-three-cycle-evidence/task-2-brief.md`. Own `internal/agentguide/instruction.go`; strict bytes and retrieval logic remain unchanged. Missing side-effect test references were already read, so prioritize retaining relevant evidence in the final inventory over adding more source output.

These are intentionally feedback steps, not speculative future patches. Each
begins only after the preceding CLI answer and trace have been audited.

- [x] Round2: Select the highest-impact remaining reproducible defect from round1. Write its exact scoped interface/test/fix brief in this plan before dispatch. Implement, review, validate, freeze, install and run the same adaptive-only comparison; retain all artifacts.
### Task 3: Third measured feedback cycle

Concrete scoped implementation: preserve primary-action test ranking for adaptive correction plans with exactly one selected DELETE endpoint; existing raw missing-transition scoring and strict semantics remain unchanged. Requirements: `.superpowers/sdd/2026-09-10-three-cycle-evidence/task-3-brief.md`. Add the completed round2 initial query as one additional local control (20fixed+1new).

- [x] Round3: Apply the same selection and execution contract to round2's remaining defect. Complete the third comparison unless every early-stop gate already passed; retain all artifacts.

The selection order is correctness/scope violation, missing requested evidence
or file inventory, avoidable source rereads/output, then measured local retrieval
cost. Separate post-tool model time from retrieval time. If a failed outcome is
not explained by a safe reproducible code defect, record that fact and verify
variance in the next authorized run rather than invent a patch. The next task's
brief supplies concrete code/test details only after its input evidence exists.
