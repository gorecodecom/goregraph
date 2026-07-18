package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

type Result struct {
	Lines    []string
	Failures int
	Warnings int
}

func Run(root string) (Result, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return Result{}, err
	}
	result := Result{Lines: []string{"GoreGraph Doctor", ""}}
	if workspaceDirectoryExists(root) {
		checkWorkspace(root, &result)
		return result, nil
	}

	checkProject(root, cfg, &result)
	workspaceRoot, ok, err := scan.WorkspaceRoot(root, cfg)
	if err != nil {
		return Result{}, err
	}
	if ok && explicitWorkspaceState(workspaceRoot, cfg) {
		checkWorkspace(workspaceRoot, &result)
	}
	return result, nil
}

func checkProject(root string, cfg config.Config, result *Result) {
	out := filepath.Join(root, cfg.OutputDir)
	if info, err := os.Stat(out); err != nil || !info.IsDir() {
		result.fail("output", fmt.Sprintf("%s is missing", cfg.OutputDir))
		result.fix("goregraph scan " + root)
		return
	}
	result.ok("output", cfg.OutputDir+" exists")
	if legacyLayoutExists(out) {
		result.fail("layout", "pre-1.3.0 flat generated output detected")
		result.fix("goregraph clean " + root + " --execute\n  goregraph build all " + root)
		return
	}

	manifest, ok := checkManifest(out, result)
	if !ok {
		result.fix("goregraph scan " + root)
		return
	}

	checkGeneratedFiles(out, manifest, result)
	checkJSONFiles(out, result)
	checkAgentContextIndex(out, manifest, result)
	checkFreshnessIntegrity(out, result)
	checkEvidenceIntegrity(out, result)
	checkAPICatalog(out, manifest, nil, result)
	checkCanonicalFeatureFlows(out, result)
	checkStaleFiles(root, manifest, result)
	if result.Failures > 0 || result.Warnings > 0 {
		result.fix("goregraph scan " + root)
	}
}

func checkCanonicalFeatureFlows(out string, result *Result) {
	path := scan.NewProjectOutputLayout(out).Index("workspace-feature-flows.json")
	var flows []scan.WorkspaceFeatureFlowRecord
	if err := readJSON(path, &flows); err != nil {
		if !os.IsNotExist(err) {
			result.fail("feature-flows", "workspace-feature-flows.json invalid: "+err.Error())
		}
		return
	}
	for _, flow := range flows {
		if err := scan.ValidateCanonicalFeatureFlow(flow); err != nil {
			result.fail("feature-flows", err.Error())
			return
		}
	}
	result.ok("feature-flows", "canonical feature-flow references valid")
}

func checkManifest(out string, result *Result) (scan.Manifest, bool) {
	var manifest scan.Manifest
	if err := readJSON(filepath.Join(out, "manifest.json"), &manifest); err != nil {
		result.fail("manifest", err.Error())
		return scan.Manifest{}, false
	}
	if manifest.Tool != scan.ToolName {
		result.fail("manifest", fmt.Sprintf("tool is %q, want %q", manifest.Tool, scan.ToolName))
		return manifest, false
	}
	result.ok("manifest", "manifest.json valid")

	if manifest.Schema != scan.SchemaVersion {
		if manifest.Schema < scan.SchemaVersion {
			result.fail("schema", fmt.Sprintf("legacy generated-output schema %d detected, want %d; clean generated output and rebuild all projections", manifest.Schema, scan.SchemaVersion))
		} else {
			result.fail("schema", fmt.Sprintf("version %d unsupported, want %d; reinstall GoreGraph", manifest.Schema, scan.SchemaVersion))
		}
		return manifest, false
	}
	result.ok("schema", fmt.Sprintf("version %d supported", manifest.Schema))
	if manifest.Scope != "project" && manifest.Scope != "workspace" {
		result.fail("manifest", fmt.Sprintf("scope is %q, want project or workspace", manifest.Scope))
		return manifest, false
	}
	if !manifest.Index.Complete {
		result.fail("manifest", "canonical index projection is incomplete")
		return manifest, false
	}
	return manifest, true
}

