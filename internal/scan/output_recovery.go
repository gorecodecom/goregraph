package scan

import (
	"context"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/outputstore"
)

// RecoverOutputPublications repairs only interrupted generated-output transactions.
// Read-only previews must not call this function.
func RecoverOutputPublications(ctx context.Context, root string, cfg config.Config) error {
	resolved, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	roots := []string{filepath.Join(resolved, cfg.OutputDir), filepath.Join(resolved, ".goregraph-workspace")}
	workspace, ok, err := WorkspaceRoot(resolved, cfg)
	if err != nil {
		return err
	}
	if ok {
		roots = append(roots, filepath.Join(workspace, ".goregraph-workspace"))
		projects, err := discoverWorkspaceProjects(workspace, resolved, cfg.OutputDir)
		if err != nil {
			return err
		}
		for _, project := range projects {
			roots = append(roots, filepath.Join(project.AbsPath, project.OutputDir))
		}
	}
	for _, output := range roots {
		if err := outputstore.Recover(ctx, output); err != nil {
			return err
		}
	}
	return nil
}
