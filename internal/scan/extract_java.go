package scan

import (
	"regexp"
	"sort"
	"strings"
)

var (
	javaPackageLineRE    = regexp.MustCompile(`^\s*package\s+([A-Za-z_][A-Za-z0-9_.]*);`)
	javaImportLineRE     = regexp.MustCompile(`^\s*import\s+(static\s+)?([^;]+);`)
	javaTypeLineRE       = regexp.MustCompile(`^\s*(?:public|protected|private|abstract|final|sealed|non-sealed|static|\s)*\s*(class|interface|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)\b(.*)$`)
	javaMethodLineRE     = regexp.MustCompile(`^\s*(public|protected|private)?\s*(?:static\s+)?(?:final\s+)?([A-Za-z_][A-Za-z0-9_$<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)\s*(?:throws\s+[^{]+)?\{?\s*$`)
	javaFieldLineRE      = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(final\s+)?([A-Za-z_][A-Za-z0-9_$<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=.*)?;\s*$`)
	javaAnnotationLineRE = regexp.MustCompile(`^\s*@([A-Za-z_][A-Za-z0-9_.]*)(?:\((.*)\))?\s*$`)
	javaConstantLineRE   = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*static\s+final\s+String\s+([A-Za-z0-9_]+)\s*=\s*"([^"]*)"\s*;`)
	javaCallRE           = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaNewCallRE        = regexp.MustCompile(`\bnew\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaBareCallRE       = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaHTTPCallRE       = regexp.MustCompile(`\b(get|post|put|delete|patch)\s*\(([^)]*)\)`)
	javaHTTPBuilderRefRE = regexp.MustCompile(`MockMvcRequestBuilders::(get|post|put|delete|patch)\s*,\s*(.+?)(?:,\s*[^)]*)?\)`)
	javaHTTPChainVerbRE  = regexp.MustCompile(`^\s*\.(get|post|put|delete|patch)\s*\(\s*\)\s*$`)
	javaHTTPChainURIRE   = regexp.MustCompile(`^\s*\.uri\s*\((.+)\)\s*$`)
	javaStringLiteralRE  = regexp.MustCompile(`"([^"]*)"`)
	javaStringVarLineRE  = regexp.MustCompile(`^\s*(?:final\s+)?String\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+);\s*$`)
	javaSecurityCallRE   = regexp.MustCompile(`\.(hasRole|hasAuthority|hasAnyRole|hasAnyAuthority|authenticated)\s*\(([^)]*)\)`)
)

