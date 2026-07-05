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

`callgraph.json` contains method-level Java call graph edges. Exact method declaration matches use `EXTRACTED`; inferred framework/repository calls use `INFERRED`.

`endpoint-flows.json` contains Spring endpoint flow records from endpoint to controller, service, repository, and other resolved method steps.

`test-map.json` contains Java test mappings. Direct method calls use `EXTRACTED`; endpoint matches from MockMvc-style HTTP calls use `INFERRED`.

`analyzers.json` describes which language/workspace analyzers were active for the scanned project and which capabilities they provided.

`spring.json` contains Java/Spring domain records. It is empty when no Spring facts are detected.

`audit.json` records the scan command, generated files, file counts, timestamps, and safety flags. Normal scans set `network_used` and `external_commands` to `false`.

`workspace.md`, `endpoints.md`, `endpoint-flows.md`, `dependencies.md`, `callgraph.md`, `analyzers.md`, and `affected.md` are deterministic human-readable reports.

Markdown reports are human-readable and deterministic, but not intended as strict machine APIs.

## Language Records

Schema version 1 may contain language-specific symbols and relations. Current symbol kinds include packages, modules, classes, interfaces, traits, functions, methods, tests, scripts, headings, namespaces, autoload hints, types, and entrypoints.

Current relation types include imports, imports_internal, imports_external, includes, sources, calls, and tests. Local Go, Python, PHP, Shell, and Java relations are resolved to root-relative files where GoreGraph can do so deterministically.
