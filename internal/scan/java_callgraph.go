package scan

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func buildJavaCallGraph(sources []JavaSourceRecord) CallGraphRecord {
	methods := javaMethodsByOwner(sources)
	fields := javaFieldTypesByOwner(sources)
	types := javaTypesByName(sources)
	var edges []CallGraphEdgeRecord

	for _, source := range sources {
		for _, method := range source.Methods {
			from := MethodRefRecord{Owner: method.Owner, Method: method.Name, File: method.File, Line: method.Line}
			for _, call := range method.Calls {
				toOwner := resolveCallOwner(call, method.Owner, fields)
				if toOwner == "" {
					continue
				}
				target := MethodRefRecord{Owner: toOwner, Method: call.Method}
				confidence := "INFERRED"
				score := 0.72
				reason := "receiver type inferred"
				if candidates := methods[toOwner]; candidates != nil {
					if exact, ok := candidates[call.Method]; ok {
						target.File = exact.File
						target.Line = exact.Line
						confidence = "EXTRACTED"
						score = 1.0
						reason = "method declaration matched"
					}
				}
				if target.File == "" {
					if typ, ok := types[toOwner]; ok {
						target.File = typ.File
						target.Line = typ.Line
						confidence = "EXTRACTED"
						score = 0.85
						reason = "type declaration matched"
					}
				}
				if target.File == "" {
					continue
				}
				edges = append(edges, CallGraphEdgeRecord{
					ID:              stableID("call", from.Owner, from.Method, target.Owner, target.Method, fmt.Sprint(call.Line)),
					From:            from,
					To:              target,
					Type:            "calls",
					Line:            call.Line,
					SourceFile:      method.File,
					Confidence:      confidence,
					ConfidenceScore: score,
					Reason:          reason,
				})
			}
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From.File != edges[j].From.File {
			return edges[i].From.File < edges[j].From.File
		}
		if edges[i].From.Line != edges[j].From.Line {
			return edges[i].From.Line < edges[j].From.Line
		}
		if edges[i].To.Owner != edges[j].To.Owner {
			return edges[i].To.Owner < edges[j].To.Owner
		}
		return edges[i].To.Method < edges[j].To.Method
	})
	return CallGraphRecord{Edges: dedupeCallGraphEdges(edges)}
}

func buildCallRelations(graph CallGraphRecord) []RelationRecord {
	seen := map[string]bool{}
	var relations []RelationRecord
	for _, edge := range graph.Edges {
		if edge.From.File == "" || edge.To.File == "" || edge.From.File == edge.To.File {
			continue
		}
		key := edge.From.File + "\x00" + edge.To.File + "\x00calls"
		if seen[key] {
			continue
		}
		seen[key] = true
		relations = append(relations, RelationRecord{From: edge.From.File, To: edge.To.File, Type: "calls", Line: edge.Line})
	}
	return relations
}

func buildEndpointFlows(index SpringIndex, graph CallGraphRecord) []SpringEndpointFlowRecord {
	componentKinds := map[string]string{}
	for _, component := range index.Components {
		componentKinds[component.Name] = component.Kind
	}
	edgesByMethod := map[string][]CallGraphEdgeRecord{}
	for _, edge := range graph.Edges {
		key := methodKey(edge.From.Owner, edge.From.Method)
		edgesByMethod[key] = append(edgesByMethod[key], edge)
	}

	var flows []SpringEndpointFlowRecord
	for _, endpoint := range index.Endpoints {
		flow := SpringEndpointFlowRecord{
			HTTPMethod: endpoint.HTTPMethod,
			Path:       endpoint.Path,
			Controller: endpoint.Controller,
			Method:     endpoint.Method,
			File:       endpoint.File,
			Line:       endpoint.Line,
		}
		start := SpringEndpointFlowStep{Owner: endpoint.Controller, Method: endpoint.Method, Kind: componentKinds[endpoint.Controller], File: endpoint.File, Line: endpoint.Line, Confidence: "EXTRACTED"}
		flow.Steps = append(flow.Steps, start)
		visited := map[string]bool{methodKey(endpoint.Controller, endpoint.Method): true}
		queue := []SpringEndpointFlowStep{start}
		for depth := 0; depth < 4 && len(queue) > 0; depth++ {
			current := queue[0]
			queue = queue[1:]
			for _, edge := range edgesByMethod[methodKey(current.Owner, current.Method)] {
				key := methodKey(edge.To.Owner, edge.To.Method)
				if visited[key] {
					continue
				}
				visited[key] = true
				step := SpringEndpointFlowStep{Owner: edge.To.Owner, Method: edge.To.Method, Kind: componentKinds[edge.To.Owner], File: edge.To.File, Line: edge.To.Line, Confidence: edge.Confidence}
				flow.Steps = append(flow.Steps, step)
				queue = append(queue, step)
			}
		}
		flows = append(flows, flow)
	}
	sort.Slice(flows, func(i, j int) bool {
		if flows[i].Path != flows[j].Path {
			return flows[i].Path < flows[j].Path
		}
		return flows[i].HTTPMethod < flows[j].HTTPMethod
	})
	return flows
}

