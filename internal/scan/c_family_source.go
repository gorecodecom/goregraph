package scan

import (
	"regexp"
	"strings"
)

type cFamilySource struct {
	file          FileRecord
	tokens        []dartToken
	pairs         map[int]int
	members       []dartMember
	types         []RichSymbolRecord
	includes      []string
	valid         bool
	unsafeMembers map[string]bool
}

var cRawStart = regexp.MustCompile(`(?:u8|u|U|L)?R"([^ ()\\\t\r\n]{0,16})\(`)

func cFamilyTokens(body string) ([]dartToken, bool) {
	masked := []byte(body)
	valid := true
	for at := 0; at < len(body); {
		found := cRawStart.FindStringSubmatchIndex(body[at:])
		if found == nil {
			break
		}
		start := at + found[0]
		delimiter := body[at+found[2] : at+found[3]]
		content := at + found[1]
		end := strings.Index(body[content:], ")"+delimiter+"\"")
		if end < 0 {
			end = len(body)
			valid = false
		} else {
			end += content + len(delimiter) + 2
		}
		for i := start; i < end; i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
		at = end
	}
	continued := false
	offset := 0
	for _, line := range strings.Split(string(masked), "\n") {
		directive := continued || strings.HasPrefix(strings.TrimSpace(line), "#")
		continued = directive && strings.HasSuffix(strings.TrimSpace(line), "\\")
		if directive {
			for i := offset; i < offset+len(line); i++ {
				masked[i] = ' '
			}
		}
		offset += len(line) + 1
	}
	tokens, lexical := dartTokens(string(masked))
	return tokens, valid && lexical
}
func parseCFamilySource(file FileRecord, body string) cFamilySource {
	tokens, valid := cFamilyTokens(body)
	pairs, balanced := dartPairs(tokens)
	s := cFamilySource{file: file, tokens: tokens, pairs: pairs, valid: valid && balanced, unsafeMembers: map[string]bool{}}
	for _, line := range strings.Split(nativeDirectiveMask(body), "\n") {
		if match := cIncludeRE.FindStringSubmatch(line); len(match) == 2 {
			s.includes = append(s.includes, match[1])
		}

	}
	s.valid = s.valid && nativePreprocessorSafe(body)
	for _, token := range tokens {
		if token.text == "template" {
			s.valid = false
		}
	}
	s.region(0, len(tokens), "")
	cGoogleTestMembers(&s)
	return s
}
func (s *cFamilySource) region(start, end int, owner string) {
	t := s.tokens
	for at := start; at < end; at++ {
		if (t[at].text == "namespace" || t[at].text == "class" || t[at].text == "struct" || t[at].text == "union" || t[at].text == "enum") && at+1 < end {
			kind := t[at].text
			nameAt := at + 1
			if (t[nameAt].text == "class" || t[nameAt].text == "struct") && nameAt+1 < end {
				nameAt++
			}
			if kind == "namespace" && t[nameAt].text == "{" {
				if close, ok := s.pairs[nameAt]; ok {
					s.region(nameAt+1, close, owner+"::anonymous@"+sourceLocation(t[at].line))
					at = close
				}
				continue
			}
			if t[nameAt].kind != "identifier" {
				continue
			}
			body := nameAt + 1
			for body < end && t[body].text != "{" && t[body].text != ";" {
				body++
			}
			if body >= end || t[body].text != "{" {
				continue
			}
			close, ok := s.pairs[body]
			if !ok {
				continue
			}
			name := t[nameAt].text
			if kind == "namespace" {
				for j := nameAt + 1; j+2 < body && t[j].text == ":" && t[j+1].text == ":" && t[j+2].kind == "identifier"; j += 3 {
					name += "::" + t[j+2].text
				}
			}
			qualified := name
			if owner != "" {
				qualified = owner + "::" + name
			}
			symbol := supplementaryTokenSymbol(s.file, kind, name, t[nameAt])
			symbol.Owner = owner
			s.types = append(s.types, symbol)
			s.region(body+1, close, qualified)
			at = close
			continue
		}
		if t[at].text == "{" {
			if close, ok := s.pairs[at]; ok {
				s.region(at+1, close, owner)
				at = close
			}
			continue
		}
		if t[at].text != "(" || at == 0 || t[at-1].kind != "identifier" {
			continue
		}
		nameAt := at - 1
		name := t[nameAt].text
		if dartControlWord(name) || name == "noexcept" || name == "decltype" || name == "sizeof" || name == "alignof" {
			continue
		}
		close, ok := s.pairs[at]
		if !ok {
			continue
		}
		body := close + 1
		for body < end && (t[body].text == "const" || t[body].text == "override" || t[body].text == "final" || t[body].text == "noexcept" || t[body].text == "&") {
			body++
		}
		if body >= end || (t[body].text != "{" && t[body].text != ";") {
			continue
		}
		signatureStart := nameAt - 1
		for signatureStart >= start && t[signatureStart].text != ";" && t[signatureStart].text != "{" && t[signatureStart].text != "}" {
			signatureStart--
		}
		prefix := t[signatureStart+1 : nameAt]
		if len(prefix) == 0 && owner == "" || dartContains(prefix, "=") || dartContains(prefix, "return") {
			continue
		}
		functionOwner := owner
		if nameAt >= 3 && t[nameAt-1].text == ":" && t[nameAt-2].text == ":" && t[nameAt-3].kind == "identifier" {
			begin := nameAt - 3
			for begin-3 >= start && t[begin-1].text == ":" && t[begin-2].text == ":" && t[begin-3].kind == "identifier" {
				begin -= 3
			}
			functionOwner = dartJoined(t[begin : nameAt-2])
			if owner != "" && !strings.HasPrefix(functionOwner, owner+"::") {
				functionOwner = owner + "::" + functionOwner
			}
		}
		kind := "function"
		if functionOwner != "" {
			kind = "method"
		}
		symbol := supplementaryTokenSymbol(s.file, kind, name, t[nameAt])
		symbol.Owner = functionOwner
		if dartContains(prefix, "template") || dartContains(prefix, "virtual") || dartContains(prefix, "friend") || dartContains(t[close+1:body], "override") {
			s.unsafeMembers[symbol.ID] = true
		}
		bodyEnd := body
		if t[body].text == "{" {
			if e, ok := s.pairs[body]; ok {
				bodyEnd = e
			}
		}
		parameters := dartParameters(t[at+1 : close])
		if len(parameters) == 1 && parameters[0].name == "void" {
			parameters = nil
		}
		s.members = append(s.members, dartMember{symbol: symbol, parameters: parameters, start: body, end: bodyEnd, static: dartContains(prefix, "static")})
		at = bodyEnd
	}
}

