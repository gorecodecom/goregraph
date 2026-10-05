package agent

import (
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestGoContextExplainsSelectedUnresolvedCalls(t *testing.T) {
	t.Setenv("GOREGRAPH_WATCH_HOME", t.TempDir())
	root := t.TempDir()
	writeSourceFile(t, root, "go.mod", "module example.test/context\n\ngo 1.24\n")
	writeSourceFile(t, root, "work.go", `package context
func KnownTarget() {}
func ProcessWork(run func()) {
 KnownTarget()
 run()
}
`)
	writeSourceFile(t, root, "unrelated.go", `package context
func Unrelated(background func()) { background() }
`)
	cfg := config.Defaults()
	cfg.Workspace, cfg.UpdateGitignore = false, false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	initializeSourceReadLocks(t, root)
	pack, err := BuildContext(ContextRequest{Root: root, Query: "ProcessWork call chain and unresolved run function", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.CallDiagnostics) != 1 || pack.CallDiagnostics[0].File != "work.go" || pack.CallDiagnostics[0].Line != 5 || pack.CallDiagnostics[0].Reason != "dynamic_function_value" {
		t.Fatalf("selected call has no precise explanation: %+v", pack)
	}
	if len(pack.SourceSections) == 0 {
		t.Fatalf("source evidence lost: %+v", pack)
	}
	if pack.Watcher == nil {
		t.Fatal("watcher metadata was displaced by diagnostics")
	}
	writeSourceFile(t, root, "work.go", `package context
func KnownTarget() {}
func ProcessWork() { KnownTarget() }
`)
	stale, err := BuildContext(ContextRequest{Root: root, Query: "ProcessWork call chain", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	if len(stale.CallDiagnostics) > 0 {
		t.Fatalf("old dynamic call was asserted against changed source: %+v", stale.CallDiagnostics)
	}
}

func TestContextDiagnosticsRequireMatchingCurrentSourceReceipt(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	file := filepath.Join(root, "main.go")
	writeSourceFile(t, root, "main.go", "package app\nfunc Run() {}\n")
	loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{SourceHashes: map[string]string{"main.go": "original"}}}
	diagnostic := scan.GoCallDiagnosticRecord{Project: "app", File: "main.go", Line: 2, Caller: "Run", Method: "Save", Reason: "dynamic_function_value"}
	section := ContextSourceSection{Project: "app", Path: "main.go", StartLine: 1, EndLine: 2, SourceState: "indexed_range_current", ReadReceipt: makeSourceReadReceipt(sourceReadFingerprint(sourceFile{Path: file, Hash: "original"}), []SourceReadRange{{1, 2}})}
	pack := ContextPack{SourceSections: []ContextSourceSection{section}}
	if !contextDiagnosticHasCurrentSource(pack, loaded, diagnostic) {
		t.Fatal("matching source receipt was rejected")
	}
	loaded.Index.SourceHashes["main.go"] = "old"
	if contextDiagnosticHasCurrentSource(pack, loaded, diagnostic) {
		t.Fatal("stale call metadata trusted")
	}
	loaded.Index.SourceHashes["main.go"] = "original"
	diagnostic.Project = "other"
	if contextDiagnosticHasCurrentSource(pack, loaded, diagnostic) {
		t.Fatal("cross-project diagnostic leaked")
	}
}
