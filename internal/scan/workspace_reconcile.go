package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
)

var workspaceGroupDirs = []string{"frontend", "frontends", "microservices", "services", "backends"}

type workspaceIndexProject struct {
	record             WorkspaceProjectRecord
	routes             []CodeRouteRecord
	legacyRelations    []RelationRecord
	symbols            []RichSymbolRecord
	relations          []RichRelationRecord
	callGraph          CallGraphRecord
	maven              MavenGraphRecord
	packages           PackageGraphRecord
	evidence           []EvidenceRecord
	loadFailures       []string
	missingFacts       []string
	contracts          []APIContractRecord
	codeFlows          []CodeFlowRecord
	spring             SpringIndex
	endpoints          []SpringEndpointRecord
	endpointFlows      []SpringEndpointFlowRecord
	testMap            []TestMapRecord
	dependencies       []WorkspaceServiceDependencyRecord
	capabilities       []CapabilityRecord
	diagnostics        []CanonicalDiagnosticRecord
	diagnosticFamilies []DiagnosticFamilyRecord
	freshness          ArtifactFreshnessIndex
}

type workspaceBackendRoute struct {
	project WorkspaceProjectRecord
	route   CodeRouteRecord
}

// ReconcileWorkspace refreshes workspace-level overlay files after a local scan.
func ReconcileWorkspace(currentRoot string, cfg config.Config) (*WorkspaceRegistryRecord, error) {
	return ReconcileWorkspaceTarget(currentRoot, cfg, BuildTargetAll)
}

func ReconcileWorkspaceTarget(currentRoot string, cfg config.Config, target BuildTarget) (*WorkspaceRegistryRecord, error) {
	if err := target.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Workspace {
		return nil, nil
	}
	currentAbs, err := filepath.Abs(currentRoot)
	if err != nil {
		return nil, err
	}
	workspaceRoot, ok, err := resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
	if err != nil || !ok {
		return nil, err
	}
	dashboardConfig, _, err := LoadWorkspaceDashboardConfig(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", WorkspaceDashboardConfigName, err)
	}
	workspaceOut := filepath.Join(workspaceRoot, ".goregraph-workspace")
	if legacyGeneratedOutputExists(workspaceOut) {
		return nil, fmt.Errorf("legacy pre-1.3.0 workspace output detected; run `goregraph workspace clean %s --execute` and `goregraph workspace build all %s`", currentRoot, currentRoot)
	}

	projects, err := discoverWorkspaceProjects(workspaceRoot, currentAbs, cfg.OutputDir)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, nil
	}

	registry := WorkspaceRegistryRecord{
		Root:           filepath.ToSlash(workspaceRoot),
		Current:        workspaceRel(workspaceRoot, currentAbs),
		ReconciledFrom: workspaceRel(workspaceRoot, currentAbs),
		Generated:      time.Now().UTC().Format(time.RFC3339),
		Projects:       projects,
	}

	indexed, err := loadWorkspaceIndexes(projects)
	if err != nil {
		return nil, err
	}
	var projectContextIndexes []AgentContextIndexRecord
	if target.IncludesAgent() {
		projectContextIndexes, err = loadWorkspaceAgentContextIndexes(indexed)
		if err != nil {
			return nil, err
		}
	}
	context := buildWorkspaceContext(registry, indexed)
	matches := buildWorkspaceContractMatches(indexed)
	featureFlows := buildWorkspaceFeatureFlows(indexed, matches)
	apiCatalog, err := BuildWorkspaceAPICatalog(registry, indexed, matches, featureFlows, registry.Generated)
	if err != nil {
		return nil, err
	}
	var symbolIndex WorkspaceSymbolIndexRecord
	var symbolUsageIndex WorkspaceSymbolUsageIndexRecord
	dataFlows := BuildDataFlows(featureFlows)
	featureDossiers := buildFeatureDossiers(featureFlows, matches)
	workspaceGraph := BuildWorkspaceGraph(registry, matches, featureFlows, featureDossiers)
	architectureLayout := BuildWorkspaceArchitectureLayout(registry, workspaceProjectNamespaces(indexed), dashboardConfig)
	serviceMap := BuildWorkspaceServiceMapWithLayout(registry, matches, featureFlows, workspaceServiceDependencies(indexed), architectureLayout)
	serviceMap.WorkspaceCoverage = BuildWorkspaceCoverage(context, serviceMap.ContractSummary)
	serviceMap.ImpactSummaries = BuildImpactSummaries(featureFlows, serviceMap, serviceMap.WorkspaceCoverage, 3)
	serviceMap.EditorURLTemplate = cfg.EditorURLTemplate
	serviceMap.DataFlows = dataFlows
	for _, project := range indexed {
		serviceMap.Capabilities = append(serviceMap.Capabilities, project.capabilities...)
		serviceMap.Diagnostics = append(serviceMap.Diagnostics, project.diagnostics...)
		serviceMap.DiagnosticFamilies = append(serviceMap.DiagnosticFamilies, project.diagnosticFamilies...)
	}
	endpointTraces := BuildWorkspaceEndpointTraces(matches, featureFlows, featureDossiers)
	directedTraces := BuildDirectedTraceIndex(endpointTraces)
	endpointTraces.Directed = directedTraces.Traces
	var agentContextIndex AgentContextIndexRecord
	if target.IncludesAgent() {
		agentContextIndex = BuildWorkspaceAgentContextIndex(
			registry,
			projectContextIndexes,
			matches,
			featureDossiers,
			endpointTraces,
			apiCatalog,
			registry.Generated,
		)
	}
	if target.IncludesDashboard() {
		symbolIndex, symbolUsageIndex, err = BuildWorkspaceSymbolProjection(registry, indexed, registry.Generated)
		if err != nil {
			return nil, err
		}
		symbolUsageIndex, err = finalizeWorkspaceSymbolUsageProjection(symbolIndex, symbolUsageIndex, matches, featureFlows, endpointTraces, indexed)
		if err != nil {
			return nil, err
		}
		if err := validateWorkspaceSymbolProjectionEvidence(symbolIndex, symbolUsageIndex, indexed); err != nil {
			return nil, err
		}
	}
	nextActions := renderWorkspaceNextActionsReport(context, matches, featureFlows)
	workspaceFreshness := withReconciledAPICatalogFreshness(BuildWorkspaceFreshness(indexed, registry.Generated), registry.Generated)
	var dashboardFiles []string
	var dashboardArtifacts workspaceDashboardArtifacts
	if target.IncludesDashboard() {
		dashboardArtifacts = buildWorkspaceDashboardArtifacts(workspaceGraph, serviceMap, endpointTraces, apiCatalog, symbolIndex, symbolUsageIndex)
		dashboardFiles = workspaceDashboardFiles(dashboardArtifacts.Assets)
	}

	if err := os.MkdirAll(workspaceOut, 0o755); err != nil {
		return nil, err
	}
	layout := NewWorkspaceOutputLayout(workspaceOut)
	previous := readCurrentOutputManifest(layout.Manifest)
	previous.Agent = currentAgentProjectionStatus(layout.Root, previous.Agent)
	previous.Dashboard = validProjectionStatus(layout.Root, previous.Dashboard)
	indexFiles := workspaceIndexFiles(target)
	if previous.Dashboard.Complete && !target.IncludesDashboard() {
		indexFiles = mergeGeneratedPaths(indexFiles, workspaceIndexFiles(BuildTargetDashboard))
	}
	manifest := OutputManifest{
		Tool:      ToolName,
		Schema:    SchemaVersion,
		Scope:     "workspace",
		OutputDir: ".goregraph-workspace",
		Index: ProjectionStatus{
			GeneratedAt: registry.Generated,
			Complete:    true,
			Files:       indexFiles,
		},
		Agent:     previous.Agent,
		Dashboard: previous.Dashboard,
	}
	if target.IncludesAgent() {
		manifest.Agent = ProjectionStatus{GeneratedAt: registry.Generated, Complete: true, Files: prefixedGeneratedFiles("agent", AgentGeneratedFiles)}
	}
	if target.IncludesDashboard() {
		manifest.Dashboard = ProjectionStatus{GeneratedAt: registry.Generated, Complete: true, Files: dashboardFiles}
	}
	if err := writeOutputManifestAtomic(layout.Manifest, incompleteManifestForTarget(manifest, target)); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(workspaceOut, "index"), 0o755); err != nil {
		return nil, err
	}
	if err := projectionWriteHook("workspace", "index"); err != nil {
		return nil, err
	}
	if target.IncludesDashboard() {
		if err := writeWorkspaceSymbolProjectionPair(filepath.Join(workspaceOut, "index"), symbolIndex, symbolUsageIndex); err != nil {
			return nil, err
		}
	}
	if err := writeJSON(layout.Index("registry.json"), registry); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("context.json"), context); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("contract-matches.json"), matches); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("feature-flows.json"), featureFlows); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("api-catalog.json"), apiCatalog); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("data-flows.json"), dataFlows); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("feature-dossiers.json"), featureDossiers); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("workspace-graph.json"), workspaceGraph); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("workspace-service-map.json"), serviceMap); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("workspace-endpoint-traces.json"), endpointTraces); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("directed-traces.json"), directedTraces); err != nil {
		return nil, err
	}
	if err := writeJSON(layout.Index("freshness.json"), workspaceFreshness); err != nil {
		return nil, err
	}
	if target.IncludesDashboard() {
		if err := os.RemoveAll(filepath.Join(workspaceOut, "dashboard")); err != nil {
			return nil, err
		}
		if err := projectionWriteHook("workspace", "dashboard"); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(workspaceOut, "dashboard"), 0o755); err != nil {
			return nil, err
		}
		for assetPath, body := range dashboardArtifacts.Assets {
			path := layout.Dashboard(filepath.FromSlash(assetPath))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(path, body, 0o644); err != nil {
				return nil, err
			}
		}
		for name, body := range map[string]string{
			"workspace-map.html":   dashboardArtifacts.HTML,
			"workspace-context.md": renderWorkspaceContextReport(context),
			"contract-matches.md":  renderWorkspaceContractMatchesReport(matches),
			"feature-flows.md":     renderWorkspaceFeatureFlowsReport(featureFlows),
			"feature-dossiers.md":  renderFeatureDossiersReport(featureDossiers),
			"next-actions.md":      nextActions,
		} {
			if err := os.WriteFile(layout.Dashboard(name), []byte(body), 0o644); err != nil {
				return nil, err
			}
		}
	}
	if target.IncludesAgent() {
		if err := os.RemoveAll(filepath.Join(workspaceOut, "agent")); err != nil {
			return nil, err
		}
		if err := projectionWriteHook("workspace", "agent"); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(workspaceOut, "agent"), 0o755); err != nil {
			return nil, err
		}
		if err := writeJSON(layout.Agent("context-index.json"), agentContextIndex); err != nil {
			return nil, err
		}
		if err := os.WriteFile(layout.Agent("agent-guide.md"), []byte(renderAgentGuideEntry()), 0o644); err != nil {
			return nil, err
		}
	}

	for _, project := range indexed {
		out := filepath.Join(project.record.AbsPath, project.record.OutputDir)
		projectLayout := NewProjectOutputLayout(out)
		projectManifest := readCurrentOutputManifest(projectLayout.Manifest)
		writeProjectDashboard := target.IncludesDashboard() &&
			validProjectionStatus(projectLayout.Root, projectManifest.Dashboard).Complete
		if err := publishProjectReconciliationIncomplete(projectLayout, writeProjectDashboard); err != nil {
			return nil, err
		}
		projectMatches := filterWorkspaceContractMatches(project.record.Path, matches)
		if err := writeJSON(projectLayout.Index("workspace-contract-matches.json"), projectMatches); err != nil {
			return nil, err
		}
		projectCatalog := filterWorkspaceAPICatalog(project.record.Path, apiCatalog)
		if err := writeJSON(projectLayout.Index("api-catalog.json"), projectCatalog); err != nil {
			return nil, err
		}
		if err := refreshProjectAPICatalogFreshness(projectLayout, registry.Generated); err != nil {
			return nil, err
		}
		projectFeatureFlows := filterWorkspaceFeatureFlows(project.record.Path, featureFlows)
		if err := writeJSON(projectLayout.Index("workspace-feature-flows.json"), projectFeatureFlows); err != nil {
			return nil, err
		}
		projectDossiers := filterFeatureDossiers(project.record.Path, featureDossiers)
		if err := writeJSON(projectLayout.Index("workspace-feature-dossiers.json"), projectDossiers); err != nil {
			return nil, err
		}
		if err := writeJSON(projectLayout.Index("workspace-graph.json"), filterWorkspaceGraph(project.record.Path, workspaceGraph)); err != nil {
			return nil, err
		}
		if err := updateWorkspaceProjectDiagnostics(projectLayout, project.record.Path, matches, writeProjectDashboard); err != nil {
			return nil, err
		}
		if writeProjectDashboard {
			for name, body := range map[string]string{
				"workspace-context.md":          renderProjectWorkspaceContextReport(context, project.record.Path),
				"workspace-contract-matches.md": renderProjectWorkspaceMatchesReport(project.record.Path, matches),
				"workspace-feature-flows.md":    renderWorkspaceFeatureFlowsReport(projectFeatureFlows),
				"workspace-feature-dossiers.md": renderFeatureDossiersReport(projectDossiers),
				"workspace-map.md":              renderProjectWorkspaceMapPointer(workspaceRoot),
				"workspace-next-actions.md":     nextActions,
				"frontend-consumers.md":         renderFrontendConsumersReport(project.record.Path, matches),
			} {
				if err := os.WriteFile(projectLayout.Dashboard(name), []byte(body), 0o644); err != nil {
					return nil, err
				}
			}
			if err := updateWorkspaceEndpointConsumers(projectLayout, project.record.Path, matches); err != nil {
				return nil, err
			}
		}
		if err := republishReconciledProjectManifest(projectLayout, writeProjectDashboard, registry.Generated); err != nil {
			return nil, err
		}
	}
	if err := writeOutputManifestAtomic(layout.Manifest, manifest); err != nil {
		return nil, err
	}
	return &registry, nil
}

func publishProjectReconciliationIncomplete(layout OutputLayout, writeDashboard bool) error {
	manifest := readCurrentOutputManifest(layout.Manifest)
	if manifest.Scope != "project" {
		return fmt.Errorf("project manifest %s is missing or invalid", layout.Manifest)
	}
	manifest.Index.Complete = false
	manifest.Agent = currentAgentProjectionStatus(layout.Root, manifest.Agent)
	if writeDashboard {
		manifest.Dashboard.Complete = false
	} else {
		manifest.Dashboard = validProjectionStatus(layout.Root, manifest.Dashboard)
	}
	return writeOutputManifestAtomic(layout.Manifest, manifest)
}

func republishReconciledProjectManifest(layout OutputLayout, writeDashboard bool, generatedAt string) error {
	manifest := readCurrentOutputManifest(layout.Manifest)
	if manifest.Scope != "project" {
		return fmt.Errorf("project manifest %s is missing or invalid", layout.Manifest)
	}
	manifest.Index.Complete = true
	manifest.Index = validProjectionStatus(layout.Root, manifest.Index)
	if !manifest.Index.Complete {
		return fmt.Errorf("project canonical index is incomplete after workspace reconciliation: %s", layout.Root)
	}
	manifest.Index.GeneratedAt = generatedAt
	manifest.Agent = currentAgentProjectionStatus(layout.Root, manifest.Agent)
	if writeDashboard {
		manifest.Dashboard.Complete = true
		manifest.Dashboard = validProjectionStatus(layout.Root, manifest.Dashboard)
		if !manifest.Dashboard.Complete {
			return fmt.Errorf("project dashboard is incomplete after workspace reconciliation: %s", layout.Root)
		}
		manifest.Dashboard.GeneratedAt = generatedAt
	} else {
		manifest.Dashboard = validProjectionStatus(layout.Root, manifest.Dashboard)
	}
	return writeOutputManifestAtomic(layout.Manifest, manifest)
}

