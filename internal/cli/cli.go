package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/doctor"
	"github.com/gorecodecom/goregraph/internal/gitignore"
	"github.com/gorecodecom/goregraph/internal/mcp"
	"github.com/gorecodecom/goregraph/internal/query"
	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/version"
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
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdout, stderr)
	case "version":
		return runVersion(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printHelp(stderr)
		return 2
	}
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Fprint(stdout, "Usage: goregraph version\n\nPrints GoreGraph build metadata.\n")
		return 0
	}
	if len(args) > 0 {
		fmt.Fprint(stderr, "error: usage: goregraph version\n")
		return 2
	}
	_, _ = stdout.Write([]byte(version.Info(scan.SchemaVersion)))
	return 0
}

func runMCP(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Fprint(stdout, "Usage: goregraph mcp\n\nStarts the read-only MCP stdio server. It reads existing GoreGraph output and does not scan or write project files.\n")
		return 0
	}
	if len(args) > 0 {
		fmt.Fprintf(stderr, "error: usage: goregraph mcp\n")
		return 2
	}
	if err := mcp.Serve(os.Stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "error: mcp failed: %v\n", err)
		return 1
	}
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Fprint(stdout, "Usage: goregraph doctor <path>\n\nChecks config and generated GoreGraph output health without scanning.\n")
		return 0
	}
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	result, err := doctor.Run(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: doctor failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(result.String()))
	if result.Failures > 0 || result.Warnings > 0 {
		return 1
	}
	return 0
}

func runQuery(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Fprint(stdout, "Usage: goregraph query <path> <term-or-output>\n\nSearches an existing goregraph-out index. Known output aliases such as graph-full, callgraph, routes, flows, navigation, endpoint-flows, spring, endpoints, dependencies, analyzers, workspace, affected, and audit print that generated file directly.\n")
		return 0
	}
	if len(args) < 2 {
		fmt.Fprint(stderr, "error: usage: goregraph query <path> <term>\n")
		return 2
	}
	result, err := query.Search(args[0], strings.Join(args[1:], " "))
	if err != nil {
		fmt.Fprintf(stderr, "error: query failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(result))
	return 0
}

func runExplain(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Fprint(stdout, "Usage: goregraph explain <path> <file-or-symbol>\n\nExplains a file or symbol from an existing goregraph-out index.\n")
		return 0
	}
	if len(args) < 2 {
		fmt.Fprint(stderr, "error: usage: goregraph explain <path> <file-or-symbol>\n")
		return 2
	}
	result, err := query.Explain(args[0], strings.Join(args[1:], " "))
	if err != nil {
		fmt.Fprintf(stderr, "error: explain failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(result))
	return 0
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
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
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
  query <path>      Search the generated index or print an output alias
  explain <path>    Explain a file or symbol from the generated index
  doctor <path>     Check generated output health
  mcp               Start the read-only MCP stdio server
  version           Print build metadata
  help              Show this help

Examples:
  goregraph scan .
  goregraph scan . --no-update-gitignore
  goregraph update
  goregraph report .
  goregraph query . StartServer
  goregraph query . graph-full
  goregraph query . audit
  goregraph explain . src/main.go
  goregraph doctor .
  goregraph mcp
  goregraph version
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
