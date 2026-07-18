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
	analysis := buildJSAPIAnalysis(source)
	lexical := analysis.lexical
	masked := lexical.comments
	code := lexical.code
	maskedLines := strings.Split(masked, "\n")
	codeLines := strings.Split(code, "\n")
	lineOffsets := sourceLineOffsets(maskedLines)
	var records []APIContractRecord
	for i, line := range codeLines {
		if match := codeHelperStartRE.FindStringSubmatch(line); len(match) == 2 {
			callText := collectCallText(maskedLines, i, 5)
			if path, ok := firstPathLiteral(callText); ok {
				record := apiContract(file, helperHTTPMethod(match[1]), path, apiContractCaller(functions, i+1), dynamicEndpointCandidatesForLine(maskedLines, functions, i+1, path), responseFieldsForLine(maskedLines, functions, i+1), i+1, "helper-call-argument")
				start := lineOffsets[i] + codeHelperStartRE.FindStringIndex(line)[0]
				record.Auth = analysis.authForCall(start, matchingCallEndCode(code, start), file.Path)
				records = append(records, record)
			}
			continue
		}
		if codeWekaRequestRE.MatchString(line) {
			callText := collectCallText(maskedLines, i, 8)
			if method, path, ok := wekaRequestMethodPath(callText); ok {
				record := apiContract(file, method, path, apiContractCaller(functions, i+1), dynamicEndpointCandidatesForLine(maskedLines, functions, i+1, path), responseFieldsForLine(maskedLines, functions, i+1), i+1, "weka-request-call")
				start := lineOffsets[i] + codeWekaRequestRE.FindStringIndex(line)[0]
				record.Auth = analysis.authForCall(start, matchingCallEndCode(code, start), file.Path)
				records = append(records, record)
			}
		}
	}
	for _, match := range codeFetchAPIRE.FindAllStringIndex(code, -1) {
		end := matchingCallEndCode(code, match[0])
		args := topLevelCallArgumentsCode(masked, code, match[0], end)
		if len(args) == 0 {
			continue
		}
		path, ok := firstPathLikeLiteral(masked[args[0].start:args[0].end])
		if !ok {
			continue
		}
		line := analysis.lineForOffset(match[0])
		method := "GET"
		if len(args) > 1 {
			if methodMatch := codeMethodRE.FindStringSubmatch(masked[args[1].start:args[1].end]); len(methodMatch) == 2 {
				method = strings.ToUpper(methodMatch[1])
			}
		}
		record := apiContract(file, method, path, apiContractCaller(functions, line), dynamicEndpointCandidatesForLine(maskedLines, functions, line, path), responseFieldsForLine(maskedLines, functions, line), line, "fetch-call")
		record.Auth = analysis.authForCall(match[0], end, file.Path)
		records = append(records, record)
	}
	for _, match := range codeHTTPClientRE.FindAllStringSubmatchIndex(code, -1) {
		if len(match) != 6 || !isCompleteJSReceiverCode(code, match[0]) {
			continue
		}
		receiver := code[match[2]:match[3]]
		if _, ok := analysis.model.resolveHTTPClient(receiver, match[0]); !ok {
			continue
		}
		end := matchingCallEndCode(code, match[0])
		args := topLevelCallArgumentsCode(masked, code, match[0], end)
		if len(args) == 0 {
			continue
		}
		path, ok := firstPathLikeLiteral(masked[args[0].start:args[0].end])
		if !ok {
			continue
		}
		line := analysis.lineForOffset(match[0])
		method := strings.ToUpper(code[match[4]:match[5]])
		record := apiContract(file, method, path, apiContractCaller(functions, line), dynamicEndpointCandidatesForLine(maskedLines, functions, line, path), responseFieldsForLine(maskedLines, functions, line), line, "http-client-call")
		record.Auth = analysis.authForCall(match[0], end, file.Path)
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

func isCompleteJSReceiver(source string, receiverStart int) bool {
	return isCompleteJSReceiverCode(scanJSLexicalSource(source).code, receiverStart)
}

func isCompleteJSReceiverCode(code string, receiverStart int) bool {
	immediate := receiverStart - 1
	if immediate < 0 {
		return true
	}
	if !isJSSpace(code[immediate]) {
		return code[immediate] != '.' && code[immediate] != '#' && !isScriptIdentifierByte(code[immediate])
	}
	previous := immediate
	for previous >= 0 && isJSSpace(code[previous]) {
		previous--
	}
	if previous < 0 {
		return true
	}
	return code[previous] != '.' && code[previous] != '#'
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
	return matchingCallEndCode(scanJSLexicalSource(source).code, callStart)
}

func matchingCallEndCode(code string, callStart int) int {
	if callStart < 0 || callStart >= len(code) {
		return callStart
	}
	open := strings.IndexByte(code[callStart:], '(')
	if open < 0 {
		return callStart
	}
	open += callStart
	depth := 0
	for i := open; i < len(code); i++ {
		switch code[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(code)
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

type jsLexicalSource struct {
	comments string
	code     string
}

func scanJSLexicalSource(source string) jsLexicalSource {
	comments := []byte(source)
	code := []byte(source)
	maskCode := func(start, end int) {
		for i := start; i < end && i < len(code); i++ {
			if code[i] != '\n' {
				code[i] = ' '
			}
		}
	}
	maskComment := func(start, end int) {
		for i := start; i < end && i < len(comments); i++ {
			if comments[i] != '\n' {
				comments[i] = ' '
			}
		}
		maskCode(start, end)
	}
	var scanCode func(int, bool) int
	var scanTemplate func(int) int
	scanQuoted := func(start int, quote byte) int {
		i := start + 1
		for i < len(source) {
			if source[i] == '\\' {
				i += 2
				continue
			}
			i++
			if source[i-1] == quote {
				break
			}
		}
		maskCode(start, i)
		return i
	}
	scanRegex := func(start int) int {
		i := start + 1
		inClass := false
		for i < len(source) {
			if source[i] == '\\' {
				i += 2
				continue
			}
			switch source[i] {
			case '[':
				inClass = true
			case ']':
				inClass = false
			case '/':
				if !inClass {
					i++
					for i < len(source) && isScriptIdentifierByte(source[i]) {
						i++
					}
					maskCode(start, i)
					return i
				}
			}
			i++
		}
		maskCode(start, i)
		return i
	}
	scanTemplate = func(start int) int {
		maskCode(start, start+1)
		for i := start + 1; i < len(source); {
			switch {
			case source[i] == '\\':
				maskCode(i, minAPIInt(i+2, len(source)))
				i += 2
			case source[i] == '`':
				maskCode(i, i+1)
				return i + 1
			case i+1 < len(source) && source[i] == '$' && source[i+1] == '{':
				maskCode(i, i+2)
				i = scanCode(i+2, true)
			default:
				maskCode(i, i+1)
				i++
			}
		}
		return len(source)
	}
	scanCode = func(start int, templateInterpolation bool) int {
		braceDepth := 0
		for i := start; i < len(source); {
			if templateInterpolation && source[i] == '}' && braceDepth == 0 {
				maskCode(i, i+1)
				return i + 1
			}
			switch {
			case i+1 < len(source) && source[i] == '/' && source[i+1] == '/':
				end := i + 2
				for end < len(source) && source[end] != '\n' {
					end++
				}
				maskComment(i, end)
				i = end
			case i+1 < len(source) && source[i] == '/' && source[i+1] == '*':
				end := i + 2
				for end+1 < len(source) && !(source[end] == '*' && source[end+1] == '/') {
					end++
				}
				if end+1 < len(source) {
					end += 2
				}
				maskComment(i, end)
				i = end
			case source[i] == '\'' || source[i] == '"':
				i = scanQuoted(i, source[i])
			case source[i] == '`':
				i = scanTemplate(i)
			case source[i] == '/' && isScriptRegexStart(code, i):
				i = scanRegex(i)
			default:
				if templateInterpolation {
					if source[i] == '{' {
						braceDepth++
					} else if source[i] == '}' && braceDepth > 0 {
						braceDepth--
					}
				}
				i++
			}
		}
		return len(source)
	}
	scanCode(0, false)
	return jsLexicalSource{comments: string(comments), code: string(code)}
}

func minAPIInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maskJSSourceComments(source string) string {
	return scanJSLexicalSource(source).comments
}

func isJSCodeOffset(source string, offset int) bool {
	if offset < 0 || offset >= len(source) {
		return false
	}
	return !isJSSpace(scanJSLexicalSource(source).code[offset])
}

func topLevelCallArguments(source string, callStart, callEnd int) []sourceSpan {
	return topLevelCallArgumentsCode(source, scanJSLexicalSource(source).code, callStart, callEnd)
}

func topLevelCallArgumentsCode(source, code string, callStart, callEnd int) []sourceSpan {
	if callStart < 0 || callEnd <= callStart || callEnd > len(code) {
		return nil
	}
	open := strings.IndexByte(code[callStart:callEnd], '(')
	if open < 0 {
		return nil
	}
	open += callStart
	close := callEnd - 1
	if close <= open || code[close] != ')' {
		return nil
	}
	return splitTopLevelCodeSpans(source, code, open+1, close)
}

func splitTopLevelSourceSpans(source string, start, end int) []sourceSpan {
	return splitTopLevelCodeSpans(source, scanJSLexicalSource(source).code, start, end)
}

func splitTopLevelCodeSpans(source, code string, start, end int) []sourceSpan {
	var spans []sourceSpan
	segmentStart := start
	parenDepth, braceDepth, bracketDepth := 0, 0, 0
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
		switch code[i] {
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
	source              string
	code                string
	scopes              []jsScope
	scopeAtOffsets      []int
	scopeByStart        map[int]int
	bindings            []jsBinding
	bindingKeys         map[string]bool
	bindingsByScopeName map[int]map[string][]int
}

type interceptorAuthEvidence struct {
	start int
	auth  []AuthRecord
}

type jsAPIAnalysis struct {
	source                   string
	lexical                  jsLexicalSource
	lineStarts               []int
	model                    jsLexicalModel
	interceptorAuthByBinding map[int][]interceptorAuthEvidence
}

const (
	jsBindingOther       = "other"
	jsBindingAxiosImport = "axios_import"
	jsBindingAxiosClient = "axios_client"
	jsBindingOAuthImport = "oauth_import"
)

func buildJSLexicalModel(source string) jsLexicalModel {
	return buildJSLexicalModelFromScan(scanJSLexicalSource(source))
}

func buildJSLexicalModelFromScan(lexical jsLexicalSource) jsLexicalModel {
	model := jsLexicalModel{
		source:              lexical.comments,
		code:                lexical.code,
		scopes:              []jsScope{{start: 0, end: len(lexical.comments), parent: -1}},
		scopeAtOffsets:      make([]int, len(lexical.code)+1),
		scopeByStart:        map[int]int{0: 0},
		bindingKeys:         map[string]bool{},
		bindingsByScopeName: map[int]map[string][]int{},
	}
	stack := []int{0}
	for i := 0; i < len(model.code); i++ {
		switch model.code[i] {
		case '{':
			parent := stack[len(stack)-1]
			model.scopes = append(model.scopes, jsScope{start: i, end: len(model.code), parent: parent})
			scope := len(model.scopes) - 1
			model.scopeByStart[i] = scope
			stack = append(stack, scope)
			model.scopeAtOffsets[i] = scope
		case '}':
			model.scopeAtOffsets[i] = stack[len(stack)-1]
			if len(stack) > 1 {
				scope := stack[len(stack)-1]
				model.scopes[scope].end = i + 1
				stack = stack[:len(stack)-1]
			}
		default:
			model.scopeAtOffsets[i] = stack[len(stack)-1]
		}
	}
	model.scopeAtOffsets[len(model.code)] = stack[len(stack)-1]
	model.addAxiosImports()
	model.addOAuthImports()
	model.addDeclarationBindings()
	model.addFunctionParameters()
	model.addMethodParameters()
	model.addCatchBindings()
	model.addLocalBindings()
	model.addVariablePatternBindings()
	model.buildBindingIndex()
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

func buildJSAPIAnalysis(source string) jsAPIAnalysis {
	lexical := scanJSLexicalSource(source)
	analysis := jsAPIAnalysis{
		source:                   source,
		lexical:                  lexical,
		lineStarts:               jsSourceLineStarts(source),
		model:                    buildJSLexicalModelFromScan(lexical),
		interceptorAuthByBinding: map[int][]interceptorAuthEvidence{},
	}
	analysis.indexRequestInterceptorAuth()
	return analysis
}

func jsSourceLineStarts(source string) []int {
	starts := []int{0}
	for offset := 0; offset < len(source); offset++ {
		if source[offset] == '\n' {
			starts = append(starts, offset+1)
		}
	}
	return starts
}

func (analysis jsAPIAnalysis) lineForOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(analysis.source) {
		offset = len(analysis.source)
	}
	return sort.Search(len(analysis.lineStarts), func(index int) bool {
		return analysis.lineStarts[index] > offset
	})
}

func (model *jsLexicalModel) addAxiosImports() {
	importRE := regexp.MustCompile(`(?m)\bimport\s+([A-Za-z_$][A-Za-z0-9_$]*)\s+from\s+["']axios["']`)
	for _, match := range importRE.FindAllStringSubmatchIndex(model.source, -1) {
		if len(match) != 4 || isJSSpace(model.code[match[0]]) {
			continue
		}
		model.bindings = append(model.bindings, jsBinding{name: model.source[match[2]:match[3]], kind: jsBindingAxiosImport, scope: 0, start: match[0]})
	}
}

func (model *jsLexicalModel) addOAuthImports() {
	supported := supportedOAuthHelperNames()
	importRE := regexp.MustCompile(`(?s)\bimport\s*\{([^}]*)\}\s*from\s*["']([^"']+)["']`)
	for _, match := range importRE.FindAllStringSubmatchIndex(model.source, -1) {
		if len(match) != 6 || isJSSpace(model.code[match[0]]) || !isOAuthModule(model.source[match[4]:match[5]]) {
			continue
		}
		for _, item := range strings.Split(model.source[match[2]:match[3]], ",") {
			parts := regexp.MustCompile(`\s+as\s+`).Split(strings.TrimSpace(item), 2)
			if len(parts) == 0 || !supported[parts[0]] {
				continue
			}
			localName := parts[0]
			if len(parts) == 2 {
				localName = strings.TrimSpace(parts[1])
			}
			if regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`).MatchString(localName) {
				model.bindings = append(model.bindings, jsBinding{name: localName, kind: jsBindingOAuthImport, scope: 0, start: match[0]})
			}
		}
	}
}

func (model *jsLexicalModel) addDeclarationBindings() {
	declarationRE := regexp.MustCompile(`\b(?:function|class)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	for _, match := range declarationRE.FindAllStringSubmatchIndex(model.code, -1) {
		if len(match) != 4 {
			continue
		}
		scope := model.scopeAt(match[0])
		model.addBinding(jsBinding{
			name:  model.source[match[2]:match[3]],
			kind:  jsBindingOther,
			scope: scope,
			start: model.scopes[scope].start,
		})
	}
}

func (model *jsLexicalModel) addFunctionParameters() {
	for search := 0; search < len(model.code); {
		relative := strings.Index(model.code[search:], "=>")
		if relative < 0 {
			break
		}
		arrow := search + relative
		paramsStart, paramsEnd, ok := jsArrowParameterSpan(model.code, arrow)
		if !ok {
			search = arrow + 2
			continue
		}
		bodyStart := nextScriptNonSpace(model.code, arrow+2)
		params := model.source[paramsStart:paramsEnd]
		if bodyStart < len(model.code) && model.code[bodyStart] == '{' {
			model.addParameterBindings(params, bodyStart)
		} else {
			model.addConciseArrowParameterBindings(params, paramsStart, conciseArrowExpressionEnd(model.code, bodyStart))
		}
		search = arrow + 2
	}
}

func jsArrowParameterSpan(code string, arrow int) (int, int, bool) {
	end := arrow
	for end > 0 && isJSSpace(code[end-1]) {
		end--
	}
	if end == 0 {
		return 0, 0, false
	}
	if code[end-1] == ')' {
		open := matchingScriptDelimiterBackward(code, end-1, '(', ')')
		if open < 0 {
			return 0, 0, false
		}
		return open + 1, end - 1, true
	}
	start := end - 1
	for start > 0 && isScriptIdentifierByte(code[start-1]) {
		start--
	}
	if start == end || !isScriptIdentifierByte(code[start]) {
		return 0, 0, false
	}
	return start, end, true
}

func (model *jsLexicalModel) addParameterBindings(params string, scopeStart int) {
	scope := model.scopeStartingAt(scopeStart)
	if scope < 0 {
		return
	}
	for _, name := range jsParameterBindingNames(params) {
		model.addBinding(jsBinding{name: name, kind: jsBindingOther, scope: scope, start: scopeStart})
	}
}

func (model *jsLexicalModel) addConciseArrowParameterBindings(params string, start, end int) {
	scope := model.scopeAt(start)
	for _, name := range jsParameterBindingNames(params) {
		model.addBinding(jsBinding{name: name, kind: jsBindingOther, scope: scope, start: start, end: end})
	}
}

func jsParameterBindingNames(params string) []string {
	seen := map[string]bool{}
	var names []string
	for _, parameter := range splitJSParameterList(params) {
		pattern := strings.TrimSpace(parameter)
		if colon := findJSParameterTopLevel(pattern, ':'); colon >= 0 {
			pattern = strings.TrimSpace(pattern[:colon])
		} else if equals := findJSParameterTopLevel(pattern, '='); equals >= 0 {
			pattern = strings.TrimSpace(pattern[:equals])
		}
		pattern = strings.TrimSuffix(pattern, "?")
		for name := range scriptBindingPatternNames(pattern) {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func splitJSParameterList(params string) []string {
	lexical := scanJSLexicalSource(params)
	var result []string
	start := 0
	round, square, curly, angle := 0, 0, 0, 0
	for index := 0; index < len(lexical.code); index++ {
		switch lexical.code[index] {
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		case '<':
			angle++
		case '>':
			if angle > 0 && (index == 0 || lexical.code[index-1] != '=') {
				angle--
			}
		case ',':
			if round == 0 && square == 0 && curly == 0 && angle == 0 {
				result = append(result, params[start:index])
				start = index + 1
			}
		}
	}
	return append(result, params[start:])
}

func findJSParameterTopLevel(value string, target byte) int {
	lexical := scanJSLexicalSource(value)
	round, square, curly, angle := 0, 0, 0, 0
	for index := 0; index < len(lexical.code); index++ {
		if lexical.code[index] == target && round == 0 && square == 0 && curly == 0 && angle == 0 {
			return index
		}
		switch lexical.code[index] {
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		case '<':
			angle++
		case '>':
			if angle > 0 && (index == 0 || lexical.code[index-1] != '=') {
				angle--
			}
		}
	}
	return -1
}

func (model *jsLexicalModel) addMethodParameters() {
	for open := strings.IndexByte(model.code, '('); open >= 0; {
		close := matchingScriptDelimiter(model.code, open, '(', ')')
		if close < 0 {
			break
		}
		bodyStart := scriptParameterBlockBodyStart(model.code, close)
		if bodyStart >= 0 && isScriptFunctionParameterScopePrefix(model.code, open, close, bodyStart) {
			model.addParameterBindings(model.source[open+1:close], bodyStart)
		}
		relative := strings.IndexByte(model.code[close+1:], '(')
		if relative < 0 {
			break
		}
		open = close + 1 + relative
	}
}

func (model *jsLexicalModel) addCatchBindings() {
	catchRE := regexp.MustCompile(`(?s)\bcatch\s*\(([^)]*)\)\s*\{`)
	for _, match := range catchRE.FindAllStringSubmatchIndex(model.code, -1) {
		if len(match) == 4 {
			model.addParameterBindings(model.source[match[2]:match[3]], match[1]-1)
		}
	}
}

func (model *jsLexicalModel) addBinding(binding jsBinding) {
	key := fmt.Sprintf("%d:%d:%d:%s", binding.scope, binding.start, binding.end, binding.name)
	if model.bindingKeys[key] {
		return
	}
	model.bindingKeys[key] = true
	model.bindings = append(model.bindings, binding)
}

func (model *jsLexicalModel) buildBindingIndex() {
	for index, binding := range model.bindings {
		byName := model.bindingsByScopeName[binding.scope]
		if byName == nil {
			byName = map[string][]int{}
			model.bindingsByScopeName[binding.scope] = byName
		}
		byName[binding.name] = append(byName[binding.name], index)
	}
	for _, byName := range model.bindingsByScopeName {
		for name := range byName {
			indices := byName[name]
			sort.SliceStable(indices, func(left, right int) bool {
				return model.bindings[indices[left]].start < model.bindings[indices[right]].start
			})
			byName[name] = indices
		}
	}
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
	for _, match := range declarationRE.FindAllStringSubmatchIndex(model.code, -1) {
		if len(match) != 6 {
			continue
		}
		binding := jsBinding{name: model.source[match[2]:match[3]], kind: jsBindingOther, scope: model.scopeAt(match[0]), start: match[0]}
		if match[4] >= 0 {
			binding.createSource = model.source[match[4]:match[5]]
		}
		model.addBinding(binding)
	}
}

func (model *jsLexicalModel) addVariablePatternBindings() {
	for _, location := range scriptVariableBindingRE.FindAllStringIndex(model.code, -1) {
		statementStart := nextScriptNonSpace(model.code, location[1])
		statementEnd := scriptVariableStatementEnd(model.code, statementStart)
		if statementStart >= statementEnd {
			continue
		}
		for _, declarator := range splitScriptTopLevel(model.code[statementStart:statementEnd], ',') {
			pattern := declarator
			if equals := findScriptTopLevel(pattern, '='); equals >= 0 {
				pattern = pattern[:equals]
			}
			for name := range scriptBindingPatternNames(pattern) {
				model.addBinding(jsBinding{name: name, kind: jsBindingOther, scope: model.scopeAt(location[0]), start: location[0]})
			}
		}
	}
}

func (model jsLexicalModel) scopeAt(offset int) int {
	if offset < 0 || len(model.scopeAtOffsets) == 0 {
		return 0
	}
	if offset >= len(model.scopeAtOffsets) {
		offset = len(model.scopeAtOffsets) - 1
	}
	return model.scopeAtOffsets[offset]
}

func (model jsLexicalModel) scopeStartingAt(start int) int {
	if scope, ok := model.scopeByStart[start]; ok {
		return scope
	}
	return -1
}

func (model jsLexicalModel) resolveBinding(name string, offset int) (int, bool) {
	for scope := model.scopeAt(offset); scope >= 0; scope = model.scopes[scope].parent {
		indices := model.bindingsByScopeName[scope][name]
		candidate := sort.Search(len(indices), func(index int) bool {
			return model.bindings[indices[index]].start >= offset
		}) - 1
		for candidate >= 0 {
			binding := model.bindings[indices[candidate]]
			if binding.end == 0 || offset < binding.end {
				return indices[candidate], true
			}
			candidate--
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
	return buildJSAPIAnalysis(source).authForCall(callStart, callEnd, file)
}

// extractHTTPCallAuth returns authentication evidence statically associated with one HTTP call.
func extractHTTPCallAuth(source string, callStart, callEnd int) []AuthRecord {
	return buildJSAPIAnalysis(source).authForCall(callStart, callEnd, "")
}

func (analysis jsAPIAnalysis) authForCall(callStart, callEnd int, file string) []AuthRecord {
	if callStart < 0 || callEnd <= callStart || callStart >= len(analysis.source) {
		return nil
	}
	if callEnd > len(analysis.source) {
		callEnd = len(analysis.source)
	}
	masked := analysis.lexical.comments
	code := analysis.lexical.code
	call := masked[callStart:callEnd]
	line := analysis.lineForOffset(callStart)
	config, configStart := httpCallConfigExpressionCode(masked, code, callStart, callEnd)
	records := extractDirectHTTPCallAuth(config, configStart, line, "EXTRACTED", "http_call_config", analysis.model)
	receiver := httpCallReceiver(call)
	if receiver != "" {
		if binding, ok := analysis.model.resolveHTTPClient(receiver, callStart); ok {
			evidence := analysis.interceptorAuthByBinding[binding]
			beforeCall := sort.Search(len(evidence), func(index int) bool {
				return evidence[index].start >= callStart
			})
			if beforeCall > 0 {
				records = appendUniqueAuth(records, evidence[beforeCall-1].auth...)
			}
		}
	}
	for index := range records {
		records[index].File = file
	}
	return records
}

func httpCallConfigExpression(source string, callStart, callEnd int) (string, int) {
	return httpCallConfigExpressionCode(source, scanJSLexicalSource(source).code, callStart, callEnd)
}

func httpCallConfigExpressionCode(source, code string, callStart, callEnd int) (string, int) {
	args := topLevelCallArgumentsCode(source, code, callStart, callEnd)
	if len(args) == 0 {
		return "", callStart
	}
	prefixEnd := strings.IndexByte(code[callStart:callEnd], '(')
	if prefixEnd < 0 {
		return "", callStart
	}
	prefix := strings.TrimSpace(code[callStart : callStart+prefixEnd])
	configIndex := -1
	switch {
	case regexp.MustCompile(`^fetch$`).MatchString(prefix):
		configIndex = 1
	case regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*\s*\.\s*(?:get|delete)$`).MatchString(prefix):
		configIndex = 1
	case regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*\s*\.\s*(?:post|put|patch)$`).MatchString(prefix):
		configIndex = 2
	default:
		return source[callStart:callEnd], callStart
	}
	if configIndex < 0 || configIndex >= len(args) {
		return "", callStart
	}
	return source[args[configIndex].start:args[configIndex].end], args[configIndex].start
}

func (analysis *jsAPIAnalysis) indexRequestInterceptorAuth() {
	interceptorRE := regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\.interceptors\.request\.use\s*\(`)
	for _, match := range interceptorRE.FindAllStringSubmatchIndex(analysis.lexical.code, -1) {
		if len(match) != 4 {
			continue
		}
		binding, ok := analysis.model.resolveHTTPClient(analysis.source[match[2]:match[3]], match[0])
		if !ok {
			continue
		}
		end := matchingCallEndCode(analysis.lexical.code, match[0])
		if end <= match[0] {
			continue
		}
		expression := analysis.source[match[0]:end]
		auth := extractDirectHTTPCallAuth(
			expression,
			match[0],
			analysis.lineForOffset(match[0]),
			"PARTIAL",
			"http_client_interceptor",
			analysis.model,
		)
		if len(auth) == 0 {
			continue
		}
		evidence := analysis.interceptorAuthByBinding[binding]
		if len(evidence) > 0 {
			auth = appendUniqueAuth(append([]AuthRecord(nil), evidence[len(evidence)-1].auth...), auth...)
		}
		analysis.interceptorAuthByBinding[binding] = append(evidence, interceptorAuthEvidence{start: match[0], auth: auth})
	}
}

func httpCallReceiver(call string) string {
	match := regexp.MustCompile(`^\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\.\s*(?:get|post|put|patch|delete)\s*\(`).FindStringSubmatch(call)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func extractDirectHTTPCallAuth(expression string, expressionStart, line int, confidence, source string, model jsLexicalModel) []AuthRecord {
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
	if source == "http_client_interceptor" {
		configName, configBinding, ok := interceptorConfigBinding(expression, expressionStart, model)
		if ok {
			authorizationAssignments := resolvedInterceptorAssignmentValues(
				expression,
				expressionStart,
				configName,
				configBinding,
				model,
				func(name string) bool { return strings.EqualFold(name, "authorization") },
			)
			for _, value := range authorizationAssignments {
				oauthCredentialValues = append(oauthCredentialValues, value)
				lower := strings.ToLower(value)
				if strings.Contains(lower, "bearer") {
					records = appendUniqueAuth(records, newHTTPAuthRecord("bearer", sanitizedCredentialExpression(value), source, confidence, line))
				} else if strings.Contains(lower, "basic") {
					records = appendUniqueAuth(records, newHTTPAuthRecord("basic", sanitizedCredentialExpression(value), source, confidence, line))
				}
			}
			apiKeyAssignments := resolvedInterceptorAssignmentValues(
				expression,
				expressionStart,
				configName,
				configBinding,
				model,
				isAPIKeyPropertyName,
			)
			for _, value := range apiKeyAssignments {
				records = appendUniqueAuth(records, newHTTPAuthRecord("api_key", sanitizedCredentialExpression(value), source, confidence, line))
			}
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
	for _, helper := range model.oauthHelperNames() {
		if !oauthHelperUsesImportedBinding(model, expression, expressionStart, helper, oauthCredentialValues) {
			continue
		}
		records = appendUniqueAuth(records, AuthRecord{Kind: "oauth2", Expression: helper, Source: oauthRecordSource(source), Confidence: confidence, Line: line})
	}
	return records
}

func interceptorConfigBinding(expression string, expressionStart int, model jsLexicalModel) (string, int, bool) {
	lexical := scanJSLexicalSource(expression)
	args := topLevelCallArgumentsCode(expression, lexical.code, 0, len(expression))
	if len(args) == 0 {
		return "", -1, false
	}
	arrow := topLevelJSArrow(lexical.code, args[0].start, args[0].end)
	if arrow < 0 {
		return "", -1, false
	}
	paramsStart, paramsEnd, ok := jsArrowParameterSpan(lexical.code, arrow)
	if !ok {
		return "", -1, false
	}
	names := jsParameterBindingNames(expression[paramsStart:paramsEnd])
	if len(names) != 1 {
		return "", -1, false
	}
	bodyStart := nextScriptNonSpace(lexical.code, arrow+2)
	if bodyStart >= len(lexical.code) || lexical.code[bodyStart] != '{' {
		return "", -1, false
	}
	name := names[0]
	binding, ok := model.resolveBinding(name, expressionStart+bodyStart+1)
	return name, binding, ok
}

func topLevelJSArrow(code string, start, end int) int {
	round, square, curly := 0, 0, 0
	for index := start; index+1 < end; index++ {
		if code[index] == '=' && code[index+1] == '>' && round == 0 && square == 0 && curly == 0 {
			return index
		}
		switch code[index] {
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		}
	}
	return -1
}

func resolvedInterceptorAssignmentValues(
	expression string,
	expressionStart int,
	configName string,
	configBinding int,
	model jsLexicalModel,
	propertyMatches func(string) bool,
) []string {
	lexical := scanJSLexicalSource(expression)
	headerRE := regexp.MustCompile(`\b` + regexp.QuoteMeta(configName) + `\s*\.\s*headers\b`)
	var values []string
	for _, match := range headerRE.FindAllStringIndex(lexical.code, -1) {
		binding, ok := model.resolveBinding(configName, expressionStart+match[0])
		if !ok || binding != configBinding {
			continue
		}
		property, propertyEnd, ok := jsMemberProperty(expression, lexical.code, match[1])
		if !ok || !propertyMatches(property) {
			continue
		}
		assignment := nextScriptNonSpace(lexical.code, propertyEnd)
		if assignment >= len(lexical.code) || lexical.code[assignment] != '=' ||
			(assignment+1 < len(lexical.code) && (lexical.code[assignment+1] == '=' || lexical.code[assignment+1] == '>')) {
			continue
		}
		valueStart := assignment + 1
		for valueStart < len(expression) && isJSSpace(expression[valueStart]) {
			valueStart++
		}
		valueEnd := jsAssignmentValueEnd(lexical.code, valueStart)
		for valueEnd > valueStart && isJSSpace(expression[valueEnd-1]) {
			valueEnd--
		}
		if valueStart < valueEnd {
			values = append(values, expression[valueStart:valueEnd])
		}
	}
	return values
}

func jsMemberProperty(source, code string, start int) (string, int, bool) {
	start = nextScriptNonSpace(code, start)
	if start >= len(code) {
		return "", start, false
	}
	if code[start] == '.' {
		nameStart := nextScriptNonSpace(code, start+1)
		nameEnd := nameStart
		for nameEnd < len(code) && isScriptIdentifierByte(code[nameEnd]) {
			nameEnd++
		}
		if nameEnd == nameStart {
			return "", start, false
		}
		return source[nameStart:nameEnd], nameEnd, true
	}
	if code[start] != '[' {
		return "", start, false
	}
	close := matchingScriptDelimiter(code, start, '[', ']')
	if close < 0 {
		return "", start, false
	}
	name := strings.TrimSpace(source[start+1 : close])
	if len(name) < 2 || (name[0] != '\'' && name[0] != '"') || name[len(name)-1] != name[0] {
		return "", start, false
	}
	return name[1 : len(name)-1], close + 1, true
}

func jsAssignmentValueEnd(code string, start int) int {
	round, square, curly := 0, 0, 0
	for index := start; index < len(code); index++ {
		switch code[index] {
		case '(':
			round++
		case ')':
			if round == 0 && square == 0 && curly == 0 {
				return index
			}
			round--
		case '[':
			square++
		case ']':
			if square > 0 {
				square--
			}
		case '{':
			curly++
		case '}':
			if curly == 0 && round == 0 && square == 0 {
				return index
			}
			curly--
		case ',', ';', '\n':
			if round == 0 && square == 0 && curly == 0 {
				return index
			}
		}
	}
	return len(code)
}

func (model jsLexicalModel) oauthHelperNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, binding := range model.bindings {
		if binding.kind == jsBindingOAuthImport && !seen[binding.name] {
			seen[binding.name] = true
			names = append(names, binding.name)
		}
	}
	sort.Strings(names)
	return names
}

func oauthHelperUsesImportedBinding(model jsLexicalModel, expression string, expressionStart int, helper string, credentialValues []string) bool {
	code := scanJSLexicalSource(expression).code
	helperRE := regexp.MustCompile(`\b` + regexp.QuoteMeta(helper) + `\s*\(`)
	for _, value := range credentialValues {
		for search := 0; search <= len(expression)-len(value); {
			relative := strings.Index(expression[search:], value)
			if relative < 0 {
				break
			}
			valueOffset := search + relative
			for _, match := range helperRE.FindAllStringIndex(code[valueOffset:valueOffset+len(value)], -1) {
				binding, ok := model.resolveBinding(helper, expressionStart+valueOffset+match[0])
				if ok && model.bindings[binding].kind == jsBindingOAuthImport {
					return true
				}
			}
			search = valueOffset + len(value)
		}
	}
	return false
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
	code := scanJSLexicalSource(source).code
	parenDepth, braceDepth, bracketDepth := 0, 0, 0
	for i := start; i < end; i++ {
		switch code[i] {
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
	code := scanJSLexicalSource(source).code
	depth := 0
	for i := open; i < len(source); i++ {
		switch code[i] {
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
	return firstCredentialIdentifier(scanJSLexicalSource(value).code)
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

func supportedOAuthHelperNames() map[string]bool {
	return map[string]bool{
		"getAccessToken": true, "getAccessTokenSilently": true, "getTokenSilently": true, "acquireTokenSilent": true,
	}
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
