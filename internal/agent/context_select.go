package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

type contextSourceOption struct {
	candidate        sourceCandidate
	section          ContextSourceSection
	estimated        int
	concernKeys      []string
	projectKey       string
	required         bool
	pathDistance     int
	candidateQuality int
	quality          int
	evidenceFamily   string
	stableMatches    int
	matchesModel     bool
	modelMatchSet    bool
	requestedModel   bool
	profiled         bool
}

type contextSourceSelectionState struct {
	selectedCandidates       map[string]bool
	selectedFactIDs          map[string]bool
	selectedProjects         map[string]bool
	coveredConcerns          map[string]bool
	coveredRoles             map[string]bool
	selectedEvidenceFamilies map[string]int
}

const contextPublicSourceConcernRank = 1_000_000

func selectContextSourceOptions(
	pack ContextPack,
	loaded loadedContextIndex,
	request ContextRequest,
) (ContextPack, error) {
	concerns := contextSourceConcerns(pack, loaded.Index)
	requestedModelIDs := contextRequestedDomainModelIDsFromConcerns(
		pack,
		loaded.Index,
		concerns,
	)
	candidates := contextSourceCandidatesForConcernsWithModels(
		pack,
		loaded.Index,
		concerns,
		requestedModelIDs,
	)
	distances := contextSourcePathDistances(pack, loaded.Index)
	options, failures, err := contextSourceRenderOptionsWithModels(
		pack,
		loaded,
		candidates,
		concerns,
		distances,
		requestedModelIDs,
	)
	if err != nil {
		return ContextPack{}, err
	}

	pack = cloneContextPack(pack)
	pack.SourceSections = nil
	pack.SourceOmissions = nil
	pack.SourceUnrepresented = len(candidates)
	state := contextSourceSelectionState{
		selectedCandidates:       make(map[string]bool, len(candidates)),
		selectedFactIDs:          make(map[string]bool, len(candidates)),
		selectedProjects:         make(map[string]bool),
		coveredConcerns:          make(map[string]bool, len(concerns)),
		coveredRoles:             make(map[string]bool),
		selectedEvidenceFamilies: make(map[string]int),
	}
	applyContextSourceCoverage(&pack, concerns, state.coveredConcerns)
	pack, err = finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}
	sectionRequest, err := contextSourceRequestWithOmissionReserve(
		pack,
		request,
		contextSourceEvidenceOmissionsWithOptions(
			contextSourceConcernsWithoutRenderedOptions(concerns, options),
			candidates,
			options,
			failures,
			state.coveredConcerns,
		),
	)
	if err != nil {
		return ContextPack{}, err
	}

	coreBoundaries := contextCoreSourceBoundaries(pack, loaded.Index, distances)
	for _, boundary := range mandatoryContextSourceBoundaries(loaded.Index, concerns, coreBoundaries, distances) {
		if contextSourceBoundaryCovered(boundary, state) {
			continue
		}
		option, ok, selectErr := smallestFittingContextSourceOption(
			pack,
			sectionRequest,
			options,
			concerns,
			state,
			boundary,
		)
		if selectErr != nil {
			return ContextPack{}, selectErr
		}
		if !ok {
			continue
		}
		pack, state, err = addContextSourceOption(pack, sectionRequest, option, concerns, state)
		if err != nil {
			return ContextPack{}, err
		}
	}
	pack, err = enrichContextCoreSourceOptions(
		pack,
		sectionRequest,
		options,
		state,
		coreBoundaries,
	)
	if err != nil {
		return ContextPack{}, err
	}

	for len(pack.SourceSections) < MaxContextSourceSections {
		productionPending := coverableContextSourceProductionPending(concerns, options, state)
		best, bestUtility, found, utilityErr := contextSourceUtilityOption(
			pack, sectionRequest, options, concerns, state, productionPending,
		)
		if utilityErr != nil {
			return ContextPack{}, utilityErr
		}
		if !found || bestUtility <= 0 {
			break
		}
		pack, state, err = addContextSourceOption(
			pack,
			sectionRequest,
			best,
			concerns,
			state,
		)
		if err != nil {
			return ContextPack{}, err
		}
	}
	applyContextSourceCoverage(&pack, concerns, state.coveredConcerns)
	for _, omission := range contextSourceEvidenceOmissionsWithOptions(
		concerns,
		candidates,
		options,
		failures,
		state.coveredConcerns,
	) {
		candidate := cloneContextPack(pack)
		candidate.SourceOmissions = append(candidate.SourceOmissions, omission)
		candidate, err = finalizeContextEstimate(candidate)
		if err != nil {
			return ContextPack{}, err
		}
		fits, fitErr := contextSourcePackFits(candidate, request)
		if fitErr != nil {
			return ContextPack{}, fitErr
		}
		if fits {
			pack = candidate
		}
	}
	if pack.SourceCoverage == "complete" {
		pack.SourceUnrepresented = 0
	}
	pack.SourceSections = contextSourceSectionsProductionFirst(pack.SourceSections)
	return finalizeContextPackWithinBudget(pack, request)
}

func contextSourceConcernsWithoutRenderedOptions(
	concerns []contextConcern,
	options []contextSourceOption,
) []contextConcern {
	result := make([]contextConcern, 0)
	for _, concern := range concerns {
		if !concern.required {
			continue
		}
		hasOption := false
		for _, option := range options {
			if contextSourceOptionHasConcern(option, concern.key) {
				hasOption = true
				break
			}
		}
		if !hasOption {
			result = append(result, concern)
		}
	}
	return result
}

func contextSourceRequestWithOmissionReserve(
	pack ContextPack,
	request ContextRequest,
	omissions []ContextSourceOmission,
) (ContextRequest, error) {
	if len(omissions) == 0 {
		return request, nil
	}
	before, err := contextBudgetView(pack)
	if err != nil {
		return ContextRequest{}, err
	}
	probe := cloneContextPack(pack)
	probe.SourceOmissions = append(
		probe.SourceOmissions,
		omissions[:min(len(omissions), MaxContextSourceOmissions)]...,
	)
	probe, err = finalizeContextEstimate(probe)
	if err != nil {
		return ContextRequest{}, err
	}
	after, err := contextBudgetView(probe)
	if err != nil {
		return ContextRequest{}, err
	}
	reserve := after.EstimatedTokens - before.EstimatedTokens
	beforeBody, err := json.Marshal(before)
	if err != nil {
		return ContextRequest{}, err
	}
	afterBody, err := json.Marshal(after)
	if err != nil {
		return ContextRequest{}, err
	}
	if byteReserve := (len(afterBody) - len(beforeBody) + 3) / 4; byteReserve > reserve {
		reserve = byteReserve
	}
	if reserve <= 0 || request.BudgetTokens-reserve < MinContextBudgetTokens {
		return request, nil
	}
	request.BudgetTokens -= reserve
	return request, nil
}

func contextSourceSectionsProductionFirst(sections []ContextSourceSection) []ContextSourceSection {
	result := make([]ContextSourceSection, 0, len(sections))
	for _, section := range sections {
		if section.Role != "test" {
			result = append(result, section)
		}
	}
	for _, section := range sections {
		if section.Role == "test" {
			result = append(result, section)
		}
	}
	return result
}

func contextSourceConcerns(pack ContextPack, index scan.AgentContextIndexRecord) []contextConcern {
	planned := []contextConcern(nil)
	if seed, ok := contextConcernPlanningSeed(index, contextSelectionQuery(pack)); ok {
		planned = planContextConcerns(contextSelectionQuery(pack), index, seed)
	}
	plannedByKey := make(map[string]int, len(planned))
	for index, concern := range planned {
		plannedByKey[concern.key] = index
	}
	requestedModels := contextRequestedDomainModelIDsFromConcerns(
		pack,
		index,
		planned,
	)
	endpointProjects, contractProjects, modelProjects :=
		contextEvidenceProjectRolesForModels(pack, index, requestedModels)

	if len(pack.Concerns) == 0 {
		sort.Slice(planned, func(i, j int) bool { return planned[i].key < planned[j].key })
		return expandContextEvidenceConcernsWithProfile(
			pack,
			index,
			planned,
			requestedModels,
			endpointProjects,
			contractProjects,
			modelProjects,
		)
	}
	concerns := make([]contextConcern, 0, len(pack.Concerns))
	added := make(map[string]bool, len(pack.Concerns))
	for _, public := range pack.Concerns {
		key := contextPublicConcernKey(public)
		if added[key] {
			continue
		}
		selected := contextSourceConcernFromPack(pack, index, public)
		if plannedIndex, ok := plannedByKey[key]; ok {
			concern := planned[plannedIndex]
			concern.rank = max(concern.rank, contextPublicSourceConcernRank)
			concern.candidateFactIDs = orderedContextConcernIDs(append(
				concern.candidateFactIDs,
				selected.candidateFactIDs...,
			))
			concerns = append(concerns, concern)
		} else {
			selected.rank = max(selected.rank, contextPublicSourceConcernRank)
			concerns = append(concerns, selected)
		}
		added[key] = true
	}
	for _, concern := range planned {
		if added[concern.key] ||
			concern.kind == contextConcernEntrypoint ||
			concern.kind == contextConcernPrimaryPath ||
			concern.kind == contextConcernProject {
			continue
		}
		concern.required = contextRequiredEvidenceConcernForRoles(
			concern,
			endpointProjects,
			contractProjects,
			modelProjects,
		)
		concerns = append(concerns, concern)
		added[concern.key] = true
	}
	sort.Slice(concerns, func(i, j int) bool { return concerns[i].key < concerns[j].key })
	return expandContextEvidenceConcernsWithProfile(
		pack,
		index,
		concerns,
		requestedModels,
		endpointProjects,
		contractProjects,
		modelProjects,
	)
}

func contextEvidenceProjectRoles(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) (map[string]bool, map[string]bool, map[string]bool) {
	return contextEvidenceProjectRolesForModels(
		pack,
		index,
		contextRequestedDomainModelIDs(pack, index),
	)
}

func contextEvidenceProjectRolesForModels(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	requestedModels map[string]bool,
) (map[string]bool, map[string]bool, map[string]bool) {
	endpointProjects := map[string]bool{}
	for _, endpoint := range pack.Endpoints {
		endpointProjects[normalizeContextProject(endpoint.Provider)] = true
	}
	contractProjects := map[string]bool{}
	for _, contract := range pack.Contracts {
		contractProjects[normalizeContextProject(contract.Project)] = true
	}
	modelProjects := map[string]bool{}
	for _, fact := range index.Facts {
		if requestedModels[fact.ID] {
			project := normalizeContextProject(fact.Project)
			if contractProjects[project] && !endpointProjects[project] {
				continue
			}
			modelProjects[project] = true
		}
	}
	return endpointProjects, contractProjects, modelProjects
}

func contextRequiredEvidenceConcernForRoles(
	concern contextConcern,
	endpointProjects map[string]bool,
	contractProjects map[string]bool,
	modelProjects map[string]bool,
) bool {
	switch concern.kind {
	case contextConcernAuth:
		return endpointProjects[concern.project] ||
			contractProjects[concern.project] ||
			modelProjects[concern.project]
	case contextConcernConfiguration, contextConcernResilience, contextConcernHTTPContract:
		return contractProjects[concern.project]
	case contextConcernPersistence, contextConcernSideEffects, contextConcernTests:
		return modelProjects[concern.project]
	default:
		return concern.required
	}
}

func expandContextEvidenceConcerns(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
) []contextConcern {
	requestedModels := contextRequestedDomainModelIDs(pack, index)
	endpointProjects, contractProjects, modelProjects :=
		contextEvidenceProjectRolesForModels(pack, index, requestedModels)
	return expandContextEvidenceConcernsWithProfile(
		pack,
		index,
		concerns,
		requestedModels,
		endpointProjects,
		contractProjects,
		modelProjects,
	)
}

