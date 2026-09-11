package scan

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestWithExistingOutputReadDoesNotBootstrapLocks(t *testing.T) {
	root := t.TempDir()
	called := false
	err := WithExistingOutputRead(context.Background(), root, func() error { called = true; return nil })
	if !errors.Is(err, os.ErrNotExist) || called {
		t.Fatalf("missing locks: %v callback=%v", err, called)
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 0 {
		t.Fatalf("read wrote state: %v %v", files, err)
	}
	if err := WithOutputRead(context.Background(), root, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := WithExistingOutputRead(context.Background(), root, func() error { called = true; return nil }); err != nil || !called {
		t.Fatalf("existing lock read: %v callback=%v", err, called)
	}
}
