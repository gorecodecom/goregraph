# GoreGraph Output Contract

GoreGraph outputs are additive. Existing field meanings must remain stable; new versions may add fields, but should not silently repurpose existing ones.

## Workspace Outputs

- `.goregraph-workspace/context.json`: discovered projects, loaded indexes, known services, and missing service details.
- `.goregraph-workspace/contract-matches.json`: frontend API contracts joined to backend routes, including unresolved and out-of-scope contracts.
- `.goregraph-workspace/feature-flows.json`: resolved frontend-to-backend flows with frontend context, backend steps, tests, request/response metadata, auth, persistence, and field risks.
- `.goregraph-workspace/feature-dossiers.json`: compact per-feature dossiers for website/code-map consumption.
- `.goregraph-workspace/feature-dossiers.md`: human-readable summary of feature dossiers.

Project overlays mirror the workspace data in each project output directory:

- `workspace-contract-matches.json`
- `workspace-feature-flows.json`
- `workspace-feature-dossiers.json`

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
