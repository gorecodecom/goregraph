package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStorybookConfigurationFactsPreserveFileEvidence(t *testing.T) {
	body := "export default {\r\n  parameters: { a11y: { test: 'off' } },\r\n};\r\n"
	for _, path := range []string{".storybook/main.ts", "apps/shop/.storybook/preview.tsx", ".storybook/main.js", ".storybook/preview.jsx", ".storybook/main.mjs", ".storybook/main.cjs", ".storybook/main.mts", ".storybook/preview.cts"} {
		t.Run(path, func(t *testing.T) {
			facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body)
			if len(facts) != 1 || facts[0].Kind != "configuration" || facts[0].File != path || facts[0].Line != 1 || facts[0].EndLine != 3 {
				t.Fatalf("missing bounded configuration fact: %#v", facts)
			}
			encoded, _ := json.Marshal(facts)
			if strings.Contains(string(encoded), "'off'") || strings.Contains(string(encoded), "Spring") {
				t.Fatalf("source values or incorrect framework leaked into metadata: %s", encoded)
			}
		})
	}
	for _, path := range []string{"src/main.ts", "storybook/main.ts", ".storybook/helpers.ts", ".storybook/main.test.ts", "archive/.storybook/main.ts", "dist/.storybook/main.ts", "node_modules/pkg/.storybook/main.ts", ".storybook/main.d.ts"} {
		if facts := extractAgentContextConfigurationFacts(FileRecord{Path: path}, body); len(facts) != 0 {
			t.Errorf("unrelated or excluded file became configuration: %s", path)
		}
	}
}

func TestStorybookConfigurationFactsKeepWorkspaceProjectIdentity(t *testing.T) {
	builder := newWorkspaceAgentContextBuilder(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "apps/shop", Indexed: true}, {Path: "apps/admin", Indexed: true},
	}})
	for _, project := range []string{"apps/shop", "apps/admin"} {
		facts := extractAgentContextConfigurationFacts(FileRecord{Path: ".storybook/preview.ts"}, "export default { parameters: { a11y: { test: 'off' } } };\n")
		builder.mergeProjectIndex(AgentContextIndexRecord{Root: project, Facts: facts, SourceHashes: map[string]string{".storybook/preview.ts": project + "-hash"}})
	}
	index := builder.index("")
	if len(index.Facts) != 2 || index.Facts[0].ID == index.Facts[1].ID {
		t.Fatalf("configurations merged across projects: %+v", index.Facts)
	}
	for _, fact := range index.Facts {
		if fact.Kind != "configuration" || fact.File != ".storybook/preview.ts" || index.SourceHashes[fact.Project+"/"+fact.File] != fact.Project+"-hash" {
			t.Errorf("configuration lost its source identity: %+v", fact)
		}
	}
}

func TestStorybookStoryFactsKeepSourceBoundsAndExclusions(t *testing.T) {
	body := "import { Card } from './Card';\r\nexport default { component: Card };\r\nexport const Ready = {};\r\n"
	for _, path := range []string{"src/Card.stories.tsx", "src/Card.stories.jsx", "src/Card.stories.ts", "src/Card.stories.js", "src/Card.stories.mts", "src/Card.stories.cts", "src/Card.stories.mjs", "src/Card.stories.cjs"} {
		facts := extractAgentContextStoryFacts(FileRecord{Path: path}, body)
		if len(facts) != 1 || facts[0].Kind != "storybook_story" || facts[0].File != path || facts[0].Line != 1 || facts[0].EndLine != 3 {
			t.Errorf("missing bounded story source %s: %+v", path, facts)
		}
	}
	for _, path := range []string{"src/Card.tsx", "src/Card.story.tsx", "src/Card.stories.d.ts", "archive/Card.stories.tsx", "dist/Card.stories.tsx", "node_modules/Card.stories.tsx"} {
		if facts := extractAgentContextStoryFacts(FileRecord{Path: path}, body); len(facts) != 0 {
			t.Errorf("unrelated or excluded source %s", path)
		}
	}
	if facts := extractAgentContextStoryFacts(FileRecord{Path: "src/Card.stories.tsx"}, "\n"); len(facts) != 0 {
		t.Error("empty source produced story evidence")
	}
}

func TestStorybookImportsKeepWorkspaceProjectIdentity(t *testing.T) {
	builder := newWorkspaceAgentContextBuilder(WorkspaceRegistryRecord{Projects: []WorkspaceProjectRecord{
		{Path: "apps/shop", Indexed: true}, {Path: "apps/admin", Indexed: true},
	}})
	for _, project := range []string{"apps/shop", "apps/admin"} {
		story := extractAgentContextStoryFacts(FileRecord{Path: "src/Card.stories.tsx"}, "import { Card } from './Card';\nexport default { component: Card };\n")
		provider := AgentContextFactRecord{ID: "provider", Kind: "symbol", File: "src/Card.tsx", Qualified: "src/Card#Card", Line: 1}
		symbols := ProjectSymbolFacts{
			Declarations: []RichSymbolRecord{{ID: "symbol", File: provider.File, QualifiedName: provider.Qualified, Name: "Card", Line: 1}},
			References:   []RichRelationRecord{{ID: "import", From: story[0].File, Type: "imports_value", TargetModule: "./Card", ToSymbolID: "symbol", Resolution: SymbolResolutionExact, Internal: true, Line: 1}},
		}
		index := appendAgentContextStoryFacts(AgentContextIndexRecord{Root: project, Facts: []AgentContextFactRecord{provider}}, project, story, symbols)
		builder.mergeProjectIndex(index)
	}
	index := builder.index("")
	if len(index.Edges) != 2 {
		t.Fatalf("lost scoped imports: %+v", index.Edges)
	}
	facts := map[string]AgentContextFactRecord{}
	for _, fact := range index.Facts {
		facts[fact.ID] = fact
	}
	for _, edge := range index.Edges {
		from, to := facts[edge.FromFactID], facts[edge.ToFactID]
		if from.Project == "" || from.Project != to.Project || !strings.HasPrefix(edge.FromFactID, from.Project+"#") || !strings.HasPrefix(edge.ToFactID, to.Project+"#") || edge.Kind != "storybook_import" {
			t.Errorf("story import crosses project identities: %+v", edge)
		}
	}
}
