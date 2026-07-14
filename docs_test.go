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
		"goregraph report",
		"goregraph query",
		"goregraph explain",
		"goregraph doctor",
		"goregraph mcp",
		"goregraph version",
	} {
		if !strings.Contains(text, command) {
			t.Fatalf("COMMANDS.md missing %q", command)
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
