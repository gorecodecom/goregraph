package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/agentbench"
)

func TestRunRequiresCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("run exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want diagnostic")
	}
}

func TestRunCommandBuildsRunnerConfig(t *testing.T) {
	root := t.TempDir()
	arguments := runnerCommandArguments(root)
	environment := strings.Join(safeCommandCodexArgs(), "\n")
	var captured agentbench.RunnerConfig
	called := false
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runRegressionCommand(
		arguments,
		&stderr,
		func(name string) (string, bool) {
			if name != "CODEX_BENCHMARK_ARGS" {
				return "", false
			}
			return environment, true
		},
		func(_ context.Context, config agentbench.RunnerConfig) error {
			called = true
			captured = config
			return nil
		},
	)

	if code != 0 {
		t.Fatalf("run command exit = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !called {
		t.Fatal("runner was not called")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q; want empty", stdout.String(), stderr.String())
	}
	if got, want := captured.CodexArgs, safeCommandCodexArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("CodexArgs = %q, want %q", got, want)
	}
	if captured.Phase != "smoke" ||
		captured.TargetCase != "g1" ||
		captured.Runs != 1 ||
		!filepath.IsAbs(captured.AnalyzerPath) ||
		filepath.Base(captured.AnalyzerPath) != "analyze-agent-context-log.sh" {
		t.Fatalf("RunnerConfig = %#v, want parsed smoke config with repository analyzer", captured)
	}
	if captured.MatrixPath != filepath.Join(root, "matrix.json") ||
		captured.Output != filepath.Join(root, "output") {
		t.Fatalf("RunnerConfig paths = %#v, want literal flag paths", captured)
	}
}

func TestRunCommandRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name        string
		mutateArgs  func([]string) []string
		environment string
		present     bool
	}{
		{
			name:        "missing required flag",
			mutateArgs:  func(args []string) []string { return args[:len(args)-2] },
			environment: strings.Join(safeCommandCodexArgs(), "\n"),
			present:     true,
		},
		{
			name: "duplicate flag",
			mutateArgs: func(args []string) []string {
				return append(args, "--matrix", args[1])
			},
			environment: strings.Join(safeCommandCodexArgs(), "\n"),
			present:     true,
		},
		{
			name: "unknown flag",
			mutateArgs: func(args []string) []string {
				return append(args, "--unknown", args[1])
			},
			environment: strings.Join(safeCommandCodexArgs(), "\n"),
			present:     true,
		},
		{
			name: "relative path",
			mutateArgs: func(args []string) []string {
				args[1] = "matrix.json"
				return args
			},
			environment: strings.Join(safeCommandCodexArgs(), "\n"),
			present:     true,
		},
		{
			name: "invalid run count",
			mutateArgs: func(args []string) []string {
				args[len(args)-3] = "many"
				return args
			},
			environment: strings.Join(safeCommandCodexArgs(), "\n"),
			present:     true,
		},
		{
			name:        "missing Codex args",
			environment: "",
			present:     false,
		},
		{
			name:        "empty Codex args",
			environment: "",
			present:     true,
		},
		{
			name:        "blank Codex argument line",
			environment: strings.Join(append(safeCommandCodexArgs(), ""), "\n"),
			present:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := runnerCommandArguments(t.TempDir())
			if test.mutateArgs != nil {
				args = test.mutateArgs(args)
			}
			called := false
			var stderr bytes.Buffer

			code := runRegressionCommand(
				args,
				&stderr,
				func(string) (string, bool) { return test.environment, test.present },
				func(context.Context, agentbench.RunnerConfig) error {
					called = true
					return nil
				},
			)

			if code != 2 {
				t.Fatalf("run command exit = %d, want 2", code)
			}
			if called {
				t.Fatal("runner called for malformed command")
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr is empty, want diagnostic")
			}
		})
	}
}

