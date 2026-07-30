# Agent Context Release Quality Stabilization Design

**Status:** Approved on 2026-07-30

## Purpose

Bring the GoreGraph Agent Context output to a stable, releasable quality level
without giving back the efficiency gains demonstrated by the completed G1
release benchmark.

The change must close the three reproducible answer-quality gaps:

- lookup attributes and domain-model evidence;
- authentication and configuration evidence;
- relevant production and test file inventory.

It must do so through general evidence-selection rules, not through private G1
repository names, file names, symbols, or exact answer wording.

## Evidence behind the change

The completed release benchmark at `main` commit `916f366` was stable across
three assisted runs:

- all assisted runs received the same Context ID;
- every pack stayed below the 4,000-token limit;
- assisted median tool calls and source reads were substantially lower than
  the baseline;
- no assisted run reconstructed the workspace or reread included source;
- all assisted answers scored 9/12 while every baseline answer scored 12/12.

The three assisted answers missed the same facets because the shared Context
Pack:

- selected a signature-only, same-named task entity from the wrong service
  instead of source that proves both lookup fields for the requested task
  models;
- reported authentication coverage without exposing the Basic Authentication
  client setup, server policy, technical role, and associated configuration;
- left configuration, repositories, tests, and related file inventory
  uncovered or represented by declaration-only omissions.

This is a deterministic Context Pack defect, not answer-model variance.

## Frozen boundaries

The quality fix operates under the following invariants:

- maximum estimated Context Pack size remains 4,000 tokens;
- maximum published file count remains 12;
- source sections already supplied by GoreGraph remain authoritative and must
  not require rereading by the consuming agent;
- current source-read, tool-call, fallback, retry, and deterministic-output
  gates are not relaxed;
- current benchmark evidence remains immutable and failed;
- no private workspace identifier may appear in production ranking,
  extraction, coverage, or prompt logic;
- no public CLI or Context schema change is required for this fix;
- token-accounting calibration is a separate prospective benchmark change and
  may not be chosen from the candidate's result.

The current `main` commit remains the Golden Build until every acceptance gate
in this design passes.

## Considered approaches

### Adjust ranking weights

Increase scores for configuration, authentication, models, repositories, and
tests. This has a small implementation footprint, but it still lets a weak
candidate claim coverage and merely moves the failure to another naming or
project layout. Previous iterations showed that isolated weight tuning is not
stable enough.

### Increase the Context budget

Publish more files and source. This could hide the missing evidence, but it
would weaken the already successful efficiency contract and make the agent
consume irrelevant material.

### Validate rendered evidence and substitute within the frozen budget

Make the rendered source or exact metadata prove each required evidence facet.
Prefer candidates that prove the correct project, owner, model, and requested
behavior. If the first selection leaves a required facet unrepresented,
deterministically replace lower-value optional evidence without exceeding the
existing limits.

This is the selected approach because it fixes the false coverage decision at
its source and preserves the established output budget.

## Design

### Evidence requirements

Intent analysis continues to produce internal concerns. Required concerns are
expanded into specific evidence requirements when the query asks for those
details:

- domain types and lookup attributes;
- selected client transport authentication;
- provider authentication policy and role;
- configuration binding and configuration consumption;
- resilience policy and recovery behavior;
- persistence for each requested model;
- requested business-side-effect facets;
- executable tests;
- bounded production and test file inventory.

Each requirement carries its normalized project, concern kind, optional facet,
candidate fact identities, and requested domain identity. Public concerns
continue to aggregate internal facets, but a public concern is covered only
when every required internal facet is proven.

### Proof comes from rendered output

A selected fact is candidate discovery evidence, not coverage proof.

After a candidate is rendered, GoreGraph derives the exact requirement keys
proved by that option. Proof requires:

- the rendered section or exact metadata belongs to the required project;
- the candidate is associated with one of the requirement's facts;
- the rendered content satisfies the concern- or facet-specific predicate;
- domain evidence has the requested model identity and contains useful model
  structure rather than only annotations or a type signature;
- test evidence contains an executable body;
- persistence evidence identifies the relevant repository or operation;
- authentication and configuration evidence proves the requested client or
  provider facet rather than an unrelated authenticated endpoint.

