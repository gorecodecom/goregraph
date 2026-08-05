# GoreGraph Output Contract

<!-- goregraph:generated current-contract start -->
Current source contract: GoreGraph 1.3.0 with output Schema 3 (unreleased). Published artifacts remain tied to the latest released tag until 1.3.0 is explicitly published.
<!-- goregraph:generated current-contract end -->

## Build Targets and Extraction

Project builds use one of three explicit targets:

```bash
goregraph build agent .
goregraph build dashboard .
goregraph build all .
goregraph update . --target agent
goregraph update . --target dashboard
goregraph update . --target all
```

Workspace builds and refreshes use the same targets:

```bash
goregraph workspace build agent .
goregraph workspace build dashboard .
goregraph workspace build all .
goregraph workspace refresh . --target agent
goregraph workspace refresh . --target dashboard
goregraph workspace refresh . --target all
```

`goregraph scan .` is a compatibility alias for `goregraph build all .`.
`goregraph workspace scan-all .` is a compatibility alias for
`goregraph workspace build all .`.

A project build extracts source once. The `all` target writes the agent and
dashboard projections from that shared extraction; it does not scan once per
projection. A workspace build scans each discovered project once, then
reconciles the workspace once after the project loop. Target-aware `update` and
`workspace refresh` preserve an already-valid projection that was not selected.

A single-project build requires no workspace marker. Workspace-wide commands
require one of:

- an auto-detectable grouped frontend/services layout;
- an explicit `--workspace <root>`;
- `.goregraph-workspace.yml` at the workspace root.

A build or scan does not create `.goregraph-workspace.yml` implicitly. The
generated `.goregraph-workspace/` directory is removable output, not a
persistent workspace marker.

## Output Ownership

| Scope | Shared machine index | Agent projection | Dashboard projection |
|---|---|---|---|
| Project | `goregraph-out/index/` | `goregraph-out/agent/` | `goregraph-out/dashboard/` |
| Workspace | `.goregraph-workspace/index/` | `.goregraph-workspace/agent/` | `.goregraph-workspace/dashboard/` |

The ownership rules are strict:

- `manifest.json` remains at the project or workspace output root and records
  Schema 3 projection status.
- `index/` is GoreGraph's complete shared machine index. It is input to
  GoreGraph commands and projections, not direct prompt input.
- `agent/context-index.json`, `agent/agent-guide.md`, and bounded Context Packs
  are the only recommended generated AI input.
- `dashboard/` is the full human exploration projection. Agents must not ingest
  dashboard Markdown, HTML, assets, or `index/symbol-usages.json` as prompt
  context.
- A project dashboard build writes human-readable Markdown reports. The
  interactive eight-view dashboard is workspace-only in 1.3.0.
- `.goregraph-dashboard.json` is user-owned workspace configuration, not
  generated output. Dashboard rebuilds read it; clean commands preserve it.

## Exact Project Tree

The complete project layout for `build all` is:

```text
goregraph-out/
├── manifest.json
├── index/
│   ├── freshness.json
│   ├── files.json
│   ├── symbols.json
│   ├── relations.json
│   ├── graph.json
│   ├── symbols-full.json
│   ├── relations-full.json
│   ├── graph-full.json
│   ├── callgraph.json
│   ├── endpoint-flows.json
│   ├── test-map.json
│   ├── routes.json
│   ├── flows.json
│   ├── api-contracts.json
│   ├── api-catalog.json
│   ├── architecture-capabilities.json
│   ├── service-dependencies.json
│   ├── frontend-usage.json
│   ├── contract-matches.json
│   ├── diagnostics.json
│   ├── diagnostics-canonical.json
│   ├── diagnostic-families.json
│   ├── package-graph.json
│   ├── maven-graph.json
│   ├── analyzers.json
│   ├── evidence.json
│   ├── capabilities.json
│   ├── coverage.json
│   ├── spring.json
│   ├── audit.json
│   ├── workspace-contract-matches.json
│   ├── workspace-feature-flows.json
│   ├── workspace-feature-dossiers.json
│   ├── workspace-graph.json
│   ├── workspace-service-map.json
│   ├── workspace-endpoint-traces.json
│   ├── directed-traces.json
│   └── data-flows.json
├── agent/
│   ├── agent-guide.md
│   └── context-index.json
└── dashboard/
    ├── workspace.md
    ├── endpoints.md
    ├── endpoint-flows.md
    ├── dependencies.md
    ├── callgraph.md
    ├── routes.md
    ├── flows.md
    ├── api-contracts.md
    ├── frontend-usage.md
    ├── contract-matches.md
    ├── potentially-broken-contracts.md
    ├── diagnostics.md
    ├── workspace-context.md
    ├── workspace-contract-matches.md
    ├── workspace-feature-flows.md
    ├── workspace-feature-dossiers.md
    ├── data-flows.md
    ├── workspace-map.md
    ├── workspace-next-actions.md
    ├── frontend-consumers.md
    ├── package-graph.md
    ├── maven-graph.md
    ├── navigation.md
    ├── analyzers.md
    ├── coverage.md
    ├── workspace-summary.md
    ├── architecture.md
    ├── affected.md
    ├── report.md
    ├── modules.md
    ├── entrypoints.md
    └── test-map.md
```

