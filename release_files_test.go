package goregraph_test

import (
	"bytes"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"text/template"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestOptionalPublisherTemplatesHandleMissingSecrets(t *testing.T) {
	body, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")

	skipTemplatePattern := regexp.MustCompile(`(?m)^    skip_upload: ['"](.*)['"]$`)
	matches := skipTemplatePattern.FindAllStringSubmatch(text, -1)
	if len(matches) != 2 {
		t.Fatalf("found %d optional publisher templates, want 2", len(matches))
	}

	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{name: "no publisher secrets", env: map[string]string{}, want: []string{"true", "true"}},
		{name: "winget only", env: map[string]string{"WINGET_TOKEN": "token"}, want: []string{"false", "true"}},
		{name: "scoop only", env: map[string]string{"SCOOP_BUCKET_TOKEN": "token"}, want: []string{"false", "true"}},
		{
			name: "both publisher secrets",
			env: map[string]string{
				"WINGET_TOKEN":       "token",
				"SCOOP_BUCKET_TOKEN": "token",
			},
			want: []string{"false", "false"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for index, match := range matches {
				skipTemplate, err := template.New("skip_upload").Option("missingkey=error").Parse(match[1])
				if err != nil {
					t.Fatalf("parse skip template %d: %v", index, err)
				}
				var output bytes.Buffer
				if err := skipTemplate.Execute(&output, map[string]any{"Env": test.env}); err != nil {
					t.Fatalf("execute skip template %d: %v", index, err)
				}
				got = append(got, output.String())
			}
			sort.Strings(got)
			if !slices.Equal(got, test.want) {
				t.Fatalf("skip decisions = %v, want %v", got, test.want)
			}
		})
	}
}

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
			"go fmt ./...",
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

func TestReleaseSourcesExcludeHistoricalDevelopmentArtifacts(t *testing.T) {
	historicalPaths := []string{
		".superpowers/sdd/issue25-task-5-report.md",
		".superpowers/sdd/issue25-task-7-report.md",
		"docs/superpowers/plans",
		"docs/superpowers/specs",
		"AI_INTEGRATION_PLAN.md",
		"BUILD_PLAN.md",
		"DISTRIBUTION_PLAN.md",
		"TOKEN_EFFICIENCY_PLAN.md",
		"architecture-current-snapshot.md",
		"design-qa.md",
	}
	for _, path := range historicalPaths {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("historical development artifact remains: %s", path)
		} else if !os.IsNotExist(err) {
			t.Errorf("inspect historical development artifact %s: %v", path, err)
		}
	}

	publicMarkdown := []string{
		"README.md",
		"COMMANDS.md",
		"ROADMAP.md",
		"SCHEMA.md",
		"docs/BENCHMARKING.md",
		"docs/OUTPUTS.md",
		"docs/RELEASE.md",
		"docs/design-system.md",
	}
	forbidden := []string{
		"/Users/gorecode/",
		"C:/Users/goretzkh/",
		"goregraph-benchmark/0442483-pre-fix",
		"rdbv",
		"weka",
	}
	for _, path := range publicMarkdown {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read retained public Markdown %s: %v", path, err)
			continue
		}
		text := strings.ToLower(string(body))
		for _, value := range forbidden {
			if strings.Contains(text, strings.ToLower(value)) {
				t.Errorf("retained public Markdown %s exposes forbidden value %q", path, value)
			}
		}
	}
}

func TestReleaseDocumentationDefinesSourceBackedContextContract(t *testing.T) {
	body, err := os.ReadFile("docs/RELEASE.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	for _, want := range []string{
		"source sections replace reads of included ranges",
		"`source_coverage` is authoritative",
		"source_unrepresented",
		"Both raw and effective counters are retained.",
		"`effective_tokens` is\n`input_tokens - cached_input_tokens + output_tokens`, or uncached input plus\noutput",
		"offline, explicit, dependency-free, and watcher-free",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release notes missing source-backed Context contract %q", want)
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
