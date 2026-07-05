package scan

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	codeGoFuncRE              = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	codePHPClassRE            = regexp.MustCompile(`^\s*(?:abstract\s+|final\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	codePHPFuncRE             = regexp.MustCompile(`^\s*(?:public\s+|protected\s+|private\s+|static\s+|final\s+|abstract\s+)*function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	codeScriptFuncRE          = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	codeScriptArrowRE         = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s*)?\(?[^=]*?\)?\s*=>`)
	codeScriptMethodRE        = regexp.MustCompile(`^\s*(?:async\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*\([^)]*\)\s*\{`)
	codeScriptTestRE          = regexp.MustCompile(`^\s*(?:it|test)\s*\(\s*["']([^"']+)["']`)
	codePythonClassRE         = regexp.MustCompile(`^(\s*)class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	codePythonFuncRE          = regexp.MustCompile(`^(\s*)def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	codeShellFuncRE           = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(\))\s*\{`)
	codeGoHTTPRouteRE         = regexp.MustCompile(`\b(?:http\.)?HandleFunc\s*\(\s*["']([^"']+)["']\s*,\s*([A-Za-z_][A-Za-z0-9_]*)`)
	codeGoRouterRouteRE       = regexp.MustCompile(`\.\s*(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD)\s*\(\s*["']([^"']+)["']\s*,\s*([A-Za-z_][A-Za-z0-9_]*)`)
	codePHPRouteRE            = regexp.MustCompile(`Route::(get|post|put|delete|patch|options|any)\s*\(\s*['"]([^'"]+)['"]\s*,\s*\[?\s*([A-Za-z_][A-Za-z0-9_]*)::class\s*,\s*['"]([^'"]+)['"]`)
	codeScriptRouteRE         = regexp.MustCompile(`\b(?:app|router|server|fastify)\s*\.\s*(get|post|put|delete|patch|options|head)\s*\(\s*["']([^"']+)["']\s*,\s*([A-Za-z_$][A-Za-z0-9_$]*)`)
	codeReactJSXRouteRE       = regexp.MustCompile(`<Route\b[^>]*\bpath=["']([^"']+)["'][^>]*\belement=\{\s*<([A-Za-z_$][A-Za-z0-9_$]*)`)
	codeReactComponentRouteRE = regexp.MustCompile(`<Route\b[^>]*\bpath=["']([^"']+)["'][^>]*\bcomponent=\{?\s*([A-Za-z_$][A-Za-z0-9_$]*)`)
	codeReduxFragmentRouteRE  = regexp.MustCompile(`<Fragment\b[^>]*\bforRoute=["']([^"']+)["']`)
	codeReactObjectRouteRE    = regexp.MustCompile(`\bpath\s*:\s*["']([^"']+)["'][^,\n]*,\s*element\s*:\s*<([A-Za-z_$][A-Za-z0-9_$]*)`)
	codePythonRouteRE         = regexp.MustCompile(`^\s*@([A-Za-z_][A-Za-z0-9_]*)\.(get|post|put|delete|patch|options|head|route)\s*\(\s*["']([^"']+)["']`)
	codeCallRE                = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	codeMemberCallRE          = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\s*(?:\.|->|::)\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
)

func mergeCodeIntelligence(target *CodeIntelligenceRecord, next CodeIntelligenceRecord) {
	target.Functions = append(target.Functions, next.Functions...)
	target.Routes = append(target.Routes, next.Routes...)
}

func extractCodeIntelligence(file FileRecord, body string) CodeIntelligenceRecord {
	switch file.Language {
	case "go", "php", "javascript", "typescript", "python", "shell":
	default:
		return CodeIntelligenceRecord{}
	}

	lines := strings.Split(body, "\n")
	record := CodeIntelligenceRecord{
		Functions: extractCodeFunctions(file, lines),
		Routes:    extractCodeRoutes(file, lines),
	}
	for i := range record.Functions {
		record.Functions[i].Calls = extractCallsForFunction(file.Language, lines, record.Functions[i])
	}
	sortCodeIntelligence(&record)
	return record
}

func extractCodeFunctions(file FileRecord, lines []string) []CodeFunctionRecord {
	switch file.Language {
	case "go":
		return extractGoCodeFunctions(file, lines)
	case "php":
		return extractPHPCodeFunctions(file, lines)
	case "javascript", "typescript":
		return extractScriptCodeFunctions(file, lines)
	case "python":
		return extractPythonCodeFunctions(file, lines)
	case "shell":
		return extractShellCodeFunctions(file, lines)
	default:
		return nil
	}
}

func extractGoCodeFunctions(file FileRecord, lines []string) []CodeFunctionRecord {
	var functions []CodeFunctionRecord
	for i, line := range lines {
		if match := codeGoFuncRE.FindStringSubmatch(line); len(match) == 2 {
			lineNo := i + 1
			kind := "function"
			if strings.HasSuffix(file.Path, "_test.go") && strings.HasPrefix(match[1], "Test") {
				kind = "test"
			}
			functions = append(functions, CodeFunctionRecord{Name: match[1], Kind: kind, Language: file.Language, File: file.Path, Line: lineNo, EndLine: findBraceBlockEnd(lines, i)})
		}
	}
	return functions
}

func extractPHPCodeFunctions(file FileRecord, lines []string) []CodeFunctionRecord {
	var functions []CodeFunctionRecord
	currentClass := ""
	classDepth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if match := codePHPClassRE.FindStringSubmatch(trimmed); len(match) == 2 {
			currentClass = match[1]
			classDepth = strings.Count(line, "{") - strings.Count(line, "}")
		} else if currentClass != "" {
			classDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if classDepth <= 0 && strings.Contains(line, "}") {
				currentClass = ""
			}
		}
		if match := codePHPFuncRE.FindStringSubmatch(trimmed); len(match) == 2 {
			lineNo := i + 1
			kind := "function"
			owner := ""
			if currentClass != "" {
				kind = "method"
				owner = currentClass
			}
			if strings.HasSuffix(match[1], "Test") || strings.HasPrefix(match[1], "test") || strings.Contains(strings.ToLower(file.Path), "test") {
				kind = "test"
			}
			functions = append(functions, CodeFunctionRecord{Name: match[1], Owner: owner, Kind: kind, Language: file.Language, File: file.Path, Line: lineNo, EndLine: findBraceBlockEnd(lines, i)})
		}
	}
	return functions
}

