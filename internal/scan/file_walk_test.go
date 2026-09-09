package scan

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestFileWalkSharesNestedIgnoreSelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "dist-offline/\n")
	writeFile(t, root, "apps/web/.gitignore", "generated/\n")
	writeFile(t, root, "apps/web/dist-offline/index.js", "compiled()")
	writeFile(t, root, "apps/web/generated/types.ts", "export type A = string")
	writeFile(t, root, "apps/web/main.ts", "export const count = 1")
	cfg := config.Defaults()
	var paths []string
	report, err := WalkProjectFiles(context.Background(), root, cfg, func(file WalkedFile) error { paths = append(paths, file.Path); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{"apps/web/.gitignore", "apps/web/main.ts"}) {
		t.Fatalf("paths = %v", paths)
	}
	if report.IgnoreDigest == "" {
		t.Fatal("missing ignore digest")
	}
	files, err := snapshotProjectFiles(root, cfg)
	if err != nil || len(files) != 2 || files[1].Path != paths[1] {
		t.Fatalf("snapshot = %v, %v", files, err)
	}
	writeFile(t, root, filepath.Join("apps", "web", ".gitignore"), "generated/\n# policy change\n")
	changed, err := WalkProjectFiles(context.Background(), root, cfg, func(WalkedFile) error { return nil })
	if err != nil || changed.IgnoreDigest == report.IgnoreDigest {
		t.Fatalf("ignore digest not changed: %v", err)
	}
}

func TestFileWalkCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := WalkProjectFiles(ctx, t.TempDir(), config.Defaults(), func(WalkedFile) error { t.Fatal("visited after cancellation"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
