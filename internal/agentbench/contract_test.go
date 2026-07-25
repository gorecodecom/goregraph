package agentbench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validContract() Contract {
	fallback := false
	return Contract{
		Schema: 1,
		ID:     "g2-java-missing-contract",
		Queries: []QueryVariant{
			{ID: "en", Language: "en", File: "query.en.txt", EndToEnd: true},
			{ID: "de", Language: "de", File: "query.de.txt"},
		},
		Pack: PackExpectation{
			Endpoint: &EndpointExpectation{
				HTTPMethod: "DELETE",
				Path:       "/catalog/{itemId}",
			},
			RequiredLocations: []LocationExpectation{
				{Section: "persistence", Project: "services/catalog", LabelContains: "deleteById"},
			},
			ForbiddenLocations: []LocationExpectation{
				{Section: "entrypoints", Project: "services/jobs", LabelContains: "listJobs"},
			},
			FallbackRequired:     &fallback,
			MaxEstimatedTokens:   4000,
			MaxSourceOmissions:   3,
			RequireBoundedSource: true,
		},
		Answer: AnswerExpectation{
			RequiredFacets: []FacetDefinition{
				{ID: "current-path", Description: "Explains the observed catalog deletion path."},
				{ID: "missing-contract", Description: "Marks the requested job deletion contract as absent."},
			},
			ForbiddenOutcomes: []FacetDefinition{
				{ID: "invented-edge", Description: "Claims that the missing deletion call already exists."},
			},
		},
		Limits: EfficiencyLimits{
			ContextTokens:              4000,
			MaxSourceOmissions:         3,
			MaxTokenIncreasePercent:    5,
			MaxLatencyIncreasePercent:  10,
			MaxPairedLatencyMultiplier: 2,
		},
	}
}

func TestLoadContract(t *testing.T) {
	t.Run("loads a complete contract", func(t *testing.T) {
		path := writeJSON(t, validContract())

		contract, err := LoadContract(path)
		if err != nil {
			t.Fatalf("LoadContract returned error: %v", err)
		}
		if contract.ID != "g2-java-missing-contract" {
			t.Fatalf("LoadContract ID = %q, want %q", contract.ID, "g2-java-missing-contract")
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		path := writeText(t, `{"schema":1,"id":"g2","queries":[],"pack":{},"answer":{},"limits":{},"unexpected":true}`)

		_, err := LoadContract(path)
		if err == nil || !strings.Contains(err.Error(), "unexpected") {
			t.Fatalf("LoadContract error = %v, want unknown-field error naming unexpected", err)
		}
	})

	t.Run("rejects trailing JSON", func(t *testing.T) {
		path := writeText(t, string(mustJSON(t, validContract()))+" {}")

		_, err := LoadContract(path)
		if err == nil || !strings.Contains(err.Error(), "trailing") {
			t.Fatalf("LoadContract error = %v, want trailing-data error", err)
		}
	})
}

func TestValidateContract(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Contract)
		wantErr string
	}{
		{
			name: "accepts complete contract",
		},
		{
			name: "rejects duplicate facet IDs across outcome categories",
			mutate: func(contract *Contract) {
				contract.Answer.ForbiddenOutcomes[0].ID = "current-path"
			},
			wantErr: "answer.forbidden_outcomes[0].id",
		},
		{
			name: "rejects empty evidence descriptions",
			mutate: func(contract *Contract) {
				contract.Answer.RequiredFacets[0].Description = ""
			},
			wantErr: "answer.required_facets[0].description",
		},
		{
			name: "rejects unsupported schema versions",
			mutate: func(contract *Contract) {
				contract.Schema = 2
			},
			wantErr: "schema",
		},
		{
			name: "rejects a budget other than 4000",
			mutate: func(contract *Contract) {
				contract.Pack.MaxEstimatedTokens = 4001
			},
			wantErr: "pack.max_estimated_tokens",
		},
		{
			name: "rejects more than three source omissions",
			mutate: func(contract *Contract) {
				contract.Limits.MaxSourceOmissions = 4
			},
			wantErr: "limits.max_source_omissions",
		},
		{
			name: "rejects contracts without query variants",
			mutate: func(contract *Contract) {
				contract.Queries = nil
			},
			wantErr: "queries",
		},
		{
			name: "rejects contracts without exactly one end-to-end query",
			mutate: func(contract *Contract) {
				contract.Queries[1].EndToEnd = true
			},
			wantErr: "queries",
		},
		{
			name: "rejects overlapping required and forbidden locations",
			mutate: func(contract *Contract) {
				contract.Pack.ForbiddenLocations[0] = contract.Pack.RequiredLocations[0]
			},
			wantErr: "pack.forbidden_locations[0]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract := validContract()
			if test.mutate != nil {
				test.mutate(&contract)
			}

			err := ValidateContract(contract)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateContract returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateContract error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateMatrix(t *testing.T) {
	valid := Matrix{
		Schema: 1,
		Cases:  []MatrixCase{{ID: "g2", Directory: "g2-java-missing-contract"}},
	}
	if err := ValidateMatrix(valid); err != nil {
		t.Fatalf("ValidateMatrix returned error: %v", err)
	}

	invalid := valid
	invalid.Cases = append(invalid.Cases, MatrixCase{ID: "g2", Directory: "duplicate"})
	if err := ValidateMatrix(invalid); err == nil || !strings.Contains(err.Error(), "cases[1].id") {
		t.Fatalf("ValidateMatrix error = %v, want duplicate case ID error", err)
	}
}

func TestValidateHypothesis(t *testing.T) {
	matrix := Matrix{
		Schema: 1,
		Cases:  []MatrixCase{{ID: "g2", Directory: "g2-java-missing-contract"}},
	}
	hypothesis := Hypothesis{
		Schema:              1,
		ID:                  "improve-g2-location-selection",
		TargetFacet:         "current-path",
		TargetCases:         []string{"g1", "g2"},
		AllowedPackChanges:  []string{"locations", "budget"},
		ProtectedPackFields: []string{"endpoint", "entrypoints", "call_chain", "contracts", "persistence"},
	}
	if err := ValidateHypothesis(hypothesis, matrix); err != nil {
		t.Fatalf("ValidateHypothesis returned error: %v", err)
	}

	hypothesis.AllowedPackChanges = []string{"unknown"}
	if err := ValidateHypothesis(hypothesis, matrix); err == nil || !strings.Contains(err.Error(), "allowed_pack_changes[0]") {
		t.Fatalf("ValidateHypothesis error = %v, want invalid allowed change error", err)
	}
}

func writeJSON(t *testing.T, value any) string {
	t.Helper()
	return writeText(t, string(mustJSON(t, value)))
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return data
}

func writeText(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "contract.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	return path
}
