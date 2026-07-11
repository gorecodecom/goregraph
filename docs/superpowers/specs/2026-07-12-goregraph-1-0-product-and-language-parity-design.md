# GoreGraph 1.0 Product And Language Parity Design

**Status:** Approved design baseline  
**Current release:** `0.9.0`  
**Target release:** `1.0.0`  
**Primary real-world acceptance workspace:** `~/projects/weka/`

## Purpose

GoreGraph 1.0 must provide two coherent products over the same deterministic local code-intelligence model:

1. A human-facing workspace dashboard for understanding architecture, endpoints, data flow, change impact, diagnostics, and test coverage.
2. A compact machine-facing orientation layer that lets coding agents use Query and MCP before reading broad areas of source code.

The implementation must remain local, deterministic, inspectable, read-only through MCP, free of telemetry, and independent of any one company, repository layout, naming convention, language, or framework.

## Product Outcomes

By `1.0.0`, a developer or agent must be able to answer:

- Which projects, services, packages, and frameworks exist?
- Which components communicate, in which direction, and through which protocol?
- Which frontend or other consumer reaches a given endpoint?
- Which handlers, functions, methods, repositories, messages, and external services are downstream?
- Which input fields are validated, transformed, persisted, forwarded, and returned?
- Which tests cover a route, symbol, flow, or change?
- Which relationships are exact, normalized, inferred, weak, unresolved, or outside the analyzed scope?
- Which statement and source location support every public relationship?
- Which analyzer capabilities were available, partial, unavailable, or failed?

## Architectural Direction

GoreGraph will use a language-independent core model. Language analyzers and framework adapters emit normalized, evidence-backed facts. Workspace graphs, traces, data flows, diagnostics, JSON, Markdown, Query, MCP, and the dashboard consume that common model.

```text
Source and project files
  -> language analyzers
  -> framework adapters
  -> normalized facts plus evidence
  -> architecture / calls / endpoints / data flow / diagnostics / coverage
  -> JSON and Markdown / Query and MCP / dashboard
```

### Capability Interfaces

Avoid one oversized analyzer interface. Use focused capabilities with explicit inputs and outputs:

- symbol analysis
- relation and import analysis
- call analysis
- route analysis
- API-client analysis
- test analysis
- persistence analysis
- messaging analysis
- data-flow analysis

An analyzer may compose multiple capabilities, but each capability must remain independently testable. Framework knowledge must be separated from base-language knowledge. For example, Java owns classes and calls while Spring owns controllers and dependency injection; TypeScript owns functions and types while React owns component and event flow.

### Normalized Facts

The shared fact model covers:

- projects, modules, packages, crates, and workspace packages
- files and symbols
- imports and dependencies
- function and method calls
- routes and handlers
- API contracts and clients
- middleware, guards, and authentication hints
- tests and test-to-production links
- persistence reads and writes
- request, response, message, and stored data shapes
- validations and transformations
- message producers, channels, topics, queues, and consumers

No dashboard renderer or MCP tool may invent a fact that is absent from this model.

## Evidence, Confidence, Resolution, And Coverage

### Evidence

Every publicly visible edge must reference at least one evidence record containing:

- stable evidence ID
- project and root-relative file
- start and end location when available
- analyzer and framework adapter
- extraction method
- concise reason
- stable source-excerpt hash when source syntax is the evidence

Supported extraction methods include syntax, framework convention, configuration, exact signature matching, normalized route matching, naming heuristic, and user configuration. Large source excerpts are not duplicated throughout generated outputs. Query and MCP may return a narrowly scoped excerpt on request.

### Confidence

Confidence only expresses certainty of a relationship:

- `EXACT`: directly represented by syntax or an unambiguous signature
- `RESOLVED`: exact facts connected deterministically
- `NORMALIZED`: connected through a documented normalization
- `INFERRED`: supported by strong but incomplete static evidence
- `WEAK`: ambiguous or incomplete evidence
- `UNKNOWN`: no reliable mapping

Separate dimensions must represent:

- resolution: `MATCHED`, `PARTIAL`, `UNRESOLVED`, `OUT_OF_SCOPE`
- diagnostic severity: `INFO`, `WARNING`, `ERROR`
- capability coverage: `COMPLETE`, `PARTIAL`, `UNAVAILABLE`, `FAILED`

This separation prevents an exactly detected frontend-internal route from appearing as an uncertain code defect.

### Diagnostics

Diagnostics are generated as canonical structured records rather than reconstructed in browser JavaScript. A diagnostic includes a stable ID, technical code, human title, category, severity, confidence, resolution, explanation, possible impact, evidence, affected artifacts, next checks, and any relevant configuration or suppression guidance.

The human-facing classifications are:

- likely defect
- missing scan coverage
- dynamic or statically ambiguous
- expected behavior
- information

`OUT_OF_SCOPE` is informational by default. Technical codes such as `indexed_backend_route_missing` remain searchable machine identifiers but are not primary UI copy.

## Directed Endpoint And Data-Flow Model

Endpoint traces become directed subgraphs rather than linear step lists. A trace contains nodes, edges, entry nodes, exit nodes, a selected main path, branches, and bounded cycles.

Supported node roles include UI route, component, event handler, API client, HTTP or gRPC route, middleware, controller or handler, function or method, validation, transformation, repository, database or table, message producer, topic or queue, message consumer, external service, and test.

Edges record relation type, callsite, evidence, confidence, synchronous or asynchronous behavior, conditionality, and known argument or result mappings.

Selecting an intermediate step must support:

- upstream and downstream highlighting
- shortest path to an entry point
- paths to persistence, messages, and external services
- relevant tests
- uncertain or truncated branches
- an explicit `Trace from here` action

Selection must not automatically change zoom or node positions. Centering remains an explicit action.

### Data Flow

Data flow models data objects, fields, parameter binding, validation, transformation, serialization, persistence reads and writes, message payloads, and return mappings. Field-level edges require evidence and confidence. Unknown transformations appear as gaps rather than fabricated mappings.

Cross-project joining uses the strongest available contract:

- HTTP method, normalized path, parameters, and content type
- gRPC package, service, method, and messages
- broker, topic or queue, event type, and payload
- datasource, schema, and table or collection
- package coordinates or workspace package ID

Service-name matching alone must not override stronger evidence.

## Dashboard Information Architecture

The dashboard is organized around developer questions:

1. `Architecture` — default view
2. `Endpoints`
3. `Data Flow`
4. `Diagnostics`
5. `Coverage`

The standalone dashboard remains local, offline, free of remote assets, and generated without requiring a frontend build pipeline.

### Architecture

The default view keeps the complete map and stable node positions. Selecting a service highlights direct incoming and outgoing relationships and slightly dims unrelated nodes without relayout. The detail panel shows role, frameworks, incoming and outgoing communication, endpoints, messages, persistence, tests, evidence, and source locations.

The useful behavior of the old Focused Service view becomes an explicit `Isolate neighborhood` action. It must preserve the full layout and offer a clear `Show full architecture` return action. Focused Service is removed as a top-level view.

Edge styles distinguish HTTP, gRPC, messaging, package dependencies, and shared persistence. Domain grouping derives from scan facts or explicit configuration rather than WEKA-specific names.

### Endpoints

Endpoints combines the useful parts of Endpoint Paths and Endpoint Trace. It supports search by route, consumer, handler, symbol, and service; groups endpoints by provider; displays status and consumer count; and opens the directed trace for a selection. Endpoint Paths is removed as a top-level view.

### Data Flow

Data Flow shows input source, fields, validation, transformations, calls, persistence, downstream payloads, and response usage. Visual treatment must clearly distinguish exact, inferred, weak, and missing mappings.

### Diagnostics

Diagnostics explains what GoreGraph could not safely connect, why it matters, whether action is required, which evidence exists, and what to check next. It replaces Open Issues as a top-level name and is not the default.

### Coverage