func filterWorkspaceAPICatalog(projectPath string, catalog APICatalogRecord) APICatalogRecord {
	filtered := APICatalogRecord{SchemaVersion: catalog.SchemaVersion, Generated: catalog.Generated, Root: filepath.ToSlash(projectPath), Endpoints: []APIEndpointRecord{}}
	for _, endpoint := range catalog.Endpoints {
		if endpoint.ProviderProject == projectPath {
			filtered.Endpoints = append(filtered.Endpoints, endpoint)
			continue
		}
		projectEndpoint := endpoint
		projectEndpoint.Consumers = nil
		projectEndpoint.Mismatches = nil
		projectRelevant := false
		for _, consumer := range endpoint.Consumers {
			if consumer.Project == projectPath {
				projectEndpoint.Consumers = append(projectEndpoint.Consumers, consumer)
				projectRelevant = true
			}
		}
		for _, mismatch := range endpoint.Mismatches {
			if mismatch.ConsumerProject == projectPath {
				projectEndpoint.Mismatches = append(projectEndpoint.Mismatches, mismatch)
				projectRelevant = true
			}
		}
		if projectRelevant {
			for _, mismatch := range endpoint.Mismatches {
				if mismatch.ConsumerProject == "" {
					projectEndpoint.Mismatches = append(projectEndpoint.Mismatches, mismatch)
				}
			}
			filtered.Endpoints = append(filtered.Endpoints, projectEndpoint)
		}
	}
	SortAPICatalog(&filtered)
	return filtered
}

func withReconciledAPICatalogFreshness(freshness ArtifactFreshnessIndex, generated string) ArtifactFreshnessIndex {
	record := ArtifactFreshnessRecord{
		Artifact: "index/api-catalog.json", GeneratedAt: generated, GoreGraphVersion: freshness.GoreGraphVersion,
		Schema: SchemaVersion, SourceFingerprint: freshness.SourceFingerprint, Stale: false,
		Reason: "generated from reconciled workspace provider and consumer indexes",
	}
	replaced := false
	for index := range freshness.Artifacts {
		if freshness.Artifacts[index].Artifact == record.Artifact {
			freshness.Artifacts[index] = record
			replaced = true
			break
		}
	}
	if !replaced {
		freshness.Artifacts = append(freshness.Artifacts, record)
	}
	sort.Slice(freshness.Artifacts, func(left, right int) bool {
		return freshness.Artifacts[left].Artifact < freshness.Artifacts[right].Artifact
	})
	return freshness
}

func refreshProjectAPICatalogFreshness(layout OutputLayout, generated string) error {
	var freshness ArtifactFreshnessIndex
	if err := readWorkspaceJSON(layout.Index("freshness.json"), &freshness); err != nil {
		return err
	}
	freshness = withReconciledAPICatalogFreshness(freshness, generated)
	return writeJSON(layout.Index("freshness.json"), freshness)
}

// WorkspaceRoot returns the workspace root that would be used for a project.
func WorkspaceRoot(root string, cfg config.Config) (string, bool, error) {
	currentAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false, err
	}
	return resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
}

func resolveWorkspaceRoot(currentAbs, override string) (string, bool, error) {
	if override != "" {
		root := override
		if !filepath.IsAbs(root) {
			root = filepath.Join(currentAbs, root)
		}
		resolved, err := filepath.Abs(root)
		if err != nil {
			return "", false, err
		}
		return resolved, true, nil
	}

	bestRoot := ""
	bestScore := 0
	for dir := currentAbs; dir != "." && dir != ""; dir = filepath.Dir(dir) {
		if score := workspaceRootScore(dir); score > bestScore {
			bestRoot = dir
			bestScore = score
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	if bestRoot != "" {
		return bestRoot, true, nil
	}
	return "", false, nil
}

func workspaceRootScore(dir string) int {
	score := 0
	if workspaceFileExists(filepath.Join(dir, ".goregraph-workspace.yml")) {
		score += 1000
	}
	if workspaceFileExists(NewWorkspaceOutputLayout(filepath.Join(dir, ".goregraph-workspace")).Index("registry.json")) {
		score += 10
	}
	if info, err := os.Stat(filepath.Join(dir, ".goregraph-workspace")); err == nil && info.IsDir() {
		score += 5
	}
	frontendGroups, backendGroups := workspaceGroupCounts(dir)
	switch {
	case frontendGroups > 0 && backendGroups > 0:
		score += 100 + frontendGroups + backendGroups
	case frontendGroups > 0 || backendGroups > 0:
		score += frontendGroups + backendGroups
	}
	return score
}

func workspaceGroupCounts(dir string) (int, int) {
	frontendGroups := 0
	backendGroups := 0
	for _, name := range workspaceGroupDirs {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !info.IsDir() {
			continue
		}
		switch name {
		case "frontend", "frontends":
			frontendGroups++
		case "microservices", "services", "backends":
			backendGroups++
		}
	}
	return frontendGroups, backendGroups
}

func discoverWorkspaceProjects(workspaceRoot, currentAbs, defaultOutput string) ([]WorkspaceProjectRecord, error) {
	projectsByPath := map[string]WorkspaceProjectRecord{}
	for _, group := range workspaceGroupDirs {
		groupPath := filepath.Join(workspaceRoot, group)
		entries, err := os.ReadDir(groupPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			abs := filepath.Join(groupPath, entry.Name())
			addWorkspaceProject(projectsByPath, workspaceRoot, currentAbs, abs, group, defaultOutput)
		}
	}

	entries, err := os.ReadDir(workspaceRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || isWorkspaceGroup(entry.Name()) || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			abs := filepath.Join(workspaceRoot, entry.Name())
			if hasProjectMarker(abs, defaultOutput) {
				addWorkspaceProject(projectsByPath, workspaceRoot, currentAbs, abs, "", defaultOutput)
			}
		}
	}

	projects := make([]WorkspaceProjectRecord, 0, len(projectsByPath))
	for _, project := range projectsByPath {
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Path < projects[j].Path })
	return projects, nil
}

func addWorkspaceProject(projects map[string]WorkspaceProjectRecord, workspaceRoot, currentAbs, abs, group, defaultOutput string) {
	rel := workspaceRel(workspaceRoot, abs)
	outputDir := projectOutputDir(abs, defaultOutput)
	indexed := validProjectOutput(filepath.Join(abs, outputDir))
	status := "not_indexed"
	if indexed {
		status = "indexed"
	}
	if samePath(abs, currentAbs) {
		status = "current"
		indexed = true
	}
	kind := workspaceProjectKind(group, abs)
	service := workspaceProjectService(group, abs)
	if service == "" && kind == "backend" {
		service = filepath.Base(abs)
	}
	projects[rel] = WorkspaceProjectRecord{
		Name:        filepath.Base(abs),
		Path:        rel,
		AbsPath:     filepath.ToSlash(abs),
		Kind:        kind,
		BuildSystem: workspaceProjectBuildSystem(abs),
		TestRunner:  workspaceProjectTestRunner(abs),
		Service:     service,
		Indexed:     indexed,
		Status:      status,
		OutputDir:   outputDir,
	}
}

func validProjectOutput(out string) bool {
	manifest := readCurrentOutputManifest(NewProjectOutputLayout(out).Manifest)
	return manifest.Scope == "project" && manifest.Index.Complete
}

func workspaceProjectBuildSystem(abs string) string {
	switch {
	case workspaceFileExists(filepath.Join(abs, "pom.xml")):
		return "maven"
	case workspaceFileExists(filepath.Join(abs, "build.gradle")) || workspaceFileExists(filepath.Join(abs, "build.gradle.kts")):
		return "gradle"
	case workspaceFileExists(filepath.Join(abs, "package.json")):
		return "node"
	case workspaceFileExists(filepath.Join(abs, "go.mod")):
		return "go"
	default:
		return ""
	}
}

func workspaceProjectTestRunner(abs string) string {
	for _, name := range []string{"playwright.config.ts", "playwright.config.js", "playwright.config.mjs"} {
		if workspaceFileExists(filepath.Join(abs, name)) {
			return "playwright"
		}
	}
	for _, name := range []string{"vitest.config.ts", "vitest.config.js", "vitest.config.mts"} {
		if workspaceFileExists(filepath.Join(abs, name)) {
			return "vitest"
		}
	}
	for _, name := range []string{"jest.config.ts", "jest.config.js", "jest.config.cjs"} {
		if workspaceFileExists(filepath.Join(abs, name)) {
			return "jest"
		}
	}
	body, err := os.ReadFile(filepath.Join(abs, "package.json"))
	if err != nil {
		return ""
	}
	text := strings.ToLower(string(body))
	switch {
	case strings.Contains(text, "@playwright/test"):
		return "playwright"
	case strings.Contains(text, "vitest"):
		return "vitest"
	case strings.Contains(text, "jest"):
		return "jest"
	default:
		return ""
	}
}

func projectOutputDir(abs, fallback string) string {
	cfg, err := config.Load(abs)
	if err == nil && cfg.OutputDir != "" {
		return cfg.OutputDir
	}
	if fallback != "" {
		return fallback
	}
	return "goregraph-out"
}

func workspaceProjectKind(group, abs string) string {
	switch group {
	case "frontend", "frontends":
		return "frontend"
	case "microservices", "services", "backends":
		return "backend"
	}
	if workspaceFileExists(filepath.Join(abs, "package.json")) {
		return "frontend"
	}
	if workspaceFileExists(filepath.Join(abs, "pom.xml")) || workspaceFileExists(filepath.Join(abs, "build.gradle")) || workspaceFileExists(filepath.Join(abs, "build.gradle.kts")) || workspaceFileExists(filepath.Join(abs, "go.mod")) || workspaceFileExists(filepath.Join(abs, "Cargo.toml")) || workspaceFileExists(filepath.Join(abs, "composer.json")) || workspaceFileExists(filepath.Join(abs, "pyproject.toml")) || workspaceFileExists(filepath.Join(abs, "requirements.txt")) || workspaceFileExists(filepath.Join(abs, "setup.py")) {
		return "backend"
	}
	return "project"
}

func workspaceProjectService(group, abs string) string {
	switch group {
	case "microservices", "services", "backends":
		return filepath.Base(abs)
	default:
		return ""
	}
}

func isWorkspaceGroup(name string) bool {
	for _, group := range workspaceGroupDirs {
		if name == group {
			return true
		}
	}
	return false
}

func hasProjectMarker(abs, outputDir string) bool {
	for _, name := range []string{"package.json", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "go.mod", "pyproject.toml", "requirements.txt", "setup.py", "Cargo.toml", "composer.json", "goregraph.yml"} {
		if workspaceFileExists(filepath.Join(abs, name)) {
			return true
		}
	}
	return workspaceFileExists(filepath.Join(abs, outputDir, "manifest.json"))
}

func loadWorkspaceIndexes(projects []WorkspaceProjectRecord) ([]workspaceIndexProject, error) {
	var result []workspaceIndexProject
	for _, project := range projects {
		if !project.Indexed {
			continue
		}
		out := filepath.Join(project.AbsPath, project.OutputDir)
		if !workspaceFileExists(filepath.Join(out, "manifest.json")) {
			continue
		}
		layout := NewProjectOutputLayout(out)
		loaded := workspaceIndexProject{record: project}
		loadSymbolFact := func(name string, dest any, reset func()) {
			err := readWorkspaceJSON(layout.Index(name), dest)
			if err == nil {
				return
			}
			if os.IsNotExist(err) {
				loaded.missingFacts = append(loaded.missingFacts, name)
				return
			}
			reset()
			loaded.loadFailures = append(loaded.loadFailures, project.Path+"/"+name+": "+err.Error())
		}
		loadSymbolFact("symbols-full.json", &loaded.symbols, func() {
			loaded.symbols = nil
		})
		loadSymbolFact("relations-full.json", &loaded.relations, func() {
			loaded.relations = nil
		})
		loadSymbolFact("callgraph.json", &loaded.callGraph, func() {
			loaded.callGraph = CallGraphRecord{}
		})
		loadSymbolFact("maven-graph.json", &loaded.maven, func() {
			loaded.maven = MavenGraphRecord{}
		})
		loadSymbolFact("package-graph.json", &loaded.packages, func() {
			loaded.packages = PackageGraphRecord{}
		})
		loadSymbolFact("evidence.json", &loaded.evidence, func() {
			loaded.evidence = nil
		})
		if err := readWorkspaceJSON(layout.Index("routes.json"), &loaded.routes); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err := readWorkspaceJSON(layout.Index("relations.json"), &loaded.legacyRelations); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err := readWorkspaceJSON(layout.Index("api-contracts.json"), &loaded.contracts); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err := readWorkspaceJSON(layout.Index("service-dependencies.json"), &loaded.dependencies); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for i := range loaded.dependencies {
			if loaded.dependencies[i].FromProject == "" {
				loaded.dependencies[i].FromProject = project.Path
			}
		}
		loadSymbolFact("flows.json", &loaded.codeFlows, func() {
			loaded.codeFlows = nil
		})
		var spring SpringIndex
		if err := readWorkspaceJSON(layout.Index("spring.json"), &spring); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		loaded.spring = spring
		loaded.endpoints = spring.Endpoints
		loadSymbolFact("endpoint-flows.json", &loaded.endpointFlows, func() {
			loaded.endpointFlows = nil
		})
		if err := readWorkspaceJSON(layout.Index("test-map.json"), &loaded.testMap); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		loadSymbolFact("capabilities.json", &loaded.capabilities, func() {
			loaded.capabilities = nil
		})
		for i := range loaded.capabilities {
			loaded.capabilities[i].Project = project.Path
		}
		if err := readWorkspaceJSON(layout.Index("diagnostics-canonical.json"), &loaded.diagnostics); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err := readWorkspaceJSON(layout.Index("diagnostic-families.json"), &loaded.diagnosticFamilies); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err := readWorkspaceJSON(layout.Index("freshness.json"), &loaded.freshness); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		result = append(result, loaded)
	}
	return result, nil
}

func workspaceProjectNamespaces(indexed []workspaceIndexProject) []WorkspaceProjectNamespaceRecord {
	records := make([]WorkspaceProjectNamespaceRecord, 0)
	seen := map[string]bool{}
	for _, project := range indexed {
		for _, symbol := range project.symbols {
			if !isWorkspaceProductionNamespacePath(project.record, symbol.File, symbol.Kind) {
				continue
			}
			namespace := strings.TrimSpace(firstNonEmpty(symbol.Package, symbol.WorkspacePackage, symbol.Module))
			if namespace == "" {
				continue
			}
			record := WorkspaceProjectNamespaceRecord{
				Project:    project.record.Path,
				Namespace:  namespace,
				Language:   symbol.Language,
				Source:     "production_package",
				Confidence: "EXTRACTED",
			}
			key := record.Project + "\x00" + record.Namespace + "\x00" + record.Language
			if seen[key] {
				continue
			}
			seen[key] = true
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Project != records[j].Project {
			return records[i].Project < records[j].Project
		}
		if records[i].Namespace != records[j].Namespace {
			return records[i].Namespace < records[j].Namespace
		}
		return records[i].Language < records[j].Language
	})
	return records
}

func isWorkspaceProductionNamespacePath(project WorkspaceProjectRecord, file, kind string) bool {
	if strings.TrimSpace(file) == "" || isWorkspaceTestNamespacePath(file, kind) || isLowSignalCodeFile(file) {
		return false
	}
	normalized := workspaceNamespaceEvidencePath(file)
	outputDir := workspaceNamespaceEvidencePath(project.OutputDir)
	if outputDir != "" && (normalized == outputDir || strings.HasPrefix(normalized, outputDir+"/")) {
		return false
	}
	for _, segment := range strings.Split(normalized, "/") {
		switch segment {
		case ".next", ".nuxt", ".svelte-kit", "target", "out", "bin", "obj":
			return false
		}
	}
	return true
}

func workspaceNamespaceEvidencePath(value string) string {
	value = strings.ToLower(filepath.ToSlash(strings.TrimSpace(value)))
	value = strings.TrimPrefix(value, "./")
	return strings.Trim(value, "/")
}

func isWorkspaceTestNamespacePath(file, kind string) bool {
	if strings.EqualFold(strings.TrimSpace(kind), "test") || isJavaTestPath(file) {
		return true
	}
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(file)))
	base := filepath.Base(normalized)
	if strings.HasSuffix(base, "_test.go") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == "test" || segment == "tests" || segment == "__tests__" {
			return true
		}
	}
	return false
}

func loadWorkspaceAgentContextIndexes(projects []workspaceIndexProject) ([]AgentContextIndexRecord, error) {
	var indexes []AgentContextIndexRecord
	for _, project := range projects {
		out := filepath.Join(project.record.AbsPath, project.record.OutputDir)
		layout := NewProjectOutputLayout(out)
		manifest := readCurrentOutputManifest(layout.Manifest)
		if manifest.Scope != "project" || !currentAgentProjectionStatus(layout.Root, manifest.Agent).Complete {
			continue
		}
		var index AgentContextIndexRecord
		if err := readWorkspaceJSON(layout.Agent("context-index.json"), &index); err != nil {
			return nil, err
		}
		index.Root = project.record.Path
		indexes = append(indexes, index)
	}
	return indexes, nil
}

