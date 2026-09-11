package agent

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const maximumFallbackSourceSections = 3

// attachAdaptiveFallbackEvidence preserves source-backed candidates without
// presenting them as a unique entrypoint or a complete answer to the task.
func attachAdaptiveFallbackEvidence(pack ContextPack, loaded loadedContextIndex, request ContextRequest) (ContextPack, error) {
	pack = adaptiveContextMetadata(pack)
	if pack.FallbackReason != ContextFallbackInsufficientRelevance && pack.FallbackReason != ContextFallbackAmbiguousEntrypoint && pack.FallbackReason != ContextFallbackEvidenceConflict {
		return finalizeContextPackWithinBudget(pack, request)
	}
	tokens := contextExpandedTokenSet(contextPrimaryQuery(request.Query))
	for token := range tokens {
		if utf8.RuneCountInString(token) < 4 || contextEndpointGenericDomainToken(token) {
			delete(tokens, token)
		}
	}
	for _, token := range strings.Fields("which what when that this have from with without keep trace explain state current existing provided supplied describe determine source sources file files project projects production evidence change changes required deren diese einer eines einen einem nicht sollen sollte welche erkläre nenne behalte vorhandene vorhandenen quellcode quelle quellen") {
		delete(tokens, token)
	}
	if len(tokens) == 0 {
		return finalizeContextPackWithinBudget(pack, request)
	}
	type evidence struct {
		section ContextSourceSection
		factID  string
		score   int
		changed bool
	}
	byPath := map[string]evidence{}
	files := map[string]sourceFile{}
	ranked := rankContextFacts(loaded.Index.Facts, request.Query)
	for i, item := range ranked {
		if i == maxContextSourceSearchHandlers {
			break
		}
		fact := item.fact
		if fact.File == "" || fact.Line <= 0 || contextFactUsesTestSource(fact) && !contextQueryRequestsTests(request.Query) {
			continue
		}
		candidate := sourceCandidate{FactID: fact.ID, Project: fact.Project, Path: fact.File, StartLine: fact.Line,
			EndLine: fact.EndLine, Kind: fact.Kind, Name: fact.Name, Qualified: fact.Qualified, Role: "candidate"}
		path, err := resolveSourcePath(loaded, candidate)
		if err != nil {
			continue
		}
		file, found := files[path]
		if !found {
			if len(files) == maxContextSourceSearchFiles {
				continue
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
		attachSourceReadReceipt(&section, file)
		contentTokens := contextExpandedTokenSet(section.Content)
		identityTokens := contextExpandedTokenSet(strings.Join([]string{fact.Name, fact.Qualified, fact.File}, " "))
		score := 0
		for token := range tokens {
			if contentTokens[token] || identityTokens[token] {
				score++
			}
		}
		if score == 0 {
			continue
		}
		key := normalizeContextProject(fact.Project) + "\x00" + candidate.Path
		previous, found := byPath[key]
		if !found || score > previous.score || score == previous.score && fact.ID < previous.factID {
			byPath[key] = evidence{section: section, factID: fact.ID, score: score, changed: adaptiveSourceHashChanged(loaded, candidate, file)}
		}
	}
	options := make([]evidence, 0, len(byPath))
	for _, option := range byPath {
		options = append(options, option)
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].score != options[j].score {
			return options[i].score > options[j].score
		}
		return options[i].factID < options[j].factID
	})
	identities := []string{}
	included := map[string]bool{}
	for _, option := range options {
		if len(pack.SourceSections) >= min(maximumFallbackSourceSections, request.MaxFiles) {
			break
		}
		candidate := cloneContextPack(pack)
		candidate.SourceSections = append(candidate.SourceSections, option.section)
		candidate.SourceCoverage = "partial"
		if option.changed {
			candidate.FallbackReason = ContextFallbackEvidenceConflict
			if candidate.Health != nil {
				candidate.Health.Freshness = "stale"
			}
			candidate.SourceSections[len(candidate.SourceSections)-1].SourceState = "current_source_changed_since_index"
		}
		if len(pack.SourceSections) == 0 {
			candidate.Uncertainties = append(candidate.Uncertainties, ContextUncertainty{
				Scope: "candidate_evidence", Reason: "Query vocabulary occurs in these current declarations; a unique entrypoint, runtime ownership and complete task coverage are not established.",
			})
		}
		fits, err := contextSourcePackFits(candidate, request)
		if err != nil {
			return ContextPack{}, err
		}
		if !fits {
			continue
		}
		pack = candidate
		included[option.factID] = true
		identities = append(identities, fallbackSourceIdentity(option.factID, option.section))
	}
	for _, option := range options {
		if len(pack.SourceOmissions) == maximumContextVerificationRequests {
			break
		}
		if included[option.factID] {
			continue
		}
		candidate := cloneContextPack(pack)
		candidate.SourceOmissions = append(candidate.SourceOmissions, ContextSourceOmission{
			Project: option.section.Project, Path: option.section.Path,
			StartLine: option.section.StartLine, EndLine: option.section.EndLine,
			Role: "candidate", Reason: "additional candidate evidence exceeds the inline source limit",
		})
		candidate = adaptiveContextMetadata(candidate)
		fits, err := contextSourcePackFits(candidate, request)
		if err != nil {
			return ContextPack{}, err
		}
		if fits {
			pack = candidate
			identities = append(identities, fallbackSourceIdentity(option.factID, option.section))
		}
	}
	if len(identities) > 0 {
		pack.ContextID = contextIdentityForProtocol(request.ProtocolVersion, pack.Generation, identities, nil, nil)
	}
	if request.PreviousContextID != "" && request.PreviousContextID == pack.ContextID {
		duplicate, err := duplicateContextPack(pack)
		duplicate.FallbackRequired, duplicate.FallbackReason = pack.FallbackRequired, pack.FallbackReason
		if err != nil {
			return ContextPack{}, err
		}
		return finalizeContextPackWithinBudget(duplicate, request)
	}
	return finalizeContextPackWithinBudget(pack, request)
}

