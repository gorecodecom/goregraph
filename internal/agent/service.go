package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	if !isSymbolTask(request.Task) {
		warnings = compactWarnings(warnings, 12)
	}
	freshness := "generated output loaded"
	if request.Task == "task-context" && len(page) > 0 {
		if value, ok := page[0].Data["freshness"].(string); ok && value != "" {
			freshness = value
		}
	}
	result := Result{Schema: scan.SchemaVersion, Task: request.Task, Freshness: freshness, CoverageWarnings: warnings, Items: page, Count: len(page), SuggestedNext: suggestedNext(request.Task)}
	if end < len(items) {
		result.Truncated = true
		result.Continuation = encodeContinuation(request.Task, end)
	}
	return result, nil
}

func isSymbolTask(task string) bool {
	return strings.HasPrefix(task, "symbol-")
}

func loadTask(request Request) ([]Item, []string, error) {
	switch request.Task {
	case "symbol-inventory":
		return loadSymbolInventory(request, false)
	case "symbol-resolve":
		return loadSymbolInventory(request, true)
	case "symbol-usages":
		return loadSymbolUsages(request, scan.SymbolUsageDirectReference)
	case "symbol-api-consumers":
		return loadSymbolUsages(request, scan.SymbolUsageReachedThroughAPI)
	case "symbol-explain":
		return loadSymbolExplain(request)
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
			items = append(items, Item{ID: "capability:" + record.Project + ":" + record.Language + ":" + string(record.ID), Kind: "capability", Title: record.Language + " / " + string(record.ID), Summary: string(record.Coverage) + " — " + record.Reason, Project: record.Project, EvidenceIDs: capabilityEvidence(record, request.Detail)})
			if record.Coverage == scan.CoverageUnavailable || record.Coverage == scan.CoverageFailed {
				warnings = appendUnique(warnings, record.Language+" / "+string(record.ID)+": "+string(record.Coverage))
			}
		}
		return items, warnings, nil
	case "diagnostics":
		var families []scan.DiagnosticFamilyRecord
		if err := readOutput(request.Root, "diagnostic-families.json", &families); err == nil {
			items := []Item{}
			for _, family := range families {
				if !matchesQuery(request.Query, family.FamilyID, family.Code, family.Service, family.RoutePattern, family.RootCause) {
					continue
				}
				items = append(items, Item{ID: family.FamilyID, Kind: "diagnostic_family", Title: family.Code + " " + family.RoutePattern, Summary: family.RootCause, EvidenceIDs: family.EvidenceIDs, Data: map[string]any{"affected_count": family.AffectedCount, "observed_count": family.ObservedCount, "resolved_count": family.ResolvedCount, "unresolved_count": family.UnresolvedCount, "out_of_scope_count": family.OutOfScopeCount, "likely_owner": family.LikelyOwner, "affected_projects": family.AffectedProjects, "diagnostic_ids": family.DiagnosticIDs, "next_checks": family.NextChecks, "suggested_check": family.SuggestedCheck}})
			}
			return items, nil, nil
		}
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
	case "impact-summary":
		var flows []scan.WorkspaceFeatureFlowRecord
		if err := readOutput(request.Root, "workspace-feature-flows.json", &flows); err != nil {
			if workspaceErr := readOutput(request.Root, "feature-flows.json", &flows); workspaceErr != nil {
				return nil, nil, err
			}
		}
		var serviceMap scan.WorkspaceServiceMapRecord
		_ = readOutput(request.Root, "workspace-service-map.json", &serviceMap)
		depth := 2
		if request.Detail == "summary" {
			depth = 1
		} else if request.Detail == "full" {
			depth = 3
		}
		summaries := scan.BuildImpactSummaries(flows, serviceMap, serviceMap.WorkspaceCoverage, depth)
		items := []Item{}
		warnings := []string{}
		for _, summary := range summaries {
			if !matchesQuery(request.Query, summary.ID, summary.TargetID, summary.TargetLabel, strings.Join(summary.AffectedPackages, " ")) {
				continue
			}
			items = append(items, Item{ID: summary.ID, Kind: "impact_summary", Title: summary.TargetLabel, Summary: strings.ToUpper(summary.RiskLevel) + " — " + strings.Join(summary.RiskReasons, " "), Data: map[string]any{"impact": summary}})
			warnings = append(warnings, summary.CoverageUncertainty...)
		}
		return items, warnings, nil
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
				data := map[string]any(nil)
				if request.Task == "service-context" && request.Query != "" {
					data = map[string]any{"requested_scope": request.Query}
				}
				items = append(items, Item{ID: firstText(r.RouteID, "route:"+r.File+":"+strconv.Itoa(r.Line)), Kind: "route", Title: r.HTTPMethod + " " + r.Path, Summary: r.Framework + " / " + r.Handler, File: r.File, Line: r.Line, Confidence: r.Confidence, EvidenceIDs: r.EvidenceIDs, Data: data})
			}
		}
		return items, nil, nil
	case "workspace-summary":
		var serviceMap scan.WorkspaceServiceMapRecord
		if err := readOutput(request.Root, "workspace-service-map.json", &serviceMap); err == nil {
			return []Item{{
				ID:      "workspace:" + serviceMap.Root,
				Kind:    "workspace",
				Title:   firstText(serviceMap.Root, request.Root),
				Summary: fmt.Sprintf("%d services; %d contracts; %d resolved", len(serviceMap.Nodes), serviceMap.ContractSummary.Total, serviceMap.ContractSummary.Resolved),
				Data:    map[string]any{"generated": serviceMap.Generated, "contract_summary": serviceMap.ContractSummary, "workspace_coverage": serviceMap.WorkspaceCoverage},
			}}, nil, nil
		}
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
	case "task-context":
		return loadTaskContext(request)
	case "workspace-delta":
		if strings.TrimSpace(request.Query) == "" {
			return nil, nil, fmt.Errorf("workspace-delta query must be the before snapshot directory")
		}
		diff, err := scan.WorkspaceDiff(request.Query, request.Root)
		if err != nil {
			return nil, nil, err
		}
		return []Item{{ID: "workspace-delta", Kind: "workspace_delta", Title: "Workspace delta", Summary: fmt.Sprintf("%d added routes, %d changed contracts, %d coverage regressions", len(diff.AddedRoutes), len(diff.ChangedContracts), len(diff.CoverageRegressions)), Data: map[string]any{"delta": diff}}}, nil, nil
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

