package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/watch"
)

func TestWorkspaceWatcherAutomaticallyCatchesSourceChangesMadeDuringPreparation(t *testing.T) {
	t.Setenv("GOREGRAPH_WATCH_HOME", t.TempDir())
	root := t.TempDir()
	project := filepath.Join(root, "services/a")
	writeFile(t, project, "go.mod", "module example.test/a\n")
	writeFile(t, project, "main.go", "package service\nfunc InitialValue() {}\n")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"workspace", "build", "all", root, "--workspace", root, "--no-update-gitignore", "--progress", "off"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	writeFile(t, project, "main.go", "package service\nfunc FirstChange() {}\n")
	selected, err := watch.Resolve(root, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var updates atomic.Int32
	var changed atomic.Bool
	done := make(chan error, 1)
	go func() {
		done <- watch.RunWithProgress(ctx, selected, func(report func(scan.BuildEvent)) error {
			options := scan.DefaultBuildOptions()
			options.Observer = func(event scan.BuildEvent) {
				report(event)
				if event.Phase == "workspace-project" && event.Outcome == "completed" && changed.CompareAndSwap(false, true) {
					writeFile(t, project, "main.go", "package service\nfunc ChangedDuringPreparation() {}\n")
				}
			}
			var out, diagnostic bytes.Buffer
			if code := runWorkspaceUpdate([]string{root, "--workspace", root, "--no-update-gitignore"}, &out, &diagnostic, buildExecution{ctx: ctx, options: options}); code != 0 {
				return fmt.Errorf("update failed: %s", diagnostic.String())
			}
			updates.Add(1)
			return nil
		})
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	deadline := time.Now().Add(40 * time.Second)
	for updates.Load() < 2 {
		if time.Now().After(deadline) {
			status, _ := watch.GetStatus(selected)
			t.Fatalf("watcher did not catch up automatically: %+v", status)
		}
		time.Sleep(50 * time.Millisecond)
	}
	body, err := os.ReadFile(filepath.Join(project, "goregraph-out/index/files.json"))
	if err != nil {
		t.Fatal(err)
	}
	var files []scan.FileRecord
	if err := json.Unmarshal(body, &files); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(project, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range files {
		if file.Path == "main.go" {
			found = file.Hash == fmt.Sprintf("%x", sha256.Sum256(source))
		}
	}
	if !found {
		t.Fatal("automatic follow-up did not publish the current source")
	}
	assertWorkspaceProjectionValid(t, root)
}
