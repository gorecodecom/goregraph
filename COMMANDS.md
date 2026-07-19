# GoreGraph Commands

This file lists every user-facing GoreGraph command, what it does, and common variations.

## Normal agent workflow

```bash
goregraph build agent <path>
goregraph context <path> --query "<current coding task>" --budget-tokens 4000 --max-files 12
```

The equivalent standard MCP tool is `task_context`.

```text
Call goregraph context once with the complete task before reading indexed source.
Treat source_sections as current source already read; do not re-read or grep included ranges.
If source_coverage is complete, continue from the included source without another navigation read.
If source_coverage is partial or none, read only relevant uncovered ranges named by source_omissions or files not represented by source_sections.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
At most one narrower retry may use an exact route, qualified symbol, or file returned by the first call; never use a call-chain label.
Do not use specialist GoreGraph queries or expert MCP tools.
```

Run `goregraph doctor <path>` only when Context reports missing or stale output.

## Manual compatibility and expert operations

The specialist query CLI remains supported for manual diagnostics and
exploration. MCP equivalents are available only with
`goregraph mcp --expert-tools`; they are not a fallback from the bounded Context
workflow.

### Workspace delta

`goregraph query <after-snapshot> workspace-delta --query <before-snapshot> --format markdown --limit 20`

In expert MCP mode, the equivalent tool is `workspace_delta`; pass the current
snapshot as `root` and the previous snapshot directory as `query`. Stable IDs
and semantic fields are compared; generation timestamps are ignored.

### Diagnostic families

`goregraph query <path> diagnostics --query <service-or-route> --format markdown --limit 20`

In expert MCP mode, the equivalent tool is `diagnostics`. Results are grouped by
canonical root cause and retain stable member diagnostic and evidence IDs.

### Impact summary

`goregraph query <path> impact-summary --query <route-or-flow> --format markdown --limit 20`

In expert MCP mode, the equivalent tool is `impact_summary`. Results keep direct
and indirect consumers, dependent tests, affected packages, public API surface,
confidence, reasons, and coverage uncertainty separate. Traversal depth is
bounded by the requested detail level and result pagination uses the normal
continuation token.

### Exact symbol operations

The exact Code Explorer operations read the canonical workspace projections
created by `goregraph workspace build all` (or its `workspace scan-all` alias):

```bash
goregraph query . symbol-inventory --query microservices/ms-user --format markdown --limit 20
goregraph query . symbol-resolve --query com.weka.UserService --format json --limit 20
goregraph query . symbol-usages --query symbol:<stable-id> --format markdown --limit 20
goregraph query . symbol-api-consumers --query symbol:<stable-id> --format json --limit 20
goregraph query . symbol-explain --query usage:<stable-id> --detail full --format markdown --limit 20
```

Expert MCP tools replace hyphens with underscores:
`symbol_inventory`, `symbol_resolve`, `symbol_usages`,
`symbol_api_consumers`, and `symbol_explain`.

Use `symbol-resolve` for human text before calling an operation that requires a
canonical symbol ID. Resolution returns every exact candidate rather than
silently choosing a same-name class or export. `symbol-usages` returns
`direct_reference` records; `symbol-api-consumers` returns
`reached_through_api` records. `symbol-explain` accepts a stable `symbol:` or
`usage:` ID.

All task operations accept `--limit <1-100>` and return a continuation token
when more records are available. Pass it back with `--continue <token>`. MCP
uses the equivalent `limit` and `continuation` fields.

## Dashboard questions

The offline workspace dashboard is the full human exploration surface. Its
eight views are Architecture, API Catalog, Endpoints, Feature Flow, Data Flow,
Code Explorer, Diagnostics, and Coverage. API Catalog is the provider inventory;
Endpoints is the relationship and implementation-trace view. Inventory views
use normal browser-scale HTML; zoom controls are reserved for spatial graphs and
implementation traces. Dashboard files are not recommended AI prompt input.

## Workspace context scope

