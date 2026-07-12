# GoreGraph Release Checklist

## Current Release

Current release target:

```text
v0.9.3
```

`1.0.0` is reserved for a stable public CLI and schema contract.

## Required Secrets

GitHub repository secrets:

- `HOMEBREW_TAP_TOKEN`: token with write access to `gorecodecom/homebrew-tap`.
- `SCOOP_BUCKET_TOKEN`: token with write access to `gorecodecom/scoop-bucket`.
- `WINGET_TOKEN`: optional token for pushing Winget manifests to the configured `gorecodecom/winget-pkgs` fork.

`GITHUB_TOKEN` is provided by GitHub Actions for publishing the GoreGraph release.

## Public Release Status

`v0.1.0` through `v0.9.0` established packaging, static code graphs, cross-project contracts, and the workspace dashboard. `v0.9.1` delivered the Architecture-first dashboard foundation. `v0.9.2` added stable evidence and honest capability coverage. `v0.9.3` is the current local development version for canonical diagnostics, bounded task-oriented Query/MCP operations, continuation, and primary agent Markdown entry points.

`v0.9.2` acceptance requires `evidence.json`, `capabilities.json`, `coverage.json`, and `coverage.md`; additive `evidence_ids` on supported public facts; valid Doctor integrity checks; and a clean installed-binary workspace rescan. It does not claim full language parity, directed trace branching, or the Schema 2 public freeze.

Completed release checks:

- `HOMEBREW_TAP_TOKEN` exists and can publish to `gorecodecom/homebrew-tap`.
- `gorecodecom/goregraph` is public.
- `gorecodecom/homebrew-tap` is public.
- GitHub release workflow completed successfully.
- Release artifacts and `checksums.txt` are present.
- Homebrew Formula was generated in `gorecodecom/homebrew-tap`.
- `brew audit --formula --strict gorecodecom/tap/goregraph` passes.
- `brew install gorecodecom/tap/goregraph` installs `v0.1.0`.
- `goregraph version` works for the Homebrew-installed binary.
- `brew test gorecodecom/tap/goregraph` passes.
- `gorecodecom/scoop-bucket` is public.
- Scoop manifest `bucket/goregraph.json` exists for `v0.1.0`.
- `gorecodecom/winget-pkgs` fork exists for Winget manifest publishing.
- `SCOOP_BUCKET_TOKEN` is visible in `gorecodecom/goregraph`.
- `WINGET_TOKEN` is visible in `gorecodecom/goregraph`.

`v0.2.0` feature checks:

- `graph-full.json`, `symbols-full.json`, and `relations-full.json` are generated.
- `audit.json` confirms normal scans use no network and execute no external commands.
- `spring.json`, `endpoints.md`, and `dependencies.md` are generated for Java/Spring projects.
- `workspace.md` captures Maven and Node package metadata.
- `goregraph query . graph-full`, `goregraph query . endpoints`, and `goregraph query . audit` work.

`v0.2.1` release checks:

- GitHub release artifacts are published.
- Homebrew tap update succeeds.
- Scoop bucket update succeeds.
- Winget manifests are generated and pushed to the configured fork branch.
- Automatic PR creation against `microsoft/winget-pkgs` is disabled until a token/workflow with upstream PR permissions is chosen.

`v0.4.0` feature checks:

- `callgraph.json` and `callgraph.md` are generated.
- `endpoint-flows.json` and `endpoint-flows.md` are generated.
- `test-map.json` is generated and `test-map.md` contains method/endpoint mappings.
- `analyzers.json` and `analyzers.md` are generated.
- `graph-full.json` edges contain `type` while keeping `relation`.
- multipart Spring endpoints include request kind/type metadata.

`v0.5.0` feature checks:

- `routes.json` and `routes.md` are generated.
- `flows.json` and `flows.md` are generated.
- `navigation.md` is generated.
- `callgraph.json` includes best-effort non-Java call edges for Go, PHP, JS/TS, Python, and Shell.
- `test-map.json` includes best-effort non-Java test-to-production mappings.
- Analyzer inventory marks implemented route/call/test support for Go, PHP, JS/TS, Python, and Shell.

`v0.6.0` feature checks:

- `api-contracts.json` and `api-contracts.md` are generated.
- `package-graph.json` and `package-graph.md` are generated.
- `routes.json` includes app-specific `route_id` values for frontend monorepos.
- Redux Little Router `Fragment forRoute` records prefer statically rendered components when visible.
- JS/TS callgraph output filters low-signal declaration/archive/storybook paths and common test/helper call targets.
- `goregraph query . api-contracts` and `goregraph query . package-graph` work.

