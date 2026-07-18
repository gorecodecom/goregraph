package scan

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	codeHelperStartRE = regexp.MustCompile(`\b(Get|Post|Put|Patch|Delete)Helper(?:WithStatus)?\s*\(`)
	codeFetchAPIRE    = regexp.MustCompile(`\bfetch\s*\(`)
	codeWekaRequestRE = regexp.MustCompile(`\bweka\.request\s*\(`)
	codeHTTPClientRE  = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\.\s*(get|post|put|patch|delete)\s*\(`)
	codeHTTPMethodRE  = regexp.MustCompile(`^\s*["'](GET|POST|PUT|PATCH|DELETE)["']\s*,\s*(.*)$`)
	codeAnyLiteralRE  = regexp.MustCompile(`["']([^"']+)["']|` + "`" + `([^` + "`" + `]+)` + "`")
	codeMethodRE      = regexp.MustCompile(`\bmethod\s*:\s*["']([A-Za-z]+)["']`)
	codePathLiteralRE = regexp.MustCompile(`["'](/[^"']+)["']|` + "`" + `(/[^` + "`" + `]+)` + "`")
	codeTemplateVarRE = regexp.MustCompile(`\$\{([^}]+)\}`)
	codeStringValueRE = regexp.MustCompile(`["']([A-Za-z0-9_./-]+)["']`)
	codeDataFieldRE   = regexp.MustCompile(`\b(?:data|result|responseBody)\.([A-Za-z_][A-Za-z0-9_]*)\b`)
)

func extractAPIContracts(file FileRecord, lines []string, functions []CodeFunctionRecord) []APIContractRecord {
	if isLowSignalCodeFile(file.Path) {
		return nil
	}
	switch file.Language {
	case "javascript", "typescript":
	default:
		return nil
	}

	source := strings.Join(lines, "\n")
	masked := maskJSSourceComments(source)
	maskedLines := strings.Split(masked, "\n")
	lineOffsets := sourceLineOffsets(maskedLines)
	model := buildJSLexicalModel(masked)
	var records []APIContractRecord
	for i, line := range maskedLines {
		if match := codeHelperStartRE.FindStringSubmatch(line); len(match) == 2 {
			callText := collectCallText(maskedLines, i, 5)
			if path, ok := firstPathLiteral(callText); ok {
				record := apiContract(file, helperHTTPMethod(match[1]), path, apiContractCaller(functions, i+1), dynamicEndpointCandidatesForLine(maskedLines, functions, i+1, path), responseFieldsForLine(maskedLines, functions, i+1), i+1, "helper-call-argument")
				start := lineOffsets[i] + codeHelperStartRE.FindStringIndex(line)[0]
				record.Auth = httpCallAuthForFile(source, start, matchingCallEnd(masked, start), file.Path)
				records = append(records, record)
			}
			continue
		}
		if codeWekaRequestRE.MatchString(line) {
			callText := collectCallText(maskedLines, i, 8)
			if method, path, ok := wekaRequestMethodPath(callText); ok {
				record := apiContract(file, method, path, apiContractCaller(functions, i+1), dynamicEndpointCandidatesForLine(maskedLines, functions, i+1, path), responseFieldsForLine(maskedLines, functions, i+1), i+1, "weka-request-call")
				start := lineOffsets[i] + codeWekaRequestRE.FindStringIndex(line)[0]
				record.Auth = httpCallAuthForFile(source, start, matchingCallEnd(masked, start), file.Path)
				records = append(records, record)
			}
		}
	}
	for _, match := range codeFetchAPIRE.FindAllStringIndex(masked, -1) {
		if !isJSCodeOffset(masked, match[0]) {
			continue
		}
		end := matchingCallEnd(masked, match[0])
		args := topLevelCallArguments(masked, match[0], end)
		if len(args) == 0 {
			continue
		}
		path, ok := firstPathLikeLiteral(masked[args[0].start:args[0].end])
		if !ok {
			continue
		}
		line := sourceLineForOffset(masked, match[0])
		method := "GET"
		if len(args) > 1 {
			if methodMatch := codeMethodRE.FindStringSubmatch(masked[args[1].start:args[1].end]); len(methodMatch) == 2 {
				method = strings.ToUpper(methodMatch[1])
			}
		}
		record := apiContract(file, method, path, apiContractCaller(functions, line), dynamicEndpointCandidatesForLine(maskedLines, functions, line, path), responseFieldsForLine(maskedLines, functions, line), line, "fetch-call")
		record.Auth = httpCallAuthForFile(source, match[0], end, file.Path)
		records = append(records, record)
	}
	for _, match := range codeHTTPClientRE.FindAllStringSubmatchIndex(masked, -1) {
		if len(match) != 6 || !isJSCodeOffset(masked, match[0]) {
			continue
		}
		receiver := masked[match[2]:match[3]]
		if _, ok := model.resolveHTTPClient(receiver, match[0]); !ok {
			continue
		}
		end := matchingCallEnd(masked, match[0])
		args := topLevelCallArguments(masked, match[0], end)
		if len(args) == 0 {
			continue
		}
		path, ok := firstPathLikeLiteral(masked[args[0].start:args[0].end])
		if !ok {
			continue
		}
		line := sourceLineForOffset(masked, match[0])
		method := strings.ToUpper(masked[match[4]:match[5]])
		record := apiContract(file, method, path, apiContractCaller(functions, line), dynamicEndpointCandidatesForLine(maskedLines, functions, line, path), responseFieldsForLine(maskedLines, functions, line), line, "http-client-call")
		record.Auth = httpCallAuthForFile(source, match[0], end, file.Path)
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].File != records[j].File {
			return records[i].File < records[j].File
		}
		if records[i].Line != records[j].Line {
			return records[i].Line < records[j].Line
		}
		return records[i].Path < records[j].Path
	})
	return records
}

