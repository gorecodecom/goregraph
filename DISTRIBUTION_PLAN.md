# GoreGraph Distribution Plan

## Purpose

This document tracks packaging and installation options for GoreGraph. Release automation is active for the public `v0.1.0` release. The CLI and schema are still pre-1.0, so distribution should remain conservative.

## Release Artifacts

GoreGraph should publish native binaries for:

- macOS arm64
- macOS amd64
- Linux amd64
- Linux arm64
- Windows amd64

Release archive examples:

```text
goregraph_Darwin_arm64.tar.gz
goregraph_Darwin_x86_64.tar.gz
goregraph_Linux_x86_64.tar.gz
goregraph_Linux_arm64.tar.gz
goregraph_Windows_x86_64.zip
```

Each release should include checksums.

Current release automation:

- `.goreleaser.yaml`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `docs/RELEASE.md`

## GitHub Releases

GitHub Releases should be the first public distribution mechanism.

Recommended automation:

- GoReleaser
- GitHub Actions

Release flow:

```text
1. Tag version, e.g. v0.1.0
2. GitHub Actions builds binaries
3. GoReleaser uploads artifacts
4. Checksums are generated
5. Package manager metadata can be updated
```

## Homebrew

Homebrew installation uses the shared GoreCode tap repository:

```text
github.com/gorecodecom/homebrew-tap
```

Formula path:

```text
Formula/goregraph.rb
```

Expected install commands:

```bash
brew tap gorecodecom/tap
brew install goregraph
```

or:

```bash
brew install gorecodecom/tap/goregraph
```

Formula shape:

```ruby
class Goregraph < Formula
  desc "Local deterministic code intelligence for safer AI-assisted development"
  homepage "https://github.com/gorecodecom/goregraph"
  url "https://github.com/gorecodecom/goregraph/releases/download/v0.1.0/goregraph_Darwin_arm64.tar.gz"
  sha256 "..."
  license "Apache-2.0"

  def install
    bin.install "goregraph"
  end

  test do
    system "#{bin}/goregraph", "version"
  end
end
```

GoReleaser updates the Homebrew tap automatically during release. The `v0.1.0` Formula has been published and verified with:

```bash
brew audit --formula --strict gorecodecom/tap/goregraph
brew install gorecodecom/tap/goregraph
brew test gorecodecom/tap/goregraph
```

## Winget

Windows Package Manager metadata is prepared for the stable package identifier.

Target command after Microsoft accepts the package:

```powershell
winget install --id GoreCode.GoreGraph -e
```

Requirements:

- Windows release artifact
- stable publisher metadata
- winget manifest
- versioned release URL
- installer or portable ZIP strategy
- `WINGET_TOKEN` with permission to push to the configured `gorecodecom/winget-pkgs` fork and open a PR against `microsoft/winget-pkgs`

GoReleaser is configured for the standard PR-based `winget-pkgs` flow:

- source fork: `gorecodecom/winget-pkgs`
- target repository: `microsoft/winget-pkgs`
- target branch: `master`
- branch template: `goregraph-{{ .Version }}`

Publishing is skipped automatically when `WINGET_TOKEN` is not present. The package becomes installable only after the PR is accepted by Microsoft.

## Scoop

Scoop is active for Windows installation through the GoreCode bucket.

Expected command:

```powershell
scoop bucket add gorecode https://github.com/gorecodecom/scoop-bucket
scoop install goregraph
```

Bucket:

```text
github.com/gorecodecom/scoop-bucket
```

Current manifest:

```text
bucket/goregraph.json
```

The `v0.1.0` manifest has been published manually. Future release automation is configured through GoReleaser and requires `SCOOP_BUCKET_TOKEN` with write access to `gorecodecom/scoop-bucket`.

## Install Script

An install script may be useful later:

```bash
curl -fsSL https://gorecode.com/goregraph/install.sh | sh
```

Rules:

- Must be transparent and auditable.
- Must verify checksums.
- Must install only the GoreGraph binary.
- Must not modify shell profiles unless explicitly requested.

## Development Install

During development:

```bash
go build -o goregraph ./cmd/goregraph
```

or:

```bash
go install ./cmd/goregraph
```

The README should document local development builds separately from official package installation.

## Public Release State

The repository is public as of `v0.1.0`.

The first public package release was made after:

- command surface is stable
- README is complete
- release automation is tested
- security model is documented
- public GitHub release artifacts are available
- `gorecodecom/homebrew-tap` is public

Future hardening should focus on broader project validation, Winget approval, and optional signing decisions before `1.0.0`.
