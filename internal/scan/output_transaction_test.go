package scan

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestProjectPublicationFailureKeepsPreviousGeneration(t *testing.T) {
	root := writeBuildFixture(t)
	cfg := config.Defaults()
	cfg.Workspace = false
	if _, err := RunBuild(root, cfg, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
	layout := NewProjectOutputLayout(filepath.Join(root, cfg.OutputDir))
	before, err := os.ReadFile(layout.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceProjectionWriteHookForTest(func(scope, projection string) error {
		if scope == "project" && projection == "agent" {
			return errors.New("injected projection failure")
		}
		return nil
	})
	defer restore()
	if _, err := RunBuildWithOptions(context.Background(), root, cfg, BuildTargetAgent, DefaultBuildOptions()); err == nil {
		t.Fatal("expected failure")
	}
	after, err := os.ReadFile(layout.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed publication changed committed manifest")
	}
	if err := validateGeneratedOutput(layout.Root, BuildTargetAll); err != nil {
		t.Fatal(err)
	}
}
