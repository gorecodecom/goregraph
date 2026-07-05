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

`api-contracts.json` contains JavaScript/TypeScript HTTP client usage detected from supported helper calls and `fetch`. Records include HTTP method, path, caller line, app/package context, confidence, and reason. Supported helper calls include direct and multiline argument forms where a literal path argument is visible, for example `GetHelper(dispatch, "/service/path")`.

`package-graph.json` contains Node workspace package nodes and package dependency edges extracted from `package.json`. Internal workspace edges use reason `workspace-package-json-dependency`.

`maven-graph.json` contains Maven module/dependency nodes and dependency edges extracted from `pom.xml`. Edges use reason `pom-dependency`.

`analyzers.json` describes which language/workspace analyzers were active for the scanned project and which capabilities they provided.

`spring.json` contains Java/Spring domain records. It is empty when no Spring facts are detected.

`audit.json` records the scan command, generated files, file counts, timestamps, and safety flags. Normal scans set `network_used` and `external_commands` to `false`.

`workspace.md`, `endpoints.md`, `endpoint-flows.md`, `dependencies.md`, `callgraph.md`, `routes.md`, `flows.md`, `api-contracts.md`, `package-graph.md`, `maven-graph.md`, `navigation.md`, `analyzers.md`, and `affected.md` are deterministic human-readable reports.

Markdown reports are human-readable and deterministic, but not intended as strict machine APIs.

## Language Records

Schema version 1 may contain language-specific symbols and relations. Current symbol kinds include packages, modules, classes, interfaces, traits, functions, methods, tests, scripts, headings, namespaces, autoload hints, types, and entrypoints.

Current relation types include imports, imports_internal, imports_external, includes, sources, calls, and tests. Local Go, Python, PHP, Shell, and Java relations are resolved to root-relative files where GoreGraph can do so deterministically.