func extractScriptCodeFunctions(file FileRecord, lines []string) []CodeFunctionRecord {
	var functions []CodeFunctionRecord
	for i, line := range lines {
		lineNo := i + 1
		kind := "function"
		if strings.Contains(strings.ToLower(file.Path), ".test.") || strings.Contains(strings.ToLower(file.Path), ".spec.") {
			kind = "test"
		}
		if match := codeScriptTestRE.FindStringSubmatch(line); len(match) == 2 {
			functions = append(functions, CodeFunctionRecord{Name: match[1], Kind: "test", Language: file.Language, File: file.Path, Line: lineNo, EndLine: findBraceBlockEnd(lines, i)})
			continue
		}
		if match := codeScriptFuncRE.FindStringSubmatch(line); len(match) == 2 {
			if isLikelyReactComponent(match[1], file.Path) {
				kind = "component"
			}
			functions = append(functions, CodeFunctionRecord{Name: match[1], Kind: kind, Language: file.Language, File: file.Path, Line: lineNo, EndLine: findBraceBlockEnd(lines, i)})
			continue
		}
		if match := codeScriptArrowRE.FindStringSubmatch(line); len(match) == 2 {
			if isLikelyReactComponent(match[1], file.Path) {
				kind = "component"
			}
			functions = append(functions, CodeFunctionRecord{Name: match[1], Kind: kind, Language: file.Language, File: file.Path, Line: lineNo, EndLine: findBraceBlockEnd(lines, i)})
			continue
		}
		if match := codeScriptMethodRE.FindStringSubmatch(line); len(match) == 2 && !isCodeKeyword(match[1]) {
			functions = append(functions, CodeFunctionRecord{Name: match[1], Kind: "method", Language: file.Language, File: file.Path, Line: lineNo, EndLine: findBraceBlockEnd(lines, i)})
		}
	}
	return functions
}

func extractPythonCodeFunctions(file FileRecord, lines []string) []CodeFunctionRecord {
	var functions []CodeFunctionRecord
	currentClass := ""
	classIndent := -1
	for i, line := range lines {
		if match := codePythonClassRE.FindStringSubmatch(line); len(match) == 3 {
			currentClass = match[2]
			classIndent = len(match[1])
			continue
		}
		if match := codePythonFuncRE.FindStringSubmatch(line); len(match) == 3 {
			indent := len(match[1])
			owner := ""
			kind := "function"
			if currentClass != "" && indent > classIndent {
				owner = currentClass
				kind = "method"
			}
			if strings.HasPrefix(match[2], "test_") || strings.Contains(strings.ToLower(file.Path), "test") {
				kind = "test"
			}
			functions = append(functions, CodeFunctionRecord{Name: match[2], Owner: owner, Kind: kind, Language: file.Language, File: file.Path, Line: i + 1, EndLine: findPythonBlockEnd(lines, i, indent)})
		}
	}
	return functions
}

