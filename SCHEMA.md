# GoreGraph Schema

GoreGraph output is designed to be deterministic and safe for humans, CLI commands, and read-only integrations.

## Current Schema

Current schema version:

```text
3
```

The schema version is written to:

```text
manifest.json
```

Example:

```json
{
  "tool": "goregraph",
  "schema": 3,
  "output_dir": "goregraph-out"
}
```

## Compatibility Rule

<!-- goregraph:generated current-contract start -->
Source version: GoreGraph 1.4.1 with output Schema 3.
<!-- goregraph:generated current-contract end -->

Older Schema 1 and
Schema 2 indexes are not rewritten in place because mixed-generation workspaces
could otherwise combine incompatible assumptions. Install the new binary,
preview `goregraph workspace clean .`, execute only the listed generated output
cleanup with `--execute`, and run `goregraph workspace build all .`. Doctor
rejects stale manifests with this rescan guidance.

Within Schema 3, existing fields and enum meanings are frozen. Compatible
releases may add optional fields and new output files; they do not silently
change existing field, evidence, coverage, confidence, resolution, severity,
Query-task, or MCP-tool meanings.

Schema 3 adds explicit `index/`, `agent/`, and `dashboard/` ownership,
`index/api-catalog.json`, compact endpoint facts in `agent/context-index.json`,
and the eight-view dashboard without changing the meanings of retained facts.
Schema 2 remains the historical stable 1.0/1.2 contract.

GoreGraph commands only support the current schema version.

If generated output uses an unsupported schema, commands such as `doctor`, `query`, and MCP mode should report an actionable error and ask the user to refresh the output:

```bash
goregraph scan .
```

## Determinism Rules

Generated output should stay stable when repository content, config, and GoreGraph version are unchanged.

Rules:

- root-relative paths only
- `/` path separators
- sorted files
- sorted symbols
- sorted relations
- stable JSON indentation
- timestamps only in metadata/audit files where the timestamp is the purpose of the file
- no random IDs

## Generated Files

Schema 3 canonical index and report names include:

- `manifest.json`
- `files.json`
- `symbols.json`
- `relations.json`
- `graph.json`
- `symbols-full.json`
- `relations-full.json`
- `graph-full.json`
- `callgraph.json`
- `endpoint-flows.json`
- `test-map.json`
- `routes.json`
- `flows.json`
- `api-contracts.json`
- `api-catalog.json`
- `architecture-capabilities.json`
- `frontend-usage.json`
- `contract-matches.json`
- `diagnostics.json`
- `package-graph.json`
- `maven-graph.json`
- `analyzers.json`
- `spring.json`
- `audit.json`
- `report.md`
- `modules.md`
- `workspace.md`
- `endpoints.md`
- `endpoint-flows.md`
- `dependencies.md`
- `callgraph.md`
- `routes.md`
- `flows.md`
- `api-contracts.md`
- `frontend-usage.md`
- `contract-matches.md`
- `potentially-broken-contracts.md`
- `diagnostics.md`
- `workspace-context.md`
- `workspace-contract-matches.md`
- `workspace-feature-flows.json`
- `workspace-feature-flows.md`
- `workspace-next-actions.md`
- `frontend-consumers.md`
- `package-graph.md`
- `maven-graph.md`
- `navigation.md`
- `analyzers.md`
- `affected.md`
- `entrypoints.md`
- `test-map.md`

## File Contracts

Schema 3 output ownership is explicit. Project output uses
`goregraph-out/index/`, `goregraph-out/agent/`, and
`goregraph-out/dashboard/`; workspace output mirrors those directories under
`.goregraph-workspace/`. `manifest.json` remains at each output root. Complete
machine indexes are not direct prompt input.

`manifest.json` describes the generated output set.

`freshness.json` records artifact generation time, GoreGraph version, Schema 3, a deterministic source fingerprint, stale state, and an explicit reason. Source fingerprints use sorted source paths and hashes; wall-clock generation metadata is excluded. Workspace reconciliation writes the same record shape to `.goregraph-workspace/index/freshness.json`, with project-relative artifact names. Missing freshness is uncertainty and requires a rescan before absence can be trusted.

