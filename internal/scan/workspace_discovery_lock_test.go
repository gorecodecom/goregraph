package scan

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/outputstore"
)

func TestWorkspaceDiscoveryRefreshesIndexedStateUnderLock(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "services", "orders")
	writeFile(t, project, "go.mod", "module example.test/orders\n")
	writeFile(t, project, "main.go", "package orders\n")
	cfg := config.Defaults()
	cfg.Workspace = false
	before, err := discoverWorkspaceProjects(root, root, cfg.OutputDir)
	if err != nil || len(before) != 1 || before[0].Indexed {
		t.Fatalf("initial discovery=%+v err=%v", before, err)
	}
	if _, err := RunBuild(project, cfg, BuildTargetAgent); err != nil {
		t.Fatal(err)
	}
	err = outputstore.WithReads(context.Background(), []string{filepath.Join(project, cfg.OutputDir)}, func() error {
		current, err := refreshLockedWorkspaceProjects(root, root, cfg.OutputDir, before)
		if err == nil && (len(current) != 1 || !current[0].Indexed) {
			t.Fatalf("retained pre-lock index state: %+v", current)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, project, "goregraph.yml", "output: relocated-out\n")
	if _, err := refreshLockedWorkspaceProjects(root, root, cfg.OutputDir, before); err == nil || !strings.Contains(err.Error(), "output roots changed") {
		t.Fatalf("accepted unlocked relocated output: %v", err)
	}
}
