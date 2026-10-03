package agent

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestAuditContextRootScope(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeContextSourceFile(t, workspaceRoot, ".goregraph-workspace.yml", "projects: []\n")
	projects := []string{"apps/a-legacy", "apps/z-current", "apps/z-current-old"}
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-10-01T00:00:00Z",
		Root:          workspaceRoot,
		AuditVersion:  1,
		SourceHashes:  map[string]string{},
	}
	for _, project := range projects {
		for _, file := range []struct {
			path       string
			content    string
			kind       string
			topics     []string
			references []scan.AgentAuditReference
		}{
			{"package.json", "{\"scripts\":{\"test\":\"vitest run\"}}\n", "package", []string{"vitest"}, nil},
			{".gitlab-ci.yml", "include:\n  - local: '/.gitlab/tests.yml'\n", "ci", nil, []scan.AgentAuditReference{{File: ".gitlab/tests.yml", Kind: "include", Line: 2}}},
			{".gitlab/tests.yml", "test:\n  script: npm test\n", "ci", nil, nil},
		} {
			path := filepath.Join(project, file.path)
			writeContextSourceFile(t, workspaceRoot, path, file.content)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(file.content)))
			index.SourceHashes[filepath.ToSlash(path)] = hash
			index.AuditSources = append(index.AuditSources, scan.AgentAuditSource{
				Project: project, File: file.path, Kind: file.kind, Hash: hash,
				Topics: file.topics, References: file.references,
			})
		}
	}
	writeContextIndexAt(t, filepath.Join(workspaceRoot, ".goregraph-workspace", "agent", "context-index.json"), index)
	missingRoot := filepath.Join(workspaceRoot, "apps", "missing")
	if err := os.MkdirAll(missingRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, protocol := range []string{"strict-v1", AdaptiveV2} {
		for _, test := range []struct {
			name     string
			root     string
			projects []string
		}{
			{"project", filepath.Join(workspaceRoot, "apps", "z-current"), []string{"apps/z-current"}},
			{"workspace", workspaceRoot, projects},
			{"project_group", filepath.Join(workspaceRoot, "apps"), projects},
			{"unindexed_project", missingRoot, nil},
		} {
			t.Run(protocol+"/"+test.name, func(t *testing.T) {
				pack, err := BuildContext(ContextRequest{
					Root: test.root, Mode: "audit", Query: "Vitest CI",
					ProtocolVersion: protocol, BudgetTokens: 6000, MaxFiles: 20,
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(pack.SourceSections) != 3*len(test.projects) || len(pack.SourceOmissions) != 0 {
					t.Fatalf("unexpected scoped sources: sections=%+v, omissions=%+v", pack.SourceSections, pack.SourceOmissions)
				}
				seen := map[string]bool{}
				for _, section := range pack.SourceSections {
					seen[section.Project+"/"+section.Path] = true
				}
				for _, project := range test.projects {
					for _, file := range []string{"package.json", ".gitlab-ci.yml", ".gitlab/tests.yml"} {
						if !seen[project+"/"+file] {
							t.Errorf("missing scoped source %s/%s", project, file)
						}
					}
				}
				if len(test.projects) == 0 && (!pack.FallbackRequired || pack.FallbackReason != "audit_sources_unavailable") {
					t.Fatalf("unindexed project borrowed neighboring evidence: %+v", pack)
				}
			})
		}
	}
}