func loadSymbolInventory(request Request, resolve bool) ([]Item, []string, error) {
	var index scan.WorkspaceSymbolIndexRecord
	if err := readWorkspaceOutput(request.Root, "symbol-index.json", &index); err != nil {
		return nil, nil, err
	}
	query := strings.TrimSpace(request.Query)
	if resolve && query == "" {
		return nil, nil, fmt.Errorf("symbol-resolve query is required")
	}
	items := []Item{}
	projects := []string{}
	for _, symbol := range index.Symbols {
		matches := matchesQuery(
			query,
			symbol.ID,
			symbol.Project,
			symbol.Service,
			symbol.Module,
			symbol.Package,
			symbol.WorkspacePackage,
			symbol.Artifact,
			symbol.Name,
			symbol.QualifiedName,
			symbol.ExportName,
			symbol.DeclarationFile,
		)
		if resolve {
			matches = symbolResolveMatches(query, symbol)
		}
		if !matches {
			continue
		}
		items = append(items, symbolItem(symbol))
		projects = append(projects, symbol.Project)
	}
	sortItems(items)
	return items, symbolCoverageWarnings(index.Coverage, projects), nil
}

func loadSymbolUsages(request Request, category scan.SymbolUsageCategory) ([]Item, []string, error) {
	symbolID := strings.TrimSpace(request.Query)
	if !strings.HasPrefix(symbolID, "symbol:") {
		return nil, nil, fmt.Errorf("%s query must be a stable symbol ID", request.Task)
	}
	var index scan.WorkspaceSymbolUsageIndexRecord
	if err := readWorkspaceOutput(request.Root, "symbol-usages.json", &index); err != nil {
		return nil, nil, err
	}
	items := []Item{}
	for _, usage := range index.Usages {
		if usage.ProviderSymbolID != symbolID || usage.Category != category {
			continue
		}
		items = append(items, symbolUsageItem(usage, "symbol_usage"))
	}
	sortItems(items)
	return items, symbolCoverageWarnings(index.Coverage, nil, categoryCapability(category)), nil
}

