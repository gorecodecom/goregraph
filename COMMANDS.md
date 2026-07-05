# GoreGraph Commands

This file lists every user-facing GoreGraph command, what it does, and common variations.

Installation commands such as `brew install gorecodecom/tap/goregraph` are documented in `README.md`. This file focuses on commands provided by the installed `goregraph` binary.

## Path Model

Most commands accept a `<path>` argument.

`<path>` means the project root that owns the GoreGraph output. It can be:

- `.` for the current working directory
- a relative path such as `../my-app`
- an absolute path such as `/Users/name/projects/my-app`

GoreGraph stores normal index paths relative to that project root. If you scan:

```bash
goregraph scan /Users/name/projects/my-app
```

then a source file is stored as:

```text
src/main.go
```

not as:

```text
/Users/name/projects/my-app/src/main.go
```

This makes output stable across different machines and checkout locations.

By default, scan output is written to:

```text
<path>/goregraph-out/
```

If `goregraph.yml` configures another output directory, commands that read generated output use that configured directory instead.

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

`<path>` must point to the project root you want to analyze. In most cases this is `.` because you run GoreGraph from inside the repository:

```bash
cd /Users/name/projects/my-app
goregraph scan .
```

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

Expected result:

- creates or replaces the generated index files
- adds `goregraph-out/` to the project `.gitignore` unless disabled
- prints a short completion summary
- returns a non-zero exit code if config, filesystem access, or scan safety checks fail

Generated files include:

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
- `workspace.md`
- `endpoints.md`
- `endpoint-flows.md`
- `dependencies.md`
- `callgraph.md`
- `analyzers.md`
- `affected.md`
- `audit.json`
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

What the generated files mean:

- `manifest.json`: metadata about the scan, schema, generated files, and scanned project.
- `files.json`: all indexed files with relative path, language, size, hash, and kind.
- `symbols.json`: extracted packages, classes, functions, methods, tests, scripts, headings, namespaces, and entrypoints.
- `relations.json`: extracted imports, includes, sources, and test relations.
- `graph.json`: a combined node/edge graph from files, symbols, and relations.
- `symbols-full.json`: normalized symbols for all supported languages with stable IDs and source locations.
- `relations-full.json`: normalized relations for all supported languages with confidence and source-location metadata.
- `graph-full.json`: Graphify-like rich directed graph with stable IDs, file nodes, symbol nodes, `type`/`relation` edge metadata, confidence, and source locations.
- `callgraph.json`: method-level Java call graph with extracted and inferred call edges.
- `endpoint-flows.json`: Spring endpoint flow records from endpoint to controller/service/repository methods.
- `test-map.json`: method-level and endpoint-level Java test mappings with confidence metadata.
- `analyzers.json`: active analyzer capability inventory for the scanned project.
- `spring.json`: Spring Boot applications, components, endpoints, dependencies, repositories, entities, and beans detected from Java source.
- `workspace.md`: Maven and Node package/workspace metadata.
- `endpoints.md`: HTTP endpoint inventory for supported backend adapters.
- `endpoint-flows.md`: human-readable endpoint call-flow report.
- `dependencies.md`: human-readable dependency view for supported domain adapters.
- `callgraph.md`: human-readable method call graph.
- `analyzers.md`: human-readable analyzer capability inventory.
- `affected.md`: best-effort high-inbound relation overview for impact orientation.
- `audit.json`: scan audit showing generated files and confirming normal scans used no network and executed no external commands.
- `report.md`: human-readable project overview.
- `modules.md`: top-level directory/module overview.
- `entrypoints.md`: likely app, CLI, script, package, and front-controller entrypoints.
- `test-map.md`: best-effort source/test associations.

Important behavior:

- scans only under the selected project root
- skips default generated/dependency/build directories
- reads the project `.gitignore` as additional exclusions unless disabled in config
- skips binary files and files over the configured size limit
- does not run project code
- does not call AI or network services

## `goregraph update`

Refreshes the current project index.

Use when:

- you are already in the project root
- you want to refresh `goregraph-out/` after code changes

Example:

```bash
goregraph update
```

`update` is intentionally a convenience command for the current directory. It is equivalent to refreshing the project at `.`:

```bash
goregraph scan .
```

Current behavior:

- performs an explicit full refresh of the current project
- does not install hooks
- does not watch files
- does not run in the background

Expected result:

- replaces the current `goregraph-out/` content with a fresh index
- keeps the same output rules as `scan`
- respects `goregraph.yml`
- returns a non-zero exit code if the refresh fails

Variations:

```bash
goregraph update
goregraph update --no-update-gitignore
```

## `goregraph report <path>`

Prints the generated Markdown project report.

`<path>` must point to a project that has already been scanned:

```bash
goregraph scan .
goregraph report .
```

This command reads:

```text
<path>/<configured-output>/report.md
```

With default config and `.` as path, that means:

```text
./goregraph-out/report.md
```

Use when:

- you want a quick human-readable project summary
- you want to inspect language/file counts
- you want to confirm a scan output exists

Example:

```bash
goregraph report .
```

Expected report content:

- project name/root summary
- scan statistics such as indexed and skipped files
- language breakdown
- important directories and top-level areas
- detected build/config files
- pointers to the other generated reports

What the report tells you:

- what kind of project GoreGraph saw
- which languages dominate the repository
- where important project areas appear to be
- whether expected files were indexed
- where to continue looking in `modules.md`, `entrypoints.md`, or `test-map.md`

What the report does not tell you:

- it is not an AI summary
- it does not judge code quality
- it does not guarantee architectural intent
- it does not replace source review

Important behavior:

- reads the generated report only
- does not rescan the project
- returns an actionable error if the report is missing

Common follow-up:

```bash
goregraph doctor .
goregraph scan .
```

