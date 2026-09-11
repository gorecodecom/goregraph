package agent

import (
	"sort"
	"strings"
	"unicode"
)

// contextEvidenceQueryForProtocol adds bounded evidence vocabulary for adaptive
// concern planning. It deliberately leaves domain terms and the public query
// untouched.
func contextEvidenceQueryForProtocol(query, protocol string) string {
	if protocol != AdaptiveV2 {
		return query
	}

	tokens := make(map[string]bool)
	for _, token := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		tokens[token] = true
	}

	aliases := map[string][]string{
		"authentication": {
			"authentifizierungs",
		},
		"configuration": {
			"konfigurations",
		},
		"tests": {
			"testmechanismus", "testmechanismen",
		},
		"retry": {
			"wiederholungsverhalten",
		},
	}

	canonical := make([]string, 0, len(aliases))
	for term, known := range aliases {
		if tokens[term] {
			continue
		}
		for _, alias := range known {
			if tokens[alias] {
				canonical = append(canonical, term)
				break
			}
		}
	}
	if !tokens["files"] {
		switch {
		case tokens["quellpfad"] || tokens["quellpfade"] || tokens["quelldatei"] || tokens["quelldateien"]:
			canonical = append(canonical, "source files")
		case tokens["sourcepath"] || tokens["sourcepaths"] || tokens["source"] && (tokens["path"] || tokens["paths"]):
			canonical = append(canonical, "files")
		}
	}
	if !tokens["change"] && contextQueryRequestsCorrectionPlan(query) {
		canonical = append(canonical, "change")
	}
	if len(canonical) == 0 {
		return query
	}
	sort.Strings(canonical)
	return strings.TrimSpace(query) + " " + strings.Join(canonical, " ")
}

func contextQueryRequestsCorrectionPlan(query string) bool {
	tokens := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	for index, token := range tokens {
		if token == "korrekturplan" {
			return true
		}
		if index+1 >= len(tokens) || tokens[index+1] != "plan" {
			continue
		}
		if token == "correction" || token == "fix" || token == "korrektur" {
			return true
		}
	}
	return false
}

func contextChangePlanInventoryEligible(pack ContextPack) bool {
	query := contextSelectionQuery(pack)
	if contextQueryPlansMissingTransition(query) {
		return true
	}
	return pack.ProtocolVersion == AdaptiveV2 &&
		contextQueryRequestsCorrectionPlan(query)
}