func BuildWorkspaceAgentContextIndex(
	registry WorkspaceRegistryRecord,
	projectIndexes []AgentContextIndexRecord,
	matches []WorkspaceContractMatchRecord,
	dossiers []FeatureDossierRecord,
	traces WorkspaceEndpointTraceIndexRecord,
	catalog APICatalogRecord,
	generated string,
) AgentContextIndexRecord {
	builder := newWorkspaceAgentContextBuilder(registry)
	for _, index := range projectIndexes {
		builder.mergeProjectIndex(index)
	}
	builder.addCatalogFacts(catalog)
	builder.indexTraceHandlerLocations(traces)
	builder.addContractEdges(matches)
	builder.addDossierFacts(dossiers)
	builder.addTraceTransitions(traces)
	return builder.index(generated)
}

func (builder *workspaceAgentContextBuilder) addCatalogFacts(catalog APICatalogRecord) {
	endpoints := append([]APIEndpointRecord(nil), catalog.Endpoints...)
	sort.Slice(endpoints, func(i, j int) bool {
		return compactCatalogEndpointKey(endpoints[i]) < compactCatalogEndpointKey(endpoints[j])
	})
	for _, endpoint := range endpoints {
		project := contextPathKey(endpoint.ProviderProject)
		if !builder.projects[project] {
			continue
		}
		method := strings.ToUpper(compactCatalogValue(endpoint.HTTPMethod))
		endpointPath := compactCatalogValue(endpoint.Path)
		name := strings.TrimSpace(method + " " + endpointPath)
		file := workspaceAgentFile(project, endpoint.File)
		securityLabels := compactCatalogSecurityLabels(endpoint.Security)
		consumers, omittedConsumers := selectCompactCatalogConsumers(endpoint.Consumers, builder.projects)

		summaryParts := []string{
			"provider " + compactCatalogValue(firstNonEmpty(endpoint.ProviderService, project)),
		}
		if handler := compactCatalogValue(endpoint.Handler); handler != "" {
			summaryParts = append(summaryParts, "handler "+handler)
		}
		if len(securityLabels) > 0 {
			summaryParts = append(summaryParts, "security "+strings.Join(securityLabels, ", "))
		}
		if requestType := compactCatalogValue(endpoint.RequestType); requestType != "" {
			summaryParts = append(summaryParts, "request "+requestType)
		}
		if responseType := compactCatalogValue(endpoint.ResponseType); responseType != "" {
			summaryParts = append(summaryParts, "response "+responseType)
		}
		if omittedConsumers > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d consumer call sites omitted", omittedConsumers))
		}

		endpointID := builder.addFact(AgentContextFactRecord{
			ID: StableWorkspaceID(
				"agent-context-fact", "api_endpoint", endpoint.ID, project, method, endpointPath,
			),
			Project: project, Kind: "api_endpoint", Name: name,
			Qualified: compactCatalogValue(endpoint.Handler), HTTPMethod: method, Path: endpointPath,
			File: file, Line: endpoint.Line, Summary: strings.Join(summaryParts, "; "),
			Confidence: string(endpoint.Confidence), EvidenceIDs: endpoint.EvidenceIDs,
			Search: compactContextSearch(
				project,
				compactCatalogValue(endpoint.ProviderService),
				method,
				endpointPath,
				compactCatalogValue(endpoint.Handler),
				file,
				strings.Join(securityLabels, " "),
				compactCatalogValue(endpoint.RequestType),
				compactCatalogValue(endpoint.ResponseType),
			),
		})
		builder.addCatalogSecurityFacts(endpointID, endpoint, project, method, endpointPath)
		builder.addCatalogConsumerFacts(endpointID, endpoint, consumers, method, endpointPath)
	}
}

func (builder *workspaceAgentContextBuilder) addCatalogSecurityFacts(
	endpointID string,
	endpoint APIEndpointRecord,
	project string,
	method string,
	endpointPath string,
) {
	security := append([]SecurityEvidenceRecord(nil), endpoint.Security...)
	sort.Slice(security, func(i, j int) bool {
		return compactCatalogSecurityKey(security[i]) < compactCatalogSecurityKey(security[j])
	})
	for _, evidence := range security {
		if !usefulCompactCatalogSecurity(evidence) {
			continue
		}
		kind := compactCatalogValue(strings.ToLower(strings.TrimSpace(evidence.Kind)))
		if kind == "" {
			kind = SecurityUnknown
		}
		file := workspaceAgentFile(project, evidence.File)
		summary := kind + " security"
		if evidence.Conflicting {
			summary += "; conflicting evidence"
		}
		securityID := builder.addFact(AgentContextFactRecord{
			ID: StableWorkspaceID(
				"agent-context-fact", "endpoint_security", endpoint.ID, kind,
				compactCatalogValue(evidence.Source), file, fmt.Sprint(evidence.Line),
			),
			Project: project, Kind: "endpoint_security", Name: kind,
			Qualified: strings.TrimSpace(method + " " + endpointPath + " " + kind),
			File:      file, Line: evidence.Line, Summary: summary,
			Confidence: string(evidence.Confidence), EvidenceIDs: evidence.EvidenceIDs,
			Search: compactContextSearch(
				method, endpointPath, kind, compactCatalogValue(evidence.Source),
			),
		})
		builder.addEdge(AgentContextEdgeRecord{
			FromFactID: endpointID, ToFactID: securityID, Kind: "requires_auth",
			Reason: "catalog security", Confidence: string(evidence.Confidence),
		})
	}
}

func (builder *workspaceAgentContextBuilder) addCatalogConsumerFacts(
	endpointID string,
	endpoint APIEndpointRecord,
	consumers []compactCatalogConsumerSelection,
	method string,
	endpointPath string,
) {
	for _, selected := range consumers {
		consumer := selected.consumer
		name := compactCatalogValue(firstNonEmpty(consumer.Caller, selected.service, contextFileStem(consumer.File)))
		consumerID := builder.addFact(AgentContextFactRecord{
			ID: StableWorkspaceID(
				"agent-context-fact", "api_consumer", consumer.ID, consumer.Project,
				consumer.File, fmt.Sprint(consumer.Line),
			),
			Project: consumer.Project, Kind: "api_consumer", Name: name,
			Qualified:  compactCatalogValue(firstNonEmpty(consumer.Caller, selected.service)),
			HTTPMethod: compactCatalogValue(firstNonEmpty(consumer.HTTPMethod, method)),
			Path:       compactCatalogValue(firstNonEmpty(consumer.Path, endpointPath)),
			File:       consumer.File, Line: consumer.Line,
			Summary:    "consumer service " + selected.service,
			Confidence: string(consumer.Confidence), EvidenceIDs: consumer.EvidenceIDs,
			Search: compactContextSearch(
				consumer.Project, selected.service, name, consumer.File, method, endpointPath,
			),
		})
		builder.addEdge(AgentContextEdgeRecord{
			FromFactID: consumerID, ToFactID: endpointID, Kind: "consumes_endpoint",
			Reason: "catalog consumer", Confidence: string(consumer.Confidence),
		})
	}
}

type workspaceAgentContextBuilder struct {
	registry         WorkspaceRegistryRecord
	projects         map[string]bool
	factsByID        map[string]AgentContextFactRecord
	edgesByID        map[string]AgentContextEdgeRecord
	coverageByKey    map[string]AgentContextCoverageRecord
	projectFactIDs   map[string]map[string]string
	handlerLocations map[string][]workspaceAgentHandlerLocation
	handlerFactIDs   map[string]string
}

type workspaceAgentHandlerLocation struct {
	file string
	line int
}

func newWorkspaceAgentContextBuilder(registry WorkspaceRegistryRecord) *workspaceAgentContextBuilder {
	projects := map[string]bool{}
	for _, project := range registry.Projects {
		if project.Indexed {
			projects[project.Path] = true
		}
	}
	return &workspaceAgentContextBuilder{
		registry:         registry,
		projects:         projects,
		factsByID:        map[string]AgentContextFactRecord{},
		edgesByID:        map[string]AgentContextEdgeRecord{},
		coverageByKey:    map[string]AgentContextCoverageRecord{},
		projectFactIDs:   map[string]map[string]string{},
		handlerLocations: map[string][]workspaceAgentHandlerLocation{},
		handlerFactIDs:   map[string]string{},
	}
}

func (builder *workspaceAgentContextBuilder) mergeProjectIndex(index AgentContextIndexRecord) {
	project := contextPathKey(index.Root)
	if project == "" || !builder.projects[project] {
		return
	}
	if builder.projectFactIDs[project] == nil {
		builder.projectFactIDs[project] = map[string]string{}
	}
	for _, fact := range index.Facts {
		originalID := fact.ID
		if originalID == "" {
			continue
		}
		fact.ID = workspaceAgentFactID(project, fact.ID)
		fact.Project = project
		fact.File = workspaceAgentFile(project, fact.File)
		builder.projectFactIDs[project][originalID] = fact.ID
		builder.addFact(fact)
	}
	for _, edge := range index.Edges {
		fromID := builder.projectFactIDs[project][edge.FromFactID]
		toID := builder.projectFactIDs[project][edge.ToFactID]
		if fromID == "" || toID == "" {
			continue
		}
		if edge.ID == "" {
			continue
		}
		edge.ID = workspaceAgentFactID(project, edge.ID)
		edge.Project = project
		edge.FromFactID = fromID
		edge.ToFactID = toID
		edge.File = workspaceAgentFile(project, edge.File)
		builder.addEdge(edge)
	}
	for _, coverage := range index.Coverage {
		if !contextAgentCapability(CapabilityID(coverage.Capability)) {
			continue
		}
		coverage.Project = project
		key := project + "\x00" + coverage.Capability
		if existing, ok := builder.coverageByKey[key]; !ok ||
			contextCoverageRank(coverage.Coverage) > contextCoverageRank(existing.Coverage) ||
			contextCoverageRank(coverage.Coverage) == contextCoverageRank(existing.Coverage) &&
				coverage.Reason < existing.Reason {
			builder.coverageByKey[key] = coverage
		}
	}
}

func (builder *workspaceAgentContextBuilder) addContractEdges(matches []WorkspaceContractMatchRecord) {
	for _, match := range matches {
		if match.BackendProject == "" || match.Issue != contractIssueMatched ||
			match.Confidence != "" && !strings.EqualFold(match.Confidence, "RESOLVED") {
			continue
		}
		fromID := builder.findFact(
			match.APIProject,
			"api_contract",
			match.APIHTTPMethod,
			match.APIPath,
			match.APIFile,
			match.APILine,
			match.APICaller,
		)
		toID := builder.findCompatibleFact(
			match.BackendProject,
			"route",
			firstNonEmpty(match.BackendHTTPMethod, match.APIHTTPMethod),
			firstNonEmpty(match.BackendPath, match.APIPath),
			match.BackendFile,
			match.BackendLine,
			match.BackendHandler,
		)
		if fromID == "" || toID == "" {
			continue
		}
		builder.addEdge(AgentContextEdgeRecord{
			Project:     match.APIProject,
			FromFactID:  fromID,
			ToFactID:    toID,
			Kind:        "http_contract",
			File:        workspaceAgentFile(match.APIProject, match.APIFile),
			Line:        match.APILine,
			Reason:      match.Reason,
			Confidence:  match.Confidence,
			EvidenceIDs: match.ResolutionEvidence,
		})
	}
}

func (builder *workspaceAgentContextBuilder) indexTraceHandlerLocations(traces WorkspaceEndpointTraceIndexRecord) {
	for _, trace := range traces.Traces {
		for _, step := range trace.Steps {
			if step.Kind != "backend_handler" || step.Project == "" {
				continue
			}
			handler := firstNonEmpty(step.Symbol, step.Label)
			key := workspaceAgentHandlerKey(step.Project, handler)
			if key == "" {
				continue
			}
			location := workspaceAgentHandlerLocation{
				file: workspaceAgentFile(step.Project, step.File),
				line: step.Line,
			}
			duplicate := false
			for _, existing := range builder.handlerLocations[key] {
				if existing == location {
					duplicate = true
					break
				}
			}
			if !duplicate {
				builder.handlerLocations[key] = append(builder.handlerLocations[key], location)
			}
		}
	}
}

func (builder *workspaceAgentContextBuilder) addDossierFacts(dossiers []FeatureDossierRecord) {
	persistenceFactIDs := builder.addDossierPersistenceFacts(dossiers)
	for _, dossier := range dossiers {
		handlerID := ""
		if dossier.BackendHandler != "" && builder.projects[dossier.BackendProject] {
			handlerID = builder.resolveHandler(
				dossier.BackendProject,
				dossier.BackendHandler,
				"",
				0,
				dossier.Route,
				dossier.Confidence,
			)
		}
		for _, auth := range dossier.Auth {
			project := firstNonEmpty(dossier.BackendProject, dossier.FrontendProject)
			if !builder.projects[project] {
				continue
			}
			authID := builder.addFact(AgentContextFactRecord{
				Project:    project,
				Kind:       "authentication",
				Name:       firstNonEmpty(auth.Expression, auth.Kind),
				File:       workspaceAgentFile(project, auth.File),
				Line:       auth.Line,
				Summary:    auth.Kind,
				Confidence: auth.Confidence,
				Search:     compactContextSearch(dossier.Route, auth.Kind, auth.Expression),
			})
			builder.addDossierEdge(handlerID, authID, "authentication", dossier.Confidence)
		}
		for _, persistence := range dossier.PersistencePath {
			if !builder.projects[dossier.BackendProject] {
				continue
			}
			baseKey := workspacePersistenceBaseKey(dossier.BackendProject, persistence)
			targetKey := workspacePersistenceTargetKey(persistence)
			persistenceID := persistenceFactIDs[baseKey][targetKey]
			builder.addDossierEdge(handlerID, persistenceID, "persistence", dossier.Confidence)
		}
		for _, test := range dossier.Tests {
			project := firstNonEmpty(dossier.BackendProject, dossier.FrontendProject)
			if !builder.projects[project] {
				continue
			}
			qualified := qualifiedContextName(test.TestClass, test.TestMethod)
			testID := builder.findFact(project, "test", "", "", test.TestFile, test.Line, qualified)
			if testID == "" {
				testID = builder.addFact(AgentContextFactRecord{
					Project:    project,
					Kind:       "test",
					Name:       firstNonEmpty(test.TestMethod, test.TestCase, test.TestClass, contextFileStem(test.TestFile)),
					Qualified:  qualified,
					File:       workspaceAgentFile(project, test.TestFile),
					Line:       test.Line,
					Summary:    "tests " + dossier.Route,
					Confidence: test.Confidence,
					Search:     compactContextSearch(dossier.Route, qualified, test.TestCase),
				})
			}
			if handlerID != "" {
				builder.addEdge(AgentContextEdgeRecord{
					Project:    project,
					FromFactID: testID,
					ToFactID:   handlerID,
					Kind:       "test_target",
					File:       workspaceAgentFile(project, test.TestFile),
					Line:       test.Line,
					Confidence: test.Confidence,
				})
			}
		}
	}
}

type workspaceDossierPersistenceOccurrence struct {
	project     string
	route       string
	persistence PersistenceStepRecord
}

type workspaceDossierPersistenceGroup struct {
	project     string
	name        string
	file        string
	line        int
	occurrences map[string][]workspaceDossierPersistenceOccurrence
}

