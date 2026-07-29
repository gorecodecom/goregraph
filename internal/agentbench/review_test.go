package agentbench

import (
	"strings"
	"testing"
)

func TestLoadReview(t *testing.T) {
	t.Run("loads a complete signed review", func(t *testing.T) {
		path := writeJSON(t, completeReview(1, 1))

		review, err := LoadReview(path)
		if err != nil {
			t.Fatalf("LoadReview returned error: %v", err)
		}
		if review.CaseID != "g2-java-missing-contract" || review.Facets["current-path"].Status != "pass" {
			t.Fatalf("LoadReview = %#v, want the complete review", review)
		}
	})

	t.Run("loads a review without forbidden outcomes", func(t *testing.T) {
		expected := completeReview(1, 1)
		expected.ForbiddenOutcomes = nil

		review, err := LoadReview(writeJSON(t, expected))
		if err != nil {
			t.Fatalf("LoadReview returned error: %v", err)
		}
		if len(review.ForbiddenOutcomes) != 0 {
			t.Fatalf("LoadReview forbidden outcomes = %#v, want none", review.ForbiddenOutcomes)
		}
	})

	for _, test := range []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "rejects unknown fields",
			content: `{"schema":1,"case_id":"g2-java-missing-contract","build":"candidate","run":1,"attempt":1,"reviewer":"reviewer","reviewed_at":"2026-07-25T10:00:00Z","signature":"signed","facets":{"current-path":{"status":"pass","evidence":"The answer follows the deletion path."}},"forbidden_outcomes":{"invented-edge":{"status":"pass","evidence":"The answer did not claim this edge exists."}},"unexpected":true}`,
			wantErr: "unexpected",
		},
		{
			name:    "rejects unsupported facet status",
			content: `{"schema":1,"case_id":"g2-java-missing-contract","build":"candidate","run":1,"attempt":1,"reviewer":"reviewer","reviewed_at":"2026-07-25T10:00:00Z","signature":"signed","facets":{"current-path":{"status":"unknown","evidence":"Unclear."}},"forbidden_outcomes":{"invented-edge":{"status":"pass","evidence":"The answer did not claim this edge exists."}}}`,
			wantErr: "facets.current-path.status",
		},
		{
			name:    "rejects blank evidence",
			content: `{"schema":1,"case_id":"g2-java-missing-contract","build":"candidate","run":1,"attempt":1,"reviewer":"reviewer","reviewed_at":"2026-07-25T10:00:00Z","signature":"signed","facets":{"current-path":{"status":"pass","evidence":" "}},"forbidden_outcomes":{"invented-edge":{"status":"pass","evidence":"The answer did not claim this edge exists."}}}`,
			wantErr: "facets.current-path.evidence",
		},
		{
			name:    "rejects trailing JSON",
			content: string(mustJSON(t, completeReview(1, 1))) + " {}",
			wantErr: "trailing",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadReview(writeText(t, test.content))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("LoadReview error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestEvaluateCase(t *testing.T) {
	t.Run("accepts retained quality and bounded metrics", func(t *testing.T) {
		contract, golden, candidate, hypothesis, diff := passingCase()

		report := EvaluateCase(contract, golden, candidate, hypothesis, diff)
		if !report.Passed || len(report.Failures) != 0 {
			t.Fatalf("passing report = %#v", report)
		}
	})

	tests := []struct {
		name    string
		mutate  func([]ReviewedRun, []ReviewedRun)
		failure string
	}{
		{
			name: "rejects a previously passing facet failure",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[1].Review.Facets["current-path"] = FacetResult{
					Status:   "fail",
					Evidence: "The answer selected the unrelated list endpoint.",
				}
			},
			failure: "current-path failed in candidate run 2",
		},
		{
			name: "rejects a review for another case",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[0].Review.CaseID = "g3-go-existing-flow"
			},
			failure: "candidate run 1 reviews case g3-go-existing-flow, want g2-java-missing-contract",
		},
		{
			name: "rejects a candidate review marked as golden",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[0].Review.Build = "golden"
			},
			failure: "candidate run 1 has build golden, want candidate",
		},
		{
			name: "rejects a golden review marked as candidate",
			mutate: func(golden []ReviewedRun, _ []ReviewedRun) {
				golden[0].Review.Build = "candidate"
			},
			failure: "golden run 1 has build candidate, want golden",
		},
		{
			name: "rejects a forbidden outcome in golden",
			mutate: func(golden []ReviewedRun, _ []ReviewedRun) {
				golden[1].Review.ForbiddenOutcomes["invented-edge"] = FacetResult{
					Status:   "fail",
					Evidence: "The Golden answer asserted the absent edge exists.",
				}
			},
			failure: "invented-edge failed in golden run 2",
		},
		{
			name: "rejects a forbidden outcome",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[2].Review.ForbiddenOutcomes["invented-edge"] = FacetResult{
					Status:   "fail",
					Evidence: "The answer asserted the absent edge exists.",
				}
			},
			failure: "invented-edge failed in candidate run 3",
		},
		{
			name: "rejects an explicit unknown failure",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[0].Review.Facets["unknown-side-effect"] = FacetResult{
					Status:   "fail",
					Evidence: "The answer invented a side effect instead of retaining the uncertainty.",
				}
			},
			failure: "unknown-side-effect failed in candidate run 1",
		},
		{
			name: "rejects only one target improvement",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[1].Review.Facets["missing-contract"] = FacetResult{
					Status:   "fail",
					Evidence: "The answer still treats the contract as present.",
				}
			},
			failure: "missing-contract passed in 1 of 3 candidate runs",
		},
		{
			name: "rejects a tool call median increase",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				for index := range candidate {
					candidate[index].Metrics.ToolCalls = 11
				}
			},
			failure: "candidate median tool calls 11 exceeds golden median 10",
		},
		{
			name: "rejects an unauthorized source read median increase",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				for index := range candidate {
					candidate[index].Metrics.UnauthorizedSourceReads = 1
				}
			},
			failure: "candidate median unauthorized source reads 1 exceeds golden median 0",
		},
		{
			name: "rejects bounded omission reads above the contract",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[0].Metrics.BoundedOmissionReads = 4
				candidate[0].Metrics.SourceReads = 14
			},
			failure: "candidate bounded omission reads 4 exceeds contract maximum 3 for run 1",
		},
		{
			name: "rejects a negative unauthorized source metric",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[0].Metrics.UnauthorizedSourceReads = -1
			},
			failure: "unauthorized_source_read_calls must not be negative",
		},
		{
			name: "rejects token median above five percent",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				for index := range candidate {
					candidate[index].Metrics.Tokens = 106
				}
			},
			failure: "candidate median tokens 106 exceeds 105 percent of golden median 100",
		},
		{
			name: "rejects latency median above ten percent",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				for index := range candidate {
					candidate[index].Metrics.ContextMillis = 111
				}
			},
			failure: "candidate median Context latency 111 exceeds 110 percent of golden median 100",
		},
		{
			name: "rejects a paired latency above twice golden",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[1].Metrics.ContextMillis = 201
			},
			failure: "candidate Context latency 201 exceeds twice golden latency 100 for run 2",
		},
		{
			name: "rejects an inaccurate run marked invalid",
			mutate: func(_ []ReviewedRun, candidate []ReviewedRun) {
				candidate[0].Invalid = &InvalidRun{
					InfrastructureFailure: false,
					Reason:                "The answer was inaccurate.",
					RetainedLog:           "candidate-run-1.jsonl",
				}
			},
			failure: "candidate run 1 attempt 1 is marked invalid without an infrastructure failure",
		},
	}

	t.Run("rejects an appended fourth logical run after a valid failure", func(t *testing.T) {
		contract, golden, candidate, hypothesis, diff := passingCase()
		candidate = append(candidate, reviewedRun(4, "candidate", "pass", RunMetrics{
			Tokens: 105, ToolCalls: 10, SourceReads: 10, ContextMillis: 110,
		}))

		requireGateFailure(t, EvaluateCase(contract, golden, candidate, hypothesis, diff), "candidate has unexpected logical run 4")
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract, golden, candidate, hypothesis, diff := passingCase()
			test.mutate(golden, candidate)

			requireGateFailure(t, EvaluateCase(contract, golden, candidate, hypothesis, diff), test.failure)
		})
	}

	t.Run("accepts total reads added only through bounded omissions", func(t *testing.T) {
		contract, golden, candidate, hypothesis, diff := passingCase()
		for index := range candidate {
			candidate[index].Metrics.SourceReads = 13
			candidate[index].Metrics.BoundedOmissionReads = 3
		}

		report := EvaluateCase(contract, golden, candidate, hypothesis, diff)
		if !report.Passed || len(report.Failures) != 0 {
			t.Fatalf("bounded omission report = %#v", report)
		}
	})

	t.Run("retains unchanged-pack efficiency drift as model variance", func(t *testing.T) {
		contract, golden, candidate, hypothesis, _ := passingCase()
		for index := range candidate {
			candidate[index].Metrics.ToolCalls = 11
			candidate[index].Metrics.UnauthorizedSourceReads = 1
			candidate[index].Metrics.Tokens = 106
			candidate[index].Metrics.ContextMillis = 201
		}

		report := EvaluateCase(contract, golden, candidate, hypothesis, PackDiff{})
		if !report.Passed || len(report.Failures) != 0 {
			t.Fatalf("unchanged-Pack report = %#v, want pass", report)
		}
		for _, fragment := range []string{
			"model variance",
			"candidate median tool calls",
			"candidate median unauthorized source reads",
			"candidate median tokens",
			"candidate median Context latency",
			"candidate Context latency",
		} {
			requireObservation(t, report, fragment)
		}
	})

	t.Run("rejects bounded omission reads for unchanged packs", func(t *testing.T) {
		contract, golden, candidate, hypothesis, _ := passingCase()
		candidate[0].Metrics.BoundedOmissionReads = 4
		candidate[0].Metrics.SourceReads = 14

		requireGateFailure(
			t,
			EvaluateCase(contract, golden, candidate, hypothesis, PackDiff{}),
			"candidate bounded omission reads 4 exceeds contract maximum 3 for run 1",
		)
	})

	t.Run("accepts an infrastructure replacement in the same logical run", func(t *testing.T) {
		contract, golden, candidate, hypothesis, diff := passingCase()
		failedAttempt := candidate[0]
		failedAttempt.Invalid = &InvalidRun{
			InfrastructureFailure: true,
			Reason:                "The local Codex process exited before responding.",
			RetainedLog:           "candidate-run-1-attempt-1.jsonl",
		}
		candidate[0].Review.Attempt = 2
		candidate = append([]ReviewedRun{failedAttempt}, candidate...)

		report := EvaluateCase(contract, golden, candidate, hypothesis, diff)
		if !report.Passed || len(report.Failures) != 0 {
			t.Fatalf("replacement report = %#v", report)
		}
	})

	t.Run("accepts a contract without forbidden outcomes", func(t *testing.T) {
		contract, golden, candidate, hypothesis, diff := passingCase()
		contract.Answer.ForbiddenOutcomes = nil
		for index := range golden {
			golden[index].Review.ForbiddenOutcomes = nil
			candidate[index].Review.ForbiddenOutcomes = nil
		}

		report := EvaluateCase(contract, golden, candidate, hypothesis, diff)
		if !report.Passed || len(report.Failures) != 0 {
			t.Fatalf("zero-forbidden-outcome report = %#v", report)
		}
	})
}