func checkGeneratedFiles(out string, manifest scan.Manifest, result *Result) {
	var expected []string
	for _, projection := range []scan.ProjectionStatus{manifest.Index, manifest.Agent, manifest.Dashboard} {
		if projection.Complete {
			expected = append(expected, projection.Files...)
		}
	}
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			result.fail("files", name+" missing")
			continue
		}
		result.ok("files", name+" present")
	}
}

func checkJSONFiles(out string, result *Result) {
	layout := scan.NewProjectOutputLayout(out)
	checks := []struct {
		name string
		dest any
	}{
		{"files.json", &[]scan.FileRecord{}},
		{"freshness.json", &scan.ArtifactFreshnessIndex{}},
		{"symbols.json", &[]scan.SymbolRecord{}},
		{"relations.json", &[]scan.RelationRecord{}},
		{"graph.json", &scan.Graph{}},
		{"symbols-full.json", &[]scan.RichSymbolRecord{}},
		{"relations-full.json", &[]scan.RichRelationRecord{}},
		{"graph-full.json", &scan.RichGraph{}},
		{"callgraph.json", &scan.CallGraphRecord{}},
		{"endpoint-flows.json", &[]scan.SpringEndpointFlowRecord{}},
		{"test-map.json", &[]scan.TestMapRecord{}},
		{"routes.json", &[]scan.CodeRouteRecord{}},
		{"flows.json", &[]scan.CodeFlowRecord{}},
		{"api-contracts.json", &[]scan.APIContractRecord{}},
		{"architecture-capabilities.json", &[]scan.ArchitectureCapabilityFact{}},
		{"contract-matches.json", &[]scan.ContractMatchRecord{}},
		{"diagnostics.json", &scan.DiagnosticsRecord{}},
		{"diagnostics-canonical.json", &[]scan.CanonicalDiagnosticRecord{}},
		{"diagnostic-families.json", &[]scan.DiagnosticFamilyRecord{}},
		{"package-graph.json", &scan.PackageGraphRecord{}},
		{"maven-graph.json", &scan.MavenGraphRecord{}},
		{"analyzers.json", &[]scan.AnalyzerRecord{}},
		{"evidence.json", &[]scan.EvidenceRecord{}},
		{"capabilities.json", &[]scan.CapabilityRecord{}},
		{"coverage.json", &scan.CoverageRecord{}},
		{"directed-traces.json", &scan.DirectedTraceIndexRecord{}},
		{"data-flows.json", &[]scan.DataFlowRecord{}},
		{"spring.json", &scan.SpringIndex{}},
		{"audit.json", &scan.AuditRecord{}},
	}
	for _, check := range checks {
		if err := readJSON(layout.Index(check.name), check.dest); err != nil {
			result.fail("json", check.name+" invalid: "+err.Error())
			continue
		}
		result.ok("json", check.name+" valid")
	}
}

func checkFreshnessIntegrity(out string, result *Result) {
	var index scan.ArtifactFreshnessIndex
	if err := readJSON(scan.NewProjectOutputLayout(out).Index("freshness.json"), &index); err != nil {
		return
	}
	if index.Schema != scan.SchemaVersion || index.GoreGraphVersion == "" || index.SourceFingerprint == "" || index.GeneratedAt == "" {
		result.fail("freshness", "freshness provenance incomplete")
		return
	}
	for _, record := range index.Artifacts {
		if record.Artifact == "" || record.GeneratedAt == "" || record.GoreGraphVersion != index.GoreGraphVersion || record.Schema != index.Schema || record.SourceFingerprint != index.SourceFingerprint {
			result.fail("freshness", "freshness provenance inconsistent")
			return
		}
		if record.Stale {
			result.warn("freshness", record.Artifact+" stale: "+record.Reason)
		}
	}
	result.ok("freshness", "artifact provenance valid")
}

