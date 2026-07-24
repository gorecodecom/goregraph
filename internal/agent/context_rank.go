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
	scoreExactRoute                           = 1000
	scoreEmbeddedExact                        = scoreExactRoute + 100
	scoreExactQualified                       = 900
	scoreExactName                            = 800
	scoreAllTerms                             = 500
	scorePerMatchedTerm                       = 60
	scoreRouteKind                            = 80
	scoreSymbolKind                           = 60
	scoreTestKind                             = 20
	scoreExactConfidence                      = 30
	scoreResolvedConfidence                   = 15
	minimumContextSeedScore                   = 180
	maximumContextUncertainty                 = 3
	maximumContextConsumers                   = 8
	maximumContextSupportingProjects          = 2
	maximumContextSupportFactsPerProject      = 5
	maximumContextSupportCandidatesPerProject = 8
)

const (
	noAuthEvidenceDetected                 = "No auth evidence detected"
	contextEndpointProviderAmbiguityReason = "matching endpoint provider is ambiguous; include the provider project or service"
	contextEndpointActionMismatchReason    = "matching production endpoints do not align with the requested primary action"
)

type rankedContextFact struct {
	fact          scan.AgentContextFactRecord
	query         string
	score         int
	exactClass    int
	embeddedExact bool
	matchedTerms  int
	routeExtras   int
	allTerms      bool
	reason        string
}

type rankedContextSupportFact struct {
	fact               scan.AgentContextFactRecord
	project            string
	explicit           bool
	semanticMatches    int
	requestedMatches   int
	operational        bool
	role               string
	genericPersistence bool
	score              int
}

func compileContextPack(index scan.AgentContextIndexRecord, request ContextRequest) (ContextPack, error) {
	ranked := rankContextFacts(index.Facts, request.Query)
	seeds := selectContextSeeds(ranked)
	endpointSeed, hasEndpoint, endpointFallbackReason := selectContextEndpoint(index, ranked, request.Query)
	if endpointFallbackReason != "" {
		return fallbackContextPack(index, request, endpointFallbackReason, nil)
	}
	if hasEndpoint {
		seeds = []rankedContextFact{endpointSeed}
	}
	if len(seeds) == 0 {
		if endpointFallbackReason == "" {
			endpointFallbackReason = "no sufficiently relevant context fact found"
		}
		return fallbackContextPack(index, request, endpointFallbackReason, nil)
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
	fits, err := contextPackFitsBudget(pack, request.BudgetTokens)
	if err != nil {
		return ContextPack{}, err
	}
	if !fits {
		return ContextPack{}, fmt.Errorf(
			"context envelope exceeds budget %d",
			request.BudgetTokens,
		)
	}
	includedFactIDs := map[string]bool{}
	acceptedEdgeIDs := map[string]bool{}
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
	pathTop := top
	if topIsEndpoint {
		if companion, ok := contextEndpointCompanion(index, top.fact); ok {
			pathTop.fact = companion
			includedFactIDs[companion.ID] = true
		}
	}
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
		}
	}

	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	concerns := planContextConcerns(request.Query, index, pathTop.fact)
	pathSelection := selectContextPaths(index, pathTop, concerns)
	pack, err = addSelectedContextPaths(
		pack,
		request,
		pathTop,
		pathSelection,
		index.Edges,
		factByID,
		includedFactIDs,
		acceptedEdgeIDs,
	)
	if err != nil {
		return ContextPack{}, err
	}

	pack.Confidence = contextPackConfidence(top, len(pack.CallChain) > 0)
	pack, err = finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}
	fits, err = contextPackFitsBudget(pack, request.BudgetTokens)
	if err != nil {
		return ContextPack{}, err
	}
	if !fits {
		return ContextPack{}, fmt.Errorf(
			"context pack exceeds budget %d",
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
	var domainModelTokens map[string]bool
	if contextQueryRequestsConcern(request.Query, contextConcernDomainModel) {
		domainModelTokens = contextDomainModelQueryTokens(request.Query, projectAliases, explicitProjects)
	}
	representedProjects := contextRepresentedProjects(pack, includedFactIDs, factByID)
	supportFactIDs := map[string]bool{}
	supportProjectCounts := map[string]int{}
	supportProjectRoles := map[string]map[string]bool{}
	acceptedSupportProjects := 0
	primaryProjects := make(map[string]bool, len(representedProjects))
	for project, represented := range representedProjects {
		primaryProjects[project] = represented
	}
	for _, relatedFact := range pathSelection.relatedProductionFacts {
		project := normalizeContextProject(relatedFact.Project)
		if primaryProjects[project] || supportProjectCounts[project] >= maximumContextSupportFactsPerProject {
			continue
		}
		if supportProjectCounts[project] > 0 {
			role := contextSupportRole(index, relatedFact, domainModelTokens)
			if role != "" && supportProjectRoles[project][role] {
				continue
			}
		}
		if supportProjectCounts[project] == 0 && acceptedSupportProjects >= maximumContextSupportingProjects {
			continue
		}
		relatedFact = lowerConfidenceForRelatedProduction(relatedFact)
		candidate := pack
		accepted := false
		var appendErr error
		if role, appendLocation := selectedContextLocationAppender(relatedFact.Kind); appendLocation != nil {
			candidate, accepted, appendErr = tryAddContextLocation(
				pack,
				request,
				relatedFact,
				"related project task and role match",
				role,
				appendLocation,
			)
		} else {
			candidate, accepted, appendErr = tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
				return mergeContextFile(
					candidate,
					contextFileForFact(relatedFact, "related_project", "full task project match"),
					request.MaxFiles,
				)
			})
		}
		if appendErr != nil {
			return ContextPack{}, appendErr
		}
		if accepted {
			pack = candidate
			includedFactIDs[relatedFact.ID] = true
			supportFactIDs[relatedFact.ID] = true
			representedProjects[project] = true
			supportProjectCounts[project]++
			if supportProjectCounts[project] == 1 {
				acceptedSupportProjects++
			}
			if role := contextSupportRole(index, relatedFact, domainModelTokens); role != "" {
				if supportProjectRoles[project] == nil {
					supportProjectRoles[project] = map[string]bool{}
				}
				supportProjectRoles[project][role] = true
			}
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
	selectedSourceFactIDs := make(map[string]bool, len(includedFactIDs)+len(pathSelection.factIDs))
	for factID := range includedFactIDs {
		selectedSourceFactIDs[factID] = true
	}
	for _, factID := range pathSelection.factIDs {
		selectedSourceFactIDs[factID] = true
	}
	retainSelectedSourceFactIDs(&pack, selectedSourceFactIDs)
	retainContextSemanticSelection(&pack, includedFactIDs, acceptedEdgeIDs, concerns)
	return finalizeContextEstimate(pack)
}

func retainContextSemanticSelection(
	pack *ContextPack,
	factIDs, edgeIDs map[string]bool,
	concerns []contextConcern,
) {
	pack.selectedFactIDs = contextSelectedMapKeys(factIDs)
	pack.selectedEdgeIDs = contextSelectedMapKeys(edgeIDs)
	pack.selectedConcernKeys = pack.selectedConcernKeys[:0]
	for _, concern := range concerns {
		pack.selectedConcernKeys = append(pack.selectedConcernKeys, concern.key)
	}
	sort.Strings(pack.selectedConcernKeys)
}

func contextSelectedMapKeys(selected map[string]bool) []string {
	keys := make([]string, 0, len(selected))
	for key, included := range selected {
		if included {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func contextRetryPermission(pack ContextPack, index scan.AgentContextIndexRecord) (bool, []string) {
	if pack.FallbackRequired || len(pack.Concerns) == 0 {
		return false, nil
	}
	selectionQuery := contextSelectionQuery(pack)
	seed, ok := contextConcernPlanningSeed(index, selectionQuery)
	if !ok {
		return false, nil
	}
	planned := planContextConcerns(selectionQuery, index, seed)
	plannedByKey := make(map[string]contextConcern, len(planned))
	for _, concern := range planned {
		plannedByKey[concern.key] = concern
	}
	selected := make(map[string]bool, len(pack.selectedFactIDs)+len(pack.selectedSourceFactIDs))
	selectedIDs := pack.selectedFactIDs
	if len(selectedIDs) == 0 {
		selectedIDs = pack.selectedSourceFactIDs
	}
	for _, factID := range selectedIDs {
		selected[factID] = true
	}
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	candidates := []rankedContextRetryCandidate{}
	for _, public := range pack.Concerns {
		if public.Covered {
			continue
		}
		concern, exists := plannedByKey[contextPublicConcernKey(public)]
		if !exists || !concern.required {
			continue
		}
		for _, factID := range concern.candidateFactIDs {
			fact, exists := factByID[factID]
			omissionMatch := exists && contextRetryFactMatchesOmission(pack, fact)
			if !exists || selected[factID] ||
				normalizeContextProject(public.Project) != "" &&
					normalizeContextProject(fact.Project) != normalizeContextProject(public.Project) ||
				len(pack.SourceOmissions) > 0 && !omissionMatch ||
				!contextRetryFactMatchesAction(fact, index, selectionQuery) ||
				!contextRetryFactHasSourceEvidence(pack, fact) {
				continue
			}
			anchor := contextRetryAnchor(fact)
			if anchor == "" {
				continue
			}
			candidates = append(candidates, rankedContextRetryCandidate{
				fact:           fact,
				anchor:         anchor,
				omissionMatch:  omissionMatch,
				qualifiedMatch: contextRetryQualifiedMatchesQuery(fact, selectionQuery),
				semanticScore:  contextRetrySemanticScore(fact, selectionQuery),
				confidence:     contextRetryConfidenceRank(fact.Confidence),
			})
		}
	}
	candidates = rankContextRetryCandidates(candidates)
	hasOmissionMatch := false
	for _, candidate := range candidates {
		hasOmissionMatch = hasOmissionMatch || candidate.omissionMatch
	}
	anchors := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if hasOmissionMatch && !candidate.omissionMatch {
			continue
		}
		if seen[candidate.anchor] {
			continue
		}
		seen[candidate.anchor] = true
		anchors = append(anchors, candidate.anchor)
		if len(anchors) == 3 {
			break
		}
	}
	if len(anchors) > 3 {
		anchors = anchors[:3]
	}
	return len(anchors) > 0, anchors
}

type rankedContextRetryCandidate struct {
	fact           scan.AgentContextFactRecord
	anchor         string
	omissionMatch  bool
	qualifiedMatch bool
	semanticScore  int
	confidence     int
}

func rankContextRetryCandidates(
	candidates []rankedContextRetryCandidate,
) []rankedContextRetryCandidate {
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].omissionMatch != candidates[right].omissionMatch {
			return candidates[left].omissionMatch
		}
		if candidates[left].qualifiedMatch != candidates[right].qualifiedMatch {
			return candidates[left].qualifiedMatch
		}
		if candidates[left].semanticScore != candidates[right].semanticScore {
			return candidates[left].semanticScore > candidates[right].semanticScore
		}
		if candidates[left].confidence != candidates[right].confidence {
			return candidates[left].confidence > candidates[right].confidence
		}
		return candidates[left].fact.ID < candidates[right].fact.ID
	})
	return candidates
}

