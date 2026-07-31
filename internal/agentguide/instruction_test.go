package agentguide

import (
	"strings"
	"testing"
)

func TestAssistedInstructionDefinesExecutableBoundedWorkflow(t *testing.T) {
	lines := strings.Split(AssistedInstruction, "\n")
	if AssistedInstructionLineCount != 12 {
		t.Fatalf("instruction line-count contract = %d, want 12", AssistedInstructionLineCount)
	}
	if len(lines) != AssistedInstructionLineCount {
		t.Fatalf("instruction line count = %d, want %d", len(lines), AssistedInstructionLineCount)
	}
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			t.Fatalf("instruction line %d is blank", index+1)
		}
	}
	if !strings.Contains(lines[0], `goregraph context . --query "<focused query>"`) {
		t.Fatalf("first instruction line is not executable: %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "specialist GoreGraph queries or expert MCP tools") {
		t.Fatalf("last instruction line does not close fallback tools: %q", lines[len(lines)-1])
	}
}

func TestAssistedInstructionKeepsFutureChangePlansSourceBacked(t *testing.T) {
	for _, want := range []string{
		"For change plans",
		"exact existing production and test paths",
		"Context Pack or bounded omission reads",
		"do not invent future filenames",
		"future route, authentication, status, lookup implementation, and cross-service transaction ordering",
		"unknown design decisions unless rendered source proves them",
	} {
		if !strings.Contains(AssistedInstruction, want) {
			t.Errorf("AssistedInstruction does not contain %q", want)
		}
	}
}