func (builder *workspaceAgentContextBuilder) addDossierPersistenceFacts(
	dossiers []FeatureDossierRecord,
) map[string]map[string]string {
	copiedFacts := make([]AgentContextFactRecord, 0, len(builder.factsByID))
	for _, fact := range builder.factsByID {
		if fact.Kind == "persistence" {
			copiedFacts = append(copiedFacts, fact)
		}
	}

	groups := map[string]*workspaceDossierPersistenceGroup{}
	for _, dossier := range dossiers {
		if !builder.projects[dossier.BackendProject] {
			continue
		}
		for _, persistence := range dossier.PersistencePath {
			baseKey := workspacePersistenceBaseKey(dossier.BackendProject, persistence)
			group := groups[baseKey]
			if group == nil {
				name := workspacePersistenceName(persistence)
				group = &workspaceDossierPersistenceGroup{
					project:     contextPathKey(dossier.BackendProject),
					name:        name,
					file:        workspaceAgentFile(dossier.BackendProject, persistence.File),
					line:        persistence.Line,
					occurrences: map[string][]workspaceDossierPersistenceOccurrence{},
				}
				groups[baseKey] = group
			}
			targetKey := workspacePersistenceTargetKey(persistence)
			group.occurrences[targetKey] = append(
				group.occurrences[targetKey],
				workspaceDossierPersistenceOccurrence{
					project:     dossier.BackendProject,
					route:       dossier.Route,
					persistence: persistence,
				},
			)
		}
	}

	baseKeys := make([]string, 0, len(groups))
	for baseKey := range groups {
		baseKeys = append(baseKeys, baseKey)
	}
	sort.Strings(baseKeys)

	factIDs := map[string]map[string]string{}
	for _, baseKey := range baseKeys {
		group := groups[baseKey]
		targetKeys := make([]string, 0, len(group.occurrences))
		for targetKey := range group.occurrences {
			targetKeys = append(targetKeys, targetKey)
		}
		sort.Strings(targetKeys)

		copiedID := uniqueCopiedPersistenceFact(copiedFacts, *group)
		factIDs[baseKey] = map[string]string{}
		for targetIndex, targetKey := range targetKeys {
			occurrences := group.occurrences[targetKey]
			sort.Slice(occurrences, func(i, j int) bool {
				return workspacePersistenceOccurrenceKey(occurrences[i]) <
					workspacePersistenceOccurrenceKey(occurrences[j])
			})
			representative := occurrences[0]
			persistenceID := ""
			if targetIndex == 0 && copiedID != "" {
				persistenceID = copiedID
			} else {
				persistence := representative.persistence
				persistenceID = builder.addFact(AgentContextFactRecord{
					ID: StableWorkspaceID(
						"agent-context-fact",
						representative.project,
						"persistence",
						group.name,
						persistence.Entity,
						persistence.Table,
						persistence.File,
						fmt.Sprint(persistence.Line),
					),
					Project:   representative.project,
					Kind:      "persistence",
					Name:      firstNonEmpty(group.name, persistence.Entity, persistence.Table),
					Qualified: group.name,
					File:      workspaceAgentFile(representative.project, persistence.File),
					Line:      persistence.Line,
				})
			}
			for _, occurrence := range occurrences {
				persistence := occurrence.persistence
				builder.addFact(AgentContextFactRecord{
					ID:         persistenceID,
					Summary:    strings.TrimSpace("entity " + persistence.Entity + " table " + persistence.Table),
					Confidence: persistence.Confidence,
					Search: compactContextSearch(
						occurrence.route,
						group.name,
						persistence.Entity,
						persistence.Table,
					),
				})
			}
			factIDs[baseKey][targetKey] = persistenceID
		}
	}
	return factIDs
}

func uniqueCopiedPersistenceFact(
	copiedFacts []AgentContextFactRecord,
	group workspaceDossierPersistenceGroup,
) string {
	var candidates []string
	for _, fact := range copiedFacts {
		if fact.Project != group.project || fact.Kind != "persistence" ||
			fact.File != group.file || fact.Line != group.line ||
			!workspaceAgentExactIdentity(fact, group.name) {
			continue
		}
		candidates = append(candidates, fact.ID)
	}
	sort.Strings(candidates)
	if len(candidates) != 1 {
		return ""
	}
	return candidates[0]
}

func workspacePersistenceOccurrenceKey(occurrence workspaceDossierPersistenceOccurrence) string {
	persistence := occurrence.persistence
	return strings.Join([]string{
		contextPathKey(occurrence.project),
		occurrence.route,
		persistence.Repository,
		persistence.Method,
		persistence.Entity,
		persistence.Table,
		persistence.File,
		fmt.Sprint(persistence.Line),
		persistence.Source,
		persistence.Confidence,
	}, "\x00")
}

func workspacePersistenceBaseKey(project string, persistence PersistenceStepRecord) string {
	return strings.Join([]string{
		contextPathKey(project),
		contextLabelKey(workspacePersistenceName(persistence)),
		workspaceAgentFile(project, persistence.File),
		fmt.Sprint(persistence.Line),
	}, "\x00")
}

func workspacePersistenceName(persistence PersistenceStepRecord) string {
	return strings.Trim(strings.TrimSpace(persistence.Repository)+"."+strings.TrimSpace(persistence.Method), ".")
}

func workspacePersistenceTargetKey(persistence PersistenceStepRecord) string {
	return contextLabelKey(persistence.Entity) + "\x00" + contextLabelKey(persistence.Table)
}

func (builder *workspaceAgentContextBuilder) addDossierEdge(fromID, toID, kind, confidence string) {
	if fromID == "" || toID == "" {
		return
	}
	builder.addEdge(AgentContextEdgeRecord{
		FromFactID: fromID,
		ToFactID:   toID,
		Kind:       kind,
		Confidence: confidence,
	})
}

func (builder *workspaceAgentContextBuilder) addTraceTransitions(traces WorkspaceEndpointTraceIndexRecord) {
	for _, trace := range traces.Traces {
		stepFactIDs := map[string]string{}
		stepsByID := map[string]WorkspaceEndpointTraceStepRecord{}
		for _, step := range trace.Steps {
			if !builder.projects[step.Project] {
				continue
			}
			kind := workspaceTraceFactKind(step.Kind)
			factID := ""
			switch step.Kind {
			case "api_contract", "backend_route":
				method, routePath := workspaceTraceEndpointIdentity(trace, step)
				if method != "" && routePath != "" {
					factID = builder.findFact(
						step.Project,
						kind,
						method,
						routePath,
						step.File,
						step.Line,
						firstNonEmpty(step.Symbol, step.Label),
					)
				}
			case "backend_handler":
				factID = builder.resolveHandler(
					step.Project,
					firstNonEmpty(step.Symbol, step.Label),
					step.File,
					step.Line,
					trace.Route,
					step.Confidence,
				)
			default:
				factID = builder.findFact(
					step.Project,
					kind,
					"",
					"",
					step.File,
					step.Line,
					firstNonEmpty(step.Symbol, step.Label),
				)
			}
			if factID == "" && step.Kind != "api_contract" && step.Kind != "backend_route" {
				factID = builder.addFact(AgentContextFactRecord{
					Project:    step.Project,
					Kind:       kind,
					Name:       step.Label,
					Qualified:  step.Symbol,
					File:       workspaceAgentFile(step.Project, step.File),
					Line:       step.Line,
					Confidence: step.Confidence,
					Search:     compactContextSearch(trace.Route, step.Label, step.Symbol),
				})
			}
			stepFactIDs[step.ID] = factID
			stepsByID[step.ID] = step
		}
		for _, edge := range trace.Edges {
			fromID := stepFactIDs[edge.From]
			toID := stepFactIDs[edge.To]
			if fromID == "" || toID == "" {
				continue
			}
			target := stepsByID[edge.To]
			builder.addEdge(AgentContextEdgeRecord{
				FromFactID: fromID,
				ToFactID:   toID,
				Kind:       firstNonEmpty(edge.Kind, "trace"),
				File:       workspaceAgentFile(target.Project, target.File),
				Line:       target.Line,
				Reason:     edge.Direction,
			})
		}
	}
}

func workspaceTraceEndpointIdentity(trace WorkspaceEndpointTraceRecord, step WorkspaceEndpointTraceStepRecord) (string, string) {
	method := strings.ToUpper(strings.TrimSpace(step.HTTPMethod))
	routePath := normalizeOptionalContextPath(step.Path)
	if step.Kind == "api_contract" {
		method = firstNonEmpty(method, strings.ToUpper(strings.TrimSpace(trace.Method)))
		routePath = firstNonEmpty(routePath, normalizeOptionalContextPath(trace.Path))
		return method, routePath
	}
	if method != "" && routePath != "" {
		return method, routePath
	}
	parts := strings.Fields(strings.TrimSpace(step.Label))
	if len(parts) < 2 {
		return method, routePath
	}
	if method == "" {
		method = strings.ToUpper(parts[0])
	}
	if routePath == "" {
		routePath = normalizeOptionalContextPath(parts[1])
	}
	return method, routePath
}

func (builder *workspaceAgentContextBuilder) resolveHandler(
	project string,
	handler string,
	file string,
	line int,
	route string,
	confidence string,
) string {
	key := workspaceAgentHandlerKey(project, handler)
	if key == "" {
		return ""
	}
	if id := builder.handlerFactIDs[key]; id != "" {
		return id
	}
	file = workspaceAgentFile(project, file)
	if file == "" && line == 0 {
		if locations := builder.handlerLocations[key]; len(locations) == 1 {
			file = locations[0].file
			line = locations[0].line
		}
	}
	if candidates := builder.nearestHandlerSymbolFacts(project, handler, file, line); len(candidates) == 1 {
		builder.handlerFactIDs[key] = candidates[0]
		return candidates[0]
	} else if len(candidates) > 1 {
		return ""
	}
	if candidates := builder.exactHandlerFacts(project, "route", handler, file, line); len(candidates) == 1 {
		builder.handlerFactIDs[key] = candidates[0]
		return candidates[0]
	} else if len(candidates) > 1 {
		return ""
	}
	id := builder.addFact(AgentContextFactRecord{
		Project:    project,
		Kind:       "backend_handler",
		Name:       contextSimpleName(handler),
		Qualified:  handler,
		File:       file,
		Line:       line,
		Summary:    route,
		Confidence: confidence,
		Search:     compactContextSearch(route, handler),
	})
	builder.handlerFactIDs[key] = id
	return id
}

func (builder *workspaceAgentContextBuilder) nearestHandlerSymbolFacts(project, handler, file string, line int) []string {
	var candidates []AgentContextFactRecord
	for _, fact := range builder.factsByID {
		if fact.Project != contextPathKey(project) || fact.Kind != "symbol" ||
			!workspaceAgentExactIdentity(fact, handler) {
			continue
		}
		if file != "" && fact.File != file {
			continue
		}
		candidates = append(candidates, fact)
	}
	if line > 0 && len(candidates) > 1 {
		minDistance := -1
		var nearest []AgentContextFactRecord
		for _, fact := range candidates {
			if fact.Line <= 0 {
				continue
			}
			distance := fact.Line - line
			if distance < 0 {
				distance = -distance
			}
			if minDistance < 0 || distance < minDistance {
				minDistance = distance
				nearest = []AgentContextFactRecord{fact}
			} else if distance == minDistance {
				nearest = append(nearest, fact)
			}
		}
		if minDistance >= 0 {
			candidates = nearest
		}
	}
	ids := make([]string, 0, len(candidates))
	for _, fact := range candidates {
		ids = append(ids, fact.ID)
	}
	sort.Strings(ids)
	return ids
}

func (builder *workspaceAgentContextBuilder) exactHandlerFacts(project, kind, handler, file string, line int) []string {
	var candidates []string
	for id, fact := range builder.factsByID {
		if fact.Project != contextPathKey(project) || fact.Kind != kind ||
			!workspaceAgentExactIdentity(fact, handler) {
			continue
		}
		if file != "" && fact.File != file {
			continue
		}
		if line > 0 && fact.Line != line {
			continue
		}
		candidates = append(candidates, id)
	}
	sort.Strings(candidates)
	return candidates
}

func workspaceAgentHandlerKey(project, handler string) string {
	project = contextPathKey(project)
	handler = contextLabelKey(handler)
	if project == "" || handler == "" {
		return ""
	}
	return project + "\x00" + handler
}

func workspaceAgentExactIdentity(fact AgentContextFactRecord, identity string) bool {
	identity = contextLabelKey(identity)
	if identity == "" {
		return true
	}
	for _, candidate := range []string{
		fact.Name,
		fact.Qualified,
		contextSimpleName(fact.Name),
		contextSimpleName(fact.Qualified),
	} {
		if contextLabelKey(candidate) == identity {
			return true
		}
	}
	return false
}

func workspaceTraceFactKind(kind string) string {
	switch kind {
	case "backend_route":
		return "route"
	case "backend_step":
		return "symbol"
	default:
		return kind
	}
}

func (builder *workspaceAgentContextBuilder) findFact(project, kind, method, routePath, file string, line int, label string) string {
	return builder.findFactWithPathMode(project, kind, method, routePath, file, line, label, false)
}

func (builder *workspaceAgentContextBuilder) findCompatibleFact(project, kind, method, routePath, file string, line int, label string) string {
	return builder.findFactWithPathMode(project, kind, method, routePath, file, line, label, true)
}

func (builder *workspaceAgentContextBuilder) findFactWithPathMode(
	project,
	kind,
	method,
	routePath,
	file string,
	line int,
	label string,
	compatiblePath bool,
) string {
	project = contextPathKey(project)
	file = workspaceAgentFile(project, file)
	method = strings.ToUpper(strings.TrimSpace(method))
	routePath = normalizeOptionalContextPath(routePath)
	labelKey := contextLabelKey(label)
	var candidates []string
	for id, fact := range builder.factsByID {
		if fact.Project != project || kind != "" && fact.Kind != kind {
			continue
		}
		if method != "" && !strings.EqualFold(fact.HTTPMethod, method) {
			continue
		}
		if routePath != "" {
			factPath := normalizeOptionalContextPath(fact.Path)
			if factPath == "" {
				continue
			}
			if compatiblePath {
				if !pathsCompatibleWithKnownBasePrefixes(routePath, factPath) {
					continue
				}
			} else if factPath != routePath {
				continue
			}
		}
		if file != "" && fact.File != file {
			continue
		}
		if line > 0 && fact.Line > 0 && fact.Line != line {
			continue
		}
		if labelKey != "" && !workspaceAgentExactIdentity(fact, labelKey) {
			continue
		}
		candidates = append(candidates, id)
	}
	sort.Strings(candidates)
	if len(candidates) != 1 {
		return ""
	}
	return candidates[0]
}

func (builder *workspaceAgentContextBuilder) addFact(fact AgentContextFactRecord) string {
	fact.Project = contextPathKey(fact.Project)
	fact.File = contextPathKey(fact.File)
	fact.EvidenceIDs = compactContextStrings(fact.EvidenceIDs)
	if fact.ID == "" {
		fact.ID = StableWorkspaceID(
			"agent-context-fact",
			fact.Project,
			fact.Kind,
			fact.Name,
			fact.Qualified,
			fact.HTTPMethod,
			fact.Path,
			fact.File,
			fmt.Sprint(fact.Line),
		)
	}
	if existing, ok := builder.factsByID[fact.ID]; ok {
		existing.EvidenceIDs = compactContextStrings(append(existing.EvidenceIDs, fact.EvidenceIDs...))
		existing.Search = mergeContextSearch(existing.Search, fact.Search)
		existing.Confidence = strongerContextConfidence(existing.Confidence, fact.Confidence)
		existing.Summary = deterministicContextText(existing.Summary, fact.Summary)
		builder.factsByID[fact.ID] = existing
		return fact.ID
	}
	builder.factsByID[fact.ID] = fact
	return fact.ID
}

func (builder *workspaceAgentContextBuilder) addEdge(edge AgentContextEdgeRecord) {
	from, hasFrom := builder.factsByID[edge.FromFactID]
	to, hasTo := builder.factsByID[edge.ToFactID]
	if !hasFrom || !hasTo || edge.FromFactID == edge.ToFactID {
		return
	}
	edge.Project = firstNonEmpty(edge.Project, from.Project)
	edge.File = contextPathKey(edge.File)
	edge.FromLabel = firstNonEmpty(edge.FromLabel, contextFactLabel(from))
	edge.ToLabel = firstNonEmpty(edge.ToLabel, contextFactLabel(to))
	edge.EvidenceIDs = compactContextStrings(edge.EvidenceIDs)
	if edge.ID == "" {
		edge.ID = StableWorkspaceID(
			"agent-context-edge",
			edge.FromFactID,
			edge.ToFactID,
			edge.Kind,
			edge.File,
			fmt.Sprint(edge.Line),
		)
	}
	if existing, ok := builder.edgesByID[edge.ID]; ok {
		existing.EvidenceIDs = compactContextStrings(append(existing.EvidenceIDs, edge.EvidenceIDs...))
		existing.Confidence = strongerContextConfidence(existing.Confidence, edge.Confidence)
		existing.Reason = deterministicContextText(existing.Reason, edge.Reason)
		builder.edgesByID[edge.ID] = existing
		return
	}
	builder.edgesByID[edge.ID] = edge
}

func (builder *workspaceAgentContextBuilder) index(generated string) AgentContextIndexRecord {
	facts := make([]AgentContextFactRecord, 0, len(builder.factsByID))
	for _, fact := range builder.factsByID {
		facts = append(facts, fact)
	}
	sortAgentContextFacts(facts)
	edges := make([]AgentContextEdgeRecord, 0, len(builder.edgesByID))
	for _, edge := range builder.edgesByID {
		edges = append(edges, edge)
	}
	sortAgentContextEdges(edges)
	edges = compactWorkspaceAgentContextEdges(edges)
	sortAgentContextEdges(edges)
	coverage := make([]AgentContextCoverageRecord, 0, len(builder.coverageByKey))
	for _, record := range builder.coverageByKey {
		coverage = append(coverage, record)
	}
	sort.Slice(coverage, func(i, j int) bool {
		return contextCoverageLess(coverage[i], coverage[j])
	})
	return AgentContextIndexRecord{
		SchemaVersion: SchemaVersion,
		Generated:     generated,
		Root:          builder.registry.Root,
		Facts:         facts,
		Edges:         edges,
		Coverage:      coverage,
	}
}

