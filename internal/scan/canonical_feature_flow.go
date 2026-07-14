package scan

import (
	"fmt"
	"strings"
)

const canonicalFeatureFlowModelVersion = 1

type CanonicalFlowNodeRecord struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Project       string   `json:"project,omitempty"`
	Service       string   `json:"service,omitempty"`
	Symbol        string   `json:"symbol,omitempty"`
	QualifiedName string   `json:"qualified_name,omitempty"`
	Signature     string   `json:"signature,omitempty"`
	File          string   `json:"file,omitempty"`
	LineStart     int      `json:"line_start,omitempty"`
	LineEnd       int      `json:"line_end,omitempty"`
	Confidence    string   `json:"confidence,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	EvidenceIDs   []string `json:"evidence_ids,omitempty"`
}

type CanonicalFlowEdgeRecord struct {
	ID             string   `json:"id"`
	FromNodeID     string   `json:"from_node_id"`
	ToNodeID       string   `json:"to_node_id"`
	EdgeType       string   `json:"edge_type"`
	Confidence     string   `json:"confidence"`
	Reason         string   `json:"reason"`
	EvidenceIDs    []string `json:"evidence_ids,omitempty"`
	SourceAnalyzer string   `json:"source_analyzer,omitempty"`
}

func BuildCanonicalFeatureFlow(flow WorkspaceFeatureFlowRecord) WorkspaceFeatureFlowRecord {
	flow.ModelVersion = canonicalFeatureFlowModelVersion
	if len(flow.TestLinks) == 0 {
		flow.TestLinks = BuildTestLinks(flow)
	}
	flow.Nodes = nil
	flow.Edges = nil
	seen := map[string]bool{}
	addNode := func(node CanonicalFlowNodeRecord) string {
		if node.Kind == "" {
			return ""
		}
		node.Confidence = firstNonEmpty(node.Confidence, flow.Confidence, string(ConfidenceUnknown))
		node.Reason = firstNonEmpty(node.Reason, flow.Reason)
		node.EvidenceIDs = uniqueSortedStrings(node.EvidenceIDs)
		node.ID = StableWorkspaceID("feature-flow-node", flow.ID, node.Kind, node.Project, node.Service, node.Symbol, node.QualifiedName, node.File, fmt.Sprint(node.LineStart))
		if seen[node.ID] {
			return node.ID
		}
		seen[node.ID] = true
		flow.Nodes = append(flow.Nodes, node)
		return node.ID
	}
	ordered := []string{}
	appendNode := func(node CanonicalFlowNodeRecord) {
		if id := addNode(node); id != "" && (len(ordered) == 0 || ordered[len(ordered)-1] != id) {
			ordered = append(ordered, id)
		}
	}
	if flow.FrontendRouteID != "" || flow.FrontendRoutePath != "" || flow.FrontendRouteFile != "" {
		appendNode(CanonicalFlowNodeRecord{Kind: "frontend_route", Project: flow.FrontendProject, Symbol: firstNonEmpty(flow.FrontendRoutePath, flow.FrontendRouteID), QualifiedName: flow.FrontendRouteID, File: flow.FrontendRouteFile, LineStart: flow.FrontendRouteLine, LineEnd: flow.FrontendRouteLine, Confidence: flow.FrontendConfidence, Reason: flow.FrontendReason})
	}
	if flow.FrontendComponent != "" {
		appendNode(CanonicalFlowNodeRecord{Kind: "component", Project: flow.FrontendProject, Symbol: flow.FrontendComponent, QualifiedName: flow.FrontendComponent, File: flow.FrontendRouteFile, LineStart: flow.FrontendRouteLine, LineEnd: flow.FrontendRouteLine, Confidence: flow.FrontendConfidence, Reason: flow.FrontendReason})
	}
	for _, step := range flow.FrontendSteps {
		appendNode(CanonicalFlowNodeRecord{Kind: firstNonEmpty(step.Kind, "frontend_step"), Project: flow.FrontendProject, Service: step.Owner, Symbol: step.Name, QualifiedName: qualifiedFlowName(step.Owner, step.Name), File: step.File, LineStart: step.Line, LineEnd: step.Line, Confidence: step.Confidence, Reason: step.Reason, EvidenceIDs: step.EvidenceIDs})
	}
	appendNode(CanonicalFlowNodeRecord{Kind: "api_call", Project: flow.FrontendProject, Service: flow.BackendService, Symbol: firstNonEmpty(flow.FrontendCaller, strings.TrimSpace(flow.HTTPMethod+" "+flow.Path)), QualifiedName: flow.FrontendCaller, Signature: strings.TrimSpace(flow.HTTPMethod + " " + flow.Path), File: flow.FrontendFile, LineStart: flow.FrontendLine, LineEnd: flow.FrontendLine, Confidence: flow.Confidence, Reason: flow.Reason})
	appendNode(CanonicalFlowNodeRecord{Kind: "endpoint", Project: flow.BackendProject, Service: flow.BackendService, Symbol: firstNonEmpty(flow.BackendMethod, strings.TrimSpace(flow.HTTPMethod+" "+flow.Path)), QualifiedName: qualifiedFlowName(flow.BackendController, flow.BackendMethod), Signature: strings.TrimSpace(flow.HTTPMethod + " " + flow.Path), File: flow.BackendFile, LineStart: flow.BackendLine, LineEnd: flow.BackendLine, Confidence: flow.Confidence, Reason: flow.Reason})
	for _, step := range flow.BackendSteps {
		appendNode(CanonicalFlowNodeRecord{Kind: firstNonEmpty(step.Kind, "service_method"), Project: flow.BackendProject, Service: flow.BackendService, Symbol: step.Method, QualifiedName: qualifiedFlowName(step.Owner, step.Method), File: step.File, LineStart: step.Line, LineEnd: step.Line, Confidence: step.Confidence, Reason: "Extracted from the backend endpoint flow."})
	}
	for _, step := range flow.PersistencePath {
		appendNode(CanonicalFlowNodeRecord{Kind: "repository_method", Project: flow.BackendProject, Service: flow.BackendService, Symbol: step.Method, QualifiedName: qualifiedFlowName(step.Repository, step.Method), File: step.File, LineStart: step.Line, LineEnd: step.Line, Confidence: step.Confidence, Reason: firstNonEmpty(step.Source, "Extracted persistence path.")})
	}
	for _, test := range flow.Tests {
		appendNode(CanonicalFlowNodeRecord{Kind: "test", Project: flow.BackendProject, Symbol: firstNonEmpty(test.TestMethod, test.TestClass, test.TestCase, test.TestFile), QualifiedName: qualifiedFlowName(test.TestClass, test.TestMethod), File: test.TestFile, LineStart: test.Line, LineEnd: test.Line, Confidence: test.Confidence, Reason: test.Reason})
	}
	for index := 1; index < len(ordered); index++ {
		from, to := ordered[index-1], ordered[index]
		target := canonicalNodeByID(flow.Nodes, to)
		edgeType := canonicalEdgeType(target.Kind)
		edge := CanonicalFlowEdgeRecord{ID: StableWorkspaceID("feature-flow-edge", flow.ID, from, to, edgeType), FromNodeID: from, ToNodeID: to, EdgeType: edgeType, Confidence: firstNonEmpty(target.Confidence, flow.Confidence, string(ConfidenceUnknown)), Reason: firstNonEmpty(target.Reason, flow.Reason, "Ordered canonical feature-flow stage."), EvidenceIDs: append([]string(nil), target.EvidenceIDs...), SourceAnalyzer: "workspace-reconcile"}
		flow.Edges = append(flow.Edges, edge)
	}
	return flow
}

func ValidateCanonicalFeatureFlow(flow WorkspaceFeatureFlowRecord) error {
	if flow.ModelVersion == 0 && len(flow.Nodes) == 0 && len(flow.Edges) == 0 {
		return nil
	}
	if flow.ModelVersion != canonicalFeatureFlowModelVersion {
		return fmt.Errorf("unsupported canonical feature-flow model version %d", flow.ModelVersion)
	}
	nodes := map[string]CanonicalFlowNodeRecord{}
	for _, node := range flow.Nodes {
		if node.ID == "" || node.Kind == "" {
			return fmt.Errorf("canonical feature-flow node has no identity or kind")
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("duplicate canonical feature-flow node %s", node.ID)
		}
		nodes[node.ID] = node
	}
	edges := map[string]bool{}
	for _, edge := range flow.Edges {
		if edge.ID == "" || edge.EdgeType == "" || edge.Confidence == "" || edge.Reason == "" {
			return fmt.Errorf("canonical feature-flow edge is incomplete")
		}
		if edges[edge.ID] {
			return fmt.Errorf("duplicate canonical feature-flow edge %s", edge.ID)
		}
		edges[edge.ID] = true
		if _, ok := nodes[edge.FromNodeID]; !ok {
			return fmt.Errorf("canonical feature-flow edge %s has dangling source %s", edge.ID, edge.FromNodeID)
		}
		if _, ok := nodes[edge.ToNodeID]; !ok {
			return fmt.Errorf("canonical feature-flow edge %s has dangling target %s", edge.ID, edge.ToNodeID)
		}
	}
	return nil
}

func canonicalNodeByID(nodes []CanonicalFlowNodeRecord, id string) CanonicalFlowNodeRecord {
	for _, node := range nodes {
		if node.ID == id {
			return node
		}
	}
	return CanonicalFlowNodeRecord{}
}

func qualifiedFlowName(owner, symbol string) string {
	if owner == "" {
		return symbol
	}
	if symbol == "" {
		return owner
	}
	return owner + "." + symbol
}

func canonicalEdgeType(targetKind string) string {
	switch targetKind {
	case "component":
		return "renders"
	case "api_call":
		return "calls_api"
	case "endpoint":
		return "invokes_api"
	case "repository_method":
		return "persists_with"
	case "test":
		return "verified_by"
	default:
		return "calls"
	}
}
