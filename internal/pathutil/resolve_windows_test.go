//go:build windows

package pathutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveJunctionUsesTargetIdentity(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", alias, target).CombinedOutput(); err != nil {
		t.Fatalf("create junction: %v: %s", err, output)
	}
	defer os.Remove(alias)
	want, err := Resolve(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(alias)
	if err != nil || got != want {
		t.Fatalf("junction resolved to %q instead of %q: %v", got, want, err)
	}
}