func extractJavaSource(file FileRecord, body string) JavaSourceRecord {
	source := JavaSourceRecord{File: file.Path, Constants: map[string]string{}}
	lines := strings.Split(body, "\n")
	var pending []JavaAnnotationRecord
	currentOwner := ""
	braceDepth := 0
	typeStack := []javaTypeScope{}
	blockComment := false
	methodSignature := ""
	methodSignatureLine := 0
	annotationSignature := ""
	annotationSignatureLine := 0

	for index, raw := range lines {
		lineNo := index + 1
		line, inBlock := stripJavaLineNoise(raw, blockComment)
		blockComment = inBlock
		if strings.TrimSpace(line) == "" {
			continue
		}
		if annotationSignature != "" {
			if isAnnotationBoundary(strings.TrimSpace(line)) {
				if annotation, ok := parseJavaAnnotationLine(annotationSignature, annotationSignatureLine); ok {
					pending = append(pending, annotation)
					source.Annotations = append(source.Annotations, annotation)
				}
				annotationSignature = ""
				annotationSignatureLine = 0
			} else {
				annotationSignature += " " + strings.TrimSpace(line)
				if balancedJavaParens(annotationSignature) {
					if annotation, ok := parseJavaAnnotationLine(annotationSignature, annotationSignatureLine); ok {
						pending = append(pending, annotation)
						source.Annotations = append(source.Annotations, annotation)
					}
					annotationSignature = ""
					annotationSignatureLine = 0
				}
				continue
			}
		}
		if methodSignature != "" {
			methodSignature += " " + strings.TrimSpace(line)
			if strings.Contains(line, "{") {
				if method, ok := parseJavaMethod(methodSignature, file.Path, currentOwner, methodSignatureLine, pending); ok {
					source.Methods = append(source.Methods, method)
					pending = nil
				}
				methodSignature = ""
				methodSignatureLine = 0
			}
			braceDepth += strings.Count(line, "{")
			braceDepth -= strings.Count(line, "}")
			continue
		}

		if match := javaPackageLineRE.FindStringSubmatch(line); len(match) == 2 {
			source.Package = match[1]
			continue
		}
		if match := javaImportLineRE.FindStringSubmatch(line); len(match) == 3 {
			source.Imports = append(source.Imports, JavaImportRecord{Name: strings.TrimSpace(match[2]), Static: strings.TrimSpace(match[1]) == "static", Line: lineNo})
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "@") && strings.Contains(line, "(") && !balancedJavaParens(line) {
			annotationSignature = strings.TrimSpace(line)
			annotationSignatureLine = lineNo
			continue
		}
		if annotation, ok := parseJavaAnnotationLine(line, lineNo); ok {
			pending = append(pending, annotation)
			source.Annotations = append(source.Annotations, annotation)
			continue
		}
		if match := javaConstantLineRE.FindStringSubmatch(line); len(match) == 3 {
			source.Constants[match[1]] = match[2]
		}
		if match := javaTypeLineRE.FindStringSubmatch(line); len(match) == 4 {
			owner := ""
			if len(typeStack) > 0 {
				owner = source.Types[typeStack[len(typeStack)-1].typeIndex].QualifiedName
			}
			typ := JavaTypeRecord{
				Name:          match[2],
				Kind:          match[1],
				Package:       source.Package,
				File:          file.Path,
				Line:          lineNo,
				Owner:         owner,
				QualifiedName: qualifiedJavaTypeName(source.Package, owner, match[2]),
				Annotations:   pending,
			}
			typ.Extends = parseJavaExtends(match[3])
			typ.Implements = parseJavaImplements(match[3])
			source.Types = append(source.Types, typ)
			currentOwner = typ.Name
			openCount := strings.Count(line, "{")
			if openCount > 0 {
				typeStack = append(typeStack, javaTypeScope{typeIndex: len(source.Types) - 1, bodyDepth: braceDepth + openCount})
			}
			pending = nil
		} else if match := javaFieldLineRE.FindStringSubmatch(line); len(match) == 4 && currentOwner != "" && javaAtCurrentTypeBody(braceDepth, typeStack) && !strings.Contains(line, "(") {
			source.Fields = append(source.Fields, JavaFieldRecord{
				Name:        match[3],
				Type:        cleanJavaType(match[2]),
				File:        file.Path,
				Line:        lineNo,
				Owner:       currentOwner,
				Final:       strings.TrimSpace(match[1]) == "final",
				Annotations: pending,
			})
			pending = nil
		} else if currentOwner != "" && looksLikeJavaMethodStart(line) && !strings.Contains(line, "{") {
			methodSignature = strings.TrimSpace(line)
			methodSignatureLine = lineNo
		} else if method, ok := parseJavaMethod(line, file.Path, currentOwner, lineNo, pending); ok && currentOwner != "" {
			source.Methods = append(source.Methods, method)
			pending = nil
		} else if len(source.Methods) > 0 {
			last := &source.Methods[len(source.Methods)-1]
			last.StringVars = mergeJavaStringVars(last.StringVars, extractJavaStringVars(line))
			last.Calls = append(last.Calls, extractJavaCalls(line, lineNo)...)
			last.Auth = append(last.Auth, extractJavaSecurityAuth(line, lineNo, file.Path)...)
			requests, pending := extractJavaHTTPRequestsWithPending(line, lineNo, last.StringVars, last.PendingHTTP)
			last.PendingHTTP = pending
			last.HTTPRequests = append(last.HTTPRequests, requests...)
		}

		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")
		for len(typeStack) > 0 && braceDepth < typeStack[len(typeStack)-1].bodyDepth {
			scope := typeStack[len(typeStack)-1]
			source.Types[scope.typeIndex].EndLine = lineNo
			typeStack = typeStack[:len(typeStack)-1]
		}
		if len(typeStack) > 0 {
			currentOwner = source.Types[typeStack[len(typeStack)-1].typeIndex].Name
		} else {
			currentOwner = ""
		}
		if braceDepth <= 0 {
			braceDepth = 0
		}
	}
	for len(typeStack) > 0 {
		scope := typeStack[len(typeStack)-1]
		source.Types[scope.typeIndex].EndLine = len(lines)
		typeStack = typeStack[:len(typeStack)-1]
	}

	if len(source.Constants) == 0 {
		source.Constants = nil
	}
	return source
}

