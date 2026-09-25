package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/testresults"
)

const resultsHelp = `Usage: goregraph results import <project-root> --suite <name> --from <junit.xml-or-glob> [--from <file-or-glob> ...] [options]

Imports JUnit XML without running tests or scanning source. Replaces only the named
suite's previous import and refreshes an existing workspace dashboard if present.
Relative report paths are resolved from the project root. No CI server is queried.

Options:
  --expected-shards N       Required for multiple files; missing shards stay incomplete
  --origin local|ci-artifact  Source label (default: local)
  --commit SHA              Declared commit; not independently verified
  --branch NAME             Declared branch
  --pipeline ID             Declared pipeline identifier
  --job ID                  Declared CI job identifier
`

func runResults(args []string, stdout, stderr io.Writer, execution buildExecution) int {
	if len(args) == 0 || (len(args) == 1 && isHelp(args[0])) {
		fmt.Fprint(stdout, resultsHelp)
		return 0
	}
	if len(args) == 2 && isHelp(args[1]) {
		fmt.Fprint(stdout, resultsHelp)
		return 0
	}
	if args[0] != "import" || len(args) < 2 {
		fmt.Fprint(stderr, "error: expected results import <project-root>\n")
		return 2
	}
	if isHelp(args[1]) {
		fmt.Fprint(stdout, resultsHelp)
		return 0
	}
	root, err := filepath.Abs(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	var options testresults.Options
	var patterns []string
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		key := args[i]
		if i+1 >= len(args) {
			fmt.Fprintf(stderr, "error: %s requires a value\n", key)
			return 2
		}
		i++
		value := args[i]
		if key != "--from" {
			if seen[key] {
				fmt.Fprintf(stderr, "error: duplicate %s\n", key)
				return 2
			}
			seen[key] = true
		}
		switch key {
		case "--from":
			patterns = append(patterns, value)
		case "--suite":
			options.Suite = value
		case "--origin":
			options.Origin = value
		case "--commit":
			options.Commit = value
		case "--branch":
			options.Branch = value
		case "--pipeline":
			options.Pipeline = value
		case "--job":
			options.Job = value
		case "--expected-shards":
			options.ExpectedShards, err = strconv.Atoi(value)
			if err != nil {
				fmt.Fprintln(stderr, "error: --expected-shards requires an integer")
				return 2
			}
		default:
			fmt.Fprintf(stderr, "error: unknown results option %s\n", key)
			return 2
		}
	}
	if len(patterns) == 0 {
		fmt.Fprintln(stderr, "error: at least one --from is required")
		return 2
	}
	if seen["--expected-shards"] && options.ExpectedShards < 1 {
		fmt.Fprintln(stderr, "error: --expected-shards must be at least 1")
		return 2
	}
	files, err := resultFiles(root, patterns)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	run, err := testresults.Parse(files, options)
	if err != nil {
		fmt.Fprintf(stderr, "error: test-result import failed: %v\n", err)
		return 1
	}
	return saveResults(root, run, stdout, stderr, execution)
}

func resultFiles(root string, patterns []string) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(root, pattern)
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid --from glob: %w", err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no JUnit reports matched %s", pattern)
		}
		files = append(files, matches...)
	}
	return files, nil
}

func saveResults(root string, run testresults.Run, stdout, stderr io.Writer, execution buildExecution) int {
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	outputRoot := filepath.Join(root, cfg.OutputDir)
	if err := testresults.Save(execution.ctx, outputRoot, run); err != nil {
		fmt.Fprintf(stderr, "error: saving test results failed: %v\n", err)
		return 1
	}
	cfg.Workspace = true
	cfg.UpdateGitignore = false
	if _, err := scan.ReconcileWorkspaceWithOptions(execution.ctx, root, cfg, scan.BuildTargetDashboard, execution.options); err != nil {
		fmt.Fprintf(stderr, "error: results saved but workspace dashboard refresh failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Imported %d JUnit report(s) for %s: %s (%d tests).\n", len(run.Reports), run.Suite, run.Status, run.Counts.Tests)
	if !run.Complete {
		fmt.Fprintln(stdout, "Result is incomplete; no aggregate pass is claimed.")
	}
	if run.Commit == "" {
		fmt.Fprintln(stdout, "Revision unknown; this result does not certify the current checkout.")
	}
	if strings.TrimSpace(run.Commit) != "" {
		fmt.Fprintln(stdout, "Commit identity is caller-declared and has not been verified.")
	}
	return 0
}
