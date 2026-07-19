package agent

import (
	"fmt"
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
	candidates := contextSourceCandidates(pack, loaded.Index)
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

	for _, boundary := range mandatoryContextSourceBoundaries(pack, loaded.Index, concerns, options, distances) {
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
	return finalizeContextPackWithinBudget(pack, request)
}

func contextSourceConcerns(pack ContextPack, index scan.AgentContextIndexRecord) []contextConcern {
	planned := []contextConcern(nil)
	if seed, ok := contextConcernPlanningSeed(index, pack.Query); ok {
		planned = planContextConcerns(pack.Query, index, seed)
	}
	plannedByKey := make(map[string]contextConcern, len(planned))
	for _, concern := range planned {
		plannedByKey[concern.key] = concern
	}

	concerns := append([]contextConcern(nil), planned...)
	for _, public := range pack.Concerns {
		key := contextPublicConcernKey(public)
		if _, ok := plannedByKey[key]; ok {
			continue
		}
		concerns = append(concerns, contextSourceConcernFromPack(pack, index, public))
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
			include = contextValueRequestsConcern(strings.Join([]string{fact.Search, fact.Name, fact.Qualified, fact.Summary}, " "), contextConcernAuth)
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
				candidateIDs = append(candidateIDs, edge.FromFactID, edge.ToFactID)
			}
		}
	}
	return newContextConcern(kind, project, true, candidateIDs, public.Reason)
}

func contextSourcePathDistances(pack ContextPack, index scan.AgentContextIndexRecord) map[string]int {
	seedID := ""
	if len(pack.Entrypoints) > 0 {
		seedID = pack.Entrypoints[0].ID
	} else if seed, ok := contextConcernPlanningSeed(index, pack.Query); ok {
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
		candidateOptions := 0
		verifiedFacts := make(map[string]bool)
		for _, mode := range []string{"body", "focused", "signature"} {
			section, renderErr := renderSourceCandidate(candidate, file, mode)
			if renderErr != nil {
				contextSourceRecordFailure(failures, candidate, stableContextSourceOmissionReason(renderErr))
				continue
			}
			verified, rejected := verifiedContextSourceFactIDs(pack, loaded.Index, candidate, file, section)
			for factID, reason := range rejected {
				if _, recorded := failures[factID]; !recorded {
					failures[factID] = reason
				}
			}
			if len(verified) == 0 {
				continue
			}
			for _, factID := range verified {
				verifiedFacts[factID] = true
			}
			optionCandidate := candidate
			optionCandidate.FactIDs = verified
			estimated, estimateErr := EstimateContextTokens(section)
			if estimateErr != nil {
				return nil, nil, estimateErr
			}
			concernKeys, required := contextSourceOptionConcerns(optionCandidate, concerns)
			projectKey := ""
			if optionCandidate.Role != "test" {
				projectKey = normalizeContextProject(optionCandidate.Project)
			}
			options = append(options, contextSourceOption{
				candidate: optionCandidate, section: section, estimated: estimated,
				concernKeys: concernKeys, projectKey: projectKey, required: required,
				pathDistance: contextSourceCandidateDistance(optionCandidate, distances),
			})
			candidateOptions++
		}
		if candidateOptions > 0 {
			for factID := range verifiedFacts {
				delete(failures, factID)
			}
		}
	}
	sort.Slice(options, func(i, j int) bool { return contextSourceOptionLess(options[i], options[j]) })
	return options, failures, nil
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
	pack ContextPack,
	index scan.AgentContextIndexRecord,
	concerns []contextConcern,
	options []contextSourceOption,
	distances map[string]int,
) []contextSourceBoundary {
	boundaries := []contextSourceBoundary{}
	entryID := ""
	entryProject := ""
	if len(pack.Entrypoints) > 0 {
		entryID = pack.Entrypoints[0].ID
		entryProject = normalizeContextProject(pack.Entrypoints[0].Project)
		boundaries = append(boundaries, contextSourceBoundary{factID: entryID})
	}
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
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
		to := factByID[edge.ToFactID]
		if edge.FromFactID == entryID && eligibleContextConcernFact(to) &&
			normalizeContextProject(to.Project) == entryProject {
			boundaries = append(boundaries, contextSourceBoundary{factID: edge.ToFactID})
			break
		}
	}
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
	for _, concern := range concerns {
		if !concern.required || concern.kind != contextConcernProject {
			continue
		}
		boundaries = append(boundaries, contextSourceBoundary{project: concern.project})
	}
	return boundaries
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
	byCandidate := make(map[string][]contextSourceOption)
	for _, option := range options {
		key := contextSourceCandidateKey(option.candidate)
		if state.selectedCandidates[key] {
			continue
		}
		byCandidate[key] = append(byCandidate[key], option)
	}
	preferred := make([]contextSourceOption, 0, len(byCandidate))
	for _, candidateOptions := range byCandidate {
		sort.Slice(candidateOptions, func(i, j int) bool {
			leftMode := contextSourceRenderModeOrder(candidateOptions[i].section.RenderMode)
			rightMode := contextSourceRenderModeOrder(candidateOptions[j].section.RenderMode)
			if leftMode != rightMode {
				return leftMode < rightMode
			}
			return contextSourceOptionLess(candidateOptions[i], candidateOptions[j])
		})
		for _, option := range candidateOptions {
			fits, err := contextSourceOptionFits(pack, request, option, concerns, state)
			if err != nil {
				return nil, err
			}
			if fits {
				preferred = append(preferred, option)
				break
			}
		}
	}
	sort.Slice(preferred, func(i, j int) bool { return contextSourceOptionLess(preferred[i], preferred[j]) })
	return preferred, nil
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
		pack.Concerns[index].Covered = covered[contextPublicConcernKey(pack.Concerns[index])]
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
	if left.candidate.Priority != right.candidate.Priority {
		return left.candidate.Priority < right.candidate.Priority
	}
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
	case "body":
		return 0
	case "focused":
		return 1
	default:
		return 2
	}
}
