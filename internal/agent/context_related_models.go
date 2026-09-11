package agent

import (
	"sort"
	"strings"
	"unicode"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func planContextSourceConcerns(query string, index scan.AgentContextIndexRecord, seed scan.AgentContextFactRecord, protocol string) []contextConcern {
	evidenceQuery := contextEvidenceQueryForProtocol(query, protocol)
	planned := planContextConcernsWithEvidenceQuery(query, evidenceQuery, index, seed)
	inferred := false
	primaryIDs := make(map[string]bool)
	for i := range planned {
		if planned[i].kind == contextConcernPrimaryPath {
			for _, id := range planned[i].candidateFactIDs {
				primaryIDs[id] = true
			}
		}
		if planned[i].kind != contextConcernDomainModel || !planned[i].requireIdentity {
			continue
		}
		inferred = true
		if protocol == AdaptiveV2 {
			planned[i].candidateFactIDs = contextInferredDomainModelCandidates(query, seed, index.Facts, primaryIDs)
		} else {
			planned[i].candidateFactIDs = contextDomainModelConcernCandidates(query+" "+seed.Name+" "+seed.Path, nil, nil, index.Facts)
		}
	}
	if inferred {
		planned = append(planned, relatedModelContextConcerns(query, index, seed, planned, protocol)...)
	}
	return planned
}

func contextInferredDomainModelCandidates(query string, seed scan.AgentContextFactRecord, facts []scan.AgentContextFactRecord, primaryIDs map[string]bool) []string {
	identity := seed.Name
	resourceIdentity := identity
	if seed.Path != "" {
		var segments []string
		for _, segment := range strings.Split(seed.Path, "/") {
			if !strings.HasPrefix(segment, "{") && !strings.HasPrefix(segment, ":") {
				segments = append(segments, segment)
				if len(contextInferredModelIdentityTokens(segment)) > 0 {
					resourceIdentity = segment
				}
			}
		}
		identity = strings.Join(segments, " ")
	}
	anchors := contextInferredModelIdentityTokens(identity)
	if len(anchors) == 0 {
		anchors = contextInferredModelIdentityTokens(contextIdentifierLeaf(contextQualifiedOwner(seed.Qualified)))
		resourceIdentity = contextIdentifierLeaf(contextQualifiedOwner(seed.Qualified))
	}
	// Require the terminal resource; parent terms only disambiguate abbreviations.
	resource := contextInferredModelIdentityTokens(resourceIdentity)
	requested := make(map[string]bool)
	for _, identifier := range strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		requested[strings.ToLower(identifier)] = true
	}
	var eligible []scan.AgentContextFactRecord
	hasConcreteModel := false
	for _, fact := range facts {
		leaf := contextIdentifierLeaf(firstNonEmptyContext(fact.Name, fact.Qualified))
		if !contextDomainModelFact(fact, contextTokenSet(leaf)) {
			continue
		}
		matches := contextInferredModelAnchorMatches(contextInferredModelIdentityTokens(leaf), anchors, resource)
		if !requested[strings.ToLower(leaf)] && (len(resource) == 0 || matches != len(resource)) {
			continue
		}
		eligible = append(eligible, fact)
		if len(resource) > 0 && matches == len(resource) && contextPrimaryDomainModelFact(fact) && contextDomainModelShapeScore(fact) >= 100 {
			hasConcreteModel = true
		}
	}
	queryTokens := contextInferredModelIdentityTokens(query)
	typeIntent := queryTokens["type"] || queryTokens["model"] || queryTokens["field"]
	models := eligible[:0]
	modelQuery := query + " " + seed.Name + " " + seed.Path
	for _, fact := range eligible {
		leaf := contextIdentifierLeaf(firstNonEmptyContext(fact.Name, fact.Qualified))
		payloadKind := contextInferredModelPayloadKind(leaf)
		if hasConcreteModel && payloadKind != "" && !primaryIDs[fact.ID] && !requested[strings.ToLower(leaf)] && !(typeIntent && queryTokens[payloadKind]) {
			continue
		}
		models = append(models, fact)
		// The declaration has already proved its anchor match, including abbreviations.
		modelQuery += " " + leaf
	}
	return contextDomainModelConcernCandidates(modelQuery, nil, nil, models)
}

