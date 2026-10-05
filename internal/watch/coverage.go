package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

// FindCoveringWorkspace returns an active workspace watcher only for an exactly registered project.
// It inspects existing state without creating files or changing watcher configuration.
func FindCoveringWorkspace(requested Root) (Root, Status, bool, error) {
	return findCoveringWorkspace(requested, false)
}

// FindResponsibleWorkspace also recognizes a supervised workspace recovering its worker.
// Coverage still requires an exactly registered project in that workspace.
func FindResponsibleWorkspace(requested Root) (Root, Status, bool, error) {
	return findCoveringWorkspace(requested, true)
}

func findCoveringWorkspace(requested Root, includeRecovering bool) (Root, Status, bool, error) {
	for directory := filepath.Dir(requested.Path); ; directory = filepath.Dir(directory) {
		root, err := Resolve(directory, true)
		if err != nil {
			return Root{}, Status{}, false, err
		}
		status, err := GetStatus(root)
		if err != nil {
			return Root{}, Status{}, false, err
		}
		if (status.Running || includeRecovering && status.Supervised) && status.Workspace {
			covered, err := workspaceRegistryIncludesProject(root.Path, requested.Path)
			if err != nil {
				return Root{}, Status{}, false, fmt.Errorf("workspace watcher at %s is active, but project coverage cannot be verified: %w", root.Path, err)
			}
			if covered {
				return root, status, true, nil
			}
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	return Root{}, Status{}, false, nil
}

func workspaceRegistryIncludesProject(workspaceRoot, projectRoot string) (bool, error) {
	body, err := os.ReadFile(filepath.Join(workspaceRoot, ".goregraph-workspace", "index", "registry.json"))
	if err != nil {
		return false, err
	}
	var registry scan.WorkspaceRegistryRecord
	if err := json.Unmarshal(body, &registry); err != nil {
		return false, err
	}
	if !coveragePathsEqual(registry.Root, workspaceRoot) {
		return false, fmt.Errorf("registry belongs to another workspace")
	}
	relative, err := filepath.Rel(workspaceRoot, projectRoot)
	if err != nil {
		return false, err
	}
	for _, project := range registry.Projects {
		if project.Indexed && coveragePathsEqual(filepath.FromSlash(project.Path), relative) {
			return true, nil
		}
	}
	return false, nil
}

func coveragePathsEqual(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
