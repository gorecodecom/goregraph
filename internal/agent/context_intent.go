package agent

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	contextConcernEntrypoint    = "entrypoint"
	contextConcernPrimaryPath   = "primary_path"
	contextConcernProject       = "project"
	contextConcernDomainModel   = "domain_model"
	contextConcernHTTPContract  = "http_contract"
	contextConcernAuth          = "authentication"
	contextConcernConfiguration = "configuration"
	contextConcernResilience    = "resilience"
	contextConcernPersistence   = "persistence"
	contextConcernSideEffects   = "side_effects"
	contextConcernTests         = "tests"

	maximumPublicContextConcerns = 14
)

type contextConcern struct {
	key              string
	publicKey        string
	facet            string
	kind             string
	project          string
	required         bool
	candidateFactIDs []string
	reason           string
	rank             int
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
	if contextQueryRequestsConcern(query, contextConcernDomainModel) {
		candidates := contextDomainModelConcernCandidates(
			query,
			aliases,
			explicitProjects,
			index.Facts,
		)
		if len(candidates) > 0 {
			concerns = append(concerns, newContextConcern(
				contextConcernDomainModel,
				"",
				true,
				candidates,
				"requested domain types and lookup attributes",
			))
		}
	}

	scopedConcernKinds := map[string]bool{}
	if len(explicitProjects) > 0 {
		for _, project := range projects {
			for _, kind := range []string{
				contextConcernHTTPContract,
				contextConcernAuth,
				contextConcernConfiguration,
				contextConcernResilience,
				contextConcernPersistence,
				contextConcernSideEffects,
				contextConcernTests,
			} {
				if !contextQueryRequestsConcern(query, kind) {
					continue
				}
				candidates := contextExplicitProjectConcernCandidates(
					contextExpandedTokenSet(query),
					project,
					kind,
					index.Facts,
					reachableFactIDs,
				)
				if len(candidates) == 0 {
					continue
				}
				concern := newContextConcern(
					kind,
					project,
					true,
					candidates,
					"requested adjacent "+strings.ReplaceAll(kind, "_", " ")+" evidence",
				)
				concern.rank = contextScopedConcernRank(
					query,
					project,
					kind,
					candidates,
					index.Facts,
				)
				concerns = append(concerns, concern)
				scopedConcernKinds[kind] = true
			}
		}
	}

	contractFactIDs := []string{}
	for _, edge := range reachableEdges {
		if normalizedContextConcernKind(edge.Kind) != contextConcernHTTPContract {
			continue
		}
		contractFactIDs = append(contractFactIDs, edge.FromFactID, edge.ToFactID)
	}
	if !scopedConcernKinds[contextConcernHTTPContract] &&
		(contextQueryRequestsConcern(query, contextConcernHTTPContract) || len(contractFactIDs) > 0) {
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
	if !scopedConcernKinds[contextConcernAuth] &&
		(contextQueryRequestsConcern(query, contextConcernAuth) || len(authCandidates) > 0) {
		concerns = append(concerns, newContextConcern(
			contextConcernAuth,
			"",
			true,
			authCandidates,
			"requested or reachable authentication evidence",
		))
	}

	for _, kind := range []string{
		contextConcernConfiguration,
		contextConcernResilience,
		contextConcernSideEffects,
	} {
		if scopedConcernKinds[kind] {
			continue
		}
		candidates := contextConcernFactCandidates(index.Facts, reachableFactIDs, kind)
		if !contextQueryRequestsConcern(query, kind) && len(candidates) == 0 {
			continue
		}
		concerns = append(concerns, newContextConcern(
			kind,
			"",
			true,
			candidates,
			"requested or reachable "+strings.ReplaceAll(kind, "_", " ")+" evidence",
		))
	}

	persistenceCandidates := contextConcernFactCandidates(index.Facts, reachableFactIDs, contextConcernPersistence)
	persistenceCandidates = append(
		persistenceCandidates,
		contextConcernEdgeCandidates(reachableEdges, contextConcernPersistence)...,
	)
	persistenceCandidates = orderedContextConcernIDs(persistenceCandidates)
	if !scopedConcernKinds[contextConcernPersistence] &&
		(contextQueryRequestsConcern(query, contextConcernPersistence) || len(persistenceCandidates) > 0) {
		concerns = append(concerns, newContextConcern(
			contextConcernPersistence,
			"",
			true,
			persistenceCandidates,
			"requested or reachable persistence evidence",
		))
	}

	if !scopedConcernKinds[contextConcernTests] && contextQueryRequestsConcern(query, contextConcernTests) {
		concerns = append(concerns, newContextConcern(
			contextConcernTests,
			"",
			true,
			contextTestConcernCandidates(index, reachableFactIDs),
			"tests requested by task",
		))
	}

	sort.Slice(concerns, func(i, j int) bool {
		return contextConcernLess(concerns[i], concerns[j])
	})
	return concerns
}

func contextConcernLess(left, right contextConcern) bool {
	leftPriority := contextConcernPriority(left)
	rightPriority := contextConcernPriority(right)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if left.kind == right.kind && left.rank != right.rank {
		return left.rank > right.rank
	}
	return left.key < right.key
}

func contextScopedConcernRank(
	query string,
	project string,
	kind string,
	candidateFactIDs []string,
	facts []scan.AgentContextFactRecord,
) int {
	domainTokens := make(map[string]bool)
	for token := range contextConcernDomainQueryTokens(contextExpandedTokenSet(query)) {
		domainTokens[token] = true
	}
	for token := range contextTokenSet(project) {
		delete(domainTokens, token)
	}
	candidates := make(map[string]bool, len(candidateFactIDs))
	for _, factID := range candidateFactIDs {
		candidates[factID] = true
	}
	best := 0
	sourceIndex := scan.AgentContextIndexRecord{Facts: facts}
	for _, fact := range facts {
		if !candidates[fact.ID] {
			continue
		}
		score := 100 * contextStableFactIdentityMatchCount(fact, domainTokens)
		if kind == contextConcernPersistence &&
			contextPersistenceMatchesPrimaryDomainModel(sourceIndex, fact, domainTokens) {
			score += 1000
		}
		if normalizedContextConcernKind(fact.Kind) == kind {
			score += 30
		}
		score += contextConcernFactShapeScore(fact, kind)
		score += 10 * contextDomainModelConfidenceScore(fact.Confidence)
		if score > best {
			best = score
		}
	}
	return best
}

func contextConcernPriority(concern contextConcern) int {
	switch concern.kind {
	case contextConcernEntrypoint:
		return 0
	case contextConcernPrimaryPath:
		return 1
	case contextConcernProject:
		return 2
	default:
		return 3
	}
}

func newContextConcern(kind, project string, required bool, candidateFactIDs []string, reason string) contextConcern {
	project = normalizeContextProject(project)
	key := kind
	if project != "" {
		key += ":" + project
	}
	return contextConcern{
		key:              key,
		publicKey:        key,
		kind:             kind,
		project:          project,
		required:         required,
		candidateFactIDs: orderedContextConcernIDs(candidateFactIDs),
		reason:           strings.TrimSpace(reason),
	}
}

func newContextEvidenceConcern(
	base contextConcern,
	facet string,
	candidateFactIDs []string,
	reason string,
) contextConcern {
	base.publicKey = firstNonEmptyContext(base.publicKey, base.key)
	base.facet = strings.TrimSpace(facet)
	base.key = base.publicKey + "#" + base.facet
	base.candidateFactIDs = orderedContextConcernIDs(candidateFactIDs)
	base.reason = strings.TrimSpace(reason)
	return base
}

var contextEvidenceFacetVocabulary = map[string]map[string][]string{
	contextConcernConfiguration: {
		"binding":  {"config", "configuration", "konfiguration", "properties"},
		"consumer": {"client", "consumer", "vertrag", "contract"},
	},
	contextConcernResilience: {
		"retry_policy": {"retry", "wiederholung"},
		"recovery":     {"error", "exception", "fehler", "fehlerbehandlung", "recover", "recovery"},
	},
	contextConcernSideEffects: {
		"mail":             {"email", "mail", "mails"},
		"audit":            {"audit", "logging", "protocol", "protokollierung", "tracking"},
		"user_information": {"benutzerinformation", "benutzerinformationen", "user_information", "userinfo"},
	},
}

func contextEvidenceFacetRequested(
	kind string,
	facet string,
	queryTokens map[string]bool,
) bool {
	for _, token := range contextEvidenceFacetVocabulary[kind][facet] {
		if queryTokens[token] {
			return true
		}
	}
	return kind == contextConcernSideEffects &&
		facet == "user_information" &&
		queryTokens["user"] &&
		queryTokens["information"]
}

func publicContextConcerns(concerns []contextConcern) []ContextConcern {
	ordered := append([]contextConcern(nil), concerns...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].required != ordered[j].required {
			return ordered[i].required
		}
		return contextConcernLess(ordered[i], ordered[j])
	})
	selected := make([]contextConcern, 0, min(len(ordered), maximumPublicContextConcerns))
	selectedKeys := make(map[string]bool, maximumPublicContextConcerns)
	appendConcern := func(concern contextConcern) {
		publicKey := firstNonEmptyContext(concern.publicKey, concern.key)
		if len(selected) >= maximumPublicContextConcerns || selectedKeys[publicKey] {
			return
		}
		concern.key = publicKey
		concern.publicKey = publicKey
		concern.facet = ""
		selected = append(selected, concern)
		selectedKeys[publicKey] = true
	}
	for _, concern := range ordered {
		if contextConcernPriority(concern) <= 2 {
			appendConcern(concern)
		}
	}
	for _, kind := range []string{
		contextConcernDomainModel,
		contextConcernHTTPContract,
		contextConcernAuth,
		contextConcernConfiguration,
		contextConcernResilience,
		contextConcernPersistence,
		contextConcernSideEffects,
		contextConcernTests,
	} {
		for _, concern := range ordered {
			if concern.kind == kind {
				appendConcern(concern)
				break
			}
		}
	}
	for _, concern := range ordered {
		appendConcern(concern)
	}

	result := make([]ContextConcern, 0, len(selected))
	for _, concern := range selected {
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

func contextExplicitProjectConcernCandidates(
	queryTokens map[string]bool,
	project string,
	kind string,
	facts []scan.AgentContextFactRecord,
	reachable map[string]bool,
) []string {
	result := []string{}
	domainTokens := make(map[string]bool)
	for token := range contextConcernDomainQueryTokens(queryTokens) {
		domainTokens[token] = true
	}
	for token := range contextTokenSet(project) {
		delete(domainTokens, token)
	}
	for _, fact := range facts {
		if normalizeContextProject(fact.Project) != project || reachable[fact.ID] {
			continue
		}
		factKind := normalizedContextConcernKind(fact.Kind)
		if kind == contextConcernTests {
			if factKind != contextConcernTests {
				continue
			}
		} else {
			if !eligibleContextConcernFact(fact) {
				continue
			}
			value := strings.Join([]string{
				fact.Search,
				fact.Name,
				fact.Qualified,
				fact.HTTPMethod,
				fact.Path,
				fact.Summary,
			}, " ")
			matchesConcern := factKind == kind || contextValueRequestsConcern(value, kind)
			if !matchesConcern &&
				kind == contextConcernSideEffects &&
				contextConcernActionAligned(queryTokens, fact) {
				matchesConcern = true
			}
			if !matchesConcern {
				continue
			}
		}
		factTokens := contextExpandedTokenSet(strings.Join([]string{
			fact.Search,
			fact.Name,
			fact.Qualified,
			fact.HTTPMethod,
			fact.Path,
			fact.Summary,
		}, " "))
		for token := range domainTokens {
			if factTokens[token] {
				result = append(result, fact.ID)
				break
			}
		}
	}
	return orderedContextConcernIDs(result)
}

func contextConcernActionAligned(
	queryTokens map[string]bool,
	fact scan.AgentContextFactRecord,
) bool {
	requested := contextActionFamilies(strings.Join(mapContextConcernKeys(queryTokens), " "), "")
	candidate := contextActionFamilies(strings.Join([]string{
		fact.Name,
		fact.Qualified,
		fact.HTTPMethod,
		fact.Path,
	}, " "), fact.HTTPMethod)
	return len(requested) > 0 && contextActionFamiliesOverlap(requested, candidate)
}

func contextConcernFactShapeScore(
	fact scan.AgentContextFactRecord,
	kind string,
) int {
	if kind != contextConcernConfiguration {
		return 0
	}
	if _, _, accessor := javaGeneratedAccessorFieldName(fact.Name); accessor {
		return -80
	}
	name := compactContextIdentifier(fact.Name)
	for _, suffix := range []string{"config", "configuration", "properties"} {
		if strings.HasSuffix(name, suffix) {
			return 120
		}
	}
	return 0
}

func contextConcernDomainQueryTokens(queryTokens map[string]bool) map[string]bool {
	result := make(map[string]bool, len(queryTokens))
	for token := range queryTokens {
		result[token] = true
	}
	for _, tokens := range contextConcernVocabulary {
		for _, vocabularyToken := range tokens {
			if strings.HasPrefix(vocabularyToken, "task_") {
				continue
			}
			for token := range contextExpandedTokenSet(vocabularyToken) {
				delete(result, token)
			}
		}
	}
	if len(result) == 0 {
		return queryTokens
	}
	return result
}

func contextProjectDomainQueryTokens(
	query string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
) map[string]bool {
	result := contextConcernDomainQueryTokens(contextExpandedTokenSet(query))
	for project := range explicitProjects {
		for _, alias := range aliases[project] {
			for token := range contextExpandedTokenSet(alias) {
				delete(result, token)
			}
		}
	}
	return result
}

func contextDomainModelQueryTokens(
	query string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
) map[string]bool {
	result := contextExpandedTokenSet(query)
	for _, vocabulary := range contextConcernVocabulary {
		for _, token := range vocabulary {
			delete(result, token)
		}
	}
	for project := range explicitProjects {
		for _, alias := range aliases[project] {
			for token := range contextTokenSet(alias) {
				delete(result, token)
			}
		}
	}
	for _, generic := range []string{
		"analyze", "analysis", "and", "attribute", "attributes", "cover", "delete", "entity", "entities",
		"field", "fields", "identifier", "identifiers", "lookup", "model", "models", "plan",
		"remove", "service", "services", "task_type", "task_types", "the", "through", "type", "types",
	} {
		delete(result, generic)
	}
	return result
}

var contextDomainModelSuffixes = []string{
	"dto", "entity", "model", "payload", "projection", "record", "request", "response",
}

func contextDomainModelFact(
	fact scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) bool {
	if contextFactUsesTestSource(fact) || contextPackSourceFile(fact.File) == "" || len(domainTokens) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(fact.Kind)) {
	case "symbol", "class", "interface", "record", "struct", "type":
	default:
		return false
	}
	identity := compactContextIdentifier(firstNonEmptyContext(fact.Qualified, fact.Name))
	modelShape := false
	for _, suffix := range contextDomainModelSuffixes {
		if strings.HasSuffix(identity, suffix) {
			modelShape = true
			break
		}
	}
	return modelShape && contextStableFactIdentityMatchCount(fact, domainTokens) > 0
}