func contextInferredModelPayloadKind(name string) string {
	for _, suffix := range []string{"dto", "payload", "request", "response"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return suffix
		}
	}
	return ""
}

func contextInferredModelIdentityTokens(identity string) map[string]bool {
	result := make(map[string]bool)
	for token := range contextTokenSet(identity) {
		if contextEndpointGenericDomainToken(token) || contextHTTPVerbs[strings.ToUpper(token)] || len(contextActionFamilies(token, "")) > 0 {
			continue
		}
		if len(token) > 4 && strings.HasSuffix(token, "ies") {
			token = strings.TrimSuffix(token, "ies") + "y"
		} else if len(token) > 3 && strings.HasSuffix(token, "s") && !strings.HasSuffix(token, "ss") && !strings.HasSuffix(token, "us") && !strings.HasSuffix(token, "is") {
			token = strings.TrimSuffix(token, "s")
		}
		result[token] = true
	}
	return result
}

func contextInferredModelAnchorMatches(tokens, anchors, resource map[string]bool) int {
	matched := make(map[string]bool)
	for token := range tokens {
		if anchors[token] {
			matched[token] = true
			continue
		}
		if len([]rune(token)) < 3 {
			continue
		}
		prefixAnchor := ""
		for anchor := range anchors {
			if !strings.HasPrefix(anchor, token) {
				continue
			}
			if prefixAnchor != "" {
				prefixAnchor = ""
				break
			}
			prefixAnchor = anchor
		}
		if prefixAnchor != "" {
			matched[prefixAnchor] = true
		}
	}
	count := 0
	for token := range resource {
		if matched[token] {
			count++
		}
	}
	return count
}

