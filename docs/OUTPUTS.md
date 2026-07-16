# GoreGraph Output Contract

GoreGraph outputs are additive. Existing field meanings must remain stable; new versions may add fields, but should not silently repurpose existing ones.

## Workspace Outputs

- `.goregraph-workspace/context.json`: discovered projects, loaded indexes, known services, and missing service details.
- `.goregraph-workspace/contract-matches.json`: frontend API contracts joined to backend routes, including unresolved and out-of-scope contracts.
- `.goregraph-workspace/feature-flows.json`: resolved frontend-to-backend flows with frontend context, backend steps, tests, request/response metadata, auth, persistence, and field risks.
- `.goregraph-workspace/feature-dossiers.json`: compact per-feature dossiers for website/code-map consumption.
- `.goregraph-workspace/feature-dossiers.md`: human-readable summary of feature dossiers.
- `.goregraph-workspace/workspace-graph.json`: normalized workspace graph with stable node and edge IDs for projects, contracts, routes, flows, features, files, symbols, and candidate routes.
- `.goregraph-workspace/workspace-service-map.json`: directed service/project relationship map with incoming/outgoing API counts, backend service-client dependencies, confidence buckets, endpoint examples, evidence files, generic service roles, architecture domains, and a canonical `contract_summary`. The summary total always equals `resolved + missing_route + method_mismatch + dynamic_unresolved + out_of_scope + other`; Workspace Markdown, dashboard, Query, and MCP use the same counts.
- `.goregraph-workspace/workspace-endpoint-traces.json`: readable endpoint traces from frontend consumer/API contract to backend route, handler, backend steps, tests, and risks.
- `.goregraph-workspace/workspace-map.html`: Schema 2 standalone offline dashboard with six top-level views. Architecture derives dynamic domain lanes from `workspace-service-map.json`, keeps stable card coordinates during service/domain/direction/risk focus, groups background relationships through shared trunks, fans selected direct relationships to card ports, and keeps a persistent count/filter summary outside the SVG transform. Call badges describe statically detected relationships and are not runtime traffic metrics. Endpoints shows caller, provider, class, symbol, file, and line plus safe source actions. Feature Flow presents the route-to-component-to-API-to-backend-to-persistence-to-test chain. Data Flow shows field-level movement and explicit evidence gaps. Diagnostics uses normal vertical HTML scrolling at 100% scale and shares canonical explanations with Query and MCP. Coverage separates workspace completeness and prioritized next scans from analyzer capability support. Every inventory-like view uses semantic HTML, and desktop or narrow layouts preserve readable controls, keyboard focus, evidence, selection ownership, and source context.

## Full Adapter Evidence

- `architecture-capabilities.json`: normalized, deterministic file/line facts emitted by the Java/Spring, JavaScript/TypeScript/Node/React, Go, PHP, Rust, and Python full adapters for routes, API clients, tests, persistence, messaging/RPC, validation, and request/response boundaries.
- `capabilities.json`: analyzer support declarations. Reference-adapter records link detected normalized facts through `evidence_ids`.
- Query and MCP `coverage` return those evidence IDs; Query and MCP `evidence` resolve both source evidence and architecture-capability evidence.

Project overlays mirror the workspace data in each project output directory:

- `service-dependencies.json`
- `workspace-contract-matches.json`
- `workspace-feature-flows.json`
- `workspace-feature-dossiers.json`
- `workspace-graph.json`
- `workspace-service-map.json`
- `workspace-endpoint-traces.json`
- `workspace-map.md`

## Workspace Navigation Commands

- `goregraph workspace dashboard <path>` prints the generated `workspace-map.html` path.
- `goregraph workspace refresh <path>` refreshes `.goregraph-workspace` overlays from existing project outputs without scanning source files.
- `goregraph workspace explain <target>` explains the matching graph node and direct connections.
- `goregraph workspace path --from <target> --to <target>` reports the shortest graph path between two targets.
- `goregraph workspace impact --changed-file <path>` reports affected feature dossiers for explicit changed files.

Graph IDs are deterministic from node kind plus normalized semantic parts. Examples:

- `project:frontend/app`
- `contract:<contract-id>`
- `route:<project>:<method>:<path>`
- `flow:<flow-id>`
- `feature:<feature-id>`

## Confidence Semantics

- `RESOLVED`: deterministic route or flow match.
- `MISMATCH`: deterministic nearby match with a concrete incompatibility, such as same path with a different HTTP method.
- `PARTIAL_MATCH`: deterministic match after a known normalization such as a gateway/proxy prefix.
- `UNRESOLVED`: indexed data exists, but no safe match was found.
- `OUT_OF_SCOPE`: intentionally not matched against backend services, for example frontend-internal API routes.
- `EXTRACTED`: value was read from source code structure.
- `MATCHED`: test, field, or relation was joined to a concrete target.

## Change-Safety Fields

- `resolution_class`: machine-readable classification for an unresolved or mismatched contract.
- `resolution_evidence`: short evidence strings explaining the classification.
- `similar_backend_routes`: nearest indexed backend routes.
- `dynamic_endpoint_candidates`: possible dynamic endpoint suffixes.
- `equivalent_route_candidates`: nearby routes that may represent a replacement or neighbor resource.
- `missing_route_kind`: more precise missing-route class such as `neighbor_resource`.
- `backend_request_fields` and `backend_response_fields`: DTO fields extracted from Java source.
- `frontend_response_fields`: response fields used by the frontend API caller.
- `auth`: backend auth/security annotation context.
- `persistence_path`: repository/entity/table path extracted from endpoint flows and Spring metadata.
- `field_risks`: conservative frontend/backend field compatibility warnings.

## Diff Mode

`goregraph workspace diff --before <workspace-output> --after <workspace-output>` compares two `.goregraph-workspace` output directories and reports:

- new contracts
- removed contracts
- changed contract issue/confidence
- lost matched test coverage on feature flows

## Language Inventory

GoreGraph has deep route/API/test analyzers for Go, Java/Spring, JavaScript/TypeScript, Python, PHP, and Shell. It also detects and indexes best-effort symbols and imports for Rust, Kotlin, Scala, Swift, Ruby, C, C++, and C# so mixed-language workspaces still appear in `files.json`, `symbols-full.json`, `relations-full.json`, `graph-full.json`, and `analyzers.json`.
