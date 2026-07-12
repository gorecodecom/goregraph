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

func LinkEvidenceReferences(project string, files []FileRecord, relations []RichRelationRecord, calls *CallGraphRecord, routes []CodeRouteRecord, flows []CodeFlowRecord, matches []ContractMatchRecord) []EvidenceRecord {
	hashes := map[string]string{}
	for _, file := range files {
		hashes[file.Path] = file.Hash
	}
	byID := map[string]EvidenceRecord{}
	link := func(file string, line int, analyzer, adapter, method, reason string) string {
		hash, ok := hashes[file]
		if !ok || file == "" {
			return ""
		}
		record := EvidenceRecord{Project: project, File: file, Start: EvidenceLocation{Line: line}, Analyzer: analyzer, Adapter: adapter, Method: method, Reason: reason, SourceHash: hash}
		record.ID = StableEvidenceID(record)
		byID[record.ID] = record
		return record.ID
	}
	for i := range relations {
		if id := link(relations[i].From, relations[i].Line, relations[i].Language, "", "syntax", relations[i].Type); id != "" {
			relations[i].EvidenceIDs = []string{id}
		}
	}
	if calls != nil {
		for i := range calls.Edges {
			if id := link(calls.Edges[i].SourceFile, calls.Edges[i].Line, "call", "", "syntax", calls.Edges[i].Reason); id != "" {
				calls.Edges[i].EvidenceIDs = []string{id}
			}
		}
	}
	for i := range routes {
		if id := link(routes[i].File, routes[i].Line, routes[i].Language, routes[i].Framework, "framework_convention", routes[i].Reason); id != "" {
			routes[i].EvidenceIDs = []string{id}
		}
	}
	for i := range flows {
		for j := range flows[i].Steps {
			step := &flows[i].Steps[j]
			if id := link(step.File, step.Line, step.Language, flows[i].Framework, "exact_signature", step.Reason); id != "" {
				step.EvidenceIDs = []string{id}
			}
		}
	}
	for i := range matches {
		ids := []string{}
		if id := link(matches[i].APIFile, matches[i].APILine, "api_client", "", "syntax", matches[i].Reason); id != "" {
			ids = append(ids, id)
		}
		if id := link(matches[i].BackendFile, matches[i].BackendLine, "route", "", "normalized_route_match", matches[i].Reason); id != "" {
			ids = append(ids, id)
		}
		matches[i].EvidenceIDs = ids
	}
	records := make([]EvidenceRecord, 0, len(byID))
	for _, record := range byID {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}
