# Monotonic Agent Context Improvement Design

**Status:** Approved on 2026-07-25

## Goal

Replace single-run tuning with a monotonic evaluation system that can improve
GoreGraph Agent Context accuracy without losing any capability demonstrated by
the current best run.

## Frozen Baseline

- Commit `1bc4408` is the initial Golden Build.
- The external Weka workspace snapshot and its best transcript form Golden Case
  G1.
- G1 is represented by required, forbidden, and unknown evidence properties,
  not by exact answer wording.
- Known gaps remain improvement targets and are not normalized as desired
  behavior.

## Benchmark Matrix

- G1: current external cross-service deletion case
- G2: synthetic Java/Spring missing-contract and competing-endpoint case
- G3: synthetic Go existing-flow case
- G4: synthetic TypeScript frontend-to-backend contract case
- G5: synthetic persistence and side-effect case
- G6: synthetic ambiguity, fallback, and budget-pressure case

Synthetic cases use generic committed fixtures. Relevant cases have English and
German query variants that must select equivalent evidence.

## Quality Contracts

Each case defines required evidence, forbidden claims, explicit unknowns,
expected entrypoint and path, permitted source reads, and efficiency limits.
G1 preserves the correct endpoint, Oracle deletion root cause, both task types,
the direct task deletion implementation and side effects, targeted omissions,
and bounded navigation. It forbids the prior create-path and broad-search
regressions.

## Evaluation Architecture

The system separates deterministic Context Pack verification from stochastic
agent evaluation. Golden and candidate binaries scan isolated workspace copies.
A semantic Pack Diff reports changed facts, sources, paths, coverage, omissions,
budget, and selection reasons.

## Staged Execution

1. Deterministic fixture and Context Pack gates
2. Paired smoke runs for G1 and the targeted case
3. Interleaved three-by-three Golden-versus-candidate runs for all six cases
4. Existing no-GoreGraph release comparison only after candidate acceptance

Every run records binary, commit, index, prompt, model, Codex, configuration,
and execution-order identities.

## Acceptance Gates

- Every previously correct G1 facet must pass in every candidate run.
- No candidate run may introduce a forbidden or unsupported claim.
- No new pathless omission, wrong entrypoint, broad search, included-source
  reread, or unapproved Context retry is allowed.
- The declared target must improve deterministically and in at least two of
  three end-to-end runs.
- Context remains within 4,000 estimated tokens.
- Median source reads and tool calls do not increase.
- Median end-to-end tokens may increase by at most 5 percent.
- Median Context latency may increase by at most 10 percent, with no unexplained
  paired outlier above 2x.

Thresholds are fixed before execution. Semantic failures are retained and may
not be replaced by additional runs.

## Change Isolation and Failure Handling

Each candidate tests one written hypothesis on an isolated branch. A failing
generic regression is added first, followed by the smallest production change.
Failures are classified as scanner truth, intent, ranking, budget, rendering,
or agent behavior before another hypothesis begins.

A run is invalid only for an outcome-independent infrastructure failure such as
a process crash, unavailable tool, or identity mismatch. Invalid logs remain in
the evidence set. Slow or inaccurate runs are valid failures.

## Promotion and Rollback

`main` remains on the Golden Build until every gate passes. An accepted
candidate becomes the next Golden Build only after the complete matrix passes;
the preceding build, inputs, transcripts, and score contracts remain
reproducible. Failed candidates are discarded without relaxing fixtures,
quality properties, or thresholds.

## Non-Goals

- No release or tag
- No proprietary identifiers in production rules or committed fixtures
- No exact transcript matching
- No multi-axis tuning in one candidate
- No replacement of the independent release benchmark