func sourceLineOffsets(lines []string) []int {
	offsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		offsets[i] = offset
		offset += len(line) + 1
	}
	return offsets
}

func matchingCallEnd(source string, callStart int) int {
	if callStart < 0 || callStart >= len(source) {
		return callStart
	}
	open := strings.IndexByte(source[callStart:], '(')
	if open < 0 {
		return callStart
	}
	open += callStart
	depth := 0
	quote := byte(0)
	escaped := false
	for i := open; i < len(source); i++ {
		char := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(source)
}

type sourceSpan struct {
	start int
	end   int
}

func sourceLineForOffset(source string, offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(source[:offset], "\n")
}

func maskJSSourceComments(source string) string {
	masked := []byte(source)
	quote := byte(0)
	escaped := false
	for i := 0; i < len(masked); i++ {
		char := masked[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		if i+1 >= len(masked) || char != '/' {
			continue
		}
		switch masked[i+1] {
		case '/':
			for masked[i] != '\n' {
				masked[i] = ' '
				i++
				if i >= len(masked) {
					return string(masked)
				}
			}
			i--
		case '*':
			masked[i], masked[i+1] = ' ', ' '
			i += 2
			for i < len(masked) {
				if i+1 < len(masked) && masked[i] == '*' && masked[i+1] == '/' {
					masked[i], masked[i+1] = ' ', ' '
					i++
					break
				}
				if masked[i] != '\n' {
					masked[i] = ' '
				}
				i++
			}
		}
	}
	return string(masked)
}

func isJSCodeOffset(source string, offset int) bool {
	quote := byte(0)
	escaped := false
	for i := 0; i < len(source) && i < offset; i++ {
		char := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
		}
	}
	return quote == 0
}

func topLevelCallArguments(source string, callStart, callEnd int) []sourceSpan {
	if callStart < 0 || callEnd <= callStart || callEnd > len(source) {
		return nil
	}
	open := strings.IndexByte(source[callStart:callEnd], '(')
	if open < 0 {
		return nil
	}
	open += callStart
	close := callEnd - 1
	if close <= open || source[close] != ')' {
		return nil
	}
	return splitTopLevelSourceSpans(source, open+1, close)
}

func splitTopLevelSourceSpans(source string, start, end int) []sourceSpan {
	var spans []sourceSpan
	segmentStart := start
	parenDepth, braceDepth, bracketDepth := 0, 0, 0
	quote := byte(0)
	escaped := false
	appendSegment := func(segmentEnd int) {
		left, right := segmentStart, segmentEnd
		for left < right && isJSSpace(source[left]) {
			left++
		}
		for right > left && isJSSpace(source[right-1]) {
			right--
		}
		if left < right {
			spans = append(spans, sourceSpan{start: left, end: right})
		}
	}
	for i := start; i < end; i++ {
		char := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case ',':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				appendSegment(i)
				segmentStart = i + 1
			}
		}
	}
	appendSegment(end)
	return spans
}

func isJSSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\r' || char == '\n'
}

type jsScope struct {
	start  int
	end    int
	parent int
}

type jsBinding struct {
	name         string
	kind         string
	scope        int
	start        int
	end          int
	createSource string
}

type jsLexicalModel struct {
	source   string
	scopes   []jsScope
	bindings []jsBinding
}

const (
	jsBindingOther       = "other"
	jsBindingAxiosImport = "axios_import"
	jsBindingAxiosClient = "axios_client"
)

func buildJSLexicalModel(source string) jsLexicalModel {
	model := jsLexicalModel{source: source, scopes: []jsScope{{start: 0, end: len(source), parent: -1}}}
	stack := []int{0}
	quote := byte(0)
	escaped := false
	for i := 0; i < len(source); i++ {
		char := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '{':
			parent := stack[len(stack)-1]
			model.scopes = append(model.scopes, jsScope{start: i, end: len(source), parent: parent})
			stack = append(stack, len(model.scopes)-1)
		case '}':
			if len(stack) > 1 {
				scope := stack[len(stack)-1]
				model.scopes[scope].end = i + 1
				stack = stack[:len(stack)-1]
			}
		}
	}
	model.addAxiosImports()
	model.addFunctionParameters()
	model.addLocalBindings()
	for i := range model.bindings {
		binding := &model.bindings[i]
		if binding.createSource == "" {
			continue
		}
		if sourceBinding, ok := model.resolveBinding(binding.createSource, binding.start); ok && model.bindings[sourceBinding].kind == jsBindingAxiosImport {
			binding.kind = jsBindingAxiosClient
		}
	}
	return model
}