func TestRunCommandReturnsInfrastructureFailure(t *testing.T) {
	var stderr bytes.Buffer

	code := runRegressionCommand(
		runnerCommandArguments(t.TempDir()),
		&stderr,
		func(string) (string, bool) {
			return strings.Join(safeCommandCodexArgs(), "\n"), true
		},
		func(context.Context, agentbench.RunnerConfig) error {
			return errors.New("injected runner failure")
		},
	)

	if code != 2 {
		t.Fatalf("run command exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "injected runner failure") {
		t.Fatalf("stderr = %q, want runner failure", stderr.String())
	}
}

func TestRunVerifyPack(t *testing.T) {
	t.Run("passes", func(t *testing.T) {
		fixture := newCommandFixture(t)

		code, stdout, stderr := invoke(
			"verify-pack",
			"--contract", fixture.contract,
			"--pack", fixture.goldenPack,
		)

		if code != 0 {
			t.Fatalf("run exit = %d, want 0; stderr = %q", code, stderr)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}
		requireStableJSON(t, stdout)
		var report agentbench.GateReport
		decodeJSON(t, stdout, &report)
		if !report.Passed || len(report.Failures) != 0 {
			t.Fatalf("report = %#v, want passing report", report)
		}
	})

	t.Run("fails evaluation", func(t *testing.T) {
		fixture := newCommandFixture(t)
		pack := validPack()
		pack.Endpoints[0].HTTPMethod = "GET"
		writeJSONFile(t, fixture.goldenPack, pack)

		code, stdout, stderr := invoke(
			"verify-pack",
			"--contract", fixture.contract,
			"--pack", fixture.goldenPack,
		)

		if code != 1 {
			t.Fatalf("run exit = %d, want 1; stderr = %q", code, stderr)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}
		var report agentbench.GateReport
		decodeJSON(t, stdout, &report)
		if report.Passed || len(report.Failures) == 0 {
			t.Fatalf("report = %#v, want failed report", report)
		}
	})

	t.Run("rejects malformed pack", func(t *testing.T) {
		fixture := newCommandFixture(t)
		writeTextFile(t, fixture.goldenPack, `{"schema":1,"unknown":true}`)

		code, stdout, stderr := invoke(
			"verify-pack",
			"--contract", fixture.contract,
			"--pack", fixture.goldenPack,
		)

		if code != 2 {
			t.Fatalf("run exit = %d, want 2", code)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "unknown") {
			t.Fatalf("stderr = %q, want unknown-field diagnostic", stderr)
		}
	})
}

func TestRunVerifyPackRejectsShortOutputWrite(t *testing.T) {
	fixture := newCommandFixture(t)
	var stderr bytes.Buffer

	code := run([]string{
		"verify-pack",
		"--contract", fixture.contract,
		"--pack", fixture.goldenPack,
	}, shortWriter{}, &stderr)

	if code != 2 {
		t.Fatalf("run exit = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want write diagnostic")
	}
}

func TestRunDiffPack(t *testing.T) {
	t.Run("passes", func(t *testing.T) {
		fixture := newCommandFixture(t)
		output := filepath.Join(t.TempDir(), "diff.json")

		code, stdout, stderr := invoke(
			"diff-pack",
			"--golden-pack", fixture.goldenPack,
			"--candidate-pack", fixture.candidatePack,
			"--hypothesis", fixture.hypothesis,
			"--output", output,
		)

		if code != 0 {
			t.Fatalf("run exit = %d, want 0; stderr = %q", code, stderr)
		}
		if stdout != "" || stderr != "" {
			t.Fatalf("stdout = %q, stderr = %q; want both empty", stdout, stderr)
		}
		body := readTextFile(t, output)
		requireStableJSON(t, body)
		var report packDiffReport
		decodeJSON(t, body, &report)
		if !report.Passed || len(report.Violations) != 0 {
			t.Fatalf("report = %#v, want passing report", report)
		}
	})

	t.Run("retains complete protected endpoint diff on failure", func(t *testing.T) {
		fixture := newCommandFixture(t)
		candidate := validPack()
		candidate.Endpoints[0].Path = "/catalog/{catalogId}"
		writeJSONFile(t, fixture.candidatePack, candidate)
		output := filepath.Join(t.TempDir(), "diff.json")

		code, stdout, stderr := invoke(
			"diff-pack",
			"--golden-pack", fixture.goldenPack,
			"--candidate-pack", fixture.candidatePack,
			"--hypothesis", fixture.hypothesis,
			"--output", output,
		)

		if code != 1 {
			t.Fatalf("run exit = %d, want 1; stderr = %q", code, stderr)
		}
		if stdout != "" || stderr != "" {
			t.Fatalf("stdout = %q, stderr = %q; want both empty", stdout, stderr)
		}
		var report packDiffReport
		decodeJSON(t, readTextFile(t, output), &report)
		if report.Passed {
			t.Fatalf("report = %#v, want failure", report)
		}
		if !report.Diff.EndpointChanged ||
			report.Diff.GoldenEndpoint == "" ||
			report.Diff.CandidateEndpoint == "" {
			t.Fatalf("diff = %#v, want complete endpoint diff", report.Diff)
		}
		if len(report.Violations) == 0 ||
			report.Violations[0].Field != "endpoint" {
			t.Fatalf("violations = %#v, want endpoint violation", report.Violations)
		}
	})
}

func TestRunGate(t *testing.T) {
	t.Run("passes", func(t *testing.T) {
		fixture := newCommandFixture(t)
		output := filepath.Join(t.TempDir(), "gate.json")

		code, stdout, stderr := invoke(
			"gate",
			"--contract", fixture.contract,
			"--hypothesis", fixture.hypothesis,
			"--pack-diff", fixture.packDiff,
			"--golden-runs", fixture.goldenRuns,
			"--candidate-runs", fixture.candidateRuns,
			"--output", output,
		)

		if code != 0 {
			t.Fatalf("run exit = %d, want 0; stderr = %q", code, stderr)
		}
		if stdout != "" || stderr != "" {
			t.Fatalf("stdout = %q, stderr = %q; want both empty", stdout, stderr)
		}
		var report agentbench.GateReport
		body := readTextFile(t, output)
		requireStableJSON(t, body)
		decodeJSON(t, body, &report)
		if !report.Passed || len(report.Failures) != 0 {
			t.Fatalf("report = %#v, want pass", report)
		}
	})

	t.Run("accepts a multi-case hypothesis for the declared case", func(t *testing.T) {
		fixture := newCommandFixture(t)
		hypothesis := validHypothesis()
		hypothesis.TargetCases = []string{"case", "other-case"}
		writeJSONFile(t, fixture.hypothesis, hypothesis)
		output := filepath.Join(t.TempDir(), "gate.json")

		code, _, stderr := invoke(gateArgs(fixture, output)...)

		if code != 0 {
			t.Fatalf("run exit = %d, want 0; stderr = %q", code, stderr)
		}
	})

	t.Run("writes complete failure report", func(t *testing.T) {
		fixture := newCommandFixture(t)
		runs := reviewedRuns("candidate", "fail")
		writeJSONFile(t, fixture.candidateRuns, runs)
		output := filepath.Join(t.TempDir(), "gate.json")

		code, stdout, stderr := invoke(
			"gate",
			"--contract", fixture.contract,
			"--hypothesis", fixture.hypothesis,
			"--pack-diff", fixture.packDiff,
			"--golden-runs", fixture.goldenRuns,
			"--candidate-runs", fixture.candidateRuns,
			"--output", output,
		)

		if code != 1 {
			t.Fatalf("run exit = %d, want 1; stderr = %q", code, stderr)
		}
		if stdout != "" || stderr != "" {
			t.Fatalf("stdout = %q, stderr = %q; want both empty", stdout, stderr)
		}
		var report agentbench.GateReport
		decodeJSON(t, readTextFile(t, output), &report)
		if report.Passed || len(report.Failures) != 1 ||
			!strings.Contains(report.Failures[0], "passed in 0 of 3") {
			t.Fatalf("report = %#v, want complete target-facet failure", report)
		}
	})

	t.Run("requires Pack Diff evidence", func(t *testing.T) {
		fixture := newCommandFixture(t)
		output := filepath.Join(t.TempDir(), "gate.json")

		code, stdout, stderr := invoke(
			"gate",
			"--contract", fixture.contract,
			"--hypothesis", fixture.hypothesis,
			"--golden-runs", fixture.goldenRuns,
			"--candidate-runs", fixture.candidateRuns,
			"--output", output,
		)

		if code != 2 {
			t.Fatalf("run exit = %d, want 2", code)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "--pack-diff") {
			t.Fatalf("stderr = %q, want --pack-diff diagnostic", stderr)
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatalf("gate output exists after missing Pack Diff: %v", err)
		}
	})
}

func TestRunRejectsMalformedCommandsAndInputs(t *testing.T) {
	tests := []struct {
		name    string
		command func(t *testing.T, fixture commandFixture) []string
	}{
		{
			name: "unknown subcommand",
			command: func(_ *testing.T, _ commandFixture) []string {
				return []string{"unknown"}
			},
		},
		{
			name: "missing required flag",
			command: func(_ *testing.T, fixture commandFixture) []string {
				return []string{"verify-pack", "--contract", fixture.contract}
			},
		},
		{
			name: "duplicate flag",
			command: func(_ *testing.T, fixture commandFixture) []string {
				return []string{
					"verify-pack",
					"--contract", fixture.contract,
					"--contract", fixture.contract,
					"--pack", fixture.goldenPack,
				}
			},
		},
		{
			name: "unknown flag",
			command: func(_ *testing.T, fixture commandFixture) []string {
				return []string{
					"verify-pack",
					"--contract", fixture.contract,
					"--pack", fixture.goldenPack,
					"--extra", fixture.goldenPack,
				}
			},
		},
		{
			name: "relative input path",
			command: func(_ *testing.T, fixture commandFixture) []string {
				return []string{
					"verify-pack",
					"--contract", "contract.json",
					"--pack", fixture.goldenPack,
				}
			},
		},
		{
			name: "relative output path",
			command: func(_ *testing.T, fixture commandFixture) []string {
				return []string{
					"diff-pack",
					"--golden-pack", fixture.goldenPack,
					"--candidate-pack", fixture.candidatePack,
					"--hypothesis", fixture.hypothesis,
					"--output", "diff.json",
				}
			},
		},
		{
			name: "unreadable input",
			command: func(_ *testing.T, fixture commandFixture) []string {
				return []string{
					"verify-pack",
					"--contract", fixture.contract,
					"--pack", filepath.Join(filepath.Dir(fixture.goldenPack), "missing.json"),
				}
			},
		},
		{
			name: "pack unknown field",
			command: func(t *testing.T, fixture commandFixture) []string {
				writeTextFile(t, fixture.goldenPack, `{"schema":1,"unexpected":true}`)
				return []string{
					"verify-pack",
					"--contract", fixture.contract,
					"--pack", fixture.goldenPack,
				}
			},
		},
		{
			name: "pack trailing JSON",
			command: func(t *testing.T, fixture commandFixture) []string {
				writeTextFile(t, fixture.goldenPack, string(mustJSON(t, validPack()))+" {}")
				return []string{
					"verify-pack",
					"--contract", fixture.contract,
					"--pack", fixture.goldenPack,
				}
			},
		},
		{
			name: "hypothesis unknown field",
			command: func(t *testing.T, fixture commandFixture) []string {
				writeTextFile(t, fixture.hypothesis, `{"schema":1,"unexpected":true}`)
				return []string{
					"diff-pack",
					"--golden-pack", fixture.goldenPack,
					"--candidate-pack", fixture.candidatePack,
					"--hypothesis", fixture.hypothesis,
					"--output", filepath.Join(t.TempDir(), "diff.json"),
				}
			},
		},
		{
			name: "hypothesis trailing JSON",
			command: func(t *testing.T, fixture commandFixture) []string {
				writeTextFile(t, fixture.hypothesis, string(mustJSON(t, validHypothesis()))+" {}")
				return []string{
					"diff-pack",
					"--golden-pack", fixture.goldenPack,
					"--candidate-pack", fixture.candidatePack,
					"--hypothesis", fixture.hypothesis,
					"--output", filepath.Join(t.TempDir(), "diff.json"),
				}
			},
		},
		{
			name: "invalid hypothesis schema",
			command: func(t *testing.T, fixture commandFixture) []string {
				hypothesis := validHypothesis()
				hypothesis.Schema = 0
				writeJSONFile(t, fixture.hypothesis, hypothesis)
				return diffArgs(fixture, filepath.Join(t.TempDir(), "diff.json"))
			},
		},
		{
			name: "duplicate hypothesis target",
			command: func(t *testing.T, fixture commandFixture) []string {
				hypothesis := validHypothesis()
				hypothesis.TargetCases = []string{"case", "case"}
				writeJSONFile(t, fixture.hypothesis, hypothesis)
				return diffArgs(fixture, filepath.Join(t.TempDir(), "diff.json"))
			},
		},
		{
			name: "unknown allowed pack change",
			command: func(t *testing.T, fixture commandFixture) []string {
				hypothesis := validHypothesis()
				hypothesis.AllowedPackChanges = []string{"unknown"}
				writeJSONFile(t, fixture.hypothesis, hypothesis)
				return diffArgs(fixture, filepath.Join(t.TempDir(), "diff.json"))
			},
		},
		{
			name: "incomplete protected pack fields",
			command: func(t *testing.T, fixture commandFixture) []string {
				hypothesis := validHypothesis()
				hypothesis.ProtectedPackFields = []string{"endpoint"}
				writeJSONFile(t, fixture.hypothesis, hypothesis)
				return diffArgs(fixture, filepath.Join(t.TempDir(), "diff.json"))
			},
		},
		{
			name: "gate contract absent from hypothesis targets",
			command: func(t *testing.T, fixture commandFixture) []string {
				hypothesis := validHypothesis()
				hypothesis.TargetCases = []string{"other-case"}
				writeJSONFile(t, fixture.hypothesis, hypothesis)
				return gateArgs(fixture, filepath.Join(t.TempDir(), "gate.json"))
			},
		},
		{
			name: "Pack Diff unknown field",
			command: func(t *testing.T, fixture commandFixture) []string {
				writeTextFile(t, fixture.packDiff, `{"unexpected":true}`)
				return gateArgs(fixture, filepath.Join(t.TempDir(), "gate.json"))
			},
		},
		{
			name: "runs unknown field",
			command: func(t *testing.T, fixture commandFixture) []string {
				writeTextFile(t, fixture.goldenRuns, `[{"unexpected":true}]`)
				return gateArgs(fixture, filepath.Join(t.TempDir(), "gate.json"))
			},
		},
		{
			name: "runs trailing JSON",
			command: func(t *testing.T, fixture commandFixture) []string {
				writeTextFile(t, fixture.goldenRuns, string(mustJSON(t, reviewedRuns("golden", "fail")))+" []")
				return gateArgs(fixture, filepath.Join(t.TempDir(), "gate.json"))
			},
		},
		{
			name: "missing output parent",
			command: func(t *testing.T, fixture commandFixture) []string {
				return []string{
					"diff-pack",
					"--golden-pack", fixture.goldenPack,
					"--candidate-pack", fixture.candidatePack,
					"--hypothesis", fixture.hypothesis,
					"--output", filepath.Join(t.TempDir(), "missing", "diff.json"),
				}
			},
		},
		{
			name: "existing output",
			command: func(t *testing.T, fixture commandFixture) []string {
				output := filepath.Join(t.TempDir(), "gate.json")
				writeTextFile(t, output, "sentinel")
				return gateArgs(fixture, output)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCommandFixture(t)
			args := test.command(t, fixture)

			code, stdout, stderr := invoke(args...)

			if code != 2 {
				t.Fatalf("run(%q) exit = %d, want 2", args, code)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr == "" {
				t.Fatal("stderr is empty, want diagnostic")
			}
		})
	}
}

func TestRunDoesNotOverwriteExistingOutput(t *testing.T) {
	fixture := newCommandFixture(t)
	output := filepath.Join(t.TempDir(), "gate.json")
	writeTextFile(t, output, "sentinel")

	code, _, _ := invoke(gateArgs(fixture, output)...)

	if code != 2 {
		t.Fatalf("run exit = %d, want 2", code)
	}
	if got := readTextFile(t, output); got != "sentinel" {
		t.Fatalf("existing output = %q, want sentinel", got)
	}
}

func TestWriteNewJSONLeavesNoArtifactOnMarshalFailure(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "report.json")

	err := writeNewJSON(output, make(chan int))

	if err == nil {
		t.Fatal("writeNewJSON error = nil, want marshal error")
	}
	requireDirectoryEntries(t, directory)
}

func TestWriteNewJSONCleansTemporaryFileOnStageFailure(t *testing.T) {
	tests := []struct {
		name   string
		inject func(*os.File) temporaryFile
	}{
		{
			name: "write",
			inject: func(file *os.File) temporaryFile {
				return &injectedTemporaryFile{File: file, writeErr: errors.New("injected write failure")}
			},
		},
		{
			name: "sync",
			inject: func(file *os.File) temporaryFile {
				return &injectedTemporaryFile{File: file, syncErr: errors.New("injected sync failure")}
			},
		},
		{
			name: "close",
			inject: func(file *os.File) temporaryFile {
				return &injectedTemporaryFile{File: file, closeErr: errors.New("injected close failure")}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			output := filepath.Join(directory, "report.json")
			operations := realFileOutput()
			operations.createTemp = func(directory, pattern string) (temporaryFile, error) {
				file, err := os.CreateTemp(directory, pattern)
				if err != nil {
					return nil, err
				}
				return test.inject(file), nil
			}

			err := writeNewJSONWithOutput(
				output,
				agentbench.GateReport{Passed: true, Failures: []string{}},
				operations,
			)

			if err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("writeNewJSONWithOutput error = %v, want injected %s failure", err, test.name)
			}
			requireDirectoryEntries(t, directory)
		})
	}
}