Coverage shows detected languages and frameworks, the capability matrix, analyzer failures, file and project coverage, unindexed projects, stale output, and known static-analysis limits. Empty results must distinguish absence of code from absence of analysis capability.

### Explanations And Interaction

Every view contains a one-sentence purpose, a compact legend, contextual `How was this determined?` evidence, useful empty states, and actionable next steps.

Interaction rules:

- selection never auto-centers
- selection never relayouts except through explicit isolation
- zoom operates around pointer or keyboard focus
- `100%` resets only zoom and pan
- `Fit` computes a real fit for visible content
- viewports are retained per view
- search and filters retain the viewport while results remain visible
- Escape exits nested focus or isolation progressively
- reduced-motion preferences are respected
- keyboard focus remains visible

The current dashboard implementation must be split into focused renderer, state, layout, interaction, detail, style, and template responsibilities as part of this work. This split is required for testability, not an unrelated rewrite.

### Design System

Before dashboard implementation, create `docs/design-system.md` with role-based color, typography, spacing, radius, focus, status, and motion tokens. The interface must remain technical, calm, precise, and restrained, with functional visual hierarchy and no decorative effects unrelated to code understanding.

## Generated Outputs

During `0.9.x`, existing outputs and meanings remain readable. The new model is additive, deprecations are explicit, and existing fields are not silently repurposed.

The intended canonical Schema 2 workspace set is:

```text
.goregraph-workspace/
  manifest.json
  capabilities.json
  projects.json
  symbols.json
  relations.json
  evidence.json
  architecture.json
  endpoints.json
  traces.json
  data-flows.json
  diagnostics.json
  coverage.json
```

Evidence is referenced by stable ID to avoid repeated metadata. Schema 2 becomes the stable public contract at `1.0.0`. Schema 1 remains detectable and produces a clear rescan instruction; generated data does not need an in-place migrator.

Primary Markdown entry points are reduced to:

- `workspace-summary.md`
- `architecture.md`
- `diagnostics.md`
- `agent-guide.md`

Specialized reports may remain as detailed references but are not presented as equal starting points.

## Query And MCP

Query and MCP operate on developer tasks rather than raw output filenames. Planned operations include workspace summary, service context, endpoint search, endpoint trace, symbol trace, data flow, change context, diagnostics, coverage, evidence, tests, and narrow source context.

Responses default to compact results with stable IDs, confidence, evidence references, coverage warnings, a suggested next query, result limits, detail levels, scopes, and continuation for large result sets. Agents should read narrow source excerpts only after generated orientation identifies likely locations.

Query supports human text, JSON, and Markdown output. MCP remains read-only over stdio, performs no automatic scan, opens no network listener, executes no project code, and modifies no project files.

Every answer verifies schema compatibility, freshness, missing workspace projects, analyzer failures, and required capabilities. Missing analysis must be described as incomplete coverage rather than an empty factual result.

## Required Language And Framework Parity

The full-support languages for `1.0.0` are Java, JavaScript/TypeScript including Node.js and React, Go, PHP, and Rust. They must answer the same developer questions using language-appropriate extraction.

### Java And Spring

- Java packages, classes, records, interfaces, enums, annotations, fields, constructors, methods, and calls
- Maven and Gradle modules
- Spring MVC and WebFlux routes, controllers, handlers, services, repositories, dependency injection, beans, filters, interceptors, security, and validation
- request and response DTOs and records
- JPA/Hibernate, JDBC, entities, tables, repositories, and queries
- RestTemplate, WebClient, and declarative HTTP clients
- JUnit and common Spring test patterns
- Kafka, RabbitMQ, and gRPC producers, consumers, stubs, and services

### JavaScript, TypeScript, Node.js, And React

- ES Modules, CommonJS, functions, classes, methods, closures, calls, TypeScript types, interfaces, enums, and relevant generics
- npm, pnpm, and Yarn workspaces
- Express, Fastify, NestJS, and Next.js route handlers and API routes
- middleware, guards, Fetch, Axios, and configurable client wrappers
- Prisma, TypeORM, Sequelize, and direct SQL foundations
- Jest, Vitest, and Node Test Runner
- KafkaJS, common AMQP patterns, and Node gRPC
- React components, JSX composition, props, state, events, hooks, effects, Context, forms, React Router, Next.js routing context, API consumers, response-field usage, and React Testing Library

