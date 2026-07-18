package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDoctorValidatesManifestListedProjectAPICatalog(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*scan.APICatalogRecord)
		wantID  string
	}{
		{
			name: "duplicate endpoint ID",
			corrupt: func(catalog *scan.APICatalogRecord) {
				catalog.Endpoints = append(catalog.Endpoints, catalog.Endpoints[0])
			},
			wantID: "endpoint:orders",
		},
		{
			name: "invalid security category",
			corrupt: func(catalog *scan.APICatalogRecord) {
				catalog.Endpoints[0].Security[0].Kind = "invalid"
			},
			wantID: "endpoint:orders",
		},
		{
			name: "dangling evidence ID",
			corrupt: func(catalog *scan.APICatalogRecord) {
				catalog.Endpoints[0].EvidenceIDs = []string{"evidence:missing"}
			},
			wantID: "endpoint:orders",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, catalogPath := projectCatalogFixture(t)
			var catalog scan.APICatalogRecord
			readTestJSON(t, catalogPath, &catalog)
			test.corrupt(&catalog)
			writeTestJSON(t, catalogPath, catalog)

			result, err := Run(root)
			if err != nil {
				t.Fatal(err)
			}
			requireAPICatalogFailure(t, result, test.wantID)
		})
	}
}

func TestDoctorValidatesManifestListedWorkspaceAPICatalogProjectReferences(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*scan.APICatalogRecord)
		wantID  func(scan.APICatalogRecord) string
	}{
		{
			name: "unknown provider project",
			corrupt: func(catalog *scan.APICatalogRecord) {
				catalog.Endpoints[0].ProviderProject = "services/missing"
			},
			wantID: func(catalog scan.APICatalogRecord) string {
				return catalog.Endpoints[0].ID
			},
		},
		{
			name: "unknown consumer project",
			corrupt: func(catalog *scan.APICatalogRecord) {
				catalog.Endpoints[0].Consumers[0].Project = "frontend/missing"
			},
			wantID: func(catalog scan.APICatalogRecord) string {
				return catalog.Endpoints[0].Consumers[0].ID
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace, catalogPath := workspaceCatalogFixture(t)
			var catalog scan.APICatalogRecord
			readTestJSON(t, catalogPath, &catalog)
			wantID := test.wantID(catalog)
			test.corrupt(&catalog)
			writeTestJSON(t, catalogPath, catalog)

			result, err := Run(workspace)
			if err != nil {
				t.Fatal(err)
			}
			requireAPICatalogFailure(t, result, wantID)
		})
	}
}

func TestDoctorRejectsWorkspaceAPICatalogDanglingEvidence(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*scan.APICatalogRecord) string
	}{
		{
			name: "endpoint evidence",
			corrupt: func(catalog *scan.APICatalogRecord) string {
				catalog.Endpoints[0].EvidenceIDs = []string{"evidence:missing"}
				return catalog.Endpoints[0].ID
			},
		},
		{
			name: "consumer evidence",
			corrupt: func(catalog *scan.APICatalogRecord) string {
				catalog.Endpoints[0].Consumers[0].EvidenceIDs = []string{"evidence:missing"}
				return catalog.Endpoints[0].Consumers[0].ID
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace, catalogPath := workspaceCatalogFixture(t)
			var catalog scan.APICatalogRecord
			readTestJSON(t, catalogPath, &catalog)
			wantID := test.corrupt(&catalog)
			writeTestJSON(t, catalogPath, catalog)

			result, err := Run(workspace)
			if err != nil {
				t.Fatal(err)
			}
			requireAPICatalogFailure(t, result, wantID)
		})
	}
}