func loadSymbolExplain(request Request) ([]Item, []string, error) {
	target := strings.TrimSpace(request.Query)
	switch {
	case strings.HasPrefix(target, "symbol:"):
		var symbols scan.WorkspaceSymbolIndexRecord
		if err := readWorkspaceOutput(request.Root, "symbol-index.json", &symbols); err != nil {
			return nil, nil, err
		}
		for _, symbol := range symbols.Symbols {
			if symbol.ID == target {
				item := symbolItem(symbol)
				item.Kind = "symbol_explanation"
				return []Item{item}, symbolCoverageWarnings(symbols.Coverage, []string{symbol.Project}), nil
			}
		}
		return nil, nil, fmt.Errorf("symbol %q not found in workspace projection", target)
	case strings.HasPrefix(target, "usage:"):
		var usages scan.WorkspaceSymbolUsageIndexRecord
		if err := readWorkspaceOutput(request.Root, "symbol-usages.json", &usages); err != nil {
			return nil, nil, err
		}
		for _, usage := range usages.Usages {
			if usage.ID == target {
				return []Item{symbolUsageItem(usage, "symbol_explanation")},
					symbolCoverageWarnings(usages.Coverage, []string{usage.ConsumerProject}), nil
			}
		}
		return nil, nil, fmt.Errorf("usage %q not found in workspace projection", target)
	default:
		return nil, nil, fmt.Errorf("symbol-explain query must be a stable symbol or usage ID")
	}
}

func symbolItem(symbol scan.CanonicalSymbolRecord) Item {
	title := firstText(symbol.QualifiedName, symbol.ExportName, symbol.Name)
	summary := symbol.Language + " " + symbol.Kind + " declared in " +
		symbol.DeclarationFile + ":" + strconv.Itoa(symbol.DeclarationLine)
	return Item{
		ID: symbol.ID, Kind: "canonical_symbol", Title: title, Summary: summary,
		Project: symbol.Project, File: symbol.DeclarationFile, Line: symbol.DeclarationLine,
		Confidence: string(symbol.Confidence), EvidenceIDs: symbol.EvidenceIDs,
		Data: map[string]any{"symbol": symbol},
	}
}

func symbolUsageItem(usage scan.CanonicalSymbolUsageRecord, kind string) Item {
	return Item{
		ID: usage.ID, Kind: kind, Title: usage.ConsumerProject + " / " + usage.RelationKind,
		Summary: string(usage.Category) + " — " + usage.Reason,
		Project: usage.ConsumerProject, File: usage.SourceFile, Line: usage.SourceLine,
		Confidence: string(usage.Confidence), Resolution: string(usage.Resolution),
		EvidenceIDs: usage.EvidenceIDs, Data: map[string]any{"usage": usage},
	}
}

func symbolResolveMatches(query string, symbol scan.CanonicalSymbolRecord) bool {
	for _, candidate := range []string{
		symbol.ID,
		symbol.Name,
		symbol.QualifiedName,
		symbol.ExportName,
	} {
		if candidate != "" && strings.EqualFold(query, candidate) {
			return true
		}
	}
	return false
}

func categoryCapability(category scan.SymbolUsageCategory) string {
	if category == scan.SymbolUsageReachedThroughAPI {
		return "http_reachability"
	}
	return "direct_usages"
}

