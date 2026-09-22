package mcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

type storybookAuditSource struct {
	Path     string   `json:"path"`
	Snippets []string `json:"snippets"`
}

type storybookAuditRequirement struct {
	ID       string                 `json:"id"`
	Expected string                 `json:"expected"`
	Sources  []storybookAuditSource `json:"sources"`
}

type storybookAuditEvidence struct {
	ID       string   `json:"id"`
	Expected string   `json:"expected"`
	Covered  bool     `json:"covered"`
	Missing  []string `json:"missing,omitempty"`
	Omitted  []string `json:"omitted,omitempty"`
}

type storybookAuditReport struct {
	Protocol       string                   `json:"protocol"`
	Query          string                   `json:"query"`
	SourceCoverage string                   `json:"source_coverage"`
	Fallback       string                   `json:"fallback,omitempty"`
	Evidence       []storybookAuditEvidence `json:"evidence"`
	Complete       bool                     `json:"complete"`
}

func loadStorybookAuditFixture(t *testing.T) (string, map[string]string, []storybookAuditRequirement) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{}
	source := os.DirFS("testdata/storybook-audit/project")
	if err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		// Source fixtures use canonical newlines on every Git checkout platform.
		files[path] = strings.ReplaceAll(string(body), "\r\n", "\n")
		writeFile(t, root, path, files[path])
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("testdata/storybook-audit/expectations.json")
	if err != nil {
		t.Fatal(err)
	}
	var requirements []storybookAuditRequirement
	if err := json.Unmarshal(body, &requirements); err != nil {
		t.Fatal(err)
	}
	if len(requirements) != 7 {
		t.Fatalf("expected seven independent audit requirements, got %d", len(requirements))
	}
	for _, requirement := range requirements {
		for _, source := range requirement.Sources {
			for _, snippet := range source.Snippets {
				if !strings.Contains(files[source.Path], snippet) {
					t.Fatalf("invalid expectation %s: %s lacks %q", requirement.ID, source.Path, snippet)
				}
			}
		}
	}
	return root, files, requirements
}

func buildStorybookAuditFixture(t *testing.T, root string) scan.Result {
	t.Helper()
	cfg := config.Defaults()
	cfg.Workspace = false
	cfg.UpdateGitignore = false
	result, err := scan.RunBuild(root, cfg, scan.BuildTargetAgent)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestStorybookAuditIndexesRealFixtureWithoutChangingSources(t *testing.T) {
	root, files, _ := loadStorybookAuditFixture(t)
	result := buildStorybookAuditFixture(t, root)
	if result.ScannedFiles != len(files) {
		t.Fatalf("scanned %d files, want %d", result.ScannedFiles, len(files))
	}
	var inventory []scan.FileRecord
	readStorybookAuditJSON(t, filepath.Join(result.OutputDir, "index/files.json"), &inventory)
	indexed := map[string]bool{}
	for _, file := range inventory {
		indexed[file.Path] = true
	}
	for path, original := range files {
		if !indexed[path] {
			t.Errorf("file inventory lost %s", path)
		}
		current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(current) != original {
			t.Errorf("build changed fixture input %s", path)
		}
	}
	for _, path := range []string{".gitignore", ".goregraph-workspace", "goregraph-out/dashboard"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Errorf("unexpected output %s: %v", path, err)
		}
	}
	var index scan.AgentContextIndexRecord
	readStorybookAuditJSON(t, filepath.Join(result.OutputDir, "agent/context-index.json"), &index)
	facts := map[string]int{}
	for _, fact := range index.Facts {
		facts[fact.File]++
	}
	for _, path := range []string{"src/orders.ts", "src/ProductCard.tsx", "src/productFixture.ts"} {
		if facts[path] == 0 {
			t.Errorf("normal source extraction lost %s", path)
		}
	}
	t.Logf("Real build: %d source files; context facts by source: %v", result.ScannedFiles, facts)
	// Missing reference images and run results are fixture facts, not conclusions
	// derived from a partial Context Pack or from package.json dependencies.
	for path := range files {
		if strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".junit.xml") {
			t.Fatalf("fixture must not contain successful-run or baseline artifacts: %s", path)
		}
	}
	if strings.Contains(files[".gitlab-ci.yml"], "unreferenced.yml") {
		t.Fatal("negative-control CI file must remain disconnected")
	}
}

func readStorybookAuditJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, value); err != nil {
		t.Fatal(err)
	}
}

func storybookAuditContext(t *testing.T, root, query, protocol string) agent.ContextPack {
	return storybookAuditContextMode(t, root, query, protocol, "")
}

func storybookAuditContextMode(t *testing.T, root, query, protocol, mode string) agent.ContextPack {
	t.Helper()
	// These retained fixtures compare the historical strict and adaptive contracts.
	if protocol == "" {
		protocol = "strict-v1"
	}
	args := map[string]any{"root": root, "query": query, "budget_tokens": 6000, "max_files": 12}
	if mode != "" {
		args["mode"], args["max_files"] = mode, 20
	}
	wire, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "task_context", "arguments": args,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := ServeWithOptions(bytes.NewReader(append(wire, '\n')), &output, Options{ProtocolVersion: protocol}); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Error  json.RawMessage `json:"error"`
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Error) > 0 || response.Result.IsError || len(response.Result.Content) != 1 {
		t.Fatalf("MCP failed: %s", output.Bytes())
	}
	var pack agent.ContextPack
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &pack); err != nil {
		t.Fatal(err)
	}
	if pack.BudgetTokens != 6000 || pack.EstimatedTokens > 6000 {
		t.Fatalf("MCP budget contract changed: %+v", pack)
	}
	return pack
}

func evaluateStorybookAudit(pack agent.ContextPack, requirements []storybookAuditRequirement) []storybookAuditEvidence {
	evidence := make([]storybookAuditEvidence, 0, len(requirements))
	for _, requirement := range requirements {
		result := storybookAuditEvidence{ID: requirement.ID, Expected: requirement.Expected, Covered: true}
		for _, source := range requirement.Sources {
			var content strings.Builder
			for _, section := range pack.SourceSections {
				if section.Path == source.Path {
					content.WriteString(section.Content)
					content.WriteByte('\n')
				}
			}
			missing := false
			for _, snippet := range source.Snippets {
				if !strings.Contains(content.String(), snippet) {
					missing = true
				}
			}
			if missing {
				result.Covered = false
				result.Missing = append(result.Missing, source.Path)
				for _, omission := range pack.SourceOmissions {
					if omission.Path == source.Path && omission.StartLine > 0 && omission.EndLine >= omission.StartLine {
						result.Omitted = append(result.Omitted, source.Path)
						break
					}
				}
			}
		}
		evidence = append(evidence, result)
	}
	return evidence
}

func storybookAuditComplete(evidence []storybookAuditEvidence) bool {
	if len(evidence) == 0 {
		return false
	}
	for _, item := range evidence {
		if !item.Covered {
			return false
		}
	}
	return true
}

