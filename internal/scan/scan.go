package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

type SymbolRecord struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type RelationRecord struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
}

type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
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
	var symbols []SymbolRecord
	var relations []RelationRecord
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
		record := FileRecord{
			Path:     rel,
			Language: detectLanguage(rel),
			Size:     info.Size(),
			Hash:     hex.EncodeToString(sum[:]),
			Kind:     detectKind(rel),
		}
		files = append(files, record)
		symbols = append(symbols, extractSymbols(record, string(body))...)
		relations = append(relations, extractRelations(record, string(body))...)
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].File != symbols[j].File {
			return symbols[i].File < symbols[j].File
		}
		if symbols[i].Line != symbols[j].Line {
			return symbols[i].Line < symbols[j].Line
		}
		if symbols[i].Kind != symbols[j].Kind {
			return symbols[i].Kind < symbols[j].Kind
		}
		return symbols[i].Name < symbols[j].Name
	})
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].From != relations[j].From {
			return relations[i].From < relations[j].From
		}
		if relations[i].To != relations[j].To {
			return relations[i].To < relations[j].To
		}
		if relations[i].Type != relations[j].Type {
			return relations[i].Type < relations[j].Type
		}
		return relations[i].Line < relations[j].Line
	})

	out := filepath.Join(resolved, cfg.OutputDir)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return Result{}, err
	}
	graph := buildGraph(files, symbols, relations)
	generated := []string{"manifest.json", "files.json", "symbols.json", "relations.json", "graph.json", "report.md"}
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
	if err := writeJSON(filepath.Join(out, "symbols.json"), symbols); err != nil {
		return Result{}, err
	}
	if err := writeJSON(filepath.Join(out, "relations.json"), relations); err != nil {
		return Result{}, err
	}
	if err := writeJSON(filepath.Join(out, "graph.json"), graph); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(out, "report.md"), []byte(renderReport(resolved, files, skipped)), 0o644); err != nil {
		return Result{}, err
	}

	return Result{ScannedFiles: len(files), SkippedFiles: skipped, OutputDir: out}, nil
}

var (
	goModuleRE      = regexp.MustCompile(`^\s*module\s+(.+)\s*$`)
	goFuncRE        = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	goTypeRE        = regexp.MustCompile(`^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(struct|interface|[A-Za-z_][A-Za-z0-9_]*)`)
	goImportOneRE   = regexp.MustCompile(`^\s*import\s+(?:[._A-Za-z0-9]+\s+)?"([^"]+)"`)
	goImportBlockRE = regexp.MustCompile(`^\s*(?:[._A-Za-z0-9]+\s+)?"([^"]+)"`)
	tsImportRE      = regexp.MustCompile(`^\s*import\s+(?:.+?\s+from\s+)?["']([^"']+)["']`)
	tsExportClassRE = regexp.MustCompile(`^\s*export\s+class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	tsClassRE       = regexp.MustCompile(`^\s*class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	tsFuncRE        = regexp.MustCompile(`^\s*(?:export\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	javaClassRE     = regexp.MustCompile(`\b(class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	javaImportRE    = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([^;]+);`)
	mdHeadingRE     = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
)

func extractSymbols(file FileRecord, body string) []SymbolRecord {
	lines := strings.Split(body, "\n")
	var symbols []SymbolRecord
	for index, line := range lines {
		lineNo := index + 1
		switch file.Language {
		case "go":
			if file.Path == "go.mod" {
				if match := goModuleRE.FindStringSubmatch(line); len(match) == 2 {
					symbols = append(symbols, SymbolRecord{Name: "module " + strings.TrimSpace(match[1]), Kind: "module", File: file.Path, Line: lineNo})
				}
			}
			if match := goFuncRE.FindStringSubmatch(line); len(match) == 2 {
				symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "function", File: file.Path, Line: lineNo})
			}
			if match := goTypeRE.FindStringSubmatch(line); len(match) == 3 {
				symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "type", File: file.Path, Line: lineNo})
			}
		case "typescript", "javascript":
			if match := tsExportClassRE.FindStringSubmatch(line); len(match) == 2 {
				symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "class", File: file.Path, Line: lineNo})
			} else if match := tsClassRE.FindStringSubmatch(line); len(match) == 2 {
				symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "class", File: file.Path, Line: lineNo})
			}
			if match := tsFuncRE.FindStringSubmatch(line); len(match) == 2 {
				symbols = append(symbols, SymbolRecord{Name: match[1], Kind: "function", File: file.Path, Line: lineNo})
			}
		case "java":
			if match := javaClassRE.FindStringSubmatch(line); len(match) == 3 {
				symbols = append(symbols, SymbolRecord{Name: match[2], Kind: match[1], File: file.Path, Line: lineNo})
			}
		case "markdown":
			if match := mdHeadingRE.FindStringSubmatch(line); len(match) == 3 {
				symbols = append(symbols, SymbolRecord{Name: strings.TrimSpace(match[2]), Kind: "heading", File: file.Path, Line: lineNo})
			}
		}
	}
	return symbols
}

func extractRelations(file FileRecord, body string) []RelationRecord {
	lines := strings.Split(body, "\n")
	var relations []RelationRecord
	inGoImportBlock := false
	for index, line := range lines {
		lineNo := index + 1
		switch file.Language {
		case "go":
			trimmed := strings.TrimSpace(line)
			if trimmed == "import (" {
				inGoImportBlock = true
				continue
			}
			if inGoImportBlock && trimmed == ")" {
				inGoImportBlock = false
				continue
			}
			if inGoImportBlock {
				if match := goImportBlockRE.FindStringSubmatch(line); len(match) == 2 {
					relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
				}
				continue
			}
			if match := goImportOneRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		case "typescript", "javascript":
			if match := tsImportRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: match[1], Type: "imports", Line: lineNo})
			}
		case "java":
			if match := javaImportRE.FindStringSubmatch(line); len(match) == 2 {
				relations = append(relations, RelationRecord{From: file.Path, To: strings.TrimSpace(match[1]), Type: "imports", Line: lineNo})
			}
		}
	}
	return relations
}

func buildGraph(files []FileRecord, symbols []SymbolRecord, relations []RelationRecord) Graph {
	nodes := make([]GraphNode, 0, len(files)+len(symbols))
	for _, file := range files {
		nodes = append(nodes, GraphNode{ID: "file:" + file.Path, Label: file.Path, Type: "file", File: file.Path})
	}
	for _, symbol := range symbols {
		nodes = append(nodes, GraphNode{
			ID:    "symbol:" + symbol.File + ":" + symbol.Kind + ":" + symbol.Name,
			Label: symbol.Name,
			Type:  symbol.Kind,
			File:  symbol.File,
			Line:  symbol.Line,
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	edges := make([]GraphEdge, 0, len(symbols)+len(relations))
	for _, symbol := range symbols {
		edges = append(edges, GraphEdge{From: "file:" + symbol.File, To: "symbol:" + symbol.File + ":" + symbol.Kind + ":" + symbol.Name, Type: "contains"})
	}
	for _, relation := range relations {
		edges = append(edges, GraphEdge{From: "file:" + relation.From, To: relation.To, Type: relation.Type})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Type < edges[j].Type
	})
	return Graph{Nodes: nodes, Edges: edges}
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
	switch strings.ToLower(filepath.Base(rel)) {
	case "go.mod":
		return "go"
	case "package.json":
		return "json"
	}
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
