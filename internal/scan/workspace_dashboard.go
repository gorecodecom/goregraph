package scan

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

const workspaceDashboardAssetDir = "workspace-map-assets"

type workspaceDashboardArtifacts struct {
	HTML           string
	Assets         map[string][]byte
	AssetByProject map[string]string
}

type workspaceDashboardPayload struct {
	Graph           WorkspaceGraphRecord              `json:"graph"`
	ServiceMap      WorkspaceServiceMapRecord         `json:"service_map"`
	EndpointTraces  WorkspaceEndpointTraceIndexRecord `json:"endpoint_traces"`
	APICatalog      APICatalogRecord                  `json:"api_catalog"`
	SymbolIndex     WorkspaceSymbolIndexRecord        `json:"symbol_index"`
	SymbolUsages    WorkspaceSymbolUsageIndexRecord   `json:"symbol_usages"`
	CodeUsageAssets map[string]string                 `json:"code_usage_assets,omitempty"`
	SourceIndex     []dashboardSourceRecord           `json:"source_index,omitempty"`
}

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
	return renderWorkspaceDashboardHTML(graph, serviceMap, endpointTraces, APICatalogRecord{SchemaVersion: SchemaVersion}, symbolIndex, symbolUsages, nil)
}

func buildWorkspaceDashboardArtifacts(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, apiCatalog APICatalogRecord, symbolIndex WorkspaceSymbolIndexRecord, symbolUsages WorkspaceSymbolUsageIndexRecord) workspaceDashboardArtifacts {
	assets, assetByProject := buildWorkspaceDashboardUsageAssets(symbolIndex, symbolUsages)
	deferredUsages := symbolUsages
	deferredUsages.Usages = nil
	return workspaceDashboardArtifacts{
		HTML:           renderWorkspaceDashboardHTML(graph, serviceMap, endpointTraces, apiCatalog, symbolIndex, deferredUsages, assetByProject),
		Assets:         assets,
		AssetByProject: assetByProject,
	}
}

func renderWorkspaceDashboardHTML(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, apiCatalog APICatalogRecord, symbolIndex WorkspaceSymbolIndexRecord, symbolUsages WorkspaceSymbolUsageIndexRecord, codeUsageAssets map[string]string) string {
	payloadValue := workspaceDashboardPayload{
		Graph:           graph,
		ServiceMap:      serviceMap,
		EndpointTraces:  endpointTraces,
		APICatalog:      apiCatalog,
		SymbolIndex:     symbolIndex,
		SymbolUsages:    symbolUsages,
		CodeUsageAssets: codeUsageAssets,
		SourceIndex:     buildDashboardSourceIndex(endpointTraces),
	}
	payload := marshalDashboardPayload(payloadValue)
	title := "GoreGraph Workspace Map"
	if graph.Root != "" {
		title += " - " + graph.Root
	}
	return renderWorkspaceDashboardDocument(title, payload)
}

func buildWorkspaceDashboardUsageAssets(symbolIndex WorkspaceSymbolIndexRecord, symbolUsages WorkspaceSymbolUsageIndexRecord) (map[string][]byte, map[string]string) {
	symbolProject := make(map[string]string, len(symbolIndex.Symbols))
	projectSet := map[string]bool{}
	for _, symbol := range symbolIndex.Symbols {
		symbolProject[symbol.ID] = symbol.Project
		if symbol.Project != "" {
			projectSet[symbol.Project] = true
		}
	}

	usagesByProject := make(map[string][]CanonicalSymbolUsageRecord, len(projectSet))
	for _, usage := range symbolUsages.Usages {
		relevantProjects := map[string]bool{}
		if project := symbolProject[usage.ProviderSymbolID]; project != "" {
			relevantProjects[project] = true
		}
		for _, candidateID := range usage.CandidateSymbolIDs {
			if project := symbolProject[candidateID]; project != "" {
				relevantProjects[project] = true
			}
		}
		if usage.Transport == "http" &&
			(usage.Category == SymbolUsageReachedThroughAPI ||
				usage.Category == SymbolUsageAmbiguous ||
				usage.Category == SymbolUsageUnresolved) {
			if project := symbolProject[usage.ConsumerSymbolID]; project != "" {
				relevantProjects[project] = true
			}
		}
		for project := range relevantProjects {
			usagesByProject[project] = append(usagesByProject[project], usage)
		}
	}

	projects := make([]string, 0, len(projectSet))
	for project := range projectSet {
		projects = append(projects, project)
	}
	sort.Strings(projects)

	assets := make(map[string][]byte, len(projects))
	assetByProject := make(map[string]string, len(projects))
	for _, project := range projects {
		sum := sha256.Sum256([]byte(project))
		assetPath := fmt.Sprintf("%s/code-usages-%x.js", workspaceDashboardAssetDir, sum[:8])
		payload := WorkspaceSymbolUsageIndexRecord{
			SchemaVersion: symbolUsages.SchemaVersion,
			Generated:     symbolUsages.Generated,
			Root:          symbolUsages.Root,
			Usages:        usagesByProject[project],
		}
		projectJSON := marshalDashboardPayload(project)
		payloadJSON := marshalDashboardPayload(payload)
		asset := make([]byte, 0, len(projectJSON)+len(payloadJSON)+64)
		asset = append(asset, "globalThis.__goregraphRegisterCodeUsageShard("...)
		asset = append(asset, projectJSON...)
		asset = append(asset, ',')
		asset = append(asset, payloadJSON...)
		asset = append(asset, ");\n"...)
		assets[assetPath] = asset
		assetByProject[project] = assetPath
	}
	return assets, assetByProject
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