// relatedModelContextConcerns discovers extension points, not runtime edges.
// Current declarations must still prove each concern during source selection.
func relatedModelContextConcerns(query string, index scan.AgentContextIndexRecord, seed scan.AgentContextFactRecord, concerns []contextConcern, protocol string) []contextConcern {
	models := map[string]bool{}
	bases := map[string]contextConcern{}
	for _, concern := range concerns {
		if concern.project == "" && concern.facet == "" {
			bases[concern.kind] = concern
		}
		if concern.kind == contextConcernDomainModel {
			for _, id := range concern.candidateFactIDs {
				models[id] = true
			}
		}
	}
	if len(models) == 0 {
		return nil
	}
	domain := map[string]bool{}
	parentIdentity := seed.Name + " " + seed.Path
	parentDomain := contextConcernDomainQueryTokensWithoutFallback(contextExpandedTokenSet(parentIdentity))
	for token := range parentDomain {
		if contextEndpointGenericDomainToken(token) || len(contextActionFamilies(token, "")) > 0 {
			delete(parentDomain, token)
		}
	}
	if protocol == AdaptiveV2 && len(parentDomain) == 0 {
		parentDomain = contextConcernDomainQueryTokensWithoutFallback(contextExpandedTokenSet(contextIdentifierLeaf(contextQualifiedOwner(seed.Qualified))))
	}
	projects := map[string]bool{}
	for _, fact := range index.Facts {
		if !models[fact.ID] {
			continue
		}
		projects[normalizeContextProject(fact.Project)] = true
		for token := range contextExpandedTokenSet(fact.Name) {
			domain[token] = true
			if protocol == AdaptiveV2 && (parentDomain[token+"s"] || strings.HasSuffix(token, "y") && parentDomain[strings.TrimSuffix(token, "y")+"ies"]) {
				parentDomain[token] = true
			}
		}
	}
	for token := range contextExpandedTokenSet(seed.Name + " " + seed.Path + " " + seed.Qualified) {
		delete(domain, token)
	}
	for _, token := range append(append([]string(nil), contextDomainModelSuffixes...), "base", "abstract", "change", "id") {
		delete(domain, token)
	}
	for token := range domain {
		if len([]rune(token)) < 4 {
			delete(domain, token)
		}
	}
	if len(domain) == 0 {
		for token := range parentDomain {
			domain[token] = true
		}
		if len(domain) == 0 {
			return nil
		}
	}
	type group struct {
		base    contextConcern
		project string
		ids     []string
	}
	groups := map[string]*group{}
	requestedKinds := make([]string, 0, len(bases))
	for _, kind := range []string{contextConcernHTTPContract, contextConcernAuth, contextConcernConfiguration, contextConcernResilience, contextConcernPersistence, contextConcernSideEffects, contextConcernTests} {
		if _, exists := bases[kind]; exists && contextQueryRequestsConcern(contextEvidenceQueryForProtocol(query, protocol), kind) {
			requestedKinds = append(requestedKinds, kind)
		}
	}
	requestedActions := contextEndpointRequestedActions(query)
	facts := append([]scan.AgentContextFactRecord(nil), index.Facts...)
	scores := make(map[string]int, len(facts))
	for _, fact := range facts {
		scores[fact.ID] = contextStableFactIdentityMatchCount(fact, domain)*10 + contextStableFactIdentityMatchCount(fact, parentDomain)
	}
	sort.Slice(facts, func(i, j int) bool {
		if scores[facts[i].ID] != scores[facts[j].ID] {
			return scores[facts[i].ID] > scores[facts[j].ID]
		}
		return facts[i].ID < facts[j].ID
	})
	for _, fact := range facts {
		if fact.File == "" || fact.Line <= 0 || models[fact.ID] {
			continue
		}
		project := normalizeContextProject(fact.Project)
		parentMatches := contextStableFactIdentityMatchCount(fact, parentDomain) > 0
		parentAligned := len(parentDomain) == 0 || parentMatches
		if protocol != AdaptiveV2 && !projects[project] && !parentMatches {
			continue
		}
		identity := fact.Name + " " + fact.Qualified
		aligned := contextStableFactIdentityMatchCount(fact, domain) > 0
		isTest := contextFactUsesTestSource(fact)
		factKind := normalizedContextConcernKind(fact.Kind)
		factTokens := contextExpandedTokenSet(identity + " " + fact.Search)
		for _, kind := range requestedKinds {
			base := bases[kind]
			if protocol == AdaptiveV2 && !parentAligned && !(kind == contextConcernAuth && projects[project]) {
				continue
			}
			if isTest != (kind == contextConcernTests) {
				continue
			}
			if !aligned && !(kind == contextConcernAuth && projects[project]) {
				continue
			}
			matches := factKind == kind
			for _, token := range contextConcernVocabulary[kind] {
				matches = matches || factTokens[token]
			}
			if kind == contextConcernHTTPContract {
				matches = matches || fact.HTTPMethod != "" || strings.HasSuffix(strings.ToLower(fact.Name), "controller")
			}
			if kind == contextConcernSideEffects {
				matches = projects[project] && contextActionFamiliesOverlap(requestedActions, contextFactActionFamilies(fact))
			}
			if !matches {
				continue
			}
			key := kind + ":" + project
			if groups[key] == nil {
				groups[key] = &group{base: base, project: project}
			}
			if protocol == AdaptiveV2 || len(groups[key].ids) < 8 {
				groups[key].ids = append(groups[key].ids, fact.ID)
			}
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	factByID := make(map[string]scan.AgentContextFactRecord, len(facts))
	for _, fact := range facts {
		factByID[fact.ID] = fact
	}
	result := make([]contextConcern, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		if protocol == AdaptiveV2 && group.base.kind == contextConcernHTTPContract && contextQueryRequestsInternalInterface(query) {
			sort.SliceStable(group.ids, func(i, j int) bool {
				return contextExactInventoryInternalInterfaceFact(factByID[group.ids[i]]) &&
					!contextExactInventoryInternalInterfaceFact(factByID[group.ids[j]])
			})
		}
		ids := make([]string, 0, min(8, len(group.ids)))
		paths := make(map[string]bool)
		selected := make(map[string]bool)
		for _, id := range group.ids {
			path := contextPackSourceFile(factByID[id].File)
			if protocol == AdaptiveV2 && paths[path] {
				continue
			}
			paths[path], selected[id] = true, true
			ids = append(ids, id)
			if len(ids) == 8 {
				break
			}
		}
		for _, id := range group.ids {
			if len(ids) == 8 {
				break
			}
			if !selected[id] {
				ids = append(ids, id)
			}
		}
		concern := newExpandedContextEvidenceConcern(group.base, "related:"+group.project,
			ids, "related model extension point; runtime ownership is unproven")
		concern.project = group.project
		result = append(result, concern)
	}
	return result
}
