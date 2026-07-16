package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func loadContextIndex(request ContextRequest) (scan.AgentContextIndexRecord, string, error) {
	cfg, err := config.Load(request.Root)
	if err != nil {
		return scan.AgentContextIndexRecord{}, "", err
	}

	candidates := []string{
		scan.NewWorkspaceOutputLayout(
			filepath.Join(request.Root, ".goregraph-workspace"),
		).Agent("context-index.json"),
	}
	if workspaceRoot, ok, resolveErr := scan.WorkspaceRoot(request.Root, cfg); resolveErr != nil {
		return scan.AgentContextIndexRecord{}, "", resolveErr
	} else if ok {
		candidates = append(candidates, scan.NewWorkspaceOutputLayout(
			filepath.Join(workspaceRoot, ".goregraph-workspace"),
		).Agent("context-index.json"))
	}
	candidates = append(candidates, scan.NewProjectOutputLayout(
		filepath.Join(request.Root, cfg.OutputDir),
	).Agent("context-index.json"))

	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		info, statErr := os.Stat(candidate)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return scan.AgentContextIndexRecord{}, "", fmt.Errorf(
				"context index %q is not readable: %w",
				candidate,
				statErr,
			)
		}
		if info.IsDir() {
			return scan.AgentContextIndexRecord{}, "", fmt.Errorf(
				"context index %q is not a file",
				candidate,
			)
		}
		body, readErr := os.ReadFile(candidate)
		if readErr != nil {
			return scan.AgentContextIndexRecord{}, "", fmt.Errorf(
				"context index %q is not readable: %w",
				candidate,
				readErr,
			)
		}
		var index scan.AgentContextIndexRecord
		if decodeErr := json.Unmarshal(body, &index); decodeErr != nil {
			return scan.AgentContextIndexRecord{}, "", fmt.Errorf(
				"context index %q is invalid JSON: %w",
				candidate,
				decodeErr,
			)
		}
		if validateErr := validateContextIndex(index); validateErr != nil {
			return scan.AgentContextIndexRecord{}, "", fmt.Errorf(
				"context index %q is invalid: %w",
				candidate,
				validateErr,
			)
		}
		return index, candidate, nil
	}

	return scan.AgentContextIndexRecord{}, "", fmt.Errorf(
		"context index is missing; run `goregraph build agent %s` first",
		request.Root,
	)
}

func validateContextIndex(index scan.AgentContextIndexRecord) error {
	if index.SchemaVersion != scan.SchemaVersion {
		return fmt.Errorf(
			"schema version %d is unsupported; expected %d",
			index.SchemaVersion,
			scan.SchemaVersion,
		)
	}
	factIDs := make(map[string]bool, len(index.Facts))
	for _, fact := range index.Facts {
		if fact.ID == "" {
			return fmt.Errorf("fact id is required")
		}
		if factIDs[fact.ID] {
			return fmt.Errorf("duplicate fact id %q", fact.ID)
		}
		factIDs[fact.ID] = true
	}
	edgeIDs := make(map[string]bool, len(index.Edges))
	for _, edge := range index.Edges {
		if edge.ID == "" {
			return fmt.Errorf("edge id is required")
		}
		if edgeIDs[edge.ID] {
			return fmt.Errorf("duplicate edge id %q", edge.ID)
		}
		if factIDs[edge.ID] {
			return fmt.Errorf("edge id %q collides with a fact id", edge.ID)
		}
		if !factIDs[edge.FromFactID] {
			return fmt.Errorf("edge %q has dangling from fact %q", edge.ID, edge.FromFactID)
		}
		if !factIDs[edge.ToFactID] {
			return fmt.Errorf("edge %q has dangling to fact %q", edge.ID, edge.ToFactID)
		}
		edgeIDs[edge.ID] = true
	}
	return nil
}
