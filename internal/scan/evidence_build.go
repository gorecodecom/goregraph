package scan

import "sort"

func BuildEvidence(project string, files []FileRecord, relations []RichRelationRecord, calls CallGraphRecord, routes []CodeRouteRecord, flows []CodeFlowRecord) []EvidenceRecord {
	hashes := map[string]string{}
	for _, file := range files {
		hashes[file.Path] = file.Hash
	}
	byID := map[string]EvidenceRecord{}
	add := func(record EvidenceRecord) {
		if record.File == "" {
			return
		}
		if _, ok := hashes[record.File]; !ok {
			return
		}
		record.Project = project
		record.SourceHash = hashes[record.File]
		record.ID = StableEvidenceID(record)
		byID[record.ID] = record
	}
	for _, relation := range relations {
		add(EvidenceRecord{File: relation.From, Start: EvidenceLocation{Line: relation.Line}, Analyzer: relation.Language, Method: "syntax", Reason: relation.Type})
	}
	for _, call := range calls.Edges {
		add(EvidenceRecord{File: call.SourceFile, Start: EvidenceLocation{Line: call.Line}, Analyzer: "call", Method: "syntax", Reason: call.Reason})
	}
	for _, route := range routes {
		add(EvidenceRecord{File: route.File, Start: EvidenceLocation{Line: route.Line}, Analyzer: route.Language, Adapter: route.Framework, Method: "framework_convention", Reason: route.Reason})
	}
	for _, flow := range flows {
		for _, step := range flow.Steps {
			add(EvidenceRecord{File: step.File, Start: EvidenceLocation{Line: step.Line}, Analyzer: step.Language, Adapter: flow.Framework, Method: "exact_signature", Reason: step.Reason})
		}
	}
	records := make([]EvidenceRecord, 0, len(byID))
	for _, record := range byID {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}
