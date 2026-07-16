package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func StableWorkspaceID(kind string, parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(filepath.ToSlash(part)))
		if value != "" {
			clean = append(clean, value)
		}
	}
	raw := kind + ":" + strings.Join(clean, ":")
	if len(raw) <= 180 && !strings.ContainsAny(raw, "\n\r\t") {
		return raw
	}
	return kind + ":" + stableID(raw)
}

func BuildWorkspaceGraph(registry WorkspaceRegistryRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, dossiers []FeatureDossierRecord) WorkspaceGraphRecord {
	builder := workspaceGraphBuilder{
		nodes: map[string]WorkspaceGraphNodeRecord{},
		edges: map[string]WorkspaceGraphEdgeRecord{},
		stats: map[string]int{},
	}
	for _, project := range registry.Projects {
		projectID := StableWorkspaceID("project", project.Path)
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:      projectID,
			Kind:    "project",
			Label:   firstNonEmpty(project.Name, project.Path),
			Project: project.Path,
			Risk:    project.Status,
			Meta: map[string]string{
				"kind":    project.Kind,
				"service": project.Service,
				"status":  project.Status,
			},
		})
	}
	for _, match := range matches {
		contractID := firstNonEmpty(match.ID, StableWorkspaceID("contract", match.APIProject, match.APIHTTPMethod, match.APIPath, match.APIFile))
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:         contractID,
			Kind:       "contract",
			Label:      strings.TrimSpace(match.APIHTTPMethod + " " + match.APIPath),
			Project:    match.APIProject,
			File:       match.APIFile,
			Line:       match.APILine,
			Symbol:     match.APICaller,
			Method:     match.APIHTTPMethod,
			Path:       match.APIPath,
			Confidence: match.Confidence,
			Risk:       match.Issue,
			Meta: map[string]string{
				"service_candidate": match.ServiceCandidate,
				"resolution_class":  match.ResolutionClass,
			},
		})
		if match.APIProject != "" {
			builder.addEdge(StableWorkspaceID("project", match.APIProject), contractID, "declares_contract", match.Confidence, nil)
		}
		if match.BackendProject != "" {
			routePath := firstNonEmpty(match.BackendPath, match.APIPath)
			routeMethod := firstNonEmpty(match.BackendHTTPMethod, match.APIHTTPMethod)
			routeID := StableWorkspaceID("route", match.BackendProject, routeMethod, routePath)
			builder.addNode(WorkspaceGraphNodeRecord{
				ID:         routeID,
				Kind:       "route",
				Label:      strings.TrimSpace(routeMethod + " " + routePath),
				Project:    match.BackendProject,
				File:       match.BackendFile,
				Line:       match.BackendLine,
				Symbol:     match.BackendHandler,
				Method:     routeMethod,
				Path:       routePath,
				Confidence: match.Confidence,
			})
			builder.addEdge(StableWorkspaceID("project", match.BackendProject), routeID, "owns_route", match.Confidence, nil)
			builder.addEdge(contractID, routeID, "resolved_by", match.Confidence, map[string]string{"issue": match.Issue})
		}
		for _, candidate := range match.SimilarBackendRoutes {
			candidateID := StableWorkspaceID("candidate-route", match.BackendProject, candidate)
			builder.addNode(WorkspaceGraphNodeRecord{
				ID:      candidateID,
				Kind:    "candidate_route",
				Label:   candidate,
				Project: match.BackendProject,
				Risk:    match.Issue,
			})
			builder.addEdge(contractID, candidateID, "has_candidate", "PARTIAL_MATCH", nil)
		}
	}
	for _, flow := range flows {
		flowID := firstNonEmpty(flow.ID, StableWorkspaceID("flow", flow.FrontendProject, flow.HTTPMethod, flow.Path, flow.FrontendFile))
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:         flowID,
			Kind:       "flow",
			Label:      strings.TrimSpace(flow.HTTPMethod + " " + flow.Path),
			Project:    firstNonEmpty(flow.FrontendProject, flow.BackendProject),
			File:       flow.FrontendFile,
			Line:       flow.FrontendLine,
			Symbol:     firstNonEmpty(flow.FrontendCaller, flow.BackendController+"."+flow.BackendMethod),
			Method:     flow.HTTPMethod,
			Path:       flow.Path,
			Confidence: flow.Confidence,
			Risk:       riskFromFlow(flow),
		})
		if flow.FrontendProject != "" {
			builder.addEdge(StableWorkspaceID("project", flow.FrontendProject), flowID, "has_flow", flow.Confidence, nil)
		}
		if flow.BackendProject != "" {
			builder.addEdge(flowID, StableWorkspaceID("project", flow.BackendProject), "reaches_project", flow.Confidence, nil)
		}
		if flow.FrontendFile != "" {
			fileID := StableWorkspaceID("file", flow.FrontendProject, flow.FrontendFile)
			builder.addNode(WorkspaceGraphNodeRecord{ID: fileID, Kind: "file", Label: flow.FrontendFile, Project: flow.FrontendProject, File: flow.FrontendFile})
			builder.addEdge(flowID, fileID, "starts_in", flow.FrontendConfidence, nil)
		}
		if flow.BackendFile != "" {
			handler := strings.Trim(strings.TrimSpace(flow.BackendController)+"."+strings.TrimSpace(flow.BackendMethod), ".")
			handlerID := StableWorkspaceID("symbol", flow.BackendProject, flow.BackendFile, handler)
			builder.addNode(WorkspaceGraphNodeRecord{ID: handlerID, Kind: "backend_handler", Label: firstNonEmpty(handler, flow.BackendFile), Project: flow.BackendProject, File: flow.BackendFile, Line: flow.BackendLine, Symbol: handler})
			builder.addEdge(flowID, handlerID, "handled_by", flow.Confidence, nil)
		}
		for _, step := range flow.FrontendSteps {
			stepID := StableWorkspaceID("symbol", flow.FrontendProject, step.File, step.Name, step.Kind)
			builder.addNode(WorkspaceGraphNodeRecord{ID: stepID, Kind: firstNonEmpty(step.Kind, "frontend_step"), Label: firstNonEmpty(step.Name, step.File), Project: flow.FrontendProject, File: step.File, Line: step.Line, Symbol: step.Name, Confidence: step.Confidence})
			builder.addEdge(flowID, stepID, "has_frontend_step", step.Confidence, nil)
		}
		for _, step := range flow.BackendSteps {
			name := strings.Trim(strings.TrimSpace(step.Owner)+"."+strings.TrimSpace(step.Method), ".")
			stepID := StableWorkspaceID("symbol", flow.BackendProject, step.File, name, step.Kind)
			builder.addNode(WorkspaceGraphNodeRecord{ID: stepID, Kind: firstNonEmpty(step.Kind, "backend_step"), Label: firstNonEmpty(name, step.File), Project: flow.BackendProject, File: step.File, Line: step.Line, Symbol: name, Confidence: step.Confidence})
			builder.addEdge(flowID, stepID, "has_backend_step", step.Confidence, nil)
		}
	}
	for _, dossier := range dossiers {
		builder.addNode(WorkspaceGraphNodeRecord{
			ID:         dossier.ID,
			Kind:       "feature",
			Label:      firstNonEmpty(dossier.Route, dossier.ID),
			Project:    firstNonEmpty(dossier.FrontendProject, dossier.BackendProject),
			Symbol:     firstNonEmpty(dossier.BackendHandler, dossier.FrontendCaller),
			Confidence: dossier.Confidence,
			Risk:       riskFromDossier(dossier),
		})
		if dossier.SourceFlowID != "" {
			builder.addEdge(dossier.ID, dossier.SourceFlowID, "summarizes_flow", dossier.Confidence, nil)
		}
	}
	return builder.record(registry.Root)
}