func buildJavaTestMap(sources []JavaSourceRecord, endpoints []SpringEndpointRecord) []TestMapRecord {
	methods := javaMethodsByOwner(sources)
	endpointByRequest := endpointMatchers(endpoints)
	helperHTTPRequests := javaHTTPRequestsByOwnerMethod(sources)
	var records []TestMapRecord
	for _, source := range sources {
		if !strings.Contains(source.File, "src/test/") && !strings.Contains(source.File, "_test") {
			continue
		}
		for _, method := range source.Methods {
			if !isJavaTestMethod(method) {
				continue
			}
			for _, call := range method.Calls {
				toOwner := call.TargetOwner
				if toOwner == "" && call.Receiver != "" {
					toOwner = strings.TrimSuffix(call.Receiver, "Test")
				}
				if toOwner == "" {
					continue
				}
				if candidates := methods[toOwner]; candidates != nil {
					if target, ok := candidates[call.Method]; ok {
						records = append(records, TestMapRecord{
							TestFile: method.File, TestClass: method.Owner, TestMethod: method.Name,
							TargetFile: target.File, TargetClass: target.Owner, TargetMethod: target.Name,
							Type: "method", Line: call.Line, Confidence: "EXTRACTED", ConfidenceScore: 1.0, Reason: "test method calls production method",
						})
					}
				}
			}
			for _, request := range javaTestMethodHTTPRequests(method, helperHTTPRequests) {
				if endpoint, ok := endpointByRequest.match(request.HTTPMethod, request.Path); ok {
					testCase, status := classifyEndpointTestCase(method.Name)
					records = append(records, TestMapRecord{
						TestFile: method.File, TestClass: method.Owner, TestMethod: method.Name,
						TargetFile: endpoint.File, TargetClass: endpoint.Controller, TargetMethod: endpoint.Method,
						HTTPMethod: endpoint.HTTPMethod, Path: endpoint.Path,
						Type: "endpoint", TestCase: testCase, StatusExpectation: status, Line: request.Line, Confidence: "MATCHED", ConfidenceScore: 0.9, Reason: "extracted test HTTP request matched endpoint pattern",
					})
				}
			}
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].TestFile != records[j].TestFile {
			return records[i].TestFile < records[j].TestFile
		}
		if records[i].TestMethod != records[j].TestMethod {
			return records[i].TestMethod < records[j].TestMethod
		}
		return records[i].TargetClass < records[j].TargetClass
	})
	return records
}

func classifyEndpointTestCase(name string) (string, string) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "noauth") || strings.Contains(lower, "no_auth"):
		return "auth_error", "401"
	case strings.Contains(lower, "forbidden"):
		return "permission_error", "403"
	case strings.Contains(lower, "badrequest") || strings.Contains(lower, "bad_request"):
		return "validation_error", "400"
	case strings.Contains(lower, "notfound") || strings.Contains(lower, "not_found"):
		return "not_found", "404"
	case strings.Contains(lower, "error") || strings.Contains(lower, "exception"):
		return "error", ""
	case strings.Contains(lower, "okay") || strings.Contains(lower, "success") || strings.Contains(lower, "isok"):
		return "success", "2xx"
	default:
		return "unspecified", ""
	}
}

func javaHTTPRequestsByOwnerMethod(sources []JavaSourceRecord) map[string]map[string][]JavaHTTPCallRecord {
	requests := map[string]map[string][]JavaHTTPCallRecord{}
	for _, source := range sources {
		for _, method := range source.Methods {
			if len(method.HTTPRequests) == 0 {
				continue
			}
			if requests[method.Owner] == nil {
				requests[method.Owner] = map[string][]JavaHTTPCallRecord{}
			}
			requests[method.Owner][method.Name] = append([]JavaHTTPCallRecord(nil), method.HTTPRequests...)
		}
	}
	return requests
}

func javaTestMethodHTTPRequests(method JavaMethodRecord, helperRequests map[string]map[string][]JavaHTTPCallRecord) []JavaHTTPCallRecord {
	requests := append([]JavaHTTPCallRecord(nil), method.HTTPRequests...)
	for _, call := range method.Calls {
		if call.Receiver != "" || call.TargetOwner != "" {
			continue
		}
		if call.Method == method.Name {
			continue
		}
		if helpers := helperRequests[method.Owner]; helpers != nil {
			requests = append(requests, specializeJavaHelperHTTPRequests(helpers[call.Method], call.Arguments)...)
			continue
		}
		if isGenericHTTPHelperName(call.Method) {
			continue
		}
		if helpers := uniqueJavaHTTPRequestsByMethodName(helperRequests); helpers != nil {
			requests = append(requests, specializeJavaHelperHTTPRequests(helpers[call.Method], call.Arguments)...)
		}
	}
	return requests
}

