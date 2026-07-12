package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

const defaultLimit = 20
const maxLimit = 100

type Service struct{}

func (Service) Run(request Request) (Result, error) {
	if request.Root == "" {
		request.Root = "."
	}
	if request.Limit == 0 {
		request.Limit = defaultLimit
	}
	if request.Limit < 1 || request.Limit > maxLimit {
		return Result{}, fmt.Errorf("limit must be between 1 and %d", maxLimit)
	}
	if request.Detail == "" {
		request.Detail = "standard"
	}
	if request.Detail != "summary" && request.Detail != "standard" && request.Detail != "full" {
		return Result{}, fmt.Errorf("detail must be summary, standard, or full")
	}
	if request.Format == "" {
		request.Format = "json"
	}
	if request.Format != "json" && request.Format != "text" && request.Format != "markdown" {
		return Result{}, fmt.Errorf("format must be json, text, or markdown")
	}
	offset, err := decodeContinuation(request.Continuation, request.Task)
	if err != nil {
		return Result{}, err
	}
	items, warnings, err := loadTask(request)
	if err != nil {
		return Result{}, err
	}
	if offset > len(items) {
		return Result{}, fmt.Errorf("continuation is outside the current result set")
	}
	end := offset + request.Limit
	if end > len(items) {
		end = len(items)
	}
	page := append([]Item(nil), items[offset:end]...)
	result := Result{Schema: scan.SchemaVersion, Task: request.Task, Freshness: "generated output loaded", CoverageWarnings: warnings, Items: page, Count: len(page), SuggestedNext: suggestedNext(request.Task)}
	if end < len(items) {
		result.Truncated = true
		result.Continuation = encodeContinuation(request.Task, end)
	}
	return result, nil
}

func loadTask(request Request) ([]Item, []string, error) {
	switch request.Task {
	case "coverage":
		var records []scan.CapabilityRecord
		if err := readOutput(request.Root, "capabilities.json", &records); err != nil {
			return nil, nil, err
		}
		items := []Item{}
		warnings := []string{}
		for _, record := range records {
			if !matchesQuery(request.Query, record.Project, record.Language, string(record.ID), string(record.Coverage), record.Reason) {
				continue
			}
			items = append(items, Item{ID: "capability:" + record.Project + ":" + record.Language + ":" + string(record.ID), Kind: "capability", Title: record.Language + " / " + string(record.ID), Summary: string(record.Coverage) + " — " + record.Reason, Project: record.Project})
			if record.Coverage == scan.CoverageUnavailable || record.Coverage == scan.CoverageFailed {
				warnings = appendUnique(warnings, record.Language+" / "+string(record.ID)+": "+string(record.Coverage))
			}
		}
		return items, warnings, nil
	case "diagnostics":
		var records []scan.CanonicalDiagnosticRecord
		if err := readOutput(request.Root, "diagnostics-canonical.json", &records); err != nil {
			return nil, nil, err
		}
		items := []Item{}
		for _, record := range records {
			if !matchesQuery(request.Query, record.Code, record.Title, record.Category) {
				continue
			}
			items = append(items, Item{ID: record.ID, Kind: "diagnostic", Title: record.Title, Summary: record.Explanation, Confidence: string(record.Confidence), Resolution: string(record.Resolution), EvidenceIDs: record.EvidenceIDs, Data: map[string]any{"code": record.Code, "severity": record.Severity, "next_checks": record.NextChecks}})
		}
		return items, nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown agent task %q", request.Task)
	}
}

func readOutput(root, name string, dest any) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, name))
	if err != nil {
		return fmt.Errorf("output %s is missing; run `goregraph scan <path>` first", name)
	}
	return json.Unmarshal(body, dest)
}
func matchesQuery(query string, values ...string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join(values, " ")), q)
}
func encodeContinuation(task string, offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(task + ":" + strconv.Itoa(offset)))
}
func decodeContinuation(token, task string) (int, error) {
	if token == "" {
		return 0, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, fmt.Errorf("invalid continuation")
	}
	parts := strings.Split(string(body), ":")
	if len(parts) != 2 || parts[0] != task {
		return 0, fmt.Errorf("continuation does not match task")
	}
	offset, err := strconv.Atoi(parts[1])
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid continuation")
	}
	return offset, nil
}
func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}
func suggestedNext(task string) string {
	if task == "coverage" {
		return "goregraph query <path> diagnostics --limit 20"
	}
	return "goregraph query <path> evidence --limit 20"
}