func contextRetryFactHasSourceEvidence(
	pack ContextPack,
	fact scan.AgentContextFactRecord,
) bool {
	if strings.TrimSpace(fact.File) == "" {
		return false
	}
	for _, omission := range pack.SourceOmissions {
		if normalizeContextProject(omission.Project) != normalizeContextProject(fact.Project) ||
			contextRetryPath(omission.Path) != contextRetryPath(fact.File) {
			continue
		}
		return strings.Contains(strings.ToLower(omission.Reason), "budget")
	}
	for _, section := range pack.SourceSections {
		if normalizeContextProject(section.Project) == normalizeContextProject(fact.Project) &&
			contextRetryPath(section.Path) == contextRetryPath(fact.File) {
			return true
		}
	}
	return true
}

func contextRetryFactMatchesOmission(
	pack ContextPack,
	fact scan.AgentContextFactRecord,
) bool {
	for _, omission := range pack.SourceOmissions {
		if normalizeContextProject(omission.Project) == normalizeContextProject(fact.Project) &&
			contextRetryPath(omission.Path) == contextRetryPath(fact.File) {
			return true
		}
	}
	return false
}

func contextRetryPath(value string) string {
	return strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "./")
}

func contextRetryQualifiedMatchesQuery(
	fact scan.AgentContextFactRecord,
	query string,
) bool {
	qualified := normalizeContextTerm(fact.Qualified)
	if qualified == "" {
		return false
	}
	for _, anchor := range contextQueryAnchors(query) {
		if normalizeContextTerm(anchor) == qualified {
			return true
		}
	}
	return false
}

