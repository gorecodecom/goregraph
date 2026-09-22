package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func toolingFixtureSources(t *testing.T) ([]AgentAuditSource, map[string][]DashboardToolingObservation) {
	t.Helper()
	var sources []AgentAuditSource
	notes := map[string][]DashboardToolingObservation{}
	err := fs.WalkDir(os.DirFS("../mcp/testdata/storybook-audit/project"), ".", func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(filepath.Join("../mcp/testdata/storybook-audit/project", filepath.FromSlash(file)))
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(body), "\r\n", "\n")
		if source, ok := extractAgentAuditSource(FileRecord{Path: file, Hash: "fixture"}, text); ok {
			sources = append(sources, source)
			notes[file] = extractDashboardToolingObservations(source, text)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sources, notes
}

func TestDashboardToolingUsesAuditEvidenceWithoutChangingIt(t *testing.T) {
	sources, notes := toolingFixtureSources(t)
	original, _ := json.Marshal(sources)
	record := buildDashboardTooling(sources, notes)
	after, _ := json.Marshal(sources)
	if string(original) != string(after) {
		t.Fatal("dashboard mutated agent audit metadata")
	}
	if record.Version != 1 || record.Total != 13 || record.Truncated {
		t.Fatalf("wrong fixture inventory: %+v", record)
	}
	files := map[string]DashboardToolingSource{}
	for _, source := range record.Sources {
		files[source.File] = source
	}
	for _, file := range []string{".gitlab/unreferenced.yml", "src/orders.ts"} {
		if _, ok := files[file]; ok {
			t.Errorf("unrelated file selected: %s", file)
		}
	}
	preview := files[".storybook/preview.ts"]
	if len(preview.Observations) != 1 || preview.Observations[0].Value != "off" {
		t.Fatalf("missing A11y declaration: %+v", preview)
	}
	if len(files[".gitlab/storybook.yml"].Observations) != 1 || files[".gitlab/storybook.yml"].Observations[0].Value != "true" {
		t.Fatal("missing non-blocking declaration")
	}
	if len(files["src/ProductCard.stories.tsx"].References) != 2 {
		t.Fatal("story/component/fixture links missing")
	}
	if got := buildDashboardTooling(nil, nil); got.Version != 1 || len(got.Sources) != 0 {
		t.Fatalf("empty inventory: %+v", got)
	}
}

func TestDashboardToolingIgnoresCommentsAndScalarBodies(t *testing.T) {
	source := AgentAuditSource{Kind: "configuration"}
	for _, body := range []string{"// a11y: { test: 'off' }", "const note = \"a11y: { test: 'off' }\";", "/* a11y: { test: 'off' } */"} {
		if notes := extractDashboardToolingObservations(source, body); len(notes) > 0 {
			t.Errorf("non-code became declaration: %s", body)
		}
	}
	notes := extractDashboardToolingObservations(source, "export default { parameters: { a11y: {\n test: 'off' } } };")
	if len(notes) != 1 || notes[0].Line != 1 {
		t.Fatalf("literal declaration: %+v", notes)
	}
	ci := extractDashboardToolingObservations(AgentAuditSource{Kind: "ci"}, "job:\n  script: |\n    allow_failure: true\n  allow_failure: false\n# allow_failure: true\n")
	if len(ci) != 1 || ci[0].Value != "false" || ci[0].Line != 4 {
		t.Fatalf("scalar/comment mistaken for CI declaration: %+v", ci)
	}
}

func TestDashboardToolingBuildAndWorkspaceProjection(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "frontend", "store")
	fixture := os.DirFS("../mcp/testdata/storybook-audit/project")
	if err := fs.WalkDir(fixture, ".", func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(fixture, file)
		if err != nil {
			return err
		}
		writeFile(t, root, file, strings.ReplaceAll(string(body), "\r\n", "\n"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	cfg.Workspace = true
	cfg.WorkspaceRoot = workspace
	cfg.UpdateGitignore = false
	result, err := RunBuild(root, cfg, BuildTargetDashboard)
	if err != nil {
		t.Fatal(err)
	}
	var record DashboardToolingRecord
	readJSON(t, filepath.Join(result.OutputDir, "index/tooling.json"), &record)
	if record.Version != 1 || record.Total != 13 {
		t.Fatalf("dashboard-only scan lost tooling: %+v", record)
	}
	body, err := os.ReadFile(filepath.Join(workspace, ".goregraph-workspace/dashboard/workspace-map.html"))
	if err != nil {
		t.Fatal(err)
	}
	marker := "const workspacePayload = "
	start := strings.Index(string(body), marker) + len(marker)
	end := strings.Index(string(body[start:]), ";\n")
	var payload workspaceDashboardPayload
	if err := json.Unmarshal(body[start:start+end], &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Tooling["frontend/store"].Total != 13 {
		t.Fatalf("workspace lost project identity: %+v", payload.Tooling)
	}
	if _, err := os.Stat(filepath.Join(root, "goregraph-out/agent/context-index.json")); !os.IsNotExist(err) {
		t.Fatal("dashboard-only build changed agent projection")
	}
	if output := os.Getenv("GOREGRAPH_TOOLING_PREVIEW"); output != "" {
		if err := os.MkdirAll(output, 0700); err != nil {
			t.Fatal(err)
		}
		payload.Graph.Root = "Tooling-Demo"
		assets := map[string][]byte{}
		for _, asset := range payload.CodeUsageAssets {
			body, err := os.ReadFile(filepath.Join(workspace, ".goregraph-workspace/dashboard", filepath.FromSlash(asset)))
			if err != nil {
				t.Fatal(err)
			}
			assets[asset] = body
		}
		writeExplorerArtifacts(t, output, workspaceDashboardArtifacts{HTML: renderWorkspaceDashboardDocument("Tests & Tooling — Vorschau", marshalDashboardPayload(payload)), Assets: assets})
	}
}

func TestDashboardToolingRebuildPreservesAgentProjection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".storybook/preview.ts", "export default { parameters: { a11y: { test: 'off' } } };\n")
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	result, err := RunBuild(root, cfg, BuildTargetAgent)
	if err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(result.OutputDir, "agent/context-index.json")
	before, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunBuild(root, cfg, BuildTargetDashboard); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dashboard rebuild changed the agent context projection")
	}
	current := CurrentBuildIdentity(cfg, DefaultBuildOptions(), "ignore", "source")
	previous := current
	previous.DashboardRevision = "2"
	if identityChange(previous, current, BuildTargetDashboard) != "dashboard revision changed" || identityChange(previous, current, BuildTargetAgent) != "" {
		t.Fatal("dashboard upgrade invalidates the agent workflow")
	}
}
