package scan

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func newAuditRecord(root string, cfgOutputDir string, started time.Time, finished time.Time, filesRead int, skipped int, generated []string) AuditRecord {
	return AuditRecord{
		Tool:             ToolName,
		Version:          "dev",
		Command:          "scan",
		ProjectRoot:      filepath.Base(root),
		OutputDir:        cfgOutputDir,
		StartedAt:        started.UTC().Format(time.RFC3339),
		FinishedAt:       finished.UTC().Format(time.RFC3339),
		FilesRead:        filesRead,
		FilesSkipped:     skipped,
		Generated:        append([]string(nil), generated...),
		NetworkUsed:      false,
		ExternalCommands: false,
	}
}

func readGitMetadata(root string) *GitMetadata {
	gitDir := filepath.Join(root, ".git")
	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	headPath := filepath.Join(gitDir, "HEAD")
	body, err := os.ReadFile(headPath)
	if err != nil {
		return nil
	}
	head := strings.TrimSpace(string(body))
	meta := &GitMetadata{}
	if strings.HasPrefix(head, "ref: ") {
		ref := strings.TrimSpace(strings.TrimPrefix(head, "ref: "))
		meta.Branch = strings.TrimPrefix(ref, "refs/heads/")
		if commit, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(ref))); err == nil {
			meta.Commit = strings.TrimSpace(string(commit))
		}
	} else {
		meta.Commit = head
	}
	return meta
}
