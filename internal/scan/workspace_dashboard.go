package scan

import (
	"bytes"
	"encoding/json"
	"strconv"
)

func RenderWorkspaceDashboardHTML(graph WorkspaceGraphRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string {
	registry := workspaceRegistryFromGraph(graph)
	serviceMap := BuildWorkspaceServiceMap(registry, matches, nil, nil)
	endpointTraces := BuildWorkspaceEndpointTraces(matches, nil, dossiers)
	return RenderWorkspaceDashboardHTMLWithModels(graph, serviceMap, endpointTraces, matches, dossiers)
}

func RenderWorkspaceDashboardHTMLWithModels(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string {
	return RenderWorkspaceDashboardHTMLWithCodeExplorer(
		graph,
		serviceMap,
		endpointTraces,
		WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion},
		WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion},
	)
}

// RenderWorkspaceDashboardHTMLWithCodeExplorer renders the offline workspace dashboard with canonical symbol projections.
func RenderWorkspaceDashboardHTMLWithCodeExplorer(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, symbolIndex WorkspaceSymbolIndexRecord, symbolUsages WorkspaceSymbolUsageIndexRecord) string {
	payloadValue := struct {
		Graph          WorkspaceGraphRecord              `json:"graph"`
		ServiceMap     WorkspaceServiceMapRecord         `json:"service_map"`
		EndpointTraces WorkspaceEndpointTraceIndexRecord `json:"endpoint_traces"`
		SymbolIndex    WorkspaceSymbolIndexRecord        `json:"symbol_index"`
		SymbolUsages   WorkspaceSymbolUsageIndexRecord   `json:"symbol_usages"`
		SourceIndex    []dashboardSourceRecord           `json:"source_index,omitempty"`
	}{
		Graph:          graph,
		ServiceMap:     serviceMap,
		EndpointTraces: endpointTraces,
		SymbolIndex:    symbolIndex,
		SymbolUsages:   symbolUsages,
		SourceIndex:    buildDashboardSourceIndex(endpointTraces),
	}
	payload := marshalDashboardPayload(payloadValue)
	title := "GoreGraph Workspace Map"
	if graph.Root != "" {
		title += " - " + graph.Root
	}
	return renderWorkspaceDashboardDocument(title, payload)
}

func marshalDashboardPayload(value any) []byte {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	_ = encoder.Encode(value)
	return bytes.TrimSpace(b.Bytes())
}

type dashboardSourceRecord struct {
	Label   string `json:"label"`
	Project string `json:"project,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
}

func buildDashboardSourceIndex(endpointTraces WorkspaceEndpointTraceIndexRecord) []dashboardSourceRecord {
	seen := map[string]bool{}
	var records []dashboardSourceRecord
	for _, trace := range endpointTraces.Traces {
		for _, step := range trace.Steps {
			if step.File == "" {
				continue
			}
			label := step.File
			if step.Line > 0 {
				label += ":" + strconv.Itoa(step.Line)
			}
			key := step.Project + "\x00" + label
			if seen[key] {
				continue
			}
			seen[key] = true
			records = append(records, dashboardSourceRecord{
				Label:   label,
				Project: step.Project,
				File:    step.File,
				Line:    step.Line,
			})
		}
	}
	return records
}

func workspaceRegistryFromGraph(graph WorkspaceGraphRecord) WorkspaceRegistryRecord {
	registry := WorkspaceRegistryRecord{Root: graph.Root}
	seen := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Kind != "project" || node.Project == "" || seen[node.Project] {
			continue
		}
		seen[node.Project] = true
		registry.Projects = append(registry.Projects, WorkspaceProjectRecord{
			Name:    firstNonEmpty(node.Label, node.Project),
			Path:    node.Project,
			Kind:    node.Meta["kind"],
			Service: node.Meta["service"],
			Indexed: true,
			Status:  node.Risk,
		})
	}
	return registry
}
