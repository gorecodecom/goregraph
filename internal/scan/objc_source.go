package scan

import (
	"fmt"
	"strings"
)

type objcMethod struct {
	symbol     RichSymbolRecord
	start, end int
	parameters map[string]string
	static     bool
}
type objcSource struct {
	file    FileRecord
	tokens  []dartToken
	pairs   map[int]int
	methods []objcMethod
	classes []RichSymbolRecord
	imports []string
	valid   bool
}

func parseObjCSource(file FileRecord, body string) objcSource {
	tokens, valid := dartTokens(body)
	pairs, balanced := dartPairs(tokens)
	s := objcSource{file: file, tokens: tokens, pairs: pairs, valid: valid && balanced}
	for _, line := range strings.Split(body, "\n") {
		text := strings.TrimSpace(line)
		if strings.HasPrefix(text, "#define") || strings.HasPrefix(text, "#if") || strings.HasPrefix(text, "#elif") || strings.HasPrefix(text, "#else") {
			s.valid = false
		}
	}
	owner := ""
	for at := 0; at < len(tokens); at++ {
		if tokens[at].text == "#" && at+1 < len(tokens) && (tokens[at+1].text == "import" || tokens[at+1].text == "include") {
			start := at + 2
			end := start
			for end < len(tokens) && tokens[end].line == tokens[at].line {
				end++
			}
			if start < end {
				if value, ok := dartLiteral(tokens[start]); ok {
					s.imports = append(s.imports, value)
				} else {
					s.imports = append(s.imports, dartJoined(tokens[start:end]))
				}
			}
			at = end - 1
			continue
		}
		if tokens[at].text == "@" && at+1 < len(tokens) {
			keyword := tokens[at+1].text
			if keyword == "end" {
				owner = ""
				at++
				continue
			}
			if (keyword == "interface" || keyword == "implementation" || keyword == "protocol") && at+2 < len(tokens) {
				owner = tokens[at+2].text
				kind := "class"
				if keyword == "protocol" {
					kind = "protocol"
				}
				s.classes = append(s.classes, supplementarySymbol(file, kind, owner, tokens[at].line))
				at += 2
				continue
			}
			if keyword == "property" && owner != "" {
				start := at + 2
				if start < len(tokens) && tokens[start].text == "(" {
					start = pairs[start] + 1
				}
				end := start
				for end < len(tokens) && tokens[end].text != ";" {
					end++
				}
				if end > start && tokens[end-1].kind == "identifier" {
					property := supplementarySymbol(file, "property", tokens[end-1].text, tokens[end-1].line)
					property.Owner = owner
					s.classes = append(s.classes, property)
				}
				at = end
				continue
			}
		}
		if owner != "" && (tokens[at].text == "-" || tokens[at].text == "+") && at+1 < len(tokens) && tokens[at+1].text == "(" {
			returnEnd, ok := pairs[at+1]
			if !ok || returnEnd+1 >= len(tokens) {
				continue
			}
			nameAt := returnEnd + 1
			if tokens[nameAt].kind != "identifier" {
				continue
			}
			name := ""
			params := map[string]string{}
			end := nameAt
			for end < len(tokens) && tokens[end].text != "{" && tokens[end].text != ";" {
				if tokens[end].kind == "identifier" && end+1 < len(tokens) && tokens[end+1].text == ":" {
					name += tokens[end].text + ":"
					end += 2
					if end < len(tokens) && tokens[end].text == "(" {
						close, ok := pairs[end]
						if !ok {
							break
						}
						typ := dartJoined(tokens[end+1 : close])
						end = close + 1
						if end < len(tokens) && tokens[end].kind == "identifier" {
							params[tokens[end].text] = strings.Trim(typ, " *")
							end++
						}
					}
					continue
				}
				if name == "" && end == nameAt {
					name = tokens[end].text
				}
				end++
			}
			if end >= len(tokens) {
				continue
			}
			symbol := supplementarySymbol(file, "method", owner+"."+name, tokens[nameAt].line)
			symbol.Name = name
			symbol.Owner = owner
			bodyEnd := end
			if tokens[end].text == "{" {
				if close, ok := pairs[end]; ok {
					bodyEnd = close
				}
			}
			s.methods = append(s.methods, objcMethod{symbol: symbol, start: end, end: bodyEnd, parameters: params, static: tokens[at].text == "+"})
			at = bodyEnd
			continue
		}
		// C ABI entrypoints in Objective-C bridges retain their declaration and call identity.
		if owner == "" && tokens[at].text == "(" && at > 0 && tokens[at-1].kind == "identifier" {
			close, ok := pairs[at]
			if !ok || close+1 >= len(tokens) || tokens[close+1].text != "{" {
				continue
			}
			name := tokens[at-1].text
			if dartControlWord(name) {
				continue
			}
			end, ok := pairs[close+1]
			if !ok {
				continue
			}
			symbol := supplementarySymbol(file, "function", name, tokens[at-1].line)
			s.methods = append(s.methods, objcMethod{symbol: symbol, start: close + 1, end: end, parameters: map[string]string{}})
			at = end
		}
	}
	if owner != "" {
		s.valid = false
	}
	return s
}

