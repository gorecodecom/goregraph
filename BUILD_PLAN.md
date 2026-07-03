# GoreGraph Build Plan

## Purpose

GoreGraph is a local, deterministic code-intelligence CLI for GoreCode projects and other repositories. It should provide the core benefits of tools like Graphify without taking over agent instructions, Git hooks, global configuration, or network services.

Milestones 4-6 are tracked in `ROADMAP.md`.

The first version must cover two use cases:

- Phase 1: generate human-readable project documentation from a scan.
- Phase 2: generate machine-readable project indexes for CLI queries and MCP access.

No AI or network access is part of the MVP.

## Product Positioning

- Product name: GoreGraph
- CLI binary: `goregraph`
- Default output directory per scanned project: `goregraph-out/`
- Brand context: GoreCode developer tooling, alongside GorePlan as a separate product/app.

GoreGraph should be a standalone developer tool, not part of the GorePlan app.

## Language Decision

GoreGraph should be built in Go.

Reasons:

- Produces native single-file binaries for macOS, Linux, and Windows.
- Users do not need Python, Node.js, Java, or another runtime.
- Good fit for CLI tools and filesystem-heavy scanning.
- Cross-compilation and release automation are straightforward.
- The Go standard library is enough for a useful MVP.

Alternatives considered:

- Rust: strong single-binary option, but higher implementation overhead.
- Node/TypeScript: good parser ecosystem, but weaker install story without requiring Node or bundling.
- Python: fast prototyping, but conflicts with the no-runtime requirement.

## Installation And Distribution Plan

Initial development can use local builds:

```bash
go build -o goregraph ./cmd/goregraph
```

Public distribution should start after the tool is stable enough for external users. The first public pre-1.0 release should be `0.1.0`; `1.0.0` is reserved for a stable public CLI/schema contract.

Later distribution should support:

- GitHub Releases with prebuilt binaries:
  - macOS arm64
  - macOS amd64
  - Linux amd64
  - Linux arm64
  - Windows amd64
- Homebrew tap:
  - repository: `gorecodecom/homebrew-tap`
  - install command: `brew install gorecodecom/tap/goregraph`
  - tap repository can later host additional GoreCode CLI formulae
- Windows package manager:
  - stable package ID: `GoreCode.GoreGraph`
  - preferred install command: `winget install --id GoreCode.GoreGraph -e`
- Optional install script later:
  - `curl -fsSL https://gorecode.com/goregraph/install.sh | sh`

GoReleaser should be used for release automation so the same release configuration can later move from GitHub Actions to GitLab CI.

Signing and notarization are deferred to release hardening after Milestone 6. Checksums are required for Milestone 6.

This distribution plan is not required for the local beta implementation, but it must shape the project structure so releases are easy later.

## Repository Structure

Planned structure:

```text
goregraph/
  cmd/
    goregraph/
      main.go
  internal/
    config/
    scan/
    detect/
    extract/
    graph/
    report/
    query/
    gitignore/
  docs/
  README.md
  BUILD_PLAN.md
  LICENSE
  go.mod
```

`internal/` should be used for implementation packages so the public API surface stays small.

## CLI Scope

Milestone 1 commands:

```bash
goregraph scan .
goregraph update
goregraph report .
goregraph help
goregraph scan help
```

Implemented Milestone 2 commands:

```bash
goregraph query . "auth"
goregraph explain . "src/auth/LoginService.ts"
```

Later commands:

```bash
goregraph config init
goregraph stats
```

Implemented Milestone 5 command:

```bash
goregraph mcp
```

Help must be built into the CLI. Every command should have short usage, examples, and important safety notes.

Preferred help forms:

```bash
goregraph help
goregraph scan help
goregraph scan --help
```

`goregraph update` should refresh an existing `goregraph-out/` after code changes. For the MVP it may behave like `goregraph scan .` from the project root, but the command should exist so users have a clear mental model:

```bash
goregraph scan .   # create or fully rebuild the index
goregraph update   # refresh the current project's existing index
```

Later, `update` can become incremental if that proves useful. It must remain explicit: no Git hooks, file watchers, or background refresh processes in the MVP.

## Scan Scope

`goregraph scan .` scans the project rooted at the provided directory.

All stored paths must be relative to the scan root:

```json
"src/auth/LoginService.ts"
```

Avoid absolute paths in normal outputs so results are stable across macOS, Linux, Windows, and CI. If needed, local absolute paths may appear only in `manifest.json` under a clearly marked local-only field.

