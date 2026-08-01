package agent

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	maximumContextPlanFiles           = 4
	contextPlanFilePrimaryActionBonus = 1000
)

type contextPlanFileFact struct {
	fact scan.AgentContextFactRecord
	use  string
}

func compactContextPlanFileInventory(pack ContextPack) ContextPack {
	if len(pack.PlanFiles) == 0 {
		return pack
	}
	for fileIndex := range pack.Files {
		pack.Files[fileIndex].Reason = ""
	}
	return pack
}

func contextPlanFileReserveView(before, after ContextPack) ContextPack {
	if len(after.PlanFiles) == 0 {
		return after
	}
	// Plan files pay for their final bytes by dropping repeated file reasons.
	// Keep them out of the proactive reserve so source selection stays monotonic;
	// the final hard-budget loop still reduces any pack that does not fit.
	after.PlanFiles = nil
	reasons := make(map[string]string, len(before.Files))
	for _, file := range before.Files {
		reasons[contextEvidenceInventoryPathKey(file.Project, file.Path)] = file.Reason
	}
	for fileIndex := range after.Files {
		key := contextEvidenceInventoryPathKey(
			after.Files[fileIndex].Project,
			after.Files[fileIndex].Path,
		)
		if reason, ok := reasons[key]; ok {
			after.Files[fileIndex].Reason = reason
		}
	}
	return after
}