func compactWorkspaceAgentContextEdges(edges []AgentContextEdgeRecord) []AgentContextEdgeRecord {
	type semanticEdgeKey struct {
		fromFactID string
		toFactID   string
		kind       string
		reason     string
		confidence string
	}
	bySemantics := make(map[semanticEdgeKey]AgentContextEdgeRecord, len(edges))
	for index := range edges {
		edges[index].Project = ""
		edges[index].FromLabel = ""
		edges[index].ToLabel = ""
		edges[index].File = ""
		edges[index].Line = 0
		edges[index].Reason = compactWorkspaceAgentContextReason(edges[index].Reason)
		edges[index].EvidenceIDs = nil
		key := semanticEdgeKey{
			fromFactID: edges[index].FromFactID,
			toFactID:   edges[index].ToFactID,
			kind:       edges[index].Kind,
			reason:     edges[index].Reason,
			confidence: edges[index].Confidence,
		}
		existing, found := bySemantics[key]
		if !found || edges[index].ID < existing.ID {
			bySemantics[key] = edges[index]
		}
	}
	result := make([]AgentContextEdgeRecord, 0, len(bySemantics))
	for _, edge := range bySemantics {
		result = append(result, edge)
	}
	return result
}

func compactWorkspaceAgentContextReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "java calls method owner reference":
		return "method"
	case "java field type reference":
		return "field"
	case "extracted test HTTP request matched endpoint pattern":
		return "HTTP match"
	case "java parameter type reference":
		return "parameter"
	case "java return type reference":
		return "return"
	case "java extends type reference":
		return ""
	case "java instantiates reference":
		return "new"
	case "java implements type reference":
		return ""
	case "static module and export binding":
		return "module"
	case "javascript test calls resolved production symbol",
		"typescript test calls resolved production symbol",
		"test method calls production method":
		return "test"
	case "javascript static call match", "typescript static call match":
		return "static"
	case "javascript effect call match", "typescript effect call match":
		return "effect"
	case "javascript event handler call match", "typescript event handler call match":
		return "event"
	case "flow transition":
		return "flow"
	default:
		return strings.TrimSpace(reason)
	}
}

func workspaceAgentFactID(project, id string) string {
	return contextPathKey(project) + "#" + strings.TrimSpace(id)
}

func workspaceAgentFile(_ string, file string) string {
	file = contextPathKey(file)
	if file == "" {
		return ""
	}
	if strings.HasPrefix(file, "/") {
		return ""
	}
	if len(file) >= 3 &&
		((file[0] >= 'A' && file[0] <= 'Z') || (file[0] >= 'a' && file[0] <= 'z')) &&
		file[1] == ':' && file[2] == '/' {
		return ""
	}
	for _, segment := range strings.Split(file, "/") {
		if segment == ".." {
			return ""
		}
	}
	return file
}

func buildWorkspaceContext(registry WorkspaceRegistryRecord, indexed []workspaceIndexProject) WorkspaceContextRecord {
	var loaded []WorkspaceProjectRecord
	serviceSet := map[string]bool{}
	projectsByService := map[string]WorkspaceProjectRecord{}
	for _, project := range registry.Projects {
		if project.Service != "" {
			projectsByService[project.Service] = project
		}
	}
	for _, project := range indexed {
		loaded = append(loaded, project.record)
		if project.record.Service != "" && hasBackendRoutes(project.routes) {
			serviceSet[project.record.Service] = true
		}
	}

	referenced := map[string]int{}
	for _, project := range indexed {
		for _, contract := range project.contracts {
			if contract.ServiceCandidate != "" {
				referenced[contract.ServiceCandidate]++
			}
		}
	}
	var known []string
	for service := range serviceSet {
		known = append(known, service)
	}
	sort.Strings(known)
	var missing []string
	var referencedServices []string
	var missingDetails []WorkspaceMissingServiceRecord
	for service := range referenced {
		referencedServices = append(referencedServices, service)
	}
	sort.Strings(referencedServices)
	for service, count := range referenced {
		if !serviceSet[service] {
			missing = append(missing, service)
			detail := WorkspaceMissingServiceRecord{
				Service:   service,
				Contracts: count,
			}
			if project, ok := projectsByService[service]; ok {
				detail.Project = project.Path
				detail.Status = project.Status
			}
			missingDetails = append(missingDetails, detail)
		}
	}
	sort.Strings(missing)
	sort.Slice(missingDetails, func(i, j int) bool {
		if missingDetails[i].Contracts != missingDetails[j].Contracts {
			return missingDetails[i].Contracts > missingDetails[j].Contracts
		}
		return missingDetails[i].Service < missingDetails[j].Service
	})
	return WorkspaceContextRecord{
		Root:                  registry.Root,
		LoadedIndexes:         loaded,
		Projects:              registry.Projects,
		KnownServices:         known,
		ReferencedServices:    referencedServices,
		MissingServices:       missing,
		MissingServiceDetails: missingDetails,
	}
}

func buildWorkspaceContractMatches(projects []workspaceIndexProject) []WorkspaceContractMatchRecord {
	var backendRoutes []workspaceBackendRoute
	knownServices := map[string]bool{}
	seenBackendRoutes := map[string]bool{}
	for _, project := range projects {
		for _, route := range project.routes {
			if route.Kind != "backend" {
				continue
			}
			appendWorkspaceBackendRoute(&backendRoutes, seenBackendRoutes, knownServices, workspaceBackendRoute{project: project.record, route: route})
		}
		for _, endpoint := range project.endpoints {
			route := CodeRouteRecord{
				Language: "java", Framework: "Spring", Kind: "backend", FrameworkBound: true,
				HTTPMethod: endpoint.HTTPMethod, Path: endpoint.Path, Handler: qualifiedName(endpoint.Controller, endpoint.Method), File: endpoint.File, Line: endpoint.Line,
			}
			appendWorkspaceBackendRoute(&backendRoutes, seenBackendRoutes, knownServices, workspaceBackendRoute{project: project.record, route: route})
		}
	}

	var records []WorkspaceContractMatchRecord
	for _, project := range projects {
		for _, contract := range project.contracts {
			records = append(records, workspaceContractMatch(project.record, contract, backendRoutes, knownServices))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].APIProject != records[j].APIProject {
			return records[i].APIProject < records[j].APIProject
		}
		if records[i].APIFile != records[j].APIFile {
			return records[i].APIFile < records[j].APIFile
		}
		if records[i].APILine != records[j].APILine {
			return records[i].APILine < records[j].APILine
		}
		return records[i].APIPath < records[j].APIPath
	})
	return records
}

func appendWorkspaceBackendRoute(routes *[]workspaceBackendRoute, seen, knownServices map[string]bool, candidate workspaceBackendRoute) {
	candidate.route.Path = canonicalProviderPath(candidate.route.Path)
	key := filepath.ToSlash(candidate.project.Path) + "\x00" + strings.ToUpper(candidate.route.HTTPMethod) + "\x00" + normalizeAPIPathParameterNames(canonicalProviderPath(candidate.route.Path)) + "\x00" + candidate.route.Handler + "\x00" + filepath.ToSlash(candidate.route.File) + "\x00" + fmt.Sprint(candidate.route.Line)
	if !seen[key] {
		seen[key] = true
		*routes = append(*routes, candidate)
	}
	service := candidate.project.Service
	if service == "" {
		service = serviceCandidateForPath(candidate.route.Path)
	}
	if service != "" {
		knownServices[service] = true
	}
}

func workspaceContractMatch(project WorkspaceProjectRecord, contract APIContractRecord, routes []workspaceBackendRoute, knownServices map[string]bool) WorkspaceContractMatchRecord {
	base := WorkspaceContractMatchRecord{
		ID:                     stableID("workspace-contract", project.Path, contract.File, fmt.Sprint(contract.Line), contract.HTTPMethod, contract.Path),
		APIProject:             project.Path,
		APIHTTPMethod:          contract.HTTPMethod,
		APIPath:                contract.Path,
		APIFile:                contract.File,
		APILine:                contract.Line,
		APICaller:              contract.Caller,
		FrontendResponseFields: contract.ResponseFields,
		ServiceCandidate:       contract.ServiceCandidate,
	}
	if isFrontendInternalAPIContract(contract) {
		base.Issue = contractIssueFrontendInternalAPI
		base.Confidence = "OUT_OF_SCOPE"
		base.ConfidenceScore = 0.8
		base.Reason = "frontend-internal API route; not matched against backend services"
		return base
	}
	exactRoutes := exactWorkspaceRoutes(contract, routes)
	if len(exactRoutes) == 1 {
		return workspaceContractIssue(base, exactRoutes[0], contractIssueMatched, "RESOLVED", 0.9, "http method and path pattern match backend route")
	}
	if len(exactRoutes) > 1 {
		base.BackendHTTPMethod = strings.ToUpper(contract.HTTPMethod)
		base.BackendPath = displayRoutePath(contract.Path)
		base.Issue = "ambiguous_route"
		base.Confidence = "AMBIGUOUS"
		base.ConfidenceScore = 0.5
		base.Reason = "multiple indexed provider routes exactly match the frontend API contract"
		base.LikelyOwner = "multiple_backends"
		base.ResolutionHint = "select the intended backend provider using service ownership or gateway evidence"
		base.ResolutionClass = "ambiguous_route"
		for _, route := range exactRoutes {
			base.EquivalentRouteCandidates = append(base.EquivalentRouteCandidates, workspaceBackendRouteCandidate(route))
		}
		sort.Strings(base.EquivalentRouteCandidates)
		return base
	}
	if route, ok := pathCompatibleWorkspaceRoute(contract, routes); ok {
		record := workspaceContractIssue(base, route, contractIssueMethodMismatch, "MISMATCH", 0.45, "path pattern exists but http method differs")
		record.LikelyOwner = "frontend_or_backend"
		record.ResolutionHint = "same backend path exists with a different HTTP method; verify whether the frontend method or backend mapping is wrong, or whether this is a legacy intent"
		record.ResolutionClass = "method_conflict"
		record.ResolutionEvidence = []string{
			"same_path_backend_method=" + strings.ToUpper(route.route.HTTPMethod),
			"frontend_method=" + strings.ToUpper(contract.HTTPMethod),
			"source=workspace_route_index",
		}
		record.SimilarBackendRoutes = []string{strings.ToUpper(route.route.HTTPMethod) + " " + displayRoutePath(route.route.Path)}
		return record
	}
	if route, ok := gatewayPrefixCompatibleWorkspaceRoute(contract, routes); ok {
		record := workspaceContractIssue(base, route, contractIssueGatewayOrProxyPrefix, "PARTIAL_MATCH", 0.4, "path pattern matches after removing a common gateway or proxy prefix")
		record.LikelyOwner = "gateway_or_proxy"
		record.ResolutionHint = "backend route matches after removing a gateway/proxy prefix; verify rewrite configuration before changing frontend or backend code"
		record.SimilarBackendRoutes = []string{strings.ToUpper(route.route.HTTPMethod) + " " + displayRoutePath(route.route.Path)}
		return record
	}
	if contract.UnsafeDynamic {
		base.Issue = contractIssueUnsafeDynamic
		base.Confidence = "UNRESOLVED"
		base.ConfidenceScore = 0.35
		base.Reason = "api path contains complex dynamic expression"
		base.LikelyOwner = "frontend_dynamic_value"
		base.ResolutionHint = "path contains a complex dynamic expression; inspect the frontend path builder and constrain possible values"
		return base
	}
	if contract.ServiceCandidate != "" && !knownServices[contract.ServiceCandidate] {
		base.Issue = contractIssueUnscanned
		base.Confidence = "OUT_OF_SCOPE"
		base.ConfidenceScore = 0.75
		base.Reason = contract.ServiceCandidate + " has no indexed backend routes in this workspace"
		return base
	}
	if contract.ServiceCandidate != "" && knownServices[contract.ServiceCandidate] {
		issue, reason := indexedBackendRouteGapIssue(contract, contract.ServiceCandidate)
		base.Issue = issue
		base.Confidence = "UNRESOLVED"
		base.ConfidenceScore = 0.35
		base.Reason = reason
		if similar := similarWorkspaceRouteHints(contract, routes, 3); len(similar) > 0 {
			base.SimilarBackendRoutes = similar
			base.Reason += "; similar backend routes: " + strings.Join(similar, ", ")
			if isNeighborRouteGap(contract, similar) {
				base.MissingRouteKind = "neighbor_resource"
				base.EquivalentRouteCandidates = similar
				base.ResolutionClass = "neighbor_route_gap"
				base.ResolutionEvidence = []string{
					"source=workspace_route_similarity",
					"service=" + contract.ServiceCandidate,
				}
			}
		}
		base.LikelyOwner = workspaceRouteGapOwner(issue)
		base.ResolutionHint = workspaceRouteGapHint(issue)
		if issue == contractIssueDynamicEndpointUnresolved {
			base.DynamicEndpointCandidates = contract.DynamicEndpointCandidates
			if len(base.DynamicEndpointCandidates) == 0 {
				base.DynamicEndpointCandidates = dynamicEndpointCandidates(contract, routes)
			}
		}
		return base
	}
	base.Issue = contractIssueMissingRoute
	base.Confidence = "UNRESOLVED"
	base.ConfidenceScore = 0.3
	base.Reason = "no compatible backend route found in indexed workspace services"
	base.LikelyOwner = "backend_or_gateway"
	base.ResolutionHint = "no indexed backend route matched; verify service ownership, gateway rewrites, or missing backend implementation"
	return base
}

func workspaceContractIssue(base WorkspaceContractMatchRecord, route workspaceBackendRoute, issue, confidence string, score float64, reason string) WorkspaceContractMatchRecord {
	base.BackendProject = route.project.Path
	base.BackendService = route.project.Service
	if base.BackendService == "" {
		base.BackendService = serviceCandidateForPath(route.route.Path)
	}
	base.BackendHTTPMethod = route.route.HTTPMethod
	base.BackendPath = displayRoutePath(route.route.Path)
	base.BackendHandler = route.route.Handler
	base.BackendFile = route.route.File
	base.BackendLine = route.route.Line
	base.Issue = issue
	base.Confidence = confidence
	base.ConfidenceScore = score
	base.Reason = reason
	return base
}

func exactWorkspaceRoutes(contract APIContractRecord, routes []workspaceBackendRoute) []workspaceBackendRoute {
	var matches []workspaceBackendRoute
	for _, route := range routes {
		if strings.EqualFold(contract.HTTPMethod, route.route.HTTPMethod) && pathsCompatibleWithKnownBasePrefixes(contract.Path, route.route.Path) {
			matches = append(matches, route)
		}
	}
	sort.Slice(matches, func(left, right int) bool {
		return workspaceBackendRouteCandidate(matches[left]) < workspaceBackendRouteCandidate(matches[right])
	})
	return matches
}

func workspaceBackendRouteCandidate(route workspaceBackendRoute) string {
	handler := route.route.Handler
	if handler == "" {
		handler = "unknown-handler"
	}
	return fmt.Sprintf("%s %s %s -> %s (%s:%d)", filepath.ToSlash(route.project.Path), strings.ToUpper(route.route.HTTPMethod), displayRoutePath(route.route.Path), handler, filepath.ToSlash(route.route.File), route.route.Line)
}

func pathCompatibleWorkspaceRoute(contract APIContractRecord, routes []workspaceBackendRoute) (workspaceBackendRoute, bool) {
	for _, route := range routes {
		if pathsCompatibleWithKnownBasePrefixes(contract.Path, route.route.Path) {
			return route, true
		}
	}
	return workspaceBackendRoute{}, false
}

func gatewayPrefixCompatibleWorkspaceRoute(contract APIContractRecord, routes []workspaceBackendRoute) (workspaceBackendRoute, bool) {
	for _, route := range routes {
		if strings.EqualFold(contract.HTTPMethod, route.route.HTTPMethod) && pathsCompatibleWithoutGatewayPrefix(contract.Path, route.route.Path) {
			return route, true
		}
	}
	return workspaceBackendRoute{}, false
}