func TestDoctorIgnoresUnmanifestedWorkspaceProjectEvidence(t *testing.T) {
	workspace, catalogPath := workspaceCatalogFixture(t)
	var catalog scan.APICatalogRecord
	readTestJSON(t, catalogPath, &catalog)
	endpoint := &catalog.Endpoints[0]
	endpoint.EvidenceIDs = []string{"evidence:unmanifested"}
	writeTestJSON(t, catalogPath, catalog)

	projectOut := workspaceProjectOutputForTest(t, workspace, endpoint.ProviderProject)
	evidencePath := filepath.Join(projectOut, "index", "evidence.json")
	var evidence []scan.EvidenceRecord
	readTestJSON(t, evidencePath, &evidence)
	evidence = append(evidence, scan.EvidenceRecord{ID: "evidence:unmanifested"})
	writeTestJSON(t, evidencePath, evidence)

	manifestPath := filepath.Join(projectOut, "manifest.json")
	var manifest scan.Manifest
	readTestJSON(t, manifestPath, &manifest)
	manifest.Index.Files = withoutManifestFile(manifest.Index.Files, "index/evidence.json")
	writeTestJSON(t, manifestPath, manifest)

	result, err := Run(workspace)
	if err != nil {
		t.Fatal(err)
	}
	requireAPICatalogFailure(t, result, endpoint.ID)
}

func TestDoctorRejectsUnsafeWorkspaceAPICatalogEvidencePaths(t *testing.T) {
	tests := []struct {
		name      string
		outputDir func(*testing.T) string
		want      string
	}{
		{
			name: "traversal",
			outputDir: func(*testing.T) string {
				return "../outside"
			},
			want: "escapes the workspace",
		},
		{
			name: "absolute",
			outputDir: func(t *testing.T) string {
				return t.TempDir()
			},
			want: "must be workspace-relative",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace, _ := workspaceCatalogFixture(t)
			registryPath := scan.NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Index("registry.json")
			var registry scan.WorkspaceRegistryRecord
			readTestJSON(t, registryPath, &registry)
			project := registry.Projects[0]
			registry.Projects[0].OutputDir = test.outputDir(t)
			writeTestJSON(t, registryPath, registry)

			result, err := Run(workspace)
			if err != nil {
				t.Fatal(err)
			}
			requireAPICatalogFailureContaining(t, result, project.Path, test.want)
		})
	}
}

func TestDoctorSkipsUnmanifestedAPICatalog(t *testing.T) {
	root, catalogPath := projectCatalogFixture(t)
	manifestPath := filepath.Join(root, "goregraph-out", "manifest.json")
	var manifest scan.Manifest
	readTestJSON(t, manifestPath, &manifest)
	manifest.Index.Files = withoutManifestFile(manifest.Index.Files, "index/api-catalog.json")
	writeTestJSON(t, manifestPath, manifest)

	var catalog scan.APICatalogRecord
	readTestJSON(t, catalogPath, &catalog)
	catalog.Endpoints[0].Security[0].Kind = "invalid"
	writeTestJSON(t, catalogPath, catalog)

	result, err := Run(root)
	if err != nil {
		t.Fatal(err)
	}
	if containsLine(result.Lines, "FAIL api-catalog:") {
		t.Fatalf("Doctor validated an unmanifested API catalog: %v", result.Lines)
	}
}

func projectCatalogFixture(t *testing.T) (string, string) {
	t.Helper()
	root := scannedProject(t)
	layout := scan.NewProjectOutputLayout(filepath.Join(root, "goregraph-out"))
	writeCatalogEvidenceFixture(t, layout.Index("evidence.json"))
	writeTestJSON(t, layout.Index("api-catalog.json"), catalogFixture())
	return root, layout.Index("api-catalog.json")
}

