# GoreGraph Output Contract

GoreGraph outputs are additive. Existing field meanings must remain stable; new versions may add fields, but should not silently repurpose existing ones.

## Workspace Outputs

- `.goregraph-workspace/context.json`: discovered projects, loaded indexes, known services, and missing service details.
- `.goregraph-workspace/contract-matches.json`: frontend API contracts joined to backend routes, including unresolved and out-of-scope contracts.
- `.goregraph-workspace/feature-flows.json`: resolved frontend-to-backend flows with frontend context, backend steps, tests, request/response metadata, auth, persistence, and field risks.
- `.goregraph-workspace/feature-dossiers.json`: compact per-feature dossiers for website/code-map consumption.
- `.goregraph-workspace/feature-dossiers.md`: human-readable summary of feature dossiers.
- `.goregraph-workspace/workspace-graph.json`: normalized workspace graph with stable node and edge IDs for projects, contracts, routes, flows, features, files, symbols, and candidate routes.
- `.goregraph-workspace/workspace-service-map.json`: directed service/project relationship map with incoming/outgoing API counts, backend service-client dependencies, confidence buckets, endpoint examples, evidence files, generic service roles, and architecture domains.
- `.goregraph-workspace/workspace-endpoint-traces.json`: readable endpoint traces from frontend consumer/API contract to backend route, handler, backend steps, tests, and risks.
- `.goregraph-workspace/workspace-map.html`: Schema 1-compatible standalone offline dashboard. Its top-level UI is Architecture, Endpoints, and Diagnostics; the existing JSON payloads and field meanings remain compatible. Architecture keeps the full service map stable during ordinary selection and offers explicit neighborhood isolation, preserving the earlier Focused Service behavior without a separate top-level view. Endpoints lets users search for and select a service, inspect its caller -> endpoint -> provider rows, and open an implementation trace, preserving the earlier Endpoint Paths workflow through the service inventory. Diagnostics groups unresolved, mismatched, dynamic, and out-of-scope relationships by meaning and provides evidence and next checks. The dashboard includes per-view pan/zoom state, a visible-content Fit control, selectable nodes, source file/line links, confidence explanations, and incoming/outgoing relationship details.

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
