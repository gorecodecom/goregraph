package scan

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/outputstore"
)

// cancelAfterChecks exercises cancellation during work without timer scheduling races.
type cancelAfterChecks struct {
	context.Context
	remaining int
}

func (ctx *cancelAfterChecks) Err() error {
	ctx.remaining--
	if ctx.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestScriptResolutionDiscardsFactsWhenCancelledDuringResolution(t *testing.T) {
	ctx := &cancelAfterChecks{Context: context.Background(), remaining: 8}
	facts := ProjectSymbolFacts{References: make([]RichRelationRecord, 100)}
	result, err := ResolveScriptSymbolFactsContext(ctx, nil, nil, nil, facts)
	if !errors.Is(err, context.Canceled) || len(result.References) != 0 {
		t.Fatalf("partial resolution leaked: %d references, error %v", len(result.References), err)
	}
}

func TestWorkspaceContractResolutionStopsDuringMatching(t *testing.T) {
	ctx := &cancelAfterChecks{Context: context.Background(), remaining: 8}
	projects := []workspaceIndexProject{{contracts: make([]APIContractRecord, 100)}}
	result, err := buildWorkspaceContractMatchesContext(ctx, projects, nil)
	if !errors.Is(err, context.Canceled) || len(result) != 0 {
		t.Fatalf("partial matches leaked: %d matches, error %v", len(result), err)
	}
}

func TestProjectSnapshotHonorsProjectBudget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package example\n")
	options := DefaultBuildOptions()
	options.ProjectTimeout = 10 * time.Millisecond
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the output lock so even a fast filesystem must wait for the budget.
	err = outputstore.Update(context.Background(), outputstore.UpdateRequest{
		Root: filepath.Join(root, cfg.OutputDir),
		Write: func(string) error {
			parent, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, snapshotErr := workspaceProjectUpdateItemWithOptions(parent, WorkspaceProjectScanItemRecord{AbsPath: root}, BuildTargetAgent, options)
			if !errors.Is(snapshotErr, context.DeadlineExceeded) || parent.Err() != nil {
				t.Errorf("snapshot ignored project budget: snapshot=%v parent=%v", snapshotErr, parent.Err())
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
