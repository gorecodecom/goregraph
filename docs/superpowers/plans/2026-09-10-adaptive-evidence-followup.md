# Adaptive Evidence Follow-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Reduce broad source fallback by excluding unrelated inferred models and preserving useful bounded follow-up evidence in adaptive-v2.

**Architecture:** Keep the deterministic context compiler and its existing source-proof checks. Correct inferred candidate eligibility before expansion, and reserve response space for missing evidence that is renderable but cannot fit. Preserve the existing protocol and caller permissions.

**Tech Stack:** Go standard library, existing Go tests and local Codex CLI benchmark.

**Spec:** `docs/AGENT-LATENCY-FIX-2026-09-10.md`, plus the user-authorized improvement sequence in this task.

## Global Constraints

- Reuse `local-update-baseline-cli`: 600.300493 seconds, 165839 uncached input plus output tokens, 12/12 coverage. Never rerun the no-GoreGraph baseline without a new explicit request.
- Preserve strict-v1 outputs and instruction text, source budgets, aggregate file limits, the three-request verification limit, safe paths and current-source proof requirements.
- Do not infer runtime calls or database cascades from names. Do not hard-code service names, routes or the private benchmark answer into production logic.
- Preserve all pre-existing uncommitted work. Work on `fix/adaptive-evidence-followup` in the current checkout, continuing the user-authorized fixes; do not relocate the dirty project or commit unrelated work.
- No dependencies, test-service edits, service builds, release, merge or push. Local GoreGraph installation and read-only CLI analysis are already authorized.
- Distinguish functional correctness, final answer coverage and measured performance. Retain failed attempts and backend interruptions; do not subtract guessed delay.

## Task 1: Anchor inferred model candidates to the actual resource

**Files:** `internal/agent/context_related_models.go`; new `internal/agent/context_inferred_models_test.go`.

**Interfaces:** Keep `planContextSourceConcerns(query, index, seed, protocol)` unchanged. It produces candidate IDs for existing source-proof and related-model expansion.

- [x] Build a synthetic route `/portfolios/{portfolioId}/invoices/{invoiceId}` with `PortfolioInvoiceEntryEntity`, its change variant, `PortfolioCacheEntity`, and `PortfolioProtocolEntity` in separate projects. Include a generic request for models, configuration, tests and protocol side effects. Assert the first two are selected as inferred target models and the unrelated parent-only models are not. Explicitly named additional models must remain eligible.
- [x] Run the focused test and record the expected failure before changing production.
- [x] Apply adaptive-only filtering using the terminal resource identity rather than accepting a match on any shared parent or generic evidence word. Reuse the existing declaration/name token helpers; retain ordinary single-resource routes and related variants. Keep explicit caller requests and strict-v1 behavior intact.
- [x] Run the new tests and existing related-model tests, including unrelated sibling clients and singular/plural routes. Check that no runtime edges are created.

## Task 2: Preserve and prioritize actionable follow-up ranges

**Files:** `internal/agent/context_select.go`, `internal/agent/context_verification.go`; new `internal/agent/context_followup_test.go`.

**Interfaces:** Keep `ContextVerificationRequest` and public CLI/MCP schemas unchanged. Source selection consumes the same concern/options lists and emits at most three existing, bounded, unseen verification ranges.

- [x] Add regressions proving that a partial adaptive pack under a full response budget retains bounded requests for existing, renderable evidence excluded by that budget; checking primary-body preservation and the byte/token/file caps is part of the fixture.
- [x] Add a regression where an uncovered contract concern must not promote alphabetically earlier domain-model omissions. Preserve the evidence compiler's priority among equal-priority adaptive omissions. Keep legacy ordering in non-adaptive direct helper calls.
- [x] Run these tests and record real assertion failures.
- [x] Reserve space for a bounded useful set of potential adaptive follow-up omissions before filling source sections. Account for both omission and verification serialization. Avoid reserving all possible concerns and starving the primary path. Match verification priority to the omission's role instead of promoting every omission in a project with an uncovered concern.
- [x] Run focused selection/verification tests and all `internal/agent` tests. Review deterministic ordering, unreadable paths, overlapping ranges and minimum-budget fallbacks.

## Task 3: Integrate, review, install and measure

**Files:** This plan; `docs/superpowers/implementation-progress.md`; new `docs/AGENT-EVIDENCE-FOLLOWUP-2026-09-10.md`; raw private artifacts outside Git.

- [x] Review each bounded patch against its task and the combined compiler flow. Resolve actionable findings and retain the review record.
- [x] Build a frozen 1.4.1 development candidate. Replay the existing 15 fixed local queries and both recorded CLI entry queries, preserving all earlier artifacts. Check strict controls, relevant target variants, primary mutation bodies, useful follow-up ranges, budgets and source/index integrity.
- [x] Run `go test ./... -timeout 20m`, `go vet ./...` and `git diff --check` once after the integrated fixes; investigate failures before proceeding.
- [x] Install the verified candidate at both existing binary paths with backups. Rescan only if current-source/index verification demonstrates a need.
- [x] Run one read-only adaptive Codex CLI diagnosis with the canonical instruction and unchanged base task. Use the existing private-source/backend consent. Compare to the saved baseline; score the same 12 criteria and validate citations. Do not relaunch the baseline.
- [x] Record all measured time/token values and limits; mark a performance improvement only if observed, and do not equate passing unit tests with a faster complete task.

## Execution record

Ruling: Continue in the existing checkout on a new fix branch because the user asked to improve the currently installed dirty implementation. Relocating it would risk omitting prior fixes. No pre-existing changes are reset or committed.

Pre-flight: Task 1 produces candidate IDs consumed by Task 2 through unchanged existing interfaces. The tasks own separate production and test files. Task 3 consumes both and changes documentation/artifacts only. No task changes the frozen private services or stored reference.

Ruling after task review: Whole-path maximum overlap was too aggressive: it dropped a valid `InvoiceEntity` when `PortfolioInvoiceEntity` existed and could admit parent-only cache models on ties. Require terminal-resource identity instead; keep other route tokens only to reject ambiguous abbreviations. Explicit named models and downstream caps remain unchanged.

Ruling after integrated replay: Filter wholly supplied budget omissions before the omission cap, not only when producing verification requests. Otherwise an already supplied controller line consumes a useful slot. Keep operational failures and pathless uncertainty visible.

Ruling after task review: Reserving the first three largest omissions can miss a later affordable range. Admit candidates only when their combined metadata fits beside the preserved primary bodies, and continue scanning after rejected candidates.

Task1 refinement from actual replay: unrelated read-contract payload models must not be elevated to required inferred models merely to fill project slots. Concrete resource models take priority; explicit named/type requests and existing primary-path dependencies are preserved. Regression tests and final review pass.

Implementation and installation complete: 1.4.1 d67d1f4ab3c3-dirty, built 2026-09-10T15:18:44Z, SHA256 c7e1ae18c590159ed2e8a99530cdfef409c2e07e720d7af0fb1b5287a276ec6f. Full suite, vet, 17 controls and 117 checks pass, including post-run integrity verification.

Measurement and audit complete: one adaptive run, exit 0, 754.945442 seconds, 105403 uncached input plus output tokens, 12/12 coverage criteria, all 33 inventory paths/ranges valid. Saved no-GoreGraph baseline unchanged and reused. Tokens decrease 36.4%; elapsed time increases 25.8% against that reference. No recorded stream interruption. Remaining ordinary fallback, repeated model reads and 250.99 seconds after the last command response prevent claiming the latency problem solved. See the evidence report for measurement limits and remaining work.
