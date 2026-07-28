package agentbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestDecodeContextPackAcceptsProductionSchema(t *testing.T) {
	body := []byte(fmt.Sprintf(
		`{"schema":%d,"query":"production","confidence":"high","fallback_required":false,"estimated_tokens":10,"budget_tokens":4000,"retry_allowed":false}`,
		scan.SchemaVersion,
	))

	pack, err := decodeContextPack(body)

	if err != nil {
		t.Fatalf("decodeContextPack: %v", err)
	}
	if pack.Schema != scan.SchemaVersion {
		t.Fatalf("schema = %d, want %d", pack.Schema, scan.SchemaVersion)
	}
}

func TestRunRegressionSmokeOrderAndIsolation(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g2-java-missing-contract", 1)
	parentPath := os.Getenv("PATH")

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	wantOrder := []string{
		"g1\tgolden\t1",
		"g1\tcandidate\t1",
		"g2-java-missing-contract\tgolden\t1",
		"g2-java-missing-contract\tcandidate\t1",
	}
	if got := codexOrder(t, fixture.log); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("Codex order = %q, want %q", got, wantOrder)
	}
	if got := endToEndContextOrder(t, fixture.log); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("Context order = %q, want %q", got, wantOrder)
	}
	assertIsolatedExecutions(t, fixture.log, 4, fixture.config.Output)
	if got := os.Getenv("PATH"); got != parentPath {
		t.Fatalf("parent PATH changed to %q, want %q", got, parentPath)
	}
	assertNoRetainedExecutionSources(t, fixture.config.Output)
	assertTemporaryExecutionRootRemoved(t, fixture.config.Output)
}

func TestRunRegressionInterleavesPreparationContextAndCodex(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g1", 1)

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	if got, want := operationOrder(t, fixture.log), []string{
		"scan\tgolden",
		"context\tgolden",
		"codex\tgolden",
		"scan\tcandidate",
		"context\tcandidate",
		"codex\tcandidate",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("operation order = %q, want %q", got, want)
	}
}

func TestRunRegressionFreezesBinariesAndSourceSnapshot(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g1", 1)
	sourceWorkspace := filepath.Join(fixture.config.ExternalCase, "workspace")
	expectedSnapshot := filepath.Join(t.TempDir(), "workspace")
	if err := copyWorkspace(
		context.Background(),
		sourceWorkspace,
		expectedSnapshot,
	); err != nil {
		t.Fatalf("copyWorkspace source: %v", err)
	}
	expectedHash, err := hashTree(context.Background(), expectedSnapshot)
	if err != nil {
		t.Fatalf("hashTree source: %v", err)
	}
	t.Setenv(
		"FAKE_MUTATE_SOURCE",
		filepath.Join(sourceWorkspace, "src", "source.txt"),
	)
	t.Setenv("FAKE_MUTATE_MARKER", filepath.Join(t.TempDir(), "mutated"))

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	if _, err := os.Lstat(
		filepath.Join(fixture.config.Output, "inputs", "cases"),
	); !os.IsNotExist(err) {
		t.Fatalf("private source snapshots retained in final output: %v", err)
	}
	for _, build := range []string{"golden", "candidate"} {
		hash := strings.TrimSpace(readRunnerText(t, filepath.Join(
			fixture.config.Output,
			"cases", "g1", "english", build+"-workspace.sha256",
		)))
		if hash != expectedHash {
			t.Fatalf("%s source hash = %q, want frozen hash %q", build, hash, expectedHash)
		}
		runtimeHash, err := hashFile(
			context.Background(),
			filepath.Join(
				fixture.config.Output, "runtime", build, "goregraph",
			),
		)
		if err != nil {
			t.Fatalf("hashFile %s runtime: %v", build, err)
		}
		identityHash := strings.TrimSpace(readRunnerText(t, filepath.Join(
			fixture.config.Output, "identity", build+"-binary.sha256",
		)))
		if identityHash != runtimeHash {
			t.Fatalf(
				"%s identity hash = %q, want frozen runtime hash %q",
				build,
				identityHash,
				runtimeHash,
			)
		}
	}
	for _, line := range readLogLines(t, fixture.log) {
		fields := strings.Split(line, "\t")
		if len(fields) < 4 || (fields[0] != "scan" && fields[0] != "context") {
			continue
		}
		build := fields[1]
		wantExecutable := filepath.Join(
			fixture.config.Output, "runtime", build, "goregraph",
		)
		if fields[3] != wantExecutable {
			t.Fatalf("%s %s executable = %q, want %q", fields[0], build, fields[3], wantExecutable)
		}
	}
	matches, err := filepath.Glob(filepath.Join(
		filepath.Dir(fixture.config.Output),
		".goregraph-agentbench-snapshot-*",
	))
	if err != nil {
		t.Fatalf("filepath.Glob source snapshots: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary source snapshots retained: %v", matches)
	}
}

func TestRunRegressionKeepsSnapshotOutsideInputWorkspace(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g1", 1)
	t.Setenv(
		"TMPDIR",
		filepath.Join(
			fixture.config.ExternalCase,
			"workspace",
			"missing",
		),
	)

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression with untrusted TMPDIR: %v", err)
	}
}

func TestRunRegressionSmokeDeduplicatesG1(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g1", 1)

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	if got, want := codexOrder(t, fixture.log), []string{
		"g1\tgolden\t1",
		"g1\tcandidate\t1",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Codex order = %q, want %q", got, want)
	}
}

