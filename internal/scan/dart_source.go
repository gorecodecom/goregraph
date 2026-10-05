package scan

import (
	"path"
	"strings"
)

type dartDirective struct {
	kind, uri, prefix     string
	show, hide            []string
	conditional, deferred bool
	line                  int
}
type dartParameter struct {
	name, typeName            string
	named, optional, required bool
}
type dartMember struct {
	symbol     RichSymbolRecord
	typeName   string
	parameters []dartParameter
	start, end int
	static     bool
}
type dartType struct {
	symbol     RichSymbolRecord
	start, end int
	bases      []string
}
type dartSource struct {
	file, library, packageName, packageRoot, partOf, declaredLibrary string
	tokens                                                           []dartToken
	pairs                                                            map[int]int
	directives                                                       []dartDirective
	types                                                            []dartType
	members                                                          []dartMember
	limitations                                                      []string
}
type dartAnalysis struct {
	facts        ProjectSymbolFacts
	code         CodeIntelligenceRecord
	graph        CallGraphRecord
	tests        []TestMapRecord
	capabilities []ArchitectureCapabilityFact
}

func parseDartSource(file FileRecord, body string) dartSource {
	tokens, lexical := dartTokens(body)
	pairs, balanced := dartPairs(tokens)
	s := dartSource{file: file.Path, library: file.Path, tokens: tokens, pairs: pairs}
	if !lexical || !balanced {
		s.limitations = append(s.limitations, "malformed or unbalanced Dart syntax")
	}
	s.parseRegion(0, len(tokens), "")
	return s
}

func (s *dartSource) symbol(kind, name, owner string, line int) RichSymbolRecord {
	qualified := s.library + "::"
	if owner != "" {
		qualified += owner + "."
	}
	qualified += name
	return RichSymbolRecord{ID: StableWorkspaceSymbolID(kind, "", "", "dart", qualified, s.file), Name: name, Kind: kind, Language: "dart", File: s.file, Line: line, Owner: owner, QualifiedName: qualified, SourceLocation: sourceLocation(line), Analyzer: "dart-source", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: append([]string{"static Dart source; runtime dispatch and generated code not present in source are not evaluated"}, s.limitations...)}
}