func readWorkspaceGraph(path string) (WorkspaceGraphRecord, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceGraphRecord{}, err
	}
	var graph WorkspaceGraphRecord
	if err := json.Unmarshal(body, &graph); err != nil {
		return WorkspaceGraphRecord{}, err
	}
	return graph, nil
}

func matchGraphNode(nodes []WorkspaceGraphNodeRecord, target string) WorkspaceGraphNodeRecord {
	query := strings.ToLower(strings.TrimSpace(target))
	best := WorkspaceGraphNodeRecord{}
	bestScore := 0
	for _, node := range nodes {
		score := workspaceGraphNodeMatchScore(node, query)
		if score > bestScore {
			best = node
			bestScore = score
		}
	}
	return best
}

func workspaceGraphNodeMatchScore(node WorkspaceGraphNodeRecord, query string) int {
	if query == "" {
		return 0
	}
	id := strings.ToLower(node.ID)
	label := strings.ToLower(node.Label)
	project := strings.ToLower(node.Project)
	file := strings.ToLower(node.File)
	symbol := strings.ToLower(node.Symbol)
	route := strings.ToLower(strings.TrimSpace(node.Method + " " + node.Path))
	switch {
	case id == query:
		return 120
	case node.Kind == "file" && file == query:
		return 115
	case node.Kind == "backend_handler" && symbol == query:
		return 114
	case symbol == query:
		return 110
	case project == query:
		return 105
	case label == query:
		return 100
	case route == query:
		return 95
	case file == query:
		return 90
	case strings.Contains(id, query):
		return 70
	case strings.Contains(symbol, query):
		return 65
	case strings.Contains(label, query):
		return 60
	case strings.Contains(project, query):
		return 55
	case strings.Contains(file, query):
		if node.Kind == "file" {
			return 54
		}
		return 45
	default:
		haystack := strings.ToLower(strings.Join([]string{
			node.ID,
			node.Label,
			node.Project,
			node.File,
			node.Symbol,
			strings.TrimSpace(node.Method + " " + node.Path),
		}, " "))
		if strings.Contains(haystack, query) {
			return 30
		}
	}
	return 0
}

