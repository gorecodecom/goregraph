package agent

import (
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

// associatedAssetEvidencePriorities follows only indexed exact saved-asset links.
// It does not reinterpret an event binding as an executable call chain.
func associatedAssetEvidencePriorities(index scan.AgentContextIndexRecord, query string, names map[string]bool) map[string]int {
	priorities := map[string]int{}
	tokens := contextTokenSet(query)
	requested := false
	for _, word := range []string{"unity", "unityevent", "prefab", "serialized", "serialisiert", "blender"} {
		requested = requested || tokens[word]
	}
	if !requested {
		return priorities
	}
	byID := map[string]scan.AgentContextFactRecord{}
	for _, fact := range index.Facts {
		byID[fact.ID] = fact
	}
	for _, edge := range index.Edges {
		from, to := byID[edge.FromFactID], byID[edge.ToFactID]
		if edge.Kind != "use" || !contextAssetSource(from.File) || !names[strings.ToLower(to.Name)] {
			continue
		}
		if strings.Contains(edge.Reason, "persistent UnityEvent") || strings.Contains(edge.Reason, "serialized property") || strings.Contains(edge.Reason, "m_Script GUID") {
			priorities[from.ID] = 3
		}
	}
	return priorities
}
