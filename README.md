<p align="center">
  <img src="assets/brand/gorecode-logo.svg" alt="GoreCode" width="240">
</p>

# GoreGraph

GoreGraph is a local, deterministic code-intelligence CLI for creating project maps that humans and AI coding assistants can use as orientation.

The tool is intentionally conservative:

- no AI calls
- no network access
- no telemetry
- no project code execution
- no Git hooks
- no agent config writes
- no global project modifications
- writes scan output to `goregraph-out/` and, when a workspace is detected, workspace metadata to `.goregraph-workspace/`
- may add generated GoreGraph output paths to the relevant `.gitignore` files

## Status

GoreGraph is a usable local CLI for generating deterministic project indexes and human-readable project maps.

Implemented:

- `goregraph help`
- `goregraph scan`
- `goregraph update`
- `goregraph report`
- `goregraph query`
- `goregraph explain`
- `goregraph doctor`
- `goregraph workspace status`
- `goregraph mcp`
- `goregraph version`
- deterministic `manifest.json`
- deterministic `files.json`
- deterministic `symbols.json`
- deterministic `relations.json`
- deterministic `graph.json`
- deterministic `symbols-full.json`
- deterministic `relations-full.json`
- deterministic `graph-full.json`
- deterministic `callgraph.json`
- deterministic `endpoint-flows.json`
- deterministic `test-map.json`
- deterministic `routes.json`
- deterministic `flows.json`
- deterministic `api-contracts.json`
- deterministic `architecture-capabilities.json` reference-adapter evidence
- deterministic `service-dependencies.json`
- deterministic `frontend-usage.json`
- deterministic `contract-matches.json`
- deterministic `diagnostics.json`
- deterministic `package-graph.json`
- deterministic `maven-graph.json`
- deterministic `analyzers.json`
- deterministic `spring.json`
- deterministic `audit.json`
- deterministic `report.md`
- deterministic `modules.md`
- deterministic `workspace.md`
- deterministic `endpoints.md`
- deterministic `endpoint-flows.md`
- deterministic `dependencies.md`
- deterministic `callgraph.md`
- deterministic `routes.md`
- deterministic `flows.md`
- deterministic `api-contracts.md`
- deterministic `frontend-usage.md`
- deterministic `contract-matches.md`
- deterministic `potentially-broken-contracts.md`
- deterministic `diagnostics.md`
- deterministic `workspace-context.md`
- deterministic `workspace-contract-matches.md`
- deterministic `workspace-feature-flows.json`
- deterministic `workspace-feature-flows.md`
- deterministic `workspace-next-actions.md`
- deterministic `frontend-consumers.md`
- deterministic `package-graph.md`
- deterministic `maven-graph.md`
- deterministic `navigation.md`
- deterministic `analyzers.md`
- deterministic `affected.md`
- deterministic `entrypoints.md`
- deterministic `test-map.md`
- workspace registry and cross-project overlay reports when a repository is part of a detected workspace
- default exclusions
- project `.gitignore` exclusions
- automatic `.gitignore` entries for `goregraph-out/` and `.goregraph-workspace/`
- optional `goregraph.yml`
- local symbol extraction for Go, Python, PHP, Shell, Java, JavaScript, TypeScript, and Markdown
- local relation extraction for Go, Python, PHP, Shell, Java, JavaScript, and TypeScript
- simple test-to-source relations
- local Go import resolution
- local Python import resolution
- local PHP namespace/use/include resolution
- local Shell source resolution
- universal rich graph output for all currently supported languages
- Java internal/external import classification
- Spring Boot application, controller, endpoint, service, repository, entity, bean, and dependency extraction
- Java/Spring method call graph extraction
- endpoint-to-controller-to-service-to-repository flow reports
- method-aware Java test mapping and MockMvc endpoint matching
- generic route, call, flow, and test mapping for Go, PHP, JavaScript, TypeScript/React, Python, and Shell
- app-specific frontend route IDs for monorepos
- component-aware frontend route flows that can follow JSX child components, React effect calls, and local event handlers to API callers
- frontend usage chains that explain which route/component flow reaches a detected API caller
- local JavaScript/TypeScript API helper and `fetch` contract extraction, including common helper calls such as `GetHelper(dispatch, "/path")`, `weka.request("GET", "path")`, and the enclosing API function/method name when available
- Java backend service-client dependency extraction for common shared clients such as user, product, license, cadaster, document, and regulation services
- frontend API to backend route contract matching with method mismatch, missing route, unscanned service, and unsafe dynamic URL reports
- compact diagnostics report for entrypoints, risky contracts, unscanned services, untested endpoints, weak flows, and likely tests
- zero-config workspace discovery for common layouts such as `frontend/` plus `microservices/`, including nested frontend group folders
- workspace reconciliation that refreshes existing `goregraph-out/` overlays when later sibling scans add new service indexes
- workspace next-action summaries for coverage, missing service scans, weak matches, and resolved flows without linked tests
- Node workspace package dependency graph extraction
- Maven dependency graph extraction from `pom.xml`
- low-signal frontend noise filtering for declarations, archives, generated files, and common test utility calls
- human-readable navigation and diagnostics reports for likely starting points, risks, and central local files
- analyzer capability inventory for supported languages
- Maven and Node workspace summaries
- scan audit metadata showing normal scans used no network and executed no external commands
- graph nodes for local files and external dependencies
- inbound/outbound relation context in `goregraph explain`
- index health checks with `goregraph doctor`
- read-only MCP stdio server with `goregraph mcp`
- build metadata with `goregraph version`
- GoReleaser release configuration
- GitHub Actions CI and release workflows

