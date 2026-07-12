package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestServiceBoundsResultsAndContinuesDeterministically(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Run(root, config.Defaults()); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	first, err := service.Run(Request{Root: root, Task: "coverage", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Schema != 1 || first.Task != "coverage" || len(first.Items) != 2 || !first.Truncated || first.Continuation == "" {
		t.Fatalf("unexpected first result: %#v", first)
	}
	second, err := service.Run(Request{Root: root, Task: "coverage", Limit: 2, Continuation: first.Continuation})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) == 0 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("continuation did not advance: %#v", second)
	}
}

func TestServiceRejectsUnsafeBoundsAndReportsCoverage(t *testing.T) {
	service := Service{}
	if _, err := service.Run(Request{Root: t.TempDir(), Task: "coverage", Limit: 101}); err == nil {
		t.Fatal("limit above maximum accepted")
	}
	if _, err := service.Run(Request{Root: t.TempDir(), Task: "coverage", Detail: "verbose"}); err == nil {
		t.Fatal("invalid detail accepted")
	}
}
