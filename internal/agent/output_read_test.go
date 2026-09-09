package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/outputstore"
)

func TestBuildContextWaitsForOutputPublication(t *testing.T) {
	root := outputReadTestRoot(t)
	release, updateDone := holdOutputPublication(t, filepath.Join(root, "goregraph-out"))

	readDone := make(chan error, 1)
	go func() {
		_, err := BuildContext(ContextRequest{
			Root: root, Query: "GET /users", ProtocolVersion: AdaptiveV2,
		})
		readDone <- err
	}()
	assertOutputReadWaits(t, readDone)
	close(release)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
}

func TestServiceRunWaitsForOutputPublication(t *testing.T) {
	root := outputReadTestRoot(t)
	release, updateDone := holdOutputPublication(t, filepath.Join(root, "goregraph-out"))

	readDone := make(chan error, 1)
	go func() {
		_, err := (Service{}).Run(Request{Root: root, Task: "coverage"})
		readDone <- err
	}()
	assertOutputReadWaits(t, readDone)
	close(release)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	if err := <-readDone; err == nil {
		t.Fatal("coverage unexpectedly loaded from an empty published output")
	}
}

func holdOutputPublication(t *testing.T, root string) (chan struct{}, chan error) {
	t.Helper()
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- outputstore.Update(context.Background(), outputstore.UpdateRequest{
			Root: root,
			Write: func(string) error {
				close(started)
				<-release
				return nil
			},
		})
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("output publication failed before staging: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("output publication did not start")
	}
	return release, done
}

func outputReadTestRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(".", ".agent-output-read-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func assertOutputReadWaits(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("output read returned before publication completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}
