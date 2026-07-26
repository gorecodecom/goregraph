package agentbench

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agent"
)

type PackProjection struct {
	Endpoint         string   `json:"endpoint"`
	Entrypoints      []string `json:"entrypoints"`
	CallChain        []string `json:"call_chain"`
	Contracts        []string `json:"contracts"`
	Persistence      []string `json:"persistence"`
	Tests            []string `json:"tests"`
	Sources          []string `json:"sources"`
	Omissions        []string `json:"omissions"`
	Uncertainties    []string `json:"uncertainties"`
	SourceCoverage   string   `json:"source_coverage"`
	EstimatedTokens  int      `json:"estimated_tokens"`
	FallbackRequired bool     `json:"fallback_required"`
	RetryAllowed     bool     `json:"retry_allowed"`
}

type PackDiff struct {
	EndpointChanged       bool     `json:"endpoint_changed"`
	GoldenEndpoint        string   `json:"golden_endpoint"`
	CandidateEndpoint     string   `json:"candidate_endpoint"`
	AddedLocations        []string `json:"added_locations"`
	RemovedLocations      []string `json:"removed_locations"`
	AddedSources          []string `json:"added_sources"`
	RemovedSources        []string `json:"removed_sources"`
	AddedOmissions        []string `json:"added_omissions"`
	RemovedOmissions      []string `json:"removed_omissions"`
	CoverageChanged       bool     `json:"coverage_changed"`
	BudgetChanged         bool     `json:"budget_changed"`
	FallbackChanged       bool     `json:"fallback_changed"`
	RetryChanged          bool     `json:"retry_changed"`
	GoldenRetryAllowed    bool     `json:"golden_retry_allowed"`
	CandidateRetryAllowed bool     `json:"candidate_retry_allowed"`
	UncertaintyChanged    bool     `json:"uncertainty_changed"`
}

type Violation struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func ProjectPack(pack agent.ContextPack) PackProjection {
	endpoints := projectEndpointKeys(pack.Endpoints)

	projection := PackProjection{
		Entrypoints:      projectLocations("entrypoints", pack.Entrypoints),
		CallChain:        projectRelationships(pack.CallChain),
		Contracts:        projectLocations("contracts", pack.Contracts),
		Persistence:      projectLocations("persistence", pack.Persistence),
		Tests:            projectLocations("tests", pack.Tests),
		Sources:          projectSources(pack.Files),
		Omissions:        projectOmissions(pack.SourceOmissions),
		Uncertainties:    projectUncertainties(pack.Uncertainties),
		SourceCoverage:   pack.SourceCoverage,
		EstimatedTokens:  pack.EstimatedTokens,
		FallbackRequired: pack.FallbackRequired,
		RetryAllowed:     pack.RetryAllowed,
	}
	if len(endpoints) > 0 {
		projection.Endpoint = endpoints[0]
	}
	return projection
}

func EvaluatePack(pack agent.ContextPack, expectation PackExpectation) []Violation {
	violations := make([]Violation, 0)
	if expectation.Endpoint != nil && !matchesEndpoint(pack.Endpoints, *expectation.Endpoint) {
		violations = append(violations, Violation{Field: "endpoint", Reason: "no endpoint matches the expected provider, HTTP method, and path"})
	}

	for _, required := range expectation.RequiredLocations {
		if !matchesLocation(pack, required) {
			violations = append(violations, Violation{Field: "required_location", Reason: fmt.Sprintf("required %s location is missing", required.Section)})
		}
	}
	for _, forbidden := range expectation.ForbiddenLocations {
		if matchesLocation(pack, forbidden) {
			violations = append(violations, Violation{Field: "forbidden_location", Reason: fmt.Sprintf("forbidden %s location is present", forbidden.Section)})
		}
	}

	for _, suffix := range expectation.RequiredSourceSuffixes {
		if !hasSourceSuffix(pack.Files, suffix) {
			violations = append(violations, Violation{Field: "required_source", Reason: fmt.Sprintf("source ending in %q is missing", suffix)})
		}
	}
	for _, required := range expectation.RequiredSourceContent {
		violations = append(violations, requiredSourceContentViolations(pack.SourceSections, required)...)
	}
	for _, suffix := range expectation.ForbiddenSourceSuffixes {
		if hasSourceSuffix(pack.Files, suffix) {
			violations = append(violations, Violation{Field: "forbidden_source", Reason: fmt.Sprintf("forbidden source ending in %q is present", suffix)})
		}
	}
	for _, unknown := range expectation.RequiredUnknowns {
		if !hasUnknown(pack.Uncertainties, unknown) {
			violations = append(violations, Violation{Field: "unknown", Reason: fmt.Sprintf("required uncertainty %q is missing", unknown)})
		}
	}

	if expectation.FallbackRequired != nil && pack.FallbackRequired != *expectation.FallbackRequired {
		violations = append(violations, Violation{Field: "fallback_required", Reason: "fallback requirement does not match"})
	}
	if expected := expectation.FallbackReasonContains; strings.TrimSpace(expected) != "" &&
		!strings.Contains(strings.ToLower(pack.FallbackReason), strings.ToLower(expected)) {
		violations = append(violations, Violation{
			Field: "fallback_reason",
			Reason: fmt.Sprintf(
				"fallback reason does not contain expected substring %q",
				expected,
			),
		})
	}
	if expectation.RetryAllowed != nil && pack.RetryAllowed != *expectation.RetryAllowed {
		violations = append(violations, Violation{Field: "retry_allowed", Reason: "retry permission does not match"})
	}
	if expectation.MaxEstimatedTokens > 0 && pack.EstimatedTokens > expectation.MaxEstimatedTokens {
		violations = append(violations, Violation{Field: "estimated_tokens", Reason: "estimated token count exceeds the maximum"})
	}
	if expectation.MaxSourceOmissions >= 0 && len(pack.SourceOmissions) > expectation.MaxSourceOmissions {
		violations = append(violations, Violation{Field: "source_omission", Reason: "source omission count exceeds the maximum"})
	}
	if expectation.RequireBoundedSource && !hasOnlyBoundedSources(pack) {
		violations = append(violations, Violation{Field: "source_omission", Reason: "source evidence or omission is not bounded to a path and line range"})
	}
	return violations
}

