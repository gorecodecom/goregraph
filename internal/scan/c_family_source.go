package scan

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

type cFamilySource struct {
	file     FileRecord
	tokens   []dartToken
	pairs    map[int]int
	members  []dartMember
	types    []RichSymbolRecord
	includes []string
	valid    bool
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
	s := cFamilySource{file: file, tokens: tokens, pairs: pairs, valid: valid && balanced}
	for _, line := range strings.Split(body, "\n") {
		if match := cIncludeRE.FindStringSubmatch(line); len(match) == 2 {
			s.includes = append(s.includes, match[1])
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#define") || strings.HasPrefix(trimmed, "#if") || strings.HasPrefix(trimmed, "#elif") || strings.HasPrefix(trimmed, "#else") {
			s.valid = false
		}
	}
	s.region(0, len(tokens), "")
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
		}
		kind := "function"
		if functionOwner != "" {
			kind = "method"
		}
		symbol := supplementaryTokenSymbol(s.file, kind, name, t[nameAt])
		symbol.Owner = functionOwner
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
		for _, m := range s.members {
			if !s.valid || m.start == m.end {
				continue
			}
			env := map[string]string{}
			for at := 0; at < m.start; at++ {
				if s.tokens[at].line < m.symbol.Line {
					continue
				}
				if at+3 < m.start && s.tokens[at].text == "(" && s.tokens[at+1].text == "*" && s.tokens[at+2].kind == "identifier" && s.tokens[at+3].text == ")" {
					env[s.tokens[at+2].text] = ""
				}
			}

			for _, parameter := range m.parameters {
				env[parameter.name] = strings.Trim(parameter.typeName, "*& ")
			}
			for at := m.start + 1; at < m.end; at++ {
				if at+3 < m.end && s.tokens[at].text == "(" && s.tokens[at+1].text == "*" && s.tokens[at+2].kind == "identifier" && s.tokens[at+3].text == ")" {
					env[s.tokens[at+2].text] = ""
				}

				if s.tokens[at].text != "(" || at == 0 || s.tokens[at-1].kind != "identifier" {
					continue
				}
				nameAt := at - 1
				name := s.tokens[nameAt].text
				if dartControlWord(name) || name == "sizeof" || name == "decltype" {
					continue
				}
				if _, shadow := env[name]; shadow {
					continue
				}
				close, ok := s.pairs[at]
				if !ok {
					continue
				}
				owner := m.symbol.Owner
				unknownReceiver := false
				if nameAt >= 2 && (s.tokens[nameAt-1].text == "." || s.tokens[nameAt-1].text == ">") {
					receiver := s.tokens[nameAt-2].text
					if s.tokens[nameAt-1].text == ">" && nameAt >= 3 && s.tokens[nameAt-2].text == "-" {
						receiver = s.tokens[nameAt-3].text
					}
					owner = env[receiver]
					unknownReceiver = owner == ""
				} else if nameAt >= 3 && s.tokens[nameAt-1].text == ":" && s.tokens[nameAt-2].text == ":" {
					owner = s.tokens[nameAt-3].text
				}
				if unknownReceiver {
					continue
				}
				candidates := functions[owner+"."+name]
				if len(candidates) == 0 && owner != "" {
					candidates = functions["."+name]
				}
				var visible []dartMember
				for _, candidate := range candidates {
					if !dartAccepts(candidate, s.tokens[at+1:close]) {
						continue
					}
					if candidate.symbol.File == s.file.Path {
						visible = append(visible, candidate)
						continue
					}
					if candidate.static {
						continue
					}
					for _, include := range s.includes {
						header, exists := files[path.Clean(path.Join(path.Dir(s.file.Path), include))]
						if !exists || !header.valid {
							continue
						}
						for _, prototype := range header.members {
							if prototype.symbol.Name == candidate.symbol.Name && prototype.symbol.Owner == candidate.symbol.Owner && len(prototype.parameters) == len(candidate.parameters) {
								visible = append(visible, candidate)
								break
							}
						}
					}
				}
				ref := supplementaryReference(s.file, name, "calls_method_owner", s.tokens[at].line)
				ref.FromSymbolID = m.symbol.ID
				if len(visible) == 1 {
					target := visible[0].symbol
					ref.To = target.File
					ref.ToSymbolID = target.ID
					ref.TargetQualifiedName = target.QualifiedName
					ref.Resolution = SymbolResolutionExact
					ref.NonPromotable = false
					ref.preventExact = false
					ref.Internal = true
					ref.Reason = "unique visible C/C++ declaration; preprocessing, linking, overload conversions and runtime dispatch are not evaluated"
					result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("c-family-call", m.symbol.ID, target.ID, fmt.Sprint(at)), From: MethodRefRecord{Owner: m.symbol.Owner, Method: m.symbol.Name, File: s.file.Path, Line: m.symbol.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, Type: "calls", Line: s.tokens[at].line, SourceFile: s.file.Path, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: m.symbol.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
				} else if len(visible) > 1 {
					ref.Resolution = SymbolResolutionAmbiguous
				}
				result.facts.References = append(result.facts.References, ref)
			}
		}
	}
}
