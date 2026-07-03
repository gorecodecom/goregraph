package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

type Result struct {
	ScannedFiles int
	SkippedFiles int
	OutputDir    string
}

type FileRecord struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	Hash     string `json:"hash"`
	Kind     string `json:"kind"`
}

type Manifest struct {
	Tool        string   `json:"tool"`
	Schema      int      `json:"schema"`
	OutputDir   string   `json:"output_dir"`
	Files       int      `json:"files"`
	Skipped     int      `json:"skipped"`
	Generated   []string `json:"generated"`
	ProjectRoot string   `json:"project_root,omitempty"`
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

	var files []FileRecord
	skipped := 0
	err = filepath.WalkDir(resolved, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			skipped++
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

		sum := sha256.Sum256(body)
		files = append(files, FileRecord{
			Path:     rel,
			Language: detectLanguage(rel),
			Size:     info.Size(),
			Hash:     hex.EncodeToString(sum[:]),
			Kind:     detectKind(rel),
		})
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	out := filepath.Join(resolved, cfg.OutputDir)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return Result{}, err
	}
	generated := []string{"manifest.json", "files.json", "report.md"}
	manifest := Manifest{
		Tool:        "goregraph",
		Schema:      1,
		OutputDir:   cfg.OutputDir,
		Files:       len(files),
		Skipped:     skipped,
		Generated:   generated,
		ProjectRoot: filepath.Base(resolved),
	}
	if err := writeJSON(filepath.Join(out, "manifest.json"), manifest); err != nil {
		return Result{}, err
	}
	if err := writeJSON(filepath.Join(out, "files.json"), files); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(out, "report.md"), []byte(renderReport(resolved, files, skipped)), 0o644); err != nil {
		return Result{}, err
	}

	return Result{ScannedFiles: len(files), SkippedFiles: skipped, OutputDir: out}, nil
}

func shouldSkipPath(rel string, isDir bool, cfg config.Config, matcher gitignore.Matcher) bool {
	for _, pattern := range cfg.Exclude {
		p := strings.TrimSuffix(filepath.ToSlash(pattern), "/")
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return matcher.Ignored(rel, isDir)
}

func isBinary(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	if strings.Contains(string(body), "\x00") {
		return true
	}
	return !utf8.Valid(body)
}

func detectLanguage(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".sh", ".bash", ".zsh":
		return "shell"
	default:
		return "text"
	}
}

func detectKind(rel string) string {
	base := strings.ToLower(filepath.Base(rel))
	switch base {
	case "go.mod", "package.json", "pom.xml", "build.gradle", "settings.gradle":
		return "build"
	case "readme.md":
		return "documentation"
	default:
		return "source"
	}
}

func writeJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func renderReport(root string, files []FileRecord, skipped int) string {
	langs := map[string]int{}
	for _, file := range files {
		langs[file.Language]++
	}
	var langNames []string
	for lang := range langs {
		langNames = append(langNames, lang)
	}
	sort.Strings(langNames)

	var b strings.Builder
	b.WriteString("# GoreGraph Report\n\n")
	b.WriteString("## Project Summary\n\n")
	b.WriteString(fmt.Sprintf("- Project: %s\n", filepath.Base(root)))
	b.WriteString(fmt.Sprintf("- Files scanned: %d\n", len(files)))
	b.WriteString(fmt.Sprintf("- Files skipped: %d\n", skipped))
	b.WriteString("\n## Languages\n\n")
	for _, lang := range langNames {
		b.WriteString(fmt.Sprintf("- %s: %d\n", lang, langs[lang]))
	}
	if len(langNames) == 0 {
		b.WriteString("- none: 0\n")
	}
	b.WriteString("\n## Files\n\n")
	for _, file := range files {
		b.WriteString(fmt.Sprintf("- `%s` (%s, %d bytes)\n", file.Path, file.Language, file.Size))
	}
	return b.String()
}