func checkEvidenceIntegrity(out string, result *Result) {
	layout := scan.NewProjectOutputLayout(out)
	var evidence []scan.EvidenceRecord
	var architectureFacts []scan.ArchitectureCapabilityFact
	var capabilities []scan.CapabilityRecord
	var coverage scan.CoverageRecord
	if readJSON(layout.Index("evidence.json"), &evidence) != nil || readJSON(layout.Index("architecture-capabilities.json"), &architectureFacts) != nil || readJSON(layout.Index("capabilities.json"), &capabilities) != nil || readJSON(layout.Index("coverage.json"), &coverage) != nil {
		return
	}
	known := map[string]bool{}
	for _, record := range evidence {
		if record.ID == "" || known[record.ID] {
			result.fail("evidence", "duplicate or empty evidence ID")
			return
		}
		known[record.ID] = true
	}
	for _, record := range architectureFacts {
		if record.ID == "" || known[record.ID] {
			result.fail("evidence", "duplicate or empty architecture capability evidence ID")
			return
		}
		known[record.ID] = true
	}
	for _, record := range capabilities {
		if err := record.Coverage.Validate(); err != nil {
			result.fail("coverage", "invalid coverage: "+err.Error())
			return
		}
		if danglingEvidence(record.EvidenceIDs, known) {
			result.fail("evidence", "dangling capability evidence reference")
			return
		}
	}
	for _, record := range coverage.Capabilities {
		if err := record.Coverage.Validate(); err != nil {
			result.fail("coverage", "invalid coverage: "+err.Error())
			return
		}
	}
	var routes []scan.CodeRouteRecord
	var calls scan.CallGraphRecord
	var diagnostics []scan.CanonicalDiagnosticRecord
	if readJSON(layout.Index("routes.json"), &routes) == nil {
		for _, route := range routes {
			if danglingEvidence(route.EvidenceIDs, known) {
				result.fail("evidence", "route contains a dangling evidence reference")
				return
			}
		}
	}
	if readJSON(layout.Index("callgraph.json"), &calls) == nil {
		for _, edge := range calls.Edges {
			if danglingEvidence(edge.EvidenceIDs, known) {
				result.fail("evidence", "call edge contains a dangling evidence reference")
				return
			}
		}
	}
	if readJSON(layout.Index("diagnostics-canonical.json"), &diagnostics) == nil {
		for _, diagnostic := range diagnostics {
			if diagnostic.ID == "" || diagnostic.Severity.Validate() != nil || diagnostic.Confidence.Validate() != nil || diagnostic.Resolution.Validate() != nil || danglingEvidence(diagnostic.EvidenceIDs, known) {
				result.fail("diagnostics", "canonical diagnostic is invalid or contains dangling evidence")
				return
			}
		}
	}
	result.ok("evidence", "evidence integrity valid")
}

func danglingEvidence(ids []string, known map[string]bool) bool {
	for _, id := range ids {
		if !known[id] {
			return true
		}
	}
	return false
}

func checkStaleFiles(root string, manifest scan.Manifest, result *Result) {
	out := filepath.Join(root, manifest.OutputDir)
	layout := scan.NewProjectOutputLayout(out)
	var files []scan.FileRecord
	if err := readJSON(layout.Index("files.json"), &files); err != nil {
		return
	}
	stale := 0
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			stale++
			continue
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != file.Hash {
			stale++
		}
	}
	if stale > 0 {
		result.warn("stale", fmt.Sprintf("%d indexed files changed or disappeared", stale))
		return
	}
	result.ok("stale", "indexed file hashes match")
}

func checkWorkspace(root string, result *Result) {
	out := filepath.Join(root, ".goregraph-workspace")
	dashboardConfig, dashboardConfigValid := checkWorkspaceDashboardConfig(root, result)
	if legacyLayoutExists(out) {
		result.fail("layout", "pre-1.3.0 flat workspace output detected")
		result.fix("goregraph workspace clean " + root + " --execute\n  goregraph workspace build all " + root)
		return
	}
	manifest, ok := checkManifest(out, result)
	if !ok {
		result.fix("goregraph workspace build all " + root)
		return
	}
	checkGeneratedFiles(out, manifest, result)
	checkWorkspaceJSONFiles(out, manifest.Index.Files, result)
	var registry scan.WorkspaceRegistryRecord
	if err := readJSON(scan.NewWorkspaceOutputLayout(out).Index("registry.json"), &registry); err == nil {
		checkAPICatalog(out, manifest, &registry, result)
	}
	checkAgentContextIndex(out, manifest, result)
	if manifest.Dashboard.Complete {
		checkWorkspaceSymbolProjection(root, result)
	}
	if dashboardConfigValid {
		checkStaleWorkspaceDashboardServices(out, dashboardConfig, result)
	}
}

