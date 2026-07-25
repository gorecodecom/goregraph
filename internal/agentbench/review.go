package agentbench

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"
	"time"
)

const logicalRunCount = 3

type RunReview struct {
	Schema            int                    `json:"schema"`
	CaseID            string                 `json:"case_id"`
	Build             string                 `json:"build"`
	Run               int                    `json:"run"`
	Attempt           int                    `json:"attempt"`
	Reviewer          string                 `json:"reviewer"`
	ReviewedAt        string                 `json:"reviewed_at"`
	Signature         string                 `json:"signature"`
	Facets            map[string]FacetResult `json:"facets"`
	ForbiddenOutcomes map[string]FacetResult `json:"forbidden_outcomes"`
}

type FacetResult struct {
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

type RunMetrics struct {
	Tokens                int64 `json:"tokens"`
	ToolCalls             int64 `json:"tool_calls"`
	SourceReads           int64 `json:"source_reads"`
	IncludedSourceRereads int64 `json:"included_source_rereads"`
	RepeatedFullPacks     int64 `json:"repeated_full_packs"`
	ContextCalls          int64 `json:"context_calls"`
	ContextMillis         int64 `json:"context_millis"`
	BroadNavigationCalls  int64 `json:"broad_navigation_calls"`
}

type InvalidRun struct {
	InfrastructureFailure bool   `json:"infrastructure_failure"`
	Reason                string `json:"reason"`
	RetainedLog           string `json:"retained_log"`
}

type ReviewedRun struct {
	Review  RunReview   `json:"review"`
	Metrics RunMetrics  `json:"metrics"`
	Invalid *InvalidRun `json:"invalid,omitempty"`
}

type GateReport struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures"`
}

func LoadReview(path string) (RunReview, error) {
	var review RunReview
	if err := loadJSON(path, &review); err != nil {
		return RunReview{}, err
	}
	if err := validateReview(review); err != nil {
		return RunReview{}, err
	}
	return review, nil
}

func EvaluateCase(contract Contract, golden, candidate []ReviewedRun, hypothesis Hypothesis) GateReport {
	failures := make([]string, 0)
	if err := ValidateContract(contract); err != nil {
		failures = append(failures, fmt.Sprintf("invalid contract: %v", err))
	}
	if hypothesis.TargetFacet == "" {
		failures = append(failures, "hypothesis.target_facet must not be empty")
	}
	if !hasFacet(contract.Answer.RequiredFacets, hypothesis.TargetFacet) {
		failures = append(failures, fmt.Sprintf("target facet %q is not required by the contract", hypothesis.TargetFacet))
	}
	if hasFacet(contract.Answer.ExplicitUnknowns, hypothesis.TargetFacet) {
		failures = append(failures, fmt.Sprintf("target facet %q remains an explicit unknown", hypothesis.TargetFacet))
	}

	goldenValid := collectValidRuns("golden", contract.ID, golden, &failures)
	candidateValid := collectValidRuns("candidate", contract.ID, candidate, &failures)
	if len(failures) > 0 {
		return gateReport(failures)
	}

	requiredFacetIDs := facetIDs(contract.Answer.RequiredFacets)
	explicitUnknownIDs := facetIDs(contract.Answer.ExplicitUnknowns)
	forbiddenOutcomeIDs := facetIDs(contract.Answer.ForbiddenOutcomes)
	for _, run := range logicalRunNumbers() {
		ensureReviewScope("golden", goldenValid[run].Review, requiredFacetIDs, explicitUnknownIDs, forbiddenOutcomeIDs, &failures)
		ensureReviewScope("candidate", candidateValid[run].Review, requiredFacetIDs, explicitUnknownIDs, forbiddenOutcomeIDs, &failures)
	}
	if len(failures) > 0 {
		return gateReport(failures)
	}

	targetPasses := 0
	for _, run := range logicalRunNumbers() {
		goldenReview := goldenValid[run].Review
		review := candidateValid[run].Review
		for _, outcomeID := range forbiddenOutcomeIDs {
			if goldenReview.ForbiddenOutcomes[outcomeID].Status != "pass" {
				failures = append(failures, fmt.Sprintf("%s failed in golden run %d", outcomeID, run))
			}
		}
		for _, facetID := range requiredFacetIDs {
			if facetID == hypothesis.TargetFacet {
				continue
			}
			if review.Facets[facetID].Status != "pass" {
				failures = append(failures, fmt.Sprintf("%s failed in candidate run %d", facetID, run))
			}
		}
		for _, facetID := range explicitUnknownIDs {
			if review.Facets[facetID].Status != "pass" {
				failures = append(failures, fmt.Sprintf("%s failed in candidate run %d", facetID, run))
			}
		}
		if review.Facets[hypothesis.TargetFacet].Status == "pass" {
			targetPasses++
		}
		for _, outcomeID := range forbiddenOutcomeIDs {
			if review.ForbiddenOutcomes[outcomeID].Status != "pass" {
				failures = append(failures, fmt.Sprintf("%s failed in candidate run %d", outcomeID, run))
			}
		}
	}
	if targetPasses < 2 {
		failures = append(failures, fmt.Sprintf("%s passed in %d of %d candidate runs, want at least 2", hypothesis.TargetFacet, targetPasses, logicalRunCount))
	}

	appendEfficiencyFailures(goldenValid, candidateValid, contract.Limits, &failures)
	return gateReport(failures)
}

