package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestBuildJSONProgressUsesStderr(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.ts"), []byte("export const value = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"build", "agent", root, "--no-workspace", "--no-update-gitignore", "--progress", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n")) {
		var event scan.BuildEvent
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("not a progress event: %s", line)
		}
		if event.File == "main.ts" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing file progress: %s", stderr.String())
	}
}

func TestBuildRejectsNegativeFileBudget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"scan", "--file-timeout", "-1s"}, &stdout, &stderr); code != 2 {
		t.Fatalf("code=%d", code)
	}
}
