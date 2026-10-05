package scan

import ()

type objcMethod struct {
	symbol     RichSymbolRecord
	start, end int
	parameters map[string]string
	static     bool
	returnType string
}
type objcSource struct {
	file       FileRecord
	tokens     []dartToken
	pairs      map[int]int
	methods    []objcMethod
	classes    []RichSymbolRecord
	imports    []string
	valid      bool
	supers     map[string]string
	properties map[string]map[string]string
}

func parseObjCSource(file FileRecord, body string) objcSource {
	tokens, valid := dartTokens(body)
	pairs, balanced := dartPairs(tokens)
	s := objcSource{file: file, tokens: tokens, pairs: pairs, valid: valid && balanced, supers: map[string]string{}, properties: map[string]map[string]string{}}
	s.valid = s.valid && nativePreprocessorSafe(body)
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
				if keyword == "interface" && at+4 < len(tokens) && tokens[at+3].text == ":" {
					s.supers[owner] = tokens[at+4].text
				}
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
					if s.properties[owner] == nil {
						s.properties[owner] = map[string]string{}
					}
					s.properties[owner][property.Name] = objcType(tokens[start : end-1])
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
						typ := objcType(tokens[end+1 : close])
						end = close + 1
						if end < len(tokens) && tokens[end].kind == "identifier" {
							params[tokens[end].text] = typ
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
			symbol := supplementaryTokenSymbol(file, "method", owner+"."+name, tokens[nameAt])
			symbol.Name = name
			symbol.Owner = owner
			bodyEnd := end
			if tokens[end].text == "{" {
				if close, ok := pairs[end]; ok {
					bodyEnd = close
				}
			}
			s.methods = append(s.methods, objcMethod{symbol: symbol, start: end, end: bodyEnd, parameters: params, static: tokens[at].text == "+", returnType: objcType(tokens[at+2 : returnEnd])})
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
			params := map[string]string{}
			for _, param := range dartParameters(tokens[at+1 : close]) {
				if param.name != "void" {
					params[param.name] = param.typeName
				}
			}
			for j := at + 1; j+3 < close; j++ {
				if tokens[j].text == "(" && tokens[j+1].text == "*" && tokens[j+3].text == ")" {
					params[tokens[j+2].text] = ""
				}
			}
			s.methods = append(s.methods, objcMethod{symbol: symbol, start: close + 1, end: end, parameters: params})
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
	classificationFiles := map[string]objcSource{}
	for _, s := range sources {
		classificationFiles[s.file.Path] = s
	}
	for i := range sources {
		files := classificationFiles
		visible := objcVisibleHeaders(sources[i], files)
		for j := range sources[i].methods {
			if objcXCTestMethod(sources[i], sources[i].methods[j], visible, files) {
				sources[i].methods[j].symbol.Kind = "test"
			}
		}
	}
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
	files := map[string]objcSource{}
	for _, s := range sources {
		files[s.file.Path] = s
	}
	for _, s := range sources {
		if !s.valid {
			continue
		}
		visible := objcVisibleHeaders(s, files)
		for _, m := range s.methods {
			if m.start == m.end {
				continue
			}
			env := map[string]string{"self": m.symbol.Owner}
			for name, typ := range m.parameters {
				env[name] = typ
			}
			scopes := []map[string]string{}
			for at := m.start + 1; at < m.end; at++ {
				t := s.tokens[at]
				if t.text == "^" {
					if end := objcBlockEnd(s, at, m.end); end > at {
						at = end
						continue
					}
				}
				if t.text == "{" {
					scopes = append(scopes, cloneNativeTypes(env))
				}
				if t.text == "}" && len(scopes) > 0 {
					env = scopes[len(scopes)-1]
					scopes = scopes[:len(scopes)-1]
				}
				objcLocalBinding(s, at, m.end, env)
				name, owner, static := "", "", false
				if t.text == "[" {
					owner, static, name = objcMessage(s, at, m, env, visible, files, byMethod, 0)
				} else if t.kind == "identifier" && at+1 < m.end && s.tokens[at+1].text == "(" {
					if at > 0 && (s.tokens[at-1].text == "." || s.tokens[at-1].text == ">" || s.tokens[at-1].text == "^") {
						continue
					}
					if _, shadow := env[t.text]; shadow {
						continue
					}
					name = t.text
				} else {
					continue
				}
				if name == "" || dartControlWord(name) {
					continue
				}
				var candidates []objcMethod
				if t.text == "[" && owner != "" {
					candidates = objcLookup(owner, name, static, visible, files, byMethod, map[string]bool{})
				}
				if t.text != "[" {
					for _, candidate := range byMethod["."+name] {
						if candidate.symbol.File == s.file.Path {
							candidates = append(candidates, candidate)
						}
					}
				}
				targets := []RichSymbolRecord{}
				for _, candidate := range candidates {
					targets = append(targets, candidate.symbol)
				}
				addNativeCall(result, s.file, m.symbol, name, t.line, at, targets, "unique indexed Objective-C selector visible through local/imported interfaces; runtime dispatch and swizzling are not evaluated")
			}
		}
	}
}
