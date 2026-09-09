package agent

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gorecodecom/goregraph/internal/scan"
)

const maxContextSourceSearchHandlers = 64
const maxContextSourceSearchFiles = 16

// withContextSourceSearch recovers missing lexical anchors from verified,
// indexed handler bodies. It never scans unindexed files or invents translations.
func withContextSourceSearch(loaded loadedContextIndex, query string) loadedContextIndex {
	ranked := rankContextFacts(loaded.Index.Facts, query)
	if len(selectContextSeeds(ranked)) > 0 {
		return loaded
	}
	if _, ok, reason := selectContextEndpoint(loaded.Index, ranked, query); ok || reason != "" {
		return loaded
	}
	actions := contextEndpointRequestedActions(query)
	if !contextActionFamiliesHaveMutation(actions) {
		return loaded
	}
	domainTokens := contextConcernDomainQueryTokensWithoutFallback(contextTokenSet(contextPrimaryQuery(query)))
	scaffolding := contextExactInventoryScaffoldingTokens()
	for token := range domainTokens {
		if utf8.RuneCountInString(token) < 4 || scaffolding[token] || contextEndpointGenericDomainToken(token) ||
			len(contextActionFamilies(token, "")) > 0 {
			delete(domainTokens, token)
		}
	}
	var candidates []scan.AgentContextFactRecord
	for _, fact := range loaded.Index.Facts {
		if eligibleContextEndpoint(fact) && reliableProductionContextSeed(fact) &&
			contextEndpointActionAligned(rankedContextFact{fact: fact}, actions) {
			candidates = append(candidates, fact)
		}
	}
	if len(candidates) > maxContextSourceSearchHandlers || len(domainTokens) == 0 {
		return loaded
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	files := make(map[string]sourceFile)
	bestID, bestHandler, bestScore := "", "", 0
	var bestTokens []string
	ambiguous := false
	for _, fact := range candidates {
		candidate := sourceCandidate{FactID: fact.ID, Project: fact.Project, Path: fact.File,
			StartLine: fact.Line, EndLine: fact.EndLine, Kind: fact.Kind, Name: fact.Name, Qualified: fact.Qualified}
		path, err := resolveSourcePath(loaded, candidate)
		if err != nil {
			continue
		}
		file, cached := files[path]
		if !cached {
			if len(files) >= maxContextSourceSearchFiles {
				return loaded
			}
			file, err = readSourceFile(path)
			files[path] = file
			if err != nil {
				continue
			}
		}
		section, err := renderSourceCandidate(candidate, file, "declaration_body")
		if err != nil {
			continue
		}
		tokens := contextTokenSet(section.Content)
		var matched []string
		for token := range domainTokens {
			if tokens[token] {
				matched = append(matched, token)
			}
		}
		handler := fact.Project + "\x00" + fact.File + "\x00" + fact.Qualified
		if len(matched) > bestScore {
			bestID, bestHandler, bestScore = fact.ID, handler, len(matched)
			bestTokens, ambiguous = matched, false
		} else if len(matched) > 0 && len(matched) == bestScore && handler != bestHandler {
			ambiguous = true
		}
	}
	if bestScore == 0 || ambiguous {
		return loaded
	}
	sort.Strings(bestTokens)
	loaded.sourceSearchID = bestID
	loaded.Index.Facts = append([]scan.AgentContextFactRecord(nil), loaded.Index.Facts...)
	for i := range loaded.Index.Facts {
		if loaded.Index.Facts[i].ID == bestID {
			loaded.Index.Facts[i].Search += " " + strings.Join(bestTokens, " ")
		}
	}
	return loaded
}
