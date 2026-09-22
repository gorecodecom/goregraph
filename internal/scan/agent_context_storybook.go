package scan

import (
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// IsStorybookStorySource identifies conventional script story modules while
// retaining the scanner's generated and archived source exclusions.
func IsStorybookStorySource(value string) bool {
	if isLowSignalCodeFile(value) {
		return false
	}
	extension := filepath.Ext(value)
	switch extension {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		return strings.HasSuffix(strings.TrimSuffix(value, extension), ".stories")
	default:
		return false
	}
}

func extractAgentContextStoryFacts(file FileRecord, body string) []AgentContextFactRecord {
	if !IsStorybookStorySource(file.Path) || strings.TrimSpace(body) == "" {
		return nil
	}
	path := contextPathKey(file.Path)
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	return []AgentContextFactRecord{{
		ID: stableID("agent-context-story", path), Kind: "storybook_story",
		Name: contextFileBase(path), Qualified: path, File: path,
		Line: 1, EndLine: strings.Count(strings.TrimSuffix(normalized, "\n"), "\n") + 1,
		Summary:    "Story source; configured inclusion and execution are not established",
		Confidence: string(ConfidenceExact), Search: compactContextSearch("storybook story", path),
	}}
}

func appendAgentContextStoryFacts(index AgentContextIndexRecord, project string, stories []AgentContextFactRecord, symbols ProjectSymbolFacts) AgentContextIndexRecord {
	if len(stories) == 0 {
		return index
	}
	byFile := map[string]AgentContextFactRecord{}
	for _, story := range stories {
		story.Project = project
		byFile[story.File] = story
		index.Facts = append(index.Facts, story)
	}
	declarations := map[string]RichSymbolRecord{}
	for _, declaration := range symbols.Declarations {
		declarations[declaration.ID] = declaration
	}
	factIDs := map[string]string{}
	for _, fact := range index.Facts {
		key := fact.File + "\x00" + fact.Qualified + "\x00" + strconv.Itoa(fact.Line)
		factIDs[key] = fact.ID
	}
	for _, reference := range symbols.References {
		story, found := byFile[reference.From]
		if !found || reference.Type != "imports_value" || reference.Resolution != SymbolResolutionExact || !reference.Internal || reference.NonPromotable {
			continue
		}
		declaration, found := declarations[reference.ToSymbolID]
		if !found || isLowSignalCodeFile(declaration.File) || !directStoryImport(story.File, reference.TargetModule, declaration.File) {
			continue
		}
		key := declaration.File + "\x00" + declaration.QualifiedName + "\x00" + strconv.Itoa(declaration.Line)
		targetID := factIDs[key]
		if targetID == "" {
			continue
		}
		index.Edges = append(index.Edges, AgentContextEdgeRecord{
			ID:      stableID("agent-context-story-import", story.ID, targetID, reference.ID),
			Project: project, FromFactID: story.ID, ToFactID: targetID,
			FromLabel: story.Name, ToLabel: declaration.Name,
			Kind: "storybook_import", File: story.File, Line: reference.Line,
			Reason:     "exact static story import; runtime use and configured inclusion are not established",
			Confidence: string(ConfidenceExact),
		})
	}
	sortAgentContextFacts(index.Facts)
	sortAgentContextEdges(index.Edges)
	return index
}

// Direct imports need no unrendered barrel or alias configuration to establish
// their source identity. Those indirect forms remain outside this projection.
func directStoryImport(story, module, target string) bool {
	if !strings.HasPrefix(module, "./") && !strings.HasPrefix(module, "../") {
		return false
	}
	base := path.Clean(path.Join(path.Dir(story), module))
	stem := strings.TrimSuffix(target, path.Ext(target))
	return base == target || base == stem || path.Join(base, "index") == stem
}
