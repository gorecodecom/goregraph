package agentbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agentmetrics"
)

const effectivenessSchemaVersion = 1

// EffectivenessManifest defines a versioned task suite independent of GoreGraph output schemas.
type EffectivenessManifest struct {
	Schema int                 `json:"schema"`
	Suite  string              `json:"suite"`
	Tasks  []EffectivenessTask `json:"tasks"`
}

// EffectivenessTask defines one bilingual synthetic task and its external evaluation contract.
type EffectivenessTask struct {
	Schema            int               `json:"schema"`
	ID                string            `json:"id"`
	Family            string            `json:"family"`
	TaskKind          string            `json:"task_kind"`
	Languages         []string          `json:"languages"`
	Prompts           map[string]string `json:"prompts"`
	Fixture           string            `json:"fixture"`
	RequiredOutcomes  []string          `json:"required_outcomes"`
	ForbiddenOutcomes []string          `json:"forbidden_outcomes"`
	Verification      TaskVerification  `json:"verification"`
}

// TaskVerification separates editable visible checks from evaluator-owned acceptance evidence.
type TaskVerification struct {
	Kind             string   `json:"kind"`
	Expected         string   `json:"expected"`
	Visible          []string `json:"visible,omitempty"`
	Acceptance       []string `json:"acceptance,omitempty"`
	EvidenceCriteria []string `json:"evidence_criteria,omitempty"`
}

// EffectivenessFixture describes a runnable synthetic workspace and its checks.
type EffectivenessFixture struct {
	Schema           int                           `json:"schema"`
	Scenario         string                        `json:"scenario"`
	Projects         []EffectivenessFixtureProject `json:"projects"`
	StaleTransition  *EffectivenessStaleTransition `json:"stale_transition,omitempty"`
	VisibleChecks    []EffectivenessFixtureCheck   `json:"visible_checks,omitempty"`
	AcceptanceChecks []EffectivenessFixtureCheck   `json:"acceptance_checks,omitempty"`
	EvidenceFiles    []string                      `json:"evidence_files,omitempty"`
}

// EffectivenessFixtureProject identifies one project boundary and its source files.
type EffectivenessFixtureProject struct {
	Root     string   `json:"root"`
	Language string   `json:"language"`
	Files    []string `json:"files"`
}

// EffectivenessFixtureCheck defines one executable visible or evaluator-owned check.
type EffectivenessFixtureCheck struct {
	Name             string   `json:"name"`
	WorkingDirectory string   `json:"working_directory"`
	Command          []string `json:"command"`
	Files            []string `json:"files"`
}

// EffectivenessStaleTransition separates bytes used for indexing from live source bytes.
type EffectivenessStaleTransition struct {
	IndexedFile string `json:"indexed_file"`
	LiveFile    string `json:"live_file"`
	Description string `json:"description"`
}

// LoadEffectivenessFixture strictly loads and validates a synthetic fixture definition.
func LoadEffectivenessFixture(path string, task EffectivenessTask) (EffectivenessFixture, error) {
	var fixture EffectivenessFixture
	if err := loadJSON(path, &fixture); err != nil {
		return EffectivenessFixture{}, err
	}
	if err := validateEffectivenessFixture(filepath.Dir(path), task, fixture); err != nil {
		return EffectivenessFixture{}, err
	}
	return fixture, nil
}

