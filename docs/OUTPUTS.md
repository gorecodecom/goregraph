# GoreGraph Output Contract

GoreGraph 1.3 uses output Schema 3. Outputs are additive: new versions may add
fields, but must not silently repurpose existing field meanings.

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
  interactive seven-view dashboard is workspace-only in 1.3.0.

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
│   └── symbol-usages.json
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

Use GoreGraph once to obtain a bounded navigation pack, then verify cited
source:

```bash
goregraph context . --query "<current coding task>" --budget-tokens 1800 --max-files 12
```

Standard MCP exposes exactly one tool, `task_context`, with equivalent values.

1. Call Context once for the current coding task.
2. Read only cited file ranges needed to verify the result.
3. If `fallback_required` is true, stop using GoreGraph immediately and inspect
   source.
4. If confidence is low, or the first result does not return one exact route or
   symbol, stop and inspect source.
5. Only when confidence is not low and one exact route or symbol was returned,
   allow one narrower retry using that exact value.
6. After that retry, inspect source. There is no third Context call and no
   specialist-query fallback cascade.
7. Run `goregraph doctor .` only when Context reports missing or stale output.

The “at most twice” ceiling applies to Context retrieval for one coding task.
Context token estimates are approximate; benchmark totals are authoritative.
Existing specialist CLI queries remain available for manual compatibility, but
are not part of the normal agent workflow. `goregraph mcp --expert-tools`
exposes legacy diagnostic and exploration tools for explicit manual use; they
are not substitutes for a third Context call.

## Human Dashboard

`.goregraph-workspace/dashboard/workspace-map.html` is the Schema 3 standalone
offline dashboard. It contains Architecture, Endpoints, Feature Flow, Data
Flow, Code Explorer, Diagnostics, and Coverage views.

- Architecture derives dynamic domain lanes, keeps stable card coordinates,
  and distinguishes statically detected relationships from runtime traffic.
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
goregraph query . symbol-resolve --query com.weka.UserService --format json --limit 20
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

GoreGraph has deep route/API/test analyzers for Go, Java/Spring,
JavaScript/TypeScript/Node.js/React, Python, PHP, and Shell. It also indexes
best-effort symbols and imports for Rust, Kotlin, Scala, Swift, Ruby, C, C++,
and C# in the shared Schema 3 index.