func Median(values []int64) (int64, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("median requires at least one value")
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(first, second int) bool { return sorted[first] < sorted[second] })
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle], nil
	}
	return sorted[middle-1]/2 + sorted[middle]/2 + (sorted[middle-1]%2+sorted[middle]%2)/2, nil
}

func validateReview(review RunReview) error {
	if review.Schema != contractSchemaVersion {
		return fmt.Errorf("schema must be %d", contractSchemaVersion)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "case_id", value: review.CaseID},
		{name: "build", value: review.Build},
		{name: "reviewer", value: review.Reviewer},
		{name: "reviewed_at", value: review.ReviewedAt},
		{name: "signature", value: review.Signature},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s must not be empty", field.name)
		}
	}
	if review.Run < 1 {
		return fmt.Errorf("run must be positive")
	}
	if review.Attempt < 1 {
		return fmt.Errorf("attempt must be positive")
	}
	if _, err := time.Parse(time.RFC3339, review.ReviewedAt); err != nil {
		return fmt.Errorf("reviewed_at must be RFC3339: %w", err)
	}
	if len(review.Facets) == 0 {
		return fmt.Errorf("facets must contain at least one result")
	}
	if err := validateFacetResults("facets", review.Facets); err != nil {
		return err
	}
	return validateFacetResults("forbidden_outcomes", review.ForbiddenOutcomes)
}

func validateFacetResults(field string, results map[string]FacetResult) error {
	ids := make([]string, 0, len(results))
	for id := range results {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		result := results[id]
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s ID must not be empty", field)
		}
		if result.Status != "pass" && result.Status != "fail" {
			return fmt.Errorf("%s.%s.status must be pass or fail", field, id)
		}
		if strings.TrimSpace(result.Evidence) == "" {
			return fmt.Errorf("%s.%s.evidence must not be empty", field, id)
		}
	}
	return nil
}