func validateEffectivenessFixture(base string, task EffectivenessTask, fixture EffectivenessFixture) error {
	if fixture.Schema != effectivenessSchemaVersion {
		return fmt.Errorf("fixture schema must be %d", effectivenessSchemaVersion)
	}
	if strings.TrimSpace(fixture.Scenario) == "" || len(fixture.Projects) == 0 {
		return fmt.Errorf("fixture scenario and projects must not be empty")
	}
	for index, project := range fixture.Projects {
		if strings.TrimSpace(project.Language) == "" || len(project.Files) == 0 {
			return fmt.Errorf("projects[%d] language and files must not be empty", index)
		}
		root, err := fixturePath(base, project.Root, true)
		if err != nil {
			return fmt.Errorf("projects[%d].root: %w", index, err)
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("projects[%d].root %q is missing", index, project.Root)
		}
		cleanRoot := filepath.Clean(filepath.FromSlash(project.Root)) + string(filepath.Separator)
		for _, relative := range project.Files {
			clean := filepath.Clean(filepath.FromSlash(relative))
			if !strings.HasPrefix(clean, cleanRoot) {
				return fmt.Errorf("projects[%d] file %q is outside project root", index, relative)
			}
			if _, err := fixturePath(base, relative, false); err != nil {
				return fmt.Errorf("projects[%d] file: %w", index, err)
			}
		}
	}
	if fixture.StaleTransition != nil {
		if strings.TrimSpace(fixture.StaleTransition.Description) == "" {
			return fmt.Errorf("stale_transition.description must not be empty")
		}
		for field, relative := range map[string]string{"indexed_file": fixture.StaleTransition.IndexedFile, "live_file": fixture.StaleTransition.LiveFile} {
			if _, err := fixturePath(base, relative, false); err != nil {
				return fmt.Errorf("stale_transition.%s: %w", field, err)
			}
		}
	}
	if err := validateFixtureChecks(base, "visible_checks", fixture.VisibleChecks); err != nil {
		return err
	}
	if err := validateFixtureChecks(base, "acceptance_checks", fixture.AcceptanceChecks); err != nil {
		return err
	}
	if task.Verification.Kind == "fixture_tests" {
		if err := requireCheckNames("visible_checks", fixture.VisibleChecks, task.Verification.Visible); err != nil {
			return err
		}
		if err := requireCheckNames("acceptance_checks", fixture.AcceptanceChecks, task.Verification.Acceptance); err != nil {
			return err
		}
	} else if len(fixture.VisibleChecks) != 0 || len(fixture.AcceptanceChecks) != 0 {
		return fmt.Errorf("blinded_source_evidence fixture must not define executable checks")
	}
	if task.Verification.Kind == "blinded_source_evidence" && len(fixture.EvidenceFiles) == 0 {
		return fmt.Errorf("evidence_files must not be empty")
	}
	for _, relative := range fixture.EvidenceFiles {
		if _, err := fixturePath(base, relative, false); err != nil {
			return fmt.Errorf("evidence_files: %w", err)
		}
	}
	return nil
}

func validateFixtureChecks(base, field string, checks []EffectivenessFixtureCheck) error {
	seen := map[string]bool{}
	for index, check := range checks {
		if strings.TrimSpace(check.Name) == "" || seen[check.Name] || len(check.Command) == 0 || strings.TrimSpace(check.Command[0]) == "" || len(check.Files) == 0 {
			return fmt.Errorf("%s[%d] must have a unique name, command, and files", field, index)
		}
		seen[check.Name] = true
		workingDirectory, err := fixturePath(base, check.WorkingDirectory, true)
		if err != nil {
			return fmt.Errorf("%s[%d].working_directory: %w", field, index, err)
		}
		info, err := os.Stat(workingDirectory)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("%s[%d].working_directory %q is missing", field, index, check.WorkingDirectory)
		}
		for _, relative := range check.Files {
			if _, err := fixturePath(base, relative, false); err != nil {
				return fmt.Errorf("%s[%d].files: %w", field, index, err)
			}
		}
	}
	return nil
}

func requireCheckNames(field string, checks []EffectivenessFixtureCheck, expected []string) error {
	if len(checks) != len(expected) {
		return fmt.Errorf("%s count does not match manifest", field)
	}
	want := map[string]bool{}
	for _, name := range expected {
		want[name] = true
	}
	for _, check := range checks {
		if !want[check.Name] {
			return fmt.Errorf("%s check %q is not named by manifest", field, check.Name)
		}
	}
	return nil
}

func fixturePath(base, relative string, allowDirectory bool) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if relative == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q must remain below the fixture directory", relative)
	}
	path := filepath.Join(base, clean)
	if allowDirectory {
		return path, nil
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("file %q is missing", relative)
	}
	return path, nil
}

// LoadEffectivenessManifest loads and strictly decodes a task suite manifest.
func LoadEffectivenessManifest(path string) (EffectivenessManifest, error) {
	var manifest EffectivenessManifest
	if err := loadJSON(path, &manifest); err != nil {
		return EffectivenessManifest{}, err
	}
	if err := ValidateEffectivenessManifest(manifest); err != nil {
		return EffectivenessManifest{}, err
	}
	return manifest, nil
}

