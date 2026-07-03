# GoreGraph Build Plan

## Purpose

GoreGraph is a local, deterministic code-intelligence CLI for GoreCode projects and other repositories. It should provide the core benefits of tools like Graphify without taking over agent instructions, Git hooks, global configuration, or network services.

The first version must cover two use cases:

- Phase 1: generate human-readable project documentation from a scan.
- Phase 2: generate machine-readable project indexes for CLI queries and later MCP access.

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

Later distribution should support:

- GitHub Releases with prebuilt binaries:
  - macOS arm64
  - macOS amd64
  - Linux amd64
  - Linux arm64
  - Windows amd64
- Homebrew tap:
  - `brew install gorecode/tap/goregraph`
- Windows package manager later:
  - `winget install GoreCode.GoreGraph`
  - or `scoop install goregraph`
- Optional install script later:
  - `curl -fsSL https://gorecode.com/goregraph/install.sh | sh`

This distribution plan is not required for the MVP implementation, but it must shape the project structure so releases are easy later.

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
goregraph doctor
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

## MVP Extraction Scope

The first implementation should favor reliable simple extraction over broad language support.

Initial file detection:

- Go
- Java
- JavaScript
- TypeScript
- JSON
- YAML
- Markdown
- shell scripts
- Maven/Gradle/package files

Initial extraction is regex/line-based:

- Go module declarations
- Go package declarations
- imports where supported
- exported and regular functions/classes where easy
- Go test functions
- Java classes/interfaces/enums
- Go functions/types
- JS/TS imports/exports/functions/classes
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

## Future MCP Mode

MCP is a later phase, not part of the first MVP.

Planned command:

```bash
goregraph mcp
```

Planned tools:

- `query_code_map`
- `get_symbol`
- `get_file`
- `get_related_files`
- `get_project_summary`

Rules:

- MCP reads `goregraph-out/`.
- MCP must not scan automatically unless explicitly requested.
- MCP must not expose a network listener by default.
- Prefer stdio transport first.
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
- No AI/network/hooks are used in the MVP.

## License Decision

Current repository license: MIT.

Decision:

- Keep MIT for now because the repository already contains an MIT `LICENSE`.
- Keep the repository private while product direction is still being validated.
- Revisit licensing before public release.

Important note:

MIT permits commercial reuse, modification, redistribution, sublicensing, and sale as long as the copyright and license notice are preserved. If GoreGraph should not be commercially reused by others later, MIT is not the right final public license.

Potential future alternatives:

- Proprietary license
- Source-available non-commercial license
- Apache-2.0 only if permissive commercial reuse is acceptable and patent terms are desired

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

Milestone 4 or later can add:

- MCP stdio mode.
- release packaging.
- richer parser support.
