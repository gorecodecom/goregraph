package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

const (
	ToolName      = "goregraph"
	SchemaVersion = 3
)

var IndexGeneratedFiles = []string{
	"freshness.json",
	"files.json",
	"symbols.json",
	"relations.json",
	"graph.json",
	"symbols-full.json",
	"relations-full.json",
	"graph-full.json",
	"callgraph.json",
	"endpoint-flows.json",
	"test-map.json",
	"routes.json",
	"flows.json",
	"api-contracts.json",
	"architecture-capabilities.json",
	"service-dependencies.json",
	"frontend-usage.json",
	"contract-matches.json",
	"diagnostics.json",
	"diagnostics-canonical.json",
	"diagnostic-families.json",
	"package-graph.json",
	"maven-graph.json",
	"analyzers.json",
	"evidence.json",
	"capabilities.json",
	"coverage.json",
	"spring.json",
	"audit.json",
	"workspace-contract-matches.json",
	"workspace-feature-flows.json",
	"workspace-feature-dossiers.json",
	"workspace-graph.json",
	"workspace-service-map.json",
	"workspace-endpoint-traces.json",
	"directed-traces.json",
	"data-flows.json",
}

var AgentGeneratedFiles = []string{
	"agent-guide.md",
}

var DashboardGeneratedFiles = []string{
	"workspace.md",
	"endpoints.md",
	"endpoint-flows.md",
	"dependencies.md",
	"callgraph.md",
	"routes.md",
	"flows.md",
	"api-contracts.md",
	"frontend-usage.md",
	"contract-matches.md",
	"potentially-broken-contracts.md",
	"diagnostics.md",
	"workspace-context.md",
	"workspace-contract-matches.md",
	"workspace-feature-flows.md",
	"workspace-feature-dossiers.md",
	"data-flows.md",
	"workspace-map.md",
	"workspace-next-actions.md",
	"frontend-consumers.md",
	"package-graph.md",
	"maven-graph.md",
	"navigation.md",
	"analyzers.md",
	"coverage.md",
	"workspace-summary.md",
	"architecture.md",
	"affected.md",
	"report.md",
	"modules.md",
	"entrypoints.md",
	"test-map.md",
}

var GeneratedFiles = generatedFilesForTarget(BuildTargetAll)

var workspaceSymbolGeneratedFiles = []string{
	"symbol-index.json",
	"symbol-usages.json",
}

func Run(root string, cfg config.Config) (Result, error) {
	return RunBuild(root, cfg, BuildTargetAll)
}

type projectExtractorFunc func(string, config.Config, gitignore.Matcher) (Index, int, error)

var projectExtractor projectExtractorFunc = scanProject

func replaceProjectExtractorForTest(replacement projectExtractorFunc) func() {
	previous := projectExtractor
	projectExtractor = replacement
	return func() {
		projectExtractor = previous
	}
}

func RunBuild(root string, cfg config.Config, target BuildTarget) (Result, error) {
	if err := target.Validate(); err != nil {
		return Result{}, err
	}
	started := time.Now().UTC()
	resolved, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Result{}, err
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("scan root %q is not a directory", root)
	}
	out := filepath.Join(resolved, cfg.OutputDir)
	if legacyGeneratedOutputExists(out) {
		return Result{}, fmt.Errorf("legacy pre-1.3.0 output detected; run `goregraph clean %s --execute` and `goregraph build all %s`", root, root)
	}

	matcher := gitignore.Matcher{}
	if cfg.UseGitignore {
		matcher = gitignore.Load(resolved)
	}

	index, skipped, err := projectExtractor(resolved, cfg, matcher)
	if err != nil {
		return Result{}, err
	}
	sortIndex(&index)

	if err := os.MkdirAll(out, 0o755); err != nil {
		return Result{}, err
	}
	if err := writeOutputs(out, resolved, cfg, index, skipped, started, target); err != nil {
		return Result{}, err
	}
	if _, err := ReconcileWorkspaceTarget(resolved, cfg, target); err != nil {
		return Result{}, err
	}
	layout := NewProjectOutputLayout(out)
	if manifest := readCurrentOutputManifest(layout.Manifest); manifest.Tool == ToolName {
		if err := writeOutputManifestAtomic(layout.Manifest, manifest); err != nil {
			return Result{}, err
		}
	}

	return Result{ScannedFiles: len(index.Files), SkippedFiles: skipped, OutputDir: out}, nil
}

