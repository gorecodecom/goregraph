package pathutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExistingPathsAndMissingPath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Source.java")
	if err := os.WriteFile(file, []byte("class Source {}"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, file} {
		resolved, err := Resolve(path)
		if err != nil {
			t.Fatal(err)
		}
		original, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.Stat(resolved)
		if err != nil || !os.SameFile(original, actual) {
			t.Fatalf("resolved path changed file identity: %q, %v", resolved, err)
		}
	}
	if _, err := Resolve(filepath.Join(root, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing path accepted: %v", err)
	}
}

func TestResolveSymlinkUsesTargetIdentity(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	want, err := Resolve(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(alias)
	if err != nil || got != want {
		t.Fatalf("alias resolved to %q instead of %q: %v", got, want, err)
	}
}