func symbolCoverageWarnings(coverage []scan.SymbolCoverageRecord, projects []string, capabilities ...string) []string {
	projectSet := make(map[string]bool, len(projects))
	for _, project := range projects {
		projectSet[project] = true
	}
	capabilitySet := make(map[string]bool, len(capabilities))
	for _, capability := range capabilities {
		capabilitySet[capability] = true
	}
	warnings := []string{}
	for _, record := range coverage {
		if record.Coverage == scan.CoverageComplete ||
			len(projectSet) > 0 && !projectSet[record.Project] ||
			len(capabilitySet) > 0 && !capabilitySet[record.Capability] {
			continue
		}
		warning := record.Project + " / " + record.Language + " / " + record.Capability +
			": " + string(record.Coverage) + " — " + record.Reason
		if len(record.Limitations) > 0 {
			warning += " (" + strings.Join(record.Limitations, ", ") + ")"
		}
		warnings = appendUnique(warnings, warning)
	}
	sort.Strings(warnings)
	return warnings
}

func loadTaskContext(request Request) ([]Item, []string, error) {
	var routes []scan.CodeRouteRecord
	if err := readOutput(request.Root, "routes.json", &routes); err != nil {
		return nil, nil, err
	}
	var mappings []scan.TestMapRecord
	if err := readOutput(request.Root, "test-map.json", &mappings); err != nil {
		return nil, nil, err
	}
	var diagnostics []scan.CanonicalDiagnosticRecord
	if err := readOutput(request.Root, "diagnostics-canonical.json", &diagnostics); err != nil {
		return nil, nil, err
	}
	var capabilities []scan.CapabilityRecord
	if err := readOutput(request.Root, "capabilities.json", &capabilities); err != nil {
		return nil, nil, err
	}
	var featureFlows []scan.WorkspaceFeatureFlowRecord
	_ = readOutput(request.Root, "workspace-feature-flows.json", &featureFlows)
	testLinks := []scan.TestLinkRecord{}
	verificationCommands := []scan.VerificationCommandRecord{}

	context := TaskContextRecord{Target: request.Query, Freshness: "generated output loaded", SuggestedNext: "goregraph query <path> evidence --limit 20"}
	var freshness scan.ArtifactFreshnessIndex
	if err := readOutput(request.Root, "freshness.json", &freshness); err != nil {
		context.Freshness = "unknown: freshness metadata is missing; rescan before relying on absence"
		context.CoverageWarnings = append(context.CoverageWarnings, context.Freshness)
	} else {
		context.Freshness = fmt.Sprintf("goregraph %s / schema %d / source fingerprint %s", freshness.GoreGraphVersion, freshness.Schema, freshness.SourceFingerprint)
	}
	for _, route := range routes {
		if !matchesQuery(request.Query, route.RouteID, route.HTTPMethod, route.Path, route.Handler, route.File) {
			continue
		}
		context.Endpoints = append(context.Endpoints, Item{
			ID: firstText(route.RouteID, "route:"+route.File+":"+strconv.Itoa(route.Line)), Kind: "route",
			Title: route.HTTPMethod + " " + route.Path, Summary: route.Framework + " / " + route.Handler,
			File: route.File, Line: route.Line, Confidence: route.Confidence, EvidenceIDs: route.EvidenceIDs,
		})
		context.Files = append(context.Files, route.File)
		context.EvidenceIDs = append(context.EvidenceIDs, route.EvidenceIDs...)
		if route.App != "" {
			context.Services = append(context.Services, route.App)
		} else if route.Package != "" {
			context.Services = append(context.Services, route.Package)
		}
	}
	for _, mapping := range mappings {
		if !matchesQuery(request.Query, mapping.TestFile, mapping.TestMethod, mapping.TargetFile, mapping.TargetMethod, mapping.HTTPMethod, mapping.Path) {
			continue
		}
		context.Tests = append(context.Tests, Item{
			ID: "test:" + mapping.TestFile + ":" + strconv.Itoa(mapping.Line), Kind: "test",
			Title: firstText(mapping.TestMethod, mapping.TestFile), Summary: firstText(mapping.TargetMethod, mapping.Path, mapping.Type),
			File: mapping.TestFile, Line: mapping.Line, Confidence: mapping.Confidence,
		})
		context.Files = append(context.Files, mapping.TestFile, mapping.TargetFile)
	}
	for _, diagnostic := range diagnostics {
		if !matchesQuery(request.Query, diagnostic.ID, diagnostic.Code, diagnostic.Title, diagnostic.Explanation, strings.Join(diagnostic.AffectedArtifacts, " ")) {
			continue
		}
		context.Risks = append(context.Risks, Item{
			ID: diagnostic.ID, Kind: "diagnostic", Title: diagnostic.Title, Summary: diagnostic.Explanation,
			Confidence: string(diagnostic.Confidence), Resolution: string(diagnostic.Resolution),
			EvidenceIDs: diagnostic.EvidenceIDs, Data: map[string]any{"code": diagnostic.Code, "severity": diagnostic.Severity, "next_checks": diagnostic.NextChecks},
		})
		context.EvidenceIDs = append(context.EvidenceIDs, diagnostic.EvidenceIDs...)
	}
	for _, capability := range capabilities {
		if capability.Coverage == scan.CoverageUnavailable || capability.Coverage == scan.CoverageFailed {
			context.CoverageWarnings = append(context.CoverageWarnings, capability.Language+" / "+string(capability.ID)+": "+string(capability.Coverage))
		}
	}
	for _, flow := range featureFlows {
		if !matchesQuery(request.Query, flow.ID, flow.HTTPMethod, flow.Path, flow.FrontendProject, flow.BackendProject) {
			continue
		}
		testLinks = append(testLinks, flow.TestLinks...)
		verificationCommands = append(verificationCommands, flow.VerificationCommands...)
	}
	context.Services = sortedUnique(context.Services)
	context.Files = sortedUnique(context.Files)
	context.EvidenceIDs = sortedUnique(context.EvidenceIDs)
	context.CoverageWarnings = compactWarnings(sortedUnique(context.CoverageWarnings), 12)
	sortItems(context.Endpoints)
	sortItems(context.Tests)
	sortItems(context.Risks)
	context.Endpoints = boundedItems(context.Endpoints, request.Limit)
	context.Tests = boundedItems(context.Tests, request.Limit)
	context.Risks = boundedItems(context.Risks, request.Limit)

	item := Item{
		ID: "task-context:" + firstText(request.Query, "workspace"), Kind: "task_context",
		Title:       firstText(request.Query, "Workspace task context"),
		Summary:     fmt.Sprintf("%d endpoints, %d tests, %d risks", len(context.Endpoints), len(context.Tests), len(context.Risks)),
		EvidenceIDs: context.EvidenceIDs,
		Data: map[string]any{
			"target": context.Target, "services": context.Services, "endpoints": context.Endpoints,
			"files": context.Files, "tests": context.Tests, "risks": context.Risks,
			"test_links": testLinks, "verification_commands": verificationCommands,
			"freshness": context.Freshness, "coverage_warnings": context.CoverageWarnings,
			"suggested_next": context.SuggestedNext,
		},
	}
	return []Item{item}, context.CoverageWarnings, nil
}

func boundedItems(items []Item, limit int) []Item {
	if len(items) <= limit {
		return items
	}
	return append([]Item(nil), items[:limit]...)
}

func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

func sortedUnique(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func capabilityEvidence(record scan.CapabilityRecord, detail string) []string {
	if detail == "summary" {
		return nil
	}
	if detail == "full" || len(record.EvidenceIDs) <= 10 {
		return record.EvidenceIDs
	}
	return record.EvidenceIDs[:10]
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
func readWorkspaceOutput(root, name string, dest any) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	workspaceRoot := root
	directPath := filepath.Join(root, ".goregraph-workspace", name)
	body, err := os.ReadFile(directPath)
	if os.IsNotExist(err) {
		resolved, ok, resolveErr := scan.WorkspaceRoot(root, cfg)
		if resolveErr != nil {
			return resolveErr
		}
		if ok {
			workspaceRoot = resolved
			body, err = os.ReadFile(filepath.Join(workspaceRoot, ".goregraph-workspace", name))
		}
	}
	if err != nil {
		return fmt.Errorf("workspace output %s is missing; run `goregraph workspace scan-all <path>` first", name)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("workspace output %s is invalid: %w", name, err)
	}
	return nil
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