func checkAPICatalog(out string, manifest scan.Manifest, registry *scan.WorkspaceRegistryRecord, result *Result) {
	const catalogManifestFile = "index/api-catalog.json"
	if !manifestListsFile(manifest.Index.Files, catalogManifestFile) {
		return
	}

	path := scan.NewProjectOutputLayout(out).Index("api-catalog.json")
	var catalog scan.APICatalogRecord
	if err := readJSON(path, &catalog); err != nil {
		result.fail("api-catalog", "api-catalog.json invalid: "+err.Error())
		return
	}
	if catalog.SchemaVersion != scan.SchemaVersion {
		result.fail("api-catalog", fmt.Sprintf("api-catalog.json schema %d unsupported, want %d", catalog.SchemaVersion, scan.SchemaVersion))
		return
	}
	if err := scan.ValidateAPICatalog(catalog); err != nil {
		result.fail("api-catalog", err.Error())
		return
	}
	if registry != nil {
		if message, unknown := catalogUnknownProjects(catalog, *registry); unknown {
			result.fail("api-catalog", message)
			return
		}
	}
	evidenceIDs, evidenceAvailable := catalogEvidenceIDs(out)
	if registry != nil {
		var err error
		evidenceIDs, err = workspaceCatalogEvidenceIDs(filepath.Dir(out), *registry)
		if err != nil {
			result.fail("api-catalog", err.Error())
			return
		}
		evidenceAvailable = true
	}
	if evidenceAvailable {
		if message, dangling := catalogDanglingEvidence(catalog, evidenceIDs); dangling {
			result.fail("api-catalog", message)
			return
		}
	}
	result.ok("api-catalog", "api-catalog.json valid")
}

func manifestListsFile(files []string, want string) bool {
	for _, file := range files {
		if filepath.ToSlash(file) == want {
			return true
		}
	}
	return false
}

func catalogEvidenceIDs(out string) (map[string]bool, bool) {
	var evidence []scan.EvidenceRecord
	if err := readJSON(scan.NewProjectOutputLayout(out).Index("evidence.json"), &evidence); err != nil {
		return nil, false
	}
	known := make(map[string]bool, len(evidence))
	for _, record := range evidence {
		known[record.ID] = true
	}
	return known, true
}

func workspaceCatalogEvidenceIDs(workspaceRoot string, registry scan.WorkspaceRegistryRecord) (map[string]bool, error) {
	known := map[string]bool{}
	for _, project := range registry.Projects {
		if !project.Indexed {
			continue
		}
		outputDir := project.OutputDir
		if outputDir == "" {
			outputDir = "goregraph-out"
		}
		evidencePath, err := workspaceProjectEvidencePath(workspaceRoot, project.Path, outputDir)
		if err != nil {
			return nil, fmt.Errorf("workspace project %q evidence path is invalid: %w", project.Path, err)
		}
		projectOut := filepath.Dir(filepath.Dir(evidencePath))
		var manifest scan.Manifest
		if err := readJSON(filepath.Join(projectOut, "manifest.json"), &manifest); err != nil {
			return nil, fmt.Errorf("workspace project %q manifest.json invalid: %w", project.Path, err)
		}
		if err := validateWorkspaceCatalogEvidenceManifest(outputDir, manifest); err != nil {
			return nil, fmt.Errorf("workspace project %q manifest.json invalid: %w", project.Path, err)
		}
		if !manifestListsFile(manifest.Index.Files, "index/evidence.json") {
			continue
		}
		var evidence []scan.EvidenceRecord
		if err := readJSON(evidencePath, &evidence); err != nil {
			return nil, fmt.Errorf("workspace project %q evidence.json invalid: %w", project.Path, err)
		}
		for _, record := range evidence {
			if record.ID != "" {
				known[record.ID] = true
			}
		}
	}
	return known, nil
}