`api-catalog.json` is written to project `goregraph-out/index/` and workspace
`.goregraph-workspace/index/`. It contains the complete provider inventory.
Endpoint records include stable provider/method/path identity, source and
handler, supported request/response types, Endpoint security, consumers,
coverage, confidence, limitations, and evidence. Consumer records keep
consumer call authentication separate. `unknown` is displayed as
`No auth evidence detected`; it is not equivalent to `public`, and neither fact
claims runtime enforcement.

`agent/context-index.json` is the compact searchable agent projection. For an
endpoint task, Context emits at most one selected endpoint and eight consumers
with an explicit omitted count under the unchanged 1800-token default. It does
not expose the complete API catalog or dashboard configuration as prompt input.

Schema 3 Context Packs may add the optional `plan_files` array for exact
missing-transition change plans. Each entry contains `project`, normalized
relative `path`, and `use`; supported uses are `provider_test`, `mock_pattern`,
and `retry_pattern`. At most four exact indexed test-source
identities are emitted.

The optional `production_plan_files` array uses project groups with an optional
normalized relative `provider_contract` path and up to two normalized relative
`primary_persistence` paths. It is emitted only for exact missing-transition
file inventories from exact production-scoped provider and domain-matched
repository-owner facts that are not represented elsewhere. Dependent
persistence and inferred future behavior are intentionally excluded.

The optional `configuration_resources` array groups an exact `project` and
sorted value-free `key_groups` with nested `resources`; each resource contains
only normalized relative `path` and `profile`. Selection remains capped at six
resources before grouping.

All three fields are additive metadata: they do not contribute to source
coverage or authorize source reads, and they do not change the 4,000-token,
12-source-file, 12-source-section, or three-source-omission limits. When compact
plan metadata is present, `files.reason` is serialized as an empty string to
avoid displacing file identities; file paths, ranges, roles, confidence, and
source sections retain their normal meanings.

Workspace-root `.goregraph-dashboard.json` is user-owned configuration, not a
generated Schema 3 artifact. Its configuration schema version is 1. The
`architecture` object stores `groupOrder`, stable group IDs with editable
labels, and project-relative service keys with group/order overrides. Rebuilds
merge automatic production package/module grouping with valid overrides,
auto-place new services, and preserve stale removed-service overrides for
Doctor warnings.

`files.json` lists indexed files with path, language, size, hash, and kind.

`symbols.json` lists extracted symbols with name, kind, file, and line.

`relations.json` lists extracted relationships with source file, target, type, and line.

`graph.json` combines files, symbols, local file targets, and external dependency nodes.

`symbols-full.json` contains additive normalized symbol records with stable IDs, language, source file, and source location.

`relations-full.json` contains additive normalized relation records with stable IDs, relation type, source location, confidence, confidence score, and best-effort internal/external classification.

`graph-full.json` contains a richer directed graph inspired by Graphify-style node/edge interchange. It preserves root-relative source files and marks extracted relationships with `EXTRACTED` confidence. Rich graph edges expose `type`; `relation` remains as a compatibility alias.

`callgraph.json` is the authoritative method/function-level call graph. Java/Spring exact method declaration matches use `EXTRACTED`; language-neutral Go, PHP, JavaScript, TypeScript/React, Python, and Shell call matches use `INFERRED`. `relations.json` and `graph.json` may include a subset of call relations for broad graph navigation, but tools should use `callgraph.json` when they need method-level calls.

`endpoint-flows.json` contains Spring endpoint flow records from endpoint to controller, service, repository, and other resolved method steps.

`test-map.json` contains Java and language-neutral test mappings. Direct Java method calls use `EXTRACTED`; endpoint matches and generic test-to-production call matches use `INFERRED`.

`routes.json` contains normalized route records. Current route sources include Spring, Go `net/http`/router calls, PHP Laravel-style routes, JavaScript/TypeScript Express/Fastify-style routes, React Router routes, Redux Little Router fragments, and Python FastAPI/Flask-style decorators. Frontend routes include app-specific `route_id` values such as `portal:/settings` when they are inside `apps/<name>/...`.

`flows.json` contains normalized route-to-handler-to-call flow records. Flow steps are best-effort static orientation data and include confidence markers.