Use `doctor` first when you are unsure whether the generated output is missing, stale, or broken.

## `goregraph query <path> <term>`

Searches the generated GoreGraph index.

`<path>` is the scanned project root. `<term>` is the search text.

Use when:

- you want to find a file, symbol, language, or relation
- you want quick orientation without scanning source files directly

Examples:

```bash
goregraph query . StartServer
goregraph query . internal/service
goregraph query . go
goregraph query . graph-full
goregraph query . endpoints
goregraph query . dependencies
goregraph query . audit
```

Searches these generated files:

- `files.json`
- `symbols.json`
- `relations.json`

If `<term>` is a known output alias, `query` prints that generated file directly instead of performing a text search. Supported aliases:

- `files` -> `files.json`
- `symbols` -> `symbols.json`
- `symbols-full` -> `symbols-full.json`
- `relations` -> `relations.json`
- `relations-full` -> `relations-full.json`
- `graph` -> `graph.json`
- `graph-full` -> `graph-full.json`
- `callgraph` -> `callgraph.json`
- `callgraph-md` -> `callgraph.md`
- `report` -> `report.md`
- `modules` -> `modules.md`
- `entrypoints` -> `entrypoints.md`
- `tests` or `test-map` -> `test-map.md`
- `test-map-json` -> `test-map.json`
- `spring` -> `spring.json`
- `workspace` -> `workspace.md`
- `endpoints` -> `endpoints.md`
- `endpoint-flows` -> `endpoint-flows.md`
- `endpoint-flows-json` -> `endpoint-flows.json`
- `dependencies` -> `dependencies.md`
- `analyzers` -> `analyzers.md`
- `analyzers-json` -> `analyzers.json`
- `affected` -> `affected.md`
- `audit` -> `audit.json`

Matches can include:

- file paths such as `internal/scan/extract_go.go`
- language names such as `python`
- file kinds such as `build`
- symbol names such as `StartServer`
- relation targets such as imported modules or source files

Expected output:

- grouped text results
- enough context to identify matching files, symbols, and relations
- no source file contents

Important behavior:

- reads generated index files
- does not rescan the project
- does not call AI
- returns an actionable error if the index is missing

Use `query` when you know roughly what you are looking for. Use `explain` when you want context for one specific file or symbol.

## `goregraph explain <path> <file-or-symbol>`

Shows indexed context for one file path or symbol name.

`<path>` is the scanned project root. `<file-or-symbol>` can be:

- a root-relative file path from `files.json`
- a symbol name from `symbols.json`

Examples:

```bash
goregraph explain . src/main.go
goregraph explain . StartServer
```

Use when:

- you want to understand a specific file
- you want to see symbols in a file
- you want inbound/outbound relations
- you want likely tests for a file

Output sections:

- file metadata
- symbols
- outbound relations
- inbound relations
- likely tests

What the sections mean:

- file metadata: the indexed file path, language, kind, size, and content hash.
- symbols: known classes, functions, methods, tests, headings, scripts, namespaces, or entrypoints inside that file.
- outbound relations: what the file points to, imports, includes, sources, or tests.
- inbound relations: which indexed files point to this file or symbol target.
- likely tests: best-effort test files associated with the file.

Expected output:

- a compact text explanation from generated index data
- no AI-generated interpretation
- no source file execution

Important behavior:

- reads generated index files only
- does not rescan the project
- works best after a fresh `goregraph scan`
- returns an actionable error if output is missing or malformed

## `goregraph doctor <path>`

Checks the health of the generated GoreGraph output without scanning.

`<path>` must point to the scanned project root whose output should be checked.

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

What the result tells you:

- `OK` means that a check passed.
- `WARN` means the index may still be readable but should probably be refreshed.
- `FAIL` means the output is missing, invalid, unsupported, or unsafe to rely on.

Typical warnings:

- indexed source files changed since the last scan
- indexed source files disappeared
- generated files are incomplete

Typical fix:

```bash
goregraph scan .
```

## `goregraph mcp`

Starts the read-only MCP stdio server.

This command is meant to be started by an MCP-capable coding assistant or editor integration, not used as an interactive shell command by a human.

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

Expected usage flow:

```bash
goregraph scan .
goregraph doctor .
```

Then configure the MCP client to run:

```bash
goregraph mcp
```

Provided tools:

- `query_code_map`
- `get_project_summary`
- `get_file`
- `get_symbol`
- `get_related_files`
- `explain_file`
- `doctor`

Tool behavior:

- `query_code_map`: searches the generated index for a term.
- `get_project_summary`: reads the generated `report.md`.
- `get_file`: returns indexed metadata for one file.
- `get_symbol`: finds indexed symbols by name.
- `get_related_files`: returns indexed relation context for one file.
- `explain_file`: returns the same kind of explanation as `goregraph explain`.
- `doctor`: checks whether generated output is usable.

Important limitation:

- MCP reads existing output only. If the index is stale or missing, run `goregraph scan .` manually.

## `goregraph version`

Prints GoreGraph build metadata.

Use when:

- you want to confirm which GoreGraph binary is installed
- you are reporting a bug
- you need to check which schema version the binary supports
- you want to verify release build metadata

Example:

```bash
goregraph version
```

Expected output:

```text
goregraph 0.1.1
commit: dev
built: unknown
go: go1.23.x
platform: darwin/arm64
schema: 1
```

Field meaning:

- `goregraph`: the CLI name and semantic version.
- `commit`: the Git commit embedded by release builds.
- `built`: the build timestamp embedded by release builds.
- `go`: the Go runtime version used to build the binary.
- `platform`: the operating system and CPU architecture of the binary.
- `schema`: the GoreGraph output schema version supported by this binary.

Important behavior:

- does not read project files
- does not require a GoreGraph project
- does not call network services

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
