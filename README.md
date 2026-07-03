# GoreGraph

GoreGraph is a local, deterministic code-intelligence CLI for creating project maps that humans and AI coding assistants can use as orientation.

The tool is intentionally conservative:

- no AI calls
- no network access
- no Git hooks
- no agent config writes
- no global project modifications
- writes scan output only to `goregraph-out/`
- may add `goregraph-out/` to the project `.gitignore`

## Status

GoreGraph is a usable local CLI for generating deterministic project indexes and human-readable project maps.

Implemented:

- `goregraph help`
- `goregraph scan`
- `goregraph update`
- `goregraph report`
- `goregraph query`
- `goregraph explain`
- deterministic `manifest.json`
- deterministic `files.json`
- deterministic `symbols.json`
- deterministic `relations.json`
- deterministic `graph.json`
- deterministic `report.md`
- deterministic `modules.md`
- deterministic `entrypoints.md`
- deterministic `test-map.md`
- default exclusions
- project `.gitignore` exclusions
- automatic project `.gitignore` entry for `goregraph-out/`
- optional `goregraph.yml`
- simple local symbol extraction for Go, Java, JavaScript, TypeScript, and Markdown
- simple local import relation extraction for Go, Java, JavaScript, and TypeScript
- simple test-to-source relations
- inbound/outbound relation context in `goregraph explain`

Planned later:

- optional MCP stdio mode
- richer parser support
- packaged releases

The next milestones are documented in `ROADMAP.md`.

## Build From Source

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

Explain one indexed file or symbol:

```bash
goregraph explain . src/main.go
```

Refresh after code changes:

```bash
goregraph update
```

`update` performs an explicit full refresh of the current project. It does not install hooks, run in the background, or watch files.

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

Scan without adding `goregraph-out/` to the project `.gitignore`.

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

`report.md` is a human-readable deterministic project report.

`modules.md` summarizes top-level project areas.

`entrypoints.md` lists likely app, CLI, and package-script entrypoints.

`test-map.md` lists best-effort source/test associations.

All normal output paths are relative to the scanned project root.

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
```

It also skips:

- binary files
- files over the configured size limit
- symlinks by default

## Project .gitignore

GoreGraph reads the project `.gitignore` and uses it as additional scan exclusions.

By default, `goregraph scan` also ensures the project `.gitignore` contains:

```gitignore
# GoreGraph local scan output
goregraph-out/
```

This prevents local scan output from being committed.

To opt out:

```bash
goregraph scan . --no-update-gitignore
```

GoreGraph only modifies the project-local `.gitignore`. It does not modify global Git config.

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
- optionally update the project `.gitignore`

## License

MIT. See `LICENSE`.
