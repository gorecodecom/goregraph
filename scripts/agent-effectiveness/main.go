package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agentbench"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return commandError(stderr, "command is required")
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "summarize":
		return runSummarize(args[1:], stderr)
	default:
		return commandError(stderr, "unknown command %q", args[0])
	}
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "task manifest path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *manifestPath == "" {
		return commandError(stderr, "validate requires --manifest <path>")
	}
	manifest, err := agentbench.LoadEffectivenessManifest(*manifestPath)
	if err != nil {
		return commandError(stderr, "validate manifest: %v", err)
	}
	if err := validateManifestFiles(*manifestPath, manifest); err != nil {
		return commandError(stderr, "validate manifest: %v", err)
	}
	return writeJSON(stdout, struct {
		Schema int    `json:"schema"`
		Suite  string `json:"suite"`
		Tasks  int    `json:"tasks"`
	}{Schema: manifest.Schema, Suite: manifest.Suite, Tasks: len(manifest.Tasks)}, stderr)
}

func validateManifestFiles(manifestPath string, manifest agentbench.EffectivenessManifest) error {
	base := filepath.Dir(manifestPath)
	for _, task := range manifest.Tasks {
		paths := []string{task.Fixture}
		for _, language := range task.Languages {
			paths = append(paths, task.Prompts[language])
		}
		for _, relative := range paths {
			if filepath.IsAbs(relative) || strings.Contains(filepath.ToSlash(relative), "../") {
				return fmt.Errorf("task %q path %q must remain below the manifest directory", task.ID, relative)
			}
			info, err := os.Stat(filepath.Join(base, filepath.FromSlash(relative)))
			if err != nil || info.IsDir() {
				return fmt.Errorf("task %q file %q is missing", task.ID, relative)
			}
		}
		if _, err := agentbench.LoadEffectivenessFixture(filepath.Join(base, filepath.FromSlash(task.Fixture)), task); err != nil {
			return fmt.Errorf("task %q fixture: %w", task.ID, err)
		}
	}
	return nil
}

func runSummarize(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("summarize", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	attemptsPath := flags.String("attempts", "", "attempt JSONL path")
	outputPath := flags.String("output", "", "new report path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *attemptsPath == "" || *outputPath == "" {
		return commandError(stderr, "summarize requires --attempts <jsonl-path> --output <path>")
	}
	if _, err := os.Stat(*attemptsPath); err != nil {
		return commandError(stderr, "attempts: %v", err)
	}
	if _, err := os.Stat(*outputPath); err == nil {
		return commandError(stderr, "output already exists: %s", *outputPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return commandError(stderr, "output: %v", err)
	}
	attempts, err := agentbench.LoadEffectivenessAttemptsJSONL(*attemptsPath)
	if err != nil {
		return commandError(stderr, "load attempts: %v", err)
	}
	report, err := agentbench.SummarizeEffectiveness(attempts)
	if err != nil {
		return commandError(stderr, "summarize attempts: %v", err)
	}
	file, err := os.OpenFile(*outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return commandError(stderr, "create output: %v", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(report)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(*outputPath)
		return commandError(stderr, "write output: %v", errors.Join(writeErr, closeErr))
	}
	return 0
}

func writeJSON(destination io.Writer, value any, stderr io.Writer) int {
	encoder := json.NewEncoder(destination)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return commandError(stderr, "write output: %v", err)
	}
	return 0
}

func commandError(stderr io.Writer, format string, arguments ...any) int {
	_, _ = fmt.Fprintf(stderr, "agent-effectiveness: "+format+"\n", arguments...)
	return 2
}