`api-contracts.json` contains statically detected Java/Spring and JavaScript/TypeScript HTTP client contracts. Java records cover imported Spring declarative clients and bound `RestClient`, `WebClient`, and `RestTemplate` receivers. JavaScript/TypeScript records cover recognized helper calls, request wrappers, and `fetch`. Records include language, HTTP method, raw and normalized path, query string, sorted query params, service candidate, caller and source location, app/package context, authentication evidence, confidence, and reason. Complex or unresolved expressions remain explicit through `unsafe_dynamic`, `dynamic_endpoint_candidates`, partial confidence, and the raw expression instead of being promoted to exact paths.

Java contract records may add `configuration_key_groups`, a sorted array of
top-level Spring configuration groups derived only from imported Spring
`@Value` placeholders or `@ConfigurationProperties` prefixes on the owning
client type, fields, methods, or parameters. The array contains group names but
never configuration values. Matching groups create exact, value-free
`configuration` edges in `agent/context-index.json`; absent or ambiguous static
evidence creates no edge.

`service_resolution_key` is an optional, value-free key derived from the first
usable normalized API-path segment. It is a workspace-only provider-resolution
hint for contracts without an explicit `service_candidate`; it does not claim a
provider identity in a standalone project scan and is omitted for
frontend-internal API paths.

`architecture-capabilities.json` contains deterministic full-adapter facts with `id`, `language`, `capability`, `kind`, `framework`, root-relative `file`, and one-based `line`. The IDs may be referenced by `capabilities.json` and are resolved by the Query/MCP evidence operation. Java/Spring, JavaScript/TypeScript/Node/React, Go, PHP, Rust, and Python use the same record shape. These facts describe detected static syntax; they do not assert that runtime-generated behavior is absent.

`frontend-usage.json` contains frontend API usage records derived from `api-contracts.json` and `flows.json`. Records include the API method/path/location, service candidate, detected API caller, best matching frontend route ID/path/component, route confidence, reason, and static chain steps when a frontend route flow reaches the API contract file or caller. `frontend-usage.md` is the readable view of the same data.

`contract-matches.json` compares frontend API contracts with backend route records from the same scan. Match records include API method/path/location, backend method/path/handler/location when available, service candidate, issue, confidence, confidence score, and reason. API contracts also include `caller` when the helper or fetch call is inside a detected JavaScript/TypeScript function or method. Issue values currently include `matched`, `method_mismatch`, `missing_backend_route`, `unscanned_service`, and `unsafe_dynamic`. `unscanned_service` means the frontend call references a recognized service candidate whose backend routes were not present in this scan, so it should not be treated as a broken route inside the scanned backend scope.

Workspace contract matches may use `ambiguous_service_identity` when a
`service_resolution_key` resolves to more than one workspace provider project.
Consumers must treat the owner as unresolved: no service candidate is selected,
and `resolution_evidence` lists the candidate projects for user review.

`diagnostics.json` contains a compact diagnosis index with `entrypoints`, `risky_contracts`, `workspace_resolved_contracts`, `unscanned_services`, `endpoints_without_tests`, `weak_flows`, and `likely_tests`. It is derived from existing route, contract, endpoint-flow, flow, test-map, and workspace overlay facts.

`diagnostic-families.json` groups repeated canonical diagnostics by technical code, service, and normalized route pattern. Each `DiagnosticFamilyRecord` contains a stable `family_id`, root cause, affected count, member diagnostic IDs, deduplicated evidence IDs, and a suggested check. Method mismatches and other distinct codes remain separate even when route prefixes match. Query, MCP, workspace service maps, and the dashboard consume these canonical families; old outputs retain trace-based dashboard grouping as a fallback.

Workspace files are additive generated outputs. When a workspace is detected, `.goregraph-workspace/registry.json` stores discovered projects with `current`, `indexed`, or `not_indexed` status. `.goregraph-workspace/context.json` stores loaded indexes, known backend services, referenced but missing services, and `missing_service_details` entries with service name, referenced contract count, matching workspace project path, and project status when available. `.goregraph-workspace/contract-matches.json` stores cross-project API contract matches between already indexed projects and may include `api_caller` from the originating API contract. `.goregraph-workspace/feature-flows.json` stores resolved end-to-end feature flows from frontend route/component/API call to backend endpoint flow and matching tests. `.goregraph-workspace/next-actions.md` summarizes workspace coverage, suggested scans for high-value missing services, weak workspace matches, and resolved flows without linked tests. Feature-flow records may include `frontend_route_id`, `frontend_route_path`, `frontend_route_file`, `frontend_route_line`, `frontend_component`, `frontend_caller`, `frontend_steps`, `frontend_confidence`, and `frontend_reason`; `frontend_caller` can come from either the resolved route flow or the API contract caller when route context remains weak. `frontend_steps` can include lightweight JavaScript/TypeScript callgraph steps such as route handlers, function calls, JSX child component hops, React effect calls, and local event handler calls that connect a rendered component to an API caller.