func fallbackSourceIdentity(factID string, section ContextSourceSection) string {
	// Keep the path-to-content association intact when the context ID sorts inputs.
	body, _ := json.Marshal(struct {
		FactID  string
		Section ContextSourceSection
	}{factID, section})
	return string(body)
}

func adaptiveSourceHashChanged(loaded loadedContextIndex, candidate sourceCandidate, file sourceFile) bool {
	expected := adaptiveIndexedSourceHash(loaded, candidate)
	return expected != "" && file.Hash != "" && expected != file.Hash
}

func adaptiveIndexedSourceHash(loaded loadedContextIndex, candidate sourceCandidate) string {
	key := contextPackSourceFile(candidate.Path)
	if loaded.Workspace {
		key = path.Join(normalizeContextProject(candidate.Project), key)
	}
	return loaded.Index.SourceHashes[key]
}

func adaptiveSelectedSourceFallbackReason(pack ContextPack, loaded loadedContextIndex) string {
	if len(loaded.Index.SourceHashes) == 0 {
		return ""
	}
	pack = contextPackWithSelectedClientPublicConcerns(pack)
	concerns := contextSourceConcerns(pack, loaded.Index)
	models := contextRequestedDomainModelIDsFromConcerns(pack, loaded.Index, concerns)
	seen := map[string]bool{}
	for _, candidate := range contextSourceCandidatesForConcernsWithModels(pack, loaded.Index, concerns, models) {
		if adaptiveIndexedSourceHash(loaded, candidate) == "" {
			continue
		}
		resolved, err := resolveSourcePath(loaded, candidate)
		if err != nil {
			return ContextFallbackSourceUnreadable
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		file, err := readSourceFile(resolved)
		if err != nil {
			return ContextFallbackSourceUnreadable
		}
		if adaptiveSourceHashChanged(loaded, candidate, file) {
			return ContextFallbackEvidenceConflict
		}
	}
	return ""
}
