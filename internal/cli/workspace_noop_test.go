package cli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnchangedWorkspaceUpdatePreservesCommittedOutput(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "services", "orders")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"go.mod": "module example.test/orders\n", "main.go": "package orders\nfunc Run() {}\n"} {
		if err := os.WriteFile(filepath.Join(project, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
		return stdout.String()
	}
	snapshot := func() map[string]string {
		t.Helper()
		result := map[string]string{}
		for _, out := range []string{filepath.Join(project, "goregraph-out"), filepath.Join(root, ".goregraph-workspace")} {
			if err := filepath.WalkDir(out, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				result[path] = fmt.Sprintf("%x/%d", sha256.Sum256(body), info.ModTime().UnixNano())
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	run("workspace", "build", "agent", root, "--workspace", root, "--no-update-gitignore", "--progress", "off")
	before := snapshot()
	output := run("workspace", "update", root, "--workspace", root, "--target", "agent", "--no-update-gitignore", "--progress", "off")
	if !strings.Contains(output, "Updated 0 workspace project(s); 1 unchanged.") {
		t.Fatalf("output=%s", output)
	}
	after := snapshot()
	if len(before) != len(after) {
		t.Fatal("unchanged update changed output file count")
	}
	for path, identity := range before {
		if after[path] != identity {
			t.Errorf("unchanged update rewrote %s", path)
		}
	}
}
