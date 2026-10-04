package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetExporterIsExplicitAndNeverOverwrites(t *testing.T) {
	for _, engine := range []string{"blender", "unity"} {
		root := t.TempDir()
		destination := filepath.Join(root, "exporter")
		var output, errors bytes.Buffer
		if code := Run([]string{"assets", "exporter", engine, "--output", destination}, &output, &errors); code != 0 {
			t.Fatalf("code=%d errors=%s", code, errors.String())
		}
		first, err := os.ReadFile(destination)
		if err != nil || len(first) == 0 {
			t.Fatal(err)
		}
		if code := Run([]string{"assets", "exporter", engine, "--output", destination}, &output, &errors); code != 1 {
			t.Fatal("exporter overwrote existing file")
		}
		after, _ := os.ReadFile(destination)
		if string(after) != string(first) {
			t.Fatal("existing exporter changed")
		}
		entries, _ := os.ReadDir(root)
		if len(entries) != 1 {
			t.Fatalf("unexpected outputs: %#v", entries)
		}
	}
}
func TestAssetExporterRejectsImplicitExecution(t *testing.T) {
	for _, args := range [][]string{{"assets", "export", "blender"}, {"assets", "exporter", "blender", "--run"}, {"assets", "exporter", "other", "--output", "ignored"}} {
		var output, errors bytes.Buffer
		if code := Run(args, &output, &errors); code != 2 {
			t.Fatal(code)
		}
		if strings.Contains(output.String(), "Exporter written") {
			t.Fatal(output.String())
		}
	}
}