func TestRunRegressionFullOrderAndArtifacts(t *testing.T) {
	fixture := newRunnerFixture(t, "full", "g4-typescript-consumer-provider", 3)

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	order := codexOrder(t, fixture.log)
	if len(order) != 36 {
		t.Fatalf("Codex executions = %d, want 36: %q", len(order), order)
	}
	wantPerCase := []string{
		"golden\t1",
		"candidate\t1",
		"candidate\t2",
		"golden\t2",
		"golden\t3",
		"candidate\t3",
	}
	for index, caseID := range runnerCaseIDs {
		start := index * len(wantPerCase)
		for offset, suffix := range wantPerCase {
			if got, want := order[start+offset], caseID+"\t"+suffix; got != want {
				t.Fatalf("Codex order[%d] = %q, want %q", start+offset, got, want)
			}
		}
	}
	assertIsolatedExecutions(t, fixture.log, 36, fixture.config.Output)
	assertRequiredArtifacts(t, fixture)
	for _, build := range []string{"golden", "candidate"} {
		requireFile(t, filepath.Join(
			fixture.config.Output,
			"cases", "g2-java-missing-contract", "german", build+"-pack.json",
		))
	}
	requireFile(t, filepath.Join(
		fixture.config.Output,
		"cases", "g2-java-missing-contract", "german", "pack-diff.json",
	))
}

func TestRunRegressionRejectsInvalidInputBeforeCodex(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, fixture *runnerFixture)
	}{
		{
			name: "unknown phase",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.Phase = "other"
			},
		},
		{
			name: "smoke requires one run",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.Runs = 3
			},
		},
		{
			name: "full requires three gateable runs",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.Phase = "full"
				fixture.config.Runs = 5
			},
		},
		{
			name: "uppercase commit",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.GoldenCommit = strings.Repeat("A", 40)
			},
		},
		{
			name: "short commit",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.CandidateCommit = "abc"
			},
		},
		{
			name: "missing matrix case",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				matrix := validRunnerMatrix()
				matrix.Cases = matrix.Cases[:4]
				writeRunnerJSON(t, fixture.config.MatrixPath, matrix)
			},
		},
		{
			name: "unexpected matrix case",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				matrix := validRunnerMatrix()
				matrix.Cases[0].ID = "g7"
				writeRunnerJSON(t, fixture.config.MatrixPath, matrix)
			},
		},
		{
			name: "external committed case",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				matrix := validRunnerMatrix()
				matrix.Cases[0].External = true
				writeRunnerJSON(t, fixture.config.MatrixPath, matrix)
			},
		},
		{
			name: "matrix directory traversal",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				matrix := validRunnerMatrix()
				matrix.Cases[0].Directory = "../escape"
				writeRunnerJSON(t, fixture.config.MatrixPath, matrix)
			},
		},
		{
			name: "matrix directory separator",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				matrix := validRunnerMatrix()
				source := filepath.Join(filepath.Dir(fixture.config.MatrixPath), matrix.Cases[0].Directory)
				matrix.Cases[0].Directory = "nested/" + matrix.Cases[0].Directory
				destination := filepath.Join(
					filepath.Dir(fixture.config.MatrixPath),
					filepath.FromSlash(matrix.Cases[0].Directory),
				)
				if err := os.Mkdir(filepath.Dir(destination), 0o700); err != nil {
					t.Fatalf("os.Mkdir nested: %v", err)
				}
				if err := os.Rename(source, destination); err != nil {
					t.Fatalf("os.Rename nested case: %v", err)
				}
				writeRunnerJSON(t, fixture.config.MatrixPath, matrix)
			},
		},
		{
			name: "query traversal",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				path := filepath.Join(
					filepath.Dir(fixture.config.MatrixPath),
					"g2-java-missing-contract",
					"contract.json",
				)
				contract := runnerContract("g2-java-missing-contract", false)
				contract.Queries[0].File = "../query.txt"
				writeRunnerJSON(t, path, contract)
			},
		},
		{
			name: "query file separator",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				caseRoot := filepath.Join(
					filepath.Dir(fixture.config.MatrixPath),
					"g2-java-missing-contract",
				)
				if err := os.Mkdir(filepath.Join(caseRoot, "queries"), 0o700); err != nil {
					t.Fatalf("os.Mkdir queries: %v", err)
				}
				if err := os.Rename(
					filepath.Join(caseRoot, "query.en.txt"),
					filepath.Join(caseRoot, "queries", "query.en.txt"),
				); err != nil {
					t.Fatalf("os.Rename query: %v", err)
				}
				contract := runnerContract("g2-java-missing-contract", false)
				contract.Queries[0].File = "queries/query.en.txt"
				writeRunnerJSON(t, filepath.Join(caseRoot, "contract.json"), contract)
			},
		},
		{
			name: "external case root symlink",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				link := filepath.Join(filepath.Dir(fixture.config.ExternalCase), "g1-link")
				if err := os.Symlink(fixture.config.ExternalCase, link); err != nil {
					t.Fatalf("os.Symlink external case: %v", err)
				}
				fixture.config.ExternalCase = link
			},
		},
		{
			name: "contract symlink",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				caseRoot := filepath.Join(
					filepath.Dir(fixture.config.MatrixPath),
					"g2-java-missing-contract",
				)
				contractPath := filepath.Join(caseRoot, "contract.json")
				externalContract := filepath.Join(t.TempDir(), "contract.json")
				writeRunnerJSON(
					t,
					externalContract,
					runnerContract("g2-java-missing-contract", false),
				)
				if err := os.Remove(contractPath); err != nil {
					t.Fatalf("os.Remove contract: %v", err)
				}
				if err := os.Symlink(externalContract, contractPath); err != nil {
					t.Fatalf("os.Symlink contract: %v", err)
				}
			},
		},
		{
			name: "workspace symlink",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				workspace := filepath.Join(fixture.config.ExternalCase, "workspace")
				if err := os.Symlink("source.txt", filepath.Join(workspace, "link.txt")); err != nil {
					t.Fatalf("os.Symlink: %v", err)
				}
			},
		},
		{
			name: "instruction is directory",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				fixture.config.InstructionPath = t.TempDir()
			},
		},
		{
			name: "matrix is directory",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				fixture.config.MatrixPath = t.TempDir()
			},
		},
		{
			name: "binary is directory",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				fixture.config.GoldenBinary = t.TempDir()
			},
		},
		{
			name: "same golden and candidate binary",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.CandidateBinary = fixture.config.GoldenBinary
			},
		},
		{
			name: "existing output",
			mutate: func(t *testing.T, fixture *runnerFixture) {
				if err := os.Mkdir(fixture.config.Output, 0o700); err != nil {
					t.Fatalf("os.Mkdir: %v", err)
				}
			},
		},
		{
			name: "output inside input workspace",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.Output = filepath.Join(fixture.config.ExternalCase, "workspace", "output")
			},
		},
		{
			name: "unknown target",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.TargetCase = "g7"
			},
		},
		{
			name: "unsafe Codex args",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.CodexArgs = append(fixture.config.CodexArgs, "--json")
			},
		},
		{
			name: "positional Codex arg",
			mutate: func(_ *testing.T, fixture *runnerFixture) {
				fixture.config.CodexArgs = append(fixture.config.CodexArgs, "prompt")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRunnerFixture(t, "smoke", "g2-java-missing-contract", 1)
			test.mutate(t, &fixture)

			err := RunRegression(context.Background(), fixture.config)

			if err == nil {
				t.Fatal("RunRegression error = nil, want validation failure")
			}
			if entries := readLogLines(t, fixture.log); len(entries) != 0 {
				t.Fatalf("process log = %q, want no Codex/scanner execution", entries)
			}
		})
	}
}