The `agent` target writes `manifest.json`, the shared `index/`, and `agent/`.
The `dashboard` target writes `manifest.json`, the shared `index/`, and
`dashboard/`. The `all` target writes the complete tree above.

## Exact Workspace Tree

The complete workspace layout for `workspace build all` is:

```text
.goregraph-workspace/
├── manifest.json
├── index/
│   ├── registry.json
│   ├── context.json
│   ├── contract-matches.json
│   ├── feature-flows.json
│   ├── data-flows.json
│   ├── feature-dossiers.json
│   ├── workspace-graph.json
│   ├── workspace-service-map.json
│   ├── workspace-endpoint-traces.json
│   ├── directed-traces.json
│   ├── freshness.json
│   ├── symbol-index.json
│   ├── symbol-usages.json
│   └── api-catalog.json
├── agent/
│   ├── agent-guide.md
│   └── context-index.json
└── dashboard/
    ├── workspace-map.html
    ├── workspace-context.md
    ├── contract-matches.md
    ├── feature-flows.md
    ├── feature-dossiers.md
    ├── next-actions.md
    └── workspace-map-assets/
        └── code-usages-<project-hash>.js
```

The two canonical symbol projections are built for the workspace dashboard and
remain under the shared `index/` ownership boundary. Code Explorer loads the
project-specific JavaScript shard only after a project is selected. Keep
`workspace-map-assets/` next to `workspace-map.html`.

## Normal Agent Workflow

Use GoreGraph once to obtain a source-backed Context Pack for the complete task:

```bash
goregraph context . --query "<current coding task>" --budget-tokens 4000 --max-files 12
```

Standard MCP exposes exactly one tool, `task_context`, with equivalent values.

<!-- goregraph:generated agent-instruction start -->
```text
Call goregraph context . --query "<focused query>" exactly once before reading indexed source; put the caller's problem statement and requested evidence scope in the query.
Preserve the caller's domain language, identifiers, and requested evidence; exclude workspace setup, tool policy, safety constraints, and output-format instructions. Do not translate or add inferred repository or component responsibilities.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; make the file reader itself range-bounded, for example with sed -n, and never pipe a whole-file reader such as nl through a downstream range filter. Do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
A missing future call, route, or symbol required by the requested fix is evidence of the current gap, not a source-fallback trigger; assess entrypoint reliability from the existing production path.
For change plans, include separate exact existing production-file and test-file inventories from files, source_sections, production_plan_files, plan_files, or bounded omission reads; name every supplied production_plan_files identity in the production-file inventory with its role because naming metadata is not reading source; name every supplied plan_files identity in the test-file inventory with its use because naming metadata is not reading source, provider_test entries may be test targets, and mock_pattern or retry_pattern entries are reference patterns, not change targets. Never read production_plan_files or plan_files unless source_omissions lists the same exact path with a bounded range; do not invent future filenames, and keep future route, authentication, status, lookup implementation, dependent persistence and cascade behavior, and cross-service transaction ordering as unknown design decisions unless rendered source proves them.
When authentication or configuration is requested, report supplied server authorization policy, client authentication construction and configuration fields, and exact paths of supplied production and test-profile resources together in one coherent answer section; name every supplied configuration_resources identity with its project, profile, and key groups, and distinguish current evidence, required additions, and unknown deployment values.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.
```
<!-- goregraph:generated agent-instruction end -->