func expandContextEvidenceConcernsWithProfile(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
	requestedModels map[string]bool,
	endpointProjects map[string]bool,
	contractProjects map[string]bool,
	modelProjects map[string]bool,
) []contextConcern {
	queryTokens := contextExpandedTokenSet(contextSelectionQuery(pack))
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	contractFactIDs := map[string][]string{}
	for _, contract := range pack.Contracts {
		project := normalizeContextProject(contract.Project)
		contractFactIDs[project] = append(contractFactIDs[project], contract.ID)
	}

	result := make([]contextConcern, 0, len(concerns)+len(requestedModels))
	for _, concern := range concerns {
		switch concern.kind {
		case contextConcernAuth:
			added := false
			if contractProjects[concern.project] {
				candidates := orderedContextConcernIDs(append(
					append([]string(nil), concern.candidateFactIDs...),
					contractFactIDs[concern.project]...,
				))
				result = append(result, newContextEvidenceConcern(
					concern,
					"client_transport",
					candidates,
					"client transport authentication",
				))
				added = true
			}
			if endpointProjects[concern.project] || modelProjects[concern.project] {
				result = append(result, newContextEvidenceConcern(
					concern,
					"server_policy",
					concern.candidateFactIDs,
					"server authentication policy",
				))
				added = true
			}
			if !added {
				result = append(result, concern)
			}
		case contextConcernConfiguration:
			if !contractProjects[concern.project] {
				result = append(result, concern)
				continue
			}
			bindingCandidates := contextEvidenceFacetCandidateIDs(
				index,
				concern.candidateFactIDs,
				concern.project,
				contextConcernConfiguration,
				"binding",
				queryTokens,
			)
			consumerCandidates := contextEvidenceFacetCandidateIDs(
				index,
				orderedContextConcernIDs(append(
					append([]string(nil), concern.candidateFactIDs...),
					contractFactIDs[concern.project]...,
				)),
				concern.project,
				contextConcernConfiguration,
				"consumer",
				queryTokens,
			)
			result = append(
				result,
				newContextEvidenceConcern(
					concern,
					"binding",
					bindingCandidates,
					"client configuration binding",
				),
				newContextEvidenceConcern(
					concern,
					"consumer",
					consumerCandidates,
					"client configuration consumption",
				),
			)
		case contextConcernResilience:
			if !contractProjects[concern.project] {
				result = append(result, concern)
				continue
			}
			candidates := orderedContextConcernIDs(append(
				append([]string(nil), concern.candidateFactIDs...),
				contractFactIDs[concern.project]...,
			))
			added := false
			for _, facet := range []string{"retry_policy", "recovery"} {
				if !contextEvidenceFacetRequested(
					contextConcernResilience,
					facet,
					queryTokens,
				) {
					continue
				}
				reason := "client retry policy"
				if facet == "recovery" {
					reason = "client recovery behavior"
				}
				result = append(result, newContextEvidenceConcern(
					concern,
					facet,
					contextEvidenceFacetCandidateIDs(
						index,
						candidates,
						concern.project,
						contextConcernResilience,
						facet,
						queryTokens,
					),
					reason,
				))
				added = true
			}
			if !added {
				result = append(result, concern)
			}
		case contextConcernPersistence:
			modelIDs := make([]string, 0, len(requestedModels))
			for modelID := range requestedModels {
				model := factByID[modelID]
				if concern.project == "" ||
					normalizeContextProject(model.Project) == concern.project {
					modelIDs = append(modelIDs, modelID)
				}
			}
			sort.Strings(modelIDs)
			if len(modelIDs) == 0 {
				result = append(result, concern)
				continue
			}
			domainTokens := contextSourceDomainModelTokens(pack, index)
			for _, modelID := range modelIDs {
				candidates := []string{}
				for _, factID := range concern.candidateFactIDs {
					fact, ok := factByID[factID]
					if ok && contextPersistenceMatchesDomainModel(
						index,
						fact,
						domainTokens,
						map[string]bool{modelID: true},
					) {
						candidates = append(candidates, factID)
					}
				}
				result = append(result, newContextEvidenceConcern(
					concern,
					"model:"+modelID,
					candidates,
					"persistence for requested model "+factByID[modelID].Name,
				))
			}
		case contextConcernSideEffects:
			if concern.project != "" && !modelProjects[concern.project] {
				result = append(result, concern)
				continue
			}
			added := false
			facets := contextEvidenceFacetVocabulary[contextConcernSideEffects]
			names := make([]string, 0, len(facets))
			for name := range facets {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				if !contextEvidenceFacetRequested(
					contextConcernSideEffects,
					name,
					queryTokens,
				) {
					continue
				}
				result = append(result, newContextEvidenceConcern(
					concern,
					name,
					contextEvidenceFacetCandidateIDs(
						index,
						concern.candidateFactIDs,
						concern.project,
						contextConcernSideEffects,
						name,
						queryTokens,
					),
					"requested "+name+" side effects",
				))
				added = true
			}
			if !added {
				result = append(result, concern)
			}
		default:
			result = append(result, concern)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].key < result[j].key })
	return result
}

func contextEvidenceFacetCandidateIDs(
	index scan.AgentContextIndexRecord,
	candidateFactIDs []string,
	project string,
	kind string,
	facet string,
	queryTokens map[string]bool,
) []string {
	project = normalizeContextProject(project)
	result := make([]string, 0, len(candidateFactIDs))
	candidateSet := make(map[string]bool, len(candidateFactIDs))
	for _, factID := range candidateFactIDs {
		candidateSet[factID] = true
	}
	for _, fact := range index.Facts {
		if project != "" && normalizeContextProject(fact.Project) != project {
			continue
		}
		facetMatch := contextEvidenceFacetFactScore(fact, kind, facet) > 0
		actionMatch := contextConcernActionAligned(queryTokens, fact)
		if kind == contextConcernSideEffects {
			if candidateSet[fact.ID] {
				if !facetMatch &&
					(!actionMatch || contextEvidenceFactHasAnyFacet(fact, kind)) {
					continue
				}
			} else if !facetMatch || !actionMatch {
				continue
			}
			factKind := strings.ToLower(strings.TrimSpace(fact.Kind))
			if factKind != "symbol" &&
				normalizedContextConcernKind(factKind) != contextConcernSideEffects &&
				normalizedContextConcernKind(factKind) != contextConcernTests {
				continue
			}
		} else if !facetMatch || !eligibleContextConcernFact(fact) {
			continue
		}
		result = append(result, fact.ID)
	}
	if len(result) == 0 {
		return orderedContextConcernIDs(candidateFactIDs)
	}
	return orderedContextConcernIDs(result)
}

func contextEvidenceFactHasAnyFacet(
	fact scan.AgentContextFactRecord,
	kind string,
) bool {
	for facet := range contextEvidenceFacetVocabulary[kind] {
		if contextEvidenceFacetFactScore(fact, kind, facet) > 0 {
			return true
		}
	}
	return false
}

func contextEvidenceFacetFactScore(
	fact scan.AgentContextFactRecord,
	kind string,
	facet string,
) int {
	value := strings.ToLower(strings.Join([]string{
		fact.Kind,
		fact.Name,
		fact.Qualified,
		fact.File,
		fact.Path,
		fact.Search,
		fact.Summary,
	}, " "))
	score := 0
	for _, token := range contextEvidenceFacetVocabulary[kind][facet] {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" && strings.Contains(value, token) {
			score++
		}
	}
	switch kind + "#" + facet {
	case contextConcernSideEffects + "#mail":
		if contextSourceContainsAny(value, "sendmail", "sendemail") {
			score++
		}
	case contextConcernSideEffects + "#audit":
		if contextSourceContainsAny(value, "audit", "protocolservice", "trackingservice") {
			score++
		}
	case contextConcernSideEffects + "#user_information":
		if contextSourceContainsAny(
			value,
			"getuser",
			"userservice",
			"usermgmt",
			"userinformation",
		) {
			score++
		}
	}
	return score
}

func contextPublicConcernKey(concern ContextConcern) string {
	key := strings.ToLower(strings.TrimSpace(concern.Kind))
	if project := normalizeContextProject(concern.Project); project != "" {
		key += ":" + project
	}
	return key
}

func contextSourceConcernFromPack(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	public ContextConcern,
) contextConcern {
	kind := strings.ToLower(strings.TrimSpace(public.Kind))
	project := normalizeContextProject(public.Project)
	selected := make(map[string]bool, len(pack.selectedSourceFactIDs))
	for _, factID := range pack.selectedSourceFactIDs {
		selected[factID] = true
	}
	locationIDs := map[string]bool{}
	switch kind {
	case contextConcernEntrypoint:
		locationIDs = contextLocationIDs(pack.Entrypoints)
	case contextConcernHTTPContract:
		locationIDs = contextLocationIDs(pack.Contracts)
	case contextConcernPersistence:
		locationIDs = contextLocationIDs(pack.Persistence)
	case contextConcernTests:
		locationIDs = contextLocationIDs(pack.Tests)
	}
	candidateIDs := []string{}
	for _, fact := range index.Facts {
		if !selected[fact.ID] {
			continue
		}
		isTest := normalizedContextConcernKind(fact.Kind) == contextConcernTests || contextFactUsesTestSource(fact)
		include := locationIDs[fact.ID]
		switch kind {
		case contextConcernPrimaryPath:
			include = !isTest
		case contextConcernProject:
			include = !isTest && normalizeContextProject(fact.Project) == project
		case contextConcernAuth:
			include = normalizedContextConcernKind(fact.Kind) == contextConcernAuth ||
				contextValueRequestsConcern(strings.Join([]string{fact.Search, fact.Name, fact.Qualified, fact.Summary}, " "), contextConcernAuth)
		case contextConcernPersistence:
			include = include || normalizedContextConcernKind(fact.Kind) == contextConcernPersistence
		case contextConcernTests:
			include = include || isTest
		}
		if include {
			candidateIDs = append(candidateIDs, fact.ID)
		}
	}
	if kind == contextConcernHTTPContract {
		for _, edge := range index.Edges {
			if normalizedContextConcernKind(edge.Kind) == contextConcernHTTPContract {
				if selected[edge.FromFactID] {
					candidateIDs = append(candidateIDs, edge.FromFactID)
				}
				if selected[edge.ToFactID] {
					candidateIDs = append(candidateIDs, edge.ToFactID)
				}
			}
		}
	}
	return newContextConcern(kind, project, true, candidateIDs, public.Reason)
}

func contextSourcePathDistances(pack ContextPack, index scan.AgentContextIndexRecord) map[string]int {
	seedID := ""
	if len(pack.Entrypoints) > 0 {
		seedID = pack.Entrypoints[0].ID
	} else if seed, ok := contextConcernPlanningSeed(index, contextSelectionQuery(pack)); ok {
		seedID = seed.ID
	}
	if seedID == "" {
		return nil
	}
	selected := make(map[string]bool, len(pack.selectedSourceFactIDs))
	for _, factID := range pack.selectedSourceFactIDs {
		selected[factID] = true
	}
	adjacency := make(map[string][]string)
	for _, edge := range index.Edges {
		if !selected[edge.FromFactID] || !selected[edge.ToFactID] || normalizedContextConcernKind(edge.Kind) == contextConcernTests {
			continue
		}
		adjacency[edge.FromFactID] = append(adjacency[edge.FromFactID], edge.ToFactID)
	}
	for factID := range adjacency {
		sort.Strings(adjacency[factID])
	}
	distances := map[string]int{seedID: 0}
	queue := []string{seedID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if _, seen := distances[next]; seen {
				continue
			}
			distances[next] = distances[current] + 1
			queue = append(queue, next)
		}
	}
	return distances
}

