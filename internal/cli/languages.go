package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/languageexport"
	"github.com/gorecodecom/goregraph/internal/scan"
)

// runLanguages writes explicitly requested templates or reads an input inventory.
// It never executes the exporter, a compiler, a build or an index mutation.
func runLanguages(args []string, stdout, stderr io.Writer) int {
	const help = "Usage: goregraph languages exporter csharp|swift --output <new-directory>\n       goregraph languages inputs csharp|swift <root>\n\nReads inputs or writes opt-in templates. Never runs compilers or builds.\n"
	if len(args) == 0 || len(args) == 1 && isHelp(args[0]) {
		fmt.Fprint(stdout, help)
		return 0
	}
	if len(args) == 3 && args[0] == "inputs" && (args[1] == "csharp" || args[1] == "swift") {
		root, err := filepath.Abs(args[2])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		cfg, err := config.Load(root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		files, _, err := scan.SnapshotProjectFiles(context.Background(), root, cfg)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		inputs, sources := []string{}, []string{}
		for _, file := range files {
			if scan.SemanticInputFile(file, args[1]) {
				inputs = append(inputs, file.Path)
			}
			if file.Language == args[1] && filepath.Base(file.Path) != "Package.swift" {
				sources = append(sources, file.Path)
			}
		}
		request := map[string]any{"root": root, "inputs": inputs, "modules": []any{map[string]any{"name": "Project", "files": sources}}}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(request); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if len(args) != 4 || args[0] != "exporter" || args[2] != "--output" {
		fmt.Fprint(stderr, help)
		return 2
	}
	files, err := languageexport.Files(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := os.Mkdir(args[3], 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(args[3], name), body, 0o644); err != nil {
			os.RemoveAll(args[3])
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	fmt.Fprintln(stdout, "Exporter written:", args[3])
	return 0
}
