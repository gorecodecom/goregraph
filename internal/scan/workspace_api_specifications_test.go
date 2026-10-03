package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestWorkspaceAPISpecificationsFindDocumentationRepositoriesWithoutChangingProjects(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "services/orders/go.mod", "module orders\n")
	writeFile(t, root, "documentation/.git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, root, "documentation/API/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, root, "documentation/nested/.git", "gitdir: /not-followed\n")
	writeFile(t, root, "documentation/nested/contract.json", `{"openapi":"3.1.0","paths":{}}`)
	writeFile(t, root, ".gitignore", "ignored/\n")
	writeFile(t, root, "ignored/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, root, "documentation/node_modules/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, root, "documentation/goregraph-out/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, root, "configuration.json", `{"application":"ordinary"}`)
	writeFile(t, root, "documentation/.gitignore", "excluded.yaml\n")
	writeFile(t, root, "documentation/excluded.yaml", swaggerSpecificationFixture)
	if err := os.Symlink(filepath.Join(root, "documentation/API/swagger.yaml"), filepath.Join(root, "linked.yaml")); err != nil {
		t.Fatal(err)
	}
	before, err := discoverWorkspaceProjects(root, root, "goregraph-out")
	if err != nil {
		t.Fatal(err)
	}
	index, fingerprint, err := snapshotWorkspaceAPISpecifications(context.Background(), root, true)
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint == "" || len(index.Documents) != 2 || index.Documents[0].File != "documentation/API/swagger.yaml" || index.Documents[0].Repository != "documentation" || index.Documents[1].Repository != "documentation/nested" {
		t.Fatalf("repository ownership or filtering = %#v", index)
	}
	after, err := discoverWorkspaceProjects(root, root, "goregraph-out")
	if err != nil || !reflect.DeepEqual(before, after) || len(after) != 1 || after[0].Path != "services/orders" {
		t.Fatalf("contract discovery changed code project boundaries: %#v (%v)", after, err)
	}
	if _, err := os.Stat(filepath.Join(root, "documentation/goregraph.yml")); !os.IsNotExist(err) {
		t.Fatal("discovery wrote configuration")
	}
}

func TestWorkspaceAPISpecificationsRespectRepositorySelectionAndDisableSwitch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/.git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, root, "docs/goregraph.yml", "output: private-output\nexclude:\n  - private/\n")
	writeFile(t, root, "docs/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, root, "docs/private/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, root, "docs/private-output/swagger.yaml", swaggerSpecificationFixture)
	index, _, err := snapshotWorkspaceAPISpecifications(context.Background(), root, true)
	if err != nil || len(index.Documents) != 1 {
		t.Fatalf("repository selection ignored: %#v (%v)", index, err)
	}
	writeFile(t, root, ".goregraph-workspace.yml", "api_specifications: false\n")
	index, fingerprint, err := snapshotWorkspaceAPISpecifications(context.Background(), root, true)
	if err != nil || index.Enabled || len(index.Documents) != 0 || fingerprint != "disabled" {
		t.Fatalf("disable switch = %#v, %q (%v)", index, fingerprint, err)
	}
}

func TestWorkspaceAPISpecificationsRespectNestedCodeProjectOutputAndExclusions(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "repository/services/orders")
	writeFile(t, root, "repository/.git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, project, "go.mod", "module orders\n")
	writeFile(t, project, "goregraph.yml", "output: private-output\nexclude:\n  - private/\n")
	writeFile(t, project, "swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, project, "private/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, project, "private-output/swagger.yaml", swaggerSpecificationFixture)
	index, before, err := snapshotWorkspaceAPISpecifications(context.Background(), root, true, project)
	if err != nil || len(index.Documents) != 1 || index.Documents[0].Repository != "repository" {
		t.Fatalf("nested project selection or Git ownership changed: %#v (%v)", index, err)
	}
	writeFile(t, project, "private-output/swagger.yaml", swaggerSpecificationFixture+"# generated change\n")
	after, err := WorkspaceAPISpecificationFingerprint(context.Background(), root, project)
	if err != nil || before != after {
		t.Fatalf("generated output causes watcher feedback: %q != %q (%v)", before, after, err)
	}
}

