package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/agentbench"
)

type packDiffReport struct {
	Passed     bool                   `json:"passed"`
	Diff       agentbench.PackDiff    `json:"diff"`
	Violations []agentbench.Violation `json:"violations"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return commandError(stderr, "command is required")
	}

	switch args[0] {
	case "verify-pack":
		return runVerifyPack(args[1:], stdout, stderr)
	case "diff-pack":
		return runDiffPack(args[1:], stderr)
	case "gate":
		return runGate(args[1:], stderr)
	default:
		return commandError(stderr, "unknown command %q", args[0])
	}
}

func runVerifyPack(args []string, stdout, stderr io.Writer) int {
	flags, err := parseFileFlags(args, "contract", "pack")
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	if err := validateInputPaths(flags); err != nil {
		return commandError(stderr, "%v", err)
	}

	contract, err := agentbench.LoadContract(flags["contract"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	pack, err := loadStrictJSON[agent.ContextPack](flags["pack"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}

	violations := agentbench.EvaluatePack(pack, contract.Pack)
	sortViolations(violations)
	report := agentbench.GateReport{
		Passed:   len(violations) == 0,
		Failures: violationFailures(violations),
	}
	if err := writeJSON(stdout, report); err != nil {
		return commandError(stderr, "write report: %v", err)
	}
	return evaluationExit(report.Passed)
}

func runDiffPack(args []string, stderr io.Writer) int {
	flags, err := parseFileFlags(args, "golden-pack", "candidate-pack", "hypothesis", "output")
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	if err := validateInputPaths(map[string]string{
		"golden-pack":    flags["golden-pack"],
		"candidate-pack": flags["candidate-pack"],
		"hypothesis":     flags["hypothesis"],
	}); err != nil {
		return commandError(stderr, "%v", err)
	}
	if err := validateOutputPath(flags["output"]); err != nil {
		return commandError(stderr, "%v", err)
	}

	golden, err := loadStrictJSON[agent.ContextPack](flags["golden-pack"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	candidate, err := loadStrictJSON[agent.ContextPack](flags["candidate-pack"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	hypothesis, err := agentbench.LoadHypothesis(flags["hypothesis"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}

	diff := agentbench.DiffPacks(golden, candidate)
	violations := agentbench.EvaluatePackDiff(diff, hypothesis)
	sortViolations(violations)
	report := packDiffReport{
		Passed:     len(violations) == 0,
		Diff:       diff,
		Violations: violations,
	}
	if err := writeNewJSON(flags["output"], report); err != nil {
		return commandError(stderr, "%v", err)
	}
	return evaluationExit(report.Passed)
}

func runGate(args []string, stderr io.Writer) int {
	flags, err := parseFileFlags(
		args,
		"contract",
		"hypothesis",
		"golden-runs",
		"candidate-runs",
		"output",
	)
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	if err := validateInputPaths(map[string]string{
		"contract":       flags["contract"],
		"hypothesis":     flags["hypothesis"],
		"golden-runs":    flags["golden-runs"],
		"candidate-runs": flags["candidate-runs"],
	}); err != nil {
		return commandError(stderr, "%v", err)
	}
	if err := validateOutputPath(flags["output"]); err != nil {
		return commandError(stderr, "%v", err)
	}

	contract, err := agentbench.LoadContract(flags["contract"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	hypothesis, err := agentbench.LoadHypothesis(flags["hypothesis"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	golden, err := loadStrictJSON[[]agentbench.ReviewedRun](flags["golden-runs"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	candidate, err := loadStrictJSON[[]agentbench.ReviewedRun](flags["candidate-runs"])
	if err != nil {
		return commandError(stderr, "%v", err)
	}

	report := agentbench.EvaluateCase(contract, golden, candidate, hypothesis)
	sort.Strings(report.Failures)
	if err := writeNewJSON(flags["output"], report); err != nil {
		return commandError(stderr, "%v", err)
	}
	return evaluationExit(report.Passed)
}

func parseFileFlags(args []string, required ...string) (map[string]string, error) {
	allowed := make(map[string]bool, len(required))
	for _, name := range required {
		allowed[name] = true
	}

	values := make(map[string]string, len(required))
	for index := 0; index < len(args); index += 2 {
		name := strings.TrimPrefix(args[index], "--")
		if !strings.HasPrefix(args[index], "--") || name == "" {
			return nil, fmt.Errorf("unexpected argument %q", args[index])
		}
		if !allowed[name] {
			return nil, fmt.Errorf("unknown flag --%s", name)
		}
		if _, duplicate := values[name]; duplicate {
			return nil, fmt.Errorf("duplicate flag --%s", name)
		}
		if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
			return nil, fmt.Errorf("flag --%s requires a value", name)
		}
		values[name] = args[index+1]
	}

	for _, name := range required {
		if values[name] == "" {
			return nil, fmt.Errorf("required flag --%s is missing", name)
		}
	}
	return values, nil
}

func validateInputPaths(paths map[string]string) error {
	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !filepath.IsAbs(paths[name]) {
			return fmt.Errorf("--%s must be an absolute path", name)
		}
	}
	return nil
}

func validateOutputPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("--output must be an absolute path")
	}
	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("inspect output parent: %w", err)
	}
	if !parentInfo.IsDir() {
		return fmt.Errorf("output parent is not a directory")
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("output path already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output path: %w", err)
	}
	return nil
}

func loadStrictJSON[T any](path string) (T, error) {
	var value T
	file, err := os.Open(path)
	if err != nil {
		return value, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("decode %s: %w", path, err)
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if err == io.EOF {
		return value, nil
	}
	if err == nil {
		return value, fmt.Errorf("decode %s: trailing JSON value", path)
	}
	return value, fmt.Errorf("decode %s: trailing data: %w", path, err)
}

func violationFailures(violations []agentbench.Violation) []string {
	failures := make([]string, 0, len(violations))
	for _, violation := range violations {
		failures = append(failures, fmt.Sprintf("%s: %s", violation.Field, violation.Reason))
	}
	return failures
}

func sortViolations(violations []agentbench.Violation) {
	sort.Slice(violations, func(first, second int) bool {
		if violations[first].Field != violations[second].Field {
			return violations[first].Field < violations[second].Field
		}
		return violations[first].Reason < violations[second].Reason
	})
}

func writeJSON(destination io.Writer, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	written, err := destination.Write(body)
	if err != nil {
		return err
	}
	if written != len(body) {
		return io.ErrShortWrite
	}
	return nil
}

func writeNewJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	if err := writeJSON(file, value); err != nil {
		_ = file.Close()
		return fmt.Errorf("write output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	return nil
}

func evaluationExit(passed bool) int {
	if passed {
		return 0
	}
	return 1
}

func commandError(stderr io.Writer, format string, arguments ...any) int {
	fmt.Fprintf(stderr, "error: "+format+"\n", arguments...)
	return 2
}