func TestWriteNewJSONCleansTemporaryFileOnPublishFailure(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "report.json")
	operations := realFileOutput()
	operations.link = func(_, _ string) error {
		return errors.New("injected publish failure")
	}

	err := writeNewJSONWithOutput(
		output,
		agentbench.GateReport{Passed: true, Failures: []string{}},
		operations,
	)

	if err == nil || !strings.Contains(err.Error(), "injected publish failure") {
		t.Fatalf("writeNewJSONWithOutput error = %v, want injected publish failure", err)
	}
	requireDirectoryEntries(t, directory)
}

func TestWriteNewJSONPreservesDestinationCreatedDuringPublish(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "report.json")
	operations := realFileOutput()
	operations.link = func(_, destination string) error {
		if err := os.WriteFile(destination, []byte("sentinel"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", destination, err)
		}
		return os.ErrExist
	}

	err := writeNewJSONWithOutput(
		output,
		agentbench.GateReport{Passed: true, Failures: []string{}},
		operations,
	)

	if err == nil {
		t.Fatal("writeNewJSONWithOutput error = nil, want publish collision")
	}
	if got := readTextFile(t, output); got != "sentinel" {
		t.Fatalf("existing output = %q, want sentinel", got)
	}
	requireDirectoryEntries(t, directory, "report.json")
}

