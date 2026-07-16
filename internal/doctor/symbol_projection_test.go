package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestValidateSymbolProjectionRejectsInvalidReferences(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*scan.WorkspaceSymbolIndexRecord, *scan.WorkspaceSymbolUsageIndexRecord, map[string]bool)
		offending string
	}{
		{
			name: "duplicate symbol ID",
			mutate: func(index *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				index.Symbols = append(index.Symbols, index.Symbols[0])
			},
			offending: "symbol:provider",
		},
		{
			name: "duplicate usage ID",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages = append(usages.Usages, usages.Usages[0])
			},
			offending: "usage:direct",
		},
		{
			name: "dangling provider",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0].ProviderSymbolID = "symbol:missing-provider"
			},
			offending: "symbol:missing-provider",
		},
		{
			name: "dangling known consumer",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0].ConsumerSymbolID = "symbol:missing-consumer"
			},
			offending: "symbol:missing-consumer",
		},
		{
			name: "dangling ambiguous candidate",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0] = validAmbiguousUsage()
				usages.Usages[0].CandidateSymbolIDs[0] = "symbol:missing-candidate"
			},
			offending: "symbol:missing-candidate",
		},
		{
			name: "dangling evidence",
			mutate: func(index *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				index.Symbols[0].EvidenceIDs = []string{"backend/service#evidence:missing"}
			},
			offending: "backend/service#evidence:missing",
		},
		{
			name: "non-namespaced evidence",
			mutate: func(index *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				index.Symbols[0].EvidenceIDs = []string{"evidence:provider"}
			},
			offending: "evidence:provider",
		},
		{
			name: "unknown project reference",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0].ConsumerProject = "unknown/project"
			},
			offending: "usage:direct",
		},
		{
			name: "project absent from registry",
			mutate: func(index *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, knownEvidence map[string]bool) {
				for key := range knownEvidence {
					delete(knownEvidence, key)
				}
				for symbolIndex := range index.Symbols {
					index.Symbols[symbolIndex].EvidenceIDs = nil
				}
				index.Coverage = nil
				for usageIndex := range usages.Usages {
					usages.Usages[usageIndex].EvidenceIDs = nil
				}
				usages.Coverage = nil
			},
			offending: "symbol:provider",
		},
		{
			name: "missing source reference",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0].SourceFile = ""
			},
			offending: "usage:direct",
		},
		{
			name: "source ending in traversal",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0].SourceFile = "src/.."
			},
			offending: "usage:direct",
		},
		{
			name: "windows source traversal",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0].SourceFile = `..\secret.go`
			},
			offending: "usage:direct",
		},
		{
			name: "unordered API positions",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0] = validAPIUsage()
				usages.Usages[0].APIPath[1].Position = 3
			},
			offending: "usage:api",
		},
		{
			name: "invalid API transport",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0] = validAPIUsage()
				usages.Usages[0].Transport = "grpc"
			},
			offending: "usage:api",
		},
		{
			name: "missing selected final symbol",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0] = validAPIUsage()
				usages.Usages[0].APIPath = usages.Usages[0].APIPath[:len(usages.Usages[0].APIPath)-1]
			},
			offending: "usage:api",
		},
		{
			name: "ambiguous API missing selected final symbol",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0] = validAmbiguousAPIUsage()
				usages.Usages[0].APIPath = usages.Usages[0].APIPath[:len(usages.Usages[0].APIPath)-1]
			},
			offending: "usage:ambiguous-api",
		},
		{
			name: "unsorted ambiguous candidates",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0] = validAmbiguousUsage()
				usages.Usages[0].CandidateSymbolIDs = []string{"symbol:provider", "symbol:alternate"}
			},
			offending: "symbol:alternate",
		},
		{
			name: "category resolution mismatch",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.Usages[0].Category = scan.SymbolUsageUnresolved
			},
			offending: "usage:direct",
		},
		{
			name: "invalid symbol coverage",
			mutate: func(index *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				index.Symbols[0].Coverage = scan.Coverage("BROKEN")
			},
			offending: "symbol:provider",
		},
		{
			name: "invalid projection coverage",
			mutate: func(index *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				index.Coverage[0].Coverage = scan.Coverage("BROKEN")
			},
			offending: "backend/service/java/declarations",
		},
		{
			name: "symbol schema other than two",
			mutate: func(index *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				index.SchemaVersion = 1
			},
			offending: "schema_version",
		},
		{
			name: "usage schema other than two",
			mutate: func(_ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord, _ map[string]bool) {
				usages.SchemaVersion = 3
			},
			offending: "schema_version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index, usages, knownEvidence := validSymbolProjection()
			test.mutate(&index, &usages, knownEvidence)

			err := ValidateSymbolProjection(index, usages, knownEvidence)

			if err == nil {
				t.Fatal("invalid projection passed validation")
			}
			if !strings.Contains(err.Error(), test.offending) {
				t.Fatalf("error %q does not identify offending value %q", err, test.offending)
			}
			for _, want := range []string{"goregraph workspace clean", "goregraph workspace scan-all"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not include remediation %q", err, want)
				}
			}
		})
	}
}