type javaTypeScope struct {
	typeIndex int
	bodyDepth int
}

func javaAtCurrentTypeBody(braceDepth int, stack []javaTypeScope) bool {
	return len(stack) > 0 && braceDepth == stack[len(stack)-1].bodyDepth
}

func extractJavaSecurityAuth(line string, lineNo int, file string) []AuthRecord {
	var records []AuthRecord
	for _, match := range javaSecurityCallRE.FindAllStringSubmatch(line, -1) {
		if len(match) != 3 {
			continue
		}
		records = append(records, AuthRecord{
			Kind:       toSnake(match[1]),
			Expression: strings.TrimSpace(match[2]),
			Source:     "security_config_call",
			Confidence: "EXTRACTED",
			File:       file,
			Line:       lineNo,
		})
	}
	return records
}

func parseJavaAnnotationLine(line string, lineNo int) (JavaAnnotationRecord, bool) {
	match := javaAnnotationLineRE.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) != 3 {
		return JavaAnnotationRecord{}, false
	}
	return JavaAnnotationRecord{Name: shortJavaName(match[1]), Arguments: strings.TrimSpace(match[2]), Attributes: parseJavaAnnotationAttributes(match[2]), Line: lineNo}, true
}

func isAnnotationBoundary(line string) bool {
	for _, prefix := range []string{"@GetMapping", "@PostMapping", "@PutMapping", "@DeleteMapping", "@PatchMapping", "@RequestMapping", "@Override", "@Test"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func balancedJavaParens(line string) bool {
	depth := 0
	inString := false
	escaped := false
	for _, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		}
	}
	return depth == 0
}

func parseJavaMethod(line, file, owner string, lineNo int, annotations []JavaAnnotationRecord) (JavaMethodRecord, bool) {
	if owner == "" || isJavaControlLine(line) {
		return JavaMethodRecord{}, false
	}
	if params, visibility, ok := parseJavaConstructorSignature(line, owner); ok {
		return JavaMethodRecord{
			Name:        owner,
			File:        file,
			Line:        lineNo,
			Owner:       owner,
			Visibility:  visibility,
			Parameters:  parseJavaParameters(params),
			Annotations: annotations,
			Calls:       extractJavaCalls(line, lineNo),
		}, true
	}
	if name, returnType, params, visibility, ok := parseJavaMethodSignature(line); ok {
		return JavaMethodRecord{
			Name:         name,
			File:         file,
			Line:         lineNo,
			Owner:        owner,
			Visibility:   visibility,
			ReturnType:   returnType,
			Parameters:   parseJavaParameters(params),
			Annotations:  annotations,
			Calls:        extractJavaCalls(line, lineNo),
			HTTPRequests: extractJavaHTTPRequestsWithVars(line, lineNo, nil),
		}, true
	}
	match := javaMethodLineRE.FindStringSubmatch(line)
	if len(match) == 5 {
		return JavaMethodRecord{
			Name:         match[3],
			File:         file,
			Line:         lineNo,
			Owner:        owner,
			Visibility:   strings.TrimSpace(match[1]),
			ReturnType:   cleanJavaType(match[2]),
			Parameters:   parseJavaParameters(match[4]),
			Annotations:  annotations,
			Calls:        extractJavaCalls(line, lineNo),
			HTTPRequests: extractJavaHTTPRequestsWithVars(line, lineNo, nil),
		}, true
	}
	return JavaMethodRecord{}, false
}

