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

## Milestone 9: Universal Language Intelligence

Status: delivered in `0.5.0`.

Goal: make GoreGraph substantially more helpful outside Java/Spring by adding deterministic route, call, flow, test, and navigation orientation for Go, PHP, JavaScript, TypeScript/React, Python, and Shell.

Delivered in this milestone:

- normalized route inventory:
  - `routes.json`
  - `routes.md`
  - Spring endpoints
  - Go `net/http` and router-style routes
  - PHP Laravel-style routes
  - JavaScript/TypeScript Express/Fastify-style routes
  - React Router routes
  - Python FastAPI/Flask-style decorators
- normalized flow output:
  - `flows.json`
  - `flows.md`
  - route -> handler -> static call steps
  - Spring endpoint flows are included in the generic flow view
- broader call graph:
  - Go function calls
  - PHP method/function calls
  - JavaScript/TypeScript function/component calls
  - Python function/method calls
  - Shell function calls
- broader test mapping:
  - Go test functions
  - PHP test methods
  - JavaScript/TypeScript `test`/`it` blocks
  - Python `test_` functions
- human navigation report:
  - `navigation.md`
  - likely route starting points
  - most connected files
  - important symbols
  - test orientation
  - analyzer coverage summary

Acceptance criteria:

- Java/Spring functionality remains intact
- new outputs are additive and deterministic
- no language analyzer executes project code
- no AI, telemetry, network, hooks, background services, or project code execution are part of `scan`
- confidence metadata remains explicit for heuristic static matches

## Milestone 10: Noise-Aware Frontend And Package Intelligence

Status: delivered in the current development branch.

Goal: make frontend monorepos and mixed JS/TS projects more useful by reducing static-analysis noise and adding deterministic package/API context.

Delivered in this milestone:

- app-specific frontend route IDs:
  - `portal:/`
  - `mein-konto:/settings`
  - route records keep `app`, `package`, `route_id`, rendered components, confidence, and reason
- Redux Little Router improvements:
  - `Fragment forRoute` records prefer the rendered component when it is statically visible
  - rendered components are listed separately from the route wrapper
- low-signal source filtering:
  - declaration files
  - generated files
  - archive/storybook paths
  - common test/helper call targets such as `find`, `text`, `match`, `push`, and `block`
- Node workspace package graph:
  - `package-graph.json`
  - `package-graph.md`
  - package nodes from `package.json`
  - package dependency edges with internal workspace detection
- JavaScript/TypeScript API contract inventory:
  - `api-contracts.json`
  - `api-contracts.md`
  - supported helper calls such as `GetHelper(...)` and `PostHelper(...)`
  - basic `fetch(...)` usage
- clearer machine-readable authority:
  - `callgraph.json` is authoritative for method/function call edges
  - `routes.json` is authoritative for route records
  - `package-graph.json` is authoritative for Node package dependencies
  - `api-contracts.json` is authoritative for detected frontend API calls

Acceptance criteria:

- previous Spring/Java and universal-language tests remain green
- frontend monorepo fixtures produce app-specific route IDs
- Storybook/archive/declaration noise does not appear as route/call targets
- package graph and API contract outputs are generated, queryable, and documented

## Milestone 11: Realistic API Contracts And Maven Graph

Status: delivered in the current development branch.

Goal: close the biggest practical gaps found in real WEKA test scans after `v0.6.0`: empty frontend API contracts, occasional cross-app route handler resolution, and missing Maven dependency graph output.

Delivered in this milestone:

- realistic JavaScript/TypeScript API helper extraction:
  - `GetHelper(dispatch, "/path")`
  - `GetHelperWithStatus(dispatch, "/path")`
  - `PostHelper`, `PutHelper`, `PatchHelper`, and `DeleteHelper`
  - multiline helper calls
  - template placeholders normalized from `${id}` to `{id}`
- app-aware frontend candidate ranking:
  - route handlers prefer same file
  - then same `apps/<name>`
  - then same `packages/<name>`
  - then same language fallback
- Maven dependency graph:
  - `maven-graph.json`
  - `maven-graph.md`
  - module/dependency nodes from `pom.xml`
  - dependency edges with reason `pom-dependency`
