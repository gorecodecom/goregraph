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
	} {
		if !strings.Contains(text, command) {
			t.Fatalf("COMMANDS.md missing %q", command)
		}
	}
}
