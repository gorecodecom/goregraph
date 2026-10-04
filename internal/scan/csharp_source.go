package scan

import (
	"strings"
)

type csharpAttribute struct {
	name         string
	value        string
	literal      bool
	hasArguments bool
	line         int
}
type csharpMember struct {
	symbol         RichSymbolRecord
	owner          string
	typeName       string
	parameters     map[string]string
	parameterTypes []string
	parameterInfo  []csharpParameter
	attributes     []csharpAttribute
	start, end     int
	static         bool
}
type csharpType struct {
	symbol     RichSymbolRecord
	bases      []string
	attributes []csharpAttribute
}
type csharpSource struct {
	file           string
	module         string
	visibleModules map[string]bool
	tokens         []csharpToken
	pairs          map[int]int
	imports        []string
	aliases        map[string]string
	types          []csharpType
	members        []csharpMember
	limitations    []string
}

func parseCSharpSource(file FileRecord, body string) csharpSource {
	source := csharpSource{file: file.Path, tokens: csharpTokens(body), aliases: map[string]string{}}
	pairs, balanced := csharpPairs(source.tokens)
	source.pairs = pairs
	if !balanced {
		source.limitations = append(source.limitations, "unbalanced or unsupported source syntax")
	}
	if strings.Contains(body, "#if") {
		source.limitations = append(source.limitations, "conditional compilation is not evaluated")
	}
	for i, token := range source.tokens {
		if token.text != "using" || token.kind != "identifier" {
			continue
		}
		end := i + 1
		for end < len(source.tokens) && source.tokens[end].text != ";" && source.tokens[end].text != "(" {
			end++
		}
		if end == len(source.tokens) || source.tokens[end].text != ";" {
			continue
		}
		parts := source.tokens[i+1 : end]
		if len(parts) >= 3 && parts[1].text == "=" {
			source.aliases[parts[0].text] = csharpJoined(parts[2:])
		} else if len(parts) > 0 && parts[0].text != "static" {
			source.imports = append(source.imports, csharpJoined(parts))
		}
	}
	source.parseRegion(0, len(source.tokens), "")
	if member, ok := csharpTopLevelMember(source); ok {
		source.members = append(source.members, member)
	}
	return source
}

func csharpTypeKeyword(token csharpToken) bool {
	if token.kind != "identifier" {
		return false
	}
	switch token.text {
	case "class", "struct", "interface", "record", "enum":
		return true
	}
	return false
}

func (s *csharpSource) parseRegion(begin, end int, prefix string) {
	segment := begin
	for i := begin; i < end; i++ {
		t := s.tokens[i]
		if t.kind != "identifier" {
			continue
		}
		if t.text == "namespace" {
			j := i + 1
			for j < end && s.tokens[j].text != "{" && s.tokens[j].text != ";" {
				j++
			}
			if j == end {
				continue
			}
			ns := csharpQualified(prefix, csharpJoined(s.tokens[i+1:j]))
			if s.tokens[j].text == ";" {
				prefix = ns
				i = j
				segment = i + 1
				continue
			}
			if close, ok := s.pairs[j]; ok {
				s.parseRegion(j+1, close, ns)
				i = close
				segment = i + 1
			}
			continue
		}
		if csharpTypeKeyword(t) {
			next := s.parseType(i, end, prefix, s.attributes(segment, i))
			if next > i {
				i = next
				segment = i + 1
			}
		}
	}
}

func (s *csharpSource) parseType(at, end int, prefix string, attributes []csharpAttribute) int {
	name := at + 1
	if name < end && (s.tokens[name].text == "class" || s.tokens[name].text == "struct") {
		name++
	}
	if name >= end || s.tokens[name].kind != "identifier" {
		return at
	}
	open := name + 1
	for open < end && s.tokens[open].text != "{" && s.tokens[open].text != ";" {
		open++
	}
	if open >= end {
		return at
	}
	close, ok := s.pairs[open]
	if !ok {
		close = open
	}
	qualified := csharpQualified(prefix, s.tokens[name].text)
	symbol := s.declaration(s.tokens[at].text, s.tokens[name].text, qualified, prefix, s.tokens[at].line)
	bases := []string{}
	for i := name + 1; i < open; i++ {
		if s.tokens[i].text == ":" {
			for _, part := range csharpSplit(s.tokens[i+1:open], ",") {
				bases = append(bases, csharpJoined(part))
			}
			break
		}
	}
	s.types = append(s.types, csharpType{symbol: symbol, bases: bases, attributes: attributes})
	if s.tokens[open].text == "{" && ok && s.tokens[at].text != "enum" {
		s.parseMembers(open+1, close, qualified, s.tokens[name].text)
	}
	return close
}