`v0.7.0` feature checks:

- realistic helper calls such as `GetHelper(dispatch, "/path")` populate `api-contracts.json`.
- multiline helper calls populate `api-contracts.json`.
- template placeholders are normalized from `${id}` to `{id}`.
- same-name frontend route handlers prefer the owning `apps/<name>` tree.
- `maven-graph.json` and `maven-graph.md` are generated.
- `goregraph query . maven-graph` and `goregraph query . maven-graph-json` work.

`v0.8.0` feature checks:

- `contract-matches.json`, `contract-matches.md`, and `potentially-broken-contracts.md` are generated.
- `api-contracts.json` preserves raw path, normalized path, query metadata, service candidate, and unsafe dynamic URL markers.
- exact frontend API method/path patterns match backend routes as `RESOLVED`.
- backend path matches with different HTTP methods are reported as `method_mismatch`.
- unsafe dynamic frontend template URLs are reported as `unsafe_dynamic`.
- `goregraph query . contract-matches`, `goregraph query . contracts`, `goregraph query . contract-matches-json`, and `goregraph query . broken-contracts` work.

`v0.8.1` local feature checks:

- `diagnostics.json` and `diagnostics.md` are generated.
- `contract-matches.json` reports recognized frontend service calls as `unscanned_service` when that backend service was not scanned.
- same-scope missing backend routes continue to report `missing_backend_route`.
- `affected.md` filters external dependency labels and focuses on local file impact.
- `goregraph query . diagnostics` and `goregraph query . diagnostics-json` work.

`v0.8.2` local feature checks:

- `goregraph scan .` auto-detects common workspaces such as `frontend/` plus `microservices/`.
- `.goregraph-workspace/registry.json` lists discovered sibling projects with `current`, `indexed`, or `not_indexed` status.
- later scans refresh `workspace-context.md`, `workspace-contract-matches.md`, and `frontend-consumers.md` in already indexed sibling outputs.
- cross-project frontend API contracts match backend routes from already indexed sibling services.
- `goregraph scan . --no-workspace` skips workspace registry and overlay writes.
- `goregraph workspace status .` shows the detected workspace without scanning or writing files.

`v0.8.3` local feature checks:

- `goregraph query . workspace-contracts` works from the workspace root when `.goregraph-workspace/` exists.
- project `workspace-context.md` distinguishes `This project` from `Last refreshed by`.
- project `diagnostics.md` and `diagnostics.json` include workspace-resolved contracts.
- backend `endpoints.md` includes a `Frontend Consumers` section.
- `workspace-feature-flows.json` and `workspace-feature-flows.md` connect frontend API calls to backend endpoint flow steps and tests.
- `goregraph query . workspace-features` works from scanned projects and workspace roots.
- `manifest.json` and `audit.json` list workspace overlay files.
- static path segments no longer method-mismatch against backend `{param}` routes.
- `goregraph scan .` adds `.goregraph-workspace/` to the detected workspace root `.gitignore` unless `--no-update-gitignore` is used.

`v0.8.4` local feature checks:

- backend `diagnostics.md` lists incoming workspace-resolved frontend contracts instead of showing `none detected` when `frontend-consumers.md` has callers.
- frontend `frontend-consumers.md` explains that the backend-consumer report is not applicable and points to workspace contract/feature reports.
- `workspace-feature-flows.md` explains why resolved flows have no linked tests and points to backend diagnostics `Endpoints Without Tests`.
- `goregraph version` reports `0.8.4`.

`v0.8.5` feature checks:

- `workspace-feature-flows.md` includes frontend route/component context when a route flow reaches the API contract caller.
- unresolved frontend route context is explicit instead of silent.
- workspace feature flow JSON includes route ID, route path, component, frontend call steps, confidence, and reason.
- `goregraph version` reports `0.8.5`.

`v0.8.6` feature checks:

