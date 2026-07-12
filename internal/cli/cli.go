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
		fmt.Fprint(stderr, "error: usage: goregraph workspace <status|scan-missing|scan-all|refresh|clean|diff|dashboard|explain|path|impact> [path] [options]\n")
		return 2
	}
	switch args[0] {
	case "status":
		return runWorkspaceStatus(args[1:], stdout, stderr)
	case "scan-missing":
		return runWorkspaceScanMissing(args[1:], stdout, stderr)
	case "scan-all":
		return runWorkspaceScanAll(args[1:], stdout, stderr)
	case "refresh":
		return runWorkspaceRefresh(args[1:], stdout, stderr)
	case "clean":
		return runWorkspaceClean(args[1:], stdout, stderr)
	case "diff":
		return runWorkspaceDiff(args[1:], stdout, stderr)
	case "dashboard":
		return runWorkspaceDashboard(args[1:], stdout, stderr)
	case "explain":
		return runWorkspaceExplain(args[1:], stdout, stderr)
	case "path":
		return runWorkspacePath(args[1:], stdout, stderr)
	case "impact":
		return runWorkspaceImpact(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown workspace command: %s\n", args[0])
		return 2
	}
}

func runWorkspaceRefresh(args []string, stdout, stderr io.Writer) int {
	cfg := config.Defaults()
	cfg.Workspace = true
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
			fmt.Fprint(stdout, "Usage: goregraph workspace refresh [path] [--workspace <path>]\n\nRefreshes workspace overlays from existing project GoreGraph outputs without scanning source files.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}
	registry, err := scan.ReconcileWorkspace(root, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace refresh failed: %v\n", err)
		return 1
	}
	if registry == nil {
		fmt.Fprint(stderr, "error: workspace root not found or no projects discovered\n")
		return 1
	}
	fmt.Fprintf(stdout, "Refreshed workspace overlay: %s\n", filepath.Join(registry.Root, ".goregraph-workspace"))
	fmt.Fprintf(stdout, "- workspace-map.html\n")
	fmt.Fprintf(stdout, "- workspace-service-map.json\n")
	fmt.Fprintf(stdout, "- workspace-endpoint-traces.json\n")
	return 0
}

func runWorkspaceDashboard(args []string, stdout, stderr io.Writer) int {
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
			fmt.Fprint(stdout, "Usage: goregraph workspace dashboard [path] [--workspace <path>]\n\nPrints the generated workspace dashboard HTML path.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}
	workspaceRoot, ok, err := scan.WorkspaceRoot(root, cfg)
	if err != nil || !ok {
		fmt.Fprintf(stderr, "error: workspace root not found: %v\n", err)
		return 1
	}
	path := filepath.Join(workspaceRoot, ".goregraph-workspace", "workspace-map.html")
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(stderr, "error: dashboard not found; run goregraph workspace scan-all first: %v\n", err)
		return 1
	}
	abs, _ := filepath.Abs(path)
	fmt.Fprintf(stdout, "%s\n", abs)
	return 0
}

func runWorkspaceExplain(args []string, stdout, stderr io.Writer) int {
	cfg := config.Defaults()
	root := "."
	target := ""
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
			fmt.Fprint(stdout, "Usage: goregraph workspace explain <target> [--workspace <path>]\n\nExplains a route, file, symbol, contract, or feature from generated workspace outputs.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			if target == "" {
				target = arg
			} else {
				target += " " + arg
			}
		}
	}
	if target == "" {
		fmt.Fprint(stderr, "error: workspace explain requires a target\n")
		return 2
	}
	workspaceRoot, ok, err := scan.WorkspaceRoot(root, cfg)
	if err != nil || !ok {
		fmt.Fprintf(stderr, "error: workspace root not found: %v\n", err)
		return 1
	}
	record, err := scan.ExplainWorkspaceTarget(filepath.Join(workspaceRoot, ".goregraph-workspace"), target)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace explain failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(scan.RenderWorkspaceExplain(record)))
	return 0
}

func runWorkspacePath(args []string, stdout, stderr io.Writer) int {
	cfg := config.Defaults()
	root := "."
	from := ""
	to := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--from":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --from requires a target\n")
				return 2
			}
			i++
			from = args[i]
		case "--to":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --to requires a target\n")
				return 2
			}
			i++
			to = args[i]
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			cfg.WorkspaceRoot = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace path --from <target> --to <target> [--workspace <path>]\n\nShows the shortest generated workspace graph path between two targets.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}
	if from == "" || to == "" {
		fmt.Fprint(stderr, "error: workspace path requires --from and --to\n")
		return 2
	}
	workspaceRoot, ok, err := scan.WorkspaceRoot(root, cfg)
	if err != nil || !ok {
		fmt.Fprintf(stderr, "error: workspace root not found: %v\n", err)
		return 1
	}
	nodes, edges, err := scan.WorkspacePath(filepath.Join(workspaceRoot, ".goregraph-workspace"), from, to)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace path failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(scan.RenderWorkspacePath(nodes, edges)))
	return 0
}

