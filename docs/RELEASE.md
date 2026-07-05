# GoreGraph Release Checklist

## Current Release

Current release target:

```text
v0.2.1
```

`1.0.0` is reserved for a stable public CLI and schema contract.

## Required Secrets

GitHub repository secrets:

- `HOMEBREW_TAP_TOKEN`: token with write access to `gorecodecom/homebrew-tap`.
- `SCOOP_BUCKET_TOKEN`: token with write access to `gorecodecom/scoop-bucket`.
- `WINGET_TOKEN`: optional token for pushing Winget manifests to the configured `gorecodecom/winget-pkgs` fork.

`GITHUB_TOKEN` is provided by GitHub Actions for publishing the GoreGraph release.

## Public Release Status

`v0.1.0` has been released publicly. `v0.1.1` validated the package-manager release flow for Homebrew, Scoop, and manual Winget PR publishing. `v0.2.0` adds the universal safe code graph outputs and Java/Spring deep analysis. `v0.2.1` keeps those features and hardens the release workflow so Winget PR submission no longer turns otherwise successful releases red.

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
goregraph 0.2.1
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
   git tag -a v0.2.1 -m "Release v0.2.1"
   git push origin v0.2.1
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