- frontend route flows treat JSX child components as callgraph steps when a route component renders the component that calls an API helper.
- workspace feature flows can upgrade from app-scope `WEAK_MATCH` to `RESOLVED` when the component-aware route flow reaches the API contract caller.
- workspace root detection prefers a parent with both frontend and backend group directories over an intermediate frontend-only grouping folder.
- frontend route flows can resolve API callers through React effect calls and local event handlers.
- workspace feature flow reasons distinguish direct, effect, event-handler, and app-scope matches.
- API contracts retain the enclosing helper/fetch caller name and workspace feature flows use it even when route context is only `WEAK_MATCH`.
- API contract, workspace contract, frontend-consumer, and backend endpoint-consumer Markdown reports show the detected API caller name when available.
- workspace context prioritizes referenced but missing services by contract count and suggests `cd <project> && goregraph scan .` for discovered unindexed service projects.
- `frontend-usage.json` and `frontend-usage.md` explain frontend route/component/API usage chains with confidence and evidence.
- `workspace-next-actions.md` summarizes workspace coverage, high-value missing service scans, weak workspace matches, and resolved flows without linked tests.
- `goregraph workspace scan-missing` previews prioritized missing service scans by default and executes them only with `--execute`.
- `goregraph version` reports `0.8.6`.

`v0.8.7` feature checks:

- workspace diagnostics update local risky contracts from workspace contract classifications after sibling scans.
- indexed workspace services with missing backend routes are no longer kept in the `Unscanned Services` diagnostics section.
- `goregraph version` reports `0.8.7`.

`v0.8.8` feature checks:

- `workspace-contract-matches.json` is generated in each project output as a project-local workspace overlay.
- later sibling scans refresh `workspace-contract-matches.json` for already indexed frontend and backend projects.
- `goregraph version` reports `0.8.8`.

`v0.8.9` feature checks:

- `goregraph workspace scan-all` scans every discovered workspace project and refreshes workspace overlays.
- `goregraph workspace scan-all --dry-run` prints the plan without scanning.
- `goregraph workspace clean` lists generated workspace output paths without deleting by default.
- `goregraph workspace clean --execute` removes project output directories and `.goregraph-workspace/`.
- unresolved contracts for indexed services are classified as `indexed_backend_route_missing` or `dynamic_endpoint_unresolved` instead of staying in the generic missing-route bucket.
- API contracts that match backend routes after dropping a common gateway/proxy prefix are classified as `gateway_or_proxy_prefix` with backend route context.
- frontend-internal Next-style `/api/...` calls are classified as `frontend_internal_api` instead of missing `ms-api` services.
- indexed route-gap reasons include the nearest backend route hints when useful.
- service gateway prefixes and Spring `ApplicationConfig.BASE_PATH` constants are normalized during contract matching.
- known frontend service prefixes such as `documentdownload`, `documenttopic`, and `containertree` are normalized when matching backend base paths.
- optional template suffixes such as `${filter}`, `${filter || ''}`, and ternary path suffixes are normalized to their stable base route.
- unsafe dynamic path placeholders can still resolve when the backend route has a placeholder at the same path position.
- known RegulationChange controller path constants are expanded for workspace contract matching.
- backend route paths and similar-route hints are rendered with expanded controller/config constants for readable reports.
- component/page-local API callers can promote otherwise weak frontend route context to resolved feature-flow context.
- MockMvc tests with Java string-concatenated request paths are normalized with dynamic placeholders for stronger endpoint test mapping.
- workspace reports no longer render missing owners as `none.ControllerName`.
- package UI, package utility, app root, container-local, and relation-backed frontend API callers resolve workspace feature-flow context.
- frontend `relations.json` is loaded into the workspace overlay so component/page/container imports can resolve API caller context.
- WebTestClient request chains, MockMvc builder references, local URI variables, Java `String.format(...)` request paths, and `ApplicationConfig.BASE_PATH + ...` test paths are extracted for endpoint test mapping.
- same-class and unambiguous inherited Java HTTP helper methods are propagated into calling test methods without crossing generic `get/post/put/delete/patch` helper names between classes.
- Java test HTTP paths strip query strings and fragments before endpoint matching.
- endpoint test links from extracted HTTP requests are reported as `MATCHED` instead of `INFERRED`.
- frontend route syntax, same-app callgraph edges, and direct JS/TS test-to-symbol links are reported as `EXTRACTED`/`MATCHED` instead of `INFERRED`.
- name-only cross-app JS/TS callgraph and test-map edges are omitted instead of being kept as weak inferred noise.
- contract gaps now use explicit `MISMATCH`, `PARTIAL_MATCH`, or `UNRESOLVED` confidence labels instead of `WEAK_MATCH`.
- Java callgraph fallback edges with a concrete target type declaration are reported as extracted type matches.
- unresolved workspace contracts include structured owner, resolution, similar-route, and dynamic-endpoint candidate hints for reports and website code maps.
- unresolved workspace contracts include `resolution_class`, `resolution_evidence`, `missing_route_kind`, and equivalent route candidates where derivable.
- Java endpoint test maps classify matched tests by success/auth/permission/validation/not-found/error cases and expected status families when derivable from test names.
- workspace feature flows include backend request kind/type, consumes, return type, stable IDs, and test case/status details.
- workspace feature flows include backend DTO request/response fields, frontend response field usage risks, auth context, and repository/entity/table persistence path data where deterministically extracted.
- `.goregraph-workspace/feature-dossiers.json` and `.md` summarize route, UI, API, backend, tests, DTO, auth, persistence, and risks for website/code-map consumption.
- `goregraph workspace diff --before <dir> --after <dir>` compares workspace output directories and reports contract changes plus lost matched test coverage.
- `docs/OUTPUTS.md` documents the additive output contract and confidence semantics.
- workspace contract matches and feature flows include stable IDs for future diff and deep-link use.
- `goregraph version` reports `0.8.9`.