func contextPlanFiles(pack ContextPack, index scan.AgentContextIndexRecord) []ContextPlanFile {
	query := contextSelectionQuery(pack)
	if !contextQueryRequestsExactEvidenceInventory(query) ||
		!contextQueryPlansMissingTransition(query) ||
		!contextQueryRequestsTests(query) {
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
	eligible := make([]scan.AgentContextFactRecord, 0)
	for _, fact := range index.Facts {
		if !contextPlanFileEligibleFact(fact) {
			continue
		}
		eligible = append(eligible, fact)
	}

	selected := make([]contextPlanFileFact, 0, maximumContextPlanFiles)
	selected = append(selected, contextPlanFileProviderTests(
		pack,
		index,
		eligible,
		providerProjects,
	)...)
	selected = append(selected, contextPlanFileCallerPatternPair(
		pack,
		index,
		eligible,
		entrypointProject,
	)...)
	if len(selected) > maximumContextPlanFiles {
		selected = selected[:maximumContextPlanFiles]
	}
	result := make([]ContextPlanFile, 0, len(selected))
	for _, candidate := range selected {
		if represented[contextEvidenceInventoryPathKey(candidate.fact.Project, candidate.fact.File)] {
			continue
		}
		result = append(result, ContextPlanFile{
			Project: normalizeContextProject(candidate.fact.Project),
			Path:    contextExactInventoryPath(candidate.fact.File),
			Use:     candidate.use,
		})
	}
	return result
}

func contextPlanFileEntrypointProject(pack ContextPack) string {
	projects := make([]string, 0, len(pack.Entrypoints))
	for _, entrypoint := range pack.Entrypoints {
		projects = append(projects, entrypoint.Project)
	}
	if project, unambiguous := contextPlanFileUniqueProject(projects); !unambiguous {
		return ""
	} else if project != "" {
		return project
	}
	projects = projects[:0]
	for _, endpoint := range pack.Endpoints {
		projects = append(projects, endpoint.Provider)
	}
	project, _ := contextPlanFileUniqueProject(projects)
	return project
}

func contextPlanFileUniqueProject(projects []string) (string, bool) {
	result := ""
	for _, value := range projects {
		project := normalizeContextProject(value)
		if project == "" {
			continue
		}
		if result != "" && project != result {
			return "", false
		}
		result = project
	}
	return result, true
}

func contextPlanFileProviderProjects(pack ContextPack, entrypointProject string) map[string]bool {
	result := make(map[string]bool)
	for _, concern := range pack.Concerns {
		project := normalizeContextProject(concern.Project)
		if project == "" || project == entrypointProject {
			continue
		}
		result[project] = true
	}
	return result
}

func contextPlanFileRepresentedPaths(pack ContextPack) map[string]bool {
	result := make(map[string]bool)
	for _, file := range pack.Files {
		result[contextEvidenceInventoryPathKey(file.Project, file.Path)] = true
	}
	for _, section := range pack.SourceSections {
		result[contextEvidenceInventoryPathKey(section.Project, section.Path)] = true
	}
	for _, omission := range pack.SourceOmissions {
		result[contextEvidenceInventoryPathKey(omission.Project, omission.Path)] = true
	}
	return result
}

func contextPlanFileEligibleFact(fact scan.AgentContextFactRecord) bool {
	if !strings.EqualFold(strings.TrimSpace(fact.Confidence), "EXACT") ||
		!contextFactUsesTestSource(fact) ||
		isContextConfigurationResource(fact.File) ||
		contextExactInventoryPath(fact.File) == "" {
		return false
	}
	kind := strings.ToLower(strings.TrimSpace(fact.Kind))
	return kind == "symbol" || kind == "test"
}

func contextPlanFileProviderTests(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	facts []scan.AgentContextFactRecord,
	projects map[string]bool,
) []contextPlanFileFact {
	var controller, service *scan.AgentContextFactRecord
	for factIndex := range facts {
		fact := facts[factIndex]
		if !projects[normalizeContextProject(fact.Project)] {
			continue
		}
		identity := compactContextIdentifier(strings.Join([]string{
			fact.Name,
			fact.Qualified,
			strings.TrimSuffix(filepath.Base(fact.File), filepath.Ext(fact.File)),
		}, " "))
		switch {
		case strings.Contains(identity, "controller") &&
			(strings.Contains(identity, "management") || strings.Contains(identity, "mgmt")) &&
			strings.Contains(identity, "test"):
			if contextPlanFileFactBetter(pack, index, fact, controller) {
				copy := fact
				controller = &copy
			}
		case strings.Contains(identity, "service") && strings.Contains(identity, "test"):
			if contextPlanFileFactBetter(pack, index, fact, service) {
				copy := fact
				service = &copy
			}
		}
	}
	result := make([]contextPlanFileFact, 0, 2)
	for _, fact := range []*scan.AgentContextFactRecord{controller, service} {
		if fact != nil {
			result = append(result, contextPlanFileFact{fact: *fact, use: "provider_test"})
		}
	}
	return result
}

func contextPlanFileCallerPatternPair(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	facts []scan.AgentContextFactRecord,
	entrypointProject string,
) []contextPlanFileFact {
	type pair struct {
		mock  *scan.AgentContextFactRecord
		retry *scan.AgentContextFactRecord
	}
	pairs := make(map[string]*pair)
	for factIndex := range facts {
		fact := facts[factIndex]
		if normalizeContextProject(fact.Project) != entrypointProject {
			continue
		}
		identity := compactContextIdentifier(firstNonEmptyContext(
			fact.Name,
			contextIdentifierLeaf(fact.Qualified),
			strings.TrimSuffix(filepath.Base(fact.File), filepath.Ext(fact.File)),
		))
		stem, use := contextPlanFilePatternStem(identity)
		if stem == "" {
			continue
		}
		candidate := pairs[stem]
		if candidate == nil {
			candidate = &pair{}
			pairs[stem] = candidate
		}
		copy := fact
		if use == "mock_pattern" && contextPlanFileFactBetter(pack, index, fact, candidate.mock) {
			candidate.mock = &copy
		}
		if use == "retry_pattern" && contextPlanFileFactBetter(pack, index, fact, candidate.retry) {
			candidate.retry = &copy
		}
	}
	represented := contextPlanFileRepresentedPaths(pack)
	representedCount := func(candidate *pair) int {
		count := 0
		for _, fact := range []*scan.AgentContextFactRecord{candidate.mock, candidate.retry} {
			if fact != nil && represented[contextEvidenceInventoryPathKey(fact.Project, fact.File)] {
				count++
			}
		}
		return count
	}
	stems := make([]string, 0, len(pairs))
	for stem, candidate := range pairs {
		if candidate.mock != nil && candidate.retry != nil &&
			representedCount(candidate) < 2 &&
			contextPlanFilePatternRelevant(pack, *candidate.mock, *candidate.retry) {
			stems = append(stems, stem)
		}
	}
	sort.Slice(stems, func(left, right int) bool {
		leftPair := pairs[stems[left]]
		rightPair := pairs[stems[right]]
		leftRepresented := representedCount(leftPair)
		rightRepresented := representedCount(rightPair)
		if leftRepresented != rightRepresented {
			return leftRepresented > rightRepresented
		}
		leftScore := contextPlanFileFactScore(pack, index, *leftPair.retry)
		rightScore := contextPlanFileFactScore(pack, index, *rightPair.retry)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return stems[left] < stems[right]
	})
	if len(stems) == 0 {
		return nil
	}
	selected := pairs[stems[0]]
	return []contextPlanFileFact{
		{fact: *selected.mock, use: "mock_pattern"},
		{fact: *selected.retry, use: "retry_pattern"},
	}
}

func contextPlanFilePatternRelevant(
	pack ContextPack,
	mock scan.AgentContextFactRecord,
	retry scan.AgentContextFactRecord,
) bool {
	anchorValues := []string{contextSelectionQuery(pack)}
	for _, endpoint := range pack.Endpoints {
		anchorValues = append(anchorValues, endpoint.Handler, endpoint.File, endpoint.Path)
	}
	for _, contract := range pack.Contracts {
		anchorValues = append(anchorValues, contract.Label, contract.File)
	}
	for _, file := range pack.Files {
		anchorValues = append(anchorValues, file.Path)
	}
	anchors := contextExpandedTokenSet(strings.Join(anchorValues, " "))
	identity := contextExpandedTokenSet(strings.Join([]string{
		contextPlanFilePatternIdentity(mock),
		contextPlanFilePatternIdentity(retry),
	}, " "))
	userInformationRequested := anchors["user_information"] ||
		anchors["benutzerinformation"] ||
		anchors["benutzerinformationen"] ||
		anchors["user"] && anchors["information"]
	for token := range identity {
		if contextPlanFilePatternGenericToken(token) {
			continue
		}
		if anchors[token] || token == "user" && userInformationRequested {
			return true
		}
	}
	return false
}

func contextPlanFilePatternIdentity(fact scan.AgentContextFactRecord) string {
	return firstNonEmptyContext(
		fact.Name,
		contextIdentifierLeaf(fact.Qualified),
		strings.TrimSuffix(filepath.Base(fact.File), filepath.Ext(fact.File)),
	)
}

func contextPlanFilePatternGenericToken(token string) bool {
	switch token {
	case "client", "common", "example", "existing", "java", "management", "mgmt",
		"mock", "outbound", "pattern", "retry", "retries", "retryable", "service",
		"src", "test", "tests":
		return true
	default:
		return len([]rune(token)) < 4
	}
}

func contextPlanFilePatternStem(identity string) (string, string) {
	for _, suffix := range []struct {
		value string
		use   string
	}{
		{value: "retryabletest", use: "retry_pattern"},
		{value: "mock", use: "mock_pattern"},
	} {
		if stem := strings.TrimSuffix(identity, suffix.value); stem != identity && stem != "" {
			return stem, suffix.use
		}
	}
	return "", ""
}

func contextPlanFileFactBetter(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	left scan.AgentContextFactRecord,
	right *scan.AgentContextFactRecord,
) bool {
	if right == nil {
		return true
	}
	leftScore := contextPlanFileFactScore(pack, index, left)
	rightScore := contextPlanFileFactScore(pack, index, *right)
	if leftScore != rightScore {
		return leftScore > rightScore
	}
	if left.Project != right.Project {
		return left.Project < right.Project
	}
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.ID < right.ID
}

func contextPlanFileFactScore(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
) int {
	concern := newContextConcern(
		contextConcernTests,
		normalizeContextProject(fact.Project),
		true,
		[]string{fact.ID},
		"requested plan-file test evidence",
	)
	score := contextSourceConcernFactScoreWithIndex(
		fact,
		concern,
		contextSelectionQuery(pack),
		map[string]bool{},
		index,
	)
	if contextQueryPlansMissingTransition(contextSelectionQuery(pack)) &&
		contextPlanFilePrimaryActionTest(fact) {
		score += contextPlanFilePrimaryActionBonus
	}
	return score
}

func contextPlanFilePrimaryActionTest(fact scan.AgentContextFactRecord) bool {
	identity := compactContextIdentifier(firstNonEmptyContext(
		fact.Name,
		contextIdentifierLeaf(fact.Qualified),
		strings.TrimSuffix(filepath.Base(fact.File), filepath.Ext(fact.File)),
	))
	for _, marker := range []string{"cleanup", "delete", "deletion", "housekeeping", "remove"} {
		if strings.Contains(identity, marker) {
			return true
		}
	}
	return false
}