func TestValidateRunnerRejectsCanonicalOutputInsideWorkspace(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g1", 1)
	alias := filepath.Join(filepath.Dir(fixture.config.ExternalCase), "workspace-alias")
	if err := os.Symlink(
		filepath.Join(fixture.config.ExternalCase, "workspace"),
		alias,
	); err != nil {
		t.Fatalf("os.Symlink workspace alias: %v", err)
	}
	fixture.config.Output = filepath.Join(alias, "output")

	_, err := validateRunner(fixture.config)

	if err == nil {
		t.Fatal("validateRunner error = nil, want canonical output containment failure")
	}
}

func TestRunRegressionRetainsCodexFailureAndStops(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g2-java-missing-contract", 1)
	t.Setenv("FAKE_CODEX_FAIL_AT", "2")

	err := RunRegression(context.Background(), fixture.config)

	if err == nil || !strings.Contains(err.Error(), "Codex") {
		t.Fatalf("RunRegression error = %v, want Codex failure", err)
	}
	if got := len(codexOrder(t, fixture.log)); got != 2 {
		t.Fatalf("Codex executions = %d, want 2", got)
	}
	requireOperationCounts(t, fixture.log, 2, 2, 2)
	runRoot := filepath.Join(
		fixture.config.Output, "runs", "g1", "english",
	)
	requireFile(t, filepath.Join(runRoot, "candidate-1-1.jsonl"))
	requireFile(t, filepath.Join(runRoot, "candidate-1-1.stderr"))
	requireFile(t, filepath.Join(runRoot, "candidate-1-1.metrics.tsv"))
	reviewPath := filepath.Join(
		fixture.config.Output, "reviews", "g1", "english", "candidate-1-1.json",
	)
	var reviewed ReviewedRun
	readRunnerJSON(t, reviewPath, &reviewed)
	if reviewed.Invalid == nil ||
		!reviewed.Invalid.InfrastructureFailure ||
		reviewed.Invalid.Reason == "" ||
		reviewed.Invalid.RetainedLog == "" {
		t.Fatalf("invalid record = %#v, want retained infrastructure failure", reviewed.Invalid)
	}
	assertRetainedLogPath(t, fixture.config.Output, reviewed.Invalid.RetainedLog)
	assertNoRetainedExecutionSources(t, fixture.config.Output)
	assertTemporaryExecutionRootRemoved(t, fixture.config.Output)
}

func TestRunRegressionRetainsProcessFailureAndStops(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantScan    int
		wantContext int
		wantCodex   int
	}{
		{
			name: "scan", environment: "FAKE_SCAN_FAIL",
			wantScan: 1, wantContext: 0, wantCodex: 0,
		},
		{
			name: "context", environment: "FAKE_CONTEXT_FAIL",
			wantScan: 1, wantContext: 1, wantCodex: 0,
		},
		{
			name: "analyzer", environment: "FAKE_ANALYZER_FAIL",
			wantScan: 1, wantContext: 1, wantCodex: 1,
		},
		{
			name: "scan missing index", environment: "FAKE_SCAN_NO_INDEX",
			wantScan: 1, wantContext: 0, wantCodex: 0,
		},
		{
			name: "context malformed output", environment: "FAKE_CONTEXT_MALFORMED",
			wantScan: 1, wantContext: 1, wantCodex: 0,
		},
		{
			name: "context unsupported schema", environment: "FAKE_CONTEXT_BAD_SCHEMA",
			wantScan: 1, wantContext: 1, wantCodex: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRunnerFixture(t, "smoke", "g1", 1)
			t.Setenv(test.environment, "1")

			err := RunRegression(context.Background(), fixture.config)

			if err == nil {
				t.Fatal("RunRegression error = nil, want infrastructure failure")
			}
			if got := len(codexOrder(t, fixture.log)); got != test.wantCodex {
				t.Fatalf("Codex executions = %d, want %d", got, test.wantCodex)
			}
			requireOperationCounts(
				t,
				fixture.log,
				test.wantScan,
				test.wantContext,
				test.wantCodex,
			)
			runRoot := filepath.Join(fixture.config.Output, "runs", "g1", "english")
			for _, suffix := range []string{".jsonl", ".stderr", ".metrics.tsv"} {
				requireFile(t, filepath.Join(runRoot, "golden-1-1"+suffix))
			}
			reviewPath := filepath.Join(
				fixture.config.Output,
				"reviews", "g1", "english", "golden-1-1.json",
			)
			var reviewed ReviewedRun
			readRunnerJSON(t, reviewPath, &reviewed)
			if reviewed.Invalid == nil || !reviewed.Invalid.InfrastructureFailure {
				t.Fatalf("invalid record = %#v, want infrastructure failure", reviewed.Invalid)
			}
			assertRetainedLogPath(t, fixture.config.Output, reviewed.Invalid.RetainedLog)
			assertNoRetainedExecutionSources(t, fixture.config.Output)
			assertTemporaryExecutionRootRemoved(t, fixture.config.Output)
		})
	}
}

