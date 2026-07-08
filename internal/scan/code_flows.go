package scan

import (
	"fmt"
	"sort"
	"strings"
)

func mergeCallGraphs(base CallGraphRecord, extra CallGraphRecord) CallGraphRecord {
	seen := map[string]bool{}
	var edges []CallGraphEdgeRecord
	for _, edge := range append(base.Edges, extra.Edges...) {
		key := edge.From.File + ":" + edge.From.Method + "->" + edge.To.File + ":" + edge.To.Method + fmt.Sprintf(":%d", edge.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, edge)
	}
	sortCallGraphEdges(edges)
	return CallGraphRecord{Edges: edges}
}

func buildGenericCallGraph(code CodeIntelligenceRecord) CallGraphRecord {
	functions := indexedCodeFunctions(code.Functions)
	var edges []CallGraphEdgeRecord
	for _, function := range code.Functions {
		for _, call := range function.Calls {
			target, ok := resolveCodeCall(function, call, functions)
			if !ok {
				continue
			}
			confidence := codeCallConfidence(function, target)
			if confidence == "INFERRED" {
				continue
			}
			edge := CallGraphEdgeRecord{
				ID: stableID("call", function.File, function.Name, target.File, target.Name, fmt.Sprint(call.Line)),
				From: MethodRefRecord{
					Owner:  function.Owner,
					Method: function.Name,
					File:   function.File,
					Line:   function.Line,
				},
				To: MethodRefRecord{
					Owner:  target.Owner,
					Method: target.Name,
					File:   target.File,
					Line:   target.Line,
				},
				Type:            "calls",
				Line:            call.Line,
				SourceFile:      function.File,
				Confidence:      confidence,
				ConfidenceScore: codeCallConfidenceScore(confidence),
				Reason:          codeCallReason(function.Language, call.Kind),
			}
			edges = append(edges, edge)
		}
	}
	sortCallGraphEdges(edges)
	return CallGraphRecord{Edges: edges}
}

func buildCodeRoutes(code CodeIntelligenceRecord, spring SpringIndex) []CodeRouteRecord {
	routes := make([]CodeRouteRecord, 0, len(code.Routes)+len(spring.Endpoints))
	routes = append(routes, code.Routes...)
	for _, endpoint := range spring.Endpoints {
		app := codeFileApp(endpoint.File)
		path := normalizeCodeRoutePath(endpoint.Path)
		routes = append(routes, CodeRouteRecord{
			Language:        "java",
			Framework:       "Spring",
			Kind:            "backend",
			App:             app,
			Package:         codeFilePackage(endpoint.File),
			RouteID:         codeRouteID(app, path),
			HTTPMethod:      endpoint.HTTPMethod,
			Path:            path,
			Handler:         endpoint.Controller + "." + endpoint.Method,
			File:            endpoint.File,
			Line:            endpoint.Line,
			Confidence:      "EXTRACTED",
			ConfidenceScore: 1.0,
			Reason:          "spring-mapping",
		})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Language != routes[j].Language {
			return routes[i].Language < routes[j].Language
		}
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		if routes[i].File != routes[j].File {
			return routes[i].File < routes[j].File
		}
		return routes[i].Line < routes[j].Line
	})
	return routes
}

func buildCodeFlows(code CodeIntelligenceRecord, spring SpringIndex, springFlows []SpringEndpointFlowRecord, graph CallGraphRecord) []CodeFlowRecord {
	routes := buildCodeRoutes(code, spring)
	edgesByMethod := map[string][]CallGraphEdgeRecord{}
	for _, edge := range graph.Edges {
		key := codeMethodKey(edge.From.File, edge.From.Owner, edge.From.Method)
		edgesByMethod[key] = append(edgesByMethod[key], edge)
	}
	functions := indexedCodeFunctions(code.Functions)
	flows := make([]CodeFlowRecord, 0, len(routes))
	for _, route := range routes {
		flow := CodeFlowRecord{
			Language:   route.Language,
			Framework:  route.Framework,
			Kind:       route.Kind,
			App:        route.App,
			Package:    route.Package,
			RouteID:    route.RouteID,
			HTTPMethod: route.HTTPMethod,
			Path:       route.Path,
			Handler:    route.Handler,
			File:       route.File,
			Line:       route.Line,
			Steps: []CodeFlowStep{{
				Name:       route.Handler,
				Kind:       "route_handler",
				Language:   route.Language,
				File:       route.File,
				Line:       route.Line,
				Confidence: route.Confidence,
				Reason:     route.Reason,
			}},
		}
		if route.Language == "java" {
			flow.Steps = appendJavaFlowSteps(flow.Steps, route, springFlows)
		} else if handler, ok := resolveRouteHandler(route, functions); ok {
			flow.Steps[0] = codeFunctionStep(handler, "route_handler", route.Confidence)
			flow.Steps = append(flow.Steps, walkCodeFlow(handler, edgesByMethod, 0, map[string]bool{})...)
		}
		flows = append(flows, flow)
	}
	sort.Slice(flows, func(i, j int) bool {
		if flows[i].Language != flows[j].Language {
			return flows[i].Language < flows[j].Language
		}
		if flows[i].Path != flows[j].Path {
			return flows[i].Path < flows[j].Path
		}
		return flows[i].Handler < flows[j].Handler
	})
	return flows
}