UI composition and executable call relationships remain distinct edge types.

### Go

- modules, packages, files, functions, methods, interfaces, structs, imports, calls, receiver relationships, and implementation hints
- `net/http`, Chi, Gin, Echo, and Fiber routes and middleware
- JSON binding, validation, outgoing HTTP clients, and common wrappers
- `database/sql`, GORM, and sqlx foundations
- Go tests and HTTP testing
- gRPC, Kafka, and AMQP patterns
- goroutines and channels as local execution relationships without falsely implying distributed communication

### PHP

- namespaces, classes, interfaces, traits, functions, methods, calls, Composer, and PSR-4
- Laravel and Symfony routes, controllers, middleware, dependency injection, and services
- requests, responses, validation, Eloquent, Doctrine, and PDO
- Guzzle and common HTTP clients
- PHPUnit and Pest
- Laravel Queues, Symfony Messenger, common AMQP patterns, and statically visible gRPC foundations

### Rust

- Cargo packages, crates, workspaces, features, modules, functions, structs, enums, traits, implementations, imports, and calls
- Axum, Actix Web, and Rocket routes, handlers, middleware, and layers
- Serde models, Reqwest, SQLx, and Diesel
- Rust tests, Tokio tasks and channels, Tonic, Kafka, and AMQP patterns
- honest confidence for trait dispatch and macros

Recognized framework macros may be normalized. Macros that cannot be safely expanded produce a coverage gap rather than invented facts.

### Other Languages

Python and Shell retain existing support and are connected to the common evidence, confidence, diagnostics, Query, and MCP model. Kotlin, Scala, Swift, Ruby, C, C++, C#, and other best-effort languages remain explicitly labeled `Index` until they pass the same end-to-end acceptance criteria. They are not advertised as full parity in `1.0.0`.

### Project-Specific Configuration

`goregraph.yml` may add API helpers, route wrappers, service aliases, gateway prefixes, message topics, internal package conventions, domain assignments, and known out-of-scope routes. Configuration extends generic analyzers; it must not embed WEKA assumptions into core behavior.

## Release Sequence

### `0.9.1` — Dashboard Foundation

Architecture becomes default; Diagnostics replaces Open Issues; explanations, stable selection, integrated service focus, combined endpoint exploration, real Fit, per-view viewport state, dashboard decomposition, README guidance, and design tokens are delivered.

### `0.9.2` — Evidence And Capabilities

Evidence records, separated status dimensions, capability interfaces, capability and coverage outputs, the dashboard matrix, README documentation, and doctor validation are delivered.

### `0.9.3` — Diagnostics And Agent Entry Points

Canonical diagnostics, human explanations, primary Markdown entry points, task-oriented Query commands, compact MCP tools, continuation, detail controls, and freshness/coverage warnings are delivered.

### `0.9.4` — Directed Traces And Data-Flow Core

Directed endpoint subgraphs, upstream/downstream traversal, main path, branches, bounded cycles, trace-from-selection, language-independent data flow, Endpoints and Data Flow views, Query, and MCP access are delivered.

### `0.9.5` — Java/Spring And JS/TS/Node/React

The two reference ecosystems reach the required common capabilities, including frameworks, persistence, testing, messaging, gRPC, and frontend-to-backend flow.

### `0.9.6` — Go And PHP

Go and PHP reach the same end-to-end acceptance using their required framework, persistence, testing, client, messaging, and gRPC support.

### `0.9.7` — Rust And Asynchronous Architecture

Rust reaches full support. Kafka, RabbitMQ/AMQP, topics, queues, producers, consumers, async architecture edges, messaging traces, and payload flow are completed across supported languages.

### `0.9.8` — Parity And Real-Workspace Hardening