func contextRetrySemanticScore(
	fact scan.AgentContextFactRecord,
	query string,
) int {
	queryTokens := contextExpandedTokenSet(query)
	factTokens := contextExpandedTokenSet(strings.Join([]string{
		fact.Name,
		fact.Qualified,
		fact.Search,
		fact.HTTPMethod,
		fact.Path,
	}, " "))
	score := 0
	for token := range queryTokens {
		if factTokens[token] {
			score++
		}
	}
	return score
}

func contextRetryConfidenceRank(confidence string) int {
	switch strings.ToUpper(strings.TrimSpace(confidence)) {
	case "EXACT":
		return 3
	case "RESOLVED":
		return 2
	case "PARTIAL":
		return 1
	default:
		return 0
	}
}

func contextRetryFactMatchesAction(
	fact scan.AgentContextFactRecord,
	index scan.AgentContextIndexRecord,
	query string,
) bool {
	requested := contextEndpointRequestedActions(query)
	if len(requested) == 0 {
		return true
	}
	factActions := contextActionFamilies(strings.Join([]string{
		fact.Name,
		fact.Qualified,
		fact.Search,
		fact.HTTPMethod,
		fact.Path,
	}, " "), fact.HTTPMethod)
	if len(factActions) > 0 {
		return contextActionFamiliesOverlap(requested, factActions)
	}
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, candidate := range index.Facts {
		factByID[candidate.ID] = candidate
	}
	for _, edge := range index.Edges {
		adjacentID := ""
		switch fact.ID {
		case edge.FromFactID:
			adjacentID = edge.ToFactID
		case edge.ToFactID:
			adjacentID = edge.FromFactID
		}
		if adjacentID == "" {
			continue
		}
		adjacent, ok := factByID[adjacentID]
		if !ok {
			continue
		}
		actions := contextActionFamilies(strings.Join([]string{
			adjacent.Name,
			adjacent.Qualified,
			adjacent.Search,
			adjacent.HTTPMethod,
			adjacent.Path,
		}, " "), adjacent.HTTPMethod)
		if contextActionFamiliesOverlap(requested, actions) {
			return true
		}
	}
	return false
}

func contextActionFamilies(value, httpMethod string) map[string]bool {
	tokens := contextExpandedTokenSet(value)
	result := map[string]bool{}
	families := map[string][]string{
		"create": {"add", "create", "insert", "new", "post"},
		"delete": {
			"delete", "deleted", "deletes", "deleting", "deletion", "deletions",
			"remove", "removed", "removes", "removing", "removal", "removals",
		},
		"read":   {"fetch", "find", "get", "list", "load", "read"},
		"update": {"change", "edit", "modify", "patch", "put", "update"},
	}
	for family, aliases := range families {
		for _, alias := range aliases {
			if tokens[alias] {
				result[family] = true
				break
			}
		}
	}
	switch strings.ToUpper(strings.TrimSpace(httpMethod)) {
	case "POST":
		result["create"] = true
	case "DELETE":
		result["delete"] = true
	case "GET", "HEAD":
		result["read"] = true
	case "PUT", "PATCH":
		result["update"] = true
	}
	return result
}

func contextActionFamiliesOverlap(left, right map[string]bool) bool {
	for family := range left {
		if right[family] {
			return true
		}
	}
	return false
}

func contextRetryAnchor(fact scan.AgentContextFactRecord) string {
	if method, route := strings.TrimSpace(fact.HTTPMethod), strings.TrimSpace(fact.Path); method != "" && route != "" {
		return strings.ToUpper(method) + " " + route
	}
	if qualified := strings.TrimSpace(fact.Qualified); qualified != "" &&
		!strings.ContainsFunc(qualified, unicode.IsSpace) {
		return qualified
	}
	return strings.TrimSpace(fact.File)
}

func addSelectedContextPaths(
	pack ContextPack,
	request ContextRequest,
	top rankedContextFact,
	selection contextPathSelection,
	edges []scan.AgentContextEdgeRecord,
	factByID map[string]scan.AgentContextFactRecord,
	includedFactIDs,
	acceptedEdgeIDs map[string]bool,
) (ContextPack, error) {
	edgeByID := make(map[string]scan.AgentContextEdgeRecord, len(edges))
	for _, edge := range edges {
		edgeByID[contextPathEdgeIdentity(edge)] = edge
	}
	for _, path := range selection.paths {
		for _, edgeID := range path.edgeIDs {
			edge, ok := edgeByID[edgeID]
			if !ok || acceptedEdgeIDs[edge.ID] {
				continue
			}
			from, fromExists := factByID[edge.FromFactID]
			to, toExists := factByID[edge.ToFactID]
			if !fromExists || !toExists ||
				!includedFactIDs[from.ID] && !includedFactIDs[to.ID] {
				break
			}
			candidate, accepted, err := tryContextPack(pack, request.BudgetTokens, func(candidate *ContextPack) bool {
				candidate.CallChain = append(candidate.CallChain, contextRelationship(edge, from, to))
				candidate.Confidence = contextPackConfidence(top, true)
				if !mergeContextFile(candidate, contextFileForFact(from, "call_chain", "selected "+edge.Kind), request.MaxFiles) {
					return false
				}
				return mergeContextFile(candidate, contextFileForFact(to, "call_chain", "selected "+edge.Kind), request.MaxFiles)
			})
			if err != nil {
				return ContextPack{}, err
			}
			if !accepted {
				break
			}
			pack = candidate
			acceptedEdgeIDs[edge.ID] = true
			includedFactIDs[from.ID] = true
			includedFactIDs[to.ID] = true
		}
	}
	sortContextRelationships(pack.CallChain)

	for _, factID := range selection.factIDs {
		if !includedFactIDs[factID] || factID == top.fact.ID {
			continue
		}
		fact := factByID[factID]
		role, appendLocation := selectedContextLocationAppender(fact.Kind)
		if appendLocation == nil {
			continue
		}
		reason := selectedContextFactReason(factID, edges, acceptedEdgeIDs)
		candidate, accepted, err := tryAddContextLocation(
			pack,
			request,
			fact,
			reason,
			role,
			appendLocation,
		)
		if err != nil {
			return ContextPack{}, err
		}
		if accepted {
			pack = candidate
		}
	}
	return pack, nil
}

