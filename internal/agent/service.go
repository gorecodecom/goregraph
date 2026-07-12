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
	traversal "github.com/gorecodecom/goregraph/internal/trace"
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
	warnings = compactWarnings(warnings, 12)
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
			items = append(items, Item{ID: "capability:" + record.Project + ":" + record.Language + ":" + string(record.ID), Kind: "capability", Title: record.Language + " / " + string(record.ID), Summary: string(record.Coverage) + " — " + record.Reason, Project: record.Project, EvidenceIDs: record.EvidenceIDs})
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
	case "evidence":
		var records []scan.EvidenceRecord
		if err := readOutput(request.Root, "evidence.json", &records); err != nil {
			return nil, nil, err
		}
		items := []Item{}
		for _, r := range records {
			if matchesQuery(request.Query, r.ID, r.Project, r.File, r.Analyzer, r.Adapter, r.Method, r.Reason) {
				items = append(items, Item{ID: r.ID, Kind: "evidence", Title: r.File, Summary: r.Method + " — " + r.Reason, Project: r.Project, File: r.File, Line: r.Start.Line})
			}
		}
		var facts []scan.ArchitectureCapabilityFact
		if err := readOutput(request.Root, "architecture-capabilities.json", &facts); err != nil {
			return nil, nil, err
		}
		for _, fact := range facts {
			if matchesQuery(request.Query, fact.ID, fact.Language, string(fact.Capability), fact.Kind, fact.Framework, fact.File) {
				items = append(items, Item{ID: fact.ID, Kind: "architecture_capability_evidence", Title: fact.File, Summary: string(fact.Capability) + " — " + fact.Framework + " / " + fact.Kind, File: fact.File, Line: fact.Line})
			}
		}
		return items, nil, nil
	case "tests":
		var records []scan.TestMapRecord
		if err := readOutput(request.Root, "test-map.json", &records); err != nil {
			return nil, nil, err
		}
		items := []Item{}
		for _, r := range records {
			id := "test:" + r.TestFile + ":" + strconv.Itoa(r.Line)
			if matchesQuery(request.Query, r.TestFile, r.TestMethod, r.TargetFile, r.TargetMethod, r.Path) {
				items = append(items, Item{ID: id, Kind: "test", Title: firstText(r.TestMethod, r.TestFile), Summary: firstText(r.TargetMethod, r.Path, r.Type), File: r.TestFile, Line: r.Line, Confidence: r.Confidence})
			}
		}
		return items, nil, nil
	case "endpoint-search", "service-context":
		var records []scan.CodeRouteRecord
		if err := readOutput(request.Root, "routes.json", &records); err != nil {
			return nil, nil, err
		}
		items := []Item{}
		for _, r := range records {
			if matchesQuery(request.Query, r.HTTPMethod, r.Path, r.Handler, r.File, r.Framework) {
				items = append(items, Item{ID: firstText(r.RouteID, "route:"+r.File+":"+strconv.Itoa(r.Line)), Kind: "route", Title: r.HTTPMethod + " " + r.Path, Summary: r.Framework + " / " + r.Handler, File: r.File, Line: r.Line, Confidence: r.Confidence, EvidenceIDs: r.EvidenceIDs})
			}
		}
		return items, nil, nil
	case "workspace-summary":
		var manifest scan.Manifest
		if err := readOutput(request.Root, "manifest.json", &manifest); err != nil {
			return nil, nil, err
		}
		return []Item{{ID: "workspace:" + manifest.ProjectRoot, Kind: "workspace", Title: manifest.ProjectRoot, Summary: fmt.Sprintf("%d indexed files; schema %d", manifest.Files, manifest.Schema), Data: map[string]any{"generated": manifest.Generated}}}, nil, nil
	case "change-context":
		body, err := readOutputText(request.Root, "affected.md")
		if err != nil {
			return nil, nil, err
		}
		return []Item{{ID: "change-context", Kind: "change_context", Title: "Generated change context", Summary: body}}, nil, nil
	case "endpoint-trace", "symbol-trace":
		var index scan.DirectedTraceIndexRecord
		if err := readOutput(request.Root, "directed-traces.json", &index); err != nil {
			return nil, nil, err
		}
		items := []Item{}
		for _, record := range index.Traces {
			if matchesQuery(request.Query, record.ID, record.Route) {
				data := map[string]any(nil)
				if request.Detail == "standard" {
					data = map[string]any{"entry_nodes": record.EntryNodes, "exit_nodes": record.ExitNodes, "main_path": record.MainPath, "branches": len(record.Branches), "cycles": len(record.Cycles), "truncated": record.Truncated}
				} else if request.Detail == "full" {
					data = map[string]any{"trace": record}
				}
				items = append(items, Item{ID: record.ID, Kind: "directed_trace", Title: firstText(record.Route, record.ID), Summary: fmt.Sprintf("%d nodes / %d edges", len(record.Nodes), len(record.Edges)), Data: data})
			}
		}
		return items, nil, nil
	case "trace-from":
		parts := strings.SplitN(request.Query, "#", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("trace-from query must be <trace-id>#<node-id>")
		}
		var index scan.DirectedTraceIndexRecord
		if err := readOutput(request.Root, "directed-traces.json", &index); err != nil {
			return nil, nil, err
		}
		for _, record := range index.Traces {
			if record.ID != parts[0] {
				continue
			}
			result, err := traversal.Traverse(record, parts[1], traversal.Options{})
			if err != nil {
				return nil, nil, err
			}
			return []Item{{ID: record.ID + "#" + parts[1], Kind: "trace_traversal", Title: "Trace from " + parts[1], Summary: record.Route, Data: map[string]any{"traversal": result}}}, nil, nil
		}
		return nil, nil, fmt.Errorf("directed trace %q not found", parts[0])
	case "data-flow":
		var records []scan.DataFlowRecord
		if err := readOutput(request.Root, "data-flows.json", &records); err != nil {
			return nil, nil, err
		}
		items := []Item{}
		for _, record := range records {
			if matchesQuery(request.Query, record.ID, record.Route, record.Project) {
				data := map[string]any(nil)
				if request.Detail == "standard" {
					data = map[string]any{"gaps": record.Gaps}
				} else if request.Detail == "full" {
					data = map[string]any{"flow": record}
				}
				items = append(items, Item{ID: record.ID, Kind: "data_flow", Title: record.Route, Summary: fmt.Sprintf("%d nodes / %d gaps", len(record.Nodes), len(record.Gaps)), Project: record.Project, Data: data})
			}
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
	if os.IsNotExist(err) {
		body, err = os.ReadFile(filepath.Join(root, ".goregraph-workspace", name))
	}
	if err != nil {
		return fmt.Errorf("output %s is missing; run `goregraph scan <path>` first", name)
	}
	return json.Unmarshal(body, dest)
}
func readOutputText(root, name string) (string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(filepath.Join(root, cfg.OutputDir, name))
	if err != nil {
		return "", fmt.Errorf("output %s is missing; run `goregraph scan <path>` first", name)
	}
	return string(body), nil
}
func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "unknown"
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
func compactWarnings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	result := append([]string(nil), values[:limit]...)
	return append(result, fmt.Sprintf("%d additional coverage gaps omitted; continue the coverage query for details", len(values)-limit))
}
func suggestedNext(task string) string {
	if task == "coverage" {
		return "goregraph query <path> diagnostics --limit 20"
	}
	return "goregraph query <path> evidence --limit 20"
}
