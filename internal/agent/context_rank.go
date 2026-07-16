package agent

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	scoreExactRoute           = 1000
	scoreExactQualified       = 900
	scoreExactName            = 800
	scoreAllTerms             = 500
	scorePerMatchedTerm       = 60
	scoreRouteKind            = 80
	scoreSymbolKind           = 60
	scoreTestKind             = 20
	scoreExactConfidence      = 30
	scoreResolvedConfidence   = 15
	minimumContextSeedScore   = 180
	maximumContextSeedFacts   = 3
	maximumContextUncertainty = 3
)

type rankedContextFact struct {
	fact         scan.AgentContextFactRecord
	score        int
	exactClass   int
	matchedTerms int
	allTerms     bool
	reason       string
}

type expandedContextEdge struct {
	edge     scan.AgentContextEdgeRecord
	seedRank int
	neighbor scan.AgentContextFactRecord
}

func compileContextPack(index scan.AgentContextIndexRecord, request ContextRequest) (ContextPack, error) {
	ranked := rankContextFacts(index.Facts, request.Query)
	seeds := selectContextSeeds(ranked)
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

	pack, added, err := tryAddContextLocation(
		pack,
		request,
		top.fact,
		top.reason,
		"entrypoint",
		func(candidate *ContextPack, location ContextLocation) {
			candidate.Entrypoints = append(candidate.Entrypoints, location)
		},
	)
	if err != nil {
		return ContextPack{}, err
	}
	if !added {
		return fallbackContextPack(index, request, "top context fact exceeds the requested budget", nil)
	}
	includedFactIDs[top.fact.ID] = true
	retainedSeeds = append(retainedSeeds, top)

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

	scopes := selectedContextScopes(index.Edges, includedFactIDs, acceptedEdgeIDs, factByID)
	uncertainties, allIncomplete := scopedContextUncertainties(index.Coverage, scopes)
	if pack.Confidence == "LOW" {
		return fallbackContextPack(index, request, "context confidence is low; inspect source directly", uncertainties)
	}
	if allIncomplete {
		return fallbackContextPack(index, request, "all selected context scopes have incomplete coverage", uncertainties)
	}
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
	pack, err = backfillContextEvidence(pack, factByID, request.BudgetTokens)
	if err != nil {
		return ContextPack{}, err
	}
	return finalizeContextEstimate(pack)
}

func rankContextFacts(facts []scan.AgentContextFactRecord, query string) []rankedContextFact {
	queryTokens := contextTokens(query)
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
		exactClass := 0
		score := 0
		reason := "lexical match"
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
		case "route":
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

func selectContextSeeds(ranked []rankedContextFact) []rankedContextFact {
	seeds := make([]rankedContextFact, 0, maximumContextSeedFacts)
	for _, candidate := range ranked {
		if candidate.score < minimumContextSeedScore {
			break
		}
		seeds = append(seeds, candidate)
		if len(seeds) == maximumContextSeedFacts {
			break
		}
	}
	return seeds
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
	for seedRank, seed := range seeds {
		for _, edge := range sortedEdges {
			if edge.FromFactID == edge.ToFactID ||
				edge.FromFactID != seed.fact.ID && edge.ToFactID != seed.fact.ID {
				continue
			}
			key := edge.ID
			if key == "" {
				key = edge.FromFactID + "\x00" + edge.ToFactID + "\x00" + edge.Kind
			}
			if seen[key] {
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
			seen[key] = true
			result = append(result, expandedContextEdge{
				edge: edge, seedRank: seedRank, neighbor: neighbor,
			})
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

func tryAddContextLocation(
	pack ContextPack,
	request ContextRequest,
	fact scan.AgentContextFactRecord,
	reason,
	role string,
	appendLocation func(*ContextPack, ContextLocation),
) (ContextPack, bool, error) {
	compacted := fact
	compacted.EvidenceIDs = nil
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

func backfillContextEvidence(
	pack ContextPack,
	factByID map[string]scan.AgentContextFactRecord,
	budget int,
) (ContextPack, error) {
	sections := []struct {
		length int
		id     func(int) string
		set    func(*ContextPack, int, []string)
	}{
		{
			length: len(pack.Entrypoints),
			id:     func(index int) string { return pack.Entrypoints[index].ID },
			set: func(candidate *ContextPack, index int, evidenceIDs []string) {
				candidate.Entrypoints[index].EvidenceIDs = evidenceIDs
			},
		},
		{
			length: len(pack.Contracts),
			id:     func(index int) string { return pack.Contracts[index].ID },
			set: func(candidate *ContextPack, index int, evidenceIDs []string) {
				candidate.Contracts[index].EvidenceIDs = evidenceIDs
			},
		},
		{
			length: len(pack.Persistence),
			id:     func(index int) string { return pack.Persistence[index].ID },
			set: func(candidate *ContextPack, index int, evidenceIDs []string) {
				candidate.Persistence[index].EvidenceIDs = evidenceIDs
			},
		},
		{
			length: len(pack.Tests),
			id:     func(index int) string { return pack.Tests[index].ID },
			set: func(candidate *ContextPack, index int, evidenceIDs []string) {
				candidate.Tests[index].EvidenceIDs = evidenceIDs
			},
		},
	}
	for _, section := range sections {
		for index := 0; index < section.length; index++ {
			fact, ok := factByID[section.id(index)]
			if !ok {
				continue
			}
			var err error
			pack, err = maximizeContextEvidencePrefix(
				pack,
				budget,
				fact.EvidenceIDs,
				func(candidate *ContextPack, evidenceIDs []string) {
					section.set(candidate, index, evidenceIDs)
				},
			)
			if err != nil {
				return ContextPack{}, err
			}
		}
	}
	return pack, nil
}

func maximizeContextEvidencePrefix(
	pack ContextPack,
	budget int,
	evidenceIDs []string,
	set func(*ContextPack, []string),
) (ContextPack, error) {
	evidenceIDs = sortedContextStrings(evidenceIDs)
	low := 0
	high := len(evidenceIDs)
	best := pack
	for low <= high {
		count := low + (high-low)/2
		candidate := cloneContextPack(pack)
		set(&candidate, append([]string(nil), evidenceIDs[:count]...))
		var err error
		candidate, err = finalizeContextEstimate(candidate)
		if err != nil {
			return ContextPack{}, err
		}
		if candidate.EstimatedTokens <= budget {
			best = candidate
			low = count + 1
			continue
		}
		high = count - 1
	}
	return best, nil
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
		Project: fact.Project, Path: fact.File,
		StartLine: fact.Line, EndLine: endLine,
		Role: role, Reason: reason, Confidence: fact.Confidence,
	}
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
	if candidate.EstimatedTokens > budget {
		return pack, false, nil
	}
	return candidate, true, nil
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
		capability := contextCapabilityForKind(fact.Kind)
		if capability == "" {
			scopes[fact.Project+"\x00"] = true
			continue
		}
		scopes[fact.Project+"\x00"+capability] = true
	}
	for _, edge := range edges {
		if !acceptedEdgeIDs[edge.ID] {
			continue
		}
		capability := contextCapabilityForKind(edge.Kind)
		if capability == "" {
			continue
		}
		project := firstNonEmptyContext(edge.Project, factByID[edge.FromFactID].Project, factByID[edge.ToFactID].Project)
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