`v0.9.0` feature checks:

- `.goregraph-workspace/workspace-graph.json` is generated from projects, contracts, routes, flows, and feature dossiers.
- `.goregraph-workspace/workspace-service-map.json` provides directed service/project relationships with incoming and outgoing API evidence.
- `.goregraph-workspace/workspace-endpoint-traces.json` provides readable endpoint traces from frontend consumer to backend handler and related steps.
- `.goregraph-workspace/workspace-map.html` provides an offline dashboard with Open Issues, Architecture Map, Focused Service, Endpoint Trace, and Endpoint Paths views.
- Open Issues is the default dashboard view and groups unresolved, mismatched, dynamic, and out-of-scope contracts by cause, including repeated `/tree/...` frontend prefix/gateway candidates.
- Endpoint Paths lists a selected service as caller -> endpoint/relation -> provider/next hop and replaces the previous low-level raw node cloud.
- The dashboard keeps scanned frontend projects visible even when no outgoing API contracts were detected, supports graph-node selection in the canvas and sidebar, supports deselection via repeated node click, Escape, or Clear selection, avoids clearing selection on accidental canvas clicks, prevents accidental page text selection while dragging, includes generated source file/line links, and explains status terms such as `RESOLVED`, `MISMATCH`, `UNRESOLVED`, `OUT_OF_SCOPE`, `EXTRACTED`, and `MATCHED`.
- `api-contracts.json` now detects project-local `weka.request(method, path, ...)` frontend clients and maps RDBV-style `tree`, `downloads`, and `regulations` paths to the expected service candidates.
- `service-dependencies.json` records Java backend service-client dependencies, and `workspace-service-map.json` merges those backend-to-backend relationships with frontend API contract relationships.
- Architecture Map groups services into generic frontend, document, cadaster/regulation, identity/commerce, and platform/internal domains from scan metadata instead of hardcoding one workspace layout.
- `goregraph workspace refresh` rebuilds workspace overlays from existing project outputs without scanning source files.
- `goregraph workspace dashboard` prints the generated dashboard path.
- `goregraph workspace explain <target>` explains a route, file, symbol, contract, or feature from generated outputs.
- `goregraph workspace path --from <target> --to <target>` reports the shortest graph path between two workspace targets.
- `goregraph workspace impact --changed-file <path>` reports affected feature dossiers for explicit changed files.
- Rust, Kotlin, Scala, Swift, Ruby, C, C++, and C# files are detected in the language inventory with best-effort symbol and import extraction.
- `goregraph version` reports `0.9.0`.

`v0.9.1` feature checks:

- Architecture is the default top-level view and shows service relationships across the full workspace map.
- Selecting a service highlights its direct incoming and outgoing relationships without auto-centering, relayout, or filtering unrelated services.
- **Isolate neighborhood** explicitly limits Architecture to the selected service and its direct neighbors; **Show full architecture** restores the full map.
- Endpoints lets users search for and select a service, inspect its caller -> endpoint -> provider rows, and open an implementation trace; returning from a trace restores the service inventory viewport.
- Diagnostics replaces Open Issues as the top-level name and explains what GoreGraph could not safely confirm, why it matters, the available evidence, and what to check next.
- **100%** resets zoom and pan only. **Fit** fits currently visible content without clearing search or selection, and each top-level view retains its own viewport.
- Static-analysis results remain evidence, not runtime proof. Missing relationships can reflect dynamic behavior, generated code, runtime configuration, unsupported analysis, stale output, or projects that were not indexed.
- Fresh-scan acceptance previews `goregraph workspace clean .`, reviews the listed generated paths, executes `goregraph workspace clean . --execute`, and then runs `goregraph workspace scan-all .` with the newly installed binary. Existing refreshed outputs are not accepted as a substitute.
- `.goregraph-workspace/workspace-map.html` remains compatible with Schema 1 payloads.
- `goregraph version` reports `0.9.1`.

