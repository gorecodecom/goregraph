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
