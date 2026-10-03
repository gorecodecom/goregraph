package scan

import (
	"path"
	"strings"
)

type swiftParameter struct {
	label, name, typeName string
	optional, variadic    bool
}
type swiftMember struct {
	symbol          RichSymbolRecord
	owner, typeName string
	parameters      []swiftParameter
	attributes      []string
	start, end      int
	static          bool
}
type swiftType struct {
	symbol RichSymbolRecord
	bases  []string
}
type swiftSource struct {
	file, module string
	tokens       []csharpToken
	pairs        map[int]int
	imports      []string
	types        []swiftType
	members      []swiftMember
	limitations  []string
}

func parseSwiftSource(file FileRecord, body string) swiftSource {
	s := swiftSource{file: file.Path, module: "project", tokens: swiftTokens(body)}
	for _, marker := range []string{"Sources/", "Tests/"} {
		if at := strings.Index(file.Path, marker); at >= 0 {
			parts := strings.Split(file.Path[at+len(marker):], "/")
			if len(parts) > 1 {
				s.module = path.Join(file.Path[:at], parts[0])
			}
		}
	}
	var balanced bool
	s.pairs, balanced = csharpPairs(s.tokens)
	if !balanced {
		s.limitations = append(s.limitations, "unbalanced source syntax")
	}
	for i, t := range s.tokens {
		if t.text == "#" && i+1 < len(s.tokens) && s.tokens[i+1].text == "if" {
			s.limitations = append(s.limitations, "conditional compilation is not evaluated")
			break
		}
	}
	for i, t := range s.tokens {
		if t.text == "import" && i+1 < len(s.tokens) && s.tokens[i+1].kind == "identifier" {
			at := i + 1
			switch s.tokens[at].text {
			case "class", "struct", "enum", "protocol", "func", "var", "let", "typealias":
				at++
			}
			if at < len(s.tokens) {
				s.imports = append(s.imports, s.tokens[at].text)
			}
		}
	}
	s.region(0, len(s.tokens), s.module, false)
	return s
}

func (s *swiftSource) declaration(kind, name, qualified, owner string, line int) RichSymbolRecord {
	return RichSymbolRecord{ID: StableWorkspaceSymbolID(kind, "", "", "swift", qualified, s.file), Name: name, Kind: kind, Language: "swift", File: s.file, Line: line, Owner: owner, QualifiedName: qualified, Module: s.module, SourceLocation: sourceLocation(line), Analyzer: "swift-source", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: append([]string{"static source patterns; compiler binding, runtime dispatch and macro expansion are not evaluated"}, s.limitations...)}
}