func validateWorkspaceCatalogEvidenceManifest(outputDir string, manifest scan.Manifest) error {
	if manifest.Tool != scan.ToolName {
		return fmt.Errorf("tool is %q, want %q", manifest.Tool, scan.ToolName)
	}
	if manifest.Schema != scan.SchemaVersion {
		return fmt.Errorf("schema is %d, want %d", manifest.Schema, scan.SchemaVersion)
	}
	if manifest.Scope != "project" {
		return fmt.Errorf("scope is %q, want project", manifest.Scope)
	}
	if !manifest.Index.Complete {
		return fmt.Errorf("canonical index projection is incomplete")
	}
	registeredOutput, err := cleanWorkspaceRelativePath(outputDir)
	if err != nil {
		return err
	}
	manifestOutput, err := cleanWorkspaceRelativePath(manifest.OutputDir)
	if err != nil {
		return err
	}
	if registeredOutput != manifestOutput {
		return fmt.Errorf("output_dir is %q, want %q", manifest.OutputDir, outputDir)
	}
	return nil
}

func catalogDanglingEvidence(catalog scan.APICatalogRecord, known map[string]bool) (string, bool) {
	for _, endpoint := range catalog.Endpoints {
		if id, dangling := firstDanglingEvidence(endpoint.EvidenceIDs, known); dangling {
			return fmt.Sprintf("endpoint %q contains dangling evidence reference %q", endpoint.ID, id), true
		}
		for _, security := range endpoint.Security {
			if id, dangling := firstDanglingEvidence(security.EvidenceIDs, known); dangling {
				return fmt.Sprintf("endpoint %q security contains dangling evidence reference %q", endpoint.ID, id), true
			}
		}
		for _, consumer := range endpoint.Consumers {
			if id, dangling := firstDanglingEvidence(consumer.EvidenceIDs, known); dangling {
				return fmt.Sprintf("consumer %q contains dangling evidence reference %q", consumer.ID, id), true
			}
			for _, auth := range consumer.CallAuth {
				if id, dangling := firstDanglingEvidence(auth.EvidenceIDs, known); dangling {
					return fmt.Sprintf("consumer %q call auth contains dangling evidence reference %q", consumer.ID, id), true
				}
			}
		}
		for _, mismatch := range endpoint.Mismatches {
			if id, dangling := firstDanglingEvidence(mismatch.EvidenceIDs, known); dangling {
				return fmt.Sprintf("endpoint %q mismatch %q contains dangling evidence reference %q", endpoint.ID, mismatch.ID, id), true
			}
		}
	}
	return "", false
}

func firstDanglingEvidence(ids []string, known map[string]bool) (string, bool) {
	for _, id := range ids {
		if !known[id] {
			return id, true
		}
	}
	return "", false
}

func catalogUnknownProjects(catalog scan.APICatalogRecord, registry scan.WorkspaceRegistryRecord) (string, bool) {
	projects := make(map[string]bool, len(registry.Projects))
	for _, project := range registry.Projects {
		projects[filepath.ToSlash(project.Path)] = true
	}
	for _, endpoint := range catalog.Endpoints {
		if !projects[filepath.ToSlash(endpoint.ProviderProject)] {
			return fmt.Sprintf("endpoint %q references unknown provider project %q", endpoint.ID, endpoint.ProviderProject), true
		}
		for _, consumer := range endpoint.Consumers {
			if !projects[filepath.ToSlash(consumer.Project)] {
				return fmt.Sprintf("consumer %q references unknown project %q", consumer.ID, consumer.Project), true
			}
		}
	}
	return "", false
}

func checkWorkspaceDashboardConfig(root string, result *Result) (scan.WorkspaceDashboardConfig, bool) {
	dashboardConfig, _, err := scan.LoadWorkspaceDashboardConfig(root)
	if err != nil {
		result.fail("dashboard-config", scan.WorkspaceDashboardConfigName+": "+err.Error())
		return scan.WorkspaceDashboardConfig{}, false
	}
	result.ok("dashboard-config", scan.WorkspaceDashboardConfigName+" valid")
	return dashboardConfig, true
}

