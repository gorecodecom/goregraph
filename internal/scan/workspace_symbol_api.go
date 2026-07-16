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

type workspaceSymbolAPIOrigin struct {
	flow       WorkspaceFeatureFlowRecord
	consumer   CanonicalSymbolRecord
	originStep CodeFlowStep
	helper     CanonicalSymbolRecord
	helperStep CodeFlowStep
}

// BuildWorkspaceSymbolAPIUsages derives HTTP-mediated symbol usages from
// resolved frontend contracts and their Spring implementation flows.
func BuildWorkspaceSymbolAPIUsages(symbols WorkspaceSymbolIndexRecord, matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, traces WorkspaceEndpointTraceIndexRecord) []CanonicalSymbolUsageRecord {
	traceByContract := make(map[string]WorkspaceEndpointTraceRecord, len(traces.Traces))
	for _, trace := range traces.Traces {
		traceByContract[trace.ID] = trace
	}

	var usages []CanonicalSymbolUsageRecord
	for _, match := range matches {
		if match.Issue != contractIssueMatched || !strings.EqualFold(match.Confidence, string(ConfidenceResolved)) {
			continue
		}
		candidateFlows := workspaceSymbolAPIFlows(match, flows)
		if len(candidateFlows) == 0 {
			usages = append(usages, unresolvedWorkspaceSymbolAPIUsage(
				symbols,
				match,
				WorkspaceFeatureFlowRecord{},
				traceByContract[match.ID],
				CanonicalSymbolRecord{},
				CodeFlowStep{},
				CanonicalSymbolRecord{},
				CodeFlowStep{},
				"resolved HTTP contract has no feature flow with the same call-site identity",
				"workspace_feature_flow_missing",
				nil,
			))
			continue
		}
		flowCandidateIDs := workspaceSymbolAPIFlowCandidateIDs(candidateFlows)
		origins := workspaceSymbolAPIOrigins(symbols.Symbols, match, candidateFlows)
		if len(origins) == 0 {
			helper, helperStep := workspaceSymbolAPIHelper(symbols.Symbols, candidateFlows[0], match)
			usages = append(usages, unresolvedWorkspaceSymbolAPIUsage(
				symbols,
				match,
				candidateFlows[0],
				traceByContract[match.ID],
				CanonicalSymbolRecord{},
				CodeFlowStep{},
				helper,
				helperStep,
				"resolved HTTP contract has feature-flow evidence but no uniquely selectable frontend origin",
				"frontend_origin_unresolved",
				flowCandidateIDs,
			))
			continue
		}
		originCandidateIDs := workspaceSymbolAPIOriginCandidateIDs(origins)
		ambiguousOrigin := len(candidateFlows) > 1 || len(originCandidateIDs) > 1
		for _, origin := range origins {
			if len(origin.flow.BackendSteps) == 0 {
				usages = append(usages, unresolvedWorkspaceSymbolAPIUsage(
					symbols,
					match,
					origin.flow,
					traceByContract[match.ID],
					origin.consumer,
					origin.originStep,
					origin.helper,
					origin.helperStep,
					"resolved HTTP contract has no Spring implementation steps",
					"backend_implementation_steps_missing",
					flowCandidateIDs,
				))
				continue
			}
			for _, implementation := range origin.flow.BackendSteps {
				candidates := workspaceSymbolAPIJavaCandidates(symbols.Symbols, match.BackendProject, implementation)
				built := buildWorkspaceSymbolAPIUsage(
					match,
					origin.flow,
					traceByContract[match.ID],
					origin.consumer,
					origin.originStep,
					origin.helper,
					origin.helperStep,
					implementation,
					candidates,
				)
				built = markWorkspaceSymbolAPIFlowCandidates(
					built,
					match,
					origin.flow,
					implementation,
					originCandidateIDs,
					flowCandidateIDs,
					ambiguousOrigin,
				)
				usages = append(usages, built...)
			}
		}
	}
	usages = dedupeWorkspaceSymbolUsages(usages)
	sort.Slice(usages, func(i, j int) bool {
		return usages[i].ID < usages[j].ID
	})
	return usages
}

