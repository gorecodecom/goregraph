package scan

import (
	"fmt"
	"sort"
	"strings"
)

const (
	symbolAPIRelationKind = "http_reachability"
	symbolAPIAnalyzer     = "workspace-symbol-api"
)

// BuildWorkspaceSymbolAPIUsages derives HTTP-mediated symbol usages from
// resolved frontend contracts and their Spring implementation flows.
func BuildWorkspaceSymbolAPIUsages(symbols WorkspaceSymbolIndexRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, traces WorkspaceEndpointTraceIndexRecord) []CanonicalSymbolUsageRecord {
	flowByContract := make(map[string]WorkspaceFeatureFlowRecord, len(flows))
	for _, match := range matches {
		if flow, ok := workspaceSymbolAPIFlow(match, flows); ok {
			flowByContract[match.ID] = flow
		}
	}
	traceByContract := make(map[string]WorkspaceEndpointTraceRecord, len(traces.Traces))
	for _, trace := range traces.Traces {
		traceByContract[trace.ID] = trace
	}

	var usages []CanonicalSymbolUsageRecord
	for _, match := range matches {
		if match.Issue != contractIssueMatched || !strings.EqualFold(match.Confidence, string(ConfidenceResolved)) {
			continue
		}
		flow, ok := flowByContract[match.ID]
		if !ok {
			continue
		}
		consumer, originStep, ok := workspaceSymbolAPIConsumer(symbols.Symbols, flow, match)
		if !ok {
			continue
		}
		helper, helperStep := workspaceSymbolAPIHelper(symbols.Symbols, flow, match)
		trace := traceByContract[match.ID]
		for _, implementation := range flow.BackendSteps {
			candidates := workspaceSymbolAPIJavaCandidates(symbols.Symbols, match.BackendProject, implementation)
			usages = append(
				usages,
				buildWorkspaceSymbolAPIUsage(match, flow, trace, consumer, originStep, helper, helperStep, implementation, candidates)...,
			)
		}
	}
	usages = dedupeWorkspaceSymbolUsages(usages)
	sort.Slice(usages, func(i, j int) bool {
		return usages[i].ID < usages[j].ID
	})
	return usages
}

func workspaceSymbolAPIFlow(match WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord) (WorkspaceFeatureFlowRecord, bool) {
	for _, flow := range flows {
		if flow.FrontendProject == match.APIProject &&
			flow.BackendProject == match.BackendProject &&
			flow.FrontendFile == match.APIFile &&
			strings.EqualFold(flow.HTTPMethod, match.APIHTTPMethod) &&
			flow.Path == match.APIPath {
			return flow, true
		}
	}
	for _, flow := range flows {
		if flow.FrontendProject == match.APIProject &&
			flow.BackendProject == match.BackendProject &&
			strings.EqualFold(flow.HTTPMethod, match.APIHTTPMethod) &&
			flow.Path == match.APIPath {
			return flow, true
		}
	}
	return WorkspaceFeatureFlowRecord{}, false
}

func workspaceSymbolAPIConsumer(symbols []CanonicalSymbolRecord, flow WorkspaceFeatureFlowRecord, match WorkspaceContractMatchRecord) (CanonicalSymbolRecord, CodeFlowStep, bool) {
	if flow.FrontendComponent != "" {
		for _, step := range flow.FrontendSteps {
			if step.Name != flow.FrontendComponent || step.File == "" {
				continue
			}
			candidates := workspaceSymbolAPIScriptCandidates(symbols, flow.FrontendProject, step.File, step.Name)
			if len(candidates) == 1 {
				return candidates[0], step, true
			}
		}
		candidates := workspaceSymbolAPIScriptCandidates(
			symbols,
			flow.FrontendProject,
			flow.FrontendRouteFile,
			flow.FrontendComponent,
		)
		if len(candidates) == 1 {
			return candidates[0], CodeFlowStep{
				Name:       flow.FrontendComponent,
				Kind:       "component",
				Language:   candidates[0].Language,
				File:       flow.FrontendRouteFile,
				Line:       flow.FrontendRouteLine,
				Confidence: flow.FrontendConfidence,
				Reason:     flow.FrontendReason,
			}, true
		}
		return CanonicalSymbolRecord{}, CodeFlowStep{}, false
	}
	for _, step := range flow.FrontendSteps {
		if step.File == "" || step.Name == "" || workspaceSymbolAPIHelperStep(step, flow, match) {
			continue
		}
		candidates := workspaceSymbolAPIScriptCandidates(symbols, flow.FrontendProject, step.File, step.Name)
		if len(candidates) == 1 {
			return candidates[0], step, true
		}
	}
	candidates := workspaceSymbolAPIScriptCandidates(symbols, flow.FrontendProject, match.APIFile, firstNonEmpty(flow.FrontendCaller, match.APICaller))
	if len(candidates) != 1 {
		return CanonicalSymbolRecord{}, CodeFlowStep{}, false
	}
	return candidates[0], CodeFlowStep{
		Name:       firstNonEmpty(flow.FrontendCaller, match.APICaller),
		Kind:       "api_helper",
		Language:   candidates[0].Language,
		File:       match.APIFile,
		Line:       match.APILine,
		Confidence: firstNonEmpty(flow.FrontendConfidence, flow.Confidence),
	}, true
}

