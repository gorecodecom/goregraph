package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
)

func workspaceInputIdentity(root string, projects []WorkspaceProjectRecord, cfg config.Config, options BuildOptions) (BuildIdentity, error) {
	parts := make([]string, 0, len(projects)+1)
	for _, project := range projects {
		project.Status = "not_indexed"
		if project.Indexed {
			project.Status = "indexed"
		}
		record, _ := json.Marshal(project)
		part := string(record)
		if project.Indexed {
			manifest, err := readProjectOutputManifest(NewProjectOutputLayout(filepath.Join(project.AbsPath, project.OutputDir)).Manifest)
			if err != nil {
				return BuildIdentity{}, fmt.Errorf("read project identity %s: %w", project.Path, err)
			}
			identity, _ := json.Marshal(manifest.BuildIdentity)
			part += "\x00" + string(identity) + "\x00" + manifest.Agent.InputFingerprint + "\x00" + manifest.Dashboard.InputFingerprint
		}
		parts = append(parts, part)
	}
	body, err := os.ReadFile(filepath.Join(root, WorkspaceDashboardConfigName))
	if err != nil && !os.IsNotExist(err) {
		return BuildIdentity{}, err
	}
	parts = append(parts, "dashboard_config\x00"+string(body))
	return CurrentBuildIdentity(cfg, options, "workspace", semanticFingerprint(parts)), nil
}

// WorkspaceProjectionCurrent checks committed identities and revalidates live
// project inputs without regenerating outputs.
func WorkspaceProjectionCurrent(root string, cfg config.Config, target BuildTarget, options BuildOptions) (bool, error) {
	return WorkspaceProjectionCurrentContext(context.Background(), root, cfg, target, options)
}

// WorkspaceProjectionCurrentContext checks committed identities and verifies
// that project inputs have not changed since update planning.
func WorkspaceProjectionCurrentContext(ctx context.Context, root string, cfg config.Config, target BuildTarget, options BuildOptions) (bool, error) {
	var current bool
	err := WithOutputReadConfig(ctx, root, cfg, func() error {
		var err error
		current, err = workspaceProjectionCurrentUnlocked(ctx, root, cfg, target, options)
		return err
	})
	return current, err
}

func workspaceProjectionCurrentUnlocked(ctx context.Context, root string, cfg config.Config, target BuildTarget, options BuildOptions) (bool, error) {
	resolved, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	projects, err := discoverWorkspaceProjects(resolved, resolved, cfg.OutputDir)
	if err != nil {
		return false, err
	}
	layout := NewWorkspaceOutputLayout(filepath.Join(resolved, ".goregraph-workspace"))
	manifest, err := readProjectOutputManifest(layout.Manifest)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, nil
	}
	if manifest.Tool != ToolName || manifest.Schema != SchemaVersion || !validProjectionStatus(layout.Root, manifest.Index).Complete {
		return false, nil
	}
	identity, err := workspaceInputIdentity(resolved, projects, cfg, options)
	if err != nil {
		return false, err
	}
	if manifest.BuildIdentity != identity {
		return false, nil
	}
	if target.IncludesAgent() && (!currentAgentProjectionStatus(layout.Root, manifest.Agent).Complete || manifest.Agent.Stale) {
		return false, nil
	}
	if target.IncludesDashboard() && (!validProjectionStatus(layout.Root, manifest.Dashboard).Complete || manifest.Dashboard.Stale) {
		return false, nil
	}
	for _, project := range projects {
		projectContext, cancel := budgetContext(ctx, options.ProjectTimeout)
		item, err := workspaceProjectUpdateItemUnlocked(projectContext, WorkspaceProjectScanItemRecord{
			Project: project.Path,
			AbsPath: project.AbsPath,
		}, target, options)
		cancel()
		if err != nil {
			return false, err
		}
		if item.Action != WorkspaceUpdateActionSkip {
			return false, fmt.Errorf("source inputs changed after planning; rerun workspace update: %s (%s)", project.Path, item.Reason)
		}
	}
	return true, nil
}