type workspaceGraphBuilder struct {
	nodes map[string]WorkspaceGraphNodeRecord
	edges map[string]WorkspaceGraphEdgeRecord
	stats map[string]int
}

func (b *workspaceGraphBuilder) addNode(node WorkspaceGraphNodeRecord) {
	if node.ID == "" {
		return
	}
	if existing, ok := b.nodes[node.ID]; ok {
		b.nodes[node.ID] = mergeWorkspaceGraphNode(existing, node)
		return
	}
	b.nodes[node.ID] = node
	b.stats["nodes_"+node.Kind]++
}

func (b *workspaceGraphBuilder) addEdge(from, to, kind, confidence string, meta map[string]string) {
	if from == "" || to == "" || kind == "" {
		return
	}
	id := StableWorkspaceID("edge", from, kind, to)
	if _, ok := b.edges[id]; ok {
		return
	}
	b.edges[id] = WorkspaceGraphEdgeRecord{ID: id, From: from, To: to, Kind: kind, Confidence: confidence, Meta: meta}
	b.stats["edges_"+kind]++
}

func (b *workspaceGraphBuilder) record(root string) WorkspaceGraphRecord {
	nodes := make([]WorkspaceGraphNodeRecord, 0, len(b.nodes))
	for _, node := range b.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	edges := make([]WorkspaceGraphEdgeRecord, 0, len(b.edges))
	for _, edge := range b.edges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	b.stats["nodes_total"] = len(nodes)
	b.stats["edges_total"] = len(edges)
	return WorkspaceGraphRecord{
		SchemaVersion: SchemaVersion,
		Generated:     time.Now().UTC().Format(time.RFC3339),
		Root:          root,
		Nodes:         nodes,
		Edges:         edges,
		Stats:         b.stats,
	}
}

func mergeWorkspaceGraphNode(existing, next WorkspaceGraphNodeRecord) WorkspaceGraphNodeRecord {
	if existing.Label == "" {
		existing.Label = next.Label
	}
	if existing.Project == "" {
		existing.Project = next.Project
	}
	if existing.File == "" {
		existing.File = next.File
	}
	if existing.Symbol == "" {
		existing.Symbol = next.Symbol
	}
	if existing.Confidence == "" {
		existing.Confidence = next.Confidence
	}
	if existing.Risk == "" {
		existing.Risk = next.Risk
	}
	return existing
}

func riskFromFlow(flow WorkspaceFeatureFlowRecord) string {
	if len(flow.FieldRisks) > 0 {
		return "risk"
	}
	if len(flow.Tests) == 0 {
		return "missing_tests"
	}
	return ""
}

func riskFromDossier(dossier FeatureDossierRecord) string {
	if len(dossier.Risks) > 0 {
		return "risk"
	}
	if len(dossier.Tests) == 0 {
		return "missing_tests"
	}
	return ""
}

func workspaceGraphOut(workspaceOut string) string {
	return NewWorkspaceOutputLayout(workspaceOut).Index("workspace-graph.json")
}

func graphLabel(node WorkspaceGraphNodeRecord) string {
	if node.Label != "" {
		return node.Label
	}
	return fmt.Sprintf("%s %s", node.Kind, node.ID)
}