Schema 3 feature-flow records contain the canonical projection `model_version`, `nodes`, and `edges`. Model version 1 gives each route, component, API call, endpoint, backend step, repository step, and linked test a deterministic node ID. Typed edges reference those IDs and retain confidence, reason, evidence IDs, and source analyzer. Existing flat fields remain readable for older consumers; when canonical fields are present, Doctor validates node identities and rejects duplicate or dangling edge references.

Existing indexed siblings receive `workspace-context.md`, `workspace-contract-matches.md`, `workspace-feature-flows.json`, `workspace-feature-flows.md`, `workspace-next-actions.md`, and `frontend-consumers.md` overlay reports in their configured output directories. The readable workspace reports show API caller names in contract matches, frontend consumers, and backend endpoint consumers when available; `workspace-context.md` prioritizes missing services by contract count and suggests scan commands for discovered unindexed service projects. Workspace reconciliation may also update `diagnostics.json`, `diagnostics.md`, and `endpoints.md` with workspace-resolved contracts and frontend consumers. These overlays are regenerated from existing scan output and do not imply sibling projects were rescanned.

`package-graph.json` contains Node workspace package nodes and package dependency edges extracted from `package.json`. Internal workspace edges use reason `workspace-package-json-dependency`.

`maven-graph.json` contains Maven module/dependency nodes and dependency edges extracted from `pom.xml`. Edges use reason `pom-dependency`.

`analyzers.json` describes which language/workspace analyzers were active for the scanned project and which capabilities they provided.

`spring.json` contains Java/Spring domain records. It is empty when no Spring facts are detected.

`audit.json` records the scan command, generated files, file counts, timestamps, and safety flags. Normal scans set `network_used` and `external_commands` to `false`.

`workspace.md`, `endpoints.md`, `endpoint-flows.md`, `dependencies.md`, `callgraph.md`, `routes.md`, `flows.md`, `api-contracts.md`, `frontend-usage.md`, `contract-matches.md`, `potentially-broken-contracts.md`, `diagnostics.md`, `package-graph.md`, `maven-graph.md`, `navigation.md`, `analyzers.md`, and `affected.md` are deterministic human-readable reports. `affected.md` focuses on local file targets and filters external dependency labels. Workspace overlay Markdown files are deterministic for the currently available sibling indexes, but they can change when another project in the same workspace is scanned later.

Markdown reports are human-readable and deterministic, but not intended as strict machine APIs.

## Confidence Values

GoreGraph confidence values are static-analysis labels, not runtime proof:

- `EXTRACTED`: the fact was directly extracted from source syntax.
- `RESOLVED`: multiple static facts were connected with a deterministic match, for example frontend API method/path to backend route method/path.
- `INFERRED`: the fact was inferred from local naming, call, test, or ownership heuristics.
- `WEAK_MATCH`: GoreGraph found a possible relationship or issue, but the source expression is dynamic, incomplete, or only loosely compatible.
- `OUT_OF_SCOPE`: GoreGraph recognized a referenced service candidate, but that backend service was not represented by scanned routes.

## Language Records

Schema version 3 may contain language-specific symbols and relations. Current symbol kinds include packages, modules, classes, interfaces, traits, functions, methods, tests, scripts, headings, namespaces, autoload hints, types, and entrypoints.

Current relation types include imports, imports_internal, imports_external, includes, sources, calls, and tests. Local Go, Python, PHP, Shell, and Java relations are resolved to root-relative files where GoreGraph can do so deterministically.

## Additive 1.4.1 metadata