// ValidateEffectivenessManifest validates task identities, outcomes, prompts, and verification shape.
func ValidateEffectivenessManifest(manifest EffectivenessManifest) error {
	if manifest.Schema != effectivenessSchemaVersion {
		return fmt.Errorf("schema must be %d", effectivenessSchemaVersion)
	}
	if strings.TrimSpace(manifest.Suite) == "" {
		return fmt.Errorf("suite must not be empty")
	}
	if len(manifest.Tasks) == 0 {
		return fmt.Errorf("tasks must contain at least one task")
	}
	ids := make(map[string]bool, len(manifest.Tasks))
	for index, task := range manifest.Tasks {
		field := fmt.Sprintf("tasks[%d]", index)
		if task.Schema != effectivenessSchemaVersion {
			return fmt.Errorf("%s.schema must be %d", field, effectivenessSchemaVersion)
		}
		if strings.TrimSpace(task.ID) == "" {
			return fmt.Errorf("%s.id must not be empty", field)
		}
		if ids[task.ID] {
			return fmt.Errorf("%s.id duplicates an earlier task", field)
		}
		ids[task.ID] = true
		if strings.TrimSpace(task.Family) == "" || strings.TrimSpace(task.TaskKind) == "" {
			return fmt.Errorf("%s family and task_kind must not be empty", field)
		}
		if len(task.Languages) == 0 {
			return fmt.Errorf("%s.languages must not be empty", field)
		}
		seenLanguages := map[string]bool{}
		for languageIndex, language := range task.Languages {
			if strings.TrimSpace(language) == "" || seenLanguages[language] {
				return fmt.Errorf("%s.languages[%d] is empty or duplicated", field, languageIndex)
			}
			seenLanguages[language] = true
			if strings.TrimSpace(task.Prompts[language]) == "" {
				return fmt.Errorf("%s.prompts[%q] must name a prompt file", field, language)
			}
		}
		if strings.TrimSpace(task.Fixture) == "" {
			return fmt.Errorf("%s.fixture must not be empty", field)
		}
		if err := requireNonBlankList(field+".required_outcomes", task.RequiredOutcomes); err != nil {
			return err
		}
		if err := requireNonBlankList(field+".forbidden_outcomes", task.ForbiddenOutcomes); err != nil {
			return err
		}
		if strings.TrimSpace(task.Verification.Kind) == "" {
			return fmt.Errorf("%s.verification.kind must not be empty", field)
		}
		if strings.TrimSpace(task.Verification.Expected) == "" {
			return fmt.Errorf("%s.verification.expected must not be empty", field)
		}
		switch task.Verification.Kind {
		case "fixture_tests":
			if err := requireNonBlankList(field+".verification.visible", task.Verification.Visible); err != nil {
				return err
			}
			if err := requireNonBlankList(field+".verification.acceptance", task.Verification.Acceptance); err != nil {
				return err
			}
			for _, visible := range task.Verification.Visible {
				for _, acceptance := range task.Verification.Acceptance {
					if visible == acceptance {
						return fmt.Errorf("%s.verification acceptance must be separate from visible tests", field)
					}
				}
			}
		case "blinded_source_evidence":
			if err := requireNonBlankList(field+".verification.evidence_criteria", task.Verification.EvidenceCriteria); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s.verification.kind is unsupported", field)
		}
	}
	return nil
}

func requireNonBlankList(field string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", field)
	}
	for index, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] must not be empty", field, index)
		}
	}
	return nil
}

// EffectivenessAttempt records one externally executed task attempt and its measured outcome.
type EffectivenessAttempt struct {
	Schema                  int     `json:"schema"`
	CaseID                  string  `json:"case_id"`
	Language                string  `json:"language"`
	Treatment               string  `json:"treatment"`
	SnapshotID              string  `json:"snapshot_id"`
	CandidateCommit         string  `json:"candidate_commit"`
	Repetition              int     `json:"repetition"`
	Completed               bool    `json:"completed"`
	Correct                 bool    `json:"correct"`
	FailureReason           string  `json:"failure_reason,omitempty"`
	RubricScore             float64 `json:"rubric_score"`
	DurationMilliseconds    int64   `json:"duration_milliseconds"`
	ColdStart               bool    `json:"cold_start"`
	IndexSetupMilliseconds  int64   `json:"index_setup_milliseconds"`
	IndexUpdateMilliseconds int64   `json:"index_update_milliseconds"`
	UsageRecorded           bool    `json:"usage_recorded"`
	InputTokens             int64   `json:"input_tokens"`
	CachedInputTokens       int64   `json:"cached_input_tokens"`
	OutputTokens            int64   `json:"output_tokens"`
	ReasoningOutputTokens   int64   `json:"reasoning_output_tokens"`
	ToolCalls               int     `json:"tool_calls"`
	SourceReads             int     `json:"source_reads"`
	Retries                 int     `json:"retries"`
	Corrections             int     `json:"corrections"`
}