func analyzeCFamilySources(sources []cFamilySource, result *supplementaryAnalysis) {
	files := map[string]cFamilySource{}
	functions := map[string][]dartMember{}
	for _, s := range sources {
		files[s.file.Path] = s
		result.facts.Declarations = append(result.facts.Declarations, s.types...)
		for _, m := range s.members {
			result.facts.Declarations = append(result.facts.Declarations, m.symbol)
			if s.valid && m.start < m.end {
				functions[m.symbol.Owner+"."+m.symbol.Name] = append(functions[m.symbol.Owner+"."+m.symbol.Name], m)
				result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: m.symbol.Name, Owner: m.symbol.Owner, Kind: m.symbol.Kind, Language: s.file.Language, File: s.file.Path, Line: m.symbol.Line, EndLine: s.tokens[m.end].line})
			}
		}
		for _, include := range s.includes {
			result.facts.References = append(result.facts.References, supplementaryReference(s.file, include, "imports_header", 1))
		}
	}
	for _, s := range sources {
		visibleFiles := cVisibleHeaders(s, files)
		for _, m := range s.members {
			if !s.valid || m.start == m.end || s.unsafeMembers[m.symbol.ID] {
				continue
			}
			env := map[string]string{}
			for _, typ := range s.types {
				if (typ.Kind == "class" || typ.Kind == "struct") && (typ.Owner+"::"+typ.Name == m.symbol.Owner || typ.Owner == "" && typ.Name == m.symbol.Owner) && !m.static {
					env["this"] = m.symbol.Owner
				}
			}
			for _, parameter := range m.parameters {
				env[parameter.name] = strings.Trim(parameter.typeName, "*& ")
			}
			for at := 0; at < m.start; at++ {
				if s.tokens[at].line >= m.symbol.Line && at+3 < m.start && s.tokens[at].text == "(" && s.tokens[at+1].text == "*" && s.tokens[at+3].text == ")" {
					env[s.tokens[at+2].text] = ""
				}
			}
			scopes := []map[string]string{}
			for at := m.start + 1; at < m.end; at++ {
				token := s.tokens[at]
				if token.text == "{" {
					scopes = append(scopes, cloneNativeTypes(env))
				}
				if token.text == "}" && len(scopes) > 0 {
					env = scopes[len(scopes)-1]
					scopes = scopes[:len(scopes)-1]
				}
				if token.text == "[" { // Lambda bodies have their own execution identity.
					if end := cLambdaEnd(s, at, m.end); end > at {
						at = end
						continue
					}
				}
				cLocalBinding(s, at, m.end, env)
				if token.text != "(" || at == 0 || s.tokens[at-1].kind != "identifier" {
					continue
				}
				nameAt := at - 1
				name := s.tokens[nameAt].text
				if dartControlWord(name) || name == "sizeof" || name == "decltype" || name == "alignof" || name == "noexcept" {
					continue
				}
				close, ok := s.pairs[at]
				if !ok {
					continue
				}
				owner, explicit, unknown := cCallOwner(s.tokens, nameAt, m.symbol.Owner, env)
				if _, shadow := env[name]; shadow && !explicit {
					continue
				}
				candidates := functions[owner+"."+name]
				if !explicit {
					candidates = cUnqualifiedCandidates(m.symbol.Owner, name, visibleFiles, files, functions)
				}
				var visible []dartMember
				seen := map[string]bool{}
				for _, candidate := range candidates {
					if unknown || !dartAccepts(candidate, s.tokens[at+1:close]) || files[candidate.symbol.File].unsafeMembers[candidate.symbol.ID] {
						continue
					}
					if !cMemberVisible(s, candidate, visibleFiles, files) || seen[candidate.symbol.ID] {
						continue
					}
					seen[candidate.symbol.ID] = true
					visible = append(visible, candidate)
				}
				addNativeCall(result, s.file, m.symbol, name, token.line, at, nativeMemberSymbols(visible), "unique visible C/C++ source declaration; conversions, linking and runtime dispatch are not evaluated")
			}
		}
	}
}
