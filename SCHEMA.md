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
- no timestamps in deterministic files
- no random IDs

## Generated Files

Schema version 1 expects:

- `manifest.json`
- `files.json`
- `symbols.json`
- `relations.json`
- `graph.json`
- `report.md`
- `modules.md`
- `entrypoints.md`
- `test-map.md`

## File Contracts

`manifest.json` describes the generated output set.

`files.json` lists indexed files with path, language, size, hash, and kind.

`symbols.json` lists extracted symbols with name, kind, file, and line.

`relations.json` lists extracted relationships with source file, target, type, and line.

`graph.json` combines files, symbols, local file targets, and external dependency nodes.

Markdown reports are human-readable and deterministic, but not intended as strict machine APIs.

## Language Records

Schema version 1 may contain language-specific symbols and relations. Current symbol kinds include packages, modules, classes, interfaces, traits, functions, methods, tests, scripts, headings, namespaces, autoload hints, types, and entrypoints.

Current relation types include imports, includes, sources, and tests. Local Go, Python, PHP, and Shell relations are resolved to root-relative files where GoreGraph can do so deterministically.