func similarWorkspaceRouteHints(contract APIContractRecord, routes []workspaceBackendRoute, limit int) []string {
	codeRoutes := make([]CodeRouteRecord, 0, len(routes))
	for _, route := range routes {
		routeService := route.project.Service
		if routeService == "" {
			routeService = serviceCandidateForPath(route.route.Path)
		}
		if contract.ServiceCandidate != "" && routeService != "" && routeService != contract.ServiceCandidate {
			continue
		}
		codeRoutes = append(codeRoutes, route.route)
	}
	return similarCodeRouteHints(contract, codeRoutes, limit)
}

func isNeighborRouteGap(contract APIContractRecord, similar []string) bool {
	if len(similar) == 0 {
		return false
	}
	contractBase := strings.TrimSuffix(displayRoutePath(contract.Path), "/availability")
	contractBase = strings.TrimSuffix(contractBase, "/available")
	contractBase = strings.TrimSuffix(contractBase, "/exists")
	if contractBase == displayRoutePath(contract.Path) {
		parts := routeParts(contract.Path)
		if len(parts) > 1 {
			contractBase = "/" + strings.Join(parts[:len(parts)-1], "/")
		}
	}
	for _, candidate := range similar {
		candidatePath := candidate
		if fields := strings.Fields(candidate); len(fields) > 1 {
			candidatePath = fields[1]
		}
		if strings.HasPrefix(candidatePath, contractBase+"/") || candidatePath == contractBase {
			return true
		}
	}
	return false
}

func workspaceRouteGapOwner(issue string) string {
	switch issue {
	case contractIssueDynamicEndpointUnresolved:
		return "frontend_dynamic_value"
	default:
		return "backend_or_gateway"
	}
}

func workspaceRouteGapHint(issue string) string {
	switch issue {
	case contractIssueDynamicEndpointUnresolved:
		return "resolve the frontend dynamic segment values and compare them with the listed backend route suffixes"
	default:
		return "indexed backend service has no compatible route; verify missing implementation, gateway rewrite, or whether a similar route is the intended replacement"
	}
}

func dynamicEndpointCandidates(contract APIContractRecord, routes []workspaceBackendRoute) []string {
	contractParts := routeParts(contract.Path)
	dynamicIndex := -1
	for i, part := range contractParts {
		if strings.EqualFold(part, "{endpoint}") || strings.EqualFold(part, "{dynamicendpoint}") {
			dynamicIndex = i
			break
		}
	}
	if dynamicIndex < 0 {
		return nil
	}
	seen := map[string]bool{}
	var candidates []string
	for _, route := range routes {
		routeService := route.project.Service
		if routeService == "" {
			routeService = serviceCandidateForPath(route.route.Path)
		}
		if contract.ServiceCandidate != "" && routeService != "" && routeService != contract.ServiceCandidate {
			continue
		}
		if !strings.EqualFold(contract.HTTPMethod, route.route.HTTPMethod) {
			continue
		}
		routeParts := routeParts(route.route.Path)
		if len(routeParts) <= dynamicIndex || len(routeParts) < len(contractParts) {
			continue
		}
		matchesPrefix := true
		for i := 0; i < dynamicIndex; i++ {
			if isPlaceholder(contractParts[i]) && isPlaceholder(routeParts[i]) {
				continue
			}
			if !strings.EqualFold(contractParts[i], routeParts[i]) {
				matchesPrefix = false
				break
			}
		}
		if !matchesPrefix {
			continue
		}
		candidate := strings.Join(routeParts[dynamicIndex:], "/")
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	return candidates
}

func hasBackendRoutes(routes []CodeRouteRecord) bool {
	for _, route := range routes {
		if route.Kind == "backend" {
			return true
		}
	}
	return false
}

func buildWorkspaceFeatureFlows(projects []workspaceIndexProject, matches []WorkspaceContractMatchRecord) []WorkspaceFeatureFlowRecord {
	byProject := map[string]workspaceIndexProject{}
	for _, project := range projects {
		byProject[project.record.Path] = project
	}
	var records []WorkspaceFeatureFlowRecord
	for _, match := range matches {
		if match.Issue != contractIssueMatched {
			continue
		}
		backendProject, ok := byProject[match.BackendProject]
		if !ok {
			continue
		}
		frontendProject, hasFrontendProject := byProject[match.APIProject]
		flow, hasFlow := findWorkspaceEndpointFlow(backendProject.endpointFlows, match)
		endpoint, hasEndpoint := findWorkspaceSpringEndpoint(backendProject.endpoints, match)
		tests := workspaceFeatureTests(backendProject.testMap, flow, match)
		record := WorkspaceFeatureFlowRecord{
			ID:                stableID("workspace-flow", match.APIProject, match.APIFile, fmt.Sprint(match.APILine), match.APIHTTPMethod, match.APIPath, match.BackendProject),
			FrontendProject:   match.APIProject,
			FrontendCaller:    match.APICaller,
			FrontendFile:      match.APIFile,
			FrontendLine:      match.APILine,
			HTTPMethod:        match.APIHTTPMethod,
			Path:              match.APIPath,
			BackendProject:    match.BackendProject,
			BackendService:    match.BackendService,
			BackendController: flow.Controller,
			BackendMethod:     flow.Method,
			BackendFile:       match.BackendFile,
			BackendLine:       match.BackendLine,
			BackendSteps:      flow.Steps,
			Tests:             tests,
			Confidence:        match.Confidence,
			Reason:            "frontend api contract resolved to indexed backend endpoint",
		}
		if hasFrontendProject {
			frontend := resolveWorkspaceFrontendContext(frontendProject, match)
			record.FrontendRouteID = frontend.routeID
			record.FrontendRoutePath = frontend.routePath
			record.FrontendRouteFile = frontend.routeFile
			record.FrontendRouteLine = frontend.routeLine
			record.FrontendComponent = frontend.component
			record.FrontendCaller = firstNonEmpty(frontend.apiCaller, match.APICaller)
			record.FrontendSteps = frontend.steps
			record.FrontendConfidence = frontend.confidence
			record.FrontendReason = frontend.reason
		}
		if len(tests) == 0 {
			record.TestReason = fmt.Sprintf("no endpoint or backend-step tests matched backend endpoint %s `%s`; check backend diagnostics Endpoints Without Tests", match.BackendHTTPMethod, match.BackendPath)
		}
		if !hasFlow {
			record.BackendMethod = match.BackendHandler
		}
		if hasEndpoint {
			record.BackendRequestType = endpoint.RequestType
			record.BackendRequestKind = endpoint.RequestKind
			record.BackendConsumes = endpoint.Consumes
			record.BackendReturnType = endpoint.ReturnType
			record.Auth = endpoint.Auth
			record.BackendRequestFields = workspaceDTOFields(backendProject.spring.DTOs, endpoint.RequestType)
			record.BackendResponseFields = workspaceDTOFields(backendProject.spring.DTOs, endpoint.ReturnType)
			record.FieldRisks = workspaceFieldRisks(match.FrontendResponseFields, record.BackendResponseFields)
		}
		record.PersistencePath = workspacePersistencePath(backendProject.spring, record.BackendSteps)
		record.TestLinks = BuildTestLinks(record)
		record.VerificationCommands = BuildVerificationCommands(backendProject.record, record.Tests)
		record = BuildCanonicalFeatureFlow(record)
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].FrontendProject != records[j].FrontendProject {
			return records[i].FrontendProject < records[j].FrontendProject
		}
		if records[i].FrontendFile != records[j].FrontendFile {
			return records[i].FrontendFile < records[j].FrontendFile
		}
		if records[i].FrontendLine != records[j].FrontendLine {
			return records[i].FrontendLine < records[j].FrontendLine
		}
		return records[i].Path < records[j].Path
	})
	return records
}

func workspaceDTOFields(dtos []DTORecord, typeName string) []DTOFieldRecord {
	typeName = baseJavaTypeName(typeName)
	if typeName == "" {
		return nil
	}
	for _, dto := range dtos {
		if dto.Name == typeName {
			return append([]DTOFieldRecord(nil), dto.Fields...)
		}
	}
	return nil
}

func baseJavaTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimPrefix(typeName, "ResponseEntity<")
	typeName = strings.TrimPrefix(typeName, "Optional<")
	typeName = strings.TrimSuffix(typeName, ">")
	if strings.HasPrefix(typeName, "List<") || strings.HasPrefix(typeName, "Collection<") || strings.HasPrefix(typeName, "Set<") {
		if start := strings.Index(typeName, "<"); start >= 0 {
			typeName = strings.TrimSuffix(typeName[start+1:], ">")
		}
	}
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimPrefix(typeName, "? extends ")
	return strings.TrimSpace(typeName)
}

func workspacePersistencePath(spring SpringIndex, steps []SpringEndpointFlowStep) []PersistenceStepRecord {
	if len(steps) == 0 || len(spring.Repositories) == 0 {
		return nil
	}
	repositoryByName := map[string]SpringRepositoryRecord{}
	entityByName := map[string]SpringEntityRecord{}
	for _, repository := range spring.Repositories {
		repositoryByName[repository.Name] = repository
	}
	for _, entity := range spring.Entities {
		entityByName[entity.Name] = entity
	}
	seen := map[string]bool{}
	var records []PersistenceStepRecord
	for _, step := range steps {
		repository, ok := repositoryByName[step.Owner]
		if !ok {
			continue
		}
		key := repository.Name + "." + step.Method
		if seen[key] {
			continue
		}
		seen[key] = true
		entity := entityByName[repository.Entity]
		records = append(records, PersistenceStepRecord{
			Repository: repository.Name,
			Method:     step.Method,
			Entity:     repository.Entity,
			Table:      entity.Table,
			File:       firstNonEmpty(repository.File, entity.File),
			Line:       step.Line,
			Source:     "endpoint_flow_repository_step",
			Confidence: firstNonEmpty(step.Confidence, "EXTRACTED"),
		})
	}
	return records
}

func workspaceFieldRisks(frontendFields []string, backendFields []DTOFieldRecord) []FieldRiskRecord {
	if len(frontendFields) == 0 || len(backendFields) == 0 {
		return nil
	}
	backendSet := map[string]bool{}
	for _, field := range backendFields {
		backendSet[field.Name] = true
	}
	var risks []FieldRiskRecord
	for _, field := range frontendFields {
		if backendSet[field] {
			continue
		}
		risks = append(risks, FieldRiskRecord{
			Kind:       "frontend_uses_field_not_in_backend_dto",
			Field:      field,
			Reason:     "frontend response field usage was not found in backend response DTO fields",
			Source:     "frontend_response_usage_vs_backend_dto",
			Confidence: "MATCHED",
		})
	}
	return risks
}

func findWorkspaceSpringEndpoint(endpoints []SpringEndpointRecord, match WorkspaceContractMatchRecord) (SpringEndpointRecord, bool) {
	for _, endpoint := range endpoints {
		if strings.EqualFold(endpoint.HTTPMethod, match.BackendHTTPMethod) && pathsCompatibleWithKnownBasePrefixes(endpoint.Path, match.BackendPath) {
			return endpoint, true
		}
	}
	return SpringEndpointRecord{}, false
}

type workspaceFrontendContext struct {
	routeID    string
	routePath  string
	routeFile  string
	routeLine  int
	component  string
	apiCaller  string
	steps      []CodeFlowStep
	confidence string
	reason     string
	score      float64
}

func resolveWorkspaceFrontendContext(project workspaceIndexProject, match WorkspaceContractMatchRecord) workspaceFrontendContext {
	var best workspaceFrontendContext
	apiApp := codeFileApp(match.APIFile)
	for _, flow := range project.codeFlows {
		if flow.Kind != "frontend" {
			continue
		}
		if apiApp != "" && flow.App != "" && flow.App != apiApp {
			continue
		}
		candidate := scoreWorkspaceFrontendFlow(flow, match)
		if candidate.score > best.score {
			best = candidate
		}
	}
	if best.score <= 0.35 {
		if relation, ok := frontendRelationToAPIFile(project.legacyRelations, match.APIFile, apiApp); ok {
			return workspaceFrontendContext{
				routeID:    best.routeID,
				routePath:  best.routePath,
				routeFile:  best.routeFile,
				routeLine:  best.routeLine,
				component:  componentNameFromPath(relation.From),
				apiCaller:  match.APICaller,
				steps:      best.steps,
				confidence: "RESOLVED",
				reason:     "frontend relation reaches API contract file",
				score:      0.84,
			}
		}
	}
	if best.score <= 0.35 && isComponentOrPageFile(match.APIFile) {
		return workspaceFrontendContext{
			routeID:    best.routeID,
			routePath:  best.routePath,
			routeFile:  best.routeFile,
			routeLine:  best.routeLine,
			component:  best.component,
			apiCaller:  match.APICaller,
			steps:      best.steps,
			confidence: "RESOLVED",
			reason:     "API contract caller is declared in a frontend component or page file",
			score:      0.78,
		}
	}
	if best.score <= 0.35 && isContainerFile(match.APIFile) {
		return workspaceFrontendContext{
			routeID:    best.routeID,
			routePath:  best.routePath,
			routeFile:  best.routeFile,
			routeLine:  best.routeLine,
			component:  best.component,
			apiCaller:  match.APICaller,
			steps:      best.steps,
			confidence: "RESOLVED",
			reason:     "API contract caller is declared in a frontend container file",
			score:      0.76,
		}
	}
	if best.score <= 0.35 && isAppRootFile(match.APIFile) {
		return workspaceFrontendContext{
			routeID:    best.routeID,
			routePath:  best.routePath,
			routeFile:  best.routeFile,
			routeLine:  best.routeLine,
			component:  best.component,
			apiCaller:  match.APICaller,
			steps:      best.steps,
			confidence: "RESOLVED",
			reason:     "API contract caller is declared in an app root file",
			score:      0.76,
		}
	}
	if best.score == 0 && isPackageUIFile(match.APIFile) {
		return workspaceFrontendContext{
			apiCaller:  match.APICaller,
			confidence: "RESOLVED",
			reason:     "API contract caller is declared in a package UI file",
			score:      0.72,
		}
	}
	if best.score == 0 && isPackageUtilityFile(match.APIFile) {
		return workspaceFrontendContext{
			apiCaller:  match.APICaller,
			confidence: "RESOLVED",
			reason:     "API contract caller is declared in a package utility file",
			score:      0.68,
		}
	}
	return best
}

func frontendRelationToAPIFile(relations []RelationRecord, apiFile, apiApp string) (RelationRecord, bool) {
	for _, relation := range relations {
		if relation.To != apiFile {
			continue
		}
		if relation.Type != "calls" && relation.Type != "imports_internal" && relation.Type != "imports" {
			continue
		}
		if apiApp != "" && codeFileApp(relation.From) != "" && codeFileApp(relation.From) != apiApp {
			continue
		}
		if isComponentOrPageFile(relation.From) || isContainerFile(relation.From) || isAppRootFile(relation.From) {
			return relation, true
		}
	}
	return RelationRecord{}, false
}

func componentNameFromPath(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func isComponentOrPageFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.Contains(normalized, "/components/") ||
		strings.Contains(normalized, "/pages/") ||
		strings.Contains(normalized, "/app/")
}

func isContainerFile(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/containers/")
}

func isAppRootFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "apps/") && strings.HasSuffix(normalized, "/src/Root.tsx")
}

func isPackageUIFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "packages/") &&
		(strings.Contains(normalized, "/components/") ||
			strings.Contains(normalized, "/organisms/") ||
			strings.Contains(normalized, "/molecules/") ||
			strings.Contains(normalized, "/atoms/"))
}

func isPackageUtilityFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "packages/") && strings.Contains(normalized, "/src/utils/")
}

func scoreWorkspaceFrontendFlow(flow CodeFlowRecord, match WorkspaceContractMatchRecord) workspaceFrontendContext {
	context := workspaceFrontendContext{
		routeID:    flow.RouteID,
		routePath:  flow.Path,
		routeFile:  flow.File,
		routeLine:  flow.Line,
		component:  flow.Handler,
		steps:      flow.Steps,
		confidence: "WEAK_MATCH",
		reason:     "frontend route shares app with API contract but no route-flow step reached the API caller",
		score:      0.35,
	}
	for _, step := range flow.Steps {
		if step.Kind == "route_handler" && step.Name != "" {
			context.component = step.Name
		}
		if step.File != match.APIFile || match.APIFile == "" {
			continue
		}
		if step.Name != "" {
			context.apiCaller = step.Name
		}
		if context.score < 0.82 {
			context.confidence = "RESOLVED"
			context.reason = "route flow reaches API contract file"
			context.score = 0.82
		}
		if step.Name != "" {
			context.confidence = "RESOLVED"
			context.reason, context.score = frontendCallerResolution(step)
		}
	}
	return context
}