func checkStaleWorkspaceDashboardServices(out string, dashboardConfig scan.WorkspaceDashboardConfig, result *Result) {
	var registry scan.WorkspaceRegistryRecord
	if err := readJSON(scan.NewWorkspaceOutputLayout(out).Index("registry.json"), &registry); err != nil {
		return
	}
	layout := scan.BuildWorkspaceArchitectureLayout(registry, nil, dashboardConfig)
	for _, project := range layout.StaleServices {
		result.warn("dashboard-config", scan.WorkspaceDashboardConfigName+" references removed service "+project)
	}
}

func checkAgentContextIndex(out string, manifest scan.Manifest, result *Result) {
	if !manifest.Agent.Complete {
		return
	}
	manifestFiles := map[string]bool{}
	for _, name := range manifest.Agent.Files {
		manifestFiles[filepath.ToSlash(name)] = true
	}
	for _, name := range scan.AgentGeneratedFiles {
		required := filepath.ToSlash(filepath.Join("agent", name))
		if !manifestFiles[required] {
			result.fail("context-index", "complete agent manifest omits required file "+required)
			return
		}
	}
	var index scan.AgentContextIndexRecord
	path := scan.NewProjectOutputLayout(out).Agent("context-index.json")
	if err := readJSON(path, &index); err != nil {
		result.fail("context-index", "context-index.json invalid: "+err.Error())
		return
	}
	if index.SchemaVersion != scan.SchemaVersion {
		result.fail("context-index", fmt.Sprintf("context index schema %d unsupported, want %d", index.SchemaVersion, scan.SchemaVersion))
		return
	}
	factIDs := map[string]bool{}
	for _, fact := range index.Facts {
		if fact.ID == "" {
			result.fail("context-index", "context-index.json contains an empty fact ID")
			return
		}
		if factIDs[fact.ID] {
			result.fail("context-index", "context-index.json contains duplicate fact ID "+fact.ID)
			return
		}
		factIDs[fact.ID] = true
		if fact.Line < 0 || fact.EndLine < 0 {
			result.fail("context-index", "context-index.json fact "+fact.ID+" contains a negative line")
			return
		}
		if !portableContextPath(fact.File) {
			result.fail("context-index", "context-index.json fact "+fact.ID+" has invalid relative path "+fact.File)
			return
		}
	}
	allIDs := map[string]bool{}
	for id := range factIDs {
		allIDs[id] = true
	}
	for _, edge := range index.Edges {
		if edge.ID == "" {
			result.fail("context-index", "context-index.json contains an empty edge ID")
			return
		}
		if allIDs[edge.ID] {
			result.fail("context-index", "context-index.json contains duplicate ID "+edge.ID)
			return
		}
		allIDs[edge.ID] = true
		if edge.FromFactID == "" {
			result.fail("context-index", "context-index.json contains an empty from_fact_id")
			return
		}
		if edge.ToFactID == "" {
			result.fail("context-index", "context-index.json contains an empty to_fact_id")
			return
		}
		if edge.Line < 0 {
			result.fail("context-index", "context-index.json edge "+edge.ID+" contains a negative line")
			return
		}
		if !portableContextPath(edge.File) {
			result.fail("context-index", "context-index.json edge "+edge.ID+" has invalid relative path "+edge.File)
			return
		}
		if !factIDs[edge.FromFactID] {
			result.fail("context-index", "context-index.json contains dangling from_fact_id "+edge.FromFactID)
			return
		}
		if !factIDs[edge.ToFactID] {
			result.fail("context-index", "context-index.json contains dangling to_fact_id "+edge.ToFactID)
			return
		}
	}
	if !contextFactsSorted(index.Facts) {
		result.fail("context-index", "context-index.json contains unsorted facts")
		return
	}
	if !contextEdgesSorted(index.Edges) {
		result.fail("context-index", "context-index.json contains unsorted edges")
		return
	}
	if !contextCoverageSorted(index.Coverage) {
		result.fail("context-index", "context-index.json contains unsorted coverage")
		return
	}
	result.ok("context-index", "context-index.json valid")
}