- CLI and integration access:
  - `goregraph query . maven-graph`
  - `goregraph query . maven-graph-json`
  - doctor validation for `maven-graph.json`

Acceptance criteria:

- realistic frontend helper fixtures produce non-empty `api-contracts.json`
- same-name frontend components in different apps resolve to the owning route app
- Maven fixture produces `maven-graph.json` and `maven-graph.md`
- new outputs are additive and deterministic

## Strategic Roadmap: From Code Index To Work Assistant

Status: planned.

Goal: keep GoreGraph realistic and useful without pretending that static analysis can fully replace code reading. GoreGraph should become a deterministic local work assistant for navigation, contract checks, impact orientation, and test selection while keeping normal scans local, offline, and free of project code execution.

Long-term principles:

- source code remains authoritative
- generated output is orientation and risk reduction, not proof of runtime behavior
- normal scans do not call AI, network services, package managers, build tools, or project code
- every heuristic edge must expose confidence and reason
- every machine-readable output should use stable IDs, root-relative paths, file/line evidence, and deterministic ordering
- expensive or non-deterministic enrichment must stay optional and explicit

Backend roadmap:

- endpoint contracts:
  - endpoint -> HTTP method -> path
  - request DTO
  - response DTO
  - content type
  - path/query params
  - validation annotations such as `@Valid`, `@NotNull`, `@Size`, and custom validators where statically visible
  - declared multipart parts
- backend contract quality:
  - OpenAPI/Swagger comparison when an existing spec is present in the repository
  - missing documentation markers
  - response schema drift markers
- deeper Spring graph:
  - Controller -> Service -> Helper -> Repository -> External Client
  - interface-to-implementation candidates
  - constructor, field, setter, and Lombok constructor injection where statically visible
  - `@Scheduled`, `@EventListener`, and async entrypoints
- security model:
  - endpoint-level `@PreAuthorize`
  - role/scope/authority strings where statically visible
  - explicit security caveats when only global filter-chain context exists
- database model:
  - Entity -> table/view
  - Entity field -> column
  - Repository method -> derived query fields
  - `@Query` JPQL/native SQL
  - read/write orientation for repositories where statically visible
- external service contracts:
  - RestClient/WebClient/Feign-style calls
  - target method/path/DTOs where statically visible
  - retry/timeout/error handling markers where local code exposes them
- Maven graph hardening:
  - parent POM/BOM/dependencyManagement records
  - version source markers
  - scope normalization
  - duplicate/conflicting direct dependency markers
  - transitive dependencies only as optional explicit enrichment if a local lock/cache/source exists
- test coverage mapping:
  - MockMvc/WebTestClient paths
  - test method -> endpoint/method candidates
  - unit vs integration test markers
  - untested endpoint report

Frontend roadmap:

- AST/type-aware resolution:
  - exact imports for relative paths
  - `tsconfig` path aliases
  - webpack aliases
  - package aliases
  - default/named exports
  - re-export chains through `index.ts`
  - mixed JS/TS projects
- React route and component model:
  - Route -> Page -> Child Components
  - JSX component usage
  - conditional rendering markers
  - props flow orientation
  - connected/container component resolution
- routing:
  - React Router `Route`, `Switch`, `Redirect`
  - Redux Little Router `Fragment forRoute`
  - nested route composition
  - dynamic params
  - redirects and fallbacks
- Redux/state flows:
  - Component -> Action Creator
  - Action -> Reducer
  - Action -> Saga/Thunk
  - Selector -> State Slice
  - Dispatch usage
  - Duck-file conventions
  - async side effects
- API contracts:
  - more robust template-string normalization
  - query param structure
  - request body hints
  - response usage hints
  - API call -> backend service candidate
- frontend/backend linking:
  - frontend API contract -> backend route match
  - method/path mismatch markers
  - missing backend endpoint markers
  - backend DTO field changes -> frontend usage candidates
- frontend tests:
  - test file -> imported component/service/reducer priority
  - Jest/Enzyme/React Testing Library patterns
  - mocked API calls
  - route/component/API test coverage markers
  - generic helper noise filtering
- package graph:
  - app -> workspace package imports
  - workspace package -> workspace package imports
  - package.json dependencies compared with actual imports
  - duplicate package names and version drift
