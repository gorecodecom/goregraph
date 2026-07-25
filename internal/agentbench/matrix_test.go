package agentbench

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/cli"
)

type parityProjection struct {
	Endpoint         string
	Entrypoints      []string
	CallChain        []string
	Contracts        []string
	Persistence      []string
	Sources          []string
	SourceCoverage   string
	FallbackRequired bool
	RetryAllowed     bool
}

func TestCommittedBenchmarkMatrix(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "testdata", "agent-context-regression")
	matrix, err := LoadMatrix(filepath.Join(fixtureRoot, "matrix.json"))
	if err != nil {
		t.Fatalf("LoadMatrix: %v", err)
	}

	for _, benchmarkCase := range matrix.Cases {
		benchmarkCase := benchmarkCase
		t.Run(strings.ToUpper(strings.SplitN(benchmarkCase.ID, "-", 2)[0]), func(t *testing.T) {
			caseRoot := filepath.Join(fixtureRoot, benchmarkCase.Directory)
			root := copyBenchmarkWorkspace(t, filepath.Join(caseRoot, "workspace"))
			var stdout, stderr bytes.Buffer
			code := cli.Run([]string{
				"workspace", "scan-all", root,
				"--workspace", root,
				"--no-update-gitignore",
			}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("workspace scan-all exit code = %d, stderr=%s", code, stderr.String())
			}

			contract, err := LoadContract(filepath.Join(caseRoot, "contract.json"))
			if err != nil {
				t.Fatalf("LoadContract: %v", err)
			}

			var baseline parityProjection
			for index, variant := range contract.Queries {
				query, err := os.ReadFile(filepath.Join(caseRoot, variant.File))
				if err != nil {
					t.Fatalf("read query %s: %v", variant.ID, err)
				}
				pack, err := agent.BuildContext(agent.ContextRequest{
					Root:         root,
					Query:        string(query),
					BudgetTokens: 4000,
					MaxFiles:     12,
				})
				if err != nil {
					t.Fatalf("BuildContext %s: %v", variant.ID, err)
				}
				if violations := EvaluatePack(pack, contract.Pack); len(violations) > 0 {
					t.Fatalf(
						"EvaluatePack %s violations: %#v\nscan stdout: %s\nscan stderr: %s\npack: %#v",
						variant.ID,
						violations,
						stdout.String(),
						stderr.String(),
						pack,
					)
				}

				projection := comparableProjection(pack)
				if index == 0 {
					baseline = projection
					continue
				}
				if !reflect.DeepEqual(projection, baseline) {
					t.Fatalf("%s semantic projection differs from %s:\n got: %#v\nwant: %#v", variant.ID, contract.Queries[0].ID, projection, baseline)
				}
			}
		})
	}
}

func copyBenchmarkWorkspace(t *testing.T, source string) string {
	t.Helper()
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o600)
	})
	if err != nil {
		t.Fatalf("copy benchmark workspace %s: %v", source, err)
	}
	return destination
}

func comparableProjection(pack agent.ContextPack) parityProjection {
	projected := ProjectPack(pack)
	return parityProjection{
		Endpoint:         projected.Endpoint,
		Entrypoints:      projected.Entrypoints,
		CallChain:        projected.CallChain,
		Contracts:        projected.Contracts,
		Persistence:      projected.Persistence,
		Sources:          projected.Sources,
		SourceCoverage:   projected.SourceCoverage,
		FallbackRequired: projected.FallbackRequired,
		RetryAllowed:     projected.RetryAllowed,
	}
}
