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
	baseURLs   map[string]bool
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
		basePath, baseRawPath, baseResolved := javaAnnotationPath(clientAnnotation, source.Constants, "path", "url", "value")
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
			methodPath, methodRawPath, methodResolved := javaAnnotationPath(mapping, source.Constants, "path", "url", "value")
			reason := fmt.Sprintf("spring %s declarative mapping", clientAnnotation.Name)
			if javaMethodIsRetryable(source, method) {
				reason += "; retryable method"
			}
			record := APIContractRecord{
				Language:         "java",
				Package:          source.Package,
				HTTPMethod:       httpMethod,
				Auth:             javaClientAuthentication(source, method),
				ServiceCandidate: serviceCandidate,
				Caller:           method.Owner + "." + method.Name,
				File:             source.File,
				Line:             mapping.Line,
				Reason:           reason,
			}
			if baseResolved && methodResolved {
				record.Path = javaJoinAPIPaths(basePath, methodPath)
				record.Confidence = "EXACT"
				record.ConfidenceScore = 1
			} else {
				record.RawPath = javaDeclarativeRawPath(baseRawPath, methodRawPath)
				record.UnsafeDynamic = true
				record.Confidence = "PARTIAL"
				record.ConfidenceScore = 0.5
				record.Reason += "; unresolved declarative path expression"
			}
			records = append(records, record)
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
			alternatives, resolution := resolveJavaContractPaths(request, source, method, paths)
			if len(alternatives) == 0 {
				alternatives = []string{""}
			}
			for _, alternative := range alternatives {
				path, query := "", ""
				var queryParams []QueryParamRecord
				unsafeDynamic := false
				if strings.TrimSpace(alternative) != "" {
					path, query, queryParams, unsafeDynamic = normalizeAPIPathDetails(alternative)
				}
				record := APIContractRecord{
					Language:    "java",
					Package:     source.Package,
					HTTPMethod:  request.HTTPMethod,
					Path:        path,
					RawPath:     strings.TrimSpace(request.PathExpression),
					Query:       query,
					QueryParams: queryParams,
					Auth:        javaClientAuthentication(source, method),
					Caller:      method.Owner + "." + method.Name,
					File:        source.File,
					Line:        request.Line,
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
				record.UnsafeDynamic = record.UnsafeDynamic || unsafeDynamic
				if javaMethodIsRetryable(source, method) {
					record.Reason += "; retryable method"
				}
				records = append(records, record)
			}
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
			key := javaGetterIndexKey(method.Owner, method.Name)
			index.getters[key] = append(index.getters[key], javaIndexedPathExpression{
				expression: strings.TrimSpace(method.ReturnExpression),
				constants:  source.Constants,
				baseURLs:   javaConfigurationBaseURLExpressions(source, method),
			})
		}
	}
	return index
}