// ValidateEffectivenessAttempt validates identities, outcome relationships, and measured counters.
func ValidateEffectivenessAttempt(attempt EffectivenessAttempt) error {
	if attempt.Schema != effectivenessSchemaVersion {
		return fmt.Errorf("schema must be %d", effectivenessSchemaVersion)
	}
	for name, value := range map[string]string{
		"case_id": attempt.CaseID, "language": attempt.Language, "treatment": attempt.Treatment,
		"snapshot_id": attempt.SnapshotID, "candidate_commit": attempt.CandidateCommit,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}
	if attempt.Repetition < 1 {
		return fmt.Errorf("repetition must be at least 1")
	}
	if attempt.Correct && !attempt.Completed {
		return fmt.Errorf("correct attempt must be completed")
	}
	if (!attempt.Completed || !attempt.Correct) && strings.TrimSpace(attempt.FailureReason) == "" {
		return fmt.Errorf("failure_reason is required for unfinished or incorrect attempts")
	}
	if attempt.RubricScore < 0 || attempt.RubricScore > 1 || math.IsNaN(attempt.RubricScore) {
		return fmt.Errorf("rubric_score must be between 0 and 1")
	}
	for name, value := range map[string]int64{
		"duration_milliseconds":     attempt.DurationMilliseconds,
		"index_setup_milliseconds":  attempt.IndexSetupMilliseconds,
		"index_update_milliseconds": attempt.IndexUpdateMilliseconds,
		"input_tokens":              attempt.InputTokens, "cached_input_tokens": attempt.CachedInputTokens,
		"output_tokens": attempt.OutputTokens, "reasoning_output_tokens": attempt.ReasoningOutputTokens,
	} {
		if value < 0 {
			return fmt.Errorf("%s must not be negative", name)
		}
	}
	for name, value := range map[string]int{
		"tool_calls": attempt.ToolCalls, "source_reads": attempt.SourceReads,
		"retries": attempt.Retries, "corrections": attempt.Corrections,
	} {
		if value < 0 {
			return fmt.Errorf("%s must not be negative", name)
		}
	}
	if !attempt.UsageRecorded {
		if attempt.InputTokens != 0 || attempt.CachedInputTokens != 0 || attempt.OutputTokens != 0 || attempt.ReasoningOutputTokens != 0 {
			return fmt.Errorf("usage_recorded is false but token counters are present")
		}
		return nil
	}
	_, err := attemptTokenUsage(attempt)
	return err
}

func attemptTokenUsage(attempt EffectivenessAttempt) (agentmetrics.TokenUsage, error) {
	body, err := json.Marshal(map[string]int64{
		"input_tokens":            attempt.InputTokens,
		"cached_input_tokens":     attempt.CachedInputTokens,
		"output_tokens":           attempt.OutputTokens,
		"reasoning_output_tokens": attempt.ReasoningOutputTokens,
	})
	if err != nil {
		return agentmetrics.TokenUsage{}, err
	}
	return agentmetrics.ParseTokenUsage(body)
}

// EffectivenessSummary aggregates all attempts while preserving paired task outcomes.
type EffectivenessSummary struct {
	Schema                 int                `json:"schema"`
	Treatments             []TreatmentSummary `json:"treatments"`
	Pairs                  []TaskPairSummary  `json:"pairs"`
	Gate                   EffectivenessGate  `json:"gate"`
	ObservedBreakEvenTasks *float64           `json:"observed_break_even_tasks,omitempty"`
}