func selectedContextLocationAppender(kind string) (string, func(*ContextPack, ContextLocation)) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "api_contract":
		return "contract", func(candidate *ContextPack, location ContextLocation) {
			candidate.Contracts = append(candidate.Contracts, location)
		}
	case "persistence":
		return "persistence", func(candidate *ContextPack, location ContextLocation) {
			candidate.Persistence = append(candidate.Persistence, location)
		}
	case "test":
		return "test", func(candidate *ContextPack, location ContextLocation) {
			candidate.Tests = append(candidate.Tests, location)
		}
	default:
		return "", nil
	}
}

func selectedContextFactReason(
	factID string,
	edges []scan.AgentContextEdgeRecord,
	acceptedEdgeIDs map[string]bool,
) string {
	sortedEdges := append([]scan.AgentContextEdgeRecord(nil), edges...)
	sort.Slice(sortedEdges, func(i, j int) bool { return contextEdgeLess(sortedEdges[i], sortedEdges[j]) })
	for _, edge := range sortedEdges {
		if !acceptedEdgeIDs[edge.ID] {
			continue
		}
		if edge.ToFactID == factID ||
			strings.EqualFold(edge.Kind, "test_target") && edge.FromFactID == factID {
			return "selected " + edge.Kind
		}
	}
	return "selected context path"
}

func lowerConfidenceForRelatedProduction(fact scan.AgentContextFactRecord) scan.AgentContextFactRecord {
	switch strings.ToUpper(strings.TrimSpace(fact.Confidence)) {
	case "EXACT":
		fact.Confidence = "RESOLVED"
	case "", "RESOLVED":
		fact.Confidence = "EXTRACTED"
	}
	return fact
}

func retainSelectedSourceFactIDs(pack *ContextPack, included map[string]bool) {
	pack.selectedSourceFactIDs = pack.selectedSourceFactIDs[:0]
	for id := range included {
		pack.selectedSourceFactIDs = append(pack.selectedSourceFactIDs, id)
	}
	sort.Strings(pack.selectedSourceFactIDs)
}

