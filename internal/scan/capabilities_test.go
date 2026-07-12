package scan

import "testing"

func TestBuildCapabilityInventoryReportsHonestCoverage(t *testing.T) {
	records := BuildCapabilityInventory([]FileRecord{
		{Path: "src/app.tsx", Language: "typescript"},
		{Path: "src/main.rs", Language: "rust"},
		{Path: "src/tool.kt", Language: "kotlin"},
	}, WorkspaceIndex{})

	assertCapabilityCoverage(t, records, "typescript", CapabilitySymbols, CoverageComplete)
	assertCapabilityCoverage(t, records, "typescript", CapabilityPersistence, CoverageComplete)
	assertCapabilityCoverage(t, records, "rust", CapabilityRoutes, CoverageUnavailable)
	assertCapabilityCoverage(t, records, "kotlin", CapabilityCalls, CoverageUnavailable)
}

func assertCapabilityCoverage(t *testing.T, records []CapabilityRecord, language string, capability CapabilityID, want Coverage) {
	t.Helper()
	for _, record := range records {
		if record.Language == language && record.ID == capability {
			if record.Coverage != want {
				t.Fatalf("%s/%s coverage = %s, want %s", language, capability, record.Coverage, want)
			}
			if record.Reason == "" {
				t.Fatalf("%s/%s has no coverage reason", language, capability)
			}
			return
		}
	}
	t.Fatalf("missing %s/%s capability", language, capability)
}