func (model *jsLexicalModel) addAxiosImports() {
	importRE := regexp.MustCompile(`(?m)\bimport\s+([A-Za-z_$][A-Za-z0-9_$]*)\s+from\s+["']axios["']`)
	for _, match := range importRE.FindAllStringSubmatchIndex(model.source, -1) {
		if len(match) != 4 || !isJSCodeOffset(model.source, match[0]) {
			continue
		}
		model.bindings = append(model.bindings, jsBinding{name: model.source[match[2]:match[3]], kind: jsBindingAxiosImport, scope: 0, start: match[0]})
	}
}

func (model *jsLexicalModel) addFunctionParameters() {
	functionRE := regexp.MustCompile(`(?s)\bfunction\s*[A-Za-z_$0-9]*\s*\(([^)]*)\)\s*\{`)
	for _, match := range functionRE.FindAllStringSubmatchIndex(model.source, -1) {
		if len(match) != 4 || !isJSCodeOffset(model.source, match[0]) {
			continue
		}
		model.addParameterBindings(model.source[match[2]:match[3]], match[1]-1)
	}
	arrowRE := regexp.MustCompile(`(?s)(?:\(([^)]*)\)|([A-Za-z_$][A-Za-z0-9_$]*))\s*=>\s*(\{)?`)
	for _, match := range arrowRE.FindAllStringSubmatchIndex(model.source, -1) {
		if len(match) != 8 || !isJSCodeOffset(model.source, match[0]) {
			continue
		}
		params := ""
		if match[2] >= 0 {
			params = model.source[match[2]:match[3]]
		} else if match[4] >= 0 {
			params = model.source[match[4]:match[5]]
		}
		if match[6] >= 0 {
			model.addParameterBindings(params, match[6])
			continue
		}
		model.addConciseArrowParameterBindings(params, match[0], conciseArrowExpressionEnd(model.source, match[1]))
	}
}

func (model *jsLexicalModel) addParameterBindings(params string, scopeStart int) {
	scope := model.scopeStartingAt(scopeStart)
	if scope < 0 {
		return
	}
	identifierRE := regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
	for _, param := range strings.Split(params, ",") {
		name := simpleJSParameterName(param)
		if identifierRE.MatchString(name) {
			model.bindings = append(model.bindings, jsBinding{name: name, kind: jsBindingOther, scope: scope, start: scopeStart})
		}
	}
}

func (model *jsLexicalModel) addConciseArrowParameterBindings(params string, start, end int) {
	identifierRE := regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
	scope := model.scopeAt(start)
	for _, param := range strings.Split(params, ",") {
		name := simpleJSParameterName(param)
		if identifierRE.MatchString(name) {
			model.bindings = append(model.bindings, jsBinding{name: name, kind: jsBindingOther, scope: scope, start: start, end: end})
		}
	}
}

func simpleJSParameterName(parameter string) string {
	name := strings.TrimSpace(strings.SplitN(parameter, "=", 2)[0])
	if colon := strings.IndexByte(name, ':'); colon >= 0 {
		name = name[:colon]
	}
	return strings.TrimSpace(strings.TrimSuffix(name, "?"))
}

func conciseArrowExpressionEnd(source string, start int) int {
	parenDepth, braceDepth, bracketDepth := 0, 0, 0
	quote := byte(0)
	escaped := false
	for i := start; i < len(source); i++ {
		char := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '(':
			parenDepth++
		case ')':
			if parenDepth == 0 {
				return i
			}
			parenDepth--
		case '{':
			braceDepth++
		case '}':
			if braceDepth == 0 {
				return i
			}
			braceDepth--
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth == 0 {
				return i
			}
			bracketDepth--
		case ',', ';', '\n':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				return i
			}
		}
	}
	return len(source)
}