// TreatmentSummary reports all-attempt quality, cost, and activity totals for one treatment.
type TreatmentSummary struct {
	Treatment                  string  `json:"treatment"`
	Attempts                   int     `json:"attempts"`
	Completed                  int     `json:"completed"`
	CorrectCompleted           int     `json:"correct_completed"`
	CriticalIncorrect          int     `json:"critical_incorrect"`
	Failed                     int     `json:"failed"`
	UsageRecorded              int     `json:"usage_recorded"`
	UsageMissing               int     `json:"usage_missing"`
	CompletionRate             float64 `json:"completion_rate"`
	CorrectCompletionRate      float64 `json:"correct_completion_rate"`
	FailureRate                float64 `json:"failure_rate"`
	MeanRubricScoreAllAttempts float64 `json:"mean_rubric_score_all_attempts"`
	DurationMilliseconds       int64   `json:"duration_milliseconds"`
	ColdStartMilliseconds      int64   `json:"cold_start_milliseconds"`
	PreindexedTaskMilliseconds int64   `json:"preindexed_task_milliseconds"`
	IndexSetupMilliseconds     int64   `json:"index_setup_milliseconds"`
	IndexUpdateMilliseconds    int64   `json:"index_update_milliseconds"`
	InputTokens                int64   `json:"input_tokens"`
	CachedInputTokens          int64   `json:"cached_input_tokens"`
	OutputTokens               int64   `json:"output_tokens"`
	ReasoningOutputTokens      int64   `json:"reasoning_output_tokens"`
	TotalTokens                int64   `json:"total_tokens"`
	EffectiveTokens            int64   `json:"effective_tokens"`
	ToolCalls                  int     `json:"tool_calls"`
	SourceReads                int     `json:"source_reads"`
	Retries                    int     `json:"retries"`
	Corrections                int     `json:"corrections"`
}

// TaskPairSummary reports treatment outcomes for one case, language, and repetition.
type TaskPairSummary struct {
	CaseID      string          `json:"case_id"`
	Language    string          `json:"language"`
	Repetition  int             `json:"repetition"`
	SnapshotID  string          `json:"snapshot_id"`
	Outcomes    []PairedOutcome `json:"outcomes"`
	Regression  bool            `json:"regression"`
	Uncertainty []string        `json:"uncertainty"`
}

// PairedOutcome is the quality and usage state of one treatment in a task pair.
type PairedOutcome struct {
	Treatment            string `json:"treatment"`
	Completed            bool   `json:"completed"`
	Correct              bool   `json:"correct"`
	UsageRecorded        bool   `json:"usage_recorded"`
	DurationMilliseconds int64  `json:"duration_milliseconds"`
	EffectiveTokens      *int64 `json:"effective_tokens,omitempty"`
}

// EffectivenessGate reports the fixed quality and successful-pair efficiency gates.
type EffectivenessGate struct {
	Status                   string   `json:"status"`
	Passed                   bool     `json:"passed"`
	QualityPassed            bool     `json:"quality_passed"`
	EfficiencyPassed         bool     `json:"efficiency_passed"`
	TokenReductionPercent    *float64 `json:"token_reduction_percent,omitempty"`
	DurationReductionPercent *float64 `json:"duration_reduction_percent,omitempty"`
	SuccessfulTokenPairs     int      `json:"successful_token_pairs"`
	SuccessfulDurationPairs  int      `json:"successful_duration_pairs"`
	Failures                 []string `json:"failures"`
}

