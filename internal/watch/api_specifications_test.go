package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceWatcherFingerprintTracksDocumentationContracts(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "services/orders")
	docs := filepath.Join(workspace, "documentation")
	for _, directory := range []string{project, filepath.Join(docs, ".git")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module orders\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := Resolve(workspace, true)
	if err != nil {
		t.Fatal(err)
	}
	current := func() string {
		t.Helper()
		value, err := fingerprint(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := current()
	contract := filepath.Join(docs, "swagger.yaml")
	if err := os.WriteFile(contract, []byte("swagger: '2.0'\npaths: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	added := current()
	if added == before {
		t.Fatal("new contract in a documentation repository did not trigger the workspace fingerprint")
	}
	if err := os.WriteFile(contract, []byte("swagger: [broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := current()
	if invalid == added {
		t.Fatal("invalid contract update was invisible")
	}
	if err := os.WriteFile(filepath.Join(docs, "README.md"), []byte("Documentation only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if value := current(); value != invalid {
		t.Fatal("unrelated documentation changed the contract fingerprint")
	}
	if err := os.Remove(contract); err != nil {
		t.Fatal(err)
	}
	if value := current(); value != before {
		t.Fatal("contract removal did not restore the original workspace inputs")
	}
}