func (s *dartSource) parseRegion(start, end int, owner string) {
	t := s.tokens
	for i := start; i < end; {
		if t[i].text == ";" || t[i].text == "," {
			i++
			continue
		}
		if t[i].text == "@" {
			i++
			for i < end && (t[i].kind == "identifier" || t[i].text == ".") {
				i++
			}
			if i < end && t[i].text == "(" {
				if close, ok := s.pairs[i]; ok {
					i = close + 1
				}
			}
			continue
		}
		if owner == "" && (t[i].text == "import" || t[i].text == "export" || t[i].text == "part" || t[i].text == "library") {
			j := i + 1
			for j < end && t[j].text != ";" {
				j++
			}
			s.parseDirective(i, j)
			i = j + 1
			continue
		}
		declaration := i
		for declaration < end && dartModifier(t[declaration].text) {
			declaration++
		}
		if declaration < end && dartTypeKeyword(t[declaration].text) {
			keyword := t[declaration].text
			nameAt := declaration + 1
			if keyword == "extension" && nameAt < end && t[nameAt].text == "type" {
				keyword = "extension_type"
				nameAt++
			}
			if nameAt < end && t[nameAt].text == "const" {
				nameAt++
			}
			if nameAt >= end {
				break
			}
			name := t[nameAt].text
			if keyword == "extension" && name == "on" {
				name = "<extension@" + sourceLocation(t[nameAt].line) + ">"
				nameAt--
			}
			j := nameAt + 1
			for j < end && t[j].text != "{" && t[j].text != ";" {
				if t[j].text == "(" {
					if close, ok := s.pairs[j]; ok {
						j = close + 1
						continue
					}
				}
				j++
			}
			if keyword == "typedef" {
				s.types = append(s.types, dartType{symbol: s.symbol("typealias", name, owner, t[nameAt].line)})
				i = j + 1
				continue
			}
			if j >= end {
				break
			}
			primaryOpen := -1
			for at := nameAt + 1; at < j; at++ {
				if t[at].text == "(" {
					primaryOpen = at
					break
				}
			}
			if primaryOpen >= 0 {
				if close, ok := s.pairs[primaryOpen]; ok {
					constructorName := dartJoined(t[nameAt:primaryOpen])
					parameters := dartParameters(t[primaryOpen+1 : close])
					s.members = append(s.members, dartMember{symbol: s.symbol("constructor", constructorName, name, t[nameAt].line), parameters: parameters, typeName: name, start: j, end: j})
					for _, parameter := range parameters {
						declaring := keyword == "extension_type"
						for at := primaryOpen + 1; at < close; at++ {
							if t[at].text == parameter.name {
								begin := at - 1
								for begin > primaryOpen && t[begin].text != "," && t[begin].text != "{" {
									if t[begin].text == "var" || t[begin].text == "final" {
										declaring = true
									}
									begin--
								}
								break
							}
						}
						if declaring {
							s.members = append(s.members, dartMember{symbol: s.symbol("field", parameter.name, name, t[nameAt].line), typeName: parameter.typeName, start: j, end: j})
						}
					}
				}
			}
			if t[j].text != "{" {
				s.types = append(s.types, dartType{symbol: s.symbol(keyword, name, owner, t[nameAt].line), bases: dartBaseTypes(t[nameAt+1 : j]), start: j, end: j})
				i = j + 1
				continue
			}
			close, ok := s.pairs[j]
			if !ok {
				break
			}
			bases := dartBaseTypes(t[nameAt+1 : j])
			sym := s.symbol(keyword, name, owner, t[nameAt].line)
			s.types = append(s.types, dartType{symbol: sym, bases: bases, start: j, end: close})
			bodyStart := j + 1
			if keyword == "enum" {
				for bodyStart < close && t[bodyStart].text != ";" {
					if t[bodyStart].kind == "identifier" && (bodyStart == j+1 || t[bodyStart-1].text == ",") {
						s.members = append(s.members, dartMember{symbol: s.symbol("enumcase", t[bodyStart].text, name, t[bodyStart].line), typeName: name, start: bodyStart, end: bodyStart})
					}
					if t[bodyStart].text == "(" || t[bodyStart].text == "{" {
						if e, ok := s.pairs[bodyStart]; ok {
							bodyStart = e + 1
							continue
						}
					}
					bodyStart++
				}
				bodyStart++
			}
			s.parseRegion(bodyStart, close, name)
			i = close + 1
			continue
		}
		next := s.parseMember(i, end, owner)
		if next <= i {
			next = i + 1
		}
		i = next
	}
}

func dartModifier(text string) bool {
	switch text {
	case "abstract", "base", "interface", "final", "sealed", "static", "external", "late", "covariant", "const", "factory":
		return true
	}
	return false
}
func dartTypeKeyword(text string) bool {
	switch text {
	case "class", "mixin", "enum", "extension", "typedef":
		return true
	}
	return false
}

func dartBaseTypes(tokens []dartToken) []string {
	var bases []string
	active := false
	for i := 0; i < len(tokens); i++ {
		if tokens[i].text == "extends" || tokens[i].text == "implements" || tokens[i].text == "with" || tokens[i].text == "on" {
			active = true
			continue
		}
		if !active || tokens[i].kind != "identifier" {
			continue
		}
		start := i
		for i+2 < len(tokens) && tokens[i+1].text == "." && tokens[i+2].kind == "identifier" {
			i += 2
		}
		bases = append(bases, dartJoined(tokens[start:i+1]))
		if i+1 < len(tokens) && tokens[i+1].text == "<" {
			depth := 1
			i += 2
			for i < len(tokens) && depth > 0 {
				if tokens[i].text == "<" {
					depth++
				}
				if tokens[i].text == ">" {
					depth--
				}
				i++
			}
			i--
		}
	}
	return bases
}