Domain-model coverage becomes rendered-evidence-aware like the existing
cross-cutting concerns. An inherited or shared base model may prove requested
fields when the index links it to the requested model and the section contains
the field declarations. A same-named type from another project cannot prove
the requirement without the requested model relationship.

### Bounded candidate frontier

Candidate discovery remains bounded and deterministic.

The selector sorts facts using semantic relevance, project scope, stable domain
identity, action alignment, confidence, and ownership. It then keeps searching
the sorted frontier until it has the bounded number of candidates that can
actually prove the requirement, rather than consuming the whole allowance with
facts whose rendered sections prove nothing.

The implementation must retain an explicit planning ceiling and the existing
subquadratic-growth tests. It must not perform unbounded source rendering or
workspace search.

### Evidence-first selection

Selection has three deterministic phases:

1. Preserve mandatory production boundaries for the reliable entrypoint and
   primary call chain.
2. Fill uncovered required evidence using the greatest marginal proof gain,
   identity quality, and evidence density per token. Requested production
   evidence remains ahead of tests, while an explicitly requested test
   inventory retains a bounded file slot.
3. Use remaining capacity for optional enrichment and additional strongly
   matched domain or persistence evidence.

Candidate quality may break ties, but it cannot turn an unproven requirement
into a covered one.

### Deterministic substitution

After initial selection, a bounded repair pass considers only still-uncovered
required requirements that have a proving option.

A replacement is allowed only when it:

- preserves mandatory entrypoint and primary-path boundaries;
- preserves all required proofs uniquely supplied by the removed option;
- strictly increases the number of proven required requirements, or preserves
  that number while strictly improving the requested identity match;
- stays within 4,000 estimated tokens and 12 files;
- uses already bounded candidate options;
- wins deterministic tie-breakers.

The pass reaches a fixed point after a finite number of strictly improving
replacements. It cannot oscillate and cannot add a retry or fallback.

### Independent final coverage audit

Before serialization, GoreGraph rebuilds source coverage from the final
published source sections and exact metadata evidence. It does not trust the
incremental selection state.

The audit:

- recalculates every internal evidence requirement;
- aggregates public coverage with logical AND across required facets;
- recomputes `source_unrepresented`;
- emits `complete` only when every required facet is proven;
- emits a narrow, project-scoped omission for every unproven requirement.

If a candidate fact exists but its rendered section does not prove the claim,
the omission says that indexed evidence was insufficient. GoreGraph must never
claim coverage merely because the fact was selected.

### Bounded file inventory

Behavioral claims still require source proof. File inventory is exact metadata
and does not require publishing every file body.

When the query explicitly asks which production and test files are involved,
the selector reserves bounded file entries for the strongest required
configuration, authentication, persistence, contract, implementation, and
test evidence. Exact paths come only from indexed facts or rendered source.
Source omissions are not treated as a complete file inventory.

The inventory remains inside the existing 12-file cap. If all requested file
roles cannot fit, coverage stays partial and omissions identify the missing
roles without inventing future files.

## Error handling

- A missing or ambiguous model relationship cannot prove domain coverage.
- A signature-only model section cannot prove requested lookup attributes.
- Generic bearer or authenticated markers cannot prove a selected client's
  transport authentication.
- A repository declaration cannot prove unrelated configuration or tests.
- A non-executable test declaration cannot prove test behavior.
- An over-budget proving option remains an omission; budgets are never
  silently raised.
- A final-audit mismatch fails closed as partial coverage and is retained in
  deterministic diagnostics.
- Selection remains successful when evidence is genuinely absent; absence is
  represented honestly rather than converted into fallback behavior.

## Testing strategy

Implementation uses test-driven development and one hypothesis at a time.

### Targeted failing regression

Add a generic Java/Spring cross-service fixture that reproduces the release
failure without private names:

- a consumer and provider contain same-named model types;
- only the provider's requested task models or linked base model declare both
  lookup fields;
- a public bearer endpoint competes with an internal Basic Authentication
  client and provider policy;
- a configuration holder contains base URL, credentials, timeouts, and retry
  values;
- two requested task variants have distinct repositories;
- relevant production and executable test files exceed the easiest initial
  selection but still fit the frozen output limits.

The current Golden Build must fail the new assertions for the same three
reasons as G1.

### Focused unit tests

Add tests proving:

