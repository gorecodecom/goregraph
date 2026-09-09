package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceUpdateMigratesIgnoreRulesBeforePlanning(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "services", "orders")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"go.mod": "module example.test/orders\n", "main.go": "package orders\n", ".gitignore": "goregraph-out/\n"} {
		if err := os.WriteFile(filepath.Join(project, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
		}
		return stdout.String()
	}
	run("workspace", "build", "agent", root, "--workspace", root, "--no-update-gitignore", "--progress", "off")
	run("workspace", "update", root, "--workspace", root, "--target", "agent", "--progress", "off")
	body, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil || !strings.Contains(string(body), ".goregraph-lock-*") {
		t.Fatalf("migration missing: %s %v", body, err)
	}
	output := run("workspace", "update", root, "--workspace", root, "--target", "agent", "--progress", "off")
	if !strings.Contains(output, "Updated 0 workspace project(s); 1 unchanged.") {
		t.Fatalf("migration left stale input identities: %s", output)
	}
}
