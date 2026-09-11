# Mac diagnostic follow-up: concrete retrieval fixes

Implementation and current acceptance limits are tracked in the
[subsequent correction round](AGENT-FIX-ROUND-2026-09-10.md). The measurements below
describe the pre-correction build.

This diagnosis follows the six-run Codex CLI comparison on the unchanged Mac
0442483 test workspace. The installed build is GoreGraph 1.4.1,
`d67d1f4ab3c3-dirty`, agent revision 2. The original strict-v1 comparison remains
separate from these diagnostic controls.

The evidence identifies several independent defects. Rebuilding the workspace or
increasing the public token budget is insufficient. No production fix is claimed
by this document.

## Confirmed findings and implementation order

### 1. Reserve adaptive metadata before compiling the fact budget

`internal/agent/context.go:220` computes the metadata allocation and compiles the
pack before `applyAdaptiveContextHealth` adds adaptive health metadata. After
adding public concerns, the pack is checked against the intermediate allocation
and can fail with `required context concerns exceed metadata budget`.

The English neutral query succeeds under strict-v1 and fails under adaptive-v2.
The failure persists with a 6,000-token public budget: the intermediate allocation
is still 1,100 tokens. An instrumented build measured 1,136 actual tokens and
4,544 JSON bytes against that allocation's 4,400-byte limit. Budget accounting
also uses the reserved public query representation, not only the shorter query
shown in the final response.

**Fix:** reserve protocol, generation, health, concerns and query representation
before selecting facts. Use the same token and byte accounting for allocation
and validation. If the complete minimum envelope cannot fit, emit a bounded,
structured budget fallback instead of aborting an otherwise valid request.
Do not solve this by silently dropping health or disabling the final limits.

**Acceptance:** the stored neutral English, explicit route and German test
requests complete under both protocols at default and maximum budgets, either
with usable evidence or an explicit bounded fallback. Preserve minimum-budget,
Unicode/JSON-byte, multi-project and duplicate-response guarantees.

### 2. Recognize requested evidence consistently in German

`contextQueryRequestsTests` in `internal/agent/context_source.go:750` checks a
small raw-token vocabulary. `contextValueRequestsConcern` and
`contextConcernVocabulary` in `internal/agent/context_intent.go:1317` use another
path. Generic German requirements are consequently lost:

- `Regressionstests` / `Testdateien`: test concern and source-test gate fail.
- `Datenvarianten` / `Zuordnungsmerkmale`: domain-model concern fails.
- `interne Schnittstellen`: HTTP-contract concern fails.

All three isolated language assertions fail on the current source. A temporary
diagnostic alias expansion makes the existing controller delete test appear in
the original German query's context.

**Fix:** normalize generic task/evidence vocabulary once and use it consistently
for concern planning, source gates and exact inventories. Cover compounds and
equivalent German/English requirements without hardcoding benchmark service names
or assuming a domain owner.

**Acceptance:** language-equivalent requests activate equivalent test, model,
contract and inventory requirements. Tests requested with German compounds are
not filtered out by a later source gate. The neutral original query must retain
the requested evidence scope.

### 3. Measure actual proving evidence, not a matching annotation

`contextSourceSectionSupportsConcern` in
`internal/agent/context_select.go:3697` accepts `@ConfigurationProperties` even
when the section is only a class signature. An isolated assertion expecting
configuration fields fails. The real pack labels configuration covered while
the relevant task-client configuration section contains only its annotation and
class declaration.

The original pack contains seven signature-only sections out of twelve.
Authentication, configuration and resilience must be evaluated for the relevant
client/provider rather than satisfied by an unrelated configuration class.

**Fix:** distinguish class identity, actual configuration fields, client
authentication construction, server authorization policy and retry behavior.
Rank proving declarations above optional signatures. Mark missing facets as
uncovered and provide their exact verification ranges where known.

**Acceptance:** an annotation without fields does not satisfy a request for
configuration behavior; an unrelated client's timeout fields do not prove the
target client's resilience. Preserve redaction and require source-backed evidence
for coverage claims. Retain valid identity metadata without calling it proof.

### 4. Discover related existing data and clients across a missing call

`planContextConcerns` in `internal/agent/context_intent.go:41` starts from reachable
facts and explicit project identities. Domain-model selection and source planning
then depend on the recognized concerns. The missing cleanup is not an existing
call edge, so following only the current delete path cannot establish the whole
correction scope.

The workspace agent projection already contains both task entities, the common
client, management controller and relevant test facts. A direct client-name
control retrieves the client body. They are not absent because of a failed scan.
Even after isolating the language and intermediate-budget barriers, the neutral
query still omits both task families and the complete client/provider chain.

**Fix:** add bounded discovery of plausible dependent data and existing service
interfaces using source-backed identity, relationship and persistence evidence.
Keep candidates separate from established runtime ownership. Assemble model,
repository, client, provider, deletion-side-effect and test evidence for the
requested plan; never fabricate the future cleanup edge or an existing DELETE
contract that is not implemented.

