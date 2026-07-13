package scan

import (
	"sort"
	"strings"
	"time"
)

func BuildWorkspaceServiceMap(registry WorkspaceRegistryRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, dependencies []WorkspaceServiceDependencyRecord) WorkspaceServiceMapRecord {
	builder := workspaceServiceMapBuilder{
		nodes: map[string]WorkspaceServiceNodeRecord{},
		edges: map[string]*WorkspaceServiceEdgeRecord{},
	}
	for _, project := range registry.Projects {
		nodeID := StableWorkspaceID("service", project.Path)
		builder.nodes[nodeID] = WorkspaceServiceNodeRecord{
			ID:      nodeID,
			Label:   firstNonEmpty(project.Service, project.Name, project.Path),
			Project: project.Path,
			Kind:    project.Kind,
			Role:    workspaceServiceRole(project),
			Domain:  workspaceServiceDomain(project),
			Service: project.Service,
			Indexed: project.Indexed,
			Status:  project.Status,
		}
	}
	serviceProjects := workspaceServiceProjectLookup(registry)
	seenRoutes := map[string]bool{}
	for _, match := range matches {
		toProject := match.BackendProject
		if toProject == "" && match.ServiceCandidate != "" {
			toProject = firstNonEmpty(serviceProjects[normalizeServiceName(match.ServiceCandidate)], match.ServiceCandidate)
		}
		if match.APIProject == "" || toProject == "" || match.APIProject == toProject {
			continue
		}
		builder.addEdge(match.APIProject, toProject, match.APIHTTPMethod, match.APIPath, match.Confidence, match.Issue, match.APIFile)
		seenRoutes[workspaceServiceRouteKey(match.APIProject, toProject, match.APIHTTPMethod, match.APIPath)] = true
	}
	for _, flow := range flows {
		if flow.FrontendProject == "" || flow.BackendProject == "" || flow.FrontendProject == flow.BackendProject {
			continue
		}
		if seenRoutes[workspaceServiceRouteKey(flow.FrontendProject, flow.BackendProject, flow.HTTPMethod, flow.Path)] {
			builder.addEvidence(flow.FrontendProject, flow.BackendProject, flow.FrontendFile)
			continue
		}
		builder.addEdge(flow.FrontendProject, flow.BackendProject, flow.HTTPMethod, flow.Path, flow.Confidence, riskFromFlow(flow), flow.FrontendFile)
	}
	for _, dependency := range dependencies {
		toProject := dependency.ToProject
		if toProject == "" && dependency.ToService != "" {
			toProject = firstNonEmpty(serviceProjects[normalizeServiceName(dependency.ToService)], dependency.ToService)
		}
		if dependency.FromProject == "" || toProject == "" || dependency.FromProject == toProject {
			continue
		}
		builder.addDependencyEdge(dependency.FromProject, toProject, dependency)
	}
	record := builder.record(registry.Root)
	record.ContractSummary = BuildWorkspaceContractSummary(matches)
	return record
}

func BuildWorkspaceContractSummary(matches []WorkspaceContractMatchRecord) WorkspaceContractSummaryRecord {
	summary := WorkspaceContractSummaryRecord{Total: len(matches)}
	for _, match := range matches {
		switch {
		case match.Issue == contractIssueMatched || strings.EqualFold(match.Confidence, "RESOLVED"):
			summary.Resolved++
		case match.Issue == contractIssueMethodMismatch || strings.EqualFold(match.Confidence, "MISMATCH"):
			summary.MethodMismatch++
		case match.Issue == contractIssueDynamicEndpointUnresolved || match.Issue == contractIssueUnsafeDynamic:
			summary.DynamicUnresolved++
		case match.Issue == contractIssueFrontendInternalAPI || strings.EqualFold(match.Confidence, "OUT_OF_SCOPE"):
			summary.OutOfScope++
		case match.Issue == contractIssueMissingRoute || match.Issue == contractIssueIndexedBackendRouteMissing || match.Issue == contractIssueScannedServiceNoRoute:
			summary.MissingRoute++
		default:
			summary.Other++
		}
	}
	return summary
}

type workspaceServiceMapBuilder struct {
	nodes map[string]WorkspaceServiceNodeRecord
	edges map[string]*WorkspaceServiceEdgeRecord
}

