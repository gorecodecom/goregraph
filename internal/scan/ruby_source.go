package scan

import (
	"fmt"
	"regexp"
	"strings"
)

type rubySource struct {
	file     FileRecord
	tokens   []dartToken
	members  []dartMember
	symbols  []RichSymbolRecord
	valid    bool
	deferred map[int]int
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
	s := rubySource{file: file, tokens: tokens, valid: valid, deferred: map[int]int{}}
	type block struct {
		owner    string
		member   int
		start    int
		deferred bool
	}
	var stack []block
	owner := ""
	for at, t := range tokens {
		if t.kind != "identifier" {
			continue
		}
		lineStart := at == 0 || tokens[at-1].line < t.line || tokens[at-1].text == ";"
		if at > 0 && tokens[at-1].text == "." {
			continue
		}
		switch t.text {
		case "class", "module":
			if !lineStart || at+1 >= len(tokens) || tokens[at+1].kind != "identifier" {
				continue
			}
			stack = append(stack, block{owner: owner, member: -1})
			name := tokens[at+1].text
			qualified := name
			if owner != "" {
				qualified = owner + "::" + name
			}
			symbol := supplementaryTokenSymbol(file, t.text, name, tokens[at+1])
			symbol.Owner = owner
			s.symbols = append(s.symbols, symbol)
			owner = qualified
		case "def":
			if !lineStart || at+1 >= len(tokens) {
				continue
			}
			nameAt := at + 1
			static := false
			if nameAt+2 < len(tokens) && tokens[nameAt].text == "self" && tokens[nameAt+1].text == "." {
				nameAt += 2
				static = true
			}
			if tokens[nameAt].kind != "identifier" {
				continue
			}
			name := tokens[nameAt].text
			if nameAt+1 < len(tokens) && (tokens[nameAt+1].text == "?" || tokens[nameAt+1].text == "!") {
				name += tokens[nameAt+1].text
			}
			symbol := supplementaryTokenSymbol(file, "method", name, tokens[nameAt])
			symbol.Owner = owner
			member := dartMember{symbol: symbol, start: nameAt + 1, end: len(tokens), static: static}
			stack = append(stack, block{owner: owner, member: len(s.members)})
			s.members = append(s.members, member)
		case "do":
			stack = append(stack, block{owner: owner, member: -1, start: at, deferred: true})
		case "if", "unless", "while", "until", "for", "case", "begin":
			if lineStart {
				stack = append(stack, block{owner: owner, member: -1})
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
		}
	}
	if len(stack) > 0 {
		s.valid = false
	}
	return s
}
func analyzeRubySources(sources []rubySource, result *supplementaryAnalysis) {
	for _, s := range sources {
		result.facts.Declarations = append(result.facts.Declarations, s.symbols...)
		for _, m := range s.members {
			result.facts.Declarations = append(result.facts.Declarations, m.symbol)
			if m.end < len(s.tokens) {
				result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: m.symbol.Name, Owner: m.symbol.Owner, Kind: m.symbol.Kind, Language: "ruby", File: s.file.Path, Line: m.symbol.Line, EndLine: s.tokens[m.end].line})
			}
		}
		if !s.valid {
			continue
		}
		for _, m := range s.members {
			if m.end >= len(s.tokens) {
				continue
			}
			for at := m.start; at < m.end; at++ {
				if end, ok := s.deferred[at]; ok {
					at = end
					continue
				}
				token := s.tokens[at]
				if token.kind != "identifier" || at+1 >= m.end || s.tokens[at+1].text != "(" {
					continue
				}
				name := token.text
				receiver := ""
				if at >= 2 && s.tokens[at-1].text == "." {
					receiver = s.tokens[at-2].text
				}
				if receiver != "" && receiver != "self" && receiver != m.symbol.Owner {
					continue
				}
				var candidates []dartMember
				// Ruby has no declared local-variable types; only explicit self/class receivers are promoted.
				if receiver != "" {
					for _, candidate := range s.members {
						if candidate.symbol.Owner == m.symbol.Owner && candidate.symbol.Name == name && candidate.static == (receiver == m.symbol.Owner) {
							candidates = append(candidates, candidate)
						}
					}
				}
				ref := supplementaryReference(s.file, name, "calls_method_owner", token.line)
				ref.FromSymbolID = m.symbol.ID
				if len(candidates) == 1 {
					target := candidates[0].symbol
					ref.To = s.file.Path
					ref.ToSymbolID = target.ID
					ref.TargetQualifiedName = target.QualifiedName
					ref.Resolution = SymbolResolutionExact
					ref.NonPromotable = false
					ref.preventExact = false
					ref.Internal = true
					ref.Reason = "unique explicit Ruby self/class declaration; monkey-patching and runtime dispatch remain unproven"
					result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("ruby-call", m.symbol.ID, target.ID, fmt.Sprint(at)), From: MethodRefRecord{Owner: m.symbol.Owner, Method: m.symbol.Name, File: s.file.Path, Line: m.symbol.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, Type: "calls", Line: token.line, SourceFile: s.file.Path, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: m.symbol.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
				} else if len(candidates) > 1 {
					ref.Resolution = SymbolResolutionAmbiguous
				}
				result.facts.References = append(result.facts.References, ref)
			}
		}
	}
}
