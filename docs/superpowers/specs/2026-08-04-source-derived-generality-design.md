# Source-Derived Generality Design

## Context

GoreGraph 1.3.0 has broad language adapters and deterministic workspace
projections, but a release audit found production heuristics that recognize
private service names, domain nouns, route constants, and one proprietary
JavaScript request helper. Those rules can improve the historical three-service
benchmark without proving that the same cross-service depth transfers to an
unseen workspace.

The 1.3.0 release must remove that ambiguity. Deep relationships must come from
the scanned source, the discovered workspace registry, and uniquely compatible
HTTP routes. Private benchmark material remains external acceptance evidence
and must not define production behavior.

## Goals

- Remove private service, repository, route, organization, and domain
  identifiers from production scanner, ranking, and dashboard logic.
- Resolve Java client dependencies generically from imported types and the
  discovered workspace project identities.
- Resolve HTTP provider ownership generically from request evidence, provider
  routes, and the workspace registry.
- Preserve uncertainty when source evidence is absent or multiple projects are
  equally plausible.
- Preserve the current public Schema 3 contract with additive resolution
  metadata where needed.
- Prove transfer with a committed, unrelated three-repository blind fixture and
  retain the historical workspace only as external regression evidence.

## Non-goals

- No user-maintained alias configuration is required for 1.3.0.
- No fuzzy or model-based service guessing is introduced.
- No private workspace, prompt, route, service, class, or expected-answer text
  is copied into production code or committed fixtures.
- No new runtime dependency, network request, watcher, or background process is
  added.
- No release, tag, or package publication is performed by this work.

## Approaches Considered

### Source-derived resolution — selected

Extract neutral resolution keys from client types, imports, paths, and project
identities. Resolve only a unique canonical match in workspace scope and prefer
an exact provider-route match over every name-derived candidate. This keeps
zero-configuration behavior while making every relationship reproducible from
the indexed workspace.

### User-configured aliases

Aliases would support unusual architectures, but requiring configuration would
weaken out-of-the-box behavior and could simply move benchmark specialization
into a workspace file. This remains a possible post-1.3.0 extension.

### Documented organization-specific adapter

Keeping the current rules as a named adapter would preserve the historical
result but would not support the advertised general product contract. It is
rejected for 1.3.0.

## Production Purity Invariant

Production Go and shell code under `cmd/`, `internal/`, and release-facing
`scripts/` must contain no private organization name, historical benchmark ID,
private service name, private route constant, private domain translation, or
private dashboard label. A regression test scans non-test production files for
the audited identifier set and fails with the exact path and line if one is
introduced.

Generic technical vocabulary remains allowed: HTTP methods, authentication,
configuration, persistence, retry, task, user information, service, client,
gateway, and standard framework names are product concepts rather than private
domain rules.

## Canonical Service Identity

A focused scanner helper produces canonical identity variants from a raw
project, service, package, import, type, or route-segment value:

1. Split path, dot, dash, underscore, and camel-case boundaries.
2. Lowercase tokens and discard empty tokens.
3. Remove technical wrapper tokens such as `ms`, `svc`, `service`, `services`,
   `management`, `mgmt`, `client`, `api`, `gateway`, and `connector`.
4. Produce conservative singular variants for terminal English plurals while
   retaining the original token variant.
5. Join the remaining ordered tokens without punctuation.

Examples:

- `InventoryMgmtService` and `services/inventory-service` both yield
  `inventory`.
- `OrderCatalogClient` and `order-catalog-api` both yield `ordercatalog`.
- `UserDetailsService` yields `userdetails` and does not match a project whose
  canonical identity is only `user`.

Resolution is exact over canonical variants. One matching project resolves the
relationship. Zero matches remain unresolved. Multiple matches are ambiguous
and must not produce an edge. There is no substring-only match, edit distance,
or fixed alias table.

## Java Service Dependencies

Project extraction records neutral dependency evidence from imported Java
types whose names end in a client boundary such as `Client`, `Service`,
`Gateway`, `Connector`, or `Api`. The record retains the import/type evidence
and a canonical `resolution_key`; it does not invent `to_service` during a
single-project scan.

Workspace reconciliation compares the resolution key with every discovered
project's service, name, and path identities. A unique match fills `to_project`
and the registry's actual service name. Framework types and local services do
not become cross-project dependencies unless an independently discovered
project has the exact canonical identity.

## HTTP Contracts and Provider Ownership

JavaScript and TypeScript extraction recognizes a generic
`<receiver>.request(<HTTP method>, <path>)` signature. The receiver name is not
special. The call is accepted only when the first argument is a supported HTTP
method and the second contains a structurally valid path literal. Existing
`fetch`, HTTP-client method, and helper extraction remains unchanged.

Path literals are validated by call context and syntax rather than a list of
known business endpoints. A single static segment is valid inside a recognized
HTTP call; URLs with schemes, whitespace, or unresolved complex expressions
remain rejected or explicitly unsafe.

