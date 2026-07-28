package agentguide

import (
	"strings"
	"testing"
)

func TestAssistedInstructionDefinesExecutableBoundedWorkflow(t *testing.T) {
	lines := strings.Split(AssistedInstruction, "\n")
	if AssistedInstructionLineCount != 11 {
		t.Fatalf("instruction line-count contract = %d, want 11", AssistedInstructionLineCount)
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
