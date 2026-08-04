package agentbench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/cli"
	"github.com/gorecodecom/goregraph/internal/scan"
)

type parityProjection struct {
	Endpoint         string
	Entrypoints      []string
	CallChain        []string
	Contracts        []string
	Persistence      []string
	Sources          []string
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
				request := agent.ContextRequest{
					Root:         root,
					Query:        string(query),
					BudgetTokens: 4000,
					MaxFiles:     12,
				}
				pack, err := agent.BuildContext(request)
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
				if benchmarkCase.ID == "g3-go-existing-flow" {
					requireG3ExistingFlowOmissions(t, pack)
				}
				requireByteStableContextPack(t, request, pack, variant.ID)

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

func requireG3ExistingFlowOmissions(t *testing.T, pack agent.ContextPack) {
	t.Helper()
	if len(pack.SourceOmissions) != 2 {
		t.Fatalf("G3 source omissions = %#v, want existing flow and test", pack.SourceOmissions)
	}
	want := map[string]string{
		"service.go":      "call_chain",
		"service_test.go": "test",
	}
	for _, omission := range pack.SourceOmissions {
		role, ok := want[omission.Path]
		if !ok || omission.Role != role {
			t.Fatalf("G3 source omission = %#v, want %#v", omission, want)
		}
		delete(want, omission.Path)
	}
	if len(want) != 0 {
		t.Fatalf("G3 source omissions missing %#v: %#v", want, pack.SourceOmissions)
	}
}

func requireByteStableContextPack(
	t *testing.T,
	request agent.ContextRequest,
	want agent.ContextPack,
	variant string,
) {
	t.Helper()
	wantBody, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal %s baseline pack: %v", variant, err)
	}
	for repeat := 2; repeat <= 3; repeat++ {
		got, err := agent.BuildContext(request)
		if err != nil {
			t.Fatalf("BuildContext %s repeat %d: %v", variant, repeat, err)
		}
		gotBody, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal %s repeat %d: %v", variant, repeat, err)
		}
		if !bytes.Equal(gotBody, wantBody) {
			t.Fatalf("%s repeat %d is not byte-stable", variant, repeat)
		}
	}
}

func TestG6AmbiguousEndpointFallbackIsStableAcrossIndexOrder(t *testing.T) {
	fixtureRoot := filepath.Join(
		"..", "..", "testdata", "agent-context-regression",
		"g6-ambiguous-entrypoint",
	)
	root := copyBenchmarkWorkspace(t, filepath.Join(fixtureRoot, "workspace"))
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{
		"workspace", "scan-all", root,
		"--workspace", root,
		"--no-update-gitignore",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("workspace scan-all exit code = %d, stderr=%s", code, stderr.String())
	}

	contract, err := LoadContract(filepath.Join(fixtureRoot, "contract.json"))
	if err != nil {
		t.Fatalf("LoadContract: %v", err)
	}
	query, err := os.ReadFile(filepath.Join(fixtureRoot, contract.Queries[0].File))
	if err != nil {
		t.Fatalf("read query: %v", err)
	}

	indexPath := filepath.Join(
		root, ".goregraph-workspace", "agent", "context-index.json",
	)
	index := readBenchmarkContextIndex(t, indexPath)
	requireG6DeleteProviders(t, index)

	request := agent.ContextRequest{
		Root:         root,
		Query:        string(query),
		BudgetTokens: 4000,
		MaxFiles:     12,
	}
	forward, err := agent.BuildContext(request)
	if err != nil {
		t.Fatalf("BuildContext forward: %v", err)
	}
	requireBoundedAmbiguousFallback(t, forward, contract.Pack)

	slices.Reverse(index.Facts)
	slices.Reverse(index.Edges)
	slices.Reverse(index.Coverage)
	writeBenchmarkContextIndex(t, indexPath, index)

	backward, err := agent.BuildContext(request)
	if err != nil {
		t.Fatalf("BuildContext reversed: %v", err)
	}
	requireBoundedAmbiguousFallback(t, backward, contract.Pack)
	if !reflect.DeepEqual(comparableProjection(forward), comparableProjection(backward)) {
		t.Fatalf(
			"G6 projection depends on index order:\nforward: %#v\nreversed: %#v",
			comparableProjection(forward),
			comparableProjection(backward),
		)
	}
}

func readBenchmarkContextIndex(
	t *testing.T,
	path string,
) scan.AgentContextIndexRecord {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context index: %v", err)
	}
	var index scan.AgentContextIndexRecord
	if err := json.Unmarshal(body, &index); err != nil {
		t.Fatalf("decode context index: %v", err)
	}
	return index
}

func writeBenchmarkContextIndex(
	t *testing.T,
	path string,
	index scan.AgentContextIndexRecord,
) {
	t.Helper()
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatalf("encode context index: %v", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write context index: %v", err)
	}
}

func requireG6DeleteProviders(t *testing.T, index scan.AgentContextIndexRecord) {
	t.Helper()
	providers := map[string]bool{}
	for _, fact := range index.Facts {
		if fact.Kind == "api_endpoint" &&
			fact.HTTPMethod == "DELETE" &&
			fact.Path == "/jobs/{jobId}" {
			providers[fact.Project] = true
		}
	}
	for _, project := range []string{
		"services/batch-jobs",
		"services/scheduled-jobs",
	} {
		if !providers[project] {
			t.Fatalf("G6 DELETE provider %q missing before BuildContext", project)
		}
	}
}

func requireBoundedAmbiguousFallback(
	t *testing.T,
	pack agent.ContextPack,
	expectation PackExpectation,
) {
	t.Helper()
	if violations := EvaluatePack(pack, expectation); len(violations) != 0 {
		t.Fatalf("EvaluatePack violations: %#v\npack: %#v", violations, pack)
	}
	if len(pack.Endpoints) != 0 || len(pack.Entrypoints) != 0 {
		t.Fatalf("ambiguous fallback selected a unique entrypoint: %#v", pack)
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
		Sources:          semanticParitySources(projected.Sources),
		FallbackRequired: projected.FallbackRequired,
		RetryAllowed:     projected.RetryAllowed,
	}
}

func semanticParitySources(sources []string) []string {
	identities := map[string]bool{}
	for _, source := range sources {
		parts := strings.SplitN(source, "|", 6)
		if len(parts) < 5 {
			identities[source] = true
			continue
		}
		identities[strings.Join([]string{parts[0], parts[1], parts[4]}, "|")] = true
	}
	result := make([]string, 0, len(identities))
	for identity := range identities {
		result = append(result, identity)
	}
	slices.Sort(result)
	return result
}
