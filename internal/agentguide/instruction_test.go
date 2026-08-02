package agentguide

import (
	"strings"
	"testing"
)

func TestAssistedInstructionDefinesExecutableBoundedWorkflow(t *testing.T) {
	lines := strings.Split(AssistedInstruction, "\n")
	if AssistedInstructionLineCount != 13 {
		t.Fatalf("instruction line-count contract = %d, want 13", AssistedInstructionLineCount)
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
		"separate exact existing production-file and test-file inventories",
		"files, source_sections, production_plan_files, plan_files, or bounded omission reads",
		"name every supplied production_plan_files identity in the production-file inventory with its role",
		"name every supplied plan_files identity in the test-file inventory with its use",
		"naming metadata is not reading source",
		"provider_test entries may be test targets",
		"mock_pattern or retry_pattern entries are reference patterns, not change targets",
		"source_omissions lists the same exact path with a bounded range",
		"do not invent future filenames",
		"future route, authentication, status, lookup implementation",
		"dependent persistence and cascade behavior",
		"cross-service transaction ordering",
		"unknown design decisions unless rendered source proves them",
	} {
		if !strings.Contains(AssistedInstruction, want) {
			t.Errorf("AssistedInstruction does not contain %q", want)
		}
	}
}

func TestAssistedInstructionRequiresReaderBoundedOmissionCommands(t *testing.T) {
	for _, want := range []string{
		"make the file reader itself range-bounded",
		"sed -n",
		"never pipe a whole-file reader such as nl",
		"downstream range filter",
	} {
		if !strings.Contains(AssistedInstruction, want) {
			t.Errorf("AssistedInstruction does not contain %q", want)
		}
	}
}

func TestAssistedInstructionKeepsRequestedAuthenticationAndConfigurationCoherent(t *testing.T) {
	for _, want := range []string{
		"When authentication or configuration is requested",
		"server authorization policy",
		"client authentication construction and configuration fields",
		"name every supplied configuration_resources identity",
		"project, profile, and key groups",
		"exact paths of supplied production and test-profile resources",
		"one coherent answer section",
	} {
		if !strings.Contains(AssistedInstruction, want) {
			t.Errorf("AssistedInstruction does not contain %q", want)
		}
	}
}
