package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	scoreExactRoute                  = 1000
	scoreExactQualified              = 900
	scoreExactName                   = 800
	scoreAllTerms                    = 500
	scorePerMatchedTerm              = 60
	scoreRouteKind                   = 80
	scoreSymbolKind                  = 60
	scoreTestKind                    = 20
	scoreExactConfidence             = 30
	scoreResolvedConfidence          = 15
	minimumContextSeedScore          = 180
	maximumContextUncertainty        = 3
	maximumContextConsumers          = 8
	maximumContextSupportingProjects = 2
)

const noAuthEvidenceDetected = "No auth evidence detected"

type rankedContextFact struct {
	fact         scan.AgentContextFactRecord
	score        int
	exactClass   int
	matchedTerms int
	routeExtras  int
	allTerms     bool
	reason       string
}

type rankedContextSupportFact struct {
	fact            scan.AgentContextFactRecord
	project         string
	explicit        bool
	semanticMatches int
	score           int
}

type expandedContextEdge struct {
	edge     scan.AgentContextEdgeRecord
	seedRank int
	neighbor scan.AgentContextFactRecord
}

func compileContextPack(index scan.AgentContextIndexRecord, request ContextRequest) (ContextPack, error) {
	ranked := rankContextFacts(index.Facts, request.Query)
	seeds := selectContextSeeds(ranked)
	endpointSeed, hasEndpoint := selectContextEndpoint(ranked, request.Query)
	if len(seeds) == 0 && hasEndpoint {
		seeds = []rankedContextFact{endpointSeed}
	}
	if len(seeds) == 0 {
		return fallbackContextPack(index, request, "no sufficiently relevant context fact found", nil)
	}

	top := seeds[0]
	pack, err := newContextEnvelope(index, request)
	if err != nil {
		return ContextPack{}, err
	}
	pack.Confidence = contextPackConfidence(top, false)
	pack, err = finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}
	if pack.EstimatedTokens > request.BudgetTokens {
		return ContextPack{}, fmt.Errorf(
			"context envelope requires %d tokens, exceeding budget %d",
			pack.EstimatedTokens,
			request.BudgetTokens,
		)
	}
	includedFactIDs := map[string]bool{}
	acceptedEdgeIDs := map[string]bool{}
	retainedSeeds := make([]rankedContextFact, 0, len(seeds))

	topIsEndpoint := hasEndpoint && top.fact.ID == endpointSeed.fact.ID
	added := false
	if topIsEndpoint {
		pack, added, err = tryAddContextEndpoint(pack, request, endpointSeed.fact, index)
	} else {
		pack, added, err = tryAddContextLocation(
			pack,
			request,
			top.fact,
			top.reason,
			"entrypoint",
			func(candidate *ContextPack, location ContextLocation) {
				candidate.Entrypoints = append(candidate.Entrypoints, location)
			},
		)
	}
	if err != nil {
		return ContextPack{}, err
	}
	if !added {
		return fallbackContextPack(index, request, "top context fact exceeds the requested budget", nil)
	}
	includedFactIDs[top.fact.ID] = true
	retainedSeeds = append(retainedSeeds, top)
	if hasEndpoint && !topIsEndpoint {
		var endpointAdded bool
		pack, endpointAdded, err = tryAddContextEndpoint(pack, request, endpointSeed.fact, index)
		if err != nil {
			return ContextPack{}, err
		}
		if endpointAdded {
			includedFactIDs[endpointSeed.fact.ID] = true
		}
	}

	for _, seed := range seeds[1:] {
		var accepted bool
		pack, accepted, err = tryAddContextLocation(
			pack,
			request,
			seed.fact,
			seed.reason,
			"entrypoint",
			func(candidate *ContextPack, location ContextLocation) {
				candidate.Entrypoints = append(candidate.Entrypoints, location)
			},
		)
		if err != nil {
			return ContextPack{}, err
		}
		if accepted {
			includedFactIDs[seed.fact.ID] = true
			retainedSeeds = append(retainedSeeds, seed)
		}
	}

	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	expanded := expandContextEdges(index.Edges, factByID, retainedSeeds)
	for _, expandedEdge := range expanded {
		from := factByID[expandedEdge.edge.FromFactID]
		to := factByID[expandedEdge.edge.ToFactID]
		relationship := contextRelationship(expandedEdge.edge, from, to)
		candidate, accepted, appendErr := tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
			candidate.CallChain = append(candidate.CallChain, relationship)
			candidate.Confidence = contextPackConfidence(top, true)
			if !mergeContextFile(candidate, contextFileForFact(from, "call_chain", "direct "+expandedEdge.edge.Kind), request.MaxFiles) {
				return false
			}
			if !mergeContextFile(candidate, contextFileForFact(to, "call_chain", "direct "+expandedEdge.edge.Kind), request.MaxFiles) {
				return false
			}
			return true
		})
		if appendErr != nil {
			return ContextPack{}, appendErr
		}
		if accepted {
			pack = candidate
			acceptedEdgeIDs[expandedEdge.edge.ID] = true
			includedFactIDs[from.ID] = true
			includedFactIDs[to.ID] = true
		}
	}
	sortContextRelationships(pack.CallChain)

	seedIDs := map[string]bool{}
	for _, seed := range retainedSeeds {
		seedIDs[seed.fact.ID] = true
	}
	neighbors := distinctContextNeighbors(expanded, seedIDs)
	for _, kind := range []string{"api_contract", "persistence", "test"} {
		for _, neighbor := range neighbors {
			if neighbor.neighbor.Kind != kind {
				continue
			}
			var appendLocation func(*ContextPack, ContextLocation)
			role := kind
			switch kind {
			case "api_contract":
				appendLocation = func(candidate *ContextPack, location ContextLocation) {
					candidate.Contracts = append(candidate.Contracts, location)
				}
				role = "contract"
			case "persistence":
				appendLocation = func(candidate *ContextPack, location ContextLocation) {
					candidate.Persistence = append(candidate.Persistence, location)
				}
			case "test":
				appendLocation = func(candidate *ContextPack, location ContextLocation) {
					candidate.Tests = append(candidate.Tests, location)
				}
			}
			var accepted bool
			pack, accepted, err = tryAddContextLocation(
				pack,
				request,
				neighbor.neighbor,
				"direct "+neighbor.edge.Kind,
				role,
				appendLocation,
			)
			if err != nil {
				return ContextPack{}, err
			}
			if accepted {
				includedFactIDs[neighbor.neighbor.ID] = true
			}
		}
	}

	pack.Confidence = contextPackConfidence(top, len(pack.CallChain) > 0)
	pack, err = finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}
	if pack.EstimatedTokens > request.BudgetTokens {
		return ContextPack{}, fmt.Errorf(
			"context pack requires %d tokens, exceeding budget %d",
			pack.EstimatedTokens,
			request.BudgetTokens,
		)
	}

	primaryScopes := selectedContextScopes(index.Edges, includedFactIDs, acceptedEdgeIDs, factByID)
	uncertainties, allIncomplete := scopedContextUncertainties(index.Coverage, primaryScopes)
	if pack.Confidence == "LOW" {
		return fallbackContextPack(index, request, "context confidence is low; inspect source directly", uncertainties)
	}
	if allIncomplete {
		return fallbackContextPack(index, request, "all selected context scopes have incomplete coverage", uncertainties)
	}

	projectAliases := contextProjectAliases(index.Facts, index.Coverage)
	explicitProjects := contextExplicitProjects(request.Query, projectAliases)
	representedProjects := contextRepresentedProjects(pack, includedFactIDs, factByID)
	supportFactIDs := map[string]bool{}
	supports := selectContextSupportFacts(
		rankContextSupportFacts(index.Facts, request.Query, projectAliases, explicitProjects),
		representedProjects,
	)
	acceptedSupportProjects := 0
	for _, support := range supports {
		if acceptedSupportProjects >= maximumContextSupportingProjects {
			break
		}
		if representedProjects[support.project] {
			continue
		}
		candidate, accepted, appendErr := tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
			return mergeContextFile(
				candidate,
				contextFileForFact(support.fact, "related_project", "full task project match"),
				request.MaxFiles,
			)
		})
		if appendErr != nil {
			return ContextPack{}, appendErr
		}
		if accepted {
			pack = candidate
			includedFactIDs[support.fact.ID] = true
			supportFactIDs[support.fact.ID] = true
			representedProjects[support.project] = true
			acceptedSupportProjects++
		}
	}
	supportScopes := selectedContextScopes(nil, supportFactIDs, nil, factByID)
	supportUncertainties, _ := scopedContextUncertainties(index.Coverage, supportScopes)
	uncertainties = deduplicateContextUncertainties(
		contextProjectSupportUncertainties(explicitProjects, representedProjects, index.Coverage),
		uncertainties,
		supportUncertainties,
	)
	for _, uncertainty := range uncertainties {
		if len(pack.Uncertainties) >= maximumContextUncertainty {
			break
		}
		candidate, accepted, appendErr := tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
			candidate.Uncertainties = append(candidate.Uncertainties, uncertainty)
			sort.Slice(candidate.Uncertainties, func(i, j int) bool {
				if candidate.Uncertainties[i].Scope != candidate.Uncertainties[j].Scope {
					return candidate.Uncertainties[i].Scope < candidate.Uncertainties[j].Scope
				}
				return candidate.Uncertainties[i].Reason < candidate.Uncertainties[j].Reason
			})
			return true
		})
		if appendErr != nil {
			return ContextPack{}, appendErr
		}
		if accepted {
			pack = candidate
		}
	}
	return finalizeContextEstimate(pack)
}