func analyzeObjCSources(sources []objcSource, result *supplementaryAnalysis) {
	byMethod := map[string][]objcMethod{}
	for _, s := range sources {
		for _, symbol := range s.classes {
			result.facts.Declarations = append(result.facts.Declarations, symbol)
		}
		for _, m := range s.methods {
			result.facts.Declarations = append(result.facts.Declarations, m.symbol)
			if s.valid && m.start < m.end {
				byMethod[m.symbol.Owner+"."+m.symbol.Name] = append(byMethod[m.symbol.Owner+"."+m.symbol.Name], m)
				result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: m.symbol.Name, Owner: m.symbol.Owner, Kind: m.symbol.Kind, Language: "objectivec", File: s.file.Path, Line: m.symbol.Line, EndLine: s.tokens[m.end].line})
			}
		}
		for _, uri := range s.imports {
			result.facts.References = append(result.facts.References, supplementaryReference(s.file, uri, "imports_header", 1))
		}
	}
	for _, s := range sources {
		if !s.valid {
			continue
		}
		for _, m := range s.methods {
			env := map[string]string{"self": m.symbol.Owner}
			for name, typ := range m.parameters {
				env[name] = typ
			}
			for at := m.start + 1; at < m.end; at++ {
				t := s.tokens[at]
				targetOwner, targetName := "", ""
				targetStatic := false
				end := at
				if t.kind == "identifier" && at+1 < m.end && s.tokens[at+1].text == "(" {
					targetName = t.text
				} else if t.text == "[" {
					close, ok := s.pairs[at]
					if !ok || at+2 >= close || s.tokens[at+1].kind != "identifier" {
						continue
					}
					receiver := s.tokens[at+1].text
					targetStatic = receiver == "self" && m.static
					targetOwner = env[receiver]
					if targetOwner == "" {
						for _, symbol := range s.classes {
							if symbol.Kind == "class" && symbol.Name == receiver {
								targetOwner = receiver
								targetStatic = true
							}
						}
					}
					for j := at + 2; j < close; j++ {
						if j+1 < close && s.tokens[j].kind == "identifier" && s.tokens[j+1].text == ":" {
							targetName += s.tokens[j].text + ":"
						}
						if targetName == "" && j == at+2 && s.tokens[j].kind == "identifier" {
							targetName = s.tokens[j].text
						}
						if close, ok := s.pairs[j]; ok && close > j {
							j = close
						}
					}
					end = close
				} else {
					continue
				}
				if targetName == "" || dartControlWord(targetName) {
					continue
				}
				if t.text == "[" && targetOwner == "" {
					result.facts.References = append(result.facts.References, supplementaryReference(s.file, targetName, "calls_method_owner", t.line))
					continue
				}
				key := targetOwner + "." + targetName
				candidates := byMethod[key]
				var visible []objcMethod
				for _, candidate := range candidates {
					if candidate.symbol.File == s.file.Path && (t.text != "[" || candidate.static == targetStatic) {
						visible = append(visible, candidate)
					}
				}
				ref := supplementaryReference(s.file, targetName, "calls_method_owner", t.line)
				ref.FromSymbolID = m.symbol.ID
				if len(visible) == 1 {
					target := visible[0].symbol
					ref.To = target.File
					ref.ToSymbolID = target.ID
					ref.TargetQualifiedName = target.QualifiedName
					ref.Internal = true
					ref.Resolution = SymbolResolutionExact
					ref.NonPromotable = false
					ref.preventExact = false
					ref.Reason = "unique local Objective-C selector or C entrypoint declaration; runtime dispatch is not evaluated"
					result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("objc-call", m.symbol.ID, target.ID, fmt.Sprint(at)), From: MethodRefRecord{Owner: m.symbol.Owner, Method: m.symbol.Name, File: s.file.Path, Line: m.symbol.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, Type: "calls", Line: t.line, SourceFile: s.file.Path, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: m.symbol.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
				} else if len(visible) > 1 {
					ref.Resolution = SymbolResolutionAmbiguous
				}
				result.facts.References = append(result.facts.References, ref)
				if end > at {
					at = end
				}
			}
		}
	}
}