func TestRunRegressionRetainsIndexDriftAndStops(t *testing.T) {
	fixture := newRunnerFixture(t, "full", "g1", 3)
	t.Setenv("FAKE_INDEX_CHANGE_AT", "3")

	err := RunRegression(context.Background(), fixture.config)

	if err == nil || !strings.Contains(err.Error(), "index hash changed") {
		t.Fatalf("RunRegression error = %v, want index drift", err)
	}
	requireOperationCounts(t, fixture.log, 3, 2, 2)
	runRoot := filepath.Join(fixture.config.Output, "runs", "g1", "english")
	for _, suffix := range []string{".jsonl", ".stderr", ".metrics.tsv"} {
		requireFile(t, filepath.Join(runRoot, "candidate-2-1"+suffix))
	}
	var reviewed ReviewedRun
	readRunnerJSON(
		t,
		filepath.Join(
			fixture.config.Output,
			"reviews", "g1", "english", "candidate-2-1.json",
		),
		&reviewed,
	)
	if reviewed.Invalid == nil || !reviewed.Invalid.InfrastructureFailure {
		t.Fatalf("invalid record = %#v, want index infrastructure failure", reviewed.Invalid)
	}
	assertRetainedLogPath(t, fixture.config.Output, reviewed.Invalid.RetainedLog)
	assertNoRetainedExecutionSources(t, fixture.config.Output)
	assertTemporaryExecutionRootRemoved(t, fixture.config.Output)
}

func TestRunRegressionIgnoresVolatileContextIndexMetadata(t *testing.T) {
	fixture := newRunnerFixture(t, "full", "g1", 3)
	t.Setenv("FAKE_INDEX_VOLATILE", "1")

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	requireOperationCounts(t, fixture.log, 36, 42, 36)
}

func TestRunRegressionKeepsSuccessfulSemanticFailureValid(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g1", 1)
	t.Setenv("FAKE_CODEX_SEMANTIC_BAD", "1")

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	reviewPath := filepath.Join(
		fixture.config.Output, "reviews", "g1", "english", "golden-1-1.json",
	)
	var reviewed ReviewedRun
	readRunnerJSON(t, reviewPath, &reviewed)
	if reviewed.Invalid != nil {
		t.Fatalf("successful semantic failure invalid record = %#v, want nil", reviewed.Invalid)
	}
	for id, facet := range reviewed.Review.Facets {
		if facet.Status != "fail" || facet.Evidence != "Review required before gating." {
			t.Fatalf("facet %s = %#v, want review-required template", id, facet)
		}
	}
}

func TestRunRegressionMapsAnalyzerMetrics(t *testing.T) {
	fixture := newRunnerFixture(t, "smoke", "g1", 1)

	if err := RunRegression(context.Background(), fixture.config); err != nil {
		t.Fatalf("RunRegression: %v", err)
	}

	summaryLines := strings.Split(
		strings.TrimSpace(readRunnerText(
			t,
			filepath.Join(fixture.config.Output, "summary.tsv"),
		)),
		"\n",
	)
	if len(summaryLines) != 3 {
		t.Fatalf("summary lines = %d, want header plus 2 runs", len(summaryLines))
	}
	fields := strings.Split(summaryLines[1], "\t")
	if got, want := fields[5:14], []string{
		"101", "11", "12", "15", "16", "17", "18", "19", "20",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("summary metrics = %q, want %q", got, want)
	}
	var reviewed ReviewedRun
	readRunnerJSON(
		t,
		filepath.Join(
			fixture.config.Output,
			"reviews", "g1", "english", "golden-1-1.json",
		),
		&reviewed,
	)
	if got, want := reviewed.Metrics, (RunMetrics{
		Tokens:                  101,
		ToolCalls:               11,
		ContextCalls:            12,
		RepeatedFullPacks:       15,
		BroadNavigationCalls:    16,
		SourceReads:             17,
		BoundedOmissionReads:    18,
		UnauthorizedSourceReads: 19,
		IncludedSourceRereads:   20,
		ContextMillis:           reviewed.Metrics.ContextMillis,
	}); !reflect.DeepEqual(got, want) {
		t.Fatalf("review metrics = %#v, want %#v", got, want)
	}
}

