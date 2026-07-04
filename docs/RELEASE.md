# GoreGraph Release Checklist

## Current Target

First public release target:

```text
v0.1.0
```

`1.0.0` is reserved for a stable public CLI and schema contract.

## Required Secrets

GitHub repository secrets:

- `HOMEBREW_TAP_TOKEN`: token with write access to `gorecodecom/homebrew-tap`.
- `WINGET_TOKEN`: optional token for future Winget publishing. Milestone 6 keeps Winget upload disabled.

`GITHUB_TOKEN` is provided by GitHub Actions for publishing the GoreGraph release.

## Release Ready Open Items

Milestone 6 is complete, but GoreGraph is not public-release-ready until these items are done:

- Add the `HOMEBREW_TAP_TOKEN` repository secret with write access to `gorecodecom/homebrew-tap`.
- Decide whether `v0.1.0` is internal/private validation only or a public release.
- For a public release, make `gorecodecom/goregraph` public or ensure release artifacts are publicly downloadable.
- For a public Homebrew install, make `gorecodecom/homebrew-tap` public.
- Create and push the first release tag only after final local validation.
- Verify the GitHub release workflow finishes successfully.
- Verify release artifacts and `checksums.txt` are present.
- Verify the Homebrew Formula is generated in `gorecodecom/homebrew-tap`.
- Test a downloaded release binary with `goregraph version`.
- Test `brew install gorecodecom/tap/goregraph` after the tap and artifacts are publicly reachable.

Recommended final validation before public release:

- run GoreGraph against two or three real projects
- run `goregraph doctor` after each scan
- inspect generated `report.md`, `entrypoints.md`, and `test-map.md`
- review README and `COMMANDS.md` from a first-time user perspective

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
goregraph 0.1.0
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
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
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
9. Verify a downloaded binary:

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

The tap may stay private during internal testing. It must be public before external users can install without GitHub authentication.

## Winget

Stable package identity:

```text
GoreCode.GoreGraph
```

Expected future install command:

```powershell
winget install --id GoreCode.GoreGraph -e
```

Milestone 6 prepares Winget metadata but does not upload to `winget-pkgs` automatically.

## Out Of Scope

- macOS notarization
- Windows paid code signing certificate
- automatic public release before the project is stable
