package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

// WalkedFile describes an eligible file relative to the project root.
type WalkedFile struct {
	Path string
	Size int64
}

// FileWalkReport records selection outcomes independently of extraction.
type FileWalkReport struct {
	Visited            int
	SkippedDirectories int
	Skipped            map[string]int
	IgnoreDigest       string
}

// WalkProjectFiles enumerates eligible files with directory-scoped ignore rules.
// An incomplete inventory is an error, never evidence that a file was deleted.
func WalkProjectFiles(ctx context.Context, root string, cfg config.Config, visit func(WalkedFile) error) (FileWalkReport, error) {
	return walkProjectFiles(ctx, root, cfg, gitignore.Matcher{}, visit)
}

func walkProjectFiles(ctx context.Context, root string, cfg config.Config, initial gitignore.Matcher, visit func(WalkedFile) error) (FileWalkReport, error) {
	report := FileWalkReport{Skipped: map[string]int{}}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return report, err
	}
	if !info.IsDir() {
		return report, fmt.Errorf("scan root %q is not a directory", root)
	}
	output := filepath.Clean(filepath.Join(resolved, cfg.OutputDir))
	digest := sha256.New()
	var walk func(string, string, gitignore.Matcher) error
	walk = func(directory, relative string, matcher gitignore.Matcher) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if cfg.UseGitignore {
			body, err := os.ReadFile(filepath.Join(directory, ".gitignore"))
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("read ignore rules %s: %w", relative, err)
			}
			if err == nil {
				matcher = matcher.WithFile(relative, string(body))
				fmt.Fprintf(digest, "%s\x00%d\x00", relative, len(body))
				_, _ = digest.Write(body)
			}
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("enumerate %s: %w", relative, err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			rel := filepath.ToSlash(filepath.Join(relative, entry.Name()))
			path := filepath.Join(directory, entry.Name())
			report.Visited++
			if filepath.Clean(path) == output || generatedScratchName(entry.Name()) {
				report.Skipped["generated_output"]++
				if entry.IsDir() {
					report.SkippedDirectories++
				}
				continue
			}
			if shouldSkipPath(rel, entry.IsDir(), cfg, gitignore.Matcher{}) {
				report.Skipped["excluded"]++
				if entry.IsDir() {
					report.SkippedDirectories++
				}
				continue
			}
			if matcher.Ignored(rel, entry.IsDir()) {
				report.Skipped["gitignore"]++
				if entry.IsDir() {
					report.SkippedDirectories++
				}
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if !cfg.FollowSymlinks {
					report.Skipped["symlink"]++
					continue
				}
				// Following directory links was never part of WalkDir traversal.
				target, err := filepath.EvalSymlinks(path)
				if err != nil {
					return fmt.Errorf("resolve %s: %w", rel, err)
				}
				relTarget, err := filepath.Rel(resolved, target)
				if err != nil || relTarget == ".." || strings.HasPrefix(relTarget, ".."+string(filepath.Separator)) {
					return fmt.Errorf("symlink %s escapes project root", rel)
				}
				info, err := os.Stat(target)
				if err != nil {
					return fmt.Errorf("stat %s: %w", rel, err)
				}
				if !info.Mode().IsRegular() {
					report.Skipped["symlink"]++
					continue
				}
				if info.Size() > cfg.MaxFileSizeBytes {
					report.Skipped["size_limit"]++
					continue
				}
				if err := visit(WalkedFile{Path: rel, Size: info.Size()}); err != nil {
					return err
				}
				continue
			}
			if entry.IsDir() {
				if err := walk(path, rel, matcher); err != nil {
					return err
				}
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("stat %s: %w", rel, err)
			}
			if !info.Mode().IsRegular() {
				report.Skipped["non_regular"]++
				continue
			}
			if info.Size() > cfg.MaxFileSizeBytes {
				report.Skipped["size_limit"]++
				continue
			}
			if err := visit(WalkedFile{Path: rel, Size: info.Size()}); err != nil {
				return err
			}
		}
		return nil
	}
	err = walk(resolved, "", initial)
	report.IgnoreDigest = hex.EncodeToString(digest.Sum(nil))
	return report, err
}

func generatedScratchName(name string) bool {
	return strings.HasPrefix(name, ".goregraph-stage-") || strings.HasPrefix(name, ".goregraph-backup-") || strings.HasPrefix(name, ".goregraph-journal-") || strings.HasPrefix(name, ".goregraph-lock-")
}

// SnapshotProjectFiles returns the same content inventory used by incremental updates.
func SnapshotProjectFiles(ctx context.Context, root string, cfg config.Config) ([]FileRecord, FileWalkReport, error) {
	var records []FileRecord
	report, err := WalkProjectFiles(ctx, root, cfg, func(file WalkedFile) error {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.Path, err)
		}
		if int64(len(body)) != file.Size {
			return fmt.Errorf("source changed during snapshot: %s", file.Path)
		}
		if !isBinary(body) {
			records = append(records, fileRecord(file.Path, file.Size, body))
		}
		return nil
	})
	return records, report, err
}
