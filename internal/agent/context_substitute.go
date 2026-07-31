package agent

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type contextSourceSelectionScore struct {
	requiredPublicProofs int
	requiredProofs       int
	identityQuality      int
	estimatedTokens      int
	key                  string
}

type resolvedContextSourceOption struct {
	option    contextSourceOption
	protected bool
}

func rebuildContextSourceSelection(
	base ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
) (ContextPack, contextSourceSelectionState, error) {
	pack := cloneContextPack(base)
	pack.SourceSections = nil
	pack.SourceOmissions = nil
	state := newContextSourceSelectionState(len(options), len(concerns))
	ordered := append([]contextSourceOption(nil), options...)
	sort.Slice(ordered, func(i, j int) bool {
		return contextSourceOptionLess(ordered[i], ordered[j])
	})
	var err error
	for _, option := range ordered {
		pack, state, err = addContextSourceOption(
			pack,
			request,
			option,
			concerns,
			state,
		)
		if err != nil {
			return ContextPack{}, contextSourceSelectionState{}, err
		}
	}
	return pack, state, nil
}

func improveContextSourceSelection(
	base ContextPack,
	current ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
	boundaries []contextSourceBoundary,
) (ContextPack, error) {
	ordered := orderedContextSourceSelectionOptions(options)
	selected, ok := resolveSelectedContextSourceOptions(
		current,
		ordered,
		boundaries,
	)
	if !ok {
		return current, nil
	}

	current = cloneContextPack(current)
	currentCovered := contextSourceCoverageFromFinalSections(
		current,
		concerns,
		ordered,
	)
	applyContextSourceCoverage(&current, concerns, currentCovered)
	var err error
	current, err = finalizeContextEstimate(current)
	if err != nil {
		return ContextPack{}, err
	}
	currentScore := contextSourceSelectionScoreFor(
		current,
		concerns,
		ordered,
		currentCovered,
	)
	seen := map[string]bool{currentScore.key: true}
	limits := boundedContextSourceSubstitutionRequest(request)

	for attempts := 0; attempts < len(ordered); attempts++ {
		selectedCandidates := contextSourceSelectedCandidateKeys(selected)
		bestPack := ContextPack{}
		bestScore := contextSourceSelectionScore{}
		found := false

		for _, replacement := range ordered {
			if selectedCandidates[contextSourceCandidateKey(replacement.candidate)] ||
				!contextSourceOptionProvesRequiredConcern(replacement, concerns) {
				continue
			}
			for removeIndex, removed := range selected {
				if removed.protected {
					continue
				}
				trialOptions := make([]contextSourceOption, 0, len(selected))
				for index, kept := range selected {
					if index != removeIndex {
						trialOptions = append(trialOptions, kept.option)
					}
				}
				trialOptions = append(trialOptions, replacement)

				fits, fitErr := contextSourceSelectionOptionsFit(
					base,
					limits,
					trialOptions,
					concerns,
				)
				if fitErr != nil {
					return ContextPack{}, fitErr
				}
				if !fits {
					continue
				}
				candidate, _, rebuildErr := rebuildContextSourceSelection(
					base,
					limits,
					trialOptions,
					concerns,
				)
				if rebuildErr != nil {
					return ContextPack{}, rebuildErr
				}
				if len(candidate.SourceSections) != len(current.SourceSections) {
					continue
				}
				candidateCovered := contextSourceCoverageFromFinalSections(
					candidate,
					concerns,
					ordered,
				)
				if !contextSourceSelectionPreservesProofs(
					currentCovered,
					candidateCovered,
					concerns,
				) {
					continue
				}
				applyContextSourceCoverage(&candidate, concerns, candidateCovered)
				candidate, err = finalizeContextEstimate(candidate)
				if err != nil {
					return ContextPack{}, err
				}
				fits, fitErr = contextSourceSubstitutionPackFits(candidate, limits)
				if fitErr != nil {
					return ContextPack{}, fitErr
				}
				if !fits {
					continue
				}
				candidateScore := contextSourceSelectionScoreFor(
					candidate,
					concerns,
					ordered,
					candidateCovered,
				)
				if !betterContextSourceSelection(candidateScore, currentScore) ||
					found && !betterContextSourceSelection(candidateScore, bestScore) {
					continue
				}
				bestPack = candidate
				bestScore = candidateScore
				found = true
			}
		}
		if !found {
			break
		}
		if seen[bestScore.key] {
			return ContextPack{}, fmt.Errorf(
				"context source substitution repeated selection %q",
				bestScore.key,
			)
		}
		seen[bestScore.key] = true
		current = bestPack
		currentScore = bestScore
		currentCovered = contextSourceCoverageFromFinalSections(
			current,
			concerns,
			ordered,
		)
		selected, ok = resolveSelectedContextSourceOptions(
			current,
			ordered,
			boundaries,
		)
		if !ok {
			return ContextPack{}, fmt.Errorf(
				"context source substitution could not resolve selection %q",
				currentScore.key,
			)
		}
	}
	return current, nil
}

