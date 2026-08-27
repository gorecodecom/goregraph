package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

const (
	// WorkspaceUpdateActionBuild marks a project that needs a new index.
	WorkspaceUpdateActionBuild = "build"
	// WorkspaceUpdateActionSkip marks a project whose existing index is current.
	WorkspaceUpdateActionSkip = "skip"
)

// WorkspaceUpdatePlanRecord describes the projects inspected by a workspace update.
type WorkspaceUpdatePlanRecord struct {
	WorkspaceRoot string                      `json:"workspace_root"`
	Current       string                      `json:"current,omitempty"`
	Items         []WorkspaceUpdateItemRecord `json:"items"`
}

// WorkspaceUpdateItemRecord describes whether and why one project needs rebuilding.
type WorkspaceUpdateItemRecord struct {
	Project  string `json:"project"`
	AbsPath  string `json:"abs_path"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	Added    int    `json:"added"`
	Modified int    `json:"modified"`
	Deleted  int    `json:"deleted"`
}

// WorkspaceUpdatePlan compares current project content with existing project indexes.
func WorkspaceUpdatePlan(root string, cfg config.Config, target BuildTarget) (WorkspaceUpdatePlanRecord, error) {
	if err := target.Validate(); err != nil {
		return WorkspaceUpdatePlanRecord{}, err
	}
	projects, err := WorkspaceProjectScanPlan(root, cfg)
	if err != nil {
		return WorkspaceUpdatePlanRecord{}, err
	}
	plan := WorkspaceUpdatePlanRecord{
		WorkspaceRoot: projects.WorkspaceRoot,
		Current:       projects.Current,
		Items:         make([]WorkspaceUpdateItemRecord, 0, len(projects.Items)),
	}
	for _, project := range projects.Items {
		item, err := workspaceProjectUpdateItem(project, target)
		if err != nil {
			return WorkspaceUpdatePlanRecord{}, fmt.Errorf("inspect %s: %w", project.Project, err)
		}
		plan.Items = append(plan.Items, item)
	}
	return plan, nil
}

func workspaceProjectUpdateItem(project WorkspaceProjectScanItemRecord, target BuildTarget) (WorkspaceUpdateItemRecord, error) {
	item := WorkspaceUpdateItemRecord{
		Project: project.Project,
		AbsPath: project.AbsPath,
		Action:  WorkspaceUpdateActionBuild,
	}
	projectConfig, err := config.Load(project.AbsPath)
	if err != nil {
		return item, err
	}
	layout := NewProjectOutputLayout(filepath.Join(project.AbsPath, projectConfig.OutputDir))
	manifest, manifestErr := readProjectOutputManifest(layout.Manifest)
	switch {
	case manifestErr != nil || manifest.Tool != ToolName || !currentProjectionStatus(layout.Root, manifest.Index, prefixedGeneratedFiles("index", IndexGeneratedFiles)).Complete:
		item.Reason = "project index is missing or invalid"
		return item, nil
	case manifest.Schema != SchemaVersion:
		item.Reason = "output schema changed"
		return item, nil
	case target.IncludesAgent() && !currentAgentProjectionStatus(layout.Root, manifest.Agent).Complete:
		item.Reason = "agent projection is missing or incomplete"
		return item, nil
	case target.IncludesDashboard() && !currentProjectionStatus(layout.Root, manifest.Dashboard, prefixedGeneratedFiles("dashboard", DashboardGeneratedFiles)).Complete:
		item.Reason = "dashboard projection is missing or incomplete"
		return item, nil
	}
	previous, err := readProjectFileRecords(layout.Index("files.json"))
	if err != nil {
		item.Reason = "project file index is missing or invalid"
		return item, nil
	}
	current, err := snapshotProjectFiles(project.AbsPath, projectConfig)
	if err != nil {
		return item, err
	}
	item.Added, item.Modified, item.Deleted = compareProjectFiles(previous, current)
	if item.Added == 0 && item.Modified == 0 && item.Deleted == 0 {
		item.Action = WorkspaceUpdateActionSkip
		item.Reason = "source files unchanged"
		return item, nil
	}
	item.Reason = "source files changed"
	return item, nil
}

func readProjectOutputManifest(path string) (OutputManifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return OutputManifest{}, err
	}
	var manifest OutputManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return OutputManifest{}, err
	}
	return manifest, nil
}

func readProjectFileRecords(path string) ([]FileRecord, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var records []FileRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(record.Path)))
		decodedHash, hashErr := hex.DecodeString(record.Hash)
		_, duplicate := seen[record.Path]
		if record.Path == "" || filepath.IsAbs(filepath.FromSlash(record.Path)) || clean != record.Path ||
			strings.HasPrefix(clean, "../") || hashErr != nil || len(decodedHash) != sha256.Size || duplicate {
			return nil, fmt.Errorf("invalid file record %q", record.Path)
		}
		seen[record.Path] = struct{}{}
	}
	return records, nil
}

func snapshotProjectFiles(root string, cfg config.Config) ([]FileRecord, error) {
	resolved, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	matcher := gitignore.Matcher{}
	if cfg.UseGitignore {
		matcher = gitignore.Load(resolved)
	}
	var records []FileRecord
	err = filepath.WalkDir(resolved, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == resolved {
			return nil
		}
		rel, err := filepath.Rel(resolved, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if shouldSkipPath(rel, entry.IsDir(), cfg, matcher) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 && !cfg.FollowSymlinks {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || info.Size() > cfg.MaxFileSizeBytes {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil || isBinary(body) {
			return nil
		}
		records = append(records, fileRecord(rel, info.Size(), body))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func compareProjectFiles(previous, current []FileRecord) (added, modified, deleted int) {
	previousByPath := make(map[string]FileRecord, len(previous))
	for _, record := range previous {
		previousByPath[record.Path] = record
	}
	for _, record := range current {
		old, ok := previousByPath[record.Path]
		if !ok {
			added++
		} else if old.Hash != record.Hash {
			modified++
		}
		delete(previousByPath, record.Path)
	}
	return added, modified, len(previousByPath)
}