func parseJavaConstructorSignature(line, owner string) (params, visibility string, ok bool) {
	signature := strings.TrimSpace(line)
	if index := strings.Index(signature, "{"); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	if index := strings.Index(signature, " throws "); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	open := strings.Index(signature, "(")
	close := strings.LastIndex(signature, ")")
	if open < 0 || close < open {
		return "", "", false
	}
	prefix := strings.Fields(strings.TrimSpace(signature[:open]))
	if len(prefix) == 2 {
		switch prefix[0] {
		case "public", "protected", "private":
			visibility = prefix[0]
			prefix = prefix[1:]
		}
	}
	if len(prefix) != 1 || prefix[0] != owner {
		return "", "", false
	}
	return signature[open+1 : close], visibility, true
}

func parseJavaMethodSignature(line string) (name, returnType, params, visibility string, ok bool) {
	signature := strings.TrimSpace(line)
	if index := strings.Index(signature, "{"); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	if index := strings.Index(signature, " throws "); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	open := strings.Index(signature, "(")
	close := strings.LastIndex(signature, ")")
	if open < 0 || close < open {
		return "", "", "", "", false
	}
	prefix := strings.TrimSpace(signature[:open])
	if strings.Contains(prefix, "=") || strings.Contains(prefix, ".") {
		return "", "", "", "", false
	}
	params = signature[open+1 : close]
	fields := strings.Fields(prefix)
	if len(fields) < 2 {
		return "", "", "", "", false
	}
	var kept []string
	for _, field := range fields {
		switch field {
		case "public", "protected", "private":
			if visibility == "" {
				visibility = field
			}
		case "static", "final", "abstract", "synchronized", "default", "native", "strictfp":
			continue
		default:
			kept = append(kept, field)
		}
	}
	if len(kept) < 2 {
		return "", "", "", "", false
	}
	name = kept[len(kept)-1]
	returnType = cleanJavaType(strings.Join(kept[:len(kept)-1], " "))
	return name, returnType, params, visibility, true
}

func looksLikeJavaMethodStart(line string) bool {
	trimmed := strings.TrimSpace(line)
	if isJavaControlLine(trimmed) || strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "@") || strings.HasSuffix(trimmed, ";") {
		return false
	}
	open := strings.Index(trimmed, "(")
	if open < 0 || strings.HasPrefix(trimmed, "new ") {
		return false
	}
	prefix := trimmed[:open]
	return !strings.Contains(prefix, "=") && !strings.Contains(prefix, ".")
}

func javaSymbols(source JavaSourceRecord) []SymbolRecord {
	var symbols []SymbolRecord
	for _, typ := range source.Types {
		symbols = append(symbols, SymbolRecord{Name: typ.Name, Kind: typ.Kind, File: typ.File, Line: typ.Line})
	}
	for _, method := range source.Methods {
		symbols = append(symbols, SymbolRecord{Name: method.Name, Kind: "method", File: method.File, Line: method.Line})
		if strings.HasPrefix(method.Name, "test") || hasAnnotation(method.Annotations, "Test") {
			symbols = append(symbols, SymbolRecord{Name: method.Name, Kind: "test", File: method.File, Line: method.Line})
		}
	}
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Line != symbols[j].Line {
			return symbols[i].Line < symbols[j].Line
		}
		if symbols[i].Kind != symbols[j].Kind {
			return symbols[i].Kind < symbols[j].Kind
		}
		return symbols[i].Name < symbols[j].Name
	})
	return symbols
}

func javaImportRelations(source JavaSourceRecord) []RelationRecord {
	relations := make([]RelationRecord, 0, len(source.Imports))
	for _, imported := range source.Imports {
		relations = append(relations, RelationRecord{From: source.File, To: imported.Name, Type: "imports", Line: imported.Line})
	}
	return relations
}

