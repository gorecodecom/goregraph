package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitupdate"
	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	gitUpdateFormatText = "text"
	gitUpdateFormatJSON = "json"
)

type gitUpdateArguments struct {
	path    string
	execute bool
	format  string
}

func runGit(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, "error: usage: goregraph git update [path] [--execute] [--format text|json]\n")
		return 2
	}
	if args[0] != "update" {
		fmt.Fprintf(stderr, "unknown git command: %s\n", args[0])
		return 2
	}
	return runGitUpdate(args[1:], stdout, stderr)
}

func runWorkspaceGit(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, "error: usage: goregraph workspace git update [path] [--execute] [--format text|json]\n")
		return 2
	}
	if args[0] != "update" {
		fmt.Fprintf(stderr, "unknown workspace git command: %s\n", args[0])
		return 2
	}
	return runWorkspaceGitUpdate(args[1:], stdout, stderr)
}

func runGitUpdate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && isHelp(args[0]) {
		printGitUpdateHelp(stdout, false)
		return 0
	}
	parsed, err := parseGitUpdateArguments(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	report, err := gitupdate.Run(context.Background(), []gitupdate.Target{{Path: parsed.path}}, gitupdate.Options{Execute: parsed.execute})
	if err != nil {
		fmt.Fprintf(stderr, "error: Git update failed: %v\n", err)
		return 1
	}
	return renderGitUpdateReport(stdout, stderr, parsed.format, report)
}

func runWorkspaceGitUpdate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && isHelp(args[0]) {
		printGitUpdateHelp(stdout, true)
		return 0
	}
	parsed, err := parseGitUpdateArguments(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	loaded, err := config.Load(parsed.path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	loaded.Workspace = true
	plan, err := scan.WorkspaceProjectScanPlan(parsed.path, loaded)
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace Git update failed: %v\n", err)
		return 1
	}
	targets := make([]gitupdate.Target, 0, len(plan.Items))
	for _, item := range plan.Items {
		targets = append(targets, gitupdate.Target{Path: item.AbsPath})
	}
	report, err := gitupdate.Run(context.Background(), targets, gitupdate.Options{
		Execute:       parsed.execute,
		WorkspaceRoot: plan.WorkspaceRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: workspace Git update failed: %v\n", err)
		return 1
	}
	return renderGitUpdateReport(stdout, stderr, parsed.format, report)
}

func parseGitUpdateArguments(args []string) (gitUpdateArguments, error) {
	parsed := gitUpdateArguments{path: ".", format: gitUpdateFormatText}
	pathSet := false
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch argument {
		case "--execute":
			parsed.execute = true
		case "--format":
			if index+1 >= len(args) {
				return gitUpdateArguments{}, fmt.Errorf("--format requires text or json")
			}
			index++
			parsed.format = args[index]
			if parsed.format != gitUpdateFormatText && parsed.format != gitUpdateFormatJSON {
				return gitUpdateArguments{}, fmt.Errorf("unknown format: %s", parsed.format)
			}
		default:
			if strings.HasPrefix(argument, "-") {
				return gitUpdateArguments{}, fmt.Errorf("unknown option: %s", argument)
			}
			if pathSet {
				return gitUpdateArguments{}, fmt.Errorf("multiple paths are not supported: %s", argument)
			}
			parsed.path = argument
			pathSet = true
		}
	}
	return parsed, nil
}

func renderGitUpdateReport(stdout, stderr io.Writer, format string, report gitupdate.Report) int {
	var err error
	if format == gitUpdateFormatJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(report)
	} else {
		err = renderGitUpdateText(stdout, report)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: rendering Git update report failed: %v\n", err)
		return 1
	}
	return report.ExitCode()
}

func renderGitUpdateText(writer io.Writer, report gitupdate.Report) error {
	var output strings.Builder
	output.WriteString("# GoreGraph Git Update\n\n")
	fmt.Fprintf(&output, "Mode: %s\n", report.Mode)
	if report.WorkspaceRoot != "" {
		fmt.Fprintf(&output, "Workspace root: %s\n", report.WorkspaceRoot)
	}
	fmt.Fprintf(&output, "Repositories: %d\n", len(report.Repositories))
	for index, repository := range report.Repositories {
		fmt.Fprintf(&output, "\nRepository %d:\n", index+1)
		fmt.Fprintf(&output, "- Path: %s\n", gitUpdateTextValue(repository.Path))
		fmt.Fprintf(&output, "- Git root: %s\n", gitUpdateTextValue(repository.GitRoot))
		fmt.Fprintf(&output, "- Remote: %s\n", gitUpdateTextValue(repository.Remote))
		fmt.Fprintf(&output, "- Branch before: %s\n", gitUpdateTextValue(repository.BranchBefore))
		fmt.Fprintf(&output, "- Branch after: %s\n", gitUpdateTextValue(repository.BranchAfter))
		fmt.Fprintf(&output, "- Commit before: %s\n", gitUpdateTextValue(repository.CommitBefore))
		fmt.Fprintf(&output, "- Commit after: %s\n", gitUpdateTextValue(repository.CommitAfter))
		fmt.Fprintf(&output, "- Status: %s\n", repository.Status)
		fmt.Fprintf(&output, "- Reason: %s\n", gitUpdateTextValue(repository.Reason))
		fmt.Fprintf(&output, "- Remediation: %s\n", gitUpdateTextValue(repository.Remediation))
		fmt.Fprintf(&output, "- Executed: %t\n", repository.Executed)
	}

	statuses := make([]string, 0, len(report.Summary))
	for status := range report.Summary {
		statuses = append(statuses, string(status))
	}
	sort.Strings(statuses)
	output.WriteString("\nSummary:\n")
	if len(statuses) == 0 {
		output.WriteString("- none\n")
	}
	for _, status := range statuses {
		fmt.Fprintf(&output, "- %s: %d\n", status, report.Summary[gitupdate.Status(status)])
	}
	_, err := io.WriteString(writer, output.String())
	return err
}

func gitUpdateTextValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func printGitUpdateHelp(writer io.Writer, workspace bool) {
	command := "goregraph git update"
	if workspace {
		command = "goregraph workspace git update"
	}
	fmt.Fprintf(writer, `Usage: %s [path] [--execute] [--format text|json]

The default mode is preview and uses cached remote refs. Add --execute to fetch and fast-forward eligible repositories.

Options:
  --execute             Execute the planned safe update
  --format text|json    Select text or indented JSON output

Examples:
  %s .
  %s . --execute
  %s . --format json
`, command, command, command, command)
}
