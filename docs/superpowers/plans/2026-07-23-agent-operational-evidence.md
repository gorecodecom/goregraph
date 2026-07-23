# Agent Operational Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve source-backed Context Packs so generated accessors resolve to current source, rendered operational annotations count as evidence, and actionable cross-cutting source beats decorative class signatures.

**Architecture:** Keep the existing deterministic concern planning and bounded source-option pipeline. Extend source verification with a conservative Java accessor-to-field fallback, enrich option coverage from verified rendered content, and refine concern/source quality scoring without adding dependencies or benchmark-specific names.

**Tech Stack:** Go, standard library, table-driven unit tests, existing `internal/agent` Context Pack fixtures.

## Global Constraints

- Preserve deterministic ordering and the existing token, byte, file, section, and omission limits.
- Accept an accessor fallback only when one unique declaration-like backing field exists in current Java source.
- Count a concern as covered only from verified source in the same scoped project.
- Do not hard-code benchmark repository, class, method, route, or property names.
- Prefer production method bodies over type-only signatures for authentication, configuration, resilience, and side-effect evidence.
- Keep all changes on `main`, as explicitly authorized after the pre-change push.

---

## Task 1: Resolve generated Java accessors to backing fields

**Files:**

- Modify: `internal/agent/context_source.go`
- Test: `internal/agent/context_source_test.go`

- [ ] Add a failing renderer test with a Lombok-style boolean accessor fact whose only current-source representation is a uniquely declared field.
- [ ] Run `go test ./internal/agent -run TestRenderSourceCandidateRelocatesGeneratedAccessorToBackingField -count=1` and confirm the expected “indexed symbol is absent” failure.
- [ ] Add conservative Java getter/setter name derivation and field-declaration verification.
- [ ] Reuse the existing `relocated_current` state and normal render-mode validation for the verified field.
- [ ] Add negative tests for ambiguous fields and non-Java source.
- [ ] Run the focused renderer tests and confirm they pass.
- [ ] Commit the completed task with an English imperative commit message.

## Task 2: Derive coverage from verified rendered evidence

**Files:**

- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_source_test.go`

- [ ] Add failing option-coverage tests showing that a rendered `@Retryable` method covers resilience and that evidence from a different project does not cover a scoped concern.
- [ ] Run the focused tests and confirm the expected failures.
- [ ] Extend source-option concern attribution with conservative source markers for authentication, configuration, resilience, persistence, side effects, and tests.
- [ ] Require project equality for project-scoped semantic coverage and exclude test-only sections from production concerns.
- [ ] Preserve Fact-ID attribution as the primary coverage mechanism and use rendered semantics only as verified augmentation.
- [ ] Run the focused coverage tests and existing context source tests.
- [ ] Commit the completed task with an English imperative commit message.

## Task 3: Prefer operational cross-cutting evidence

**Files:**

- Modify: `internal/agent/context_intent.go`
- Modify: `internal/agent/context_select.go`
- Test: `internal/agent/context_change_analysis_test.go`
- Test: `internal/agent/context_source_test.go`

- [ ] Add failing selection tests where a domain-specific configuration holder must outrank an unrelated application accessor.
- [ ] Add a failing quality test where a deletion method body must outrank a mail-service class signature for side-effect analysis.
- [ ] Run both focused tests and confirm the intended ranking failures.
- [ ] Refine scoped concern ranking using stable domain identity, concern-specific type shape, action alignment, and production-source quality.
- [ ] Penalize signature-only cross-cutting evidence even when its class name matches the domain.
- [ ] Permit same-domain, same-action production methods as side-effect candidates when the task explicitly requests side effects.
- [ ] Extend the missing-contract integration fixture to assert operational configuration, resilience, and side-effect source content rather than type names alone.
- [ ] Run all `internal/agent` tests and confirm the bounded deterministic pack remains within limits.
- [ ] Commit the completed task with an English imperative commit message.

## Task 4: Verify the full product and benchmark behavior

**Files:**

- Verify: `internal/agent`
- Verify: `scripts/analyze-agent-context-log_test.sh`
- Verify: `scripts/benchmark-agent-context_test.sh`
- Verify: benchmark workspace `.goregraph-workspace/agent/context-index.json`

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `go vet ./...`.
- [ ] Run `bash scripts/analyze-agent-context-log_test.sh`.
- [ ] Run `bash scripts/benchmark-agent-context_test.sh`.
- [ ] Run `git diff --check`.
- [ ] Install the verified CLI locally with the repository’s documented install target.
- [ ] Generate the same one-shot 4,000-token benchmark Context Pack from the existing prepared index.
- [ ] Confirm the generated accessor omission is gone, rendered retry evidence is covered, and selected cross-cutting sections contain actionable source.
- [ ] Record before/after section quality, source coverage, context time, and token use in the completion report.
- [ ] Commit any final test-only adjustments separately; do not release.
