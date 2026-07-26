# Connected Agent Context Retry Design

**Status:** Approved on 2026-07-26

## Goal

Prevent an Agent Context retry when its only anchor is a disconnected
operational path that cannot extend the selected production flow, while
preserving retries for genuinely missing bounded evidence.

## Evidence

The G1 smoke candidate returned a complete enough first Context Pack but also
offered `DELETE /cadastertaskmgmt/markanddeletetasks` as a retry anchor. That
endpoint is an adjacent housekeeping operation. It is not connected to the
selected regulation-deletion flow and cannot establish the missing requested
transition.

The current retry selector already requires an uncovered concern, a concrete
unselected fact, action compatibility, source evidence, and—when omissions
exist—a matching omitted file. It does not distinguish a disconnected
`call_chain` from an extension of the selected primary path.

## Evaluation Preparation

The monotonic evaluator currently rejects every change to `retry_allowed`.
Before changing production behavior, extend the hypothesis schema with
`retry_permission` as an allowed change category.

This permission is directional:

- a declared `true` to `false` change is allowed;
- an undeclared retry change is rejected;
- a `false` to `true` change is always rejected;
- no other protected or undeclared Pack change is relaxed.

The semantic Pack Diff records the Golden and candidate retry values so the
direction is explicit in retained evidence.

## Production Hypothesis

Compute the facts reachable from the selected planning seed using the same
directed production graph used by concern planning.

When a retry candidate matches a bounded source omission whose role includes
`call_chain`, retain it only if the candidate fact is reachable from the
selected seed. A same-action endpoint in a disconnected adjacent path is not a
valid continuation.

The reachability rule applies only to `call_chain` omissions. Existing bounded
retries for configuration, authentication, resilience, persistence, tests, or
other support evidence retain their current eligibility rules.

## Acceptance

The deterministic candidate must:

- return `retry_allowed: false` and no retry anchors for G1;
- preserve the selected endpoint, entrypoints, call chain, contracts,
  persistence, sources, omissions, uncertainties, and token budget except for
  changes explicitly declared by the hypothesis;
- keep all G2–G6 matrix contracts green;
- preserve a retry for a connected omitted call-chain fact;
- preserve existing bounded support-evidence retries;
- remain deterministic and within the 4,000-token limit.

The evaluator preparation and production behavior are separate commits.

## Staged Validation

1. Run focused evaluator tests.
2. Run focused retry-selection tests.
3. Run the complete Go suite, vet, race tests, and shell harnesses.
4. Build a candidate binary and verify the direct G1 Context Pack locally.
5. Stop before any external Codex run.

A new external G1 smoke comparison requires fresh explicit authorization from
the user. A smoke failure ends this hypothesis; it does not trigger additional
tuning on the same candidate.

## Non-Goals

- No G1-, Weka-, service-, endpoint-, or repository-specific production rule
- No Context Pack ranking, source selection, budget, or rendering change
- No change to the general one-retry capability
- No attempt to fix missing answer unknowns or test inventory in this hypothesis
- No external Codex execution without fresh authorization
- No release, tag, push, or promotion to Golden
