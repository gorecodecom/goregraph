package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

// Real editor exports are optional fixtures; normal tests never start an editor.
func TestRealAssetExportFixtures(t *testing.T) {
	root := os.Getenv("GOREGRAPH_ASSET_SMOKE_ROOT")
	if root == "" {
		t.Skip("explicit external asset-export fixture was not supplied")
	}
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	index, _, err := scanProject(root, cfg, gitignore.Matcher{})
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, node := range index.Assets.Nodes {
		if node.Properties["source_asset"] != nil {
			active++
		}
	}
	if active < 4 {
		t.Fatalf("real export missing from asset/context analysis: %d diagnostics=%#v", active, index.Assets.Diagnostics)
	}
	for _, diagnostic := range index.Assets.Diagnostics {
		if diagnostic.Code == "asset_export_stale" || diagnostic.Code == "asset_invalid_export" {
			t.Fatal(diagnostic)
		}
	}
	if _, err := RunBuild(root, cfg, BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	var context AgentContextIndexRecord
	if err := readWorkspaceJSON(filepath.Join(root, cfg.OutputDir, "agent", "context-index.json"), &context); err != nil {
		t.Fatal(err)
	}
	summaries := 0
	for _, fact := range context.Facts {
		if fact.Summary != "" {
			summaries++
		}
	}
	if summaries == 0 {
		t.Fatal("real asset properties did not enter agent context")
	}
}