Planned later:

- richer parser support
- code signing and notarization

The next milestones are documented in `ROADMAP.md`.

Every CLI command is documented in `COMMANDS.md`.

Generated output compatibility is documented in `SCHEMA.md`.

## Installation

The current development release target is `v0.9.5`.

The currently working installation paths are Homebrew, Scoop, manual release archives, and building from source. Winget metadata is generated by the release workflow, but the Microsoft PR is a manual publishing step until the package is accepted.

### Homebrew macOS/Linux

Recommended macOS/Linux installation:

```bash
brew install gorecodecom/tap/goregraph
```

This command works without a separate `brew tap` step. Homebrew discovers the GoreCode tap from the fully qualified formula name.

Optional two-step installation:

```bash
brew tap gorecodecom/tap
brew install goregraph
```

Verify the installed binary:

```bash
goregraph version
```

Upgrade an existing Homebrew installation:

```bash
brew update
brew upgrade goregraph
```

`brew install goregraph` installs a missing formula. Updating an already installed version is done with `brew upgrade`.

### Scoop Windows

Recommended Windows installation until Winget publishing is approved:

```powershell
scoop bucket add gorecode https://github.com/gorecodecom/scoop-bucket
scoop install goregraph
```

Verify the installed binary:

```powershell
goregraph version
```

### Manual Install Windows

Download the Windows archive from the latest GitHub release:

```text
goregraph_Windows_x86_64.zip
```

Extract the ZIP and run:

```powershell
.\goregraph.exe version
```

For regular use, place `goregraph.exe` in a directory on your Windows `PATH`.

### GitHub Releases

Prebuilt archives are published for macOS, Linux, and Windows:

```text
https://github.com/gorecodecom/goregraph/releases
```

Each release includes `checksums.txt`.

Manual archive names:

```text
goregraph_Darwin_arm64.tar.gz
goregraph_Darwin_x86_64.tar.gz
goregraph_Linux_arm64.tar.gz
goregraph_Linux_x86_64.tar.gz
goregraph_Windows_x86_64.zip
```

After extracting the archive, run:

```bash
./goregraph version
```

On Windows PowerShell:

```powershell
.\goregraph.exe version
```

### Build From Source

Requirements:

- Go 1.23 or newer

Build:

```bash
go build -o goregraph ./cmd/goregraph
```

Run:

```bash
./goregraph help
```

During development you can also run:

```bash
go run ./cmd/goregraph help
```

