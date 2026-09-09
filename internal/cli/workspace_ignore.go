package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
	"github.com/gorecodecom/goregraph/internal/scan"
)

// migrateWorkspaceIgnores runs before snapshot planning, including unchanged projects.
func migrateWorkspaceIgnores(ctx context.Context, root string, cfg config.Config, stdout io.Writer) error {
	plan, err := scan.WorkspaceProjectScanPlan(root, cfg)
	if err != nil {
		return err
	}
	for _, project := range plan.Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		projectConfig, err := config.Load(project.AbsPath)
		if err != nil {
			return err
		}
		changed, err := gitignore.EnsureOutputIgnored(project.AbsPath, projectConfig.OutputDir)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(stdout, "- Updated .gitignore: %s\n", filepath.Join(project.AbsPath, ".gitignore"))
		}
	}
	changed, err := gitignore.EnsureWorkspaceIgnored(plan.WorkspaceRoot)
	if changed {
		fmt.Fprintf(stdout, "- Updated .gitignore: %s\n", filepath.Join(plan.WorkspaceRoot, ".gitignore"))
	}
	return err
}