func TestWriteNewJSONReportsCleanupFailureAfterWriteFailure(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "report.json")
	operations := realFileOutput()
	operations.createTemp = func(directory, pattern string) (temporaryFile, error) {
		file, err := os.CreateTemp(directory, pattern)
		if err != nil {
			return nil, err
		}
		return &injectedTemporaryFile{
			File:     file,
			writeErr: errors.New("injected write failure"),
		}, nil
	}
	operations.remove = func(string) error {
		return errors.New("injected cleanup failure")
	}

	err := writeNewJSONWithOutput(
		output,
		agentbench.GateReport{Passed: true, Failures: []string{}},
		operations,
	)

	if err == nil ||
		!strings.Contains(err.Error(), "injected write failure") ||
		!strings.Contains(err.Error(), "injected cleanup failure") {
		t.Fatalf("writeNewJSONWithOutput error = %v, want write and cleanup failures", err)
	}
}

func TestWriteNewJSONReportsCleanupFailureAfterPublication(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "report.json")
	operations := realFileOutput()
	operations.remove = func(string) error {
		return errors.New("injected cleanup failure")
	}

	err := writeNewJSONWithOutput(
		output,
		agentbench.GateReport{Passed: true, Failures: []string{}},
		operations,
	)

	if err == nil || !strings.Contains(err.Error(), "injected cleanup failure") {
		t.Fatalf("writeNewJSONWithOutput error = %v, want cleanup failure", err)
	}
	var report agentbench.GateReport
	decodeJSON(t, readTextFile(t, output), &report)
	if !report.Passed {
		t.Fatalf("published report = %#v, want passing report", report)
	}
}

