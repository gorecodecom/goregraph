package agent

import (
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	maximumContextPathHops     = 7
	maximumContextVisitedFacts = 256
	maximumContextPaths        = 8
	maximumContextEdgesPerNode = 24
)

var contextTraversalCost = map[string]int{
	"call":              1,
	"http_contract":     1,
	"persistence":       1,
	"use":               2,
	"implements":        2,
	"extends":           2,
	"consumes_endpoint": 2,
	"requires_auth":     2,
	"test_target":       3,
}

type contextSelectedPath struct {
	factIDs []string
	edgeIDs []string
	cost    int
	key     string
}

type contextPathSelection struct {
	factIDs                  []string
	edgeIDs                  []string
	distances                map[string]int
	concernCoverage          map[string]bool
	relatedProductionFactIDs []string
	relatedProductionFacts   []scan.AgentContextFactRecord
	paths                    []contextSelectedPath
}

type contextTraversalStep struct {
	edge   scan.AgentContextEdgeRecord
	nextID string
}

type contextTraversalState struct {
	factIDs []string
	edges   []scan.AgentContextEdgeRecord
	cost    int
	key     string
}

func selectContextPaths(
	index scan.AgentContextIndexRecord,
	seed rankedContextFact,
	concerns []contextConcern,
) contextPathSelection {
	selection := contextPathSelection{
		distances:       map[string]int{seed.fact.ID: 0},
		concernCoverage: make(map[string]bool, len(concerns)),
	}
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	if _, ok := factByID[seed.fact.ID]; !ok || !reliableProductionContextSeed(seed.fact) {
		return selection
	}

	testsRequired := contextPathTestsRequired(concerns)
	adjacency := contextPathAdjacency(index.Edges, factByID, testsRequired)
	candidates, reachable := enumerateContextPathCandidates(seed.fact.ID, adjacency)
	lexicalScores := contextPathLexicalScores(index.Facts, seed.query)
	covered := contextPathCoveredConcerns([]string{seed.fact.ID}, concerns)
	delete(covered, contextConcernPrimaryPath)
	boundarySelected := false
	selectedFacts := map[string]bool{seed.fact.ID: true}
	selectedEdges := map[string]bool{}

	for len(selection.paths) < maximumContextPaths {
		bestIndex := -1
		bestScore := 0
		for candidateIndex, candidate := range candidates {
			if !contextPathAddsFact(candidate, selectedFacts) {
				continue
			}
			score, meaningful := scoreContextPath(
				candidate,
				concerns,
				covered,
				boundarySelected,
				lexicalScores,
				factByID,
			)
			if !meaningful || score <= 0 {
				continue
			}
			if bestIndex < 0 || score > bestScore ||
				score == bestScore && candidate.key < candidates[bestIndex].key {
				bestIndex = candidateIndex
				bestScore = score
			}
		}
		if bestIndex < 0 {
			break
		}

		selected := candidates[bestIndex]
		selection.paths = append(selection.paths, contextSelectedPath{
			factIDs: append([]string(nil), selected.factIDs...),
			edgeIDs: contextPathEdgeIDs(selected.edges),
			cost:    selected.cost,
			key:     selected.key,
		})
		for distance, factID := range selected.factIDs {
			selectedFacts[factID] = true
			if existing, ok := selection.distances[factID]; !ok || distance < existing {
				selection.distances[factID] = distance
			}
		}
		for _, edge := range selected.edges {
			selectedEdges[contextPathEdgeIdentity(edge)] = true
		}
		for key := range contextPathCoveredConcerns(selected.factIDs[1:], concerns) {
			covered[key] = true
		}
		if contextPathCrossesContractBoundary(selected, factByID) {
			boundarySelected = true
		}
	}

	selection.relatedProductionFacts = selectRelatedContextProduction(
		index,
		seed.query,
		selectedFacts,
		reachable,
	)
	for _, fact := range selection.relatedProductionFacts {
		selection.relatedProductionFactIDs = append(selection.relatedProductionFactIDs, fact.ID)
	}
	sort.Strings(selection.relatedProductionFactIDs)
	selection.factIDs = contextPathFactIDs(selectedFacts)
	for key := range contextPathCoveredConcerns(selection.factIDs, concerns) {
		covered[key] = true
	}
	if len(selection.paths) == 0 {
		delete(covered, contextConcernPrimaryPath)
	}
	for _, concern := range concerns {
		selection.concernCoverage[concern.key] = covered[concern.key]
	}
	selection.edgeIDs = contextPathSelectedEdgeIDs(index.Edges, selectedEdges)
	return selection
}