func rankContextFacts(facts []scan.AgentContextFactRecord, query string) []rankedContextFact {
	primaryQuery := contextPrimaryQuery(query)
	queryTokens := contextQueryTokens(primaryQuery)
	queryTerm := normalizeContextTerm(query)
	ranked := make([]rankedContextFact, 0, len(facts))
	for _, fact := range facts {
		factTokens := contextTokenSet(strings.Join([]string{
			fact.Search,
			fact.Name,
			fact.Qualified,
			fact.HTTPMethod,
			fact.Path,
			fact.Summary,
		}, " "))
		matched := 0
		for _, token := range queryTokens {
			if factTokens[token] {
				matched++
			}
		}
		routeExtras := 0
		if strings.EqualFold(fact.Kind, "route") || strings.EqualFold(fact.Kind, "api_endpoint") {
			routeTokens := contextTokenSet(strings.TrimSpace(fact.HTTPMethod + " " + fact.Path))
			routeMatches := 0
			for _, token := range queryTokens {
				if routeTokens[token] {
					routeMatches++
				}
			}
			routeExtras = len(routeTokens) - routeMatches
		}
		exactClass := 0
		score := 0
		reason := "primary task match"
		if normalizeContextTerm(primaryQuery) == queryTerm {
			reason = "lexical match"
		}
		routeTerm := normalizeContextTerm(strings.TrimSpace(fact.HTTPMethod + " " + fact.Path))
		qualifiedTerm := normalizeContextTerm(fact.Qualified)
		nameTerm := normalizeContextTerm(fact.Name)
		switch {
		case routeTerm != "" && queryTerm == routeTerm:
			exactClass = 3
			score += scoreExactRoute
			reason = "exact route"
		case qualifiedTerm != "" && queryTerm == qualifiedTerm:
			exactClass = 2
			score += scoreExactQualified
			reason = "exact qualified name"
		case nameTerm != "" && queryTerm == nameTerm:
			exactClass = 1
			score += scoreExactName
			reason = "exact name"
		}
		allTerms := len(queryTokens) > 0 && matched == len(queryTokens)
		if allTerms {
			score += scoreAllTerms
		}
		score += matched * scorePerMatchedTerm
		switch strings.ToLower(fact.Kind) {
		case "route", "api_endpoint":
			score += scoreRouteKind
		case "symbol", "backend_handler":
			score += scoreSymbolKind
		case "test":
			score += scoreTestKind
		}
		switch strings.ToUpper(strings.TrimSpace(fact.Confidence)) {
		case "EXACT":
			score += scoreExactConfidence
		case "RESOLVED":
			score += scoreResolvedConfidence
		}
		ranked = append(ranked, rankedContextFact{
			fact:         fact,
			score:        score,
			exactClass:   exactClass,
			matchedTerms: matched,
			routeExtras:  routeExtras,
			allTerms:     allTerms,
			reason:       reason,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		if left.score != right.score {
			return left.score > right.score
		}
		if left.exactClass != right.exactClass {
			return left.exactClass > right.exactClass
		}
		if left.matchedTerms != right.matchedTerms {
			return left.matchedTerms > right.matchedTerms
		}
		if strings.EqualFold(left.fact.Kind, "route") &&
			strings.EqualFold(right.fact.Kind, "route") &&
			left.routeExtras != right.routeExtras {
			return left.routeExtras < right.routeExtras
		}
		if left.fact.Project != right.fact.Project {
			return left.fact.Project < right.fact.Project
		}
		if left.fact.Kind != right.fact.Kind {
			return left.fact.Kind < right.fact.Kind
		}
		if left.fact.Qualified != right.fact.Qualified {
			return left.fact.Qualified < right.fact.Qualified
		}
		if left.fact.Name != right.fact.Name {
			return left.fact.Name < right.fact.Name
		}
		if left.fact.File != right.fact.File {
			return left.fact.File < right.fact.File
		}
		if left.fact.Line != right.fact.Line {
			return left.fact.Line < right.fact.Line
		}
		return left.fact.ID < right.fact.ID
	})
	return ranked
}

func contextProjectAliases(
	facts []scan.AgentContextFactRecord,
	coverage []scan.AgentContextCoverageRecord,
) map[string][]string {
	projects := map[string]bool{}
	for _, fact := range facts {
		if project := normalizeContextProject(fact.Project); project != "" {
			projects[project] = true
		}
	}
	for _, record := range coverage {
		if project := normalizeContextProject(record.Project); project != "" {
			projects[project] = true
		}
	}

	projectNames := make([]string, 0, len(projects))
	basenameCounts := map[string]int{}
	for project := range projects {
		projectNames = append(projectNames, project)
		basenameCounts[normalizeContextTerm(contextProjectBasename(project))]++
	}
	sort.Strings(projectNames)

	aliases := make(map[string][]string, len(projectNames))
	for _, project := range projectNames {
		aliases[project] = []string{project}
		basenameAlias := normalizeContextTerm(contextProjectBasename(project))
		if basenameAlias != "" && basenameCounts[basenameAlias] == 1 && basenameAlias != project {
			aliases[project] = append(aliases[project], basenameAlias)
		}
		sort.Strings(aliases[project])
	}
	return aliases
}

func normalizeContextProject(project string) string {
	project = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(project), "\\", "/"))
	for strings.HasPrefix(project, "./") {
		project = strings.TrimPrefix(project, "./")
	}
	return strings.Trim(project, "/")
}