func contextSourceRenderOptions(
	pack ContextPack,
	loaded loadedContextIndex,
	candidates []sourceCandidate,
	concerns []contextConcern,
	distances map[string]int,
) ([]contextSourceOption, map[string]string, error) {
	return contextSourceRenderOptionsWithModels(
		pack,
		loaded,
		candidates,
		concerns,
		distances,
		contextRequestedDomainModelIDs(pack, loaded.Index),
	)
}

func contextSourceRenderOptionsWithModels(
	pack ContextPack,
	loaded loadedContextIndex,
	candidates []sourceCandidate,
	concerns []contextConcern,
	distances map[string]int,
	requestedModelIDs map[string]bool,
) ([]contextSourceOption, map[string]string, error) {
	options := []contextSourceOption{}
	failures := make(map[string]string)
	factByID := make(map[string]scan.AgentContextFactRecord, len(loaded.Index.Facts))
	for _, fact := range loaded.Index.Facts {
		factByID[fact.ID] = fact
	}
	domainTokens := contextSourceDomainModelTokens(pack, loaded.Index)
	for _, candidate := range candidates {
		path, err := resolveSourcePath(loaded, candidate)
		if err != nil {
			contextSourceRecordFailure(failures, candidate, stableContextSourceOmissionReason(err))
			continue
		}
		file, err := readSourceFile(path)
		if err != nil {
			contextSourceRecordFailure(failures, candidate, stableContextSourceOmissionReason(err))
			continue
		}
		verifiedFacts := make(map[string]bool)
		candidateOptions, renderErr := appendContextSourceCandidateOptions(
			&options,
			failures,
			verifiedFacts,
			pack,
			loaded.Index,
			candidate,
			file,
			concerns,
			distances,
			requestedModelIDs,
			factByID,
			domainTokens,
		)
		if renderErr != nil {
			return nil, nil, renderErr
		}
		if candidateOptions == 0 && len(contextSourceCandidateFactIDs(candidate)) > 1 {
			for _, factID := range contextSourceCandidateFactIDs(candidate) {
				constituent, ok := contextSourceCandidateForFact(pack, loaded.Index, factID)
				if !ok {
					continue
				}
				_, renderErr = appendContextSourceCandidateOptions(
					&options,
					failures,
					verifiedFacts,
					pack,
					loaded.Index,
					constituent,
					file,
					concerns,
					distances,
					requestedModelIDs,
					factByID,
					domainTokens,
				)
				if renderErr != nil {
					return nil, nil, renderErr
				}
			}
		}
		if candidateOptions == 0 {
			if owner, ok := contextInheritedOwnerCandidate(loaded.Index, candidate); ok {
				_, renderErr = appendContextSourceCandidateOptions(
					&options,
					failures,
					verifiedFacts,
					pack,
					loaded.Index,
					owner,
					file,
					concerns,
					distances,
					requestedModelIDs,
					factByID,
					domainTokens,
				)
				if renderErr != nil {
					return nil, nil, renderErr
				}
			}
		}
		for factID := range verifiedFacts {
			delete(failures, factID)
		}
	}
	sort.Slice(options, func(i, j int) bool { return contextSourceOptionLess(options[i], options[j]) })
	return options, failures, nil
}

func appendContextSourceCandidateOptions(
	options *[]contextSourceOption,
	failures map[string]string,
	verifiedFacts map[string]bool,
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	candidate sourceCandidate,
	file sourceFile,
	concerns []contextConcern,
	distances map[string]int,
	requestedModelIDs map[string]bool,
	factByID map[string]scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) (int, error) {
	added := 0
	facts := contextSourceCandidateFacts(candidate, factByID)
	matchesModel := contextPersistenceFactsMatchRequestedDomainModel(
		index,
		facts,
		domainTokens,
		requestedModelIDs,
	)
	requestedModel := contextSourceCandidateHasRequestedModel(
		candidate,
		requestedModelIDs,
	)
	stableMatches := contextSourceStableDomainMatchesForFacts(facts, domainTokens)
	requestedActions := contextActionFamilies(contextSelectionQuery(pack), "")
	actionAligned := len(requestedActions) == 0 ||
		contextSourceFactsActionAligned(facts, candidate, requestedActions)
	for _, mode := range []string{"declaration_body", "body", "focused", "signature"} {
		section, renderErr := renderSourceCandidate(candidate, file, mode)
		if renderErr != nil {
			contextSourceRecordFailure(failures, candidate, stableContextSourceOmissionReason(renderErr))
			continue
		}
		verified, rejected := verifiedContextSourceFactIDs(pack, index, candidate, file, section)
		for factID, reason := range rejected {
			if _, recorded := failures[factID]; !recorded {
				failures[factID] = reason
			}
		}
		if len(verified) != len(contextSourceCandidateFactIDs(candidate)) {
			continue
		}
		for _, factID := range verified {
			verifiedFacts[factID] = true
		}
		optionCandidate := candidate
		optionCandidate.FactIDs = verified
		estimated, err := EstimateContextTokens(section)
		if err != nil {
			return 0, err
		}
		concernKeys, required := contextSourceOptionConcernsWithAction(
			optionCandidate,
			section,
			concerns,
			index,
			actionAligned,
		)
		projectKey := ""
		if optionCandidate.Role != "test" {
			projectKey = normalizeContextProject(optionCandidate.Project)
		}
		option := contextSourceOption{
			candidate: optionCandidate, section: section, estimated: estimated,
			concernKeys: concernKeys, projectKey: projectKey, required: required,
			pathDistance: contextSourceCandidateDistance(optionCandidate, distances),
		}
		option.evidenceFamily = contextSourceEvidenceFamilyForFacts(
			pack,
			option,
			facts,
			domainTokens,
		)
		option.matchesModel = matchesModel
		option.modelMatchSet = true
		option.requestedModel = requestedModel
		option.candidateQuality = contextSourceCandidateQualityForFacts(
			pack,
			index,
			option,
			facts,
			domainTokens,
		)
		option.quality = contextSourceOptionQualityForProfile(option)
		option.stableMatches = stableMatches
		option.profiled = true
		*options = append(*options, option)
		added++
	}
	return added, nil
}

func contextSourceCandidateForFact(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	factID string,
) (sourceCandidate, bool) {
	for _, fact := range index.Facts {
		if fact.ID != factID {
			continue
		}
		role := contextSourceRole(pack, index, fact)
		return sourceCandidate{
			FactID: fact.ID, FactIDs: []string{fact.ID}, Project: fact.Project, Path: fact.File,
			StartLine: fact.Line, EndLine: fact.EndLine, Role: role,
			Kind: fact.Kind, Name: fact.Name, Qualified: fact.Qualified,
			Priority: contextSourceRolePriority[role],
		}, true
	}
	return sourceCandidate{}, false
}

func contextSourceRecordFailure(failures map[string]string, candidate sourceCandidate, reason string) {
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if _, recorded := failures[factID]; !recorded {
			failures[factID] = reason
		}
	}
}

func verifiedContextSourceFactIDs(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	candidate sourceCandidate,
	file sourceFile,
	section ContextSourceSection,
) ([]string, map[string]string) {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	verified := []string{}
	rejected := make(map[string]string)
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		fact, ok := factByID[factID]
		if !ok {
			continue
		}
		if candidate.SourceState == "inherited_owner_current" {
			if contextInheritedFactMatchesOwner(fact, candidate) {
				verified = append(verified, factID)
			} else {
				rejected[factID] = "indexed inherited owner does not match current source"
			}
			continue
		}
		raw := sourceCandidate{
			FactID: fact.ID, FactIDs: []string{fact.ID}, Project: fact.Project, Path: fact.File,
			StartLine: fact.Line, EndLine: fact.EndLine, Role: contextSourceRole(pack, index, fact),
			Kind: fact.Kind, Name: fact.Name, Qualified: fact.Qualified,
		}
		declaration, err := renderSourceCandidate(raw, file, "signature")
		if err != nil {
			rejected[factID] = stableContextSourceOmissionReason(err)
			continue
		}
		if declaration.StartLine >= section.StartLine && declaration.EndLine <= section.EndLine {
			verified = append(verified, factID)
		}
	}
	return orderedContextConcernIDs(verified), rejected
}

func contextInheritedOwnerCandidate(
	index scan.AgentContextIndexRecord,
	candidate sourceCandidate,
) (sourceCandidate, bool) {
	if len(contextSourceCandidateFactIDs(candidate)) != 1 {
		return sourceCandidate{}, false
	}
	factID := contextSourceCandidateFactIDs(candidate)[0]
	var inherited scan.AgentContextFactRecord
	for _, fact := range index.Facts {
		if fact.ID == factID {
			inherited = fact
			break
		}
	}
	if inherited.ID == "" || !contextGenericPersistenceFact(inherited) {
		return sourceCandidate{}, false
	}
	ownerQualified := contextQualifiedOwner(inherited.Qualified)
	if ownerQualified == "" {
		return sourceCandidate{}, false
	}
	ownerShort := ownerQualified
	if separator := strings.LastIndex(ownerShort, "."); separator >= 0 {
		ownerShort = ownerShort[separator+1:]
	}

	owners := make(map[string]scan.AgentContextFactRecord)
	for _, fact := range index.Facts {
		if normalizeContextProject(fact.Project) != normalizeContextProject(inherited.Project) ||
			filepath.ToSlash(fact.File) != filepath.ToSlash(inherited.File) ||
			normalizedContextConcernKind(fact.Kind) == contextConcernPersistence {
			continue
		}
		if strings.TrimSpace(fact.Qualified) != ownerQualified &&
			strings.TrimSpace(fact.Name) != ownerShort {
			continue
		}
		key := strings.Join([]string{
			normalizeContextProject(fact.Project),
			filepath.ToSlash(fact.File),
			strconv.Itoa(fact.Line),
			compactContextIdentifier(fact.Name),
		}, "\x00")
		current, found := owners[key]
		if !found ||
			strings.TrimSpace(fact.Qualified) == ownerQualified &&
				strings.TrimSpace(current.Qualified) != ownerQualified ||
			strings.TrimSpace(fact.Qualified) == strings.TrimSpace(current.Qualified) &&
				contextDomainModelConfidenceScore(fact.Confidence) >
					contextDomainModelConfidenceScore(current.Confidence) {
			owners[key] = fact
		}
	}
	if len(owners) != 1 {
		return sourceCandidate{}, false
	}
	var owner scan.AgentContextFactRecord
	for _, candidateOwner := range owners {
		owner = candidateOwner
	}
	result := candidate
	result.Name = firstNonEmptyContext(owner.Name, ownerShort)
	result.Qualified = ownerQualified
	result.StartLine = owner.Line
	result.EndLine = owner.EndLine
	result.SourceState = "inherited_owner_current"
	return result, true
}

func contextInheritedFactMatchesOwner(
	fact scan.AgentContextFactRecord,
	owner sourceCandidate,
) bool {
	return contextGenericPersistenceFact(fact) &&
		normalizeContextProject(fact.Project) == normalizeContextProject(owner.Project) &&
		filepath.ToSlash(fact.File) == filepath.ToSlash(owner.Path) &&
		contextQualifiedOwner(fact.Qualified) == strings.TrimSpace(owner.Qualified)
}

func contextQualifiedOwner(qualified string) string {
	qualified = strings.TrimSpace(qualified)
	separatorIndex := -1
	for _, separator := range []string{"::", "#", "."} {
		if index := strings.LastIndex(qualified, separator); index > separatorIndex {
			separatorIndex = index
		}
	}
	if separatorIndex <= 0 {
		return ""
	}
	return strings.TrimSpace(qualified[:separatorIndex])
}