func TestStorybookAuditOracleRejectsUnsupportedConclusions(t *testing.T) {
	_, files, requirements := loadStorybookAuditFixture(t)
	full := agent.ContextPack{SourceCoverage: "complete"}
	for path, content := range files {
		full.SourceSections = append(full.SourceSections, agent.ContextSourceSection{Path: path, Content: content})
	}
	if !storybookAuditComplete(evaluateStorybookAudit(full, requirements)) {
		t.Fatal("oracle rejected all required fixture evidence")
	}
	// A generic source_coverage label cannot stand in for audit-domain evidence.
	if storybookAuditComplete(evaluateStorybookAudit(agent.ContextPack{SourceCoverage: "complete"}, requirements)) {
		t.Fatal("empty pack passed as a complete audit")
	}
	for _, removed := range []string{".storybook/preview.ts", ".gitlab-ci.yml", ".gitlab/storybook.yml", "src/productFixture.ts", "tests/storybook/visual.spec.ts"} {
		t.Run(removed, func(t *testing.T) {
			partial := agent.ContextPack{SourceCoverage: "complete"}
			for _, section := range full.SourceSections {
				if section.Path != removed {
					partial.SourceSections = append(partial.SourceSections, section)
				}
			}
			partial.SourceOmissions = []agent.ContextSourceOmission{{Path: removed, StartLine: 1, EndLine: 10}}
			if storybookAuditComplete(evaluateStorybookAudit(partial, requirements)) {
				t.Fatalf("audit accepted absent source %s", removed)
			}
		})
	}
	for _, source := range []storybookAuditSource{{Path: ".storybook/preview.ts", Snippets: []string{"test: 'off'", "test: 'error'"}}, {Path: ".gitlab/storybook.yml", Snippets: []string{"allow_failure: true", "allow_failure: false"}}} {
		changed := agent.ContextPack{SourceCoverage: "complete"}
		for _, section := range full.SourceSections {
			if section.Path == source.Path {
				section.Content = strings.ReplaceAll(section.Content, source.Snippets[0], source.Snippets[1])
			}
			changed.SourceSections = append(changed.SourceSections, section)
		}
		if storybookAuditComplete(evaluateStorybookAudit(changed, requirements)) {
			t.Fatalf("oracle missed changed semantics in %s", source.Path)
		}
	}
}

