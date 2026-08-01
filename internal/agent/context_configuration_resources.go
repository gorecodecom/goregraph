package agent

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const maximumContextConfigurationResources = 6

type rankedContextConfigurationResource struct {
	resource ContextConfigurationResource
	score    int
}

func contextConfigurationResources(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) []ContextConfigurationResource {
	query := contextSelectionQuery(pack)
	if !contextQueryRequestsConcern(query, contextConcernConfiguration) ||
		!contextQueryRequestsExactEvidenceInventory(query) {
		return nil
	}
	aliases := contextProjectAliases(index.Facts, index.Coverage)
	explicitProjects := contextExplicitProjects(query, aliases)
	if len(explicitProjects) == 0 {
		return nil
	}
	represented := contextPlanFileRepresentedPaths(pack)

	rankedByPath := make(map[string]rankedContextConfigurationResource)
	for _, fact := range index.Facts {
		project := normalizeContextProject(fact.Project)
		path := contextExactInventoryPath(fact.File)
		if !explicitProjects[project] ||
			normalizedContextConcernKind(fact.Kind) != contextConcernConfiguration ||
			!strings.EqualFold(strings.TrimSpace(fact.Confidence), "EXACT") ||
			path == "" || !isContextConfigurationResource(path) ||
			strings.EqualFold(strings.TrimSpace(fact.Summary), "Spring configuration resource") ||
			strings.EqualFold(strings.TrimSpace(fact.Name), filepath.Base(fact.File)) {
			continue
		}
		score := contextRequestedConfigurationFactScore(query, fact)
		if score == 0 {
			continue
		}
		key := contextEvidenceInventoryPathKey(project, path)
		if represented[key] {
			continue
		}
		candidate, exists := rankedByPath[key]
		if !exists {
			candidate = rankedContextConfigurationResource{
				resource: ContextConfigurationResource{
					Project: project, Path: path,
					Profile: contextConfigurationProfile(path),
				},
			}
		}
		name := strings.TrimSpace(fact.Name)
		if name != "" && !slicesContainsString(candidate.resource.KeyGroups, name) {
			candidate.resource.KeyGroups = append(candidate.resource.KeyGroups, name)
		}
		candidate.score = max(candidate.score, score)
		rankedByPath[key] = candidate
	}

	ranked := make([]rankedContextConfigurationResource, 0, len(rankedByPath))
	for _, candidate := range rankedByPath {
		sort.Strings(candidate.resource.KeyGroups)
		ranked = append(ranked, candidate)
	}
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].score != ranked[right].score {
			return ranked[left].score > ranked[right].score
		}
		leftKey := contextEvidenceInventoryPathKey(
			ranked[left].resource.Project,
			ranked[left].resource.Path,
		)
		rightKey := contextEvidenceInventoryPathKey(
			ranked[right].resource.Project,
			ranked[right].resource.Path,
		)
		return leftKey < rightKey
	})
	if len(ranked) > maximumContextConfigurationResources {
		ranked = ranked[:maximumContextConfigurationResources]
	}
	result := make([]ContextConfigurationResource, 0, len(ranked))
	for _, candidate := range ranked {
		result = append(result, candidate.resource)
	}
	sort.Slice(result, func(left, right int) bool {
		return contextEvidenceInventoryPathKey(result[left].Project, result[left].Path) <
			contextEvidenceInventoryPathKey(result[right].Project, result[right].Path)
	})
	return result
}

func contextConfigurationProfile(path string) string {
	path = filepath.ToSlash(path)
	base := filepath.Base(path)
	extension := filepath.Ext(base)
	name := strings.TrimSuffix(base, extension)
	for _, prefix := range []string{"application-", "bootstrap-"} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	if strings.Contains("/"+path, "/src/test/") {
		return "test"
	}
	return "production"
}
