package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	case "workspace":
		return runWorkspace(args[1:], stdout, stderr)
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

func runWorkspace(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		printWorkspaceHelp(stdout)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, "error: usage: goregraph workspace <status|scan-missing> [path] [options]\n")
		return 2
	}
	switch args[0] {
	case "status":
		return runWorkspaceStatus(args[1:], stdout, stderr)
	case "scan-missing":
		return runWorkspaceScanMissing(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown workspace command: %s\n", args[0])
		return 2
	}
}

func runWorkspaceStatus(args []string, stdout, stderr io.Writer) int {
	cfg := config.Defaults()
	root := "."
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			cfg.WorkspaceRoot = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace status [path] [--workspace <path>]\n\nShows discovered workspace projects and loaded GoreGraph indexes without scanning.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}

	body, err := scan.WorkspaceStatus(root, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace status failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(body))
	return 0
}

func runWorkspaceScanMissing(args []string, stdout, stderr io.Writer) int {
	root := "."
	top := 5
	execute := false
	overrides := config.Defaults()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--top":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --top requires a positive number\n")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprint(stderr, "error: --top requires a positive number\n")
				return 2
			}
			top = value
		case "--execute":
			execute = true
		case "--no-update-gitignore":
			overrides.UpdateGitignore = false
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			overrides.WorkspaceRoot = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace scan-missing [path] [--top N] [--execute] [--workspace <path>] [--no-update-gitignore]\n\nShows a prioritized missing-service scan plan. Add --execute to run the scans.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}

	loaded, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	loaded.UpdateGitignore = overrides.UpdateGitignore
	loaded.Workspace = true
	loaded.WorkspaceRoot = overrides.WorkspaceRoot

	plan, err := scan.WorkspaceMissingScanPlan(root, loaded, top)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace scan-missing failed: %v\n", err)
		return 1
	}
	if !execute {
		printWorkspaceMissingScanPlan(stdout, plan, true)
		return 0
	}
	printWorkspaceMissingScanPlan(stdout, plan, false)
	scanned := 0
	for _, item := range plan.Items {
		projectCfg, err := config.Load(item.AbsPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: loading %s failed: %v\n", item.Project, err)
			return 1
		}
		projectCfg.UpdateGitignore = loaded.UpdateGitignore
		projectCfg.Workspace = true
		projectCfg.WorkspaceRoot = plan.WorkspaceRoot
		if projectCfg.UpdateGitignore {
			if _, err := gitignore.EnsureOutputIgnored(item.AbsPath, projectCfg.OutputDir); err != nil {
				fmt.Fprintf(stderr, "error: updating %s .gitignore failed: %v\n", item.Project, err)
				return 1
			}
		}
		result, err := scan.Run(item.AbsPath, projectCfg)
		if err != nil {
			fmt.Fprintf(stderr, "error: scanning %s failed: %v\n", item.Project, err)
			return 1
		}
		scanned++
		fmt.Fprintf(stdout, "- Scanned `%s` (%d files, skipped %d)\n", item.Project, result.ScannedFiles, result.SkippedFiles)
	}
	if loaded.UpdateGitignore {
		if _, err := gitignore.EnsureWorkspaceIgnored(plan.WorkspaceRoot); err != nil {
			fmt.Fprintf(stderr, "error: updating workspace .gitignore failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "\nScanned %d missing workspace project(s).\n", scanned)
	return 0
}

func printWorkspaceMissingScanPlan(w io.Writer, plan scan.WorkspaceMissingScanPlanRecord, dryRun bool) {
	fmt.Fprint(w, "# GoreGraph Workspace Missing Scan Plan\n\n")
	fmt.Fprintf(w, "- Workspace root: `%s`\n", plan.WorkspaceRoot)
	if plan.Current != "" {
		fmt.Fprintf(w, "- Current project: `%s`\n", plan.Current)
	}
	fmt.Fprintf(w, "- Top: %d\n", plan.Top)
	fmt.Fprintf(w, "- Dry run: %t\n\n", dryRun)
	if len(plan.Items) == 0 {
		fmt.Fprint(w, "- none\n")
		return
	}
	for index, item := range plan.Items {
		fmt.Fprintf(w, "%d. `%s` - %d contracts - project `%s` - %s\n", index+1, item.Service, item.Contracts, item.Project, emptyCLI(item.Status))
	}
	if dryRun {
		fmt.Fprint(w, "\nRun with `goregraph workspace scan-missing <path> --top N --execute` to scan these projects.\n")
	}
}

func emptyCLI(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func runQuery(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Fprint(stdout, "Usage: goregraph query <path> <term-or-output>\n\nSearches an existing goregraph-out index. Known output aliases such as graph-full, callgraph, routes, flows, api-contracts, frontend-usage, package-graph, maven-graph, navigation, endpoint-flows, spring, endpoints, dependencies, analyzers, workspace, workspace-context, workspace-contracts, workspace-features, workspace-next-actions, frontend-consumers, affected, and audit print that generated file directly.\n")
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
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--no-update-gitignore":
			cfg.UpdateGitignore = false
		case "--no-workspace":
			cfg.Workspace = false
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			cfg.WorkspaceRoot = args[i]
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
	loaded.Workspace = cfg.Workspace
	loaded.WorkspaceRoot = cfg.WorkspaceRoot

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
	if loaded.UpdateGitignore && loaded.Workspace {
		workspaceRoot, ok, err := scan.WorkspaceRoot(root, loaded)
		if err != nil {
			fmt.Fprintf(stderr, "error: detecting workspace root failed: %v\n", err)
			return 1
		}
		if ok {
			if _, err := gitignore.EnsureWorkspaceIgnored(workspaceRoot); err != nil {
				fmt.Fprintf(stderr, "error: updating workspace .gitignore failed: %v\n", err)
				return 1
			}
		}
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
  workspace         Show workspace status or scan prioritized missing services
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
  goregraph query . api-contracts
  goregraph query . package-graph
  goregraph query . maven-graph
  goregraph query . audit
  goregraph explain . src/main.go
  goregraph doctor .
  goregraph workspace status .
  goregraph workspace scan-missing . --top 5
  goregraph mcp
  goregraph version
`)
}

func printWorkspaceHelp(w io.Writer) {
	fmt.Fprint(w, `Usage: goregraph workspace <command> [options]

Commands:
  status [path]        Show workspace projects and loaded indexes without scanning
  scan-missing [path]  Show prioritized missing service scans; add --execute to scan

Examples:
  goregraph workspace status .
  goregraph workspace scan-missing .
  goregraph workspace scan-missing . --top 5 --execute
`)
}

func printScanHelp(w io.Writer) {
	fmt.Fprint(w, `Usage: goregraph scan <path> [options]

Creates deterministic GoreGraph output for a project.

Options:
  --no-update-gitignore   Do not add generated GoreGraph output to .gitignore files
  --no-workspace          Do not discover or refresh workspace overlays
  --workspace <path>      Use an explicit workspace root

Examples:
  goregraph scan .
  goregraph scan . --no-update-gitignore
  goregraph scan . --workspace ..
`)
}