func javaClientAuthentication(source JavaSourceRecord, method JavaMethodRecord) []AuthRecord {
	hasBasicImport := javaHasImport(source.Imports, "org.springframework.http.client.support.BasicAuthenticationInterceptor")
	if hasBasicImport {
		for _, candidate := range source.Methods {
			for _, typeName := range candidate.ConstructedTypes {
				if typeName == "BasicAuthenticationInterceptor" {
					return []AuthRecord{{Kind: "basic", Source: "spring_client_interceptor", Confidence: "EXTRACTED"}}
				}
			}
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
		if left.Query != right.Query {
			return left.Query < right.Query
		}
		if javaQueryParamsKey(left.QueryParams) != javaQueryParamsKey(right.QueryParams) {
			return javaQueryParamsKey(left.QueryParams) < javaQueryParamsKey(right.QueryParams)
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

func javaAnnotationPath(annotation JavaAnnotationRecord, constants map[string]string, keys ...string) (string, string, bool) {
	for _, key := range keys {
		value := strings.TrimSpace(annotation.Attributes[key])
		raw, present := javaRawAnnotationAttribute(annotation, key)
		if !present {
			continue
		}
		if resolved, ok := constants[value]; ok {
			return resolved, raw, true
		}
		if javaQuotedAnnotationPath(raw) && !strings.Contains(value, "${") {
			return value, raw, true
		}
		return "", raw, false
	}
	return "", "", true
}

func javaRawAnnotationAttribute(annotation JavaAnnotationRecord, key string) (string, bool) {
	for _, part := range splitTopLevel(annotation.Arguments, ',') {
		piece := strings.TrimSpace(part)
		if piece == "" {
			continue
		}
		if index := strings.Index(piece, "="); index >= 0 {
			if strings.TrimSpace(piece[:index]) == key {
				return strings.TrimSpace(piece[index+1:]), true
			}
			continue
		}
		if key == "value" {
			return piece, true
		}
	}
	return "", false
}

func javaQuotedAnnotationPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	return len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"'
}

func javaDeclarativeRawPath(base, method string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(base) != "" {
		parts = append(parts, strings.TrimSpace(base))
	}
	if strings.TrimSpace(method) != "" {
		parts = append(parts, strings.TrimSpace(method))
	}
	return strings.Join(parts, " + ")
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

func resolveJavaContractPaths(request JavaHTTPCallRecord, source JavaSourceRecord, method JavaMethodRecord, paths javaPathIndex) ([]string, string) {
	expression := strings.TrimSpace(request.PathExpression)
	if expression == "" && request.Path != "" {
		return []string{request.Path}, "exact"
	}
	if value, ok := method.StringVars[expression]; ok {
		return []string{value}, "exact"
	}
	if value, ok := source.Constants[expression]; ok {
		return []string{value}, "exact"
	}
	if value, ok := method.StringExpressions[expression]; ok {
		if alternatives := javaRequestPathAlternatives(value, source, method, paths, 0); len(alternatives) > 0 {
			return alternatives, "resolved"
		}
	}
	if receiver, getter, ok := javaZeroArgumentGetterCall(expression); ok {
		if alternatives := javaGetterPathAlternatives(receiver, getter, source, method, paths, 0); len(alternatives) > 0 {
			return alternatives, "getter"
		}
		return nil, "partial"
	}
	if len(splitTopLevel(expression, '+')) > 1 {
		if alternatives := javaResolvedPathAlternatives(
			expression,
			source.Constants,
			method.StringExpressions,
			javaConfigurationBaseURLExpressions(source, method),
			0,
		); len(alternatives) > 0 {
			return alternatives, "resolved"
		}
		return nil, "partial"
	}
	if request.Path != "" {
		resolution := "exact"
		if strings.Contains(request.Path, "{dynamic}") {
			resolution = "resolved"
		}
		return []string{request.Path}, resolution
	}
	if alternatives := javaRequestPathAlternatives(expression, source, method, paths, 0); len(alternatives) > 0 {
		return alternatives, "resolved"
	}
	return nil, "partial"
}

func javaRequestPathAlternatives(
	expression string,
	source JavaSourceRecord,
	method JavaMethodRecord,
	paths javaPathIndex,
	depth int,
) []string {
	if depth > 8 {
		return nil
	}
	expression = strings.TrimSpace(expression)
	if value, ok := method.StringExpressions[expression]; ok && strings.TrimSpace(value) != expression {
		return javaRequestPathAlternatives(value, source, method, paths, depth+1)
	}
	if branches, ok := javaTernaryBranches(expression); ok {
		alternatives := make([]string, 0, len(branches))
		for _, branch := range branches {
			alternatives = append(alternatives, javaRequestPathAlternatives(branch, source, method, paths, depth+1)...)
			if len(alternatives) > 4 {
				return nil
			}
		}
		return javaUniquePathAlternatives(alternatives)
	}
	if javaTopLevelTernaryQuestion(expression) >= 0 {
		return nil
	}
	if receiver, getter, ok := javaZeroArgumentGetterCall(expression); ok {
		return javaGetterPathAlternatives(receiver, getter, source, method, paths, depth+1)
	}
	return javaResolvedPathAlternatives(
		expression,
		source.Constants,
		method.StringExpressions,
		javaConfigurationBaseURLExpressions(source, method),
		depth+1,
	)
}

func javaGetterPathAlternatives(
	receiver string,
	getter string,
	source JavaSourceRecord,
	method JavaMethodRecord,
	paths javaPathIndex,
	depth int,
) []string {
	receiverType := javaDeclaredReceiverType(source, method, receiver)
	candidates := paths.getters[javaGetterIndexKey(receiverType, getter)]
	if receiverType == "" || len(candidates) != 1 {
		return nil
	}
	candidate := candidates[0]
	return javaResolvedPathAlternatives(
		candidate.expression,
		candidate.constants,
		nil,
		candidate.baseURLs,
		depth+1,
	)
}

func javaResolvedPathAlternatives(
	expression string,
	constants, locals map[string]string,
	baseURLs map[string]bool,
	depth int,
) []string {
	if depth > 8 {
		return nil
	}
	expression = strings.TrimSpace(expression)
	if value, ok := locals[expression]; ok && strings.TrimSpace(value) != expression {
		return javaResolvedPathAlternatives(value, constants, locals, baseURLs, depth+1)
	}
	if branches, ok := javaTernaryBranches(expression); ok {
		alternatives := make([]string, 0, len(branches))
		for _, branch := range branches {
			alternatives = append(alternatives, javaResolvedPathAlternatives(branch, constants, locals, baseURLs, depth+1)...)
			if len(alternatives) > 4 {
				return nil
			}
		}
		return javaUniquePathAlternatives(alternatives)
	}
	if javaTopLevelTernaryQuestion(expression) >= 0 {
		return nil
	}
	if path, ok := javaResolvedPathExpression(expression, constants, locals, baseURLs, depth+1); ok {
		return []string{path}
	}
	return nil
}

func javaTernaryBranches(expression string) ([]string, bool) {
	question := javaTopLevelTernaryQuestion(expression)
	if question < 0 {
		return nil, false
	}
	parentheses, braces, brackets := 0, 0, 0
	nestedTernaries := 0
	quote := byte(0)
	escaped := false
	for index := question + 1; index < len(expression); index++ {
		current := expression[index]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == quote {
				quote = 0
			}
			continue
		}
		switch current {
		case '\'', '"':
			quote = current
		case '(':
			parentheses++
		case ')':
			if parentheses > 0 {
				parentheses--
			}
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case '?':
			if parentheses == 0 && braces == 0 && brackets == 0 {
				nestedTernaries++
			}
		case ':':
			if parentheses != 0 || braces != 0 || brackets != 0 {
				continue
			}
			if nestedTernaries > 0 {
				nestedTernaries--
				continue
			}
			left := strings.TrimSpace(expression[question+1 : index])
			right := strings.TrimSpace(expression[index+1:])
			if left == "" || right == "" {
				return nil, false
			}
			return []string{left, right}, true
		}
	}
	return nil, false
}

func javaTopLevelTernaryQuestion(expression string) int {
	parentheses, braces, brackets := 0, 0, 0
	quote := byte(0)
	escaped := false
	for index := 0; index < len(expression); index++ {
		current := expression[index]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == quote {
				quote = 0
			}
			continue
		}
		switch current {
		case '\'', '"':
			quote = current
		case '(':
			parentheses++
		case ')':
			if parentheses > 0 {
				parentheses--
			}
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case '?':
			if parentheses == 0 && braces == 0 && brackets == 0 {
				return index
			}
		}
	}
	return -1
}

func javaUniquePathAlternatives(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ReplaceAll(strings.TrimSpace(value), "//", "/")
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	if len(result) > 4 {
		return nil
	}
	return result
}

func javaResolvedPathExpression(expression string, constants, locals map[string]string, baseURLs map[string]bool, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	expression = strings.TrimSpace(expression)
	if value, ok := constants[expression]; ok {
		return value, strings.HasPrefix(value, "/")
	}
	if value, ok := locals[expression]; ok && strings.TrimSpace(value) != expression {
		return javaResolvedPathExpression(value, constants, locals, baseURLs, depth+1)
	}
	if value, ok := javaQuotedStringValue(expression); ok {
		return value, strings.HasPrefix(value, "/")
	}
	parts := splitTopLevel(expression, '+')
	if len(parts) > 1 {
		var path strings.Builder
		for _, rawPart := range parts {
			part := strings.TrimSpace(rawPart)
			value, resolved := javaResolvedPathPart(part, constants, locals, baseURLs, depth+1)
			if !resolved {
				if path.Len() > 0 {
					appendDynamicPathSegment(&path)
				} else if !baseURLs[part] {
					return "", false
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

func javaResolvedPathPart(part string, constants, locals map[string]string, baseURLs map[string]bool, depth int) (string, bool) {
	if value, ok := javaQuotedStringValue(part); ok {
		return value, true
	}
	if value, ok := constants[part]; ok {
		return value, true
	}
	if value, ok := locals[part]; ok && strings.TrimSpace(value) != part {
		return javaResolvedPathExpression(value, constants, locals, baseURLs, depth)
	}
	return "", false
}

func javaConfigurationBaseURLExpressions(source JavaSourceRecord, method JavaMethodRecord) map[string]bool {
	result := map[string]bool{}
	valueImported := javaHasImport(source.Imports, "org.springframework.beans.factory.annotation.Value")
	configurationPropertiesImported := javaHasImport(source.Imports, "org.springframework.boot.context.properties.ConfigurationProperties")
	configurationOwner := false
	if configurationPropertiesImported {
		for _, javaType := range source.Types {
			if javaType.Name == method.Owner && javaAnnotationsContain(javaType.Annotations, "ConfigurationProperties") {
				configurationOwner = true
				break
			}
		}
	}
	for _, field := range source.Fields {
		if field.Owner != method.Owner {
			continue
		}
		if javaHasImportedConfigurationAnnotation(field.Annotations, valueImported, configurationPropertiesImported) ||
			configurationOwner && javaConfigurationBaseURLField(field) {
			result[field.Name] = true
		}
	}
	for _, parameter := range method.Parameters {
		if javaHasImportedConfigurationAnnotation(parameter.Annotations, valueImported, configurationPropertiesImported) {
			result[parameter.Name] = true
		}
	}
	return result
}

func javaConfigurationBaseURLField(field JavaFieldRecord) bool {
	name := strings.ToLower(strings.TrimSpace(field.Name))
	return !field.Final && cleanJavaType(field.Type) == "String" &&
		(strings.Contains(name, "baseurl") || strings.Contains(name, "base_url") || name == "url")
}

func javaHasImportedConfigurationAnnotation(annotations []JavaAnnotationRecord, valueImported, configurationPropertiesImported bool) bool {
	return valueImported && javaAnnotationsContain(annotations, "Value") ||
		configurationPropertiesImported && javaAnnotationsContain(annotations, "ConfigurationProperties")
}

func javaAnnotationsContain(annotations []JavaAnnotationRecord, names ...string) bool {
	for _, annotation := range annotations {
		for _, name := range names {
			if annotation.Name == name {
				return true
			}
		}
	}
	return false
}

func javaQuotedStringValue(expression string) (string, bool) {
	expression = strings.TrimSpace(expression)
	if len(expression) < 2 || expression[0] != '"' || expression[len(expression)-1] != '"' {
		return "", false
	}
	return expression[1 : len(expression)-1], true
}

func javaZeroArgumentGetterCall(expression string) (string, string, bool) {
	expression = strings.TrimSpace(expression)
	if !strings.HasSuffix(expression, "()") {
		return "", "", false
	}
	prefix := strings.TrimSpace(strings.TrimSuffix(expression, "()"))
	dot := strings.LastIndex(prefix, ".")
	if dot <= 0 || dot == len(prefix)-1 {
		return "", "", false
	}
	receiver := strings.TrimSpace(prefix[:dot])
	getter := strings.TrimSpace(prefix[dot+1:])
	if !isJavaIdentifierPath(receiver) || !isJavaIdentifierPath(getter) || strings.Contains(getter, ".") {
		return "", "", false
	}
	return receiver, getter, true
}

func javaGetterIndexKey(owner, method string) string {
	return shortJavaName(cleanJavaType(owner)) + "\x00" + method
}

func javaDeclaredReceiverType(source JavaSourceRecord, method JavaMethodRecord, receiver string) string {
	if dot := strings.LastIndex(receiver, "."); dot >= 0 {
		receiver = receiver[dot+1:]
	}
	if typeName := method.LocalTypes[receiver]; typeName != "" {
		return shortJavaName(typeName)
	}
	for _, parameter := range method.Parameters {
		if parameter.Name == receiver {
			return shortJavaName(parameter.Type)
		}
	}
	for _, field := range source.Fields {
		if field.Owner == method.Owner && field.Name == receiver {
			return shortJavaName(field.Type)
		}
	}
	return ""
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
		key := fmt.Sprintf(
			"%s\x00%d\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s",
			record.File,
			record.Line,
			record.HTTPMethod,
			record.Path,
			record.RawPath,
			record.Query,
			javaQueryParamsKey(record.QueryParams),
			record.Caller,
			record.Reason,
		)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, record)
	}
	return result
}

func javaQueryParamsKey(params []QueryParamRecord) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, param.Name+"="+param.Value)
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}
