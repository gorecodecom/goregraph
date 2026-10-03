package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestSemanticSnapshotContextRejectsChangedUnselectedConfiguration(t *testing.T) {
	for _, change := range []string{"configuration", "added_source", "report"} {
		t.Run(change, func(t *testing.T) { assertSemanticSnapshotChange(t, change) })
	}
}

func assertSemanticSnapshotChange(t *testing.T, change string) {
	t.Helper()
	root := t.TempDir()
	body := "class Service { public void Target(){} public void Run(){Target();} }"
	writeSourceFile(t, root, "Service.cs", body)
	writeSourceFile(t, root, "App.csproj", "<Project></Project>")
	hash := func(text string) string { sum := sha256.Sum256([]byte(text)); return hex.EncodeToString(sum[:]) }
	inputs := map[string]string{"Service.cs": hash(body), "App.csproj": hash("<Project></Project>")}
	report := map[string]any{"schema_version": 1, "language": "csharp", "producer": "Roslyn fixture", "inputs": inputs, "covered_files": []string{"Service.cs"}, "declarations": []any{}, "references": []any{}}
	data, _ := json.Marshal(report)
	writeSourceFile(t, root, "Report.goregraph-csharp.json", string(data))
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadContextIndex(ContextRequest{Root: root})
	if err != nil || !semanticDependenciesCurrent(loaded) {
		t.Fatal(err)
	}
	switch change {
	case "configuration":
		writeSourceFile(t, root, "App.csproj", "<Project>changed</Project>")
	case "added_source":
		writeSourceFile(t, root, "Other.cs", "class Other {}")
	case "report":
		writeSourceFile(t, root, "Report.goregraph-csharp.json", string(data)+"\n")
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "Explain Service.Run and Target in C#", ProtocolVersion: AdaptiveV2})
	if err != nil || !pack.FallbackRequired || pack.FallbackReason != ContextFallbackIndexStale {
		t.Fatal(err, pack)
	}
	if len(pack.CallChain) != 0 || len(pack.SourceSections) != 0 {
		t.Fatal("stale compiler bindings were delivered", pack)
	}
}

func TestSemanticDependencyPathsCannotEscapeProject(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	body := "private"
	writeSourceFile(t, outside, "Private.cs", body)
	sum := sha256.Sum256([]byte(body))
	os.Symlink(filepath.Join(outside, "Private.cs"), filepath.Join(root, "Escaping.cs"))
	loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{SemanticDependencies: []scan.SemanticDependencyRecord{{Inputs: map[string]string{"Escaping.cs": hex.EncodeToString(sum[:])}}}}}
	if semanticDependenciesCurrent(loaded) {
		t.Fatal("escaping semantic dependency accepted")
	}
}
