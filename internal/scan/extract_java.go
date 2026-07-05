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
	javaMethodLineRE     = regexp.MustCompile(`^\s*(public|protected|private)?\s*(?:static\s+)?(?:final\s+)?([A-Za-z_][A-Za-z0-9_<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)\s*(?:throws\s+[^{]+)?\{?\s*$`)
	javaFieldLineRE      = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(final\s+)?([A-Za-z_][A-Za-z0-9_<>, ?\[\].]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=.*)?;\s*$`)
	javaAnnotationLineRE = regexp.MustCompile(`^\s*@([A-Za-z_][A-Za-z0-9_.]*)(?:\((.*)\))?\s*$`)
	javaConstantLineRE   = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*static\s+final\s+String\s+([A-Za-z0-9_]+)\s*=\s*"([^"]*)"\s*;`)
	javaCallRE           = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

func extractJavaSource(file FileRecord, body string) JavaSourceRecord {
	source := JavaSourceRecord{File: file.Path, Constants: map[string]string{}}
	lines := strings.Split(body, "\n")
	var pending []JavaAnnotationRecord
	currentOwner := ""
	braceDepth := 0
	blockComment := false

	for index, raw := range lines {
		lineNo := index + 1
		line, inBlock := stripJavaLineNoise(raw, blockComment)
		blockComment = inBlock
		if strings.TrimSpace(line) == "" {
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
		if match := javaAnnotationLineRE.FindStringSubmatch(line); len(match) == 3 {
			annotation := JavaAnnotationRecord{Name: shortJavaName(match[1]), Arguments: strings.TrimSpace(match[2]), Attributes: parseJavaAnnotationAttributes(match[2]), Line: lineNo}
			pending = append(pending, annotation)
			source.Annotations = append(source.Annotations, annotation)
			continue
		}
		if match := javaConstantLineRE.FindStringSubmatch(line); len(match) == 3 {
			source.Constants[match[1]] = match[2]
		}
		if match := javaTypeLineRE.FindStringSubmatch(line); len(match) == 4 {
			currentOwner = match[2]
			typ := JavaTypeRecord{
				Name:        match[2],
				Kind:        match[1],
				Package:     source.Package,
				File:        file.Path,
				Line:        lineNo,
				Annotations: pending,
			}
			typ.Extends = parseJavaExtends(match[3])
			typ.Implements = parseJavaImplements(match[3])
			source.Types = append(source.Types, typ)
			pending = nil
		} else if match := javaFieldLineRE.FindStringSubmatch(line); len(match) == 4 && currentOwner != "" && !strings.Contains(line, "(") {
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
		} else if match := javaMethodLineRE.FindStringSubmatch(line); len(match) == 5 && currentOwner != "" && !isJavaControlLine(line) {
			source.Methods = append(source.Methods, JavaMethodRecord{
				Name:        match[3],
				File:        file.Path,
				Line:        lineNo,
				Owner:       currentOwner,
				Visibility:  strings.TrimSpace(match[1]),
				ReturnType:  cleanJavaType(match[2]),
				Parameters:  parseJavaParameters(match[4]),
				Annotations: pending,
				Calls:       extractJavaCalls(line, lineNo),
			})
			pending = nil
		} else if len(source.Methods) > 0 {
			last := &source.Methods[len(source.Methods)-1]
			last.Calls = append(last.Calls, extractJavaCalls(line, lineNo)...)
		}

		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")
		if braceDepth <= 0 {
			currentOwner = ""
			braceDepth = 0
		}
	}

	if len(source.Constants) == 0 {
		source.Constants = nil
	}
	return source
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
	for _, prefix := range []string{"if", "for", "while", "switch", "catch", "return", "new"} {
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
		var annotations []JavaAnnotationRecord
		for strings.HasPrefix(part, "@") {
			fields := strings.Fields(part)
			if len(fields) == 0 {
				break
			}
			annotationName := strings.TrimPrefix(fields[0], "@")
			annotationName = strings.TrimSuffix(annotationName, "()")
			annotations = append(annotations, JavaAnnotationRecord{Name: shortJavaName(annotationName)})
			part = strings.TrimSpace(strings.TrimPrefix(part, fields[0]))
		}
		part = strings.ReplaceAll(part, "final ", "")
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

func extractJavaCalls(line string, lineNo int) []JavaCallRecord {
	var calls []JavaCallRecord
	for _, match := range javaCallRE.FindAllStringSubmatch(line, -1) {
		if len(match) == 3 {
			calls = append(calls, JavaCallRecord{Receiver: match[1], Method: match[2], Line: lineNo})
		}
	}
	return calls
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
