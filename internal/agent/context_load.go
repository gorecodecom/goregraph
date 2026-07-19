package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

type loadedContextIndex struct {
	Index     scan.AgentContextIndexRecord
	Path      string
	ScopeRoot string
	Workspace bool
}

type contextIndexCandidate struct {
	Path      string
	ScopeRoot string
	Workspace bool
}

func loadContextIndex(request ContextRequest) (loadedContextIndex, error) {
	requestRoot, err := filepath.Abs(request.Root)
	if err != nil {
		return loadedContextIndex{}, fmt.Errorf("resolve context root: %w", err)
	}
	cfg, err := config.Load(requestRoot)
	if err != nil {
		return loadedContextIndex{}, err
	}

	candidates := []contextIndexCandidate{
		{
			Path: scan.NewWorkspaceOutputLayout(
				filepath.Join(requestRoot, ".goregraph-workspace"),
			).Agent("context-index.json"),
			ScopeRoot: requestRoot,
			Workspace: true,
		},
	}
	if workspaceRoot, ok, resolveErr := scan.WorkspaceRoot(requestRoot, cfg); resolveErr != nil {
		return loadedContextIndex{}, resolveErr
	} else if ok {
		candidates = append(candidates, contextIndexCandidate{
			Path: scan.NewWorkspaceOutputLayout(
				filepath.Join(workspaceRoot, ".goregraph-workspace"),
			).Agent("context-index.json"),
			ScopeRoot: workspaceRoot,
			Workspace: true,
		})
	}
	candidates = append(candidates, contextIndexCandidate{
		Path:      filepath.Join(requestRoot, cfg.OutputDir, "agent", "context-index.json"),
		ScopeRoot: requestRoot,
	})

	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate.Path = filepath.Clean(candidate.Path)
		if seen[candidate.Path] {
			continue
		}
		seen[candidate.Path] = true
		info, statErr := os.Stat(candidate.Path)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return loadedContextIndex{}, fmt.Errorf(
				"context index %q is not readable: %w",
				candidate.Path,
				statErr,
			)
		}
		if info.IsDir() {
			return loadedContextIndex{}, fmt.Errorf(
				"context index %q is not a file",
				candidate.Path,
			)
		}
		body, readErr := os.ReadFile(candidate.Path)
		if readErr != nil {
			return loadedContextIndex{}, fmt.Errorf(
				"context index %q is not readable: %w",
				candidate.Path,
				readErr,
			)
		}
		var index scan.AgentContextIndexRecord
		if decodeErr := json.Unmarshal(body, &index); decodeErr != nil {
			return loadedContextIndex{}, fmt.Errorf(
				"context index %q is invalid JSON: %w",
				candidate.Path,
				decodeErr,
			)
		}
		if validateErr := validateContextIndex(index); validateErr != nil {
			return loadedContextIndex{}, fmt.Errorf(
				"context index %q is invalid: %w",
				candidate.Path,
				validateErr,
			)
		}
		return loadedContextIndex{
			Index: index, Path: candidate.Path, ScopeRoot: candidate.ScopeRoot, Workspace: candidate.Workspace,
		}, nil
	}

	return loadedContextIndex{}, fmt.Errorf(
		"context index is missing; run `goregraph build agent %s` first",
		requestRoot,
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
