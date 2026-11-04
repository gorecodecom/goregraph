package agent

import (
	"path/filepath"
	"sort"

	"github.com/gorecodecom/goregraph/internal/pathutil"
)

// ContextNavigationCandidate is an authored search hint for an indexed directory.
type ContextNavigationCandidate struct {
	Path         string   `json:"path"`
	Purpose      string   `json:"purpose"`
	MatchedTerms []string `json:"matched_terms"`
}

func contextModuleCandidates(loaded loadedContextIndex, root string, queryTerms map[string]bool) []ContextNavigationCandidate {
	if len(loaded.ModulePurposes) == 0 {
		return nil
	}
	root, err := pathutil.Resolve(root)
	if err != nil {
		return nil
	}
	scope, err := pathutil.Resolve(loaded.ScopeRoot)
	if err != nil || !pathIsWithin(scope, root) {
		return nil
	}
	var candidates []ContextNavigationCandidate
	for _, module := range loaded.ModulePurposes {
		candidate := ContextNavigationCandidate{Path: module.Path, Purpose: module.Description}
		for word := range contextExpandedTokenSet(module.Description) {
			if queryTerms[word] {
				candidate.MatchedTerms = append(candidate.MatchedTerms, word)
			}
		}
		if len(candidate.MatchedTerms) < 2 {
			continue
		}
		sort.Strings(candidate.MatchedTerms)
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if len(candidates[i].MatchedTerms) != len(candidates[j].MatchedTerms) {
			return len(candidates[i].MatchedTerms) > len(candidates[j].MatchedTerms)
		}
		return candidates[i].Path < candidates[j].Path
	})
	var matches []ContextNavigationCandidate
	for _, candidate := range candidates {
		directory := filepath.Join(root, filepath.FromSlash(candidate.Path))
		resolved, err := pathutil.Resolve(directory)
		if err != nil || resolved != directory {
			continue
		}
		for _, fact := range loaded.Index.Facts {
			project := ""
			if loaded.Workspace {
				project = fact.Project
			}
			if fact.Line < 1 || !safeSourceReadPath(fact.File) || sourceReadGeneratedPath(fact.File) ||
				!eligibleContextPathFact(fact, false) ||
				(loaded.Workspace && project != "." && !safeSourceReadPath(project)) {
				continue
			}
			file := filepath.Join(scope, filepath.FromSlash(project), filepath.FromSlash(fact.File))
			if !pathIsWithin(directory, file) {
				continue
			}
			resolvedFile, err := resolveSourcePath(loaded, sourceCandidate{Project: fact.Project, Path: fact.File})
			if err != nil || !pathIsWithin(directory, resolvedFile) {
				continue
			}
			candidate.MatchedTerms = candidate.MatchedTerms[:min(4, len(candidate.MatchedTerms))]
			matches = append(matches, candidate)
			break
		}
		if len(matches) == 3 {
			break
		}
	}
	return matches
}

// Prepare only ranges already authorized by the pack and accepted by the reader.
func attachContextNextRead(pack ContextPack, loaded loadedContextIndex, request ContextRequest) (ContextPack, error) {
	if pack.ProtocolVersion != AdaptiveV2 || len(pack.VerificationRequests) == 0 ||
		pack.DuplicateOf != "" || adaptiveHealthFallbackReason(pack) != "" {
		return pack, nil
	}
	root, err := pathutil.Resolve(request.Root)
	if err != nil {
		return pack, nil
	}
	read := ReadSourceRequest{}
	for _, verify := range pack.VerificationRequests {
		resolved, err := resolveSourcePath(loaded, sourceCandidate{Project: verify.Project, Path: verify.Path})
		if err != nil || !pathIsWithin(root, resolved) {
			continue
		}
		relative, err := filepath.Rel(root, resolved)
		if err != nil {
			continue
		}
		file := SourceReadFileRequest{
			Path:   filepath.ToSlash(relative),
			Ranges: []SourceReadRange{{verify.StartLine, verify.EndLine}},
		}
		for _, section := range pack.SourceSections {
			if section.Path == verify.Path && normalizeContextProject(section.Project) == normalizeContextProject(verify.Project) && section.ReadReceipt != "" {
				file.Seen = append(file.Seen, section.ReadReceipt)
			}
		}
		candidate := ReadSourceRequest{Files: append(append([]SourceReadFileRequest(nil), read.Files...), file)}
		if validateSourceReadRequest(candidate) != nil {
			continue
		}
		if _, err := resolveSourceReadPath(root, loaded, file.Path); err != nil {
			continue
		}
		next, accepted, err := tryContextPack(pack, request.BudgetTokens, func(candidatePack *ContextPack) bool {
			candidatePack.NextRead = &candidate
			return true
		})
		if err != nil {
			return ContextPack{}, err
		}
		if accepted {
			pack, read = next, candidate
		}
	}
	return pack, nil
}
