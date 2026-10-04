package outputstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func movedProjectPreparation(t *testing.T) (journal, string) {
	t.Helper()
	base := t.TempDir()
	roots := []string{filepath.Join(base, ".goregraph-workspace"), filepath.Join(base, "service", "goregraph-out"), filepath.Join(base, "game", "CrownAndRunes", "goregraph-out")}
	for i, root := range roots {
		if err := os.MkdirAll(filepath.Dir(root), 0755); err != nil {
			t.Fatal(err)
		}
		roots[i] = setupReviewOutput(t, root)
	}
	sort.Strings(roots)
	transaction := journal{Version: 1, Generation: "0123456789abcdef0123456789abcdef", State: "prepared"}
	for _, root := range roots {
		generation, err := readGeneration(root)
		if err != nil {
			t.Fatal(err)
		}
		entry := journalEntry{Root: root, Stage: stagePath(root, transaction.Generation), Backup: backupPath(root, transaction.Generation), Existed: true, PreviousGeneration: generation}
		if err := os.MkdirAll(entry.Stage, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(entry.Stage, "sentinel"), []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}
		transaction.Entries = append(transaction.Entries, entry)
	}
	for _, entry := range transaction.Entries {
		if err := writeJournal(journalPath(entry.Root), transaction, systemOperations()); err != nil {
			t.Fatal(err)
		}
	}
	oldParent := filepath.Join(base, "game", "CrownAndRunes")
	moved := filepath.Join(base, "game", "Unity")
	if err := os.Rename(oldParent, moved); err != nil {
		t.Fatal(err)
	}
	return transaction, moved
}

func assertMovedOutputsPreserved(t *testing.T, transaction journal, moved string) {
	t.Helper()
	for _, entry := range transaction.Entries {
		root := entry.Root
		if filepath.Base(filepath.Dir(root)) == "CrownAndRunes" {
			if _, err := os.Lstat(filepath.Dir(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("recreated old project: %v", err)
			}
			root = filepath.Join(moved, "goregraph-out")
		}
		body, err := os.ReadFile(filepath.Join(root, "sentinel"))
		if err != nil || string(body) != "old" {
			t.Fatalf("changed original %s: %q %v", root, body, err)
		}
		generation, err := readGeneration(root)
		if err != nil || generation != entry.PreviousGeneration {
			t.Fatalf("changed generation %s: %q %v", root, generation, err)
		}
	}
}

func TestRecoverPreparedPublicationAfterProjectMove(t *testing.T) {
	transaction, moved := movedProjectPreparation(t)
	for i := 0; i < 2; i++ {
		if err := Recover(context.Background(), transaction.Entries[0].Root); err != nil {
			t.Fatal(err)
		}
		assertMovedOutputsPreserved(t, transaction, moved)
	}
	for _, entry := range transaction.Entries {
		if filepath.Base(filepath.Dir(entry.Root)) == "CrownAndRunes" {
			// Relocated metadata is retained; recovery does not guess its owner.
			if _, err := os.Stat(filepath.Join(moved, filepath.Base(entry.Stage))); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := WithRead(context.Background(), entry.Root, func(string) error { return nil }); err != nil {
			t.Fatalf("reader still blocked: %v", err)
		}
	}
}

func TestPreparedRecoveryAfterPartialStageCleanupAndProjectMove(t *testing.T) {
	transaction, moved := movedProjectPreparation(t)
	_, absent, err := recoveryRoots(transaction)
	if err != nil {
		t.Fatal(err)
	}
	ops := systemOperations()
	remove := ops.removeAll
	removed := 0
	ops.removeAll = func(path string) error {
		removed++
		if removed == 2 {
			return errors.New("injected failure after first stage cleanup")
		}
		return remove(path)
	}
	if err := cleanupPreparedExcept(transaction, ops, absent); err == nil || removed != 2 {
		t.Fatalf("failure not exercised: %v (%d)", err, removed)
	}
	assertMovedOutputsPreserved(t, transaction, moved)
	if err := Recover(context.Background(), transaction.Entries[0].Root); err != nil {
		t.Fatal(err)
	}
	assertMovedOutputsPreserved(t, transaction, moved)
}

func TestMovedProjectRecoveryRejectsPublicationThatStarted(t *testing.T) {
	for _, state := range []string{"publishing", "committed", "cleaning"} {
		t.Run(state, func(t *testing.T) {
			transaction, moved := movedProjectPreparation(t)
			transaction.State = state
			if err := writeJournal(journalPath(transaction.Entries[0].Root), transaction, systemOperations()); err != nil {
				t.Fatal(err)
			}
			if err := Recover(context.Background(), transaction.Entries[0].Root); err == nil {
				t.Fatal("discarded publication after promotion might have begun")
			}
			assertMovedOutputsPreserved(t, transaction, moved)
		})
	}
}

func TestMovedProjectRecoveryRejectsMissingSurvivingOutput(t *testing.T) {
	transaction, _ := movedProjectPreparation(t)
	if err := os.RemoveAll(transaction.Entries[0].Root); err != nil {
		t.Fatal(err)
	}
	if err := Recover(context.Background(), transaction.Entries[0].Root); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("missing output with existing parent must block: %v", err)
	}
	if _, err := os.Stat(transaction.Entries[0].Stage); err != nil {
		t.Fatalf("changed staging before validating all originals: %v", err)
	}
}

func TestPreparedRecoveryRejectsReappearedProjectParent(t *testing.T) {
	transaction, moved := movedProjectPreparation(t)
	_, absent, err := recoveryRoots(transaction)
	if err != nil {
		t.Fatal(err)
	}
	for root := range absent {
		if err := os.MkdirAll(filepath.Dir(root), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanupPreparedExcept(transaction, systemOperations(), absent); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("reappeared unlocked parent accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(moved, "goregraph-out", "sentinel")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(transaction.Entries[0].Stage); err != nil {
		t.Fatalf("removed stage on rejected recovery: %v", err)
	}
}