func contextProjectBasename(project string) string {
	if index := strings.LastIndex(project, "/"); index >= 0 {
		return project[index+1:]
	}
	return project
}

func contextExplicitProjects(query string, aliases map[string][]string) map[string]bool {
	normalizedQuery := " " + normalizeContextTerm(query) + " "
	explicit := map[string]bool{}
	for project, projectAliases := range aliases {
		if contextQueryContainsProjectPath(query, project) {
			explicit[project] = true
			continue
		}
		for _, alias := range projectAliases {
			if alias != project && alias != "" && strings.Contains(normalizedQuery, " "+alias+" ") {
				explicit[project] = true
				break
			}
		}
	}
	return explicit
}

func contextQueryContainsProjectPath(query, project string) bool {
	queryRunes := []rune(strings.ToLower(strings.ReplaceAll(query, "\\", "/")))
	projectRunes := []rune(project)
	if len(projectRunes) == 0 || len(projectRunes) > len(queryRunes) {
		return false
	}
	for start := 0; start+len(projectRunes) <= len(queryRunes); start++ {
		matched := true
		for offset := range projectRunes {
			if queryRunes[start+offset] != projectRunes[offset] {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		before := contextProjectPathStartBoundary(queryRunes, start)
		afterIndex := start + len(projectRunes)
		after := afterIndex == len(queryRunes) || contextProjectPathBoundary(queryRunes[afterIndex])
		if before && after {
			return true
		}
	}
	return false
}

func contextProjectPathStartBoundary(value []rune, start int) bool {
	for start >= 2 && value[start-2] == '.' && value[start-1] == '/' {
		start -= 2
	}
	return start == 0 || contextProjectPathBoundary(value[start-1])
}

func contextProjectPathBoundary(value rune) bool {
	return !unicode.IsLetter(value) && !unicode.IsDigit(value) && !strings.ContainsRune("/._-", value)
}

func rankContextSupportFacts(
	facts []scan.AgentContextFactRecord,
	query string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
) []rankedContextSupportFact {
	queryTokens := contextQueryTokens(query)
	ranked := make([]rankedContextSupportFact, 0, len(facts))
	for _, fact := range facts {
		project := normalizeContextProject(fact.Project)
		if project == "" || !eligibleContextSupportFact(fact) {
			continue
		}
		projectTokens := contextTokenSet(strings.Join(aliases[project], " "))
		factTokens := contextTokenSet(strings.Join([]string{
			fact.Search,
			fact.Name,
			fact.Qualified,
			fact.HTTPMethod,
			fact.Path,
			fact.Summary,
		}, " "))
		semanticMatches := 0
		for _, token := range queryTokens {
			if !projectTokens[token] && factTokens[token] {
				semanticMatches++
			}
		}
		ranked = append(ranked, rankedContextSupportFact{
			fact:            fact,
			project:         project,
			explicit:        explicitProjects[project],
			semanticMatches: semanticMatches,
			score:           contextSupportFactScore(fact, semanticMatches),
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		if left.explicit != right.explicit {
			return left.explicit
		}
		if left.semanticMatches != right.semanticMatches {
			return left.semanticMatches > right.semanticMatches
		}
		if left.score != right.score {
			return left.score > right.score
		}
		if left.project != right.project {
			return left.project < right.project
		}
		if left.fact.Kind != right.fact.Kind {
			return left.fact.Kind < right.fact.Kind
		}
		leftName := firstNonEmptyContext(left.fact.Qualified, left.fact.Name)
		rightName := firstNonEmptyContext(right.fact.Qualified, right.fact.Name)
		if leftName != rightName {
			return leftName < rightName
		}
		if left.fact.File != right.fact.File {
			return left.fact.File < right.fact.File
		}
		if left.fact.Line != right.fact.Line {
			return left.fact.Line < right.fact.Line
		}
		return left.fact.ID < right.fact.ID
	})
	return ranked
}

func eligibleContextSupportFact(fact scan.AgentContextFactRecord) bool {
	kind := strings.ToLower(strings.TrimSpace(fact.Kind))
	if contextCapabilityForKind(kind) == "tests" || strings.Contains(kind, "generated") || strings.Contains(kind, "metadata") {
		return false
	}
	return !contextFactUsesTestSource(fact) && contextPackSourceFile(fact.File) != ""
}

func contextSupportFactScore(fact scan.AgentContextFactRecord, semanticMatches int) int {
	score := semanticMatches * scorePerMatchedTerm
	switch strings.ToLower(strings.TrimSpace(fact.Kind)) {
	case "route", "api_endpoint":
		score += scoreRouteKind
	case "symbol", "backend_handler":
		score += scoreSymbolKind
	}
	switch strings.ToUpper(strings.TrimSpace(fact.Confidence)) {
	case "EXACT":
		score += scoreExactConfidence
	case "RESOLVED":
		score += scoreResolvedConfidence
	}
	return score
}

func selectContextSupportFacts(
	ranked []rankedContextSupportFact,
	representedProjects map[string]bool,
) []rankedContextSupportFact {
	selected := make([]rankedContextSupportFact, 0, len(ranked))
	for _, candidate := range ranked {
		minimumMatches := 2
		if candidate.explicit {
			minimumMatches = 1
		}
		if candidate.semanticMatches >= minimumMatches && !representedProjects[candidate.project] {
			selected = append(selected, candidate)
		}
	}
	return selected
}

func deduplicateContextUncertainties(groups ...[]ContextUncertainty) []ContextUncertainty {
	seen := map[string]bool{}
	result := []ContextUncertainty{}
	for _, group := range groups {
		for _, uncertainty := range group {
			key := uncertainty.Scope + "\x00" + uncertainty.Reason
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, uncertainty)
		}
	}
	return result
}

func contextRepresentedProjects(
	pack ContextPack,
	includedFactIDs map[string]bool,
	factByID map[string]scan.AgentContextFactRecord,
) map[string]bool {
	represented := map[string]bool{}
	for factID := range includedFactIDs {
		if project := normalizeContextProject(factByID[factID].Project); project != "" {
			represented[project] = true
		}
	}
	for _, file := range pack.Files {
		if project := normalizeContextProject(file.Project); project != "" {
			represented[project] = true
		}
	}
	return represented
}

func contextProjectSupportUncertainties(
	explicitProjects map[string]bool,
	representedProjects map[string]bool,
	coverage []scan.AgentContextCoverageRecord,
) []ContextUncertainty {
	projects := make([]string, 0, len(explicitProjects))
	for project := range explicitProjects {
		if !representedProjects[project] {
			projects = append(projects, project)
		}
	}
	sort.Strings(projects)

	uncertainties := make([]ContextUncertainty, 0, len(projects))
	for _, project := range projects {
		reason := "no relevant production fact selected"
		if record, ok := strongestIncompleteContextProjectCoverage(project, coverage); ok {
			reason = strings.ToUpper(strings.TrimSpace(record.Coverage))
			if detail := strings.TrimSpace(record.Reason); detail != "" {
				reason += " — " + detail
			}
		}
		uncertainties = append(uncertainties, ContextUncertainty{
			Scope: project + "/project_context", Reason: reason,
		})
		if len(uncertainties) >= maximumContextUncertainty {
			break
		}
	}
	return uncertainties
}

func strongestIncompleteContextProjectCoverage(
	project string,
	coverage []scan.AgentContextCoverageRecord,
) (scan.AgentContextCoverageRecord, bool) {
	var strongest scan.AgentContextCoverageRecord
	strongestRank := 0
	for _, record := range coverage {
		if normalizeContextProject(record.Project) != project {
			continue
		}
		rank := contextIncompleteCoverageRank(record.Coverage)
		if rank == 0 {
			continue
		}
		if rank > strongestRank || rank == strongestRank && contextCoverageRecordLess(record, strongest) {
			strongest = record
			strongestRank = rank
		}
	}
	return strongest, strongestRank > 0
}

func contextIncompleteCoverageRank(coverage string) int {
	switch strings.ToUpper(strings.TrimSpace(coverage)) {
	case "FAILED":
		return 3
	case "PARTIAL":
		return 2
	case "UNAVAILABLE":
		return 1
	default:
		return 0
	}
}

func contextCoverageRecordLess(left, right scan.AgentContextCoverageRecord) bool {
	if left.Capability != right.Capability {
		return left.Capability < right.Capability
	}
	if left.Coverage != right.Coverage {
		return left.Coverage < right.Coverage
	}
	return left.Reason < right.Reason
}

func selectContextEndpoint(ranked []rankedContextFact, query string) (rankedContextFact, bool) {
	candidates := make([]rankedContextFact, 0)
	for _, candidate := range ranked {
		if candidate.score < minimumContextSeedScore ||
			!eligibleContextEndpoint(candidate.fact) ||
			!contextEndpointRouteMatchesQuery(candidate.fact, query) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return rankedContextFact{}, false
	}

	top := candidates[0]
	routeKey := contextEndpointRouteKey(top.fact)
	providers := map[string][]rankedContextFact{}
	for _, candidate := range ranked {
		if eligibleContextEndpoint(candidate.fact) && contextEndpointRouteKey(candidate.fact) == routeKey {
			providerKey := candidate.fact.Project + "\x00" + contextSummaryField(candidate.fact.Summary, "provider")
			providers[providerKey] = append(providers[providerKey], candidate)
		}
	}
	if len(providers) <= 1 {
		return top, true
	}
	if !contextEndpointMethodAndPathMatchQuery(top.fact, query) {
		return rankedContextFact{}, false
	}

	bestScore := 0
	bestProvider := ""
	tied := false
	for providerKey, providerCandidates := range providers {
		score := contextEndpointProviderQueryScore(providerCandidates[0].fact, query)
		switch {
		case score > bestScore:
			bestScore = score
			bestProvider = providerKey
			tied = false
		case score == bestScore && score > 0:
			tied = true
		}
	}
	if bestScore == 0 || tied {
		return rankedContextFact{}, false
	}
	return providers[bestProvider][0], true
}

func eligibleContextEndpoint(fact scan.AgentContextFactRecord) bool {
	return strings.EqualFold(fact.Kind, "api_endpoint") &&
		strings.TrimSpace(fact.HTTPMethod) != "" && strings.TrimSpace(fact.Path) != "" &&
		!contextFactUsesTestSource(fact) && !contextFactUsesGeneratedMetadata(fact)
}

func contextEndpointRouteKey(fact scan.AgentContextFactRecord) string {
	return strings.ToUpper(strings.TrimSpace(fact.HTTPMethod)) + "\x00" + strings.TrimSpace(fact.Path)
}

func contextEndpointRouteMatchesQuery(fact scan.AgentContextFactRecord, query string) bool {
	queryTokens := contextTokenSet(query)
	for token := range contextTokenSet(strings.TrimSpace(fact.HTTPMethod + " " + fact.Path)) {
		if queryTokens[token] {
			return true
		}
	}
	return false
}

func contextEndpointMethodAndPathMatchQuery(fact scan.AgentContextFactRecord, query string) bool {
	queryTokens := contextTokenSet(query)
	if !queryTokens[strings.ToLower(strings.TrimSpace(fact.HTTPMethod))] {
		return false
	}
	for token := range contextTokenSet(fact.Path) {
		if queryTokens[token] {
			return true
		}
	}
	return false
}

func contextEndpointProviderQueryScore(endpoint scan.AgentContextFactRecord, query string) int {
	queryTerm := normalizeContextTerm(query)
	providerValues := []string{endpoint.Project, contextSummaryField(endpoint.Summary, "provider")}
	score := 0
	queryTokens := contextTokenSet(query)
	routeTokens := contextTokenSet(strings.TrimSpace(endpoint.HTTPMethod + " " + endpoint.Path))
	for _, provider := range providerValues {
		providerTerm := normalizeContextTerm(provider)
		if providerTerm != "" && strings.Contains(queryTerm, providerTerm) {
			score += 100
		}
		for token := range contextTokenSet(provider) {
			if !routeTokens[token] && queryTokens[token] {
				score++
			}
		}
	}
	return score
}

func contextQueryTokens(value string) []string {
	tokens := contextTokens(value)
	expanded := append([]string(nil), tokens...)
	for _, token := range tokens {
		expanded = append(expanded, contextQueryTokenAliases[token]...)
	}
	sort.Strings(expanded)
	result := expanded[:0]
	for _, token := range expanded {
		if len(result) == 0 || result[len(result)-1] != token {
			result = append(result, token)
		}
	}
	return result
}

func contextPrimaryQuery(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	segments := strings.FieldsFunc(value, func(current rune) bool {
		return current == '.' || current == '?' || current == '!' || current == '\n' || current == '\r'
	})
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if strings.HasSuffix(segment, ":") && len(strings.Fields(segment)) <= 3 {
			continue
		}
		return segment
	}
	return value
}

var contextQueryTokenAliases = map[string][]string{
	"vorschrift":         {"regulation", "regulations"},
	"vorschriften":       {"regulation", "regulations"},
	"vorschriftendienst": {"regulation", "regulations"},
	"kataster":           {"cadaster", "cadasters"},
	"katasters":          {"cadaster", "cadasters"},
	"entferne":           {"delete", "remove"},
	"entfernen":          {"delete", "remove"},
	"entfernt":           {"delete", "remove"},
	"entfernung":         {"delete", "remove"},
	"gelöscht":           {"delete", "remove"},
	"loeschen":           {"delete", "remove"},
	"löschen":            {"delete", "remove"},
	"löscht":             {"delete", "remove"},
	"verbunden":          {"related"},
	"verbundene":         {"related"},
	"verbundenen":        {"related"},
	"verknüpft":          {"related"},
}

func selectContextSeeds(ranked []rankedContextFact) []rankedContextFact {
	for _, candidate := range ranked {
		if candidate.score < minimumContextSeedScore {
			break
		}
		if reliableProductionContextSeed(candidate.fact) {
			return []rankedContextFact{candidate}
		}
	}
	for _, candidate := range ranked {
		if candidate.score < minimumContextSeedScore {
			break
		}
		if strings.EqualFold(candidate.fact.Kind, "test") && candidate.exactClass > 0 {
			return []rankedContextFact{candidate}
		}
	}
	return nil
}

func reliableProductionContextSeed(fact scan.AgentContextFactRecord) bool {
	if contextFactUsesTestSource(fact) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(fact.Kind)) {
	case "route", "symbol", "backend_handler":
	default:
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(fact.Confidence)) {
	case "", "EXACT", "RESOLVED", "EXTRACTED":
		return true
	default:
		return false
	}
}

