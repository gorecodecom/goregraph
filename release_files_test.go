package goregraph_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
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
			"v1.3.0",
			"v1.2.0",
			"Git tags, GitHub Releases, Homebrew publication, Scoop publication, and Winget publication all remain pending.",
			"Schema 2",
			"v0.9.4",
			"Architecture",
			"Endpoints",
			"Feature Flow",
			"Diagnostics",
			"impact-summary",
			"v0.1.1",
			"goregraph version",
			"GoReleaser",
			"checksums.txt",
			"brew install gorecodecom/tap/goregraph",
			"winget install --id GoreCode.GoreGraph -e",
		},
		"internal/scan/scan.go": {
			"SchemaVersion = 3",
			"IndexGeneratedFiles",
			"AgentGeneratedFiles",
			"DashboardGeneratedFiles",
		},
		"internal/scan/output_layout.go": {
			"BuildTargetAgent",
			"BuildTargetDashboard",
			"BuildTargetAll",
			"manifest.json",
			"index",
			"agent",
			"dashboard",
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

func TestReleaseFilesKeep130Unreleased(t *testing.T) {
	files := []string{"README.md", "docs/RELEASE.md"}
	var combined strings.Builder
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("%s is missing: %v", file, err)
		}
		combined.Write(body)
		combined.WriteByte('\n')
	}
	text := combined.String()
	for _, want := range []string{
		"unreleased 1.3.0",
		"Git tags, GitHub Releases, Homebrew publication, Scoop publication, and Winget publication all remain pending.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release documentation missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"`v1.3.0` is published",
		"`v1.3.0` has been released",
		"`v1.3.0` tag was created",
		"Homebrew was updated to `v1.3.0`",
		"Scoop was updated to `v1.3.0`",
		"Winget was updated to `v1.3.0`",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("release documentation must not claim publication: %q", forbidden)
		}
	}
}

func TestReleaseNotesDescribeEditableDashboardWithoutPublishing130(t *testing.T) {
	body, err := os.ReadFile("docs/RELEASE.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"unreleased 1.3.0",
		"goregraph workspace dashboard edit .",
		".goregraph-dashboard.json",
		"API Catalog",
		"No auth evidence detected",
		"agent/context-index.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release notes missing %q", want)
		}
	}
}

func TestContextIndexOutputIsPartOfReleaseContract(t *testing.T) {
	if !slices.Contains(scan.AgentGeneratedFiles, "context-index.json") {
		t.Fatal("agent/context-index.json is not registered as agent output")
	}
	if slices.Contains(scan.IndexGeneratedFiles, "context-index.json") {
		t.Fatal("context-index.json must not be registered as canonical index output")
	}
	if slices.Contains(scan.DashboardGeneratedFiles, "context-index.json") {
		t.Fatal("context-index.json must not be registered as dashboard output")
	}
	if !slices.Contains(scan.GeneratedFiles, "agent/context-index.json") {
		t.Fatal("agent/context-index.json is missing from the complete generated-file contract")
	}
}
