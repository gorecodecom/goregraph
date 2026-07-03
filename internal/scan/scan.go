package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

const (
	ToolName      = "goregraph"
	SchemaVersion = 1
)

var GeneratedFiles = []string{
	"manifest.json",
	"files.json",
	"symbols.json",
	"relations.json",
	"graph.json",
	"report.md",
	"modules.md",
	"entrypoints.md",
	"test-map.md",
}

func Run(root string, cfg config.Config) (Result, error) {
	resolved, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Result{}, err
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("scan root %q is not a directory", root)
	}

	matcher := gitignore.Matcher{}
	if cfg.UseGitignore {
		matcher = gitignore.Load(resolved)
	}

	index, skipped, err := scanProject(resolved, cfg, matcher)
	if err != nil {
		return Result{}, err
	}
	sortIndex(&index)

	out := filepath.Join(resolved, cfg.OutputDir)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return Result{}, err
	}
	if err := writeOutputs(out, resolved, cfg, index, skipped); err != nil {
		return Result{}, err
	}

	return Result{ScannedFiles: len(index.Files), SkippedFiles: skipped, OutputDir: out}, nil
}

func scanProject(root string, cfg config.Config, matcher gitignore.Matcher) (Index, int, error) {
	var index Index
	skipped := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			skipped++
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			skipped++
			return nil
		}
		rel = filepath.ToSlash(rel)

		info, err := entry.Info()
		if err != nil {
			skipped++
			return nil
		}
		if shouldSkipPath(rel, entry.IsDir(), cfg, matcher) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			skipped++
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 && !cfg.FollowSymlinks {
			skipped++
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if info.Size() > cfg.MaxFileSizeBytes {
			skipped++
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			skipped++
			return nil
		}
		if isBinary(body) {
			skipped++
			return nil
		}

		record := fileRecord(rel, info.Size(), body)
		index.Files = append(index.Files, record)
		index.Symbols = append(index.Symbols, extractSymbols(record, string(body))...)
		index.Relations = append(index.Relations, extractRelations(record, string(body))...)
		return nil
	})
	return index, skipped, err
}

func fileRecord(rel string, size int64, body []byte) FileRecord {
	sum := sha256.Sum256(body)
	return FileRecord{
		Path:     rel,
		Language: detectLanguage(rel),
		Size:     size,
		Hash:     hex.EncodeToString(sum[:]),
		Kind:     detectKind(rel),
	}
}

func sortIndex(index *Index) {
	sort.Slice(index.Files, func(i, j int) bool { return index.Files[i].Path < index.Files[j].Path })
	sort.Slice(index.Symbols, func(i, j int) bool {
		if index.Symbols[i].File != index.Symbols[j].File {
			return index.Symbols[i].File < index.Symbols[j].File
		}
		if index.Symbols[i].Line != index.Symbols[j].Line {
			return index.Symbols[i].Line < index.Symbols[j].Line
		}
		if index.Symbols[i].Kind != index.Symbols[j].Kind {
			return index.Symbols[i].Kind < index.Symbols[j].Kind
		}
		return index.Symbols[i].Name < index.Symbols[j].Name
	})
	index.Relations = append(index.Relations, buildTestRelations(index.Files)...)
	resolveLocalImportRelations(index)
	sort.Slice(index.Relations, func(i, j int) bool {
		if index.Relations[i].From != index.Relations[j].From {
			return index.Relations[i].From < index.Relations[j].From
		}
		if index.Relations[i].To != index.Relations[j].To {
			return index.Relations[i].To < index.Relations[j].To
		}
		if index.Relations[i].Type != index.Relations[j].Type {
			return index.Relations[i].Type < index.Relations[j].Type
		}
		return index.Relations[i].Line < index.Relations[j].Line
	})
}

func writeOutputs(out, root string, cfg config.Config, index Index, skipped int) error {
	graph := buildGraph(index.Files, index.Symbols, index.Relations)
	manifest := Manifest{
		Tool:        ToolName,
		Schema:      SchemaVersion,
		OutputDir:   cfg.OutputDir,
		Files:       len(index.Files),
		Skipped:     skipped,
		Generated:   GeneratedFiles,
		ProjectRoot: filepath.Base(root),
	}
	writes := []struct {
		name  string
		value any
	}{
		{"manifest.json", manifest},
		{"files.json", index.Files},
		{"symbols.json", index.Symbols},
		{"relations.json", index.Relations},
		{"graph.json", graph},
	}
	for _, write := range writes {
		if err := writeJSON(filepath.Join(out, write.name), write.value); err != nil {
			return err
		}
	}

	reports := []struct {
		name string
		body string
	}{
		{"report.md", renderReport(root, index.Files, skipped)},
		{"modules.md", renderModulesReport(index.Files)},
		{"entrypoints.md", renderEntrypointsReport(index.Files, index.Symbols)},
		{"test-map.md", renderTestMapReport(index.Relations)},
	}
	for _, report := range reports {
		if err := os.WriteFile(filepath.Join(out, report.name), []byte(report.body), 0o644); err != nil {
			return err
		}
	}
	return nil
}
