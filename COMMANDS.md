# GoreGraph Commands

This file lists every user-facing GoreGraph command, what it does, and common variations.

Installation commands such as `brew install gorecodecom/tap/goregraph` are documented in `README.md`. This file focuses on commands provided by the installed `goregraph` binary.

For release acceptance or whenever generated workspace output may come from an older binary, verify the executable first and rebuild from a clean workspace:

```bash
command -v goregraph
goregraph version
goregraph workspace clean .
goregraph workspace clean . --execute
goregraph workspace scan-all .
goregraph workspace status .
goregraph workspace dashboard .
```

Review the dry-run output from `workspace clean` before adding `--execute`. `workspace refresh` only rebuilds overlays from existing project indexes; it is not a replacement for a clean scan when validating a new GoreGraph version.

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
- adds generated GoreGraph output paths to relevant `.gitignore` files unless disabled
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
- `routes.json`
- `flows.json`
- `api-contracts.json`
- `frontend-usage.json`
- `contract-matches.json`
- `diagnostics.json`
- `package-graph.json`
- `maven-graph.json`
- `analyzers.json`
- `spring.json`
- `workspace.md`
- `endpoints.md`
- `endpoint-flows.md`
- `dependencies.md`
- `callgraph.md`
- `routes.md`
- `flows.md`
- `api-contracts.md`
- `frontend-usage.md`
- `contract-matches.md`
- `potentially-broken-contracts.md`
- `diagnostics.md`
- `workspace-context.md`
- `workspace-contract-matches.json`
- `workspace-contract-matches.md`
- `workspace-feature-flows.json`
- `workspace-feature-flows.md`
- `workspace-next-actions.md`
- `frontend-consumers.md`
- `package-graph.md`
- `maven-graph.md`
- `navigation.md`
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
goregraph scan . --no-workspace
goregraph scan . --workspace /path/to/workspace
goregraph scan help
goregraph scan --help
```

`--no-update-gitignore` prevents GoreGraph from adding generated output entries to project and workspace root `.gitignore` files.

`--no-workspace` disables workspace discovery and skips `.goregraph-workspace/` plus sibling overlay refreshes.

`--workspace <path>` forces the workspace root used to discover sibling projects and existing `goregraph-out/` indexes.

Without `--workspace`, GoreGraph scores ancestor directories and prefers a parent that contains both frontend and backend group directories, such as `frontend/` plus `microservices/`, over an intermediate frontend-only grouping folder.

What the generated files mean:

- `manifest.json`: metadata about the scan, schema, generated files, and scanned project.
- `files.json`: all indexed files with relative path, language, size, hash, and kind.
- `symbols.json`: extracted packages, classes, functions, methods, tests, scripts, headings, namespaces, and entrypoints.
- `relations.json`: extracted imports, includes, sources, and test relations.
- `graph.json`: a combined node/edge graph from files, symbols, and relations.
- `symbols-full.json`: normalized symbols for all supported languages with stable IDs and source locations.
- `relations-full.json`: normalized relations for all supported languages with confidence and source-location metadata.
- `graph-full.json`: Graphify-like rich directed graph with stable IDs, file nodes, symbol nodes, `type`/`relation` edge metadata, confidence, and source locations.
- `callgraph.json`: method/function-level call graph with extracted Java/Spring edges and inferred Go, PHP, JS/TS/React, Python, and Shell call edges.
- `endpoint-flows.json`: Spring endpoint flow records from endpoint to controller/service/repository methods.
- `test-map.json`: method-level, endpoint-level, and best-effort cross-language test mappings with confidence metadata.
- `routes.json`: normalized route records for Spring, Go, PHP/Laravel-style routes, JS/TS Express/Fastify-style routes, React Router routes, and Python FastAPI/Flask-style routes.
- `flows.json`: normalized route-to-handler-to-call flow records across supported languages.
- `api-contracts.json`: JavaScript/TypeScript HTTP client calls detected from supported helpers and `fetch`, including realistic helper argument shapes, method, raw path, normalized path, query metadata, service candidate, enclosing caller function or method when available, file, app, confidence, and reason.
- `frontend-usage.json`: frontend API usage chains from API contract back to the best matching frontend route flow, including route, component, API caller, confidence, and static evidence steps.
- `contract-matches.json`: static frontend API call to backend route matches, including resolved method/path matches, method mismatches, missing backend routes, unscanned services, and unsafe dynamic URL patterns.
- `diagnostics.json`: compact diagnosis index derived from routes, contracts, endpoint flows, route flows, and tests.
- `package-graph.json`: Node workspace package nodes and package dependency edges from `package.json`.
- `maven-graph.json`: Maven package nodes and dependency edges from `pom.xml`.
- `analyzers.json`: active analyzer capability inventory for the scanned project.
- `spring.json`: Spring Boot applications, components, endpoints, dependencies, repositories, entities, and beans detected from Java source.
- `workspace.md`: Maven and Node package/workspace metadata.
- `endpoints.md`: HTTP endpoint inventory for supported backend adapters.
- `endpoint-flows.md`: human-readable endpoint call-flow report.
- `dependencies.md`: human-readable dependency view for supported domain adapters.
- `callgraph.md`: human-readable method call graph.
- `routes.md`: human-readable route inventory.
- `flows.md`: human-readable route and handler flow report.
- `api-contracts.md`: human-readable API client call inventory, including the enclosing caller function or method when detected.
- `frontend-usage.md`: readable frontend route/component/API usage chains with confidence and evidence.
- `contract-matches.md`: human-readable frontend API to backend route match report.
- `potentially-broken-contracts.md`: focused report for API calls that could not be safely matched to backend routes.
- `diagnostics.md`: prioritized human-readable diagnosis report with entrypoints, risky contracts, workspace-resolved contracts, unscanned services, untested endpoints, weak flows, and likely tests.
- `workspace-context.md`: readable workspace project/index summary with missing services prioritized by referenced contract count and scan suggestions when a matching workspace project is known, or a no-workspace placeholder.
- `workspace-contract-matches.md`: readable cross-project contract matches relevant to a scanned project, including API caller names when detected.
- `workspace-feature-flows.json`: cross-project feature flows from frontend route/component/API call to backend endpoint flow and tests, including JSX child component hops, React effect calls, and local event handlers when those steps connect a route component to the API caller.
- `workspace-feature-flows.md`: readable end-to-end feature-flow report, including frontend route context when resolved, confidence reasons such as direct/effect/event-handler API caller matches, and reasons for unresolved route context or missing linked tests.
- `workspace-next-actions.md`: workspace coverage summary with high-value missing service scans, weak workspace matches, and resolved flows without linked tests.
- `frontend-consumers.md`: backend-oriented view of frontend API callers with caller names when detected; frontend projects explain that this report is not applicable and point to workspace contract/feature reports.
- `package-graph.md`: human-readable Node package/workspace dependency graph.
- `maven-graph.md`: human-readable Maven dependency graph.
- `navigation.md`: human-readable starting-point report with likely routes, central files, important symbols, test orientation, and analyzer coverage.
- `analyzers.md`: human-readable analyzer capability inventory.
- `affected.md`: best-effort local-file impact overview that filters external dependency noise.
- `audit.json`: scan audit showing generated files and confirming normal scans used no network and executed no external commands.
- `report.md`: human-readable project overview.
- `modules.md`: top-level directory/module overview.
- `entrypoints.md`: likely app, CLI, script, package, and front-controller entrypoints.
- `test-map.md`: best-effort source/test associations.

Workspace overlays:

- `.goregraph-workspace/registry.json`: discovered sibling projects with `current`, `indexed`, or `not_indexed` status.
- `.goregraph-workspace/context.json`: loaded indexes, known backend services, and referenced but missing services with contract counts and matching workspace project status when known.
- `.goregraph-workspace/contract-matches.json`: cross-project API-to-backend matches from already scanned projects, including the API caller name when detected.
- `.goregraph-workspace/feature-flows.json`: cross-project frontend route/component/API-to-backend feature flows from already scanned projects, with component-aware frontend route steps, React effect calls, and local event handler calls when available.
- `.goregraph-workspace/next-actions.md`: workspace coverage summary and prioritized follow-up actions from already scanned project indexes.
- `workspace-context.md`: readable workspace project/index summary with prioritized missing services and suggested next scans.
- `workspace-contract-matches.json`: project-local cross-project contract matches relevant to the scanned project, refreshed after later sibling scans.
- `workspace-contract-matches.md`: readable cross-project contract matches relevant to a scanned project, including API caller names when detected.
- `workspace-feature-flows.md`: readable frontend-route-to-API-to-backend-to-test feature flows, including JSX component hops, effect/event-handler reasons, API caller fallback for weak route matches, unresolved-route reasons, and missing-test reasons.
- `workspace-next-actions.md`: project-local copy of the workspace next-action summary.
- `frontend-consumers.md`: backend-oriented view of frontend API callers with caller names when detected; frontend projects explain the report scope instead of showing an ambiguous empty result.

Workspace reconciliation also refreshes:

- `diagnostics.md` and `diagnostics.json` with outgoing frontend contracts and incoming backend consumers for the project.
- `endpoints.md` with a `Frontend Consumers` section for backend projects.

When workspace discovery is active, `scan` also adds `.goregraph-workspace/` to the detected workspace root `.gitignore` unless `--no-update-gitignore` is used.

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
goregraph query . api-contracts
goregraph query . frontend-usage
goregraph query . contract-matches
goregraph query . broken-contracts
goregraph query . diagnostics
goregraph query . package-graph
goregraph query . maven-graph
goregraph query . workspace-contracts
goregraph query . workspace-features
goregraph query . audit
```

