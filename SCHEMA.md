# GoreGraph Schema

GoreGraph output is designed to be deterministic and safe for humans, CLI commands, and read-only integrations.

## Current Schema

Current schema version:

```text
1
```

The schema version is written to:

```text
manifest.json
```

Example:

```json
{
  "tool": "goregraph",
  "schema": 1,
  "output_dir": "goregraph-out"
}
```

## Compatibility Rule

GoreGraph commands only support the current schema version.

If generated output uses an unsupported schema, commands such as `doctor`, `query`, and MCP mode should report an actionable error and ask the user to refresh the output:

```bash
goregraph scan .
```

## Determinism Rules

Generated output should stay stable when repository content, config, and GoreGraph version are unchanged.

Rules:

- root-relative paths only
- `/` path separators
- sorted files
- sorted symbols
- sorted relations
- stable JSON indentation
- timestamps only in metadata/audit files where the timestamp is the purpose of the file
- no random IDs

## Generated Files

Schema version 1 expects:

- `manifest.json`
- `files.json`
- `symbols.json`
- `relations.json`
- `graph.json`
- `symbols-full.json`
- `relations-full.json`
- `graph-full.json`
- `callgraph.json`
- `endpoint-flows.json`
- `test-map.json`
- `routes.json`
- `flows.json`
- `api-contracts.json`
- `contract-matches.json`
- `diagnostics.json`
- `package-graph.json`
- `maven-graph.json`
- `analyzers.json`
- `spring.json`
- `audit.json`
- `report.md`
- `modules.md`
- `workspace.md`
- `endpoints.md`
- `endpoint-flows.md`
- `dependencies.md`
- `callgraph.md`
- `routes.md`
- `flows.md`
- `api-contracts.md`
- `contract-matches.md`
- `potentially-broken-contracts.md`
- `diagnostics.md`
- `workspace-context.md`
- `workspace-contract-matches.md`
- `workspace-feature-flows.json`
- `workspace-feature-flows.md`
- `frontend-consumers.md`
- `package-graph.md`
- `maven-graph.md`
- `navigation.md`
- `analyzers.md`
- `affected.md`
- `entrypoints.md`
- `test-map.md`

## File Contracts

`manifest.json` describes the generated output set.

`files.json` lists indexed files with path, language, size, hash, and kind.

`symbols.json` lists extracted symbols with name, kind, file, and line.

`relations.json` lists extracted relationships with source file, target, type, and line.

`graph.json` combines files, symbols, local file targets, and external dependency nodes.

`symbols-full.json` contains additive normalized symbol records with stable IDs, language, source file, and source location.

`relations-full.json` contains additive normalized relation records with stable IDs, relation type, source location, confidence, confidence score, and best-effort internal/external classification.

`graph-full.json` contains a richer directed graph inspired by Graphify-style node/edge interchange. It preserves root-relative source files and marks extracted relationships with `EXTRACTED` confidence. Rich graph edges expose `type`; `relation` remains as a compatibility alias.

`callgraph.json` is the authoritative method/function-level call graph. Java/Spring exact method declaration matches use `EXTRACTED`; language-neutral Go, PHP, JavaScript, TypeScript/React, Python, and Shell call matches use `INFERRED`. `relations.json` and `graph.json` may include a subset of call relations for broad graph navigation, but tools should use `callgraph.json` when they need method-level calls.

`endpoint-flows.json` contains Spring endpoint flow records from endpoint to controller, service, repository, and other resolved method steps.

`test-map.json` contains Java and language-neutral test mappings. Direct Java method calls use `EXTRACTED`; endpoint matches and generic test-to-production call matches use `INFERRED`.

`routes.json` contains normalized route records. Current route sources include Spring, Go `net/http`/router calls, PHP Laravel-style routes, JavaScript/TypeScript Express/Fastify-style routes, React Router routes, Redux Little Router fragments, and Python FastAPI/Flask-style decorators. Frontend routes include app-specific `route_id` values such as `portal:/settings` when they are inside `apps/<name>/...`.

`flows.json` contains normalized route-to-handler-to-call flow records. Flow steps are best-effort static orientation data and include confidence markers.

`api-contracts.json` contains JavaScript/TypeScript HTTP client usage detected from supported helper calls and `fetch`. Records include HTTP method, raw path, normalized path, query string, sorted query params, service candidate, enclosing caller function or method when detected, caller line, app/package context, confidence, and reason. Supported helper calls include direct and multiline argument forms where a literal path argument is visible, for example `GetHelper(dispatch, "/service/path")`. Template placeholders such as `${id}` normalize to `{id}`. Complex dynamic expressions such as ternaries are marked with `unsafe_dynamic` and normalized to `{dynamic}`.

