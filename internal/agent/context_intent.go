package agent

import (
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	contextConcernEntrypoint   = "entrypoint"
	contextConcernPrimaryPath  = "primary_path"
	contextConcernProject      = "project"
	contextConcernHTTPContract = "http_contract"
	contextConcernAuth         = "authentication"
	contextConcernPersistence  = "persistence"
	contextConcernTests        = "tests"

	maximumPublicContextConcerns = 8
)

type contextConcern struct {
	key              string
	kind             string
	project          string
	required         bool
	candidateFactIDs []string
	reason           string
}

func planContextConcerns(
	query string,
	index scan.AgentContextIndexRecord,
	seed scan.AgentContextFactRecord,
) []contextConcern {
	reachableFactIDs, reachableEdges := reachableContextConcernEvidence(index, seed.ID)
	concerns := []contextConcern{
		newContextConcern(contextConcernEntrypoint, "", true, []string{seed.ID}, "selected entrypoint"),
		newContextConcern(contextConcernPrimaryPath, "", true, mapContextConcernKeys(reachableFactIDs), "reachable production path"),
	}

	aliases := contextProjectAliases(index.Facts, index.Coverage)
	explicitProjects := contextExplicitProjects(query, aliases)
	semanticQueryTokens := contextProjectSemanticQueryTokens(query, aliases, explicitProjects)
	projects := make([]string, 0, len(explicitProjects))
	for project := range explicitProjects {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	for _, project := range projects {
		candidates := semanticContextProjectFacts(semanticQueryTokens, project, index.Facts)
		reason := "explicit project task match"
		if len(candidates) == 0 {
			coverage, incomplete := strongestIncompleteContextProjectCoverage(project, index.Coverage)
			if !incomplete {
				continue
			}
			reason = strings.ToUpper(strings.TrimSpace(coverage.Coverage))
			if detail := strings.TrimSpace(coverage.Reason); detail != "" {
				reason += " — " + detail
			}
		}
		concerns = append(concerns, newContextConcern(
			contextConcernProject,
			project,
			true,
			candidates,
			reason,
		))
	}

	contractFactIDs := []string{}
	for _, edge := range reachableEdges {
		if normalizedContextConcernKind(edge.Kind) != contextConcernHTTPContract {
			continue
		}
		contractFactIDs = append(contractFactIDs, edge.FromFactID, edge.ToFactID)
	}
	if len(contractFactIDs) > 0 {
		concerns = append(concerns, newContextConcern(
			contextConcernHTTPContract,
			"",
			true,
			contractFactIDs,
			"reachable HTTP contract boundary",
		))
	}

	authCandidates := contextConcernFactCandidates(index.Facts, reachableFactIDs, contextConcernAuth)
	authCandidates = append(authCandidates, contextConcernEdgeCandidates(reachableEdges, contextConcernAuth)...)
	authCandidates = orderedContextConcernIDs(authCandidates)
	if contextQueryRequestsConcern(query, contextConcernAuth) || len(authCandidates) > 0 {
		concerns = append(concerns, newContextConcern(
			contextConcernAuth,
			"",
			true,
			authCandidates,
			"requested or reachable authentication evidence",
		))
	}

	persistenceCandidates := contextConcernFactCandidates(index.Facts, reachableFactIDs, contextConcernPersistence)
	persistenceCandidates = append(
		persistenceCandidates,
		contextConcernEdgeCandidates(reachableEdges, contextConcernPersistence)...,
	)
	persistenceCandidates = orderedContextConcernIDs(persistenceCandidates)
	if contextQueryRequestsConcern(query, contextConcernPersistence) || len(persistenceCandidates) > 0 {
		concerns = append(concerns, newContextConcern(
			contextConcernPersistence,
			"",
			true,
			persistenceCandidates,
			"requested or reachable persistence evidence",
		))
	}

	if contextQueryRequestsConcern(query, contextConcernTests) {
		concerns = append(concerns, newContextConcern(
			contextConcernTests,
			"",
			true,
			contextTestConcernCandidates(index, reachableFactIDs),
			"tests requested by task",
		))
	}

	sort.Slice(concerns, func(i, j int) bool {
		return concerns[i].key < concerns[j].key
	})
	return concerns
}

func newContextConcern(kind, project string, required bool, candidateFactIDs []string, reason string) contextConcern {
	project = normalizeContextProject(project)
	key := kind
	if project != "" {
		key += ":" + project
	}
	return contextConcern{
		key:              key,
		kind:             kind,
		project:          project,
		required:         required,
		candidateFactIDs: orderedContextConcernIDs(candidateFactIDs),
		reason:           strings.TrimSpace(reason),
	}
}

func publicContextConcerns(concerns []contextConcern) []ContextConcern {
	ordered := append([]contextConcern(nil), concerns...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].required != ordered[j].required {
			return ordered[i].required
		}
		return ordered[i].key < ordered[j].key
	})
	if len(ordered) > maximumPublicContextConcerns {
		ordered = ordered[:maximumPublicContextConcerns]
	}

	result := make([]ContextConcern, 0, len(ordered))
	for _, concern := range ordered {
		result = append(result, ContextConcern{
			Kind:    concern.kind,
			Project: concern.project,
			Covered: len(concern.candidateFactIDs) > 0,
			Reason:  concern.reason,
		})
	}
	return result
}