func TestValidateSymbolProjectionAcceptsValidAndUnresolvedRecords(t *testing.T) {
	index, usages, knownEvidence := validSymbolProjection()
	usages.Usages = append(usages.Usages, scan.CanonicalSymbolUsageRecord{
		ID:              "usage:unresolved",
		ConsumerProject: "frontend/app",
		Category:        scan.SymbolUsageUnresolved,
		Language:        "typescript",
		RelationKind:    "imports_value",
		SourceFile:      "src/missing.ts",
		SourceLine:      7,
		Confidence:      scan.ConfidenceNormalized,
		Resolution:      scan.SymbolResolutionUnresolved,
		Reason:          "no indexed declaration matches the evidenced target",
		Analyzer:        "typescript",
	})

	if err := ValidateSymbolProjection(index, usages, knownEvidence); err != nil {
		t.Fatalf("valid projection failed validation: %v", err)
	}
}

func TestDoctorValidatesWorkspaceRootSymbolProjection(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceProjectionFixture(t, root)

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures != 0 {
		t.Fatalf("valid workspace projection failed Doctor: %v", result.Lines)
	}
	if !containsLine(result.Lines, "workspace symbol projection valid") {
		t.Fatalf("Doctor did not report workspace symbol validation: %v", result.Lines)
	}
}

func TestDoctorRejectsMissingCurrentWorkspaceSymbolProjection(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(out, "registry.json"), scan.WorkspaceRegistryRecord{
		Root: filepath.ToSlash(root),
	})

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "symbol-index.json") {
		t.Fatalf("missing workspace symbol projection passed Doctor: %v", result.Lines)
	}
	want := "goregraph workspace clean " + root + " --execute && goregraph workspace scan-all " + root
	if !containsLine(result.Lines, want) {
		t.Fatalf("Doctor remediation missing %q: %v", want, result.Lines)
	}
}

func TestDoctorRejectsMissingProjectionAtDetectedWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".goregraph-workspace.yml"), []byte("projects: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 ||
		!containsLine(result.Lines, "symbol-index.json") ||
		!containsLine(result.Lines, "symbol-usages.json") {
		t.Fatalf("missing detected workspace projection passed Doctor: %v", result.Lines)
	}
	want := "goregraph workspace clean " + root + " --execute && goregraph workspace scan-all " + root
	if !containsLine(result.Lines, want) {
		t.Fatalf("Doctor remediation missing %q: %v", want, result.Lines)
	}
}

func TestDoctorValidatesParentWorkspaceProjectionFromProjectRoot(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "frontend", "app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := scan.Run(project, cfg); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceProjectionFixture(t, workspace)
	out := filepath.Join(workspace, ".goregraph-workspace", "symbol-usages.json")
	_, usages, _ := validSymbolProjection()
	usages.Root = filepath.ToSlash(workspace)
	usages.Usages[0].ProviderSymbolID = "symbol:missing-parent-provider"
	writeTestJSON(t, out, usages)

	result, err := Run(project)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "symbol:missing-parent-provider") {
		t.Fatalf("invalid parent workspace projection passed Doctor: %v", result.Lines)
	}
}

func TestDoctorKeepsLegacyProjectValidUnderWorkspaceLikeParent(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "frontend", "app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := scan.Run(project, cfg); err != nil {
		t.Fatal(err)
	}

	result, err := Run(project)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures != 0 {
		t.Fatalf("legacy project-only output failed Doctor: %v", result.Lines)
	}
	if containsLine(result.Lines, "workspace symbol projection missing") {
		t.Fatalf("heuristic parent triggered workspace validation: %v", result.Lines)
	}
}