func TestRunRegressionRetainsFirstPackAndRejectsProjectionDrift(t *testing.T) {
	t.Run("retains first canonical pack", func(t *testing.T) {
		fixture := newRunnerFixture(t, "full", "g1", 3)

		if err := RunRegression(context.Background(), fixture.config); err != nil {
			t.Fatalf("RunRegression: %v", err)
		}

		for build, wantContextID := range map[string]string{
			"golden": "call-1", "candidate": "call-2",
		} {
			var pack struct {
				ContextID string `json:"context_id"`
			}
			readRunnerJSON(
				t,
				filepath.Join(
					fixture.config.Output,
					"cases", "g1", "english", build+"-pack.json",
				),
				&pack,
			)
			if pack.ContextID != wantContextID {
				t.Fatalf(
					"%s retained Context ID = %q, want %q",
					build,
					pack.ContextID,
					wantContextID,
				)
			}
		}
	})

	t.Run("rejects later projection drift", func(t *testing.T) {
		fixture := newRunnerFixture(t, "full", "g1", 3)
		t.Setenv("FAKE_CONTEXT_CHANGE_AT", "3")

		err := RunRegression(context.Background(), fixture.config)

		if err == nil || !strings.Contains(err.Error(), "projection changed") {
			t.Fatalf("RunRegression error = %v, want projection drift", err)
		}
		requireOperationCounts(t, fixture.log, 3, 3, 2)
		runRoot := filepath.Join(fixture.config.Output, "runs", "g1", "english")
		for _, suffix := range []string{".jsonl", ".stderr", ".metrics.tsv"} {
			requireFile(t, filepath.Join(runRoot, "candidate-2-1"+suffix))
		}
		var reviewed ReviewedRun
		readRunnerJSON(
			t,
			filepath.Join(
				fixture.config.Output,
				"reviews", "g1", "english", "candidate-2-1.json",
			),
			&reviewed,
		)
		if reviewed.Invalid == nil || !reviewed.Invalid.InfrastructureFailure {
			t.Fatalf("invalid record = %#v, want projection infrastructure failure", reviewed.Invalid)
		}
		assertRetainedLogPath(t, fixture.config.Output, reviewed.Invalid.RetainedLog)
		assertNoRetainedExecutionSources(t, fixture.config.Output)
		assertTemporaryExecutionRootRemoved(t, fixture.config.Output)
	})
}

func TestRunRegressionHonorsCancellation(t *testing.T) {
	t.Run("before start", func(t *testing.T) {
		fixture := newRunnerFixture(t, "smoke", "g1", 1)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := RunRegression(ctx, fixture.config)

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunRegression error = %v, want context.Canceled", err)
		}
		if _, statErr := os.Lstat(fixture.config.Output); !os.IsNotExist(statErr) {
			t.Fatalf("output stat error = %v, want nonexistent output", statErr)
		}
		if entries := readLogLines(t, fixture.log); len(entries) != 0 {
			t.Fatalf("process log = %q, want no execution", entries)
		}
	})

	t.Run("during scanner", func(t *testing.T) {
		fixture := newRunnerFixture(t, "smoke", "g1", 1)
		t.Setenv("FAKE_SCAN_BLOCK", "1")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := RunRegression(ctx, fixture.config)

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RunRegression error = %v, want context deadline", err)
		}
		requireOperationCounts(t, fixture.log, 1, 0, 0)
		var reviewed ReviewedRun
		readRunnerJSON(
			t,
			filepath.Join(
				fixture.config.Output,
				"reviews", "g1", "english", "golden-1-1.json",
			),
			&reviewed,
		)
		if reviewed.Invalid == nil || !reviewed.Invalid.InfrastructureFailure {
			t.Fatalf("invalid record = %#v, want cancellation infrastructure failure", reviewed.Invalid)
		}
		assertRetainedLogPath(t, fixture.config.Output, reviewed.Invalid.RetainedLog)
		assertNoRetainedExecutionSources(t, fixture.config.Output)
		assertTemporaryExecutionRootRemoved(t, fixture.config.Output)
	})
}

type runnerFixture struct {
	config RunnerConfig
	log    string
}

var runnerCaseIDs = []string{
	"g1",
	"g2-java-missing-contract",
	"g3-go-existing-flow",
	"g4-typescript-consumer-provider",
	"g5-java-persistence-side-effects",
	"g6-ambiguous-entrypoint",
}

func newRunnerFixture(t *testing.T, phase, target string, runs int) runnerFixture {
	t.Helper()
	root := t.TempDir()
	matrixRoot := filepath.Join(root, "matrix")
	if err := os.Mkdir(matrixRoot, 0o700); err != nil {
		t.Fatalf("os.Mkdir matrix: %v", err)
	}
	matrixPath := filepath.Join(matrixRoot, "matrix.json")
	writeRunnerJSON(t, matrixPath, validRunnerMatrix())
	for _, benchmarkCase := range validRunnerMatrix().Cases {
		writeRunnerCase(
			t,
			filepath.Join(matrixRoot, benchmarkCase.Directory),
			benchmarkCase.ID,
			benchmarkCase.ID == "g2-java-missing-contract",
		)
	}
	external := filepath.Join(root, "external-g1")
	writeRunnerCase(t, external, "g1", false)

	logPath := filepath.Join(root, "process.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatalf("os.WriteFile log: %v", err)
	}
	counterPath := filepath.Join(root, "codex-count")
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatalf("os.Mkdir fake-bin: %v", err)
	}
	golden := filepath.Join(fakeBin, "golden")
	candidate := filepath.Join(fakeBin, "candidate")
	writeFakeGoreGraph(t, golden, "golden")
	writeFakeGoreGraph(t, candidate, "candidate")
	writeFakeCodex(t, filepath.Join(fakeBin, "codex"))
	analyzer := filepath.Join(root, "fake-analyzer.sh")
	writeFakeAnalyzer(t, analyzer)
	instruction := filepath.Join(root, "instruction.txt")
	writeRunnerText(t, instruction, "Use the prepared Context Pack and answer from bounded evidence.\n", 0o600)

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_RUNNER_LOG", logPath)
	t.Setenv("FAKE_CODEX_COUNTER", counterPath)
	t.Setenv("FAKE_CONTEXT_COUNTER", filepath.Join(root, "context-count"))
	t.Setenv("FAKE_SCAN_COUNTER", filepath.Join(root, "scan-count"))

	return runnerFixture{
		log: logPath,
		config: RunnerConfig{
			MatrixPath:      matrixPath,
			ExternalCase:    external,
			GoldenBinary:    golden,
			GoldenCommit:    strings.Repeat("a", 40),
			CandidateBinary: candidate,
			CandidateCommit: strings.Repeat("b", 40),
			InstructionPath: instruction,
			Phase:           phase,
			TargetCase:      target,
			Runs:            runs,
			Output:          filepath.Join(root, "output"),
			CodexArgs:       safeRunnerCodexArgs(),
			AnalyzerPath:    analyzer,
		},
	}
}

