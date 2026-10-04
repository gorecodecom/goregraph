package outputstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSharedPreparationPreservesReadsAndRollsBackAnEarlierPromotion(t *testing.T) {
	parent := t.TempDir()
	roots := []string{filepath.Join(parent, "a-project"), filepath.Join(parent, "z-workspace")}
	for i, root := range roots {
		canonical, err := canonicalRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		roots[i] = canonical
	}
	requests := make([]UpdateRequest, len(roots))
	for i, root := range roots {
		requests[i] = UpdateRequest{Root: root, Write: func(stage string) error {
			return os.WriteFile(filepath.Join(stage, "evidence.json"), []byte("old-reference"), 0644)
		}}
	}
	if err := UpdateMany(context.Background(), requests); err != nil {
		t.Fatal(err)
	}
	oldGenerations := make(map[string]string)
	for _, root := range roots {
		body, err := os.ReadFile(filepath.Join(root, generationFile))
		if err != nil {
			t.Fatal(err)
		}
		oldGenerations[root] = string(body)
	}
	for i := range requests {
		requests[i].Write = func(string) error { return nil }
	}
	ready, release := make(chan struct{}), make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	defer finish()
	preparation := func(stages map[string]string) error {
		for _, root := range roots {
			if stages[root] == "" {
				return errors.New("output root has no private stage")
			}
			if err := os.WriteFile(filepath.Join(stages[root], "evidence.json"), []byte("new-reference"), 0644); err != nil {
				return err
			}
		}
		close(ready)
		<-release
		return nil
	}
	promoted := false
	ops := systemOperations()
	rename := ops.rename
	ops.rename = func(from, to string) error {
		if strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
			if to == roots[1] {
				return errors.New("injected later promotion failure")
			}
		}
		err := rename(from, to)
		if err == nil && to == roots[0] && strings.HasPrefix(filepath.Base(from), ".goregraph-stage-") {
			promoted = true
		}
		return err
	}
	done := make(chan error, 1)
	go func() { done <- updateManyPrepared(context.Background(), requests, ops, preparation, nil, nil) }()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("shared preparation failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("shared preparation did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := WithExistingReads(ctx, roots, func() error {
		for _, root := range roots {
			body, err := os.ReadFile(filepath.Join(root, "evidence.json"))
			if err != nil || string(body) != "old-reference" {
				return errors.New("mixed references were visible during preparation")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	finish()
	if err := <-done; err == nil || !promoted {
		t.Fatalf("failure did not occur after the earlier promotion: %v", err)
	}
	for _, root := range roots {
		body, err := os.ReadFile(filepath.Join(root, "evidence.json"))
		if err != nil || string(body) != "old-reference" {
			t.Fatalf("rollback did not restore %s: %s %v", root, body, err)
		}
		generation, err := os.ReadFile(filepath.Join(root, generationFile))
		if err != nil || string(generation) != oldGenerations[root] {
			t.Fatalf("rollback changed the committed generation for %s", root)
		}
	}
}