// SummarizeEffectiveness validates and aggregates external attempts deterministically.
func SummarizeEffectiveness(attempts []EffectivenessAttempt) (EffectivenessSummary, error) {
	if len(attempts) == 0 {
		return EffectivenessSummary{}, fmt.Errorf("attempts must not be empty")
	}
	byTreatment := map[string]*TreatmentSummary{}
	pairs := map[string][]EffectivenessAttempt{}
	for index, attempt := range attempts {
		if err := ValidateEffectivenessAttempt(attempt); err != nil {
			return EffectivenessSummary{}, fmt.Errorf("attempt %d: %w", index+1, err)
		}
		key := fmt.Sprintf("%s\x00%s\x00%09d", attempt.CaseID, attempt.Language, attempt.Repetition)
		pairs[key] = append(pairs[key], attempt)
		summary := byTreatment[attempt.Treatment]
		if summary == nil {
			summary = &TreatmentSummary{Treatment: attempt.Treatment}
			byTreatment[attempt.Treatment] = summary
		}
		if err := addAttempt(summary, attempt); err != nil {
			return EffectivenessSummary{}, fmt.Errorf("attempt %d: %w", index+1, err)
		}
	}

	report := EffectivenessSummary{Schema: effectivenessSchemaVersion}
	for _, summary := range byTreatment {
		summary.CompletionRate = float64(summary.Completed) / float64(summary.Attempts)
		summary.CorrectCompletionRate = float64(summary.CorrectCompleted) / float64(summary.Attempts)
		summary.FailureRate = float64(summary.Failed) / float64(summary.Attempts)
		summary.MeanRubricScoreAllAttempts /= float64(summary.Attempts)
		report.Treatments = append(report.Treatments, *summary)
	}
	sort.Slice(report.Treatments, func(i, j int) bool { return report.Treatments[i].Treatment < report.Treatments[j].Treatment })

	keys := make([]string, 0, len(pairs))
	for key := range pairs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pair, err := summarizePair(pairs[key])
		if err != nil {
			return EffectivenessSummary{}, err
		}
		report.Pairs = append(report.Pairs, pair)
	}
	report.Gate = calculateEffectivenessGate(pairs, byTreatment)
	report.ObservedBreakEvenTasks = calculateBreakEven(byTreatment)
	return report, nil
}

func addAttempt(summary *TreatmentSummary, attempt EffectivenessAttempt) error {
	summary.Attempts++
	if attempt.Completed {
		summary.Completed++
	}
	if attempt.Completed && attempt.Correct {
		summary.CorrectCompleted++
	}
	if attempt.Completed && !attempt.Correct {
		summary.CriticalIncorrect++
	}
	if !attempt.Completed || !attempt.Correct {
		summary.Failed++
	}
	summary.MeanRubricScoreAllAttempts += attempt.RubricScore
	var err error
	if summary.DurationMilliseconds, err = checkedAdd(summary.DurationMilliseconds, attempt.DurationMilliseconds); err != nil {
		return fmt.Errorf("duration: %w", err)
	}
	if attempt.ColdStart {
		cold, addErr := checkedSum(attempt.DurationMilliseconds, attempt.IndexSetupMilliseconds, attempt.IndexUpdateMilliseconds)
		if addErr != nil {
			return fmt.Errorf("cold-start duration: %w", addErr)
		}
		if summary.ColdStartMilliseconds, err = checkedAdd(summary.ColdStartMilliseconds, cold); err != nil {
			return err
		}
	} else if summary.PreindexedTaskMilliseconds, err = checkedAdd(summary.PreindexedTaskMilliseconds, attempt.DurationMilliseconds); err != nil {
		return err
	}
	if summary.IndexSetupMilliseconds, err = checkedAdd(summary.IndexSetupMilliseconds, attempt.IndexSetupMilliseconds); err != nil {
		return err
	}
	if summary.IndexUpdateMilliseconds, err = checkedAdd(summary.IndexUpdateMilliseconds, attempt.IndexUpdateMilliseconds); err != nil {
		return err
	}
	if summary.ToolCalls, err = checkedAddInt(summary.ToolCalls, attempt.ToolCalls); err != nil {
		return fmt.Errorf("tool_calls: %w", err)
	}
	if summary.SourceReads, err = checkedAddInt(summary.SourceReads, attempt.SourceReads); err != nil {
		return fmt.Errorf("source_reads: %w", err)
	}
	if summary.Retries, err = checkedAddInt(summary.Retries, attempt.Retries); err != nil {
		return fmt.Errorf("retries: %w", err)
	}
	if summary.Corrections, err = checkedAddInt(summary.Corrections, attempt.Corrections); err != nil {
		return fmt.Errorf("corrections: %w", err)
	}
	if !attempt.UsageRecorded {
		summary.UsageMissing++
		return nil
	}
	usage, err := attemptTokenUsage(attempt)
	if err != nil {
		return err
	}
	summary.UsageRecorded++
	for target, value := range map[*int64]int64{
		&summary.InputTokens: usage.InputTokens, &summary.CachedInputTokens: usage.CachedInputTokens,
		&summary.OutputTokens: usage.OutputTokens, &summary.ReasoningOutputTokens: usage.ReasoningOutputTokens,
		&summary.TotalTokens: usage.TotalTokens, &summary.EffectiveTokens: usage.EffectiveTokens,
	} {
		*target, err = checkedAdd(*target, value)
		if err != nil {
			return fmt.Errorf("token aggregate overflows int64")
		}
	}
	return nil
}

