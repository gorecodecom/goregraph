package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/agentbench"
)

func TestValidateCommandAcceptsDevelopmentManifest(t *testing.T) {
	manifest := filepath.Join("..", "..", "testdata", "agent-effectiveness", "development", "manifest.json")
	code, stdout, stderr := invoke("validate", "--manifest", manifest)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"tasks": 12`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestSummarizeCommandWritesReportWithoutLaunchingAnything(t *testing.T) {
	directory := t.TempDir()
	attemptsPath := filepath.Join(directory, "attempts.jsonl")
	outputPath := filepath.Join(directory, "report.json")
	attempt := agentbench.EffectivenessAttempt{
		Schema: 1, CaseID: "case", Language: "en", Treatment: "ordinary",
		SnapshotID: "snapshot", CandidateCommit: "commit", Repetition: 1,
		Completed: true, Correct: true, DurationMilliseconds: 10, UsageRecorded: true,
		InputTokens: 10, CachedInputTokens: 4, OutputTokens: 2, ToolCalls: 1,
	}
	body, err := json.Marshal(attempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attemptsPath, append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := invoke("summarize", "--attempts", attemptsPath, "--output", outputPath)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	result, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"effective_tokens": 8`) {
		t.Fatalf("report = %s", result)
	}
}

func TestCommandsRejectMalformedOrUnsafeInputs(t *testing.T) {
	tests := [][]string{
		{"validate"},
		{"summarize", "--attempts", "missing.jsonl", "--output", filepath.Join(t.TempDir(), "out.json")},
		{"unknown"},
	}
	for _, args := range tests {
		code, stdout, stderr := invoke(args...)
		if code != 2 || stdout != "" || stderr == "" {
			t.Fatalf("run(%q) exit=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
}

func TestSummarizeDoesNotOverwriteExistingOutput(t *testing.T) {
	directory := t.TempDir()
	attempts := filepath.Join(directory, "attempts.jsonl")
	output := filepath.Join(directory, "report.json")
	if err := os.WriteFile(attempts, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, _ := invoke("summarize", "--attempts", attempts, "--output", output)
	if code != 2 {
		t.Fatalf("exit = %d", code)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "sentinel" {
		t.Fatalf("output = %q", got)
	}
}

func invoke(args ...string) (int, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}