func workspaceSymbolAPIHelper(symbols []CanonicalSymbolRecord, flow WorkspaceFeatureFlowRecord, match WorkspaceContractMatchRecord) (CanonicalSymbolRecord, CodeFlowStep) {
	for _, step := range flow.FrontendSteps {
		if workspaceSymbolAPIHelperStep(step, flow, match) {
			candidates := workspaceSymbolAPIScriptCandidates(symbols, flow.FrontendProject, step.File, step.Name)
			if len(candidates) == 1 {
				return candidates[0], step
			}
			return CanonicalSymbolRecord{}, step
		}
	}
	name := firstNonEmpty(flow.FrontendCaller, match.APICaller)
	step := CodeFlowStep{
		Name:       name,
		Kind:       "api_helper",
		File:       match.APIFile,
		Line:       match.APILine,
		Confidence: flow.Confidence,
	}
	candidates := workspaceSymbolAPIScriptCandidates(symbols, flow.FrontendProject, match.APIFile, name)
	if len(candidates) == 1 {
		return candidates[0], step
	}
	return CanonicalSymbolRecord{}, step
}

func workspaceSymbolAPIHelperStep(step CodeFlowStep, flow WorkspaceFeatureFlowRecord, match WorkspaceContractMatchRecord) bool {
	name := firstNonEmpty(flow.FrontendCaller, match.APICaller)
	return step.File == match.APIFile && step.Name == name
}

