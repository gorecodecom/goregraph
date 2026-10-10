package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplacementValidatesChangedContentAndRejectsRacingInstallation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goregraph")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path+".next", []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".next", path); err != nil {
			t.Fatal(err)
		}
	}
	write("version-a")
	identity, err := readExecutable(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	replacement := &Replacement{Path: path, active: identity}
	validations := 0
	validate := func(context.Context, string) error { validations++; return nil }
	if changed, err := replacement.Changed(context.Background(), validate); changed || err != nil || validations != 0 {
		t.Fatalf("unchanged executable was probed: changed=%v err=%v calls=%d", changed, err, validations)
	}
	write("version-a")
	if changed, err := replacement.Changed(context.Background(), validate); changed || err != nil {
		t.Fatalf("identical reinstall: changed=%v err=%v", changed, err)
	}
	write("version-b")
	rejected := errors.New("invalid candidate")
	if changed, err := replacement.Changed(context.Background(), func(context.Context, string) error { return rejected }); changed || !errors.Is(err, rejected) {
		t.Fatalf("invalid replacement: changed=%v err=%v", changed, err)
	}
	if changed, err := replacement.Changed(context.Background(), validate); !changed || err != nil {
		t.Fatalf("valid replacement: changed=%v err=%v", changed, err)
	}
	if changed, err := replacement.Changed(context.Background(), func(context.Context, string) error { write("version-c"); return nil }); changed || err == nil {
		t.Fatalf("racing replacement: changed=%v err=%v", changed, err)
	}
}

func TestReplacementDetectsImageReplacedBeforeStartupSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goregraph")
	if err := os.WriteFile(path, []byte("new-installed-image"), 0o755); err != nil {
		t.Fatal(err)
	}
	identity, err := readExecutable(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	replacement := &Replacement{Path: path, active: identity, startedReplaced: true}
	if changed, err := replacement.Changed(context.Background(), func(context.Context, string) error { return nil }); !changed || err != nil {
		t.Fatalf("startup mismatch: changed=%v err=%v", changed, err)
	}
}