func runWorkspaceImpact(args []string, stdout, stderr io.Writer) int {
	cfg := config.Defaults()
	root := "."
	var changedFiles []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--changed-file":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --changed-file requires a path\n")
				return 2
			}
			i++
			changedFiles = append(changedFiles, args[i])
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			cfg.WorkspaceRoot = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace impact --changed-file <path> [--changed-file <path>] [--workspace <path>]\n\nShows affected feature dossiers for changed files using generated workspace outputs.\n")
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			root = arg
		}
	}
	if len(changedFiles) == 0 {
		fmt.Fprint(stderr, "error: workspace impact requires at least one --changed-file\n")
		return 2
	}
	workspaceRoot, ok, err := scan.WorkspaceRoot(root, cfg)
	if err != nil || !ok {
		fmt.Fprintf(stderr, "error: workspace root not found: %v\n", err)
		return 1
	}
	record, err := scan.WorkspaceImpact(filepath.Join(workspaceRoot, ".goregraph-workspace"), changedFiles)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace impact failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(scan.RenderWorkspaceImpact(record)))
	return 0
}

func runWorkspaceDiff(args []string, stdout, stderr io.Writer) int {
	before := ""
	after := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--before":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --before requires a path\n")
				return 2
			}
			i++
			before = args[i]
		case "--after":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --after requires a path\n")
				return 2
			}
			i++
			after = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace diff --before <workspace-output> --after <workspace-output>\n\nCompares two .goregraph-workspace output directories without scanning.\n")
			return 0
		default:
			fmt.Fprintf(stderr, "unknown option: %s\n", arg)
			return 2
		}
	}
	if before == "" || after == "" {
		fmt.Fprint(stderr, "error: workspace diff requires --before and --after\n")
		return 2
	}
	diff, err := scan.WorkspaceDiff(before, after)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace diff failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(scan.RenderWorkspaceDiffReport(diff)))
	return 0
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

func runWorkspaceScanAll(args []string, stdout, stderr io.Writer) int {
	root := "."
	dryRun := false
	overrides := config.Defaults()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dry-run":
			dryRun = true
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
			fmt.Fprint(stdout, "Usage: goregraph workspace scan-all [path] [--dry-run] [--workspace <path>] [--no-update-gitignore]\n\nScans every discovered project in the detected workspace and refreshes workspace overlays after each scan.\n")
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

	plan, err := scan.WorkspaceProjectScanPlan(root, loaded)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace scan-all failed: %v\n", err)
		return 1
	}
	printWorkspaceScanAllPlan(stdout, plan, dryRun)
	if dryRun {
		return 0
	}
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
	fmt.Fprintf(stdout, "\nScanned %d workspace project(s).\n", scanned)
	return 0
}

func runWorkspaceClean(args []string, stdout, stderr io.Writer) int {
	root := "."
	execute := false
	overrides := config.Defaults()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--execute":
			execute = true
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprint(stderr, "error: --workspace requires a path\n")
				return 2
			}
			i++
			overrides.WorkspaceRoot = args[i]
		case "--help", "help":
			fmt.Fprint(stdout, "Usage: goregraph workspace clean [path] [--execute] [--workspace <path>]\n\nShows generated GoreGraph workspace output paths by default. Add --execute to remove project output directories and .goregraph-workspace.\n")
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
	loaded.Workspace = true
	loaded.WorkspaceRoot = overrides.WorkspaceRoot
	plan, err := scan.WorkspaceCleanPlan(root, loaded)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace clean failed: %v\n", err)
		return 1
	}
	printWorkspaceCleanPlan(stdout, plan, !execute)
	if !execute {
		return 0
	}
	removed := 0
	for _, item := range plan.Items {
		if !item.Exists {
			continue
		}
		if err := os.RemoveAll(item.Path); err != nil {
			fmt.Fprintf(stderr, "error: removing %s failed: %v\n", item.Path, err)
			return 1
		}
		removed++
	}
	fmt.Fprintf(stdout, "\nRemoved %d GoreGraph output path(s).\n", removed)
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