func contextPathTestsRequired(concerns []contextConcern) bool {
	for _, concern := range concerns {
		if concern.required && concern.kind == contextConcernTests {
			return true
		}
	}
	return false
}

func contextPathAdjacency(
	edges []scan.AgentContextEdgeRecord,
	factByID map[string]scan.AgentContextFactRecord,
	testsRequired bool,
) map[string][]contextTraversalStep {
	adjacency := make(map[string][]contextTraversalStep)
	for _, edge := range edges {
		kind := strings.ToLower(strings.TrimSpace(edge.Kind))
		if _, ok := contextTraversalCost[kind]; !ok || edge.FromFactID == edge.ToFactID {
			continue
		}
		from, fromExists := factByID[edge.FromFactID]
		to, toExists := factByID[edge.ToFactID]
		if !fromExists || !toExists || !eligibleContextPathFact(from, testsRequired) ||
			!eligibleContextPathFact(to, testsRequired) {
			continue
		}
		currentID, nextID := edge.FromFactID, edge.ToFactID
		if kind == "test_target" {
			if !testsRequired {
				continue
			}
			currentID, nextID = edge.ToFactID, edge.FromFactID
		}
		adjacency[currentID] = append(adjacency[currentID], contextTraversalStep{edge: edge, nextID: nextID})
	}
	for factID := range adjacency {
		sort.Slice(adjacency[factID], func(i, j int) bool {
			left, right := adjacency[factID][i], adjacency[factID][j]
			leftCost := contextTraversalCost[strings.ToLower(strings.TrimSpace(left.edge.Kind))]
			rightCost := contextTraversalCost[strings.ToLower(strings.TrimSpace(right.edge.Kind))]
			if leftCost != rightCost {
				return leftCost < rightCost
			}
			if contextEdgeLess(left.edge, right.edge) {
				return true
			}
			if contextEdgeLess(right.edge, left.edge) {
				return false
			}
			return left.nextID < right.nextID
		})
		if len(adjacency[factID]) > maximumContextEdgesPerNode {
			adjacency[factID] = adjacency[factID][:maximumContextEdgesPerNode]
		}
	}
	return adjacency
}

func eligibleContextPathFact(fact scan.AgentContextFactRecord, testsRequired bool) bool {
	kind := strings.ToLower(strings.TrimSpace(fact.Kind))
	if strings.Contains(kind, "generated") || strings.Contains(kind, "metadata") ||
		contextFactUsesGeneratedMetadata(fact) {
		return false
	}
	isTest := normalizedContextConcernKind(kind) == contextConcernTests || contextFactUsesTestSource(fact)
	return testsRequired || !isTest
}

func enumerateContextPathCandidates(
	seedID string,
	adjacency map[string][]contextTraversalStep,
) ([]contextTraversalState, map[string]bool) {
	frontier := []contextTraversalState{{factIDs: []string{seedID}, key: seedID}}
	visited := map[string]bool{}
	candidates := []contextTraversalState{}
	for len(frontier) > 0 && len(visited) < maximumContextVisitedFacts {
		sort.Slice(frontier, func(i, j int) bool {
			left, right := frontier[i], frontier[j]
			if left.cost != right.cost {
				return left.cost < right.cost
			}
			if len(left.edges) != len(right.edges) {
				return len(left.edges) < len(right.edges)
			}
			return left.key < right.key
		})
		current := frontier[0]
		frontier = frontier[1:]
		currentID := current.factIDs[len(current.factIDs)-1]
		if visited[currentID] {
			continue
		}
		visited[currentID] = true
		if len(current.edges) > 0 {
			candidates = append(candidates, current)
		}
		if len(current.edges) >= maximumContextPathHops {
			continue
		}
		for _, step := range adjacency[currentID] {
			if contextPathContainsFact(current.factIDs, step.nextID) {
				continue
			}
			kind := strings.ToLower(strings.TrimSpace(step.edge.Kind))
			frontier = append(frontier, contextTraversalState{
				factIDs: append(append([]string(nil), current.factIDs...), step.nextID),
				edges:   append(append([]scan.AgentContextEdgeRecord(nil), current.edges...), step.edge),
				cost:    current.cost + contextTraversalCost[kind],
				key:     current.key + "\x00" + contextPathEdgeIdentity(step.edge) + "\x00" + step.nextID,
			})
		}
	}
	return candidates, visited
}

func contextPathLexicalScores(facts []scan.AgentContextFactRecord, query string) map[string]int {
	scores := make(map[string]int, len(facts))
	for _, candidate := range rankContextFacts(facts, query) {
		scores[candidate.fact.ID] = candidate.score
	}
	return scores
}