func legacyGeneratedOutputExists(out string) bool {
	for _, name := range []string{"routes.json", "report.md", "workspace-map.html", "context-index.json"} {
		if info, err := os.Stat(filepath.Join(out, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func scanProject(root string, cfg config.Config, matcher gitignore.Matcher) (Index, int, error) {
	var index Index
	javaBodies := map[string]string{}
	scriptBodies := map[string]string{}
	skipped := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			skipped++
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			skipped++
			return nil
		}
		rel = filepath.ToSlash(rel)

		info, err := entry.Info()
		if err != nil {
			skipped++
			return nil
		}
		if shouldSkipPath(rel, entry.IsDir(), cfg, matcher) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			skipped++
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 && !cfg.FollowSymlinks {
			skipped++
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if info.Size() > cfg.MaxFileSizeBytes {
			skipped++
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			skipped++
			return nil
		}
		if isBinary(body) {
			skipped++
			return nil
		}

		record := fileRecord(rel, info.Size(), body)
		index.Files = append(index.Files, record)
		text := string(body)
		index.Symbols = append(index.Symbols, extractSymbols(record, text)...)
		index.Relations = append(index.Relations, extractRelations(record, text)...)
		if record.Language == "java" {
			source := extractJavaSource(record, text)
			index.JavaSources = append(index.JavaSources, source)
			javaBodies[source.File] = text
		}
		if record.Language == "javascript" || record.Language == "typescript" {
			scriptBodies[record.Path] = text
		}
		if base := filepath.Base(record.Path); base == "tsconfig.json" || base == "jsconfig.json" {
			if scriptConfig, ok := ExtractScriptResolutionConfig(record.Path, text); ok {
				if index.ScriptConfigs == nil {
					index.ScriptConfigs = map[string]ScriptResolutionConfig{}
				}
				index.ScriptConfigs[record.Path] = scriptConfig
			} else {
				index.scriptConfigLimitations = append(index.scriptConfigLimitations, record.Path+": malformed script resolution config")
			}
		}
		mergeCodeIntelligence(&index.Code, extractCodeIntelligence(record, text))
		index.ArchitectureCapabilities = append(index.ArchitectureCapabilities, extractArchitectureCapabilityFacts(record, text)...)
		mergeWorkspaceIndex(&index.Workspace, extractWorkspaceRecord(record, text))
		return nil
	})
	if err == nil {
		javaFacts := ExtractJavaProjectSymbolFacts(index.JavaSources, javaBodies, index.Workspace)
		var scriptFacts ProjectSymbolFacts
		for _, file := range index.Files {
			if file.Language != "javascript" && file.Language != "typescript" {
				continue
			}
			MergeProjectSymbolFacts(&scriptFacts, ExtractScriptSymbolFacts(file, scriptBodies[file.Path]))
		}
		scriptFacts = ResolveScriptSymbolFacts(index.Files, index.Workspace.NodePackages, index.ScriptConfigs, scriptFacts)
		if len(index.scriptConfigLimitations) > 0 {
			for factIndex := range scriptFacts.Declarations {
				scriptFacts.Declarations[factIndex].Coverage = CoveragePartial
				scriptFacts.Declarations[factIndex].Limitations = append([]string(nil), index.scriptConfigLimitations...)
			}
		}
		index.SymbolFacts = javaFacts
		MergeProjectSymbolFacts(&index.SymbolFacts, scriptFacts)
		index.SymbolFacts = FinalizeProjectSymbolFacts(index.Files, index.Workspace, index.SymbolFacts)
	}
	return index, skipped, err
}

func fileRecord(rel string, size int64, body []byte) FileRecord {
	sum := sha256.Sum256(body)
	return FileRecord{
		Path:     rel,
		Language: detectLanguage(rel),
		Size:     size,
		Hash:     hex.EncodeToString(sum[:]),
		Kind:     detectKind(rel),
	}
}

func sortIndex(index *Index) {
	sortArchitectureCapabilityFacts(index.ArchitectureCapabilities)
	sort.Slice(index.Files, func(i, j int) bool { return index.Files[i].Path < index.Files[j].Path })
	sort.Slice(index.Symbols, func(i, j int) bool {
		if index.Symbols[i].File != index.Symbols[j].File {
			return index.Symbols[i].File < index.Symbols[j].File
		}
		if index.Symbols[i].Line != index.Symbols[j].Line {
			return index.Symbols[i].Line < index.Symbols[j].Line
		}
		if index.Symbols[i].Kind != index.Symbols[j].Kind {
			return index.Symbols[i].Kind < index.Symbols[j].Kind
		}
		return index.Symbols[i].Name < index.Symbols[j].Name
	})
	index.Relations = append(index.Relations, buildTestRelations(index.Files)...)
	index.Relations = append(index.Relations, javaTestRelations(index.JavaSources)...)
	index.Relations = replaceJavaImportRelations(index.Relations, resolveJavaImportRelations(index.JavaSources))
	resolveLocalImportRelations(index)
	sort.Slice(index.Relations, func(i, j int) bool {
		if index.Relations[i].From != index.Relations[j].From {
			return index.Relations[i].From < index.Relations[j].From
		}
		if index.Relations[i].To != index.Relations[j].To {
			return index.Relations[i].To < index.Relations[j].To
		}
		if index.Relations[i].Type != index.Relations[j].Type {
			return index.Relations[i].Type < index.Relations[j].Type
		}
		return index.Relations[i].Line < index.Relations[j].Line
	})
}

func writeOutputs(out, root string, cfg config.Config, index Index, skipped int, started time.Time, target BuildTarget) error {
	graph := buildGraph(index.Files, index.Symbols, index.Relations)
	springIndex := buildSpringIndex(index.JavaSources)
	callGraph := buildJavaCallGraph(index.JavaSources)
	callGraph = mergeCallGraphs(callGraph, buildGenericCallGraph(index.Code))
	linkCallGraphSymbolFacts(&callGraph, index.SymbolFacts)
	index.Relations = append(index.Relations, buildCallRelations(callGraph)...)
	sort.Slice(index.Relations, func(i, j int) bool {
		if index.Relations[i].From != index.Relations[j].From {
			return index.Relations[i].From < index.Relations[j].From
		}
		if index.Relations[i].To != index.Relations[j].To {
			return index.Relations[i].To < index.Relations[j].To
		}
		if index.Relations[i].Type != index.Relations[j].Type {
			return index.Relations[i].Type < index.Relations[j].Type
		}
		return index.Relations[i].Line < index.Relations[j].Line
	})
	graph = buildGraph(index.Files, index.Symbols, index.Relations)
	endpointFlows := buildEndpointFlows(springIndex, callGraph)
	codeFlows := buildCodeFlows(index.Code, springIndex, endpointFlows, callGraph)
	testMap := append(buildJavaTestMap(index.JavaSources, springIndex.Endpoints), buildGenericTestMap(index.Code)...)
	routes := buildCodeRoutes(index.Code, springIndex)
	frontendUsage := buildFrontendUsage(index.Code.APIContracts, codeFlows)
	contractMatches := buildContractMatches(index.Code.APIContracts, routes)
	serviceDependencies := buildServiceDependencies(WorkspaceProjectRecord{}, index.JavaSources)
	diagnostics := buildDiagnostics(routes, contractMatches, endpointFlows, codeFlows, testMap)
	packageGraph := buildPackageGraph(index.Workspace)
	mavenGraph := buildMavenGraph(index.Workspace)
	analyzers := buildAnalyzerInventory(index.Files, index.Workspace)
	prefixAnalyzerOutputPaths(analyzers)
	capabilities := BuildCapabilityInventory(index.Files, index.Workspace, index.ArchitectureCapabilities)
	coverage := BuildCoverage(index.Files, capabilities)
	richSymbols := dedupeRichSymbolFacts(append(buildRichSymbols(index.Files, index.Symbols), index.SymbolFacts.Declarations...))
	richRelations := dedupeRichRelationFacts(append(buildRichRelations(index.Files, index.Relations), index.SymbolFacts.References...))
	richGraph := buildRichGraph(index.Files, richSymbols, richRelations)
	evidence := LinkEvidenceReferences(filepath.Base(root), index.Files, richSymbols, richRelations, &callGraph, routes, codeFlows, contractMatches)
	canonicalDiagnostics := BuildCanonicalDiagnostics(contractMatches, capabilities)
	diagnosticFamilies := BuildDiagnosticFamilies(filepath.Base(root), canonicalDiagnostics)
	finished := time.Now().UTC()
	layout := NewProjectOutputLayout(out)
	previous := readCurrentOutputManifest(layout.Manifest)
	previous.Agent = validProjectionStatus(layout.Root, previous.Agent)
	previous.Dashboard = validProjectionStatus(layout.Root, previous.Dashboard)
	manifest := OutputManifest{
		Tool:        ToolName,
		Schema:      SchemaVersion,
		Scope:       "project",
		OutputDir:   cfg.OutputDir,
		Files:       len(index.Files),
		Skipped:     skipped,
		ProjectRoot: filepath.Base(root),
		Git:         readGitMetadata(root),
		Index: ProjectionStatus{
			GeneratedAt: finished.Format(time.RFC3339),
			Complete:    true,
			Files:       prefixedGeneratedFiles("index", IndexGeneratedFiles),
		},
		Agent:     previous.Agent,
		Dashboard: previous.Dashboard,
	}
	if target.IncludesAgent() {
		manifest.Agent = ProjectionStatus{
			GeneratedAt: finished.Format(time.RFC3339),
			Complete:    true,
			Files:       prefixedGeneratedFiles("agent", AgentGeneratedFiles),
		}
	}
	if target.IncludesDashboard() {
		manifest.Dashboard = ProjectionStatus{
			GeneratedAt: finished.Format(time.RFC3339),
			Complete:    true,
			Files:       prefixedGeneratedFiles("dashboard", DashboardGeneratedFiles),
		}
	}
	freshness := BuildArtifactFreshness(manifest.Schema, finished.Format(time.RFC3339), index.Files, prefixedGeneratedFiles("index", IndexGeneratedFiles))
	audit := newAuditRecord(root, cfg.OutputDir, started, finished, len(index.Files), skipped, generatedFilesForTarget(target))
	writes := []struct {
		name  string
		value any
	}{
		{"freshness.json", freshness},
		{"files.json", index.Files},
		{"symbols.json", index.Symbols},
		{"relations.json", index.Relations},
		{"graph.json", graph},
		{"symbols-full.json", richSymbols},
		{"relations-full.json", richRelations},
		{"graph-full.json", richGraph},
		{"callgraph.json", callGraph},
		{"endpoint-flows.json", endpointFlows},
		{"test-map.json", testMap},
		{"routes.json", routes},
		{"flows.json", codeFlows},
		{"api-contracts.json", index.Code.APIContracts},
		{"architecture-capabilities.json", index.ArchitectureCapabilities},
		{"service-dependencies.json", serviceDependencies},
		{"frontend-usage.json", frontendUsage},
		{"contract-matches.json", contractMatches},
		{"diagnostics.json", diagnostics},
		{"diagnostics-canonical.json", canonicalDiagnostics},
		{"diagnostic-families.json", diagnosticFamilies},
		{"workspace-contract-matches.json", []WorkspaceContractMatchRecord{}},
		{"workspace-feature-flows.json", []WorkspaceFeatureFlowRecord{}},
		{"workspace-feature-dossiers.json", []FeatureDossierRecord{}},
		{"workspace-graph.json", WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Nodes: []WorkspaceGraphNodeRecord{}, Edges: []WorkspaceGraphEdgeRecord{}}},
		{"workspace-service-map.json", WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Nodes: []WorkspaceServiceNodeRecord{}, Edges: []WorkspaceServiceEdgeRecord{}}},
		{"workspace-endpoint-traces.json", WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Traces: []WorkspaceEndpointTraceRecord{}}},
		{"directed-traces.json", DirectedTraceIndexRecord{SchemaVersion: SchemaVersion, Traces: []DirectedTraceRecord{}}},
		{"data-flows.json", []DataFlowRecord{}},
		{"package-graph.json", packageGraph},
		{"maven-graph.json", mavenGraph},
		{"analyzers.json", analyzers},
		{"evidence.json", evidence},
		{"capabilities.json", capabilities},
		{"coverage.json", coverage},
		{"spring.json", springIndex},
		{"audit.json", audit},
	}
	if err := os.RemoveAll(filepath.Join(out, "index")); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(out, "index"), 0o755); err != nil {
		return err
	}
	for _, write := range writes {
		if err := writeJSON(layout.Index(write.name), write.value); err != nil {
			return err
		}
	}

	reports := []struct {
		name string
		body string
	}{
		{"report.md", renderReport(root, index.Files, skipped)},
		{"modules.md", renderModulesReport(index.Files)},
		{"workspace.md", renderWorkspaceReport(index.Workspace)},
		{"endpoints.md", renderEndpointsReport(springIndex)},
		{"endpoint-flows.md", renderEndpointFlowsReport(endpointFlows)},
		{"dependencies.md", renderDependenciesReport(springIndex)},
		{"callgraph.md", renderCallGraphReport(callGraph)},
		{"routes.md", renderRoutesReport(routes)},
		{"flows.md", renderCodeFlowsReport(codeFlows)},
		{"api-contracts.md", renderAPIContractsReport(index.Code.APIContracts)},
		{"frontend-usage.md", renderFrontendUsageReport(frontendUsage)},
		{"contract-matches.md", renderContractMatchesReport(contractMatches)},
		{"potentially-broken-contracts.md", renderPotentiallyBrokenContractsReport(contractMatches)},
		{"diagnostics.md", renderCanonicalDiagnosticsEntry(canonicalDiagnostics) + "\n## Detailed legacy-compatible diagnostics\n\n" + renderDiagnosticsReport(diagnostics)},
		{"workspace-context.md", renderNoWorkspaceContextReport()},
		{"workspace-contract-matches.md", renderProjectWorkspaceMatchesReport("", nil)},
		{"workspace-feature-flows.md", renderWorkspaceFeatureFlowsReport(nil)},
		{"workspace-feature-dossiers.md", renderFeatureDossiersReport(nil)},
		{"workspace-map.md", "Workspace map is available after workspace reconciliation.\n"},
		{"workspace-next-actions.md", renderNoWorkspaceNextActionsReport()},
		{"frontend-consumers.md", renderFrontendConsumersReport("", nil)},
		{"package-graph.md", renderPackageGraphReport(packageGraph)},
		{"maven-graph.md", renderMavenGraphReport(mavenGraph)},
		{"navigation.md", renderNavigationReport(index.Files, index.Symbols, index.Relations, routes, codeFlows, testMap, analyzers)},
		{"analyzers.md", renderAnalyzersReport(analyzers)},
		{"coverage.md", RenderCoverageReport(coverage)},
		{"workspace-summary.md", renderWorkspaceSummaryEntry(filepath.Base(root), len(index.Files), coverage)},
		{"architecture.md", renderArchitectureEntry(routes, richRelations)},
		{"data-flows.md", RenderDataFlowsReport(nil)},
		{"affected.md", renderAffectedReport(richGraph)},
		{"entrypoints.md", renderEntrypointsReport(index.Files, index.Symbols, springIndex)},
		{"test-map.md", renderTestMapReport(index.Relations, testMap)},
	}
	if target.IncludesDashboard() {
		if err := os.RemoveAll(filepath.Join(out, "dashboard")); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(out, "dashboard"), 0o755); err != nil {
			return err
		}
	}
	for _, report := range reports {
		if !target.IncludesDashboard() {
			break
		}
		if err := os.WriteFile(layout.Dashboard(report.name), []byte(report.body), 0o644); err != nil {
			return err
		}
	}
	if target.IncludesAgent() {
		if err := os.RemoveAll(filepath.Join(out, "agent")); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(out, "agent"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(layout.Agent("agent-guide.md"), []byte(renderAgentGuideEntry()), 0o644); err != nil {
			return err
		}
	}
	return writeOutputManifestAtomic(layout.Manifest, manifest)
}