Remaining release-hardening items:

- Validate GoreGraph against more real-world projects before considering `1.0.0`.
- Verify after tagging that the Scoop bucket was updated automatically.
- Open the Winget PR manually when a new manifest branch is generated.
- Wait for Microsoft acceptance before documenting Winget as an active install path.
- Decide later whether macOS notarization or Windows code signing is worth the operational cost.

## Pre-Release Checks

Run locally before tagging:

```bash
gofmt -w .
go vet ./...
go test ./...
go build -o /tmp/goregraph ./cmd/goregraph
/tmp/goregraph version
```

Expected version output shape:

```text
goregraph 0.9.1
commit: <commit>
built: <timestamp>
go: <go-version>
platform: <os>/<arch>
schema: 1
```

## Release Flow

1. Confirm `main` is clean and pushed.
2. Confirm README installation instructions are current.
3. Confirm `CHANGELOG` content through GitHub release notes or generated GoReleaser changelog.
4. Create an annotated release tag:

   ```bash
   git tag -a v0.9.0 -m "Release v0.9.0"
   git push origin v0.9.0
   ```

5. GitHub Actions runs GoReleaser.
6. GoReleaser builds:
   - macOS arm64
   - macOS amd64
   - Linux arm64
   - Linux amd64
   - Windows amd64
7. GoReleaser uploads release archives and `checksums.txt`.
8. GoReleaser updates the Homebrew tap.
9. GoReleaser updates the Scoop bucket when `SCOOP_BUCKET_TOKEN` is present.
10. GoReleaser pushes Winget manifests to the configured fork when `WINGET_TOKEN` is present.
11. Verify a downloaded binary:

   ```bash
   goregraph version
   ```

The release workflow pins GoReleaser to `v2.15.2` because GoreGraph uses a Homebrew Formula through `brews` for the desired `brew install gorecodecom/tap/goregraph` command. Newer GoReleaser versions mark `brews` as deprecated in favor of casks, but casks would change the install model.

## Homebrew

Tap repository:

```text
gorecodecom/homebrew-tap
```

Expected install command after public release:

```bash
brew install gorecodecom/tap/goregraph
```

The tap and GoreGraph release artifacts are public.

After tapping, users can also run:

```bash
brew install goregraph
```

## Winget

Stable package identity:

```text
GoreCode.GoreGraph
```

Expected command after Microsoft accepts the package:

```powershell
winget install --id GoreCode.GoreGraph -e
```

GoReleaser is configured to generate Winget manifests and push them to the configured fork. The package is not live until the manifest is accepted in `microsoft/winget-pkgs`.

`v0.1.1` status:

- GoReleaser generated the Winget manifests.
- GoReleaser pushed branch `goregraph-0.1.1` to `gorecodecom/winget-pkgs`.
- Automatic PR creation failed with GitHub `403 Resource not accessible by personal access token`.
- The PR was opened manually: `https://github.com/microsoft/winget-pkgs/pull/397959`.
- The PR is waiting on Microsoft CLA/review checks.

Current release behavior:

- GoReleaser generates the Winget manifests.
- GoReleaser pushes a `goregraph-<version>` branch to `gorecodecom/winget-pkgs`.
- The PR to `microsoft/winget-pkgs` is opened manually.

For future fully automatic Winget PR creation, `WINGET_TOKEN` must be able to create pull requests against public upstream repositories. A classic GitHub PAT with `public_repo` is the simplest known-good option. If a fine-grained token cannot create the upstream PR, keep the generated branch and open the PR manually.

## Scoop

Bucket repository:

```text
gorecodecom/scoop-bucket
```

Current Windows install command:

```powershell
scoop bucket add gorecode https://github.com/gorecodecom/scoop-bucket
scoop install goregraph
```

Releases update the bucket automatically when `SCOOP_BUCKET_TOKEN` is present.

## Out Of Scope

- macOS notarization
- Windows paid code signing certificate
- automatic public release before the project is stable
