package agent

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const maximumContextProductionPersistenceFiles = 2

type rankedContextProductionFile struct {
	path  string
	score int
}

func contextProductionPlanFiles(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) []ContextProductionPlanFiles {
	query := contextSelectionQuery(pack)
	if !contextQueryRequestsExactEvidenceInventory(query) ||
		!contextQueryPlansMissingTransition(query) {
		return nil
	}
	entrypointProject := contextPlanFileEntrypointProject(pack)
	if entrypointProject == "" {
		return nil
	}
	providerProjects := contextPlanFileProviderProjects(pack, entrypointProject)
	if len(providerProjects) == 0 {
		return nil
	}
	represented := contextPlanFileRepresentedPaths(pack)
	domainTokens := contextSourceDomainModelTokens(pack, index)
	requestedActions := contextEndpointRequestedActions(query)

	projects := make([]string, 0, len(providerProjects))
	for project := range providerProjects {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	result := make([]ContextProductionPlanFiles, 0, len(projects))
	for _, project := range projects {
		group := ContextProductionPlanFiles{
			Project:          project,
			ProviderContract: contextProductionProviderContract(index, project, represented, requestedActions),
			PrimaryPersistence: contextProductionPrimaryPersistence(
				index,
				project,
				represented,
				domainTokens,
			),
		}
		if group.ProviderContract != "" || len(group.PrimaryPersistence) > 0 {
			result = append(result, group)
		}
	}
	return result
}

func contextProductionProviderContract(
	index scan.AgentContextIndexRecord,
	project string,
	represented map[string]bool,
	requestedActions map[string]bool,
) string {
	byPath := make(map[string]rankedContextProductionFile)
	for _, fact := range index.Facts {
		path := contextExactInventoryPath(fact.File)
		if normalizeContextProject(fact.Project) != project ||
			!strings.EqualFold(strings.TrimSpace(fact.Confidence), "EXACT") ||
			path == "" || contextFactUsesTestSource(fact) ||
			represented[contextEvidenceInventoryPathKey(project, path)] ||
			!contextProductionProviderContractFact(fact) {
			continue
		}
		score := 100
		switch strings.ToLower(strings.TrimSpace(fact.Kind)) {
		case "route", "api_endpoint", "api_contract", "http_contract":
			score += 40
		case "symbol", "class", "interface":
			score += 20
		}
		identity := compactContextIdentifier(strings.Join([]string{
			fact.Name,
			fact.Qualified,
			strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		}, " "))
		if strings.Contains(identity, "controller") || strings.Contains(identity, "handler") {
			score += 30
		}
		if contextActionFamiliesOverlap(requestedActions, contextFactActionFamilies(fact)) {
			score += 10
		}
		current := byPath[path]
		if score > current.score {
			byPath[path] = rankedContextProductionFile{path: path, score: score}
		}
	}
	return contextBestProductionFile(byPath)
}

func contextProductionProviderContractFact(fact scan.AgentContextFactRecord) bool {
	if !contextExactInventoryInternalInterfaceFact(fact) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(fact.Kind)) {
	case "route", "api_endpoint", "api_contract", "http_contract":
		return true
	case "symbol", "class", "interface":
		identity := compactContextIdentifier(strings.Join([]string{
			fact.Name,
			contextIdentifierLeaf(fact.Qualified),
			strings.TrimSuffix(filepath.Base(fact.File), filepath.Ext(fact.File)),
		}, " "))
		return strings.Contains(identity, "controller") || strings.Contains(identity, "handler")
	default:
		return false
	}
}

func contextProductionPrimaryPersistence(
	index scan.AgentContextIndexRecord,
	project string,
	represented map[string]bool,
	domainTokens map[string]bool,
) []string {
	if len(domainTokens) == 0 {
		return nil
	}
	byPath := make(map[string]rankedContextProductionFile)
	for _, fact := range index.Facts {
		path := contextExactInventoryPath(fact.File)
		if normalizeContextProject(fact.Project) != project ||
			!strings.EqualFold(strings.TrimSpace(fact.Confidence), "EXACT") ||
			path == "" || contextFactUsesTestSource(fact) ||
			represented[contextEvidenceInventoryPathKey(project, path)] ||
			!contextExactInventoryPersistenceOwnerFact(fact) ||
			contextDomainModelDependencyFact(fact) {
			continue
		}
		matches := contextStableFactIdentityMatchCount(fact, domainTokens)
		if matches == 0 {
			continue
		}
		score := 100 + matches*20
		current := byPath[path]
		if score > current.score {
			byPath[path] = rankedContextProductionFile{path: path, score: score}
		}
	}
	ranked := make([]rankedContextProductionFile, 0, len(byPath))
	for _, candidate := range byPath {
		ranked = append(ranked, candidate)
	}
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].score != ranked[right].score {
			return ranked[left].score > ranked[right].score
		}
		return ranked[left].path < ranked[right].path
	})
	if len(ranked) > maximumContextProductionPersistenceFiles {
		ranked = ranked[:maximumContextProductionPersistenceFiles]
	}
	result := make([]string, 0, len(ranked))
	for _, candidate := range ranked {
		result = append(result, candidate.path)
	}
	sort.Strings(result)
	return result
}

func contextBestProductionFile(candidates map[string]rankedContextProductionFile) string {
	ranked := make([]rankedContextProductionFile, 0, len(candidates))
	for _, candidate := range candidates {
		ranked = append(ranked, candidate)
	}
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].score != ranked[right].score {
			return ranked[left].score > ranked[right].score
		}
		return ranked[left].path < ranked[right].path
	})
	if len(ranked) == 0 {
		return ""
	}
	return ranked[0].path
}
