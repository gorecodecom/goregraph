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
	evidenceQuery := contextEvidenceSelectionQuery(pack)
	if !contextQueryRequestsConcern(evidenceQuery, contextConcernConfiguration) ||
		!contextQueryRequestsExactEvidenceInventory(evidenceQuery) {
		return nil
	}
	aliases := contextProjectAliases(index.Facts, index.Coverage)
	explicitProjects := contextExplicitProjects(query, aliases)
	eligibleProjects := explicitProjects
	if len(eligibleProjects) == 0 && pack.ProtocolVersion == AdaptiveV2 {
		eligibleProjects = contextSelectedConfigurationProjects(pack, index, evidenceQuery)
	}
	if len(eligibleProjects) == 0 {
		return nil
	}
	represented := contextPlanFileRepresentedPaths(pack)
	identityProjects := make(map[string]bool)
	if pack.ProtocolVersion == AdaptiveV2 && contextChangePlanInventoryEligible(pack) {
		if entrypointProject := contextPlanFileEntrypointProject(pack); entrypointProject != "" {
			identityProjects = contextSelectedSourceNavigationProjects(pack, index)
			identityProjects[entrypointProject] = true
		}
	}

	rankedByPath := make(map[string]rankedContextConfigurationResource)
	matchedKeyProjects := make(map[string]bool)
	for _, fact := range index.Facts {
		project := normalizeContextProject(fact.Project)
		path := contextExactInventoryPath(fact.File)
		if !eligibleProjects[project] ||
			normalizedContextConcernKind(fact.Kind) != contextConcernConfiguration ||
			!strings.EqualFold(strings.TrimSpace(fact.Confidence), "EXACT") ||
			path == "" || !isContextConfigurationResource(path) {
			continue
		}
		resourceIdentity := strings.EqualFold(strings.TrimSpace(fact.Summary), "Spring configuration resource")
		score := contextRequestedConfigurationFactScore(evidenceQuery, fact)
		if resourceIdentity {
			if !identityProjects[project] {
				continue
			}
			score = 1
		} else if score == 0 || strings.EqualFold(strings.TrimSpace(fact.Name), filepath.Base(fact.File)) {
			continue
		} else {
			matchedKeyProjects[project] = true
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
		if !resourceIdentity && name != "" && !slicesContainsString(candidate.keyGroups, name) {
			candidate.keyGroups = append(candidate.keyGroups, name)
		}
		candidate.score = max(candidate.score, score)
		rankedByPath[key] = candidate
	}

	ranked := make([]rankedContextConfigurationResource, 0, len(rankedByPath))
	for _, candidate := range rankedByPath {
		if candidate.score == 1 && matchedKeyProjects[candidate.project] {
			continue
		}
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

func contextSelectedConfigurationProjects(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	query string,
) map[string]bool {
	projects := make(map[string]bool)
	add := func(value string) {
		if project := normalizeContextProject(value); project != "" {
			projects[project] = true
		}
	}
	for _, entrypoint := range pack.Entrypoints {
		add(entrypoint.Project)
	}
	for _, section := range pack.SourceSections {
		add(section.Project)
	}
	selectedSourceFacts := make(map[string]bool, len(pack.selectedSourceFactIDs))
	for _, factID := range pack.selectedSourceFactIDs {
		selectedSourceFacts[factID] = true
	}
	for _, fact := range index.Facts {
		if selectedSourceFacts[fact.ID] {
			add(fact.Project)
		}
	}
	for _, concern := range pack.Concerns {
		if contextQueryRequestsConcern(query, normalizedContextConcernKind(concern.Kind)) {
			add(concern.Project)
		}
	}
	return projects
}

func fitInferredConfigurationNavigationWithinBudget(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	request ContextRequest,
) (ContextPack, bool, error) {
	if pack.ProtocolVersion != AdaptiveV2 || len(pack.ConfigurationResources) == 0 {
		return pack, false, nil
	}
	aliases := contextProjectAliases(index.Facts, index.Coverage)
	if len(contextExplicitProjects(contextSelectionQuery(pack), aliases)) != 0 {
		return pack, false, nil
	}

	candidate := cloneContextPack(pack)
	if contextChangePlanInventoryEligible(pack) &&
		contextQueryRequestsExactEvidenceInventory(contextEvidenceSelectionQuery(pack)) {
		fits, err := contextSourcePackFits(pack, request)
		if err != nil {
			return ContextPack{}, false, err
		}
		if !fits {
			files := candidate.Files[:0]
			for _, file := range candidate.Files {
				if !contextConfigurationFileRepeatedBySource(file, candidate.SourceSections) {
					files = append(files, file)
				}
			}
			candidate.Files = files
			if len(candidate.Files) != len(pack.Files) {
				candidate, err = finalizeContextEstimate(candidate)
				if err != nil {
					return ContextPack{}, false, err
				}
				fits, err = contextSourcePackFits(candidate, request)
				if err != nil {
					return ContextPack{}, false, err
				}
				if fits {
					return candidate, true, nil
				}
			}
		}
	}
	for len(candidate.ConfigurationResources) > 0 {
		lastGroup := len(candidate.ConfigurationResources) - 1
		resources := candidate.ConfigurationResources[lastGroup].Resources
		resources = resources[:len(resources)-1]
		if len(resources) == 0 {
			candidate.ConfigurationResources = candidate.ConfigurationResources[:lastGroup]
		} else {
			candidate.ConfigurationResources[lastGroup].Resources = resources
		}
		var err error
		candidate, err = finalizeContextEstimate(candidate)
		if err != nil {
			return ContextPack{}, false, err
		}
		fits, err := contextSourcePackFits(candidate, request)
		if err != nil {
			return ContextPack{}, false, err
		}
		if fits {
			return candidate, true, nil
		}
	}
	return pack, false, nil
}

func contextConfigurationFileRepeatedBySource(file ContextFile, sections []ContextSourceSection) bool {
	project := normalizeContextProject(file.Project)
	path := contextExactInventoryPath(file.Path)
	if project == "" || path == "" || file.StartLine <= 0 || file.EndLine < file.StartLine ||
		file.Reason != "" || file.Confidence != "" {
		return false
	}
	for _, section := range sections {
		if normalizeContextProject(section.Project) == project &&
			contextExactInventoryPath(section.Path) == path &&
			section.StartLine == file.StartLine && section.EndLine == file.EndLine &&
			section.Role == file.Role && strings.TrimSpace(section.Content) != "" &&
			(section.SourceState == "indexed_range_current" || section.SourceState == "relocated_current") {
			return true
		}
	}
	return false
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