func (b *workspaceServiceMapBuilder) addEdge(fromProject, toProject, method, path, confidence, issue, evidence string) {
	fromID := StableWorkspaceID("service", fromProject)
	toID := StableWorkspaceID("service", toProject)
	b.ensureNode(fromID, fromProject)
	b.ensureNode(toID, toProject)
	edgeID := StableWorkspaceID("service-edge", fromProject, toProject)
	edge := b.edges[edgeID]
	if edge == nil {
		edge = &WorkspaceServiceEdgeRecord{
			ID:          edgeID,
			From:        fromID,
			To:          toID,
			FromProject: fromProject,
			ToProject:   toProject,
			Direction:   fromProject + " -> " + toProject,
		}
		b.edges[edgeID] = edge
	}
	edge.Total++
	bucket := serviceEdgeBucket(confidence, issue)
	switch bucket {
	case "resolved":
		edge.Resolved++
	case "mismatched":
		edge.Mismatched++
	case "out_of_scope":
		edge.OutOfScope++
	default:
		edge.Unresolved++
	}
	if route := strings.TrimSpace(strings.TrimSpace(method) + " " + strings.TrimSpace(path)); route != "" {
		edge.Endpoints = appendUniqueLimit(edge.Endpoints, route, 12)
		if bucket != "resolved" && bucket != "out_of_scope" {
			edge.Problems = appendUniqueLimit(edge.Problems, strings.TrimSpace(route+" - "+issue), 8)
		}
	}
	if evidence != "" {
		edge.Evidence = appendUniqueLimit(edge.Evidence, evidence, 8)
	}
	edge.Risk = serviceEdgeRisk(edge)
}

func (b *workspaceServiceMapBuilder) addEvidence(fromProject, toProject, evidence string) {
	edge := b.edges[StableWorkspaceID("service-edge", fromProject, toProject)]
	if edge == nil || evidence == "" {
		return
	}
	edge.Evidence = appendUniqueLimit(edge.Evidence, evidence, 8)
}

func (b *workspaceServiceMapBuilder) addDependencyEdge(fromProject, toProject string, dependency WorkspaceServiceDependencyRecord) {
	fromID := StableWorkspaceID("service", fromProject)
	toID := StableWorkspaceID("service", toProject)
	b.ensureNode(fromID, fromProject)
	b.ensureNode(toID, toProject)
	edgeID := StableWorkspaceID("service-edge", fromProject, toProject)
	edge := b.edges[edgeID]
	if edge == nil {
		edge = &WorkspaceServiceEdgeRecord{
			ID:          edgeID,
			From:        fromID,
			To:          toID,
			FromProject: fromProject,
			ToProject:   toProject,
			Direction:   fromProject + " -> " + toProject,
		}
		b.edges[edgeID] = edge
	}
	edge.Total++
	edge.Resolved++
	if dependency.Kind != "" {
		edge.Endpoints = appendUniqueLimit(edge.Endpoints, dependency.Kind, 6)
	}
	if dependency.Evidence != "" {
		edge.Evidence = appendUniqueLimit(edge.Evidence, dependency.Evidence, 12)
	}
	edge.Risk = serviceEdgeRisk(edge)
}

func (b *workspaceServiceMapBuilder) ensureNode(id, project string) {
	if _, ok := b.nodes[id]; ok {
		return
	}
	b.nodes[id] = WorkspaceServiceNodeRecord{
		ID:      id,
		Label:   project,
		Project: project,
		Kind:    "project",
		Role:    workspaceServiceRole(WorkspaceProjectRecord{Path: project, Name: project}),
		Domain:  workspaceServiceDomain(WorkspaceProjectRecord{Path: project, Name: project}),
		Indexed: false,
		Status:  "referenced",
	}
}

func (b *workspaceServiceMapBuilder) record(root string) WorkspaceServiceMapRecord {
	for _, edge := range b.edges {
		from := b.nodes[edge.From]
		from.Outgoing += edge.Total
		b.nodes[edge.From] = from
		to := b.nodes[edge.To]
		to.Incoming += edge.Total
		b.nodes[edge.To] = to
	}
	nodes := make([]WorkspaceServiceNodeRecord, 0, len(b.nodes))
	for _, node := range b.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Project < nodes[j].Project
	})
	edges := make([]WorkspaceServiceEdgeRecord, 0, len(b.edges))
	for _, edge := range b.edges {
		edges = append(edges, *edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromProject == edges[j].FromProject {
			return edges[i].ToProject < edges[j].ToProject
		}
		return edges[i].FromProject < edges[j].FromProject
	})
	return WorkspaceServiceMapRecord{
		SchemaVersion: SchemaVersion,
		Generated:     time.Now().UTC().Format(time.RFC3339),
		Root:          root,
		Nodes:         nodes,
		Edges:         edges,
		Stats: map[string]int{
			"nodes": len(nodes),
			"edges": len(edges),
		},
	}
}

