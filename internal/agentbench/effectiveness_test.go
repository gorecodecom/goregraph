package agentbench

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEffectivenessFixtureValidatesRunnableFiles(t *testing.T) {
	directory := t.TempDir()
	project := filepath.Join(directory, "workspace", "app")
	if err := os.MkdirAll(filepath.Join(project, "test"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(project, "source.js"), filepath.Join(project, "test", "visible.test.mjs"), filepath.Join(project, "test", "acceptance.test.mjs")} {
		if err := os.WriteFile(path, []byte("export {};\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	task := validEffectivenessManifest().Tasks[0]
	task.Verification.Visible = []string{"visible"}
	task.Verification.Acceptance = []string{"acceptance"}
	fixture := EffectivenessFixture{Schema: 1, Scenario: "runnable", Projects: []EffectivenessFixtureProject{{Root: "workspace/app", Language: "javascript", Files: []string{"workspace/app/source.js"}}}, VisibleChecks: []EffectivenessFixtureCheck{{Name: "visible", WorkingDirectory: "workspace/app", Command: []string{"node", "--test", "test/visible.test.mjs"}, Files: []string{"workspace/app/test/visible.test.mjs"}}}, AcceptanceChecks: []EffectivenessFixtureCheck{{Name: "acceptance", WorkingDirectory: "workspace/app", Command: []string{"node", "--test", "test/acceptance.test.mjs"}, Files: []string{"workspace/app/test/acceptance.test.mjs"}}}}
	body, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(directory, "fixture.json")
	if err := os.WriteFile(fixturePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEffectivenessFixture(fixturePath, task); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(project, "test", "acceptance.test.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEffectivenessFixture(fixturePath, task); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing acceptance file error = %v", err)
	}
}

func TestValidateEffectivenessManifestRejectsInvalidTasks(t *testing.T) {
	valid := validEffectivenessManifest()
	tests := []struct {
		name    string
		mutate  func(*EffectivenessManifest)
		wantErr string
	}{
		{name: "missing ID", mutate: func(m *EffectivenessManifest) { m.Tasks[0].ID = "" }, wantErr: "tasks[0].id"},
		{name: "duplicate ID", mutate: func(m *EffectivenessManifest) { m.Tasks = append(m.Tasks, m.Tasks[0]) }, wantErr: "tasks[2].id"},
		{name: "empty verification", mutate: func(m *EffectivenessManifest) { m.Tasks[0].Verification = TaskVerification{} }, wantErr: "verification.kind"},
		{name: "missing failure criteria", mutate: func(m *EffectivenessManifest) { m.Tasks[0].ForbiddenOutcomes = nil }, wantErr: "forbidden_outcomes"},
		{name: "change task without hidden acceptance", mutate: func(m *EffectivenessManifest) { m.Tasks[0].Verification.Acceptance = nil }, wantErr: "verification.acceptance"},
		{name: "analysis task without evidence rubric", mutate: func(m *EffectivenessManifest) { m.Tasks[1].Verification.EvidenceCriteria = nil }, wantErr: "verification.evidence_criteria"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := validEffectivenessManifest()
			test.mutate(&manifest)
			err := ValidateEffectivenessManifest(manifest)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
	if err := ValidateEffectivenessManifest(valid); err != nil {
		t.Fatalf("valid manifest: %v", err)
	}
}

func TestSummarizeEffectivenessKeepsFailuresAndMissingUsage(t *testing.T) {
	attempts := []EffectivenessAttempt{
		validEffectivenessAttempt("ordinary", true, true, true),
		validEffectivenessAttempt("ordinary", false, false, false),
	}
	attempts[1].FailureReason = "timeout"
	attempts[1].Repetition = 2
	attempts[1].InputTokens = 0
	attempts[1].CachedInputTokens = 0
	attempts[1].OutputTokens = 0
	attempts[1].ReasoningOutputTokens = 0

	report, err := SummarizeEffectiveness(attempts)
	if err != nil {
		t.Fatal(err)
	}
	got := report.Treatments[0]
	if got.Attempts != 2 || got.Completed != 1 || got.Failed != 1 || got.UsageRecorded != 1 || got.UsageMissing != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if got.CompletionRate != 0.5 || got.CorrectCompletionRate != 0.5 {
		t.Fatalf("rates = completion %v, correct %v", got.CompletionRate, got.CorrectCompletionRate)
	}
	if got.TotalTokens != 125 || got.EffectiveTokens != 105 {
		t.Fatalf("tokens = total %d, effective %d", got.TotalTokens, got.EffectiveTokens)
	}
}

func TestSummarizeEffectivenessCountsCompletedIncorrectAttemptAsFailure(t *testing.T) {
	attempt := validEffectivenessAttempt("ordinary", true, false, true)
	attempt.FailureReason = "changed the wrong route"

	report, err := SummarizeEffectiveness([]EffectivenessAttempt{attempt})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Treatments[0]
	if got.Completed != 1 || got.CriticalIncorrect != 1 || got.Failed != 1 || got.FailureRate != 1 {
		t.Fatalf("summary = %#v", got)
	}
}

func TestSummarizeEffectivenessDoesNotDoubleCountCachedOrReasoningTokens(t *testing.T) {
	attempt := validEffectivenessAttempt("adaptive-v2", true, true, true)
	attempt.InputTokens = 100
	attempt.CachedInputTokens = 80
	attempt.OutputTokens = 25
	attempt.ReasoningOutputTokens = 10

	report, err := SummarizeEffectiveness([]EffectivenessAttempt{attempt})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Treatments[0]
	if got.TotalTokens != 125 || got.EffectiveTokens != 45 || got.OutputTokens != 25 || got.ReasoningOutputTokens != 10 {
		t.Fatalf("token summary = %#v", got)
	}
	pair := report.Pairs[0].Outcomes[0]
	if pair.DurationMilliseconds != 100 || pair.EffectiveTokens == nil || *pair.EffectiveTokens != 45 {
		t.Fatalf("paired outcome = %#v", pair)
	}
}

func TestSummarizeEffectivenessRejectsMismatchedPairedSnapshots(t *testing.T) {
	ordinary := validEffectivenessAttempt("ordinary", true, true, true)
	adaptive := validEffectivenessAttempt("adaptive-v2", true, true, true)
	adaptive.SnapshotID = "snapshot-b"

	_, err := SummarizeEffectiveness([]EffectivenessAttempt{ordinary, adaptive})
	if err == nil || !strings.Contains(err.Error(), "different snapshots") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateEffectivenessAttemptRejectsInvalidCountersAndIdentity(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*EffectivenessAttempt)
		wantErr string
	}{
		{name: "missing treatment", mutate: func(a *EffectivenessAttempt) { a.Treatment = "" }, wantErr: "treatment"},
		{name: "negative duration", mutate: func(a *EffectivenessAttempt) { a.DurationMilliseconds = -1 }, wantErr: "duration_milliseconds"},
		{name: "negative tool calls", mutate: func(a *EffectivenessAttempt) { a.ToolCalls = -1 }, wantErr: "tool_calls"},
		{name: "token overflow", mutate: func(a *EffectivenessAttempt) {
			a.InputTokens = math.MaxInt64
			a.OutputTokens = 1
			a.ReasoningOutputTokens = 0
		}, wantErr: "overflows"},
		{name: "correct unfinished", mutate: func(a *EffectivenessAttempt) { a.Completed = false }, wantErr: "correct"},
		{name: "missing usage with counters", mutate: func(a *EffectivenessAttempt) { a.UsageRecorded = false }, wantErr: "usage_recorded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attempt := validEffectivenessAttempt("strict-v1", true, true, true)
			test.mutate(&attempt)
			err := ValidateEffectivenessAttempt(attempt)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestSummarizeEffectivenessReportsScenariosAndBreakEven(t *testing.T) {
	ordinary := validEffectivenessAttempt("ordinary", true, true, true)
	ordinary.DurationMilliseconds = 1000
	adaptive := validEffectivenessAttempt("adaptive-v2", true, true, true)
	adaptive.DurationMilliseconds = 600
	adaptive.IndexSetupMilliseconds = 1000
	adaptive.IndexUpdateMilliseconds = 100

	report, err := SummarizeEffectiveness([]EffectivenessAttempt{ordinary, adaptive})
	if err != nil {
		t.Fatal(err)
	}
	if report.ObservedBreakEvenTasks == nil || *report.ObservedBreakEvenTasks != 2.75 {
		t.Fatalf("break even = %#v", report.ObservedBreakEvenTasks)
	}
}

func TestEffectivenessGateDoesNotEvaluateUnpairedTreatments(t *testing.T) {
	ordinary := validEffectivenessAttempt("ordinary", true, true, true)
	adaptive := validEffectivenessAttempt("adaptive-v2", true, true, true)
	adaptive.CaseID = "different-case"

	report, err := SummarizeEffectiveness([]EffectivenessAttempt{ordinary, adaptive})
	if err != nil {
		t.Fatal(err)
	}
	if report.Gate.Status != "not_evaluated" || report.Gate.Passed {
		t.Fatalf("gate = %#v", report.Gate)
	}
}

func TestSummarizeEffectivenessRejectsActivityCounterOverflow(t *testing.T) {
	first := validEffectivenessAttempt("ordinary", true, true, true)
	first.ToolCalls = math.MaxInt
	second := validEffectivenessAttempt("ordinary", true, true, true)
	second.Repetition = 2
	second.ToolCalls = 1

	_, err := SummarizeEffectiveness([]EffectivenessAttempt{first, second})
	if err == nil || !strings.Contains(err.Error(), "tool_calls") || !strings.Contains(err.Error(), "overflows") {
		t.Fatalf("error = %v", err)
	}
}

func validEffectivenessManifest() EffectivenessManifest {
	return EffectivenessManifest{Schema: 1, Suite: "development", Tasks: []EffectivenessTask{
		{
			Schema: 1, ID: "java-fix", Family: "java-spring", TaskKind: "fix",
			Languages: []string{"de", "en"}, Prompts: map[string]string{"de": "java-fix/prompt.de.txt", "en": "java-fix/prompt.en.txt"},
			Fixture: "java-fix/fixture.json", RequiredOutcomes: []string{"Returns 404 for a missing item"},
			ForbiddenOutcomes: []string{"Returns 200 for a missing item"},
			Verification:      TaskVerification{Kind: "fixture_tests", Expected: "pass", Visible: []string{"ItemControllerTest"}, Acceptance: []string{"MissingItemAcceptanceTest"}},
		},
		{
			Schema: 1, ID: "route-trace", Family: "cross-project", TaskKind: "investigation",
			Languages: []string{"de", "en"}, Prompts: map[string]string{"de": "route-trace/prompt.de.txt", "en": "route-trace/prompt.en.txt"},
			Fixture: "route-trace/fixture.json", RequiredOutcomes: []string{"Names the provider route"},
			ForbiddenOutcomes: []string{"Invents a second provider"},
			Verification:      TaskVerification{Kind: "blinded_source_evidence", Expected: "rubric", EvidenceCriteria: []string{"Cites Gateway.ts and CatalogController.java"}},
		},
	}}
}

func validEffectivenessAttempt(treatment string, completed, correct, usage bool) EffectivenessAttempt {
	return EffectivenessAttempt{
		Schema: 1, CaseID: "case-a", Language: "en", Treatment: treatment,
		SnapshotID: "snapshot-a", CandidateCommit: "0123456789abcdef", Repetition: 1,
		Completed: completed, Correct: correct, DurationMilliseconds: 100,
		UsageRecorded: usage, InputTokens: 100, CachedInputTokens: 20, OutputTokens: 25,
		ReasoningOutputTokens: 10, ToolCalls: 3, SourceReads: 2, RubricScore: 1,
	}
}
