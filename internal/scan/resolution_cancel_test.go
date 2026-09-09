package scan

import (
	"context"
	"errors"
	"testing"
	"time"
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
	options.ProjectTimeout = time.Nanosecond
	_, err := workspaceProjectUpdateItemWithOptions(context.Background(), WorkspaceProjectScanItemRecord{AbsPath: root}, BuildTargetAgent, options)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("snapshot ignored project budget: %v", err)
	}
}
