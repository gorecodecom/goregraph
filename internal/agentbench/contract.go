package agentbench

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	contractSchemaVersion = 1
	fixedContextTokens    = 4000
	maxSourceOmissions    = 3
)

var allowedPackChangeKinds = map[string]bool{
	"locations": true,
	"sources":   true,
	"coverage":  true,
	"omissions": true,
	"budget":    true,
}

var protectedPackFields = map[string]bool{
	"endpoint":    true,
	"entrypoints": true,
	"call_chain":  true,
	"contracts":   true,
	"persistence": true,
}

type Contract struct {
	Schema  int               `json:"schema"`
	ID      string            `json:"id"`
	Queries []QueryVariant    `json:"queries"`
	Pack    PackExpectation   `json:"pack"`
	Answer  AnswerExpectation `json:"answer"`
	Limits  EfficiencyLimits  `json:"limits"`
}

type QueryVariant struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	File     string `json:"file"`
	EndToEnd bool   `json:"end_to_end"`
}

type PackExpectation struct {
	Endpoint                *EndpointExpectation  `json:"endpoint,omitempty"`
	RequiredLocations       []LocationExpectation `json:"required_locations"`
	ForbiddenLocations      []LocationExpectation `json:"forbidden_locations"`
	RequiredSourceSuffixes  []string              `json:"required_source_suffixes"`
	ForbiddenSourceSuffixes []string              `json:"forbidden_source_suffixes"`
	RequiredUnknowns        []string              `json:"required_unknowns"`
	FallbackRequired        *bool                 `json:"fallback_required,omitempty"`
	RetryAllowed            *bool                 `json:"retry_allowed,omitempty"`
	MaxEstimatedTokens      int                   `json:"max_estimated_tokens"`
	MaxSourceOmissions      int                   `json:"max_source_omissions"`
	RequireBoundedSource    bool                  `json:"require_bounded_source"`
}

type EndpointExpectation struct {
	ProviderContains string `json:"provider_contains,omitempty"`
	HTTPMethod       string `json:"http_method"`
	Path             string `json:"path"`
}

type LocationExpectation struct {
	Section       string `json:"section"`
	Project       string `json:"project,omitempty"`
	Kind          string `json:"kind,omitempty"`
	LabelContains string `json:"label_contains"`
}

type AnswerExpectation struct {
	RequiredFacets    []FacetDefinition `json:"required_facets"`
	ForbiddenOutcomes []FacetDefinition `json:"forbidden_outcomes"`
	ExplicitUnknowns  []FacetDefinition `json:"explicit_unknowns"`
}

type FacetDefinition struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type EfficiencyLimits struct {
	ContextTokens              int `json:"context_tokens"`
	MaxSourceOmissions         int `json:"max_source_omissions"`
	MaxTokenIncreasePercent    int `json:"max_token_increase_percent"`
	MaxLatencyIncreasePercent  int `json:"max_latency_increase_percent"`
	MaxPairedLatencyMultiplier int `json:"max_paired_latency_multiplier"`
}

type Matrix struct {
	Schema int          `json:"schema"`
	Cases  []MatrixCase `json:"cases"`
}

type MatrixCase struct {
	ID        string `json:"id"`
	Directory string `json:"directory"`
	External  bool   `json:"external"`
}

type Hypothesis struct {
	Schema              int      `json:"schema"`
	ID                  string   `json:"id"`
	TargetFacet         string   `json:"target_facet"`
	TargetCases         []string `json:"target_cases"`
	AllowedPackChanges  []string `json:"allowed_pack_changes"`
	ProtectedPackFields []string `json:"protected_pack_fields"`
}

