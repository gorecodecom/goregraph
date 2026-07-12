package scan

import "sort"

type DataFlowRecord struct {
	ID      string               `json:"id"`
	Route   string               `json:"route"`
	Project string               `json:"project,omitempty"`
	Nodes   []DataFlowNodeRecord `json:"nodes"`
	Edges   []DataFlowEdgeRecord `json:"edges"`
	Gaps    []DataFlowGapRecord  `json:"gaps"`
}
type DataFlowNodeRecord struct {
	ID          string     `json:"id"`
	Role        string     `json:"role"`
	Label       string     `json:"label"`
	Field       string     `json:"field,omitempty"`
	DataType    string     `json:"data_type,omitempty"`
	File        string     `json:"file,omitempty"`
	Line        int        `json:"line,omitempty"`
	EvidenceIDs []string   `json:"evidence_ids,omitempty"`
	Confidence  Confidence `json:"confidence"`
}
type DataFlowEdgeRecord struct {
	ID          string                  `json:"id"`
	From        string                  `json:"from"`
	To          string                  `json:"to"`
	Relation    string                  `json:"relation"`
	Mapping     *DataFieldMappingRecord `json:"mapping,omitempty"`
	EvidenceIDs []string                `json:"evidence_ids,omitempty"`
	Confidence  Confidence              `json:"confidence"`
}
type DataFlowGapRecord struct {
	From               string       `json:"from"`
	To                 string       `json:"to"`
	Reason             string       `json:"reason"`
	RequiredCapability CapabilityID `json:"required_capability"`
	Confidence         Confidence   `json:"confidence"`
}

func BuildDataFlows(flows []WorkspaceFeatureFlowRecord) []DataFlowRecord {
	records := []DataFlowRecord{}
	for _, flow := range flows {
		route := flow.HTTPMethod + " " + flow.Path
		record := DataFlowRecord{ID: StableWorkspaceID("data-flow", flow.ID, route), Route: route, Project: firstNonEmpty(flow.BackendProject, flow.FrontendProject), Nodes: []DataFlowNodeRecord{}, Edges: []DataFlowEdgeRecord{}, Gaps: []DataFlowGapRecord{}}
		inputID := StableWorkspaceID("data-node", record.ID, "input")
		record.Nodes = append(record.Nodes, DataFlowNodeRecord{ID: inputID, Role: "request", Label: route, File: flow.FrontendFile, Line: flow.FrontendLine, Confidence: normalizeDiagnosticConfidence(flow.Confidence)})
		last := inputID
		for _, field := range flow.BackendRequestFields {
			id := StableWorkspaceID("data-node", record.ID, "request-field", field.Name)
			record.Nodes = append(record.Nodes, DataFlowNodeRecord{ID: id, Role: "request_field", Label: field.Name, Field: field.Name, DataType: field.Type, Confidence: normalizeDiagnosticConfidence(field.Confidence)})
			record.Edges = append(record.Edges, dataEdge(record.ID, last, id, "binds", normalizeDiagnosticConfidence(field.Confidence)))
			last = id
		}
		for _, step := range flow.PersistencePath {
			id := StableWorkspaceID("data-node", record.ID, "persistence", step.Repository, step.Method, step.Entity, step.Table)
			record.Nodes = append(record.Nodes, DataFlowNodeRecord{ID: id, Role: "persistence", Label: firstNonEmpty(step.Repository+"."+step.Method, step.Entity, step.Table), File: step.File, Line: step.Line, Confidence: normalizeDiagnosticConfidence(step.Confidence)})
			record.Gaps = append(record.Gaps, DataFlowGapRecord{From: last, To: id, Reason: "No field-level transformation mapping was extracted between request data and persistence.", RequiredCapability: CapabilityDataFlow, Confidence: ConfidenceUnknown})
			record.Edges = append(record.Edges, dataEdge(record.ID, last, id, "persists", ConfidenceInferred))
			last = id
		}
		for _, field := range flow.BackendResponseFields {
			id := StableWorkspaceID("data-node", record.ID, "response-field", field.Name)
			record.Nodes = append(record.Nodes, DataFlowNodeRecord{ID: id, Role: "response_field", Label: field.Name, Field: field.Name, DataType: field.Type, Confidence: normalizeDiagnosticConfidence(field.Confidence)})
			record.Edges = append(record.Edges, dataEdge(record.ID, last, id, "returns", normalizeDiagnosticConfidence(field.Confidence)))
			last = id
		}
		if len(flow.BackendRequestFields) == 0 {
			record.Gaps = append(record.Gaps, DataFlowGapRecord{From: inputID, Reason: "No request field shape was extracted.", RequiredCapability: CapabilityDataFlow, Confidence: ConfidenceUnknown})
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}
func dataEdge(flowID, from, to, relation string, confidence Confidence) DataFlowEdgeRecord {
	if confidence == "" {
		confidence = ConfidenceUnknown
	}
	return DataFlowEdgeRecord{ID: StableWorkspaceID("data-edge", flowID, from, to, relation), From: from, To: to, Relation: relation, Confidence: confidence}
}
