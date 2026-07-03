# GoreGraph Commands

This file lists every user-facing GoreGraph command, what it does, and common variations.

## `goregraph help`

Shows global CLI help.

Use when:

- you want to see available commands
- you forgot the basic syntax

Variations:

```bash
goregraph help
goregraph --help
goregraph -h
```

## `goregraph scan <path>`

Creates or fully rebuilds GoreGraph output for a project.

Use when:

- you run GoreGraph for the first time in a project
- source files changed and you want a fresh index
- `doctor`, `query`, or `explain` says the index is missing or stale

Example:

```bash
goregraph scan .
```

Writes to the configured output directory. By default this is:

```text
goregraph-out/
```

Generated files include:

- `manifest.json`
- `files.json`
- `symbols.json`
- `relations.json`
- `graph.json`
- `report.md`
- `modules.md`
- `entrypoints.md`
- `test-map.md`

Variations:

```bash
goregraph scan .
goregraph scan /path/to/project
goregraph scan . --no-update-gitignore
goregraph scan help
goregraph scan --help
```

`--no-update-gitignore` prevents GoreGraph from adding the output directory to the project `.gitignore`.

## `goregraph update`

Refreshes the current project index.

Use when:

- you are already in the project root
- you want to refresh `goregraph-out/` after code changes

Example:

```bash
goregraph update
```

Current behavior:

- performs an explicit full refresh of the current project
- does not install hooks
- does not watch files
- does not run in the background

Variations:

```bash
goregraph update
goregraph update --no-update-gitignore
```

## `goregraph report <path>`

Prints the generated Markdown project report.

Use when:

- you want a quick human-readable project summary
- you want to inspect language/file counts
- you want to confirm a scan output exists

Example:

```bash
goregraph report .
```

Reads:

```text
<path>/<configured-output>/report.md
```

## `goregraph query <path> <term>`

Searches the generated GoreGraph index.

Use when:

- you want to find a file, symbol, language, or relation
- you want quick orientation without scanning source files directly

Examples:

```bash
goregraph query . StartServer
goregraph query . internal/service
goregraph query . go
```

Important behavior:

- reads generated index files
- does not rescan the project
- does not call AI
- returns an actionable error if the index is missing

## `goregraph explain <path> <file-or-symbol>`

Shows indexed context for one file path or symbol name.

Use when:

- you want to understand a specific file
- you want to see symbols in a file
- you want inbound/outbound relations
- you want likely tests for a file

Examples:

```bash
goregraph explain . src/main.go
goregraph explain . StartServer
```

Output sections:

- file metadata
- symbols
- outbound relations
- inbound relations
- likely tests

## `goregraph doctor <path>`

Checks the health of the generated GoreGraph output without scanning.

Use when:

- `query` or `explain` does not work
- you want to verify the generated index before MCP or automation uses it
- you want to know whether the output is missing, broken, unsupported, or stale

Example:

```bash
goregraph doctor .
```

Checks:

- project config can be loaded
- output directory exists
- expected generated files exist
- `manifest.json` is valid
- schema version is supported
- JSON index files are valid
- indexed source hashes still match current files

Exit behavior:

- exits `0` when the index is healthy
- exits `1` when failures or warnings are found

Common output:

```text
OK   output: goregraph-out exists
OK   schema: version 1 supported
WARN stale: 1 indexed files changed or disappeared

Suggested fix:
  goregraph scan .
```

## `goregraph mcp`

Starts the read-only MCP stdio server.

Use when:

- an MCP-capable coding assistant should read an existing GoreGraph index
- you want tool access to query/explain/project-summary functionality
- you want local integration without network listeners or project writes

Example:

```bash
goregraph mcp
```

Important behavior:

- uses stdio
- does not open a network port
- does not scan automatically
- does not write project files
- reads the existing configured GoreGraph output

Provided tools:

- `query_code_map`
- `get_project_summary`
- `get_file`
- `get_symbol`
- `get_related_files`
- `explain_file`
- `doctor`

## Configuration

All commands that read generated output respect `goregraph.yml` when present.

Supported example:

```yaml
version: 1
output: goregraph-out
include:
  - src/**
  - tests/**
exclude:
  - generated/**
max_file_size_kb: 512
follow_symlinks: false
use_gitignore: true
update_gitignore: true
```