func portableContextPath(value string) bool {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return false
	}
	if len(value) >= 3 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':' && value[2] == '/' {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

func contextFactsSorted(records []scan.AgentContextFactRecord) bool {
	if len(records) < 2 {
		return true
	}
	sorted := append([]scan.AgentContextFactRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if left.Project != right.Project {
			return left.Project < right.Project
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Qualified != right.Qualified {
			return left.Qualified < right.Qualified
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})
	return reflect.DeepEqual(records, sorted)
}

func contextEdgesSorted(records []scan.AgentContextEdgeRecord) bool {
	if len(records) < 2 {
		return true
	}
	sorted := append([]scan.AgentContextEdgeRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if left.FromLabel != right.FromLabel {
			return left.FromLabel < right.FromLabel
		}
		if left.ToLabel != right.ToLabel {
			return left.ToLabel < right.ToLabel
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})
	return reflect.DeepEqual(records, sorted)
}

func contextCoverageSorted(records []scan.AgentContextCoverageRecord) bool {
	if len(records) < 2 {
		return true
	}
	sorted := append([]scan.AgentContextCoverageRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if left.Project != right.Project {
			return left.Project < right.Project
		}
		if left.Capability != right.Capability {
			return left.Capability < right.Capability
		}
		if left.Coverage != right.Coverage {
			return left.Coverage < right.Coverage
		}
		return left.Reason < right.Reason
	})
	return reflect.DeepEqual(records, sorted)
}

func checkWorkspaceJSONFiles(out string, files []string, result *Result) {
	for _, name := range files {
		canonical := filepath.ToSlash(name)
		if !strings.HasPrefix(canonical, "index/") || !strings.HasSuffix(canonical, ".json") {
			continue
		}
		dest := workspaceJSONDestination(filepath.Base(canonical))
		if err := readJSON(filepath.Join(out, filepath.FromSlash(canonical)), dest); err != nil {
			result.fail("json", filepath.Base(canonical)+" invalid: "+err.Error())
			continue
		}
		result.ok("json", filepath.Base(canonical)+" valid")
	}
}

func workspaceJSONDestination(name string) any {
	switch name {
	case "registry.json":
		return &scan.WorkspaceRegistryRecord{}
	case "context.json":
		return &scan.WorkspaceContextRecord{}
	case "contract-matches.json":
		return &[]scan.WorkspaceContractMatchRecord{}
	case "feature-flows.json":
		return &[]scan.WorkspaceFeatureFlowRecord{}
	case "data-flows.json":
		return &[]scan.DataFlowRecord{}
	case "feature-dossiers.json":
		return &[]scan.FeatureDossierRecord{}
	case "workspace-graph.json":
		return &scan.WorkspaceGraphRecord{}
	case "workspace-service-map.json":
		return &scan.WorkspaceServiceMapRecord{}
	case "workspace-endpoint-traces.json":
		return &scan.WorkspaceEndpointTraceIndexRecord{}
	case "directed-traces.json":
		return &scan.DirectedTraceIndexRecord{}
	case "freshness.json":
		return &scan.ArtifactFreshnessIndex{}
	case "symbol-index.json":
		return &scan.WorkspaceSymbolIndexRecord{}
	case "symbol-usages.json":
		return &scan.WorkspaceSymbolUsageIndexRecord{}
	default:
		return new(anyJSON)
	}
}

type anyJSON any

func legacyLayoutExists(out string) bool {
	for _, name := range []string{"routes.json", "report.md", "workspace-map.html", "context-index.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err == nil {
			return true
		}
	}
	return false
}

func readJSON(path string, dest any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func (r *Result) ok(scope, message string) {
	r.Lines = append(r.Lines, fmt.Sprintf("OK   %s: %s", scope, message))
}

func (r *Result) warn(scope, message string) {
	r.Warnings++
	r.Lines = append(r.Lines, fmt.Sprintf("WARN %s: %s", scope, message))
}

func (r *Result) fail(scope, message string) {
	r.Failures++
	r.Lines = append(r.Lines, fmt.Sprintf("FAIL %s: %s", scope, message))
}

func (r *Result) fix(command string) {
	r.Lines = append(r.Lines, "", "Suggested fix:", "  "+strings.TrimSpace(command))
}

func (r Result) String() string {
	return strings.Join(r.Lines, "\n") + "\n"
}