func printWorkspaceScanAllPlan(w io.Writer, plan scan.WorkspaceProjectScanPlanRecord, dryRun bool) {
	fmt.Fprint(w, "# GoreGraph Workspace Scan All Plan\n\n")
	fmt.Fprintf(w, "- Workspace root: `%s`\n", plan.WorkspaceRoot)
	if plan.Current != "" {
		fmt.Fprintf(w, "- Current project: `%s`\n", plan.Current)
	}
	fmt.Fprintf(w, "- Dry run: %t\n\n", dryRun)
	if len(plan.Items) == 0 {
		fmt.Fprint(w, "- none\n")
		return
	}
	for index, item := range plan.Items {
		fmt.Fprintf(w, "%d. project `%s` - %s\n", index+1, item.Project, emptyCLI(item.Status))
	}
	if dryRun {
		fmt.Fprint(w, "\nRun without `--dry-run` to scan these projects.\n")
	}
}

func printWorkspaceCleanPlan(w io.Writer, plan scan.WorkspaceCleanPlanRecord, dryRun bool) {
	fmt.Fprint(w, "# GoreGraph Workspace Clean Plan\n\n")
	fmt.Fprintf(w, "- Workspace root: `%s`\n", plan.WorkspaceRoot)
	if plan.Current != "" {
		fmt.Fprintf(w, "- Current project: `%s`\n", plan.Current)
	}
	fmt.Fprintf(w, "- Dry run: %t\n\n", dryRun)
	if len(plan.Items) == 0 {
		fmt.Fprint(w, "- none\n")
		return
	}
	for index, item := range plan.Items {
		status := "missing"
		if item.Exists {
			status = "exists"
		}
		fmt.Fprintf(w, "%d. `%s` - %s - %s\n", index+1, item.Path, status, item.Reason)
	}
	if dryRun {
		fmt.Fprint(w, "\nRun with `--execute` to remove these generated output paths.\n")
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
	if isAgentQueryTask(args[1]) {
		options := query.TaskOptions{Root: args[0], Task: args[1], Format: "json"}
		for i := 2; i < len(args); i++ {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "error: query option %s requires a value\n", args[i])
				return 2
			}
			value := args[i+1]
			i++
			switch args[i-1] {
			case "--query":
				options.Query = value
			case "--format":
				options.Format = value
			case "--detail":
				options.Detail = value
			case "--limit":
				parsed, err := strconv.Atoi(value)
				if err != nil {
					fmt.Fprintln(stderr, "error: --limit must be an integer")
					return 2
				}
				options.Limit = parsed
			case "--continue":
				options.Continuation = value
			default:
				fmt.Fprintf(stderr, "error: unknown query option %s\n", args[i-1])
				return 2
			}
		}
		result, err := query.RunTask(options)
		if err != nil {
			fmt.Fprintf(stderr, "error: query failed: %v\n", err)
			return 1
		}
		_, _ = stdout.Write([]byte(result))
		return 0
	}
	result, err := query.Search(args[0], strings.Join(args[1:], " "))
	if err != nil {
		fmt.Fprintf(stderr, "error: query failed: %v\n", err)
		return 1
	}
	_, _ = stdout.Write([]byte(result))
	return 0
}

func isAgentQueryTask(value string) bool {
	switch value {
	case "workspace-summary", "service-context", "endpoint-search", "endpoint-trace", "symbol-trace", "trace-from", "diagnostics", "coverage", "evidence", "tests", "change-context":
		return true
	default:
		return false
	}
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
  workspace         Show, scan, clean, and inspect workspace projects
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
  goregraph workspace scan-all .
  goregraph workspace refresh .
  goregraph workspace clean . --execute
  goregraph workspace dashboard .
  goregraph workspace explain "GET /users/{userId}"
  goregraph workspace path --from frontend/app --to UserController.get
  goregraph workspace impact --changed-file src/api/users.ts
  goregraph mcp
  goregraph version
`)
}

func printWorkspaceHelp(w io.Writer) {
	fmt.Fprint(w, `Usage: goregraph workspace <command> [options]

Commands:
  status [path]        Show workspace projects and loaded indexes without scanning
  scan-missing [path]  Show prioritized missing service scans; add --execute to scan
  scan-all [path]      Scan every discovered project in the workspace
  refresh [path]       Refresh workspace overlays without scanning source files
  clean [path]         Show generated workspace outputs; add --execute to remove
  dashboard [path]     Print generated workspace dashboard path
  explain <target>     Explain a route, file, symbol, contract, or feature
  path                 Show graph path between two workspace targets
  impact               Show affected features for changed files

Examples:
  goregraph workspace status .
  goregraph workspace scan-missing .
  goregraph workspace scan-missing . --top 5 --execute
  goregraph workspace scan-all .
  goregraph workspace refresh .
  goregraph workspace clean . --execute
  goregraph workspace dashboard .
  goregraph workspace explain "GET /users/{userId}"
  goregraph workspace path --from frontend/app --to UserController.get
  goregraph workspace impact --changed-file frontend/app/src/api/users.ts
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