func serviceEdgeBucket(confidence, issue string) string {
	confidence = strings.ToUpper(strings.TrimSpace(confidence))
	issue = strings.ToLower(strings.TrimSpace(issue))
	if issue == "frontend_internal_api" || confidence == "OUT_OF_SCOPE" {
		return "out_of_scope"
	}
	if confidence == "RESOLVED" || confidence == "MATCHED" || confidence == "EXTRACTED" {
		return "resolved"
	}
	if strings.Contains(issue, "mismatch") || confidence == "MISMATCH" || confidence == "WEAK_MATCH" {
		return "mismatched"
	}
	return "unresolved"
}

func serviceEdgeRisk(edge *WorkspaceServiceEdgeRecord) string {
	switch {
	case edge.Mismatched > 0:
		return "has_mismatches"
	case edge.Unresolved > 0:
		return "has_unresolved"
	case edge.OutOfScope > 0 && edge.Resolved == 0:
		return "out_of_scope"
	default:
		return "resolved"
	}
}

func appendUniqueLimit(values []string, value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	if limit > 0 && len(values) >= limit {
		return values
	}
	return append(values, value)
}

func workspaceServiceRouteKey(fromProject, toProject, method, path string) string {
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(fromProject),
		strings.TrimSpace(toProject),
		strings.TrimSpace(method),
		strings.TrimSpace(path),
	}, "\x00"))
}

func workspaceServiceProjectLookup(registry WorkspaceRegistryRecord) map[string]string {
	lookup := map[string]string{}
	for _, project := range registry.Projects {
		for _, candidate := range []string{project.Service, project.Name, project.Path} {
			key := normalizeServiceName(candidate)
			if key != "" {
				lookup[key] = project.Path
			}
		}
	}
	return lookup
}

func workspaceServiceDependencies(projects []workspaceIndexProject) []WorkspaceServiceDependencyRecord {
	var records []WorkspaceServiceDependencyRecord
	for _, project := range projects {
		for _, dependency := range project.dependencies {
			if dependency.FromProject == "" {
				dependency.FromProject = project.record.Path
			}
			records = append(records, dependency)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].FromProject != records[j].FromProject {
			return records[i].FromProject < records[j].FromProject
		}
		if records[i].ToService != records[j].ToService {
			return records[i].ToService < records[j].ToService
		}
		return records[i].Evidence < records[j].Evidence
	})
	return records
}

func normalizeServiceName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "microservices/")
	value = strings.TrimPrefix(value, "services/")
	return value
}

func workspaceServiceRole(project WorkspaceProjectRecord) string {
	text := strings.ToLower(strings.Join([]string{project.Kind, project.Path, project.Name, project.Service}, " "))
	switch {
	case strings.Contains(text, "frontend/") || strings.Contains(text, "frontend") || strings.Contains(text, "playwright"):
		return "frontend"
	case strings.Contains(text, "microservices/") || strings.Contains(text, "services/") || strings.HasPrefix(strings.ToLower(project.Name), "ms-") || strings.HasPrefix(strings.ToLower(project.Service), "ms-"):
		return "backend"
	default:
		return "internal"
	}
}

func workspaceServiceDomain(project WorkspaceProjectRecord) string {
	if workspaceServiceRole(project) == "frontend" {
		return "frontend"
	}
	text := strings.ToLower(strings.Join([]string{project.Path, project.Name, project.Service}, " "))
	switch {
	case strings.Contains(text, "document"), strings.Contains(text, "container"), strings.Contains(text, "topic"):
		return "document"
	case strings.Contains(text, "cadaster"), strings.Contains(text, "regulation"):
		return "cadaster"
	case strings.Contains(text, "user"), strings.Contains(text, "license"), strings.Contains(text, "product"), strings.Contains(text, "invoice"), strings.Contains(text, "shop"):
		return "identity"
	case strings.Contains(text, "task"), strings.Contains(text, "search"), strings.Contains(text, "portal"), strings.Contains(text, "pdf"), strings.Contains(text, "mail"), strings.Contains(text, "sync"), strings.Contains(text, "import"), strings.Contains(text, "update"):
		return "platform"
	default:
		return "platform"
	}
}