func corePublicContextConcerns(concerns []ContextConcern) []ContextConcern {
	result := make([]ContextConcern, 0, 2)
	for _, concern := range concerns {
		if concern.Kind == contextConcernEntrypoint || concern.Kind == contextConcernPrimaryPath {
			result = append(result, concern)
		}
	}
	return result
}

func reachableContextConcernEvidence(
	index scan.AgentContextIndexRecord,
	seedID string,
) (map[string]bool, []scan.AgentContextEdgeRecord) {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	reachable := map[string]bool{}
	if _, ok := factByID[seedID]; !ok {
		return reachable, nil
	}

	edges := append([]scan.AgentContextEdgeRecord(nil), index.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromFactID != edges[j].FromFactID {
			return edges[i].FromFactID < edges[j].FromFactID
		}
		if edges[i].ToFactID != edges[j].ToFactID {
			return edges[i].ToFactID < edges[j].ToFactID
		}
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		return edges[i].ID < edges[j].ID
	})
	adjacency := make(map[string][]scan.AgentContextEdgeRecord)
	for _, edge := range edges {
		if _, fromExists := factByID[edge.FromFactID]; !fromExists {
			continue
		}
		if _, toExists := factByID[edge.ToFactID]; !toExists {
			continue
		}
		adjacency[edge.FromFactID] = append(adjacency[edge.FromFactID], edge)
	}

	queue := []string{seedID}
	reachable[seedID] = true
	reachableEdges := []scan.AgentContextEdgeRecord{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range adjacency[current] {
			if normalizedContextConcernKind(edge.Kind) == contextConcernTests {
				continue
			}
			reachableEdges = append(reachableEdges, edge)
			if !reachable[edge.ToFactID] {
				reachable[edge.ToFactID] = true
				queue = append(queue, edge.ToFactID)
			}
		}
	}
	return reachable, reachableEdges
}

func semanticContextProjectFacts(
	queryTokens map[string]bool,
	project string,
	facts []scan.AgentContextFactRecord,
) []string {
	result := []string{}
	for _, fact := range facts {
		if normalizeContextProject(fact.Project) != project || !eligibleContextConcernFact(fact) {
			continue
		}
		factTokens := contextTokenSet(strings.Join([]string{
			fact.Search,
			fact.Name,
			fact.Qualified,
			fact.HTTPMethod,
			fact.Path,
			fact.Summary,
		}, " "))
		for token := range queryTokens {
			if factTokens[token] {
				result = append(result, fact.ID)
				break
			}
		}
	}
	return orderedContextConcernIDs(result)
}

