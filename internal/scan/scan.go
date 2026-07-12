package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

const (
	ToolName      = "goregraph"
	SchemaVersion = 1
)

var GeneratedFiles = []string{
	"manifest.json",
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
	"service-dependencies.json",
	"frontend-usage.json",
	"contract-matches.json",
	"diagnostics.json",
	"diagnostics-canonical.json",
	"package-graph.json",
	"maven-graph.json",
	"analyzers.json",
	"evidence.json",
	"capabilities.json",
	"coverage.json",
	"spring.json",
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
	"workspace-contract-matches.json",
	"workspace-contract-matches.md",
	"workspace-feature-flows.json",
	"workspace-feature-flows.md",
	"workspace-feature-dossiers.json",
	"workspace-feature-dossiers.md",
	"workspace-graph.json",
	"workspace-service-map.json",
	"workspace-endpoint-traces.json",
	"directed-traces.json",
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
	"agent-guide.md",
	"affected.md",
	"audit.json",
	"report.md",
	"modules.md",
	"entrypoints.md",
	"test-map.md",
}

func Run(root string, cfg config.Config) (Result, error) {
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

	matcher := gitignore.Matcher{}
	if cfg.UseGitignore {
		matcher = gitignore.Load(resolved)
	}

	index, skipped, err := scanProject(resolved, cfg, matcher)
	if err != nil {
		return Result{}, err
	}
	sortIndex(&index)

	out := filepath.Join(resolved, cfg.OutputDir)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return Result{}, err
	}
	if err := writeOutputs(out, resolved, cfg, index, skipped, started); err != nil {
		return Result{}, err
	}
	if _, err := ReconcileWorkspace(resolved, cfg); err != nil {
		return Result{}, err
	}

	return Result{ScannedFiles: len(index.Files), SkippedFiles: skipped, OutputDir: out}, nil
}

func scanProject(root string, cfg config.Config, matcher gitignore.Matcher) (Index, int, error) {
	var index Index
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
			index.JavaSources = append(index.JavaSources, extractJavaSource(record, text))
		}
		mergeCodeIntelligence(&index.Code, extractCodeIntelligence(record, text))
		mergeWorkspaceIndex(&index.Workspace, extractWorkspaceRecord(record, text))
		return nil
	})
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

func writeOutputs(out, root string, cfg config.Config, index Index, skipped int, started time.Time) error {
	graph := buildGraph(index.Files, index.Symbols, index.Relations)
	springIndex := buildSpringIndex(index.JavaSources)
	callGraph := buildJavaCallGraph(index.JavaSources)
	callGraph = mergeCallGraphs(callGraph, buildGenericCallGraph(index.Code))
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
	capabilities := BuildCapabilityInventory(index.Files, index.Workspace)
	coverage := BuildCoverage(index.Files, capabilities)
	richSymbols := buildRichSymbols(index.Files, index.Symbols)
	richRelations := buildRichRelations(index.Files, index.Relations)
	richGraph := buildRichGraph(index.Files, richSymbols, richRelations)
	evidence := LinkEvidenceReferences(filepath.Base(root), index.Files, richRelations, &callGraph, routes, codeFlows, contractMatches)
	canonicalDiagnostics := BuildCanonicalDiagnostics(contractMatches, capabilities)
	finished := time.Now().UTC()
	manifest := Manifest{
		Tool:        ToolName,
		Schema:      SchemaVersion,
		OutputDir:   cfg.OutputDir,
		Files:       len(index.Files),
		Skipped:     skipped,
		Generated:   GeneratedFiles,
		ProjectRoot: filepath.Base(root),
		GeneratedAt: finished.Format(time.RFC3339),
		Git:         readGitMetadata(root),
	}
	audit := newAuditRecord(root, cfg.OutputDir, started, finished, len(index.Files), skipped)
	writes := []struct {
		name  string
		value any
	}{
		{"manifest.json", manifest},
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
		{"service-dependencies.json", serviceDependencies},
		{"frontend-usage.json", frontendUsage},
		{"contract-matches.json", contractMatches},
		{"diagnostics.json", diagnostics},
		{"diagnostics-canonical.json", canonicalDiagnostics},
		{"workspace-contract-matches.json", []WorkspaceContractMatchRecord{}},
		{"workspace-feature-flows.json", []WorkspaceFeatureFlowRecord{}},
		{"workspace-feature-dossiers.json", []FeatureDossierRecord{}},
		{"workspace-graph.json", WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Nodes: []WorkspaceGraphNodeRecord{}, Edges: []WorkspaceGraphEdgeRecord{}}},
		{"workspace-service-map.json", WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, Nodes: []WorkspaceServiceNodeRecord{}, Edges: []WorkspaceServiceEdgeRecord{}}},
		{"workspace-endpoint-traces.json", WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Traces: []WorkspaceEndpointTraceRecord{}}},
		{"directed-traces.json", DirectedTraceIndexRecord{SchemaVersion: SchemaVersion, Traces: []DirectedTraceRecord{}}},
		{"package-graph.json", packageGraph},
		{"maven-graph.json", mavenGraph},
		{"analyzers.json", analyzers},
		{"evidence.json", evidence},
		{"capabilities.json", capabilities},
		{"coverage.json", coverage},
		{"spring.json", springIndex},
		{"audit.json", audit},
	}
	for _, write := range writes {
		if err := writeJSON(filepath.Join(out, write.name), write.value); err != nil {
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
		{"agent-guide.md", renderAgentGuideEntry()},
		{"affected.md", renderAffectedReport(richGraph)},
		{"entrypoints.md", renderEntrypointsReport(index.Files, index.Symbols, springIndex)},
		{"test-map.md", renderTestMapReport(index.Relations, testMap)},
	}
	for _, report := range reports {
		if err := os.WriteFile(filepath.Join(out, report.name), []byte(report.body), 0o644); err != nil {
			return err
		}
	}
	return nil
}