func workspaceSymbolAPIFlows(match WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord) []WorkspaceFeatureFlowRecord {
	var candidates []WorkspaceFeatureFlowRecord
	for _, flow := range flows {
		if flow.FrontendProject != match.APIProject ||
			flow.BackendProject != match.BackendProject ||
			flow.FrontendFile != match.APIFile ||
			!strings.EqualFold(flow.HTTPMethod, match.APIHTTPMethod) ||
			flow.Path != match.APIPath {
			continue
		}
		if match.APILine > 0 && flow.FrontendLine != match.APILine {
			continue
		}
		if match.APICaller != "" && flow.FrontendCaller != match.APICaller {
			continue
		}
		candidates = append(candidates, flow)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return workspaceSymbolAPIFlowIdentity(candidates[i]) < workspaceSymbolAPIFlowIdentity(candidates[j])
	})
	return candidates
}

func workspaceSymbolAPIFlowIdentity(flow WorkspaceFeatureFlowRecord) string {
	return strings.Join([]string{
		flow.ID,
		flow.FrontendProject,
		flow.FrontendFile,
		fmt.Sprint(flow.FrontendLine),
		flow.FrontendCaller,
		flow.FrontendComponent,
		flow.BackendProject,
		flow.HTTPMethod,
		flow.Path,
	}, "\x00")
}

func workspaceSymbolAPIOrigins(symbols []CanonicalSymbolRecord, match WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord) []workspaceSymbolAPIOrigin {
	var origins []workspaceSymbolAPIOrigin
	for _, flow := range flows {
		consumer, originStep, ok := workspaceSymbolAPIConsumer(symbols, flow, match)
		if !ok {
			continue
		}
		helper, helperStep := workspaceSymbolAPIHelper(symbols, flow, match)
		origins = append(origins, workspaceSymbolAPIOrigin{
			flow:       flow,
			consumer:   consumer,
			originStep: originStep,
			helper:     helper,
			helperStep: helperStep,
		})
	}
	sort.Slice(origins, func(i, j int) bool {
		if origins[i].consumer.ID != origins[j].consumer.ID {
			return origins[i].consumer.ID < origins[j].consumer.ID
		}
		return workspaceSymbolAPIFlowIdentity(origins[i].flow) < workspaceSymbolAPIFlowIdentity(origins[j].flow)
	})
	return origins
}

func workspaceSymbolAPIOriginCandidateIDs(origins []workspaceSymbolAPIOrigin) []string {
	var ids []string
	for _, origin := range origins {
		if origin.consumer.ID != "" {
			ids = append(ids, origin.consumer.ID)
		}
	}
	return sortedUniqueStrings(ids)
}

