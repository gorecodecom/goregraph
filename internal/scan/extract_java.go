package scan

import (
	"regexp"
	"sort"
	"strings"
)

var (
	javaPackageLineRE         = regexp.MustCompile(`^\s*package\s+([A-Za-z_][A-Za-z0-9_.]*);`)
	javaImportLineRE          = regexp.MustCompile(`^\s*import\s+(static\s+)?([^;]+);`)
	javaTypeLineRE            = regexp.MustCompile(`^\s*(?:public|protected|private|abstract|final|sealed|non-sealed|static|\s)*\s*(class|interface|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)\b(.*)$`)
	javaMethodLineRE          = regexp.MustCompile(`^\s*(public|protected|private)?\s*(?:static\s+)?(?:final\s+)?([A-Za-z_][A-Za-z0-9_$<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)\s*(?:throws\s+[^{]+)?\{?\s*$`)
	javaFieldLineRE           = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(?:static\s+)?(final\s+)?([A-Za-z_][A-Za-z0-9_$<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)(\s*(?:\[\s*\])*)\s*(?:=.*)?;\s*$`)
	javaAnnotationLineRE      = regexp.MustCompile(`^\s*@([A-Za-z_][A-Za-z0-9_.]*)(?:\((.*)\))?\s*$`)
	javaConstantLineRE        = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*static\s+final\s+String\s+([A-Za-z0-9_]+)\s*=\s*"([^"]*)"\s*;`)
	javaCallRE                = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaChainedMatcherRE      = regexp.MustCompile(`^\s*\.securityMatcher\s*\(`)
	javaNewCallRE             = regexp.MustCompile(`\bnew\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaConstructorTypeRE     = regexp.MustCompile(`\bnew\s+([A-Za-z_][A-Za-z0-9_$.]*)\s*\(`)
	javaBareCallRE            = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaHTTPCallRE            = regexp.MustCompile(`\b(get|post|put|delete|patch)\s*\(([^)]*)\)`)
	javaHTTPBuilderRefRE      = regexp.MustCompile(`MockMvcRequestBuilders::(get|post|put|delete|patch)\s*,\s*(.+?)(?:,\s*[^)]*)?\)`)
	javaHTTPChainVerbRE       = regexp.MustCompile(`^\s*\.(get|post|put|delete|patch)\s*\(\s*\)\s*$`)
	javaHTTPChainURIRE        = regexp.MustCompile(`^\s*\.uri\s*\((.+)\)\s*$`)
	javaHTTPChainReceiverRE   = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*$`)
	javaStringLiteralRE       = regexp.MustCompile(`"([^"]*)"`)
	javaStringVarLineRE       = regexp.MustCompile(`^\s*(?:final\s+)?String\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+);\s*$`)
	javaStringVarStartRE      = regexp.MustCompile(`^(?:final\s+)?String\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)
	javaLocalTypeLineRE       = regexp.MustCompile(`^\s*(?:final\s+)?([A-Za-z_][A-Za-z0-9_$.<>?, ]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*.+;\s*$`)
	javaReturnExpressionRE    = regexp.MustCompile(`^\s*return\s+(.+);\s*$`)
	javaSecurityCallRE        = regexp.MustCompile(`\.(permitAll|authenticated|hasRole|hasAnyRole|hasAuthority|hasAnyAuthority|httpBasic|oauth2ResourceServer|oauth2Login|formLogin|x509)\s*\(([^)]*)\)`)
	javaParameterDeclaratorRE = regexp.MustCompile(`^(.+?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*((?:\[\s*\])*)$`)
	javaTypeParameterNameRE   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func extractJavaSource(file FileRecord, body string) JavaSourceRecord {
	source := JavaSourceRecord{File: file.Path, Constants: map[string]string{}}
	lines := strings.Split(body, "\n")
	lexicalLines := strings.Split(sanitizeJavaLexical(body), "\n")
	literalLines := strings.Split(sanitizeJavaComments(body), "\n")
	var pending []JavaAnnotationRecord
	currentOwner := ""
	braceDepth := 0
	typeStack := []javaTypeScope{}
	blockComment := false
	methodSignature := ""
	methodSignatureSource := ""
	methodSignatureLine := 0
	typeSignature := ""
	typeSignatureLine := 0
	annotationSignature := ""
	annotationSignatureLine := 0

	for index, raw := range lines {
		lineNo := index + 1
		line, inBlock := stripJavaLineNoise(raw, blockComment)
		blockComment = inBlock
		lexicalLine := strings.TrimSpace(lexicalLines[index])
		if annotationSignature != "" {
			if isAnnotationBoundary(lexicalLine) {
				if annotation, ok := parseJavaAnnotationLine(annotationSignature, annotationSignatureLine); ok {
					pending = append(pending, annotation)
					source.Annotations = append(source.Annotations, annotation)
				}
				annotationSignature = ""
				annotationSignatureLine = 0
			} else {
				if cleanedLine := strings.TrimSpace(line); cleanedLine != "" {
					annotationSignature += " " + cleanedLine
				}
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
		if lexicalLine == "" {
			continue
		}
		if typeSignature != "" {
			typeSignature += " " + lexicalLine
			if javaDeclarationBodyOpen(typeSignature) >= 0 {
				if typeIndex, ok := appendJavaType(&source, typeSignature, typeSignatureLine, file.Path, pending, typeStack); ok {
					typeStack = append(typeStack, javaTypeScope{typeIndex: typeIndex, bodyDepth: braceDepth + 1})
					currentOwner = source.Types[typeIndex].Name
					pending = nil
				}
				typeSignature = ""
				typeSignatureLine = 0
			}
			braceDepth += strings.Count(lexicalLine, "{")
			braceDepth -= strings.Count(lexicalLine, "}")
			typeStack, currentOwner, braceDepth = finalizeJavaTypeScopes(&source, typeStack, braceDepth, lineNo)
			continue
		}
		if methodSignature != "" {
			methodSignature += " " + lexicalLines[index]
			methodSignatureSource += " " + literalLines[index]
			if javaDeclarationBodyOpen(methodSignature) >= 0 {
				if method, ok := parseJavaMethodWithSource(methodSignature, methodSignatureSource, file.Path, currentOwner, methodSignatureLine, pending); ok {
					source.Methods = append(source.Methods, method)
					pending = nil
				}
				methodSignature = ""
				methodSignatureSource = ""
				methodSignatureLine = 0
			}
			braceDepth += strings.Count(lexicalLine, "{")
			braceDepth -= strings.Count(lexicalLine, "}")
			typeStack, currentOwner, braceDepth = finalizeJavaTypeScopes(&source, typeStack, braceDepth, lineNo)
			continue
		}

		if match := javaPackageLineRE.FindStringSubmatch(lexicalLine); len(match) == 2 {
			source.Package = match[1]
			continue
		}
		if match := javaImportLineRE.FindStringSubmatch(lexicalLine); len(match) == 3 {
			source.Imports = append(source.Imports, JavaImportRecord{Name: strings.TrimSpace(match[2]), Static: strings.TrimSpace(match[1]) == "static", Line: lineNo})
			continue
		}
		if strings.HasPrefix(lexicalLine, "@") && strings.Contains(lexicalLine, "(") && !balancedJavaParens(line) {
			annotationSignature = strings.TrimSpace(line)
			annotationSignatureLine = lineNo
			continue
		}
		if strings.HasPrefix(lexicalLine, "@") {
			if annotation, ok := parseJavaAnnotationLine(line, lineNo); ok {
				pending = append(pending, annotation)
				source.Annotations = append(source.Annotations, annotation)
				continue
			}
		}
		if match := javaConstantLineRE.FindStringSubmatch(line); len(match) == 3 {
			source.Constants[match[1]] = match[2]
		}
		if javaTypeLineRE.MatchString(lexicalLine) {
			if javaDeclarationBodyOpen(lexicalLine) < 0 {
				typeSignature = lexicalLine
				typeSignatureLine = lineNo
				continue
			}
			if typeIndex, ok := appendJavaType(&source, lexicalLine, lineNo, file.Path, pending, typeStack); ok {
				currentOwner = source.Types[typeIndex].Name
				typeStack = append(typeStack, javaTypeScope{typeIndex: typeIndex, bodyDepth: braceDepth + 1})
				pending = nil
			}
		} else if match := javaFieldLineRE.FindStringSubmatch(lexicalLine); len(match) == 5 && currentOwner != "" && javaAtCurrentTypeBody(braceDepth, typeStack) {
			source.Fields = append(source.Fields, JavaFieldRecord{
				Name:        match[3],
				Type:        cleanJavaType(match[2] + strings.ReplaceAll(match[4], " ", "")),
				File:        file.Path,
				Line:        lineNo,
				Owner:       currentOwner,
				Final:       strings.TrimSpace(match[1]) == "final",
				Annotations: pending,
			})
			pending = nil
		} else if currentOwner != "" && (looksLikeJavaMethodStart(lexicalLine) || looksLikeJavaGenericMethodPrefix(lexicalLine)) && javaDeclarationBodyOpen(lexicalLine) < 0 {
			methodSignature = lexicalLines[index]
			methodSignatureSource = literalLines[index]
			methodSignatureLine = lineNo
		} else if method, ok := parseJavaMethodWithSource(lexicalLines[index], literalLines[index], file.Path, currentOwner, lineNo, pending); ok && currentOwner != "" {
			source.Methods = append(source.Methods, method)
			pending = nil
		} else if len(source.Methods) > 0 {
			last := &source.Methods[len(source.Methods)-1]
			last.LocalTypes = mergeJavaLocalTypes(last.LocalTypes, extractJavaLocalType(strings.TrimSpace(lexicalLines[index])))
			if match := javaReturnExpressionRE.FindStringSubmatch(strings.TrimSpace(literalLines[index])); len(match) == 2 {
				last.ReturnExpression = strings.TrimSpace(match[1])
			}
			last.StringExpressions = mergeJavaStringExpressions(
				last.StringExpressions,
				extractJavaStringExpressionAt(literalLines, index),
			)
			last.StringVars = mergeJavaStringVars(last.StringVars, extractJavaStringVars(strings.TrimSpace(literalLines[index])))
			last.ConstructedTypes = append(last.ConstructedTypes, extractJavaConstructedTypes(lexicalLines[index])...)
			last.Calls = append(last.Calls, extractJavaCallsWithSource(lexicalLines[index], literalLines[index], lineNo)...)
			last.Auth = append(last.Auth, extractJavaSecurityAuth(lexicalLine, lineNo, file.Path)...)
			requests, pending := extractJavaHTTPRequestsWithPendingSource(lexicalLines[index], literalLines[index], lineNo, last.StringVars, last.PendingHTTP)
			last.PendingHTTP = pending
			last.HTTPRequests = append(last.HTTPRequests, requests...)
		}

		braceDepth += strings.Count(lexicalLine, "{")
		braceDepth -= strings.Count(lexicalLine, "}")
		typeStack, currentOwner, braceDepth = finalizeJavaTypeScopes(&source, typeStack, braceDepth, lineNo)
	}
	for len(typeStack) > 0 {
		scope := typeStack[len(typeStack)-1]
		source.Types[scope.typeIndex].EndLine = len(lines)
		typeStack = typeStack[:len(typeStack)-1]
	}

	if len(source.Constants) == 0 {
		source.Constants = nil
	}
	resolveJavaHTTPClientKinds(&source)
	resolveJavaHTTPCallConfidence(&source)
	return source
}

type javaTypeScope struct {
	typeIndex int
	bodyDepth int
}

func javaAtCurrentTypeBody(braceDepth int, stack []javaTypeScope) bool {
	return len(stack) > 0 && braceDepth == stack[len(stack)-1].bodyDepth
}

func finalizeJavaTypeScopes(source *JavaSourceRecord, stack []javaTypeScope, braceDepth, line int) ([]javaTypeScope, string, int) {
	for len(stack) > 0 && braceDepth < stack[len(stack)-1].bodyDepth {
		scope := stack[len(stack)-1]
		source.Types[scope.typeIndex].EndLine = line
		stack = stack[:len(stack)-1]
	}
	owner := ""
	if len(stack) > 0 {
		owner = source.Types[stack[len(stack)-1].typeIndex].Name
	}
	if braceDepth < 0 {
		braceDepth = 0
	}
	return stack, owner, braceDepth
}

func appendJavaType(source *JavaSourceRecord, signature string, line int, file string, annotations []JavaAnnotationRecord, stack []javaTypeScope) (int, bool) {
	match := javaTypeLineRE.FindStringSubmatch(signature)
	if len(match) != 4 {
		return -1, false
	}
	owner := ""
	if len(stack) > 0 {
		owner = source.Types[stack[len(stack)-1].typeIndex].QualifiedName
	}
	tail := match[3]
	if index := javaDeclarationBodyOpen(tail); index >= 0 {
		tail = tail[:index]
	}
	typeParameters, inheritanceTail := parseLeadingJavaTypeParameters(tail)
	typ := JavaTypeRecord{
		Name:           match[2],
		Kind:           match[1],
		Package:        source.Package,
		File:           file,
		Line:           line,
		Owner:          owner,
		QualifiedName:  qualifiedJavaTypeName(source.Package, owner, match[2]),
		TypeParameters: typeParameters,
		Extends:        parseJavaExtends(inheritanceTail),
		Implements:     parseJavaImplements(inheritanceTail),
		Annotations:    annotations,
	}
	source.Types = append(source.Types, typ)
	return len(source.Types) - 1, true
}

func sanitizeJavaLexical(body string) string {
	return sanitizeJavaSource(body, false)
}

func sanitizeJavaComments(body string) string {
	return sanitizeJavaSource(body, true)
}

func sanitizeJavaSource(body string, preserveLiterals bool) string {
	const (
		javaLexCode = iota
		javaLexLineComment
		javaLexBlockComment
		javaLexString
		javaLexChar
		javaLexTextBlock
	)
	result := []byte(body)
	state := javaLexCode
	for index := 0; index < len(result); {
		if result[index] == '\n' {
			if state == javaLexLineComment {
				state = javaLexCode
			}
			index++
			continue
		}
		switch state {
		case javaLexCode:
			switch {
			case index+1 < len(result) && result[index] == '/' && result[index+1] == '/':
				result[index], result[index+1] = ' ', ' '
				index += 2
				state = javaLexLineComment
			case index+1 < len(result) && result[index] == '/' && result[index+1] == '*':
				result[index], result[index+1] = ' ', ' '
				index += 2
				state = javaLexBlockComment
			case index+2 < len(result) && result[index] == '"' && result[index+1] == '"' && result[index+2] == '"':
				if !preserveLiterals {
					result[index], result[index+1], result[index+2] = ' ', ' ', ' '
				}
				index += 3
				state = javaLexTextBlock
			case result[index] == '"':
				if !preserveLiterals {
					result[index] = ' '
				}
				index++
				state = javaLexString
			case result[index] == '\'':
				if !preserveLiterals {
					result[index] = ' '
				}
				index++
				state = javaLexChar
			default:
				index++
			}
		case javaLexLineComment:
			result[index] = ' '
			index++
		case javaLexBlockComment:
			if index+1 < len(result) && result[index] == '*' && result[index+1] == '/' {
				result[index], result[index+1] = ' ', ' '
				index += 2
				state = javaLexCode
			} else {
				result[index] = ' '
				index++
			}
		case javaLexString, javaLexChar:
			quote := byte('"')
			if state == javaLexChar {
				quote = '\''
			}
			if result[index] == '\\' && index+1 < len(result) {
				if !preserveLiterals {
					result[index], result[index+1] = ' ', ' '
				}
				index += 2
			} else if result[index] == quote {
				if !preserveLiterals {
					result[index] = ' '
				}
				index++
				state = javaLexCode
			} else {
				if !preserveLiterals {
					result[index] = ' '
				}
				index++
			}
		case javaLexTextBlock:
			if index+2 < len(result) && result[index] == '"' && result[index+1] == '"' && result[index+2] == '"' {
				if !preserveLiterals {
					result[index], result[index+1], result[index+2] = ' ', ' ', ' '
				}
				index += 3
				state = javaLexCode
			} else {
				if !preserveLiterals {
					result[index] = ' '
				}
				index++
			}
		}
	}
	return string(result)
}

func parseLeadingJavaTypeParameters(value string) ([]string, string) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "<") {
		return nil, trimmed
	}
	depth := 0
	closeIndex := -1
	for index, char := range trimmed {
		switch char {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				closeIndex = index
			}
		}
		if closeIndex >= 0 {
			break
		}
	}
	if closeIndex < 0 {
		return nil, trimmed
	}
	var names []string
	for _, raw := range splitTopLevel(trimmed[1:closeIndex], ',') {
		fields := strings.Fields(stripLeadingJavaAnnotations(strings.TrimSpace(raw)))
		if len(fields) > 0 && javaTypeParameterNameRE.MatchString(fields[0]) {
			names = append(names, fields[0])
		}
	}
	return names, strings.TrimSpace(trimmed[closeIndex+1:])
}

func stripLeadingJavaAnnotations(value string) string {
	for strings.HasPrefix(strings.TrimSpace(value), "@") {
		_, rest, ok := consumeJavaParameterAnnotation(value)
		if !ok {
			break
		}
		value = rest
	}
	return strings.TrimSpace(value)
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
	return parseJavaMethodWithSource(line, line, file, owner, lineNo, annotations)
}

func parseJavaMethodWithSource(line, sourceLine, file, owner string, lineNo int, annotations []JavaAnnotationRecord) (JavaMethodRecord, bool) {
	if owner == "" || isJavaControlLine(line) {
		return JavaMethodRecord{}, false
	}
	if params, visibility, ok := parseJavaConstructorSignature(line, owner); ok {
		if sourceParams, sourceOK := javaSourceParameters(line, sourceLine); sourceOK {
			params = sourceParams
		}
		return JavaMethodRecord{
			Name:        owner,
			File:        file,
			Line:        lineNo,
			Owner:       owner,
			Visibility:  visibility,
			Parameters:  parseJavaParameters(params),
			Annotations: annotations,
			Calls:       extractJavaCallsWithSource(line, sourceLine, lineNo),
		}, true
	}
	if name, returnType, params, visibility, typeParameters, ok := parseJavaMethodSignatureWithTypeParameters(line); ok {
		if sourceParams, sourceOK := javaSourceParameters(line, sourceLine); sourceOK {
			params = sourceParams
		}
		return JavaMethodRecord{
			Name:           name,
			File:           file,
			Line:           lineNo,
			Owner:          owner,
			Visibility:     visibility,
			ReturnType:     returnType,
			Parameters:     parseJavaParameters(params),
			Annotations:    annotations,
			Calls:          extractJavaCallsWithSource(line, sourceLine, lineNo),
			HTTPRequests:   extractJavaHTTPRequestsWithSource(line, sourceLine, lineNo, nil),
			TypeParameters: typeParameters,
		}, true
	}
	match := javaMethodLineRE.FindStringSubmatch(line)
	if len(match) == 5 {
		params := match[4]
		if sourceParams, sourceOK := javaSourceParameters(line, sourceLine); sourceOK {
			params = sourceParams
		}
		return JavaMethodRecord{
			Name:         match[3],
			File:         file,
			Line:         lineNo,
			Owner:        owner,
			Visibility:   strings.TrimSpace(match[1]),
			ReturnType:   cleanJavaType(match[2]),
			Parameters:   parseJavaParameters(params),
			Annotations:  annotations,
			Calls:        extractJavaCallsWithSource(line, sourceLine, lineNo),
			HTTPRequests: extractJavaHTTPRequestsWithSource(line, sourceLine, lineNo, nil),
		}, true
	}
	return JavaMethodRecord{}, false
}

func javaSourceParameters(line, sourceLine string) (string, bool) {
	open := javaMethodParameterOpen(line)
	close := matchingJavaParen(line, open)
	if open < 0 || close < open || close > len(sourceLine) {
		return "", false
	}
	return sourceLine[open+1 : close], true
}

func parseJavaConstructorSignature(line, owner string) (params, visibility string, ok bool) {
	signature := strings.TrimSpace(line)
	if index := javaDeclarationBodyOpen(signature); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	if index := strings.Index(signature, " throws "); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	open := javaMethodParameterOpen(signature)
	close := matchingJavaParen(signature, open)
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
	name, returnType, params, visibility, _, ok = parseJavaMethodSignatureWithTypeParameters(line)
	return name, returnType, params, visibility, ok
}

func parseJavaMethodSignatureWithTypeParameters(line string) (name, returnType, params, visibility string, typeParameters []string, ok bool) {
	signature := strings.TrimSpace(line)
	if index := javaDeclarationBodyOpen(signature); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	if index := strings.Index(signature, " throws "); index >= 0 {
		signature = strings.TrimSpace(signature[:index])
	}
	open := javaMethodParameterOpen(signature)
	close := matchingJavaParen(signature, open)
	if open < 0 || close < open {
		return "", "", "", "", nil, false
	}
	prefix := strings.TrimSpace(signature[:open])
	if strings.Contains(prefix, "=") || strings.Contains(prefix, ".") {
		return "", "", "", "", nil, false
	}
	params = signature[open+1 : close]
	for {
		fields := strings.Fields(prefix)
		if len(fields) == 0 {
			return "", "", "", "", nil, false
		}
		field := fields[0]
		modifier := true
		switch field {
		case "public", "protected", "private":
			if visibility == "" {
				visibility = field
			}
		case "static", "final", "abstract", "synchronized", "default", "native", "strictfp":
		default:
			modifier = false
		}
		if !modifier {
			break
		}
		prefix = strings.TrimSpace(strings.TrimPrefix(prefix, field))
	}
	typeParameters, prefix = parseLeadingJavaTypeParameters(prefix)
	kept := strings.Fields(prefix)
	if len(kept) < 2 {
		return "", "", "", "", nil, false
	}
	name = kept[len(kept)-1]
	returnType = cleanJavaType(strings.Join(kept[:len(kept)-1], " "))
	return name, returnType, params, visibility, typeParameters, true
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

func looksLikeJavaGenericMethodPrefix(line string) bool {
	trimmed := strings.TrimSpace(line)
	for {
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			return false
		}
		switch fields[0] {
		case "public", "protected", "private", "static", "final", "abstract", "synchronized", "default", "native", "strictfp":
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
		default:
			return strings.HasPrefix(trimmed, "<")
		}
	}
}

func matchingJavaParen(value string, open int) int {
	if open < 0 || open >= len(value) || value[open] != '(' {
		return -1
	}
	depth := 0
	quote := byte(0)
	escaped := false
	for index := open; index < len(value); index++ {
		char := value[index]
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
		case '\'', '"':
			quote = char
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func javaMethodParameterOpen(value string) int {
	angleDepth := 0
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case '(':
			if angleDepth == 0 {
				return index
			}
		}
	}
	return -1
}

func javaDeclarationBodyOpen(value string) int {
	angleDepth := 0
	parenDepth := 0
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{':
			if angleDepth == 0 && parenDepth == 0 {
				return index
			}
		}
	}
	return -1
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
		match := javaParameterDeclaratorRE.FindStringSubmatch(part)
		if len(match) != 4 {
			continue
		}
		arraySuffix := strings.ReplaceAll(match[3], " ", "")
		typ := cleanJavaType(match[1] + arraySuffix)
		records = append(records, JavaParameterRecord{Name: match[2], Type: typ, Annotations: annotations})
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
		close := matchingJavaParen(rest, 0)
		if close < 0 {
			return JavaAnnotationRecord{}, part, false
		}
		args = rest[1:close]
		rest = strings.TrimSpace(rest[close+1:])
	}
	return JavaAnnotationRecord{Name: shortJavaName(name), Arguments: strings.TrimSpace(args), Attributes: parseJavaAnnotationAttributes(args)}, rest, true
}

func extractJavaCalls(line string, lineNo int) []JavaCallRecord {
	return extractJavaCallsWithSource(line, line, lineNo)
}

func extractJavaCallsWithSource(scanLine, argumentLine string, lineNo int) []JavaCallRecord {
	var calls []JavaCallRecord
	if location := javaChainedMatcherRE.FindStringIndex(scanLine); len(location) == 2 {
		open := strings.Index(scanLine[location[0]:location[1]], "(") + location[0]
		calls = append(calls, JavaCallRecord{
			Method:    "securityMatcher",
			Line:      lineNo,
			Arguments: javaCallArguments(argumentLine, open),
		})
	}
	for _, match := range javaNewCallRE.FindAllStringSubmatch(scanLine, -1) {
		if len(match) == 3 {
			calls = append(calls, JavaCallRecord{TargetOwner: match[1], Method: match[2], Line: lineNo})
		}
	}
	for _, match := range javaCallRE.FindAllStringSubmatchIndex(scanLine, -1) {
		if len(match) == 6 {
			receiver := scanLine[match[2]:match[3]]
			if receiver == "new" {
				continue
			}
			open := strings.Index(scanLine[match[5]:], "(") + match[5]
			calls = append(calls, JavaCallRecord{
				Receiver:  receiver,
				Method:    scanLine[match[4]:match[5]],
				Line:      lineNo,
				Arguments: javaCallArguments(argumentLine, open),
			})
		}
	}
	for _, match := range javaBareCallRE.FindAllStringSubmatchIndex(scanLine, -1) {
		if len(match) != 4 {
			continue
		}
		name := scanLine[match[2]:match[3]]
		if isIgnoredJavaBareCall(name) || isJavaQualifiedCall(scanLine, match[0]) {
			continue
		}
		calls = append(calls, JavaCallRecord{Method: name, Line: lineNo, Arguments: javaCallArguments(argumentLine, match[1]-1)})
	}
	return calls
}

func extractJavaConstructedTypes(line string) []string {
	var result []string
	for _, match := range javaConstructorTypeRE.FindAllStringSubmatch(line, -1) {
		if len(match) == 2 {
			result = append(result, shortJavaName(match[1]))
		}
	}
	return result
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
	requests, _ := extractJavaHTTPRequestsWithPending(line, lineNo, vars, javaPendingHTTPRecord{})
	return requests
}

func extractJavaHTTPRequestsWithPending(line string, lineNo int, vars map[string]string, pending javaPendingHTTPRecord) ([]JavaHTTPCallRecord, javaPendingHTTPRecord) {
	return extractJavaHTTPRequestsWithPendingSource(line, line, lineNo, vars, pending)
}

func extractJavaHTTPRequestsWithSource(scanLine, literalLine string, lineNo int, vars map[string]string) []JavaHTTPCallRecord {
	requests, _ := extractJavaHTTPRequestsWithPendingSource(scanLine, literalLine, lineNo, vars, javaPendingHTTPRecord{})
	return requests
}

func extractJavaHTTPRequestsWithPendingSource(scanLine, literalLine string, lineNo int, vars map[string]string, pending javaPendingHTTPRecord) ([]JavaHTTPCallRecord, javaPendingHTTPRecord) {
	var requests []JavaHTTPCallRecord
	for _, match := range javaCallRE.FindAllStringSubmatchIndex(literalLine, -1) {
		if len(match) != 6 || !javaSourceSpanIsStructural(scanLine, literalLine, match[4], match[5]) {
			continue
		}
		method := strings.ToLower(literalLine[match[4]:match[5]])
		if !isJavaHTTPVerb(method) {
			continue
		}
		receiver := literalLine[match[2]:match[3]]
		open := strings.Index(literalLine[match[5]:], "(") + match[5]
		args := javaCallArguments(literalLine, open)
		if len(args) == 0 {
			if expression, ok := javaInlineURIExpression(literalLine, matchingJavaParen(literalLine, open)+1); ok {
				path, _ := javaHTTPRequestPath(expression, vars)
				requests = append(requests, JavaHTTPCallRecord{
					Receiver: receiver, HTTPMethod: strings.ToUpper(method), Path: path,
					PathExpression: expression, Line: lineNo, Confidence: javaExtractedHTTPCallConfidence(expression, path),
				})
				continue
			}
			pending = javaPendingHTTPRecord{Receiver: receiver, HTTPMethod: strings.ToUpper(method), Line: lineNo}
			continue
		}
		expression := strings.TrimSpace(args[0])
		path, _ := javaHTTPRequestPath(expression, vars)
		requests = append(requests, JavaHTTPCallRecord{
			Receiver: receiver, HTTPMethod: strings.ToUpper(method), Path: path,
			PathExpression: expression, Line: lineNo, Confidence: javaExtractedHTTPCallConfidence(expression, path),
		})
	}
	for _, match := range javaHTTPCallRE.FindAllStringSubmatchIndex(literalLine, -1) {
		if len(match) != 6 || !javaSourceSpanIsStructural(scanLine, literalLine, match[2], match[3]) || isJavaQualifiedCall(literalLine, match[0]) {
			continue
		}
		expression := strings.TrimSpace(literalLine[match[4]:match[5]])
		if path, ok := javaHTTPRequestPath(expression, vars); ok {
			requests = append(requests, JavaHTTPCallRecord{HTTPMethod: strings.ToUpper(literalLine[match[2]:match[3]]), Path: path, PathExpression: expression, Line: lineNo, Confidence: javaExtractedHTTPCallConfidence(expression, path)})
		}
	}
	for _, match := range javaHTTPBuilderRefRE.FindAllStringSubmatchIndex(literalLine, -1) {
		if len(match) != 6 || !javaSourceSpanIsStructural(scanLine, literalLine, match[2], match[3]) {
			continue
		}
		if path, ok := javaHTTPRequestPath(literalLine[match[4]:match[5]], vars); ok {
			expression := strings.TrimSpace(literalLine[match[4]:match[5]])
			requests = append(requests, JavaHTTPCallRecord{HTTPMethod: strings.ToUpper(literalLine[match[2]:match[3]]), Path: path, PathExpression: expression, Line: lineNo, Confidence: javaExtractedHTTPCallConfidence(expression, path)})
		}
	}
	if match := javaHTTPChainVerbRE.FindStringSubmatchIndex(literalLine); len(match) == 4 && javaSourceSpanIsStructural(scanLine, literalLine, match[2], match[3]) {
		pending.HTTPMethod = strings.ToUpper(literalLine[match[2]:match[3]])
		pending.Line = lineNo
	}
	if pending.HTTPMethod != "" {
		if match := javaHTTPChainURIRE.FindStringSubmatchIndex(literalLine); len(match) == 4 {
			uriStart := strings.Index(literalLine, ".uri")
			if uriStart >= 0 && javaSourceSpanIsStructural(scanLine, literalLine, uriStart, uriStart+4) {
				expression := strings.TrimSpace(literalLine[match[2]:match[3]])
				path, _ := javaHTTPRequestPath(expression, vars)
				requests = append(requests, JavaHTTPCallRecord{
					Receiver: pending.Receiver, ClientKind: pending.ClientKind,
					HTTPMethod: pending.HTTPMethod, Path: path, PathExpression: expression, Line: pending.Line,
					Confidence: javaExtractedHTTPCallConfidence(expression, path),
				})
				pending = javaPendingHTTPRecord{}
			}
		}
	}
	if pending.HTTPMethod == "" {
		if match := javaHTTPChainReceiverRE.FindStringSubmatch(literalLine); len(match) == 2 {
			pending.Receiver = match[1]
		}
	}
	return requests, pending
}

func javaExtractedHTTPCallConfidence(expression, path string) string {
	if strings.TrimSpace(path) == "" {
		return "PARTIAL"
	}
	expression = strings.TrimSpace(expression)
	if len(expression) >= 2 && expression[0] == '"' && expression[len(expression)-1] == '"' &&
		len(javaStringLiteralRE.FindAllStringIndex(expression, -1)) == 1 {
		return "EXACT"
	}
	return "RESOLVED"
}

func isJavaHTTPVerb(method string) bool {
	switch strings.ToLower(method) {
	case "get", "post", "put", "delete", "patch":
		return true
	default:
		return false
	}
}

func javaInlineURIExpression(line string, start int) (string, bool) {
	if start < 0 || start >= len(line) {
		return "", false
	}
	relative := strings.Index(line[start:], ".uri")
	if relative < 0 {
		return "", false
	}
	uriStart := start + relative
	open := strings.Index(line[uriStart+4:], "(")
	if open < 0 {
		return "", false
	}
	open += uriStart + 4
	args := javaCallArguments(line, open)
	if len(args) == 0 {
		return "", false
	}
	return strings.TrimSpace(args[0]), true
}

func javaSourceSpanIsStructural(scanLine, literalLine string, start, end int) bool {
	return start >= 0 && end <= len(scanLine) && end <= len(literalLine) && scanLine[start:end] == literalLine[start:end]
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

func extractJavaStringExpression(line string) map[string]string {
	match := javaStringVarLineRE.FindStringSubmatch(line)
	if len(match) != 3 {
		return nil
	}
	return map[string]string{match[1]: strings.TrimSpace(match[2])}
}

func extractJavaStringExpressionAt(lines []string, index int) map[string]string {
	if index < 0 || index >= len(lines) {
		return nil
	}
	if expression := extractJavaStringExpression(strings.TrimSpace(lines[index])); expression != nil {
		return expression
	}
	start := strings.TrimSpace(lines[index])
	match := javaStringVarStartRE.FindStringSubmatch(start)
	if len(match) != 3 || strings.HasSuffix(start, ";") {
		return nil
	}
	name := match[1]
	parts := []string{strings.TrimSpace(match[2])}
	for lineIndex := index + 1; lineIndex < len(lines) && lineIndex <= index+11; lineIndex++ {
		line := strings.TrimSpace(lines[lineIndex])
		if line == "" {
			continue
		}
		complete := strings.HasSuffix(line, ";")
		parts = append(parts, strings.TrimSpace(strings.TrimSuffix(line, ";")))
		if complete {
			return map[string]string{name: strings.TrimSpace(strings.Join(parts, " "))}
		}
	}
	return nil
}

func mergeJavaStringExpressions(existing, next map[string]string) map[string]string {
	if len(next) == 0 {
		return existing
	}
	if existing == nil {
		existing = map[string]string{}
	}
	for name, expression := range next {
		existing[name] = expression
	}
	return existing
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

func extractJavaLocalType(line string) map[string]string {
	match := javaLocalTypeLineRE.FindStringSubmatch(line)
	if len(match) != 3 {
		return nil
	}
	typeName := cleanJavaType(match[1])
	if typeName == "String" {
		return nil
	}
	return map[string]string{match[2]: typeName}
}

func mergeJavaLocalTypes(existing, next map[string]string) map[string]string {
	if len(next) == 0 {
		return existing
	}
	if existing == nil {
		existing = map[string]string{}
	}
	for name, typeName := range next {
		existing[name] = typeName
	}
	return existing
}

func resolveJavaHTTPClientKinds(source *JavaSourceRecord) {
	if source == nil {
		return
	}
	imported := map[string]string{}
	for _, record := range source.Imports {
		if record.Static {
			continue
		}
		if kind := javaSpringHTTPClientImport(record.Name); kind != "" {
			imported[kind] = record.Name
		}
	}
	if len(imported) == 0 {
		return
	}
	for methodIndex := range source.Methods {
		method := &source.Methods[methodIndex]
		for requestIndex := range method.HTTPRequests {
			request := &method.HTTPRequests[requestIndex]
			receiver := request.Receiver
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
			if _, ok := imported[kind]; ok {
				request.ClientKind = kind
			}
		}
	}
}

func resolveJavaHTTPCallConfidence(source *JavaSourceRecord) {
	if source == nil {
		return
	}
	for methodIndex := range source.Methods {
		method := &source.Methods[methodIndex]
		for requestIndex := range method.HTTPRequests {
			request := &method.HTTPRequests[requestIndex]
			expression := strings.TrimSpace(request.PathExpression)
			if request.Path == "" {
				if value, ok := method.StringVars[expression]; ok {
					request.Path = value
				} else if value, ok := source.Constants[expression]; ok {
					request.Path = value
				}
			}
			request.Confidence = javaExtractedHTTPCallConfidence(expression, request.Path)
		}
	}
}

func javaSpringHTTPClientImport(importName string) string {
	switch strings.TrimSpace(importName) {
	case "org.springframework.web.client.RestClient":
		return "RestClient"
	case "org.springframework.web.reactive.function.client.WebClient":
		return "WebClient"
	case "org.springframework.web.client.RestTemplate":
		return "RestTemplate"
	default:
		return ""
	}
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
	quote := rune(0)
	escaped := false
	for index, r := range value {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
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
	return strings.Trim(strings.TrimSpace(value), `"`)
}