func contextFactUsesTestSource(fact scan.AgentContextFactRecord) bool {
	path := "/" + strings.TrimPrefix(
		strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fact.File), "\\", "/")),
		"/",
	)
	for _, marker := range []string{"/src/test/", "/test/", "/tests/", "/__tests__/"} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return strings.HasSuffix(path, "_test.go") ||
		strings.Contains(path, ".test.") || strings.Contains(path, ".spec.")
}

func contextTokens(value string) []string {
	words := contextOrderedTokens(value)
	sort.Strings(words)
	result := words[:0]
	for _, word := range words {
		if len(result) == 0 || result[len(result)-1] != word {
			result = append(result, word)
		}
	}
	return result
}

func contextOrderedTokens(value string) []string {
	runes := []rune(value)
	words := []string{}
	current := []rune{}
	flush := func() {
		if len(current) == 0 {
			return
		}
		word := strings.ToLower(string(current))
		if len([]rune(word)) >= 2 || contextHTTPVerbs[strings.ToUpper(word)] {
			words = append(words, word)
		}
		current = nil
	}
	for index, currentRune := range runes {
		if !unicode.IsLetter(currentRune) && !unicode.IsDigit(currentRune) {
			flush()
			continue
		}
		if len(current) > 0 && unicode.IsUpper(currentRune) {
			previous := runes[index-1]
			nextLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) ||
				unicode.IsUpper(previous) && nextLower {
				flush()
			}
		}
		current = append(current, currentRune)
	}
	flush()
	return words
}