func (s *swiftSource) region(begin, end int, owner string, inType bool) {
	segment := begin
	for i := begin; i < end; i++ {
		t := s.tokens[i]
		if t.kind != "identifier" {
			continue
		}
		switch t.text {
		case "import":
			for i+1 < end && s.tokens[i+1].line == t.line && s.tokens[i+1].text != ";" {
				i++
			}
			segment = i + 1
		case "class", "struct", "enum", "protocol", "actor", "extension":
			if t.text == "class" && i+1 < end && (s.tokens[i+1].text == "func" || s.tokens[i+1].text == "var") {
				continue
			}
			if i+1 >= end || s.tokens[i+1].kind != "identifier" {
				continue
			}
			name := s.tokens[i+1].text
			open := i + 2
			for open < end && s.tokens[open].text != "{" && s.tokens[open].text != ";" {
				open++
			}
			close, ok := s.pairs[open]
			if !ok || open >= end || s.tokens[open].text != "{" {
				continue
			}
			qualified := owner + "." + name
			var bases []string
			for j := i + 2; j < open; j++ {
				if s.tokens[j].text == ":" {
					for _, p := range csharpSplit(s.tokens[j+1:open], ",") {
						bases = append(bases, csharpJoined(p))
					}
					break
				}
			}
			if t.text == "extension" {
				qualified = s.module + "." + name
			} else {
				declaration := s.declaration(t.text, name, qualified, owner, t.line)
				if csharpHas(s.tokens[i+2:open], "<") || csharpHas(s.tokens[i+2:open], "where") {
					declaration.Limitations = append(declaration.Limitations, "generic type constraints are not evaluated")
				}
				s.types = append(s.types, swiftType{declaration, bases})
			}
			memberStart := len(s.members)
			s.region(open+1, close, qualified, true)
			if csharpHas(s.tokens[i+2:open], "where") || csharpHas(s.tokens[i+2:open], "<") {
				for n := memberStart; n < len(s.members); n++ {
					s.members[n].symbol.Limitations = append(s.members[n].symbol.Limitations, "generic type or extension constraints are not evaluated")
				}
			}
			i, segment = close, close+1
		case "func", "init", "deinit":
			if t.text == "deinit" && i+1 < end && s.tokens[i+1].text == "{" {
				if close, ok := s.pairs[i+1]; ok {
					s.members = append(s.members, swiftMember{symbol: s.declaration("destructor", "deinit", owner+".deinit", owner, t.line), owner: owner, start: i + 2, end: close})
					i, segment = close, close+1
				}
				continue
			}
			nameAt := i + 1
			name := t.text
			if t.text == "func" {
				if nameAt >= end || s.tokens[nameAt].kind != "identifier" {
					continue
				}
				name = s.tokens[nameAt].text
			} else {
				nameAt = i
			}
			open := nameAt + 1
			for open < end && s.tokens[open].text != "(" && s.tokens[open].text != "{" && s.tokens[open].line == s.tokens[nameAt].line {
				open++
			}
			paramEnd, ok := s.pairs[open]
			if !ok || open >= end || s.tokens[open].text != "(" {
				continue
			}
			bodyStart := paramEnd + 1
			for bodyStart < end && s.tokens[bodyStart].text != "{" && s.tokens[bodyStart].text != ";" {
				if s.tokens[bodyStart].line > s.tokens[paramEnd].line && swiftDeclarationStart(s.tokens[bodyStart].text) {
					break
				}
				bodyStart++
			}
			memberEnd := paramEnd
			if bodyStart < end && s.tokens[bodyStart].text == "{" {
				if close, ok := s.pairs[bodyStart]; ok {
					memberEnd = close
				}
			}
			params := swiftParameters(s.tokens[open+1 : paramEnd])
			var signature []string
			for _, p := range params {
				signature = append(signature, p.label+":"+p.typeName)
			}
			kind := "method"
			if !inType {
				kind = "function"
			}
			if t.text == "init" {
				kind = "constructor"
			}
			member := swiftMember{symbol: s.declaration(kind, name, owner+"."+name+"("+strings.Join(signature, ",")+")", owner, t.line), owner: owner, parameters: params, attributes: swiftAttributes(s.tokens[segment:i]), start: bodyStart + 1, end: memberEnd, static: csharpHas(s.tokens[segment:i], "static") || csharpHas(s.tokens[segment:i], "class")}
			if csharpHas(s.tokens[nameAt+1:open], "<") || csharpHas(s.tokens[paramEnd+1:bodyStart], "where") {
				member.symbol.Limitations = append(member.symbol.Limitations, "generic method constraints are not evaluated")
			}
			for j := paramEnd + 1; j < bodyStart && j+1 < len(s.tokens); j++ {
				if s.tokens[j].text == "->" {
					member.typeName = csharpJoined(s.tokens[j+1 : bodyStart])
					break
				}
			}
			s.members = append(s.members, member)
			i, segment = memberEnd, memberEnd+1
		case "typealias", "associatedtype":
			if i+1 < end && s.tokens[i+1].kind == "identifier" {
				name := s.tokens[i+1]
				s.members = append(s.members, swiftMember{symbol: s.declaration(t.text, name.text, owner+"."+name.text, owner, name.line), owner: owner})
			}
		case "case":
			isEnum := false
			for _, typ := range s.types {
				if typ.symbol.QualifiedName == owner && typ.symbol.Kind == "enum" {
					isEnum = true
				}
			}
			if !isEnum || i+1 >= end {
				continue
			}
			for j := i + 1; j < end && s.tokens[j].line == t.line; j++ {
				if s.tokens[j].kind == "identifier" && (j == i+1 || s.tokens[j-1].text == ",") {
					name := s.tokens[j]
					s.members = append(s.members, swiftMember{symbol: s.declaration("enum_case", name.text, owner+"."+name.text, owner, name.line), owner: owner})
				}
				if close, ok := s.pairs[j]; ok && close > j {
					j = close
				}
			}
		case "var", "let":
			if i+1 >= end || s.tokens[i+1].kind != "identifier" {
				continue
			}
			name := s.tokens[i+1]
			stop := i + 2
			for stop < end && s.tokens[stop].text != ";" && s.tokens[stop].text != "{" && s.tokens[stop].line == name.line {
				stop++
			}
			typ := ""
			for j := i + 2; j < stop; j++ {
				if s.tokens[j].text == ":" {
					to := j + 1
					for to < stop && s.tokens[to].text != "=" {
						to++
					}
					typ = csharpJoined(s.tokens[j+1 : to])
					break
				}
			}
			if typ == "" {
				for j := i + 2; j+2 < stop; j++ {
					if s.tokens[j].text == "=" && s.tokens[j+1].kind == "identifier" && s.tokens[j+2].text == "(" {
						typ = s.tokens[j+1].text
						break
					}
				}
			}
			kind := "property"
			if !inType {
				kind = "variable"
			}
			member := swiftMember{symbol: s.declaration(kind, name.text, owner+"."+name.text, owner, name.line), owner: owner, typeName: typ, attributes: swiftAttributes(s.tokens[segment:i])}
			if stop < end && s.tokens[stop].text == "{" {
				if close, ok := s.pairs[stop]; ok {
					member.start, member.end = stop+1, close
					stop = close + 1
				}
			}
			s.members = append(s.members, member)
			i, segment = stop-1, stop
		}
	}
}

func swiftDeclarationStart(name string) bool {
	switch name {
	case "func", "var", "let", "init", "class", "struct", "enum", "actor", "protocol", "extension":
		return true
	}
	return false
}

func swiftAttributes(tokens []csharpToken) []string {
	var attrs []string
	for i, t := range tokens {
		if t.text == "@" && i+1 < len(tokens) {
			attrs = append(attrs, tokens[i+1].text)
		}
	}
	return attrs
}

func swiftParameters(tokens []csharpToken) []swiftParameter {
	var result []swiftParameter
	for _, parts := range csharpSplit(tokens, ",") {
		colon := -1
		equal := len(parts)
		for i, t := range parts {
			if t.text == ":" && colon < 0 {
				colon = i
			}
			if t.text == "=" {
				equal = i
				break
			}
		}
		if colon < 1 {
			continue
		}
		name := parts[colon-1].text
		label := name
		if colon > 1 && parts[colon-2].kind == "identifier" {
			label = parts[colon-2].text
		}
		result = append(result, swiftParameter{label: label, name: name, typeName: csharpJoined(parts[colon+1 : equal]), optional: equal < len(parts), variadic: strings.Contains(csharpJoined(parts[colon+1:equal]), "...")})
	}
	return result
}