func (s *csharpSource) declaration(kind, name, qualified, owner string, line int) RichSymbolRecord {
	return RichSymbolRecord{ID: StableWorkspaceSymbolID(kind, "", "", "csharp", qualified, s.file), Name: name, Kind: kind, Language: "csharp", File: s.file, Line: line, QualifiedName: qualified, Owner: owner, SourceLocation: sourceLocation(line), Analyzer: "csharp-source", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: append([]string{"static source analysis; runtime dispatch, reflection and compiler binding are not evaluated"}, s.limitations...)}
}

func (s *csharpSource) parseMembers(begin, end int, owner, typeName string) {
	for at := begin; at < end; {
		start := at
		for at < end && s.tokens[at].text == "[" {
			close, ok := s.pairs[at]
			if !ok {
				return
			}
			at = close + 1
		}
		attrs := s.attributes(start, at)
		i := at
		for i < end && s.tokens[i].text != "(" && s.tokens[i].text != "{" && s.tokens[i].text != ";" && s.tokens[i].text != "=>" {
			if csharpTypeKeyword(s.tokens[i]) {
				next := s.parseType(i, end, owner, attrs)
				if next > i {
					at = next + 1
					i = -1
					break
				}
			}
			i++
		}
		if i < 0 {
			continue
		}
		if i >= end {
			return
		}
		nameAt := i - 1
		if nameAt > at && s.tokens[nameAt].text == ">" {
			depth := 1
			for nameAt--; nameAt > at; nameAt-- {
				if s.tokens[nameAt].text == ">" {
					depth++
				}
				if s.tokens[nameAt].text == "<" {
					depth--
					if depth == 0 {
						nameAt--
						break
					}
				}
			}
		}
		if s.tokens[i].text == "(" && nameAt >= at && s.tokens[nameAt].kind == "identifier" && !csharpHas(s.tokens[at:i], "=") {
			paramEnd, ok := s.pairs[i]
			if !ok {
				return
			}
			bodyStart := paramEnd + 1
			for bodyStart < end && s.tokens[bodyStart].text != "{" && s.tokens[bodyStart].text != "=>" && s.tokens[bodyStart].text != ";" {
				bodyStart++
			}
			if bodyStart >= end {
				return
			}
			memberEnd := bodyStart
			if s.tokens[bodyStart].text == "{" {
				var found bool
				memberEnd, found = s.pairs[bodyStart]
				if !found {
					return
				}
			} else if s.tokens[bodyStart].text == "=>" {
				for memberEnd < end && s.tokens[memberEnd].text != ";" {
					memberEnd++
				}
			}
			name := s.tokens[nameAt].text
			parameters, types := csharpParameters(s.tokens[i+1 : paramEnd])
			info := csharpParameterInfo(s.tokens[i+1 : paramEnd])
			var signature []string
			for _, parameter := range info {
				signature = append(signature, strings.TrimSpace(parameter.modifier+" "+parameter.typeName))
			}
			qualified := owner + "." + name + "(" + strings.Join(signature, ",") + ")"
			kind := "method"
			if name == typeName {
				kind = "constructor"
			}
			member := csharpMember{symbol: s.declaration(kind, name, qualified, owner, s.tokens[nameAt].line), owner: owner, parameters: parameters, parameterTypes: types, parameterInfo: info, attributes: attrs, start: bodyStart + 1, end: memberEnd, static: csharpHas(s.tokens[at:i], "static")}
			if nameAt > at {
				member.typeName = csharpJoined(csharpWithoutModifiers(s.tokens[at:nameAt]))
			}
			s.members = append(s.members, member)
			at = memberEnd + 1
			continue
		}
		if s.tokens[i].text == "{" {
			close, ok := s.pairs[i]
			if !ok {
				return
			}
			if i > at && s.tokens[i-1].kind == "identifier" && (csharpHas(s.tokens[i+1:close], "get") || csharpHas(s.tokens[i+1:close], "set") || csharpHas(s.tokens[i+1:close], "init")) {
				name := s.tokens[i-1].text
				s.members = append(s.members, csharpMember{symbol: s.declaration("property", name, owner+"."+name, owner, s.tokens[i-1].line), owner: owner, typeName: csharpJoined(csharpWithoutModifiers(s.tokens[at : i-1])), attributes: attrs, start: i + 1, end: close})
			}
			at = close + 1
			continue
		}
		// Field initializers may contain calls or collection initializers. Skip
		// their balanced ranges instead of interpreting them as declarations.
		stop := i
		for stop < end && s.tokens[stop].text != ";" {
			if close, ok := s.pairs[stop]; ok && close > stop {
				stop = close
			}
			stop++
		}
		parts := s.tokens[at:stop]
		equal := len(parts)
		for j, t := range parts {
			if t.text == "=" {
				equal = j
				break
			}
		}
		prefix := csharpWithoutModifiers(parts[:equal])
		if len(prefix) >= 2 && prefix[len(prefix)-1].kind == "identifier" {
			name := prefix[len(prefix)-1]
			s.members = append(s.members, csharpMember{symbol: s.declaration("field", name.text, owner+"."+name.text, owner, name.line), owner: owner, typeName: csharpJoined(prefix[:len(prefix)-1]), attributes: attrs})
		}
		at = stop + 1
	}
}

