package scan

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestWorkspaceAPISpecificationIndexedEvidence replays existing selected sources
// and indexes without scanning, refreshing, or modifying the inspected workspace.
func TestWorkspaceAPISpecificationIndexedEvidence(t *testing.T) {
	root := os.Getenv("GOREGRAPH_WORKSPACE_EVIDENCE_ROOT")
	if root == "" {
		t.Skip("set GOREGRAPH_WORKSPACE_EVIDENCE_ROOT for read-only indexed evidence replay")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	layout := NewWorkspaceOutputLayout(filepath.Join(root, ".goregraph-workspace"))
	var registry WorkspaceRegistryRecord
	var specifications APISpecificationIndexRecord
	readJSON(t, layout.Index("registry.json"), &registry)
	readJSON(t, layout.Index("api-specifications.json"), &specifications)
	if !specifications.Enabled || !specifications.InventoryComplete {
		t.Fatal("evidence replay requires a complete enabled contract inventory")
	}
	var projects []workspaceIndexProject
	for _, record := range registry.Projects {
		if !record.Indexed {
			continue
		}
		relative, err := filepath.Rel(root, record.AbsPath)
		if err != nil || !filepath.IsLocal(relative) {
			t.Fatalf("indexed project is outside evidence root: %s", record.Path)
		}
		project := workspaceIndexProject{record: record}
		output := NewProjectOutputLayout(filepath.Join(record.AbsPath, record.OutputDir))
		if err := readWorkspaceJSON(output.Index("routes.json"), &project.routes); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		var spring SpringIndex
		if err := readWorkspaceJSON(output.Index("spring.json"), &spring); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		project.endpoints = spring.Endpoints
		projects = append(projects, project)
	}
	for i, old := range specifications.Documents {
		if !filepath.IsLocal(old.File) {
			t.Fatalf("contract is outside evidence root: %s", old.File)
		}
		body, err := os.ReadFile(filepath.Join(root, old.File))
		if err != nil {
			t.Fatal(err)
		}
		if len(body) > 512*1024 || fmt.Sprintf("%x", sha256.Sum256(body)) != old.SourceHash {
			t.Fatalf("contract changed after the indexed snapshot: %s", old.File)
		}
		record, recognized := parseAPISpecification(old.File, body)
		if !recognized || record.Status != old.Status {
			t.Fatalf("contract parsing changed unexpectedly: %s", old.File)
		}
		record.Repository, record.OwnerProject, record.SourceHash = old.Repository, old.OwnerProject, old.SourceHash
		specifications.Documents[i] = record
	}
	linkWorkspaceAPISpecifications(&specifications, projects)
	counts := map[string]int{}
	services := map[string]bool{}
	for _, document := range specifications.Documents {
		for _, operation := range document.Operations {
			counts[operation.LinkStatus]++
			for _, route := range operation.CodeRoutes {
				services[route.Project] = true
			}
		}
	}
	if counts["prefix_candidate"] == 0 || len(services) == 0 {
		t.Fatalf("indexed evidence reproduced the missing prefix candidates: %v", counts)
	}
	t.Logf("%d contract documents; operation links %v; %d service projects", len(specifications.Documents), counts, len(services))
	var graph WorkspaceGraphRecord
	var serviceMap WorkspaceServiceMapRecord
	var traces WorkspaceEndpointTraceIndexRecord
	var catalog APICatalogRecord
	readJSON(t, layout.Index("workspace-graph.json"), &graph)
	readJSON(t, layout.Index("workspace-service-map.json"), &serviceMap)
	readJSON(t, layout.Index("workspace-endpoint-traces.json"), &traces)
	readJSON(t, layout.Index("api-catalog.json"), &catalog)
	serviceMap.APISpecifications = &specifications
	preview := renderWorkspaceDashboardHTML(graph, serviceMap, traces, catalog, WorkspaceSymbolIndexRecord{}, WorkspaceSymbolUsageIndexRecord{}, nil)
	directory := t.TempDir()
	if evidence := os.Getenv("GOREGRAPH_DASHBOARD_EVIDENCE_DIR"); evidence != "" {
		directory = evidence
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "workspace-contract-evidence-preview.html"), []byte(preview), 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(specifications, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "workspace-contract-evidence.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}