Workspace aliases can be read from either a scanned project root or the workspace root:

```bash
cd /Users/name/projects/weka
goregraph query . workspace-context
goregraph query . workspace-contracts
goregraph query . workspace-features
goregraph query . workspace-next-actions
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
- `routes` -> `routes.md`
- `routes-json` -> `routes.json`
- `flows` -> `flows.md`
- `flows-json` -> `flows.json`
- `api-contracts` -> `api-contracts.md`
- `api-contracts-json` -> `api-contracts.json`
- `frontend-usage` -> `frontend-usage.md`
- `frontend-usage-json` -> `frontend-usage.json`
- `contract-matches` -> `contract-matches.md`
- `contracts` -> `contract-matches.md`
- `contract-matches-json` -> `contract-matches.json`
- `broken-contracts` -> `potentially-broken-contracts.md`
- `diagnostics` -> `diagnostics.md`
- `diagnostics-json` -> `diagnostics.json`
- `package-graph` -> `package-graph.md`
- `package-graph-json` -> `package-graph.json`
- `maven-graph` -> `maven-graph.md`
- `maven-graph-json` -> `maven-graph.json`
- `navigation` -> `navigation.md`
- `spring` -> `spring.json`
- `workspace` -> `workspace.md`
- `workspace-context` -> `workspace-context.md`
- `workspace-contracts` -> `workspace-contract-matches.md`
- `workspace-features` or `workspace-feature-flows` -> `workspace-feature-flows.md`
- `workspace-feature-flows-json` -> `workspace-feature-flows.json`
- `workspace-next-actions` -> `workspace-next-actions.md`
- `frontend-consumers` -> `frontend-consumers.md`
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

## `goregraph workspace status [path]`

Shows the workspace GoreGraph would use for a project without scanning or writing files.

Use when:

- you want to confirm the auto-detected workspace root
- you want to see which sibling projects are already indexed
- you want to know which backend services are known from existing scans

Examples:

```bash
goregraph workspace status .
goregraph workspace status frontend/frontend-monorepo
goregraph workspace status . --workspace /Users/name/projects/weka
```

Expected output:

- workspace root
- current project
- discovered sibling projects with status
- loaded indexes
- known backend services
- referenced but missing services, prioritized by contract count
- suggested `goregraph scan .` commands for missing services whose workspace project was discovered

Important behavior:

- does not scan
- does not write files
- uses the same auto-detection as `goregraph scan`

## `goregraph workspace scan-missing [path]`

Shows or executes prioritized scans for referenced backend services that are discovered in the workspace but not indexed yet.

Use when:

- `workspace-next-actions.md` shows many `unscanned_service` contracts
- you want better frontend-to-backend coverage without manually picking services
- you want to scan only the highest-value missing services first

Examples:

```bash
goregraph workspace scan-missing .
goregraph workspace scan-missing . --top 5
goregraph workspace scan-missing . --top 5 --execute
goregraph workspace scan-missing frontend/frontend-monorepo --workspace /Users/name/projects/weka
```

Default behavior:

- dry run only
- scans nothing
- writes nothing
- shows the top 5 missing service projects ranked by referenced contract count

Options:

- `--top N`: limit the plan to the first N missing services.
- `--execute`: run the scans for the planned projects.
- `--workspace <path>`: force the workspace root used for discovery.
- `--no-update-gitignore`: skip generated-output `.gitignore` updates while executing scans.

Execution behavior:

- scans each selected missing service project with normal GoreGraph scan logic
- refreshes workspace overlays after each scan
- keeps existing project output locations from each project's config
- stops on the first scan error and reports the failing project

## `goregraph workspace scan-all [path]`

Scans every project discovered in the detected workspace.

Use when:

- you want a fresh index for the full workspace
- frontend and backend source changed and all project-local indexes should be rebuilt
- you want workspace overlays refreshed after each project scan

Examples:

```bash
goregraph workspace scan-all .
goregraph workspace scan-all . --dry-run
goregraph workspace scan-all frontend/frontend-monorepo --workspace /Users/name/projects/weka
goregraph workspace scan-all . --no-update-gitignore
```

Default behavior:

- scans every discovered workspace project
- refreshes workspace overlays after each project scan
- keeps each project's configured output directory
- stops on the first scan error and reports the failing project

Options:

- `--dry-run`: show the scan plan without scanning or writing files.
- `--workspace <path>`: force the workspace root used for discovery.
- `--no-update-gitignore`: skip generated-output `.gitignore` updates while scanning.

## `goregraph workspace clean [path]`

Shows or removes generated GoreGraph output for a detected workspace.

Use when:

- you want to delete all project `goregraph-out/` directories in a workspace
- you want to delete the workspace-level `.goregraph-workspace/` overlay directory
- you want a clean rebuild before running `goregraph workspace scan-all`

Examples:

```bash
goregraph workspace clean .
goregraph workspace clean . --execute
goregraph workspace clean . --workspace /Users/name/projects/weka
```

Default behavior:

- dry run only
- deletes nothing
- lists project output directories and the workspace `.goregraph-workspace/` directory

Options:

- `--execute`: remove the listed generated output paths.
- `--workspace <path>`: force the workspace root used for discovery.

## `goregraph workspace refresh [path]`

Rebuilds workspace overlays from existing project indexes without scanning source files.

Use when:

- project indexes are already current
- workspace-level context or cross-project overlays need to be regenerated
- you deliberately want a faster overlay-only update

Examples:

```bash
goregraph workspace refresh .
goregraph workspace refresh frontend/frontend-monorepo --workspace /Users/name/projects/weka
```

Important behavior:

- reads existing project outputs
- does not rescan source files
- does not prove that project indexes were produced by the current binary
- must not replace `workspace clean . --execute` followed by `workspace scan-all .` during release acceptance

## `goregraph workspace dashboard [path]`

Prints the path to the generated standalone workspace dashboard.

Example:

```bash
goregraph workspace dashboard .
```

The dashboard is normally generated at:

```text
<workspace>/.goregraph-workspace/workspace-map.html
```

The 0.9.2 dashboard is organized around four views:

- **Architecture** is the first and default view. Selecting a service highlights its direct incoming and outgoing relationships without moving the full map. **Isolate neighborhood** explicitly narrows the graph; **Show full architecture** restores it.
- **Endpoints** shows the endpoint inventory for a selected service and opens a directed implementation trace for a selected endpoint. Selecting a trace step focuses that point in the path, and **Back to endpoint inventory** restores the inventory context.
- **Diagnostics** explains relationships GoreGraph could not safely confirm, including the classification, reason, possible impact, evidence, and suggested next check. Expected frontend-internal behavior is distinguished from likely defects or incomplete scan coverage.
- **Coverage** shows analyzer support per project, language, and capability as `COMPLETE`, `PARTIAL`, `UNAVAILABLE`, or `FAILED`. It describes analysis coverage, not whether source behavior exists.

Important behavior:

- prints an existing generated dashboard path; it does not launch a browser
- does not scan source files
- requires workspace output generated by `scan`, `workspace scan-all`, or `workspace refresh`
- the dashboard is self-contained and works offline

## `goregraph workspace explain <target>`

Explains a workspace route, file, symbol, contract, or feature using generated workspace evidence.

Examples:

```bash
goregraph workspace explain "GET /users/{userId}"
goregraph workspace explain UserController.get
goregraph workspace explain frontend/app/src/api/users.ts
```

Use this before reading broad areas of source code when you need a compact cross-project orientation for one target.

Important behavior:

- reads generated workspace indexes and overlays
- does not rescan or execute project code
- reports available evidence and uncertainty rather than inventing missing relationships

## `goregraph workspace path --from <target> --to <target>`

Finds a directed path between two workspace targets.

Examples:

```bash
goregraph workspace path --from frontend/app --to UserController.get
goregraph workspace path --from "GET /users/{userId}" --to UserRepository.findById
```

Use when you want to understand how a consumer, route, handler, symbol, or service is connected to another known target.

Important behavior:

- searches the generated workspace graph only
- returns a bounded evidence-backed path when one is known
- an absent path means GoreGraph could not establish one from the current indexes, not necessarily that no runtime relationship exists

## `goregraph workspace impact --changed-file <path>`

Shows workspace features and relationships that may be affected by one or more changed files.

Examples:

```bash
goregraph workspace impact --changed-file frontend/app/src/api/users.ts
goregraph workspace impact --changed-file microservices/ms-userservice/src/main/java/example/UserController.java
```

Use when planning or reviewing a change and you want likely cross-project consumers, endpoints, flows, and tests from the generated graph.

Important behavior:

- accepts workspace-relative changed-file paths
- uses static indexed evidence and is intentionally best effort
- does not modify files, run tests, or execute project code

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
goregraph 0.9.3
commit: dev
built: unknown
go: go1.26.x
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
