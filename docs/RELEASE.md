# GoreGraph Release Checklist

## Current Release

Current release target:

```text
v0.8.4
```

`1.0.0` is reserved for a stable public CLI and schema contract.

## Required Secrets

GitHub repository secrets:

- `HOMEBREW_TAP_TOKEN`: token with write access to `gorecodecom/homebrew-tap`.
- `SCOOP_BUCKET_TOKEN`: token with write access to `gorecodecom/scoop-bucket`.
- `WINGET_TOKEN`: optional token for pushing Winget manifests to the configured `gorecodecom/winget-pkgs` fork.

`GITHUB_TOKEN` is provided by GitHub Actions for publishing the GoreGraph release.

## Public Release Status

`v0.1.0` has been released publicly. `v0.1.1` validated the package-manager release flow for Homebrew, Scoop, and manual Winget PR publishing. `v0.2.0` adds the universal safe code graph outputs and Java/Spring deep analysis. `v0.2.1` keeps those features and hardens the release workflow so Winget PR submission no longer turns otherwise successful releases red. `v0.4.0` adds endpoint hardening, Java/Spring call graph output, endpoint flows, method-aware test mapping, and analyzer inventory. `v0.5.0` adds route, flow, call, test, and navigation intelligence for Go, PHP, JavaScript/TypeScript/React, Python, and Shell. `v0.6.0` adds frontend monorepo hardening, package graphs, API contracts, and lower-noise JS/TS analysis. `v0.7.0` targets realistic frontend API helper extraction, app-aware frontend resolver ranking, and Maven dependency graph output. `v0.8.0` adds frontend API to backend route contract matching, safer URL normalization, and explicit weak/static contract issue reports. `v0.8.1` adds diagnostics, unscanned-service classification, and lower-noise affected output. `v0.8.2` adds zero-config workspace discovery and cross-project overlay refreshes. `v0.8.3` adds workspace query fallback, integrated workspace diagnostics, frontend consumers in endpoints, end-to-end workspace feature flows, stricter path matching, and generated-output gitignore handling. `v0.8.4` is the current local development version for consistent workspace diagnostics, clearer frontend-consumer report scope, and actionable missing-test reasons in feature flows; it is not released until a tag is pushed.

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
goregraph 0.8.4
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
   git tag -a v0.8.4 -m "Release v0.8.4"
   git push origin v0.8.4
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
