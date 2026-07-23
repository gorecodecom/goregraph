package agent

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

type contextSourceOption struct {
	candidate    sourceCandidate
	section      ContextSourceSection
	estimated    int
	concernKeys  []string
	projectKey   string
	required     bool
	pathDistance int
}

type contextSourceSelectionState struct {
	selectedCandidates map[string]bool
	selectedFactIDs    map[string]bool
	selectedProjects   map[string]bool
	coveredConcerns    map[string]bool
	coveredRoles       map[string]bool
}

func selectContextSourceOptions(
	pack ContextPack,
	loaded loadedContextIndex,
	request ContextRequest,
) (ContextPack, error) {
	concerns := contextSourceConcerns(pack, loaded.Index)
	candidates := contextSourceCandidatesForConcerns(pack, loaded.Index, concerns)
	distances := contextSourcePathDistances(pack, loaded.Index)
	options, failures, err := contextSourceRenderOptions(pack, loaded, candidates, concerns, distances)
	if err != nil {
		return ContextPack{}, err
	}

	pack = cloneContextPack(pack)
	pack.SourceSections = nil
	pack.SourceOmissions = nil
	pack.SourceUnrepresented = len(candidates)
	state := contextSourceSelectionState{
		selectedCandidates: make(map[string]bool, len(candidates)),
		selectedFactIDs:    make(map[string]bool, len(candidates)),
		selectedProjects:   make(map[string]bool),
		coveredConcerns:    make(map[string]bool, len(concerns)),
		coveredRoles:       make(map[string]bool),
	}
	applyContextSourceCoverage(&pack, concerns, state.coveredConcerns)
	pack, err = finalizeContextEstimate(pack)
	if err != nil {
		return ContextPack{}, err
	}

	coreBoundaries := contextCoreSourceBoundaries(pack, loaded.Index, distances)
	for _, boundary := range mandatoryContextSourceBoundaries(loaded.Index, concerns, coreBoundaries, distances) {
		if contextSourceBoundaryCovered(boundary, state) {
			continue
		}
		option, ok, selectErr := smallestFittingContextSourceOption(pack, request, options, concerns, state, boundary)
		if selectErr != nil {
			return ContextPack{}, selectErr
		}
		if !ok {
			continue
		}
		pack, state, err = addContextSourceOption(pack, request, option, concerns, state)
		if err != nil {
			return ContextPack{}, err
		}
	}
	pack, err = enrichContextCoreSourceOptions(pack, request, options, state, coreBoundaries)
	if err != nil {
		return ContextPack{}, err
	}

	for len(pack.SourceSections) < MaxContextSourceSections {
		productionPending := coverableContextSourceProductionPending(concerns, options, state)
		best, bestUtility, found, utilityErr := contextSourceUtilityOption(
			pack, request, options, concerns, state, productionPending,
		)
		if utilityErr != nil {
			return ContextPack{}, utilityErr
		}
		if !found || bestUtility <= 0 {
			break
		}
		pack, state, err = addContextSourceOption(pack, request, best, concerns, state)
		if err != nil {
			return ContextPack{}, err
		}
	}
	applyContextSourceCoverage(&pack, concerns, state.coveredConcerns)
	for _, concern := range concerns {
		if !concern.required || state.coveredConcerns[concern.key] || len(pack.SourceOmissions) >= MaxContextSourceOmissions {
			continue
		}
		omission := contextSourceConcernOmission(concern, candidates, failures)
		if contextSourceOmissionContains(pack.SourceOmissions, omission) {
			continue
		}
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
	return finalizeContextPackWithinBudget(pack, request)
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

	concerns := append([]contextConcern(nil), planned...)
	for _, public := range pack.Concerns {
		key := contextPublicConcernKey(public)
		selected := contextSourceConcernFromPack(pack, index, public)
		if plannedIndex, ok := plannedByKey[key]; ok {
			concerns[plannedIndex].candidateFactIDs = orderedContextConcernIDs(append(
				concerns[plannedIndex].candidateFactIDs,
				selected.candidateFactIDs...,
			))
			continue
		}
		concerns = append(concerns, selected)
	}
	sort.Slice(concerns, func(i, j int) bool { return concerns[i].key < concerns[j].key })
	return concerns
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
	options := []contextSourceOption{}
	failures := make(map[string]string)
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
) (int, error) {
	added := 0
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
		concernKeys, required := contextSourceOptionConcerns(optionCandidate, concerns)
		projectKey := ""
		if optionCandidate.Role != "test" {
			projectKey = normalizeContextProject(optionCandidate.Project)
		}
		*options = append(*options, contextSourceOption{
			candidate: optionCandidate, section: section, estimated: estimated,
			concernKeys: concernKeys, projectKey: projectKey, required: required,
			pathDistance: contextSourceCandidateDistance(optionCandidate, distances),
		})
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
		role := contextSourceRole(pack, fact)
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
			StartLine: fact.Line, EndLine: fact.EndLine, Role: contextSourceRole(pack, fact),
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

	var owner scan.AgentContextFactRecord
	matches := 0
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
		owner = fact
		matches++
	}
	if matches != 1 {
		return sourceCandidate{}, false
	}
	result := candidate
	result.Name = firstNonEmptyContext(owner.Name, ownerShort)
	result.Qualified = firstNonEmptyContext(owner.Qualified, ownerQualified)
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

func contextSourceRole(pack ContextPack, fact scan.AgentContextFactRecord) string {
	if contextLocationIDs(pack.Tests)[fact.ID] || normalizedContextConcernKind(fact.Kind) == contextConcernTests || contextFactUsesTestSource(fact) {
		return "test"
	}
	switch {
	case strings.EqualFold(fact.Kind, "api_endpoint") && contextFactMatchesSelectedEndpoint(fact, pack.Endpoints):
		return "entrypoint"
	case contextLocationIDs(pack.Entrypoints)[fact.ID]:
		return "entrypoint"
	case contextLocationIDs(pack.Contracts)[fact.ID] || strings.EqualFold(fact.Kind, "api_contract"):
		return "contract"
	case contextLocationIDs(pack.Persistence)[fact.ID] || normalizedContextConcernKind(fact.Kind) == contextConcernPersistence:
		return "persistence"
	default:
		return "call_chain"
	}
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

func contextSourceOptionConcerns(candidate sourceCandidate, concerns []contextConcern) ([]string, bool) {
	factIDs := make(map[string]bool)
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		factIDs[factID] = true
	}
	keys := []string{}
	required := false
	for _, concern := range concerns {
		covered := false
		for _, factID := range concern.candidateFactIDs {
			if factIDs[factID] {
				covered = true
				break
			}
		}
		if concern.kind == contextConcernProject {
			covered = candidate.Role != "test" && normalizeContextProject(candidate.Project) == concern.project
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

func contextSourceCandidateDistance(candidate sourceCandidate, distances map[string]int) int {
	best := maximumContextPathHops + 1
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if distance, ok := distances[factID]; ok && distance < best {
			best = distance
		}
	}
	return best
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
	return boundaries
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
		if !found || option.estimated < best.estimated ||
			option.estimated == best.estimated && contextSourceOptionLess(option, best) {
			best, found = option, true
		}
	}
	return best, found, nil
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
	if len(pack.SourceSections) >= MaxContextSourceSections {
		return false, nil
	}
	candidate := cloneContextPack(pack)
	candidate.SourceSections = append(candidate.SourceSections, option.section)
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
	pack.SourceSections = append(pack.SourceSections, option.section)
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
		utility := 1200*newConcerns + 300*newProjects + 150*newRoles + 80*connected -
			option.estimated - 25*option.pathDistance
		if !found || utility > bestUtility || utility == bestUtility && contextSourceOptionLess(option, best) {
			best, bestUtility, found = option, utility, true
		}
	}
	return best, bestUtility, found, nil
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
	requiredMissing := false
	for _, concern := range concerns {
		if concern.required && !covered[concern.key] {
			requiredMissing = true
		}
	}
	for index := range pack.Concerns {
		key := contextPublicConcernKey(pack.Concerns[index])
		pack.Concerns[index].Covered = covered[key]
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
	failures map[string]string,
) ContextSourceOmission {
	matching := []sourceCandidate{}
	for _, candidate := range candidates {
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
	sort.Slice(matching, func(i, j int) bool { return contextSourceCandidateLess(matching[i], matching[j]) })
	if len(matching) == 0 {
		return ContextSourceOmission{
			Project: concern.project, Role: contextSourceConcernRole(concern.kind),
			Reason: "required concern has no indexed source candidate",
		}
	}
	candidate := matching[0]
	reason := "source section does not fit the response budget"
	for _, factID := range contextSourceCandidateFactIDs(candidate) {
		if failure := failures[factID]; failure != "" {
			reason = failure
			break
		}
	}
	return ContextSourceOmission{
		Project: candidate.Project, Path: candidate.Path, Role: candidate.Role, Reason: reason,
	}
}

func contextSourceOmissionContains(omissions []ContextSourceOmission, candidate ContextSourceOmission) bool {
	for _, omission := range omissions {
		if omission == candidate {
			return true
		}
	}
	return false
}

func contextSourceConcernRole(kind string) string {
	switch kind {
	case contextConcernEntrypoint:
		return "entrypoint"
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
