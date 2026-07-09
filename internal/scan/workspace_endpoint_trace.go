package scan

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

func BuildWorkspaceEndpointTraces(matches []WorkspaceContractMatchRecord, flows []WorkspaceFeatureFlowRecord, dossiers []FeatureDossierRecord) WorkspaceEndpointTraceIndexRecord {
	flowByRoute := map[string]WorkspaceFeatureFlowRecord{}
	for _, flow := range flows {
		key := endpointTraceKey(flow.FrontendProject, flow.HTTPMethod, flow.Path, flow.FrontendFile)
		flowByRoute[key] = flow
		if flow.FrontendFile != "" {
			flowByRoute[endpointTraceKey(flow.FrontendProject, flow.HTTPMethod, flow.Path, "")] = flow
		}
	}
	dossierByFlow := map[string]FeatureDossierRecord{}
	for _, dossier := range dossiers {
		if dossier.SourceFlowID != "" {
			dossierByFlow[dossier.SourceFlowID] = dossier
		}
	}
	traces := make([]WorkspaceEndpointTraceRecord, 0, len(matches))
	for _, match := range matches {
		flow := flowByRoute[endpointTraceKey(match.APIProject, match.APIHTTPMethod, match.APIPath, match.APIFile)]
		if flow.ID == "" {
			flow = flowByRoute[endpointTraceKey(match.APIProject, match.APIHTTPMethod, match.APIPath, "")]
		}
		trace := buildWorkspaceEndpointTrace(match, flow, dossierByFlow[flow.ID])
		traces = append(traces, trace)
	}
	sort.Slice(traces, func(i, j int) bool {
		if traces[i].FromProject == traces[j].FromProject {
			return traces[i].Route < traces[j].Route
		}
		return traces[i].FromProject < traces[j].FromProject
	})
	return WorkspaceEndpointTraceIndexRecord{
		SchemaVersion: SchemaVersion,
		Generated:     time.Now().UTC().Format(time.RFC3339),
		Traces:        traces,
		Stats: map[string]int{
			"traces": len(traces),
		},
	}
}

