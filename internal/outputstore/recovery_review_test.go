package outputstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupReviewOutput(t *testing.T, root string) string {
	t.Helper()
	if err := Update(context.Background(), UpdateRequest{Root: root, Write: func(stage string) error { return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("old"), 0644) }}); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}
func TestReviewPartialRollbackDeletion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	root = setupReviewOutput(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ops := systemOperations()
	rename := ops.rename
	remove := ops.removeAll
	ops.rename = func(from, to string) error {
		err := rename(from, to)
		if err == nil && to == root && strings.Contains(filepath.Base(from), ".goregraph-stage-") {
			cancel()
		}
		return err
	}
	ops.removeAll = func(path string) error {
		if path == root {
			if err := os.Remove(filepath.Join(path, generationFile)); err != nil {
				return err
			}
			return errors.New("sharing violation after deleting generation marker")
		}
		return remove(path)
	}
	err := updateMany(ctx, []UpdateRequest{{Root: root, Write: func(stage string) error { return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("new"), 0644) }}}, ops)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("post-promotion cancellation was not exercised: %v", err)
	}
	if err := Recover(context.Background(), root); err != nil {
		t.Fatalf("cannot recover intact backup after partial rollback deletion: %v", err)
	}
	value, err := os.ReadFile(filepath.Join(root, "sentinel"))
	if err != nil || string(value) != "old" {
		t.Fatalf("last good output not restored: %q %v", value, err)
	}
}

func TestRollbackRecoversAfterPartialDiscardedStageDeletion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	root = setupReviewOutput(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ops := systemOperations()
	rename, remove := ops.rename, ops.removeAll
	ops.rename = func(from, to string) error {
		err := rename(from, to)
		if err == nil && to == root && strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
			cancel()
		}
		return err
	}
	failed := false
	ops.removeAll = func(path string) error {
		if strings.HasPrefix(filepath.Base(path), ".goregraph-stage-") {
			if err := os.Remove(filepath.Join(path, generationFile)); err != nil {
				return err
			}
			failed = true
			return errors.New("sharing violation after partially deleting discarded generation")
		}
		return remove(path)
	}
	if err := updateMany(ctx, []UpdateRequest{replacement(root, "new")}, ops); !errors.Is(err, context.Canceled) || !failed {
		t.Fatalf("partial discard failure was not exercised: %v", err)
	}
	value, err := os.ReadFile(filepath.Join(root, "sentinel"))
	if err != nil || string(value) != "old" {
		t.Fatalf("old output unavailable during discard cleanup: %q %v", value, err)
	}
	if err := Recover(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if err := Recover(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "old")
}
func TestReviewPartialCommittedBackupDeletion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	root = setupReviewOutput(t, root)
	ops := systemOperations()
	remove := ops.removeAll
	ops.removeAll = func(path string) error {
		if strings.HasPrefix(filepath.Base(path), ".goregraph-backup-") {
			if err := os.Remove(filepath.Join(path, generationFile)); err != nil {
				return err
			}
			return errors.New("sharing violation after deleting backup generation marker")
		}
		return remove(path)
	}
	err := updateMany(context.Background(), []UpdateRequest{{Root: root, Write: func(stage string) error { return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("new"), 0644) }}}, ops)
	if !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("partial backup cleanup failure was not exercised: %v", err)
	}
	if err := Recover(context.Background(), root); err != nil {
		t.Fatalf("cannot finish committed cleanup: %v", err)
	}
	if err := Recover(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	assertSnapshot(t, root, "new")
}
func TestReviewPeerUpdateAfterPartialJournalCleanup(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		for _, retired := range []int{1, 2} {
			t.Run(fmt.Sprintf("rollback=%v/retired=%d", rollback, retired), func(t *testing.T) {
				roots := interruptedRetirement(t, rollback, retired)
				if err := Update(context.Background(), replacement(roots[retired], "later")); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if err := Recover(context.Background(), roots[0]); err != nil {
						t.Fatalf("peer progressed but coordinator cannot finish retirement: %v", err)
					}
				}
				for i, root := range roots {
					want := "new"
					if rollback {
						want = "old"
					}
					if i == retired {
						want = "later"
					}
					assertSnapshot(t, root, want)
				}
			})
		}
	}
}