func contextSourceRole(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
) string {
	if contextLocationIDs(pack.Tests)[fact.ID] || normalizedContextConcernKind(fact.Kind) == contextConcernTests || contextFactUsesTestSource(fact) {
		return "test"
	}
	switch {
	case strings.EqualFold(fact.Kind, "api_endpoint") && contextFactMatchesSelectedEndpoint(fact, pack.Endpoints):
		return "entrypoint"
	case contextLocationIDs(pack.Entrypoints)[fact.ID]:
		return "entrypoint"
	case contextDomainModelFact(fact, contextSourceDomainModelTokens(pack, index)):
		return contextConcernDomainModel
	case contextLocationIDs(pack.Contracts)[fact.ID] || strings.EqualFold(fact.Kind, "api_contract"):
		return "contract"
	case contextLocationIDs(pack.Persistence)[fact.ID] ||
		normalizedContextConcernKind(fact.Kind) == contextConcernPersistence ||
		contextPersistenceOwnerFact(fact):
		return "persistence"
	default:
		return "call_chain"
	}
}

func contextPersistenceOwnerFact(fact scan.AgentContextFactRecord) bool {
	if contextFactUsesTestSource(fact) || contextPackSourceFile(fact.File) == "" {
		return false
	}
	for _, identity := range contextPersistenceOwnerIdentities(fact) {
		if strings.HasSuffix(identity, "repository") {
			return true
		}
	}
	return false
}

func contextPersistenceDerivedOwnerFact(fact scan.AgentContextFactRecord) bool {
	if !contextPersistenceOwnerFact(fact) {
		return false
	}
	for _, identity := range contextPersistenceOwnerIdentities(fact) {
		if strings.HasSuffix(identity, "vrepository") {
			return true
		}
	}
	tokens := contextTokenSet(strings.Join([]string{fact.Name, fact.Qualified}, " "))
	for _, qualifier := range []string{"projection", "readmodel", "view"} {
		if tokens[qualifier] {
			return true
		}
	}
	return false
}

func contextPersistenceOwnerIdentities(fact scan.AgentContextFactRecord) []string {
	base := filepath.Base(fact.File)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return []string{
		compactContextIdentifier(fact.Name),
		compactContextIdentifier(contextQualifiedOwner(fact.Qualified)),
		compactContextIdentifier(base),
	}
}

func contextPersistenceMatchesPrimaryDomainModel(
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) bool {
	return contextPersistenceMatchesDomainModel(index, fact, domainTokens, nil)
}

func contextPersistenceMatchesSelectedDomainModel(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
) bool {
	return contextPersistenceFactMatchesRequestedDomainModel(
		pack,
		index,
		fact,
		contextRequestedDomainModelIDs(pack, index),
	)
}

func contextRequestedDomainModelIDs(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) map[string]bool {
	planned := []contextConcern(nil)
	if seed, ok := contextConcernPlanningSeed(index, contextSelectionQuery(pack)); ok {
		planned = planContextConcerns(contextSelectionQuery(pack), index, seed)
	}
	return contextRequestedDomainModelIDsFromConcerns(pack, index, planned)
}

func contextRequestedDomainModelIDsFromConcerns(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
) map[string]bool {
	domainTokens := contextSourceDomainModelTokens(pack, index)
	selected := make(map[string]bool, len(pack.selectedSourceFactIDs))
	for _, factID := range pack.selectedSourceFactIDs {
		selected[factID] = true
	}
	selectedModels := make(map[string]bool)
	for _, model := range index.Facts {
		if selected[model.ID] && contextDomainModelFact(model, domainTokens) {
			selectedModels[model.ID] = true
		}
	}
	for _, concern := range concerns {
		if concern.kind != contextConcernDomainModel {
			continue
		}
		for _, factID := range concern.candidateFactIDs {
			selectedModels[factID] = true
		}
	}
	return selectedModels
}

func contextPersistenceMatchesRequestedDomainModel(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
	requestedModelIDs map[string]bool,
) bool {
	for _, fact := range contextSourceOptionFacts(index, option) {
		if contextPersistenceFactMatchesRequestedDomainModel(
			pack,
			index,
			fact,
			requestedModelIDs,
		) {
			return true
		}
	}
	return false
}

func contextSourceCandidateHasRequestedModel(
	candidate sourceCandidate,
	requestedModelIDs map[string]bool,
) bool {
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if requestedModelIDs[factID] {
			return true
		}
	}
	return false
}

func contextPersistenceFactMatchesRequestedDomainModel(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
	requestedModelIDs map[string]bool,
) bool {
	domainTokens := contextSourceDomainModelTokens(pack, index)
	return contextPersistenceFactMatchesRequestedDomainModelWithTokens(
		index,
		fact,
		domainTokens,
		requestedModelIDs,
	)
}

func contextPersistenceFactsMatchRequestedDomainModel(
	index scan.AgentContextIndexRecord,
	facts []scan.AgentContextFactRecord,
	domainTokens map[string]bool,
	requestedModelIDs map[string]bool,
) bool {
	for _, fact := range facts {
		if contextPersistenceFactMatchesRequestedDomainModelWithTokens(
			index,
			fact,
			domainTokens,
			requestedModelIDs,
		) {
			return true
		}
	}
	return false
}

func contextPersistenceFactMatchesRequestedDomainModelWithTokens(
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
	domainTokens map[string]bool,
	requestedModelIDs map[string]bool,
) bool {
	if len(requestedModelIDs) == 0 {
		return contextPersistenceMatchesPrimaryDomainModel(index, fact, domainTokens)
	}
	return contextPersistenceMatchesDomainModel(index, fact, domainTokens, requestedModelIDs)
}

func contextPersistenceMatchesDomainModel(
	index scan.AgentContextIndexRecord,
	fact scan.AgentContextFactRecord,
	domainTokens map[string]bool,
	allowedModels map[string]bool,
) bool {
	repositoryStems := make(map[string]bool)
	for _, identity := range contextPersistenceOwnerIdentities(fact) {
		if !strings.HasSuffix(identity, "repository") {
			continue
		}
		stem := strings.TrimSuffix(identity, "repository")
		if stem != "" {
			repositoryStems[stem] = true
		}
	}
	if len(repositoryStems) == 0 {
		return false
	}
	project := normalizeContextProject(fact.Project)
	for _, model := range index.Facts {
		if len(allowedModels) > 0 && !allowedModels[model.ID] ||
			normalizeContextProject(model.Project) != project ||
			!contextDomainModelFact(model, domainTokens) ||
			!contextPrimaryDomainModelFact(model) ||
			contextStableFactIdentityMatchCount(model, domainTokens) < 2 {
			continue
		}
		identity := compactContextIdentifier(firstNonEmptyContext(model.Name, model.Qualified))
		for _, suffix := range contextDomainModelSuffixes {
			identity = strings.TrimSuffix(identity, suffix)
		}
		if repositoryStems[identity] {
			return true
		}
	}
	return false
}

func contextSourceCandidateFactIDs(candidate sourceCandidate) []string {
	if len(candidate.FactIDs) > 0 {
		return candidate.FactIDs
	}
	if candidate.FactID == "" {
		return nil
	}
	return []string{candidate.FactID}
}

func contextSourceOptionConcerns(
	candidate sourceCandidate,
	section ContextSourceSection,
	concerns []contextConcern,
	index scan.AgentContextIndexRecord,
) ([]string, bool) {
	return contextSourceOptionConcernsForQuery(
		candidate,
		section,
		concerns,
		index,
		"",
	)
}

func contextSourceOptionConcernsForQuery(
	candidate sourceCandidate,
	section ContextSourceSection,
	concerns []contextConcern,
	index scan.AgentContextIndexRecord,
	query string,
) ([]string, bool) {
	requestedActions := contextActionFamilies(query, "")
	actionAligned := len(requestedActions) == 0 ||
		contextSourceCandidateActionAligned(candidate, index, requestedActions)
	return contextSourceOptionConcernsWithAction(
		candidate,
		section,
		concerns,
		index,
		actionAligned,
	)
}

func contextSourceOptionConcernsWithAction(
	candidate sourceCandidate,
	section ContextSourceSection,
	concerns []contextConcern,
	index scan.AgentContextIndexRecord,
	actionAligned bool,
) ([]string, bool) {
	factIDs := make(map[string]bool)
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		factIDs[factID] = true
	}
	keys := []string{}
	required := false
	for _, concern := range concerns {
		if !concern.required {
			continue
		}
		covered := false
		for _, factID := range concern.candidateFactIDs {
			if factIDs[factID] {
				covered = true
				break
			}
		}
		if covered && concern.project != "" &&
			normalizeContextProject(candidate.Project) != concern.project {
			covered = false
		}
		if concern.facet != "" {
			if !covered ||
				concern.kind == contextConcernSideEffects && !actionAligned ||
				!contextSourceSectionSupportsEvidence(section, concern) {
				continue
			}
			keys = append(keys, concern.key)
			required = true
			continue
		}
		if covered && contextSourceRequiresRenderedConcernEvidence(concern.kind) {
			covered = contextSourceSectionSupportsConcern(section, concern)
		}
		if concern.kind == contextConcernProject {
			covered = candidate.Role != "test" && normalizeContextProject(candidate.Project) == concern.project
		}
		if !covered {
			covered = contextSourceSectionSupportsConcern(section, concern)
		}
		if !covered {
			continue
		}
		keys = append(keys, concern.key)
		required = required || concern.required
	}
	sort.Strings(keys)
	return keys, required
}

func contextSourceFactsActionAligned(
	facts []scan.AgentContextFactRecord,
	candidate sourceCandidate,
	requestedActions map[string]bool,
) bool {
	for _, fact := range facts {
		actions := contextActionFamilies(
			strings.Join([]string{fact.Name, fact.Qualified}, " "),
			fact.HTTPMethod,
		)
		if contextActionFamiliesOverlap(requestedActions, actions) {
			return true
		}
	}
	actions := contextActionFamilies(
		strings.Join([]string{candidate.Name, candidate.Qualified}, " "),
		"",
	)
	return contextActionFamiliesOverlap(requestedActions, actions)
}

func contextSourceCandidateActionAligned(
	candidate sourceCandidate,
	index scan.AgentContextIndexRecord,
	requestedActions map[string]bool,
) bool {
	candidateFactIDs := make(map[string]bool)
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		candidateFactIDs[factID] = true
	}
	facts := make([]scan.AgentContextFactRecord, 0, len(candidateFactIDs))
	for _, fact := range index.Facts {
		if !candidateFactIDs[fact.ID] {
			continue
		}
		facts = append(facts, fact)
	}
	return contextSourceFactsActionAligned(facts, candidate, requestedActions)
}

func contextSourceSectionSupportsEvidence(
	section ContextSourceSection,
	concern contextConcern,
) bool {
	if concern.project != "" &&
		normalizeContextProject(section.Project) != concern.project {
		return false
	}
	if section.Role == "test" &&
		concern.kind != contextConcernTests &&
		concern.kind != contextConcernSideEffects {
		return false
	}
	if concern.facet == "" {
		return contextSourceSectionSupportsConcern(section, concern)
	}
	content := strings.ToLower(contextSourceSemanticContent(section.Content))
	switch concern.kind + "#" + concern.facet {
	case contextConcernAuth + "#client_transport":
		return contextSourceContainsAny(
			content,
			"basicauthenticationinterceptor",
			"basicauthentication(",
			"defaultheader",
			"authorization",
			"oauth2authorizedclient",
			".setbasicauth(",
		)
	case contextConcernAuth + "#server_policy":
		return contextSourceContainsAny(
			content,
			"securityfilterchain",
			".httpbasic(",
			".oauth2resourceserver(",
			"@securityrequirement",
		)
	case contextConcernConfiguration + "#binding":
		return contextSourceContainsAny(
			content,
			"@configurationproperties",
			"@value(",
			"connecttimeout",
			"readtimeout",
			"maxretries",
		)
	case contextConcernConfiguration + "#consumer":
		return contextSourceContainsAny(
			content,
			"configuration.",
			"config.get",
			"getconfig(",
			"getbaseurl(",
			"getconnecttimeout(",
			"getreadtimeout(",
			"getmaxretries(",
			"getpath(",
		)
	case contextConcernResilience + "#retry_policy":
		return contextSourceContainsAny(content, "@retryable", "maxattempts")
	case contextConcernResilience + "#recovery":
		return contextSourceContainsAny(content, "@recover", "recovering", "recovery")
	case contextConcernSideEffects + "#mail":
		return contextSourceContainsAny(content, "mailservice", "sendmail", "sendemail")
	case contextConcernSideEffects + "#audit":
		return contextSourceContainsAny(
			content,
			"protocolservice.",
			"trackingservice.",
			"audit",
			"log.",
		)
	case contextConcernSideEffects + "#user_information":
		return contextSourceContainsAny(
			content,
			"userservice.",
			"usermgmt",
			"getuser",
			"userinformation",
		)
	default:
		return contextSourceSectionSupportsConcern(section, concern)
	}
}

