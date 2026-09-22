package agent

import (
	"sort"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const maximumStorybookFallbackSourceSections = 5

func contextQueryRequestsStorySources(query string) bool {
	tokens := contextTokenSet(query)
	return tokens["storybook"] && (tokens["story"] || tokens["stories"] || tokens["fixture"] || tokens["fixtures"])
}

// storybookFallbackStoryCandidates selects one bounded story bundle. Index
// import edges are used only while the current story still matches its hash.
func storybookFallbackStoryCandidates(loaded loadedContextIndex, query string, files map[string]sourceFile) ([]rankedContextFact, map[string]int) {
	priorities := map[string]int{}
	tokens := contextTokenSet(query)
	var stories []scan.AgentContextFactRecord
	scores := map[string]int{}
	byID := map[string]scan.AgentContextFactRecord{}
	for _, fact := range loaded.Index.Facts {
		byID[fact.ID] = fact
		if fact.Kind != "storybook_story" || !scan.IsStorybookStorySource(fact.File) {
			continue
		}
		identity := contextTokenSet(fact.Project + " " + fact.File)
		for token := range tokens {
			if token != "storybook" && token != "stories" && identity[token] {
				scores[fact.ID]++
			}
		}
		stories = append(stories, fact)
	}
	sort.Slice(stories, func(i, j int) bool {
		if scores[stories[i].ID] != scores[stories[j].ID] {
			return scores[stories[i].ID] > scores[stories[j].ID]
		}
		return stories[i].ID < stories[j].ID
	})
	if len(stories) == 0 {
		return nil, priorities
	}
	story := stories[0]
	ranked := []rankedContextFact{{fact: story}}
	priorities[story.ID] = 3
	candidate := sourceCandidate{Project: story.Project, Path: story.File}
	resolved, err := resolveSourcePath(loaded, candidate)
	if err != nil {
		return ranked, priorities
	}
	file, err := readSourceFile(resolved)
	if err != nil {
		return ranked, priorities
	}
	files[resolved] = file
	expected := adaptiveIndexedSourceHash(loaded, candidate)
	if expected == "" || file.Hash != expected {
		return ranked, priorities
	}
	var targets []scan.AgentContextFactRecord
	for _, edge := range loaded.Index.Edges {
		if edge.FromFactID != story.ID || edge.Kind != "storybook_import" || edge.Confidence != "EXACT" {
			continue
		}
		target, found := byID[edge.ToFactID]
		if !found || target.Project != story.Project || target.File == "" || target.Line < 1 || priorities[target.ID] != 0 {
			continue
		}
		priorities[target.ID] = 2
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	for _, target := range targets {
		ranked = append(ranked, rankedContextFact{fact: target})
	}
	return ranked, priorities
}
