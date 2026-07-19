package scan

import (
	"fmt"
	"sort"
	"strings"
)

type javaPathIndex struct {
	getters map[string][]javaIndexedPathExpression
}

type javaIndexedPathExpression struct {
	expression string
	constants  map[string]string
}

func buildJavaAPIContracts(sources []JavaSourceRecord) []APIContractRecord {
	paths := buildJavaPathIndex(sources)
	var records []APIContractRecord
	for _, source := range sources {
		records = append(records, javaDeclarativeAPIContracts(source)...)
		records = append(records, javaImperativeAPIContracts(source, paths)...)
	}
	sortAPIContracts(records)
	return dedupeJavaAPIContracts(records)
}

func javaDeclarativeAPIContracts(source JavaSourceRecord) []APIContractRecord {
	declarativeImports := javaDeclarativeClientImports(source.Imports)
	if len(declarativeImports) == 0 {
		return nil
	}
	var records []APIContractRecord
	for _, javaType := range source.Types {
		clientAnnotation, ok := javaDeclarativeClientAnnotation(javaType.Annotations, declarativeImports)
		if !ok {
			continue
		}
		basePath := javaAnnotationPath(clientAnnotation, source.Constants, "path", "url", "value")
		serviceCandidate := ""
		if clientAnnotation.Name == "FeignClient" {
			serviceCandidate = javaAnnotationAttribute(clientAnnotation, "name", "value")
		}
		for _, method := range source.Methods {
			if method.Owner != javaType.Name {
				continue
			}
			mapping, httpMethod, ok := javaDeclarativeMethodMapping(method.Annotations)
			if !ok {
				continue
			}
			methodPath := javaAnnotationPath(mapping, source.Constants, "path", "url", "value")
			reason := fmt.Sprintf("spring %s declarative mapping", clientAnnotation.Name)
			if javaMethodIsRetryable(source, method) {
				reason += "; retryable method"
			}
			records = append(records, APIContractRecord{
				Language:         "java",
				Package:          source.Package,
				HTTPMethod:       httpMethod,
				Path:             javaJoinAPIPaths(basePath, methodPath),
				Auth:             javaClientAuthentication(source, method),
				ServiceCandidate: serviceCandidate,
				Caller:           method.Owner + "." + method.Name,
				File:             source.File,
				Line:             mapping.Line,
				Confidence:       "EXACT",
				ConfidenceScore:  1,
				Reason:           reason,
			})
		}
	}
	return records
}

func javaImperativeAPIContracts(source JavaSourceRecord, paths javaPathIndex) []APIContractRecord {
	var records []APIContractRecord
	for _, method := range source.Methods {
		for _, request := range method.HTTPRequests {
			if !isBoundJavaSpringClient(request) {
				continue
			}
			path, resolution := resolveJavaContractPath(request, source, method, paths)
			record := APIContractRecord{
				Language:   "java",
				Package:    source.Package,
				HTTPMethod: request.HTTPMethod,
				Path:       path,
				RawPath:    strings.TrimSpace(request.PathExpression),
				Auth:       javaClientAuthentication(source, method),
				Caller:     method.Owner + "." + method.Name,
				File:       source.File,
				Line:       request.Line,
			}
			switch resolution {
			case "getter":
				record.Confidence = "RESOLVED"
				record.ConfidenceScore = 0.9
				record.Reason = fmt.Sprintf("spring %s receiver with statically resolved path getter", request.ClientKind)
			case "resolved":
				record.Confidence = "RESOLVED"
				record.ConfidenceScore = 0.9
				record.Reason = fmt.Sprintf("spring %s receiver with statically resolved path expression", request.ClientKind)
			case "exact":
				record.Confidence = "EXACT"
				record.ConfidenceScore = 1
				record.Reason = fmt.Sprintf("spring %s receiver with statically resolved path", request.ClientKind)
			default:
				record.UnsafeDynamic = true
				record.Confidence = "PARTIAL"
				record.ConfidenceScore = 0.5
				record.Reason = fmt.Sprintf("spring %s receiver with unresolved dynamic path", request.ClientKind)
			}
			if javaMethodIsRetryable(source, method) {
				record.Reason += "; retryable method"
			}
			records = append(records, record)
		}
	}
	return records
}

func buildJavaPathIndex(sources []JavaSourceRecord) javaPathIndex {
	index := javaPathIndex{getters: map[string][]javaIndexedPathExpression{}}
	for _, source := range sources {
		for _, method := range source.Methods {
			if len(method.Parameters) != 0 || strings.TrimSpace(method.ReturnExpression) == "" {
				continue
			}
			index.getters[method.Name] = append(index.getters[method.Name], javaIndexedPathExpression{
				expression: strings.TrimSpace(method.ReturnExpression),
				constants:  source.Constants,
			})
		}
	}
	return index
}

