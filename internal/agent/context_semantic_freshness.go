package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func selectedSemanticDependencies(loaded loadedContextIndex, pack ContextPack) []scan.SemanticDependencyRecord {
	if len(loaded.Index.SemanticDependencies) == 0 {
		return nil
	}
	type selectedSource struct{ project, file string }
	var sources []selectedSource
	for _, file := range pack.Files {
		sources = append(sources, selectedSource{file.Project, file.Path})
	}
	for _, source := range pack.SourceSections {
		sources = append(sources, selectedSource{source.Project, source.Path})
	}
	ids := map[string]bool{}
	for _, id := range append(append([]string{}, pack.selectedFactIDs...), pack.selectedSourceFactIDs...) {
		ids[id] = true
	}
	edges := map[string]bool{}
	for _, id := range pack.selectedEdgeIDs {
		edges[id] = true
	}
	for _, edge := range loaded.Index.Edges {
		if edges[edge.ID] {
			ids[edge.FromFactID], ids[edge.ToFactID] = true, true
		}
	}
	for _, fact := range loaded.Index.Facts {
		if ids[fact.ID] {
			sources = append(sources, selectedSource{fact.Project, fact.File})
		}
	}
	var result []scan.SemanticDependencyRecord
	for _, dependency := range loaded.Index.SemanticDependencies {
		for _, source := range sources {
			if loaded.Workspace && normalizeContextProject(dependency.Project) != normalizeContextProject(source.project) {
				continue
			}
			ext := strings.ToLower(filepath.Ext(source.file))
			if dependency.Language == "csharp" && (ext == ".cs" || strings.HasSuffix(source.file, ".goregraph-csharp.json")) || dependency.Language == "swift" && (ext == ".swift" || strings.HasSuffix(source.file, ".goregraph-swift.json")) {
				result = append(result, dependency)
				break
			}
		}
	}
	return result
}

// semanticDependenciesCurrent hashes only recorded project-owned snapshot inputs.
// It never invokes a compiler or reads through an escaping symbolic link.
func semanticDependenciesCurrent(loaded loadedContextIndex) bool {
	remaining := int64(64 * 1024 * 1024)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	seen := map[string]bool{}
	for _, dependency := range loaded.Index.SemanticDependencies {
		if len(dependency.Inputs) == 0 {
			return false
		}
		if dependency.Language == "csharp" || dependency.Language == "swift" {
			root := loaded.ScopeRoot
			if loaded.Workspace {
				root = filepath.Join(root, dependency.Project)
				if dependency.Project == "" || isPortableAbsolutePath(dependency.Project) || !pathIsWithin(loaded.ScopeRoot, root) {
					return false
				}
			}
			// Resolve the project through the same scope checks as source reads.
			if dependency.Project != "" && loaded.Workspace {
				for file := range dependency.Inputs {
					resolved, err := resolveSourcePath(loaded, sourceCandidate{Project: dependency.Project, Path: file})
					if err != nil || resolved == "" {
						return false
					}
					break
				}
			}
			cfg, err := config.Load(root)
			if err != nil {
				return false
			}
			_, err = scan.WalkProjectFiles(ctx, root, cfg, func(file scan.WalkedFile) error {
				if scan.SemanticInputFile(scan.FileRecord{Path: file.Path}, dependency.Language) && dependency.Inputs[file.Path] == "" {
					return fmt.Errorf("new semantic input")
				}
				return nil
			})
			if err != nil {
				return false
			}
		}
		for file, expected := range dependency.Inputs {
			key := dependency.Project + "/" + file + "/" + expected
			if seen[key] {
				continue
			}
			seen[key] = true
			if len(expected) != 64 {
				return false
			}
			resolved, err := resolveSourcePath(loaded, sourceCandidate{Project: dependency.Project, Path: file})
			if err != nil {
				return false
			}
			input, err := os.Open(resolved)
			if err != nil {
				return false
			}
			before, err := input.Stat()
			if err != nil || !before.Mode().IsRegular() || before.Size() > remaining {
				input.Close()
				return false
			}
			digest := sha256.New()
			count, readErr := io.Copy(digest, io.LimitReader(input, remaining+1))
			after, statErr := input.Stat()
			input.Close()
			if readErr != nil || statErr != nil || count != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || hex.EncodeToString(digest.Sum(nil)) != expected {
				return false
			}
			remaining -= count
		}
	}
	return true
}