func betterContextSourceSelection(
	left contextSourceSelectionScore,
	right contextSourceSelectionScore,
) bool {
	if left.requiredPublicProofs != right.requiredPublicProofs {
		return left.requiredPublicProofs > right.requiredPublicProofs
	}
	if left.requiredProofs != right.requiredProofs {
		return left.requiredProofs > right.requiredProofs
	}
	if left.identityQuality != right.identityQuality {
		return left.identityQuality > right.identityQuality
	}
	if left.estimatedTokens != right.estimatedTokens {
		return left.estimatedTokens < right.estimatedTokens
	}
	return left.key < right.key
}

func orderedContextSourceSelectionOptions(
	options []contextSourceOption,
) []contextSourceOption {
	ordered := append([]contextSourceOption(nil), options...)
	sort.Slice(ordered, func(left, right int) bool {
		if contextSourceOptionLess(ordered[left], ordered[right]) {
			return true
		}
		if contextSourceOptionLess(ordered[right], ordered[left]) {
			return false
		}
		return contextSourceSelectionOptionKey(ordered[left]) <
			contextSourceSelectionOptionKey(ordered[right])
	})
	return ordered
}

func resolveSelectedContextSourceOptions(
	pack ContextPack,
	options []contextSourceOption,
	boundaries []contextSourceBoundary,
) ([]resolvedContextSourceOption, bool) {
	selected := make([]resolvedContextSourceOption, 0, len(pack.SourceSections))
	for _, section := range pack.SourceSections {
		matching := make([]contextSourceOption, 0, 1)
		protected := false
		for _, option := range options {
			if option.section != section {
				continue
			}
			matching = append(matching, option)
			for _, boundary := range boundaries {
				if contextSourceOptionContainsBoundary(option, boundary) {
					protected = true
				}
			}
		}
		if len(matching) == 0 {
			return nil, false
		}
		sort.Slice(matching, func(left, right int) bool {
			return contextSourceSelectionOptionKey(matching[left]) <
				contextSourceSelectionOptionKey(matching[right])
		})
		selected = append(selected, resolvedContextSourceOption{
			option:    matching[0],
			protected: protected,
		})
	}
	sort.Slice(selected, func(left, right int) bool {
		return contextSourceSelectionOptionKey(selected[left].option) <
			contextSourceSelectionOptionKey(selected[right].option)
	})
	return selected, true
}

func contextSourceOptionContainsBoundary(
	option contextSourceOption,
	boundary contextSourceBoundary,
) bool {
	if boundary.factID != "" {
		return contextSourceCandidateHasFact(option.candidate, boundary.factID)
	}
	return boundary.project != "" &&
		option.projectKey == boundary.project &&
		option.candidate.Role != "test"
}

func contextSourceOptionProvesRequiredConcern(
	option contextSourceOption,
	concerns []contextConcern,
) bool {
	for _, concern := range concerns {
		if concern.required && contextSourceOptionHasConcern(option, concern.key) {
			return true
		}
	}
	return false
}

func contextSourceSelectionPreservesProofs(
	previous map[string]bool,
	candidate map[string]bool,
	concerns []contextConcern,
) bool {
	for _, concern := range concerns {
		if concern.required && previous[concern.key] && !candidate[concern.key] {
			return false
		}
	}
	return true
}

func contextSourceSelectionOptionsFit(
	base ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
) (bool, error) {
	pack := cloneContextPack(base)
	pack.SourceSections = nil
	pack.SourceOmissions = nil
	state := newContextSourceSelectionState(len(options), len(concerns))
	for _, option := range orderedContextSourceSelectionOptions(options) {
		fits, err := contextSourceOptionFits(
			pack,
			request,
			option,
			concerns,
			state,
		)
		if err != nil {
			return false, err
		}
		if !fits {
			return false, nil
		}
		pack, state, err = addContextSourceOption(
			pack,
			request,
			option,
			concerns,
			state,
		)
		if err != nil {
			return false, err
		}
	}
	return contextSourceSubstitutionPackFits(pack, request)
}