func TestRunProducesStableOutputForReorderedInputs(t *testing.T) {
	t.Run("diff pack", func(t *testing.T) {
		fixture := newCommandFixture(t)
		firstGolden := validPack()
		firstGolden.Tests = []agent.ContextLocation{
			{ID: "test-b", Kind: "test", Label: "B"},
			{ID: "test-a", Kind: "test", Label: "A"},
		}
		firstCandidate := firstGolden
		firstCandidate.Tests = append([]agent.ContextLocation(nil), firstGolden.Tests...)
		writeJSONFile(t, fixture.goldenPack, firstGolden)
		writeJSONFile(t, fixture.candidatePack, firstCandidate)
		firstOutput := filepath.Join(t.TempDir(), "first.json")
		if code, _, stderr := invoke(diffArgs(fixture, firstOutput)...); code != 0 {
			t.Fatalf("first run exit = %d, want 0; stderr = %q", code, stderr)
		}

		secondGolden := firstGolden
		secondGolden.Tests[0], secondGolden.Tests[1] = secondGolden.Tests[1], secondGolden.Tests[0]
		secondCandidate := secondGolden
		secondCandidate.Tests = append([]agent.ContextLocation(nil), secondGolden.Tests...)
		writeJSONFile(t, fixture.goldenPack, secondGolden)
		writeJSONFile(t, fixture.candidatePack, secondCandidate)
		secondOutput := filepath.Join(t.TempDir(), "second.json")
		if code, _, stderr := invoke(diffArgs(fixture, secondOutput)...); code != 0 {
			t.Fatalf("second run exit = %d, want 0; stderr = %q", code, stderr)
		}

		if first, second := readTextFile(t, firstOutput), readTextFile(t, secondOutput); first != second {
			t.Fatalf("reordered diff output differs:\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})

	t.Run("gate", func(t *testing.T) {
		fixture := newCommandFixture(t)
		firstOutput := filepath.Join(t.TempDir(), "first.json")
		if code, _, stderr := invoke(gateArgs(fixture, firstOutput)...); code != 0 {
			t.Fatalf("first run exit = %d, want 0; stderr = %q", code, stderr)
		}

		golden := reviewedRuns("golden", "fail")
		candidate := reviewedRuns("candidate", "pass")
		reverseRuns(golden)
		reverseRuns(candidate)
		writeJSONFile(t, fixture.goldenRuns, golden)
		writeJSONFile(t, fixture.candidateRuns, candidate)
		secondOutput := filepath.Join(t.TempDir(), "second.json")
		if code, _, stderr := invoke(gateArgs(fixture, secondOutput)...); code != 0 {
			t.Fatalf("second run exit = %d, want 0; stderr = %q", code, stderr)
		}

		if first, second := readTextFile(t, firstOutput), readTextFile(t, secondOutput); first != second {
			t.Fatalf("reordered gate output differs:\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})
}

type commandFixture struct {
	contract      string
	goldenPack    string
	candidatePack string
	hypothesis    string
	packDiff      string
	goldenRuns    string
	candidateRuns string
}

func runnerCommandArguments(root string) []string {
	return []string{
		"--matrix", filepath.Join(root, "matrix.json"),
		"--external-case", filepath.Join(root, "g1"),
		"--golden-binary", filepath.Join(root, "golden"),
		"--golden-commit", strings.Repeat("a", 40),
		"--candidate-binary", filepath.Join(root, "candidate"),
		"--candidate-commit", strings.Repeat("b", 40),
		"--instruction", filepath.Join(root, "instruction.txt"),
		"--phase", "smoke",
		"--target-case", "g1",
		"--runs", "1",
		"--output", filepath.Join(root, "output"),
	}
}

func safeCommandCodexArgs() []string {
	return []string{
		"-a", "never",
		"exec",
		"--sandbox", "read-only",
		"--skip-git-repo-check",
		"--ephemeral",
		"--ignore-user-config",
		"--ignore-rules",
		"--color", "never",
		"-m", "test-model",
		"-c", `model_reasoning_effort="high"`,
	}
}

func newCommandFixture(t *testing.T) commandFixture {
	t.Helper()
	directory := t.TempDir()
	fixture := commandFixture{
		contract:      filepath.Join(directory, "contract.json"),
		goldenPack:    filepath.Join(directory, "golden-pack.json"),
		candidatePack: filepath.Join(directory, "candidate-pack.json"),
		hypothesis:    filepath.Join(directory, "hypothesis.json"),
		packDiff:      filepath.Join(directory, "pack-diff.json"),
		goldenRuns:    filepath.Join(directory, "golden-runs.json"),
		candidateRuns: filepath.Join(directory, "candidate-runs.json"),
	}
	writeJSONFile(t, fixture.contract, validContract())
	writeJSONFile(t, fixture.goldenPack, validPack())
	writeJSONFile(t, fixture.candidatePack, validPack())
	writeJSONFile(t, fixture.hypothesis, validHypothesis())
	writeJSONFile(t, fixture.packDiff, agentbench.PackDiff{
		AddedSources: []string{"services/catalog/NewEvidence.java"},
	})
	writeJSONFile(t, fixture.goldenRuns, reviewedRuns("golden", "fail"))
	writeJSONFile(t, fixture.candidateRuns, reviewedRuns("candidate", "pass"))
	return fixture
}

func validContract() agentbench.Contract {
	fallback := false
	retry := false
	return agentbench.Contract{
		Schema: 1,
		ID:     "case",
		Queries: []agentbench.QueryVariant{{
			ID: "en", Language: "en", File: "query.en.txt", EndToEnd: true,
		}},
		Pack: agentbench.PackExpectation{
			Endpoint: &agentbench.EndpointExpectation{
				HTTPMethod: "DELETE",
				Path:       "/catalog/{itemId}",
			},
			RequiredLocations: []agentbench.LocationExpectation{{
				Section: "persistence", LabelContains: "deleteById",
			}},
			FallbackRequired:     &fallback,
			RetryAllowed:         &retry,
			MaxEstimatedTokens:   4000,
			MaxSourceOmissions:   3,
			RequireBoundedSource: true,
		},
		Answer: agentbench.AnswerExpectation{
			RequiredFacets: []agentbench.FacetDefinition{
				{ID: "current-path", Description: "Explains the current path."},
				{ID: "target", Description: "Explains the target behavior."},
			},
			ForbiddenOutcomes: []agentbench.FacetDefinition{{
				ID: "invented-edge", Description: "Does not invent an edge.",
			}},
		},
		Limits: agentbench.EfficiencyLimits{
			ContextTokens:              4000,
			MaxSourceOmissions:         3,
			MaxTokenIncreasePercent:    5,
			MaxLatencyIncreasePercent:  10,
			MaxPairedLatencyMultiplier: 2,
		},
	}
}

func validPack() agent.ContextPack {
	return agent.ContextPack{
		Schema:     1,
		Query:      "delete catalog item",
		Confidence: "high",
		Persistence: []agent.ContextLocation{{
			ID: "persistence", Kind: "method", Label: "deleteById",
		}},
		Endpoints: []agent.ContextEndpoint{{
			Provider: "catalog", HTTPMethod: "DELETE", Path: "/catalog/{itemId}",
			Security: "authenticated",
		}},
		Files: []agent.ContextFile{{
			Path: "CatalogRepository.java", StartLine: 1, EndLine: 10,
			Role: "persistence", Reason: "Defines deleteById.",
		}},
		SourceCoverage:  "complete",
		EstimatedTokens: 100,
		BudgetTokens:    4000,
	}
}

func validHypothesis() agentbench.Hypothesis {
	return agentbench.Hypothesis{
		Schema:              1,
		ID:                  "hypothesis",
		TargetFacet:         "target",
		TargetCases:         []string{"case"},
		AllowedPackChanges:  []string{"sources"},
		ProtectedPackFields: []string{"endpoint", "entrypoints", "call_chain", "contracts", "persistence"},
	}
}

func reviewedRuns(build, targetStatus string) []agentbench.ReviewedRun {
	runs := make([]agentbench.ReviewedRun, 3)
	for index := range runs {
		runs[index] = agentbench.ReviewedRun{
			Review: agentbench.RunReview{
				Schema:     1,
				CaseID:     "case",
				Build:      build,
				Run:        index + 1,
				Attempt:    1,
				Reviewer:   "reviewer",
				ReviewedAt: "2026-07-25T10:00:00Z",
				Signature:  "signed",
				Facets: map[string]agentbench.FacetResult{
					"current-path": {Status: "pass", Evidence: "Current path is covered."},
					"target":       {Status: targetStatus, Evidence: "Target behavior is covered."},
				},
				ForbiddenOutcomes: map[string]agentbench.FacetResult{
					"invented-edge": {Status: "pass", Evidence: "No edge was invented."},
				},
			},
			Metrics: agentbench.RunMetrics{
				Tokens: 100, ToolCalls: 10, SourceReads: 10, ContextMillis: 100,
			},
		}
	}
	return runs
}

func gateArgs(fixture commandFixture, output string) []string {
	return []string{
		"gate",
		"--contract", fixture.contract,
		"--hypothesis", fixture.hypothesis,
		"--pack-diff", fixture.packDiff,
		"--golden-runs", fixture.goldenRuns,
		"--candidate-runs", fixture.candidateRuns,
		"--output", output,
	}
}

func diffArgs(fixture commandFixture, output string) []string {
	return []string{
		"diff-pack",
		"--golden-pack", fixture.goldenPack,
		"--candidate-pack", fixture.candidatePack,
		"--hypothesis", fixture.hypothesis,
		"--output", output,
	}
}

func invoke(args ...string) (int, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	writeTextFile(t, path, string(mustJSON(t, value)))
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return body
}

func writeTextFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", path, err)
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}
	return string(body)
}

func decodeJSON(t *testing.T, body string, destination any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), destination); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", body, err)
	}
}

func requireStableJSON(t *testing.T, body string) {
	t.Helper()
	if !json.Valid([]byte(body)) {
		t.Fatalf("body is not valid JSON: %q", body)
	}
	if !strings.HasSuffix(body, "\n") || strings.HasSuffix(body, "\n\n") {
		t.Fatalf("body must end in exactly one newline: %q", body)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(body)); err != nil {
		t.Fatalf("json.Compact: %v", err)
	}
	var indented bytes.Buffer
	err := json.Indent(&indented, compact.Bytes(), "", "  ")
	if err != nil {
		t.Fatalf("json.Indent: %v", err)
	}
	if want := indented.String() + "\n"; body != want {
		t.Fatalf("body is not stable indented JSON:\ngot:\n%s\nwant:\n%s", body, want)
	}
}

func reverseRuns(runs []agentbench.ReviewedRun) {
	for left, right := 0, len(runs)-1; left < right; left, right = left+1, right-1 {
		runs[left], runs[right] = runs[right], runs[left]
	}
}

type shortWriter struct{}

func (shortWriter) Write(body []byte) (int, error) {
	return len(body) - 1, nil
}

type injectedTemporaryFile struct {
	*os.File
	writeErr error
	syncErr  error
	closeErr error
}

func (file *injectedTemporaryFile) Write(body []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return file.File.Write(body)
}

func (file *injectedTemporaryFile) Sync() error {
	if file.syncErr != nil {
		return file.syncErr
	}
	return file.File.Sync()
}

func (file *injectedTemporaryFile) Close() error {
	closeErr := file.File.Close()
	if closeErr != nil {
		return closeErr
	}
	return file.closeErr
}

func requireDirectoryEntries(t *testing.T, directory string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("os.ReadDir(%q): %v", directory, err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	if len(got) != len(want) {
		t.Fatalf("directory entries = %q, want %q", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("directory entries = %q, want %q", got, want)
		}
	}
}

func TestValidCommandFixture(t *testing.T) {
	contract := validContract()
	if err := agentbench.ValidateContract(contract); err != nil {
		t.Fatalf("validContract: %v", err)
	}
	if got, want := validHypothesis().TargetFacet, "target"; !reflect.DeepEqual(got, want) {
		t.Fatalf("target facet = %q, want %q", got, want)
	}
}
