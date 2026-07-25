package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/agentbench"
)

type packDiffReport struct {
	Passed     bool                   `json:"passed"`
	Diff       agentbench.PackDiff    `json:"diff"`
	Violations []agentbench.Violation `json:"violations"`
}

type temporaryFile interface {
	io.Writer
	Name() string
	Sync() error
	Close() error
}

type fileOutput struct {
	createTemp func(directory, pattern string) (temporaryFile, error)
	link       func(oldPath, newPath string) error
	remove     func(path string) error
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return commandError(stderr, "command is required")
	}

	switch args[0] {
	case "run":
		return runRegressionCommand(
			args[1:],
			stderr,
			os.LookupEnv,
			agentbench.RunRegression,
		)
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

func runRegressionCommand(
	args []string,
	stderr io.Writer,
	getenv func(string) (string, bool),
	execute func(context.Context, agentbench.RunnerConfig) error,
) int {
	flags, err := parseFileFlags(
		args,
		"matrix",
		"external-case",
		"golden-binary",
		"golden-commit",
		"candidate-binary",
		"candidate-commit",
		"instruction",
		"phase",
		"target-case",
		"runs",
		"output",
	)
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	if err := validateInputPaths(map[string]string{
		"matrix":           flags["matrix"],
		"external-case":    flags["external-case"],
		"golden-binary":    flags["golden-binary"],
		"candidate-binary": flags["candidate-binary"],
		"instruction":      flags["instruction"],
	}); err != nil {
		return commandError(stderr, "%v", err)
	}
	if !filepath.IsAbs(flags["output"]) {
		return commandError(stderr, "--output must be an absolute path")
	}
	runs, err := strconv.Atoi(flags["runs"])
	if err != nil {
		return commandError(stderr, "--runs must be an integer")
	}
	codexSource, present := getenv("CODEX_BENCHMARK_ARGS")
	if !present || codexSource == "" {
		return commandError(stderr, "CODEX_BENCHMARK_ARGS must contain one literal argument per line")
	}
	codexArgs := strings.Split(codexSource, "\n")
	for _, argument := range codexArgs {
		if argument == "" || strings.ContainsRune(argument, '\r') {
			return commandError(stderr, "CODEX_BENCHMARK_ARGS must not contain empty or carriage-return arguments")
		}
	}
	analyzerPath, err := repositoryAnalyzerPath()
	if err != nil {
		return commandError(stderr, "%v", err)
	}
	config := agentbench.RunnerConfig{
		MatrixPath:      flags["matrix"],
		ExternalCase:    flags["external-case"],
		GoldenBinary:    flags["golden-binary"],
		GoldenCommit:    flags["golden-commit"],
		CandidateBinary: flags["candidate-binary"],
		CandidateCommit: flags["candidate-commit"],
		InstructionPath: flags["instruction"],
		Phase:           flags["phase"],
		TargetCase:      flags["target-case"],
		Runs:            runs,
		Output:          flags["output"],
		CodexArgs:       codexArgs,
		AnalyzerPath:    analyzerPath,
	}
	commandContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	if err := execute(commandContext, config); err != nil {
		return commandError(stderr, "%v", err)
	}
	return 0
}

func repositoryAnalyzerPath() (string, error) {
	_, sourcePath, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve command source path")
	}
	path := filepath.Join(filepath.Dir(filepath.Dir(sourcePath)), "analyze-agent-context-log.sh")
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("analyzer path is not absolute")
	}
	return path, nil
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
	if err := validateHypothesis(hypothesis); err != nil {
		return commandError(stderr, "invalid hypothesis: %v", err)
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
	if err := validateHypothesis(hypothesis); err != nil {
		return commandError(stderr, "invalid hypothesis: %v", err)
	}
	if !hypothesisTargetsCase(hypothesis, contract.ID) {
		return commandError(stderr, "hypothesis does not target contract %q", contract.ID)
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

func validateHypothesis(hypothesis agentbench.Hypothesis) error {
	cases := make([]agentbench.MatrixCase, 0, len(hypothesis.TargetCases))
	for _, caseID := range hypothesis.TargetCases {
		cases = append(cases, agentbench.MatrixCase{ID: caseID, Directory: "."})
	}
	if len(cases) == 0 {
		cases = append(cases, agentbench.MatrixCase{ID: "validation", Directory: "."})
	}
	return agentbench.ValidateHypothesis(hypothesis, agentbench.Matrix{
		Schema: 1,
		Cases:  cases,
	})
}

func hypothesisTargetsCase(hypothesis agentbench.Hypothesis, caseID string) bool {
	for _, targetCase := range hypothesis.TargetCases {
		if targetCase == caseID {
			return true
		}
	}
	return false
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
	body, err := marshalJSON(value)
	if err != nil {
		return err
	}
	return writeAll(destination, body)
}

func marshalJSON(value any) ([]byte, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func writeAll(destination io.Writer, body []byte) error {
	for len(body) > 0 {
		written, err := destination.Write(body)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(body) {
			return io.ErrShortWrite
		}
		body = body[written:]
	}
	return nil
}

func writeNewJSON(path string, value any) error {
	return writeNewJSONWithOutput(path, value, realFileOutput())
}

func realFileOutput() fileOutput {
	return fileOutput{
		createTemp: func(directory, pattern string) (temporaryFile, error) {
			return os.CreateTemp(directory, pattern)
		},
		link:   os.Link,
		remove: os.Remove,
	}
}

func writeNewJSONWithOutput(path string, value any, operations fileOutput) (resultErr error) {
	body, err := marshalJSON(value)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}

	file, err := operations.createTemp(filepath.Dir(path), ".agent-context-regression-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tempPath := file.Name()
	defer func() {
		if err := operations.remove(tempPath); err != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("remove temporary output: %w", err),
			)
		}
	}()

	if err := writeAll(file, body); err != nil {
		_ = file.Close()
		return fmt.Errorf("write output: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if err := operations.link(tempPath, path); err != nil {
		return fmt.Errorf("publish output: %w", err)
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
