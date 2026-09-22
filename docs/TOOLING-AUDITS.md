# Tooling audits

Development version 1.4.3 adds an explicit source-inventory mode to the existing Context entrypoint. This implements the source-evidence workflow proposed in [the original audit proposal](storybook-context-audit-proposal.md); that document remains a historical record. No Weka pipeline or global agent instruction file is changed.

## Usage

After installing a binary built from this version, refresh an older agent index:

```sh
goregraph update <project-root> --target agent
```

For a workspace, use the corresponding workspace update command with the agent target. The installed 1.4.2 binary does not provide this new mode.

```sh
goregraph context <project-or-workspace-root> --mode audit --query "Prüfe Storybook, Stories, Fixtures, Runner, Playwright, CI und Dokumentation." --max-files 20 --budget-tokens 6000
```

The same request through MCP is `task_context` with `mode: "audit"`, the full `query`, `root`, `max_files: 20` and `budget_tokens: 6000`. Both `strict-v1` and `adaptive-v2` support explicit audit mode. Omit `mode` for ordinary production-code work. No extra MCP tools or dependencies are required.

## Evidence and scope

Audit mode selects indexed Storybook, Playwright or Vitest sources named by the query and follows literal project-local file references. CI requests start at the project's `.gitlab-ci.yml` or indexed GitHub workflow files; unrelated GitLab YAML files are not assumed active. Literal local GitLab includes are followed recursively, with cycles bounded. External, conditional, complex or dynamic includes remain unverified. GitHub workflow source is delivered without interpreting actions or reusable workflows.

Package scripts can lead to runner configuration; literal imports and configuration paths can lead to setup, story, component and fixture source. Simple `*`, `**` and `?` file globs are supported. Links express static file references, not JavaScript binding resolution, effective test selection or execution. Module aliases, computed paths, external triggers and shell indirection are not resolved. A changed parent source is delivered with its changed-state marker, but its old indexed links are not followed. Both endpoints must be delivered and current for a link to appear.

Sources are distributed across configuration, stories/dependencies, runner, CI, visual tests and documentation so a large story collection does not consume every slot. These categories are file groups, not an authoritative classification of runtime behavior. Whole files are delivered only when they fit; source text preserves exact numbered lines and read receipts. A configuration file can therefore show both an installed A11y addon and disabled automatic tests without equating installation with activation.

`audit.areas` describes only selected indexed files. Even an area's `complete` status does not prove that all relevant repository files were indexed or that a feature is enabled. Overall source coverage remains `partial`. `audit.execution` is always `unknown`: prepared screenshot tests, documented mappings and CI job configuration do not prove approved baselines, service consumption or a successful pipeline. Binary baselines and runtime artifacts are not interpreted. The audit delivers evidence for the caller to assess; it does not automatically pronounce a setup production-ready.

## Bounds and fallback

The existing limits remain 256–6000 tokens and 1–20 delivered files. Selection uses the entire original query; the displayed query may be shortened. `audit.query_hash` identifies the full input and `query_truncated` reports shortening.

Traversal is capped at 2048 sources, 256 references per source, 64 source reads and 32 delivered links. Sources for link expansion are checked within 44 reads, leaving capacity for other areas. Readable source files retain the existing 2 MiB limit. Dynamic expressions and unsupported glob syntax require manual verification. Static links may be shortened to fit the response budget and are not a complete dependency graph.

Budgeted-out files receive exact project/path/start/end omissions where possible. Only those ranges authorize additional source reads under the audit guidance. If omission metadata cannot fit, `source_unrepresented` records the missing files; narrow the request or increase the supported budget instead of guessing paths. No usable source yields a low-confidence fallback. `audit_index_unavailable` specifically identifies an index created without the new metadata; partial evidence alone does not require an update. Workspace indexes warn when registered projects lack audit metadata.

## Acceptance and regression evidence

The source-only fixture uses the real scanner and JSON-RPC MCP entrypoint. All seven groups are mandatory in the ordinary test suite; no opt-in acceptance environment variable is required:

| Group | Required source evidence |
| --- | --- |
| Storybook setup | Main configuration and addons |
| Automatic accessibility | Preview with `test: 'off'` |
| Stories and data | Story, imported component and fixture |
| Interaction runner | Package script, Vitest configuration and setup |
| CI integration | Root include and referenced job with `allow_failure: true` |
| Visual preparation | Playwright configuration and screenshot test |
| Documented limits | Source documentation; no implied successful run |

A separate multi-project fixture covers frontend and backend deployment-trigger configuration, INT/TEST environments, explicit multiple-app parameters, frontend `release` versus Playwright `master`, and non-blocking failure handling. It checks project identity, unknown external includes and unavailable-project coverage. No backend-to-app relationship is inferred from naming similarity.

Regression checks also cover multi-sentence German and long Unicode queries over MCP and CLI, both protocols, all budget boundaries, exact omissions, duplicate identity, changed CI parents, nested include cycles, missing/dynamic/external includes, 100 additional stories, scanner exclusions and ordinary production lookups. The tests install no npm packages, execute no Storybook/Playwright browser suite and contact no CI service.

```sh
go test ./internal/mcp ./internal/scan ./internal/cli -run 'Test(Storybook|CompleteAudit|Audit|ContextCLIAudit|MCPTaskContextValidates)' -count=1
go test ./... -p 4 -count=1 -timeout=20m
go vet ./...
```

The optional fixture evidence report is documented in [the fixture README](../internal/mcp/testdata/storybook-audit/README.md). Successful GoreGraph regression tests verify retrieval and evidence contracts, not the runtime health of an audited application.

## Dashboard: Tests & Tooling

Service Code offers two subviews: Classes & Usages, and Tests & Tooling. Both retain the selected project. Tooling lists configuration, stories and referenced data, runners, visual-test sources, connected local CI sources and documentation. Search, category filters, source-to-source navigation and links back to indexed code symbols reuse the existing dashboard patterns. File counts are not story-case counts or test coverage.

The new optional project `index/tooling.json` feeds the dashboard payload. It is independent of agent Context Packs; dashboard-only rebuilds preserve the existing agent projection. Dashboard revision 3 causes older dashboard projections to rebuild through the usual update command. `goregraph workspace update <root> --target dashboard` refreshes project inputs and workspace display; a dashboard refresh alone can only reuse metadata already present in project outputs.

Literal `a11y.test` and `allow_failure` declarations appear with source line numbers. They describe the matched declaration only, not effective activation, all jobs or pipeline execution. Unsupported or dynamic settings remain unclassified. External includes, unresolved paths and execution/reference approval limits stay visible. Tooling inventories have at most 2048 displayed source records per project, with an explicit truncation notice. Older exports without metadata show an unavailable state; an empty current inventory never proves absence of tests.