var contextHTTPVerbs = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true,
}

func contextTokenSet(value string) map[string]bool {
	set := map[string]bool{}
	for _, token := range contextTokens(value) {
		set[token] = true
	}
	return set
}

func normalizeContextTerm(value string) string {
	return strings.Join(contextOrderedTokens(value), " ")
}

func expandContextEdges(
	edges []scan.AgentContextEdgeRecord,
	factByID map[string]scan.AgentContextFactRecord,
	seeds []rankedContextFact,
) []expandedContextEdge {
	sortedEdges := append([]scan.AgentContextEdgeRecord(nil), edges...)
	sort.Slice(sortedEdges, func(i, j int) bool {
		return contextEdgeLess(sortedEdges[i], sortedEdges[j])
	})
	seen := map[string]bool{}
	result := []expandedContextEdge{}
	type contextExpansionFrontier struct {
		factID   string
		seedRank int
	}
	frontier := []contextExpansionFrontier{}
	appendEdge := func(edge scan.AgentContextEdgeRecord, seedRank int, neighbor scan.AgentContextFactRecord) {
		key := edge.ID
		if key == "" {
			key = edge.FromFactID + "\x00" + edge.ToFactID + "\x00" + edge.Kind
		}
		if seen[key] {
			return
		}
		seen[key] = true
		result = append(result, expandedContextEdge{
			edge: edge, seedRank: seedRank, neighbor: neighbor,
		})
	}
	for seedRank, seed := range seeds {
		for _, edge := range sortedEdges {
			if contextStructuredEndpointEdge(edge.Kind) || edge.FromFactID == edge.ToFactID ||
				edge.FromFactID != seed.fact.ID && edge.ToFactID != seed.fact.ID {
				continue
			}
			neighborID := edge.ToFactID
			if neighborID == seed.fact.ID {
				neighborID = edge.FromFactID
			}
			neighbor, ok := factByID[neighborID]
			if !ok {
				continue
			}
			appendEdge(edge, seedRank, neighbor)
			if edge.FromFactID == seed.fact.ID && reliableProductionContextSeed(neighbor) {
				frontier = append(frontier, contextExpansionFrontier{factID: neighbor.ID, seedRank: seedRank})
			}
		}
	}
	for _, current := range frontier {
		for _, edge := range sortedEdges {
			if contextStructuredEndpointEdge(edge.Kind) || edge.FromFactID != current.factID || edge.FromFactID == edge.ToFactID {
				continue
			}
			neighbor, ok := factByID[edge.ToFactID]
			if !ok {
				continue
			}
			appendEdge(edge, current.seedRank, neighbor)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].seedRank != result[j].seedRank {
			return result[i].seedRank < result[j].seedRank
		}
		return contextEdgeLess(result[i].edge, result[j].edge)
	})
	return result
}

func contextStructuredEndpointEdge(kind string) bool {
	return kind == "consumes_endpoint" || kind == "requires_auth"
}