func requiredSourceContentViolations(
	sections []agent.ContextSourceSection,
	expectation RequiredSourceContentExpectation,
) []Violation {
	matchedPath := false
	bestMissing := []string(nil)
	bestSectionKey := ""
	for _, section := range sections {
		if !hasSourceSectionSuffix(section, expectation.PathSuffix) {
			continue
		}
		matchedPath = true
		missing := make([]string, 0, len(expectation.Required))
		for _, fragment := range expectation.Required {
			if !strings.Contains(section.Content, fragment) {
				missing = append(missing, fragment)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		sectionKey := sourceKey(
			section.Project,
			section.Path,
			section.StartLine,
			section.EndLine,
			section.Role,
			section.Content,
		)
		if bestMissing == nil ||
			len(missing) < len(bestMissing) ||
			len(missing) == len(bestMissing) && sectionKey < bestSectionKey {
			bestMissing = missing
			bestSectionKey = sectionKey
		}
	}
	if !matchedPath {
		return []Violation{{
			Field:  "required_source_content",
			Reason: fmt.Sprintf("source section ending in %q is missing", expectation.PathSuffix),
		}}
	}
	violations := make([]Violation, 0, len(bestMissing))
	for _, fragment := range bestMissing {
		violations = append(violations, Violation{
			Field: "required_source_content",
			Reason: fmt.Sprintf(
				"source section ending in %q is missing required fragment %q",
				expectation.PathSuffix,
				fragment,
			),
		})
	}
	return violations
}

func hasSourceSectionSuffix(section agent.ContextSourceSection, suffix string) bool {
	return strings.HasSuffix(section.Path, suffix) ||
		strings.HasSuffix(strings.TrimPrefix(section.Project+"/"+section.Path, "/"), suffix)
}

func DiffPacks(golden, candidate agent.ContextPack) PackDiff {
	goldenProjection := ProjectPack(golden)
	candidateProjection := ProjectPack(candidate)
	addedLocations, removedLocations := diffLocationKeys(goldenProjection, candidateProjection)
	addedSources, removedSources := diffKeys(goldenProjection.Sources, candidateProjection.Sources, sourceDisplay)
	addedOmissions, removedOmissions := diffKeys(goldenProjection.Omissions, candidateProjection.Omissions, sourceDisplay)

	return PackDiff{
		EndpointChanged:       !equalStringSlices(projectEndpointKeys(golden.Endpoints), projectEndpointKeys(candidate.Endpoints)),
		GoldenEndpoint:        goldenProjection.Endpoint,
		CandidateEndpoint:     candidateProjection.Endpoint,
		AddedLocations:        addedLocations,
		RemovedLocations:      removedLocations,
		AddedSources:          addedSources,
		RemovedSources:        removedSources,
		AddedOmissions:        addedOmissions,
		RemovedOmissions:      removedOmissions,
		CoverageChanged:       goldenProjection.SourceCoverage != candidateProjection.SourceCoverage,
		BudgetChanged:         goldenProjection.EstimatedTokens != candidateProjection.EstimatedTokens,
		FallbackChanged:       goldenProjection.FallbackRequired != candidateProjection.FallbackRequired,
		RetryChanged:          goldenProjection.RetryAllowed != candidateProjection.RetryAllowed,
		GoldenRetryAllowed:    goldenProjection.RetryAllowed,
		CandidateRetryAllowed: candidateProjection.RetryAllowed,
		UncertaintyChanged:    !equalStringSlices(goldenProjection.Uncertainties, candidateProjection.Uncertainties),
	}
}

func EvaluatePackDiff(diff PackDiff, hypothesis Hypothesis) []Violation {
	violations := make([]Violation, 0)
	if diff.EndpointChanged {
		violations = append(violations, Violation{Field: "endpoint", Reason: "protected endpoint changed"})
	}
	for _, section := range []string{"entrypoints", "call_chain", "contracts", "persistence"} {
		if diffChangesSection(diff, section) {
			violations = append(violations, Violation{Field: section, Reason: "protected pack field changed"})
		}
	}

	allowed := make(map[string]bool, len(hypothesis.AllowedPackChanges))
	for _, change := range hypothesis.AllowedPackChanges {
		allowed[change] = true
	}
	if (len(diff.AddedLocations) > 0 || len(diff.RemovedLocations) > 0) && !allowed["locations"] {
		violations = append(violations, Violation{Field: "locations", Reason: "location changes are not allowed"})
	}
	if (len(diff.AddedSources) > 0 || len(diff.RemovedSources) > 0) && !allowed["sources"] {
		violations = append(violations, Violation{Field: "sources", Reason: "source changes are not allowed"})
	}
	if (len(diff.AddedOmissions) > 0 || len(diff.RemovedOmissions) > 0) && !allowed["omissions"] {
		violations = append(violations, Violation{Field: "omissions", Reason: "omission changes are not allowed"})
	}
	if diff.CoverageChanged && !allowed["coverage"] {
		violations = append(violations, Violation{Field: "coverage", Reason: "coverage changes are not allowed"})
	}
	if diff.BudgetChanged && !allowed["budget"] {
		violations = append(violations, Violation{Field: "budget", Reason: "budget changes are not allowed"})
	}
	if diff.FallbackChanged {
		violations = append(violations, Violation{Field: "fallback_required", Reason: "fallback requirement changes are not allowed"})
	}
	retryChanged := diff.GoldenRetryAllowed != diff.CandidateRetryAllowed
	if diff.RetryChanged != retryChanged {
		violations = append(violations, Violation{
			Field:  "retry_allowed",
			Reason: "retry change metadata is inconsistent",
		})
	} else if retryChanged {
		switch {
		case !allowed["retry_permission"]:
			violations = append(violations, Violation{Field: "retry_allowed", Reason: "retry permission changes are not allowed"})
		case !diff.GoldenRetryAllowed || diff.CandidateRetryAllowed:
			violations = append(violations, Violation{
				Field:  "retry_allowed",
				Reason: "retry permission may only tighten from true to false",
			})
		}
	}
	if diff.UncertaintyChanged {
		violations = append(violations, Violation{Field: "uncertainties", Reason: "uncertainty changes are not allowed"})
	}
	return violations
}

func projectEndpointKeys(endpoints []agent.ContextEndpoint) []string {
	keys := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		keys = append(keys, endpointKey(endpoint))
	}
	return sortedUnique(keys)
}

func endpointKey(endpoint agent.ContextEndpoint) string {
	return strings.Join([]string{endpoint.Provider, endpoint.HTTPMethod, endpoint.Path}, "|")
}

func projectLocations(section string, locations []agent.ContextLocation) []string {
	keys := make([]string, 0, len(locations))
	for _, location := range locations {
		keys = append(keys, strings.Join([]string{section, location.Project, location.Kind, location.Label, location.File, fmt.Sprint(location.Line)}, "|"))
	}
	return sortedUnique(keys)
}

func projectRelationships(relationships []agent.ContextRelationship) []string {
	keys := make([]string, 0, len(relationships))
	for _, relationship := range relationships {
		keys = append(keys, strings.Join([]string{relationship.From, relationship.To, relationship.Kind, relationship.Reason}, "|"))
	}
	return sortedUnique(keys)
}

func projectSources(sources []agent.ContextFile) []string {
	keys := make([]string, 0, len(sources))
	for _, source := range sources {
		keys = append(keys, sourceKey(source.Project, source.Path, source.StartLine, source.EndLine, source.Role, source.Reason))
	}
	return sortedUnique(keys)
}

func projectOmissions(omissions []agent.ContextSourceOmission) []string {
	keys := make([]string, 0, len(omissions))
	for _, omission := range omissions {
		keys = append(keys, sourceKey(omission.Project, omission.Path, omission.StartLine, omission.EndLine, omission.Role, omission.Reason))
	}
	return sortedUnique(keys)
}

func sourceKey(project, path string, startLine, endLine int, role, reason string) string {
	return strings.Join([]string{project, path, fmt.Sprint(startLine), fmt.Sprint(endLine), role, reason}, "|")
}

func projectUncertainties(uncertainties []agent.ContextUncertainty) []string {
	keys := make([]string, 0, len(uncertainties))
	for _, uncertainty := range uncertainties {
		keys = append(keys, strings.Join([]string{uncertainty.Scope, uncertainty.Reason}, "|"))
	}
	return sortedUnique(keys)
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func matchesEndpoint(endpoints []agent.ContextEndpoint, expectation EndpointExpectation) bool {
	for _, endpoint := range endpoints {
		if expectation.ProviderContains != "" && !strings.Contains(endpoint.Provider, expectation.ProviderContains) {
			continue
		}
		if endpoint.HTTPMethod == expectation.HTTPMethod && endpoint.Path == expectation.Path {
			return true
		}
	}
	return false
}

func matchesLocation(pack agent.ContextPack, expectation LocationExpectation) bool {
	for _, location := range locationsForSection(pack, expectation.Section) {
		if expectation.Project != "" && location.Project != expectation.Project {
			continue
		}
		if expectation.Kind != "" && location.Kind != expectation.Kind {
			continue
		}
		if strings.Contains(location.Label, expectation.LabelContains) {
			return true
		}
	}
	return false
}

func locationsForSection(pack agent.ContextPack, section string) []agent.ContextLocation {
	switch section {
	case "entrypoints":
		return pack.Entrypoints
	case "contracts":
		return pack.Contracts
	case "persistence":
		return pack.Persistence
	case "tests":
		return pack.Tests
	default:
		return nil
	}
}

func hasSourceSuffix(sources []agent.ContextFile, suffix string) bool {
	for _, source := range sources {
		if strings.HasSuffix(source.Path, suffix) || strings.HasSuffix(strings.TrimPrefix(source.Project+"/"+source.Path, "/"), suffix) {
			return true
		}
	}
	return false
}

func hasUnknown(uncertainties []agent.ContextUncertainty, expected string) bool {
	for _, uncertainty := range uncertainties {
		if strings.Contains(uncertainty.Scope, expected) || strings.Contains(uncertainty.Reason, expected) {
			return true
		}
	}
	return false
}

func hasOnlyBoundedSources(pack agent.ContextPack) bool {
	for _, source := range pack.Files {
		if !isBoundedSource(source.Path, source.StartLine, source.EndLine) {
			return false
		}
	}
	for _, omission := range pack.SourceOmissions {
		if !isBoundedSource(omission.Path, omission.StartLine, omission.EndLine) {
			return false
		}
	}
	return true
}

func isBoundedSource(path string, startLine, endLine int) bool {
	return path != "" && startLine > 0 && endLine >= startLine
}

func diffLocationKeys(golden, candidate PackProjection) ([]string, []string) {
	goldenKeys := appendLocationKeys(golden)
	candidateKeys := appendLocationKeys(candidate)
	return diffKeys(goldenKeys, candidateKeys, locationDisplay)
}

func appendLocationKeys(projection PackProjection) []string {
	keys := make([]string, 0, len(projection.Entrypoints)+len(projection.CallChain)+len(projection.Contracts)+len(projection.Persistence)+len(projection.Tests))
	keys = append(keys, projection.Entrypoints...)
	for _, relationship := range projection.CallChain {
		keys = append(keys, "call_chain|"+relationship)
	}
	keys = append(keys, projection.Contracts...)
	keys = append(keys, projection.Persistence...)
	keys = append(keys, projection.Tests...)
	return sortedUnique(keys)
}

func diffKeys(golden, candidate []string, display func(string) string) ([]string, []string) {
	goldenSet := toSet(golden)
	candidateSet := toSet(candidate)
	added := make([]string, 0)
	removed := make([]string, 0)
	for _, key := range candidate {
		if !goldenSet[key] {
			added = append(added, display(key))
		}
	}
	for _, key := range golden {
		if !candidateSet[key] {
			removed = append(removed, display(key))
		}
	}
	return sortedUnique(added), sortedUnique(removed)
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func locationDisplay(key string) string {
	if strings.HasPrefix(key, "call_chain|") {
		return key
	}
	parts := strings.Split(key, "|")
	if len(parts) < 4 {
		return key
	}
	return strings.Join([]string{parts[0], parts[1], parts[3]}, "|")
}

func sourceDisplay(key string) string {
	parts := strings.Split(key, "|")
	if len(parts) < 2 {
		return key
	}
	return strings.TrimPrefix(parts[0]+"/"+parts[1], "/")
}

func diffChangesSection(diff PackDiff, section string) bool {
	for _, location := range append(append([]string{}, diff.AddedLocations...), diff.RemovedLocations...) {
		if strings.HasPrefix(location, section+"|") {
			return true
		}
	}
	return false
}
