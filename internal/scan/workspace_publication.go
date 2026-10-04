package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/outputstore"
)

// WithWorkspaceUpdatePublication keeps rebuilt projects and their workspace
// overlays private until every snapshot is ready for one recoverable publication.
func WithWorkspaceUpdatePublication(ctx context.Context, cfg config.Config, plan WorkspaceUpdatePlanRecord, options BuildOptions, prepare func(BuildOptions) error) error {
	projects, err := discoverWorkspaceProjects(plan.WorkspaceRoot, plan.WorkspaceRoot, cfg.OutputDir)
	if err != nil {
		return err
	}
	roots := map[string]bool{filepath.Join(plan.WorkspaceRoot, ".goregraph-workspace"): true}
	for _, project := range projects {
		if project.Indexed {
			roots[filepath.Join(project.AbsPath, project.OutputDir)] = true
		}
	}
	for _, item := range plan.Items {
		if item.Action != WorkspaceUpdateActionBuild {
			continue
		}
		projectConfig, err := config.Load(item.AbsPath)
		if err != nil {
			return err
		}
		roots[filepath.Join(item.AbsPath, projectConfig.OutputDir)] = true
	}
	var requests []outputstore.UpdateRequest
	for root := range roots {
		requests = append(requests, outputstore.UpdateRequest{Root: root, Observe: options.outputObserver(plan.WorkspaceRoot, "workspace"), Write: func(string) error {
			return nil
		}, Validate: func(stage string) error {
			return validateGeneratedOutput(stage, "")
		}})
	}
	started := time.Now()
	options.emit("stage-workspace", plan.WorkspaceRoot, "", "started", 0, len(roots), started)
	return outputstore.UpdateManyPrepared(ctx, requests, func(stages map[string]string) error {
		stagedOptions := options
		stagedOptions.stagedOutputs = make(map[string]string, len(stages))
		stagedOptions.stagedProjects = projects
		ordered := make([]string, 0, len(stages))
		for root := range stages {
			ordered = append(ordered, root)
		}
		sort.Strings(ordered)
		for _, root := range ordered {
			stage := stages[root]
			snapshot, err := os.MkdirTemp(stage, ".prepared-")
			if err != nil {
				return err
			}
			if err := movePreparedFiles(stage, snapshot, filepath.Base(snapshot)); err != nil {
				return err
			}
			stagedOptions.stagedOutputs[workspaceOutputKey(root)] = snapshot
		}
		if err := prepare(stagedOptions); err != nil {
			return err
		}
		for i, root := range ordered {
			if err := ctx.Err(); err != nil {
				return err
			}
			snapshot := stagedOptions.outputRoot(root)
			entries, err := os.ReadDir(stages[root])
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".goregraph-lock-") {
					if err := os.Remove(filepath.Join(stages[root], entry.Name())); err != nil {
						return err
					}
				}
			}
			if err := movePreparedFiles(snapshot, stages[root], ""); err != nil {
				return err
			}
			if err := os.Remove(snapshot); err != nil {
				return err
			}
			options.emit("prepare-publication", plan.WorkspaceRoot, root, "completed", i+1, len(ordered), started)
		}
		return nil
	}, nil)
}

func movePreparedFiles(from, to, except string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == except {
			continue
		}
		if err := os.Rename(filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func workspaceOutputKey(root string) string {
	root = filepath.Clean(root)
	if parent, err := filepath.EvalSymlinks(filepath.Dir(root)); err == nil {
		root = filepath.Join(parent, filepath.Base(root))
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(root)
	}
	return root
}