func javaClientAuthentication(source JavaSourceRecord, method JavaMethodRecord) []AuthRecord {
	hasBasicImport := javaHasImport(source.Imports, "org.springframework.http.client.support.BasicAuthenticationInterceptor")
	if hasBasicImport {
		for _, candidate := range source.Methods {
			for _, call := range candidate.Calls {
				if shortJavaName(call.TargetOwner) == "BasicAuthenticationInterceptor" {
					return []AuthRecord{{Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED"}}
				}
			}
		}
	}
	for _, call := range method.Calls {
		if call.Method == "defaultHeader" && javaCallSetsAuthorizationHeader(call) && javaReceiverIsBoundSpringClient(source, method, call.Receiver) {
			return []AuthRecord{{Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED"}}
		}
	}
	return nil
}

func javaCallSetsAuthorizationHeader(call JavaCallRecord) bool {
	if len(call.Arguments) == 0 {
		return false
	}
	header := strings.Trim(strings.TrimSpace(call.Arguments[0]), `"'`)
	return strings.EqualFold(header, "Authorization")
}

func sortAPIContracts(records []APIContractRecord) {
	sort.Slice(records, func(i, j int) bool {
		left, right := records[i], records[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.HTTPMethod != right.HTTPMethod {
			return left.HTTPMethod < right.HTTPMethod
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Caller != right.Caller {
			return left.Caller < right.Caller
		}
		return left.Reason < right.Reason
	})
}

func javaDeclarativeClientImports(imports []JavaImportRecord) map[string]bool {
	result := map[string]bool{}
	for _, record := range imports {
		switch record.Name {
		case "org.springframework.cloud.openfeign.FeignClient":
			result["FeignClient"] = true
		case "org.springframework.web.service.annotation.HttpExchange":
			result["HttpExchange"] = true
		}
	}
	return result
}

func javaDeclarativeClientAnnotation(annotations []JavaAnnotationRecord, imports map[string]bool) (JavaAnnotationRecord, bool) {
	for _, annotation := range annotations {
		if imports[annotation.Name] {
			return annotation, true
		}
	}
	return JavaAnnotationRecord{}, false
}

func javaDeclarativeMethodMapping(annotations []JavaAnnotationRecord) (JavaAnnotationRecord, string, bool) {
	for _, annotation := range annotations {
		switch annotation.Name {
		case "GetMapping", "GetExchange":
			return annotation, "GET", true
		case "PostMapping", "PostExchange":
			return annotation, "POST", true
		case "PutMapping", "PutExchange":
			return annotation, "PUT", true
		case "DeleteMapping", "DeleteExchange":
			return annotation, "DELETE", true
		case "PatchMapping", "PatchExchange":
			return annotation, "PATCH", true
		}
	}
	return JavaAnnotationRecord{}, "", false
}

func javaAnnotationPath(annotation JavaAnnotationRecord, constants map[string]string, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(annotation.Attributes[key])
		if value == "" {
			continue
		}
		if resolved, ok := constants[value]; ok {
			return resolved
		}
		return value
	}
	return ""
}

func javaAnnotationAttribute(annotation JavaAnnotationRecord, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(annotation.Attributes[key]); value != "" {
			return value
		}
	}
	return ""
}

func javaJoinAPIPaths(basePath, methodPath string) string {
	joined := strings.TrimSuffix(strings.TrimSpace(basePath), "/") + "/" + strings.TrimPrefix(strings.TrimSpace(methodPath), "/")
	return normalizeAPIPath(joined)
}

func isBoundJavaSpringClient(request JavaHTTPCallRecord) bool {
	if strings.TrimSpace(request.Receiver) == "" {
		return false
	}
	switch request.ClientKind {
	case "RestClient", "WebClient", "RestTemplate":
		return true
	default:
		return false
	}
}

func resolveJavaContractPath(request JavaHTTPCallRecord, source JavaSourceRecord, method JavaMethodRecord, paths javaPathIndex) (string, string) {
	expression := strings.TrimSpace(request.PathExpression)
	if expression == "" && request.Path != "" {
		return normalizeAPIPath(request.Path), "exact"
	}
	if value, ok := method.StringVars[expression]; ok {
		return normalizeAPIPath(value), "exact"
	}
	if value, ok := source.Constants[expression]; ok {
		return normalizeAPIPath(value), "exact"
	}
	if value, ok := method.StringExpressions[expression]; ok {
		if path, ok := javaResolvedPathExpression(value, source.Constants, method.StringExpressions, 0); ok {
			return normalizeAPIPath(path), "resolved"
		}
	}
	if getter := javaZeroArgumentGetterName(expression); getter != "" {
		if candidates := paths.getters[getter]; len(candidates) == 1 {
			if path, ok := javaResolvedPathExpression(candidates[0].expression, candidates[0].constants, nil, 0); ok {
				return normalizeAPIPath(path), "getter"
			}
		}
		return "", "partial"
	}
	if request.Path != "" {
		resolution := "exact"
		if strings.Contains(request.Path, "{dynamic}") {
			resolution = "resolved"
		}
		return normalizeAPIPath(request.Path), resolution
	}
	if path, ok := javaResolvedPathExpression(expression, source.Constants, method.StringExpressions, 0); ok {
		return normalizeAPIPath(path), "resolved"
	}
	return "", "partial"
}

func javaResolvedPathExpression(expression string, constants, locals map[string]string, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	expression = strings.TrimSpace(expression)
	if value, ok := constants[expression]; ok {
		return value, strings.HasPrefix(value, "/")
	}
	if value, ok := locals[expression]; ok && strings.TrimSpace(value) != expression {
		return javaResolvedPathExpression(value, constants, locals, depth+1)
	}
	if value, ok := javaQuotedStringValue(expression); ok {
		return value, strings.HasPrefix(value, "/")
	}
	parts := splitTopLevel(expression, '+')
	if len(parts) > 1 {
		var path strings.Builder
		for _, rawPart := range parts {
			part := strings.TrimSpace(rawPart)
			value, resolved := javaResolvedPathPart(part, constants, locals, depth+1)
			if !resolved {
				if path.Len() > 0 {
					appendDynamicPathSegment(&path)
				}
				continue
			}
			if path.Len() == 0 && !strings.HasPrefix(value, "/") {
				continue
			}
			path.WriteString(value)
		}
		value := strings.ReplaceAll(path.String(), "//", "/")
		return value, strings.HasPrefix(value, "/")
	}
	if path, ok := javaHTTPRequestPath(expression, nil); ok {
		return path, true
	}
	literals := javaStringLiteralRE.FindAllStringSubmatchIndex(expression, -1)
	for _, literal := range literals {
		if !strings.HasPrefix(expression[literal[2]:literal[3]], "/") {
			continue
		}
		return javaHTTPRequestPath(expression[literal[0]:], nil)
	}
	return "", false
}

func javaResolvedPathPart(part string, constants, locals map[string]string, depth int) (string, bool) {
	if value, ok := javaQuotedStringValue(part); ok {
		return value, true
	}
	if value, ok := constants[part]; ok {
		return value, true
	}
	if value, ok := locals[part]; ok && strings.TrimSpace(value) != part {
		return javaResolvedPathExpression(value, constants, locals, depth)
	}
	return "", false
}

func javaQuotedStringValue(expression string) (string, bool) {
	expression = strings.TrimSpace(expression)
	if len(expression) < 2 || expression[0] != '"' || expression[len(expression)-1] != '"' {
		return "", false
	}
	return expression[1 : len(expression)-1], true
}

func javaZeroArgumentGetterName(expression string) string {
	expression = strings.TrimSpace(expression)
	if !strings.HasSuffix(expression, "()") {
		return ""
	}
	prefix := strings.TrimSpace(strings.TrimSuffix(expression, "()"))
	if dot := strings.LastIndex(prefix, "."); dot >= 0 {
		prefix = prefix[dot+1:]
	}
	if !isJavaIdentifierPath(prefix) || strings.Contains(prefix, ".") {
		return ""
	}
	return prefix
}

func javaMethodIsRetryable(source JavaSourceRecord, method JavaMethodRecord) bool {
	if !javaHasImport(source.Imports, "org.springframework.retry.annotation.Retryable") {
		return false
	}
	for _, annotation := range method.Annotations {
		if annotation.Name == "Retryable" {
			return true
		}
	}
	return false
}

func javaHasImport(imports []JavaImportRecord, name string) bool {
	for _, record := range imports {
		if !record.Static && record.Name == name {
			return true
		}
	}
	return false
}

func javaReceiverIsBoundSpringClient(source JavaSourceRecord, method JavaMethodRecord, receiver string) bool {
	if dot := strings.LastIndex(receiver, "."); dot >= 0 {
		receiver = receiver[dot+1:]
	}
	typeName := method.LocalTypes[receiver]
	if typeName == "" {
		for _, field := range source.Fields {
			if field.Owner == method.Owner && field.Name == receiver {
				typeName = field.Type
				break
			}
		}
	}
	kind := shortJavaName(typeName)
	for _, record := range source.Imports {
		if javaSpringHTTPClientImport(record.Name) == kind {
			return true
		}
	}
	return false
}

func dedupeJavaAPIContracts(records []APIContractRecord) []APIContractRecord {
	if len(records) < 2 {
		return records
	}
	result := make([]APIContractRecord, 0, len(records))
	seen := map[string]bool{}
	for _, record := range records {
		key := fmt.Sprintf("%s\x00%d\x00%s\x00%s\x00%s\x00%s\x00%s", record.File, record.Line, record.HTTPMethod, record.Path, record.RawPath, record.Caller, record.Reason)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, record)
	}
	return result
}