func extractShellCodeFunctions(file FileRecord, lines []string) []CodeFunctionRecord {
	var functions []CodeFunctionRecord
	for i, line := range lines {
		if match := codeShellFuncRE.FindStringSubmatch(line); len(match) == 2 {
			functions = append(functions, CodeFunctionRecord{Name: match[1], Kind: "function", Language: file.Language, File: file.Path, Line: i + 1, EndLine: findBraceBlockEnd(lines, i)})
		}
	}
	return functions
}

func extractCodeRoutes(file FileRecord, lines []string) []CodeRouteRecord {
	var routes []CodeRouteRecord
	var pendingPythonRoute *CodeRouteRecord
	for i, line := range lines {
		lineNo := i + 1
		if isRouteCommentLine(file.Language, line) {
			continue
		}
		switch file.Language {
		case "go":
			if match := codeGoHTTPRouteRE.FindStringSubmatch(line); len(match) == 3 {
				routes = append(routes, codeRoute(file, "net/http", "backend", "GET", match[1], match[2], lineNo))
			}
			if match := codeGoRouterRouteRE.FindStringSubmatch(line); len(match) == 4 {
				routes = append(routes, codeRoute(file, "Go Router", "backend", strings.ToUpper(match[1]), match[2], match[3], lineNo))
			}
		case "php":
			if match := codePHPRouteRE.FindStringSubmatch(line); len(match) == 5 {
				routes = append(routes, codeRoute(file, "Laravel", "backend", strings.ToUpper(match[1]), match[2], match[3]+"."+match[4], lineNo))
			}
		case "javascript", "typescript":
			if match := codeScriptRouteRE.FindStringSubmatch(line); len(match) == 4 {
				routes = append(routes, codeRoute(file, scriptRouteFramework(line), "backend", strings.ToUpper(match[1]), match[2], match[3], lineNo))
			}
			if match := codeReactJSXRouteRE.FindStringSubmatch(line); len(match) == 3 {
				routes = append(routes, codeRoute(file, "React Router", "frontend", "ROUTE", match[1], match[2], lineNo))
			}
			if match := codeReactComponentRouteRE.FindStringSubmatch(line); len(match) == 3 {
				routes = append(routes, codeRoute(file, "React Router", "frontend", "ROUTE", match[1], match[2], lineNo))
			}
			if match := codeReduxFragmentRouteRE.FindStringSubmatch(line); len(match) == 2 {
				routes = append(routes, codeRoute(file, "Redux Little Router", "frontend", "ROUTE", match[1], "Fragment", lineNo))
			}
			if match := codeReactObjectRouteRE.FindStringSubmatch(line); len(match) == 3 {
				routes = append(routes, codeRoute(file, "React Router", "frontend", "ROUTE", match[1], match[2], lineNo))
			}
		case "python":
			if match := codePythonRouteRE.FindStringSubmatch(line); len(match) == 4 {
				framework := "FastAPI"
				if match[2] == "route" {
					framework = "Flask"
				}
				route := codeRoute(file, framework, "backend", strings.ToUpper(match[2]), match[3], "", lineNo)
				pendingPythonRoute = &route
				continue
			}
			if pendingPythonRoute != nil {
				if match := codePythonFuncRE.FindStringSubmatch(line); len(match) == 3 {
					pendingPythonRoute.Handler = match[2]
					routes = append(routes, *pendingPythonRoute)
					pendingPythonRoute = nil
				}
			}
		}
	}
	return routes
}

func isRouteCommentLine(language, line string) bool {
	trimmed := strings.TrimSpace(line)
	switch language {
	case "python", "shell":
		return strings.HasPrefix(trimmed, "#")
	default:
		return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*")
	}
}

func codeRoute(file FileRecord, framework, kind, method, path, handler string, line int) CodeRouteRecord {
	return CodeRouteRecord{
		Language:   file.Language,
		Framework:  framework,
		Kind:       kind,
		HTTPMethod: strings.ToUpper(method),
		Path:       normalizeCodeRoutePath(path),
		Handler:    handler,
		File:       file.Path,
		Line:       line,
		Confidence: "INFERRED",
	}
}

