package agent

import (
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func contextDomainModelEvidenceConcerns(
	base contextConcern,
	index scan.AgentContextIndexRecord,
	requestedModels map[string]bool,
) []contextConcern {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	modelIDs := make([]string, 0, len(requestedModels))
	for modelID := range requestedModels {
		if slicesContainsString(base.candidateFactIDs, modelID) {
			modelIDs = append(modelIDs, modelID)
		}
	}
	sort.Strings(modelIDs)

	result := make([]contextConcern, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		model, ok := factByID[modelID]
		if !ok {
			continue
		}
		concern := newContextEvidenceConcern(
			base,
			"model:"+modelID,
			contextDomainModelEvidenceFactIDs(index, modelID),
			"domain model evidence for requested model "+model.Name,
		)
		concern.project = normalizeContextProject(model.Project)
		result = append(result, concern)
	}
	return result
}

func contextDomainModelEvidenceFactIDs(
	index scan.AgentContextIndexRecord,
	modelID string,
) []string {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	model, ok := factByID[modelID]
	if !ok {
		return nil
	}
	result := []string{modelID}
	for _, edge := range index.Edges {
		if edge.FromFactID != modelID ||
			strings.ToLower(strings.TrimSpace(edge.Kind)) != "extends" {
			continue
		}
		base, found := factByID[edge.ToFactID]
		if found && normalizeContextProject(base.Project) == normalizeContextProject(model.Project) {
			result = append(result, base.ID)
		}
	}
	return orderedContextConcernIDs(result)
}

func slicesContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