func TestStorybookAuditContextEvidence(t *testing.T) {
	root, files, requirements := loadStorybookAuditFixture(t)
	result := buildStorybookAuditFixture(t, root)
	indexPath := filepath.Join(result.OutputDir, "agent/context-index.json")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	queries := []string{
		"Welche Storybook-Erweiterungen fehlen? Berücksichtige Konfiguration, Stories, Fixtures, Test-Runner, CI und Dokumentation. Prüfe automatische Accessibility-Prüfungen, visuelle Referenzbilder und erfolgreiche Ausführung.",
		"Prüfe den Stand für Version 1.4.3. Welche Storybook-Erweiterungen fehlen? Berücksichtige Konfiguration, Stories, Fixtures, Test-Runner, CI und Dokumentation. Prüfe automatische Accessibility-Prüfungen, visuelle Referenzbilder und erfolgreiche Ausführung.",
	}
	var reports []storybookAuditReport
	for _, protocol := range []string{"", "adaptive-v2"} {
		control := storybookAuditContext(t, root, "Explain src/orders.ts orderTotal price quantity calculation", protocol)
		verifyStorybookAuditSources(t, control, files)
		if control.FallbackRequired || control.SourceCoverage != "complete" {
			t.Fatalf("normal production lookup regressed: %+v", control)
		}
		found := false
		for _, section := range control.SourceSections {
			if strings.Contains(section.Path, ".storybook/") {
				t.Fatalf("ordinary production query pulled in Storybook configuration: %+v", section)
			}
			if section.Path == "src/orders.ts" && strings.Contains(section.Content, "return price * quantity;") {
				found = true
			}
		}
		if !found {
			t.Fatal("normal production lookup lost implementation evidence")
		}
		for _, query := range queries {
			pack := storybookAuditContextMode(t, root, query, protocol, "audit")
			verifyStorybookAuditSources(t, pack, files)
			direct, err := agent.BuildContext(agent.ContextRequest{Root: root, Query: query, BudgetTokens: 6000, MaxFiles: 20, ProtocolVersion: protocol, Mode: "audit"})
			if err != nil {
				t.Fatal(err)
			}
			evidence := evaluateStorybookAudit(pack, requirements)
			if !reflect.DeepEqual(evidence, evaluateStorybookAudit(direct, requirements)) || pack.Query != direct.Query {
				t.Fatal("MCP transport changed query or selected audit evidence")
			}
			if len(pack.SourceSections) == 0 && (!pack.FallbackRequired || pack.FallbackReason == "" || pack.SourceCoverage == "complete") {
				t.Fatalf("missing evidence must not be presented as complete: %+v", pack)
			}
			report := storybookAuditReport{Protocol: protocol, Query: query, SourceCoverage: pack.SourceCoverage, Fallback: pack.FallbackReason, Evidence: evidence, Complete: storybookAuditComplete(evidence)}
			if report.Protocol == "" {
				report.Protocol = "strict"
			}
			reports = append(reports, report)
			t.Logf("%s: complete=%t; source_coverage=%s; fallback=%s", report.Protocol, report.Complete, report.SourceCoverage, report.Fallback)
			for _, item := range evidence {
				if !item.Covered {
					t.Logf("  %s: missing=%v; bounded omissions=%v", item.ID, item.Missing, item.Omitted)
				}
			}
		}
	}
	after, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("context requests modified the generated index")
	}
	if destination := os.Getenv("GOREGRAPH_STORYBOOK_AUDIT_REPORT"); destination != "" {
		body, err := json.MarshalIndent(reports, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, append(body, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Full audit evidence is mandatory; normal production-mode safety is checked separately.
	{
		for _, report := range reports {
			if !report.Complete {
				t.Errorf("Storybook audit acceptance not met: protocol=%s query=%q", report.Protocol, report.Query)
			}
		}
	}
}

func verifyStorybookAuditSources(t *testing.T, pack agent.ContextPack, files map[string]string) {
	t.Helper()
	for _, section := range pack.SourceSections {
		content, ok := files[section.Path]
		if !ok {
			t.Fatalf("unrecognized source %s", section.Path)
		}
		lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
		if section.StartLine < 1 || section.EndLine < section.StartLine || section.EndLine > len(lines) {
			t.Fatalf("invalid source range: %+v", section)
		}
		for _, rendered := range strings.Split(section.Content, "\n") {
			prefix, line, ok := strings.Cut(rendered, "\t")
			if !ok {
				t.Fatalf("source line has no number: %q", rendered)
			}
			var number int
			if _, err := fmt.Sscan(prefix, &number); err != nil || number < section.StartLine || number > section.EndLine || lines[number-1] != line {
				t.Fatalf("source differs from fixture: %s %q", section.Path, rendered)
			}
		}
	}
}

func TestStorybookAuditConfigurationEvidence(t *testing.T) {
	root, files, requirements := loadStorybookAuditFixture(t)
	buildStorybookAuditFixture(t, root)
	for _, query := range []string{
		"Welche Storybook-Erweiterungen fehlen? Prüfe Konfiguration und automatische Accessibility-Prüfungen.",
		"Prüfe den Stand für Version 1.4.3. Welche Storybook-Erweiterungen fehlen? Prüfe Konfiguration und automatische Accessibility-Prüfungen.",
	} {
		pack := storybookAuditContext(t, root, query, "adaptive-v2")
		verifyStorybookAuditSources(t, pack, files)
		for _, evidence := range evaluateStorybookAudit(pack, requirements[:2]) {
			if !evidence.Covered {
				t.Errorf("%q: missing %s: %v", query, evidence.ID, evidence.Missing)
			}
		}
		if !pack.FallbackRequired || pack.SourceCoverage != "partial" || len(pack.Entrypoints) != 0 {
			t.Errorf("configuration must remain partial candidate evidence: %+v", pack)
		}
	}
}

func TestStorybookAuditConfigurationBudgetsAndChanges(t *testing.T) {
	root, files, _ := loadStorybookAuditFixture(t)
	buildStorybookAuditFixture(t, root)
	query := "Prüfe Version 1.4.3. Welche Storybook-Konfiguration und Accessibility-Prüfungen sind vorhanden?"
	request := agent.ContextRequest{Root: root, Query: query, ProtocolVersion: "adaptive-v2", BudgetTokens: 6000, MaxFiles: 12}
	initial, err := agent.BuildContext(request)
	if err != nil || len(initial.SourceSections) != 2 {
		t.Fatalf("initial evidence: %+v, %v", initial, err)
	}
	for _, section := range initial.SourceSections {
		if section.ReadReceipt == "" || section.Role != "candidate" {
			t.Fatalf("missing receipt or incorrect role: %+v", section)
		}
	}
	for _, budget := range []int{256, 400, 700, 1200, 6000} {
		boundedRequest := request
		boundedRequest.BudgetTokens, boundedRequest.MaxFiles = budget, 1
		pack, err := agent.BuildContext(boundedRequest)
		if err != nil || pack.EstimatedTokens > budget || len(pack.SourceSections) > 1 || pack.SourceCoverage == "complete" {
			t.Fatalf("budget/file limit %d: %+v, %v", budget, pack, err)
		}
		verifyStorybookAuditSources(t, pack, files)
		if budget == 6000 && (len(pack.SourceSections) != 1 || len(pack.SourceOmissions) != 1 || len(pack.VerificationRequests) != 1) {
			t.Fatalf("omitted configuration lacks bounded verification: %+v", pack)
		}
	}
	strict := storybookAuditContext(t, root, query, "")
	if len(strict.SourceSections) != 0 || !strict.FallbackRequired {
		t.Fatalf("strict fallback changed: %+v", strict)
	}
	unrelated := storybookAuditContext(t, root, "Explain main configuration activation execution", "adaptive-v2")
	if len(unrelated.SourceSections) != 0 {
		t.Fatalf("unrequested Storybook evidence: %+v", unrelated)
	}
	request.PreviousContextID = initial.ContextID
	duplicate, err := agent.BuildContext(request)
	if err != nil || duplicate.DuplicateOf != initial.ContextID {
		t.Fatalf("unchanged evidence not deduplicated: %+v, %v", duplicate, err)
	}
	files[".storybook/preview.ts"] = strings.ReplaceAll(files[".storybook/preview.ts"], "test: 'off'", "test: 'error'")
	writeFile(t, root, ".storybook/preview.ts", files[".storybook/preview.ts"])
	changed, err := agent.BuildContext(request)
	if err != nil || changed.DuplicateOf != "" || changed.ContextID == initial.ContextID || changed.FallbackReason != "evidence_conflict" {
		t.Fatalf("changed activation hidden or treated as indexed: %+v, %v", changed, err)
	}
	verifyStorybookAuditSources(t, changed, files)
	found := false
	for _, section := range changed.SourceSections {
		if section.Path == ".storybook/preview.ts" && strings.Contains(section.Content, "test: 'error'") && section.SourceState == "current_source_changed_since_index" {
			found = true
		}
	}
	if !found {
		t.Fatalf("changed activation evidence missing: %+v", changed)
	}
}

func TestStorybookAuditStoryComponentFixtureEvidence(t *testing.T) {
	root, files, requirements := loadStorybookAuditFixture(t)
	result := buildStorybookAuditFixture(t, root)
	var index scan.AgentContextIndexRecord
	readStorybookAuditJSON(t, filepath.Join(result.OutputDir, "agent/context-index.json"), &index)
	facts := map[string]scan.AgentContextFactRecord{}
	storyID := ""
	for _, fact := range index.Facts {
		facts[fact.ID] = fact
		if fact.Kind == "storybook_story" && fact.File == "src/ProductCard.stories.tsx" {
			storyID = fact.ID
		}
	}
	if storyID == "" {
		t.Error("story module has no source fact")
	}
	targets := map[string]bool{}
	for _, edge := range index.Edges {
		if edge.FromFactID == storyID && edge.Kind == "storybook_import" {
			if edge.Confidence != "EXACT" || edge.Line < 1 {
				t.Errorf("unproven import edge: %+v", edge)
			}
			targets[facts[edge.ToFactID].File] = true
		}
	}
	for _, path := range []string{"src/ProductCard.tsx", "src/productFixture.ts"} {
		if !targets[path] {
			t.Errorf("story lacks resolved import to %s", path)
		}
	}
	for _, prefix := range []string{"", "Prüfe Version 1.4.3. "} {
		query := prefix + "Welche Storybook-Erweiterungen fehlen? Prüfe Konfiguration, Stories, Komponenten und Fixtures."
		pack := storybookAuditContext(t, root, query, "adaptive-v2")
		verifyStorybookAuditSources(t, pack, files)
		for _, evidence := range evaluateStorybookAudit(pack, requirements[:3]) {
			if !evidence.Covered {
				t.Errorf("%q: missing %s: %v", query, evidence.ID, evidence.Missing)
			}
		}
		if pack.SourceCoverage != "partial" || !pack.FallbackRequired || len(pack.Entrypoints) != 0 {
			t.Errorf("story sources became a complete production answer: %+v", pack)
		}
	}
	for _, protocol := range []string{"", "adaptive-v2"} {
		for _, query := range []string{"Explain src/orders.ts orderTotal price quantity calculation", "Explain src/ProductCard.tsx ProductCard", "Explain src/productFixture.ts productFixture"} {
			pack := storybookAuditContext(t, root, query, protocol)
			verifyStorybookAuditSources(t, pack, files)
			if pack.FallbackRequired || pack.SourceCoverage != "complete" {
				t.Errorf("production lookup changed: %+v", pack)
			}
			for _, section := range pack.SourceSections {
				if strings.Contains(section.Path, ".stories.") || strings.Contains(section.Path, ".storybook/") {
					t.Errorf("production lookup includes Storybook: %+v", section)
				}
			}
		}
	}
}

func TestStorybookAuditImportsFollowBindingsAndRejectMissingTargets(t *testing.T) {
	for _, scenario := range []string{"alias", "missing", "ambiguous", "barrel"} {
		t.Run(scenario, func(t *testing.T) {
			root, files, _ := loadStorybookAuditFixture(t)
			story := files["src/ProductCard.stories.tsx"]
			want := ""
			switch scenario {
			case "alias":
				files["src/alternativeFixture.ts"] = "export function productFixture() { return { title: 'alternative' }; }\n"
				story = strings.ReplaceAll(story, "import { productFixture } from './productFixture';", "import { productFixture as makeFixture } from './alternativeFixture';")
				story = strings.ReplaceAll(story, "args: productFixture()", "args: makeFixture()")
				want = "src/alternativeFixture.ts"
			case "missing":
				story = strings.ReplaceAll(story, "from './productFixture'", "from './missingFixture'")
			case "ambiguous":
				files["src/productFixture.tsx"] = files["src/productFixture.ts"]
			case "barrel":
				files["src/fixtureBarrel.ts"] = "export { productFixture } from './productFixture';\n"
				story = strings.ReplaceAll(story, "from './productFixture'", "from './fixtureBarrel'")
			}
			files["src/ProductCard.stories.tsx"] = story
			for path, body := range files {
				writeFile(t, root, path, body)
			}
			buildStorybookAuditFixture(t, root)
			pack := storybookAuditContext(t, root, "Prüfe Storybook Stories, Komponenten und Fixtures.", "adaptive-v2")
			verifyStorybookAuditSources(t, pack, files)
			found := false
			for _, section := range pack.SourceSections {
				if section.Path == "src/productFixture.ts" || section.Path == "src/productFixture.tsx" {
					t.Errorf("unproven fixture link: %+v", section)
				}
				if section.Path == want {
					found = true
				}
			}
			if want != "" && !found {
				t.Errorf("aliased import lost provider %s: %+v", want, pack)
			}
			if pack.SourceCoverage != "partial" || !pack.FallbackRequired {
				t.Errorf("incomplete story analysis claims completeness: %+v", pack)
			}
		})
	}
}

func TestStorybookAuditStoryChangesAndBudgets(t *testing.T) {
	root, files, _ := loadStorybookAuditFixture(t)
	buildStorybookAuditFixture(t, root)
	query := "Prüfe Storybook Stories, Komponenten und Fixtures."
	request := agent.ContextRequest{Root: root, Query: query, ProtocolVersion: "adaptive-v2", BudgetTokens: 6000, MaxFiles: 12}
	initial, err := agent.BuildContext(request)
	if err != nil || len(initial.SourceSections) != 5 {
		t.Fatalf("initial bundle: %+v, %v", initial, err)
	}
	for _, budget := range []int{256, 400, 700, 1200, 6000} {
		boundedRequest := request
		boundedRequest.BudgetTokens, boundedRequest.MaxFiles = budget, 1
		bounded, err := agent.BuildContext(boundedRequest)
		if err != nil || bounded.EstimatedTokens > budget || len(bounded.SourceSections) > 1 || bounded.SourceCoverage == "complete" {
			t.Fatalf("budget %d: %+v, %v", budget, bounded, err)
		}
		verifyStorybookAuditSources(t, bounded, files)
		for _, omission := range bounded.SourceOmissions {
			if omission.StartLine < 1 || omission.EndLine < omission.StartLine || files[omission.Path] == "" {
				t.Errorf("unbounded or invented omission: %+v", omission)
			}
		}
	}
	request.PreviousContextID = initial.ContextID
	duplicate, err := agent.BuildContext(request)
	if err != nil || duplicate.DuplicateOf != initial.ContextID {
		t.Fatalf("bundle is not deterministic: %+v, %v", duplicate, err)
	}
	files["src/ProductCard.stories.tsx"] = strings.ReplaceAll(files["src/ProductCard.stories.tsx"], "from './productFixture'", "from './missingFixture'")
	writeFile(t, root, "src/ProductCard.stories.tsx", files["src/ProductCard.stories.tsx"])
	changed, err := agent.BuildContext(request)
	if err != nil || changed.FallbackReason != "evidence_conflict" || changed.ContextID == initial.ContextID || changed.DuplicateOf != "" {
		t.Fatalf("changed story reused stale imports: %+v, %v", changed, err)
	}
	verifyStorybookAuditSources(t, changed, files)
	for _, section := range changed.SourceSections {
		if section.Path == "src/productFixture.ts" || section.Path == "src/ProductCard.tsx" {
			t.Errorf("changed story still follows indexed imports: %+v", section)
		}
	}
}

func TestStorybookAuditSelectsNamedStoryFromLaterSentence(t *testing.T) {
	root, files, _ := loadStorybookAuditFixture(t)
	files["src/OtherCard.stories.tsx"] = strings.ReplaceAll(files["src/ProductCard.stories.tsx"], "title: 'Products/ProductCard'", "title: 'Products/OtherCard'")
	writeFile(t, root, "src/OtherCard.stories.tsx", files["src/OtherCard.stories.tsx"])
	buildStorybookAuditFixture(t, root)
	pack := storybookAuditContext(t, root, "Prüfe Version 1.4.3. Zeige die Storybook Stories und Fixtures für OtherCard.", "adaptive-v2")
	verifyStorybookAuditSources(t, pack, files)
	found := false
	for _, section := range pack.SourceSections {
		if section.Path == "src/ProductCard.stories.tsx" {
			t.Error("selected an unrelated story")
		}
		if section.Path == "src/OtherCard.stories.tsx" {
			found = true
		}
	}
	if !found {
		t.Fatalf("named story missing: %+v", pack)
	}
}

func TestStorybookCompleteAudit(t *testing.T) {
	root, files, requirements := loadStorybookAuditFixture(t)
	buildStorybookAuditFixture(t, root)
	for _, protocol := range []string{"", "adaptive-v2"} {
		for _, prefix := range []string{"", "Prüfe den Stand für Version 1.4.3. "} {
			query := prefix + "Welche Storybook-Erweiterungen fehlen? Berücksichtige Konfiguration, Stories, Fixtures, Test-Runner, CI und Dokumentation. Prüfe automatische Accessibility-Prüfungen, visuelle Referenzbilder und erfolgreiche Ausführung."
			pack := storybookAuditContextMode(t, root, query, protocol, "audit")
			verifyStorybookAuditSources(t, pack, files)
			for _, item := range evaluateStorybookAudit(pack, requirements) {
				if !item.Covered {
					t.Errorf("%s %q: missing %s: %v", protocol, query, item.ID, item.Missing)
				}
			}
			if pack.FallbackRequired || len(pack.Entrypoints) != 0 {
				t.Errorf("audit incorrectly requires a production entrypoint: %+v", pack)
			}
			for _, section := range pack.SourceSections {
				if section.Path == ".gitlab/unreferenced.yml" || section.Path == "src/orders.ts" {
					t.Errorf("unrelated source included: %s", section.Path)
				}
			}
		}
	}
}