func validRunnerMatrix() Matrix {
	cases := make([]MatrixCase, 0, 5)
	for _, id := range runnerCaseIDs[1:] {
		cases = append(cases, MatrixCase{ID: id, Directory: id})
	}
	return Matrix{Schema: 1, Cases: cases}
}

func runnerContract(id string, parity bool) Contract {
	contract := validContract()
	contract.ID = id
	contract.Queries = []QueryVariant{{
		ID: "english", Language: "en", File: "query.en.txt", EndToEnd: true,
	}}
	if parity {
		contract.Queries = append(contract.Queries, QueryVariant{
			ID: "german", Language: "de", File: "query.de.txt",
		})
	}
	return contract
}

func writeRunnerCase(t *testing.T, root, id string, parity bool) {
	t.Helper()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, "src"), 0o700); err != nil {
		t.Fatalf("os.MkdirAll workspace: %v", err)
	}
	writeRunnerText(t, filepath.Join(workspace, "src", "source.txt"), id+"\n", 0o600)
	for _, excluded := range []string{
		".git", ".goregraph-workspace", "goregraph-out", "node_modules",
		"target", "build", "dist",
	} {
		path := filepath.Join(workspace, excluded)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("os.Mkdir excluded %s: %v", excluded, err)
		}
		writeRunnerText(t, filepath.Join(path, "secret.txt"), "excluded\n", 0o600)
	}
	writeRunnerJSON(t, filepath.Join(root, "contract.json"), runnerContract(id, parity))
	writeRunnerText(t, filepath.Join(root, "query.en.txt"), "Explain "+id+".\n", 0o600)
	if parity {
		writeRunnerText(t, filepath.Join(root, "query.de.txt"), "Erkläre "+id+".\n", 0o600)
	}
}

func safeRunnerCodexArgs() []string {
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

func writeFakeGoreGraph(t *testing.T, path, build string) {
	t.Helper()
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "version" ]; then
  printf '%s\n'
  exit 0
fi
command_name=${1:-}
workspace=${3:-}
case "$command_name" in
  workspace)
    printf 'scan\t%s\t%%s\t%%s\n' "$workspace" "$0" >>"$FAKE_RUNNER_LOG"
    for excluded in .git .goregraph-workspace goregraph-out node_modules target build dist; do
      if [ -e "$workspace/$excluded/secret.txt" ]; then
        printf 'excluded source retained: %%s\n' "$excluded" >&2
        exit 9
      fi
    done
    if [ -n "${FAKE_SCAN_BLOCK:-}" ]; then
      exec sleep 5
    fi
    if [ -n "${FAKE_MUTATE_SOURCE:-}" ] && [ ! -e "${FAKE_MUTATE_MARKER:-}" ]; then
      printf 'mutated\n' >"$FAKE_MUTATE_SOURCE"
      : >"$FAKE_MUTATE_MARKER"
    fi
    if [ -n "${FAKE_SCAN_FAIL:-}" ]; then
      printf 'injected scan failure\n' >&2
      exit 9
    fi
    if [ -n "${FAKE_SCAN_NO_INDEX:-}" ]; then
      exit 0
    fi
    count=0
    if [ -f "$FAKE_SCAN_COUNTER" ]; then
      count=$(cat "$FAKE_SCAN_COUNTER")
    fi
    count=$((count + 1))
    printf '%%s\n' "$count" >"$FAKE_SCAN_COUNTER"
    changed=false
    if [ "${FAKE_INDEX_CHANGE_AT:-}" = "$count" ]; then
      changed=true
    fi
    mkdir -p "$workspace/.goregraph-workspace/agent"
    if [ -n "${FAKE_INDEX_VOLATILE:-}" ]; then
      printf '{"schema":1,"build":"%s","changed":%%s,"generated":"scan-%%s","root":"%%s"}\n' \
        "$changed" "$count" "$workspace" >"$workspace/.goregraph-workspace/agent/context-index.json"
    else
      printf '{"schema":1,"build":"%s","changed":%%s}\n' \
        "$changed" >"$workspace/.goregraph-workspace/agent/context-index.json"
    fi
    ;;
  context)
    printf 'context\t%s\t%%s\t%%s\t%%s\n' \
      "${2:-}" "$0" "${4:-}" >>"$FAKE_RUNNER_LOG"
    if [ -n "${FAKE_CONTEXT_FAIL:-}" ]; then
      printf 'injected Context failure\n' >&2
      exit 9
    fi
    if [ -n "${FAKE_CONTEXT_MALFORMED:-}" ]; then
      printf '{'
      exit 0
    fi
    schema=%d
    if [ -n "${FAKE_CONTEXT_BAD_SCHEMA:-}" ]; then
      schema=$((schema + 1))
    fi
    count=0
    if [ -f "$FAKE_CONTEXT_COUNTER" ]; then
      count=$(cat "$FAKE_CONTEXT_COUNTER")
    fi
    count=$((count + 1))
    printf '%%s\n' "$count" >"$FAKE_CONTEXT_COUNTER"
    fallback=false
    if [ "${FAKE_CONTEXT_CHANGE_AT:-}" = "$count" ]; then
      fallback=true
    fi
    printf '{"schema":%%s,"query":"fake","confidence":"high","fallback_required":%%s,"estimated_tokens":10,"budget_tokens":4000,"context_id":"call-%%s","retry_allowed":false}\n' \
      "$schema" "$fallback" "$count"
    ;;
  *)
    printf 'unexpected fake goregraph args: %%s\n' "$*" >&2
    exit 9
    ;;
