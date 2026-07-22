package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
)

func TestScanWritesDeterministicArtifactFreshness(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	first := readFreshnessTestIndex(t, root)
	if len(first.Artifacts) == 0 {
		t.Fatal("freshness index contains no artifacts")
	}
	for _, record := range first.Artifacts {
		if record.Artifact == "" || record.GeneratedAt == "" || record.GoreGraphVersion == "" || record.Schema != SchemaVersion || record.SourceFingerprint == "" {
			t.Fatalf("incomplete freshness record: %#v", record)
		}
		if record.Stale || record.Reason == "" {
			t.Fatalf("fresh scan marked stale or missing reason: %#v", record)
		}
	}
	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	second := readFreshnessTestIndex(t, root)
	if first.SourceFingerprint != second.SourceFingerprint {
		t.Fatalf("source fingerprint changed across identical scans: %q != %q", first.SourceFingerprint, second.SourceFingerprint)
	}
}

func TestWorkspaceWritesFreshnessIndex(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "weka")
	root := filepath.Join(workspace, "microservices", "users")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/users\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package users\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Index("freshness.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index ArtifactFreshnessIndex
	if err := json.Unmarshal(body, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Artifacts) == 0 || index.SourceFingerprint == "" {
		t.Fatalf("workspace freshness incomplete: %#v", index)
	}
}

func readFreshnessTestIndex(t *testing.T, root string) ArtifactFreshnessIndex {
	t.Helper()
	body, err := os.ReadFile(NewProjectOutputLayout(filepath.Join(root, config.Defaults().OutputDir)).Index("freshness.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index ArtifactFreshnessIndex
	if err := json.Unmarshal(body, &index); err != nil {
		t.Fatal(err)
	}
	return index
}