The source sections replace reads of included ranges. `source_coverage` is authoritative:
with complete coverage, run no source-reading commands on indexed project files;
answer only from `source_sections` and mark absent details as unknown. With partial
or none coverage, inspect only exact project/path and `start_line`/`end_line`
ranges in `source_omissions`; do not inspect outside those ranges or other
files, and report pathless or unbounded omissions as uncertainty.
`source_unrepresented` counts visible required concerns without selected source;
`files` remain metadata rather than automatic fallback scope. A second Context
call is allowed only when `retry_allowed` is true and must carry one returned
anchor plus the first `context_id`. Context token estimates are approximate;
the release benchmark's prospective comparison metric is `effective_tokens`.

The optional `plan_files` array is emitted only for exact missing-transition
production/test inventories. It contains at most four exact indexed paths below
a test source root, with `project`, `path`, and `use`. Uses are `provider_test`,
`mock_pattern`, and `retry_pattern`; mock and retry patterns are
emitted only from a validated existing pair. These identities are metadata
only, do not count toward the 12 source files, 12 source sections, or three
source omissions, and do not authorize reading. A path may be read only when
the same path has a bounded range in `source_omissions`; pattern entries are not
change targets. These packs clear repetitive `files.reason` text to pay for the
new metadata while preserving file paths, ranges, roles, confidence, source
sections, and every hard limit. `plan_files` are excluded from the proactive
final-decision reserve; the ordinary final token/byte check still reduces any
pack that does not fit.

The optional `production_plan_files` array is emitted for the same exact
missing-transition production/test inventories when exact production identities
are not otherwise represented. Each group contains `project`, at most one
existing `provider_contract`, and at most two `primary_persistence` paths.
Selection requires exact production-scoped facts in a derived provider project
and domain-matched repository owners. Dependent/comment repositories, generic
persistence operations, test sources, foreign projects, absolute or traversal
paths, and already represented identities are excluded. The field is metadata
only, does not assert that a future operation exists, and does not authorize a
read without the same bounded path in `source_omissions`.

For exact change inventories that request configuration, the optional
`configuration_resources` array groups relevant exact indexed Spring
`application` and `bootstrap` resources by shared `project` and value-free
`key_groups`. Every group's nested `resources` contain `path` and `profile`;
selection is capped at six resources before grouping. It exposes no property
values, does not add source file slots, and never grants read permission; only
an exact bounded entry in `source_omissions` authorizes inspection.

The `domain_model` concern and source role identify current source for
explicitly requested types, entities, payloads, identifiers, or lookup
attributes. The selector prefers declaration bodies with stable domain identity
over unrelated one-line cross-cutting signatures. Within one project, up to two
distinct domain-model families and two distinct persistence families may retain
the diversity preference. `source_coverage: complete` still means that every
required concern has current source, not that every candidate was serialized.
The hard limits remain 4,000 tokens, 12 files, and 12 source sections; complete
coverage permits no source-reading commands on indexed project files.

`source_coverage: complete` requires every requested evidence facet to be
backed by a verified `source_section`. In particular, one repository cannot cover multiple requested domain models, and one side-effect section cannot
implicitly cover mail, audit, and user-information behavior.

When the bounded pack cannot represent every facet, `source_coverage` is
`partial` or `none`. `source_omissions` may combine missing facets for the same project and path so an agent can inspect that one indexed file without widening
navigation. Non-faceted omissions retain their concrete source failure reason.

The Context Pack `query` field is the normalized request text verbatim when it
is at most 256 runes and its JSON encoding, including quotes and escapes, is at
most 256 bytes; otherwise it is a compact primary-task summary. The complete
input remains internal for the request lifecycle so ranking, concerns, source
selection, and retry planning use the full task, but that input is neither
emitted nor included in `context_id`.

For an API task, the agent compiler selects at most one relevant endpoint and
eight consumer call sites, with an explicit omitted count. The default budget
remains 4000 tokens. `agent/context-index.json` contains compact searchable
facts; Context output never includes the full `index/api-catalog.json`, the
dashboard payload, or `.goregraph-dashboard.json` merely because an agent uses
GoreGraph.

## Benchmark metric meaning