func collectValidRuns(build, caseID string, runs []ReviewedRun, failures *[]string) map[int]ReviewedRun {
	byLogicalRun := make(map[int][]ReviewedRun)
	for _, run := range runs {
		if err := validateReview(run.Review); err != nil {
			*failures = append(*failures, fmt.Sprintf("%s run %d attempt %d has invalid review: %v", build, run.Review.Run, run.Review.Attempt, err))
		}
		if err := validateMetrics(run.Metrics); err != nil {
			*failures = append(*failures, fmt.Sprintf("%s run %d attempt %d has invalid metrics: %v", build, run.Review.Run, run.Review.Attempt, err))
		}
		if run.Review.CaseID != caseID {
			*failures = append(*failures, fmt.Sprintf("%s run %d reviews case %s, want %s", build, run.Review.Run, run.Review.CaseID, caseID))
		}
		if run.Review.Build != build {
			*failures = append(*failures, fmt.Sprintf("%s run %d has build %s, want %s", build, run.Review.Run, run.Review.Build, build))
		}
		if run.Invalid != nil {
			if !run.Invalid.InfrastructureFailure {
				*failures = append(*failures, fmt.Sprintf("%s run %d attempt %d is marked invalid without an infrastructure failure", build, run.Review.Run, run.Review.Attempt))
			}
			if strings.TrimSpace(run.Invalid.Reason) == "" {
				*failures = append(*failures, fmt.Sprintf("%s run %d attempt %d has an invalid record without a reason", build, run.Review.Run, run.Review.Attempt))
			}
			if strings.TrimSpace(run.Invalid.RetainedLog) == "" {
				*failures = append(*failures, fmt.Sprintf("%s run %d attempt %d has an invalid record without a retained log", build, run.Review.Run, run.Review.Attempt))
			}
		}
		if run.Review.Run < 1 || run.Review.Run > logicalRunCount {
			*failures = append(*failures, fmt.Sprintf("%s has unexpected logical run %d", build, run.Review.Run))
			continue
		}
		byLogicalRun[run.Review.Run] = append(byLogicalRun[run.Review.Run], run)
	}

	valid := make(map[int]ReviewedRun, logicalRunCount)
	for _, logicalRun := range logicalRunNumbers() {
		attempts := byLogicalRun[logicalRun]
		if len(attempts) == 0 {
			*failures = append(*failures, fmt.Sprintf("%s is missing logical run %d", build, logicalRun))
			continue
		}
		sort.Slice(attempts, func(first, second int) bool {
			return attempts[first].Review.Attempt < attempts[second].Review.Attempt
		})
		validCount := 0
		for index, attempt := range attempts {
			if attempt.Review.Attempt != index+1 {
				*failures = append(*failures, fmt.Sprintf("%s run %d must retain consecutive attempts starting at 1", build, logicalRun))
				break
			}
			if attempt.Invalid == nil {
				validCount++
				valid[logicalRun] = attempt
				if index != len(attempts)-1 {
					*failures = append(*failures, fmt.Sprintf("%s run %d replaces a valid attempt", build, logicalRun))
				}
			}
		}
		if validCount != 1 {
			*failures = append(*failures, fmt.Sprintf("%s run %d has %d valid attempts, want exactly 1", build, logicalRun, validCount))
		}
	}
	return valid
}

func validateMetrics(metrics RunMetrics) error {
	for _, field := range []struct {
		name  string
		value int64
	}{
		{name: "tokens", value: metrics.Tokens},
		{name: "tool_calls", value: metrics.ToolCalls},
		{name: "source_reads", value: metrics.SourceReads},
		{name: "included_source_rereads", value: metrics.IncludedSourceRereads},
		{name: "repeated_full_packs", value: metrics.RepeatedFullPacks},
		{name: "context_calls", value: metrics.ContextCalls},
		{name: "context_millis", value: metrics.ContextMillis},
		{name: "broad_navigation_calls", value: metrics.BroadNavigationCalls},
	} {
		if field.value < 0 {
			return fmt.Errorf("%s must not be negative", field.name)
		}
	}
	return nil
}

func ensureReviewScope(build string, review RunReview, required, explicitUnknowns, forbidden []string, failures *[]string) {
	for _, facetID := range append(append([]string{}, required...), explicitUnknowns...) {
		if _, ok := review.Facets[facetID]; !ok {
			*failures = append(*failures, fmt.Sprintf("%s run %d is missing facet %s", build, review.Run, facetID))
		}
	}
	for _, outcomeID := range forbidden {
		if _, ok := review.ForbiddenOutcomes[outcomeID]; !ok {
			*failures = append(*failures, fmt.Sprintf("%s run %d is missing forbidden outcome %s", build, review.Run, outcomeID))
		}
	}
	if len(review.Facets) != len(required)+len(explicitUnknowns) {
		*failures = append(*failures, fmt.Sprintf("%s run %d has unexpected facets", build, review.Run))
	}
	if len(review.ForbiddenOutcomes) != len(forbidden) {
		*failures = append(*failures, fmt.Sprintf("%s run %d has unexpected forbidden outcomes", build, review.Run))
	}
}

