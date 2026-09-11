package outputstore

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFailedWritePreservesPreviousOutput(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "sentinel")
	if err := os.WriteFile(original, []byte("previous"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Update(context.Background(), UpdateRequest{Root: root,
		Write:    func(string) error { return errors.New("injected write failure") },
		Validate: func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("expected write failure")
	}
	got, readErr := os.ReadFile(original)
	if readErr != nil || string(got) != "previous" {
		t.Fatalf("lost previous output: %q %v", got, readErr)
	}
}

func TestFilesystemFailuresPreserveRecoverableSnapshot(t *testing.T) {
	for _, phase := range []string{"stage", "journal sync", "backup", "promotion", "commit", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			root := previousOutput(t)
			injected := errors.New("injected " + phase)
			ops := systemOperations()
			realRename, realSync, realRemove, realMkdir := ops.rename, ops.sync, ops.removeAll, ops.mkdir
			journalSyncs := 0
			ops.sync = func(file *os.File) error {
				if strings.HasPrefix(filepath.Base(file.Name()), ".goregraph-journal-tmp-") {
					journalSyncs++
					if phase == "journal sync" || phase == "commit" && journalSyncs == 3 {
						return injected
					}
				}
				return realSync(file)
			}
			ops.mkdir = func(path string, mode os.FileMode) error {
				if phase == "stage" && strings.HasPrefix(filepath.Base(path), ".goregraph-stage-") {
					return injected
				}
				return realMkdir(path, mode)
			}
			ops.rename = func(from, to string) error {
				if phase == "backup" && from == root {
					return injected
				}
				if phase == "promotion" && to == root && strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
					return injected
				}
				return realRename(from, to)
			}
			ops.removeAll = func(path string) error {
				if phase == "cleanup" && strings.HasPrefix(filepath.Base(path), ".goregraph-backup-") {
					return injected
				}
				return realRemove(path)
			}
			err := updateMany(context.Background(), []UpdateRequest{replacement(root, "new")}, ops)
			if !errors.Is(err, injected) {
				t.Fatalf("failure not returned: %v", err)
			}
			if phase == "commit" || phase == "cleanup" {
				called := false
				err := WithRead(context.Background(), root, func(string) error { called = true; return nil })
				if !errors.Is(err, ErrRecoveryRequired) || called {
					t.Fatalf("uncertain output served: %v %v", err, called)
				}
			}
			if err := Recover(context.Background(), root); err != nil {
				t.Fatal(err)
			}
			if err := Recover(context.Background(), root); err != nil {
				t.Fatal(err)
			}
			want := "previous"
			if phase == "cleanup" {
				want = "new"
			}
			assertSnapshot(t, root, want)
		})
	}
}

func TestSecondPromotionFailureRollsBackFirstSibling(t *testing.T) {
	parent := t.TempDir()
	first := filepath.Join(parent, "a")
	second := filepath.Join(parent, "b")
	for _, root := range []string{first, second} {
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "sentinel"), []byte("previous"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	first, err = canonicalRoot(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err = canonicalRoot(second)
	if err != nil {
		t.Fatal(err)
	}
	ops := systemOperations()
	rename := ops.rename
	promotions := 0
	injected := errors.New("second promotion failure")
	ops.rename = func(from, to string) error {
		if strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
			promotions++
			if to == second {
				return injected
			}
		}
		return rename(from, to)
	}
	err = updateMany(context.Background(), []UpdateRequest{replacement(second, "new b"), replacement(first, "new a")}, ops)
	if !errors.Is(err, injected) || promotions != 2 {
		t.Fatalf("did not fail after first promotion: %v, promotions=%d", err, promotions)
	}
	assertSnapshot(t, first, "previous")
	assertSnapshot(t, second, "previous")
}

func TestReaderRejectsPoisonedJournalWithoutRepair(t *testing.T) {
	root := previousOutput(t)
	path := journalPath(root)
	original := []byte("{invalid journal")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	err := WithRead(context.Background(), root, func(string) error { called = true; return nil })
	if !errors.Is(err, ErrRecoveryRequired) || called {
		t.Fatalf("poisoned journal was read: %v", err)
	}
	if err := Recover(context.Background(), root); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("invalid recovery: %v", err)
	}
	value, err := os.ReadFile(path)
	if err != nil || string(value) != string(original) {
		t.Fatalf("read repaired journal: %q %v", value, err)
	}
	value, err = os.ReadFile(filepath.Join(root, "sentinel"))
	if err != nil || string(value) != "previous" {
		t.Fatalf("poisoned journal damaged output: %q %v", value, err)
	}
}

