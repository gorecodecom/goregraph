package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/watch"
)

// coveringWorkspaceWatcher reads existing state and registry metadata only.
func coveringWorkspaceWatcher(requested watch.Root) (watch.Root, watch.Status, bool, error) {
	for directory := filepath.Dir(requested.Path); ; directory = filepath.Dir(directory) {
		root, err := watch.Resolve(directory, true)
		if err != nil {
			return watch.Root{}, watch.Status{}, false, err
		}
		status, err := watch.GetStatus(root)
		if err != nil {
			return watch.Root{}, watch.Status{}, false, err
		}
		if status.Running && status.Workspace {
			covered, err := workspaceRegistryIncludesProject(root.Path, requested.Path)
			if err != nil {
				return watch.Root{}, watch.Status{}, false, fmt.Errorf("workspace watcher at %s is active, but project coverage cannot be verified: %w", root.Path, err)
			}
			if covered {
				return root, status, true, nil
			}
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	return watch.Root{}, watch.Status{}, false, nil
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
	if !watchStatusPathsEqual(registry.Root, workspaceRoot) {
		return false, fmt.Errorf("registry belongs to another workspace")
	}
	relative, err := filepath.Rel(workspaceRoot, projectRoot)
	if err != nil {
		return false, err
	}
	for _, project := range registry.Projects {
		if project.Indexed && watchStatusPathsEqual(filepath.FromSlash(project.Path), relative) {
			return true, nil
		}
	}
	return false, nil
}

func watchStatusPathsEqual(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