Workspace-root context is intentionally neutral. Project-root reports show
`Requested scope`; task results populate `requested_scope` only from an explicit
query target. `reconciled_from` is operational provenance, not user task scope.

Installation commands such as `brew install gorecodecom/tap/goregraph` are documented in `README.md`. This file focuses on commands provided by the installed `goregraph` binary.

For release acceptance or whenever generated workspace output may come from an older binary, verify the executable first and rebuild from a clean workspace:

```bash
command -v goregraph
goregraph version
goregraph workspace clean .
goregraph workspace clean . --execute
goregraph workspace build all .
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

Project output uses this ownership split:

```text
<path>/goregraph-out/
  manifest.json
  index/                  # complete shared machine index; not prompt input
    api-catalog.json      # complete project provider inventory
  agent/
    context-index.json    # only generated index recommended for AI context
    agent-guide.md
  dashboard/              # full human project reports
    report.md
    ...
```

Workspace output mirrors it:

```text
<workspace>/.goregraph-workspace/
  manifest.json
  index/                  # registry, graphs, symbols, usages, flows
    api-catalog.json      # complete workspace provider inventory
  agent/
    context-index.json
    agent-guide.md
  dashboard/
    workspace-map.html
    workspace-map-assets/
    ...
```

`agent/` and bounded Context Packs are the only recommended AI inputs.
`dashboard/` is for human exploration, including Code Explorer. `index/` is
GoreGraph's internal canonical data and should not be added directly to prompts.
If `goregraph.yml` configures another project output directory, the same three
subtrees live below that directory.

The editable dashboard config is user-owned at
`<workspace>/.goregraph-dashboard.json`, outside generated output. Rebuilds read
and merge it; clean operations do not treat it as generated output.

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

## `goregraph build <agent|dashboard|all> [path]`

Builds one project projection or both from one shared source extraction. A
single-project build requires no workspace marker.

```bash
goregraph build agent .
goregraph build dashboard .
goregraph build all .
```

- `agent` writes `index/` plus `agent/context-index.json` and
  `agent/agent-guide.md`.
- `dashboard` writes `index/` plus human-readable `dashboard/` reports.
- `all` writes both projections without extracting source twice.
- a valid non-selected projection is preserved.

Options are `--no-update-gitignore` and `--no-workspace`. Missing or unknown
targets are usage errors.

Project build scope:

- `goregraph build dashboard .` scans only the selected project and writes its
  human-readable reports under `goregraph-out/dashboard/`. It does not scan
  sibling projects. If GoreGraph detects a workspace, it refreshes that
  workspace overlay from the sibling indexes that already exist.
- `goregraph build dashboard . --no-workspace` scans the selected project and
  skips workspace discovery and reconciliation entirely.
- `goregraph workspace build dashboard .` is the full-workspace form: it scans
  every discovered project and writes the interactive cross-service dashboard
  under `.goregraph-workspace/dashboard/`.

The same project, isolated-project, and full-workspace scopes apply to `agent`
and `all`. A project `dashboard` build produces reports; the interactive Code
Explorer is a workspace dashboard feature.

## `goregraph scan <path>`

Compatibility alias for `goregraph build all <path>`.

`<path>` must point to the project root you want to analyze. In most cases this is `.` because you run GoreGraph from inside the repository:

```bash
cd /Users/name/projects/my-app
goregraph scan .
```

Use this alias when:

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

Generated files are grouped by owner:

- root `manifest.json` records index, agent, and dashboard projection status;
- `index/*.json` contains the complete canonical machine index;
- `agent/context-index.json` and `agent/agent-guide.md` are the AI projection;
- `dashboard/*.md` contains the human-readable project reports.

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

What the generated files mean follows below using logical names. Physically,
JSON machine data lives under `index/`, Markdown reports live under
`dashboard/`, and only `manifest.json` lives at the output root:

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
- `architecture-capabilities.json`: deterministic full-adapter facts for routes, HTTP clients, tests, persistence, messaging/RPC, validation, and request/response boundaries, with language, framework, file, line, and stable evidence ID.
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

Workspace records use the same ownership split:

- `.goregraph-workspace/index/` contains registry, context, contracts, feature
  flows, graphs, symbols, usages, traces, and other canonical JSON.
- `.goregraph-workspace/agent/` contains only `context-index.json` and
  `agent-guide.md` for the recommended AI workflow.
- `.goregraph-workspace/dashboard/` contains human summaries,
  `workspace-map.html`, and `workspace-map-assets/`.
- project-local reconciled JSON remains under each project's `index/`; readable
  reconciled reports remain under its `dashboard/`.

Workspace reconciliation also refreshes:

- `dashboard/diagnostics.md` and `index/diagnostics.json` with outgoing frontend
  contracts and incoming backend consumers for the project.
- `dashboard/endpoints.md` with a `Frontend Consumers` section for backend
  projects.

When workspace discovery is active, `scan` also adds `.goregraph-workspace/` to the detected workspace root `.gitignore` unless `--no-update-gitignore` is used.

Important behavior:

- scans only under the selected project root
- skips default generated/dependency/build directories
- reads the project `.gitignore` as additional exclusions unless disabled in config
- skips binary files and files over the configured size limit
- does not run project code
- does not call AI or network services

## `goregraph update [path] [--target agent|dashboard|all]`

Refreshes generated output for one project. `[path]` defaults to `.` and
`--target` defaults to `all`.

Targets:

- `agent` rebuilds only `agent/` from one fresh extraction.
- `dashboard` rebuilds only the full human-facing `dashboard/` surface from one
  fresh extraction.
- `all` rebuilds `index/`, `agent/`, and `dashboard/` together while sharing
  that extraction.

Examples:

```bash
goregraph update
goregraph update . --target agent
goregraph update . --target dashboard
goregraph update . --target all
```

Important behavior:

- preserves a valid non-selected projection when refreshing only `agent` or
  `dashboard`
- respects `goregraph.yml`
- does not install hooks, watch files, or run in the background
- returns a non-zero exit code if the refresh fails

## `goregraph dashboard path|open [path]`

Locates or opens the generated human-facing project dashboard. Build it first
with `goregraph build dashboard [path]` or `goregraph build all [path]`.

Examples:

```bash
goregraph dashboard path .
goregraph dashboard open .
```

`path` prints `<configured-output>/dashboard/`. `open` opens the primary
`dashboard/report.md` entry point. Project dashboards are Markdown reports;
the interactive Code Explorer is part of the workspace dashboard.

## `goregraph context <path> --query <task> [--budget-tokens 4000] [--max-files 12]`

Returns the bounded Context Pack for the normal AI workflow. It reads only the
generated `agent/context-index.json` projection and never asks an assistant to
ingest `index/`, `dashboard/`, dashboard assets, or broad symbol-usage files.

Example:

```bash
goregraph build agent .
goregraph context . --query "Trace GET /users/{id}" --budget-tokens 4000 --max-files 12
```

Use the first Context Pack directly. Long task descriptions are ranked by their
first substantive problem statement; later analysis requirements do not outrank
the primary action. Tests can be supporting neighbors, but never ordinary
entrypoints.

In the structured Context Pack schema, `query` preserves the normalized request
text verbatim when it is at most 256 runes and its JSON encoding, including
quotes and escapes, is at most 256 bytes. Longer or JSON-escape-heavy input is
represented by a compact primary-task summary. The complete input remains
request-lifecycle internal for semantic selection and is neither serialized nor
hashed into `context_id`.

```text
Call goregraph context once with the complete task before reading indexed source.
Treat source_sections as current source already read; do not re-read or grep included ranges.
If source_coverage is complete, continue from the included source without another navigation read.
If source_coverage is partial or none, read only relevant uncovered ranges named by source_omissions or files not represented by source_sections.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
At most one narrower retry may use an exact route, qualified symbol, or file returned by the first call; never use a call-chain label.
Do not use specialist GoreGraph queries or expert MCP tools.
```

Endpoint tasks select at most one endpoint and eight consumer call sites, with
an explicit omitted count when more consumers exist. The 4000-token default is
unchanged. Context reads the compact `agent/context-index.json`; it does not send
the complete `index/api-catalog.json`, dashboard payload, or
`.goregraph-dashboard.json` to the assistant.

## `goregraph git update [path]`

Safely previews or updates the Git repository containing `[path]`. The path
defaults to `.`.

This command updates source checkouts. It is separate from `goregraph update`,
which rebuilds the current project's GoreGraph index.

Examples:

```bash
goregraph git update .
goregraph git update . --execute
goregraph git update . --format json
```

Options:

- `--execute`: fetch and apply an eligible safe update. Without this option, the
  command is a preview.
- `--format text|json`: select text or indented JSON output. The default is
  `text`.

Preview behavior:

- performs strictly local, read-only Git inspection
- makes no network request and does not fetch or update remote references
- does not change the branch, checkout, index, working tree, or project files
- does not run Git hooks, project code, or a GoreGraph scan
- uses only cached `origin` references and disables lazy fetching

The preview resolves the default branch from cached `origin/HEAD`. If that is
unavailable, it uses `origin/main` or `origin/master` only when exactly one of
those cached references exists. A preview can therefore be stale when the remote
has changed since the last fetch, and it cannot discover a new remote default
branch. `would_update` describes the change relative to the cached reference;
`--execute` fetches and repeats the inspection before changing the checkout.

Execution sequence for each eligible repository:

1. Resolve and canonicalize the containing Git root.
2. Inspect the working tree, active Git operation, HEAD, origin, branches,
   commits, worktrees, repository-local Git configuration, and attributes.
3. Stop without mutation if the local state is unsafe.
4. Fetch the inspected `origin` URL with the explicit refspec
   `+refs/heads/*:refs/remotes/origin/*`, `--prune`, `--no-prune-tags`,
   `--no-tags`, `--no-recurse-submodules`, and `--no-auto-maintenance`.
   Hooks, terminal prompts, and credential-manager interaction are disabled;
   SSH and scp-style transports use batch mode with strict host-key checking.
5. Resolve the default branch again and repeat all safety checks against the
   fetched references.
6. With submodule recursion disabled, switch to the resolved local default
   branch, or create its exact tracking branch from `origin/<branch>` when the
   local branch does not exist.
7. Run `git merge --ff-only origin/<branch>` with hooks disabled.
8. Inspect and report the final branch and commit.

Execution may update cached remote references even when a later safety check
blocks the checkout update. A fetch failure leaves the checkout unchanged.

Safety behavior:

- never stashes, resets, rebases, force-switches, force-updates, discards files,
  or creates merge commits
- never continues or aborts an existing merge, rebase, cherry-pick, or revert
- starts Git directly without a shell
- supplies an empty temporary `core.hooksPath` to fetch, switch, and merge, so
  repository hooks do not run
- refuses repository-local Git settings that may execute commands, active Git
  filter attributes, and custom or unsupported remote transports
- does not run project code or start a GoreGraph scan

Text and JSON are rendered from the same report. The report contains:

- `mode`: `preview` or `execute`
- `workspace_root`: the detected root for workspace updates; omitted for a
  single-repository update
- `repositories`: deterministic repository results
- `summary`: result counts grouped by status

Every repository result uses these fields; optional empty values are omitted from
JSON:

- `path`: requested project or workspace member path
- `git_root`: canonical repository root
- `remote`: resolved `origin` URL
- `branch_before`: checked-out branch before processing
- `branch_after`: resolved target branch in preview, or actual final branch in
  execute mode
- `commit_before`: checked-out commit before processing
- `commit_after`: cached target commit in preview, or actual final commit in
  execute mode
- `status`: stable status value
- `reason`: evidence for the classification
- `remediation`: next action for a blocker; omitted from JSON and shown as `-` in
  text when no remediation is needed
- `executed`: whether a mutating Git process was started. Preflight blockers are
  `false`; a fetch that was started is `true`, even if it failed.

Successful statuses:

- `up_to_date`: the default branch already matches the relevant cached or
  freshly fetched `origin` reference
- `would_update`: preview would switch or fast-forward using the cached reference
- `updated`: execution switched or fast-forwarded the repository successfully

Blocker and failure statuses:

- `dirty`: tracked, untracked, staged, or unstaged changes exist
- `ahead`: the current or target branch has local commits not in its matching
  remote reference
- `diverged`: the current or target branch and its matching remote reference
  both have unique commits
- `not_git`: the requested path is not inside a Git repository
- `missing_remote`: `origin` is not configured
- `blocked_worktree`: the target branch is checked out in another worktree
- `detached_head`: HEAD is detached
- `operation_in_progress`: a merge, rebase, cherry-pick, or revert is active
- `default_branch_unknown`: cached or fetched origin references do not identify
  one unambiguous default branch
- `fetch_failed`: fetch, inspection, switch, or fast-forward failed, or a safety
  inspection refused the operation

Exit codes:

- `0`: every repository is `up_to_date`, `would_update`, or `updated`
- `1`: any operational failure, including a repository blocker or failure,
  partial workspace success, a workspace configuration or discovery error, a
  `gitupdate` infrastructure error, or a report rendering failure
- `2`: command syntax, an option, or a format is invalid

## `goregraph report <path>`

Prints the generated Markdown project report.

`<path>` must point to a project whose dashboard projection has already been
built:

```bash
goregraph build dashboard .
goregraph report .
```

This command reads:

```text
<path>/<configured-output>/dashboard/report.md
```

With default config and `.` as path, that means:

```text
./goregraph-out/dashboard/report.md
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
goregraph build dashboard .
```

Use `doctor` first when you are unsure whether the generated output is missing, stale, or broken.

## `goregraph query <path> <term>`

Searches the generated GoreGraph index.

This is a manual compatibility interface, not the recommended AI input. For an
AI task, use one `goregraph context` call or the MCP `task_context` tool.

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
- `symbol-index` -> `.goregraph-workspace/index/symbol-index.json`
- `symbol-usages-json` -> `.goregraph-workspace/index/symbol-usages.json`

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

Canonical symbol operations read `.goregraph-workspace/index/symbol-index.json`
and `.goregraph-workspace/index/symbol-usages.json`. Their records preserve canonical
symbol IDs, canonical usage IDs, analyzer, confidence, source file and line,
evidence IDs, dependency or artifact evidence, and coverage warnings.

Categories and resolutions are intentionally separate:

- `direct_reference` / `EXACT`: one statically proven source or compile
  relationship;
- `reached_through_api` / `EXACT`: one proven HTTP chain with ordered API path
  steps ending at the selected provider;
- `ambiguous` / `AMBIGUOUS`: multiple disclosed candidates;
- `unresolved` / `UNRESOLVED`: no safe provider was selected.

Evidence IDs are namespaced as `<project>#<local-evidence-id>`. Rich fields are
additive Schema 3 fields; existing output aliases and legacy fields retain their
meaning. A missing result is not proof that no usage exists: read coverage
warnings and limitations for partial, unavailable, or failed analyzer coverage.

Use `query` when you know roughly what you are looking for. Use `explain` when you want context for one specific file or symbol.

## `goregraph explain <path> <file-or-symbol>`

Shows indexed context for one file path or symbol name.
This is a manual compatibility operation, not a normal agent-context step.

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
- workspace `index/symbol-index.json` and `index/symbol-usages.json` exist, use Schema 3,
  reference valid projects/evidence/source paths, and contain consistent exact,
  ambiguous, unresolved, and HTTP API path records

Exit behavior:

- exits `0` when the index is healthy
- exits `1` when failures or warnings are found

Common output:

```text
OK   output: goregraph-out exists
OK   schema: version 3 supported
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

For a missing or invalid workspace symbol projection, Doctor prints the
workspace-specific remediation:

```bash
goregraph workspace clean . --execute
goregraph workspace scan-all .
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

Reliable workspace signals are a `.goregraph-workspace.yml` marker, an explicit
`--workspace <path>`, or common group directories such as `frontend/`,
`microservices/`, `services/`, and `backends/`. Generated
`.goregraph-workspace/` is removable output and is not a persistent marker
guarantee. A flat directory containing sibling projects is not inferred from
project markers alone; pass `--workspace <path>` or add an empty
`.goregraph-workspace.yml` file to that directory.

## `goregraph workspace git update [path]`

Previews or executes safe Git updates for every project discovered in the
workspace containing `[path]`. The path defaults to `.`.

Examples:

```bash
goregraph workspace git update .
goregraph workspace git update . --execute
goregraph workspace git update . --format json
```

This command uses the same preview, execution, safety, status, field, format, and
exit-code behavior as `goregraph git update`. In addition, it:

- uses normal GoreGraph workspace discovery
- resolves canonical Git roots and reports each unique repository once, so
  multiple projects in one monorepo do not cause repeated updates
- processes repositories in deterministic canonical-root order
- continues after a blocked or failed repository, allowing safe repositories to
  succeed
- returns exit code `1` after processing every repository when any result is a
  blocker or failure, including a mix of successful and unsuccessful results

Recommended update-then-build flow:

```bash
goregraph workspace git update .
goregraph workspace git update . --execute
goregraph workspace build all .
```

Review the preview before adding `--execute`. If execution reports a blocker or
failure, follow its remediation and retry before relying on a new workspace scan.
Git updates and scans remain separate so network access and checkout changes are
always explicit.

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
- reconciles workspace output once after the selected project scans
- keeps existing project output locations from each project's config
- stops on the first scan error and reports the failing project

## `goregraph workspace build <agent|dashboard|all> [path]`

Builds the selected projection for every project discovered in a workspace and
then reconciles the workspace once. Each project is scanned once; `all` shares
that extraction across `index/`, `agent/`, and `dashboard/` instead of running
separate agent and dashboard scans.

Examples:

```bash
goregraph workspace build agent .
goregraph workspace build dashboard .
goregraph workspace build all .
goregraph workspace build all . --dry-run
goregraph workspace build all frontend/frontend-monorepo --workspace /Users/name/projects/weka
```

Targets:

- `agent` builds the recommended workspace Context Pack input under
  `.goregraph-workspace/agent/`.
- `dashboard` builds the full human-facing workspace reports and interactive
  Code Explorer under `.goregraph-workspace/dashboard/`.
- `all` builds `index/`, `agent/`, and `dashboard/` in one pass.

Unlike `goregraph build <target> .`, this command scans every discovered
workspace project before reconciliation. A project build scans only its selected
project and may reuse existing sibling indexes for an overlay refresh; it never
rescans those siblings. Use `--no-workspace` on a project build when no workspace
overlay should be discovered or refreshed.

Options:

- `--dry-run`: show the build plan without scanning or writing files.
- `--workspace <path>`: force the workspace root used for discovery.
- `--no-update-gitignore`: skip generated-output `.gitignore` updates.

A workspace build requires one of these workspace signals: grouped-directory
auto-detection, an explicit `--workspace <path>`, or a permanent
`.goregraph-workspace.yml` marker. The generated `.goregraph-workspace/`
directory is removable output, not the persistent marker, and the build command
does not create `.goregraph-workspace.yml`.

## `goregraph workspace scan-all [path]`

Compatibility alias for `goregraph workspace build all [path]`.

Use when:

- you want a fresh index for the full workspace
- frontend and backend source changed and all project-local indexes should be rebuilt
- you use the established `scan-all` spelling in scripts

Examples:

```bash
goregraph workspace scan-all .
goregraph workspace scan-all . --dry-run
goregraph workspace scan-all frontend/frontend-monorepo --workspace /Users/name/projects/weka
goregraph workspace scan-all . --no-update-gitignore
```

Default behavior:

- scans every discovered workspace project once
- reconciles workspace output once after all project scans
- shares each project's extraction across `index/`, `agent/`, and `dashboard/`
- keeps each project's configured output directory
- stops on the first scan error and reports the failing project

Options:

- `--dry-run`: show the scan plan without scanning or writing files.
- `--workspace <path>`: force the workspace root used for discovery.
- `--no-update-gitignore`: skip generated-output `.gitignore` updates while scanning.

For a flat workspace, the first scan can be run as:

```bash
goregraph workspace scan-all . --workspace .
```

Do not rely on generated `.goregraph-workspace/` output for later detection:
`workspace clean --execute` removes it. Use `--workspace`, a grouped layout, or
the permanent `.goregraph-workspace.yml` marker; `workspace clean` does not
remove that YAML marker.

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

## `goregraph workspace refresh [path] [--target agent|dashboard|all]`

Rebuilds selected workspace projections from existing project indexes without
scanning source files. `--target` defaults to `all`.

Use when:

- project indexes are already current
- workspace-level context or cross-project overlays need to be regenerated
- you deliberately want a faster overlay-only update

Examples:

```bash
goregraph workspace refresh .
goregraph workspace refresh . --target agent
goregraph workspace refresh . --target dashboard
goregraph workspace refresh . --target all
goregraph workspace refresh frontend/frontend-monorepo --workspace /Users/name/projects/weka
```

Important behavior:

- reads existing project outputs
- does not rescan source files
- preserves a valid non-selected projection when refreshing only `agent` or
  `dashboard`
- does not prove that project indexes were produced by the current binary
- must not replace `workspace clean . --execute` followed by
  `workspace build all .` during release acceptance

## `goregraph workspace dashboard [path]|path [path]|open [path]|edit [path]`

Compatibility form that prints the path to the generated standalone workspace
dashboard. Prefer the explicit action:

```bash
goregraph workspace dashboard edit [path]
goregraph workspace dashboard path .
goregraph workspace dashboard open .
goregraph workspace dashboard edit .
goregraph workspace dashboard .
```

Build it with `goregraph workspace build dashboard` or `build all` first.

The dashboard is normally generated at:

```text
<workspace>/.goregraph-workspace/dashboard/workspace-map.html
```

The 1.3.0 dashboard is organized around eight views:

- **Architecture** is the first and default view. Dynamic domain lanes come from service-map metadata. Service selection keeps stable card positions while highlighting all direct incoming and outgoing relationships and dimming unrelated context. Background relationships share bundled trunks; selected relationships fan out to explicit card ports. A persistent summary reports relationship, neighboring-service, resolved, unresolved, and mismatch counts and filters by direction or risk. `N calls` means statically detected relationships, not runtime request frequency.
- **API Catalog** is the complete provider inventory and remains before Endpoints. It includes endpoints without known consumers and expands static parameters, media types, request/response identities, provider security, and per-consumer evidence from `index/api-catalog.json`.
- **Endpoints** shows a normal-scale, scrollable caller -> endpoint -> provider inventory and opens a directed implementation trace for a selected endpoint. Long routes wrap instead of shrinking the view. **Back to endpoint inventory** restores service, filters, and scroll position.
- **Feature Flow** shows the evidence-backed route-to-component-to-API-to-backend-to-persistence implementation chain, linked tests, and safe verification commands.
- **Data Flow** uses a sidebar master list and renders one selected request/field/persistence/response chain at normal scale, with unknown mappings displayed in place as explicit gaps.
- **Code Explorer** lists services with indexed classes and symbols, then opens the exact symbol inventory with separate Direct references, Reached through API, All, and uncertainty views.
- **Diagnostics** explains relationships GoreGraph could not safely confirm, including the classification, reason, possible impact, evidence, and suggested next check. Expected frontend-internal behavior is distinguished from likely defects or incomplete scan coverage.
- **Coverage** uses a dedicated normal-scale workbench grouped by project and language. It summarizes analyzed groups and gaps, then shows each capability as `COMPLETE`, `PARTIAL`, `UNAVAILABLE`, or `FAILED`. It describes analysis coverage, not whether source behavior exists.

Open **Code Explorer** directly and choose a service, or select a service in
**Architecture** and choose **Explore classes & symbols**. The workbench keeps
exact symbol inventory separate from usage evidence, with **Direct references**,
**Reached through API**, **All**, and an ambiguity/unresolved view when
uncertainty exists. Filters cover consumer, category, relation kind, language,
and confidence. Source locations expose **Copy path** and **Open source** actions.

Direct references are static source or compile relationships, not runtime call
counts. HTTP reachability is a static consumer-to-route-to-implementation chain,
not a direct import and not a runtime request count. Empty results retain
coverage warnings because missing indexed evidence is not proof that a symbol
is unused.

Important behavior:

- `path` and the no-action compatibility form print the existing dashboard path
- `open` launches the generated static read-only dashboard in the configured browser
- only `edit` starts an authenticated loopback server; it stays in the foreground until Ctrl-C
- does not scan source files
- requires a dashboard projection generated by `workspace build dashboard`,
  `workspace build all`, the `scan-all` alias, or a dashboard-targeted refresh
- the dashboard is self-contained and works offline
- Endpoints, Feature Flow, Data Flow, and Coverage are HTML workbenches; Architecture and implementation traces are spatial SVG graphs with zoom controls
- long implementation traces start with readable cards at 100%; drag or wheel navigation explores the path, while **Fit** shows the complete overview

The editor derives automatic Architecture groups from production package/module
evidence. It supports drag-and-drop plus keyboard controls for group order,
service order, and service placement, and allows group labels to be renamed.
Save writes intentional overrides to the workspace-root
`.goregraph-dashboard.json`; Discard restores the saved layout, and Reset to
detected removes architecture overrides after confirmation. Rebuilds merge that
file with current discovery: valid manual choices survive, new services are
auto-placed, and removed-service overrides remain visible as stale Doctor
warnings instead of being silently deleted.

Endpoint security and consumer call authentication are distinct static facts.
Provider requirements do not prove what a caller sends, and caller headers do
not prove provider enforcement. Missing evidence is `unknown`, displayed as
`No auth evidence detected`, never inferred as `public`; runtime authorization
and enforcement remain out of scope.

## `goregraph workspace explain <target>`

Explains a workspace route, file, symbol, contract, or feature using generated workspace evidence.
This is a manual compatibility operation, not a normal agent-context step.

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
This is a manual compatibility operation, not a normal agent-context step.

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
This is a manual compatibility operation, not a normal agent-context step.

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

- an MCP-capable coding assistant should request a bounded task Context Pack
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
goregraph build agent .
```

Then configure the MCP client to run:

```bash
goregraph mcp
```

The standard server exposes only `task_context`. Give it the complete task and
use its first Context Pack directly.

```text
Call goregraph context once with the complete task before reading indexed source.
Treat source_sections as current source already read; do not re-read or grep included ranges.
If source_coverage is complete, continue from the included source without another navigation read.
If source_coverage is partial or none, read only relevant uncovered ranges named by source_omissions or files not represented by source_sections.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
At most one narrower retry may use an exact route, qualified symbol, or file returned by the first call; never use a call-chain label.
Do not use specialist GoreGraph queries or expert MCP tools.
```

Legacy specialist tools are manual compatibility operations. Expose them only
with the explicit expert mode:

```bash
goregraph mcp --expert-tools
```

Do not cascade specialist tools as the normal AI workflow.

Important limitation:

- MCP reads existing output only. If the agent projection is stale or missing,
  run `goregraph doctor .` to confirm the problem, then
  `goregraph build agent .` manually.

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
goregraph 1.3.0
commit: dev
built: unknown
go: go1.26.x
platform: darwin/arm64
schema: 3
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
