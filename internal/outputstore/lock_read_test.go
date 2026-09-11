package outputstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSharedLockUsesReadOnlyHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.lock")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := acquireFileLock(context.Background(), path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.release()
	// A write outside the locked byte must still fail on a read-only handle.
	if _, err := reader.file.WriteAt([]byte("x"), 10); err == nil {
		t.Fatal("shared lock handle permits writes")
	}
	second, err := acquireFileLock(context.Background(), path, true)
	if err != nil {
		t.Fatalf("concurrent reader blocked: %v", err)
	}
	if err := second.release(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	writer, err := acquireFileLock(ctx, path, false)
	if writer != nil {
		writer.release()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reader did not exclude writer: %v", err)
	}
}

func TestSharedLockOpensReadOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.lock")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	reader, err := acquireFileLock(context.Background(), path, true)
	if err != nil {
		t.Fatalf("read-only lock file rejected: %v", err)
	}
	if err := reader.release(); err != nil {
		t.Fatal(err)
	}
}

func TestWithExistingReadsNeverCreatesMissingLock(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "output")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	called := false
	err := WithExistingReads(context.Background(), []string{root}, func() error { called = true; return nil })
	if !errors.Is(err, os.ErrNotExist) || called {
		t.Fatalf("missing lock did not fail closed: %v callback=%v", err, called)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 || entries[0].Name() != "output" {
		t.Fatalf("read created state: %v %v", entries, err)
	}
	if err := WithReads(context.Background(), []string{root}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := WithExistingReads(context.Background(), []string{root}, func() error { called = true; return nil }); err != nil || !called {
		t.Fatalf("existing lock read failed: %v callback=%v", err, called)
	}
}