func rankContextFacts(facts []scan.AgentContextFactRecord, query string) []rankedContextFact {
	primaryQuery := contextPrimaryQuery(query)
	queryTokens := contextQueryTokens(primaryQuery)
	queryTerm := normalizeContextTerm(query)
	queryAnchors := contextQueryAnchors(query)
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
		embeddedExact := false
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
		case contextQueryHasAnchor(queryAnchors, routeTerm):
			exactClass = 3
			embeddedExact = true
			score += scoreEmbeddedExact
			reason = "embedded exact route"
		case contextQueryHasAnchor(queryAnchors, qualifiedTerm):
			exactClass = 2
			embeddedExact = true
			score += scoreEmbeddedExact
			reason = "embedded exact qualified name"
		case contextQueryHasAnchor(queryAnchors, normalizeContextTerm(fact.File)):
			exactClass = 2
			embeddedExact = true
			score += scoreEmbeddedExact
			reason = "embedded exact file"
		case contextQueryHasAnchor(queryAnchors, nameTerm):
			exactClass = 1
			embeddedExact = true
			score += scoreEmbeddedExact
			reason = "embedded exact name"
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
			fact:          fact,
			query:         query,
			score:         score,
			exactClass:    exactClass,
			embeddedExact: embeddedExact,
			matchedTerms:  matched,
			routeExtras:   routeExtras,
			allTerms:      allTerms,
			reason:        reason,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		if left.embeddedExact != right.embeddedExact {
			return left.embeddedExact
		}
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
	index scan.AgentContextIndexRecord,
	query string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
	representedProjects map[string]bool,
) []rankedContextSupportFact {
	supportQuery := contextPrimaryQuery(query)
	if strings.TrimSpace(supportQuery) == "" {
		supportQuery = query
	}
	queryTokens := contextExpandedTokens(supportQuery)
	requestedTokens := contextSupportRequestedTokens(query)
	var domainModelTokens map[string]bool
	if contextQueryRequestsConcern(query, contextConcernDomainModel) {
		domainModelTokens = contextDomainModelQueryTokens(query, aliases, explicitProjects)
	}
	utility := newContextForwardUtility(index)
	ranked := make([]rankedContextSupportFact, 0, len(index.Facts))
	for _, fact := range index.Facts {
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
		requestedMatches := 0
		for _, token := range queryTokens {
			if !projectTokens[token] && factTokens[token] {
				semanticMatches++
			}
		}
		for token := range requestedTokens {
			if !projectTokens[token] && factTokens[token] {
				requestedMatches++
			}
		}
		operational, operationalScore := contextSupportOperationalScore(
			utility,
			fact,
			query,
			domainModelTokens,
		)
		operationalScore += contextSupportProjectAffinityScore(
			fact,
			project,
			aliases,
			explicitProjects,
			representedProjects,
		)
		ranked = append(ranked, rankedContextSupportFact{
			fact:               fact,
			project:            project,
			explicit:           explicitProjects[project],
			semanticMatches:    semanticMatches,
			requestedMatches:   requestedMatches,
			operational:        operational,
			role:               contextSupportRole(index, fact, domainModelTokens),
			genericPersistence: contextGenericPersistenceFact(fact),
			score: contextSupportFactScore(fact, semanticMatches) +
				requestedMatches*scorePerMatchedTerm/2 + operationalScore,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		if left.explicit != right.explicit {
			return left.explicit
		}
		if left.role == contextConcernPersistence &&
			right.role == contextConcernPersistence &&
			left.genericPersistence != right.genericPersistence {
			return !left.genericPersistence
		}
		if left.score != right.score {
			return left.score > right.score
		}
		if left.semanticMatches != right.semanticMatches {
			return left.semanticMatches > right.semanticMatches
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

func contextSupportRequestedTokens(query string) map[string]bool {
	requested := make(map[string]bool)
	for _, token := range contextExpandedTokens(query) {
		requested[token] = true
	}
	for _, token := range contextExpandedTokens(contextPrimaryQuery(query)) {
		delete(requested, token)
	}
	return requested
}

func contextSupportProjectAffinityScore(
	fact scan.AgentContextFactRecord,
	factProject string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
	representedProjects map[string]bool,
) int {
	identifier := compactContextIdentifier(strings.Join([]string{
		fact.Name,
		fact.Qualified,
		fact.File,
	}, " "))
	if identifier == "" {
		return 0
	}
	best := 0
	for project := range explicitProjects {
		if project == factProject || representedProjects[project] {
			continue
		}
		for _, alias := range aliases[project] {
			basename := contextProjectBasename(normalizeContextProject(alias))
			for _, segment := range contextProjectIdentifierSegments(basename) {
				if distinctiveContextProjectIdentifierSegment(segment) && strings.Contains(identifier, segment) {
					best = 240
				}
			}
		}
	}
	return best
}

func compactContextIdentifier(value string) string {
	var result strings.Builder
	for _, current := range strings.ToLower(value) {
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			result.WriteRune(current)
		}
	}
	return result.String()
}

func contextProjectIdentifierSegments(value string) []string {
	parts := strings.FieldsFunc(strings.ToLower(value), func(current rune) bool {
		return !unicode.IsLetter(current) && !unicode.IsDigit(current)
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func distinctiveContextProjectIdentifierSegment(segment string) bool {
	if len([]rune(segment)) < 4 {
		return false
	}
	switch segment {
	case "app", "lib", "ms", "service", "services", "svc":
		return false
	default:
		return true
	}
}

func eligibleContextSupportFact(fact scan.AgentContextFactRecord) bool {
	kind := strings.ToLower(strings.TrimSpace(fact.Kind))
	if strings.Contains(kind, "generated") || strings.Contains(kind, "metadata") {
		return false
	}
	if normalizedContextConcernKind(kind) == contextConcernTests {
		return strings.EqualFold(kind, "test") && contextPackSourceFile(fact.File) != ""
	}
	return !contextFactUsesTestSource(fact) && contextPackSourceFile(fact.File) != ""
}

func contextSupportFactScore(fact scan.AgentContextFactRecord, semanticMatches int) int {
	score := semanticMatches * scorePerMatchedTerm
	switch strings.ToLower(strings.TrimSpace(fact.Kind)) {
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

func contextSupportOperationalScore(
	utility *contextForwardUtility,
	fact scan.AgentContextFactRecord,
	query string,
	domainModelTokens map[string]bool,
) (bool, int) {
	kind := strings.ToLower(strings.TrimSpace(fact.Kind))
	score := 0
	operational := false
	switch kind {
	case "api_endpoint", "route":
		if contextEndpointMethodAndPathMatchQuery(fact, contextPrimaryQuery(query)) {
			score += 180
			operational = true
		} else {
			requestedActions := contextEndpointRequestedActions(query)
			factActions := contextActionFamilies(
				strings.Join([]string{fact.Name, fact.Qualified, fact.HTTPMethod, fact.Path}, " "),
				fact.HTTPMethod,
			)
			if contextActionFamiliesOverlap(requestedActions, factActions) {
				score += 120
				operational = true
			}
		}
	case "api_contract":
		score += 120
		operational = true
		if contextQueryRequestsConcern(query, contextConcernHTTPContract) {
			score += 100
		}
	case "persistence":
		score += 140
		operational = true
		if contextQueryRequestsConcern(query, contextConcernPersistence) {
			score += 100
		}
	case "authentication", "requires_auth", "security":
		score += 140
		operational = true
		if contextQueryRequestsConcern(query, contextConcernAuth) {
			score += 100
		}
	case "configuration", "config":
		score += 140
		operational = true
		if contextQueryRequestsConcern(query, contextConcernConfiguration) {
			score += 100
		}
	case "resilience", "retry":
		score += 140
		operational = true
		if contextQueryRequestsConcern(query, contextConcernResilience) {
			score += 100
		}
	case "side_effect", "side_effects":
		score += 140
		operational = true
		if contextQueryRequestsConcern(query, contextConcernSideEffects) {
			score += 100
		}
	case "test":
		if contextQueryRequestsConcern(query, contextConcernTests) {
			score += 100
			operational = true
		}
	}
	if contextQueryRequestsConcern(query, contextConcernDomainModel) &&
		contextDomainModelFact(fact, domainModelTokens) {
		score += 180
		operational = true
	}

	value := strings.Join([]string{fact.Search, fact.Name, fact.Qualified, fact.Summary}, " ")
	for _, concernKind := range []string{
		contextConcernAuth,
		contextConcernConfiguration,
		contextConcernResilience,
		contextConcernPersistence,
		contextConcernSideEffects,
		contextConcernTests,
	} {
		if contextQueryRequestsConcern(query, concernKind) &&
			contextValueRequestsConcern(value, concernKind) {
			score += 100
			operational = true
		}
	}
	graphUtility := utility.score(fact.ID)
	if graphUtility > 0 {
		score += graphUtility
		operational = true
	} else if kind == "api_endpoint" || kind == "route" {
		score -= scoreRouteKind
	}
	return operational, score
}

func contextSupportRole(
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
	domainModelTokens map[string]bool,
) string {
	if contextDomainModelFact(fact, domainModelTokens) {
		return contextConcernDomainModel
	}
	switch normalizedContextConcernKind(fact.Kind) {
	case contextConcernHTTPContract:
		return "contract"
	case contextConcernConfiguration:
		return contextConcernConfiguration
	case contextConcernResilience:
		return contextConcernResilience
	case contextConcernPersistence:
		return contextConcernPersistence
	case contextConcernAuth:
		return contextConcernAuth
	case contextConcernSideEffects:
		return contextConcernSideEffects
	case contextConcernTests:
		return contextConcernTests
	}
	identifier := strings.ToLower(strings.Join([]string{fact.Name, fact.Qualified}, " "))
	if strings.Contains(identifier, "client") {
		return "client"
	}
	if strings.EqualFold(fact.Kind, "route") || strings.EqualFold(fact.Kind, "api_endpoint") {
		return "provider"
	}
	for _, edge := range index.Edges {
		if edge.FromFactID != fact.ID {
			continue
		}
		switch normalizedContextConcernKind(edge.Kind) {
		case contextConcernHTTPContract, contextConcernPersistence, contextConcernAuth:
			return "service"
		}
		switch strings.ToLower(strings.TrimSpace(edge.Kind)) {
		case "call", "consumes_endpoint", "extends", "implements", "use":
			return "service"
		}
	}
	value := strings.Join([]string{fact.Search, fact.Name, fact.Qualified, fact.Summary}, " ")
	for _, kind := range []string{
		contextConcernConfiguration,
		contextConcernAuth,
		contextConcernResilience,
		contextConcernPersistence,
		contextConcernSideEffects,
	} {
		if contextValueRequestsConcern(value, kind) {
			return kind
		}
	}
	return ""
}

func contextGenericPersistenceFact(fact scan.AgentContextFactRecord) bool {
	if normalizedContextConcernKind(fact.Kind) != contextConcernPersistence {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(fact.Name)) {
	case "count", "deleteall", "existsbyid", "findall", "findbyid", "save", "saveall":
		return true
	default:
		return strings.Contains(strings.ToLower(fact.Summary), "inherited")
	}
}

var contextSupportRoleOrder = []string{
	"client",
	"contract",
	"provider",
	"service",
	contextConcernDomainModel,
	contextConcernConfiguration,
	contextConcernAuth,
	contextConcernResilience,
	contextConcernPersistence,
	contextConcernSideEffects,
	contextConcernTests,
}

func selectContextSupportFacts(
	ranked []rankedContextSupportFact,
	representedProjects map[string]bool,
) []rankedContextSupportFact {
	eligible := make([]rankedContextSupportFact, 0, len(ranked))
	for _, candidate := range ranked {
		matchesTask := candidate.semanticMatches >= 2
		if candidate.explicit {
			matchesTask = candidate.semanticMatches >= 1 || candidate.requestedMatches > 0
		}
		if !matchesTask || representedProjects[candidate.project] {
			continue
		}
		if !candidate.operational && candidate.semanticMatches < 2 && candidate.requestedMatches == 0 {
			continue
		}
		if (strings.EqualFold(candidate.fact.Kind, "api_endpoint") ||
			strings.EqualFold(candidate.fact.Kind, "route")) && !candidate.operational {
			continue
		}
		eligible = append(eligible, candidate)
	}

	selected := make([]rankedContextSupportFact, 0, len(eligible))
	selectedIDs := map[string]bool{}
	projectCounts := map[string]int{}
	trySelect := func(candidate rankedContextSupportFact) {
		if selectedIDs[candidate.fact.ID] ||
			projectCounts[candidate.project] >= maximumContextSupportCandidatesPerProject {
			return
		}
		selected = append(selected, candidate)
		selectedIDs[candidate.fact.ID] = true
		projectCounts[candidate.project]++
	}
	for _, role := range contextSupportRoleOrder {
		for _, candidate := range eligible {
			if candidate.role == role {
				trySelect(candidate)
			}
		}
	}
	projectsWithStructuredEvidence := map[string]bool{}
	for _, candidate := range selected {
		if candidate.role != "" {
			projectsWithStructuredEvidence[candidate.project] = true
		}
	}
	for _, candidate := range eligible {
		if candidate.role == "" && !projectsWithStructuredEvidence[candidate.project] {
			trySelect(candidate)
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

func selectContextEndpoint(
	index scan.AgentContextIndexRecord,
	ranked []rankedContextFact,
	query string,
) (rankedContextFact, bool, string) {
	candidates := make([]rankedContextFact, 0)
	requestedActions := contextEndpointRequestedActions(query)
	actionMismatch := false
	for _, candidate := range ranked {
		if candidate.score < minimumContextSeedScore ||
			!eligibleContextEndpoint(candidate.fact) ||
			!contextEndpointRouteMatchesQuery(candidate.fact, query) {
			continue
		}
		if !contextEndpointActionAligned(candidate, requestedActions) {
			actionMismatch = true
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		if actionMismatch && contextQueryRequestsEndpoint(query, ranked) {
			return rankedContextFact{}, false, contextEndpointActionMismatchReason
		}
		return rankedContextFact{}, false, ""
	}
	utility := newContextForwardUtility(index)
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		leftAnchor := contextEndpointExplicitAnchor(left)
		rightAnchor := contextEndpointExplicitAnchor(right)
		if leftAnchor != rightAnchor {
			return leftAnchor
		}
		if primaryAction, ok := contextEndpointPrimaryActionClause(query); ok {
			leftPrimary := contextEndpointPrimaryActionScore(left.fact, primaryAction)
			rightPrimary := contextEndpointPrimaryActionScore(right.fact, primaryAction)
			if leftPrimary != rightPrimary {
				return leftPrimary > rightPrimary
			}
		}
		leftUtility := left.score + contextEndpointPathUtility(index, utility, left.fact)
		rightUtility := right.score + contextEndpointPathUtility(index, utility, right.fact)
		if leftUtility != rightUtility {
			return leftUtility > rightUtility
		}
		return false
	})

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
		return top, true, ""
	}
	if !contextEndpointMethodAndPathMatchQuery(top.fact, query) {
		return rankedContextFact{}, false, contextEndpointProviderAmbiguityReason
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
		return rankedContextFact{}, false, contextEndpointProviderAmbiguityReason
	}
	return providers[bestProvider][0], true, ""
}

func contextEndpointPrimaryActionClause(query string) (string, bool) {
	primary := contextPrimaryQuery(query)
	index := strings.IndexAny(primary, ",;")
	if index < 0 {
		return "", false
	}
	clause := strings.TrimSpace(primary[:index])
	if clause == "" ||
		!contextActionFamiliesHaveMutation(contextActionFamilies(clause, "")) {
		return "", false
	}
	return clause, true
}

func contextEndpointPrimaryActionScore(
	fact scan.AgentContextFactRecord,
	primaryAction string,
) int {
	queryTokens := contextQueryTokens(primaryAction)
	routeTokens := contextTokenSet(strings.TrimSpace(fact.HTTPMethod + " " + fact.Path))
	matched := 0
	for _, token := range queryTokens {
		if routeTokens[token] {
			matched++
		}
	}
	extras := len(routeTokens) - matched
	return matched*100 - extras*10
}

func contextEndpointRequestedActions(query string) map[string]bool {
	primaryQuery := contextPrimaryQuery(query)
	actions := contextActionFamilies(primaryQuery, contextRequestedHTTPMethod(query))
	for _, anchor := range contextQueryAnchors(query) {
		for action := range contextActionFamilies(anchor, "") {
			actions[action] = true
		}
	}
	if actions["read"] &&
		(actions["create"] || actions["delete"] || actions["update"]) {
		delete(actions, "read")
	}
	return actions
}

func contextQueryRequestsEndpoint(query string, ranked []rankedContextFact) bool {
	for _, candidate := range ranked {
		if candidate.exactClass == 0 ||
			strings.EqualFold(candidate.fact.Kind, "api_endpoint") {
			continue
		}
		switch candidate.reason {
		case "exact qualified name", "exact name",
			"embedded exact qualified name", "embedded exact name":
			return false
		}
	}
	tokens := contextExpandedTokenSet(query)
	for _, token := range []string{"endpoint", "endpunkt", "http", "rest", "route"} {
		if tokens[token] {
			return true
		}
	}
	return false
}

func contextEndpointActionAligned(
	candidate rankedContextFact,
	requestedActions map[string]bool,
) bool {
	if len(requestedActions) == 0 || contextEndpointExplicitAnchor(candidate) {
		return true
	}
	candidateActions := contextActionFamilies(
		strings.Join([]string{
			candidate.fact.Name,
			candidate.fact.Qualified,
			candidate.fact.HTTPMethod,
			candidate.fact.Path,
		}, " "),
		candidate.fact.HTTPMethod,
	)
	return contextActionFamiliesOverlap(requestedActions, candidateActions)
}

func contextEndpointExplicitAnchor(candidate rankedContextFact) bool {
	return candidate.exactClass == 3 &&
		(candidate.reason == "exact route" || candidate.reason == "embedded exact route")
}

func contextEndpointPathUtility(
	index scan.AgentContextIndexRecord,
	utility *contextForwardUtility,
	endpoint scan.AgentContextFactRecord,
) int {
	if companion, ok := contextEndpointCompanion(index, endpoint); ok {
		return utility.score(companion.ID)
	}
	return utility.score(endpoint.ID)
}

func contextEndpointCompanion(
	index scan.AgentContextIndexRecord,
	endpoint scan.AgentContextFactRecord,
) (scan.AgentContextFactRecord, bool) {
	if !strings.EqualFold(strings.TrimSpace(endpoint.Kind), "api_endpoint") {
		return scan.AgentContextFactRecord{}, false
	}
	project := normalizeContextProject(endpoint.Project)
	routeKey := contextEndpointRouteKey(endpoint)
	qualified := normalizeContextTerm(endpoint.Qualified)
	candidates := []scan.AgentContextFactRecord{}
	exactMatches := []scan.AgentContextFactRecord{}
	for _, fact := range index.Facts {
		if !strings.EqualFold(strings.TrimSpace(fact.Kind), "route") ||
			normalizeContextProject(fact.Project) != project ||
			contextEndpointRouteKey(fact) != routeKey ||
			!reliableProductionContextSeed(fact) {
			continue
		}
		candidates = append(candidates, fact)
		if qualified != "" && normalizeContextTerm(fact.Qualified) == qualified {
			exactMatches = append(exactMatches, fact)
		}
	}
	if qualified != "" {
		if len(exactMatches) == 1 {
			return exactMatches[0], true
		}
		return scan.AgentContextFactRecord{}, false
	}
	if len(candidates) != 1 {
		return scan.AgentContextFactRecord{}, false
	}
	return candidates[0], true
}

type contextForwardUtility struct {
	adjacency map[string][]contextTraversalStep
	cache     map[string]int
}

func newContextForwardUtility(index scan.AgentContextIndexRecord) *contextForwardUtility {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	return &contextForwardUtility{
		adjacency: contextPathAdjacency(index.Edges, factByID, false),
		cache:     make(map[string]int),
	}
}

func (utility *contextForwardUtility) score(startID string) int {
	if score, ok := utility.cache[startID]; ok {
		return score
	}
	if len(utility.adjacency[startID]) == 0 {
		utility.cache[startID] = -120
		return -120
	}
	type utilityState struct {
		id    string
		depth int
	}
	queue := []utilityState{{id: startID}}
	visited := map[string]bool{startID: true}
	seenEdges := map[string]bool{}
	score := 0
	for len(queue) > 0 && len(visited) <= 64 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= 4 {
			continue
		}
		for _, step := range utility.adjacency[current.id] {
			edgeID := contextPathEdgeIdentity(step.edge)
			if !seenEdges[edgeID] {
				seenEdges[edgeID] = true
				weight := 0
				switch normalizedContextConcernKind(step.edge.Kind) {
				case contextConcernHTTPContract, contextConcernPersistence:
					weight = 180
				case contextConcernAuth:
					weight = 100
				default:
					switch strings.ToLower(strings.TrimSpace(step.edge.Kind)) {
					case "call", "implements", "extends":
						weight = 140
					case "use", "consumes_endpoint":
						weight = 60
					}
				}
				weight -= 20 * current.depth
				if weight > 0 {
					score += weight
				}
			}
			if !visited[step.nextID] {
				visited[step.nextID] = true
				queue = append(queue, utilityState{id: step.nextID, depth: current.depth + 1})
			}
		}
	}
	if score > 480 {
		score = 480
	}
	utility.cache[startID] = score
	return score
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
	routeTerm := normalizeContextTerm(strings.TrimSpace(fact.HTTPMethod + " " + fact.Path))
	if contextQueryHasAnchor(contextQueryAnchors(query), routeTerm) {
		return true
	}
	queryTokens := make(map[string]bool)
	for _, token := range contextQueryTokens(contextPrimaryQuery(query)) {
		queryTokens[token] = true
	}
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

func contextExpandedTokenSet(value string) map[string]bool {
	result := make(map[string]bool)
	for _, token := range contextQueryTokens(value) {
		result[token] = true
	}
	for _, token := range contextTokens(value) {
		for _, alias := range contextIntentTokenAliases[token] {
			result[alias] = true
		}
	}
	return result
}

func contextExpandedTokens(value string) []string {
	tokens := contextExpandedTokenSet(value)
	result := make([]string, 0, len(tokens))
	for token := range tokens {
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

func contextPrimaryQuery(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if problemStatement := contextProblemStatement(value); problemStatement != "" {
		value = problemStatement
	}
	value = contextFirstParagraph(value)
	segments := strings.FieldsFunc(value, func(current rune) bool {
		return current == '.' || current == '?' || current == '!'
	})
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if strings.HasSuffix(segment, ":") && len(strings.Fields(segment)) <= 8 {
			continue
		}
		return segment
	}
	return value
}

func contextProblemStatement(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for index, line := range lines {
		switch normalizeContextProblemHeading(line) {
		case "problem statement", "problemstellung", "problem", "task", "aufgabe":
			return contextFirstParagraph(strings.Join(lines[index+1:], "\n"))
		}
	}
	return ""
}

func normalizeContextProblemHeading(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSpace(strings.TrimLeft(value, "#"))
	value = strings.TrimSpace(strings.TrimSuffix(value, ":"))
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func contextFirstParagraph(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	parts := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if strings.HasSuffix(line, ":") && len(strings.Fields(line)) <= 8 && len(parts) == 0 {
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, " ")
}

var contextIntentTokenAliases = map[string][]string{
	"aufgabe":             {"job", "jobs", "task", "tasks"},
	"aufgaben":            {"job", "jobs", "task", "tasks"},
	"aufgabenart":         {"task_type", "task_types", "type", "types"},
	"aufgabenarten":       {"task_type", "task_types", "type", "types"},
	"attribute":           {"attributes", "field", "fields", "identifier", "identifiers"},
	"attributes":          {"attribute", "field", "fields", "identifier", "identifiers"},
	"authentifizierung":   {"auth", "authentication"},
	"benutzerinformation": {"side_effects", "user_information"},
	"contract":            {"contracts"},
	"contracts":           {"contract"},
	"effect":              {"side_effect", "side_effects"},
	"effects":             {"side_effect", "side_effects"},
	"fehlerbehandlung":    {"exception", "resilience"},
	"job":                 {"jobs", "task", "tasks"},
	"jobs":                {"job", "task", "tasks"},
	"konfiguration":       {"config", "configuration"},
	"nebenwirkung":        {"side_effect", "side_effects"},
	"nebenwirkungen":      {"side_effect", "side_effects"},
	"persistenz":          {"persistence", "repository"},
	"protokollierung":     {"logging", "side_effects"},
	"retries":             {"resilience", "retry"},
	"retry":               {"resilience", "retries"},
	"suchattribut":        {"attribute", "attributes", "field", "fields", "identifier", "identifiers"},
	"suchattribute":       {"attribute", "attributes", "field", "fields", "identifier", "identifiers"},
	"task":                {"job", "jobs", "tasks"},
	"tasks":               {"job", "jobs", "task"},
	"type":                {"task_type", "task_types", "types"},
	"types":               {"task_type", "task_types", "type"},
	"vertrag":             {"contract", "contracts"},
	"verträge":            {"contract", "contracts"},
	"wiederholung":        {"resilience", "retry"},
	"wiederholungen":      {"resilience", "retry"},
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
	"gelöschte":          {"delete", "remove"},
	"gelöschten":         {"delete", "remove"},
	"gelöscht":           {"delete", "remove"},
	"loeschen":           {"delete", "remove"},
	"löschung":           {"delete", "remove"},
	"löschungen":         {"delete", "remove"},
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
		if !strings.EqualFold(candidate.fact.Kind, "api_endpoint") && reliableProductionContextSeed(candidate.fact) {
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
	case "route", "api_endpoint", "symbol", "backend_handler":
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

const maximumContextQueryAnchors = 8
const maximumContextQueryAnchorRunes = 256

type contextQueryAnchorToken struct {
	start int
	value string
}

func contextQueryAnchors(query string) []string {
	occurrences := make([]contextQueryAnchorToken, 0)
	tokens := contextQueryAnchorTokens(query)
	for index := 0; index+1 < len(tokens); index++ {
		if contextHTTPVerbs[strings.ToUpper(tokens[index].value)] && strings.HasPrefix(tokens[index+1].value, "/") {
			occurrences = append(occurrences, contextQueryAnchorToken{
				start: tokens[index].start,
				value: tokens[index].value + " " + tokens[index+1].value,
			})
		}
	}
	for _, token := range tokens {
		if sourceFile, ok := contextQuerySourceFileAnchor(token.value); ok {
			occurrences = append(occurrences, contextQueryAnchorToken{start: token.start, value: sourceFile})
			continue
		}
		if !strings.Contains(token.value, "/") &&
			(strings.Contains(token.value, ".") || strings.Contains(token.value, "#") || strings.Contains(token.value, "::")) {
			occurrences = append(occurrences, token)
		}
	}

	runes := []rune(query)
	for start := 0; start < len(runes); start++ {
		if runes[start] != '`' {
			continue
		}
		end := start + 1
		for end < len(runes) && runes[end] != '`' {
			end++
		}
		if end >= len(runes) {
			break
		}
		occurrences = append(occurrences, contextQueryAnchorToken{
			start: start,
			value: strings.TrimSpace(string(runes[start+1 : end])),
		})
		start = end
	}

	sort.SliceStable(occurrences, func(i, j int) bool {
		return occurrences[i].start < occurrences[j].start
	})
	anchors := make([]string, 0, maximumContextQueryAnchors)
	seen := map[string]bool{}
	for _, occurrence := range occurrences {
		value := strings.TrimSpace(occurrence.value)
		if value == "" || len([]rune(value)) > maximumContextQueryAnchorRunes {
			continue
		}
		normalized := normalizeContextTerm(value)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		anchors = append(anchors, value)
		if len(anchors) == maximumContextQueryAnchors {
			break
		}
	}
	return anchors
}

func contextQueryAnchorTokens(query string) []contextQueryAnchorToken {
	runes := []rune(query)
	tokens := []contextQueryAnchorToken{}
	for start := 0; start < len(runes); {
		for start < len(runes) && unicode.IsSpace(runes[start]) {
			start++
		}
		end := start
		for end < len(runes) && !unicode.IsSpace(runes[end]) {
			end++
		}
		if start == end {
			continue
		}
		value := strings.Trim(string(runes[start:end]), "`\\\"'()[]<>,;:!?")
		if value != "" {
			tokens = append(tokens, contextQueryAnchorToken{start: start, value: value})
		}
		start = end
	}
	return tokens
}

func contextQuerySourceFileAnchor(value string) (string, bool) {
	if scan.IsSupportedSourceFile(value) {
		return value, true
	}
	value = strings.TrimRight(value, ".!?")
	if value == "" || !scan.IsSupportedSourceFile(value) {
		return "", false
	}
	return value, true
}

func contextQueryHasAnchor(anchors []string, term string) bool {
	if term == "" {
		return false
	}
	for _, anchor := range anchors {
		if normalizeContextTerm(anchor) == term {
			return true
		}
	}
	return false
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
	view, err := contextBudgetView(pack)
	if err != nil {
		return false, err
	}
	if view.EstimatedTokens > budget {
		return false, nil
	}
	body, err := json.Marshal(view)
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
	pack.SourceSections = append([]ContextSourceSection(nil), pack.SourceSections...)
	pack.SourceOmissions = append([]ContextSourceOmission(nil), pack.SourceOmissions...)
	pack.RetryAnchors = append([]string(nil), pack.RetryAnchors...)
	pack.selectedSourceFactIDs = append([]string(nil), pack.selectedSourceFactIDs...)
	pack.selectedFactIDs = append([]string(nil), pack.selectedFactIDs...)
	pack.selectedEdgeIDs = append([]string(nil), pack.selectedEdgeIDs...)
	pack.selectedConcernKeys = append([]string(nil), pack.selectedConcernKeys...)
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