func appendEfficiencyFailures(golden, candidate map[int]ReviewedRun, limits EfficiencyLimits, failures *[]string) {
	goldenMetrics := metricValues(golden)
	candidateMetrics := metricValues(candidate)

	goldenToolCalls, _ := Median(goldenMetrics.toolCalls)
	candidateToolCalls, _ := Median(candidateMetrics.toolCalls)
	if candidateToolCalls > goldenToolCalls {
		*failures = append(*failures, fmt.Sprintf("candidate median tool calls %d exceeds golden median %d", candidateToolCalls, goldenToolCalls))
	}
	goldenSourceReads, _ := Median(goldenMetrics.sourceReads)
	candidateSourceReads, _ := Median(candidateMetrics.sourceReads)
	if candidateSourceReads > goldenSourceReads {
		*failures = append(*failures, fmt.Sprintf("candidate median source reads %d exceeds golden median %d", candidateSourceReads, goldenSourceReads))
	}
	goldenTokens, _ := Median(goldenMetrics.tokens)
	candidateTokens, _ := Median(candidateMetrics.tokens)
	if !withinRatio(candidateTokens, goldenTokens, 100+int64(limits.MaxTokenIncreasePercent)) {
		*failures = append(*failures, fmt.Sprintf("candidate median tokens %d exceeds %d percent of golden median %d", candidateTokens, 100+limits.MaxTokenIncreasePercent, goldenTokens))
	}
	goldenLatency, _ := Median(goldenMetrics.contextMillis)
	candidateLatency, _ := Median(candidateMetrics.contextMillis)
	if !withinRatio(candidateLatency, goldenLatency, 100+int64(limits.MaxLatencyIncreasePercent)) {
		*failures = append(*failures, fmt.Sprintf("candidate median Context latency %d exceeds %d percent of golden median %d", candidateLatency, 100+limits.MaxLatencyIncreasePercent, goldenLatency))
	}
	for _, run := range logicalRunNumbers() {
		candidateLatency := candidate[run].Metrics.ContextMillis
		goldenLatency := golden[run].Metrics.ContextMillis
		if !withinMultiplier(candidateLatency, goldenLatency, int64(limits.MaxPairedLatencyMultiplier)) {
			*failures = append(*failures, fmt.Sprintf("candidate Context latency %d exceeds twice golden latency %d for run %d", candidateLatency, goldenLatency, run))
		}
	}
}

type metricsByRun struct {
	tokens        []int64
	toolCalls     []int64
	sourceReads   []int64
	contextMillis []int64
}

func metricValues(runs map[int]ReviewedRun) metricsByRun {
	metrics := metricsByRun{}
	for _, run := range logicalRunNumbers() {
		metrics.tokens = append(metrics.tokens, runs[run].Metrics.Tokens)
		metrics.toolCalls = append(metrics.toolCalls, runs[run].Metrics.ToolCalls)
		metrics.sourceReads = append(metrics.sourceReads, runs[run].Metrics.SourceReads)
		metrics.contextMillis = append(metrics.contextMillis, runs[run].Metrics.ContextMillis)
	}
	return metrics
}

func withinRatio(candidate, golden, percent int64) bool {
	leftHigh, leftLow := bits.Mul64(uint64(candidate), 100)
	rightHigh, rightLow := bits.Mul64(uint64(golden), uint64(percent))
	return leftHigh < rightHigh || leftHigh == rightHigh && leftLow <= rightLow
}

func withinMultiplier(candidate, golden, multiplier int64) bool {
	leftHigh, leftLow := bits.Mul64(uint64(candidate), 1)
	rightHigh, rightLow := bits.Mul64(uint64(golden), uint64(multiplier))
	return leftHigh < rightHigh || leftHigh == rightHigh && leftLow <= rightLow
}

func hasFacet(facets []FacetDefinition, id string) bool {
	for _, facet := range facets {
		if facet.ID == id {
			return true
		}
	}
	return false
}

func facetIDs(facets []FacetDefinition) []string {
	ids := make([]string, 0, len(facets))
	for _, facet := range facets {
		ids = append(ids, facet.ID)
	}
	return ids
}

func logicalRunNumbers() []int {
	return []int{1, 2, 3}
}

func gateReport(failures []string) GateReport {
	return GateReport{Passed: len(failures) == 0, Failures: failures}
}