Install the local checkout as the `goregraph` command:

```bash
go install ./cmd/goregraph
goregraph version
goregraph scan .
```

On Windows, `go install` writes `goregraph.exe` to `go env GOPATH` + `\bin`. Make sure that directory is on `PATH` before running `goregraph scan .` from another project.

### Winget

Future Windows install command after the package is accepted into `microsoft/winget-pkgs`:

```powershell
winget install --id GoreCode.GoreGraph -e
```

Winget metadata is generated during releases. The command is not live until the Winget manifest PR is accepted by Microsoft.

## Quick Start

From a project root:

```bash
goregraph scan .
```

This creates:

```text
goregraph-out/
  manifest.json
  files.json
  symbols.json
  relations.json
  graph.json
  symbols-full.json
  relations-full.json
  graph-full.json
  callgraph.json
  endpoint-flows.json
  test-map.json
  routes.json
  flows.json
  api-contracts.json
  frontend-usage.json
  contract-matches.json
  diagnostics.json
  package-graph.json
  maven-graph.json
  analyzers.json
  spring.json
  workspace.md
  endpoints.md
  endpoint-flows.md
  dependencies.md
  callgraph.md
  routes.md
  flows.md
  api-contracts.md
  frontend-usage.md
  contract-matches.md
  potentially-broken-contracts.md
  diagnostics.md
  workspace-context.md
  workspace-contract-matches.json
  workspace-contract-matches.md
  workspace-feature-flows.json
  workspace-feature-flows.md
  workspace-next-actions.md
  frontend-consumers.md
  package-graph.md
  maven-graph.md
  navigation.md
  analyzers.md
  affected.md
  audit.json
  report.md
  modules.md
  entrypoints.md
  test-map.md
```

Print the generated report:

```bash
goregraph report .
```

Search the generated index:

```bash
goregraph query . StartServer
```

Read a generated output directly:

```bash
goregraph query . graph-full
goregraph query . callgraph
goregraph query . routes
goregraph query . flows
goregraph query . api-contracts
goregraph query . frontend-usage
goregraph query . contract-matches
goregraph query . broken-contracts
goregraph query . diagnostics
goregraph query . package-graph
goregraph query . maven-graph
goregraph query . workspace-contracts
goregraph query . workspace-features
goregraph query . navigation
goregraph query . endpoint-flows
goregraph query . analyzers
goregraph query . endpoints
goregraph query . audit
```

Workspace aliases also work from the workspace root after at least one project scan has created `.goregraph-workspace/`:

```bash
cd ~/projects/weka
goregraph query . workspace-context
goregraph query . workspace-contracts
goregraph query . workspace-features
goregraph query . workspace-next-actions
```

When GoreGraph detects a workspace above the scanned project, it also writes:

```text
.goregraph-workspace/
  registry.json
  context.json
  contract-matches.json
  feature-flows.json
  next-actions.md
  workspace-context.md
  contract-matches.md
  feature-flows.md
```

and refreshes workspace overlays in every already indexed project:

```text
goregraph-out/
  workspace-context.md
  workspace-contract-matches.json
  workspace-contract-matches.md
  workspace-feature-flows.json
  workspace-feature-flows.md
  workspace-next-actions.md
  frontend-consumers.md
```

Explain one indexed file or symbol:

```bash
goregraph explain . src/main.go
```

Refresh after code changes:

```bash
goregraph update
```

`update` performs an explicit full refresh of the current project. It does not install hooks, run in the background, or watch files.

Inspect the detected workspace without scanning:

```bash
goregraph workspace status .
```

Preview the highest-value missing service scans without scanning anything:

```bash
goregraph workspace scan-missing . --top 5
```

Run those prioritized scans explicitly:

```bash
goregraph workspace scan-missing . --top 5 --execute
```

Scan every discovered project in the workspace:

```bash
goregraph workspace scan-all .
```

Refresh workspace overlays from existing project outputs without scanning source files:

```bash
goregraph workspace refresh .
```

Preview and then remove generated GoreGraph workspace output:

