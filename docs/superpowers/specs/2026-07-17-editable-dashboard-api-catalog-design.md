# Editable Dashboard and API Catalog Design

**Status:** Approved
**Date:** 2026-07-17
**Release target:** Unreleased `1.3.0`

## Purpose

GoreGraph 1.3.0 will make its workspace dashboard adaptable to real workspace
structures without embedding company-specific naming rules. It will also add a
provider-oriented API catalog that combines endpoint inventory, consumers, and
security evidence while keeping agent context deliberately compact.

The design serves two distinct consumers from the same canonical facts:

1. humans receive a complete, editable, offline-capable dashboard;
2. coding agents receive only task-relevant endpoint context within existing
   file and token budgets.

The dashboard remains a durable analysis surface. Its complete data is not used
as an agent prompt.

## Scope

### Editable architecture organization

- Replace workspace-specific architecture grouping heuristics with generic,
  evidence-backed grouping derived primarily from production package or module
  paths.
- Allow users to rename and reorder groups, reorder services, and move services
  between groups directly in a local dashboard editor.
- Persist intentional overrides in `.goregraph-dashboard.json` at the workspace
  root so they survive scans and dashboard rebuilds.
- Automatically place newly discovered services without discarding existing
  manual organization.
- Detect and expose stale overrides for projects that no longer exist.

### Dashboard usability

- Keep the existing Architecture and Endpoints views.
- Add API Catalog as the second main view, immediately after Architecture.
- Correct long source-path layout in Code Explorer so paths wrap and actions
  occupy a separate row.
- Preserve both full dashboard data and the token-efficient agent workflow.

### API Catalog

- List every statically discovered endpoint offered by a selected service,
  including endpoints with no known consumer.
- Present a compact inventory table with expandable Swagger-like details.
- Show endpoint security separately from each consumer's outbound call
  authentication.
- Preserve unknown, partial, ambiguous, and contradictory evidence instead of
  guessing.
- Support Java/Spring and JavaScript/TypeScript projects through one
  language-neutral model.

### Agent usability

- Write a complete canonical API catalog for tools and the dashboard.
- Project only relevant route, security, consumer, request, and response facts
  into agent context.
- Keep existing context token, file, result, and continuation budgets.
- Never include dashboard layout configuration or the full API catalog merely
  because an agent was told to use GoreGraph.

### Documentation and release handling

- Explain the editor, static dashboard, API Catalog, agent behavior, and file
  outputs in `README.md` and `COMMANDS.md`.
- Keep the source version at unreleased `1.3.0`.
- Do not create a tag, GitHub Release, Homebrew/Scoop publication, or other
  release artifact as part of this work.

## Non-goals

- A remotely hosted, multi-user dashboard editor.
- Browser-local persistence as the authoritative configuration.
- Runtime API traffic, call frequency, latency, or production authorization
  decisions.
- Executing scanned source code, build scripts, services, or security
  configuration.
- Inferring authentication from service names, endpoint names, or conventions
  without source evidence.
- Replacing OpenAPI or Swagger as a complete runtime API specification.
- Sending the complete catalog, dashboard payload, or layout configuration to
  coding agents.
- Hardcoded WEKA groups such as VD, WPO, Hekate, Portal, or RDBV.

## Architecture Overview

Existing project scans remain the source of language-specific facts. Workspace
reconciliation produces canonical, language-neutral projections. The dashboard
and agent context consume different projections of those facts:

```text
project analyzers
  -> project route, call, type, and security evidence
  -> workspace reconciliation
       -> index/api-catalog.json            complete canonical catalog
       -> dashboard payload                 complete human presentation
       -> agent/context-index.json          compact searchable projection
       -> goregraph context                 selected, budgeted task context

.goregraph-dashboard.json
  -> architecture layout merge only
  -> dashboard payload
  -> never agent context
```

There is one canonical interpretation of an endpoint, consumer, security fact,
confidence, and coverage state. Dashboard generation and agent projection do
not independently re-resolve source evidence.

## Workspace Dashboard Configuration

### Location and ownership

The authoritative dashboard customization file is:

```text
<workspace-root>/.goregraph-dashboard.json
```

It is user-owned, persistent, and not generated into
`.goregraph-workspace/`. A rescan reads and merges it but never replaces it
unless the user explicitly saves changes in the editor or invokes an explicit
reset action.

The file contains presentation choices only. It must not contain source text,
credentials, tokens, endpoint secrets, or scan results.

### Initial schema

The first schema version is intentionally small and extensible:

```json
{
  "schema": 1,
  "architecture": {
    "groupOrder": ["com.weka.vd", "com.weka.wpo"],
    "groups": {
      "com.weka.vd": {
        "label": "VD"
      },
      "com.weka.wpo": {
        "label": "WPO"
      }
    },
    "services": {
      "microservices/ms-cadaster": {
        "group": "com.weka.vd",
        "order": 20
      }
    }
  }
}
```

Group keys and service keys are stable machine identities; labels are editable
presentation text. A renamed label therefore does not break future automatic
matching. Project-relative workspace paths are used as service keys because
display names may collide or change.

Unknown fields are rejected for the active schema version to catch spelling
errors. Later schema versions require an explicit migration path.

### Merge semantics

Dashboard generation follows this deterministic order:

1. discover current workspace projects;
2. calculate automatic group proposals and stable group identities;
3. apply configured service-to-group assignments that still reference current
   projects;
4. apply configured labels, group order, and service order;
5. place new or unconfigured services in their detected groups;
6. append new groups deterministically after explicitly ordered groups;
7. retain stale overrides in the config file and report them as stale instead
   of deleting them silently.

Repeated builds with unchanged inputs produce byte-stable ordering.

## Generic Automatic Grouping

### Evidence priority

Automatic grouping uses the strongest available production evidence in this
order:

1. Java/Kotlin production package namespaces from production source sets;
2. JavaScript/TypeScript workspace package names, module namespaces, and
   production import/source paths;
3. other supported language module/package identities when equivalent evidence
   exists;
4. project role and workspace-relative project path as an explicit fallback.

Test fixtures, generated sources, vendored dependencies, build outputs, and
`goregraph-out` directories do not influence grouping.

### Namespace differentiation

For a set of services, GoreGraph removes meaningful shared namespace prefixes
and selects the first stable segment that differentiates the services. For
example:

```text
com.weka.vd.api.cadaster      -> group identity com.weka.vd, label VD
com.weka.wpo.product          -> group identity com.weka.wpo, label WPO
com.weka.portal.navigation    -> group identity com.weka.portal, label Portal
```

The strings `vd`, `wpo`, and `portal` have no special meaning to GoreGraph.
They are results of the namespace structure. The same algorithm must produce
useful group proposals for unrelated organizations and naming conventions.

A segment is not selected merely because it appears once. The implementation
must consider namespace frequency, common-prefix depth, project coverage, and
whether the result separates more than one service meaningfully. When package
evidence is sparse or contradictory, the fallback is used and the lower
confidence is exposed.

Every automatic assignment records its source, detected value, confidence, and
fallback reason so the editor can distinguish `Auto` from `Manual` placement.

## Local Dashboard Editor

### Command contract

The writable editor is started explicitly:

```shell
goregraph workspace dashboard edit [path]
```

The path defaults according to existing workspace marker and path rules. The
command validates the workspace, loads the current dashboard outputs and
configuration, starts an ephemeral loopback server, prints the local URL, and
opens it with the same platform integration as the existing dashboard `open`
command. The command remains in the foreground until interrupted with Ctrl-C.
If automatic browser opening fails, the printed URL remains usable and the
server continues running.

Existing static dashboard commands remain backward compatible and read-only.
Opening an exported HTML file directly never grants write access.

### Security boundary

The editor server:

- binds only to `127.0.0.1` or the equivalent explicit IPv6 loopback;
- selects an available ephemeral port;
- creates a cryptographically random per-process session token;
- requires the token for configuration reads and writes;
- validates `Host` and `Origin` against the active loopback session;
- accepts only the allowlisted dashboard-config endpoint and schema;
- limits request body size and request method;
- never accepts an arbitrary filesystem path from an HTTP request;
- serves only known dashboard assets and data from the selected workspace;
- does not enable permissive CORS;
- stops cleanly on process termination.

The token must not be written into generated static files or persistent config.

### Safe persistence

Saving validates the complete schema before touching the authoritative file.
The server writes to a sibling temporary file, flushes as supported, and
atomically replaces `.goregraph-dashboard.json`.

The editor carries a revision derived from the loaded configuration. A save
against a changed revision is rejected as a conflict instead of overwriting
newer filesystem changes. The user can reload and reapply the pending layout.

### Edit interaction

Architecture includes an explicit `Edit layout` mode. Normal browsing cannot
change persistent layout accidentally. Edit mode provides:

