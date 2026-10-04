package outputstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateManySharedValidationRunsOnceAfterAllStagesBeforePromotion(t *testing.T) {
	roots := []string{previousOutput(t), previousOutput(t), previousOutput(t)}
	staged, validated, shared := 0, 0, 0
	var requests []UpdateRequest
	for _, root := range roots {
		requests = append(requests, UpdateRequest{Root: root, Write: func(stage string) error {
			staged++
			return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("new"), 0644)
		}, Validate: func(stage string) error {
			validated++
			body, err := os.ReadFile(filepath.Join(stage, "sentinel"))
			if err != nil || string(body) != "new" {
				return errors.Join(err, errors.New("invalid stage"))
			}
			return nil
		}})
	}
	err := UpdateManyValidated(context.Background(), requests, func() error {
		shared++
		if staged != len(roots) || validated != len(roots) {
			return errors.New("shared validation ran before all stages were ready")
		}
		for _, root := range roots {
			body, err := os.ReadFile(filepath.Join(root, "sentinel"))
			if err != nil || string(body) != "previous" {
				return errors.Join(err, errors.New("an output was promoted before shared validation"))
			}
		}
		return nil
	})
	if err != nil || shared != 1 {
		t.Fatalf("shared validations=%d, error=%v", shared, err)
	}
	for _, root := range roots {
		assertSnapshot(t, root, "new")
	}
}

func TestUpdateManyRejectsLateInputChangeWithoutPublishingAnyFamily(t *testing.T) {
	roots := []string{previousOutput(t), previousOutput(t)}
	source := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(source, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	staged := 0
	var requests []UpdateRequest
	for _, root := range roots {
		requests = append(requests, UpdateRequest{Root: root, Write: func(stage string) error {
			staged++
			if staged == len(roots) {
				if err := os.WriteFile(source, []byte("after"), 0644); err != nil {
					return err
				}
			}
			return os.WriteFile(filepath.Join(stage, "sentinel"), []byte("new"), 0644)
		}})
	}
	changed := errors.New("source inputs changed during staging")
	err := UpdateManyValidated(context.Background(), requests, func() error {
		body, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if string(body) != "before" {
			return changed
		}
		return nil
	})
	if !errors.Is(err, changed) || staged != len(roots) {
		t.Fatalf("stages=%d, error=%v", staged, err)
	}
	for _, root := range roots {
		assertSnapshot(t, root, "previous")
	}
}

func TestUpdateManySharedValidationCancellationPreservesAllOutputs(t *testing.T) {
	roots := []string{previousOutput(t), previousOutput(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := UpdateManyValidated(ctx, []UpdateRequest{replacement(roots[0], "new"), replacement(roots[1], "new")}, func() error {
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled shared validation was ignored: %v", err)
	}
	for _, root := range roots {
		assertSnapshot(t, root, "previous")
	}
}

func TestUpdateManyValidatedRequiresSharedValidator(t *testing.T) {
	root := previousOutput(t)
	if err := UpdateManyValidated(context.Background(), []UpdateRequest{replacement(root, "new")}, nil); err == nil {
		t.Fatal("missing shared validator was accepted")
	}
	assertSnapshot(t, root, "previous")
}
