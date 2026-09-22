package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditLiteralReferencesAreBoundedAndConservative(t *testing.T) {
	files := map[string]string{
		".storybook/main.ts":           "// import './comment.ts'\n/* './comment.ts' */\nexport default { stories: ['../src/**/*.stories.tsx'], setup: './setup', computed: `./${name}.ts`, missing: './missing' };\n",
		".storybook/setup.ts":          "export default {};\n",
		".storybook/comment.ts":        "export default {};\n",
		"src/Card.stories.tsx":         "import { Card } from './Card';\nexport default { component: Card };\n",
		"src/Card.tsx":                 "export function Card() {}\n",
		"src/Card.ts":                  "export function Card() {}\n",
		"src/nested/Other.stories.tsx": "export default {};\n",
		"package.json":                 "{\n  \"scripts\": {\n    \"storybook:test\": \"vitest --config vitest.storybook.config.ts\"\n  }\n}\n",
		"vitest.storybook.config.ts":   "export default {};\n",
	}
	var sources []AgentAuditSource
	for file, body := range files {
		source, ok := extractAgentAuditSource(FileRecord{Path: file, Hash: "hash"}, body)
		if !ok {
			t.Fatal(file)
		}
		sources = append(sources, source)
	}
	sources = finalizeAgentAuditSources(sources, "frontend")
	for _, source := range sources {
		for _, ref := range source.References {
			if ref.File == ".storybook/comment.ts" || ref.File == "src/Card.ts" || ref.File == "src/Card.tsx" {
				t.Fatalf("comment or ambiguous reference resolved: %+v", ref)
			}
			if source.File == "package.json" && ref.Line != 3 {
				t.Fatalf("wrong script source line: %+v", ref)
			}
		}
		if source.File == ".storybook/main.ts" && len(source.References) != 3 {
			t.Fatalf("relative/glob links: %+v", source)
		}
		if source.File == "src/Card.stories.tsx" && !strings.Contains(strings.Join(source.Unknown, " "), "ambiguous") {
			t.Fatal("lost ambiguity")
		}
		encoded, _ := json.Marshal(source)
		if strings.Contains(string(encoded), "export default") || strings.Contains(string(encoded), "--config") {
			t.Fatal("source values copied into audit metadata")
		}
	}
	for _, file := range []string{"snapshot.png", "results.xml", "secrets.json", "Service.java"} {
		if _, ok := extractAgentAuditSource(FileRecord{Path: file}, "storybook"); ok {
			t.Errorf("unsupported source %s", file)
		}
	}
}

func TestAuditWorkspaceMetadataDoesNotMergeIdenticalPaths(t *testing.T) {
	builder := newWorkspaceAgentContextBuilder(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{{Path: "apps/shop", Indexed: true}, {Path: "apps/admin", Indexed: true}, {Path: "missing"}}})
	for _, project := range []string{"apps/shop", "apps/admin"} {
		builder.mergeProjectIndex(AgentContextIndexRecord{Root: project, AuditVersion: 1, AuditSources: []AgentAuditSource{{File: ".gitlab-ci.yml", Kind: "ci", Hash: project, References: []AgentAuditReference{{File: ".gitlab/jobs.yml", Kind: "include", Line: 2}}}}})
	}
	index := builder.index("")
	if index.AuditVersion != 1 || !index.AuditIncomplete || len(index.AuditSources) != 2 {
		t.Fatalf("lost audit completeness: %+v", index)
	}
	for _, source := range index.AuditSources {
		if source.Hash != source.Project || source.File != ".gitlab-ci.yml" || source.References[0].File != ".gitlab/jobs.yml" {
			t.Fatalf("lost project-relative identity: %+v", source)
		}
	}
}

func TestAuditGlobAndUnterminatedLiteralBoundaries(t *testing.T) {
	for _, item := range []struct {
		pattern, path string
		want          bool
	}{
		{"src/**/*.stories.tsx", "src/Card.stories.tsx", true},
		{"src/**/*.stories.tsx", "src/nested/Card.stories.tsx", true},
		{"src/*.stories.tsx", "src/nested/Card.stories.tsx", false},
		{"tests/case?.ts", "tests/case1.ts", true},
		{"tests/case?.ts", "tests/case11.ts", false},
	} {
		if got := compileAuditGlob(item.pattern).MatchString(item.path); got != item.want {
			t.Errorf("%s %s: %v", item.pattern, item.path, got)
		}
	}
	if refs := auditScriptPaths("import './unfinished.ts"); len(refs) != 0 {
		t.Fatalf("unterminated literal resolved: %+v", refs)
	}
}