func generatedFilesForTarget(target BuildTarget) []string {
	files := prefixedGeneratedFiles("index", IndexGeneratedFiles)
	if target.IncludesAgent() {
		files = append(files, prefixedGeneratedFiles("agent", AgentGeneratedFiles)...)
	}
	if target.IncludesDashboard() {
		files = append(files, prefixedGeneratedFiles("dashboard", DashboardGeneratedFiles)...)
	}
	return append([]string{"manifest.json"}, files...)
}

func prefixedGeneratedFiles(dir string, names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, filepath.ToSlash(filepath.Join(dir, name)))
	}
	return result
}

func readCurrentOutputManifest(path string) OutputManifest {
	var manifest OutputManifest
	body, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(body, &manifest) != nil ||
		manifest.Tool != ToolName || manifest.Schema != SchemaVersion {
		return OutputManifest{}
	}
	return manifest
}

func validProjectionStatus(root string, status ProjectionStatus) ProjectionStatus {
	if !status.Complete {
		return ProjectionStatus{}
	}
	for _, name := range status.Files {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || info.IsDir() {
			return ProjectionStatus{}
		}
	}
	return status
}

func mergeGeneratedPaths(groups ...[]string) []string {
	seen := map[string]bool{}
	var result []string
	for _, group := range groups {
		for _, name := range group {
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}
	sort.Strings(result)
	return result
}

func prefixAnalyzerOutputPaths(records []AnalyzerRecord) {
	for recordIndex := range records {
		for outputIndex, name := range records[recordIndex].Outputs {
			if strings.HasSuffix(name, ".json") {
				records[recordIndex].Outputs[outputIndex] = filepath.ToSlash(filepath.Join("index", name))
			} else {
				records[recordIndex].Outputs[outputIndex] = filepath.ToSlash(filepath.Join("dashboard", name))
			}
		}
	}
}

func writeOutputManifestAtomic(path string, manifest OutputManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