Schema 3 manifests may contain `generation_id`, `build_identity`,
`analysis_coverage`, and `analysis_issues`. Each projection may contain
`input_fingerprint` and `stale`. Readers must tolerate absent fields in older
outputs and report unknown identity/freshness rather than infer currency from a
generation timestamp. See [output lifecycle](docs/OUTPUTS.md#141-publication-identity-and-health).

Projection health exposes `integrity` (valid/invalid/unavailable), `freshness`
(current/stale/unknown), `coverage` (complete/partial/unsupported/unknown), a
generation ID and bounded reason codes. These axes are independent.

The opt-in `adaptive-v2` Context Pack adds protocol/generation metadata and at
most three `verification_requests`, each with a project-relative path, positive
start/end line bounds and a reason. All metadata counts against the Context Pack
budget. Strict-v1 remains the default; the historical instruction and bounded
source policy remain unchanged. Adaptive fallback codes include `index_missing`,
`index_stale`, `ambiguous_entrypoint`, `insufficient_relevance`, `insufficient_evidence`, `unsupported_analysis`, `budget_exhausted`,
`source_unreadable` and `evidence_conflict`. Missing evidence is never converted
into proof that a provider or behavior is absent.

`insufficient_evidence` preserves entrypoint confidence while requiring fallback
for uncovered requested concerns. Exact verification ranges remain bounded;
ordinary source investigation still requires the caller's existing authorization.
Inferred model and extension-point candidates do not establish runtime ownership
or create call/HTTP edges. A shared base declaration alone does not identify a
newly inferred concrete model. Adaptive health and verification metadata are
reserved before evidence selection, including their UTF-8 JSON byte costs.
Adaptive source selection reserves file capacity for verified primary declarations.
The primary-path concern requires the entrypoint and first local call bodies;
signatures alone do not satisfy it. Missing primary bodies take precedence over
supporting inventory gaps in bounded verification requests. This reservation uses
the same aggregate file union as the public limit and does not create coverage.

Low-relevance or ambiguous adaptive fallbacks can include up to three current
source sections with role `candidate`, plus up to three bounded verification
requests for omitted candidates. Query vocabulary matches do not establish a
unique entrypoint or runtime owner: confidence stays LOW, fallback stays required,
and source coverage stays partial. Source rendering, path validation, configuration
value redaction and the requested token/file limits also apply to this evidence.

The compact agent index may contain `source_hashes`, a map of indexed source paths
to SHA-256 hashes of the raw file bytes. Project indexes use project-relative keys;
workspace indexes prefix each key with the project path. Adaptive context checks
selected and concern-expanded source against these snapshots before duplicate
suppression. Changed source produces `evidence_conflict`, missing or unreadable
source produces `source_unreadable`, and stale endpoint metadata is discarded.
Absent hashes in older indexes cannot establish current source freshness. Rebuild
the agent projection to populate hashes; the agent build revision is now 2.


### Bounded CLI source reads and adaptive delivery receipts

`goregraph read <root> --request '<JSON>'` is a read-only CLI interface. It does not
change the default MCP `task_context` surface or grant permission to read source.
Strict omission bounds, adaptive verification requests, or explicit caller fallback
permissions still determine which ranges may be requested.

Request (paths are relative to the requested root; a workspace path includes the
project prefix):

```json
{"files":[{"path":"service/src/Handler.java","ranges":[[10,30],[40,50]],"seen":["<copied receipt>"]},{"path":"service/src/Model.java","ranges":[[1,40]]}]}
```

Each entry requires exactly one of nonempty `files[].ranges` or `files[].find`.
These canonical selectors are preferred. The CLI also normalizes these shorthand
forms before calling the unchanged agent API:

```json
{"files":[{"path":"service/src/Handler.java","start_line":10,"end_line":30}]}
```

```json
{"files":[{"path":"service/src/Handler.java","start_line":10,"find":{"pattern":"handle"}}]}
```

The first becomes `"ranges":[[10,30]]`. The second sets
`files[].find.start_line` to 10 only when the nested cursor is zero or omitted.
Shorthand endpoints must be positive JSON integers; nulls, decimals, strings,
booleans, missing range endpoints, nonempty ranges mixed with shorthand,
`find` with `end_line`, and cursors supplied at both levels with a nonzero nested
cursor (even equal values) are rejected. Empty or null ranges may accompany find.
Bounds are neither defaulted nor swapped. Normalization grants no read authority
and preserves all path, index, redaction, receipt, and size checks below.
The original JSON is checked against the 64 KiB cap before decoding. Unknown
root, file, and find fields and multiple JSON values remain rejected. After one
complete, strictly decoded request object, the CLI tolerates exactly one redundant
trailing `]}` pair; it cannot add fields or read authority. Other malformed wire
requests fail with exit code 2 and empty stdout. If strict decoding fails only
because a JSON string uses a backslash before a non-JSON escape character, the CLI
treats that backslash literally and decodes once more. This supports shell-nested
regexp forms such as `\(`; unknown fields, malformed standard escapes and all
reader validation remain enforced. Downstream reader errors retain exit code 1
and empty stdout. No partial source is emitted.

For caller-authorized file search, use
`"find":{"pattern":"handle|validate","before":2,"after":5,"max_matches":4}`.
`pattern` is a nonempty Go regexp (RE2), at most 1024 bytes, evaluated independently
against each fully redacted current-content line without rendered line-number
prefixes. It cannot search across lines or discover hidden configuration values.
`before`/`after` default to 0 and allow 0..100; `max_matches` defaults to 10 for
0/omitted and otherwise allows 1..32; `files[].find.start_line` defaults to 1 for
0/omitted and otherwise allows 1..2097153. Invalid selectors fail before source reads.
Multiple `find` entries for one canonical file, including aliases, retain their
independent patterns, cursors, match limits, and windows. The file is read and
redacted once, and the union of selected windows is delivered once. Mixing
`ranges` and `find` for that same canonical file remains an error; issue separate
requests and carry the receipt forward.
Find does not widen strict omission or verification read authority.

Each result `files` entry contains `path` (canonical root-relative path), `sections`
(an array of `start_line`, `end_line`, numbered `content`), `skipped_ranges`
(previously delivered inclusive pairs), and a cumulative `receipt` if any current
lines have been delivered. `eof_ranges` optionally reports requested lines past
EOF; an end beyond EOF is clamped and a start past EOF returns no section. The
reader uses the same normalized line splitting as context source sections.
`ignored_receipts` optionally counts syntactically valid receipts for a different
content/path fingerprint. Ranges-only duplicate aliases become one file entry;
all requested and seen intervals are merged before delivery.

Find results additionally contain `find.match_lines` (ascending unique selected
line numbers, always `[]` for zero), `match_count` (all matching lines at/after
`files[].find.start_line`, including already-seen matches), and
`files[].find.next_start_line` only when more matches remain (one line after the
last selected match). Copy output `files[].find.next_start_line` to request
`files[].find.start_line` and pass the cumulative receipt in `seen` to paginate.
If selected find windows would exceed the combined interval, line, or 24 KiB
result limit, GoreGraph reduces match pages in reverse request order while
retaining at least one selected match for every selector that has a match. A
reduced result has `output_limited: true` and a `next_start_line`. Resume only
the selectors whose remaining matches matter, using their cursor and cumulative
receipt. `match_count` continues to describe all matches at or after that page's
start. If the aggregate response is still too large, later whole-file results have
file-level `output_limited: true` with empty `sections`. Repeat the same selectors
with the cumulative receipts returned for files already delivered. Exact range
requests stay atomic per file and are never shortened.
When several find selectors address one canonical file, the single `find` field
is replaced by `find_results`: an array of `{"request_index":0,"result":{...}}`.
Each result has the same match/pagination fields; `request_index` identifies the
original zero-based position in request `files`, before alias grouping. Resume
each selector from its own result cursor. Match counts across selectors may
overlap; they are not counts of unique delivered lines. Single-selector output
gains only the optional `output_limited` field when an automatic result page is
necessary.
The first `max_matches` matching lines select
context windows, clamped to actual EOF including the terminal empty line where
present, then merged across overlaps/adjacency. These windows use the same receipt
subtraction and delivery as ranges; there are no duplicate search snippets.
Metadata is navigation, not a claim of source delivery. No matches succeeds with
empty sections and explicit zero metadata; a receipt is retained only when valid
previous ranges exist. Changed-source receipts are ignored as above while search
uses the current redacted content.

Optional adaptive-v2 `source_sections[].read_receipt` seeds already-delivered
ranges from the existing current-source hash without another source read. Receipts
are attached before source-option token estimation and fallback budget fitting;
strict-v1 source sections and canonical guide bytes remain unchanged. Copy the
complete receipt into `seen` for that file; carry later cumulative receipts forward.
The format is `r1:<lowercase SHA256 fingerprint>:<start>-<end>,...`. The fingerprint
hashes the version domain, canonical absolute real source path and current raw file
SHA256 with NUL separators. A changed file invalidates old ranges. Receipts describe
caller-reported delivery, not security proofs or read authority.

Requests allow at most 16 entries, 32 requested ranges or merged find windows,
500 lines per original range, 1000 requested lines total, 64 supplied receipts,
64 intervals per receipt and 64 KiB JSON. Find windows are resolved and merged
before combined interval/line caps apply; ranges-only counting is unchanged.
Find pages are reduced automatically until these caps are met, retaining at
least one match per matching selector in every delivered file, after which later
whole files are paged. A request still fails atomically when one file's exact
ranges or minimum find page cannot fit; reduce ranges, `before`, or `after`, or
split the request. Each receipt
is at most 4096 bytes. Responses, including receipts and the CLI newline, are
bounded to 24 KiB. Cumulative receipts also allow at most 64 intervals. Malformed,
versionless, structurally out-of-bounds receipts, and matching receipts beyond
current EOF fail atomically. Valid receipts for different content/path are reported
as ignored even if the old file was longer. Automatic find paging never claims
unreturned lines as delivered; its explicit cursor and `output_limited` marker
distinguish a partial page from a complete selection.

Only explicitly named indexed source paths are eligible. Generated/Git paths,
traversal, portable absolute paths, symlink escapes, nonregular, oversized and
non-UTF8 files are rejected. Requested-root and loaded-workspace confinement both
apply. Configuration values are redacted with whole-file context before slicing;
fingerprints still bind the raw bytes. Reads use the existing output read lock and
write no delivery ledger or cache, and do not modify source or generated data. Missing output locks cause an actionable error without
creating files; initialize legacy locks separately only under caller authority. Shell readers/searches are not
intercepted, so their delivered ranges must still be tracked by the caller.

### Answer path and citation validation

`goregraph answer-check --request '<JSON>'` or `--request-file <path>` checks
explicit Markdown references against a caller-supplied discovery/delivery ledger.
It neither reads the referenced sources nor authenticates that ledger. The only
file opened is an explicitly supplied request JSON file. Integrators must derive
the ledger from actual successful tool output, not receipts or match metadata.

```json
{"answer":"See `Handler.java:10-12`.","root":"/workspace","files":[{"path":"service/src/Handler.java","ranges":[[10,12]],"redacted_ranges":[]}],"repair_paths":true}
```

`files[].path` is an exact root-relative discovered identity. `ranges` contains
inclusive source lines actually delivered; `redacted_ranges` contains delivered
redacted representations, which do not prove hidden values. A metadata-only
identity may omit both arrays. `root` supports absolute citations within the
workspace; it is not opened or searched. Missing, escaping or ambiguous identities
and uncovered cited lines produce findings. Adjacent delivered intervals cover
a citation, but disjoint intervals do not prove their unread gap.

When `repair_paths` is true, only uniquely resolvable explicit path tokens are
expanded from the supplied ledger. If several files share an abbreviated name and
the citation includes ranges, the identity is resolvable only when exactly one
candidate covers every cited range in its delivered or redacted ranges. Claims,
line ranges and identities that remain ambiguous are not rewritten. The result includes `answer`, `valid`, `findings`, `repairs`,
`checked_references`, `checked_ranges`, `limitations` and
`semantic_validity: "not_verified"`. A valid result means only that the supported
explicit syntax passes these checks; it does not certify factual claims, test
behavior, authorization or completeness of freeform prose. Supported syntax and
parser limits are listed in command help. Independent semantic review remains
necessary.

Limits: 1 MiB request, 512 KiB UTF-8 answer, 1024 files, 4096 ledger ranges,
4096 explicit references, 4096 cited ranges, 4096 bytes per path, and one-based
line numbers up to 2147483647. Unknown fields, extra JSON values, malformed
ranges and exceeded limits reject with exit 2. Exit 1 reports validation findings
as JSON (or an output error); exit 0 reports validity within the stated scope.
Zero recognized file references fails rather than certifying an unchecked answer.
