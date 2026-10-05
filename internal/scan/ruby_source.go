package scan

import (
	"regexp"
	"strings"
)

type rubySource struct {
	file          FileRecord
	tokens        []dartToken
	members       []dartMember
	symbols       []RichSymbolRecord
	valid         bool
	deferred      map[int]int
	imports       []rubyImport
	mixins        map[string][]string
	supers        map[string]string
	testFramework bool
	unsafeOwners  map[string]bool
}

var rubyHeredocStart = regexp.MustCompile(`<<[-~]?["']?([A-Za-z_][A-Za-z0-9_]*)["']?`)

func rubyCodeMask(body string) string {
	translated := strings.ReplaceAll(body, "#{", "${")
	masked := []byte(body)
	blank := func(start, end int) {
		for i := start; i < end; i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	for at := 0; at < len(body); {
		if (at == 0 || body[at-1] == '\n') && strings.HasPrefix(body[at:], "=begin") && (at+6 == len(body) || strings.ContainsRune(" \t\r\n", rune(body[at+6]))) {
			end := len(body)
			cursor := at + 6
			for cursor < len(body) {
				newline := strings.IndexByte(body[cursor:], '\n')
				if newline < 0 {
					break
				}
				cursor += newline + 1
				if strings.HasPrefix(body[cursor:], "=end") && (cursor+4 == len(body) || strings.ContainsRune(" \t\r\n", rune(body[cursor+4]))) {
					end = cursor + 4
					break
				}
			}
			blank(at, end)
			at = end
			continue
		}

		if body[at] == '#' {
			end := at
			for end < len(body) && body[end] != '\n' {
				end++
			}
			blank(at, end)
			at = end
			continue
		}
		if body[at] == '\'' || body[at] == '"' || body[at] == '`' {
			if body[at] == '`' {
				end := at + 1
				for end < len(body) && body[end] != '`' {
					if body[end] == '\\' {
						end++
					}
					end++
				}
				if end < len(body) {
					end++
				}
				blank(at, end)
				at = end
				continue
			}
			end, _ := dartStringEnd(translated, at, 0)
			blank(at, end)
			at = end
			continue
		}
		if strings.HasPrefix(body[at:], "<<") {
			match := rubyHeredocStart.FindStringSubmatchIndex(body[at:])
			if match != nil && match[0] == 0 {
				tag := body[at+match[2] : at+match[3]]
				end := strings.IndexByte(body[at:], '\n')
				if end < 0 {
					blank(at, len(body))
					break
				}
				cursor := at + end + 1
				for cursor < len(body) {
					next := strings.IndexByte(body[cursor:], '\n')
					if next < 0 {
						next = len(body) - cursor
					}
					lineEnd := cursor + next
					if strings.TrimSpace(body[cursor:lineEnd]) == tag {
						cursor = lineEnd
						break
					}
					cursor = lineEnd
					if cursor < len(body) {
						cursor++
					}
				}
				blank(at, cursor)
				at = cursor
				continue
			}
		}
		if body[at] == '/' {
			previous := at - 1
			for previous >= 0 && (body[previous] == ' ' || body[previous] == '\t') {
				previous--
			}
			if previous < 0 || strings.ContainsRune("=(,:;!~\n", rune(body[previous])) {
				end := at + 1
				bracket := false
				for end < len(body) {
					if body[end] == '\\' {
						end += 2
						continue
					}
					if body[end] == '[' {
						bracket = true
					}
					if body[end] == ']' {
						bracket = false
					}
					if body[end] == '/' && !bracket {
						end++
						break
					}
					end++
				}
				if end > len(body) {
					end = len(body)
				}
				blank(at, end)
				at = end
				continue
			}
		}
		if body[at] == '%' && at+2 < len(body) && strings.ContainsRune("qQwWiIxrs", rune(body[at+1])) {
			open := body[at+2]
			close := map[byte]byte{'{': '}', '(': ')', '[': ']', '<': '>'}[open]
			if close == 0 {
				close = open
			}
			depth := 1
			end := at + 3
			for end < len(body) && depth > 0 {
				if body[end] == '\\' {
					end += 2
					continue
				}
				if open != close && body[end] == open {
					depth++
				}
				if body[end] == close {
					depth--
				}
				end++
			}
			if end > len(body) {
				end = len(body)
			}
			blank(at, end)
			at = end
			continue
		}
		at++
	}
	return string(masked)
}
func parseRubySource(file FileRecord, body string) rubySource {
	tokens, valid := dartTokens(rubyCodeMask(body))
	s := rubySource{file: file, tokens: tokens, valid: valid, deferred: map[int]int{}, mixins: map[string][]string{}, supers: map[string]string{}, unsafeOwners: map[string]bool{}}
	type block struct {
		owner       string
		member      int
		start       int
		deferred    bool
		static      bool
		declaration bool
	}
	var stack []block
	owner := ""
	currentStatic := false
	for at, t := range tokens {
		if t.kind != "identifier" {
			continue
		}
		lineStart := at == 0 || tokens[at-1].line < t.line || tokens[at-1].text == ";"
		if t.text == "class_eval" || t.text == "module_eval" || t.text == "instance_eval" || t.text == "define_singleton_method" {
			s.valid = false
		}
		if t.kind == "identifier" && t.text != "" && t.text[0] >= 'A' && t.text[0] <= 'Z' && at+1 < len(tokens) && tokens[at+1].text == "=" {
			qualified := t.text
			if owner != "" {
				qualified = owner + "::" + t.text
			}
			s.unsafeOwners[qualified] = true
		}
		if at > 0 && tokens[at-1].text == "." {
			continue
		}
		if lineStart && t.text == "require" && len(stack) == 0 {
			if uri, ok := rubyRelativeLiteral(body[t.end:]); ok && (uri == "minitest/autorun" || uri == "minitest/test") {
				s.testFramework = true
			}
		}
		if lineStart && t.text == "require_relative" && len(stack) == 0 {
			if uri, ok := rubyRelativeLiteral(body[t.end:]); ok {
				s.imports = append(s.imports, rubyImport{uri, t.line})
			}
		}
		if lineStart && owner != "" {
			switch t.text {
			case "include":
				if len(stack) == 0 || !stack[len(stack)-1].declaration || currentStatic {
					s.unsafeOwners[owner] = true
					continue
				}
				if at+1 < len(tokens) && tokens[at+1].kind == "identifier" {
					j := at + 2
					name := tokens[at+1].text
					for j+2 < len(tokens) && tokens[j].text == ":" && tokens[j+1].text == ":" && tokens[j+2].kind == "identifier" {
						name += "::" + tokens[j+2].text
						j += 3
					}
					if j == len(tokens) || tokens[j].line > t.line {
						s.mixins[owner] = append(s.mixins[owner], name)
					} else {
						s.unsafeOwners[owner] = true
					}
				} else {
					s.unsafeOwners[owner] = true
				}
			case "prepend", "extend", "alias", "undef", "alias_method", "define_method", "remove_method", "undef_method", "module_function", "class_eval", "module_eval":
				s.unsafeOwners[owner] = true
			}
		}
		switch t.text {
		case "class", "module":
			if lineStart && t.text == "class" && at+3 < len(tokens) && tokens[at+1].text == "<" && tokens[at+2].text == "<" && tokens[at+3].text == "self" {
				stack = append(stack, block{owner: owner, member: -1, static: currentStatic, declaration: true})
				currentStatic = true
				continue
			}
			if !lineStart || at+1 >= len(tokens) || tokens[at+1].kind != "identifier" {
				continue
			}
			stack = append(stack, block{owner: owner, member: -1, static: currentStatic, declaration: true})
			name := tokens[at+1].text
			j := at + 2
			for j+2 < len(tokens) && tokens[j].text == ":" && tokens[j+1].text == ":" && tokens[j+2].kind == "identifier" {
				name += "::" + tokens[j+2].text
				j += 3
			}
			qualified := name
			if owner != "" {
				qualified = owner + "::" + name
			}
			symbol := supplementaryTokenSymbol(file, t.text, name, tokens[at+1])
			symbol.Owner = owner
			s.symbols = append(s.symbols, symbol)
			owner = qualified
			currentStatic = false
			if t.text == "class" && j+1 < len(tokens) && tokens[j].text == "<" && tokens[j+1].kind == "identifier" {
				parent := tokens[j+1].text
				k := j + 2
				for k+2 < len(tokens) && tokens[k].text == ":" && tokens[k+1].text == ":" && tokens[k+2].kind == "identifier" {
					parent += "::" + tokens[k+2].text
					k += 3
				}
				s.supers[owner] = parent
			}
		case "def":
			if !lineStart || at+1 >= len(tokens) {
				continue
			}
			nameAt := at + 1
			static := currentStatic
			if nameAt+2 < len(tokens) && tokens[nameAt].text == "self" && tokens[nameAt+1].text == "." {
				nameAt += 2
				static = true
			}
			if tokens[nameAt].kind != "identifier" {
				continue
			}
			if nameAt+1 < len(tokens) && tokens[nameAt+1].text == "." {
				s.unsafeOwners[owner] = true
				continue
			}
			name := tokens[nameAt].text
			if nameAt+1 < len(tokens) && (tokens[nameAt+1].text == "?" || tokens[nameAt+1].text == "!") {
				name += tokens[nameAt+1].text
			}
			symbol := supplementaryTokenSymbol(file, "method", name, tokens[nameAt])
			symbol.Owner = owner
			bodyStart := nameAt + 1
			if bodyStart < len(tokens) && (tokens[bodyStart].text == "?" || tokens[bodyStart].text == "!") {
				bodyStart++
			}
			parameters := []dartParameter{}
			if bodyStart < len(tokens) && tokens[bodyStart].text == "(" {
				pairs, _ := dartPairs(tokens)
				if close, ok := pairs[bodyStart]; ok {
					parameters = dartParameters(tokens[bodyStart+1 : close])
					bodyStart = close + 1
				}
			}
			if bodyStart < len(tokens) && tokens[bodyStart].line == tokens[nameAt].line && tokens[bodyStart].text != "=" {
				parameterEnd := bodyStart
				for parameterEnd < len(tokens) && tokens[parameterEnd].line == tokens[nameAt].line && tokens[parameterEnd].text != ";" {
					parameterEnd++
				}
				parameters = dartParameters(tokens[bodyStart:parameterEnd])
				bodyStart = parameterEnd
			}
			member := dartMember{symbol: symbol, start: bodyStart, end: len(tokens), static: static, parameters: parameters}
			if bodyStart < len(tokens) && tokens[bodyStart].text == "=" {
				end := bodyStart + 1
				for end < len(tokens) && tokens[end].line == tokens[bodyStart].line {
					end++
				}
				member.start = bodyStart
				member.end = end - 1
				s.members = append(s.members, member)
				continue
			}
			stack = append(stack, block{owner: owner, member: len(s.members), static: currentStatic})
			s.members = append(s.members, member)
		case "do":
			stack = append(stack, block{owner: owner, member: -1, start: at, deferred: true, static: currentStatic})
		case "if", "unless", "while", "until", "for", "case", "begin":
			if lineStart {
				stack = append(stack, block{owner: owner, member: -1, static: currentStatic})
			}
		case "end":
			if len(stack) == 0 {
				s.valid = false
				continue
			}
			last := stack[len(stack)-1]
			if last.deferred {
				s.deferred[last.start] = at
			}
			stack = stack[:len(stack)-1]
			if last.member >= 0 {
				s.members[last.member].end = at
			}
			owner = last.owner
			currentStatic = last.static
		}
	}
	if len(stack) > 0 {
		s.valid = false
	}
	for i := range s.members {
		m := &s.members[i]
		if s.testFramework && !m.static && strings.HasPrefix(m.symbol.Name, "test_") && s.supers[m.symbol.Owner] == "Minitest::Test" {
			m.symbol.Kind = "test"
		}
	}
	return s
}
func analyzeRubySources(sources []rubySource, result *supplementaryAnalysis) {
	files := map[string]rubySource{}
	for _, s := range sources {
		files[s.file.Path] = s
		result.facts.Declarations = append(result.facts.Declarations, s.symbols...)
		for _, m := range s.members {
			result.facts.Declarations = append(result.facts.Declarations, m.symbol)
			if m.end < len(s.tokens) {
				result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: m.symbol.Name, Owner: m.symbol.Owner, Kind: m.symbol.Kind, Language: "ruby", File: s.file.Path, Line: m.symbol.Line, EndLine: s.tokens[m.end].line})
			}
		}
		for _, imp := range s.imports {
			ref := supplementaryReference(s.file, imp.uri, "imports_ruby", imp.line)
			target := rubyImportTarget(s.file.Path, imp.uri)
			for _, other := range sources {
				if other.file.Path == target {
					ref.To = target
					ref.Internal = true
				}
			}
			result.facts.References = append(result.facts.References, ref)
		}
	}
	for _, s := range sources {
		if !s.valid {
			continue
		}
		visible := rubyVisibleSources(s, files)
		pairs, _ := dartPairs(s.tokens)
		for _, m := range s.members {
			if m.end >= len(s.tokens) {
				continue
			}
			shadows := map[string]bool{}
			for _, p := range m.parameters {
				shadows[p.name] = true
			}
			for at := m.start; at < m.end; at++ {
				if at+1 < m.end && s.tokens[at].kind == "identifier" && s.tokens[at+1].text == "=" {
					shadows[s.tokens[at].text] = true
				}
			}
			for at := m.start; at < m.end; at++ {
				if end, ok := s.deferred[at]; ok {
					at = end
					continue
				}
				if s.tokens[at].text == "{" {
					if end, ok := pairs[at]; ok {
						at = end
						continue
					}
				}
				if s.tokens[at].text == "def" {
					for _, nested := range s.members {
						if nested.symbol.Line == s.tokens[at].line && nested.symbol.ID != m.symbol.ID && nested.end < m.end {
							at = nested.end
							break
						}
					}
					continue
				}
				token := s.tokens[at]
				if token.kind != "identifier" || dartControlWord(token.text) || token.text == "self" || token.text == "end" || token.text == "return" {
					continue
				}
				name := token.text
				after := at + 1
				if after < m.end && (s.tokens[after].text == "?" || s.tokens[after].text == "!") {
					name += s.tokens[after].text
					after++
				}
				parens := after < m.end && s.tokens[after].text == "("
				receiver := ""
				if at >= 2 && s.tokens[at-1].text == "." {
					receiver = s.tokens[at-2].text
					begin := at - 2
					for begin >= 3 && s.tokens[begin-1].text == ":" && s.tokens[begin-2].text == ":" && s.tokens[begin-3].kind == "identifier" {
						begin -= 3
					}
					receiver = dartJoined(s.tokens[begin : at-1])
				}
				bare := at == m.start || s.tokens[at-1].line < token.line || s.tokens[at-1].text == ";" || s.tokens[at-1].text == "return"
				if !parens && !bare && receiver == "" {
					continue
				}
				if after < m.end && (s.tokens[after].text == "=" || s.tokens[after].text == "." || s.tokens[after].text == ":") {
					continue
				}
				if !parens && receiver == "" && shadows[name] {
					continue
				}
				owner, static := m.symbol.Owner, m.static
				if receiver != "" && receiver != "self" {
					owner = receiver
					static = true
				}
				targets := []RichSymbolRecord{}
				if receiver == "" || receiver == "self" || rubyKnownOwner(owner, visible, files) {
					for _, candidate := range rubyLookup(owner, name, static, visible, files, map[string]bool{}) {
						targets = append(targets, candidate.symbol)
					}
				}
				addNativeCall(result, s.file, m.symbol, name, token.line, at, targets, "unique visible Ruby source method; reopening conflicts, metaprogramming and runtime dispatch remain unproven")
			}
		}
	}
}