func isGenericHTTPHelperName(name string) bool {
	switch strings.ToLower(name) {
	case "get", "post", "put", "delete", "patch":
		return true
	default:
		return false
	}
}

func uniqueJavaHTTPRequestsByMethodName(helperRequests map[string]map[string][]JavaHTTPCallRecord) map[string][]JavaHTTPCallRecord {
	counts := map[string]int{}
	byName := map[string][]JavaHTTPCallRecord{}
	for _, methods := range helperRequests {
		for name, requests := range methods {
			counts[name]++
			byName[name] = requests
		}
	}
	for name, count := range counts {
		if count != 1 {
			delete(byName, name)
		}
	}
	return byName
}

func specializeJavaHelperHTTPRequests(requests []JavaHTTPCallRecord, args []string) []JavaHTTPCallRecord {
	if len(requests) == 0 {
		return nil
	}
	suffix := javaLiteralPathArgument(args)
	if suffix == "" {
		return requests
	}
	specialized := append([]JavaHTTPCallRecord(nil), requests...)
	for index := range specialized {
		if !strings.HasSuffix(specialized[index].Path, suffix) {
			specialized[index].Path += suffix
		}
	}
	return specialized
}

func javaLiteralPathArgument(args []string) string {
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if len(arg) >= 2 && strings.HasPrefix(arg, "\"/") && strings.HasSuffix(arg, "\"") {
			return strings.TrimSuffix(strings.TrimPrefix(arg, "\""), "\"")
		}
	}
	return ""
}

func javaMethodsByOwner(sources []JavaSourceRecord) map[string]map[string]JavaMethodRecord {
	methods := map[string]map[string]JavaMethodRecord{}
	for _, source := range sources {
		for _, method := range source.Methods {
			if methods[method.Owner] == nil {
				methods[method.Owner] = map[string]JavaMethodRecord{}
			}
			methods[method.Owner][method.Name] = method
		}
	}
	return methods
}

func javaFieldTypesByOwner(sources []JavaSourceRecord) map[string]map[string]string {
	fields := map[string]map[string]string{}
	for _, source := range sources {
		for _, field := range source.Fields {
			if fields[field.Owner] == nil {
				fields[field.Owner] = map[string]string{}
			}
			fields[field.Owner][field.Name] = field.Type
		}
	}
	return fields
}

func javaTypesByName(sources []JavaSourceRecord) map[string]JavaTypeRecord {
	types := map[string]JavaTypeRecord{}
	for _, source := range sources {
		for _, typ := range source.Types {
			types[typ.Name] = typ
		}
	}
	return types
}

func resolveCallOwner(call JavaCallRecord, owner string, fields map[string]map[string]string) string {
	if call.TargetOwner != "" {
		return call.TargetOwner
	}
	if call.Receiver == "this" {
		return owner
	}
	if call.Receiver != "" {
		if fieldType := fields[owner][call.Receiver]; fieldType != "" {
			return fieldType
		}
		if startsUpper(call.Receiver) {
			return call.Receiver
		}
	}
	return ""
}

func isJavaTestMethod(method JavaMethodRecord) bool {
	return hasAnnotation(method.Annotations, "Test") || strings.HasPrefix(method.Name, "test")
}

func methodKey(owner, method string) string {
	return owner + "." + method
}

func startsUpper(value string) bool {
	if value == "" {
		return false
	}
	first := value[0]
	return first >= 'A' && first <= 'Z'
}

func dedupeCallGraphEdges(edges []CallGraphEdgeRecord) []CallGraphEdgeRecord {
	seen := map[string]bool{}
	var result []CallGraphEdgeRecord
	for _, edge := range edges {
		key := edge.From.Owner + "\x00" + edge.From.Method + "\x00" + edge.To.Owner + "\x00" + edge.To.Method
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, edge)
	}
	return result
}

type endpointMatcherSet []SpringEndpointRecord

func endpointMatchers(endpoints []SpringEndpointRecord) endpointMatcherSet {
	return endpointMatcherSet(endpoints)
}

func (set endpointMatcherSet) match(method, path string) (SpringEndpointRecord, bool) {
	for _, endpoint := range set {
		if endpoint.HTTPMethod != method {
			continue
		}
		if springEndpointPathMatches(endpoint.Path, path) {
			return endpoint, true
		}
	}
	return SpringEndpointRecord{}, false
}

func springEndpointPathMatches(pattern, path string) bool {
	for _, patternVariant := range knownBasePrefixPathVariants(pattern) {
		for _, pathVariant := range knownBasePrefixPathVariants(path) {
			if springEndpointPathPatternMatches(patternVariant, pathVariant) {
				return true
			}
		}
	}
	return false
}

func springEndpointPathPatternMatches(pattern, path string) bool {
	quoted := regexp.QuoteMeta(pattern)
	expr := regexp.MustCompile(`\\\{[^/]+\\\}`).ReplaceAllString(quoted, `[^/]+`)
	expr = "^" + expr + "$"
	return regexp.MustCompile(expr).MatchString(path)
}
