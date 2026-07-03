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

## Milestone 5: Language Expansion And Read-Only MCP

Status: delivered.

Goal: deepen non-Go analysis and allow AI coding tools to read GoreGraph indexes through a controlled local MCP stdio server.

Planned work:

- add deeper Python symbol, import, local-module, test, and entrypoint extraction
- add deeper PHP namespace, class, interface, trait, function, use, include, Composer, and entrypoint extraction
- add deeper Shell function, source, and entrypoint extraction
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

Delivered in this milestone:

- Python class/function/method/test/main-guard symbol extraction
- Python import and local-module relation resolution
- PHP namespace/class/interface/trait/function/method/front-controller symbol extraction
- PHP `use`, `require`, `include`, and Composer PSR-4 relation support
- Shell function, source, and shebang entrypoint extraction
- `goregraph mcp`
- read-only stdio MCP tools:
  - `query_code_map`
  - `get_project_summary`
  - `get_file`
  - `get_symbol`
  - `get_related_files`
  - `explain_file`
  - `doctor`
- command documentation for MCP mode

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

Status: delivered.

Goal: make GoreGraph easy to install and update on macOS, Linux, and Windows.

Target release: `0.1.0`.

Reasoning: the repository should stay private until GoreGraph is stable enough for external users. `1.0.0` is reserved for a stable public CLI/schema contract.

Planned work:

- add `goregraph version`
- add conservative CI:
  - `gofmt` check
  - `go vet ./...`
  - `go test ./...`
- add GoReleaser config
- add cross-platform builds:
  - macOS arm64
  - macOS amd64
  - Linux amd64
  - Linux arm64
  - Windows amd64
- publish checksums
- create GitHub Releases while the repository is hosted on GitHub
- keep release automation portable enough to move to GitLab CI later
- prepare Homebrew tap release flow:
  - tap repository: `gorecodecom/homebrew-tap`
  - install command: `brew install gorecodecom/tap/goregraph`
  - tap repository can later host additional GoreCode CLI formulae
- prepare Winget package metadata:
  - package ID: `GoreCode.GoreGraph`
  - install command: `winget install --id GoreCode.GoreGraph -e`
- switch project license to Apache-2.0
- document install, upgrade, and uninstall flows
- add a release checklist

Delivered in this milestone:

- `goregraph version`
- version metadata package with ldflag-ready fields
- conservative GitHub Actions CI:
  - `gofmt` check
  - `go vet ./...`
  - `go test ./...`
- GoReleaser v2 configuration
- release workflow for tag-based GitHub Releases
- cross-platform release targets for macOS, Linux, and Windows
- checksum publishing configuration
- Homebrew Formula publishing configuration for `gorecodecom/homebrew-tap`
- Winget metadata prepared with upload disabled for review
- release checklist in `docs/RELEASE.md`
- README installation guidance

Acceptance criteria:

- users can install a prebuilt binary without Go
- release artifacts are checksummed
- the README documents the recommended install path
- release process is repeatable from CI
- local source builds continue to work
- `goregraph version` prints version, commit, build date, Go version, platform, and schema version
- license and release docs are consistent

Out of scope:

- hosted SaaS
- remote telemetry
- code signing and notarization
- Windows paid code signing certificate

## Cross-Cutting Rules

- keep GoreGraph local by default
- avoid global side effects
- keep generated output root-relative and deterministic
- prefer Go standard library where practical
- add dependencies only when they replace fragile custom logic with a maintained, focused package
- preserve explicit user control over scans and writes
