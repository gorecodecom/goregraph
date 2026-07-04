package goregraph_test

import (
	"os"
	"strings"
	"testing"
)

func TestMilestone6ReleaseFilesAreConfigured(t *testing.T) {
	files := map[string][]string{
		".goreleaser.yaml": {
			"version: 2",
			"main: ./cmd/goregraph",
			"darwin",
			"linux",
			"windows",
			"arm64",
			"amd64",
			"checksums.txt",
			"brews:",
			"gorecodecom",
			"homebrew-tap",
			"winget:",
			"GoreCode.GoreGraph",
			"gorecodecom",
			"winget-pkgs",
			"pull_request:",
			"scoops:",
			"scoop-bucket",
			"internal/version.Version",
			"internal/version.Commit",
			"internal/version.Built",
		},
		".github/workflows/ci.yml": {
			"go test ./...",
			"go vet ./...",
			"gofmt",
		},
		".github/workflows/release.yml": {
			"goreleaser/goreleaser-action",
			"v2.15.2",
			"tags:",
			"GITHUB_TOKEN",
			"HOMEBREW_TAP_TOKEN",
			"SCOOP_BUCKET_TOKEN",
			"WINGET_TOKEN",
		},
		"docs/RELEASE.md": {
			"v0.1.0",
			"goregraph version",
			"GoReleaser",
			"checksums.txt",
			"brew install gorecodecom/tap/goregraph",
			"winget install --id GoreCode.GoreGraph -e",
		},
	}

	for path, required := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s is missing: %v", path, err)
		}
		text := string(body)
		for _, want := range required {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}
