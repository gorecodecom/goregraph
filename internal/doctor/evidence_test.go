package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDoctorValidatesEvidenceAndCoverageIntegrity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	result, err := Run(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Failures != 0 {
		t.Fatalf("valid output failed Doctor: %v", result.Lines)
	}
	if !containsLine(result.Lines, "evidence integrity valid") {
		t.Fatalf("Doctor did not report evidence validation: %v", result.Lines)
	}
}

func TestDoctorRejectsInvalidCoverageValue(t *testing.T) {
	root := scannedProject(t)
	path := filepath.Join(root, "goregraph-out", "coverage.json")
	var coverage scan.CoverageRecord
	readTestJSON(t, path, &coverage)
	coverage.Capabilities[0].Coverage = scan.Coverage("BROKEN")
	writeTestJSON(t, path, coverage)
	result, err := Run(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 || !containsLine(result.Lines, "invalid coverage") {
		t.Fatalf("invalid coverage passed Doctor: %v", result.Lines)
	}
}

func scannedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	return root
}

func readTestJSON(t *testing.T, path string, dest any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatal(err)
	}
}
func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}
