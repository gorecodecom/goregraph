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

Released versions: `0.1.0`, `0.1.1`.

Reasoning: `0.1.0` is the first public pre-1.0 release. `0.1.1` validates package-manager release automation for Homebrew, Scoop, and manual Winget PR publishing. `1.0.0` is reserved for a stable public CLI/schema contract.

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
  - publish through a PR to `microsoft/winget-pkgs` once `WINGET_TOKEN` and the fork workflow are ready
- prepare Scoop metadata:
  - bucket repository: `gorecodecom/scoop-bucket`
  - install command: `scoop bucket add gorecode https://github.com/gorecodecom/scoop-bucket` then `scoop install goregraph`
  - release updates use `SCOOP_BUCKET_TOKEN`
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
- Scoop bucket manifest published for `gorecodecom/scoop-bucket`
- Winget metadata generation configured; package acceptance is still pending Microsoft review
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

## Milestone 7: Universal Safe Code Graph

Status: delivered in `0.2.0`.

Goal: provide the useful orientation layer of Graphify-style code graphs while keeping GoreGraph local, explicit, and inspectable.

Delivered in this milestone:

- universal rich graph outputs for all currently supported languages:
  - `symbols-full.json`
  - `relations-full.json`
  - `graph-full.json`
- scan audit output:
  - `audit.json`
  - normal scans report `network_used: false`
  - normal scans report `external_commands: false`
- Java extraction hardened beyond the previous regex:
  - packages
  - imports
  - classes
  - interfaces
  - enums
  - records
  - methods
  - fields
  - annotations
  - simple receiver method calls
- Java import classification:
  - `imports_internal`
  - `imports_external`
- Spring Boot domain extraction:
  - applications
  - REST controllers
  - HTTP endpoints
  - services
  - repositories
  - entities and table names
  - bean dependencies
  - `@Bean` methods
- additional reports:
  - `workspace.md`
  - `endpoints.md`
  - `dependencies.md`
  - `affected.md`
- Maven and Node workspace metadata extraction without running Maven, npm, yarn, pnpm, Java, or Node
- query aliases for generated outputs, for example:
  - `goregraph query . graph-full`
  - `goregraph query . spring`
  - `goregraph query . endpoints`
  - `goregraph query . audit`
- MCP read-only access to generated outputs through `get_output`

Acceptance criteria:

- broad existing language support remains available for Go, JavaScript, TypeScript, Python, PHP, Shell, Java, Markdown, `package.json`, and `composer.json`
- Java/Spring projects get deeper framework-oriented navigation
- generated paths remain root-relative
- normal scans remain local and deterministic except metadata timestamps
- no AI, telemetry, network, hooks, background services, or project code execution are part of `scan`

Out of scope:

- tree-sitter dependency
- SCIP ingestion
- AI-generated summaries
- global cross-repository graph
- file watcher

## Milestone 8: Call Graph And Analyzer Expansion

Status: delivered in `0.4.0`.

Goal: move GoreGraph from a useful architecture inventory toward a stronger local navigation graph for endpoint-level debugging and AI-assisted code orientation.

Delivered in this milestone:

- endpoint hardening:
  - path variables keep closing braces, for example `/cadasters/{cadasterId}/import`
  - multipart endpoints are detected through `consumes`, `@RequestPart`, and `MultipartFile`
  - controller method names are preserved for multi-line handler signatures
- richer graph schema:
  - `graph-full.json` edges expose `type`
  - `relation` remains available as a compatibility alias
- Java/Spring call graph:
  - `callgraph.json`
  - `callgraph.md`
  - controller-to-service and service-to-repository method calls
  - extracted vs inferred confidence metadata
- endpoint flows:
  - `endpoint-flows.json`
  - `endpoint-flows.md`
  - endpoint -> controller method -> service method -> repository method orientation
- method-aware test mapping:
  - `test-map.json`
  - enriched `test-map.md`
  - direct Java test method calls
  - MockMvc-style endpoint path matching
- analyzer inventory:
  - `analyzers.json`
  - `analyzers.md`
  - per-language capability summary for symbols, relations, calls, endpoints, tests, and workspace metadata

Acceptance criteria:

- existing scan outputs remain compatible
- new outputs are additive
- Java/Spring depth improves without making GoreGraph Spring-only
- normal scans still do not use AI, network, telemetry, hooks, background services, or project code execution
- generated files remain root-relative and deterministic except metadata timestamps