func contextEdgeLess(left, right scan.AgentContextEdgeRecord) bool {
	if left.FromLabel != right.FromLabel {
		return left.FromLabel < right.FromLabel
	}
	if left.ToLabel != right.ToLabel {
		return left.ToLabel < right.ToLabel
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	if left.Reason != right.Reason {
		return left.Reason < right.Reason
	}
	if left.Confidence != right.Confidence {
		return left.Confidence < right.Confidence
	}
	if left.FromFactID != right.FromFactID {
		return left.FromFactID < right.FromFactID
	}
	if left.ToFactID != right.ToFactID {
		return left.ToFactID < right.ToFactID
	}
	return left.ID < right.ID
}

func distinctContextNeighbors(
	expanded []expandedContextEdge,
	seedIDs map[string]bool,
) []expandedContextEdge {
	byID := map[string]expandedContextEdge{}
	for _, candidate := range expanded {
		if seedIDs[candidate.neighbor.ID] {
			continue
		}
		existing, ok := byID[candidate.neighbor.ID]
		if !ok || candidate.seedRank < existing.seedRank ||
			candidate.seedRank == existing.seedRank && contextEdgeLess(candidate.edge, existing.edge) {
			byID[candidate.neighbor.ID] = candidate
		}
	}
	result := make([]expandedContextEdge, 0, len(byID))
	for _, candidate := range byID {
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].seedRank != result[j].seedRank {
			return result[i].seedRank < result[j].seedRank
		}
		left, right := result[i].neighbor, result[j].neighbor
		if left.Project != right.Project {
			return left.Project < right.Project
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Qualified != right.Qualified {
			return left.Qualified < right.Qualified
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})
	return result
}

func contextRelationship(
	edge scan.AgentContextEdgeRecord,
	from scan.AgentContextFactRecord,
	to scan.AgentContextFactRecord,
) ContextRelationship {
	return ContextRelationship{
		From:       firstNonEmptyContext(edge.FromLabel, contextFactLabel(from)),
		To:         firstNonEmptyContext(edge.ToLabel, contextFactLabel(to)),
		Kind:       edge.Kind,
		Reason:     edge.Reason,
		Confidence: edge.Confidence,
	}
}

type contextEndpointConsumerCandidate struct {
	fact scan.AgentContextFactRecord
	edge scan.AgentContextEdgeRecord
}

func tryAddContextEndpoint(
	pack ContextPack,
	request ContextRequest,
	fact scan.AgentContextFactRecord,
	index scan.AgentContextIndexRecord,
) (ContextPack, bool, error) {
	consumers := contextEndpointConsumers(fact.ID, index.Facts, index.Edges)
	endpoint := contextEndpointForFact(fact, index.Facts, index.Edges)
	endpoint.OmittedConsumers = contextEndpointIndexedOmittedConsumers(fact.Summary) + len(consumers)

	pack, added, err := tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
		candidate.Endpoints = append(candidate.Endpoints, endpoint)
		return mergeContextFile(
			candidate,
			contextFileForFact(fact, "endpoint", "selected API endpoint"),
			request.MaxFiles,
		)
	})
	if err != nil || !added {
		return pack, added, err
	}

	acceptedConsumers := 0
	for _, consumer := range consumers {
		if acceptedConsumers >= maximumContextConsumers {
			break
		}
		consumerRecord := contextEndpointConsumerForFact(consumer.fact, consumer.edge)
		candidate, accepted, appendErr := tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
			endpointIndex := len(candidate.Endpoints) - 1
			candidate.Endpoints[endpointIndex].Consumers = append(
				candidate.Endpoints[endpointIndex].Consumers,
				consumerRecord,
			)
			candidate.Endpoints[endpointIndex].OmittedConsumers--
			return mergeContextFile(
				candidate,
				contextFileForFact(consumer.fact, "endpoint_consumer", "calls selected API endpoint"),
				request.MaxFiles,
			)
		})
		if appendErr != nil {
			return ContextPack{}, false, appendErr
		}
		if accepted {
			pack = candidate
			acceptedConsumers++
		}
	}
	return pack, true, nil
}

func contextEndpointForFact(
	fact scan.AgentContextFactRecord,
	facts []scan.AgentContextFactRecord,
	edges []scan.AgentContextEdgeRecord,
) ContextEndpoint {
	securityFacts := contextEndpointSecurityFacts(fact, facts, edges)
	security := contextSummaryField(fact.Summary, "security")
	if security == "" {
		securityLabels := make([]string, 0, len(securityFacts))
		for _, securityFact := range securityFacts {
			securityLabels = append(securityLabels, securityFact.Name)
		}
		security = contextAuthenticationLabel(securityLabels...)
	} else {
		security = contextAuthenticationLabel(security)
	}
	return ContextEndpoint{
		Provider:           fact.Project,
		HTTPMethod:         strings.ToUpper(strings.TrimSpace(fact.HTTPMethod)),
		Path:               strings.TrimSpace(fact.Path),
		Handler:            strings.TrimSpace(fact.Qualified),
		File:               strings.TrimSpace(fact.File),
		Line:               fact.Line,
		RequestType:        contextSummaryField(fact.Summary, "request"),
		ResponseType:       contextSummaryField(fact.Summary, "response"),
		Security:           security,
		SecurityConfidence: contextEndpointSecurityConfidence(securityFacts),
	}
}

func contextEndpointConsumers(
	endpointID string,
	facts []scan.AgentContextFactRecord,
	edges []scan.AgentContextEdgeRecord,
) []contextEndpointConsumerCandidate {
	factByID := make(map[string]scan.AgentContextFactRecord, len(facts))
	for _, fact := range facts {
		factByID[fact.ID] = fact
	}
	byFactID := map[string]contextEndpointConsumerCandidate{}
	for _, edge := range edges {
		if edge.Kind != "consumes_endpoint" || edge.ToFactID != endpointID {
			continue
		}
		fact, ok := factByID[edge.FromFactID]
		if !ok || fact.Kind != "api_consumer" || contextFactUsesGeneratedMetadata(fact) {
			continue
		}
		candidate := contextEndpointConsumerCandidate{fact: fact, edge: edge}
		existing, found := byFactID[fact.ID]
		if !found || contextEdgeLess(edge, existing.edge) {
			byFactID[fact.ID] = candidate
		}
	}
	result := make([]contextEndpointConsumerCandidate, 0, len(byFactID))
	for _, candidate := range byFactID {
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		leftTest := contextFactUsesTestSource(result[i].fact)
		rightTest := contextFactUsesTestSource(result[j].fact)
		if leftTest != rightTest {
			return !leftTest
		}
		left, right := result[i].fact, result[j].fact
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
	})
	return result
}

func contextEndpointConsumerForFact(
	fact scan.AgentContextFactRecord,
	edge scan.AgentContextEdgeRecord,
) ContextEndpointConsumer {
	authentication := contextSummaryField(fact.Summary, "auth")
	if authentication == "" {
		authentication = contextAuthenticationLabel(edge.Reason, fact.Summary, fact.Search)
	} else {
		authentication = contextAuthenticationLabel(authentication)
	}
	return ContextEndpointConsumer{
		Project:        fact.Project,
		File:           fact.File,
		Line:           fact.Line,
		Authentication: authentication,
		Confidence:     firstNonEmptyContext(fact.Confidence, edge.Confidence),
	}
}

func contextSummaryField(summary, key string) string {
	for _, part := range strings.Split(summary, ";") {
		part = strings.TrimSpace(part)
		prefix := key + " "
		if strings.HasPrefix(strings.ToLower(part), prefix) {
			return strings.TrimSpace(part[len(prefix):])
		}
	}
	return ""
}

func contextAuthenticationLabel(values ...string) string {
	kinds := map[string]bool{}
	for _, value := range values {
		for _, token := range strings.FieldsFunc(strings.ToLower(value), func(current rune) bool {
			return !unicode.IsLetter(current) && !unicode.IsDigit(current) && current != '_'
		}) {
			switch token {
			case scan.SecurityBasic, scan.SecurityBearer, scan.SecurityOAuth2, scan.SecurityAPIKey,
				scan.SecuritySession, scan.SecurityMTLS, scan.SecurityRole, scan.SecurityAuthenticated,
				scan.SecurityPublic, scan.SecurityUnknown:
				kinds[token] = true
			}
		}
	}
	delete(kinds, scan.SecurityUnknown)
	if len(kinds) == 0 {
		return noAuthEvidenceDetected
	}
	labels := make([]string, 0, len(kinds))
	for kind := range kinds {
		labels = append(labels, kind)
	}
	sort.Strings(labels)
	return strings.Join(labels, ", ")
}

