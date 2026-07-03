# GoreGraph Distribution Plan

## Purpose

This document tracks future packaging and installation options for GoreGraph. The MVP should be structured so these paths are easy later, but publishing packages is not required for the first implementation.

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

Homebrew installation needs a tap repository:

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
  license "MIT"

  def install
    bin.install "goregraph"
  end

  test do
    system "#{bin}/goregraph", "help"
  end
end
```

GoReleaser can update the Homebrew tap automatically later.

## winget

Windows Package Manager support can come after the CLI is stable.

Expected command:

```powershell
winget install GoreCode.GoreGraph
```

Requirements:

- Windows release artifact
- stable publisher metadata
- winget manifest
- versioned release URL
- installer or portable ZIP strategy

This is more administrative than Homebrew and should not block the MVP.

## Scoop

Scoop can be easier than winget for early Windows distribution.

Expected command:

```powershell
scoop bucket add gorecode https://github.com/gorecodecom/scoop-bucket
scoop install goregraph
```

Potential bucket:

```text
github.com/gorecodecom/scoop-bucket
```

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

## Private Phase

The repository can stay private while GoreGraph is validated.

Even with MIT licensing in the repository, public package distribution should wait until:

- command surface is stable
- README is complete
- release automation is tested
- licensing decision is re-reviewed
- security model is documented
