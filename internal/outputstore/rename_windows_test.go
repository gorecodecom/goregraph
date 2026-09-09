//go:build windows

package outputstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestPublicationWaitsForTransientWindowsReader(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	request := replacement(root, "new")
	var released chan error
	request.Validate = func(stage string) error {
		reader, err := os.Open(filepath.Join(stage, "sentinel"))
		if err != nil {
			return err
		}
		released = make(chan error, 1)
		go func() {
			time.Sleep(150 * time.Millisecond)
			released <- reader.Close()
		}()
		return nil
	}
	err := Update(context.Background(), request)
	if released != nil {
		if closeErr := <-released; closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if err != nil {
		t.Fatalf("brief reader lock aborted publication: %v", err)
	}
	assertSnapshot(t, root, "new")
	if err := WithRead(context.Background(), root, func(string) error { return nil }); err != nil {
		t.Fatalf("published output is not readable: %v", err)
	}
}

func TestPublicationReportsPersistentWindowsReader(t *testing.T) {
	root := previousOutput(t)
	reader, err := os.Open(filepath.Join(root, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	started := time.Now()
	err = Update(context.Background(), replacement(root, "new"))
	if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32)) {
		t.Fatalf("expected Windows lock error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("persistent lock was not bounded: %s", elapsed)
	}
	assertSnapshot(t, root, "previous")
	if err := WithRead(context.Background(), root, func(string) error { return nil }); err != nil {
		t.Fatalf("previous output is not readable: %v", err)
	}
}
