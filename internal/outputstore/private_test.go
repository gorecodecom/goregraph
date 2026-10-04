package outputstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivatePreparationRejectsUnownedRootsBeforeWriting(t *testing.T) {
	for _, name := range []string{"live", ".prepared-alone", ".goregraph-stage-fixture/live"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			valid := filepath.Join(parent, ".goregraph-stage-fixture", ".prepared-safe")
			invalid := filepath.Join(parent, name)
			for _, root := range []string{valid, invalid} {
				if err := os.MkdirAll(root, 0755); err != nil {
					t.Fatal(err)
				}
			}
			written := false
			write := func(string) error { written = true; return nil }
			err := PreparePrivate(context.Background(), []UpdateRequest{{Root: valid, Write: write}, {Root: invalid, Write: write}}, nil)
			if err == nil || written {
				t.Fatalf("unsafe preparation was accepted: written=%t err=%v", written, err)
			}
		})
	}
}

func TestPrivatePreparationRetainsFinalValidationAndDurability(t *testing.T) {
	for _, failure := range []string{"none", "write", "validate", "sync"} {
		t.Run(failure, func(t *testing.T) {
			root := previousOutput(t)
			canonical, err := canonicalRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			root = canonical
			var snapshot string
			var privateEvents, outerEvents []Event
			syncs := 0
			ops := systemOperations()
			originalSync := ops.sync
			ops.sync = func(file *os.File) error {
				if filepath.Base(file.Name()) == "sentinel" && strings.Contains(file.Name(), ".goregraph-stage-") {
					syncs++
					if failure == "sync" {
						return errors.New("injected final sync failure")
					}
				}
				return originalSync(file)
			}
			request := UpdateRequest{
				Root:    root,
				Observe: func(event Event) { outerEvents = append(outerEvents, event) },
				Write: func(stage string) error {
					if err := os.Rename(filepath.Join(snapshot, "sentinel"), filepath.Join(stage, "sentinel")); err != nil {
						return err
					}
					return os.Remove(snapshot)
				},
				Validate: func(stage string) error {
					body, err := os.ReadFile(filepath.Join(stage, "sentinel"))
					if err != nil {
						return err
					}
					if string(body) != "new" {
						return errors.New("invalid final artifact")
					}
					if failure == "validate" {
						return errors.New("injected final validation failure")
					}
					return nil
				},
			}
			err = updateManyPrepared(context.Background(), []UpdateRequest{request}, ops, func(stages map[string]string) error {
				snapshot = filepath.Join(stages[root], ".prepared-fixture")
				if err := os.Mkdir(snapshot, 0755); err != nil {
					return err
				}
				return PreparePrivate(context.Background(), []UpdateRequest{{
					Root: snapshot, Observe: func(event Event) { privateEvents = append(privateEvents, event) },
					Write: func(stage string) error {
						if err := os.WriteFile(filepath.Join(stage, "sentinel"), []byte("new"), 0644); err != nil {
							return err
						}
						if failure == "write" {
							return errors.New("injected private write failure")
						}
						return nil
					},
				}}, nil)
			}, nil, nil)
			if failure == "none" {
				if err != nil {
					t.Fatal(err)
				}
				assertSnapshot(t, root, "new")
				if syncs != 1 {
					t.Fatalf("final artifact flushed %d times, want once", syncs)
				}
			} else {
				if err == nil {
					t.Fatal("injected failure was ignored")
				}
				assertSnapshot(t, root, "previous")
			}
			for _, event := range privateEvents {
				if event.Phase == "copy" || event.Phase == "sync" || event.Phase == "publish" {
					t.Fatalf("private preparation repeated publication work: %+v", event)
				}
			}
			if failure != "none" {
				for _, event := range outerEvents {
					if event.Phase == "publish" {
						t.Fatalf("invalid output reached publication: %+v", event)
					}
				}
			}
		})
	}
}