- drag-and-drop group ordering;
- drag-and-drop service ordering within a group;
- moving a service between groups;
- direct group-label editing;
- `Auto` and `Manual` assignment indicators;
- Save, Discard, and Reset to detected actions;
- a keyboard-accessible alternative for every drag operation;
- visible unsaved, saving, saved, validation-error, and conflict states.

Reset to detected removes architecture overrides after confirmation but does
not delete scan outputs. Discard restores the last loaded persistent state.

## Dashboard Navigation

The main workspace navigation order becomes:

```text
Architecture
API Catalog
Endpoints
Feature Flow
Data Flow
Code Explorer
Diagnostics
Coverage
```

`Endpoints` is retained as the relationship- and trace-oriented view. `API
Catalog` is the provider-oriented inventory. The two views must not silently
change meaning or duplicate each other under different names.

## Code Explorer Path Layout

For a selected symbol, the center panel presents content in this order:

1. qualified/export name;
2. direct-reference, API-reachability, confidence, and coverage summary;
3. full source path on its own wrapping row;
4. `Copy path` and `Open source` on a separate action row.

The right details panel and inventory cards follow the same path-then-actions
pattern. Long paths wrap within the available width and never require the user
to horizontally scroll the workbench. Buttons must not float inside path text.

## Canonical API Catalog

### Output

Workspace reconciliation writes the complete catalog to:

```text
.goregraph-workspace/index/api-catalog.json
```

Project-level builds write the corresponding canonical project catalog under
their existing `goregraph-out/index/` structure. Workspace IDs namespace project
records without changing their project source identity.

### Endpoint record

Each endpoint contains at least:

- stable endpoint ID;
- provider project/service identity and role;
- transport, HTTP method, normalized path, and raw path evidence;
- language and framework;
- controller/module, handler symbol, source file, and one-based line;
- path, query, header, cookie, and body parameter facts;
- consumed and produced content types;
- request and response type identities when statically supported;
- endpoint security requirements;
- zero or more resolved or candidate consumers;
- extraction confidence, reconciliation confidence, coverage, limitations, and
  evidence IDs.

An endpoint with no known consumer remains in the catalog with an empty
consumer list and an honest coverage explanation.

Stable endpoint identity is derived from provider identity, transport, method,
normalized path, and handler provenance. Discovery order is never part of the
ID.

### Consumer record

Each endpoint consumer contains at least:

- consumer project/service identity and role;
- caller symbol/module when known;
- source file and one-based line;
- resolved contract or call path;
- outbound call authentication evidence;
- resolution and confidence;
- mismatch classifications;
- evidence IDs and coverage limitations.

Multiple call sites from one service may be summarized in the table but remain
available as individual evidence in expanded details.

## Security and Authentication Model

Endpoint security and consumer call authentication are separate facts:

- endpoint security describes what the provider requires or permits;
- call authentication describes what a specific consumer appears to send or
  configure.

The normalized categories are:

- `basic`
- `bearer`
- `oauth2`
- `api_key`
- `session`
- `mtls`
- `role`
- `authenticated`
- `public`
- `unknown`

Records retain framework-specific evidence such as a Spring security expression,
OpenAPI security scheme, interceptor configuration, client filter, or explicit
header construction. A normalized category never replaces the source evidence.

`public` requires explicit evidence such as `permitAll`, an equivalent
framework rule, or an explicit OpenAPI declaration that means no authentication
for that operation. Absence of detected security is `unknown`, displayed as
`No auth evidence detected`.

When evidence conflicts, GoreGraph preserves each source, marks the normalized
result as conflicting or partial, and avoids choosing the most convenient
interpretation. Static findings are not claims about runtime authorization.

### Mismatch diagnostics

The catalog may report evidence-backed warnings such as:

- provider requires Basic but consumer evidence indicates Bearer;
- provider requires authentication but no consumer call-auth evidence is found;
- consumer sends credentials while provider is explicitly public;
- multiple provider security rules conflict;
- consumer-to-provider resolution is ambiguous.

Missing call-auth evidence is not automatically a broken call. The warning must
state that static evidence is incomplete and include its confidence.

## API Catalog Experience

### Service selection and filters

The view opens with a searchable provider-service selector. Selecting a service
shows all of its discovered endpoints. Filters cover:

- free text;
- HTTP method;
- endpoint security;
- consumer service;
- resolution/status.

The empty state differentiates `no endpoints discovered`, `service not
analyzed`, `analysis partial`, and `filters removed all rows`.

### Compact inventory table

The collapsed table prioritizes scanability. Each row shows:

- method and path;
- concise operation/handler identity;
- endpoint security;
- consumer count and compact consumer summary;
- consumer call-auth summary;
- confidence/status and warning indicator.

Sorting is deterministic. User-selected filters and sorting are presentation
state only unless a later schema explicitly persists them.

### Expandable Swagger-like details

Expanding a row shows, when supported:

- handler/controller and source evidence;
- path, query, header, cookie, and body parameters;
- request and response type identities;
- consumed and produced content types;
- provider security evidence and confidence;
- each consumer, call site, call authentication, and resolution evidence;
- mismatch explanations and coverage limitations;
- safe source and copy-path actions.

Unsupported fields are omitted or explicitly labeled unsupported; they are not
filled with guessed values.

## Language Support

### Java and Spring

The Java/Spring analyzers extend existing route, type, feature-flow, and auth
facts. Supported evidence may include route annotations, controller methods,
method parameters, request/response types, Spring Security expressions and
annotations, explicit filter/configuration rules, OpenAPI annotations, and
statically traceable HTTP clients.

Security extraction must distinguish endpoint-local rules from broader
configuration whose path matching is partial or ambiguous.

### JavaScript and TypeScript

JavaScript/TypeScript analyzers use existing HTTP contracts, API helpers,
imports, callers, and type evidence. Provider routes are included for supported
server frameworks when a handler and route are statically identifiable.
Consumer call authentication may be derived from explicit headers, client
configuration, interceptors, or supported auth helpers only when the call
association is evidenced.

Framework-specific gaps remain visible in coverage. The canonical model and UI
do not imply identical extraction depth across languages.

## Agent Projection and Query Behavior

### Compact index

The agent projection remains under:

```text
.goregraph-workspace/agent/context-index.json
```

It gains compact searchable endpoint facts sufficient to locate relevant
catalog records. It does not embed expanded Swagger details, all consumers, the
dashboard payload, or `.goregraph-dashboard.json`.

### Context selection

When a query selects a productive endpoint, `goregraph context` may return:

- provider and normalized method/path;
- handler and primary source location;
- request and response type identities when relevant;
- endpoint security category, evidence summary, and confidence;
- relevant consumer call sites and their call authentication;
- mismatches and coverage limitations;
- stable IDs or follow-up commands for deeper evidence.

Only consumers relevant to the query are preferred. If the query requires a
general endpoint overview, consumers are bounded and an omitted count is shown.
Continuation or explicit follow-up retrieves more detail.

Existing hard budgets remain authoritative. Catalog enrichment may improve
selection quality but may not expand prompts beyond those budgets. Relevance is
measured against productive source files; tests, generated artifacts, reports,
and dashboard files remain deprioritized or excluded according to existing
context policy.

## Error Handling and Recovery

### Invalid configuration

- The editor rejects invalid writes with field-specific errors.
- CLI dashboard generation reports the file, schema path, and reason.
- `goregraph doctor` reports invalid schema, unknown project references, invalid
  group references, and stale overrides.
- Invalid configuration is never ignored silently.
- A failed rebuild does not delete or partially replace the last successfully
  generated dashboard.

### Editor conflicts and process failure

- Revision conflicts return a dedicated conflict response and preserve both the
  filesystem file and pending browser state.
- Interrupted atomic writes leave either the old valid file or the new complete
  file, not a partial JSON document.
- If the editor server stops, the browser clearly reports that saving is no
  longer available; the exported dashboard remains readable.

### Partial analysis

Missing analyzer support, failed project scans, unavailable source indexes, and
ambiguous endpoint resolution remain coverage states in the catalog and UI.
They do not become empty-success results.

## Compatibility and Migration

- Without `.goregraph-dashboard.json`, builds use deterministic automatic
  grouping and remain fully non-interactive.
- The file is created only after an explicit successful editor save.
- Existing architecture selections, risk semantics, source actions, and static
  offline dashboard behavior remain supported.
- Existing Endpoints behavior is retained under its current name.
- Existing canonical route, contract, feature-flow, and symbol outputs remain
  valid; the API catalog is an additive projection.
- No Weka-specific legacy group name is migrated into the generic detector.
  Users may preserve desired labels through the editor.

## Documentation

`README.md` must include:

- the distinction between static viewing and writable editor mode;
- the exact editor command and workspace requirement;
- the Architecture, API Catalog, and Endpoints responsibilities;
- automatic package/module grouping and manual persistence;
- the endpoint-security versus consumer-auth distinction;
- the fact that agent context uses a compact projection rather than the full
  dashboard/catalog;