func LoadContract(path string) (Contract, error) {
	var contract Contract
	if err := loadJSON(path, &contract); err != nil {
		return Contract{}, err
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func LoadMatrix(path string) (Matrix, error) {
	var matrix Matrix
	if err := loadJSON(path, &matrix); err != nil {
		return Matrix{}, err
	}
	if err := ValidateMatrix(matrix); err != nil {
		return Matrix{}, err
	}
	return matrix, nil
}

func LoadHypothesis(path string) (Hypothesis, error) {
	var hypothesis Hypothesis
	if err := loadJSON(path, &hypothesis); err != nil {
		return Hypothesis{}, err
	}
	return hypothesis, nil
}

func ValidateContract(contract Contract) error {
	if contract.Schema != contractSchemaVersion {
		return fmt.Errorf("schema must be %d", contractSchemaVersion)
	}
	if contract.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if len(contract.Queries) == 0 {
		return fmt.Errorf("queries must contain at least one variant")
	}

	queryIDs := make(map[string]bool, len(contract.Queries))
	endToEndQueries := 0
	for index, query := range contract.Queries {
		field := fmt.Sprintf("queries[%d]", index)
		if query.ID == "" {
			return fmt.Errorf("%s.id must not be empty", field)
		}
		if queryIDs[query.ID] {
			return fmt.Errorf("%s.id duplicates an earlier query ID", field)
		}
		queryIDs[query.ID] = true
		if query.Language == "" {
			return fmt.Errorf("%s.language must not be empty", field)
		}
		if query.File == "" {
			return fmt.Errorf("%s.file must not be empty", field)
		}
		if query.EndToEnd {
			endToEndQueries++
		}
	}
	if endToEndQueries != 1 {
		return fmt.Errorf("queries must contain exactly one end-to-end variant")
	}

	if err := validatePack(contract.Pack); err != nil {
		return err
	}
	if err := validateFacets(contract.Answer); err != nil {
		return err
	}
	if err := validateLimits(contract.Limits); err != nil {
		return err
	}
	return nil
}

func ValidateMatrix(matrix Matrix) error {
	if matrix.Schema != contractSchemaVersion {
		return fmt.Errorf("schema must be %d", contractSchemaVersion)
	}
	if len(matrix.Cases) == 0 {
		return fmt.Errorf("cases must contain at least one case")
	}

	caseIDs := make(map[string]bool, len(matrix.Cases))
	for index, benchmarkCase := range matrix.Cases {
		field := fmt.Sprintf("cases[%d]", index)
		if benchmarkCase.ID == "" {
			return fmt.Errorf("%s.id must not be empty", field)
		}
		if caseIDs[benchmarkCase.ID] {
			return fmt.Errorf("%s.id duplicates an earlier case ID", field)
		}
		caseIDs[benchmarkCase.ID] = true
		if benchmarkCase.Directory == "" {
			return fmt.Errorf("%s.directory must not be empty", field)
		}
	}
	return nil
}

func ValidateHypothesis(hypothesis Hypothesis, matrix Matrix) error {
	if err := ValidateMatrix(matrix); err != nil {
		return fmt.Errorf("matrix: %w", err)
	}
	if hypothesis.Schema != contractSchemaVersion {
		return fmt.Errorf("schema must be %d", contractSchemaVersion)
	}
	if hypothesis.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if hypothesis.TargetFacet == "" {
		return fmt.Errorf("target_facet must not be empty")
	}
	if len(hypothesis.TargetCases) == 0 {
		return fmt.Errorf("target_cases must contain at least one case")
	}

	caseIDs := map[string]bool{"g1": true}
	for _, benchmarkCase := range matrix.Cases {
		caseIDs[benchmarkCase.ID] = true
	}
	targetCases := make(map[string]bool, len(hypothesis.TargetCases))
	for index, targetCase := range hypothesis.TargetCases {
		field := fmt.Sprintf("target_cases[%d]", index)
		if !caseIDs[targetCase] {
			return fmt.Errorf("%s references unknown case %q", field, targetCase)
		}
		if targetCases[targetCase] {
			return fmt.Errorf("%s duplicates an earlier target case", field)
		}
		targetCases[targetCase] = true
	}

	if len(hypothesis.AllowedPackChanges) == 0 {
		return fmt.Errorf("allowed_pack_changes must contain at least one change kind")
	}
	allowedChanges := make(map[string]bool, len(hypothesis.AllowedPackChanges))
	for index, change := range hypothesis.AllowedPackChanges {
		field := fmt.Sprintf("allowed_pack_changes[%d]", index)
		if !allowedPackChangeKinds[change] {
			return fmt.Errorf("%s must be one of locations, sources, coverage, omissions, budget", field)
		}
		if allowedChanges[change] {
			return fmt.Errorf("%s duplicates an earlier change kind", field)
		}
		allowedChanges[change] = true
	}

	if len(hypothesis.ProtectedPackFields) != len(protectedPackFields) {
		return fmt.Errorf("protected_pack_fields must contain endpoint, entrypoints, call_chain, contracts, persistence")
	}
	protectedFields := make(map[string]bool, len(hypothesis.ProtectedPackFields))
	for index, fieldName := range hypothesis.ProtectedPackFields {
		field := fmt.Sprintf("protected_pack_fields[%d]", index)
		if !protectedPackFields[fieldName] {
			return fmt.Errorf("%s is not a protected pack field", field)
		}
		if protectedFields[fieldName] {
			return fmt.Errorf("%s duplicates an earlier protected pack field", field)
		}
		protectedFields[fieldName] = true
	}
	return nil
}

func loadJSON(path string, destination any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}

	var extra any
	err = decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("decode %s: trailing JSON value", path)
	}
	return fmt.Errorf("decode %s: trailing data: %w", path, err)
}