Project extraction stores a generic `service_resolution_key` derived from the
first stable path segment and leaves the concrete service candidate empty.
Workspace reconciliation first matches exact method/path provider routes. Only
when no route resolves the owner may the canonical service resolver select one
unique registry project. Workspace output then records that project's actual
service identity. Project-only output must not report an unscanned named
service from a guessed prefix.

## Routes and Constants

The fixed route-constant replacement table is removed. Spring route extraction
uses the existing source-derived constant index, including owner-qualified and
fully qualified constants. A constant absent from indexed source remains an
unresolved expression.

Fixed placeholder values and fixed service-prefix lists are removed. Generic
configuration prefixes remain recognizable by syntax such as `base_path`, and
technical service prefixes remain recognizable by suffixes such as `service`,
`svc`, `api`, or `mgmt`. A placeholder does not exactly match an arbitrary
static segment. Multiple compatible provider routes remain ambiguous and are
listed as candidates rather than selecting the first route.

## Context Ranking

Business-noun translations are removed from built-in query aliases. Generic
intent vocabulary remains, including delete/remove, authentication,
configuration, persistence, retry, task, side effect, and testing terms.

Context selection must succeed through explicit project identities, endpoint
and symbol evidence, source-backed relationships, and requested technical
concerns. It must not translate a private problem noun into the corresponding
source noun. Optional user-defined language or domain aliases may be designed
after 1.3.0, but are not part of this change.

## Dashboard

The dashboard removes private labels and route-family presentation branches.
Diagnostic grouping uses the existing canonical diagnostic family, route
pattern, project, and resolution status. No route prefix receives a
domain-specific title.

## Error and Confidence Semantics

- Unique source-derived resolution: `RESOLVED` or `EXTRACTED`, with the raw
  evidence and canonical key retained.
- No matching project or route: `UNRESOLVED`; no absence claim is made.
- Multiple matching projects or routes: `AMBIGUOUS`, with stable sorted
  candidates.
- Complex dynamic path: existing unsafe/dynamic diagnostics remain.
- A project scan cannot infer workspace ownership and therefore leaves the
  concrete service empty.

All ordering is deterministic. Maps are converted to sorted candidate lists
before output or selection.

## Test Strategy

### Production purity guard

A Go regression test scans non-test production files and rejects every audited
private identifier. It also rejects reintroduction of a receiver-specific
request adapter or fixed route-constant map.

### Focused scanner tests

- Resolve `InventoryMgmtService` to an unrelated `inventory-service` project.
- Do not resolve `UserDetailsService` to a `user-service` project.
- Leave duplicate canonical project identities ambiguous.
- Extract `transport.request("DELETE", "inventory/items/{id}")` without
  depending on the receiver name.
- Accept an unrelated single-segment HTTP path only inside a recognized HTTP
  call.
- Resolve owner-qualified Spring constants from indexed Java declarations.
- Leave absent constants and placeholder-versus-static routes unresolved.

### Blind three-repository fixture

Create a committed synthetic workspace with unrelated names and behavior:

- `order-service` exposes a public cancellation endpoint.
- `inventory-service` owns stock-reservation and allocation-reservation cleanup
  and persistence.
- `platform-clients` provides the shared authenticated client boundary used by
  `order-service` to call `inventory-service`.

The fixture must prove endpoint selection, the current missing call,
source-derived client ownership, both reservation repositories, authentication,
configuration identities, persistence, side effects, tests, and bounded
uncertainty without any private benchmark vocabulary.

The Context Pack must be deterministic, fit the unchanged 4,000-token and
12-file limits, require no fallback, and expose only bounded omission reads.

### Existing regression coverage

All current language adapter, scanner, workspace, Context, Doctor, CLI,
benchmark-harness, and documentation tests remain green. Existing tests may
retain historical examples as regression inputs, but production behavior may
not branch on their names.

## Release Validation

1. Run focused red/green tests for each removed heuristic and generic
   replacement.
2. Run `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`,
   documentation synchronization, and every benchmark shell test.
3. Cross-build macOS amd64/arm64, Linux amd64/arm64, and Windows amd64.
4. Run the pinned GoReleaser snapshot and verify all archives and checksums.
5. Install the exact committed candidate and verify installed help and version.
6. Scan the blind fixture and require all Doctor and Context acceptance gates.
7. Clean and rescan the historical workspace, require unchanged source
   identity, all Doctor checks, and a locally equivalent bounded Context Pack.
8. Run external assisted and matched release benchmarks only under explicit
   authorization for the data sent to the external Codex service.

## Acceptance Criteria

- The production purity guard reports zero private identifiers.
- No production mapping returns a fixed private service name or route.
- Generic client, route, constant, and workspace resolution tests pass.
- Ambiguous cases produce no invented service edge or provider.
- The unrelated three-repository fixture passes its deterministic Context gates.
- The historical local Context Pack retains one reliable entrypoint, no
  fallback, no retry, and the existing budget limits without private rules.
- Full tests, race tests, vet, documentation, cross-builds, and release snapshot
  pass on the exact final tree.
- Documentation describes only evidence-backed generic behavior and keeps the
  historical token result scoped to that single frozen case.
