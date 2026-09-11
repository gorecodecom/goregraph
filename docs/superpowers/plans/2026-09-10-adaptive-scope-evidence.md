# Adaptive Scope Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Track steps with checkboxes.

**Goal:** Recognize requested evidence in ordinary German compound phrases and expose relevant configuration identities without requiring the caller to know project names.

**Architecture:** Normalize evidence vocabulary only inside adaptive query planning; preserve the original public query and domain identifiers. Configuration resource selection may use already selected projects when no explicit project was requested. Retain source-proof requirements, existing public fields and response limits.

**Tech Stack:** Go standard library, synthetic Go fixtures, fixed local retrieval controls and one authorized adaptive Codex CLI run.

**Spec:** The remaining work in `docs/AGENT-EVIDENCE-FOLLOWUP-2026-09-10.md` and the user's instruction to continue. The saved command audit is outside Git in `scope-evidence-round/command-audit.json` under the existing Mac benchmark artifact directory.

## Global Constraints

- Continue the existing dirty checkout on `fix/adaptive-evidence-followup`; preserve all earlier work. No commit, merge, push or release is part of this round.
- Never rerun the no-GoreGraph reference: reuse `local-update-baseline-cli`, 600.300493 seconds, 165839 uncached input plus output tokens and 12/12 coverage.
- Preserve strict-v1 behavior and instruction text, response token/byte/file limits, safe source paths, the three-request verification cap and explicit project scopes.
- No private-domain aliases, inferred runtime calls, new dependencies, schema changes or private service source edits.
- Local installation and read-only CLI analysis, including private sources sent to OpenAI Codex, are already authorized. Rescan only if source/index integrity requires it.
- Keep failed attempts and all measurement limits. A context fix is not proof of a faster complete task.

## Diagnosis

The prior 57 commands comprise one context call, three bounded verification reads
and 53 ordinary fallback calls. Configuration appears in 14 calls, persistence
in 11 and tests in six (categories overlap). The first pack lacks authentication,
configuration and test concerns despite the query requesting them. Known noun
aliases do not match German linking forms or `Testmechanismen`. Configuration
resource metadata additionally requires exact inventory intent and explicit
project names. Change-plan inventories are deliberately gated to change plans;
this diagnosis query should not silently become one.

## Task 1: Adaptive evidence vocabulary

**Files:** New `internal/agent/context_evidence_query.go` and test file; narrow call-site changes in `context.go`, `context_rank.go`, and `context_related_models.go` only as needed.

**Interfaces:** Add `contextEvidenceQueryForProtocol(query, protocol string) string` and keep `contextSelectionQuery` raw. Initial and later concern planning must receive separate raw and evidence queries through `planContextConcernsWithEvidenceQuery`; evidence-only gates may use expanded vocabulary. Project, seed, domain ranking and source search continue using the original problem statement.

- [x] Add synthetic tests for the phrase `Authentifizierungs-, Konfigurations-, Fehler-/Retry- und Testmechanismen`, its ordinary full-noun and English equivalents, explicit source-path requests and unrelated words. Assert authentication/configuration/resilience/tests concerns through real planning, not only a string snapshot. Assert strict-v1 and the public query remain unchanged.
- [x] Run focused tests and record the expected missing-concern failure.
- [x] Implement bounded whole-token evidence aliases, using known terms rather than unrestricted substring matching or global German stemming. Make normalization deterministic and idempotent. For exact source-path wording, recognize file identity without implying a missing transition or inventing production ownership.

```go
query := contextEvidenceQueryForProtocol(contextSelectionQuery(pack), pack.ProtocolVersion)
// Return the input unchanged for strict-v1; only append recognized evidence
// vocabulary for adaptive planning. Do not translate domain terms.
```

- [x] Run focused planning/source tests and review the scoped patch.

## Task 2: Configuration navigation and reuse

**Files:** `internal/agent/context_configuration_resources.go`, new focused configuration tests, and `internal/agentguide/instruction.go` for the fallback guidance.

**Interfaces:** Preserve `contextConfigurationResources(pack ContextPack, index scan.AgentContextIndexRecord) []ContextConfigurationResourceGroup`. Use Task 1's existing `contextSelectionQuery` result.

- [x] Add a synthetic adaptive fixture with no project names in the query, a selected project, exact application/profile configuration facts and an unrelated project. Assert selected configuration paths/key groups appear, unrelated project and values do not, explicit scope wins, strict behavior stays unchanged and final budget limits hold.
- [x] Run and retain the failing result.
- [x] When explicit projects are absent and the protocol is adaptive, derive eligible projects only from existing selected evidence/concerns. Retain exact-confidence, key relevance, represented-path and six-resource filters. Do not broaden change-plan inventories.
- [x] Before repeating full source selection for an over-budget final pack, trim newly inferred adaptive configuration navigation to the available space. Preserve primary source and existing explicit-project/strict behavior. Add a saturated fixture showing retained source and a useful metadata subset when affordable; omit optional navigation when no subset fits.

```go
if len(explicitProjects) == 0 && pack.ProtocolVersion == AdaptiveV2 {
    // Collect normalized nonempty projects from selected entrypoint/source
    // evidence and scoped concerns, never from every index fact.
}
```

- [x] Clarify authorized fallback guidance: use supplied identities as navigation, distinguish metadata from read evidence, subtract already read line ranges, and reuse a recorded negative search unless new evidence changes its scope. Preserve permissions, stale-source verification and the retry limit. Validate prose through the subsequent CLI trace, not a text-only test.
- [x] Run focused tests and obtain combined review.

## Task 3: Replay, install and measure

**Files:** This plan, a new evidence report, implementation progress and private artifacts outside Git.

- [x] Replay the prior 17 local queries plus the actual latest CLI query with a frozen development candidate. Check recognized scope, relevant evidence, unchanged strict controls, core mutation bodies and budgets. Inspect whether configuration metadata fits rather than assuming it does.
- [x] Run the full Go suite and vet after integrated fixes, formatting and diff checks. Resolve review findings.
- [x] Install the verified candidate at both existing paths with backups; verify source/index hashes and skip rescan if unchanged.
- [x] Run one adaptive-only CLI analysis with unchanged base task/model/settings and current canonical guide. Reuse the stored baseline. Audit the same 12 criteria, source citations, missing-evidence searches and repeated reads.
- [x] Record metrics and limitations, and report whether the latency gate actually passed. Preserve the checkout and artifacts.

## Follow-up: bounded reader instructions

The completed run exposed a generated AWK rule/action error and excessive broad
search output. A single instruction-only follow-up isolates this observation.

- [x] Replace vague adaptive batching guidance with independently bounded readers and a tested example; use scoped filename discovery before source excerpts.
- [x] Verify synthetic exact ranges, unchanged strict guide and existing instruction consumers; obtain scoped review.
- [x] Freeze and install with backups; preserve retrieval code, settings, frozen services and indices.
- [x] Run one additional adaptive-only analysis; retain all earlier results and audit output breadth, coverage, time and usage.
