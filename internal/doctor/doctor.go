package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	checkFreshnessIntegrity(out, result)
	checkEvidenceIntegrity(out, result)
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
	if manifest.Dashboard.Complete {
		checkWorkspaceSymbolProjection(root, result)
	}
}

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