esac
`, build, build, build, build, build, scan.SchemaVersion)
	writeRunnerText(t, path, script, 0o700)
}

func writeFakeCodex(t *testing.T, path string) {
	t.Helper()
	script := `#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "--version" ]; then
  printf 'codex-fake 1.0\n'
  exit 0
fi
workspace=""
previous=""
for argument in "$@"; do
  if [ "$previous" = "-C" ]; then
    workspace=$argument
  fi
  previous=$argument
done
build=$(goregraph version)
case_id=$(basename "$(dirname "$workspace")")
run_name=$(basename "$workspace")
run_number=${run_name#*-}
run_number=${run_number%%-*}
printf 'codex\t%s\t%s\t%s\t%s\t%s\n' "$case_id" "$build" "$run_number" "$workspace" "$(command -v goregraph)" >>"$FAKE_RUNNER_LOG"
cat >/dev/null
count=0
if [ -f "$FAKE_CODEX_COUNTER" ]; then
  count=$(cat "$FAKE_CODEX_COUNTER")
fi
count=$((count + 1))
printf '%s\n' "$count" >"$FAKE_CODEX_COUNTER"
printf '{"type":"item.completed","item":{"id":"tool-1","type":"command_execution","command":"pwd"}}\n'
if [ "${FAKE_CODEX_FAIL_AT:-}" = "$count" ]; then
  printf 'injected Codex failure\n' >&2
  exit 9
fi
if [ -n "${FAKE_CODEX_SEMANTIC_BAD:-}" ]; then
  printf '{"type":"item.completed","item":{"id":"answer","type":"agent_message","text":"semantically bad"}}\n'
fi
printf '{"type":"turn.completed","usage":{"total_tokens":100}}\n'
`
	writeRunnerText(t, path, script, 0o700)
}

func writeFakeAnalyzer(t *testing.T, path string) {
	t.Helper()
	script := `#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "--tokens" ]; then
  if [ -n "${FAKE_ANALYZER_FAIL:-}" ]; then
    printf 'injected analyzer failure\n' >&2
    exit 9
  fi
  printf '101\n'
  exit 0
fi
printf '11\t12\t13\t14\t15\t16\t17\t18\t19\t20\t21\n'
`
	writeRunnerText(t, path, script, 0o700)
}

func codexOrder(t *testing.T, logPath string) []string {
	t.Helper()
	var order []string
	for _, line := range readLogLines(t, logPath) {
		fields := strings.Split(line, "\t")
		if len(fields) >= 4 && fields[0] == "codex" {
			order = append(order, strings.Join(fields[1:4], "\t"))
		}
	}
	return order
}

func endToEndContextOrder(t *testing.T, logPath string) []string {
	t.Helper()
	var order []string
	for _, line := range readLogLines(t, logPath) {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 ||
			fields[0] != "context" ||
			!strings.HasPrefix(fields[4], "Explain ") {
			continue
		}
		workspace := fields[2]
		caseID := filepath.Base(filepath.Dir(workspace))
		runName := filepath.Base(workspace)
		runNumber := strings.Split(runName, "-")[1]
		order = append(order, strings.Join([]string{caseID, fields[1], runNumber}, "\t"))
	}
	return order
}

func operationOrder(t *testing.T, logPath string) []string {
	t.Helper()
	var order []string
	for _, line := range readLogLines(t, logPath) {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		buildIndex := 1
		if fields[0] == "codex" {
			buildIndex = 2
		}
		order = append(order, fields[0]+"\t"+fields[buildIndex])
	}
	return order
}

func requireOperationCounts(
	t *testing.T,
	logPath string,
	wantScan int,
	wantContext int,
	wantCodex int,
) {
	t.Helper()
	counts := make(map[string]int)
	for _, line := range readLogLines(t, logPath) {
		fields := strings.Split(line, "\t")
		if len(fields) > 0 {
			counts[fields[0]]++
		}
	}
	if counts["scan"] != wantScan ||
		counts["context"] != wantContext ||
		counts["codex"] != wantCodex {
		t.Fatalf(
			"operation counts = scan:%d context:%d codex:%d, want %d/%d/%d",
			counts["scan"],
			counts["context"],
			counts["codex"],
			wantScan,
			wantContext,
			wantCodex,
		)
	}
}

func assertIsolatedExecutions(
	t *testing.T,
	logPath string,
	want int,
	output string,
) {
	t.Helper()
	seenWorkspaces := make(map[string]bool)
	for _, line := range readLogLines(t, logPath) {
		fields := strings.Split(line, "\t")
		if len(fields) < 6 || fields[0] != "codex" {
			continue
		}
		build, workspace, frontBinary := fields[2], fields[4], fields[5]
		if seenWorkspaces[workspace] {
			t.Fatalf("workspace reused by Codex: %s", workspace)
		}
		seenWorkspaces[workspace] = true
		if workspace == output ||
			strings.HasPrefix(workspace, output+string(filepath.Separator)) {
			t.Fatalf("execution workspace retained inside output: %s", workspace)
		}
		if filepath.Base(frontBinary) != "goregraph" {
			t.Fatalf("front PATH binary = %q, want goregraph", frontBinary)
		}
		if !strings.Contains(frontBinary, string(filepath.Separator)+build+string(filepath.Separator)) {
			t.Fatalf("%s Codex front binary = %q, want matching private build path", build, frontBinary)
		}
	}
	if len(seenWorkspaces) != want {
		t.Fatalf("isolated Codex workspaces = %d, want %d", len(seenWorkspaces), want)
	}
	for _, line := range readLogLines(t, logPath) {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 || fields[0] != "scan" {
			continue
		}
		if !strings.HasPrefix(filepath.Base(fields[2]), fields[1]+"-") {
			t.Fatalf("%s scanner used mismatched workspace %q", fields[1], fields[2])
		}
	}
}

func assertNoRetainedExecutionSources(t *testing.T, output string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(output, "workspaces")); !os.IsNotExist(err) {
		t.Fatalf("output/workspaces retained: %v", err)
	}
	err := filepath.WalkDir(output, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() == "source.txt" || entry.Name() == "secret.txt" {
			t.Fatalf("fixture source retained in final output: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("filepath.WalkDir output: %v", err)
	}
}

func assertTemporaryExecutionRootRemoved(t *testing.T, output string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(
		filepath.Dir(output),
		".goregraph-agentbench-snapshot-*",
	))
	if err != nil {
		t.Fatalf("filepath.Glob temporary execution roots: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary execution roots retained: %v", matches)
	}
}

func assertRetainedLogPath(t *testing.T, output, logPath string) {
	t.Helper()
	if !filepath.IsAbs(logPath) ||
		(logPath != output &&
			!strings.HasPrefix(logPath, output+string(filepath.Separator))) {
		t.Fatalf("retained log path = %q, want path under output %q", logPath, output)
	}
	requireFile(t, logPath)
}

func assertRequiredArtifacts(t *testing.T, fixture runnerFixture) {
	t.Helper()
	for _, relative := range []string{
		"inputs/matrix.json",
		"inputs/external-case/contract.json",
		"inputs/instruction.txt",
		"inputs/prompt-digests.tsv",
		"identity/golden-binary.sha256",
		"identity/candidate-binary.sha256",
		"identity/golden-commit.txt",
		"identity/candidate-commit.txt",
		"identity/codex-version.txt",
		"identity/codex-args.txt",
		"identity/run-order.tsv",
		"summary.tsv",
	} {
		requireFile(t, filepath.Join(fixture.config.Output, filepath.FromSlash(relative)))
	}
	for _, build := range []string{"golden", "candidate"} {
		hash := strings.TrimSpace(readRunnerText(t, filepath.Join(
			fixture.config.Output, "identity", build+"-binary.sha256",
		)))
		if len(hash) != 64 {
			t.Fatalf("%s binary hash = %q, want 64 hex", build, hash)
		}
	}
	for _, caseID := range runnerCaseIDs {
		caseRoot := filepath.Join(fixture.config.Output, "cases", caseID, "english")
		for _, relative := range []string{
			"golden-pack.json",
			"candidate-pack.json",
			"pack-diff.json",
			"golden-workspace.sha256",
			"candidate-workspace.sha256",
			"golden-index.sha256",
			"candidate-index.sha256",
		} {
			requireFile(t, filepath.Join(caseRoot, relative))
		}
		for run := 1; run <= 3; run++ {
			for _, build := range []string{"golden", "candidate"} {
				name := fmt.Sprintf("%s-%d-1", build, run)
				for _, suffix := range []string{".jsonl", ".stderr", ".metrics.tsv"} {
					requireFile(t, filepath.Join(
						fixture.config.Output, "runs", caseID, "english", name+suffix,
					))
				}
				requireFile(t, filepath.Join(
					fixture.config.Output, "reviews", caseID, "english", name+".json",
				))
			}
		}
	}
	summary := readRunnerText(t, filepath.Join(fixture.config.Output, "summary.tsv"))
	const header = "case\tquery\tbuild\trun\tattempt\ttokens\ttool_calls\tcontext_calls\trepeated_full_packs\tbroad_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tcontext_millis\tlog\n"
	if !strings.HasPrefix(summary, header) {
		t.Fatalf("summary header = %q, want %q", strings.SplitN(summary, "\n", 2)[0], strings.TrimSpace(header))
	}
	summaryLines := strings.Split(strings.TrimSpace(summary), "\n")[1:]
	summaryOrder := make([]string, 0, len(summaryLines))
	for _, line := range summaryLines {
		fields := strings.Split(line, "\t")
		assertRetainedLogPath(t, fixture.config.Output, fields[15])
		summaryOrder = append(
			summaryOrder,
			strings.Join([]string{fields[0], fields[2], fields[3]}, "\t"),
		)
	}
	if want := codexOrder(t, fixture.log); !reflect.DeepEqual(summaryOrder, want) {
		t.Fatalf("summary order = %q, want %q", summaryOrder, want)
	}
	assertNoRetainedExecutionSources(t, fixture.config.Output)
	assertTemporaryExecutionRootRemoved(t, fixture.config.Output)
}

func readLogLines(t *testing.T, path string) []string {
	t.Helper()
	body := strings.TrimSpace(readRunnerText(t, path))
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

func requireFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q): %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s mode = %s, want regular file", path, info.Mode())
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("%s permissions = %o, want 600", path, info.Mode().Perm())
	}
}

func writeRunnerJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}
	writeRunnerText(t, path, string(append(body, '\n')), 0o600)
}

func readRunnerJSON(t *testing.T, path string, destination any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}
	if err := json.Unmarshal(body, destination); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", path, err)
	}
}

func writeRunnerText(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", path, err)
	}
}

func readRunnerText(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}
	return string(body)
}

func sortedRunnerPaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("filepath.WalkDir(%q): %v", root, err)
	}
	sort.Strings(paths)
	return paths
}