func summarizePair(attempts []EffectivenessAttempt) (TaskPairSummary, error) {
	first := attempts[0]
	result := TaskPairSummary{CaseID: first.CaseID, Language: first.Language, Repetition: first.Repetition, SnapshotID: first.SnapshotID}
	seen := map[string]bool{}
	var baseline, candidate *EffectivenessAttempt
	for index := range attempts {
		attempt := attempts[index]
		if attempt.SnapshotID != first.SnapshotID {
			return TaskPairSummary{}, fmt.Errorf("paired task %s/%s repetition %d uses different snapshots", first.CaseID, first.Language, first.Repetition)
		}
		if seen[attempt.Treatment] {
			return TaskPairSummary{}, fmt.Errorf("paired task %s/%s repetition %d duplicates treatment %q", first.CaseID, first.Language, first.Repetition, attempt.Treatment)
		}
		seen[attempt.Treatment] = true
		outcome := PairedOutcome{
			Treatment: attempt.Treatment, Completed: attempt.Completed, Correct: attempt.Correct,
			UsageRecorded: attempt.UsageRecorded, DurationMilliseconds: attempt.DurationMilliseconds,
		}
		if attempt.UsageRecorded {
			usage, err := attemptTokenUsage(attempt)
			if err != nil {
				return TaskPairSummary{}, err
			}
			effective := usage.EffectiveTokens
			outcome.EffectiveTokens = &effective
		}
		result.Outcomes = append(result.Outcomes, outcome)
		if attempt.Treatment == "ordinary" {
			copy := attempt
			baseline = &copy
		}
		if attempt.Treatment == "adaptive-v2" {
			copy := attempt
			candidate = &copy
		}
	}
	sort.Slice(result.Outcomes, func(i, j int) bool { return result.Outcomes[i].Treatment < result.Outcomes[j].Treatment })
	if baseline == nil {
		result.Uncertainty = append(result.Uncertainty, "ordinary treatment missing")
	}
	if candidate == nil {
		result.Uncertainty = append(result.Uncertainty, "adaptive-v2 treatment missing")
	}
	if baseline != nil && candidate != nil {
		result.Regression = baseline.Completed && baseline.Correct && (!candidate.Completed || !candidate.Correct)
		if !baseline.UsageRecorded || !candidate.UsageRecorded {
			result.Uncertainty = append(result.Uncertainty, "paired token usage incomplete")
		}
	}
	return result, nil
}