func (s *dartSource) parseDirective(start, end int) {
	t := s.tokens
	kind := t[start].text
	if kind == "part" && start+1 < end && t[start+1].text == "of" {
		s.partOf = dartJoined(t[start+2 : end])
		if start+3 == end {
			if value, ok := dartLiteral(t[start+2]); ok {
				s.partOf = value
			}
		}
		return
	}
	if kind == "library" {
		s.declaredLibrary = dartJoined(t[start+1 : end])
		return
	}
	d := dartDirective{kind: kind, line: t[start].line}
	if start+1 >= end {
		return
	}
	uri, ok := dartLiteral(t[start+1])
	if !ok {
		s.limitations = append(s.limitations, "nonliteral Dart library directive")
		return
	}
	d.uri = uri
	mode := ""
	for i := start + 2; i < end; i++ {
		switch t[i].text {
		case "if":
			d.conditional = true
		case "deferred":
			d.deferred = true
		case "as":
			if i+1 < end {
				d.prefix = t[i+1].text
				i++
			}
		case "show", "hide":
			mode = t[i].text
		default:
			if t[i].kind == "identifier" {
				if mode == "show" {
					d.show = append(d.show, t[i].text)
				}
				if mode == "hide" {
					d.hide = append(d.hide, t[i].text)
				}
			}
		}
	}
	s.directives = append(s.directives, d)
}

func (s *dartSource) parseMember(start, end int, owner string) int {
	t := s.tokens
	paren, equals, getter := -1, -1, -1
	j := start
	for j < end {
		text := t[j].text
		if text == "get" || text == "set" {
			getter = j
		}
		if text == "=" || text == "=>" {
			equals = j
			break
		}
		if text == "(" {
			paren = j
			break
		}
		if text == ";" || text == "{" {
			break
		}
		j++
	}
	if paren >= 0 && equals < 0 {
		close, ok := s.pairs[paren]
		if !ok || close >= end {
			return end
		}
		nameAt := paren - 1
		if nameAt >= start && t[nameAt].text == ">" {
			depth := 1
			nameAt--
			for nameAt >= start && depth > 0 {
				if t[nameAt].text == ">" {
					depth++
				}
				if t[nameAt].text == "<" {
					depth--
				}
				nameAt--
			}
		}
		if nameAt < start || t[nameAt].kind != "identifier" {
			return close + 1
		}
		name := t[nameAt].text
		kind := "function"
		if owner != "" {
			kind = "method"
		}
		nameStart := nameAt
		if name == owner {
			kind = "constructor"
		} else if nameAt-2 >= start && t[nameAt-1].text == "." && t[nameAt-2].text == owner {
			name = owner + "." + name
			kind = "constructor"
			nameStart = nameAt - 2
		}
		if getter >= start {
			kind = "setter"
		}
		k := close + 1
		for k < end && t[k].text != "{" && t[k].text != "=>" && t[k].text != ";" {
			if t[k].text == "(" {
				if e, ok := s.pairs[k]; ok {
					k = e + 1
					continue
				}
			}
			k++
		}
		bodyStart, bodyEnd := k, k
		if k < end && t[k].text == "{" {
			if e, ok := s.pairs[k]; ok {
				bodyEnd = e
				k = e
			}
		} else if k < end && t[k].text == "=>" {
			bodyStart = k + 1
			k++
			for k < end && t[k].text != ";" {
				if t[k].text == "(" || t[k].text == "[" || t[k].text == "{" {
					if e, ok := s.pairs[k]; ok {
						k = e + 1
						continue
					}
				}
				k++
			}
			bodyEnd = k
		} else {
			bodyStart = close
			bodyEnd = close
		}
		member := dartMember{symbol: s.symbol(kind, name, owner, t[nameAt].line), typeName: dartDeclaredType(t[start:nameStart]), parameters: dartParameters(t[paren+1 : close]), start: bodyStart, end: bodyEnd, static: dartContains(t[start:nameStart], "static")}
		if kind == "constructor" {
			member.typeName = owner
		}
		s.members = append(s.members, member)
		return k + 1
	}
	if getter >= start && getter+1 < end && t[getter].text == "get" {
		nameAt := getter + 1
		k := j
		bodyStart, bodyEnd := k, k
		if k < end && t[k].text == "{" {
			if e, ok := s.pairs[k]; ok {
				bodyEnd = e
				k = e
			}
		} else {
			for k < end && t[k].text != ";" {
				if e, ok := s.pairs[k]; ok && e > k {
					k = e + 1
					continue
				}
				k++
			}
			bodyEnd = k
		}
		s.members = append(s.members, dartMember{symbol: s.symbol("getter", t[nameAt].text, owner, t[nameAt].line), typeName: dartDeclaredType(t[start:getter]), start: bodyStart, end: bodyEnd})
		return k + 1
	}
	// Initializers, collections, and closure bodies belong to the field, not new declarations.
	k := j
	for k < end && t[k].text != ";" {
		if e, ok := s.pairs[k]; ok && e > k {
			k = e + 1
			continue
		}
		k++
	}
	declarationEnd := j
	if equals >= 0 {
		declarationEnd = equals
	}
	firstComma := declarationEnd
	for at := start; at < declarationEnd; at++ {
		if t[at].text == "," {
			firstComma = at
			break
		}
	}
	firstName := firstComma - 1
	if firstName >= start && t[firstName].kind == "identifier" {
		kind := "variable"
		if owner != "" {
			kind = "field"
		}
		declared := dartDeclaredType(t[start:firstName])
		for _, chunk := range dartSplit(t[firstName:k]) {
			if len(chunk) == 0 || chunk[0].kind != "identifier" {
				continue
			}
			typeName := declared
			if typeName == "" && len(chunk) > 2 && chunk[1].text == "=" {
				typeName = dartConstructorType(chunk[2:])
			}
			s.members = append(s.members, dartMember{symbol: s.symbol(kind, chunk[0].text, owner, chunk[0].line), typeName: typeName, start: j, end: k, static: dartContains(t[start:firstName], "static")})
		}
	}
	return k + 1
}