func validatePack(pack PackExpectation) error {
	if pack.Endpoint != nil {
		if pack.Endpoint.HTTPMethod == "" {
			return fmt.Errorf("pack.endpoint.http_method must not be empty")
		}
		if pack.Endpoint.Path == "" {
			return fmt.Errorf("pack.endpoint.path must not be empty")
		}
	}
	if pack.MaxEstimatedTokens != fixedContextTokens {
		return fmt.Errorf("pack.max_estimated_tokens must be %d", fixedContextTokens)
	}
	if pack.MaxSourceOmissions < 0 || pack.MaxSourceOmissions > maxSourceOmissions {
		return fmt.Errorf("pack.max_source_omissions must be between 0 and %d", maxSourceOmissions)
	}

	for index, location := range pack.RequiredLocations {
		if err := validateLocation(location, fmt.Sprintf("pack.required_locations[%d]", index)); err != nil {
			return err
		}
	}
	for index, location := range pack.ForbiddenLocations {
		if err := validateLocation(location, fmt.Sprintf("pack.forbidden_locations[%d]", index)); err != nil {
			return err
		}
		for _, required := range pack.RequiredLocations {
			if locationsOverlap(required, location) {
				return fmt.Errorf("pack.forbidden_locations[%d] overlaps a required location matcher", index)
			}
		}
	}
	return nil
}

func validateLocation(location LocationExpectation, field string) error {
	if location.Section == "" {
		return fmt.Errorf("%s.section must not be empty", field)
	}
	if location.LabelContains == "" {
		return fmt.Errorf("%s.label_contains must not be empty", field)
	}
	return nil
}

func locationsOverlap(first, second LocationExpectation) bool {
	return first.Section == second.Section &&
		(first.Project == "" || second.Project == "" || first.Project == second.Project) &&
		(first.Kind == "" || second.Kind == "" || first.Kind == second.Kind) &&
		(strings.Contains(first.LabelContains, second.LabelContains) || strings.Contains(second.LabelContains, first.LabelContains))
}

func validateFacets(answer AnswerExpectation) error {
	facetIDs := make(map[string]bool)
	for _, category := range []struct {
		field  string
		facets []FacetDefinition
	}{
		{field: "answer.required_facets", facets: answer.RequiredFacets},
		{field: "answer.forbidden_outcomes", facets: answer.ForbiddenOutcomes},
		{field: "answer.explicit_unknowns", facets: answer.ExplicitUnknowns},
	} {
		for index, facet := range category.facets {
			field := fmt.Sprintf("%s[%d]", category.field, index)
			if facet.ID == "" {
				return fmt.Errorf("%s.id must not be empty", field)
			}
			if facetIDs[facet.ID] {
				return fmt.Errorf("%s.id duplicates an earlier facet ID", field)
			}
			facetIDs[facet.ID] = true
			if facet.Description == "" {
				return fmt.Errorf("%s.description must not be empty", field)
			}
		}
	}
	return nil
}

func validateLimits(limits EfficiencyLimits) error {
	if limits.ContextTokens != fixedContextTokens {
		return fmt.Errorf("limits.context_tokens must be %d", fixedContextTokens)
	}
	if limits.MaxSourceOmissions < 0 || limits.MaxSourceOmissions > maxSourceOmissions {
		return fmt.Errorf("limits.max_source_omissions must be between 0 and %d", maxSourceOmissions)
	}
	if limits.MaxTokenIncreasePercent != 5 {
		return fmt.Errorf("limits.max_token_increase_percent must be 5")
	}
	if limits.MaxLatencyIncreasePercent != 10 {
		return fmt.Errorf("limits.max_latency_increase_percent must be 10")
	}
	if limits.MaxPairedLatencyMultiplier != 2 {
		return fmt.Errorf("limits.max_paired_latency_multiplier must be 2")
	}
	return nil
}