func contextProjectSemanticQueryTokens(
	query string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
) map[string]bool {
	queryTokens := contextTokenSet(query)
	for project := range explicitProjects {
		for _, alias := range aliases[project] {
			for token := range contextTokenSet(alias) {
				delete(queryTokens, token)
			}
		}
	}
	return queryTokens
}

func eligibleContextConcernFact(fact scan.AgentContextFactRecord) bool {
	kind := strings.ToLower(strings.TrimSpace(fact.Kind))
	return kind != "test" && !strings.Contains(kind, "generated") &&
		!strings.Contains(kind, "metadata") && !contextFactUsesTestSource(fact)
}

func contextConcernFactCandidates(
	facts []scan.AgentContextFactRecord,
	reachable map[string]bool,
	kind string,
) []string {
	candidates := []string{}
	for _, fact := range facts {
		if !reachable[fact.ID] {
			continue
		}
		factKind := normalizedContextConcernKind(fact.Kind)
		if factKind == kind || contextValueRequestsConcern(strings.Join([]string{
			fact.Search, fact.Name, fact.Qualified, fact.Summary,
		}, " "), kind) {
			candidates = append(candidates, fact.ID)
		}
	}
	return orderedContextConcernIDs(candidates)
}

func contextConcernEdgeCandidates(edges []scan.AgentContextEdgeRecord, kind string) []string {
	candidates := []string{}
	for _, edge := range edges {
		if normalizedContextConcernKind(edge.Kind) == kind {
			candidates = append(candidates, edge.ToFactID)
		}
	}
	return orderedContextConcernIDs(candidates)
}

func contextTestConcernCandidates(index scan.AgentContextIndexRecord, reachable map[string]bool) []string {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	candidates := []string{}
	for _, fact := range index.Facts {
		if reachable[fact.ID] && normalizedContextConcernKind(fact.Kind) == contextConcernTests {
			candidates = append(candidates, fact.ID)
		}
	}
	for _, edge := range index.Edges {
		if normalizedContextConcernKind(edge.Kind) != contextConcernTests || !reachable[edge.ToFactID] {
			continue
		}
		if fact, ok := factByID[edge.FromFactID]; ok && normalizedContextConcernKind(fact.Kind) == contextConcernTests {
			candidates = append(candidates, fact.ID)
		}
	}
	return orderedContextConcernIDs(candidates)
}

func contextQueryRequestsConcern(query, kind string) bool {
	return contextValueRequestsConcern(query, kind)
}

func contextValueRequestsConcern(value, kind string) bool {
	tokens := contextTokenSet(value)
	for _, token := range contextConcernVocabulary[kind] {
		if tokens[token] {
			return true
		}
	}
	return false
}

var contextConcernVocabulary = map[string][]string{
	contextConcernAuth: {
		"auth", "authenticate", "authentication", "authorization", "credential", "credentials", "security",
	},
	contextConcernPersistence: {
		"database", "persistence", "persistent", "repositories", "repository", "storage", "store",
	},
	contextConcernTests: {
		"spec", "specs", "test", "testing", "tests",
	},
}

func normalizedContextConcernKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "http_contract":
		return contextConcernHTTPContract
	case "authentication", "requires_auth", "security":
		return contextConcernAuth
	case "persistence":
		return contextConcernPersistence
	case "test", "test_target":
		return contextConcernTests
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func mapContextConcernKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return orderedContextConcernIDs(result)
}

func orderedContextConcernIDs(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	write := 0
	for _, value := range result {
		value = strings.TrimSpace(value)
		if value == "" || write > 0 && result[write-1] == value {
			continue
		}
		result[write] = value
		write++
	}
	return result[:write]
}

func contextConcernPlanningSeed(index scan.AgentContextIndexRecord, query string) (scan.AgentContextFactRecord, bool) {
	ranked := rankContextFacts(index.Facts, query)
	seeds := selectContextSeeds(ranked)
	endpoint, hasEndpoint, _ := selectContextEndpoint(ranked, query)
	if hasEndpoint {
		return endpoint.fact, true
	}
	if len(seeds) == 0 {
		return scan.AgentContextFactRecord{}, false
	}
	return seeds[0].fact, true
}
