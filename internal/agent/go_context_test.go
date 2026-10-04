package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestGoContextPreservesUnconnectedCLIImplementationAndReadableSource(t *testing.T) {
	root := t.TempDir()
	const path = "internal/cli/watch_status.go"
	writeSourceFile(t, root, "go.mod", "module example.test/watch\n\ngo 1.24\n")
	writeSourceFile(t, root, path, "package cli\n\ntype Activity struct { Updating bool }\n\nfunc printWatchActivity(activity Activity) string {\n\tif activity.Updating { return \"updating index\" }\n\treturn \"monitoring files\"\n}\n")
	writeSourceFile(t, root, "internal/cli/limits.go", "package cli\nconst maxWatcherDelay = 30\nvar activeWatcherCount = 0\n")
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, "agent", "context-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index scan.AgentContextIndexRecord
	if err := json.Unmarshal(body, &index); err != nil {
		t.Fatal(err)
	}
	if index.SourceHashes[path] == "" {
		t.Fatal("unconnected Go implementation has no indexed source hash")
	}
	if index.SourceHashes["internal/cli/limits.go"] == "" {
		t.Fatal("Go configuration declarations have no indexed source hash")
	}
	initializeSourceReadLocks(t, root)
	read, err := ReadSource(ReadSourceRequest{Root: root, Files: []SourceReadFileRequest{{Path: path, Ranges: []SourceReadRange{{5, 8}}}}})
	if err != nil || len(read.Files) != 1 || len(read.Files[0].Sections) != 1 {
		t.Fatalf("indexed Go source is not readable: %+v %v", read, err)
	}
	pack, err := BuildContext(ContextRequest{Root: root, Query: "printWatchActivity Go watcher activity formatting", ProtocolVersion: AdaptiveV2})
	if err != nil || !contextPackHasFile(pack, path) {
		t.Fatalf("Go CLI implementation is absent from task context: %+v %v", pack, err)
	}
	if len(pack.SourceSections) == 0 {
		t.Fatalf("Go context contains no source evidence: %+v", pack)
	}
	limits, err := BuildContext(ContextRequest{Root: root, Query: "maxWatcherDelay Go watcher delay configuration", ProtocolVersion: AdaptiveV2})
	if err != nil || !contextPackHasFile(limits, "internal/cli/limits.go") {
		t.Fatalf("Go configuration declaration is absent from context: %+v %v", limits, err)
	}
}

func TestGoContextFollowsImportedCalleeInsteadOfSameNamedUtility(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "go.mod", "module example.test/graph\n\ngo 1.24\n")
	writeSourceFile(t, root, "internal/cli/watch_status.go", `package cli
import "example.test/graph/internal/watch"
func coveringWorkspaceWatcher(directory string) { watch.Resolve(directory, true) }
`)
	writeSourceFile(t, root, "internal/watch/watch.go", `package watch
func Resolve(directory string, workspace bool) {}
`)
	writeSourceFile(t, root, "internal/pathutil/resolve.go", `package pathutil
func Resolve(directory string) {}
`)
	cfg := config.Defaults()
	cfg.Workspace, cfg.UpdateGitignore = false, false
	if _, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	initializeSourceReadLocks(t, root)
	pack, err := BuildContext(ContextRequest{Root: root, Query: "coveringWorkspaceWatcher invokes watch.Resolve while determining the active workspace watcher", ProtocolVersion: AdaptiveV2})
	if err != nil {
		t.Fatal(err)
	}
	if !contextPackHasFile(pack, "internal/cli/watch_status.go") || !contextPackHasFile(pack, "internal/watch/watch.go") {
		t.Fatalf("imported Go call chain missing from context: %+v", pack)
	}
	if contextPackHasFile(pack, "internal/pathutil/resolve.go") {
		t.Fatalf("unrelated utility was presented as a callee: %+v", pack)
	}
}
