package agent

import (
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

// Follow selected local calls and persistence flow steps; configuration links
// and disconnected cycles do not establish a production call chain.
func contextLocalCallDistances(entryID, project string, index scan.AgentContextIndexRecord, selected map[string]bool) map[string]int {
	local := make(map[string]bool)
	for _, fact := range index.Facts {
		if selected[fact.ID] && normalizeContextProject(fact.Project) == project && !contextFactUsesTestSource(fact) {
			local[fact.ID] = true
		}
	}
	children := make(map[string][]string)
	for _, edge := range index.Edges {
		if local[edge.FromFactID] && local[edge.ToFactID] && contextExecutableFlowEdge(edge) {
			children[edge.FromFactID] = append(children[edge.FromFactID], edge.ToFactID)
		}
	}
	distances := map[string]int{entryID: 0}
	queue := []string{entryID}
	for next := 0; next < len(queue); next++ {
		parent := queue[next]
		if distances[parent] >= maximumContextPathHops {
			continue
		}
		for _, child := range children[parent] {
			if _, visited := distances[child]; visited {
				continue
			}
			distances[child] = distances[parent] + 1
			queue = append(queue, child)
		}
	}
	return distances
}

func contextExecutableFlowEdge(edge scan.AgentContextEdgeRecord) bool {
	kind := strings.ToLower(strings.TrimSpace(edge.Kind))
	return kind == "call" || kind == "calls" ||
		(kind == "persistence" && strings.EqualFold(strings.TrimSpace(edge.Reason), "flow"))
}