func workspaceSymbolAPIFlowCandidateIDs(flows []WorkspaceFeatureFlowRecord) []string {
	var ids []string
	for _, flow := range flows {
		id := flow.ID
		if id == "" {
			id = StableWorkspaceID("workspace-symbol-api-flow", workspaceSymbolAPIFlowIdentity(flow))
		}
		ids = append(ids, id)
	}
	return sortedUniqueStrings(ids)
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

func unresolvedWorkspaceSymbolAPIUsage(
	symbols WorkspaceSymbolIndexRecord,
	match WorkspaceContractMatchRecord,
	flow WorkspaceFeatureFlowRecord,
	trace WorkspaceEndpointTraceRecord,
	consumer CanonicalSymbolRecord,
	originStep CodeFlowStep,
	helper CanonicalSymbolRecord,
	helperStep CodeFlowStep,
	reason string,
	limitation string,
	candidatePathIDs []string,
) CanonicalSymbolUsageRecord {
	if flow.FrontendProject == "" {
		flow.FrontendProject = match.APIProject
		flow.FrontendCaller = match.APICaller
		flow.FrontendFile = match.APIFile
		flow.FrontendLine = match.APILine
		flow.HTTPMethod = match.APIHTTPMethod
		flow.Path = match.APIPath
		flow.BackendProject = match.BackendProject
		flow.BackendFile = match.BackendFile
		flow.BackendLine = match.BackendLine
	}
	sourceFile := firstNonEmpty(consumer.DeclarationFile, match.APIFile)
	sourceLine := firstNonZero(consumer.DeclarationLine, match.APILine)
	usage := CanonicalSymbolUsageRecord{
		ConsumerProject:  match.APIProject,
		ConsumerSymbolID: consumer.ID,
		Category:         SymbolUsageUnresolved,
		Language: workspaceSymbolAPIUsageLanguage(
			symbols,
			match,
			flow,
			consumer,
			originStep,
			helper,
			helperStep,
		),
		RelationKind:     symbolAPIRelationKind,
		SourceFile:       sourceFile,
		SourceLine:       sourceLine,
		Confidence:       ConfidenceNormalized,
		Resolution:       SymbolResolutionUnresolved,
		Reason:           reason,
		Analyzer:         symbolAPIAnalyzer,
		Transport:        "http",
		Limitations:      []string{limitation},
		CandidatePathIDs: append([]string(nil), candidatePathIDs...),
		APIPath: workspaceSymbolAPIPath(
			match,
			trace,
			consumer,
			originStep,
			helper,
			helperStep,
			SpringEndpointFlowStep{},
			CanonicalSymbolRecord{},
		),
	}
	usage.EvidenceIDs = workspaceSymbolAPIPathEvidence(usage.APIPath)
	usage.ID = StableWorkspaceUsageID(
		"",
		usage.ConsumerProject,
		usage.ConsumerSymbolID,
		usage.Category,
		usage.RelationKind,
		strings.Join([]string{match.ID, flow.ID, limitation}, "\x00"),
		usage.SourceFile,
		usage.SourceLine,
	)
	return usage
}

func markWorkspaceSymbolAPIFlowCandidates(
	usages []CanonicalSymbolUsageRecord,
	match WorkspaceContractMatchRecord,
	flow WorkspaceFeatureFlowRecord,
	implementation SpringEndpointFlowStep,
	originCandidateIDs []string,
	flowCandidateIDs []string,
	ambiguousOrigin bool,
) []CanonicalSymbolUsageRecord {
	for index := range usages {
		if usages[index].Resolution != SymbolResolutionExact || len(flowCandidateIDs) > 1 {
			usages[index].CandidatePathIDs = append([]string(nil), flowCandidateIDs...)
		}
		if len(flowCandidateIDs) < 2 {
			continue
		}
		usages[index].Limitations = sortedUniqueStrings(append(usages[index].Limitations, "feature_flow_join_ambiguous"))
		if usages[index].Category == SymbolUsageReachedThroughAPI && ambiguousOrigin {
			usages[index].Category = SymbolUsageAmbiguous
			usages[index].Resolution = SymbolResolutionAmbiguous
			usages[index].Confidence = ConfidenceNormalized
			usages[index].Reason = "multiple feature flows survive the complete contract call-site identity"
			usages[index].CandidateSymbolIDs = append([]string(nil), originCandidateIDs...)
		} else if usages[index].Category == SymbolUsageAmbiguous {
			usages[index].Reason += "; multiple feature flows survive the complete contract call-site identity"
		}
		usages[index].ID = workspaceSymbolAPIUsageID(usages[index], match.ID+"\x00"+flow.ID, implementation)
	}
	return usages
}

func workspaceSymbolAPIUsageLanguage(
	symbols WorkspaceSymbolIndexRecord,
	match WorkspaceContractMatchRecord,
	flow WorkspaceFeatureFlowRecord,
	consumer CanonicalSymbolRecord,
	originStep CodeFlowStep,
	helper CanonicalSymbolRecord,
	helperStep CodeFlowStep,
) string {
	for _, language := range []string{
		consumer.Language,
		originStep.Language,
		helper.Language,
		helperStep.Language,
	} {
		if isScriptLanguage(language) {
			return language
		}
	}
	for _, step := range flow.FrontendSteps {
		if isScriptLanguage(step.Language) {
			return step.Language
		}
	}
	for _, symbol := range symbols.Symbols {
		if symbol.Project == match.APIProject &&
			symbol.DeclarationFile == match.APIFile &&
			isScriptLanguage(symbol.Language) {
			return symbol.Language
		}
	}
	for _, coverage := range symbols.Coverage {
		if coverage.Project == match.APIProject && isScriptLanguage(coverage.Language) {
			return coverage.Language
		}
	}
	if language := detectLanguage(match.APIFile); isScriptLanguage(language) {
		return language
	}
	return "unknown"
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
	originProject := firstNonEmpty(consumer.Project, match.APIProject)
	originLabel := firstNonEmpty(consumer.QualifiedName, consumer.Name, originStep.Name, match.APICaller, match.APIFile)
	originFile := firstNonEmpty(consumer.DeclarationFile, originStep.File, match.APIFile)
	originLine := firstNonZero(consumer.DeclarationLine, originStep.Line, match.APILine)
	path := []SymbolAPIPathStepRecord{
		{
			Kind: "frontend_symbol", Project: originProject, SymbolID: consumer.ID,
			Label: originLabel,
			File:  originFile, Line: originLine, EvidenceIDs: originEvidence,
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
	}
	if implementation.Owner != "" || implementation.Method != "" || implementation.File != "" || implementation.Line > 0 {
		path = append(path, SymbolAPIPathStepRecord{
			Kind: "java_implementation", Project: match.BackendProject,
			Label: strings.Trim(strings.TrimSpace(implementation.Owner)+"."+strings.TrimSpace(implementation.Method), "."),
			File:  implementation.File, Line: implementation.Line,
		})
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

// BuildWorkspaceSymbolAPIUsageCoverage reports whether each supported project
// has enough static evidence to distinguish verified HTTP reachability from
// missing or incomplete reachability evidence.
func BuildWorkspaceSymbolAPIUsageCoverage(
	symbols WorkspaceSymbolIndexRecord,
	matches []WorkspaceContractMatchRecord,
	flows []WorkspaceFeatureFlowRecord,
	projects []workspaceIndexProject,
) []SymbolCoverageRecord {
	if len(projects) == 0 {
		return nil
	}
	var records []SymbolCoverageRecord
	for _, project := range projects {
		for _, language := range workspaceSymbolAPIUsageLanguages(project, symbols.Symbols) {
			if project.record.Kind == "frontend" && !isScriptLanguage(language) {
				continue
			}
			if project.record.Kind == "backend" && language != "java" {
				continue
			}
			records = append(records, workspaceSymbolAPIUsageCoverageRecord(project, language, symbols, matches, flows))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Project != records[j].Project {
			return records[i].Project < records[j].Project
		}
		return records[i].Language < records[j].Language
	})
	return records
}

func workspaceSymbolAPIUsageLanguages(project workspaceIndexProject, symbols []CanonicalSymbolRecord) []string {
	languages := map[string]bool{}
	add := func(language string) {
		if isWorkspaceSymbolLanguageSupported(language) {
			languages[language] = true
		}
	}
	for _, symbol := range symbols {
		if symbol.Project == project.record.Path {
			add(symbol.Language)
		}
	}
	for _, language := range workspaceSymbolLanguages(project) {
		add(language)
	}
	for _, route := range project.routes {
		add(route.Language)
	}
	for _, flow := range project.codeFlows {
		add(flow.Language)
		for _, step := range flow.Steps {
			add(step.Language)
		}
	}
	if project.record.Kind == "backend" &&
		(project.record.BuildSystem == "maven" ||
			project.record.BuildSystem == "gradle" ||
			len(project.spring.Applications) > 0 ||
			len(project.spring.Components) > 0 ||
			len(project.spring.Endpoints) > 0 ||
			len(project.endpoints) > 0 ||
			len(project.endpointFlows) > 0) {
		add("java")
	}
	result := make([]string, 0, len(languages))
	for language := range languages {
		result = append(result, language)
	}
	sort.Strings(result)
	return result
}

func workspaceSymbolAPIUsageCoverageRecord(
	project workspaceIndexProject,
	language string,
	symbols WorkspaceSymbolIndexRecord,
	matches []WorkspaceContractMatchRecord,
	flows []WorkspaceFeatureFlowRecord,
) SymbolCoverageRecord {
	record := SymbolCoverageRecord{
		Project:    project.record.Path,
		Language:   language,
		Capability: symbolAPIRelationKind,
		Coverage:   CoverageComplete,
	}
	requiredFile := "endpoint-flows.json"
	failedLimitation := "endpoint_flows_unreadable"
	missingLimitation := "endpoint_flows_missing"
	emptyLimitation := "endpoint_flows_empty"
	if isScriptLanguage(language) {
		requiredFile = "flows.json"
		failedLimitation = "flows_unreadable"
		missingLimitation = "flows_missing"
		emptyLimitation = "flows_empty"
	}
	if len(workspaceSymbolFactFailures(project.loadFailures, []string{requiredFile})) > 0 {
		record.Coverage = CoverageFailed
		record.Reason = requiredFile + " could not be read; HTTP reachability is not verified for this project"
		record.Limitations = []string{failedLimitation}
		return record
	}
	if len(workspaceSymbolMissingFacts(project.missingFacts, []string{requiredFile})) > 0 {
		record.Coverage = CoveragePartial
		record.Reason = requiredFile + " is missing; absence of HTTP reachability records is not evidence of no usage"
		record.Limitations = []string{missingLimitation}
		return record
	}
	resolved := workspaceSymbolAPIResolvedMatches(project.record.Path, matches)
	if len(resolved) == 0 {
		record.Reason = "required reachability inputs were loaded; no resolved HTTP contracts involve this project"
		return record
	}
	if isScriptLanguage(language) && len(project.codeFlows) == 0 {
		record.Coverage = CoveragePartial
		record.Reason = "flows.json contains no frontend flow evidence for resolved HTTP contracts"
		record.Limitations = []string{emptyLimitation}
		return record
	}
	if language == "java" && len(project.endpointFlows) == 0 {
		record.Coverage = CoveragePartial
		record.Reason = "endpoint-flows.json contains no Spring implementation flow evidence for resolved HTTP contracts"
		record.Limitations = []string{emptyLimitation}
		return record
	}

	var limitations []string
	for _, match := range resolved {
		candidateFlows := workspaceSymbolAPIFlows(match, flows)
		if len(candidateFlows) == 0 {
			limitations = append(limitations, "workspace_feature_flow_missing")
			continue
		}
		origins := workspaceSymbolAPIOrigins(symbols.Symbols, match, candidateFlows)
		switch {
		case len(origins) == 0:
			limitations = append(limitations, "frontend_origin_unresolved")
		case len(candidateFlows) > 1 || len(workspaceSymbolAPIOriginCandidateIDs(origins)) > 1:
			limitations = append(limitations, "feature_flow_join_ambiguous")
		}
		for _, flow := range candidateFlows {
			if len(flow.BackendSteps) == 0 {
				limitations = append(limitations, "backend_implementation_steps_missing")
				continue
			}
			for _, step := range flow.BackendSteps {
				switch len(workspaceSymbolAPIJavaCandidates(symbols.Symbols, match.BackendProject, step)) {
				case 0:
					limitations = append(limitations, "java_provider_unresolved")
				case 1:
				default:
					limitations = append(limitations, "java_provider_ambiguous")
				}
			}
		}
	}
	if len(limitations) > 0 {
		record.Limitations = sortedUniqueStrings(limitations)
		record.Coverage = CoveragePartial
		record.Reason = workspaceSymbolAPIUsageCoverageReason(record.Limitations)
		return record
	}
	record.Reason = "resolved HTTP contracts have uniquely selected frontend origins, paths, and Java providers"
	return record
}

func workspaceSymbolAPIUsageCoverageReason(limitations []string) string {
	for _, limitation := range limitations {
		if limitation == "java_provider_unresolved" || limitation == "java_provider_ambiguous" {
			return "resolved HTTP contracts do not select exactly one indexed Java provider"
		}
	}
	return "resolved HTTP contracts have incomplete or ambiguous static reachability evidence"
}

func workspaceSymbolAPIResolvedMatches(project string, matches []WorkspaceContractMatchRecord) []WorkspaceContractMatchRecord {
	var result []WorkspaceContractMatchRecord
	for _, match := range matches {
		if match.Issue != contractIssueMatched || !strings.EqualFold(match.Confidence, string(ConfidenceResolved)) {
			continue
		}
		if match.APIProject == project || match.BackendProject == project {
			result = append(result, match)
		}
	}
	return result
}