func (s *csharpSource) attributes(begin, end int) []csharpAttribute {
	var attrs []csharpAttribute
	for i := begin; i < end; i++ {
		if s.tokens[i].text != "[" {
			continue
		}
		close, ok := s.pairs[i]
		if !ok || close > end {
			continue
		}
		for _, part := range csharpSplit(s.tokens[i+1:close], ",") {
			if len(part) == 0 {
				continue
			}
			nameEnd := 0
			for nameEnd < len(part) && part[nameEnd].text != "(" {
				nameEnd++
			}
			attr := csharpAttribute{name: csharpJoined(part[:nameEnd]), line: part[0].line, hasArguments: nameEnd+2 < len(part)}
			if nameEnd+3 == len(part) && part[nameEnd+1].kind == "literal" && part[nameEnd+2].text == ")" {
				attr.value = part[nameEnd+1].text
				attr.literal = true
			}
			attrs = append(attrs, attr)
		}
		i = close
	}
	return attrs
}

func csharpQualified(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
func csharpHas(tokens []csharpToken, value string) bool {
	for _, t := range tokens {
		if t.kind != "literal" && t.kind != "dynamic_literal" && t.text == value {
			return true
		}
	}
	return false
}
func csharpSplit(tokens []csharpToken, separator string) [][]csharpToken {
	var result [][]csharpToken
	start, depth := 0, 0
	for i, t := range tokens {
		if t.kind != "punctuation" {
			continue
		}
		switch t.text {
		case "(", "[", "{", "<":
			depth++
		case ")", "]", "}", ">":
			depth--
		}
		if t.text == separator && depth == 0 {
			result = append(result, tokens[start:i])
			start = i + 1
		}
	}
	if start < len(tokens) {
		result = append(result, tokens[start:])
	}
	return result
}
func csharpWithoutModifiers(tokens []csharpToken) []csharpToken {
	for len(tokens) > 0 && strings.Contains(" public private protected internal static readonly const async virtual override abstract sealed partial extern unsafe new ref out in params this required volatile ", " "+tokens[0].text+" ") {
		tokens = tokens[1:]
	}
	return tokens
}
func csharpParameters(tokens []csharpToken) (map[string]string, []string) {
	parameters := map[string]string{}
	var types []string
	for _, part := range csharpSplit(tokens, ",") {
		for i, t := range part {
			if t.text == "=" {
				part = part[:i]
				break
			}
		}
		part = csharpWithoutModifiers(part)
		if len(part) < 2 {
			types = append(types, "unknown")
			continue
		}
		name := part[len(part)-1].text
		typ := csharpJoined(part[:len(part)-1])
		parameters[name] = typ
		types = append(types, typ)
	}
	return parameters, types
}
func csharpAttributeName(attr csharpAttribute) string {
	name := attr.name
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, "Attribute")
}
func csharpParameterCount(tokens []csharpToken) int {
	for _, token := range tokens {
		if token.kind == "punctuation" && (token.text == "<" || token.text == ">") {
			return -1
		}
	}
	return len(csharpSplit(tokens, ","))
}
