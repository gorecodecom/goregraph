package scan

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gorecodecom/goregraph/internal/outputstore"
)

// BuildEvent reports completed work and the current expensive operation.
type BuildEvent struct {
	Phase     string        `json:"phase"`
	Project   string        `json:"project"`
	File      string        `json:"file,omitempty"`
	Outcome   string        `json:"outcome"`
	Completed int           `json:"completed"`
	Total     int           `json:"total,omitempty"`
	Elapsed   time.Duration `json:"elapsed_ns"`
}

// BuildOptions controls cooperative budgets and optional progress reporting.
type BuildOptions struct {
	FileTimeout    time.Duration
	ProjectTimeout time.Duration
	Observer       func(BuildEvent)
	stagedOutputs  map[string]string
	stagedProjects []WorkspaceProjectRecord
}

func (options BuildOptions) requireStagedOutput(root string) error {
	if len(options.stagedOutputs) > 0 && options.stagedOutputs[workspaceOutputKey(root)] == "" {
		return fmt.Errorf("workspace output root changed during preparation: %s", root)
	}
	return nil
}

func (options BuildOptions) outputRoot(root string) string {
	if len(options.stagedOutputs) == 0 {
		return root
	}
	if stage := options.stagedOutputs[workspaceOutputKey(root)]; stage != "" {
		return stage
	}
	return root
}

func (options BuildOptions) outputRoots(roots []string) []string {
	result := make([]string, len(roots))
	for i, root := range roots {
		result[i] = options.outputRoot(root)
	}
	return result
}

func (options BuildOptions) outputObserver(workspaceRoot, prefix string) func(outputstore.Event) {
	if options.Observer == nil {
		return nil
	}
	originalRoots := make(map[string]string, len(options.stagedOutputs))
	for original, private := range options.stagedOutputs {
		originalRoots[workspaceOutputKey(private)] = original
	}
	return func(event outputstore.Event) {
		root := event.Root
		if original := originalRoots[workspaceOutputKey(root)]; original != "" {
			root = original
		}
		options.Observer(BuildEvent{Phase: prefix + "-" + event.Phase, Project: workspaceRel(workspaceRoot, filepath.Dir(root)), File: event.File, Outcome: event.Outcome, Completed: event.Completed, Total: event.Total, Elapsed: event.Elapsed})
	}
}

// DefaultBuildOptions supplies bounded per-file analysis without a project deadline.
func DefaultBuildOptions() BuildOptions { return BuildOptions{FileTimeout: 5 * time.Second} }

func (options BuildOptions) validate() error {
	if options.FileTimeout < 0 || options.ProjectTimeout < 0 {
		return fmt.Errorf("analysis timeouts must not be negative")
	}
	return nil
}

func (options BuildOptions) emit(phase, root, file, outcome string, completed, total int, started time.Time) {
	if options.Observer != nil {
		options.Observer(BuildEvent{Phase: phase, Project: filepath.Base(root), File: file, Outcome: outcome, Completed: completed, Total: total, Elapsed: time.Since(started)})
	}
}

func budgetContext(ctx context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if budget > 0 {
		return context.WithTimeout(ctx, budget)
	}
	return ctx, func() {}
}

// AnalysisIssue explains an eligible source file whose extraction did not complete.
type AnalysisIssue struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}
