package outputstore

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCommittedSnapshotsRemainReadableDuringPreparation(t *testing.T) {
	for _, phase := range []string{"write", "validate", "shared validation"} {
		t.Run(phase, func(t *testing.T) {
			roots := []string{previousOutput(t), previousOutput(t)}
			ready, release := make(chan struct{}), make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			defer finish()
			pause := func() { close(ready); <-release }
			requests := []UpdateRequest{replacement(roots[0], "new"), replacement(roots[1], "new")}
			write := requests[1].Write
			requests[1].Write = func(stage string) error {
				if err := write(stage); err != nil {
					return err
				}
				if phase == "write" {
					pause()
				}
				return nil
			}
			requests[1].Validate = func(string) error {
				if phase == "validate" {
					pause()
				}
				return nil
			}
			done := make(chan error, 1)
			go func() {
				done <- UpdateManyValidated(context.Background(), requests, func() error {
					if phase == "shared validation" {
						pause()
					}
					return nil
				})
			}()
			select {
			case <-ready:
			case err := <-done:
				t.Fatalf("preparation failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("preparation did not start")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := WithExistingReads(ctx, roots, func() error {
				for _, root := range roots {
					body, err := os.ReadFile(filepath.Join(root, "sentinel"))
					if err != nil || string(body) != "previous" {
						return errors.Join(err, errors.New("unpublished snapshot became visible"))
					}
				}
				return nil
			}); err != nil {
				t.Fatalf("committed snapshot unavailable during %s: %v", phase, err)
			}
			finish()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			for _, root := range roots {
				assertSnapshot(t, root, "new")
			}
		})
	}
}

func TestConcurrentWritersSerializePreparation(t *testing.T) {
	root := previousOutput(t)
	ready, release := make(chan struct{}), make(chan struct{})
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	secondEntered := make(chan struct{})
	go func() {
		firstDone <- Update(context.Background(), UpdateRequest{Root: root, Write: func(stage string) error {
			close(ready)
			<-release
			return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("first"), 0644)
		}})
	}()
	<-ready
	go func() {
		secondDone <- Update(context.Background(), UpdateRequest{Root: root, Write: func(stage string) error {
			close(secondEntered)
			body, err := os.ReadFile(filepath.Join(stage, "sentinel"))
			if err != nil || string(body) != "first" {
				return errors.Join(err, errors.New("second writer copied an obsolete snapshot"))
			}
			return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("second"), 0644)
		}})
	}()
	select {
	case <-secondEntered:
		t.Error("second writer entered before first publication")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "second")
}

func TestReaderWaitsForCrossRootRollbackAfterEarlierPromotion(t *testing.T) {
	roots := []string{previousOutput(t), previousOutput(t)}
	sort.Strings(roots)
	ready, release := make(chan struct{}), make(chan struct{})
	ops := systemOperations()
	rename := ops.rename
	ops.rename = func(from, to string) error {
		if to == roots[1] && strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
			close(ready)
			<-release
			return errors.New("second promotion failed")
		}
		return rename(from, to)
	}
	done := make(chan error, 1)
	go func() {
		done <- updateMany(context.Background(), []UpdateRequest{replacement(roots[0], "new"), replacement(roots[1], "new")}, ops)
	}()
	<-ready
	reader := make(chan error, 1)
	go func() {
		reader <- WithReads(context.Background(), roots, func() error {
			for _, root := range roots {
				body, err := os.ReadFile(filepath.Join(root, "sentinel"))
				if err != nil || string(body) != "previous" {
					return errors.Join(err, errors.New("reader observed a partially published transaction"))
				}
			}
			return nil
		})
	}()
	readEarly := false
	select {
	case err := <-reader:
		readEarly = true
		t.Errorf("reader entered partial publication: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("injected publication failure was ignored")
	}
	if !readEarly {
		if err := <-reader; err != nil {
			t.Fatal(err)
		}
	}
	for _, root := range roots {
		assertSnapshot(t, root, "previous")
	}
}

func TestInterruptedStagingKeepsCommittedSnapshotReadable(t *testing.T) {
	const environment = "GOREGRAPH_STAGING_CRASH_ROOT"
	if root := os.Getenv(environment); root != "" {
		_ = Update(context.Background(), UpdateRequest{Root: root, Write: func(stage string) error {
			if err := os.WriteFile(filepath.Join(stage, "sentinel"), []byte("partial"), 0644); err != nil {
				return err
			}
			os.Exit(0)
			return nil
		}})
		os.Exit(1)
	}
	root := previousOutput(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestInterruptedStagingKeepsCommittedSnapshotReadable$")
	command.Env = append(os.Environ(), environment+"="+root)
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("staging child: %v %s", err, body)
	}
	assertSnapshot(t, root, "previous")
	if err := Update(context.Background(), replacement(root, "complete")); err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "complete")
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".goregraph-stage-") {
			t.Fatalf("abandoned stage remains: %s", entry.Name())
		}
	}
}

func TestCancellationWhileWaitingToPublishPreservesCommittedSnapshot(t *testing.T) {
	root := previousOutput(t)
	readerEntered, releaseReader := make(chan struct{}), make(chan struct{})
	finishReader := sync.OnceFunc(func() { close(releaseReader) })
	defer finishReader()
	readerDone := make(chan error, 1)
	go func() {
		readerDone <- WithRead(context.Background(), root, func(string) error { close(readerEntered); <-releaseReader; return nil })
	}()
	<-readerEntered
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	validated := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- UpdateManyValidated(ctx, []UpdateRequest{replacement(root, "new")}, func() error { close(validated); return nil })
	}()
	select {
	case <-validated:
	case err := <-done:
		t.Fatalf("preparation did not finish: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("preparation blocked on a committed reader")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled publisher returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled publisher did not release its locks")
	}
	assertSnapshot(t, root, "previous")
	finishReader()
	if err := <-readerDone; err != nil {
		t.Fatal(err)
	}
	if err := Update(context.Background(), replacement(root, "retry")); err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "retry")
}

func TestAbandonedStageCleanupDoesNotRemoveForeignOrBackupData(t *testing.T) {
	root := previousOutput(t)
	generation := strings.Repeat("a", 32)
	abandoned := stagePath(root, generation)
	retained := []string{backupPath(root, generation), stagePath(filepath.Join(filepath.Dir(root), "other"), generation), stagePath(root, "not-a-generation")}
	for _, path := range append(retained, abandoned) {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "sentinel"), []byte("retained"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Update(context.Background(), replacement(root, "new")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(abandoned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("abandoned scratch directory was not removed: %v", err)
	}
	for _, path := range retained {
		body, err := os.ReadFile(filepath.Join(path, "sentinel"))
		if err != nil || string(body) != "retained" {
			t.Fatalf("cleanup changed unrelated data at %s: %v", path, err)
		}
	}
}