**Acceptance:** the original neutral German problem finds both task families and
their parent/object scope, the common-client/provider extension points,
relevant existing tests and current deletion mail/protocol behavior. The absent
Oracle body, database cascades and distributed ordering remain explicit unknowns.
Use additional unrelated fixtures to prevent benchmark-specific ranking rules.

### 5. Separate entrypoint confidence from task completeness

The original adaptive pack still says `MEDIUM`, `partial`, and
`fallback_required=false`. Its three verification requests cover the same narrow
omissions as strict-v1. The candidate expansion at
`internal/agent/context.go:289` runs for fallback/low-confidence packs; a good
entrypoint with insufficient downstream evidence does not enter that path.

The adaptive guide already permits ordinary source fallback under caller
authorization. This capability should be preserved and surfaced through explicit
missing requirements, rather than relying on the agent to infer every gap from
an apparently successful pack.

**Fix:** retain confidence in the entrypoint while exposing unresolved requested
evidence and an actionable bounded verification or caller-authorized fallback
decision. A future call that does not yet exist is not itself a stale-index error.

**Acceptance:** a known handler with missing models/tests/auth details cannot be
presented as sufficient for the requested plan. Authorized fallback recovers the
missing evidence; without that authorization, report the limitation. Do not grant
new filesystem or tool authority through context output.

## Diagnostic controls

Fifteen installed-binary requests compare strict/adaptive behavior, default and
maximum limits, neutral wording, English translation, punctuation, and explicitly
named positive controls. Eight adaptive requests fail at the intermediate
metadata check. These controls are causal probes, not a new efficacy benchmark.

Four isolated desired-behavior cases are intentionally red: three language cases
and one configuration-signature case. They were executed with a Go overlay of an
existing test file. The normal checkout test suite was not modified.

A separate diagnostic build temporarily adds six generic language aliases,
uses expanded tokens in the test gate, and bypasses only the intermediate
metadata abort. Six probes then finish within their final token and byte limits.
Some test source becomes available, but cross-service completeness remains
insufficient. This build is an isolation experiment, not an approved implementation
or an installed replacement. The proper budget repair is described above.

The full natural-language query is retained internally for concern planning.
`contextPrimaryQuery` uses the first sentence for several ranking paths, while
the public query is also compacted for output. Therefore a shortened displayed
query alone is not proof that all later requirements were discarded. Ranking
across multi-sentence problem descriptions needs regression coverage after the
confirmed intent and budget defects are repaired.

## Completed adaptive CLI diagnostic

One additional fresh Codex CLI session used `gpt-5.6-sol` with high reasoning,
the unchanged base task and canonical adaptive guidance. Ordinary read-only
source fallback was explicitly permitted inside the workspace. The agent's
initial natural-language context request hit the metadata-budget error, returned
no source and allowed no retry anchor. It then used the authorized ordinary
source workflow.

| Measure | Adaptive attempt with ordinary fallback |
| --- | ---: |
| End-to-end duration | 748.65 seconds (12:29) |
| Input / cached input / output | 2,801,755 / 2,649,728 / 20,195 |
| Effective tokens: input minus cache plus output | 172,222 |
| Tool calls | 51 |
| GoreGraph calls / successful context packs | 1 / 0 |
| Coverage rubric | 11/12 |
| Existing inventory paths and ranges checked | 42/42 |
| External skill reads | 0 |

Both task families and the cross-project correction plan were recovered. The
answer did not explicitly address the mail/protocol behavior of existing
single-task deletion, so that rubric criterion remains unmet. New routes are
identified as proposals and unknown database/distributed semantics are disclosed.

This demonstrates recovery through ordinary source investigation after a failed
context request. It does not demonstrate improved adaptive retrieval or savings
at equal quality. This single diagnostic attempt is not pooled with the original
three-per-variant series. Quality was assessed by Codex against the same source
reference, not by an independent blinded evaluator.

The prepared 374-file snapshot, installed binaries and selected index artifacts
remain unchanged. The original six measurements and their raw logs also remain
intact. No experimental overlay was installed. The four expected-red diagnostic
cases are retained outside the checkout; no failing test was added to its normal
suite. `git diff --check` passed after the documentation changes.

## Evidence location

Local raw packs, controls, overlay sources and logs are retained at:

`/Users/gorecode/.codex/visualizations/2026/09/10/01a08a04-dfec-7502-a22a-af1ed717fd03/benchmark-0442483-macos/diagnosis/`

Key files: `probe-inputs.json`, `probe-results.json`, `probe-contexts.py`,
`diagnostic-tests.log`, `diagnostic-tests.go.txt`, `budget-trace.json`,
`budget-byte-trace.json`, `isolation-results.json`, and `index-presence.json`.
The completed agent attempt is recorded in `adaptive-agent.log`,
`adaptive-answer.md`, `adaptive-metrics.json`, `adaptive-quality.json`,
`adaptive-citations.json`, and `timings/timing-1.json`.
