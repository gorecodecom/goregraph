package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageExporterTemplatesAndReadOnlyInventory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Game.cs")
	os.WriteFile(file, []byte("class Game {}"), 0o644)
	var out, errs bytes.Buffer
	if code := Run([]string{"languages", "inputs", "csharp", root}, &out, &errs); code != 0 || !strings.Contains(out.String(), "Game.cs") {
		t.Fatal(code, out.String(), errs.String())
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("inventory mutated root", entries)
	}
	for _, language := range []string{"csharp", "swift"} {
		destination := filepath.Join(t.TempDir(), language)
		out.Reset()
		errs.Reset()
		if code := Run([]string{"languages", "exporter", language, "--output", destination}, &out, &errs); code != 0 {
			t.Fatal(code, errs.String())
		}
		if code := Run([]string{"languages", "exporter", language, "--output", destination}, &out, &errs); code != 1 {
			t.Fatal("existing exporter overwritten", code)
		}
	}
	if code := Run([]string{"languages", "exporter", "swift", "--run"}, &out, &errs); code != 2 {
		t.Fatal(code)
	}
}