func contextStableFactIdentityMatchCount(
	fact scan.AgentContextFactRecord,
	tokens map[string]bool,
) int {
	identity := strings.Join([]string{
		fact.Name,
		fact.Qualified,
		filepath.Base(fact.File),
		fact.HTTPMethod,
		fact.Path,
	}, " ")
	identityTokens := contextExpandedTokenSet(identity)
	compactIdentity := compactContextIdentifier(identity)
	matches := 0
	for token := range tokens {
		if identityTokens[token] {
			matches++
			continue
		}
		if len([]rune(token)) >= 4 && strings.Contains(compactIdentity, compactContextIdentifier(token)) {
			matches++
		}
	}
	return matches
}

func contextDomainModelConcernCandidates(
	query string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
	facts []scan.AgentContextFactRecord,
) []string {
	domainTokens := contextDomainModelQueryTokens(query, aliases, explicitProjects)
	if len(domainTokens) == 0 || len(explicitProjects) == 0 {
		return nil
	}
	candidates := make([]scan.AgentContextFactRecord, 0)
	for _, fact := range facts {
		if !explicitProjects[normalizeContextProject(fact.Project)] ||
			!contextDomainModelFact(fact, domainTokens) {
			continue
		}
		candidates = append(candidates, fact)
	}
	bestBySource := make(map[string]scan.AgentContextFactRecord, len(candidates))
	for _, candidate := range candidates {
		key := strings.Join([]string{
			normalizeContextProject(candidate.Project),
			contextPackSourceFile(candidate.File),
			strconv.Itoa(candidate.Line),
			compactContextIdentifier(candidate.Name),
		}, "\x00")
		current, found := bestBySource[key]
		if !found || contextDomainModelCandidateLess(candidate, current, domainTokens) {
			bestBySource[key] = candidate
		}
	}
	candidates = candidates[:0]
	for _, candidate := range bestBySource {
		candidates = append(candidates, candidate)
	}
	primaryModelsByProject := make(map[string]int)
	variantCountsByProject := make(map[string]map[string]int)
	for _, candidate := range candidates {
		if contextPrimaryDomainModelFact(candidate) {
			project := normalizeContextProject(candidate.Project)
			primaryModelsByProject[project]++
			if contextDomainModelShapeScore(candidate) >= 100 {
				if variantCountsByProject[project] == nil {
					variantCountsByProject[project] = make(map[string]int)
				}
				variantCountsByProject[project][contextDomainModelVariantStem(candidate)]++
			}
		}
	}
	primaryVariantsByProject := make(map[string]int)
	for project, variants := range variantCountsByProject {
		for _, count := range variants {
			primaryVariantsByProject[project] = max(primaryVariantsByProject[project], count)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftProject := normalizeContextProject(candidates[i].Project)
		rightProject := normalizeContextProject(candidates[j].Project)
		leftProjectCount := primaryVariantsByProject[leftProject]
		rightProjectCount := primaryVariantsByProject[rightProject]
		if leftProjectCount != rightProjectCount {
			return leftProjectCount > rightProjectCount
		}
		leftVariantCount := variantCountsByProject[leftProject][contextDomainModelVariantStem(candidates[i])]
		rightVariantCount := variantCountsByProject[rightProject][contextDomainModelVariantStem(candidates[j])]
		if leftVariantCount != rightVariantCount {
			return leftVariantCount > rightVariantCount
		}
		return contextDomainModelCandidateLess(candidates[i], candidates[j], domainTokens)
	})
	selected := make([]scan.AgentContextFactRecord, 0, min(len(candidates), 4))
	selectedByProject := make(map[string]int)
	selectedVariantByProject := make(map[string]string)
	for _, candidate := range candidates {
		project := normalizeContextProject(candidate.Project)
		if primaryModelsByProject[project] > 0 && !contextPrimaryDomainModelFact(candidate) {
			continue
		}
		projectLimit := 1
		if primaryVariantsByProject[project] >= 2 {
			projectLimit = 2
		}
		variant := contextDomainModelVariantStem(candidate)
		if selectedByProject[project] > 0 && selectedVariantByProject[project] != variant {
			continue
		}
		if selectedByProject[project] >= projectLimit {
			continue
		}
		selected = append(selected, candidate)
		selectedByProject[project]++
		selectedVariantByProject[project] = variant
		if len(selected) == 4 {
			break
		}
	}
	result := make([]string, 0, len(selected))
	for _, candidate := range selected {
		result = append(result, candidate.ID)
	}
	return result
}