func extractCallsForFunction(language string, lines []string, function CodeFunctionRecord) []CodeCallRecord {
	start := function.Line - 1
	end := function.EndLine
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	seen := map[string]bool{}
	var calls []CodeCallRecord
	for i := start; i < end; i++ {
		line := stripCodeLineComment(language, lines[i])
		if language == "shell" {
			if call, ok := extractShellBareCall(line, i+1); ok && call.Method != function.Name {
				key := call.Method + "@" + strings.TrimSpace(line)
				if !seen[key] {
					seen[key] = true
					calls = append(calls, call)
				}
			}
		}
		for _, match := range codeMemberCallRE.FindAllStringSubmatch(line, -1) {
			if len(match) != 3 || isCodeKeyword(match[2]) {
				continue
			}
			key := match[1] + "." + match[2] + "@" + strings.TrimSpace(line)
			if seen[key] {
				continue
			}
			seen[key] = true
			calls = append(calls, CodeCallRecord{Receiver: match[1], Method: match[2], Raw: strings.TrimSpace(line), Line: i + 1})
		}
		for _, match := range codeCallRE.FindAllStringSubmatch(line, -1) {
			if len(match) != 2 || isCodeKeyword(match[1]) || match[1] == function.Name {
				continue
			}
			if strings.Contains(line, "function "+match[1]) || strings.Contains(line, "def "+match[1]) || strings.Contains(line, "func "+match[1]) {
				continue
			}
			key := match[1] + "@" + strings.TrimSpace(line)
			if seen[key] {
				continue
			}
			seen[key] = true
			calls = append(calls, CodeCallRecord{Method: match[1], Raw: strings.TrimSpace(line), Line: i + 1})
		}
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Line != calls[j].Line {
			return calls[i].Line < calls[j].Line
		}
		return calls[i].Method < calls[j].Method
	})
	return calls
}

func extractShellBareCall(line string, lineNo int) (CodeCallRecord, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.Contains(trimmed, "()") || strings.HasSuffix(trimmed, "{") {
		return CodeCallRecord{}, false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 || isCodeKeyword(fields[0]) {
		return CodeCallRecord{}, false
	}
	if strings.ContainsAny(fields[0], "$=|&;<>") {
		return CodeCallRecord{}, false
	}
	return CodeCallRecord{Method: fields[0], Raw: trimmed, Line: lineNo}, true
}

func findBraceBlockEnd(lines []string, start int) int {
	depth := 0
	seenOpen := false
	for i := start; i < len(lines); i++ {
		depth += strings.Count(lines[i], "{")
		if strings.Contains(lines[i], "{") {
			seenOpen = true
		}
		depth -= strings.Count(lines[i], "}")
		if seenOpen && depth <= 0 {
			return i + 1
		}
	}
	return start + 1
}

func findPythonBlockEnd(lines []string, start, indent int) int {
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		nextIndent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
		if nextIndent <= indent && !strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
			return i
		}
	}
	return len(lines)
}

func isLikelyReactComponent(name, path string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	return first >= 'A' && first <= 'Z' && (strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".jsx") || strings.Contains(path, "components/") || strings.Contains(path, "pages/"))
}

func scriptRouteFramework(line string) string {
	if strings.Contains(line, "fastify") {
		return "Fastify"
	}
	return "Express"
}

func normalizeCodeRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func stripCodeLineComment(language, line string) string {
	switch language {
	case "python", "shell":
		if index := strings.Index(line, "#"); index >= 0 {
			return line[:index]
		}
	default:
		if index := strings.Index(line, "//"); index >= 0 {
			return line[:index]
		}
	}
	return line
}

func isCodeKeyword(value string) bool {
	switch value {
	case "", "if", "for", "while", "switch", "return", "func", "function", "def", "class", "new", "echo", "print", "println", "String", "Integer", "Boolean", "Number", "Array", "Object", "Promise", "fetch", "test", "it", "describe", "expect", "assert", "require", "include", "include_once", "require_once", "source":
		return true
	default:
		return false
	}
}

func sortCodeIntelligence(record *CodeIntelligenceRecord) {
	sort.Slice(record.Functions, func(i, j int) bool {
		if record.Functions[i].File != record.Functions[j].File {
			return record.Functions[i].File < record.Functions[j].File
		}
		if record.Functions[i].Line != record.Functions[j].Line {
			return record.Functions[i].Line < record.Functions[j].Line
		}
		return record.Functions[i].Name < record.Functions[j].Name
	})
	sort.Slice(record.Routes, func(i, j int) bool {
		if record.Routes[i].File != record.Routes[j].File {
			return record.Routes[i].File < record.Routes[j].File
		}
		if record.Routes[i].Line != record.Routes[j].Line {
			return record.Routes[i].Line < record.Routes[j].Line
		}
		return record.Routes[i].Path < record.Routes[j].Path
	})
}

func sourceBaseName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
