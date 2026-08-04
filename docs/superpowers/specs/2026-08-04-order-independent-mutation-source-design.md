# Order-Independent Mutation Source Selection

## Problem

GoreGraph can distinguish a triggering mutation from a richer dependent-service endpoint when a problem statement says that related records remain after the mutation. The current detector only recognizes the token order `mutation -> relation -> lingering`. It misses equally valid clauses such as `mutation -> lingering -> relation`, which lets a short, unrelated mutation endpoint win the later primary-action score.

The fix must not encode historical repository names, business nouns, route names, or translations between a private domain and source identifiers.

## Goals

- Recognize a mutation and its dependent symptom regardless of whether the relation marker appears before or after the lingering marker.
- Keep direct requests such as “remove related jobs” targeted at the dependent endpoint.
- Prefer a source route only when source-derived route structure connects it to a dependent route.
- Fail closed when a lingering-dependent problem has multiple mutation endpoints but no unique exact, transition, lexical-domain, or structural source.
- Preserve deterministic output across index order.

## Non-Goals

- General natural-language parsing or a new NLP dependency.
- Translation of business nouns.
- Inferring a missing HTTP contract, route, transaction order, or compensation policy.
- Changing token budgets, source-selection limits, or retry behavior.

## Design

### Clause-aware symptom detection

The primary problem sentence is split into comma- and semicolon-delimited clauses while retaining normalized token order. The first clause containing a mutation action remains the trigger clause.

A dependent symptom exists when a relation marker and a lingering marker occur either:

1. together in the clause following the mutation clause, in either order; or
2. after the mutation in the same clause, separated from the mutation target by a consequence boundary or sufficient source phrase.

Relation markers remain technical vocabulary such as `related`, `linked`, `dependent`, `verbunden`, and `abhängig`. Lingering markers remain technical vocabulary such as `remain`, `persist`, `linger`, `bleiben`, and `verbleiben`. No business noun aliases are added.

For same-clause problems, the symptom begins at the earlier of the relation and lingering markers. The mutation source clause ends before that symptom. For adjacent clauses, the mutation clause is already the complete source clause.

### Source evidence and ranking

Existing ranking precedence remains:

1. exact route or symbol anchor;
2. explicit `from -> to` transition source;
3. source-derived parent route for a lingering-dependent symptom;
4. primary mutation-domain match;
5. forward graph utility.

A route is a structural source only when its ordered path tokens form a strict subsequence of a dependent endpoint’s ordered path tokens and the dependent endpoint matches the dependent entity language. Reordered token sets do not count as ancestry.

Mutation verbs and transport vocabulary do not count as domain evidence. A short endpoint cannot win merely because all candidates match `DELETE`.

### Fail-closed behavior

For a detected lingering-dependent problem with multiple action-aligned endpoints, GoreGraph returns endpoint ambiguity instead of selecting a candidate when all of the following are true:

- there is no exact route anchor;
- there is no explicit transition source;
- there is no structurally connected source route;
- no single endpoint has a positive source-clause domain match.

This keeps clear cases productive and prevents confident selection of unrelated housekeeping or administrative mutations.

## Tests

Neutral `catalog`, `jobs`, and `shared` fixtures cover:

- relation before lingering;
- lingering before relation;
- English and German technical phrasing;
- a direct mutation of the dependent entity;
- a short unrelated housekeeping `DELETE` endpoint;
- reordered path tokens;
- ambiguous disconnected mutation endpoints;
- reversed index order.

The existing G2-G6 matrix, independent generality workspace, full Go suite, Vet, race detector, documentation sync, and the local historical read-only Context gate remain mandatory.

## Acceptance Criteria

- Both German word orders select the neutral source endpoint.
- The direct dependent-target query still selects the dependent endpoint.
- Disconnected ambiguous mutations return a documented ambiguity instead of an arbitrary endpoint.
- No new production identifier names a historical repository, service, business object, or private organization.
- The historical benchmark prompt selects exactly one public regulation-removal entrypoint, stays within 4,000 Context tokens, requests no retry, and retains the missing-contract uncertainty.
