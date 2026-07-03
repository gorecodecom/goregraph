# GoreGraph

GoreGraph is a local, deterministic code-intelligence CLI for creating project maps that humans and AI coding assistants can use as orientation.

The MVP is intentionally conservative:

- no AI calls
- no network access
- no Git hooks
- no agent config writes
- no global project modifications
- writes scan output only to `goregraph-out/`
- may add `goregraph-out/` to the project `.gitignore`

## Status

GoreGraph is in early development.

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
- default exclusions
- project `.gitignore` exclusions
- automatic project `.gitignore` entry for `goregraph-out/`
- simple local symbol extraction for Go, Java, JavaScript, TypeScript, and Markdown
- simple local import relation extraction for Go, Java, JavaScript, and TypeScript

Planned later:

- optional MCP stdio mode
- optional `goregraph.yml`
- richer human-readable reports

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

For the MVP, `update` performs an explicit full refresh of the current project. It does not install hooks, run in the background, or watch files.

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

The current milestone uses built-in defaults only.

Later versions will support optional project configuration via:

```text
goregraph.yml
```

The planned config model is documented in `BUILD_PLAN.md`.

## Security Model

The MVP is local and explicit.

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