func interruptedRetirement(t *testing.T, rollback bool, retired int) []string {
	t.Helper()
	parent := t.TempDir()
	roots := []string{filepath.Join(parent, "a"), filepath.Join(parent, "b"), filepath.Join(parent, "c")}
	var requests []UpdateRequest
	for i, root := range roots {
		roots[i] = setupReviewOutput(t, root)
		requests = append(requests, replacement(roots[i], "new"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ops := systemOperations()
	rename, remove := ops.rename, ops.remove
	ops.rename = func(from, to string) error {
		err := rename(from, to)
		if err == nil && rollback && to == roots[0] && strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
			cancel()
		}
		return err
	}
	injected := errors.New("interrupted after journal unlink")
	ops.remove = func(path string) error {
		if err := remove(path); err != nil {
			return err
		}
		if path == journalPath(roots[retired]) {
			return injected
		}
		return nil
	}
	if err := updateMany(ctx, requests, ops); !errors.Is(err, injected) {
		t.Fatalf("retirement interruption not exercised: %v", err)
	}
	decision, err := readJournal(journalPath(roots[0]))
	if err != nil || decision.State != "finalized" {
		t.Fatalf("peer released before durable finalization: state=%q err=%v", decision.State, err)
	}
	return roots
}

func TestFinalizedRecoveryPreservesNewerPeerJournal(t *testing.T) {
	roots := interruptedRetirement(t, false, 2)
	peer := roots[2]
	ops := systemOperations()
	realSync := ops.sync
	injected := errors.New("newer transaction commit flush failure")
	journalSyncs := 0
	ops.sync = func(file *os.File) error {
		if strings.HasPrefix(filepath.Base(file.Name()), ".goregraph-journal-tmp-") {
			journalSyncs++
			if journalSyncs == 3 {
				return injected
			}
		}
		return realSync(file)
	}
	if err := updateMany(context.Background(), []UpdateRequest{replacement(peer, "later")}, ops); !errors.Is(err, injected) {
		t.Fatalf("newer interruption not exercised: %v", err)
	}
	before, err := os.ReadFile(journalPath(peer))
	if err != nil {
		t.Fatal(err)
	}
	if err := Recover(context.Background(), roots[0]); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(journalPath(peer))
	if err != nil || string(before) != string(after) {
		t.Fatalf("newer transaction journal changed: %v", err)
	}
	if err := WithRead(context.Background(), peer, func(string) error { return nil }); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("newer pending publication became readable: %v", err)
	}
	if err := Recover(context.Background(), peer); err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		assertSnapshot(t, root, "new")
	}
}

func TestCleanupDecisionWriteFailuresRemainRecoverable(t *testing.T) {
	for _, state := range []string{"cleaning", "finalized"} {
		for _, afterRename := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/afterRename=%v", state, afterRename), func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "out")
				root = setupReviewOutput(t, root)
				ops := systemOperations()
				rename := ops.rename
				injected := errors.New("cleanup decision write interrupted")
				ops.rename = func(from, to string) error {
					if strings.HasPrefix(filepath.Base(from), ".goregraph-journal-tmp-") {
						body, err := os.ReadFile(from)
						if err != nil {
							return err
						}
						var decision journal
						if err := json.Unmarshal(body, &decision); err != nil {
							return err
						}
						if decision.State == state {
							if afterRename {
								if err := rename(from, to); err != nil {
									return err
								}
							}
							return injected
						}
					}
					return rename(from, to)
				}
				if err := updateMany(context.Background(), []UpdateRequest{replacement(root, "new")}, ops); !errors.Is(err, injected) {
					t.Fatalf("decision write failure not exercised: %v", err)
				}
				if err := WithRead(context.Background(), root, func(string) error { return nil }); !errors.Is(err, ErrRecoveryRequired) {
					t.Fatalf("pending cleanup became readable: %v", err)
				}
				if err := Recover(context.Background(), root); err != nil {
					t.Fatal(err)
				}
				assertSnapshot(t, root, "new")
			})
		}
	}
}

func TestCleanupProcessInterruptionRecovers(t *testing.T) {
	for _, mode := range []string{"rollback rename", "partial backup cleanup", "peer retirement"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			root, peer := filepath.Join(parent, "a"), filepath.Join(parent, "b")
			root = setupReviewOutput(t, root)
			peer = setupReviewOutput(t, peer)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestCleanupProcessInterruptionHelper$")
			command.Env = append(os.Environ(), "GOREGRAPH_CLEANUP_CRASH_ROOT="+root, "GOREGRAPH_CLEANUP_CRASH_MODE="+mode)
			output, err := command.CombinedOutput()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 24 {
				t.Fatalf("helper did not interrupt %s: %v %s", mode, err, output)
			}
			if mode == "peer retirement" {
				if err := Update(context.Background(), replacement(peer, "later")); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				if err := Recover(context.Background(), root); err != nil {
					t.Fatal(err)
				}
			}
			want := "new"
			if mode == "rollback rename" {
				want = "old"
			}
			assertSnapshot(t, root, want)
			if mode == "peer retirement" {
				assertSnapshot(t, peer, "later")
			}
		})
	}
}

func TestCleanupProcessInterruptionHelper(t *testing.T) {
	root := os.Getenv("GOREGRAPH_CLEANUP_CRASH_ROOT")
	if root == "" {
		return
	}
	mode := os.Getenv("GOREGRAPH_CLEANUP_CRASH_MODE")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ops := systemOperations()
	rename, removeAll, remove := ops.rename, ops.removeAll, ops.remove
	ops.rename = func(from, to string) error {
		err := rename(from, to)
		if err == nil && mode == "rollback rename" {
			if to == root && strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
				cancel()
			}
			if from == root && strings.HasPrefix(filepath.Base(to), ".goregraph-stage-") {
				os.Exit(24)
			}
		}
		return err
	}
	ops.removeAll = func(path string) error {
		if mode == "partial backup cleanup" && strings.HasPrefix(filepath.Base(path), ".goregraph-backup-") {
			if err := os.Remove(filepath.Join(path, generationFile)); err != nil {
				return err
			}
			os.Exit(24)
		}
		return removeAll(path)
	}
	peer := filepath.Join(filepath.Dir(root), "b")
	ops.remove = func(path string) error {
		err := remove(path)
		if err == nil && mode == "peer retirement" && path == journalPath(peer) {
			os.Exit(24)
		}
		return err
	}
	requests := []UpdateRequest{replacement(root, "new")}
	if mode == "peer retirement" {
		requests = append(requests, replacement(peer, "new"))
	}
	if err := updateMany(ctx, requests, ops); err != nil {
		t.Fatal(err)
	}
	t.Fatal("expected process interruption")
}