func scoreContextPath(
	path contextTraversalState,
	concerns []contextConcern,
	covered map[string]bool,
	boundarySelected bool,
	lexicalScores map[string]int,
	factByID map[string]scan.AgentContextFactRecord,
) (int, bool) {
	pathCovered := contextPathCoveredConcerns(path.factIDs[1:], concerns)
	newConcerns := 0
	newProjects := 0
	for _, concern := range concerns {
		if !concern.required || covered[concern.key] || !pathCovered[concern.key] {
			continue
		}
		newConcerns++
		if concern.kind == contextConcernProject {
			newProjects++
		}
	}
	newBoundary := !boundarySelected && contextPathCrossesContractBoundary(path, factByID)
	terminalScore := lexicalScores[path.factIDs[len(path.factIDs)-1]]
	meaningful := newConcerns > 0 || newProjects > 0 || newBoundary || terminalScore >= minimumContextSeedScore
	score := 1000*newConcerns + 300*newProjects + terminalScore - 40*len(path.edges) - path.cost
	if newBoundary {
		score += 200
	}
	return score, meaningful
}

func contextPathCoveredConcerns(factIDs []string, concerns []contextConcern) map[string]bool {
	selected := make(map[string]bool, len(factIDs))
	for _, factID := range factIDs {
		selected[factID] = true
	}
	covered := make(map[string]bool, len(concerns))
	for _, concern := range concerns {
		for _, candidateID := range concern.candidateFactIDs {
			if selected[candidateID] {
				covered[concern.key] = true
				break
			}
		}
	}
	return covered
}

func contextPathCrossesContractBoundary(
	path contextTraversalState,
	factByID map[string]scan.AgentContextFactRecord,
) bool {
	for _, edge := range path.edges {
		if strings.ToLower(strings.TrimSpace(edge.Kind)) != contextConcernHTTPContract {
			continue
		}
		fromProject := normalizeContextProject(factByID[edge.FromFactID].Project)
		toProject := normalizeContextProject(factByID[edge.ToFactID].Project)
		if fromProject != "" && toProject != "" && fromProject != toProject {
			return true
		}
	}
	return false
}

func selectRelatedContextProduction(
	index scan.AgentContextIndexRecord,
	query string,
	selectedFacts map[string]bool,
	reachable map[string]bool,
) []scan.AgentContextFactRecord {
	aliases := contextProjectAliases(index.Facts, index.Coverage)
	explicitProjects := contextExplicitProjects(query, aliases)
	representedProjects := make(map[string]bool)
	for factID := range selectedFacts {
		if project := normalizeContextProject(contextPathFactByID(index.Facts, factID).Project); project != "" {
			representedProjects[project] = true
		}
	}
	result := []scan.AgentContextFactRecord{}
	for _, support := range selectContextSupportFacts(
		rankContextSupportFacts(index.Facts, query, aliases, explicitProjects),
		representedProjects,
	) {
		if reachable[support.fact.ID] {
			continue
		}
		result = append(result, support.fact)
	}
	return result
}

func contextPathFactByID(facts []scan.AgentContextFactRecord, id string) scan.AgentContextFactRecord {
	for _, fact := range facts {
		if fact.ID == id {
			return fact
		}
	}
	return scan.AgentContextFactRecord{}
}

func contextPathAddsFact(path contextTraversalState, selected map[string]bool) bool {
	for _, factID := range path.factIDs {
		if !selected[factID] {
			return true
		}
	}
	return false
}

func contextPathContainsFact(factIDs []string, factID string) bool {
	for _, existing := range factIDs {
		if existing == factID {
			return true
		}
	}
	return false
}

func contextPathEdgeIdentity(edge scan.AgentContextEdgeRecord) string {
	if edge.ID != "" {
		return edge.ID
	}
	return strings.Join([]string{edge.FromFactID, edge.ToFactID, edge.Kind}, "\x00")
}

func contextPathEdgeIDs(edges []scan.AgentContextEdgeRecord) []string {
	ids := make([]string, 0, len(edges))
	for _, edge := range edges {
		ids = append(ids, contextPathEdgeIdentity(edge))
	}
	return ids
}

func contextPathFactIDs(selected map[string]bool) []string {
	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func contextPathSelectedEdgeIDs(
	edges []scan.AgentContextEdgeRecord,
	selected map[string]bool,
) []string {
	ids := make([]string, 0, len(selected))
	for _, edge := range edges {
		id := contextPathEdgeIdentity(edge)
		if selected[id] {
			ids = append(ids, id)
			delete(selected, id)
		}
	}
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
