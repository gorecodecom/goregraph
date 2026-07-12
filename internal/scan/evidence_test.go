package scan

import (
	"strings"
	"testing"
)

func TestStableEvidenceIDIsDeterministicAndRootRelative(t *testing.T) {
	record := EvidenceRecord{
		Project:    "app",
		File:       "src/users.go",
		Start:      EvidenceLocation{Line: 7, Column: 3},
		End:        EvidenceLocation{Line: 7, Column: 18},
		Analyzer:   "go",
		Adapter:    "net/http",
		Method:     "syntax",
		Reason:     "call expression",
		SourceHash: "abc123",
	}
	first := StableEvidenceID(record)
	second := StableEvidenceID(record)
	if first != second {
		t.Fatalf("stable evidence ID changed: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "evidence:") {
		t.Fatalf("stable evidence ID %q has no evidence prefix", first)
	}
	if strings.Contains(first, "/Users/") {
		t.Fatalf("stable evidence ID leaks an absolute root: %q", first)
	}
}

func TestEvidenceStatusDimensionsValidateIndependently(t *testing.T) {
	valid := []interface{ Validate() error }{
		ConfidenceExact,
		ConfidenceResolved,
		ResolutionMatched,
		ResolutionOutOfScope,
		SeverityWarning,
		CoveragePartial,
	}
	for _, value := range valid {
		if err := value.Validate(); err != nil {
			t.Fatalf("valid value rejected: %v", err)
		}
	}
	invalid := []interface{ Validate() error }{
		Confidence("MATCHED"),
		Resolution("EXACT"),
		Severity("CRITICAL"),
		Coverage("MISSING"),
	}
	for _, value := range invalid {
		if err := value.Validate(); err == nil {
			t.Fatal("invalid status value accepted")
		}
	}
}