func contextEndpointSecurityFacts(
	endpoint scan.AgentContextFactRecord,
	facts []scan.AgentContextFactRecord,
	edges []scan.AgentContextEdgeRecord,
) []scan.AgentContextFactRecord {
	securityFactIDs := map[string]bool{}
	for _, edge := range edges {
		if edge.FromFactID == endpoint.ID && edge.Kind == "requires_auth" {
			securityFactIDs[edge.ToFactID] = true
		}
	}
	prefix := strings.ToLower(strings.TrimSpace(endpoint.HTTPMethod+" "+endpoint.Path)) + " "
	result := make([]scan.AgentContextFactRecord, 0)
	for _, fact := range facts {
		if fact.Kind != "endpoint_security" {
			continue
		}
		direct := securityFactIDs[fact.ID]
		qualified := strings.ToLower(strings.TrimSpace(fact.Qualified))
		routeMatch := fact.Project == endpoint.Project && strings.HasPrefix(qualified, prefix) &&
			contextKnownSecurityKind(strings.TrimSpace(strings.TrimPrefix(qualified, prefix)))
		if !direct && !routeMatch {
			continue
		}
		result = append(result, fact)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func contextEndpointSecurityConfidence(facts []scan.AgentContextFactRecord) string {
	confidence := ""
	for _, fact := range facts {
		confidence = strongerContextConfidence(confidence, fact.Confidence)
	}
	return confidence
}

func contextKnownSecurityKind(kind string) bool {
	switch kind {
	case scan.SecurityBasic, scan.SecurityBearer, scan.SecurityOAuth2, scan.SecurityAPIKey,
		scan.SecuritySession, scan.SecurityMTLS, scan.SecurityRole, scan.SecurityAuthenticated,
		scan.SecurityPublic, scan.SecurityUnknown:
		return true
	default:
		return false
	}
}

func contextEndpointIndexedOmittedConsumers(summary string) int {
	for _, part := range strings.Split(summary, ";") {
		var omitted int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d consumer call sites omitted", &omitted); err == nil && omitted > 0 {
			return omitted
		}
	}
	return 0
}

func contextFactUsesGeneratedMetadata(fact scan.AgentContextFactRecord) bool {
	return !contextSourceFileAllowed(fact.File)
}

func contextSourceFileAllowed(file string) bool {
	file = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(file), "\\", "/"))
	if file == "" {
		return true
	}
	return !strings.HasSuffix(file, "/api-catalog.json") && file != "api-catalog.json" &&
		!strings.HasSuffix(file, "/.goregraph-dashboard.json") && file != ".goregraph-dashboard.json"
}

func tryAddContextLocation(
	pack ContextPack,
	request ContextRequest,
	fact scan.AgentContextFactRecord,
	reason,
	role string,
	appendLocation func(*ContextPack, ContextLocation),
) (ContextPack, bool, error) {
	compacted := retainFirstContextEvidence(fact)
	location := contextLocation(compacted, reason)
	return tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
		appendLocation(candidate, location)
		return mergeContextFile(
			candidate,
			contextFileForFact(compacted, role, reason),
			request.MaxFiles,
		)
	})
}

func retainFirstContextEvidence(fact scan.AgentContextFactRecord) scan.AgentContextFactRecord {
	if len(fact.EvidenceIDs) > 1 {
		fact.EvidenceIDs = append([]string(nil), fact.EvidenceIDs[:1]...)
	} else {
		fact.EvidenceIDs = append([]string(nil), fact.EvidenceIDs...)
	}
	return fact
}

func contextLocation(fact scan.AgentContextFactRecord, reason string) ContextLocation {
	return ContextLocation{
		ID:          fact.ID,
		Project:     fact.Project,
		Kind:        fact.Kind,
		Label:       contextFactLabel(fact),
		File:        fact.File,
		Line:        fact.Line,
		EndLine:     fact.EndLine,
		Reason:      reason,
		Confidence:  fact.Confidence,
		EvidenceIDs: sortedContextStrings(fact.EvidenceIDs),
	}
}

func contextFactLabel(fact scan.AgentContextFactRecord) string {
	if fact.Kind == "route" || fact.Kind == "api_contract" {
		if methodPath := strings.TrimSpace(fact.HTTPMethod + " " + fact.Path); methodPath != "" {
			return methodPath
		}
	}
	return firstNonEmptyContext(fact.Qualified, fact.Name, fact.ID)
}

func contextFileForFact(fact scan.AgentContextFactRecord, role, reason string) ContextFile {
	endLine := fact.EndLine
	if endLine == 0 {
		endLine = fact.Line
	}
	return ContextFile{
		Project: fact.Project, Path: contextPackSourceFile(fact.File),
		StartLine: fact.Line, EndLine: endLine,
		Role: role, Reason: reason, Confidence: fact.Confidence,
	}
}

func contextPackSourceFile(file string) string {
	if !contextSourceFileAllowed(file) {
		return ""
	}
	return file
}

func mergeContextFile(pack *ContextPack, file ContextFile, maxFiles int) bool {
	if strings.TrimSpace(file.Path) == "" {
		return true
	}
	for index := range pack.Files {
		if pack.Files[index].Project != file.Project || pack.Files[index].Path != file.Path {
			continue
		}
		existing := pack.Files[index]
		existing.StartLine = minimumPositiveContextLine(existing.StartLine, file.StartLine)
		if file.EndLine > existing.EndLine {
			existing.EndLine = file.EndLine
		}
		existing.Role = mergeContextList(existing.Role, file.Role, ",")
		existing.Reason = mergeContextList(existing.Reason, file.Reason, ";")
		existing.Confidence = strongerContextConfidence(existing.Confidence, file.Confidence)
		pack.Files[index] = existing
		sortContextFiles(pack.Files)
		return true
	}
	if len(pack.Files) >= maxFiles {
		return false
	}
	pack.Files = append(pack.Files, file)
	sortContextFiles(pack.Files)
	return true
}

func tryContextPack(
	pack ContextPack,
	budget int,
	mutate func(*ContextPack) bool,
) (ContextPack, bool, error) {
	candidate := cloneContextPack(pack)
	if !mutate(&candidate) {
		return pack, false, nil
	}
	candidate, err := finalizeContextEstimate(candidate)
	if err != nil {
		return ContextPack{}, false, err
	}
	fits, err := contextPackFitsBudget(candidate, budget)
	if err != nil {
		return ContextPack{}, false, err
	}
	if !fits {
		return pack, false, nil
	}
	return candidate, true, nil
}