func buildWorkspaceEndpointTrace(match WorkspaceContractMatchRecord, flow WorkspaceFeatureFlowRecord, dossier FeatureDossierRecord) WorkspaceEndpointTraceRecord {
	route := strings.TrimSpace(match.APIHTTPMethod + " " + match.APIPath)
	trace := WorkspaceEndpointTraceRecord{
		ID:          firstNonEmpty(match.ID, StableWorkspaceID("endpoint-trace", match.APIProject, match.APIHTTPMethod, match.APIPath, match.APIFile)),
		Route:       route,
		Method:      match.APIHTTPMethod,
		Path:        match.APIPath,
		FromProject: match.APIProject,
		ToProject:   firstNonEmpty(match.BackendProject, match.ServiceCandidate),
		Status:      firstNonEmpty(match.Confidence, flow.Confidence),
		Risk:        firstNonEmpty(match.Issue, riskFromFlow(flow), riskFromDossier(dossier)),
	}
	for _, step := range flow.FrontendSteps {
		trace.addStep(WorkspaceEndpointTraceStepRecord{
			ID:         StableWorkspaceID("trace-step", trace.ID, "frontend", step.File, step.Name, step.LineString()),
			Kind:       "frontend_step",
			Label:      firstNonEmpty(step.Name, step.File),
			Project:    firstNonEmpty(flow.FrontendProject, match.APIProject),
			File:       step.File,
			Line:       step.Line,
			Symbol:     step.Name,
			Confidence: step.Confidence,
		})
	}
	trace.addStep(WorkspaceEndpointTraceStepRecord{
		ID:         StableWorkspaceID("trace-step", trace.ID, "api-contract", match.APIFile, match.APICaller),
		Kind:       "api_contract",
		Label:      firstNonEmpty(match.APICaller, route),
		Project:    match.APIProject,
		File:       match.APIFile,
		Line:       match.APILine,
		Symbol:     match.APICaller,
		Confidence: match.Confidence,
	})
	if match.BackendProject != "" || match.BackendPath != "" {
		backendRoute := strings.TrimSpace(firstNonEmpty(match.BackendHTTPMethod, match.APIHTTPMethod) + " " + firstNonEmpty(match.BackendPath, match.APIPath))
		trace.addStep(WorkspaceEndpointTraceStepRecord{
			ID:         StableWorkspaceID("trace-step", trace.ID, "backend-route", match.BackendProject, backendRoute),
			Kind:       "backend_route",
			Label:      backendRoute,
			Project:    match.BackendProject,
			File:       match.BackendFile,
			Line:       match.BackendLine,
			Symbol:     match.BackendHandler,
			Confidence: match.Confidence,
		})
	}
	if match.BackendHandler != "" || flow.BackendController != "" || flow.BackendMethod != "" {
		handler := firstNonEmpty(match.BackendHandler, strings.Trim(strings.TrimSpace(flow.BackendController)+"."+strings.TrimSpace(flow.BackendMethod), "."))
		trace.addStep(WorkspaceEndpointTraceStepRecord{
			ID:         StableWorkspaceID("trace-step", trace.ID, "backend-handler", match.BackendProject, match.BackendFile, handler),
			Kind:       "backend_handler",
			Label:      handler,
			Project:    firstNonEmpty(match.BackendProject, flow.BackendProject),
			File:       firstNonEmpty(match.BackendFile, flow.BackendFile),
			Line:       firstNonZero(match.BackendLine, flow.BackendLine),
			Symbol:     handler,
			Confidence: firstNonEmpty(match.Confidence, flow.Confidence),
		})
	}
	for _, step := range flow.BackendSteps {
		label := strings.Trim(strings.TrimSpace(step.Owner)+"."+strings.TrimSpace(step.Method), ".")
		trace.addStep(WorkspaceEndpointTraceStepRecord{
			ID:         StableWorkspaceID("trace-step", trace.ID, "backend", step.File, label, step.Kind),
			Kind:       "backend_step",
			Label:      firstNonEmpty(label, step.File),
			Project:    firstNonEmpty(flow.BackendProject, match.BackendProject),
			File:       step.File,
			Line:       step.Line,
			Symbol:     label,
			Confidence: step.Confidence,
		})
	}
	for _, test := range flow.Tests {
		trace.addStep(WorkspaceEndpointTraceStepRecord{
			ID:         StableWorkspaceID("trace-step", trace.ID, "test", test.TestFile, test.TestClass, test.TestMethod),
			Kind:       "test",
			Label:      firstNonEmpty(strings.Trim(strings.TrimSpace(test.TestClass)+"."+strings.TrimSpace(test.TestMethod), "."), test.TestFile),
			Project:    firstNonEmpty(flow.BackendProject, match.BackendProject),
			File:       test.TestFile,
			Line:       test.Line,
			Symbol:     strings.Trim(strings.TrimSpace(test.TestClass)+"."+strings.TrimSpace(test.TestMethod), "."),
			Confidence: test.Confidence,
		})
	}
	trace.rebuildEdges()
	return trace
}

func (trace *WorkspaceEndpointTraceRecord) addStep(step WorkspaceEndpointTraceStepRecord) {
	if step.ID == "" || step.Label == "" {
		return
	}
	for _, existing := range trace.Steps {
		if existing.ID == step.ID {
			return
		}
	}
	trace.Steps = append(trace.Steps, step)
}

func (trace *WorkspaceEndpointTraceRecord) rebuildEdges() {
	trace.Edges = nil
	for i := 0; i+1 < len(trace.Steps); i++ {
		from := trace.Steps[i]
		to := trace.Steps[i+1]
		trace.Edges = append(trace.Edges, WorkspaceEndpointTraceEdgeRecord{
			From:      from.ID,
			To:        to.ID,
			Kind:      from.Kind + "_to_" + to.Kind,
			Direction: from.Label + " -> " + to.Label,
		})
	}
}

func endpointTraceKey(project, method, path, file string) string {
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(project),
		strings.TrimSpace(method),
		strings.TrimSpace(path),
		strings.TrimSpace(file),
	}, "\x00"))
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (step CodeFlowStep) LineString() string {
	if step.Line == 0 {
		return ""
	}
	return strconv.Itoa(step.Line)
}