func frontendCallerResolution(step CodeFlowStep) (string, float64) {
	switch step.Kind {
	case "effect_call":
		return "route flow reaches API contract caller through effect", 0.92
	case "event_handler":
		return "route flow reaches API contract caller through event handler", 0.90
	default:
		return "route flow reaches API contract caller", 0.95
	}
}

func findWorkspaceEndpointFlow(flows []SpringEndpointFlowRecord, match WorkspaceContractMatchRecord) (SpringEndpointFlowRecord, bool) {
	for _, flow := range flows {
		if strings.EqualFold(flow.HTTPMethod, match.BackendHTTPMethod) && pathsCompatible(flow.Path, match.BackendPath) {
			return flow, true
		}
	}
	return SpringEndpointFlowRecord{}, false
}

func workspaceFeatureTests(tests []TestMapRecord, flow SpringEndpointFlowRecord, match WorkspaceContractMatchRecord) []TestMapRecord {
	var result []TestMapRecord
	seen := map[string]bool{}
	for _, test := range tests {
		if workspaceFeatureTestMatches(test, flow, match) {
			key := test.TestFile + "|" + test.TestClass + "|" + test.TestMethod + "|" + test.HTTPMethod + "|" + test.Path
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, test)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TestFile != result[j].TestFile {
			return result[i].TestFile < result[j].TestFile
		}
		return result[i].TestMethod < result[j].TestMethod
	})
	return result
}

func workspaceFeatureTestMatches(test TestMapRecord, flow SpringEndpointFlowRecord, match WorkspaceContractMatchRecord) bool {
	if test.Type == "endpoint" && strings.EqualFold(test.HTTPMethod, match.BackendHTTPMethod) && pathsCompatibleWithKnownBasePrefixes(test.Path, match.BackendPath) {
		return true
	}
	if flow.Controller != "" && test.TargetClass == flow.Controller && test.TargetMethod == flow.Method {
		return true
	}
	for _, step := range flow.Steps {
		if test.TargetClass == step.Owner && test.TargetMethod == step.Method {
			return true
		}
	}
	return false
}