Both raw and effective counters are retained. `effective_tokens` is
`input_tokens - cached_input_tokens + output_tokens`, or uncached input plus
output; `total_tokens` is `input_tokens + output_tokens`. Reasoning output is
recorded separately but is already part of output, so reasoning output is not
double-counted. The 80% matched threshold uses effective tokens, and the
116,560 absolute cap uses effective tokens. Context Pack `estimated_tokens`
remains unrelated to end-to-end usage.

`external_skill_read_calls` is plugin-agnostic transcript evidence: it counts
read or search targets outside the benchmark workspace that resolve to a skill
bundle. Both controlled variants require zero external skill reads across the
complete transcript. `--ignore-user-config` is not a skill-isolation guarantee.
The harness records plugin state but never mutates it. Normal GoreGraph use
remains compatible with Brainstorming, TDD, debugging, and review skills.

The last prospectively calibrated three-by-three release matrix passed for
candidate `fb14d65`: effective-token medians were 162,410 baseline and 28,215
assisted (82.63% lower), and means were 162,089 baseline and 26,390 assisted
(83.72% lower). Tool-call medians were 28 baseline and 3 assisted; source-read
medians were 21 baseline and 2 assisted. All six runs had zero external skill
reads. Quality medians were 11 baseline and 12 assisted, with every assisted
run scoring 12/12. This qualifies runtime candidate `fb14d65` and
documentation-only descendants. It covers one frozen historical
three-repository Java case, not a general savings guarantee.

## Human Dashboard

`.goregraph-workspace/dashboard/workspace-map.html` is the Schema 3 standalone
offline dashboard. It contains Architecture, API Catalog, Endpoints, Feature
Flow, Data Flow, Code Explorer, Diagnostics, and Coverage views.

- Architecture derives dynamic domain lanes, keeps stable card coordinates,
  and distinguishes statically detected relationships from runtime traffic.
- API Catalog is the complete provider endpoint inventory, including endpoints
  without known consumers. It reads `index/api-catalog.json` and appears before
  Endpoints.
- Endpoints links callers, providers, symbols, files, and lines to source.
- Feature Flow presents route-to-component-to-API-to-backend-to-persistence-to-
  test chains.
- Data Flow shows field movement and explicit evidence gaps.
- Code Explorer keeps **Direct references** and **Reached through API**
  separate under **Explore classes & symbols**.
- Diagnostics uses normal vertical scrolling at 100% scale.
- Coverage separates workspace completeness and prioritized next scans from
  analyzer capability support.

Dashboard output is the complete human exploration surface. It is not Context
Pack input.

`goregraph workspace dashboard path .` prints the generated static file and
`goregraph workspace dashboard open .` opens the same offline, read-only file.
Only `goregraph workspace dashboard edit .` starts an authenticated loopback
server. The editor supports group rename/reorder and service drag-and-drop or
keyboard movement. Save persists stable group labels, order, and service
placement in the workspace-root `.goregraph-dashboard.json`; Discard restores
the saved draft, and Reset to detected removes architecture overrides after
confirmation. Rebuilds preserve valid manual choices, auto-place new services
from production package/module evidence, and retain stale removed-service
overrides so Doctor can report them.

Endpoint security and consumer call authentication are separate static
evidence. Missing evidence remains `unknown` and is displayed as
`No auth evidence detected`; it is not treated as `public`. Runtime enforcement,
traffic, and production authorization are outside the output contract.

## API Catalog and Dashboard Configuration Schemas

Project `goregraph-out/index/api-catalog.json` and workspace
`.goregraph-workspace/index/api-catalog.json` use the same Schema 3 canonical
model. Each endpoint records a stable ID, provider project, HTTP method/path,
handler/source, supported request and response identities, provider security,
consumers, confidence, coverage, limitations, and evidence IDs. Each consumer
keeps its project, caller/source, resolution, call authentication, limitations,
and evidence. An empty consumer list does not mean the endpoint is unused.

`.goregraph-dashboard.json` uses its own versioned configuration schema. Schema
1 contains an `architecture` object with `groupOrder`, stable `groups` keyed by
machine ID and editable `label`, plus `services` keyed by project-relative path
with `group` and `order`. Unknown fields are rejected. Labels are presentation
only; stable IDs let renamed groups survive later scans.

## Exact Symbol and Usage Semantics

The selected-service Code Explorer reads
`.goregraph-workspace/index/symbol-index.json` and
`.goregraph-workspace/index/symbol-usages.json`.