func contextSourceSubstitutionPackFits(
	pack ContextPack,
	request ContextRequest,
) (bool, error) {
	if len(pack.SourceSections) > MaxContextSourceSections ||
		contextSourceFileCount(pack) > request.MaxFiles {
		return false, nil
	}
	return contextSourcePackFits(pack, request)
}

func boundedContextSourceSubstitutionRequest(
	request ContextRequest,
) ContextRequest {
	if request.BudgetTokens <= 0 ||
		request.BudgetTokens > DefaultContextBudgetTokens {
		request.BudgetTokens = DefaultContextBudgetTokens
	}
	if request.MaxFiles <= 0 || request.MaxFiles > DefaultContextMaxFiles {
		request.MaxFiles = DefaultContextMaxFiles
	}
	return request
}

func contextSourceSelectionScoreFor(
	pack ContextPack,
	concerns []contextConcern,
	options []contextSourceOption,
	covered map[string]bool,
) contextSourceSelectionScore {
	score := contextSourceSelectionScore{
		estimatedTokens: pack.EstimatedTokens,
		key:             contextSourceSelectionKey(pack, options),
	}
	for _, concern := range concerns {
		if concern.required && covered[concern.key] {
			score.requiredProofs++
		}
	}
	score.requiredPublicProofs = contextSourceRequiredPublicProofs(concerns, covered)
	for _, section := range pack.SourceSections {
		best := 0
		for _, option := range options {
			if option.section != section {
				continue
			}
			best = max(
				best,
				contextSourceRequestedIdentityQuality(pack, option),
			)
		}
		score.identityQuality += best
	}
	return score
}

func contextSourceRequiredPublicProofs(
	concerns []contextConcern,
	covered map[string]bool,
) int {
	publicCovered := map[string]bool{}
	for _, concern := range concerns {
		if !concern.required {
			continue
		}
		publicKey := firstNonEmptyContext(concern.publicKey, concern.key)
		if _, seen := publicCovered[publicKey]; !seen {
			publicCovered[publicKey] = true
		}
		if !covered[concern.key] {
			publicCovered[publicKey] = false
		}
	}
	proofs := 0
	for _, complete := range publicCovered {
		if complete {
			proofs++
		}
	}
	return proofs
}

func contextSourceRequestedIdentityQuality(
	pack ContextPack,
	option contextSourceOption,
) int {
	family := contextSourceEffectiveEvidenceFamily(pack, option)
	if family != contextConcernDomainModel &&
		family != contextConcernPersistence &&
		!option.requestedModel &&
		!option.matchesModel {
		return 0
	}
	quality := 100 * min(
		contextSourceEffectiveStableDomainMatches(pack, option),
		3,
	)
	if option.matchesModel {
		quality += 5_000
	}
	if option.requestedModel {
		quality += 10_000
	}
	return quality + contextSourceEffectiveCandidateQuality(pack, option)
}

func contextSourceSelectedCandidateKeys(
	selected []resolvedContextSourceOption,
) map[string]bool {
	keys := make(map[string]bool, len(selected))
	for _, item := range selected {
		keys[contextSourceCandidateKey(item.option.candidate)] = true
	}
	return keys
}

func contextSourceSelectionKey(
	pack ContextPack,
	options []contextSourceOption,
) string {
	keys := make([]string, 0, len(pack.SourceSections))
	for _, section := range pack.SourceSections {
		key := ""
		for _, option := range options {
			if option.section != section {
				continue
			}
			optionKey := contextSourceSelectionOptionKey(option)
			if key == "" || optionKey < key {
				key = optionKey
			}
		}
		if key == "" {
			key = contextSourceSelectionSectionKey(section)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, "\n")
}

func contextSourceSelectionOptionKey(option contextSourceOption) string {
	return contextSourceCandidateKey(option.candidate) + "\x00" +
		contextSourceSelectionSectionKey(option.section)
}

func contextSourceSelectionSectionKey(section ContextSourceSection) string {
	return strings.Join([]string{
		normalizeContextProject(section.Project),
		contextPackSourceFile(section.Path),
		strconv.Itoa(section.StartLine),
		strconv.Itoa(section.EndLine),
		section.Role,
		section.RenderMode,
		section.Content,
	}, "\x00")
}