func TestMedian(t *testing.T) {
	for _, test := range []struct {
		name    string
		values  []int64
		want    int64
		wantErr string
	}{
		{name: "returns the middle value without changing input order", values: []int64{9, 1, 5}, want: 5},
		{name: "truncates the even median to an integer", values: []int64{8, 1, 3, 4}, want: 3},
		{name: "rejects no values", wantErr: "at least one"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Median(test.values)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("Median error = %v, want error containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("Median(%v) = %d, %v; want %d, nil", test.values, got, err, test.want)
			}
		})
	}
}

func completeReview(run, attempt int) RunReview {
	return RunReview{
		Schema:     1,
		CaseID:     "g2-java-missing-contract",
		Build:      "candidate",
		Run:        run,
		Attempt:    attempt,
		Reviewer:   "reviewer",
		ReviewedAt: "2026-07-25T10:00:00Z",
		Signature:  "signed",
		Facets: map[string]FacetResult{
			"current-path": {Status: "pass", Evidence: "The answer follows the deletion path."},
		},
		ForbiddenOutcomes: map[string]FacetResult{
			"invented-edge": {Status: "pass", Evidence: "The answer does not claim the absent edge exists."},
		},
	}
}

func passingCase() (Contract, []ReviewedRun, []ReviewedRun, Hypothesis, PackDiff) {
	contract := validContract()
	contract.Answer.RequiredFacets = []FacetDefinition{
		{ID: "current-path", Description: "Explains the observed catalog deletion path."},
		{ID: "missing-contract", Description: "Marks the requested job deletion contract as absent."},
	}
	contract.Answer.ExplicitUnknowns = []FacetDefinition{
		{ID: "unknown-side-effect", Description: "States that the side effect cannot be confirmed."},
	}
	hypothesis := Hypothesis{TargetFacet: "missing-contract"}
	golden := make([]ReviewedRun, 3)
	candidate := make([]ReviewedRun, 3)
	for index := range golden {
		golden[index] = reviewedRun(index+1, "golden", "fail", RunMetrics{
			Tokens: 100, ToolCalls: 10, SourceReads: 10, ContextMillis: 100,
		})
		status := "pass"
		if index == 2 {
			status = "fail"
		}
		candidate[index] = reviewedRun(index+1, "candidate", status, RunMetrics{
			Tokens: 105, ToolCalls: 10, SourceReads: 10, ContextMillis: 110,
		})
	}
	diff := PackDiff{AddedSources: []string{"services/catalog/NewEvidence.java"}}
	return contract, golden, candidate, hypothesis, diff
}

func reviewedRun(run int, build, targetStatus string, metrics RunMetrics) ReviewedRun {
	review := completeReview(run, 1)
	review.Build = build
	review.Facets["missing-contract"] = FacetResult{
		Status:   targetStatus,
		Evidence: "The answer identifies whether the requested contract is present.",
	}
	review.Facets["unknown-side-effect"] = FacetResult{
		Status:   "pass",
		Evidence: "The answer explicitly retains the unconfirmed side effect as unknown.",
	}
	return ReviewedRun{Review: review, Metrics: metrics}
}

func requireGateFailure(t *testing.T, report GateReport, want string) {
	t.Helper()
	if report.Passed {
		t.Fatalf("gate report passed, want failure containing %q: %#v", want, report)
	}
	for _, failure := range report.Failures {
		if strings.Contains(failure, want) {
			return
		}
	}
	t.Fatalf("gate failures = %#v, want one containing %q", report.Failures, want)
}

func requireObservation(t *testing.T, report GateReport, want string) {
	t.Helper()
	for _, observation := range report.Observations {
		if strings.Contains(observation, want) {
			return
		}
	}
	t.Fatalf("gate observations = %#v, want one containing %q", report.Observations, want)
}