func contextSourceSectionSupportsConcern(
	section ContextSourceSection,
	concern contextConcern,
) bool {
	if concern.project != "" &&
		normalizeContextProject(section.Project) != concern.project {
		return false
	}
	if section.Role == "test" {
		if concern.kind != contextConcernTests {
			return false
		}
	}

	semanticContent := contextSourceSemanticContent(section.Content)
	content := strings.ToLower(semanticContent)
	switch concern.kind {
	case contextConcernAuth:
		return contextSourceContainsAny(content,
			"@securityrequirement",
			".authenticated(",
			"authorization",
			"basicauth",
			"basic_auth",
			"oauth2",
			"securityfilterchain",
		)
	case contextConcernConfiguration:
		return section.RenderMode != "signature" && contextValueRequestsConcern(semanticContent, contextConcernConfiguration) ||
			contextSourceContainsAny(content,
				"@configurationproperties",
				"@value(",
				"configuration.",
				"config.",
				"connecttimeout",
				"readtimeout",
				"maxretries",
			)
	case contextConcernResilience:
		return contextSourceContainsAny(content,
			"@retryable",
			"maxattempts",
			"recover",
			"retrytemplate",
			"retry_template",
			"timeout",
		)
	case contextConcernPersistence:
		return contextSourceContainsAny(content,
			"@transactional",
			"entitymanager",
			"jparepository",
			"crudrepository",
			"repository.",
		) || section.Role == contextConcernPersistence &&
			section.RenderMode != "signature" &&
			contextSourceContainsAny(content, ".delete(", ".remove(")
	case contextConcernSideEffects:
		return contextSourceContainsAny(content,
			"mailservice.",
			"protocolservice.",
			"trackingservice.",
			"eventpublisher.",
			"log.",
			" publish",
		)
	case contextConcernTests:
		return section.Role == "test" &&
			contextSourceSectionHasExecutableTest(section, semanticContent)
	default:
		return false
	}
}

func contextSourceSectionHasExecutableTest(section ContextSourceSection, content string) bool {
	if section.RenderMode == "signature" {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") ||
			line == "{" || line == "}" || strings.HasSuffix(line, "{") ||
			strings.HasSuffix(line, ":") ||
			contextSourceTestDeclarationLine(line) {
			continue
		}
		if contextSourceTestLineIsEmptyInlineWrapper(line) {
			continue
		}
		if contextSourceTestLineHasAssignment(line) ||
			contextSourceTestLineHasCall(line) ||
			strings.HasPrefix(line, "assert ") {
			return true
		}
	}
	return false
}

func contextSourceTestLineIsEmptyInlineWrapper(line string) bool {
	normalized := strings.Join(strings.Fields(line), "")
	isTestWrapper := false
	for _, prefix := range []string{"test(", "it(", "describe("} {
		if strings.HasPrefix(normalized, prefix) {
			isTestWrapper = true
			break
		}
	}
	if !isTestWrapper {
		return false
	}

	wrapperStart := strings.IndexByte(line, '(')
	if wrapperStart < 0 {
		return false
	}
	callbackStart := -1
	parenthesisDepth, bracketDepth, braceDepth := 1, 0, 0
	for index := wrapperStart + 1; index < len(line); index++ {
		switch line[index] {
		case '(':
			parenthesisDepth++
		case ')':
			parenthesisDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		default:
			if parenthesisDepth != 1 || bracketDepth != 0 || braceDepth != 0 {
				continue
			}
			if strings.HasPrefix(line[index:], "=>") {
				callbackStart = index + len("=>")
			} else if strings.HasPrefix(line[index:], "function") &&
				isWholeSourceToken(line, index, index+len("function")) {
				callbackStart = index + len("function")
			}
		}
		if callbackStart >= 0 || parenthesisDepth == 0 {
			break
		}
	}
	if callbackStart < 0 {
		return false
	}

	bodyOffset := strings.IndexByte(line[callbackStart:], '{')
	if bodyOffset < 0 {
		return false
	}
	bodyStart := callbackStart + bodyOffset
	depth := 0
	for index := bodyStart; index < len(line); index++ {
		switch line[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(line[bodyStart+1:index]) == ""
			}
		}
	}
	return false
}

func contextSourceTestDeclarationLine(line string) bool {
	for _, prefix := range []string{
		"package ",
		"import ",
		"class ",
		"interface ",
		"type ",
		"struct ",
		"enum ",
		"func ",
		"def ",
		"function ",
		"export function ",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return line == "pass"
}

func contextSourceTestLineHasAssignment(line string) bool {
	if strings.Contains(line, ":=") {
		return true
	}
	for index := 0; index < len(line); index++ {
		if line[index] != '=' {
			continue
		}
		var previous, next byte
		if index > 0 {
			previous = line[index-1]
		}
		if index+1 < len(line) {
			next = line[index+1]
		}
		if previous != '=' && previous != '!' && previous != '<' && previous != '>' &&
			next != '=' && next != '>' {
			return true
		}
	}
	return false
}

func contextSourceTestLineHasCall(line string) bool {
	open := strings.IndexByte(line, '(')
	if open < 0 || !strings.Contains(line[open+1:], ")") {
		return false
	}
	before := strings.TrimSpace(line[:open])
	if before == "" {
		return false
	}
	if strings.Contains(before, ".") || len(strings.Fields(before)) == 1 {
		return true
	}
	for _, prefix := range []string{"return ", "throw ", "await ", "go ", "defer "} {
		if strings.HasPrefix(before, prefix) {
			return true
		}
	}
	return false
}

func contextSourceSemanticContent(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for index, line := range lines {
		prefix, source, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(prefix)); err == nil {
			lines[index] = source
		}
	}
	return strings.Join(sourceCodeMask(lines), "\n")
}

func contextSourceContainsAny(content string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(content, value) {
			return true
		}
	}
	return false
}

func contextSourceCandidateDistance(candidate sourceCandidate, distances map[string]int) int {
	best := maximumContextPathHops + 1
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if distance, ok := distances[factID]; ok && distance < best {
			best = distance
		}
	}
	return best
}

func contextSourceDomainModelTokens(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
) map[string]bool {
	query := contextSelectionQuery(pack)
	if !contextQueryRequestsConcern(query, contextConcernDomainModel) {
		return nil
	}
	aliases := contextProjectAliases(index.Facts, index.Coverage)
	explicitProjects := contextExplicitProjects(query, aliases)
	return contextDomainModelQueryTokens(query, aliases, explicitProjects)
}

func contextSourceEvidenceFamily(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
) string {
	facts := contextSourceOptionFacts(index, option)
	domainTokens := contextSourceDomainModelTokens(pack, index)
	return contextSourceEvidenceFamilyForFacts(pack, option, facts, domainTokens)
}

func contextSourceEvidenceFamilyForFacts(
	pack ContextPack,
	option contextSourceOption,
	facts []scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) string {
	if option.candidate.Role == contextConcernDomainModel {
		return contextConcernDomainModel
	}
	for _, fact := range facts {
		if contextDomainModelFact(fact, domainTokens) {
			return contextConcernDomainModel
		}
	}
	switch option.candidate.Role {
	case "entrypoint":
		return "action"
	case "contract":
		return "contract"
	case "persistence":
		return contextConcernPersistence
	case "test":
		return contextConcernTests
	}
	for _, fact := range facts {
		switch normalizedContextConcernKind(fact.Kind) {
		case contextConcernHTTPContract:
			return "contract"
		case contextConcernAuth:
			return contextConcernAuth
		case contextConcernConfiguration:
			return contextConcernConfiguration
		case contextConcernResilience:
			return contextConcernResilience
		case contextConcernPersistence:
			return contextConcernPersistence
		case contextConcernSideEffects:
			return contextConcernSideEffects
		case contextConcernTests:
			return contextConcernTests
		}
	}
	for _, key := range option.concernKeys {
		kind, _, _ := strings.Cut(key, ":")
		if contextSourceCrossCuttingFamily(kind) {
			return kind
		}
	}
	if contextSourceOptionActionAligned(pack, option) {
		return "action"
	}
	return "other"
}

func contextSourceCandidateQuality(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
) int {
	facts := contextSourceOptionFacts(index, option)
	domainTokens := contextSourceDomainModelTokens(pack, index)
	option.evidenceFamily = contextSourceEvidenceFamilyForFacts(
		pack,
		option,
		facts,
		domainTokens,
	)
	return contextSourceCandidateQualityForFacts(
		pack,
		index,
		option,
		facts,
		domainTokens,
	)
}

func contextSourceCandidateQualityForFacts(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
	facts []scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) int {
	family := option.evidenceFamily
	stableMatches := 0
	confidence := 0
	genericPersistence := false
	dependentDomainModel := false
	dependentPersistence := false
	derivedPersistence := false
	for _, fact := range facts {
		stableMatches = max(
			stableMatches,
			contextStableFactIdentityMatchCount(fact, domainTokens),
		)
		confidence = max(confidence, contextSourceConfidenceQuality(fact.Confidence))
		genericPersistence = genericPersistence || contextGenericPersistenceFact(fact)
		dependentDomainModel = dependentDomainModel || contextDomainModelDependencyFact(fact)
		dependentPersistence = dependentPersistence ||
			family == contextConcernPersistence && contextDomainModelDependencyFact(fact)
		derivedPersistence = derivedPersistence ||
			family == contextConcernPersistence && contextPersistenceDerivedOwnerFact(fact)
	}
	stableMatches = min(stableMatches, 3)
	quality := 60*stableMatches + confidence
	switch family {
	case contextConcernDomainModel:
		quality += 260
		if dependentDomainModel {
			quality -= 180
		}
	case contextConcernPersistence:
		if genericPersistence {
			quality -= 180
		} else {
			quality += 220
		}
		if dependentPersistence {
			quality -= 180
		}
		if derivedPersistence {
			quality -= 180
		}
		matchesModel := option.matchesModel
		if !option.modelMatchSet {
			for _, fact := range facts {
				if contextPersistenceMatchesPrimaryDomainModel(index, fact, domainTokens) {
					matchesModel = true
					break
				}
			}
		}
		if matchesModel {
			quality += 260
		}
	}
	if contextSourceOptionActionAligned(pack, option) {
		quality += 200
	}
	if option.pathDistance > maximumContextPathHops {
		quality -= 120
	}
	return max(-500, min(quality, 1000))
}

func contextSourceOptionQuality(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
) int {
	option.evidenceFamily = contextSourceEvidenceFamily(pack, index, option)
	option.candidateQuality = contextSourceCandidateQuality(pack, index, option)
	return contextSourceOptionQualityForProfile(option)
}

func contextSourceOptionQualityForProfile(option contextSourceOption) int {
	quality := option.candidateQuality
	switch option.section.RenderMode {
	case "declaration_body", "body":
		quality += 180
	case "focused":
		quality += 100
	}
	if option.section.RenderMode == "signature" &&
		contextSourceCrossCuttingFamily(option.evidenceFamily) {
		quality -= 320
	}
	return max(-500, min(quality, 1000))
}

