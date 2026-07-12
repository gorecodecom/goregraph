package scan

import (
	"sort"
	"strings"
)

type TraceNodeRole string

const (
	TraceRoleUIRoute         TraceNodeRole = "ui_route"
	TraceRoleComponent       TraceNodeRole = "component"
	TraceRoleEventHandler    TraceNodeRole = "event_handler"
	TraceRoleAPIClient       TraceNodeRole = "api_client"
	TraceRoleHTTPRoute       TraceNodeRole = "http_route"
	TraceRoleMiddleware      TraceNodeRole = "middleware"
	TraceRoleController      TraceNodeRole = "controller_handler"
	TraceRoleFunction        TraceNodeRole = "function_method"
	TraceRoleValidation      TraceNodeRole = "validation"
	TraceRoleTransformation  TraceNodeRole = "transformation"
	TraceRoleRepository      TraceNodeRole = "repository"
	TraceRoleDatabase        TraceNodeRole = "database_table"
	TraceRoleMessageProducer TraceNodeRole = "message_producer"
	TraceRoleChannel         TraceNodeRole = "topic_queue"
	TraceRoleMessageConsumer TraceNodeRole = "message_consumer"
	TraceRoleExternal        TraceNodeRole = "external_service"
	TraceRoleTest            TraceNodeRole = "test"
)

type DirectedTraceIndexRecord struct {
	SchemaVersion int                   `json:"schema_version"`
	Traces        []DirectedTraceRecord `json:"traces"`
}
type DirectedTraceRecord struct {
	ID         string                    `json:"id"`
	Route      string                    `json:"route,omitempty"`
	Nodes      []DirectedTraceNodeRecord `json:"nodes"`
	Edges      []DirectedTraceEdgeRecord `json:"edges"`
	EntryNodes []string                  `json:"entry_nodes"`
	ExitNodes  []string                  `json:"exit_nodes"`
	MainPath   []string                  `json:"main_path"`
	Branches   []TraceBranchRecord       `json:"branches"`
	Cycles     []TraceCycleRecord        `json:"cycles"`
	Truncated  bool                      `json:"truncated,omitempty"`
}
type DirectedTraceNodeRecord struct {
	ID          string        `json:"id"`
	StableID    string        `json:"stable_id"`
	Role        TraceNodeRole `json:"role"`
	Label       string        `json:"label"`
	Project     string        `json:"project,omitempty"`
	File        string        `json:"file,omitempty"`
	Line        int           `json:"line,omitempty"`
	Symbol      string        `json:"symbol,omitempty"`
	Confidence  string        `json:"confidence,omitempty"`
	EvidenceIDs []string      `json:"evidence_ids,omitempty"`
}
type DirectedTraceEdgeRecord struct {
	ID          string                   `json:"id"`
	From        string                   `json:"from"`
	To          string                   `json:"to"`
	Relation    string                   `json:"relation"`
	Callsite    string                   `json:"callsite,omitempty"`
	EvidenceIDs []string                 `json:"evidence_ids,omitempty"`
	Confidence  string                   `json:"confidence,omitempty"`
	Async       bool                     `json:"async,omitempty"`
	Conditional bool                     `json:"conditional,omitempty"`
	Mappings    []DataFieldMappingRecord `json:"mappings,omitempty"`
}
type TraceBranchRecord struct {
	From  string     `json:"from"`
	Paths [][]string `json:"paths"`
}
type TraceCycleRecord struct {
	Nodes   []string `json:"nodes"`
	Bounded bool     `json:"bounded"`
}
type DataFieldMappingRecord struct {
	From        string     `json:"from"`
	To          string     `json:"to"`
	Confidence  Confidence `json:"confidence"`
	EvidenceIDs []string   `json:"evidence_ids,omitempty"`
}