func calculateEffectivenessGate(pairs map[string][]EffectivenessAttempt, summaries map[string]*TreatmentSummary) EffectivenessGate {
	gate := EffectivenessGate{Status: "not_evaluated", Failures: []string{}}
	baseline, baselineOK := summaries["ordinary"]
	candidate, candidateOK := summaries["adaptive-v2"]
	if !baselineOK || !candidateOK {
		gate.Failures = append(gate.Failures, "ordinary and adaptive-v2 treatments are required")
		return gate
	}
	for _, attempts := range pairs {
		ordinary, adaptive := false, false
		for _, attempt := range attempts {
			ordinary = ordinary || attempt.Treatment == "ordinary"
			adaptive = adaptive || attempt.Treatment == "adaptive-v2"
		}
		if !ordinary || !adaptive {
			gate.Failures = append(gate.Failures, "every evaluated pair requires ordinary and adaptive-v2 treatments")
			return gate
		}
	}
	gate.Status = "evaluated"
	gate.QualityPassed = candidate.CompletionRate >= baseline.CompletionRate && candidate.CriticalIncorrect <= baseline.CriticalIncorrect && candidate.MeanRubricScoreAllAttempts >= baseline.MeanRubricScoreAllAttempts
	if !gate.QualityPassed {
		gate.Failures = append(gate.Failures, "quality must not decline and critical incorrect edits must not increase")
	}
	var baselineTokens, candidateTokens, baselineDuration, candidateDuration []int64
	for _, attempts := range pairs {
		var ordinary, adaptive *EffectivenessAttempt
		for index := range attempts {
			if attempts[index].Treatment == "ordinary" {
				ordinary = &attempts[index]
			}
			if attempts[index].Treatment == "adaptive-v2" {
				adaptive = &attempts[index]
			}
		}
		if ordinary == nil || adaptive == nil || !ordinary.Completed || !ordinary.Correct || !adaptive.Completed || !adaptive.Correct {
			continue
		}
		baselineDuration = append(baselineDuration, ordinary.DurationMilliseconds)
		candidateDuration = append(candidateDuration, adaptive.DurationMilliseconds)
		if ordinary.UsageRecorded && adaptive.UsageRecorded {
			ordinaryUsage, _ := attemptTokenUsage(*ordinary)
			adaptiveUsage, _ := attemptTokenUsage(*adaptive)
			baselineTokens = append(baselineTokens, ordinaryUsage.EffectiveTokens)
			candidateTokens = append(candidateTokens, adaptiveUsage.EffectiveTokens)
		}
	}
	gate.SuccessfulTokenPairs = len(baselineTokens)
	gate.SuccessfulDurationPairs = len(baselineDuration)
	if len(baselineTokens) > 0 {
		value := reductionPercent(median(baselineTokens), median(candidateTokens))
		gate.TokenReductionPercent = &value
	}
	if len(baselineDuration) > 0 {
		value := reductionPercent(median(baselineDuration), median(candidateDuration))
		gate.DurationReductionPercent = &value
	}
	gate.EfficiencyPassed = gate.TokenReductionPercent != nil && gate.DurationReductionPercent != nil && *gate.TokenReductionPercent >= 25 && *gate.DurationReductionPercent >= 20
	if !gate.EfficiencyPassed {
		gate.Failures = append(gate.Failures, "successful paired medians require at least 25% effective-token and 20% duration reduction")
	}
	gate.Passed = gate.QualityPassed && gate.EfficiencyPassed
	return gate
}

func calculateBreakEven(summaries map[string]*TreatmentSummary) *float64 {
	baseline, baselineOK := summaries["ordinary"]
	candidate, candidateOK := summaries["adaptive-v2"]
	if !baselineOK || !candidateOK || baseline.Attempts == 0 || candidate.Attempts == 0 {
		return nil
	}
	baselineMean := float64(baseline.DurationMilliseconds) / float64(baseline.Attempts)
	candidateMean := float64(candidate.DurationMilliseconds) / float64(candidate.Attempts)
	saving := baselineMean - candidateMean
	if saving <= 0 {
		return nil
	}
	value := (float64(candidate.IndexSetupMilliseconds) + float64(candidate.IndexUpdateMilliseconds)) / saving
	return &value
}

func reductionPercent(baseline, candidate float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (baseline - candidate) * 100 / baseline
}

func median(values []int64) float64 {
	copyValues := append([]int64(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	middle := len(copyValues) / 2
	if len(copyValues)%2 == 1 {
		return float64(copyValues[middle])
	}
	return float64(copyValues[middle-1])/2 + float64(copyValues[middle])/2
}

func checkedAdd(left, right int64) (int64, error) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, fmt.Errorf("overflows int64")
	}
	return left + right, nil
}

func checkedAddInt(left, right int) (int, error) {
	maximum := int(^uint(0) >> 1)
	if right > 0 && left > maximum-right {
		return 0, fmt.Errorf("overflows int")
	}
	return left + right, nil
}

func checkedSum(values ...int64) (int64, error) {
	var result int64
	for _, value := range values {
		var err error
		result, err = checkedAdd(result, value)
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}

// LoadEffectivenessAttemptsJSONL strictly loads one attempt object per non-empty line.
func LoadEffectivenessAttemptsJSONL(path string) ([]EffectivenessAttempt, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(body, []byte("\n"))
	attempts := make([]EffectivenessAttempt, 0, len(lines))
	for index, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		var attempt EffectivenessAttempt
		if err := decoder.Decode(&attempt); err != nil {
			return nil, fmt.Errorf("line %d: %w", index+1, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err == nil {
				err = fmt.Errorf("trailing JSON value")
			}
			return nil, fmt.Errorf("line %d: %w", index+1, err)
		}
		attempts = append(attempts, attempt)
	}
	if len(attempts) == 0 {
		return nil, fmt.Errorf("attempts file is empty")
	}
	return attempts, nil
}