func TestRecoveryRejectsJournalPathOutsideOutputParent(t *testing.T) {
	root := previousOutput(t)
	source := filepath.Join(t.TempDir(), "source")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config"), []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	transaction, err := prepareJournal([]UpdateRequest{replacement(root, "new")})
	if err != nil {
		t.Fatal(err)
	}
	transaction.Entries[0].Stage = source
	if err := persistPrepared(transaction, systemOperations()); err != nil {
		t.Fatal(err)
	}
	if err := Recover(context.Background(), root); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("unsafe journal accepted: %v", err)
	}
	value, err := os.ReadFile(filepath.Join(source, "config"))
	if err != nil || string(value) != "source" {
		t.Fatalf("source removed: %q %v", value, err)
	}
}

func TestWriterCancellationWhileReaderHoldsSnapshot(t *testing.T) {
	root := previousOutput(t)
	readerEntered := make(chan struct{})
	releaseReader := make(chan struct{})
	readerDone := make(chan error, 1)
	go func() {
		readerDone <- WithRead(context.Background(), root, func(string) error { close(readerEntered); <-releaseReader; return nil })
	}()
	<-readerEntered
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := Update(ctx, replacement(root, "new"))
	close(releaseReader)
	if readErr := <-readerDone; readErr != nil {
		t.Fatal(readErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked writer ignored cancellation: %v", err)
	}
	assertSnapshot(t, root, "previous")
}

func TestWithReadsLocksExistingParentsWithoutCreatingOptionalOutputs(t *testing.T) {
	root := previousOutput(t)
	optional := filepath.Join(filepath.Dir(root), "optional")
	called := false
	err := WithReads(context.Background(), []string{root, optional, root}, func() error {
		called = true
		_, err := os.Stat(optional)
		if !errors.Is(err, os.ErrNotExist) {
			return errors.New("optional output was created")
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("optional outputs not readable: %v", err)
	}
	called = false
	err = WithReads(context.Background(), []string{filepath.Join(optional, "nested")}, func() error { called = true; return nil })
	if !errors.Is(err, os.ErrNotExist) || called {
		t.Fatalf("missing parent read unprotected: %v %v", err, called)
	}
}

func TestWithReadsRejectsAnyPendingSibling(t *testing.T) {
	first, second := previousOutput(t), previousOutput(t)
	if err := os.WriteFile(journalPath(second), []byte("interrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	err := WithReads(context.Background(), []string{second, first}, func() error { called = true; return nil })
	if !errors.Is(err, ErrRecoveryRequired) || called {
		t.Fatalf("pending sibling served: %v", err)
	}
}

func TestReaderSeesCompletePublishedGeneration(t *testing.T) {
	root := previousOutput(t)
	staged := make(chan struct{})
	publish := make(chan struct{})
	writerDone := make(chan error, 1)
	readerDone := make(chan error, 1)
	go func() {
		writerDone <- Update(context.Background(), UpdateRequest{Root: root, Write: func(stage string) error {
			if err := os.WriteFile(filepath.Join(stage, "sentinel"), []byte("new"), 0644); err != nil {
				return err
			}
			close(staged)
			<-publish
			return os.WriteFile(filepath.Join(stage, "second"), []byte("new"), 0644)
		}})
	}()
	<-staged
	go func() {
		readerDone <- WithRead(context.Background(), root, func(committed string) error {
			for _, name := range []string{"sentinel", "second"} {
				value, err := os.ReadFile(filepath.Join(committed, name))
				if err != nil {
					return err
				}
				if string(value) != "new" {
					return errors.New("mixed generation")
				}
			}
			return nil
		})
	}()
	close(publish)
	if err := <-writerDone; err != nil {
		t.Fatal(err)
	}
	if err := <-readerDone; err != nil {
		t.Fatal(err)
	}
}

func TestProcessInterruptionReleasesLockAndRecoversBackup(t *testing.T) {
	root := previousOutput(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestOutputStoreProcessInterruptionHelper$")
	command.Env = append(os.Environ(), "GOREGRAPH_OUTPUTSTORE_CRASH_ROOT="+root)
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 23 {
		t.Fatalf("helper did not interrupt promotion: %v %s", err, output)
	}
	if err := WithRead(context.Background(), root, func(string) error { return nil }); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("interrupted root served: %v", err)
	}
	if err := Recover(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "previous")
	if err := Update(context.Background(), replacement(root, "next")); err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "next")
}

func TestWindowsOpenFileSharingFailureKeepsPreviousOutput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows sharing semantics")
	}
	root := previousOutput(t)
	file, err := os.Open(filepath.Join(root, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	err = Update(context.Background(), replacement(root, "new"))
	closeErr := file.Close()
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if err == nil {
		t.Fatal("directory promotion ignored an open file without delete sharing")
	}
	if err := Recover(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "previous")
}

func TestCancellationAfterStagedWritePreservesPreviousOutput(t *testing.T) {
	root := previousOutput(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Update(ctx, UpdateRequest{Root: root, Write: func(stage string) error {
		if err := os.WriteFile(filepath.Join(stage, "sentinel"), []byte("new"), 0644); err != nil {
			return err
		}
		cancel()
		return nil
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("staged cancellation ignored: %v", err)
	}
	assertSnapshot(t, root, "previous")
}

func TestSourceRepositoryCannotBecomeOutputRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Update(context.Background(), replacement(root, "generated"))
	if err == nil {
		t.Fatal("source repository accepted as output root")
	}
	value, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(value) != "source" {
		t.Fatalf("source changed: %q %v", value, err)
	}
}

func TestOutputStoreProcessInterruptionHelper(t *testing.T) {
	root := os.Getenv("GOREGRAPH_OUTPUTSTORE_CRASH_ROOT")
	if root == "" {
		return
	}
	ops := systemOperations()
	rename := ops.rename
	ops.rename = func(from, to string) error {
		err := rename(from, to)
		if err == nil && from == root {
			os.Exit(23)
		}
		return err
	}
	if err := updateMany(context.Background(), []UpdateRequest{replacement(root, "new")}, ops); err != nil {
		t.Fatal(err)
	}
	t.Fatal("expected process interruption")
}

func previousOutput(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "out")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sentinel"), []byte("previous"), 0644); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func replacement(root, value string) UpdateRequest {
	return UpdateRequest{Root: root, Write: func(stage string) error { return os.WriteFile(filepath.Join(stage, "sentinel"), []byte(value), 0644) }}
}

func assertSnapshot(t *testing.T, root, want string) {
	t.Helper()
	if err := WithRead(context.Background(), root, func(committed string) error {
		value, err := os.ReadFile(filepath.Join(committed, "sentinel"))
		if err != nil {
			return err
		}
		if string(value) != want {
			return errors.New("unexpected snapshot: " + string(value))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdatePreservesUnrequestedProjection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "retained.json"), []byte("retained"), 0644); err != nil {
		t.Fatal(err)
	}
	originalTime := time.Date(2023, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(root, "retained.json"), originalTime, originalTime); err != nil {
		t.Fatal(err)
	}
	err := Update(context.Background(), UpdateRequest{Root: root,
		Write: func(stage string) error {
			return os.WriteFile(filepath.Join(stage, "changed.json"), []byte("new"), 0644)
		},
		Validate: func(stage string) error {
			value, err := os.ReadFile(filepath.Join(stage, "retained.json"))
			if err != nil {
				return err
			}
			if string(value) != "retained" {
				return errors.New("missing staged old projection")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	retained, err := os.Stat(filepath.Join(root, "retained.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !retained.ModTime().Equal(originalTime) {
		t.Fatalf("untargeted projection timestamp changed: %v", retained.ModTime())
	}
	err = WithRead(context.Background(), root, func(committed string) error {
		for name, want := range map[string]string{"retained.json": "retained", "changed.json": "new"} {
			value, err := os.ReadFile(filepath.Join(committed, name))
			if err != nil {
				return err
			}
			if string(value) != want {
				return errors.New("mixed projection")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRejectedValidationPreservesPreviousOutput(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sentinel"), []byte("previous"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Update(context.Background(), UpdateRequest{Root: root,
		Write: func(stage string) error {
			return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("invalid"), 0644)
		},
		Validate: func(string) error { return errors.New("invalid generation") },
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	value, err := os.ReadFile(filepath.Join(root, "sentinel"))
	if err != nil || string(value) != "previous" {
		t.Fatalf("previous output lost: %q %v", value, err)
	}
}

func TestUpdateRejectsNestedRootsBeforeWriting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	called := false
	write := func(string) error { called = true; return nil }
	err := UpdateMany(context.Background(), []UpdateRequest{{Root: root, Write: write}, {Root: filepath.Join(root, "nested"), Write: write}})
	if err == nil || called {
		t.Fatalf("unsafe roots accepted: err=%v called=%v", err, called)
	}
}