- domain coverage rejects annotation-only and wrong-project duplicate types;
- linked base-model fields can prove requested lookup attributes;
- selected-client transport authentication is not aliased to bearer server
  authentication;
- configuration binding and consumption remain distinct required facets;
- final coverage is derived from final sections, not selected fact IDs;
- substitution removes only lower-value, non-mandatory evidence;
- substitution cannot oscillate or exceed either budget;
- requested test inventory retains a bounded slot behind required production
  evidence;
- omissions stay exact, project-scoped, and deterministic;
- candidate planning remains subquadratic.

### Regression matrix

Run:

- focused `internal/agent` tests;
- all generic cross-service language fixtures;
- context size, determinism, retry, fallback, and omission tests;
- the fake agent-context regression harness;
- `go test ./... -count=1`;
- `go vet ./...`;
- formatting and repository documentation checks.

Unrelated Context Pack changes are failures unless explicitly explained by the
new proof contract.

## Staged acceptance

External runs start only under an active data-sharing authorization. The six
runs retained by the completed release benchmark consume their exact
six-run authorization and do not implicitly authorize the later smoke or final
matrix.

### Stage 1: deterministic local proof

- The new generic fixture proves all requested evidence facets.
- Existing generic fixtures retain their required evidence and forbidden
  outcomes.
- Packs remain at or below 4,000 tokens and 12 files.
- Repeated identical requests produce byte-stable semantic output.
- Full local verification passes from a clean candidate commit.

Failure stops the candidate. No external run is used to compensate for a local
failure.

### Stage 2: private G1 smoke proof

Install the exact candidate commit, clean and rescan a fresh copy of the
historical workspace, and run one read-only assisted smoke evaluation using
the frozen prompt and settings.

The smoke must:

- retain every previously correct facet;
- prove the three previously missing facets;
- produce no forbidden claim, broad search, included-source reread, fallback,
  or retry;
- stay within the frozen pack and file budgets;
- avoid increasing source reads or tool calls above the accepted assisted
  behavior.

A semantic failure returns to local diagnosis. It does not authorize another
ranking adjustment layered on the same hypothesis.

### Stage 3: prospective token-metric freeze

Before the final release matrix, define and commit a separate measurement
contract for the current Codex token fields. The contract must:

- record cached input, non-cached input, output, and any available reasoning
  counters separately;
- use unchanged-build control evidence to establish comparability;
- freeze thresholds before seeing the final candidate matrix;
- retain relative assisted-versus-baseline efficiency;
- leave the completed failed benchmark unchanged.

Product ranking must not be tuned against this calibration.

### Stage 4: final release matrix

Run the frozen three-baseline/three-assisted interleaved matrix with identical
workspace, prompt, model, reasoning, sandbox, and approval settings.

Release acceptance requires:

- all three assisted answers score 12/12 under independent review;
- no assisted run introduces a forbidden or unsupported claim;
- all deterministic pack contracts pass;
- source reads and tool calls meet the frozen relative gates;
- token metrics pass the prospectively frozen measurement contract;
- every artifact records exact binary, commit, scan, prompt, configuration,
  and execution-order identity.

No threshold changes, replacement runs, or post-result reinterpretation are
allowed.

## Release completion

After the candidate passes all stages:

- synchronize README, language-capability tables, help, benchmark
  documentation, and release notes with verified behavior;
- run documentation-truth and cross-platform CI checks;
- verify the release commit is clean, reproducible, and installed locally;
- obtain the independent benchmark review and final release decision;
- tag or publish only after explicit user approval.

Until then, GoreGraph 1.3.0 is not release-ready.

## Rollback and stop rules

- The Golden Build and all retained evidence remain reproducible.
- Each implementation slice has one written hypothesis and one failing
  regression before production code changes.
- A failed slice is reverted or corrected before another axis is changed.
- After three failed hypotheses, implementation stops for an architectural
  review instead of applying another ranking tweak.
- Promotion occurs only after the complete acceptance sequence passes.

## Non-goals

- No private G1-specific production rules or fixtures
- No response-budget increase
- No weakening of answer-quality or bounded-navigation gates
- No retrospective rescue of the failed release benchmark
- No public schema or CLI expansion unless implementation evidence proves it
  unavoidable and a separate design is approved
- No release, tag, or publication as part of the quality implementation
