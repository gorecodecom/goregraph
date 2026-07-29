# Agent Benchmark Causal Stability Design

## Purpose

Stabilize the monotonic agent benchmark so it distinguishes GoreGraph product
regressions from answer-model variance. The benchmark must keep answer quality
strict while preventing cost-only fluctuations from triggering another
Context-ranking change when Golden and candidate produce the same semantic
Context Pack.

## Evidence behind the change

The latest completed matrix exposed two measurement defects:

- G3 asked the answer reviewer to state an internal serialization rule:
  generic Go routes are represented in `ContextPack.Entrypoints`, not
  `ContextPack.Endpoints`. Every Golden and candidate answer omitted that
  implementation detail even though the packs were correct.
- G2 showed a large candidate token and tool-call increase while its semantic
  Pack Diff was empty. The other unchanged generic packs produced both positive
  and negative cost movement. These differences cannot be attributed to a
  GoreGraph Context Pack change.

The current production candidate remains unchanged by this work. This design
changes only benchmark contracts, evaluation, tests, and benchmark
documentation.

## Considered approaches

### Documentation only

Document that reviewers should ignore G3's internal representation statement
and manually classify unchanged-pack efficiency differences. This has the
smallest code footprint, but it leaves the same interpretation error possible
in every future run.

### Increase the number of external runs

Use more repetitions to reduce median variance. This raises cost and duration
without fixing the category error: an answer rubric still cannot prove an
internal Pack invariant, and a cost difference still does not establish a
GoreGraph cause when the semantic Pack Diff is empty.

### Encode causal classification in the benchmark

Move the G3 invariant into the deterministic Pack contract and pass the
recorded semantic Pack Diff into the case gate. Keep answer-quality failures
hard in all cases. Apply comparative efficiency thresholds only when the Pack
Diff contains a semantic change; otherwise retain threshold breaches as
explicit model-variance observations.

This is the selected approach because it makes the intended interpretation
machine-enforced without weakening gates for candidates that actually change a
Context Pack.

## Design

### Deterministic G3 Pack invariant

`PackExpectation` gains an optional `max_endpoints` integer. When present it
must be non-negative, and `EvaluatePack` fails if the generated pack contains
more endpoints than allowed.

G3 sets `max_endpoints` to `0`. Its existing required entrypoint matcher
continues to require `DELETE /orders/{orderId}`, so the pair of assertions
proves the intended representation:

- the generic Go route exists in `Entrypoints`;
- the pack publishes no structured `Endpoints` record.

The `structured-go-endpoint-record` answer facet is removed. Reviewers are no
longer asked to repeat an internal representation detail in a user-facing
answer.

### Causal efficiency assessment

The case gate consumes the already recorded `PackDiff`.

A semantic Pack Diff is unchanged when all change flags are false and all
added/removed semantic collections are empty. The unchanged decision ignores
the recorded Golden and candidate retry values when those values are equal.

`GateReport` gains an `observations` list:

- Answer facets, forbidden outcomes, uncertainty disclosures, review
  integrity, run validity, and metric validity remain hard failures.
- Candidate bounded-read limits remain hard per-run safety failures.
- When the Pack Diff changed, comparative tool-call, unauthorized-read, token,
  median-latency, and paired-latency thresholds remain hard failures.
- When the Pack Diff is unchanged, comparative efficiency threshold breaches
  are retained as observations prefixed with an unchanged-pack/model-variance
  explanation. They do not make the GoreGraph product gate fail.

This distinction prevents an unchanged Context Pack from causing ranking work
while preserving the raw evidence needed to diagnose model or environment
variance.

### Command contract

The internal benchmark `gate` command requires an absolute `--pack-diff` path.
It reads the runner's existing `pack-diff.json` artifact strictly and passes it
to the evaluator. The gate report writes observations deterministically and
keeps failures sorted.

This is intentionally a benchmark-tool contract change, not a public GoreGraph
CLI change.

## Error handling

- A missing, relative, malformed, or unknown-field Pack Diff is a command
  error and no gate report is published.
- A negative `max_endpoints` contract value is rejected during contract
  validation.
- An unchanged Pack Diff never hides invalid metrics or bounded omission reads
  above the contract limit.
- Existing output files remain protected by the current no-overwrite behavior.

## Testing

Use test-driven development:

1. Add contract and pack-evaluation tests for `max_endpoints`, verify they fail,
   then add the minimal contract implementation.
2. Add evaluator tests proving unchanged Pack Diffs convert comparative
   efficiency breaches into observations while changed Pack Diffs still fail.
3. Add command tests proving `--pack-diff` is required and malformed input is
   rejected.
4. Update G3's committed contract and run the deterministic benchmark matrix.
5. Run focused package tests, the fake regression harness, formatting checks,
   and the full Go suite.

No private workspace or external Codex run is required for implementation
verification. Existing benchmark evidence is not rescored retroactively.

## Success criteria

- G3's internal endpoint representation is enforced by deterministic Pack
  evaluation and absent from the answer rubric.
- A changed semantic Pack Diff retains all existing comparative efficiency
  gates.
- An unchanged semantic Pack Diff retains comparative efficiency breaches as
  model-variance observations without reporting a product regression.
- Answer quality and bounded-read safety remain strict in both paths.
- Benchmark documentation describes the causal rule and contract-freeze
  boundary for future runs.
- The full local test suite passes, and no release or merge to `main` occurs.
