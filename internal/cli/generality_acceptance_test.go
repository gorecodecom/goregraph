package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/scan"
)

const generalityQuery = "Trace cancellation from the public order endpoint to inventory reservation cleanup. Identify the missing client call, both persistence repositories, authentication and configuration evidence, side effects, and tests. Report uncertainty when source evidence is absent."

func TestSourceDerivedGeneralityWorkspace(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "generality-workspace")
	copyGeneralityFixture(t, filepath.Join("testdata", "generality-workspace"), workspace)

	first := buildGeneralityContext(t, workspace)
	assertGeneralityRegistry(t, workspace)
	assertGeneralityDoctor(t, workspace)
	assertGeneralityContext(t, first.pack, first.body)

	second := buildGeneralityContext(t, workspace)
	assertGeneralityRegistry(t, workspace)
	assertGeneralityDoctor(t, workspace)
	assertGeneralityContext(t, second.pack, second.body)
	if normalizeGeneralityOutput(t, first.body, workspace) != normalizeGeneralityOutput(t, second.body, workspace) {
		t.Fatalf("repeated Context Packs differ:\nfirst=%s\nsecond=%s", first.body, second.body)
	}
	assertNoPrivateGeneralityTerms(t, workspace, first.body, second.body)
}

type generalityContextResult struct {
	pack agent.ContextPack
	body string
}

func buildGeneralityContext(t *testing.T, workspace string) generalityContextResult {
	t.Helper()
	var buildOut, buildErr bytes.Buffer
	if code := Run([]string{
		"workspace", "scan-all", workspace,
		"--workspace", workspace,
		"--no-update-gitignore",
	}, &buildOut, &buildErr); code != 0 {
		t.Fatalf("workspace build exit code = %d, stderr=%s\nstdout=%s", code, buildErr.String(), buildOut.String())
	}

	var contextOut, contextErr bytes.Buffer
	if code := Run([]string{
		"context", workspace,
		"--query", generalityQuery,
		"--budget-tokens", "4000",
		"--max-files", "12",
		"--format", "json",
	}, &contextOut, &contextErr); code != 0 {
		t.Fatalf("context exit code = %d, stderr=%s", code, contextErr.String())
	}
	var pack agent.ContextPack
	if err := json.Unmarshal(contextOut.Bytes(), &pack); err != nil {
		t.Fatalf("decode Context Pack: %v\n%s", err, contextOut.String())
	}
	return generalityContextResult{pack: pack, body: contextOut.String()}
}

func assertGeneralityRegistry(t *testing.T, workspace string) {
	t.Helper()
	path := scan.NewWorkspaceOutputLayout(filepath.Join(workspace, ".goregraph-workspace")).Index("registry.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workspace registry: %v", err)
	}
	var registry scan.WorkspaceRegistryRecord
	if err := json.Unmarshal(body, &registry); err != nil {
		t.Fatalf("decode workspace registry: %v", err)
	}
	if len(registry.Projects) != 3 {
		t.Fatalf("registry project count = %d, want 3: %#v", len(registry.Projects), registry.Projects)
	}
	for _, project := range registry.Projects {
		if !project.Indexed || project.Status == "not_indexed" {
			t.Fatalf("project %s is not current: %#v", project.Path, project)
		}
	}
}

func assertGeneralityDoctor(t *testing.T, workspace string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"doctor", workspace}, &stdout, &stderr); code != 0 {
		t.Fatalf("Doctor exit code = %d, stderr=%s\nstdout=%s", code, stderr.String(), stdout.String())
	}
}

func assertGeneralityContext(t *testing.T, pack agent.ContextPack, body string) {
	t.Helper()
	if pack.FallbackRequired || len(pack.Entrypoints)+len(pack.Endpoints) != 1 {
		t.Fatalf("Context must use one entrypoint without fallback: %#v", pack)
	}
	if len(pack.Endpoints) != 1 || pack.Endpoints[0].Provider != "order-service" ||
		pack.Endpoints[0].HTTPMethod != "DELETE" || pack.Endpoints[0].Path != "/orders/{orderId}" {
		t.Fatalf("unexpected primary API entrypoint: entrypoints=%#v endpoints=%#v", pack.Entrypoints, pack.Endpoints)
	}
	if pack.EstimatedTokens > 4000 || len(pack.Files) > 12 {
		t.Fatalf("Context exceeds budget: tokens=%d files=%d", pack.EstimatedTokens, len(pack.Files))
	}
	lower := strings.ToLower(body)
	for _, want := range []string{
		"inventory-service",
		"platform-clients",
		"inventorymgmtservice",
		"stockreservationrepository",
		"allocationreservationrepository",
		"authentication",
		"configuration",
		"side effect",
		"test",
	} {
		if !strings.Contains(lower, strings.ToLower(want)) {
			t.Fatalf("Context Pack missing %q:\n%s", want, body)
		}
	}
	for _, relationship := range pack.CallChain {
		joined := strings.ToLower(relationship.From + " " + relationship.To + " " + relationship.Reason)
		if strings.Contains(joined, "ordercancellationservice") && strings.Contains(joined, "inventorymgmtservice") {
			t.Fatalf("Context invented the intentionally missing client call: %#v", relationship)
		}
	}
	if len(pack.Uncertainties) == 0 {
		t.Fatalf("Context must retain uncertainty for absent source evidence: %#v", pack)
	}
	for _, kind := range []string{"authentication", "configuration", "persistence", "side_effects", "tests"} {
		covered := false
		for _, concern := range pack.Concerns {
			if concern.Kind == kind && concern.Covered {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("requested concern %q is not covered: %#v", kind, pack.Concerns)
		}
	}
}

func copyGeneralityFixture(t *testing.T, source, destination string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
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
		return os.WriteFile(target, body, 0o644)
	}); err != nil {
		t.Fatalf("copy generality fixture: %v", err)
	}
}

func normalizeGeneralityOutput(t *testing.T, body, workspace string) string {
	t.Helper()
	body = strings.ReplaceAll(body, filepath.ToSlash(workspace), "<workspace>")
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(body), &pack); err != nil {
		t.Fatalf("normalize Context Pack: %v", err)
	}
	pack.Freshness = "<freshness>"
	pack.ContextID = "<context-id>"
	normalized, err := json.Marshal(pack)
	if err != nil {
		t.Fatalf("marshal normalized Context Pack: %v", err)
	}
	return string(normalized)
}

func assertNoPrivateGeneralityTerms(t *testing.T, workspace string, outputs ...string) {
	t.Helper()
	forbidden := []string{"0442483", "cadaster", "rdbv", "regulationchangebasecontroller", "vorschrift", "weka.request"}
	check := func(scope, body string) {
		lower := strings.ToLower(body)
		for _, term := range forbidden {
			if strings.Contains(lower, term) {
				t.Errorf("private term %q in %s", term, scope)
			}
		}
	}
	for index, output := range outputs {
		check("Context output "+strconv.Itoa(index+1), output)
	}
	if err := filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		check(filepath.ToSlash(path), string(body))
		return nil
	}); err != nil {
		t.Fatalf("audit generality workspace: %v", err)
	}
}