func contextPackFitsBudget(pack ContextPack, budget int) (bool, error) {
	if pack.EstimatedTokens > budget {
		return false, nil
	}
	body, err := json.Marshal(pack)
	if err != nil {
		return false, err
	}
	return len(body) <= budget*4, nil
}

func cloneContextPack(pack ContextPack) ContextPack {
	cloneLocations := func(values []ContextLocation) []ContextLocation {
		result := append([]ContextLocation(nil), values...)
		for index := range result {
			result[index].EvidenceIDs = append([]string(nil), result[index].EvidenceIDs...)
		}
		return result
	}
	pack.Entrypoints = cloneLocations(pack.Entrypoints)
	pack.Endpoints = append([]ContextEndpoint(nil), pack.Endpoints...)
	for index := range pack.Endpoints {
		pack.Endpoints[index].Consumers = append(
			[]ContextEndpointConsumer(nil),
			pack.Endpoints[index].Consumers...,
		)
		pack.Endpoints[index].Limitations = append([]string(nil), pack.Endpoints[index].Limitations...)
	}
	pack.CallChain = append([]ContextRelationship(nil), pack.CallChain...)
	pack.Contracts = cloneLocations(pack.Contracts)
	pack.Persistence = cloneLocations(pack.Persistence)
	pack.Tests = cloneLocations(pack.Tests)
	pack.Files = append([]ContextFile(nil), pack.Files...)
	pack.Uncertainties = append([]ContextUncertainty(nil), pack.Uncertainties...)
	return pack
}

func contextPackConfidence(top rankedContextFact, hasRelationship bool) string {
	if top.exactClass > 0 && strings.EqualFold(top.fact.Confidence, "EXACT") {
		return "EXACT"
	}
	if top.allTerms && hasRelationship {
		return "HIGH"
	}
	if top.score >= 240 {
		return "MEDIUM"
	}
	return "LOW"
}

func selectedContextScopes(
	edges []scan.AgentContextEdgeRecord,
	includedFactIDs,
	acceptedEdgeIDs map[string]bool,
	factByID map[string]scan.AgentContextFactRecord,
) map[string]bool {
	scopes := map[string]bool{}
	for factID := range includedFactIDs {
		fact := factByID[factID]
		project := normalizeContextProject(fact.Project)
		capability := contextCapabilityForKind(fact.Kind)
		if capability == "" {
			scopes[project+"\x00"] = true
			continue
		}
		scopes[project+"\x00"+capability] = true
	}
	for _, edge := range edges {
		if !acceptedEdgeIDs[edge.ID] {
			continue
		}
		capability := contextCapabilityForKind(edge.Kind)
		if capability == "" {
			continue
		}
		project := normalizeContextProject(firstNonEmptyContext(
			edge.Project,
			factByID[edge.FromFactID].Project,
			factByID[edge.ToFactID].Project,
		))
		scopes[project+"\x00"+capability] = true
	}
	return scopes
}

func contextCapabilityForKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "route":
		return "routes"
	case "api_contract", "http_contract":
		return "api_clients"
	case "test", "test_target":
		return "tests"
	case "persistence":
		return "persistence"
	case "symbol", "backend_handler", "call", "calls", "use", "extends", "implements":
		return "calls"
	}
	if strings.Contains(kind, "_to_") || strings.Contains(kind, "trace") {
		return "calls"
	}
	return ""
}

func scopedContextUncertainties(
	coverage []scan.AgentContextCoverageRecord,
	scopes map[string]bool,
) ([]ContextUncertainty, bool) {
	records := append([]scan.AgentContextCoverageRecord(nil), coverage...)
	for index := range records {
		records[index].Project = normalizeContextProject(records[index].Project)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Project != records[j].Project {
			return records[i].Project < records[j].Project
		}
		if records[i].Capability != records[j].Capability {
			return records[i].Capability < records[j].Capability
		}
		if records[i].Coverage != records[j].Coverage {
			return records[i].Coverage < records[j].Coverage
		}
		return records[i].Reason < records[j].Reason
	})
	type coverageState struct {
		complete bool
		record   scan.AgentContextCoverageRecord
		has      bool
	}
	states := map[string]coverageState{}
	for _, record := range records {
		key := record.Project + "\x00" + record.Capability
		if !scopes[key] {
			continue
		}
		state := states[key]
		switch strings.ToUpper(strings.TrimSpace(record.Coverage)) {
		case "COMPLETE":
			state.has = true
			state.complete = true
		case "PARTIAL", "UNAVAILABLE", "FAILED":
			state.has = true
			if state.record.Coverage == "" {
				state.record = record
			}
		default:
			continue
		}
		states[key] = state
	}
	keys := make([]string, 0, len(scopes))
	for key := range scopes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	allIncomplete := len(keys) > 0
	uncertainties := []ContextUncertainty{}
	for _, key := range keys {
		state, ok := states[key]
		if !ok || !state.has || state.complete {
			allIncomplete = false
			continue
		}
		project, capability, _ := strings.Cut(key, "\x00")
		uncertainties = append(uncertainties, ContextUncertainty{
			Scope:  project + "/" + capability,
			Reason: strings.TrimSpace(state.record.Coverage + " — " + state.record.Reason),
		})
	}
	return uncertainties, allIncomplete
}

func sortContextRelationships(values []ContextRelationship) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].From != values[j].From {
			return values[i].From < values[j].From
		}
		if values[i].To != values[j].To {
			return values[i].To < values[j].To
		}
		if values[i].Kind != values[j].Kind {
			return values[i].Kind < values[j].Kind
		}
		if values[i].Reason != values[j].Reason {
			return values[i].Reason < values[j].Reason
		}
		return values[i].Confidence < values[j].Confidence
	})
}

func sortContextFiles(values []ContextFile) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Project != values[j].Project {
			return values[i].Project < values[j].Project
		}
		if values[i].Path != values[j].Path {
			return values[i].Path < values[j].Path
		}
		if values[i].StartLine != values[j].StartLine {
			return values[i].StartLine < values[j].StartLine
		}
		if values[i].EndLine != values[j].EndLine {
			return values[i].EndLine < values[j].EndLine
		}
		return values[i].Role < values[j].Role
	})
}

func sortedContextStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func mergeContextList(left, right, separator string) string {
	values := strings.Split(left, separator)
	values = append(values, strings.Split(right, separator)...)
	return strings.Join(sortedContextStrings(values), separator)
}

func strongerContextConfidence(left, right string) string {
	rank := func(value string) int {
		switch strings.ToUpper(strings.TrimSpace(value)) {
		case "EXACT":
			return 5
		case "RESOLVED":
			return 4
		case "EXTRACTED":
			return 3
		case "INFERRED":
			return 2
		case "":
			return 0
		default:
			return 1
		}
	}
	if rank(right) > rank(left) ||
		rank(right) == rank(left) && right != "" && (left == "" || right < left) {
		return right
	}
	return left
}

func minimumPositiveContextLine(left, right int) int {
	switch {
	case left <= 0:
		return right
	case right <= 0:
		return left
	case right < left:
		return right
	default:
		return left
	}
}

func firstNonEmptyContext(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
