package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

type Result struct {
	Lines    []string
	Failures int
	Warnings int
}

func Run(root string) (Result, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return Result{}, err
	}
	out := filepath.Join(root, cfg.OutputDir)
	result := Result{Lines: []string{"GoreGraph Doctor", ""}}

	if info, err := os.Stat(out); err != nil || !info.IsDir() {
		result.fail("output", fmt.Sprintf("%s is missing", cfg.OutputDir))
		result.fix("goregraph scan " + root)
		return result, nil
	}
	result.ok("output", cfg.OutputDir+" exists")

	manifest, ok := checkManifest(out, &result)
	if !ok {
		result.fix("goregraph scan " + root)
		return result, nil
	}

	checkGeneratedFiles(out, manifest, &result)
	checkJSONFiles(out, &result)
	checkStaleFiles(root, manifest, &result)
	if result.Failures > 0 || result.Warnings > 0 {
		result.fix("goregraph scan " + root)
	}
	return result, nil
}

func checkManifest(out string, result *Result) (scan.Manifest, bool) {
	var manifest scan.Manifest
	if err := readJSON(filepath.Join(out, "manifest.json"), &manifest); err != nil {
		result.fail("manifest", err.Error())
		return scan.Manifest{}, false
	}
	if manifest.Tool != scan.ToolName {
		result.fail("manifest", fmt.Sprintf("tool is %q, want %q", manifest.Tool, scan.ToolName))
		return manifest, false
	}
	result.ok("manifest", "manifest.json valid")

	if manifest.Schema != scan.SchemaVersion {
		result.fail("schema", fmt.Sprintf("version %d unsupported, want %d", manifest.Schema, scan.SchemaVersion))
		return manifest, false
	}
	result.ok("schema", fmt.Sprintf("version %d supported", manifest.Schema))
	return manifest, true
}

func checkGeneratedFiles(out string, manifest scan.Manifest, result *Result) {
	expected := scan.GeneratedFiles
	if len(manifest.Generated) > 0 {
		expected = manifest.Generated
	}
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			result.fail("files", name+" missing")
			continue
		}
		result.ok("files", name+" present")
	}
}

func checkJSONFiles(out string, result *Result) {
	checks := []struct {
		name string
		dest any
	}{
		{"files.json", &[]scan.FileRecord{}},
		{"symbols.json", &[]scan.SymbolRecord{}},
		{"relations.json", &[]scan.RelationRecord{}},
		{"graph.json", &scan.Graph{}},
		{"symbols-full.json", &[]scan.RichSymbolRecord{}},
		{"relations-full.json", &[]scan.RichRelationRecord{}},
		{"graph-full.json", &scan.RichGraph{}},
		{"callgraph.json", &scan.CallGraphRecord{}},
		{"endpoint-flows.json", &[]scan.SpringEndpointFlowRecord{}},
		{"test-map.json", &[]scan.TestMapRecord{}},
		{"analyzers.json", &[]scan.AnalyzerRecord{}},
		{"spring.json", &scan.SpringIndex{}},
		{"audit.json", &scan.AuditRecord{}},
	}
	for _, check := range checks {
		if err := readJSON(filepath.Join(out, check.name), check.dest); err != nil {
			result.fail("json", check.name+" invalid: "+err.Error())
			continue
		}
		result.ok("json", check.name+" valid")
	}
}

func checkStaleFiles(root string, manifest scan.Manifest, result *Result) {
	out := filepath.Join(root, manifest.OutputDir)
	var files []scan.FileRecord
	if err := readJSON(filepath.Join(out, "files.json"), &files); err != nil {
		return
	}
	stale := 0
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			stale++
			continue
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != file.Hash {
			stale++
		}
	}
	if stale > 0 {
		result.warn("stale", fmt.Sprintf("%d indexed files changed or disappeared", stale))
		return
	}
	result.ok("stale", "indexed file hashes match")
}

func readJSON(path string, dest any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func (r *Result) ok(scope, message string) {
	r.Lines = append(r.Lines, fmt.Sprintf("OK   %s: %s", scope, message))
}

func (r *Result) warn(scope, message string) {
	r.Warnings++
	r.Lines = append(r.Lines, fmt.Sprintf("WARN %s: %s", scope, message))
}

func (r *Result) fail(scope, message string) {
	r.Failures++
	r.Lines = append(r.Lines, fmt.Sprintf("FAIL %s: %s", scope, message))
}

func (r *Result) fix(command string) {
	r.Lines = append(r.Lines, "", "Suggested fix:", "  "+strings.TrimSpace(command))
}

func (r Result) String() string {
	return strings.Join(r.Lines, "\n") + "\n"
}