`contract-matches.json` compares frontend API contracts with backend route records from the same scan. Match records include API method/path/location, backend method/path/handler/location when available, service candidate, issue, confidence, confidence score, and reason. API contracts also include `caller` when the helper or fetch call is inside a detected JavaScript/TypeScript function or method. Issue values currently include `matched`, `method_mismatch`, `missing_backend_route`, `unscanned_service`, and `unsafe_dynamic`. `unscanned_service` means the frontend call references a recognized service candidate whose backend routes were not present in this scan, so it should not be treated as a broken route inside the scanned backend scope.

`diagnostics.json` contains a compact diagnosis index with `entrypoints`, `risky_contracts`, `workspace_resolved_contracts`, `unscanned_services`, `endpoints_without_tests`, `weak_flows`, and `likely_tests`. It is derived from existing route, contract, endpoint-flow, flow, test-map, and workspace overlay facts.

Workspace files are additive generated outputs. When a workspace is detected, `.goregraph-workspace/registry.json` stores discovered projects with `current`, `indexed`, or `not_indexed` status. `.goregraph-workspace/context.json` stores loaded indexes, known backend services, referenced but missing services, and `missing_service_details` entries with service name, referenced contract count, matching workspace project path, and project status when available. `.goregraph-workspace/contract-matches.json` stores cross-project API contract matches between already indexed projects and may include `api_caller` from the originating API contract. `.goregraph-workspace/feature-flows.json` stores resolved end-to-end feature flows from frontend route/component/API call to backend endpoint flow and matching tests. Feature-flow records may include `frontend_route_id`, `frontend_route_path`, `frontend_route_file`, `frontend_route_line`, `frontend_component`, `frontend_caller`, `frontend_steps`, `frontend_confidence`, and `frontend_reason`; `frontend_caller` can come from either the resolved route flow or the API contract caller when route context remains weak. `frontend_steps` can include lightweight JavaScript/TypeScript callgraph steps such as route handlers, function calls, JSX child component hops, React effect calls, and local event handler calls that connect a rendered component to an API caller. Existing indexed siblings receive `workspace-context.md`, `workspace-contract-matches.md`, `workspace-feature-flows.json`, `workspace-feature-flows.md`, and `frontend-consumers.md` overlay reports in their configured output directories. The readable workspace reports show API caller names in contract matches, frontend consumers, and backend endpoint consumers when available; `workspace-context.md` prioritizes missing services by contract count and suggests scan commands for discovered unindexed service projects. Workspace reconciliation may also update `diagnostics.json`, `diagnostics.md`, and `endpoints.md` with workspace-resolved contracts and frontend consumers. These overlays are regenerated from existing scan output and do not imply sibling projects were rescanned.

`package-graph.json` contains Node workspace package nodes and package dependency edges extracted from `package.json`. Internal workspace edges use reason `workspace-package-json-dependency`.

`maven-graph.json` contains Maven module/dependency nodes and dependency edges extracted from `pom.xml`. Edges use reason `pom-dependency`.

`analyzers.json` describes which language/workspace analyzers were active for the scanned project and which capabilities they provided.

`spring.json` contains Java/Spring domain records. It is empty when no Spring facts are detected.

`audit.json` records the scan command, generated files, file counts, timestamps, and safety flags. Normal scans set `network_used` and `external_commands` to `false`.

`workspace.md`, `endpoints.md`, `endpoint-flows.md`, `dependencies.md`, `callgraph.md`, `routes.md`, `flows.md`, `api-contracts.md`, `contract-matches.md`, `potentially-broken-contracts.md`, `diagnostics.md`, `package-graph.md`, `maven-graph.md`, `navigation.md`, `analyzers.md`, and `affected.md` are deterministic human-readable reports. `affected.md` focuses on local file targets and filters external dependency labels. Workspace overlay Markdown files are deterministic for the currently available sibling indexes, but they can change when another project in the same workspace is scanned later.

Markdown reports are human-readable and deterministic, but not intended as strict machine APIs.

## Confidence Values

GoreGraph confidence values are static-analysis labels, not runtime proof:

- `EXTRACTED`: the fact was directly extracted from source syntax.
- `RESOLVED`: multiple static facts were connected with a deterministic match, for example frontend API method/path to backend route method/path.
- `INFERRED`: the fact was inferred from local naming, call, test, or ownership heuristics.
- `WEAK_MATCH`: GoreGraph found a possible relationship or issue, but the source expression is dynamic, incomplete, or only loosely compatible.
- `OUT_OF_SCOPE`: GoreGraph recognized a referenced service candidate, but that backend service was not represented by scanned routes.

## Language Records

Schema version 1 may contain language-specific symbols and relations. Current symbol kinds include packages, modules, classes, interfaces, traits, functions, methods, tests, scripts, headings, namespaces, autoload hints, types, and entrypoints.

Current relation types include imports, imports_internal, imports_external, includes, sources, calls, and tests. Local Go, Python, PHP, Shell, and Java relations are resolved to root-relative files where GoreGraph can do so deterministically.
