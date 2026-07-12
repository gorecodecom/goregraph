package scan

import "sort"

type CapabilityID string

const (
	CapabilitySymbols     CapabilityID = "symbols"
	CapabilityRelations   CapabilityID = "relations"
	CapabilityCalls       CapabilityID = "calls"
	CapabilityRoutes      CapabilityID = "routes"
	CapabilityAPIClients  CapabilityID = "api_clients"
	CapabilityTests       CapabilityID = "tests"
	CapabilityPersistence CapabilityID = "persistence"
	CapabilityMessaging   CapabilityID = "messaging"
	CapabilityDataFlow    CapabilityID = "data_flow"
)

var capabilityOrder = []CapabilityID{
	CapabilitySymbols,
	CapabilityRelations,
	CapabilityCalls,
	CapabilityRoutes,
	CapabilityAPIClients,
	CapabilityTests,
	CapabilityPersistence,
	CapabilityMessaging,
	CapabilityDataFlow,
}

type CapabilityRecord struct {
	ID          CapabilityID `json:"id"`
	Project     string       `json:"project,omitempty"`
	Language    string       `json:"language"`
	Adapter     string       `json:"adapter,omitempty"`
	Coverage    Coverage     `json:"coverage"`
	Reason      string       `json:"reason"`
	FilesSeen   int          `json:"files_seen"`
	EvidenceIDs []string     `json:"evidence_ids,omitempty"`
	Failure     string       `json:"failure,omitempty"`
}

type CoverageRecord struct {
	Capabilities []CapabilityRecord `json:"capabilities"`
	FilesSeen    int                `json:"files_seen"`
}

func BuildCapabilityInventory(files []FileRecord, workspace WorkspaceIndex) []CapabilityRecord {
	fileCounts := map[string]int{}
	for _, file := range files {
		if file.Language != "" && file.Language != "unknown" {
			fileCounts[file.Language]++
		}
	}
	if len(workspace.MavenPackages) > 0 {
		fileCounts["maven"] = len(workspace.MavenPackages)
	}
	if len(workspace.NodePackages) > 0 {
		fileCounts["node"] = len(workspace.NodePackages)
	}
	legacy := analyzerCapabilities()
	records := make([]CapabilityRecord, 0, len(fileCounts)*len(capabilityOrder))
	for language, count := range fileCounts {
		analyzer, known := legacy[language]
		for _, capability := range capabilityOrder {
			coverage, reason := capabilityCoverage(analyzer, known, capability)
			records = append(records, CapabilityRecord{
				ID:        capability,
				Language:  language,
				Adapter:   analyzer.Scope,
				Coverage:  coverage,
				Reason:    reason,
				FilesSeen: count,
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Language != records[j].Language {
			return records[i].Language < records[j].Language
		}
		return capabilityIndex(records[i].ID) < capabilityIndex(records[j].ID)
	})
	return records
}

func BuildCoverage(files []FileRecord, capabilities []CapabilityRecord) CoverageRecord {
	return CoverageRecord{Capabilities: capabilities, FilesSeen: len(files)}
}

func capabilityCoverage(analyzer AnalyzerRecord, known bool, capability CapabilityID) (Coverage, string) {
	if !known {
		if capability == CapabilitySymbols {
			return CoveragePartial, "Generic indexing provides best-effort symbols without a full language adapter."
		}
		return CoverageUnavailable, "No analyzer capability is registered for this language."
	}
	complete := map[CapabilityID]bool{
		CapabilitySymbols:   analyzer.Symbols,
		CapabilityRelations: analyzer.Relations,
		CapabilityCalls:     analyzer.Calls,
		CapabilityRoutes:    analyzer.Endpoints,
		CapabilityTests:     analyzer.Tests,
	}
	if complete[capability] {
		return CoverageComplete, "The active analyzer emits this capability for indexed files."
	}
	if capability == CapabilityAPIClients && (analyzer.Language == "javascript" || analyzer.Language == "typescript") {
		return CoveragePartial, "Supported client patterns are extracted; configurable and dynamic wrappers may remain unresolved."
	}
	if capability == CapabilityPersistence && (analyzer.Language == "java" || analyzer.Language == "javascript" || analyzer.Language == "typescript" || analyzer.Language == "go" || analyzer.Language == "php") {
		return CoveragePartial, "Some persistence relationships are visible, but framework parity is not complete."
	}
	if capability == CapabilityDataFlow && (analyzer.Calls || analyzer.Endpoints) {
		return CoveragePartial, "Call and route steps exist, but field-level data flow is not yet complete."
	}
	return CoverageUnavailable, "The active analyzer does not emit this capability yet."
}

func capabilityIndex(id CapabilityID) int {
	for index, candidate := range capabilityOrder {
		if candidate == id {
			return index
		}
	}
	return len(capabilityOrder)
}