- feature navigation:
  - feature module grouping by route/page/components/state/api/tests
  - design-system usage
  - forms, modals, flyouts, tables, and wizards
- forms/data flow:
  - field -> validation -> submit handler -> API payload
  - API response -> mapper -> state -> selector -> component

Cross-cutting roadmap:

- confidence model:
  - `EXTRACTED`: direct syntactic fact from source
  - `RESOLVED`: source fact resolved through imports/types/package boundaries
  - `INFERRED`: plausible static match without full resolution
  - `WEAK_MATCH`: name/string similarity only
- all edges include:
  - reason
  - file
  - line
  - stable ID
  - source artifact
- high-value reports:
  - `impact-index.json`
  - `route-index.json`
  - `contract-matches.json`
  - `missing-tests.md`
  - `potentially-broken-contracts.md`
  - `high-impact-files.md`
- cross-repo orientation:
  - frontend API call -> microservice endpoint
  - backend endpoint -> frontend callers
  - shared DTO/contract drift candidates
- optional diff mode:
  - changed files since last scan
  - changed symbols/contracts/routes
  - likely affected routes/tests/API calls

## Milestone 12: Contract Matching And Better Confidence

Status: implemented for `v0.8.0`.

Goal: create the next large but controlled jump by connecting existing frontend API contracts to existing backend route contracts, hardening URL normalization, and introducing clearer confidence levels. This makes GoreGraph more useful for frontend/backend change impact without attempting a full AST/type-system rewrite yet.

Why this milestone is the next best step:

- `api-contracts.json` is now populated in real frontend scans
- backend routes and Spring endpoint data are already strong
- frontend/backend matching gives immediate practical value for bugfixes and refactors
- the scope is still deterministic and local
- it avoids prematurely building a full TypeScript compiler-level analyzer

Implemented outputs:

- `contract-matches.json`
- `contract-matches.md`
- `potentially-broken-contracts.md`
- extended `api-contracts.json` normalization fields

Implemented work:

- URL normalization hardening:
  - normalize `${id}` to `{id}`
  - mark unsafe complex template expressions such as `${stateName ? ...}`
  - split path and query string
  - normalize query params into stable sorted metadata
  - preserve raw path separately from normalized path
- frontend API contract classification:
  - service candidate from first path segment
  - e.g. `/cadasters/...` -> `ms-cadaster` candidate
  - e.g. `/userservice/...` -> `ms-userservice` candidate
  - explicit unsafe dynamic marker and reason
- backend endpoint contract extraction:
  - backend route key: method + normalized path
  - controller method
  - existing backend route metadata from `routes.json`
- matching engine:
  - exact method + normalized path match -> `RESOLVED`
  - method + compatible path pattern match -> `RESOLVED`
  - compatible path with different method -> `WEAK_MATCH` issue
  - missing backend route -> reported
  - HTTP method mismatch -> reported
  - unsafe dynamic URL -> reported
- confidence model update:
  - introduce `RESOLVED` and `WEAK_MATCH`
  - keep existing `EXTRACTED` and `INFERRED`
  - document confidence definitions in `SCHEMA.md`
- reports:
  - matched frontend API calls with backend method/path/handler where available
  - missing backend endpoint candidates
  - method mismatches
  - unsafe dynamic URL patterns
  - focused potential breakage report

Acceptance criteria status:

- normal scan remains local, deterministic, and offline: done
- no project code, Maven, npm, TypeScript compiler, or bundler execution: done
- same-scan frontend/backend matches produce `contract-matches.json`: done
- unsafe dynamic frontend URLs are visible rather than silently normalized into misleading paths: done
- backend endpoint route matching remains additive and does not break existing `spring.json`, `routes.json`, or `api-contracts.json`: done
- docs explain that contract matching is static orientation, not runtime proof: done

Deferred beyond Milestone 12:

- cross-repository contract matching across separately scanned frontend/backend projects
- backend request/response DTO matching inside `contract-matches.json`
- full TypeScript AST/type resolver
- full Redux/Saga/Thunk semantic flow graph
- transitive Maven dependency resolution
- full SecurityFilterChain evaluation
- database stored procedure and Oracle function analysis