func TestDoctorRejectsCopiedWorkspaceRootMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*scan.WorkspaceRegistryRecord, *scan.WorkspaceSymbolIndexRecord, *scan.WorkspaceSymbolUsageIndexRecord)
	}{
		{
			name: "registry root",
			mutate: func(registry *scan.WorkspaceRegistryRecord, _ *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord) {
				registry.Root = "/original/workspace"
			},
		},
		{
			name: "symbol index root",
			mutate: func(_ *scan.WorkspaceRegistryRecord, index *scan.WorkspaceSymbolIndexRecord, _ *scan.WorkspaceSymbolUsageIndexRecord) {
				index.Root = "/original/workspace"
			},
		},
		{
			name: "symbol usages root",
			mutate: func(_ *scan.WorkspaceRegistryRecord, _ *scan.WorkspaceSymbolIndexRecord, usages *scan.WorkspaceSymbolUsageIndexRecord) {
				usages.Root = "/original/workspace"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeWorkspaceProjectionFixture(t, root)
			out := filepath.Join(root, ".goregraph-workspace")
			var registry scan.WorkspaceRegistryRecord
			var index scan.WorkspaceSymbolIndexRecord
			var usages scan.WorkspaceSymbolUsageIndexRecord
			readTestJSON(t, filepath.Join(out, "registry.json"), &registry)
			readTestJSON(t, filepath.Join(out, "symbol-index.json"), &index)
			readTestJSON(t, filepath.Join(out, "symbol-usages.json"), &usages)
			test.mutate(&registry, &index, &usages)
			writeTestJSON(t, filepath.Join(out, "registry.json"), registry)
			writeTestJSON(t, filepath.Join(out, "symbol-index.json"), index)
			writeTestJSON(t, filepath.Join(out, "symbol-usages.json"), usages)

			result, err := Run(root)

			if err != nil {
				t.Fatal(err)
			}
			if result.Failures == 0 || !containsLine(result.Lines, "/original/workspace") {
				t.Fatalf("copied workspace metadata passed Doctor: %v", result.Lines)
			}
		})
	}
}

func TestDoctorDoesNotTrustRegistryAbsPathOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceProjectionFixture(t, root)
	external := t.TempDir()
	externalOut := filepath.Join(external, "goregraph-out")
	if err := os.MkdirAll(externalOut, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(externalOut, "evidence.json"), []scan.EvidenceRecord{
		{ID: "evidence:external", Project: "backend/service", File: "src/Service.java"},
	})

	out := filepath.Join(root, ".goregraph-workspace")
	var registry scan.WorkspaceRegistryRecord
	readTestJSON(t, filepath.Join(out, "registry.json"), &registry)
	for index := range registry.Projects {
		if registry.Projects[index].Path == "backend/service" {
			registry.Projects[index].AbsPath = filepath.ToSlash(external)
		}
	}
	writeTestJSON(t, filepath.Join(out, "registry.json"), registry)
	var symbols scan.WorkspaceSymbolIndexRecord
	readTestJSON(t, filepath.Join(out, "symbol-index.json"), &symbols)
	symbols.Symbols[0].EvidenceIDs = []string{"backend/service#evidence:external"}
	writeTestJSON(t, filepath.Join(out, "symbol-index.json"), symbols)

	result, err := Run(root)

	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "backend/service#evidence:external") {
		t.Fatalf("external registry AbsPath supplied workspace evidence: %v", result.Lines)
	}
}

func TestLoadWorkspaceProjectionEvidenceRejectsEscapingProjectPath(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	externalOut := filepath.Join(external, "goregraph-out")
	if err := os.MkdirAll(externalOut, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(externalOut, "evidence.json"), []scan.EvidenceRecord{})

	_, err := loadWorkspaceProjectionEvidence(root, scan.WorkspaceRegistryRecord{
		Projects: []scan.WorkspaceProjectRecord{{
			Path:      "../outside",
			AbsPath:   filepath.ToSlash(external),
			Indexed:   true,
			OutputDir: "goregraph-out",
		}},
	})

	if err == nil || !strings.Contains(err.Error(), "../outside") {
		t.Fatalf("escaping project path was accepted: %v", err)
	}
}