```bash
goregraph workspace clean .
goregraph workspace clean . --execute
```

## Commands

```bash
goregraph help
```

Show global help.

```bash
goregraph scan <path>
```

Create or rebuild GoreGraph output for a project.

```bash
goregraph scan <path> --no-update-gitignore
```

Scan without adding GoreGraph-generated output paths to `.gitignore` files.

```bash
goregraph scan <path> --no-workspace
```

Scan only the selected project output and skip workspace discovery/reconciliation.

```bash
goregraph scan <path> --workspace <workspace-root>
```

Scan a project while forcing the workspace root used for sibling discovery.

```bash
goregraph update
```

Refresh the current project's `goregraph-out/`.

```bash
goregraph report <path>
```

Print `<path>/goregraph-out/report.md`.

```bash
goregraph query <path> <term>
```

Search the generated index for matching files, symbols, and relations.

```bash
goregraph explain <path> <file-or-symbol>
```

Print indexed context for a file path or symbol name.

```bash
goregraph doctor <path>
```

Check generated output health without scanning.

```bash
goregraph workspace status <path>
```

Show discovered workspace projects, indexed projects, known backend services, and referenced but missing services without scanning or writing files. Missing services are prioritized by the number of referenced contracts and include scan suggestions when GoreGraph found a matching workspace project.

```bash
goregraph workspace scan-missing <path>
```

Show a prioritized missing-service scan plan without scanning. By default this is a dry run and shows the top 5 unindexed services with the most referenced frontend contracts.

```bash
goregraph workspace scan-missing <path> --top 5 --execute
```

Scan the selected top-N missing service projects and refresh workspace overlays. Use `--no-update-gitignore` to skip generated-output `.gitignore` updates.

```bash
goregraph workspace scan-all <path>
```

Scan every discovered project in the detected workspace and refresh workspace overlays after each scan. Use `--dry-run` to print the plan without scanning.

```bash
goregraph workspace refresh <path>
```

Refresh `.goregraph-workspace/` outputs and the dashboard from existing project `goregraph-out/` directories without scanning source files.

```bash
goregraph workspace clean <path>
```

Show generated GoreGraph output paths for the detected workspace without deleting anything. Add `--execute` to remove project `goregraph-out/` directories and the workspace `.goregraph-workspace/` directory.

```bash
goregraph mcp
```

Start the read-only MCP stdio server for MCP-capable coding assistants.

```bash
goregraph version
```

Print build metadata including version, commit, build date, Go version, platform, and schema version.

## Language Support

Current extraction is local and deterministic. It does not run project code or call external services.

- Go: packages, modules, functions, methods, types, tests, imports, local module import resolution, `net/http`/router routes, calls, route flows, and test mappings.
- Python: classes, functions, methods, `test_` functions, imports, `from` imports, local module resolution, FastAPI/Flask-style routes, calls, route flows, test mappings, and main-guard entrypoints.
- PHP: namespaces, classes, interfaces, traits, functions, methods, `use` imports, `require`/`include` relations, Composer PSR-4 autoload hints, Laravel-style routes, calls, test mappings, and `index.php` front controllers.
- Shell: shell-script entrypoints, functions, `source`/`.` file relations, and function-call flows.
- JavaScript/TypeScript: functions, classes, imports, exports, package scripts, Express/Fastify-style routes, React Router routes, app-specific route IDs in monorepos, realistic API helper/`fetch` contracts, app-aware handler resolution, calls, route flows, and test mappings.
- Java: classes, interfaces, enums, imports, Maven dependency graphs, Spring endpoints, Spring dependencies, Java/Spring callgraph, endpoint flows, and test mappings.
- Markdown: headings.
- JSON/YAML and common build files: indexed as files and build/config context where supported.

## Output Files

`manifest.json` contains scan metadata:

- tool name
- schema version
- output directory
- scanned file count
- skipped file count
- generated files
- project root name

`files.json` contains indexed files with root-relative paths:

- path
- language
- size
- SHA-256 hash
- kind

`symbols.json` contains simple extracted symbols:

- name
- kind
- root-relative file path
- line number

`relations.json` contains simple extracted relations:

- source file
- target
- relation type
- line number

`graph.json` contains combined nodes and edges derived from files, symbols, and relations.

`callgraph.json` contains method/function call edges with confidence metadata.

`routes.json` contains normalized backend and frontend route records.

`flows.json` contains route-to-handler-to-call flow records.

`api-contracts.json` contains JavaScript/TypeScript HTTP client usage detected from supported helpers, `weka.request(...)`, and `fetch` calls. Helper extraction handles common argument shapes such as `GetHelper(dispatch, "/path")`, `weka.request("GET", "tree/regulations")`, and multiline calls. Records preserve the raw path, normalized path, query metadata, service candidate, enclosing caller function or method when available, and unsafe dynamic URL marker when a template expression is too complex to trust. `api-contracts.md` shows the caller next to the API contract when detected.

`service-dependencies.json` contains backend service-client relationships extracted from Java source, for example imports or fields referencing shared clients such as `UserMgmtService`, `ProductServiceMgmt`, or `LicenseMgmtService`. Workspace service maps merge these backend-to-backend dependencies with frontend API contract relationships.

`frontend-usage.json` and `frontend-usage.md` connect detected frontend API contracts back to the best matching frontend route flow. They show route ID/path, component, API caller, confidence, and the static evidence chain when a route flow reaches the API contract file or caller.

`contract-matches.json` compares detected frontend API calls with backend routes discovered in the same scan. Exact method and compatible path patterns are marked `RESOLVED`; method mismatches, missing backend routes, unscanned services, and unsafe dynamic URL patterns are reported as weak/static findings. `contract-matches.md` is the readable match view, while `potentially-broken-contracts.md` focuses on issues that deserve manual review.

`diagnostics.json` and `diagnostics.md` summarize the most useful diagnostic entrypoints: top routes/endpoints, risky contracts, workspace-resolved contracts, unscanned services, endpoints without detected tests, weak inferred flows, and likely tests.

Workspace files are additive overlays. `.goregraph-workspace/registry.json` lists discovered sibling projects and whether each one has a scan index. `.goregraph-workspace/context.json` summarizes loaded indexes, known services, and referenced-but-missing services with contract counts and matching project status when known. `workspace-context.md` also suggests concrete `cd <project> && goregraph scan .` commands for referenced services that have not been indexed yet. `.goregraph-workspace/contract-matches.json` links API contracts from indexed projects to backend routes from indexed sibling services and carries the API caller name when the helper call is inside a detected function or method; `workspace-contract-matches.json`, `workspace-contract-matches.md`, `frontend-consumers.md`, and backend `endpoints.md` expose the project-relevant subset in machine-readable and readable forms. `.goregraph-workspace/feature-flows.json` links resolved frontend route/component/API calls to backend endpoint flows and tests, including JSX child component hops, React effect calls, and local event handlers when they connect a route component to an API caller. `workspace-next-actions.md` summarizes workspace coverage, the highest-value missing service scans, weak workspace matches, and resolved flows without linked tests. Workspace feature flows still show the API caller for app-scope `WEAK_MATCH` routes when the API contract itself has caller context. Workspace root detection prefers the parent that contains both frontend and backend group directories over an intermediate frontend-only grouping folder. Existing scanned siblings receive refreshed `workspace-context.md`, `workspace-contract-matches.json`, `workspace-contract-matches.md`, `workspace-feature-flows.json`, `workspace-feature-flows.md`, `workspace-next-actions.md`, and `frontend-consumers.md` files after each later scan. Workspace reconciliation also updates `diagnostics.md` with outgoing frontend contracts and incoming backend consumers, appends frontend consumers to backend `endpoints.md`, and explains missing linked frontend routes or tests in `workspace-feature-flows.md`.

The workspace dashboard at `.goregraph-workspace/workspace-map.html` is a standalone offline UI organized around five questions:

- **Architecture:** understand how projects and services communicate. Selecting a service highlights its direct incoming and outgoing relationships without changing the full map; **Isolate neighborhood** explicitly narrows the canvas, and **Show full architecture** returns to the complete map.
- **Endpoints:** search for and select a service, inspect its caller -> endpoint -> provider rows, then open an endpoint to follow its implementation trace. This combines the earlier Endpoint Paths and Endpoint Trace workflows in one view.
- **Data Flow:** inspect evidence-backed request fields, transformations, persistence, and response fields. Unknown mappings are displayed as explicit gaps instead of invented connections.
- **Diagnostics:** review relationships GoreGraph could not safely confirm, why each result matters, its available evidence, and what to check next. Categories distinguish likely code defects, missing scan coverage, dynamic or statically ambiguous paths, and expected frontend-internal behavior.
- **Coverage:** inspect which language and framework capabilities were analyzed completely, partially, not at all, or with a failure. Coverage describes analyzer support and is not proof that a source-code behavior is absent.

Endpoints provides multi-select HTTP method filters, separate caller and provider service filters, and resolution-status filters. Filters remain active while a trace is open and after returning to the endpoint inventory.

`evidence.json` stores deterministic root-relative source evidence with stable IDs. Generated route and call facts reference those records through additive `evidence_ids`. `capabilities.json`, `coverage.json`, and `coverage.md` report analyzer support separately from relationship confidence, match resolution, and diagnostic severity. Existing Schema 1 fields keep their previous meanings.

Version 0.9.5 establishes Java/Spring and JavaScript/TypeScript/Node/React as the reference adapters. `architecture-capabilities.json` records deterministic file/line evidence for HTTP clients, routes, tests, persistence, messaging/RPC, validation, and request/response boundaries. Coverage and the Query/MCP `coverage` operation link to these facts; the `evidence` operation resolves the same IDs. Supported framework families include Spring MVC/WebFlux, Spring Data/JDBC, RestTemplate/WebClient/Feign-style clients, JUnit/Spring Test, Kafka/RabbitMQ/gRPC, Fetch/Axios, Express/Fastify, NestJS, Next.js, Prisma/TypeORM/Sequelize/Mongoose/SQL calls, Jest/Vitest/node:test, React Testing Library, KafkaJS/AMQP, and Node gRPC.

The analysis remains static and pattern-backed. Runtime-generated routes, reflective dispatch, arbitrary client wrappers, dependency-injection aliases, ORM metaprogramming, and configuration assembled outside indexed source may remain gaps; GoreGraph reports those limits instead of inferring unsupported behavior.

Selection does not auto-center or relayout the graph. **100%** resets only zoom and pan, while **Fit** calculates a fit for the currently visible content without clearing the selection or search. Viewport state is retained per top-level view, including when returning from an endpoint trace to the endpoint inventory.

GoreGraph performs static analysis. A relationship that is absent from the dashboard is not proof that no runtime relationship exists: dynamic dispatch, generated routes, reflection, runtime configuration, unindexed projects, and unsupported analyzer capabilities can leave static evidence incomplete. Treat confidence labels and diagnostics as evidence-backed guidance, and inspect the cited source or run a fresh complete scan before drawing operational conclusions.

For release acceptance, first preview generated cleanup with `goregraph workspace clean .`, review the paths, then run `goregraph workspace clean . --execute` and `goregraph workspace scan-all .`. Validate only outputs produced by the newly installed binary and that fresh scan; `goregraph workspace refresh .` reuses existing project outputs and is not a substitute for this clean-scan workflow.

`package-graph.json` contains Node workspace package nodes and package-to-package dependency edges from `package.json`.

`maven-graph.json` contains Maven package nodes and dependency edges extracted from `pom.xml`.

`navigation.md` summarizes likely starting points, central local files, important symbols, test orientation, and analyzer coverage.

`affected.md` lists local files with inbound impact signals. It filters external packages such as `react` or design-system imports so the report is better suited for concrete change-impact orientation.

`report.md` is a human-readable deterministic project report.

`modules.md` summarizes top-level project areas.

