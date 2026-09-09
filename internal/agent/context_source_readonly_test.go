package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSourcePathRejectsSymlinkOutsideProject(t *testing.T) {
	root := t.TempDir()
	outside := writeSourceFile(t, t.TempDir(), "Outside.java", "class Outside {}\n")
	if err := os.Symlink(outside, filepath.Join(root, "Alias.java")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := resolveSourcePath(loadedContextIndex{ScopeRoot: root}, sourceCandidate{Path: "Alias.java"})
	if err == nil || !strings.Contains(err.Error(), "source path is unsafe") {
		t.Fatalf("source outside project accepted: %v", err)
	}
}
