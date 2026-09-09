package scan

import (
	"testing"
)

func TestDashboardAssetsChangeIdentityWithContent(t *testing.T) {
	symbols := WorkspaceSymbolIndexRecord{Symbols: []CanonicalSymbolRecord{{ID: "symbol:a", Project: "services/a"}}}
	before, beforePaths := buildWorkspaceDashboardUsageAssets(symbols, WorkspaceSymbolUsageIndexRecord{Generated: "old"})
	after, afterPaths := buildWorkspaceDashboardUsageAssets(symbols, WorkspaceSymbolUsageIndexRecord{Generated: "new"})
	if beforePaths["services/a"] == afterPaths["services/a"] {
		t.Fatal("changed shard reused old asset URL")
	}
	for name, data := range before {
		if len(data) == 0 || len(after[name]) > 0 {
			t.Fatal("old asset was replaced")
		}
	}
}