Cross-language end-to-end fixtures, mixed multi-repository fixtures, performance, cycles, deterministic output, cross-platform paths, Python and Shell integration, Index labeling, project pattern configuration, and removal of generic WEKA assumptions are completed.

### `1.0.0-rc.1`

Schema 2, CLI, Query, and MCP contracts freeze. Documentation, migration guidance, platform installation, representative real workspaces, accessibility, keyboard behavior, and all release criteria are validated. Further release candidates contain fixes only.

### `1.0.0`

Release occurs only after all required language capabilities, evidence rules, diagnostic semantics, dashboard flows, Query/MCP parity, schema stability, documentation, and clean-install-rescan acceptance pass. Pushes, tags, and public releases require explicit user authorization.

## Testing And Acceptance

### Cross-Language Fixtures

Every full-support language requires realistic fixtures covering:

```text
consumer
  -> API client
  -> route
  -> middleware or auth
  -> handler
  -> service or use case
  -> persistence
  -> downstream HTTP, gRPC, or message
  -> response
  -> tests
```

Mixed-language and multi-repository fixtures verify that architecture behavior is not tied to WEKA layout or names.

### Development Verification

Every self-contained change follows TDD and runs focused tests followed by:

```bash
gofmt
go vet ./...
go test ./...
```

Fixture, golden-file, schema, determinism, CLI, Query, and MCP contract tests are included according to the changed capability.

### Local Installation

Each completed implementation is built with the planned local version metadata and installed as the executable used for acceptance. Before scanning, verify the resolved executable and version:

```bash
command -v goregraph
goregraph version
```

The concrete installation step must replace the locally used GoreGraph executable without pushing, tagging, or publishing a release. Writing outside the repository requires the normal environment approval at execution time.

### Real Workspace Clean And Rescan

The required real-world workspace is:

```text
~/projects/weka/
```

Before destructive cleanup, review the dry-run:

```bash
cd ~/projects/weka
goregraph workspace clean .
```

Only when the plan contains exclusively generated GoreGraph output paths, execute:

```bash
goregraph workspace clean . --execute
```

Then perform a complete documented workspace scan and verify:

```bash
goregraph doctor .
goregraph workspace status .
```

Acceptance must use only outputs from the newly installed binary and fresh scan. The clean command must never remove source files or non-GoreGraph outputs.

### Output Verification

Verify schema, manifest, JSON validity, stable IDs, evidence references, absence of orphan edges, diagnostics, coverage, primary Markdown entry points, Query and MCP responses, and deterministic repeated scans.

### Dashboard Verification

Browser verification covers Architecture default state, stable service focus, isolation and return, endpoint traces and branches, Data Flow, Diagnostics, Coverage, search, filters, real Fit, zoom, pan, no unexpected jumps, keyboard use, visible focus, reduced motion, desktop and narrow viewport, long labels, empty states, large graphs, and browser-console errors.

Repeatable core paths receive automated browser tests. Visual and exploratory testing remains required after every installed fresh scan.

### Regression Rule

Every discovered regression receives a reproducible test and the smallest practical fixture. A change is complete only after the full focused-test, full-suite, local-install, Weka-clean, Weka-rescan, output, Query/MCP, and UI cycle passes.

## 1.0 Release Gates

`1.0.0` is blocked unless:

- every advertised full-support language passes the common end-to-end suite
- every public edge has evidence
- Dashboard, JSON, Markdown, Query, and MCP share the same semantics
- diagnostics distinguish likely defects from missing coverage and expected behavior
- no critical or high-severity GoreGraph defect remains open
- no analyzer failure is silent
- Schema 2, CLI, Query, and MCP contracts are documented and stable
- clean installation and fresh scan acceptance passes on supported platforms and `~/projects/weka/`
- no Weka-specific convention is required for generic success

## Out Of Scope Before 1.0

- cloud scanning or hosted graph storage
- telemetry
- AI-generated canonical facts
- automatic MCP-triggered scans
- execution of analyzed project code
- full parity claims for Index-level languages
- pushes, tags, or public releases without explicit user authorization