- `direct_reference` with `EXACT` is a statically proven source or compile
  relationship to one canonical symbol. It is not a runtime invocation count.
- `reached_through_api` with `EXACT` is HTTP reachability proven through ordered
  API path steps from a consumer origin to a route, backend implementation, and
  selected provider. It is not a direct import or runtime request count.
- `ambiguous` with `AMBIGUOUS` preserves every candidate symbol or path.
- `unresolved` with `UNRESOLVED` preserves the attempted target and reason when
  no safe provider can be selected.

A canonical symbol ID includes symbol kind, project, module/artifact/package
scope, language, qualified or export name, and declaration file. File or
identifier name alone is insufficient. A canonical usage ID also includes the
consumer, category, relation kind, target identity, source file, and source
line. Evidence namespacing uses `<project>#<local-evidence-id>`.

Coverage records use `COMPLETE`, `PARTIAL`, `UNAVAILABLE`, and `FAILED` to
describe what static analysis could index. Missing records are reported
separately, so an empty usage list is not proof that a symbol is unused.

These legacy/manual CLI operations remain supported:

```bash
goregraph query . symbol-inventory --query microservices/ms-user --format markdown --limit 20
goregraph query . symbol-resolve --query com.acme.UserService --format json --limit 20
goregraph query . symbol-usages --query symbol:<stable-id> --format markdown --limit 20
goregraph query . symbol-api-consumers --query symbol:<stable-id> --format json --limit 20
goregraph query . symbol-explain --query usage:<stable-id> --detail full --format markdown --limit 20
```

Their MCP equivalents—`symbol_inventory`, `symbol_resolve`, `symbol_usages`,
`symbol_api_consumers`, and `symbol_explain`—exist only in explicit
`--expert-tools` mode. Task pagination uses `limit` plus `continuation`; CLI
uses `--limit` plus `--continue`.

Doctor validates Schema 3, stable references, evidence namespacing,
project-relative sources, categories, resolutions, candidate sets, and API path
steps. Missing or invalid generated output can be rebuilt with:

```bash
goregraph workspace clean . --execute
goregraph workspace build all .
```

## Evidence and Confidence

`architecture-capabilities.json` stores deterministic normalized evidence from
full adapters. `capabilities.json` declares analyzer support, and
`evidence.json` resolves source evidence IDs. Specialist Query and expert MCP
operations can inspect these complete index projections manually; normal agents
consume the bounded Context Pack instead.

Confidence values retain these meanings:

- `RESOLVED`: deterministic route or flow match.
- `MISMATCH`: a nearby match has a concrete incompatibility.
- `PARTIAL_MATCH`: a known normalization produced the match.
- `UNRESOLVED`: indexed data exists, but no safe match was found.
- `OUT_OF_SCOPE`: the record is intentionally not matched to a backend.
- `EXTRACTED`: the value came from source structure.
- `MATCHED`: a test, field, or relation joined to a concrete target.

The additive change-safety fields include `resolution_class`,
`resolution_evidence`, `similar_backend_routes`, `dynamic_endpoint_candidates`,
`equivalent_route_candidates`, `missing_route_kind`, backend/frontend DTO
fields, `auth`, `persistence_path`, and `field_risks`.

## Workspace Navigation and Diff

```bash
goregraph workspace dashboard .
goregraph workspace explain "GET /users/{id}"
goregraph workspace path --from frontend/app --to UserController.get
goregraph workspace impact --changed-file frontend/app/src/api/users.ts
goregraph workspace diff --before <workspace-output> --after <workspace-output>
```

Graph IDs are deterministic from node kind plus normalized semantic parts,
including `project:`, `contract:`, `route:`, `flow:`, and `feature:` IDs. Diff
mode reports new and removed contracts, changed issue/confidence, and lost
matched test coverage.

## Language Inventory

<!-- goregraph:generated language-inventory start -->
GoreGraph provides full adapters for Go, Java / Spring, JavaScript / TypeScript / Node.js / React, PHP, Python, and Rust. They emit normalized symbols, imports, calls, routes, tests, and pattern-backed architecture evidence for their supported static syntax.

Shell integration provides symbols, imports, and calls, but does not provide routes, tests, or architecture facts. Index adapters for C, C++, C#, Kotlin, Ruby, Scala, and Swift provide best-effort declarations and imports only. All records share the Schema 3 index.
<!-- goregraph:generated language-inventory end -->