func stripJavaLineNoise(line string, inBlock bool) (string, bool) {
	if inBlock {
		end := strings.Index(line, "*/")
		if end < 0 {
			return "", true
		}
		line = line[end+2:]
		inBlock = false
	}
	for {
		start := strings.Index(line, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(line[start+2:], "*/")
		if end < 0 {
			line = line[:start]
			inBlock = true
			break
		}
		line = line[:start] + line[start+2+end+2:]
	}
	if index := strings.Index(line, "//"); index >= 0 {
		line = line[:index]
	}
	return strings.TrimSpace(line), inBlock
}

func shortJavaName(name string) string {
	name = strings.TrimSpace(name)
	if index := strings.LastIndex(name, "."); index >= 0 {
		return name[index+1:]
	}
	return name
}

func isJavaControlLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimSpace(strings.TrimLeft(trimmed, "}"))
	for _, prefix := range []string{"if", "else", "for", "while", "switch", "catch", "try", "finally", "return", "throw", "new"} {
		if strings.HasPrefix(trimmed, prefix+" ") || strings.HasPrefix(trimmed, prefix+"(") {
			return true
		}
	}
	return false
}

func parseJavaAnnotationAttributes(args string) map[string]string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	attrs := map[string]string{}
	for _, part := range splitTopLevel(args, ',') {
		piece := strings.TrimSpace(part)
		if piece == "" {
			continue
		}
		key := "value"
		value := piece
		if index := strings.Index(piece, "="); index >= 0 {
			key = strings.TrimSpace(piece[:index])
			value = strings.TrimSpace(piece[index+1:])
		}
		attrs[key] = trimJavaValue(value)
	}
	return attrs
}

func parseJavaParameters(params string) []JavaParameterRecord {
	params = strings.TrimSpace(params)
	if params == "" {
		return nil
	}
	var records []JavaParameterRecord
	for _, raw := range splitTopLevel(params, ',') {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		part = strings.TrimSpace(strings.ReplaceAll(part, "final ", ""))
		var annotations []JavaAnnotationRecord
		for strings.HasPrefix(part, "@") {
			annotation, rest, ok := consumeJavaParameterAnnotation(part)
			if !ok {
				break
			}
			annotations = append(annotations, annotation)
			part = rest
		}
		part = strings.TrimSpace(strings.ReplaceAll(part, "final ", ""))
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		typ := cleanJavaType(strings.Join(fields[:len(fields)-1], " "))
		records = append(records, JavaParameterRecord{Name: strings.TrimSpace(name), Type: typ, Annotations: annotations})
	}
	return records
}

func consumeJavaParameterAnnotation(part string) (JavaAnnotationRecord, string, bool) {
	part = strings.TrimSpace(part)
	if !strings.HasPrefix(part, "@") {
		return JavaAnnotationRecord{}, part, false
	}
	nameEnd := 1
	for nameEnd < len(part) {
		ch := part[nameEnd]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			nameEnd++
			continue
		}
		break
	}
	if nameEnd <= 1 {
		return JavaAnnotationRecord{}, part, false
	}
	name := part[1:nameEnd]
	rest := strings.TrimSpace(part[nameEnd:])
	args := ""
	if strings.HasPrefix(rest, "(") {
		depth := 0
		close := -1
		for index, r := range rest {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					close = index
					break
				}
			}
		}
		if close < 0 {
			return JavaAnnotationRecord{}, part, false
		}
		args = rest[1:close]
		rest = strings.TrimSpace(rest[close+1:])
	}
	return JavaAnnotationRecord{Name: shortJavaName(name), Arguments: strings.TrimSpace(args), Attributes: parseJavaAnnotationAttributes(args)}, rest, true
}

func extractJavaCalls(line string, lineNo int) []JavaCallRecord {
	var calls []JavaCallRecord
	for _, match := range javaNewCallRE.FindAllStringSubmatch(line, -1) {
		if len(match) == 3 {
			calls = append(calls, JavaCallRecord{TargetOwner: match[1], Method: match[2], Line: lineNo})
		}
	}
	for _, match := range javaCallRE.FindAllStringSubmatch(line, -1) {
		if len(match) == 3 {
			if match[1] == "new" {
				continue
			}
			calls = append(calls, JavaCallRecord{Receiver: match[1], Method: match[2], Line: lineNo})
		}
	}
	for _, match := range javaBareCallRE.FindAllStringSubmatchIndex(line, -1) {
		if len(match) != 4 {
			continue
		}
		name := line[match[2]:match[3]]
		if isIgnoredJavaBareCall(name) || isJavaQualifiedCall(line, match[0]) {
			continue
		}
		calls = append(calls, JavaCallRecord{Method: name, Line: lineNo, Arguments: javaCallArguments(line, match[1]-1)})
	}
	return calls
}