func TestWorkspaceAPISpecificationFingerprintTracksEditRemovalAndOwnership(t *testing.T) {
	root := t.TempDir()
	file := "docs/swagger.yaml"
	writeFile(t, root, file, swaggerSpecificationFixture)
	fingerprint := func() string {
		t.Helper()
		value, err := WorkspaceAPISpecificationFingerprint(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := fingerprint()
	writeFile(t, root, "docs/.git/HEAD", "ref: refs/heads/main\n")
	owned := fingerprint()
	if first == owned {
		t.Fatal("repository ownership change was invisible")
	}
	writeFile(t, root, file, swaggerSpecificationFixture+"# modification\n")
	edited := fingerprint()
	if edited == owned {
		t.Fatal("contract edit was invisible")
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(file))); err != nil {
		t.Fatal(err)
	}
	if removed := fingerprint(); removed == edited || removed != "" {
		t.Fatalf("removed contract remains fingerprinted: %q", removed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WorkspaceAPISpecificationFingerprint(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

func TestWorkspaceContractSupplementPreservesAgentFactsAndExistingMatches(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	writeFile(t, projects[1], "src/main/java/OrdersController.java", "package orders;\nimport org.springframework.web.bind.annotation.*;\n@RestController\npublic class OrdersController {\n@GetMapping(\"/api\")\npublic String list() { return \"orders\"; }\n@GetMapping(\"/orders/{id}\")\npublic String getOrder() { return \"order\"; }\n}\n")
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	layout := NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace"))
	var beforeAgent, afterAgent AgentContextIndexRecord
	var beforeMatches, afterMatches []WorkspaceContractMatchRecord
	var beforeRegistry, afterRegistry WorkspaceRegistryRecord
	readJSON(t, layout.Agent("context-index.json"), &beforeAgent)
	readJSON(t, layout.Index("contract-matches.json"), &beforeMatches)
	readJSON(t, layout.Index("registry.json"), &beforeRegistry)
	if len(beforeMatches) != 1 || beforeMatches[0].BackendProject != "services/api" || beforeMatches[0].Issue != contractIssueMatched {
		t.Fatalf("fixture did not exercise a resolved cross-service call: %#v", beforeMatches)
	}
	writeFile(t, workspace, "docs/.git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, workspace, "docs/swagger.yaml", swaggerSpecificationFixture)
	writeFile(t, workspace, "docs/openapi-broken.yaml", "openapi: [broken")
	cfg := config.Defaults()
	cfg.WorkspaceRoot = workspace
	plan, err := WorkspaceUpdatePlan(workspace, cfg, BuildTargetAll)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range plan.Items {
		if item.Action != WorkspaceUpdateActionSkip {
			t.Fatalf("documentation change forced a code project scan: %#v", item)
		}
	}
	if current, err := WorkspaceProjectionCurrent(workspace, cfg, BuildTargetAll, DefaultBuildOptions()); err != nil || current {
		t.Fatalf("changed contracts reported current: %v (%v)", current, err)
	}
	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	readJSON(t, layout.Agent("context-index.json"), &afterAgent)
	readJSON(t, layout.Index("contract-matches.json"), &afterMatches)
	readJSON(t, layout.Index("registry.json"), &afterRegistry)
	beforeAgent.Generated, afterAgent.Generated = "", ""
	if !reflect.DeepEqual(beforeAgent, afterAgent) || !reflect.DeepEqual(beforeMatches, afterMatches) || !reflect.DeepEqual(beforeRegistry.Projects, afterRegistry.Projects) {
		t.Fatal("contract supplement changed existing agent evidence, call matches or code projects")
	}
	var specifications APISpecificationIndexRecord
	readJSON(t, layout.Index("api-specifications.json"), &specifications)
	if len(specifications.Documents) != 2 || specifications.Documents[0].Status != "invalid" || specifications.Documents[1].Status != "parsed" {
		t.Fatalf("invalid contract prevented publication or lost diagnostics: %#v", specifications)
	}
	if specifications.Documents[1].Operations[0].LinkStatus != "prefix_candidate" || specifications.Documents[1].Operations[0].MatchBasis != "document_path" || specifications.Documents[1].Operations[0].CodeRoutes[0].Project != "services/api" {
		t.Fatalf("documented contract lost its matching backend source: %#v", specifications)
	}
	if err := os.Remove(filepath.Join(workspace, "docs/swagger.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	readJSON(t, layout.Index("api-specifications.json"), &specifications)
	if len(specifications.Documents) != 1 || specifications.Documents[0].File != "docs/openapi-broken.yaml" {
		t.Fatalf("removed contract survived: %#v", specifications)
	}
}

func TestWorkspaceContractPublicationFailurePreservesCodeAndContractOutputs(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	writeFile(t, workspace, "docs/swagger.yaml", swaggerSpecificationFixture)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	layout := NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace"))
	paths := []string{layout.Manifest, layout.Agent("context-index.json"), layout.Index("api-specifications.json"), layout.Index("workspace-service-map.json"), layout.Dashboard("workspace-map.html")}
	for _, project := range projects {
		paths = append(paths, filepath.Join(project, "goregraph-out/manifest.json"), filepath.Join(project, "goregraph-out/agent/context-index.json"))
	}
	before := map[string][]byte{}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = body
	}
	writeFile(t, workspace, "docs/swagger.yaml", swaggerSpecificationFixture+"# changed before failing publication\n")
	failure := errors.New("injected failure after staging contract and code projections")
	restore := replaceProjectionWriteHookForTest(func(scope, projection string) error {
		if scope == "workspace" && projection == "agent" {
			return failure
		}
		return nil
	})
	defer restore()
	cfg := config.Defaults()
	cfg.WorkspaceRoot = workspace
	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAll); !errors.Is(err, failure) {
		t.Fatalf("failure was not exercised: %v", err)
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil || !reflect.DeepEqual(body, before[path]) {
			t.Fatalf("failed publication changed committed output %s: %v", path, err)
		}
	}
}

func TestWorkspaceUnreadableContractDoesNotBlockChangedCodePublication(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	writeFile(t, workspace, "docs/swagger.yaml", swaggerSpecificationFixture)
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	original := specificationReadFile
	specificationReadFile = func(path string) ([]byte, error) {
		if filepath.Base(path) == "swagger.yaml" {
			return nil, os.ErrPermission
		}
		return original(path)
	}
	defer func() { specificationReadFile = original }()
	writeFile(t, projects[0], "src/api.ts", "export async function load() { return fetch('/changed'); }\n")
	projectConfig := config.Defaults()
	projectConfig.Workspace = false
	if _, err := RunBuild(projects[0], projectConfig, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.WorkspaceRoot = workspace
	if _, err := ReconcileWorkspaceTarget(projects[0], cfg, BuildTargetAll); err != nil {
		t.Fatalf("unreadable document blocked code publication: %v", err)
	}
	layout := NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace"))
	var index APISpecificationIndexRecord
	readJSON(t, layout.Index("api-specifications.json"), &index)
	if index.InventoryComplete || !slices.Contains(index.Issues, "source_unreadable") || len(index.Documents) != 1 || index.Documents[0].Status != "invalid" || len(index.Documents[0].Operations) != 0 {
		t.Fatalf("unreadable contract was reported as usable or silently omitted: %#v", index)
	}
	var contextIndex AgentContextIndexRecord
	readJSON(t, layout.Agent("context-index.json"), &contextIndex)
	found := false
	for _, fact := range contextIndex.Facts {
		if fact.Project == "frontend/web" && fact.Path == "/changed" {
			found = true
		}
	}
	if !found {
		t.Fatal("changed production code did not reach the agent index")
	}
	if current, err := WorkspaceProjectionCurrent(workspace, cfg, BuildTargetAll, DefaultBuildOptions()); err != nil || !current {
		t.Fatalf("degraded supplement destabilized the committed workspace identity: %v (%v)", current, err)
	}
}

func TestWorkspaceInvalidSupplementConfigurationDoesNotBlockCodePublication(t *testing.T) {
	workspace, projects := writeWorkspaceBuildFixture(t)
	writeFile(t, workspace, ".goregraph-workspace.yml", "api_specifications: [broken\n")
	buildWorkspaceProjects(t, workspace, projects, BuildTargetAll)
	var index APISpecificationIndexRecord
	readJSON(t, filepath.Join(workspace, ".goregraph-workspace/index/api-specifications.json"), &index)
	if index.InventoryComplete || !slices.Contains(index.Issues, "workspace_configuration_unavailable") {
		t.Fatalf("invalid supplement configuration was not diagnosed: %#v", index)
	}
}