`entrypoints.md` lists likely app, CLI, and package-script entrypoints.

`test-map.md` lists best-effort source/test associations.

All normal output paths are relative to the scanned project root.

## MCP Mode

`goregraph mcp` starts a read-only stdio server for MCP-capable tools.

Version 0.9.3 adds task-oriented, bounded agent operations shared by Query and MCP: `workspace-summary`, `service-context`, `endpoint-search`, `diagnostics`, `coverage`, `evidence`, `tests`, and `change-context`. Query accepts `--format json|text|markdown`, `--detail summary|standard|full`, `--limit 1..100`, `--continue <token>`, and `--query <text>`. MCP exposes equivalent underscore-named read-only tools with strict input schemas. Results include freshness, coverage warnings, stable evidence references, truncation, continuation, and a suggested next operation.

Primary agent entry points are `workspace-summary.md`, `architecture.md`, `diagnostics.md`, and `agent-guide.md`. Specialized generated reports remain available as references.

Version 0.9.4 adds `directed-traces.json` and `data-flows.json`. Directed traces expose stable node roles, edges, entries, exits, a deterministic main path, branches, bounded cycles, upstream/downstream traversal, paths to persistence/messages/external services/tests, and explicit `Trace from here`. Query/MCP add `endpoint-trace`, `symbol-trace`, `trace-from`, and `data-flow`. Selecting a trace node never changes zoom, pan, or layout automatically.

It:

- reads an existing `goregraph-out/` or configured output directory
- exposes query, summary, file, symbol, relation, explain, and doctor tools
- does not scan automatically
- does not write project files
- does not open a network port

Run `goregraph scan .` first, then point the MCP client at the `goregraph mcp` command.

## Exclusions

GoreGraph skips common generated, dependency, build, VCS, editor, and local output paths by default:

```text
.git/
node_modules/
vendor/
target/
build/
dist/
coverage/
.idea/
.vscode/
.gitignore
goregraph-out/
.goregraph-workspace/
```

It also skips:

- binary files
- files over the configured size limit
- symlinks by default

## Generated Output .gitignore

GoreGraph reads the project `.gitignore` and uses it as additional scan exclusions.

By default, `goregraph scan` also ensures the project `.gitignore` contains:

```gitignore
# GoreGraph local scan output
goregraph-out/
```

This prevents local scan output from being committed.

When workspace discovery is active and a workspace root is detected, GoreGraph also ensures the workspace root `.gitignore` contains:

```gitignore
# GoreGraph local workspace output
.goregraph-workspace/
```

This prevents central workspace overlays from being committed when the workspace root itself is a Git repository.

To opt out:

```bash
goregraph scan . --no-update-gitignore
```

GoreGraph only modifies `.gitignore` files in the scanned project and detected workspace root. It does not modify global Git config.

## Configuration

GoreGraph works without config. Projects can optionally add:

```text
goregraph.yml
```

Supported keys:

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

Config values are merged with built-in safety defaults. Configured `exclude` patterns are added to the default exclusions; they do not remove safety exclusions such as `.git/` or `node_modules/`.

`include` limits the scan to matching root-relative paths. If `include` is omitted, GoreGraph scans the whole project except exclusions and safety skips.

The configured `output` directory is used by `scan`, `report`, `query`, and `explain`.

Unsupported nested config sections are intentionally rejected for now so configuration mistakes do not silently change scan behavior.

## Explain Context

```text
goregraph explain . src/main.go
```

`explain` prints:

- file metadata
- symbols in the file
- outbound relations
- inbound relations
- likely tests

## Security Model

GoreGraph is local and explicit.

GoreGraph does not:

- call AI providers
- call external network services
- install Git hooks
- modify agent instruction files
- modify editor settings
- run background daemons
- follow symlinks by default

GoreGraph does:

- read files under the selected scan root
- write to `goregraph-out/`
- write workspace registry/overlay files under a detected workspace root and already indexed sibling output directories
- optionally update project and workspace root `.gitignore` files for generated GoreGraph output

## License

Apache-2.0. See `LICENSE`.