func javaCallArguments(line string, open int) []string {
	if open < 0 || open >= len(line) || line[open] != '(' {
		return nil
	}
	depth := 0
	for index := open; index < len(line); index++ {
		switch line[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return splitJavaCallArguments(line[open+1 : index])
			}
		}
	}
	return nil
}

func splitJavaCallArguments(args string) []string {
	var result []string
	depth := 0
	start := 0
	for index, r := range args {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(args[start:index]))
				start = index + 1
			}
		}
	}
	if strings.TrimSpace(args[start:]) != "" {
		result = append(result, strings.TrimSpace(args[start:]))
	}
	return result
}

func isIgnoredJavaBareCall(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "return", "new", "super", "this", "assertThat":
		return true
	default:
		return false
	}
}

func isJavaQualifiedCall(line string, start int) bool {
	if start <= 0 {
		return false
	}
	prev := line[start-1]
	return prev == '.' || prev == ':'
}

func extractJavaHTTPRequests(line string, lineNo int) []JavaHTTPCallRecord {
	return extractJavaHTTPRequestsWithVars(line, lineNo, nil)
}

func extractJavaHTTPRequestsWithVars(line string, lineNo int, vars map[string]string) []JavaHTTPCallRecord {
	requests, _ := extractJavaHTTPRequestsWithPending(line, lineNo, vars, "")
	return requests
}

func extractJavaHTTPRequestsWithPending(line string, lineNo int, vars map[string]string, pending string) ([]JavaHTTPCallRecord, string) {
	var requests []JavaHTTPCallRecord
	for _, match := range javaHTTPCallRE.FindAllStringSubmatch(line, -1) {
		if len(match) == 3 {
			if path, ok := javaHTTPRequestPath(match[2], vars); ok {
				requests = append(requests, JavaHTTPCallRecord{HTTPMethod: strings.ToUpper(match[1]), Path: path, Line: lineNo})
			}
		}
	}
	for _, match := range javaHTTPBuilderRefRE.FindAllStringSubmatch(line, -1) {
		if len(match) == 3 {
			if path, ok := javaHTTPRequestPath(match[2], vars); ok {
				requests = append(requests, JavaHTTPCallRecord{HTTPMethod: strings.ToUpper(match[1]), Path: path, Line: lineNo})
			}
		}
	}
	if match := javaHTTPChainVerbRE.FindStringSubmatch(line); len(match) == 2 {
		pending = strings.ToUpper(match[1])
	}
	if pending != "" {
		if match := javaHTTPChainURIRE.FindStringSubmatch(line); len(match) == 2 {
			if path, ok := javaHTTPRequestPath(match[1], vars); ok {
				requests = append(requests, JavaHTTPCallRecord{HTTPMethod: pending, Path: path, Line: lineNo})
				pending = ""
			}
		}
	}
	return requests, pending
}

func javaHTTPRequestPath(expression string, vars map[string]string) (string, bool) {
	expression = strings.TrimSpace(expression)
	if vars != nil {
		if value, ok := vars[expression]; ok {
			return value, strings.HasPrefix(value, "/")
		}
	}
	if strings.HasPrefix(expression, "String.format(") {
		if args := javaCallArguments(expression, strings.Index(expression, "(")); len(args) > 0 {
			return javaHTTPRequestPath(args[0], vars)
		}
		if args := splitJavaCallArguments(strings.TrimPrefix(expression, "String.format(")); len(args) > 0 {
			return javaHTTPRequestPath(args[0], vars)
		}
	}
	literals := javaStringLiteralRE.FindAllStringSubmatchIndex(expression, -1)
	if len(literals) == 0 {
		return "", false
	}

	var b strings.Builder
	lastEnd := 0
	if literals[0][0] != 0 {
		prefix, ok := javaRequestBasePathPrefix(expression[:literals[0][0]])
		if !ok {
			return "", false
		}
		b.WriteString(prefix)
		lastEnd = literals[0][0]
	}
	for _, literal := range literals {
		if hasJavaConcatExpression(expression[lastEnd:literal[0]]) {
			appendDynamicPathSegment(&b)
		}
		b.WriteString(expression[literal[2]:literal[3]])
		lastEnd = literal[1]
	}
	if hasJavaConcatExpression(expression[lastEnd:]) {
		appendDynamicPathSegment(&b)
	}
	path := strings.ReplaceAll(stripJavaRequestQuery(normalizeJavaFormatPath(b.String())), "//", "/")
	return path, strings.HasPrefix(path, "/")
}

