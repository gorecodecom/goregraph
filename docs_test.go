package goregraph_test

import (
	"os"
	"strings"
	"testing"
)

func TestCommandsReferenceDocumentsEveryUserCommand(t *testing.T) {
	body, err := os.ReadFile("COMMANDS.md")
	if err != nil {
		t.Fatalf("COMMANDS.md is missing: %v", err)
	}
	text := string(body)
	for _, command := range []string{
		"goregraph help",
		"goregraph scan",
		"goregraph update",
		"goregraph git update",
		"goregraph report",
		"goregraph query",
		"goregraph explain",
		"goregraph doctor",
		"goregraph workspace git update",
		"goregraph mcp",
		"goregraph version",
	} {
		if !strings.Contains(text, command) {
			t.Fatalf("COMMANDS.md missing %q", command)
		}
	}
}

func TestArchitectureMapDocumentationMatchesIssue23(t *testing.T) {
	files := []string{"README.md", "COMMANDS.md", "docs/OUTPUTS.md", "docs/RELEASE.md", "docs/design-system.md"}
	var combined strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(content)
		combined.WriteByte('\n')
	}
	text := strings.ToLower(combined.String())
	for _, want := range []string{"dynamic domain lanes", "incoming and outgoing", "statically detected", "not runtime", "1.3.0"} {
		if !strings.Contains(text, strings.ToLower(want)) {
			t.Fatalf("architecture documentation missing %q", want)
		}
	}
}

func TestDashboardDesignSystemDocumentsRequiredTokensAndBehavior(t *testing.T) {
	content, err := os.ReadFile("docs/design-system.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"--color-background",
		"--color-surface",
		"--color-text",
		"--color-muted",
		"--color-accent",
		"--color-focus",
		"--space-1",
		"--radius-control",
		"prefers-reduced-motion",
		"Selection does not relayout the Architecture view",
		"Fit preserves search, filters, and selection",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("design system missing %q", want)
		}
	}
}

func TestDashboardDocumentationCoversSixDistinctViews(t *testing.T) {
	content, err := os.ReadFile("docs/OUTPUTS.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{"Architecture", "Endpoints", "Feature Flow", "Data Flow", "Diagnostics", "Coverage", "100% scale", "prioritized next scans"} {
		if !strings.Contains(text, want) {
			t.Fatalf("dashboard documentation missing %q", want)
		}
	}
}

func TestDocumentationCoversExactCodeExplorer(t *testing.T) {
	files := []string{"README.md", "COMMANDS.md", "docs/OUTPUTS.md", "docs/RELEASE.md"}
	var combined strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(content)
		combined.WriteByte('\n')
	}
	text := combined.String()
	for _, want := range []string{
		"Explore classes & symbols",
		"Direct references",
		"Reached through API",
		"symbol-index.json",
		"symbol-usages.json",
		"symbol-inventory",
		"symbol-resolve",
		"symbol-usages",
		"symbol-api-consumers",
		"symbol-explain",
		"Java / Spring",
		"JavaScript / TypeScript / Node.js / React",
		"Exact symbols",
		"Direct usages",
		"HTTP reachability",
		"canonical symbol ID",
		"direct_reference",
		"reached_through_api",
		"AMBIGUOUS",
		"UNRESOLVED",
		"coverage warnings",
		"API path steps",
		"unreleased 1.3.0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("exact Code Explorer documentation missing %q", want)
		}
	}
}

func TestDocumentationCoversEditableDashboardAndAPIContext(t *testing.T) {
	required := map[string][]string{
		"README.md": {
			"goregraph workspace dashboard edit [path]",
			"goregraph workspace dashboard path .",
			"goregraph workspace dashboard open .",
			"Only `edit` starts an authenticated loopback server",
			".goregraph-dashboard.json",
			"API Catalog",
			"Endpoint security",
			"consumer call authentication",
			"No auth evidence detected",
			"index/api-catalog.json",
			"agent/context-index.json",
			"1800",
			"unreleased 1.3.0",
		},
		"COMMANDS.md": {
			"goregraph workspace dashboard edit [path]",
			"goregraph workspace dashboard path .",
			"goregraph workspace dashboard open .",
			"only `edit` starts an authenticated loopback server",
			".goregraph-dashboard.json",
			"API Catalog",
			"Endpoint security",
			"consumer call authentication",
			"No auth evidence detected",
			"index/api-catalog.json",
			"agent/context-index.json",
			"1800",
		},
		"docs/OUTPUTS.md": {
			"goregraph workspace dashboard edit .",
			".goregraph-dashboard.json",
			"API Catalog",
			"No auth evidence detected",
			"index/api-catalog.json",
			"agent/context-index.json",
		},
		"SCHEMA.md": {
			".goregraph-dashboard.json",
			"Endpoint security",
			"consumer call authentication",
			"No auth evidence detected",
			"index/api-catalog.json",
			"agent/context-index.json",
		},
		"docs/RELEASE.md": {
			"goregraph workspace dashboard edit .",
			".goregraph-dashboard.json",
			"API Catalog",
			"No auth evidence detected",
			"agent/context-index.json",
			"unreleased 1.3.0",
		},
	}
	for file, wants := range required {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing editable dashboard/API contract %q", file, want)
			}
		}
	}
}

func TestREADMESeparatesAPIIntegrationDepthByLanguage(t *testing.T) {
	content, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	start := strings.Index(text, "### API integration depth")
	end := strings.Index(text, "## Output Files")
	if start < 0 || end <= start {
		t.Fatal("README API integration depth table is missing")
	}
	section := text[start:end]
	for _, want := range []string{
		"Endpoint inventory",
		"Consumers",
		"Security/auth",
		"Request/response types",
		"Dashboard",
		"Agent context",
		"| Java / Spring |",
		"| JavaScript / TypeScript / Node.js / React |",
		"Consumer call authentication; provider security unknown",
		"Handler identity; request/response types unknown",
	} {
		if !strings.Contains(section, want) {
			t.Fatalf("README API integration depth table missing %q", want)
		}
	}
}

func TestDashboardDocumentationDoesNotUseStaleViewCount(t *testing.T) {
	for _, file := range []string{"README.md", "COMMANDS.md"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(content)), "seven views") {
			t.Fatalf("%s contains stale seven views wording", file)
		}
	}
}

func TestREADMELaterWorkspaceDashboardReferenceDocumentsEditMode(t *testing.T) {
	content, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	start := strings.LastIndex(text, "```bash\ngoregraph workspace dashboard [path]")
	if start < 0 {
		t.Fatal("README later workspace dashboard reference is missing")
	}
	endOffset := strings.Index(text[start:], "```bash\ngoregraph workspace clean <path>")
	if endOffset < 0 {
		t.Fatal("README later workspace dashboard reference boundary is missing")
	}
	section := text[start : start+endOffset]
	for _, want := range []string{
		"goregraph workspace dashboard edit [path]",
		"static read-only",
		"Only `edit` starts an authenticated loopback editor",
		".goregraph-dashboard.json",
	} {
		if !strings.Contains(section, want) {
			t.Fatalf("README later workspace dashboard reference missing %q:\n%s", want, section)
		}
	}
}