func contextDomainModelVariantStem(fact scan.AgentContextFactRecord) string {
	identity := compactContextIdentifier(firstNonEmptyContext(fact.Name, fact.Qualified))
	for _, suffix := range contextDomainModelSuffixes {
		identity = strings.TrimSuffix(identity, suffix)
	}
	return strings.ReplaceAll(identity, "change", "")
}

func contextDomainModelCandidateLess(
	left,
	right scan.AgentContextFactRecord,
	domainTokens map[string]bool,
) bool {
	leftShape := contextDomainModelShapeScore(left)
	rightShape := contextDomainModelShapeScore(right)
	if leftShape != rightShape {
		return leftShape > rightShape
	}
	leftMatches := contextStableFactIdentityMatchCount(left, domainTokens)
	rightMatches := contextStableFactIdentityMatchCount(right, domainTokens)
	if leftMatches != rightMatches {
		return leftMatches > rightMatches
	}
	leftConfidence := contextDomainModelConfidenceScore(left.Confidence)
	rightConfidence := contextDomainModelConfidenceScore(right.Confidence)
	if leftConfidence != rightConfidence {
		return leftConfidence > rightConfidence
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

func contextDomainModelShapeScore(fact scan.AgentContextFactRecord) int {
	identity := strings.ToLower(firstNonEmptyContext(fact.Qualified, fact.Name))
	score := 0
	for _, suffix := range []string{"entity", "model", "projection", "record"} {
		if strings.HasSuffix(compactContextIdentifier(identity), suffix) {
			score = 100
			break
		}
	}
	if score == 0 {
		score = 40
	}
	name := strings.ToLower(strings.TrimSpace(fact.Name))
	if strings.HasPrefix(name, "base") || strings.HasPrefix(name, "abstract") {
		score -= 40
	}
	if contextDomainModelDependencyFact(fact) {
		score -= 30
	}
	if contextDerivedDomainModelFact(fact) {
		score -= 30
	}
	return score
}

func contextDomainModelDependencyFact(fact scan.AgentContextFactRecord) bool {
	identityTokens := contextTokenSet(strings.Join([]string{
		fact.Name,
		fact.Qualified,
		filepath.Base(fact.File),
	}, " "))
	for _, qualifier := range []string{"audit", "comment", "detail", "event", "history", "key", "log", "metadata"} {
		if identityTokens[qualifier] {
			return true
		}
	}
	return false
}

func contextPrimaryDomainModelFact(fact scan.AgentContextFactRecord) bool {
	name := strings.ToLower(strings.TrimSpace(fact.Name))
	return !strings.HasPrefix(name, "base") &&
		!strings.HasPrefix(name, "abstract") &&
		!contextDomainModelDependencyFact(fact) &&
		!contextDerivedDomainModelFact(fact)
}

func contextDerivedDomainModelFact(fact scan.AgentContextFactRecord) bool {
	identity := compactContextIdentifier(firstNonEmptyContext(fact.Name, fact.Qualified))
	if strings.HasSuffix(identity, "ventity") {
		return true
	}
	tokens := contextTokenSet(strings.Join([]string{fact.Name, fact.Qualified}, " "))
	for _, qualifier := range []string{"projection", "readmodel", "view"} {
		if tokens[qualifier] {
			return true
		}
	}
	return false
}

func contextDomainModelConfidenceScore(confidence string) int {
	switch strings.ToUpper(strings.TrimSpace(confidence)) {
	case "EXACT":
		return 2
	case "RESOLVED", "EXTRACTED":
		return 1
	default:
		return 0
	}
}

func contextProjectSemanticQueryTokens(
	query string,
	aliases map[string][]string,
	explicitProjects map[string]bool,
) map[string]bool {
	queryTokens := contextExpandedTokenSet(query)
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
	tokens := contextExpandedTokenSet(value)
	for _, token := range contextConcernVocabulary[kind] {
		if tokens[token] {
			return true
		}
	}
	return false
}

var contextConcernVocabulary = map[string][]string{
	contextConcernDomainModel: {
		"attribute", "attributes", "entity", "entities", "field", "fields",
		"identifier", "identifiers", "model", "models", "task_type", "task_types",
		"type", "types",
	},
	contextConcernHTTPContract: {
		"api", "apis", "client", "clients", "contract", "contracts",
	},
	contextConcernAuth: {
		"auth", "authenticate", "authentication", "authorization", "credential", "credentials", "security",
	},
	contextConcernConfiguration: {
		"config", "configuration", "properties", "property",
	},
	contextConcernResilience: {
		"exception", "resilience", "retries", "retry", "timeout",
	},
	contextConcernPersistence: {
		"database", "persistence", "persistent", "repositories", "repository", "storage", "store",
	},
	contextConcernSideEffects: {
		"event", "logging", "mail", "notification", "side_effect", "side_effects", "user_information",
	},
	contextConcernTests: {
		"spec", "specs", "test", "testing", "tests",
	},
}

func normalizedContextConcernKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "domain_model":
		return contextConcernDomainModel
	case "api_contract", "http_contract":
		return contextConcernHTTPContract
	case "authentication", "requires_auth", "security":
		return contextConcernAuth
	case "config", "configuration":
		return contextConcernConfiguration
	case "resilience", "retry":
		return contextConcernResilience
	case "persistence":
		return contextConcernPersistence
	case "side_effect", "side_effects":
		return contextConcernSideEffects
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
	endpoint, hasEndpoint, _ := selectContextEndpoint(index, ranked, query)
	if hasEndpoint {
		if companion, companionExists := contextEndpointCompanion(index, endpoint.fact); companionExists {
			return companion, true
		}
		return endpoint.fact, true
	}
	if len(seeds) == 0 {
		return scan.AgentContextFactRecord{}, false
	}
	return seeds[0].fact, true
}