func workspaceCatalogFixture(t *testing.T) (string, string) {
	t.Helper()
	workspace := t.TempDir()
	frontend := filepath.Join(workspace, "frontend", "web")
	backend := filepath.Join(workspace, "services", "orders")
	writeWorkspaceProjectFile(t, frontend, "package.json", `{"name":"web"}`)
	writeWorkspaceProjectFile(t, frontend, "src/api.ts", "export async function loadOrder(id: string) { return fetch(`/orders/${id}`); }\n")
	writeWorkspaceProjectFile(t, backend, "pom.xml", `<project><modelVersion>4.0.0</modelVersion><groupId>example</groupId><artifactId>orders</artifactId><version>1</version></project>`)
	writeWorkspaceProjectFile(t, backend, "src/main/java/example/OrderController.java", `package example;
@RestController
class OrderController {
  @GetMapping("/orders/{id}")
  String get() { return "ok"; }
}`)
	for _, project := range []string{frontend, backend} {
		if _, err := scan.RunBuild(project, config.Defaults(), scan.BuildTargetAgent); err != nil {
			t.Fatal(err)
		}
	}
	workspaceConfig := config.Defaults()
	workspaceConfig.Workspace = true
	workspaceConfig.WorkspaceRoot = workspace
	if _, err := scan.ReconcileWorkspaceTarget(frontend, workspaceConfig, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	layout := scan.NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace"))
	return workspace, layout.Index("api-catalog.json")
}

func writeWorkspaceProjectFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func workspaceProjectOutputForTest(t *testing.T, workspace, projectPath string) string {
	t.Helper()
	registryPath := scan.NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Index("registry.json")
	var registry scan.WorkspaceRegistryRecord
	readTestJSON(t, registryPath, &registry)
	for _, project := range registry.Projects {
		if project.Path == projectPath {
			return filepath.Join(workspace, filepath.FromSlash(project.Path), filepath.FromSlash(project.OutputDir))
		}
	}
	t.Fatalf("workspace registry does not contain project %q", projectPath)
	return ""
}

func writeCatalogEvidenceFixture(t *testing.T, path string) {
	t.Helper()
	var evidence []scan.EvidenceRecord
	readTestJSON(t, path, &evidence)
	for _, id := range []string{"evidence:auth", "evidence:consumer", "evidence:endpoint"} {
		evidence = append(evidence, scan.EvidenceRecord{ID: id})
	}
	writeTestJSON(t, path, evidence)
}

func catalogFixture() scan.APICatalogRecord {
	return scan.APICatalogRecord{
		SchemaVersion: scan.SchemaVersion,
		Endpoints: []scan.APIEndpointRecord{{
			ID:              "endpoint:orders",
			ProviderProject: "services/orders",
			Transport:       "http",
			HTTPMethod:      "GET",
			Path:            "/orders/{id}",
			File:            "src/OrderController.java",
			Line:            10,
			Security: []scan.SecurityEvidenceRecord{{
				Kind:        scan.SecurityUnknown,
				Summary:     "No auth evidence detected",
				File:        "src/Security.java",
				Line:        2,
				Confidence:  scan.ConfidenceUnknown,
				EvidenceIDs: []string{"evidence:auth"},
			}},
			Consumers: []scan.APIConsumerRecord{{
				ID:          "consumer:web",
				Project:     "frontend/web",
				File:        "src/api.ts",
				Line:        3,
				CallAuth:    []scan.SecurityEvidenceRecord{},
				Resolution:  "MATCHED",
				Confidence:  scan.ConfidenceResolved,
				EvidenceIDs: []string{"evidence:consumer"},
			}},
			Mismatches:  []scan.APIMismatchRecord{},
			Confidence:  scan.ConfidenceExact,
			Coverage:    scan.CoverageComplete,
			EvidenceIDs: []string{"evidence:endpoint"},
		}},
	}
}

func withoutManifestFile(files []string, name string) []string {
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if file != name {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func requireAPICatalogFailure(t *testing.T, result Result, wantID string) {
	t.Helper()
	for _, line := range result.Lines {
		if strings.HasPrefix(line, "FAIL api-catalog:") && strings.Contains(line, wantID) {
			return
		}
	}
	t.Fatalf("Doctor did not report an API catalog failure for %q: %v", wantID, result.Lines)
}

func requireAPICatalogFailureContaining(t *testing.T, result Result, wants ...string) {
	t.Helper()
	for _, line := range result.Lines {
		if !strings.HasPrefix(line, "FAIL api-catalog:") {
			continue
		}
		matches := true
		for _, want := range wants {
			if !strings.Contains(line, want) {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	t.Fatalf("Doctor did not report an API catalog failure containing %q: %v", wants, result.Lines)
}
