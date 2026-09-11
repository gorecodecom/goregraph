package agent

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

// contextAdaptivePlanTestFiles adds navigation identities; it grants no source-read or proof authority.
func contextAdaptivePlanTestFiles(pack ContextPack, index scan.AgentContextIndexRecord, facts []scan.AgentContextFactRecord, projects map[string]bool, entrypoint string) []ContextPlanFile {
	explicit := contextExplicitProjects(contextSelectionQuery(pack), contextProjectAliases(index.Facts, index.Coverage))
	inScope := func(project string) bool { return projects[project] && (len(explicit) == 0 || explicit[project]) }
	effects := contextTestInventoryEffectFamilies(pack, index)
	ordinary := make([]scan.AgentContextFactRecord, 0, len(facts))
	sideEffects := make([]contextPlanFileFact, 0)
	for _, fact := range facts {
		project := normalizeContextProject(fact.Project)
		if project != entrypoint && !inScope(project) {
			continue
		}
		identity := contextTestInventoryIdentity(fact.File)
		recognized, relevant := false, false
		for _, effect := range effects {
			if effect.project != project || !contextTestInventoryContains(identity, effect.identity) {
				continue
			}
			recognized = true
			relevant = relevant || effect.selected
		}
		if recognized {
			if relevant && inScope(project) {
				sideEffects = append(sideEffects, contextPlanFileFact{fact: fact, use: "side_effect_test"})
			}
		} else {
			ordinary = append(ordinary, fact)
		}
	}
	sort.Slice(sideEffects, func(i, j int) bool {
		return contextPlanFileFactBetter(pack, index, sideEffects[i].fact, &sideEffects[j].fact)
	})
	selected := contextPlanFileProviderTests(pack, index, ordinary, projects)
	selected = append(selected, sideEffects...)
	selected = append(selected, contextPlanFileCallerPatternPair(pack, index, ordinary, entrypoint)...)
	represented := contextPlanFileRepresentedPaths(pack)
	result := make([]ContextPlanFile, 0, maximumContextPlanFiles)
	for _, candidate := range selected {
		key := contextEvidenceInventoryPathKey(candidate.fact.Project, candidate.fact.File)
		if represented[key] {
			continue
		}
		represented[key] = true
		result = append(result, ContextPlanFile{Project: normalizeContextProject(candidate.fact.Project), Path: contextExactInventoryPath(candidate.fact.File), Use: candidate.use})
		if len(result) == maximumContextPlanFiles {
			break
		}
	}
	return result
}

type contextTestInventoryEffectFamily struct {
	project  string
	identity map[string]bool
	selected bool
}

func contextTestInventoryEffectFamilies(pack ContextPack, index scan.AgentContextIndexRecord) []contextTestInventoryEffectFamily {
	selectedIDs := make(map[string]bool)
	for _, id := range pack.selectedSourceFactIDs {
		selectedIDs[id] = true
	}
	for _, contract := range pack.Contracts {
		if strings.EqualFold(contract.Confidence, "EXACT") {
			selectedIDs[contract.ID] = true
		}
	}
	effectPaths := make(map[string]bool)
	for _, fact := range index.Facts {
		if strings.EqualFold(fact.Kind, "side_effects") && !contextFactUsesTestSource(fact) && contextExactInventoryPath(fact.File) != "" {
			effectPaths[contextEvidenceInventoryPathKey(fact.Project, fact.File)] = true
		}
	}
	anchors := make(map[string][]map[string]bool)
	declarations := make([]scan.AgentContextFactRecord, 0)
	for _, fact := range index.Facts {
		if !strings.EqualFold(fact.Confidence, "EXACT") || contextFactUsesTestSource(fact) || contextExactInventoryPath(fact.File) == "" || contextPlanFileConfigurationSource(fact.File) {
			continue
		}
		if contextTestInventoryFactSelected(pack, fact, selectedIDs[fact.ID]) {
			project := normalizeContextProject(fact.Project)
			anchors[project] = append(anchors[project], contextTestInventoryIdentity(fact.File))
		}
		if effectPaths[contextEvidenceInventoryPathKey(fact.Project, fact.File)] {
			declarations = append(declarations, fact)
		}
	}
	result := make([]contextTestInventoryEffectFamily, 0, len(declarations))
	seen := make(map[string]bool)
	for _, fact := range declarations {
		key := contextEvidenceInventoryPathKey(fact.Project, fact.File)
		if seen[key] {
			continue
		}
		seen[key] = true
		family := contextTestInventoryEffectFamily{project: normalizeContextProject(fact.Project), identity: contextTestInventoryIdentity(fact.File)}
		for _, anchor := range anchors[family.project] {
			common := 0
			for token := range family.identity {
				if anchor[token] {
					common++
				}
			}
			if common >= 2 && len(family.identity) > 0 {
				family.selected = true
				break
			}
		}
		result = append(result, family)
	}
	return result
}

func contextTestInventoryIdentity(path string) map[string]bool {
	result := make(map[string]bool)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, token := range contextTokens(name) {
		switch token {
		case "base", "abstract", "entity", "model", "record", "service", "controller", "test", "tests", "integration", "regression", "failure":
			continue
		}
		result[token] = true
	}
	return result
}

func contextTestInventoryContains(identity, family map[string]bool) bool {
	if len(family) < 2 {
		return false
	}
	for token := range family {
		if !identity[token] {
			return false
		}
	}
	return true
}

// A current returned declaration is selected evidence even when it was not an initial source seed.
func contextTestInventoryFactSelected(pack ContextPack, fact scan.AgentContextFactRecord, selected bool) bool {
	for _, section := range pack.SourceSections {
		if contextEvidenceInventoryPathKey(section.Project, section.Path) != contextEvidenceInventoryPathKey(fact.Project, fact.File) {
			continue
		}
		if section.SourceState != "indexed_range_current" {
			return false
		}
		if section.StartLine > 0 && section.EndLine >= section.StartLine && fact.Line >= section.StartLine && fact.Line <= section.EndLine {
			selected = true
		}
	}
	return selected
}