func contextSourceOptionFacts(
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
) []scan.AgentContextFactRecord {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	return contextSourceCandidateFacts(option.candidate, factByID)
}

func contextSourceCandidateFacts(
	candidate sourceCandidate,
	factByID map[string]scan.AgentContextFactRecord,
) []scan.AgentContextFactRecord {
	facts := make([]scan.AgentContextFactRecord, 0, len(contextSourceCandidateFactIDs(candidate)))
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if fact, ok := factByID[factID]; ok {
			facts = append(facts, fact)
		}
	}
	if len(facts) == 0 {
		facts = append(facts, scan.AgentContextFactRecord{
			ID: candidate.FactID, Project: candidate.Project,
			Kind: candidate.Kind, Name: candidate.Name,
			Qualified: candidate.Qualified, File: candidate.Path,
		})
	}
	return facts
}

func contextSourceConfidenceQuality(confidence string) int {
	switch strings.ToUpper(strings.TrimSpace(confidence)) {
	case "EXACT":
		return 40
	case "RESOLVED", "EXTRACTED":
		return 20
	default:
		return 0
	}
}

func contextSourceStableDomainMatches(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
) int {
	domainTokens := contextSourceDomainModelTokens(pack, index)
	return contextSourceStableDomainMatchesForFacts(
		contextSourceOptionFacts(index, option),
		domainTokens,
	)
}

func contextSourceStableDomainMatchesForFacts(
	facts []scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) int {
	matches := 0
	for _, fact := range facts {
		matches = max(matches, contextStableFactIdentityMatchCount(fact, domainTokens))
	}
	return matches
}

func contextSourceMatchesPrimaryDomainModel(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	option contextSourceOption,
) bool {
	return contextPersistenceMatchesRequestedDomainModel(
		pack,
		index,
		option,
		contextRequestedDomainModelIDs(pack, index),
	)
}

func contextSourceCrossCuttingFamily(family string) bool {
	switch family {
	case contextConcernAuth, contextConcernConfiguration, contextConcernResilience, contextConcernSideEffects:
		return true
	default:
		return false
	}
}

func contextSourceRequiresRenderedConcernEvidence(kind string) bool {
	switch kind {
	case contextConcernAuth,
		contextConcernConfiguration,
		contextConcernResilience,
		contextConcernPersistence,
		contextConcernSideEffects,
		contextConcernTests:
		return true
	default:
		return false
	}
}

func contextSourceOptionActionAligned(pack ContextPack, option contextSourceOption) bool {
	query := contextSelectionQuery(pack)
	requested := contextActionFamilies(query, contextRequestedHTTPMethod(query))
	candidate := contextActionFamilies(
		strings.Join([]string{
			option.candidate.Name,
			option.candidate.Qualified,
			option.candidate.Kind,
		}, " "),
		"",
	)
	return contextActionFamiliesOverlap(requested, candidate)
}

func contextSourceEffectiveCandidateQuality(pack ContextPack, option contextSourceOption) int {
	if option.profiled {
		return option.candidateQuality
	}
	return contextSourceCandidateQuality(pack, scan.AgentContextIndexRecord{}, option)
}

func contextSourceEffectiveQuality(pack ContextPack, option contextSourceOption) int {
	if option.profiled {
		return option.quality
	}
	return contextSourceOptionQuality(pack, scan.AgentContextIndexRecord{}, option)
}

func contextSourceEffectiveEvidenceFamily(pack ContextPack, option contextSourceOption) string {
	if option.profiled {
		return option.evidenceFamily
	}
	return contextSourceEvidenceFamily(pack, scan.AgentContextIndexRecord{}, option)
}

func contextSourceEffectiveStableDomainMatches(pack ContextPack, option contextSourceOption) int {
	if option.profiled {
		return option.stableMatches
	}
	return contextSourceStableDomainMatches(pack, scan.AgentContextIndexRecord{}, option)
}

func contextSourceEvidenceFamilyLimit(family string) int {
	switch family {
	case contextConcernDomainModel, contextConcernPersistence:
		return 2
	default:
		return 1
	}
}

type contextSourceBoundary struct {
	factID  string
	project string
}

func mandatoryContextSourceBoundaries(
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
	core []contextSourceBoundary,
	distances map[string]int,
) []contextSourceBoundary {
	boundaries := append([]contextSourceBoundary(nil), core...)
	edges := append([]scan.AgentContextEdgeRecord(nil), index.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		leftDistance, leftOK := distances[edges[i].FromFactID]
		rightDistance, rightOK := distances[edges[j].FromFactID]
		if leftOK != rightOK {
			return leftOK
		}
		if leftDistance != rightDistance {
			return leftDistance < rightDistance
		}
		return contextEdgeLess(edges[i], edges[j])
	})
	for _, edge := range edges {
		if normalizedContextConcernKind(edge.Kind) != contextConcernHTTPContract {
			continue
		}
		if _, connected := distances[edge.FromFactID]; !connected {
			continue
		}
		boundaries = append(boundaries,
			contextSourceBoundary{factID: edge.FromFactID},
			contextSourceBoundary{factID: edge.ToFactID},
		)
		break
	}
	return boundaries
}

func contextCoreSourceBoundaries(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	distances map[string]int,
) []contextSourceBoundary {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	selectedFacts := make(map[string]bool, len(pack.selectedSourceFactIDs))
	for _, factID := range pack.selectedSourceFactIDs {
		selectedFacts[factID] = true
	}
	entryID := ""
	if len(pack.Entrypoints) > 0 {
		entryID = pack.Entrypoints[0].ID
	} else {
		entryID = contextEndpointHandlerFactID(pack, index, selectedFacts)
	}
	if entryID == "" {
		selected := append([]string(nil), pack.selectedSourceFactIDs...)
		sort.Strings(selected)
		for _, factID := range selected {
			if distances[factID] == 0 {
				entryID = factID
				break
			}
		}
	}
	entry, ok := factByID[entryID]
	if !ok {
		return nil
	}
	boundaries := []contextSourceBoundary{{factID: entryID}}
	entryProject := normalizeContextProject(entry.Project)
	entryFile := contextPackSourceFile(entry.File)
	localCallTargets := make(map[string]bool)
	for _, edge := range index.Edges {
		kind := strings.ToLower(strings.TrimSpace(edge.Kind))
		if !selectedFacts[edge.FromFactID] || !selectedFacts[edge.ToFactID] ||
			(kind != "call" && kind != "calls" && kind != "use") {
			continue
		}
		localCallTargets[edge.ToFactID] = true
	}
	type coreCandidate struct {
		factID   string
		distance int
		file     string
	}
	candidates := []coreCandidate{}
	for _, factID := range pack.selectedSourceFactIDs {
		fact := factByID[factID]
		distance, connected := distances[factID]
		if !connected {
			distance = maximumContextPathHops + 1
		}
		kind := strings.ToLower(strings.TrimSpace(fact.Kind))
		file := contextPackSourceFile(fact.File)
		if distance <= 0 || normalizeContextProject(fact.Project) != entryProject ||
			file == "" || file == entryFile || !localCallTargets[factID] ||
			(kind != "symbol" && kind != "backend_handler") || contextFactUsesTestSource(fact) {
			continue
		}
		candidates = append(candidates, coreCandidate{factID: factID, distance: distance, file: file})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].file != candidates[j].file {
			return candidates[i].file < candidates[j].file
		}
		return candidates[i].factID < candidates[j].factID
	})
	if len(candidates) > 0 {
		boundaries = append(boundaries, contextSourceBoundary{factID: candidates[0].factID})
	}
	boundaryFactIDs := make(map[string]bool, len(boundaries))
	for _, boundary := range boundaries {
		boundaryFactIDs[boundary.factID] = true
	}
	for _, contract := range pack.Contracts {
		if contract.ID == "" || !selectedFacts[contract.ID] || boundaryFactIDs[contract.ID] {
			continue
		}
		if _, exists := factByID[contract.ID]; !exists {
			continue
		}
		boundaries = append(boundaries, contextSourceBoundary{factID: contract.ID})
		boundaryFactIDs[contract.ID] = true
	}
	boundaryProjects := make(map[string]bool, len(boundaries))
	for _, boundary := range boundaries {
		if fact, exists := factByID[boundary.factID]; exists {
			boundaryProjects[normalizeContextProject(fact.Project)] = true
		}
	}
	bestRelatedByProject := make(map[string]scan.AgentContextFactRecord)
	bestRelatedQuality := make(map[string]int)
	for _, file := range pack.Files {
		if !strings.Contains(file.Role, "related_project") {
			continue
		}
		project := normalizeContextProject(file.Project)
		if boundaryProjects[project] {
			continue
		}
		matches := []scan.AgentContextFactRecord{}
		for _, fact := range index.Facts {
			if !selectedFacts[fact.ID] ||
				normalizeContextProject(fact.Project) != project ||
				contextPackSourceFile(fact.File) != contextPackSourceFile(file.Path) {
				continue
			}
			if file.StartLine > 0 && fact.Line > 0 &&
				(fact.Line < file.StartLine || file.EndLine > 0 && fact.Line > file.EndLine) {
				continue
			}
			matches = append(matches, fact)
		}
		sort.Slice(matches, func(left, right int) bool {
			if matches[left].Line != matches[right].Line {
				return matches[left].Line < matches[right].Line
			}
			return matches[left].ID < matches[right].ID
		})
		if len(matches) == 0 || boundaryFactIDs[matches[0].ID] {
			continue
		}
		fact := matches[0]
		role := contextSourceRole(pack, index, fact)
		option := contextSourceOption{
			candidate: sourceCandidate{
				FactID: fact.ID, FactIDs: []string{fact.ID}, Project: fact.Project,
				Path: fact.File, StartLine: fact.Line, EndLine: fact.EndLine,
				Role: role, Kind: fact.Kind, Name: fact.Name, Qualified: fact.Qualified,
			},
			projectKey: project, pathDistance: distances[fact.ID],
		}
		quality := contextSourceCandidateQuality(pack, index, option)
		current, found := bestRelatedByProject[project]
		if !found || quality > bestRelatedQuality[project] ||
			quality == bestRelatedQuality[project] && contextRelatedSourceFactLess(fact, current) {
			bestRelatedByProject[project] = fact
			bestRelatedQuality[project] = quality
		}
	}
	projects := make([]string, 0, len(bestRelatedByProject))
	for project := range bestRelatedByProject {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	for _, project := range projects {
		fact := bestRelatedByProject[project]
		if boundaryFactIDs[fact.ID] {
			continue
		}
		boundaries = append(boundaries, contextSourceBoundary{factID: fact.ID})
		boundaryFactIDs[fact.ID] = true
	}
	return boundaries
}

func contextRelatedSourceFactLess(
	left scan.AgentContextFactRecord,
	right scan.AgentContextFactRecord,
) bool {
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.ID < right.ID
}

