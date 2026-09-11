package scan

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/outputstore"
)

// WithOutputRead holds one stable boundary for built-in project/workspace readers.
// Callbacks must not acquire output locks again or mutate generated output.
func WithOutputRead(ctx context.Context, root string, read func() error) error {
	return withOutputRead(ctx, root, read, true)
}

// WithExistingOutputRead reads the applicable outputs without creating lock files.
// Missing locks require a separate authorized initialization operation.
func WithExistingOutputRead(ctx context.Context, root string, read func() error) error {
	return withOutputRead(ctx, root, read, false)
}

func withOutputRead(ctx context.Context, root string, read func() error, createMissing bool) error {
	if root == "" {
		root = "."
	}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	cfg, err := config.Load(resolved)
	if err != nil {
		return err
	}
	return withOutputReadConfigMode(ctx, resolved, cfg, read, createMissing)
}

// WithOutputReadConfig holds a stable boundary for the caller's effective
// output and workspace configuration.
func WithOutputReadConfig(ctx context.Context, root string, cfg config.Config, read func() error) error {
	if root == "" {
		root = "."
	}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	return withOutputReadConfig(ctx, resolved, cfg, read)
}

func withOutputReadConfig(ctx context.Context, resolved string, cfg config.Config, read func() error) error {
	return withOutputReadConfigMode(ctx, resolved, cfg, read, true)
}

func withOutputReadConfigMode(ctx context.Context, resolved string, cfg config.Config, read func() error, createMissing bool) error {
	roots := []string{filepath.Join(resolved, cfg.OutputDir), filepath.Join(resolved, ".goregraph-workspace")}
	workspaceRoot, ok, err := WorkspaceRoot(resolved, cfg)
	if err != nil {
		return err
	}
	if ok {
		roots = append(roots, filepath.Join(workspaceRoot, ".goregraph-workspace"))
		projects, err := discoverWorkspaceProjects(workspaceRoot, resolved, cfg.OutputDir)
		if err != nil {
			return err
		}
		for _, project := range projects {
			if info, err := os.Stat(project.AbsPath); err == nil && info.IsDir() {
				roots = append(roots, filepath.Join(project.AbsPath, project.OutputDir))
			}
		}
	}
	if !createMissing {
		return outputstore.WithExistingReads(ctx, roots, read)
	}
	return outputstore.WithReads(ctx, roots, read)
}