func (model *jsLexicalModel) addLocalBindings() {
	declarationRE := regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)(?:\s*=\s*([A-Za-z_$][A-Za-z0-9_$]*)\.create\s*\()?`)
	for _, match := range declarationRE.FindAllStringSubmatchIndex(model.source, -1) {
		if len(match) != 6 || !isJSCodeOffset(model.source, match[0]) {
			continue
		}
		binding := jsBinding{name: model.source[match[2]:match[3]], kind: jsBindingOther, scope: model.scopeAt(match[0]), start: match[0]}
		if match[4] >= 0 {
			binding.createSource = model.source[match[4]:match[5]]
		}
		model.bindings = append(model.bindings, binding)
	}
}

func (model jsLexicalModel) scopeAt(offset int) int {
	best := 0
	for i := 1; i < len(model.scopes); i++ {
		scope := model.scopes[i]
		if offset >= scope.start && offset < scope.end && scope.start >= model.scopes[best].start && scope.end <= model.scopes[best].end {
			best = i
		}
	}
	return best
}

func (model jsLexicalModel) scopeStartingAt(start int) int {
	for i, scope := range model.scopes {
		if scope.start == start {
			return i
		}
	}
	return -1
}

func (model jsLexicalModel) resolveBinding(name string, offset int) (int, bool) {
	for scope := model.scopeAt(offset); scope >= 0; scope = model.scopes[scope].parent {
		best := -1
		for i, binding := range model.bindings {
			if binding.scope != scope || binding.name != name || binding.start >= offset || (binding.end > 0 && offset >= binding.end) {
				continue
			}
			if best < 0 || binding.start > model.bindings[best].start {
				best = i
			}
		}
		if best >= 0 {
			return best, true
		}
	}
	return -1, false
}

func (model jsLexicalModel) resolveHTTPClient(receiver string, offset int) (int, bool) {
	binding, ok := model.resolveBinding(receiver, offset)
	if !ok {
		return -1, false
	}
	kind := model.bindings[binding].kind
	return binding, kind == jsBindingAxiosImport || kind == jsBindingAxiosClient
}

func httpCallAuthForFile(source string, callStart, callEnd int, file string) []AuthRecord {
	records := extractHTTPCallAuth(source, callStart, callEnd)
	for i := range records {
		records[i].File = file
	}
	return records
}

// extractHTTPCallAuth returns authentication evidence statically associated with one HTTP call.
func extractHTTPCallAuth(source string, callStart, callEnd int) []AuthRecord {
	if callStart < 0 || callEnd <= callStart || callStart >= len(source) {
		return nil
	}
	if callEnd > len(source) {
		callEnd = len(source)
	}
	masked := maskJSSourceComments(source)
	call := masked[callStart:callEnd]
	line := 1 + strings.Count(source[:callStart], "\n")
	config := httpCallConfigExpression(masked, callStart, callEnd)
	records := extractDirectHTTPCallAuth(masked, config, line, "EXTRACTED", "http_call_config")
	receiver := httpCallReceiver(call)
	if receiver == "" {
		return records
	}
	model := buildJSLexicalModel(masked)
	binding, ok := model.resolveHTTPClient(receiver, callStart)
	if !ok {
		return records
	}
	for _, interceptor := range associatedRequestInterceptors(masked, receiver, callStart, model, binding) {
		interceptorLine := 1 + strings.Count(masked[:interceptor.start], "\n")
		partial := extractDirectHTTPCallAuth(masked, interceptor.expression, interceptorLine, "PARTIAL", "http_client_interceptor")
		records = appendUniqueAuth(records, partial...)
	}
	return records
}

func httpCallConfigExpression(source string, callStart, callEnd int) string {
	args := topLevelCallArguments(source, callStart, callEnd)
	if len(args) == 0 {
		return ""
	}
	prefixEnd := strings.IndexByte(source[callStart:callEnd], '(')
	if prefixEnd < 0 {
		return ""
	}
	prefix := strings.TrimSpace(source[callStart : callStart+prefixEnd])
	configIndex := -1
	switch {
	case regexp.MustCompile(`^fetch$`).MatchString(prefix):
		configIndex = 1
	case regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*\s*\.\s*(?:get|delete)$`).MatchString(prefix):
		configIndex = 1
	case regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*\s*\.\s*(?:post|put|patch)$`).MatchString(prefix):
		configIndex = 2
	default:
		return source[callStart:callEnd]
	}
	if configIndex < 0 || configIndex >= len(args) {
		return ""
	}
	return source[args[configIndex].start:args[configIndex].end]
}

type requestInterceptorExpression struct {
	start      int
	expression string
}

func associatedRequestInterceptors(source, receiver string, callStart int, model jsLexicalModel, callBinding int) []requestInterceptorExpression {
	startRE := regexp.MustCompile(`\b` + regexp.QuoteMeta(receiver) + `\.interceptors\.request\.use\s*\(`)
	indices := startRE.FindAllStringIndex(source[:callStart], -1)
	result := make([]requestInterceptorExpression, 0, len(indices))
	for _, index := range indices {
		if !isJSCodeOffset(source, index[0]) {
			continue
		}
		binding, ok := model.resolveHTTPClient(receiver, index[0])
		if !ok || binding != callBinding {
			continue
		}
		end := matchingCallEnd(source, index[0])
		if end <= index[0] {
			continue
		}
		result = append(result, requestInterceptorExpression{start: index[0], expression: source[index[0]:end]})
	}
	return result
}

func httpCallReceiver(call string) string {
	match := regexp.MustCompile(`^\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\.\s*(?:get|post|put|patch|delete)\s*\(`).FindStringSubmatch(call)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func extractDirectHTTPCallAuth(fullSource, expression string, line int, confidence, source string) []AuthRecord {
	var records []AuthRecord
	var oauthCredentialValues []string
	properties := topLevelObjectProperties(expression)
	for _, headers := range propertyValuesByName(properties, "headers") {
		for _, header := range topLevelObjectProperties(headers) {
			lowerName := strings.ToLower(header.name)
			switch {
			case lowerName == "authorization":
				oauthCredentialValues = append(oauthCredentialValues, header.value)
				lowerValue := strings.ToLower(header.value)
				if strings.Contains(lowerValue, "bearer") {
					records = appendUniqueAuth(records, newHTTPAuthRecord("bearer", sanitizedCredentialExpression(header.value), source, confidence, line))
				} else if strings.Contains(lowerValue, "basic") {
					records = appendUniqueAuth(records, newHTTPAuthRecord("basic", sanitizedCredentialExpression(header.value), source, confidence, line))
				}
			case isAPIKeyPropertyName(header.name):
				records = appendUniqueAuth(records, newHTTPAuthRecord("api_key", sanitizedCredentialExpression(header.value), source, confidence, line))
			}
		}
	}
	assignmentRE := regexp.MustCompile(`(?is)\.headers(?:\.authorization|\[\s*["']authorization["']\s*\])\s*=\s*([^,;\n]+)`)
	for _, value := range assignmentRE.FindAllStringSubmatch(expression, -1) {
		if len(value) != 2 {
			continue
		}
		oauthCredentialValues = append(oauthCredentialValues, value[1])
		lower := strings.ToLower(value[1])
		if strings.Contains(lower, "bearer") {
			records = appendUniqueAuth(records, newHTTPAuthRecord("bearer", sanitizedCredentialExpression(value[1]), source, confidence, line))
		} else if strings.Contains(lower, "basic") {
			records = appendUniqueAuth(records, newHTTPAuthRecord("basic", sanitizedCredentialExpression(value[1]), source, confidence, line))
		}
	}
	apiKeyAssignmentRE := regexp.MustCompile(`(?is)\.headers(?:\.(?:apiKey|apikey)|\[\s*["'](?:x-api-key|api-key|api_key|apikey|ocp-apim-subscription-key)["']\s*\])\s*=\s*([^,;\n]+)`)
	for _, value := range apiKeyAssignmentRE.FindAllStringSubmatch(expression, -1) {
		if len(value) == 2 {
			records = appendUniqueAuth(records, newHTTPAuthRecord("api_key", sanitizedCredentialExpression(value[1]), source, confidence, line))
		}
	}
	for _, authBlock := range propertyValuesByName(properties, "auth") {
		authProperties := topLevelObjectProperties(authBlock)
		userValues := propertyValuesByName(authProperties, "username")
		passwordValues := propertyValuesByName(authProperties, "password")
		if len(userValues) == 0 && len(passwordValues) == 0 {
			continue
		}
		user, password := "", ""
		if len(userValues) > 0 {
			user = sanitizedCredentialExpression(userValues[0])
		}
		if len(passwordValues) > 0 {
			password = sanitizedCredentialExpression(passwordValues[0])
		}
		basicExpression := strings.Trim(strings.Join([]string{user, password}, ","), ",")
		records = appendUniqueAuth(records, newHTTPAuthRecord("basic", basicExpression, source, confidence, line))
	}
	for _, blockName := range []string{"params", "query"} {
		for _, block := range propertyValuesByName(properties, blockName) {
			for _, property := range topLevelObjectProperties(block) {
				if isAPIKeyPropertyName(property.name) {
					records = appendUniqueAuth(records, newHTTPAuthRecord("api_key", sanitizedCredentialExpression(property.value), source, confidence, line))
				}
			}
		}
	}
	credentials := propertyValuesByName(properties, "credentials")
	withCredentials := propertyValuesByName(properties, "withCredentials")
	if anyValueMatches(credentials, `(?i)^\s*["'](?:include|same-origin)["']\s*$`) || anyValueMatches(withCredentials, `(?i)^\s*true\s*$`) {
		records = appendUniqueAuth(records, newHTTPAuthRecord("session", "", source, confidence, line))
	}
	for _, helper := range importedOAuthHelpers(fullSource) {
		used := false
		for _, value := range oauthCredentialValues {
			if regexp.MustCompile(`\b` + regexp.QuoteMeta(helper) + `\s*\(`).MatchString(value) {
				used = true
				break
			}
		}
		if !used {
			continue
		}
		records = appendUniqueAuth(records, AuthRecord{Kind: "oauth2", Expression: helper, Source: oauthRecordSource(source), Confidence: confidence, Line: line})
	}
	return records
}

type jsObjectProperty struct {
	name  string
	value string
}

func topLevelObjectProperties(expression string) []jsObjectProperty {
	start := 0
	for start < len(expression) && isJSSpace(expression[start]) {
		start++
	}
	if start >= len(expression) || expression[start] != '{' {
		return nil
	}
	end := matchingBraceEnd(expression, start)
	if end <= start {
		return nil
	}
	var properties []jsObjectProperty
	for _, span := range splitTopLevelSourceSpans(expression, start+1, end-1) {
		colon := topLevelPropertyColon(expression, span.start, span.end)
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(expression[span.start:colon])
		name = strings.Trim(name, `"'`)
		if name == "" {
			continue
		}
		properties = append(properties, jsObjectProperty{name: name, value: strings.TrimSpace(expression[colon+1 : span.end])})
	}
	return properties
}

func topLevelPropertyColon(source string, start, end int) int {
	parenDepth, braceDepth, bracketDepth := 0, 0, 0
	quote := byte(0)
	escaped := false
	for i := start; i < end; i++ {
		char := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case ':':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				return i
			}
		}
	}
	return -1
}

func propertyValuesByName(properties []jsObjectProperty, name string) []string {
	var values []string
	for _, property := range properties {
		if strings.EqualFold(property.name, name) {
			values = append(values, property.value)
		}
	}
	return values
}

func isAPIKeyPropertyName(name string) bool {
	switch strings.ToLower(name) {
	case "x-api-key", "api-key", "api_key", "apikey", "ocp-apim-subscription-key":
		return true
	default:
		return false
	}
}

func anyValueMatches(values []string, pattern string) bool {
	re := regexp.MustCompile(pattern)
	for _, value := range values {
		if re.MatchString(value) {
			return true
		}
	}
	return false
}

func newHTTPAuthRecord(kind, expression, source, confidence string, line int) AuthRecord {
	return AuthRecord{Kind: kind, Expression: expression, Source: source, Confidence: confidence, Line: line}
}

func oauthRecordSource(source string) string {
	if source == "http_call_config" {
		return "oauth_helper"
	}
	return source
}

func matchingBraceEnd(source string, open int) int {
	depth := 0
	quote := byte(0)
	escaped := false
	for i := open; i < len(source); i++ {
		char := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func sanitizedCredentialExpression(value string) string {
	for _, match := range regexp.MustCompile(`\$\{\s*([^}]+)\s*\}`).FindAllStringSubmatch(value, -1) {
		if len(match) == 2 {
			if candidate := firstCredentialIdentifier(match[1]); candidate != "" {
				return candidate
			}
		}
	}
	withoutStrings := regexp.MustCompile(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|`+"`[^`]*`"+``).ReplaceAllString(value, " ")
	return firstCredentialIdentifier(withoutStrings)
}

func firstCredentialIdentifier(value string) string {
	ignored := map[string]bool{
		"basic": true, "bearer": true, "true": true, "false": true, "null": true, "undefined": true,
	}
	identifierRE := regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*(?:\.[A-Za-z_$][A-Za-z0-9_$]*)*`)
	for _, candidate := range identifierRE.FindAllString(value, -1) {
		if !ignored[strings.ToLower(candidate)] {
			return strings.TrimSpace(candidate)
		}
	}
	return ""
}

func importedOAuthHelpers(source string) []string {
	supported := map[string]bool{
		"getAccessToken": true, "getAccessTokenSilently": true, "getTokenSilently": true, "acquireTokenSilent": true,
	}
	seen := map[string]bool{}
	var result []string
	importRE := regexp.MustCompile(`(?s)\bimport\s*\{([^}]*)\}\s*from\s*["']([^"']+)["']`)
	for _, match := range importRE.FindAllStringSubmatch(source, -1) {
		if len(match) != 3 || !isOAuthModule(match[2]) {
			continue
		}
		for _, item := range strings.Split(match[1], ",") {
			parts := regexp.MustCompile(`\s+as\s+`).Split(strings.TrimSpace(item), 2)
			if len(parts) == 0 || !supported[parts[0]] {
				continue
			}
			localName := parts[0]
			if len(parts) == 2 {
				localName = strings.TrimSpace(parts[1])
			}
			if regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`).MatchString(localName) && !seen[localName] {
				seen[localName] = true
				result = append(result, localName)
			}
		}
	}
	sort.Strings(result)
	return result
}

func isOAuthModule(module string) bool {
	lower := strings.ToLower(module)
	for _, marker := range []string{"auth0", "oauth", "oidc", "msal", "keycloak"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func appendUniqueAuth(records []AuthRecord, additions ...AuthRecord) []AuthRecord {
	for _, addition := range additions {
		duplicate := false
		for _, record := range records {
			if record.Kind == addition.Kind && record.Expression == addition.Expression && record.Source == addition.Source && record.Confidence == addition.Confidence {
				duplicate = true
				break
			}
		}
		if !duplicate {
			records = append(records, addition)
		}
	}
	return records
}

func wekaRequestMethodPath(callText string) (string, string, bool) {
	open := strings.Index(callText, "(")
	close := strings.LastIndex(callText, ")")
	if open < 0 || close <= open {
		return "", "", false
	}
	args := strings.TrimSpace(callText[open+1 : close])
	match := codeHTTPMethodRE.FindStringSubmatch(args)
	if len(match) != 3 {
		return "", "", false
	}
	method := strings.ToUpper(match[1])
	remainder := strings.TrimSpace(match[2])
	path, ok := firstPathLikeLiteral(remainder)
	return method, path, ok
}

func apiContractCaller(functions []CodeFunctionRecord, line int) string {
	var best CodeFunctionRecord
	for _, function := range functions {
		if function.Line <= 0 || function.EndLine <= 0 {
			continue
		}
		if line < function.Line || line > function.EndLine {
			continue
		}
		if best.Name == "" || function.Line >= best.Line {
			best = function
		}
	}
	return best.Name
}

func dynamicEndpointCandidatesForLine(lines []string, functions []CodeFunctionRecord, line int, rawPath string) []string {
	if !strings.Contains(rawPath, "${endpoint}") && !strings.Contains(strings.ToLower(rawPath), "${dynamicendpoint}") {
		return nil
	}
	var owner CodeFunctionRecord
	for _, function := range functions {
		if function.Line <= 0 || function.EndLine <= 0 {
			continue
		}
		if line >= function.Line && line <= function.EndLine && (owner.Name == "" || function.Line >= owner.Line) {
			owner = function
		}
	}
	if owner.Name == "" {
		return nil
	}
	start := owner.Line - 1
	end := owner.EndLine
	if start < 0 || end > len(lines) || start >= end {
		return nil
	}
	seen := map[string]bool{}
	var candidates []string
	for i := start; i < end; i++ {
		for _, match := range codeStringValueRE.FindAllStringSubmatch(lines[i], -1) {
			if len(match) != 2 || !isLikelyDynamicEndpointCandidate(match[1]) {
				continue
			}
			if seen[match[1]] {
				continue
			}
			seen[match[1]] = true
			candidates = append(candidates, match[1])
		}
	}
	sort.Strings(candidates)
	return candidates
}

func responseFieldsForLine(lines []string, functions []CodeFunctionRecord, line int) []string {
	start := line - 1
	end := line
	for _, function := range functions {
		if function.Line <= 0 || function.EndLine <= 0 {
			continue
		}
		if line >= function.Line && line <= function.EndLine {
			start = function.Line - 1
			end = function.EndLine
			break
		}
	}
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	seen := map[string]bool{}
	var fields []string
	for i := start; i < end; i++ {
		for _, match := range codeDataFieldRE.FindAllStringSubmatch(lines[i], -1) {
			if len(match) != 2 || seen[match[1]] {
				continue
			}
			seen[match[1]] = true
			fields = append(fields, match[1])
		}
	}
	sort.Strings(fields)
	return fields
}

func isLikelyDynamicEndpointCandidate(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "://") {
		return false
	}
	if !strings.Contains(value, "/") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || strings.ContainsAny(part, "${}?&=:") {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func apiContract(file FileRecord, method, path, caller string, dynamicEndpointCandidates, responseFields []string, line int, reason string) APIContractRecord {
	normalizedPath, query, params, unsafeDynamic := normalizeAPIPathDetails(path)
	serviceCandidate := serviceCandidateForPath(normalizedPath)
	if isFrontendInternalAPIPath(file.Path, normalizedPath) {
		serviceCandidate = ""
		reason += "; frontend-internal-api-route"
	}
	return APIContractRecord{
		Language:                  file.Language,
		App:                       codeFileApp(file.Path),
		Package:                   codeFilePackage(file.Path),
		HTTPMethod:                method,
		Path:                      normalizedPath,
		RawPath:                   path,
		Query:                     query,
		QueryParams:               params,
		ResponseFields:            responseFields,
		ServiceCandidate:          serviceCandidate,
		UnsafeDynamic:             unsafeDynamic,
		DynamicEndpointCandidates: dynamicEndpointCandidates,
		Caller:                    strings.TrimSpace(caller),
		File:                      file.Path,
		Line:                      line,
		Confidence:                "EXTRACTED",
		ConfidenceScore:           0.9,
		Reason:                    reason,
	}
}

func isFrontendInternalAPIPath(filePath, apiPath string) bool {
	if !strings.HasPrefix(apiPath, "/api/") {
		return false
	}
	path := filepath.ToSlash(filePath)
	if strings.Contains(path, "/app/api/") || strings.Contains(path, "/pages/api/") {
		return false
	}
	return strings.Contains(path, "/src/app/") ||
		strings.HasPrefix(path, "src/app/") ||
		strings.HasPrefix(path, "app/") ||
		strings.Contains(path, "/app/")
}

func helperHTTPMethod(name string) string {
	name = strings.TrimSuffix(name, "Helper")
	name = strings.TrimSuffix(name, "HelperWithStatus")
	switch strings.ToLower(name) {
	case "get":
		return "GET"
	case "post":
		return "POST"
	case "put":
		return "PUT"
	case "patch":
		return "PATCH"
	case "delete":
		return "DELETE"
	default:
		return strings.ToUpper(name)
	}
}

func collectCallText(lines []string, start, maxLines int) string {
	depth := 0
	seenOpen := false
	var parts []string
	for i := start; i < len(lines) && i < start+maxLines; i++ {
		line := stripCodeLineComment("javascript", lines[i])
		parts = append(parts, strings.TrimSpace(line))
		depth += strings.Count(line, "(")
		if strings.Contains(line, "(") {
			seenOpen = true
		}
		depth -= strings.Count(line, ")")
		if seenOpen && depth <= 0 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func firstPathLiteral(callText string) (string, bool) {
	for _, match := range codePathLiteralRE.FindAllStringSubmatch(callText, -1) {
		for _, group := range match[1:] {
			if strings.HasPrefix(group, "/") {
				return group, true
			}
		}
	}
	return "", false
}

func firstPathLikeLiteral(callText string) (string, bool) {
	for _, match := range codeAnyLiteralRE.FindAllStringSubmatch(callText, -1) {
		for _, group := range match[1:] {
			group = strings.TrimSpace(group)
			if group == "" || !isLikelyAPIPathLiteral(group) {
				continue
			}
			return group, true
		}
	}
	return "", false
}

func isLikelyAPIPathLiteral(value string) bool {
	if strings.Contains(value, "://") {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return true
	}
	first := strings.Split(strings.TrimPrefix(value, "./"), "/")[0]
	if first == "" || strings.ContainsAny(first, "{}$?&=:") {
		return false
	}
	if strings.Contains(value, "/") {
		return true
	}
	switch first {
	case "search", "tree", "userservice", "useritem", "documenttopic", "documentdownload", "documentinfo", "documentexport", "containertree", "cadastertask", "cadasters", "productservice", "licenseservice", "swlicenseservice", "task", "portal":
		return true
	default:
		return false
	}
}

func normalizeAPIPath(path string) string {
	normalized, _, _, _ := normalizeAPIPathDetails(path)
	return normalized
}

func normalizeAPIPathDetails(raw string) (string, string, []QueryParamRecord, bool) {
	raw = strings.TrimSpace(raw)
	rawPath, query := splitRawPathQuery(raw)
	path, unsafePath := normalizeTemplatePath(rawPath)
	params, unsafeQuery := normalizeQueryParams(query)
	return path, query, params, unsafePath || unsafeQuery
}

func splitRawPathQuery(raw string) (string, string) {
	templateDepth := 0
	for i := 0; i < len(raw); i++ {
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			templateDepth++
			i++
			continue
		}
		if raw[i] == '}' && templateDepth > 0 {
			templateDepth--
			continue
		}
		if raw[i] == '?' && templateDepth == 0 {
			return raw[:i], raw[i+1:]
		}
	}
	return raw, ""
}

func normalizeTemplatePath(path string) (string, bool) {
	unsafeDynamic := false
	var trimmed bool
	path, trimmed = trimOptionalTemplatePathSuffix(path)
	if trimmed {
		unsafeDynamic = false
	}
	path = codeTemplateVarRE.ReplaceAllStringFunc(path, func(match string) string {
		groups := codeTemplateVarRE.FindStringSubmatch(match)
		if len(groups) != 2 {
			unsafeDynamic = true
			return "{dynamic}"
		}
		name, safe := normalizeTemplateExpression(groups[1])
		if !safe {
			unsafeDynamic = true
		}
		return "{" + name + "}"
	})
	return normalizeCodeRoutePath(path), unsafeDynamic
}

func trimOptionalTemplatePathSuffix(path string) (string, bool) {
	idx := strings.LastIndex(path, "${")
	if idx < 0 {
		return path, false
	}
	end := strings.Index(path[idx:], "}")
	expression := path[idx+2:]
	if end >= 0 {
		expression = path[idx+2 : idx+end]
	}
	trimStart := idx
	if strings.Contains(expression, "?") {
		if idx > 0 && path[idx-1] == '/' {
			trimStart = idx - 1
		}
		return path[:trimStart], true
	}
	if strings.Contains(expression, "||") && idx > 0 && path[idx-1] != '/' {
		return path[:idx], true
	}
	if end >= 0 && idx > 0 && path[idx-1] != '/' && isOptionalPathSuffixExpression(expression) {
		return path[:idx], true
	}
	return path, false
}

func isOptionalPathSuffixExpression(expression string) bool {
	switch strings.ToLower(strings.TrimSpace(expression)) {
	case "filter", "query", "params", "search":
		return true
	default:
		return false
	}
}

func normalizeQueryParams(query string) ([]QueryParamRecord, bool) {
	if query == "" {
		return nil, false
	}
	unsafeDynamic := false
	var params []QueryParamRecord
	for _, pair := range strings.Split(query, "&") {
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		normalizedValue, unsafeValue := normalizeTemplateValue(value)
		if unsafeValue {
			unsafeDynamic = true
		}
		params = append(params, QueryParamRecord{Name: strings.TrimSpace(name), Value: normalizedValue})
	}
	sort.Slice(params, func(i, j int) bool {
		if params[i].Name != params[j].Name {
			return params[i].Name < params[j].Name
		}
		return params[i].Value < params[j].Value
	})
	return params, unsafeDynamic
}

func normalizeTemplateValue(value string) (string, bool) {
	unsafeDynamic := false
	value = codeTemplateVarRE.ReplaceAllStringFunc(value, func(match string) string {
		groups := codeTemplateVarRE.FindStringSubmatch(match)
		if len(groups) != 2 {
			unsafeDynamic = true
			return "{dynamic}"
		}
		name, safe := normalizeTemplateExpression(groups[1])
		if !safe {
			unsafeDynamic = true
		}
		return "{" + name + "}"
	})
	return value, unsafeDynamic
}

func normalizeTemplateExpression(expr string) (string, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" || strings.ContainsAny(expr, "?:+-*/<>=&|![](){}'\"` ") {
		return "dynamic", false
	}
	parts := strings.FieldsFunc(expr, func(r rune) bool {
		return r == '.' || r == '_'
	})
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return sanitizePlaceholder(parts[len(parts)-1]), true
	}
	return sanitizePlaceholder(expr), true
}

func sanitizePlaceholder(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "dynamic"
	}
	return b.String()
}

func serviceCandidateForPath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	segment := strings.ToLower(strings.Split(path, "/")[0])
	segment = strings.Trim(segment, "{}")
	switch segment {
	case "cadasters", "cadastermgmt", "cadastertask":
		return "ms-cadaster"
	case "tree":
		return "ms-regulationtree"
	case "downloads":
		return "ms-regulationdownload"
	case "regulations":
		return "ms-regulationinfo"
	case "users":
		return "ms-userservice"
	case "products":
		return "ms-productservice"
	case "tasks":
		return "ms-task"
	case "licenses":
		return "ms-licenseservice"
	}
	segment = strings.TrimSuffix(segment, "s")
	if segment == "" || segment == "dynamic" {
		return ""
	}
	return "ms-" + segment
}

func renderAPIContractsReport(records []APIContractRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph API Contracts\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		dynamic := ""
		if record.UnsafeDynamic {
			dynamic = ", unsafe dynamic URL"
		}
		query := ""
		if record.Query != "" {
			query = "?" + record.Query
		}
		caller := ""
		if record.Caller != "" {
			caller = fmt.Sprintf(", caller `%s`", record.Caller)
		}
		b.WriteString(fmt.Sprintf("- %s `%s%s` from `%s:%d` (app `%s`, service `%s`%s, %s%s)\n",
			record.HTTPMethod,
			record.Path,
			query,
			record.File,
			record.Line,
			record.App,
			emptyAsNone(record.ServiceCandidate),
			caller,
			record.Reason,
			dynamic,
		))
	}
	return b.String()
}
