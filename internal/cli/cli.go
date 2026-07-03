package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || isHelp(args[0]) {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "scan":
		return runScan(args[1:], stdout, stderr, false)
	case "update":
		return runScan(args[1:], stdout, stderr, true)
	case "report":
		return runReport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printHelp(stderr)
		return 2
	}
}

func runReport(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Fprint(stdout, "Usage: goregraph report <path>\n\nPrints goregraph-out/report.md for a project.\n")
		return 0
	}
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	cfg := config.Defaults()
	body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, "report.md"))
	if err != nil {
		fmt.Fprintf(stderr, "error: reading report failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write(body)
	return 0
}

func runScan(args []string, stdout, stderr io.Writer, update bool) int {
	if len(args) > 0 && isHelp(args[0]) {
		printScanHelp(stdout)
		return 0
	}

	root := "."
	cfg := config.Defaults()
	for _, arg := range args {
		switch arg {
		case "--no-update-gitignore":
			cfg.UpdateGitignore = false
		case "--help", "help":
			printScanHelp(stdout)
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}

	if update && root == "." {
		// Explicit for readability: update refreshes the current project in MVP.
		root = "."
	}

	loaded, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	loaded.UpdateGitignore = cfg.UpdateGitignore

	if loaded.UpdateGitignore {
		if _, err := gitignore.EnsureOutputIgnored(root, loaded.OutputDir); err != nil {
			fmt.Fprintf(stderr, "error: updating .gitignore failed: %v\n", err)
			return 1
		}
	}

	result, err := scan.Run(root, loaded)
	if err != nil {
		fmt.Fprintf(stderr, "error: scan failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Scanned %d files, skipped %d files.\n", result.ScannedFiles, result.SkippedFiles)
	fmt.Fprintf(stdout, "Output written to %s\n", loaded.OutputDir)
	return 0
}

func isHelp(arg string) bool {
	return arg == "help" || arg == "--help" || arg == "-h"
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `GoreGraph creates deterministic local code maps.

Usage: goregraph <command> [options]

Commands:
  scan <path>       Create or rebuild goregraph-out for a project
  update            Refresh the current project's goregraph-out
  report <path>     Print the generated Markdown report
  help              Show this help

Examples:
  goregraph scan .
  goregraph scan . --no-update-gitignore
  goregraph update
  goregraph report .
`)
}

func printScanHelp(w io.Writer) {
	fmt.Fprint(w, `Usage: goregraph scan <path> [options]

Creates deterministic GoreGraph output for a project.

Options:
  --no-update-gitignore   Do not add goregraph-out/ to the project .gitignore

Examples:
  goregraph scan .
  goregraph scan . --no-update-gitignore
`)
}