func contextEndpointHandlerFactID(
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	selectedFacts map[string]bool,
) string {
	if len(pack.Endpoints) == 0 {
		return ""
	}
	endpoint := pack.Endpoints[0]
	outgoing := make(map[string]bool)
	for _, edge := range index.Edges {
		kind := strings.ToLower(strings.TrimSpace(edge.Kind))
		if selectedFacts[edge.FromFactID] && selectedFacts[edge.ToFactID] &&
			(kind == "call" || kind == "calls" || kind == "use") {
			outgoing[edge.FromFactID] = true
		}
	}
	candidates := []scan.AgentContextFactRecord{}
	for _, fact := range index.Facts {
		if !selectedFacts[fact.ID] ||
			normalizeContextProject(fact.Project) != normalizeContextProject(endpoint.Provider) ||
			contextPackSourceFile(fact.File) != contextPackSourceFile(endpoint.File) ||
			strings.TrimSpace(fact.Qualified) != strings.TrimSpace(endpoint.Handler) ||
			fact.Line != endpoint.Line {
			continue
		}
		candidates = append(candidates, fact)
	}
	sort.Slice(candidates, func(left, right int) bool {
		if outgoing[candidates[left].ID] != outgoing[candidates[right].ID] {
			return outgoing[candidates[left].ID]
		}
		leftEndpoint := strings.EqualFold(candidates[left].Kind, "api_endpoint")
		rightEndpoint := strings.EqualFold(candidates[right].Kind, "api_endpoint")
		if leftEndpoint != rightEndpoint {
			return !leftEndpoint
		}
		return candidates[left].ID < candidates[right].ID
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].ID
}

func enrichContextCoreSourceOptions(
	pack ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	state contextSourceSelectionState,
	boundaries []contextSourceBoundary,
) (ContextPack, error) {
	var err error
	for _, mode := range []string{"declaration_body", "focused", "body"} {
		pack, err = enrichContextCoreSourceMode(pack, request, options, state, boundaries, mode)
		if err != nil {
			return ContextPack{}, err
		}
	}
	return pack, nil
}

func enrichContextCoreSourceMode(
	pack ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	state contextSourceSelectionState,
	boundaries []contextSourceBoundary,
	mode string,
) (ContextPack, error) {
	enriched := make(map[string]bool, len(boundaries))
	desiredMode := contextSourceRenderModeOrder(mode)
	for _, boundary := range boundaries {
		candidateKey, sectionIndex, currentMode, ok := selectedContextSourceOption(pack, options, state, boundary)
		if !ok || enriched[candidateKey] || currentMode <= desiredMode {
			continue
		}
		enriched[candidateKey] = true
		upgrade := contextSourceOption{}
		found := false
		for _, option := range options {
			if contextSourceCandidateKey(option.candidate) != candidateKey || option.section.RenderMode != mode {
				continue
			}
			if !found || contextSourceOptionLess(option, upgrade) {
				upgrade = option
				found = true
			}
		}
		if !found {
			continue
		}
		candidate := cloneContextPack(pack)
		candidate.SourceSections[sectionIndex] = upgrade.section
		candidate, err := finalizeContextEstimate(candidate)
		if err != nil {
			return ContextPack{}, err
		}
		fits, fitErr := contextSourcePackFits(candidate, request)
		if fitErr != nil {
			return ContextPack{}, fitErr
		}
		if fits {
			pack = candidate
		}
	}
	return pack, nil
}

func selectedContextSourceOption(
	pack ContextPack,
	options []contextSourceOption,
	state contextSourceSelectionState,
	boundary contextSourceBoundary,
) (string, int, int, bool) {
	for _, option := range options {
		key := contextSourceCandidateKey(option.candidate)
		if !state.selectedCandidates[key] ||
			boundary.factID != "" && !contextSourceCandidateHasFact(option.candidate, boundary.factID) {
			continue
		}
		for sectionIndex, section := range pack.SourceSections {
			if section == option.section {
				return key, sectionIndex, contextSourceRenderModeOrder(section.RenderMode), true
			}
		}
	}
	return "", 0, 0, false
}

func contextSourceBoundaryCovered(boundary contextSourceBoundary, state contextSourceSelectionState) bool {
	if boundary.factID != "" {
		return state.selectedFactIDs[boundary.factID]
	}
	return state.selectedProjects[boundary.project]
}

func smallestFittingContextSourceOption(
	pack ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
	state contextSourceSelectionState,
	boundary contextSourceBoundary,
) (contextSourceOption, bool, error) {
	best := contextSourceOption{}
	found := false
	fitting, err := fittingContextSourceOptions(pack, request, options, concerns, state)
	if err != nil {
		return contextSourceOption{}, false, err
	}
	for _, option := range fitting {
		if boundary.factID != "" && !contextSourceCandidateHasFact(option.candidate, boundary.factID) {
			continue
		}
		if boundary.project != "" && (option.projectKey != boundary.project || option.candidate.Role == "test") {
			continue
		}
		better := option.estimated < best.estimated ||
			option.estimated == best.estimated && contextSourceOptionLess(option, best)
		if boundary.factID != "" {
			optionGain := contextSourceBoundaryConcernGain(option, boundary, concerns, state)
			bestGain := contextSourceBoundaryConcernGain(best, boundary, concerns, state)
			if optionGain != bestGain {
				better = optionGain > bestGain
			}
		} else if boundary.project != "" {
			better = betterContextProjectBoundaryOption(pack, option, best)
		}
		if !found || better {
			best, found = option, true
		}
	}
	return best, found, nil
}

func contextSourceBoundaryConcernGain(
	option contextSourceOption,
	boundary contextSourceBoundary,
	concerns []contextConcern,
	state contextSourceSelectionState,
) int {
	if boundary.factID == "" {
		return 0
	}
	optionKeys := make(map[string]bool, len(option.concernKeys))
	for _, key := range option.concernKeys {
		optionKeys[key] = true
	}
	gain := 0
	for _, concern := range concerns {
		if !concern.required || state.coveredConcerns[concern.key] ||
			!optionKeys[concern.key] ||
			!slices.Contains(concern.candidateFactIDs, boundary.factID) {
			continue
		}
		gain++
	}
	return gain
}

func betterContextProjectBoundaryOption(
	pack ContextPack,
	left contextSourceOption,
	right contextSourceOption,
) bool {
	leftQuality := contextSourceEffectiveCandidateQuality(pack, left)
	rightQuality := contextSourceEffectiveCandidateQuality(pack, right)
	if leftQuality != rightQuality {
		return leftQuality > rightQuality
	}
	if left.estimated != right.estimated {
		return left.estimated < right.estimated
	}
	return contextSourceOptionLess(left, right)
}

func fittingContextSourceOptions(
	pack ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
	state contextSourceSelectionState,
) ([]contextSourceOption, error) {
	fitting := make([]contextSourceOption, 0, len(options))
	for _, option := range options {
		key := contextSourceCandidateKey(option.candidate)
		if state.selectedCandidates[key] {
			continue
		}
		fits, err := contextSourceOptionFits(pack, request, option, concerns, state)
		if err != nil {
			return nil, err
		}
		if fits {
			fitting = append(fitting, option)
		}
	}
	sort.Slice(fitting, func(i, j int) bool { return contextSourceOptionLess(fitting[i], fitting[j]) })
	return fitting, nil
}

func contextSourceOptionFits(
	pack ContextPack,
	request ContextRequest,
	option contextSourceOption,
	concerns []contextConcern,
	state contextSourceSelectionState,
) (bool, error) {
	reusesSection := contextSourceSectionAlreadyPresent(pack, option.section)
	if len(pack.SourceSections) >= MaxContextSourceSections && !reusesSection {
		return false, nil
	}
	candidate := cloneContextPack(pack)
	if !reusesSection {
		candidate.SourceSections = append(candidate.SourceSections, option.section)
	}
	if candidate.SourceUnrepresented > 0 {
		candidate.SourceUnrepresented--
	}
	covered := make(map[string]bool, len(state.coveredConcerns)+len(option.concernKeys))
	for key, value := range state.coveredConcerns {
		covered[key] = value
	}
	for _, key := range option.concernKeys {
		covered[key] = true
	}
	applyContextSourceCoverage(&candidate, concerns, covered)
	if request.MaxFiles > 0 && contextSourceFileCount(candidate) > request.MaxFiles {
		return false, nil
	}
	candidate, err := finalizeContextEstimate(candidate)
	if err != nil {
		return false, err
	}
	return contextSourcePackFits(candidate, request)
}

func contextSourceSectionAlreadyPresent(pack ContextPack, section ContextSourceSection) bool {
	project := normalizeContextProject(section.Project)
	path := contextPackSourceFile(section.Path)
	content := strings.TrimSpace(section.Content)
	for _, current := range pack.SourceSections {
		if normalizeContextProject(current.Project) == project &&
			contextPackSourceFile(current.Path) == path &&
			current.StartLine == section.StartLine &&
			current.EndLine == section.EndLine &&
			strings.TrimSpace(current.Content) == content {
			return true
		}
	}
	return false
}

func contextSourceFileCount(pack ContextPack) int {
	files := make(map[string]bool)
	add := func(project, path string) {
		project = normalizeContextProject(project)
		path = contextPackSourceFile(path)
		if path != "" {
			files[project+"\x00"+path] = true
		}
	}
	for _, location := range pack.Entrypoints {
		add(location.Project, location.File)
	}
	for _, endpoint := range pack.Endpoints {
		add(endpoint.Provider, endpoint.File)
	}
	for _, location := range pack.Contracts {
		add(location.Project, location.File)
	}
	for _, location := range pack.Persistence {
		add(location.Project, location.File)
	}
	for _, location := range pack.Tests {
		add(location.Project, location.File)
	}
	for _, file := range pack.Files {
		add(file.Project, file.Path)
	}
	for _, section := range pack.SourceSections {
		add(section.Project, section.Path)
	}
	return len(files)
}

func addContextSourceOption(
	pack ContextPack,
	request ContextRequest,
	option contextSourceOption,
	concerns []contextConcern,
	state contextSourceSelectionState,
) (ContextPack, contextSourceSelectionState, error) {
	if !contextSourceSectionAlreadyPresent(pack, option.section) {
		pack.SourceSections = append(pack.SourceSections, option.section)
	}
	if pack.SourceUnrepresented > 0 {
		pack.SourceUnrepresented--
	}
	state.selectedCandidates[contextSourceCandidateKey(option.candidate)] = true
	for _, factID := range contextSourceCandidateFactIDs(option.candidate) {
		state.selectedFactIDs[factID] = true
	}
	for _, key := range option.concernKeys {
		state.coveredConcerns[key] = true
	}
	state.coveredRoles[option.candidate.Role] = true
	if state.selectedEvidenceFamilies == nil {
		state.selectedEvidenceFamilies = make(map[string]int)
	}
	family := contextSourceEffectiveEvidenceFamily(pack, option)
	countFamily := true
	if family == contextConcernDomainModel || family == contextConcernPersistence {
		countFamily = option.candidate.Role == family
	}
	if countFamily {
		familyKey := option.projectKey + "\x00" + family
		state.selectedEvidenceFamilies[familyKey]++
	}
	if option.projectKey != "" {
		state.selectedProjects[option.projectKey] = true
	}
	applyContextSourceCoverage(&pack, concerns, state.coveredConcerns)
	pack, err := finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, state, err
	}
	fits, err := contextSourcePackFits(pack, request)
	if err != nil {
		return ContextPack{}, state, err
	}
	if !fits {
		return ContextPack{}, state, fmt.Errorf("selected context source option no longer fits the response budget")
	}
	return pack, state, nil
}

func coverableContextSourceProductionPending(
	concerns []contextConcern,
	options []contextSourceOption,
	state contextSourceSelectionState,
) bool {
	for _, concern := range concerns {
		if !concern.required || concern.kind == contextConcernTests || state.coveredConcerns[concern.key] {
			continue
		}
		for _, option := range options {
			if option.candidate.Role != "test" && contextSourceOptionHasConcern(option, concern.key) {
				return true
			}
		}
	}
	return false
}

