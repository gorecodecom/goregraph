//go:build windows

package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSourcePathRejectsJunctionOutsideProject(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeSourceFile(t, outside, "Outside.java", "class Outside {}\n")
	alias := filepath.Join(root, "alias")
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", alias, outside).CombinedOutput(); err != nil {
		t.Fatalf("create junction: %v: %s", err, output)
	}
	defer os.Remove(alias)
	_, err := resolveSourcePath(loadedContextIndex{ScopeRoot: root}, sourceCandidate{Path: "alias/Outside.java"})
	if err == nil || !strings.Contains(err.Error(), "source path is unsafe") {
		t.Fatalf("junction escaped project: %v", err)
	}
}