func buildGenericTestMap(code CodeIntelligenceRecord) []TestMapRecord {
	functions := indexedCodeFunctions(code.Functions)
	records := make([]TestMapRecord, 0)
	for _, function := range code.Functions {
		if function.Kind != "test" {
			continue
		}
		for _, call := range function.Calls {
			target, ok := resolveCodeCall(function, call, functions)
			if !ok || target.Kind == "test" {
				continue
			}
			confidence := codeTestMapConfidence(function, target)
			if confidence == "INFERRED" {
				continue
			}
			records = append(records, TestMapRecord{
				TestFile:        function.File,
				TestClass:       function.Owner,
				TestMethod:      function.Name,
				TargetFile:      target.File,
				TargetClass:     target.Owner,
				TargetMethod:    target.Name,
				Type:            "method",
				Line:            call.Line,
				Confidence:      confidence,
				ConfidenceScore: codeTestMapConfidenceScore(confidence),
				Reason:          function.Language + " test calls resolved production symbol",
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].TestFile != records[j].TestFile {
			return records[i].TestFile < records[j].TestFile
		}
		if records[i].TestMethod != records[j].TestMethod {
			return records[i].TestMethod < records[j].TestMethod
		}
		return records[i].TargetMethod < records[j].TargetMethod
	})
	return records
}

func codeCallConfidence(from, target CodeFunctionRecord) string {
	if from.File == target.File {
		return "EXTRACTED"
	}
	if codeFileApp(from.File) != "" && codeFileApp(from.File) == codeFileApp(target.File) {
		return "EXTRACTED"
	}
	if codeFilePackage(from.File) != "" && codeFilePackage(from.File) == codeFilePackage(target.File) {
		return "EXTRACTED"
	}
	return "INFERRED"
}

func codeCallConfidenceScore(confidence string) float64 {
	if confidence == "EXTRACTED" {
		return 0.9
	}
	return 0.72
}

func codeTestMapConfidence(test, target CodeFunctionRecord) string {
	if codeCallConfidence(test, target) == "EXTRACTED" {
		return "MATCHED"
	}
	return "INFERRED"
}

func codeTestMapConfidenceScore(confidence string) float64 {
	if confidence == "MATCHED" {
		return 0.9
	}
	return 0.7
}

type codeFunctionIndex struct {
	byName      map[string][]CodeFunctionRecord
	byOwnerName map[string]CodeFunctionRecord
}

func indexedCodeFunctions(functions []CodeFunctionRecord) codeFunctionIndex {
	index := codeFunctionIndex{byName: map[string][]CodeFunctionRecord{}, byOwnerName: map[string]CodeFunctionRecord{}}
	for _, function := range functions {
		index.byName[function.Name] = append(index.byName[function.Name], function)
		if function.Owner != "" {
			index.byOwnerName[function.Owner+"."+function.Name] = function
		}
	}
	return index
}

func resolveCodeCall(from CodeFunctionRecord, call CodeCallRecord, index codeFunctionIndex) (CodeFunctionRecord, bool) {
	if isLowValueCallTarget(call.Method) {
		return CodeFunctionRecord{}, false
	}
	if call.Owner != "" {
		if target, ok := index.byOwnerName[call.Owner+"."+call.Method]; ok {
			return target, true
		}
	}
	if call.Receiver != "" {
		receiver := strings.TrimPrefix(call.Receiver, "$")
		if target, ok := index.byOwnerName[receiver+"."+call.Method]; ok {
			return target, true
		}
	}
	candidates := index.byName[call.Method]
	if len(candidates) == 0 {
		return CodeFunctionRecord{}, false
	}
	if target, ok := bestCodeCandidate(from, candidates); ok {
		return target, true
	}
	return candidates[0], candidates[0].Name != from.Name || candidates[0].File != from.File
}

func resolveRouteHandler(route CodeRouteRecord, index codeFunctionIndex) (CodeFunctionRecord, bool) {
	handler := strings.TrimSpace(route.Handler)
	if handler == "" {
		return CodeFunctionRecord{}, false
	}
	if target, ok := index.byOwnerName[handler]; ok {
		return target, true
	}
	if strings.Contains(handler, ".") {
		parts := strings.Split(handler, ".")
		handler = parts[len(parts)-1]
	}
	candidates := index.byName[handler]
	return bestRouteCandidate(route, candidates)
}

func bestCodeCandidate(from CodeFunctionRecord, candidates []CodeFunctionRecord) (CodeFunctionRecord, bool) {
	for _, candidate := range candidates {
		if candidate.Language == from.Language && candidate.File == from.File && candidate.Name != from.Name {
			return candidate, true
		}
	}
	fromApp := codeFileApp(from.File)
	for _, candidate := range candidates {
		if candidate.Language == from.Language && codeFileApp(candidate.File) == fromApp && candidate.Name != from.Name {
			return candidate, true
		}
	}
	fromPackage := codeFilePackage(from.File)
	for _, candidate := range candidates {
		if candidate.Language == from.Language && fromPackage != "" && codeFilePackage(candidate.File) == fromPackage && candidate.Name != from.Name {
			return candidate, true
		}
	}
	for _, candidate := range candidates {
		if candidate.Language == from.Language && candidate.Name != from.Name {
			return candidate, true
		}
	}
	if len(candidates) == 0 {
		return CodeFunctionRecord{}, false
	}
	return candidates[0], candidates[0].Name != from.Name || candidates[0].File != from.File
}

func bestRouteCandidate(route CodeRouteRecord, candidates []CodeFunctionRecord) (CodeFunctionRecord, bool) {
	for _, candidate := range candidates {
		if candidate.Language == route.Language && candidate.File == route.File {
			return candidate, true
		}
	}
	for _, candidate := range candidates {
		if candidate.Language == route.Language && route.App != "" && codeFileApp(candidate.File) == route.App {
			return candidate, true
		}
	}
	for _, candidate := range candidates {
		if candidate.Language == route.Language && route.Package != "" && codeFilePackage(candidate.File) == route.Package {
			return candidate, true
		}
	}
	for _, candidate := range candidates {
		if candidate.Language == route.Language {
			return candidate, true
		}
	}
	if len(candidates) > 0 {
		return candidates[0], true
	}
	return CodeFunctionRecord{}, false
}

func walkCodeFlow(function CodeFunctionRecord, edgesByMethod map[string][]CallGraphEdgeRecord, depth int, visited map[string]bool) []CodeFlowStep {
	if depth >= 4 {
		return nil
	}
	key := codeMethodKey(function.File, function.Owner, function.Name)
	if visited[key] {
		return nil
	}
	visited[key] = true
	var steps []CodeFlowStep
	for _, edge := range edgesByMethod[key] {
		step := CodeFlowStep{
			Name:       edge.To.Method,
			Owner:      edge.To.Owner,
			Kind:       codeFlowStepKind(edge),
			File:       edge.To.File,
			Line:       edge.To.Line,
			Confidence: edge.Confidence,
			Reason:     edge.Reason,
		}
		steps = append(steps, step)
		next := CodeFunctionRecord{Name: edge.To.Method, Owner: edge.To.Owner, File: edge.To.File}
		steps = append(steps, walkCodeFlow(next, edgesByMethod, depth+1, visited)...)
	}
	return steps
}

func codeCallReason(language, kind string) string {
	switch kind {
	case "effect":
		return language + " effect call match"
	case "event_handler":
		return language + " event handler call match"
	default:
		return language + " static call match"
	}
}

func codeFlowStepKind(edge CallGraphEdgeRecord) string {
	switch {
	case strings.Contains(edge.Reason, "effect call"):
		return "effect_call"
	case strings.Contains(edge.Reason, "event handler call"):
		return "event_handler"
	default:
		return "call"
	}
}

func appendJavaFlowSteps(steps []CodeFlowStep, route CodeRouteRecord, springFlows []SpringEndpointFlowRecord) []CodeFlowStep {
	for _, flow := range springFlows {
		if flow.HTTPMethod != route.HTTPMethod || flow.Path != route.Path {
			continue
		}
		for _, step := range flow.Steps {
			steps = append(steps, CodeFlowStep{
				Name:       step.Method,
				Owner:      step.Owner,
				Kind:       step.Kind,
				Language:   "java",
				File:       step.File,
				Line:       step.Line,
				Confidence: step.Confidence,
			})
		}
		return steps
	}
	return steps
}

func codeFunctionStep(function CodeFunctionRecord, kind, confidence string) CodeFlowStep {
	name := function.Name
	if function.Owner != "" {
		name = function.Owner + "." + function.Name
	}
	return CodeFlowStep{Name: name, Owner: function.Owner, Kind: kind, Language: function.Language, File: function.File, Line: function.Line, Confidence: confidence}
}

func codeMethodKey(file, owner, method string) string {
	return file + "\x00" + owner + "\x00" + method
}

func sortCallGraphEdges(edges []CallGraphEdgeRecord) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].SourceFile != edges[j].SourceFile {
			return edges[i].SourceFile < edges[j].SourceFile
		}
		if edges[i].From.Method != edges[j].From.Method {
			return edges[i].From.Method < edges[j].From.Method
		}
		if edges[i].To.Method != edges[j].To.Method {
			return edges[i].To.Method < edges[j].To.Method
		}
		return edges[i].Line < edges[j].Line
	})
}