- the integration-depth table updated for Java/Spring and JavaScript/TypeScript
  endpoint, consumer, security, and type support.

`COMMANDS.md` must include:

- command syntax and examples for `goregraph workspace dashboard edit`;
- lifecycle, loopback-only behavior, Save/Discard/Reset semantics, and errors;
- static project versus workspace dashboard build scope;
- API Catalog output locations and relevant context/query operations;
- examples that do not imply unsupported security certainty.

Global and command-specific help must clearly distinguish build, open/view, and
edit operations. Generated-output documentation must describe
`index/api-catalog.json`, `.goregraph-dashboard.json`, and the compact agent
projection.

## Test-Driven Implementation Requirements

Every behavior change begins with a failing test. The implementation includes:

### Configuration and grouping tests

- Java/Kotlin package grouping across multiple neutral fixture namespaces;
- JavaScript/TypeScript package/module grouping;
- production-source precedence over tests and generated files;
- sparse, conflicting, and absent namespace fallbacks;
- deterministic IDs and ordering across discovery permutations;
- merge behavior for renamed, moved, reordered, new, and removed services;
- schema validation, stale references, migration rejection, and atomic writes;
- optimistic revision-conflict handling.

### Editor server tests

- loopback-only binding;
- token, Host, Origin, method, route, body-size, and schema enforcement;
- rejection of path traversal and arbitrary file access;
- clean shutdown and interrupted-save recovery;
- preservation of the last valid dashboard after failure.

### API catalog tests

- full endpoint inventory including zero-consumer endpoints;
- stable identity and deterministic serialization;
- Java/Spring and JavaScript/TypeScript provider/consumer fixtures;
- parameters, media types, request/response types, handlers, and evidence;
- Basic, Bearer, OAuth2, API Key, session, mTLS, role, authenticated, explicit
  public, unknown, partial, and conflicting security cases;
- separate provider-security and consumer-call-auth fields;
- mismatch classifications without false runtime certainty;
- ambiguous, unresolved, failed, and unsupported coverage states.

### Dashboard tests and visual verification

- main navigation order and view semantics;
- searchable service selector and every filter;
- compact-row rendering and expanded Swagger-like details;
- drag-and-drop and keyboard-equivalent organization;
- Save, Discard, Reset, conflict, stale, and validation states;
- persistence across a new process, rescan, and dashboard rebuild;
- Code Explorer path wrapping and separate action rows;
- keyboard navigation, focus visibility, labels, and screen-reader semantics;
- browser checks at 1280, 1440, and 1920 pixel widths;
- manual visual inspection using representative dense workspace fixtures.

### Agent and regression tests

- relevant endpoint selection for productive Java and JS/TS queries;
- bounded consumer detail with explicit omitted counts;
- no full-catalog or dashboard-config leakage into agent context;
- unchanged hard file/token/result budgets;
- benchmark comparison against current 1.3.0 context fixtures to detect token
  regressions and irrelevant-file expansion;
- global help, command help, README examples, `COMMANDS.md`, and output docs;
- complete existing Go test, lint, and dashboard browser suites.

## Acceptance Criteria

The design is complete when all of the following are true:

1. A workspace with packages such as `com.example.alpha` and
   `com.example.beta` receives useful automatic groups without organization-
   specific rules.
2. A user can rename and reorder groups, reorder services, and move services in
   the local editor; the result survives a rescan and dashboard rebuild.
3. New services are placed automatically while existing manual choices remain
   stable, and removed services are reported as stale.
4. Static exported dashboards remain offline, read-only, and usable without a
   running server.
5. Code Explorer displays long paths below symbol metadata with actions on a
   separate row and without horizontal workbench scrolling.
6. API Catalog lists every discovered provider endpoint, including endpoints
   with no consumers, and expands to supported Swagger-like details.
7. Endpoint security and per-consumer call authentication are visibly separate;
   missing evidence is `unknown`, never implicitly public.
8. Java/Spring and JavaScript/TypeScript facts share the canonical model while
   retaining honest framework-specific coverage.
9. Agent context retrieves relevant endpoint facts without including the full
   catalog or increasing existing hard budgets.
10. README, `COMMANDS.md`, CLI help, and output documentation explain the same
    workflows and limitations consistently.
11. All new behavior is covered by failing-first tests, the complete regression
    suite passes, and the final dashboard receives documented visual inspection.
12. The repository remains on unreleased `1.3.0` with no publication or release
    artifact.