func contextSourceUtilityOption(
	pack ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
	state contextSourceSelectionState,
	productionPending bool,
) (contextSourceOption, int, bool, error) {
	best := contextSourceOption{}
	bestUtility := 0
	found := false
	fitting, err := fittingContextSourceOptions(pack, request, options, concerns, state)
	if err != nil {
		return contextSourceOption{}, 0, false, err
	}
	for _, option := range fitting {
		if productionPending && option.candidate.Role == "test" {
			continue
		}
		newConcerns := 0
		newProjects := 0
		for _, key := range option.concernKeys {
			if state.coveredConcerns[key] {
				continue
			}
			newConcerns++
			if strings.HasPrefix(key, contextConcernProject+":") {
				newProjects++
			}
		}
		newRoles := 0
		if !state.coveredRoles[option.candidate.Role] {
			newRoles = 1
		}
		connected := 0
		if option.pathDistance <= maximumContextPathHops {
			connected = 1
		}
		family := contextSourceEffectiveEvidenceFamily(pack, option)
		if family == contextConcernDomainModel &&
			option.profiled &&
			!option.requestedModel &&
			!contextSourceOptionAddsConcernKind(option, state, contextConcernDomainModel) {
			continue
		}
		if family == contextConcernPersistence &&
			option.profiled &&
			!option.matchesModel &&
			!contextSourceOptionAddsConcernKind(option, state, contextConcernPersistence) {
			continue
		}
		familyKey := option.projectKey + "\x00" + family
		familyCount := state.selectedEvidenceFamilies[familyKey]
		if newConcerns == 0 && newProjects == 0 && newRoles == 0 {
			if family != contextConcernDomainModel && family != contextConcernPersistence ||
				familyCount >= contextSourceEvidenceFamilyLimit(family) ||
				contextSourceEffectiveStableDomainMatches(pack, option) < 2 ||
				family == contextConcernPersistence && option.profiled && !option.matchesModel {
				continue
			}
		}
		familyBonus := 0
		if familyCount < contextSourceEvidenceFamilyLimit(family) {
			familyBonus = 260
		}
		cost := option.estimated
		if contextSourceSectionAlreadyPresent(pack, option.section) {
			cost = 0
		}
		utility := 1200*newConcerns + 300*newProjects + 150*newRoles + 80*connected +
			familyBonus + contextSourceEffectiveQuality(pack, option) -
			cost - 25*option.pathDistance
		if !found || utility > bestUtility || utility == bestUtility && contextSourceOptionLess(option, best) {
			best, bestUtility, found = option, utility, true
		}
	}
	return best, bestUtility, found, nil
}

func contextSourceOptionAddsConcernKind(
	option contextSourceOption,
	state contextSourceSelectionState,
	kind string,
) bool {
	for _, key := range option.concernKeys {
		if state.coveredConcerns[key] {
			continue
		}
		if key == kind || strings.HasPrefix(key, kind+":") {
			return true
		}
	}
	return false
}

func contextSourceOptionHasConcern(option contextSourceOption, key string) bool {
	for _, candidateKey := range option.concernKeys {
		if candidateKey == key {
			return true
		}
	}
	return false
}

func contextSourceCandidateHasFact(candidate sourceCandidate, factID string) bool {
	for _, candidateID := range contextSourceCandidateFactIDs(candidate) {
		if candidateID == factID {
			return true
		}
	}
	return false
}

func applyContextSourceCoverage(
	pack *ContextPack,
	concerns []contextConcern,
	covered map[string]bool,
) {
	publicCovered := map[string]bool{}
	publicSeen := map[string]bool{}
	requiredMissing := false
	for _, concern := range concerns {
		if !concern.required {
			continue
		}
		publicKey := firstNonEmptyContext(concern.publicKey, concern.key)
		if !publicSeen[publicKey] {
			publicSeen[publicKey] = true
			publicCovered[publicKey] = true
		}
		if !covered[concern.key] {
			publicCovered[publicKey] = false
			requiredMissing = true
		}
	}
	for index := range pack.Concerns {
		key := contextPublicConcernKey(pack.Concerns[index])
		if publicSeen[key] {
			pack.Concerns[index].Covered = publicCovered[key]
		} else {
			pack.Concerns[index].Covered = covered[key]
		}
	}
	pack.SourceUnrepresented = 0
	for _, concern := range pack.Concerns {
		if !concern.Covered {
			pack.SourceUnrepresented++
		}
	}
	switch {
	case len(pack.SourceSections) == 0:
		pack.SourceCoverage = "none"
	case requiredMissing:
		pack.SourceCoverage = "partial"
	default:
		pack.SourceCoverage = "complete"
	}
}

func contextSourceConcernOmission(
	concern contextConcern,
	candidates []sourceCandidate,
	options []contextSourceOption,
	failures map[string]string,
) ContextSourceOmission {
	matching := []sourceCandidate{}
	for _, candidate := range candidates {
		if concern.project != "" &&
			normalizeContextProject(candidate.Project) != concern.project {
			continue
		}
		matches := concern.kind == contextConcernProject && candidate.Role != "test" &&
			normalizeContextProject(candidate.Project) == concern.project
		if !matches {
			for _, factID := range concern.candidateFactIDs {
				if contextSourceCandidateHasFact(candidate, factID) {
					matches = true
					break
				}
			}
		}
		if matches {
			matching = append(matching, candidate)
		}
	}
	if len(matching) == 0 {
		return ContextSourceOmission{
			Project: concern.project, Role: contextSourceConcernRole(concern.kind),
			Reason: "required concern has no indexed source candidate",
		}
	}
	candidate := sourceCandidate{}
	startLine := 0
	endLine := 0
	if option, ok := contextSourceOmissionEvidenceOption(concern, options); ok {
		candidate = option.candidate
		startLine = option.section.StartLine
		endLine = option.section.EndLine
	} else {
		sort.Slice(matching, func(i, j int) bool {
			leftScore := contextSourceOmissionFacetScore(concern, matching[i])
			rightScore := contextSourceOmissionFacetScore(concern, matching[j])
			if leftScore != rightScore {
				return leftScore > rightScore
			}
			return contextSourceCandidateLess(matching[i], matching[j])
		})
		candidate = matching[0]
		startLine = candidate.StartLine
		endLine = candidate.EndLine
	}
	reason := "source section does not fit the response budget"
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if failure := failures[factID]; failure != "" {
			reason = failure
			break
		}
	}
	if startLine > 0 && endLine <= 0 {
		endLine = startLine
	}
	return ContextSourceOmission{
		Project: candidate.Project, Path: candidate.Path,
		StartLine: startLine, EndLine: endLine,
		Role: candidate.Role, Reason: reason,
	}
}

func contextSourceEvidenceOmissions(
	concerns []contextConcern,
	candidates []sourceCandidate,
	failures map[string]string,
	covered map[string]bool,
) []ContextSourceOmission {
	return contextSourceEvidenceOmissionsWithOptions(
		concerns,
		candidates,
		nil,
		failures,
		covered,
	)
}

func contextSourceEvidenceOmissionsWithOptions(
	concerns []contextConcern,
	candidates []sourceCandidate,
	options []contextSourceOption,
	failures map[string]string,
	covered map[string]bool,
) []ContextSourceOmission {
	grouped := map[string]ContextSourceOmission{}
	reasons := map[string][]string{}
	ranks := map[string]int{}
	for _, concern := range concerns {
		if !concern.required || covered[concern.key] {
			continue
		}
		omission := contextSourceConcernOmission(
			concern,
			candidates,
			options,
			failures,
		)
		pathRank := "1"
		if contextPackSourceFile(omission.Path) != "" {
			pathRank = "0"
		}
		key := pathRank + "\x000facet\x00" + normalizeContextProject(omission.Project) + "\x00" +
			contextPackSourceFile(omission.Path) + "\x00" + omission.Role
		if concern.facet == "" {
			key = pathRank + "\x001concern\x00" + normalizeContextProject(omission.Project) + "\x00" +
				contextPackSourceFile(omission.Path) + "\x00" + omission.Role + "\x00" +
				omission.Reason
		}
		if _, exists := grouped[key]; !exists {
			grouped[key] = omission
		}
		ranks[key] = max(ranks[key], concern.rank)
		if concern.facet == "" {
			continue
		}
		reason := strings.TrimSpace(concern.reason)
		if reason == "" {
			reason = omission.Reason
		}
		reasons[key] = append(reasons[key], reason)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool {
		leftPathBound := strings.HasPrefix(keys[left], "0")
		rightPathBound := strings.HasPrefix(keys[right], "0")
		if leftPathBound != rightPathBound {
			return leftPathBound
		}
		if ranks[keys[left]] != ranks[keys[right]] {
			return ranks[keys[left]] > ranks[keys[right]]
		}
		return keys[left] < keys[right]
	})
	result := make([]ContextSourceOmission, 0, min(len(keys), MaxContextSourceOmissions))
	for _, key := range keys {
		omission := grouped[key]
		if len(reasons[key]) > 0 {
			values := orderedContextConcernIDs(reasons[key])
			omission.Reason = "missing evidence: " + strings.Join(values, "; ")
		}
		result = append(result, omission)
		if len(result) == MaxContextSourceOmissions {
			break
		}
	}
	return result
}

func contextSourceOmissionEvidenceOption(
	concern contextConcern,
	options []contextSourceOption,
) (contextSourceOption, bool) {
	matching := make([]contextSourceOption, 0)
	for _, option := range options {
		if concern.project != "" &&
			normalizeContextProject(option.candidate.Project) != concern.project ||
			!contextSourceOptionHasConcern(option, concern.key) {
			continue
		}
		matching = append(matching, option)
	}
	if len(matching) == 0 {
		return contextSourceOption{}, false
	}
	sort.Slice(matching, func(left, right int) bool {
		if matching[left].candidateQuality != matching[right].candidateQuality {
			return matching[left].candidateQuality > matching[right].candidateQuality
		}
		if matching[left].quality != matching[right].quality {
			return matching[left].quality > matching[right].quality
		}
		if matching[left].estimated != matching[right].estimated {
			return matching[left].estimated < matching[right].estimated
		}
		return contextSourceOptionLess(matching[left], matching[right])
	})
	return matching[0], true
}

func contextSourceOmissionFacetScore(
	concern contextConcern,
	candidate sourceCandidate,
) int {
	value := strings.ToLower(strings.Join([]string{
		candidate.Kind,
		candidate.Name,
		candidate.Qualified,
		candidate.Path,
	}, " "))
	tokens := append(
		[]string(nil),
		contextEvidenceFacetVocabulary[concern.kind][concern.facet]...,
	)
	tokens = append(tokens, strings.Split(concern.facet, "_")...)
	score := 0
	for _, token := range orderedContextConcernIDs(tokens) {
		if token != "" && strings.Contains(value, strings.ToLower(token)) {
			score++
		}
	}
	return score
}

func contextSourceConcernRole(kind string) string {
	switch kind {
	case contextConcernEntrypoint:
		return "entrypoint"
	case contextConcernDomainModel:
		return contextConcernDomainModel
	case contextConcernHTTPContract:
		return "contract"
	case contextConcernPersistence:
		return "persistence"
	case contextConcernTests:
		return "test"
	default:
		return "call_chain"
	}
}

func contextSourceCandidateKey(candidate sourceCandidate) string {
	return candidate.Project + "\x00" + candidate.Path + "\x00" + candidate.FactID
}

func contextSourceOptionLess(left, right contextSourceOption) bool {
	if left.candidate.Role != right.candidate.Role {
		return left.candidate.Role < right.candidate.Role
	}
	if left.candidate.Project != right.candidate.Project {
		return left.candidate.Project < right.candidate.Project
	}
	if left.candidate.Path != right.candidate.Path {
		return left.candidate.Path < right.candidate.Path
	}
	if left.section.StartLine != right.section.StartLine {
		return left.section.StartLine < right.section.StartLine
	}
	leftMode, rightMode := contextSourceRenderModeOrder(left.section.RenderMode), contextSourceRenderModeOrder(right.section.RenderMode)
	if leftMode != rightMode {
		return leftMode < rightMode
	}
	return left.candidate.FactID < right.candidate.FactID
}

func contextSourceRenderModeOrder(mode string) int {
	switch mode {
	case "declaration_body":
		return 0
	case "body":
		return 1
	case "focused":
		return 2
	default:
		return 3
	}
}