func dartContains(tokens []dartToken, name string) bool {
	for _, t := range tokens {
		if t.text == name {
			return true
		}
	}
	return false
}
func dartDeclaredType(tokens []dartToken) string {
	var filtered []dartToken
	for _, t := range tokens {
		if dartModifier(t.text) || t.text == "var" || t.text == "required" || t.text == "async" {
			continue
		}
		filtered = append(filtered, t)
	}
	return dartJoined(filtered)
}
func dartConstructorType(tokens []dartToken) string {
	i := 0
	for i < len(tokens) && (tokens[i].text == "new" || tokens[i].text == "const" || tokens[i].text == "await") {
		i++
	}
	start := i
	for i < len(tokens) && (tokens[i].kind == "identifier" || tokens[i].text == ".") {
		i++
	}
	if i < len(tokens) && tokens[i].text == "<" {
		depth := 1
		i++
		for i < len(tokens) && depth > 0 {
			if tokens[i].text == "<" {
				depth++
			}
			if tokens[i].text == ">" {
				depth--
			}
			i++
		}
	}
	if i < len(tokens) && tokens[i].text == "(" && i > start {
		return dartJoined(tokens[start:i])
	}
	return ""
}
func dartParameters(tokens []dartToken) []dartParameter {
	var result []dartParameter
	// Named and optional positional sections are parameter lists, not nested expressions.
	for _, chunk := range dartSplit(tokens) {
		named, optional := false, false
		if len(chunk) > 0 && (chunk[0].text == "{" || chunk[0].text == "[") {
			named = chunk[0].text == "{"
			optional = true
			chunk = chunk[1:]
			if len(chunk) > 0 {
				chunk = chunk[:len(chunk)-1]
			}
		}
		for _, part := range dartSplit(chunk) {
			p := dartParameter{named: named, optional: optional}
			cut := len(part)
			for i, t := range part {
				if t.text == "required" {
					p.required = true
				}
				if t.text == "=" || t.text == ":" {
					cut = i
					break
				}
			}
			part = part[:cut]
			if len(part) == 0 {
				continue
			}
			nameAt := len(part) - 1
			if part[nameAt].kind != "identifier" {
				continue
			}
			p.name = part[nameAt].text
			p.typeName = dartDeclaredType(part[:nameAt])
			if nameAt >= 2 && (part[nameAt-2].text == "this" || part[nameAt-2].text == "super") && part[nameAt-1].text == "." {
				p.typeName = ""
			}
			result = append(result, p)
		}
	}
	return result
}

func assignDartPackages(sources []dartSource, packages []DartPackageRecord) {
	for i := range sources {
		s := &sources[i]
		best := -1
		for _, p := range packages {
			root := path.Dir(p.Path)
			if root == "." {
				root = ""
			}
			if (root == "" || s.file == root || strings.HasPrefix(s.file, root+"/")) && len(root) > best {
				best = len(root)
				s.packageName = p.Name
				s.packageRoot = root
			}
		}
	}
}