func stripJavaRequestQuery(path string) string {
	if index := strings.IndexAny(path, "?#"); index >= 0 {
		return path[:index]
	}
	return path
}

func javaRequestBasePathPrefix(expression string) (string, bool) {
	expression = strings.Trim(expression, "+ \t")
	fields := strings.FieldsFunc(expression, func(r rune) bool {
		return r == '+' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	for i := len(fields) - 1; i >= 0; i-- {
		field := strings.TrimSpace(fields[i])
		if strings.Contains(field, "BASE_PATH") && isJavaIdentifierPath(field) {
			return "/" + field, true
		}
	}
	return "", false
}

func isJavaIdentifierPath(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r == '.' || r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func normalizeJavaFormatPath(path string) string {
	formatPlaceholderRE := regexp.MustCompile(`%[0-9.]*[A-Za-z]`)
	return formatPlaceholderRE.ReplaceAllString(path, "{dynamic}")
}

func extractJavaStringVars(line string) map[string]string {
	match := javaStringVarLineRE.FindStringSubmatch(line)
	if len(match) != 3 {
		return nil
	}
	if path, ok := javaHTTPRequestPath(match[2], nil); ok {
		return map[string]string{match[1]: path}
	}
	return nil
}

func mergeJavaStringVars(existing, next map[string]string) map[string]string {
	if len(next) == 0 {
		return existing
	}
	if existing == nil {
		existing = map[string]string{}
	}
	for key, value := range next {
		existing[key] = value
	}
	return existing
}

func hasJavaConcatExpression(part string) bool {
	part = strings.TrimSpace(part)
	if part == "" {
		return false
	}
	if strings.HasPrefix(part, ".formatted(") || strings.HasPrefix(part, ".format(") {
		return false
	}
	part = strings.Trim(part, "+ \t")
	return part != ""
}

func appendDynamicPathSegment(b *strings.Builder) {
	value := b.String()
	if value == "" || strings.HasSuffix(value, "/") {
		b.WriteString("{dynamic}")
		return
	}
	b.WriteString("/{dynamic}")
}

func parseJavaExtends(tail string) string {
	match := regexp.MustCompile(`\bextends\s+([A-Za-z_][A-Za-z0-9_<>.,\s]*)`).FindStringSubmatch(tail)
	if len(match) != 2 {
		return ""
	}
	value := strings.TrimSpace(match[1])
	if index := strings.Index(value, " implements "); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func parseJavaImplements(tail string) []string {
	match := regexp.MustCompile(`\bimplements\s+(.+)$`).FindStringSubmatch(tail)
	if len(match) != 2 {
		return nil
	}
	var values []string
	for _, value := range splitTopLevel(match[1], ',') {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func cleanJavaType(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "final ", "")
	return strings.Join(strings.Fields(value), " ")
}

func splitTopLevel(value string, sep rune) []string {
	var parts []string
	depth := 0
	start := 0
	for index, r := range value {
		switch r {
		case '(', '{', '[', '<':
			depth++
		case ')', '}', ']', '>':
			if depth > 0 {
				depth--
			}
		default:
			if r == sep && depth == 0 {
				parts = append(parts, value[start:index])
				start = index + len(string(r))
			}
		}
	}
	parts = append(parts, value[start:])
	return parts
}

func trimJavaValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "{"), "}"))
	}
	return strings.Trim(value, `"`)
}
