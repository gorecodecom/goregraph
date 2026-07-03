# GoreGraph Roadmap

This roadmap captures the next planned milestones after the current local code-intelligence tool baseline.

## Milestone 4: Index Quality And Reliability

Status: delivered.

Goal: make the generated index more reliable, easier to validate, and safe to build future integrations on.

Planned work:

- split remaining package responsibilities where files grow beyond a focused size
- introduce schema constants and documented schema compatibility rules
- add `goregraph doctor` for checking generated output health
- add golden-file tests for deterministic generated outputs
- improve include/exclude matching coverage
- improve Go extraction with `go/parser` from the standard library
- resolve local Go imports to repository-relative files where possible
- distinguish local graph nodes from external dependency nodes
- improve error messages for broken config and stale output
- add fixture projects for Go, Java, and TypeScript scans

Delivered in this milestone:

- `goregraph doctor`
- schema constants shared by scan and doctor
- schema compatibility documented in `SCHEMA.md`
- deterministic manifest golden test
- Go extraction through `go/parser`
- local Go import resolution for repository-relative files
- graph dependency nodes for external imports
- clearer command documentation in `COMMANDS.md`

Acceptance criteria:

- generated output is deterministic across repeated scans
- stale or missing indexes produce actionable errors
- no large multi-responsibility implementation files
- schema behavior is documented before MCP depends on it
- all existing commands keep their current behavior unless explicitly documented

Out of scope:

- MCP server
- release packaging
- AI provider calls

## Milestone 5: Read-Only MCP Integration

Goal: allow AI coding tools to read GoreGraph indexes through a controlled local MCP stdio server.

Planned work:

- add `goregraph mcp`
- use stdio transport only
- read existing `goregraph-out` or configured output directory
- never scan automatically on MCP startup
- return clear errors if output is missing, stale, or schema-incompatible
- expose read-only tools:
  - `query_code_map`
  - `get_project_summary`
  - `get_file`
  - `get_symbol`
  - `get_related_files`
  - `explain_file`
- document how Codex and other MCP clients can connect

Acceptance criteria:

- MCP mode has no network listener
- MCP mode does not modify project files
- MCP tools work from generated index files only
- missing or stale output tells the user to run `goregraph scan`
- integration docs are explicit and copy-pasteable

Out of scope:

- automatic agent instruction injection
- global editor or agent config writes
- cloud sync
- AI summaries

## Milestone 6: Distribution And Release

Goal: make GoreGraph easy to install and update on macOS, Linux, and Windows.

Planned work:

- add `goregraph version`
- add GitHub Actions CI
- add cross-platform builds:
  - macOS arm64
  - macOS amd64
  - Linux amd64
  - Linux arm64
  - Windows amd64
- publish checksums
- create GitHub Releases
- prepare Homebrew tap release flow
- evaluate Windows distribution:
  - Winget
  - Scoop
- document install, upgrade, and uninstall flows
- add a release checklist

Acceptance criteria:

- users can install a prebuilt binary without Go
- release artifacts are checksummed
- the README documents the recommended install path
- release process is repeatable from CI
- local source builds continue to work

Out of scope:

- hosted SaaS
- remote telemetry
- commercial licensing change

## Cross-Cutting Rules

- keep GoreGraph local by default
- avoid global side effects
- keep generated output root-relative and deterministic
- prefer Go standard library where practical
- add dependencies only when they replace fragile custom logic with a maintained, focused package
- preserve explicit user control over scans and writes
