package agent

import (
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func prioritizeContextDecisionGaps(
	decisionGaps []ContextUncertainty,
	existing []ContextUncertainty,
) []ContextUncertainty {
	result := make([]ContextUncertainty, 0, maximumContextUncertainty)
	for _, group := range [][]ContextUncertainty{decisionGaps, existing} {
		for _, uncertainty := range group {
			if len(result) >= maximumContextUncertainty {
				return result
			}
			if contextUncertaintyExists(result, uncertainty.Scope, uncertainty.Reason) {
				continue
			}
			result = append(result, uncertainty)
		}
	}
	return result
}

func contextDependentPersistenceGaps(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) []ContextUncertainty {
	query := contextSelectionQuery(pack)
	if len(pack.SourceSections) == 0 ||
		!contextQueryRequestsConcern(query, contextConcernPersistence) {
		return nil
	}
	projects := make(map[string]bool)
	for _, concern := range pack.Concerns {
		if normalizedContextConcernKind(concern.Kind) != contextConcernPersistence {
			continue
		}
		if project := normalizeContextProject(concern.Project); project != "" {
			projects[project] = true
		}
	}
	if len(projects) == 0 {
		return nil
	}
	domainTokens := contextSourceDomainModelTokens(pack, index)
	if len(domainTokens) == 0 {
		return nil
	}
	represented := contextPlanFileRepresentedPaths(pack)
	type dependentPersistenceIdentity struct {
		identity string
		quality  int
	}
	identitiesByProject := make(map[string]map[string]dependentPersistenceIdentity)
	for _, fact := range index.Facts {
		project := normalizeContextProject(fact.Project)
		if !projects[project] ||
			!contextDependentPersistenceIdentityFact(fact) ||
			!contextDomainModelDependencyFact(fact) ||
			contextStableFactIdentityMatchCount(fact, domainTokens) == 0 ||
			contextExactInventoryPath(fact.File) == "" ||
			represented[contextEvidenceInventoryPathKey(fact.Project, fact.File)] {
			continue
		}
		identity := strings.TrimSpace(fact.Qualified)
		if identity == "" {
			identity = strings.TrimSpace(fact.Name)
		}
		if identity == "" {
			continue
		}
		path := contextExactInventoryPath(fact.File)
		quality := 1
		if normalizedContextConcernKind(fact.Kind) == contextConcernPersistence {
			quality = 2
		}
		if identitiesByProject[project] == nil {
			identitiesByProject[project] = make(map[string]dependentPersistenceIdentity)
		}
		current, exists := identitiesByProject[project][path]
		if !exists || quality > current.quality ||
			quality == current.quality && identity < current.identity {
			identitiesByProject[project][path] = dependentPersistenceIdentity{
				identity: identity,
				quality:  quality,
			}
		}
	}
	projectNames := make([]string, 0, len(identitiesByProject))
	for project := range identitiesByProject {
		projectNames = append(projectNames, project)
	}
	sort.Strings(projectNames)
	result := make([]ContextUncertainty, 0, len(projectNames))
	for _, project := range projectNames {
		identities := make([]string, 0, len(identitiesByProject[project]))
		for _, candidate := range identitiesByProject[project] {
			identities = append(identities, candidate.identity)
		}
		sort.Strings(identities)
		if len(identities) > 2 {
			identities = identities[:2]
		}
		result = append(result, ContextUncertainty{
			Scope: project + "/dependent_persistence",
			Reason: "indexed dependent persistence " + strings.Join(identities, ", ") +
				" is not represented; its deletion ordering and cascade behavior remains unknown",
		})
	}
	return result
}

func contextDependentPersistenceIdentityFact(fact scan.AgentContextFactRecord) bool {
	if normalizedContextConcernKind(fact.Kind) == contextConcernPersistence {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(fact.Kind), "symbol") {
		return false
	}
	identity := compactContextIdentifier(firstNonEmptyContext(fact.Qualified, fact.Name))
	return strings.HasSuffix(identity, "repository")
}
