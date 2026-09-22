package mcp

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agent"
	"github.com/gorecodecom/goregraph/internal/query"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestCompleteAuditBudgetsQueryAndChanges(t *testing.T) {
	root, files, _ := loadStorybookAuditFixture(t)
	result := buildStorybookAuditFixture(t, root)
	fullQuery := strings.Repeat("Prüfe Überprüfung. ", 110) + "Storybook, Playwright, Vitest und CI: Konfiguration, Runner, Stories und Dokumentation."
	request := agent.ContextRequest{Root: root, Mode: "audit", Query: fullQuery, BudgetTokens: 6000, MaxFiles: 20}
	initial, err := agent.BuildContext(request)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(fullQuery))
	if initial.Audit == nil || initial.Audit.QueryHash != fmt.Sprintf("%x", hash) || !initial.Audit.QueryTruncated || initial.Audit.Execution != "unknown" || len(initial.SourceSections) == 0 {
		t.Fatalf("lost full query or honest execution state: %+v", initial)
	}
	wire := storybookAuditContextMode(t, root, fullQuery, "", "audit")
	if wire.ContextID != initial.ContextID {
		t.Fatal("MCP lost the full query")
	}
	rendered, err := query.RunContext(query.ContextOptions{Root: root, Mode: "audit", Query: fullQuery, BudgetTokens: 6000, MaxFiles: 20, Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	var cliPack agent.ContextPack
	if err := json.Unmarshal([]byte(rendered), &cliPack); err != nil || cliPack.ContextID != initial.ContextID {
		t.Fatalf("CLI query transport: %v", err)
	}
	for _, protocol := range []string{"", "adaptive-v2"} {
		for _, budget := range []int{256, 400, 700, 1200, 4000, 6000} {
			boundedRequest := request
			boundedRequest.ProtocolVersion, boundedRequest.BudgetTokens, boundedRequest.MaxFiles = protocol, budget, 1
			bounded, err := agent.BuildContext(boundedRequest)
			if err != nil || bounded.EstimatedTokens > budget || len(bounded.SourceSections) > 1 || bounded.SourceCoverage == "complete" {
				t.Fatalf("%s budget %d: %+v, %v", protocol, budget, bounded, err)
			}
			verifyStorybookAuditSources(t, bounded, files)
			for _, omission := range bounded.SourceOmissions {
				if files[omission.Path] == "" || omission.StartLine < 1 || omission.EndLine < omission.StartLine {
					t.Fatalf("unbounded omission: %+v", omission)
				}
			}
			if len(bounded.SourceOmissions) == 0 && bounded.SourceUnrepresented == 0 {
				t.Fatal("lost missing-source accounting")
			}
		}
	}
	request.PreviousContextID = initial.ContextID
	duplicate, err := agent.BuildContext(request)
	if err != nil || duplicate.DuplicateOf != initial.ContextID || len(duplicate.SourceSections) != 0 {
		t.Fatalf("duplicate: %+v, %v", duplicate, err)
	}
	request.MaxFiles = 1
	narrower, err := agent.BuildContext(request)
	if err != nil || narrower.DuplicateOf != "" {
		t.Fatalf("budget change reused duplicate: %+v, %v", narrower, err)
	}
	request.MaxFiles = 20
	files[".gitlab-ci.yml"] = "stages: [build]\n# Removed the local include.\n"
	writeFile(t, root, ".gitlab-ci.yml", files[".gitlab-ci.yml"])
	changed, err := agent.BuildContext(request)
	if err != nil || changed.ContextID == initial.ContextID || changed.DuplicateOf != "" {
		t.Fatalf("stale CI identity: %+v, %v", changed, err)
	}
	verifyStorybookAuditSources(t, changed, files)
	for _, section := range changed.SourceSections {
		if section.Path == ".gitlab/storybook.yml" {
			t.Fatal("followed removed CI include")
		}
	}
	for _, link := range changed.Audit.Links {
		if strings.Contains(link.From, ".gitlab-ci.yml") {
			t.Fatal("reported stale CI link")
		}
	}
	indexPath := filepath.Join(result.OutputDir, "agent/context-index.json")
	var index scan.AgentContextIndexRecord
	readStorybookAuditJSON(t, indexPath, &index)
	index.AuditVersion = 0
	body, _ := json.Marshal(index)
	if err := os.WriteFile(indexPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	unavailable, err := agent.BuildContext(request)
	if err != nil || unavailable.FallbackReason != "audit_index_unavailable" || unavailable.SourceCoverage != "none" {
		t.Fatalf("legacy index: %+v, %v", unavailable, err)
	}
}

func TestCompleteAuditNestedCIAndManyStories(t *testing.T) {
	root, files, _ := loadStorybookAuditFixture(t)
	files[".gitlab/storybook.yml"] += "\ninclude:\n  - local: '/.gitlab/nested.yml'\n  - project: external/ci\n    file: /jobs.yml\n  - local: '$CI_PROJECT_DIR/dynamic.yml'\n  - local: '/.gitlab/missing.yml'\n"
	files[".gitlab/nested.yml"] = "include:\n  - local: '/.gitlab/storybook.yml'\ncontract:\n  script: echo checked\n"
	for i := 0; i < 100; i++ {
		files[fmt.Sprintf("src/Extra%03d.stories.tsx", i)] = "export default { title: 'Extra' };\nexport const Ready = {};\n"
	}
	for path, body := range files {
		writeFile(t, root, path, body)
	}
	buildStorybookAuditFixture(t, root)
	pack := storybookAuditContextMode(t, root, "Storybook Playwright Vitest CI Stories Dokumentation", "", "audit")
	verifyStorybookAuditSources(t, pack, files)
	seen := map[string]bool{}
	for _, section := range pack.SourceSections {
		seen[section.Path] = true
	}
	for _, path := range []string{".gitlab/nested.yml", ".storybook/main.ts", "docs/storybook.md", "package.json", "tests/storybook/playwright.config.ts"} {
		if !seen[path] {
			t.Errorf("area starved by stories: %s", path)
		}
	}
	if seen[".gitlab/unreferenced.yml"] {
		t.Error("unreferenced CI promoted to evidence")
	}
	unknown := strings.Join(pack.Audit.Unknown, "\n")
	for _, reason := range []string{"external or conditional", "dynamic CI include", "missing or excluded"} {
		if !strings.Contains(unknown, reason) {
			t.Errorf("missing uncertainty %s: %s", reason, unknown)
		}
	}
	if pack.SourceUnrepresented == 0 && len(pack.SourceOmissions) == 0 {
		t.Error("large inventory lost omissions")
	}
	if pack.Audit.Execution != "unknown" || pack.SourceCoverage == "complete" {
		t.Error("configuration became execution/completeness proof")
	}
}

func TestCompleteAuditWorkspaceDeploymentTriggers(t *testing.T) {
	root := t.TempDir()
	projects := map[string]map[string]string{
		"frontend": {
			".gitlab-ci.yml":     "include:\n  - local: '/.gitlab/deploy.yml'\n",
			".gitlab/deploy.yml": "deploy-test:\n  stage: deploy\n  environment: TEST\n  script: deploy frontend\nplaywright-test:\n  needs: [deploy-test]\n  trigger:\n    project: qa/playwright\n    branch: master\n  variables:\n    APPS: 'WPO VD'\n    TARGET: TEST\n    FRONTEND_REF: release\n  allow_failure: true\n",
		},
		"backend": {
			".gitlab-ci.yml":     "include:\n  - local: '/.gitlab/deploy.yml'\n  - project: shared/deployment\n    file: /templates.yml\n",
			".gitlab/deploy.yml": "deploy-int:\n  stage: deploy\n  environment: INT\n  script: deploy backend\nplaywright-int:\n  needs: [deploy-int]\n  trigger:\n    project: qa/playwright\n    branch: master\n  variables:\n    SERVICE: shared-orders\n    APPS: 'WPO VD'\n    TARGET: INT\n  allow_failure: true\n",
		},
		"playwright": {
			"playwright.config.ts": "export default { projects: [{ name: 'WPO' }, { name: 'VD' }] };\n",
			"docs/playwright.md":   "Frontend release is deployed to TEST. The separate Playwright master branch tests that deployment. shared-orders is consumed by WPO and VD; wpo-only is consumed by WPO. Trigger configuration is not proof of a completed pipeline.\n",
		},
	}
	var indexes []scan.AgentContextIndexRecord
	registry := scan.WorkspaceRegistryRecord{}
	for _, project := range []string{"frontend", "backend", "playwright"} {
		projectRoot := filepath.Join(root, project)
		for path, body := range projects[project] {
			writeFile(t, projectRoot, path, body)
		}
		result := buildStorybookAuditFixture(t, projectRoot)
		var index scan.AgentContextIndexRecord
		readStorybookAuditJSON(t, filepath.Join(result.OutputDir, "agent/context-index.json"), &index)
		index.Root = project
		indexes = append(indexes, index)
		registry.Projects = append(registry.Projects, scan.WorkspaceProjectRecord{Path: project, Indexed: true})
	}
	// A registered, unavailable project must not disappear from the coverage warning.
	registry.Projects = append(registry.Projects, scan.WorkspaceProjectRecord{Path: "unavailable", Indexed: false})
	merged := scan.BuildWorkspaceAgentContextIndex(registry, indexes, nil, nil, scan.WorkspaceEndpointTraceIndexRecord{}, scan.APICatalogRecord{}, "2026-09-22T00:00:00Z")
	body, err := json.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".goregraph-workspace/agent/context-index.json", string(body))
	for _, protocol := range []string{"", "adaptive-v2"} {
		pack := storybookAuditContextMode(t, root, "Prüfe die Playwright CI Trigger nach INT und TEST Deployments, Service-zu-App-Mehrfachzuordnung, release und master sowie nicht blockierende Fehler.", protocol, "audit")
		found := map[string]string{}
		for _, section := range pack.SourceSections {
			original := projects[section.Project][section.Path]
			if original == "" {
				t.Fatalf("invented project/source: %+v", section)
			}
			found[section.Project+"/"+section.Path] = section.Content
		}
		for _, project := range []string{"frontend", "backend"} {
			if !strings.Contains(found[project+"/.gitlab/deploy.yml"], "APPS: 'WPO VD'") || !strings.Contains(found[project+"/.gitlab/deploy.yml"], "allow_failure: true") {
				t.Errorf("missing scoped trigger evidence: %s", project)
			}
		}
		if !strings.Contains(found["frontend/.gitlab/deploy.yml"], "FRONTEND_REF: release") || !strings.Contains(found["frontend/.gitlab/deploy.yml"], "branch: master") || !strings.Contains(found["playwright/docs/playwright.md"], "wpo-only") {
			t.Error("lost branch or service mapping evidence")
		}
		if !strings.Contains(strings.Join(pack.Audit.Unknown, "\n"), "cross-project coverage is incomplete") || !strings.Contains(strings.Join(pack.Audit.Unknown, "\n"), "external or conditional") {
			t.Errorf("missing workspace uncertainty: %+v", pack.Audit)
		}
		if pack.FallbackRequired || len(pack.Entrypoints) != 0 || pack.Audit.Execution != "unknown" {
			t.Fatalf("workspace audit contract: %+v", pack)
		}
		for _, link := range pack.Audit.Links {
			if found[link.From] == "" || found[link.To] == "" {
				t.Fatalf("unproven cross-project link: %+v", link)
			}
		}
	}
}

func TestAuditMetadataDoesNotIncreaseProductionContextTokens(t *testing.T) {
	root, _, _ := loadStorybookAuditFixture(t)
	result := buildStorybookAuditFixture(t, root)
	indexPath := filepath.Join(result.OutputDir, "agent/context-index.json")
	var index scan.AgentContextIndexRecord
	readStorybookAuditJSON(t, indexPath, &index)
	original, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, protocol := range []string{"", "adaptive-v2"} {
		if err := os.WriteFile(indexPath, original, 0600); err != nil {
			t.Fatal(err)
		}
		request := agent.ContextRequest{Root: root, Query: "Explain src/orders.ts orderTotal price quantity calculation", ProtocolVersion: protocol}
		withAudit, err := agent.BuildContext(request)
		if err != nil {
			t.Fatal(err)
		}
		withoutIndex := index
		withoutIndex.AuditVersion, withoutIndex.AuditIncomplete, withoutIndex.AuditSources = 0, false, nil
		body, _ := json.Marshal(withoutIndex)
		if err := os.WriteFile(indexPath, body, 0600); err != nil {
			t.Fatal(err)
		}
		withoutAudit, err := agent.BuildContext(request)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(withoutAudit)
		after, _ := json.Marshal(withAudit)
		if string(before) != string(after) {
			t.Fatalf("%s audit metadata changed normal response: before %s; after %s", protocol, before, after)
		}
		if withAudit.Audit != nil || withAudit.Mode != "" || withAudit.SourceCoverage != "complete" {
			t.Fatalf("audit leaked into production: %+v", withAudit)
		}
		t.Logf("%s unchanged normal context: %d tokens, %d sources", protocol, withAudit.EstimatedTokens, len(withAudit.SourceSections))
	}
}

func TestCompleteAuditOmissionsUseCurrentShortenedFileBounds(t *testing.T) {
	root, files, _ := loadStorybookAuditFixture(t)
	buildStorybookAuditFixture(t, root)
	files["docs/storybook.md"] = "Storybook execution remains unknown.\n"
	writeFile(t, root, "docs/storybook.md", files["docs/storybook.md"])
	pack, err := agent.BuildContext(agent.ContextRequest{Root: root, Mode: "audit", Query: "Storybook CI", BudgetTokens: 6000, MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, omission := range pack.SourceOmissions {
		if omission.Path == "docs/storybook.md" {
			found = true
			if omission.StartLine != 1 || omission.EndLine != 1 {
				t.Fatalf("omission exceeds current source: %+v", omission)
			}
		}
	}
	if !found {
		t.Fatal("missing bounded documentation omission")
	}
}