func filterWorkspaceFeatureFlows(projectPath string, records []WorkspaceFeatureFlowRecord) []WorkspaceFeatureFlowRecord {
	var filtered []WorkspaceFeatureFlowRecord
	for _, record := range records {
		if record.FrontendProject == projectPath || record.BackendProject == projectPath {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterFeatureDossiers(projectPath string, records []FeatureDossierRecord) []FeatureDossierRecord {
	var filtered []FeatureDossierRecord
	for _, record := range records {
		if record.FrontendProject == projectPath || record.BackendProject == projectPath {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterWorkspaceGraph(projectPath string, graph WorkspaceGraphRecord) WorkspaceGraphRecord {
	keep := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Project == projectPath || node.Kind == "project" && node.ID == StableWorkspaceID("project", projectPath) {
			keep[node.ID] = true
		}
	}
	changed := true
	for changed {
		changed = false
		for _, edge := range graph.Edges {
			if keep[edge.From] && !keep[edge.To] {
				keep[edge.To] = true
				changed = true
			}
			if keep[edge.To] && !keep[edge.From] {
				keep[edge.From] = true
				changed = true
			}
		}
	}
	filtered := WorkspaceGraphRecord{
		SchemaVersion: graph.SchemaVersion,
		Generated:     graph.Generated,
		Root:          graph.Root,
		Stats:         map[string]int{},
	}
	for _, node := range graph.Nodes {
		if keep[node.ID] {
			filtered.Nodes = append(filtered.Nodes, node)
			filtered.Stats["nodes_"+node.Kind]++
		}
	}
	for _, edge := range graph.Edges {
		if keep[edge.From] && keep[edge.To] {
			filtered.Edges = append(filtered.Edges, edge)
			filtered.Stats["edges_"+edge.Kind]++
		}
	}
	filtered.Stats["nodes_total"] = len(filtered.Nodes)
	filtered.Stats["edges_total"] = len(filtered.Edges)
	return filtered
}

func renderProjectWorkspaceMapPointer(workspaceRoot string) string {
	return "# GoreGraph Workspace Map\n\nOpen the workspace dashboard at:\n\n`" + filepath.ToSlash(filepath.Join(workspaceRoot, ".goregraph-workspace", "dashboard", "workspace-map.html")) + "`\n"
}

func renderWorkspaceContextReport(record WorkspaceContextRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Context\n\n")
	b.WriteString(fmt.Sprintf("- Workspace root: `%s`\n", record.Root))
	if record.RequestedScope != "" {
		b.WriteString(fmt.Sprintf("- Requested scope: `%s`\n", record.RequestedScope))
	}
	b.WriteString("\n")
	b.WriteString(renderWorkspaceContextSections(record))
	return b.String()
}

func renderProjectWorkspaceContextReport(record WorkspaceContextRecord, projectPath string) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Context\n\n")
	b.WriteString(fmt.Sprintf("- Workspace root: `%s`\n", record.Root))
	if projectPath != "" {
		b.WriteString(fmt.Sprintf("- Requested scope: `%s`\n", projectPath))
	}
	b.WriteString("\n")
	b.WriteString(renderWorkspaceContextSections(record))
	return b.String()
}

func renderNoWorkspaceContextReport() string {
	return "# GoreGraph Workspace Context\n\n- No GoreGraph workspace detected for this scan.\n"
}

func renderNoWorkspaceNextActionsReport() string {
	return "# GoreGraph Workspace Next Actions\n\n- No GoreGraph workspace detected for this scan.\n"
}

func renderWorkspaceContextSections(record WorkspaceContextRecord) string {
	var b strings.Builder
	b.WriteString("## Projects\n\n")
	for _, project := range record.Projects {
		b.WriteString(fmt.Sprintf("- `%s` - %s", project.Path, project.Status))
		if project.Service != "" {
			b.WriteString(fmt.Sprintf(", service `%s`", project.Service))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Loaded Indexes\n\n")
	if len(record.LoadedIndexes) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, project := range record.LoadedIndexes {
			b.WriteString(fmt.Sprintf("- `%s`\n", project.Path))
		}
	}
	b.WriteString("\n## Known Backend Services\n\n")
	if len(record.KnownServices) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, service := range record.KnownServices {
			b.WriteString(fmt.Sprintf("- `%s`\n", service))
		}
	}
	b.WriteString("\n## Referenced But Missing Services\n\n")
	if len(record.MissingServices) == 0 {
		b.WriteString("- none\n")
	} else if len(record.MissingServiceDetails) > 0 {
		for _, service := range record.MissingServiceDetails {
			b.WriteString(fmt.Sprintf("- `%s` - %d contracts", service.Service, service.Contracts))
			if service.Project != "" {
				b.WriteString(fmt.Sprintf(" - project `%s`", service.Project))
			}
			if service.Status != "" {
				b.WriteString(fmt.Sprintf(" - %s", service.Status))
			}
			b.WriteString("\n")
		}
	} else {
		for _, service := range record.MissingServices {
			b.WriteString(fmt.Sprintf("- `%s`\n", service))
		}
	}
	writeWorkspaceScanSuggestions(&b, record)
	return b.String()
}

func renderWorkspaceNextActionsReport(context WorkspaceContextRecord, matches []WorkspaceContractMatchRecord, featureFlows []WorkspaceFeatureFlowRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Next Actions\n\n")
	b.WriteString("## Workspace Coverage\n\n")
	indexedProjects := len(context.LoadedIndexes)
	totalProjects := len(context.Projects)
	referencedServices := workspaceReferencedServices(context, matches)
	indexedReferencedServices := 0
	knownServices := stringSet(context.KnownServices)
	for service := range referencedServices {
		if knownServices[service] {
			indexedReferencedServices++
		}
	}
	contractSummary := BuildWorkspaceContractSummary(matches)
	b.WriteString(fmt.Sprintf("- Projects indexed: %d / %d\n", indexedProjects, totalProjects))
	b.WriteString(fmt.Sprintf("- Referenced services indexed: %d / %d\n", indexedReferencedServices, len(referencedServices)))
	b.WriteString(fmt.Sprintf("- Workspace contracts: %d total; resolved: %d; missing route: %d; method mismatch: %d; dynamic unresolved: %d; out of scope: %d; other: %d\n",
		contractSummary.Total,
		contractSummary.Resolved,
		contractSummary.MissingRoute,
		contractSummary.MethodMismatch,
		contractSummary.DynamicUnresolved,
		contractSummary.OutOfScope,
		contractSummary.Other,
	))

	b.WriteString("\n## Most Useful Next Scans\n\n")
	wroteScan := false
	for _, service := range context.MissingServiceDetails {
		if service.Project == "" || service.Status == "indexed" {
			continue
		}
		wroteScan = true
		b.WriteString(fmt.Sprintf("- `%s` - %d contracts - project `%s` - %s\n", service.Service, service.Contracts, service.Project, emptyAsNone(service.Status)))
		b.WriteString(fmt.Sprintf("  - `%s`\n", workspaceScanCommand(service.Project)))
	}
	if !wroteScan {
		b.WriteString("- none\n")
	}

	b.WriteString("\n## Weak Workspace Matches\n\n")
	weak := 0
	for _, match := range matches {
		if match.Issue == contractIssueMatched {
			continue
		}
		weak++
		b.WriteString(fmt.Sprintf("- %s `%s` from `%s` `%s:%d`%s - %s (%s)\n",
			match.APIHTTPMethod,
			match.APIPath,
			match.APIProject,
			match.APIFile,
			match.APILine,
			workspaceCallerSuffix(match.APICaller),
			match.Issue,
			match.Confidence,
		))
		if match.LikelyOwner != "" {
			b.WriteString(fmt.Sprintf("  - Likely owner: `%s`\n", match.LikelyOwner))
		}
		if match.ResolutionHint != "" {
			b.WriteString(fmt.Sprintf("  - Resolution: %s\n", match.ResolutionHint))
		}
		if len(match.SimilarBackendRoutes) > 0 {
			b.WriteString(fmt.Sprintf("  - Similar backend routes: `%s`\n", strings.Join(match.SimilarBackendRoutes, "`, `")))
		}
		if weak >= 10 {
			break
		}
	}
	if weak == 0 {
		b.WriteString("- none\n")
	}

	b.WriteString("\n## Resolved Flows Without Linked Tests\n\n")
	missingTests := 0
	for _, flow := range featureFlows {
		if len(flow.Tests) > 0 {
			continue
		}
		missingTests++
		b.WriteString(fmt.Sprintf("- %s `%s` from `%s` `%s:%d` -> `%s`",
			flow.HTTPMethod,
			flow.Path,
			flow.FrontendProject,
			flow.FrontendFile,
			flow.FrontendLine,
			qualifiedName(flow.BackendController, flow.BackendMethod),
		))
		if flow.TestReason != "" {
			b.WriteString(fmt.Sprintf(" (%s)", flow.TestReason))
		}
		b.WriteString("\n")
		if missingTests >= 10 {
			break
		}
	}
	if missingTests == 0 {
		b.WriteString("- none\n")
	}
	return b.String()
}

func workspaceReferencedServices(context WorkspaceContextRecord, matches []WorkspaceContractMatchRecord) map[string]bool {
	referenced := map[string]bool{}
	for _, service := range context.MissingServices {
		referenced[service] = true
	}
	for _, match := range matches {
		service := firstNonEmpty(match.ServiceCandidate, match.BackendService)
		if service != "" {
			referenced[service] = true
		}
	}
	return referenced
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func workspaceScanCommand(project string) string {
	return fmt.Sprintf("cd %s && goregraph scan .", project)
}

// WorkspaceMissingScanPlan returns the highest-value missing service projects to scan.
func WorkspaceMissingScanPlan(root string, cfg config.Config, top int) (WorkspaceMissingScanPlanRecord, error) {
	currentAbs, err := filepath.Abs(root)
	if err != nil {
		return WorkspaceMissingScanPlanRecord{}, err
	}
	workspaceRoot, ok, err := resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
	if err != nil {
		return WorkspaceMissingScanPlanRecord{}, err
	}
	if !ok {
		return WorkspaceMissingScanPlanRecord{}, fmt.Errorf("no GoreGraph workspace detected")
	}
	projects, err := discoverWorkspaceProjects(workspaceRoot, currentAbs, cfg.OutputDir)
	if err != nil {
		return WorkspaceMissingScanPlanRecord{}, err
	}
	indexed, err := loadWorkspaceIndexes(projects)
	if err != nil {
		return WorkspaceMissingScanPlanRecord{}, err
	}
	registry := WorkspaceRegistryRecord{
		Root:     filepath.ToSlash(workspaceRoot),
		Current:  workspaceRel(workspaceRoot, currentAbs),
		Projects: projects,
	}
	context := buildWorkspaceContext(registry, indexed)
	projectsByPath := map[string]WorkspaceProjectRecord{}
	for _, project := range projects {
		projectsByPath[project.Path] = project
	}
	var items []WorkspaceMissingScanItemRecord
	for _, missing := range context.MissingServiceDetails {
		if missing.Project == "" || missing.Status == "indexed" {
			continue
		}
		project, ok := projectsByPath[missing.Project]
		if !ok {
			continue
		}
		items = append(items, WorkspaceMissingScanItemRecord{
			Service:   missing.Service,
			Contracts: missing.Contracts,
			Project:   missing.Project,
			AbsPath:   project.AbsPath,
			Status:    missing.Status,
		})
		if top > 0 && len(items) >= top {
			break
		}
	}
	return WorkspaceMissingScanPlanRecord{
		WorkspaceRoot: filepath.ToSlash(workspaceRoot),
		Current:       registry.Current,
		Top:           top,
		Items:         items,
	}, nil
}

// WorkspaceProjectScanPlan returns all discovered workspace projects to scan.
func WorkspaceProjectScanPlan(root string, cfg config.Config) (WorkspaceProjectScanPlanRecord, error) {
	currentAbs, err := filepath.Abs(root)
	if err != nil {
		return WorkspaceProjectScanPlanRecord{}, err
	}
	workspaceRoot, ok, err := resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
	if err != nil {
		return WorkspaceProjectScanPlanRecord{}, err
	}
	if !ok {
		return WorkspaceProjectScanPlanRecord{}, fmt.Errorf("no GoreGraph workspace detected")
	}
	projects, err := discoverWorkspaceProjects(workspaceRoot, currentAbs, cfg.OutputDir)
	if err != nil {
		return WorkspaceProjectScanPlanRecord{}, err
	}
	items := make([]WorkspaceProjectScanItemRecord, 0, len(projects))
	for _, project := range projects {
		items = append(items, WorkspaceProjectScanItemRecord{
			Project: project.Path,
			AbsPath: project.AbsPath,
			Status:  project.Status,
		})
	}
	return WorkspaceProjectScanPlanRecord{
		WorkspaceRoot: filepath.ToSlash(workspaceRoot),
		Current:       workspaceRel(workspaceRoot, currentAbs),
		Items:         items,
	}, nil
}

// WorkspaceCleanPlan returns generated GoreGraph output paths in a workspace.
func WorkspaceCleanPlan(root string, cfg config.Config) (WorkspaceCleanPlanRecord, error) {
	currentAbs, err := filepath.Abs(root)
	if err != nil {
		return WorkspaceCleanPlanRecord{}, err
	}
	workspaceRoot, ok, err := resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
	if err != nil {
		return WorkspaceCleanPlanRecord{}, err
	}
	if !ok {
		return WorkspaceCleanPlanRecord{}, fmt.Errorf("no GoreGraph workspace detected")
	}
	projects, err := discoverWorkspaceProjects(workspaceRoot, currentAbs, cfg.OutputDir)
	if err != nil {
		return WorkspaceCleanPlanRecord{}, err
	}
	items := make([]WorkspaceCleanItemRecord, 0, len(projects)+1)
	for _, project := range projects {
		path := filepath.Join(project.AbsPath, project.OutputDir)
		items = append(items, WorkspaceCleanItemRecord{
			Path:   filepath.ToSlash(path),
			Reason: "project output for " + project.Path,
			Exists: workspaceFileExists(path),
		})
	}
	workspaceOut := filepath.Join(workspaceRoot, ".goregraph-workspace")
	includes := make([]string, 0, len(workspaceSymbolGeneratedFiles))
	for _, name := range workspaceSymbolGeneratedFiles {
		includes = append(includes, filepath.ToSlash(NewWorkspaceOutputLayout(workspaceOut).Index(name)))
	}
	items = append(items, WorkspaceCleanItemRecord{
		Path:     filepath.ToSlash(workspaceOut),
		Reason:   "workspace overlay output",
		Exists:   workspaceFileExists(workspaceOut),
		Includes: includes,
	})
	return WorkspaceCleanPlanRecord{
		WorkspaceRoot: filepath.ToSlash(workspaceRoot),
		Current:       workspaceRel(workspaceRoot, currentAbs),
		Items:         items,
	}, nil
}

func writeWorkspaceScanSuggestions(b *strings.Builder, record WorkspaceContextRecord) {
	var suggestions []string
	for _, service := range record.MissingServiceDetails {
		if service.Project == "" || service.Status == "indexed" {
			continue
		}
		suggestions = append(suggestions, workspaceScanCommand(service.Project))
	}
	if len(suggestions) == 0 {
		return
	}
	b.WriteString("\n## Suggested Next Scans\n\n")
	for _, suggestion := range suggestions {
		b.WriteString(fmt.Sprintf("- `%s`\n", suggestion))
	}
}

func renderWorkspaceContractMatchesReport(records []WorkspaceContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Contract Matches\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		renderWorkspaceContractMatchLine(&b, record)
	}
	return b.String()
}

func renderProjectWorkspaceMatchesReport(projectPath string, records []WorkspaceContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Contract Matches\n\n")
	count := 0
	for _, record := range records {
		if record.APIProject != projectPath && record.BackendProject != projectPath {
			continue
		}
		count++
		renderWorkspaceContractMatchLine(&b, record)
	}
	if count == 0 {
		b.WriteString("- none detected\n")
	}
	return b.String()
}

func filterWorkspaceContractMatches(projectPath string, records []WorkspaceContractMatchRecord) []WorkspaceContractMatchRecord {
	var filtered []WorkspaceContractMatchRecord
	for _, record := range records {
		if record.APIProject == projectPath || record.BackendProject == projectPath {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func renderWorkspaceContractMatchLine(b *strings.Builder, record WorkspaceContractMatchRecord) {
	if record.Issue == contractIssueMatched {
		service := emptyAsNone(record.BackendService)
		b.WriteString(fmt.Sprintf("- %s `%s` -> %s %s `%s` via `%s:%d` (%s, frontend `%s` `%s:%d`%s)\n",
			record.APIHTTPMethod,
			record.APIPath,
			service,
			record.BackendHTTPMethod,
			record.BackendPath,
			record.BackendFile,
			record.BackendLine,
			record.Confidence,
			record.APIProject,
			record.APIFile,
			record.APILine,
			workspaceCallerSuffix(record.APICaller),
		))
		return
	}
	b.WriteString(fmt.Sprintf("- %s `%s` from `%s` `%s:%d`%s: %s (%s, %s)\n",
		record.APIHTTPMethod,
		record.APIPath,
		record.APIProject,
		record.APIFile,
		record.APILine,
		workspaceCallerSuffix(record.APICaller),
		record.Issue,
		record.Confidence,
		record.Reason,
	))
	if record.LikelyOwner != "" {
		b.WriteString(fmt.Sprintf("  - Likely owner: `%s`\n", record.LikelyOwner))
	}
	if record.ResolutionHint != "" {
		b.WriteString(fmt.Sprintf("  - Resolution: %s\n", record.ResolutionHint))
	}
	if len(record.SimilarBackendRoutes) > 0 {
		b.WriteString(fmt.Sprintf("  - Similar backend routes: `%s`\n", strings.Join(record.SimilarBackendRoutes, "`, `")))
	}
	if len(record.DynamicEndpointCandidates) > 0 {
		b.WriteString(fmt.Sprintf("  - Dynamic endpoint candidates: `%s`\n", strings.Join(record.DynamicEndpointCandidates, "`, `")))
	}
}

func workspaceCallerSuffix(caller string) string {
	if caller == "" {
		return ""
	}
	return fmt.Sprintf(", caller `%s`", caller)
}

func renderFrontendConsumersReport(projectPath string, records []WorkspaceContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Frontend Consumers\n\n")
	count := 0
	for _, record := range records {
		if record.BackendProject != projectPath || record.Issue != contractIssueMatched {
			continue
		}
		count++
		b.WriteString(fmt.Sprintf("- %s `%s` used by `%s` `%s:%d`%s -> `%s.%s`\n",
			record.APIHTTPMethod,
			record.APIPath,
			record.APIProject,
			record.APIFile,
			record.APILine,
			workspaceCallerSuffix(record.APICaller),
			record.BackendService,
			record.BackendHandler,
		))
	}
	if count == 0 {
		if hasOutgoingWorkspaceContracts(projectPath, records) {
			b.WriteString("- not applicable for frontend projects; this report lists frontend consumers of backend endpoints. See `workspace-contract-matches.md` and `workspace-feature-flows.md` for this project's outgoing API calls.\n")
			return b.String()
		}
		b.WriteString("- none detected\n")
	}
	return b.String()
}

func hasOutgoingWorkspaceContracts(projectPath string, records []WorkspaceContractMatchRecord) bool {
	for _, record := range records {
		if record.APIProject == projectPath && record.Issue == contractIssueMatched {
			return true
		}
	}
	return false
}

func renderWorkspaceFeatureFlowsReport(records []WorkspaceFeatureFlowRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph Workspace Feature Flows\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		b.WriteString(fmt.Sprintf("## %s `%s`\n\n", record.HTTPMethod, record.Path))
		if record.FrontendRouteID != "" || record.FrontendRoutePath != "" || record.FrontendComponent != "" {
			b.WriteString(fmt.Sprintf("- Frontend route: `%s` `%s` -> `%s`",
				emptyAsNone(record.FrontendRouteID),
				emptyAsNone(record.FrontendRoutePath),
				emptyAsNone(record.FrontendComponent),
			))
			if record.FrontendRouteFile != "" {
				b.WriteString(fmt.Sprintf(" in `%s:%d`", record.FrontendRouteFile, record.FrontendRouteLine))
			}
			if record.FrontendConfidence != "" {
				b.WriteString(fmt.Sprintf(" (%s", record.FrontendConfidence))
				if record.FrontendReason != "" {
					b.WriteString(fmt.Sprintf(", %s", record.FrontendReason))
				}
				b.WriteString(")")
			}
			b.WriteString("\n")
		} else {
			b.WriteString("- Frontend route: none resolved (no route flow reached this API contract)\n")
		}
		b.WriteString(fmt.Sprintf("- Frontend API: `%s:%d` `%s`\n", record.FrontendFile, record.FrontendLine, emptyAsNone(record.FrontendCaller)))
		b.WriteString(fmt.Sprintf("- Frontend project: `%s`\n", record.FrontendProject))
		backendName := qualifiedName(record.BackendController, record.BackendMethod)
		b.WriteString(fmt.Sprintf("- Backend: `%s` `%s` -> `%s` in `%s:%d`\n", record.BackendProject, emptyAsNone(record.BackendService), emptyAsNone(backendName), record.BackendFile, record.BackendLine))
		if record.BackendRequestKind != "" || record.BackendRequestType != "" || record.BackendConsumes != "" {
			b.WriteString(fmt.Sprintf("- Request: `%s` `%s`", emptyAsNone(record.BackendRequestKind), emptyAsNone(record.BackendRequestType)))
			if record.BackendConsumes != "" {
				b.WriteString(fmt.Sprintf(" consumes `%s`", record.BackendConsumes))
			}
			b.WriteString("\n")
		}
		if record.BackendReturnType != "" {
			b.WriteString(fmt.Sprintf("- Returns: `%s`\n", record.BackendReturnType))
		}
		if len(record.BackendRequestFields) > 0 {
			b.WriteString("- Request fields:")
			for _, field := range record.BackendRequestFields {
				required := ""
				if field.Required {
					required = " required"
				}
				b.WriteString(fmt.Sprintf(" `%s:%s%s`", field.Name, field.Type, required))
			}
			b.WriteString("\n")
		}
		if len(record.BackendResponseFields) > 0 {
			b.WriteString("- Response fields:")
			for _, field := range record.BackendResponseFields {
				b.WriteString(fmt.Sprintf(" `%s:%s`", field.Name, field.Type))
			}
			b.WriteString("\n")
		}
		for _, auth := range record.Auth {
			b.WriteString(fmt.Sprintf("- Auth: `%s` `%s`\n", auth.Kind, auth.Expression))
		}
		for _, step := range record.PersistencePath {
			b.WriteString(fmt.Sprintf("- Persistence: `%s.%s` entity `%s` table `%s`\n", step.Repository, step.Method, step.Entity, step.Table))
		}
		for _, risk := range record.FieldRisks {
			b.WriteString(fmt.Sprintf("- Risk: `%s` field `%s` - %s\n", risk.Kind, risk.Field, risk.Reason))
		}
		b.WriteString("- Backend steps:\n")
		if len(record.BackendSteps) == 0 {
			b.WriteString("  - none resolved\n")
		} else {
			for _, step := range record.BackendSteps {
				b.WriteString(fmt.Sprintf("  - `%s.%s`", step.Owner, step.Method))
				if step.Kind != "" {
					b.WriteString(fmt.Sprintf(" (%s)", step.Kind))
				}
				if step.File != "" {
					b.WriteString(fmt.Sprintf(" - `%s:%d`", step.File, step.Line))
				}
				b.WriteString(fmt.Sprintf(" - %s\n", step.Confidence))
			}
		}
		b.WriteString("- Tests:\n")
		if len(record.Tests) == 0 {
			if record.TestReason != "" {
				b.WriteString(fmt.Sprintf("  - none detected (%s)\n", record.TestReason))
			} else {
				b.WriteString("  - none detected\n")
			}
		} else {
			for _, test := range record.Tests {
				label := qualifiedName(test.TestClass, test.TestMethod)
				b.WriteString(fmt.Sprintf("  - `%s` in `%s` (%s", emptyAsNone(label), test.TestFile, test.Confidence))
				if test.HTTPMethod != "" && test.Path != "" {
					b.WriteString(fmt.Sprintf(", %s `%s`", test.HTTPMethod, test.Path))
				}
				if test.TestCase != "" && test.TestCase != "unspecified" {
					b.WriteString(fmt.Sprintf(", case `%s`", test.TestCase))
				}
				if test.StatusExpectation != "" {
					b.WriteString(fmt.Sprintf(", status `%s`", test.StatusExpectation))
				}
				b.WriteString(")\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func updateWorkspaceProjectDiagnostics(layout OutputLayout, projectPath string, matches []WorkspaceContractMatchRecord, writeReport bool) error {
	path := layout.Index("diagnostics.json")
	var diagnostics DiagnosticsRecord
	if err := readWorkspaceJSON(path, &diagnostics); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var resolved []WorkspaceContractMatchRecord
	indexedServices := map[string]bool{}
	workspaceIssues := map[string]WorkspaceContractMatchRecord{}
	for _, match := range matches {
		if match.Issue != contractIssueMatched {
			if match.APIProject == projectPath && match.ServiceCandidate != "" && match.Issue != contractIssueUnscanned {
				indexedServices[match.ServiceCandidate] = true
			}
		} else {
			if match.APIProject == projectPath || match.BackendProject == projectPath {
				resolved = append(resolved, match)
			}
			if match.APIProject == projectPath {
				if match.BackendService != "" {
					indexedServices[match.BackendService] = true
				}
				if match.ServiceCandidate != "" {
					indexedServices[match.ServiceCandidate] = true
				}
			}
		}
		if match.APIProject == projectPath {
			workspaceIssues[workspaceContractDiagnosticKey(match.APIHTTPMethod, match.APIPath, match.APIFile, match.APILine)] = match
		}
	}
	diagnostics.WorkspaceResolvedContracts = resolved
	if len(workspaceIssues) > 0 {
		for i, contract := range diagnostics.RiskyContracts {
			match, ok := workspaceIssues[workspaceContractDiagnosticKey(contract.APIHTTPMethod, contract.APIPath, contract.APIFile, contract.APILine)]
			if !ok || match.Issue == contractIssueUnscanned {
				continue
			}
			diagnostics.RiskyContracts[i].Issue = match.Issue
			diagnostics.RiskyContracts[i].Confidence = match.Confidence
			diagnostics.RiskyContracts[i].ConfidenceScore = match.ConfidenceScore
			diagnostics.RiskyContracts[i].Reason = match.Reason
			diagnostics.RiskyContracts[i].BackendHTTPMethod = match.BackendHTTPMethod
			diagnostics.RiskyContracts[i].BackendPath = match.BackendPath
			diagnostics.RiskyContracts[i].BackendHandler = match.BackendHandler
			diagnostics.RiskyContracts[i].BackendFile = match.BackendFile
			diagnostics.RiskyContracts[i].BackendLine = match.BackendLine
		}
	}
	if len(indexedServices) > 0 {
		var filtered []DiagnosticServiceRecord
		for _, service := range diagnostics.UnscannedServices {
			if indexedServices[service.Service] {
				continue
			}
			filtered = append(filtered, service)
		}
		diagnostics.UnscannedServices = filtered
	}
	if err := writeJSON(path, diagnostics); err != nil {
		return err
	}
	if !writeReport {
		return nil
	}
	return os.WriteFile(layout.Dashboard("diagnostics.md"), []byte(renderDiagnosticsReport(diagnostics)), 0o644)
}

func workspaceContractDiagnosticKey(method, path, file string, line int) string {
	return strings.ToUpper(method) + "\x00" + path + "\x00" + file + "\x00" + fmt.Sprint(line)
}

func updateWorkspaceEndpointConsumers(layout OutputLayout, projectPath string, matches []WorkspaceContractMatchRecord) error {
	path := layout.Dashboard("endpoints.md")
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	updated := stripMarkdownSection(string(body), "## Frontend Consumers") + renderEndpointFrontendConsumersSection(projectPath, matches)
	return os.WriteFile(path, []byte(updated), 0o644)
}

func workspaceIndexFiles(target BuildTarget) []string {
	names := []string{
		"registry.json",
		"context.json",
		"contract-matches.json",
		"feature-flows.json",
		"api-catalog.json",
		"data-flows.json",
		"feature-dossiers.json",
		"workspace-graph.json",
		"workspace-service-map.json",
		"workspace-endpoint-traces.json",
		"directed-traces.json",
		"freshness.json",
	}
	if target.IncludesDashboard() {
		names = append(names, workspaceSymbolGeneratedFiles...)
	}
	return prefixedGeneratedFiles("index", names)
}

func workspaceDashboardFiles(assets map[string][]byte) []string {
	files := prefixedGeneratedFiles("dashboard", []string{
		"workspace-map.html",
		"workspace-context.md",
		"contract-matches.md",
		"feature-flows.md",
		"feature-dossiers.md",
		"next-actions.md",
	})
	for name := range assets {
		files = append(files, filepath.ToSlash(filepath.Join("dashboard", name)))
	}
	sort.Strings(files)
	return files
}

func renderEndpointFrontendConsumersSection(projectPath string, records []WorkspaceContractMatchRecord) string {
	var b strings.Builder
	b.WriteString("\n## Frontend Consumers\n\n")
	count := 0
	for _, record := range records {
		if record.BackendProject != projectPath || record.Issue != contractIssueMatched {
			continue
		}
		count++
		b.WriteString(fmt.Sprintf("- %s `%s` used by `%s` `%s:%d`%s -> `%s.%s`\n",
			record.BackendHTTPMethod,
			record.BackendPath,
			record.APIProject,
			record.APIFile,
			record.APILine,
			workspaceCallerSuffix(record.APICaller),
			record.BackendService,
			record.BackendHandler,
		))
	}
	if count == 0 {
		b.WriteString("- none detected\n")
	}
	return b.String()
}

func stripMarkdownSection(body, heading string) string {
	index := strings.Index(body, heading)
	if index == -1 {
		if strings.HasSuffix(body, "\n") {
			return body
		}
		return body + "\n"
	}
	return strings.TrimRight(body[:index], "\n") + "\n"
}

// WorkspaceStatus renders the auto-detected workspace without scanning or writing files.
func WorkspaceStatus(root string, cfg config.Config) (string, error) {
	currentAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	workspaceRoot, ok, err := resolveWorkspaceRoot(currentAbs, cfg.WorkspaceRoot)
	if err != nil {
		return "", err
	}
	if !ok {
		return "No GoreGraph workspace detected.\n", nil
	}
	projects, err := discoverWorkspaceProjects(workspaceRoot, currentAbs, cfg.OutputDir)
	if err != nil {
		return "", err
	}
	indexed, err := loadWorkspaceIndexes(projects)
	if err != nil {
		return "", err
	}
	registry := WorkspaceRegistryRecord{
		Root:     filepath.ToSlash(workspaceRoot),
		Current:  workspaceRel(workspaceRoot, currentAbs),
		Projects: projects,
	}
	return renderWorkspaceContextReport(buildWorkspaceContext(registry, indexed)), nil
}

func readWorkspaceJSON(path string, dest any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func workspaceRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func workspaceFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
