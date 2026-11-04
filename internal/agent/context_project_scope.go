package agent

import (
	"path/filepath"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

// scopeProductionIndex keeps source ranking within the requested workspace subtree,
// plus explicitly named projects and facts linked directly to the selected scope.
func scopeProductionIndex(loaded loadedContextIndex, request ContextRequest) (loadedContextIndex, error) {
	if !loaded.Workspace {
		return loaded, nil
	}
	root, err := filepath.Abs(request.Root)
	if err != nil {
		return loadedContextIndex{}, err
	}
	if filepath.Clean(root) == filepath.Clean(loaded.ScopeRoot) {
		return loaded, nil
	}
	index := loaded.Index
	explicit := map[string]bool{}
	query := " " + normalizeContextTerm(request.Query) + " "
	for project, aliases := range contextProjectAliases(index.Facts, index.Coverage) {
		for _, alias := range aliases {
			if contextQueryContainsProjectPath(request.Query, alias) ||
				(alias != project && strings.Contains(query, " "+alias+" ")) {
				explicit[project] = true
			}
		}
	}
	base := map[string]bool{}
	for _, fact := range index.Facts {
		if explicit[normalizeContextProject(fact.Project)] ||
			(fact.Project != "" && !isPortableAbsolutePath(fact.Project) &&
				pathIsWithin(root, filepath.Join(loaded.ScopeRoot, filepath.FromSlash(fact.Project)))) {
			base[fact.ID] = true
		}
	}
	linked := map[string]bool{}
	for _, edge := range index.Edges {
		if !reviewScopeEdge(edge) {
			continue
		}
		if base[edge.FromFactID] {
			linked[edge.ToFactID] = true
		}
		if base[edge.ToFactID] {
			linked[edge.FromFactID] = true
		}
	}
	index.Facts = nil
	index.Edges = nil
	index.Coverage = nil
	projects := map[string]bool{}
	for _, fact := range loaded.Index.Facts {
		if base[fact.ID] || linked[fact.ID] {
			index.Facts = append(index.Facts, fact)
			projects[normalizeContextProject(fact.Project)] = true
		}
	}
	for _, edge := range loaded.Index.Edges {
		if (base[edge.FromFactID] && base[edge.ToFactID]) ||
			(reviewScopeEdge(edge) && ((base[edge.FromFactID] && linked[edge.ToFactID]) ||
				(base[edge.ToFactID] && linked[edge.FromFactID]))) {
			index.Edges = append(index.Edges, edge)
		}
	}
	for _, coverage := range loaded.Index.Coverage {
		if projects[normalizeContextProject(coverage.Project)] {
			index.Coverage = append(index.Coverage, coverage)
		}
	}
	loaded.Index = index
	return loaded, nil
}

func reviewScopeEdge(edge scan.AgentContextEdgeRecord) bool {
	if edge.FromFactID == "" || edge.ToFactID == "" {
		return false
	}
	switch edge.Confidence {
	case string(scan.ConfidenceExact), string(scan.ConfidenceResolved), string(scan.ConfidenceNormalized):
		return edge.Kind == "call" || edge.Kind == "http_contract" || edge.Kind == "reference" || edge.Kind == "import"
	default:
		return false
	}
}