func workspaceSymbolAPIScriptCandidates(symbols []CanonicalSymbolRecord, project, file, name string) []CanonicalSymbolRecord {
	if project == "" || file == "" || name == "" {
		return nil
	}
	var candidates []CanonicalSymbolRecord
	for _, symbol := range symbols {
		if symbol.Project != project ||
			!isScriptLanguage(symbol.Language) ||
			symbol.DeclarationFile != file ||
			!workspaceSymbolAPINameMatches(symbol, name) {
			continue
		}
		candidates = append(candidates, symbol)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
	return candidates
}

func workspaceSymbolAPIJavaCandidates(symbols []CanonicalSymbolRecord, project string, step SpringEndpointFlowStep) []CanonicalSymbolRecord {
	if project == "" || step.File == "" || step.Owner == "" {
		return nil
	}
	var candidates []CanonicalSymbolRecord
	for _, symbol := range symbols {
		if symbol.Project != project ||
			symbol.Language != "java" ||
			symbol.DeclarationFile != step.File ||
			!workspaceSymbolAPINameMatches(symbol, step.Owner) {
			continue
		}
		candidates = append(candidates, symbol)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
	return candidates
}

func workspaceSymbolAPINameMatches(symbol CanonicalSymbolRecord, name string) bool {
	return symbol.Name == name ||
		symbol.ExportName == name ||
		symbol.QualifiedName == name ||
		strings.HasSuffix(symbol.QualifiedName, "."+name)
}

func buildWorkspaceSymbolAPIUsage(
	match WorkspaceContractMatchRecord,
	flow WorkspaceFeatureFlowRecord,
	trace WorkspaceEndpointTraceRecord,
	consumer CanonicalSymbolRecord,
	originStep CodeFlowStep,
	helper CanonicalSymbolRecord,
	helperStep CodeFlowStep,
	implementation SpringEndpointFlowStep,
	candidates []CanonicalSymbolRecord,
) []CanonicalSymbolUsageRecord {
	base := CanonicalSymbolUsageRecord{
		ConsumerProject:  flow.FrontendProject,
		ConsumerSymbolID: consumer.ID,
		Language:         consumer.Language,
		RelationKind:     symbolAPIRelationKind,
		SourceFile:       consumer.DeclarationFile,
		SourceLine:       consumer.DeclarationLine,
		Analyzer:         symbolAPIAnalyzer,
		Transport:        "http",
	}
	if workspaceSymbolAPIPartialFrontendContext(flow.FrontendConfidence) {
		base.Limitations = []string{"frontend_route_context_partial"}
	}
	switch len(candidates) {
	case 0:
		base.Category = SymbolUsageUnresolved
		base.Confidence = ConfidenceNormalized
		base.Resolution = SymbolResolutionUnresolved
		base.Reason = "resolved HTTP contract reaches a Spring implementation step without a uniquely indexed Java declaration"
		base.APIPath = workspaceSymbolAPIPath(match, trace, consumer, originStep, helper, helperStep, implementation, CanonicalSymbolRecord{})
		base.EvidenceIDs = workspaceSymbolAPIPathEvidence(base.APIPath)
		base.ID = workspaceSymbolAPIUsageID(base, match.ID, implementation)
		return []CanonicalSymbolUsageRecord{base}
	case 1:
		base.ProviderSymbolID = candidates[0].ID
		base.Category = SymbolUsageReachedThroughAPI
		base.Confidence = ConfidenceExact
		if len(base.Limitations) > 0 {
			base.Confidence = ConfidenceNormalized
		}
		base.Resolution = SymbolResolutionExact
		base.Reason = "resolved HTTP contract and ordered Spring implementation evidence select one indexed Java declaration"
		base.APIPath = workspaceSymbolAPIPath(match, trace, consumer, originStep, helper, helperStep, implementation, candidates[0])
		base.EvidenceIDs = workspaceSymbolAPIPathEvidence(base.APIPath)
		base.ID = workspaceSymbolAPIUsageID(base, match.ID, implementation)
		return []CanonicalSymbolUsageRecord{base}
	default:
		candidateIDs := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			candidateIDs = append(candidateIDs, candidate.ID)
		}
		var usages []CanonicalSymbolUsageRecord
		for _, candidate := range candidates {
			usage := base
			usage.ProviderSymbolID = candidate.ID
			usage.Category = SymbolUsageAmbiguous
			usage.Confidence = ConfidenceNormalized
			usage.Resolution = SymbolResolutionAmbiguous
			usage.Reason = "multiple indexed Java declarations match the resolved HTTP implementation step"
			usage.CandidateSymbolIDs = append([]string(nil), candidateIDs...)
			usage.APIPath = workspaceSymbolAPIPath(match, trace, consumer, originStep, helper, helperStep, implementation, candidate)
			usage.EvidenceIDs = workspaceSymbolAPIPathEvidence(usage.APIPath)
			usage.ID = workspaceSymbolAPIUsageID(usage, match.ID, implementation)
			usages = append(usages, usage)
		}
		return usages
	}
}

func workspaceSymbolAPIPartialFrontendContext(confidence string) bool {
	switch strings.ToUpper(strings.TrimSpace(confidence)) {
	case "", "EXACT", "EXTRACTED", "MATCHED", "RESOLVED":
		return false
	default:
		return true
	}
}

func workspaceSymbolAPIPath(
	match WorkspaceContractMatchRecord,
	trace WorkspaceEndpointTraceRecord,
	consumer CanonicalSymbolRecord,
	originStep CodeFlowStep,
	helper CanonicalSymbolRecord,
	helperStep CodeFlowStep,
	implementation SpringEndpointFlowStep,
	selected CanonicalSymbolRecord,
) []SymbolAPIPathStepRecord {
	originEvidence := mergeWorkspaceSymbolAPIEvidence(
		consumer.EvidenceIDs,
		namespaceWorkspaceSymbolAPIEvidence(match.APIProject, originStep.EvidenceIDs),
	)
	helperEvidence := namespaceWorkspaceSymbolAPIEvidence(match.APIProject, helperStep.EvidenceIDs)
	if helper.ID != "" {
		helperEvidence = mergeWorkspaceSymbolAPIEvidence(helperEvidence, helper.EvidenceIDs)
	}
	route := workspaceSymbolAPITraceStep(trace, "backend_route")
	handler := workspaceSymbolAPITraceStep(trace, "backend_handler")
	routeLabel := strings.TrimSpace(firstNonEmpty(match.BackendHTTPMethod, match.APIHTTPMethod) + " " + firstNonEmpty(match.BackendPath, match.APIPath))
	handlerLabel := firstNonEmpty(match.BackendHandler, strings.Trim(strings.TrimSpace(match.BackendHandler), "."))
	if route.Label == "" {
		route = WorkspaceEndpointTraceStepRecord{
			Label:   routeLabel,
			Project: match.BackendProject,
			File:    match.BackendFile,
			Line:    match.BackendLine,
		}
	}
	if handler.Label == "" {
		handler = WorkspaceEndpointTraceStepRecord{
			Label:   handlerLabel,
			Project: match.BackendProject,
			File:    match.BackendFile,
			Line:    match.BackendLine,
		}
	}
	helperLabel := firstNonEmpty(helperStep.Name, helper.Name, match.APICaller, strings.TrimSpace(match.APIHTTPMethod+" "+match.APIPath))
	helperFile := firstNonEmpty(helperStep.File, helper.DeclarationFile, match.APIFile)
	helperLine := firstNonZero(helperStep.Line, helper.DeclarationLine, match.APILine)
	path := []SymbolAPIPathStepRecord{
		{
			Kind: "frontend_symbol", Project: consumer.Project, SymbolID: consumer.ID,
			Label: firstNonEmpty(consumer.QualifiedName, consumer.Name),
			File:  consumer.DeclarationFile, Line: consumer.DeclarationLine, EvidenceIDs: originEvidence,
		},
		{
			Kind: "api_helper", Project: match.APIProject, SymbolID: helper.ID,
			Label: helperLabel, File: helperFile, Line: helperLine, EvidenceIDs: helperEvidence,
		},
		{
			Kind: "http_contract", Project: match.APIProject,
			Label: strings.TrimSpace(match.APIHTTPMethod + " " + match.APIPath),
			File:  match.APIFile, Line: match.APILine,
		},
		{
			Kind: "workspace_contract", Project: match.APIProject,
			Label: firstNonEmpty(match.ID, strings.TrimSpace(match.APIHTTPMethod+" "+match.APIPath)),
		},
		{
			Kind: "spring_route", Project: firstNonEmpty(route.Project, match.BackendProject),
			Label: route.Label, File: route.File, Line: route.Line,
		},
		{
			Kind: "spring_handler", Project: firstNonEmpty(handler.Project, match.BackendProject),
			Label: handler.Label, File: handler.File, Line: handler.Line,
		},
		{
			Kind: "java_implementation", Project: match.BackendProject,
			Label: strings.Trim(strings.TrimSpace(implementation.Owner)+"."+strings.TrimSpace(implementation.Method), "."),
			File:  implementation.File, Line: implementation.Line,
		},
	}
	if selected.ID != "" {
		path = append(path, SymbolAPIPathStepRecord{
			Kind: "selected_symbol", Project: selected.Project, SymbolID: selected.ID,
			Label: firstNonEmpty(selected.QualifiedName, selected.Name),
			File:  selected.DeclarationFile, Line: selected.DeclarationLine,
			EvidenceIDs: append([]string(nil), selected.EvidenceIDs...),
		})
	}
	for position := range path {
		path[position].Position = position
	}
	return path
}

func workspaceSymbolAPITraceStep(trace WorkspaceEndpointTraceRecord, kind string) WorkspaceEndpointTraceStepRecord {
	for _, step := range trace.Steps {
		if step.Kind == kind {
			return step
		}
	}
	return WorkspaceEndpointTraceStepRecord{}
}

func workspaceSymbolAPIUsageID(base CanonicalSymbolUsageRecord, contractID string, implementation SpringEndpointFlowStep) string {
	target := strings.Join([]string{
		contractID,
		implementation.Owner,
		implementation.Method,
		implementation.File,
		fmt.Sprint(implementation.Line),
	}, "\x00")
	return StableWorkspaceUsageID(
		base.ProviderSymbolID,
		base.ConsumerProject,
		base.ConsumerSymbolID,
		base.Category,
		base.RelationKind,
		target,
		base.SourceFile,
		base.SourceLine,
	)
}

func workspaceSymbolAPIPathEvidence(path []SymbolAPIPathStepRecord) []string {
	var evidence []string
	for _, step := range path {
		evidence = append(evidence, step.EvidenceIDs...)
	}
	return mergeWorkspaceSymbolAPIEvidence(evidence)
}

func namespaceWorkspaceSymbolAPIEvidence(project string, evidence []string) []string {
	result := make([]string, 0, len(evidence))
	for _, id := range evidence {
		if id == "" {
			continue
		}
		if strings.Contains(id, "#") {
			result = append(result, id)
			continue
		}
		result = append(result, WorkspaceEvidenceID(project, id))
	}
	return mergeWorkspaceSymbolAPIEvidence(result)
}

func mergeWorkspaceSymbolAPIEvidence(groups ...[]string) []string {
	var result []string
	for _, group := range groups {
		result = append(result, group...)
	}
	sort.Strings(result)
	return dedupeStrings(result)
}