func BuildDirectedTraceIndex(legacy WorkspaceEndpointTraceIndexRecord) DirectedTraceIndexRecord {
	result := DirectedTraceIndexRecord{SchemaVersion: legacy.SchemaVersion, Traces: []DirectedTraceRecord{}}
	for _, trace := range legacy.Traces {
		result.Traces = append(result.Traces, buildDirectedTrace(trace))
	}
	sort.Slice(result.Traces, func(i, j int) bool { return result.Traces[i].ID < result.Traces[j].ID })
	return result
}
func buildDirectedTrace(legacy WorkspaceEndpointTraceRecord) DirectedTraceRecord {
	trace := DirectedTraceRecord{ID: legacy.ID, Route: legacy.Route, Nodes: []DirectedTraceNodeRecord{}, Edges: []DirectedTraceEdgeRecord{}, Branches: []TraceBranchRecord{}, Cycles: []TraceCycleRecord{}}
	incoming := map[string]int{}
	outgoing := map[string]int{}
	for _, step := range legacy.Steps {
		trace.Nodes = append(trace.Nodes, DirectedTraceNodeRecord{ID: step.ID, StableID: StableWorkspaceID("directed-node", legacy.ID, step.ID), Role: traceRole(step), Label: step.Label, Project: step.Project, File: step.File, Line: step.Line, Symbol: step.Symbol, Confidence: step.Confidence})
		trace.MainPath = append(trace.MainPath, step.ID)
	}
	for _, edge := range legacy.Edges {
		id := StableWorkspaceID("directed-edge", legacy.ID, edge.From, edge.To, edge.Kind)
		trace.Edges = append(trace.Edges, DirectedTraceEdgeRecord{ID: id, From: edge.From, To: edge.To, Relation: firstNonEmpty(edge.Kind, "calls")})
		outgoing[edge.From]++
		incoming[edge.To]++
	}
	for _, node := range trace.Nodes {
		if incoming[node.ID] == 0 {
			trace.EntryNodes = append(trace.EntryNodes, node.ID)
		}
		if outgoing[node.ID] == 0 {
			trace.ExitNodes = append(trace.ExitNodes, node.ID)
		}
	}
	return trace
}
func traceRole(step WorkspaceEndpointTraceStepRecord) TraceNodeRole {
	text := strings.ToLower(step.Kind + " " + step.Label + " " + step.Symbol)
	switch {
	case step.Kind == "api_contract":
		return TraceRoleAPIClient
	case step.Kind == "backend_route":
		return TraceRoleHTTPRoute
	case step.Kind == "backend_handler":
		return TraceRoleController
	case step.Kind == "test":
		return TraceRoleTest
	case strings.Contains(text, "repository") || strings.Contains(text, "dao"):
		return TraceRoleRepository
	case strings.Contains(text, "validation") || strings.Contains(text, "validate"):
		return TraceRoleValidation
	case strings.Contains(text, "component"):
		return TraceRoleComponent
	case step.Kind == "frontend_step":
		return TraceRoleEventHandler
	default:
		return TraceRoleFunction
	}
}
func (trace DirectedTraceRecord) LegacyProjection() WorkspaceEndpointTraceRecord {
	result := WorkspaceEndpointTraceRecord{ID: trace.ID, Route: trace.Route, Steps: []WorkspaceEndpointTraceStepRecord{}, Edges: []WorkspaceEndpointTraceEdgeRecord{}}
	nodes := map[string]DirectedTraceNodeRecord{}
	for _, node := range trace.Nodes {
		nodes[node.ID] = node
	}
	for _, id := range trace.MainPath {
		node, ok := nodes[id]
		if !ok {
			continue
		}
		result.Steps = append(result.Steps, WorkspaceEndpointTraceStepRecord{ID: node.ID, Kind: legacyKind(node.Role), Label: node.Label, Project: node.Project, File: node.File, Line: node.Line, Symbol: node.Symbol, Confidence: node.Confidence})
	}
	result.rebuildEdges()
	return result
}
func legacyKind(role TraceNodeRole) string {
	switch role {
	case TraceRoleAPIClient:
		return "api_contract"
	case TraceRoleHTTPRoute:
		return "backend_route"
	case TraceRoleController:
		return "backend_handler"
	case TraceRoleTest:
		return "test"
	case TraceRoleEventHandler, TraceRoleComponent, TraceRoleUIRoute:
		return "frontend_step"
	default:
		return "backend_step"
	}
}