func validSymbolProjection() (scan.WorkspaceSymbolIndexRecord, scan.WorkspaceSymbolUsageIndexRecord, map[string]bool) {
	index := scan.WorkspaceSymbolIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Root:          "/workspace",
		Symbols: []scan.CanonicalSymbolRecord{
			{
				ID:              "symbol:provider",
				Project:         "backend/service",
				Language:        "java",
				Kind:            "method",
				Name:            "load",
				QualifiedName:   "example.Service.load",
				DeclarationFile: "src/Service.java",
				DeclarationLine: 12,
				EvidenceIDs:     []string{"backend/service#evidence:provider"},
				Analyzer:        "java",
				Confidence:      scan.ConfidenceExact,
				Coverage:        scan.CoverageComplete,
			},
			{
				ID:              "symbol:consumer",
				Project:         "frontend/app",
				Language:        "typescript",
				Kind:            "function",
				Name:            "page",
				QualifiedName:   "page",
				DeclarationFile: "src/page.ts",
				DeclarationLine: 4,
				EvidenceIDs:     []string{"frontend/app#evidence:consumer"},
				Analyzer:        "typescript",
				Confidence:      scan.ConfidenceExact,
				Coverage:        scan.CoverageComplete,
			},
			{
				ID:              "symbol:alternate",
				Project:         "backend/service",
				Language:        "java",
				Kind:            "method",
				Name:            "loadAlternate",
				QualifiedName:   "example.Service.loadAlternate",
				DeclarationFile: "src/Service.java",
				DeclarationLine: 20,
				Analyzer:        "java",
				Confidence:      scan.ConfidenceExact,
				Coverage:        scan.CoverageComplete,
			},
		},
		Coverage: []scan.SymbolCoverageRecord{
			{Project: "backend/service", Language: "java", Capability: "declarations", Coverage: scan.CoverageComplete, Reason: "indexed"},
			{Project: "frontend/app", Language: "typescript", Capability: "declarations", Coverage: scan.CoverageComplete, Reason: "indexed"},
		},
	}
	usages := scan.WorkspaceSymbolUsageIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Root:          "/workspace",
		Usages: []scan.CanonicalSymbolUsageRecord{
			{
				ID:                  "usage:direct",
				ProviderSymbolID:    "symbol:provider",
				ConsumerProject:     "frontend/app",
				ConsumerSymbolID:    "symbol:consumer",
				Category:            scan.SymbolUsageDirectReference,
				Language:            "typescript",
				RelationKind:        "imports_value",
				TargetQualifiedName: "example.Service.load",
				SourceFile:          "src/page.ts",
				SourceLine:          8,
				Confidence:          scan.ConfidenceExact,
				Resolution:          scan.SymbolResolutionExact,
				Reason:              "one indexed declaration matches",
				Analyzer:            "typescript",
				EvidenceIDs:         []string{"frontend/app#evidence:usage"},
			},
		},
		Coverage: []scan.SymbolCoverageRecord{
			{Project: "frontend/app", Language: "typescript", Capability: "direct_usages", Coverage: scan.CoveragePartial, Reason: "static analysis"},
		},
	}
	knownEvidence := map[string]bool{
		"backend/service":                    true,
		"frontend/app":                       true,
		"backend/service#evidence:provider":  true,
		"frontend/app#evidence:consumer":     true,
		"frontend/app#evidence:usage":        true,
		"frontend/app#evidence:api-consumer": true,
		"backend/service#evidence:api":       true,
	}
	return index, usages, knownEvidence
}

func validAmbiguousUsage() scan.CanonicalSymbolUsageRecord {
	return scan.CanonicalSymbolUsageRecord{
		ID:                 "usage:ambiguous",
		ProviderSymbolID:   "symbol:provider",
		ConsumerProject:    "frontend/app",
		ConsumerSymbolID:   "symbol:consumer",
		Category:           scan.SymbolUsageAmbiguous,
		Language:           "typescript",
		RelationKind:       "imports_value",
		SourceFile:         "src/page.ts",
		SourceLine:         8,
		Confidence:         scan.ConfidenceNormalized,
		Resolution:         scan.SymbolResolutionAmbiguous,
		Reason:             "multiple declarations match",
		Analyzer:           "typescript",
		EvidenceIDs:        []string{"frontend/app#evidence:usage"},
		CandidateSymbolIDs: []string{"symbol:alternate", "symbol:provider"},
	}
}

