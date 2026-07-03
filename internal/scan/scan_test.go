package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestRunWritesDeterministicFilesManifestAndReport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n")
	writeFile(t, root, "src/main.go", "package main\nfunc main() {}\n")
	writeFile(t, root, "dist/bundle.js", "ignored")

	result, err := Run(root, config.Defaults())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 2 {
		t.Fatalf("ScannedFiles = %d, want 2", result.ScannedFiles)
	}

	for _, name := range []string{"manifest.json", "files.json", "symbols.json", "relations.json", "graph.json", "report.md", "modules.md", "entrypoints.md", "test-map.md"} {
		if _, err := os.Stat(filepath.Join(root, "goregraph-out", name)); err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
	}

	var files []FileRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "files.json"), &files)
	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}
	if files[0].Path != "README.md" || files[1].Path != "src/main.go" {
		t.Fatalf("files sorted/filtered incorrectly: %#v", files)
	}
	for _, file := range files {
		if filepath.IsAbs(file.Path) {
			t.Fatalf("file path %q is absolute, want root-relative", file.Path)
		}
	}
}

func TestRunExtractsSymbolsRelationsAndGraph(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/demo\n")
	writeFile(t, root, "src/main.go", `package main

import "fmt"

type Server struct{}

func main() {
	fmt.Println("hello")
}
`)
	writeFile(t, root, "web/app.ts", `import { api } from "./api";

export class App {}

export function start() {
  api();
}
`)
	writeFile(t, root, "README.md", "# Demo\n\n## Usage\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "module example.test/demo", "module", "go.mod")
	assertHasSymbol(t, symbols, "Server", "type", "src/main.go")
	assertHasSymbol(t, symbols, "main", "package", "src/main.go")
	assertHasSymbol(t, symbols, "main", "function", "src/main.go")
	assertHasSymbol(t, symbols, "App", "class", "web/app.ts")
	assertHasSymbol(t, symbols, "start", "function", "web/app.ts")
	assertHasSymbol(t, symbols, "Demo", "heading", "README.md")

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "src/main.go", "fmt", "imports")
	assertHasRelation(t, relations, "web/app.ts", "./api", "imports")

	var graph Graph
	readJSON(t, filepath.Join(root, "goregraph-out", "graph.json"), &graph)
	if len(graph.Nodes) == 0 {
		t.Fatal("graph has no nodes")
	}
	if len(graph.Edges) == 0 {
		t.Fatal("graph has no edges")
	}
}

func TestRunUsesIncludePatternsFromConfig(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Include = []string{"src/**"}
	writeFile(t, root, "README.md", "# Demo\n")
	writeFile(t, root, "src/app.go", "package src\n")
	writeFile(t, root, "tools/tool.go", "package tools\n")

	result, err := Run(root, cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
	var files []FileRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "files.json"), &files)
	if len(files) != 1 || files[0].Path != "src/app.go" {
		t.Fatalf("files = %#v, want only src/app.go", files)
	}
}

func TestRunWritesProjectIntelligenceReports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/demo\n")
	writeFile(t, root, "cmd/api/main.go", "package main\nfunc main() {}\n")
	writeFile(t, root, "internal/service/service.go", "package service\nfunc StartServer() {}\n")
	writeFile(t, root, "internal/service/service_test.go", "package service\nfunc TestStartServer() {}\n")
	writeFile(t, root, "package.json", `{"scripts":{"dev":"vite --host 0.0.0.0","test":"vitest"}}`)

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	modules := readText(t, filepath.Join(root, "goregraph-out", "modules.md"))
	if !strings.Contains(modules, "`cmd/`") || !strings.Contains(modules, "`internal/`") {
		t.Fatalf("modules report missing top-level modules:\n%s", modules)
	}

	entrypoints := readText(t, filepath.Join(root, "goregraph-out", "entrypoints.md"))
	if !strings.Contains(entrypoints, "cmd/api/main.go") {
		t.Fatalf("entrypoints report missing Go main file:\n%s", entrypoints)
	}
	if !strings.Contains(entrypoints, "package.json") || !strings.Contains(entrypoints, "dev") {
		t.Fatalf("entrypoints report missing package scripts:\n%s", entrypoints)
	}

	testMap := readText(t, filepath.Join(root, "goregraph-out", "test-map.md"))
	if !strings.Contains(testMap, "internal/service/service_test.go") || !strings.Contains(testMap, "internal/service/service.go") {
		t.Fatalf("test map missing source/test association:\n%s", testMap)
	}
}

func TestRunDetectsTestSymbolsAndRelations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "internal/service/service.go", "package service\nfunc StartServer() {}\n")
	writeFile(t, root, "internal/service/service_test.go", "package service\nfunc TestStartServer() {}\n")

	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var symbols []SymbolRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "symbols.json"), &symbols)
	assertHasSymbol(t, symbols, "service", "package", "internal/service/service.go")
	assertHasSymbol(t, symbols, "TestStartServer", "test", "internal/service/service_test.go")

	var relations []RelationRecord
	readJSON(t, filepath.Join(root, "goregraph-out", "relations.json"), &relations)
	assertHasRelation(t, relations, "internal/service/service_test.go", "internal/service/service.go", "tests")
}

func TestRunUsesProjectGitignoreAsExclusions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "local/\n*.tmp\n")
	writeFile(t, root, "src/app.go", "package src\n")
	writeFile(t, root, "local/cache.go", "package local\n")
	writeFile(t, root, "scratch.tmp", "tmp")

	result, err := Run(root, config.Defaults())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
}

func assertHasSymbol(t *testing.T, symbols []SymbolRecord, name, kind, file string) {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind && symbol.File == file {
			return
		}
	}
	t.Fatalf("missing symbol name=%q kind=%q file=%q in %#v", name, kind, file, symbols)
}

func assertHasRelation(t *testing.T, relations []RelationRecord, from, to, kind string) {
	t.Helper()
	for _, relation := range relations {
		if relation.From == from && relation.To == to && relation.Type == kind {
			return
		}
	}
	t.Fatalf("missing relation from=%q to=%q type=%q in %#v", from, to, kind, relations)
}

func TestRunSkipsLargeBinaryAndSymlinkFiles(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.MaxFileSizeBytes = 4
	writeFile(t, root, "src/app.go", "pkg")
	writeFile(t, root, "big.txt", "12345")
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "src/app.go"), filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}

	result, err := Run(root, cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ScannedFiles != 1 {
		t.Fatalf("ScannedFiles = %d, want 1", result.ScannedFiles)
	}
	if result.SkippedFiles != 3 {
		t.Fatalf("SkippedFiles = %d, want 3", result.SkippedFiles)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string, dest any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
