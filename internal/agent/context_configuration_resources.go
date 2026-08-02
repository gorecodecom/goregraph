package agent

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const maximumContextConfigurationResources = 6

type rankedContextConfigurationResource struct {
	project   string
	resource  ContextConfigurationResource
	keyGroups []string
	score     int
}

func contextConfigurationResources(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) []ContextConfigurationResourceGroup {
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
				project: project,
				resource: ContextConfigurationResource{
					Path: path, Profile: contextConfigurationProfile(path),
				},
			}
		}
		name := strings.TrimSpace(fact.Name)
		if name != "" && !slicesContainsString(candidate.keyGroups, name) {
			candidate.keyGroups = append(candidate.keyGroups, name)
		}
		candidate.score = max(candidate.score, score)
		rankedByPath[key] = candidate
	}

	ranked := make([]rankedContextConfigurationResource, 0, len(rankedByPath))
	for _, candidate := range rankedByPath {
		sort.Strings(candidate.keyGroups)
		ranked = append(ranked, candidate)
	}
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].score != ranked[right].score {
			return ranked[left].score > ranked[right].score
		}
		leftKey := contextEvidenceInventoryPathKey(
			ranked[left].project,
			ranked[left].resource.Path,
		)
		rightKey := contextEvidenceInventoryPathKey(
			ranked[right].project,
			ranked[right].resource.Path,
		)
		return leftKey < rightKey
	})
	if len(ranked) > maximumContextConfigurationResources {
		ranked = ranked[:maximumContextConfigurationResources]
	}
	grouped := make(map[string]*ContextConfigurationResourceGroup)
	for _, candidate := range ranked {
		key := candidate.project + "\x00" + strings.Join(candidate.keyGroups, "\x00")
		group := grouped[key]
		if group == nil {
			group = &ContextConfigurationResourceGroup{
				Project: candidate.project, KeyGroups: append([]string(nil), candidate.keyGroups...),
			}
			grouped[key] = group
		}
		group.Resources = append(group.Resources, candidate.resource)
	}
	result := make([]ContextConfigurationResourceGroup, 0, len(grouped))
	for _, group := range grouped {
		sort.Slice(group.Resources, func(left, right int) bool {
			if group.Resources[left].Profile != group.Resources[right].Profile {
				return group.Resources[left].Profile < group.Resources[right].Profile
			}
			return group.Resources[left].Path < group.Resources[right].Path
		})
		result = append(result, *group)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Project != result[right].Project {
			return result[left].Project < result[right].Project
		}
		return strings.Join(result[left].KeyGroups, "\x00") <
			strings.Join(result[right].KeyGroups, "\x00")
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