func validAPIUsage() scan.CanonicalSymbolUsageRecord {
	return scan.CanonicalSymbolUsageRecord{
		ID:               "usage:api",
		ProviderSymbolID: "symbol:provider",
		ConsumerProject:  "frontend/app",
		ConsumerSymbolID: "symbol:consumer",
		Category:         scan.SymbolUsageReachedThroughAPI,
		Language:         "typescript",
		RelationKind:     "http_reachability",
		SourceFile:       "src/page.ts",
		SourceLine:       8,
		Confidence:       scan.ConfidenceExact,
		Resolution:       scan.SymbolResolutionExact,
		Reason:           "resolved HTTP path",
		Analyzer:         "workspace-symbol-api",
		EvidenceIDs: []string{
			"backend/service#evidence:api",
			"frontend/app#evidence:api-consumer",
		},
		Transport: "http",
		APIPath: []scan.SymbolAPIPathStepRecord{
			{
				Position:    0,
				Kind:        "frontend_symbol",
				Project:     "frontend/app",
				SymbolID:    "symbol:consumer",
				Label:       "page",
				File:        "src/page.ts",
				Line:        4,
				EvidenceIDs: []string{"frontend/app#evidence:api-consumer"},
			},
			{
				Position: 1,
				Kind:     "selected_symbol",
				Project:  "backend/service",
				SymbolID: "symbol:provider",
				Label:    "example.Service.load",
				File:     "src/Service.java",
				Line:     12,
				EvidenceIDs: []string{
					"backend/service#evidence:api",
				},
			},
		},
	}
}

func validAmbiguousAPIUsage() scan.CanonicalSymbolUsageRecord {
	usage := validAPIUsage()
	usage.ID = "usage:ambiguous-api"
	usage.Category = scan.SymbolUsageAmbiguous
	usage.Confidence = scan.ConfidenceNormalized
	usage.Resolution = scan.SymbolResolutionAmbiguous
	usage.Reason = "multiple declarations match the HTTP implementation"
	usage.CandidateSymbolIDs = []string{"symbol:alternate", "symbol:provider"}
	return usage
}

func writeWorkspaceProjectionFixture(t *testing.T, root string) {
	t.Helper()
	index, usages, _ := validSymbolProjection()
	index.Root = filepath.ToSlash(root)
	usages.Root = filepath.ToSlash(root)
	out := filepath.Join(root, ".goregraph-workspace")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	projects := []scan.WorkspaceProjectRecord{
		workspaceDoctorProject(t, root, "backend/service", []scan.EvidenceRecord{
			{ID: "evidence:provider", Project: "backend/service", File: "src/Service.java"},
			{ID: "evidence:api", Project: "backend/service", File: "src/Service.java"},
		}),
		workspaceDoctorProject(t, root, "frontend/app", []scan.EvidenceRecord{
			{ID: "evidence:consumer", Project: "frontend/app", File: "src/page.ts"},
			{ID: "evidence:usage", Project: "frontend/app", File: "src/page.ts"},
			{ID: "evidence:api-consumer", Project: "frontend/app", File: "src/page.ts"},
		}),
	}
	writeTestJSON(t, filepath.Join(out, "registry.json"), scan.WorkspaceRegistryRecord{
		Root:     filepath.ToSlash(root),
		Projects: projects,
	})
	writeTestJSON(t, filepath.Join(out, "symbol-index.json"), index)
	writeTestJSON(t, filepath.Join(out, "symbol-usages.json"), usages)
}

func workspaceDoctorProject(t *testing.T, root, projectPath string, evidence []scan.EvidenceRecord) scan.WorkspaceProjectRecord {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(projectPath))
	out := filepath.Join(abs, "goregraph-out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(out, "evidence.json"), evidence)
	return scan.WorkspaceProjectRecord{
		Name:      filepath.Base(projectPath),
		Path:      projectPath,
		AbsPath:   filepath.ToSlash(abs),
		Indexed:   true,
		Status:    "indexed",
		OutputDir: "goregraph-out",
	}
}