The default scan should include the whole project except configured exclusions and safety skips.

## Default Exclusions

GoreGraph should skip common generated, dependency, build, IDE, VCS, and output directories by default:

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
goregraph-out/
```

It should also skip:

- binary files
- files above the configured size limit
- unsupported file types for symbol extraction
- symlink targets by default

## Gitignore-Aware Exclusions

GoreGraph should read the project `.gitignore` and add matching ignored paths to the scan exclusions.

Reasoning:

- Files ignored by Git are usually generated, local, large, sensitive, or irrelevant for code navigation.
- This improves scan speed and avoids indexing local-only artifacts.

Rules:

- Only the project `.gitignore` is required for MVP.
- Global Git excludes may be considered later, but are not required initially.
- `.gitignore` patterns should be interpreted relative to the project root.
- Explicit GoreGraph include rules in `goregraph.yml` are supported as a scan limiter. Exclusions and safety skips still apply.

## Safety Rules

MVP hard rules:

- No network access.
- No AI provider calls.
- No Git hooks.
- No writes to `AGENTS.md`, `CLAUDE.md`, Codex config, editor config, or global agent config.
- No global install side effects beyond installing the `goregraph` binary itself.
- Read source files only under the selected scan root.
- Write only to `goregraph-out/` and, when enabled, the project `.gitignore`.
- Do not follow symlinks by default.
- Skip binary files.
- Enforce a default max file size.
- Keep output deterministic.

`goregraph scan . --no-update-gitignore` should disable automatic `.gitignore` modification.

## Gitignore Output Rule

During scan, GoreGraph should check the project `.gitignore`.

If `goregraph-out/` is missing, append this block:

```gitignore
# GoreGraph local scan output
goregraph-out/
```

Requirements:

- Only modify the project-local `.gitignore`.
- Do not modify global Git config.
- Do not create Git hooks.
- Operation must be idempotent.
- Document this behavior clearly in the README.
- Support opt-out with `--no-update-gitignore`.

## Config Model

GoreGraph should support both built-in defaults and optional project config.

### Built-In Defaults

The tool must work without any config file:

```bash
goregraph scan .
```

Built-in defaults cover:

- output directory
- default exclusions
- file size limit
- symlink behavior
- report generation
- AI disabled

### Optional Project Config

Projects may add:

```text
goregraph.yml
```

Example:

```yaml
version: 1
output: goregraph-out
include:
  - src/**
  - tests/**
exclude:
  - generated/**
  - fixtures/large/**
max_file_size_kb: 512
follow_symlinks: false
use_gitignore: true
update_gitignore: true
```

Project config should override only the specified values. Missing values fall back to built-in defaults.

Configured `exclude` patterns are added to the default safety exclusions. Unsupported nested config sections are rejected for now instead of being silently ignored.

The README must document every supported config key.

## Output Structure

`goregraph scan .` currently writes:

```text
goregraph-out/
  manifest.json
  files.json
  symbols.json
  relations.json
  graph.json
  report.md
  modules.md
  entrypoints.md
  test-map.md
```

### Machine-Readable Files

`manifest.json`:

- GoreGraph version
- schema version
- scan root name
- optional local scan root path
- Git commit if available
- config hash
- file count
- skipped file count
- generated file list

`files.json`:

- relative path
- language
- size
- hash
- detected kind

`symbols.json`:

- symbol name
- kind
- relative file path
- line number when available
- Go package symbols
- package.json script symbols
- parent symbol can be added later

`relations.json`:

- imports
- best-effort test-to-source relations
- local references can be added later
- route/entrypoint hints can be added later

`graph.json`:

- combined nodes and edges for CLI query and future MCP access

### Human-Readable Files

`report.md`:

- project summary
- scan stats
- language breakdown
- important directories
- detected build files

`modules.md`:

- top-level directory/module overview

`entrypoints.md`:

- likely app entrypoints, CLIs, server starts, tests, routes, build files

`test-map.md`:

- best-effort source/test associations

## Determinism Rules

Outputs should be stable when these inputs are identical:

- same repository content
- same GoreGraph version
- same config
- same parser rules

Rules:

- Sort files by normalized relative path.
- Sort symbols by file path, line, kind, name.
- Sort relations by source path, target path, relation type.
- Use stable JSON formatting.
- Avoid timestamps in deterministic report content.
- Put timestamps only in `manifest.json` if needed.
- Do not use random IDs.
- Use normalized `/` path separators in output.
- Markdown reports should be generated from fixed templates.

## Extraction Scope

The implementation favors reliable local extraction over running project code or adding heavyweight parser dependencies.

Current file detection:

- Go
- Java
- JavaScript
- TypeScript
- Python
- PHP
- JSON
- YAML
- Markdown
- shell scripts
- Maven/Gradle/package/Composer files

Current extraction:

- Go packages, modules, imports, functions, methods, types, and tests through the Go standard parser where possible.
- Python classes, functions, methods, `test_` functions, imports, `from` imports, local module resolution, and main-guard entrypoints.
- PHP namespaces, classes, interfaces, traits, functions, methods, `use` imports, `require`/`include` relations, Composer PSR-4 autoload hints, and front-controller entrypoints.
- Shell script entrypoints, functions, and `source`/`.` relations.
- Java classes/interfaces/enums and imports.
- JS/TS imports/exports/functions/classes.
- package.json scripts
- Markdown headings
- common build files are detected as file kinds

Tree-sitter or language-server integration can come later if needed.

## Query Behavior

`goregraph query <path> "term"` searches the generated index, not source files directly.

It should return:

- matching files
- matching symbols
- matching relations
- short reason for relevance

`goregraph explain <path> <file-or-symbol>` returns:

- summary from indexed data
- symbols in file
- outbound relations
- inbound relations
- likely tests

The query feature must be read-only.

## MCP Mode

Milestone 5 adds a read-only MCP stdio mode.

```bash
goregraph mcp
```

Provided tools:

- `query_code_map`
- `get_project_summary`
- `get_symbol`
- `get_file`
- `get_related_files`
- `explain_file`
- `doctor`

Rules:

- MCP reads `goregraph-out/`.
- MCP does not scan automatically.
- MCP does not expose a network listener.
- MCP uses stdio transport.
- No global agent config modification.

## README Requirements

The user-facing README must include:

- What GoreGraph is.
- What it does not do.
- Installation.
- Quick start.
- CLI commands.
- Output files.
- `.gitignore` behavior.
- Config file reference.
- Security model.
- Examples.
- License.

README must clearly state:

- `goregraph-out/` is local scan output.
- The tool may add `goregraph-out/` to the project `.gitignore`.
- Use `--no-update-gitignore` to opt out.
- No AI/network/hooks are used by GoreGraph.

## License Decision

Current repository license: Apache-2.0.

Decision:

- Use Apache-2.0 before public release.
- Keep the repository private while product direction and stability are still being validated.
- Make the repository public only when the CLI, schema, docs, and release process are stable enough for external users.

Important note:

Apache-2.0 is still permissive open source and allows commercial reuse, modification, redistribution, sublicensing, and sale under its terms. Compared with MIT, it has more explicit patent language and trademark limitations, which is a better fit for a public developer tool.

## Non-Goals For MVP

- No AI summaries.
- No cloud provider integrations.
- No HTTP server.
- No editor extension.
- No Git hooks.
- No automatic agent instruction injection.
- No global config writes.
- No dependency on Python, Node.js, or Java.

## First Implementation Milestone

Milestone 1 should deliver:

- Go module setup.
- `goregraph help`.
- `goregraph scan .`.
- `goregraph update` as an explicit full refresh alias for the current project.
- deterministic `goregraph-out/manifest.json`.
- deterministic `goregraph-out/files.json`.
- deterministic `goregraph-out/report.md`.
- default exclusions.
- `.gitignore` read support for scan exclusions.
- `.gitignore` update support for `goregraph-out/`.
- `--no-update-gitignore`.
- README with installation/build-from-source and usage.

Milestone 2 delivered:

- language-specific symbol extraction.
- `symbols.json`.
- `relations.json`.
- `graph.json`.
- `goregraph query`.
- `goregraph explain`.
- tests for generated indexes and query/explain behavior.

Milestone 3 delivered:

- richer reports.
- optional `goregraph.yml`.
- include/exclude config.
- configured output directories for scan/report/query/explain.
- package and test symbol extraction.
- package.json script extraction.
- best-effort test-to-source relations.
- inbound/outbound explain context.

Milestone 4 delivered:

- `goregraph doctor`.
- schema constants.
- deterministic manifest golden test.
- Go parser extraction.
- local Go import resolution.
- graph dependency nodes.
- command reference documentation.

Milestone 5 or later can add:

- MCP stdio mode.
- release packaging.
- richer parser support.
