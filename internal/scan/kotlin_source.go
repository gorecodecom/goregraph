package scan

import (
	"fmt"
	"path"
	"strings"
)

type kotlinSource struct {
	file                FileRecord
	tokens              []dartToken
	pairs               map[int]int
	packageName, module string
	imports             []string
	members             []dartMember
	types               []RichSymbolRecord
	valid               bool
}

func kotlinModifier(name string) bool {
	switch name {
	case "public", "private", "protected", "internal", "open", "override", "abstract", "final", "sealed", "data", "enum", "annotation", "suspend", "inline", "tailrec", "operator", "infix", "external", "expect", "actual", "crossinline", "noinline", "vararg", "val", "var":
		return true
	}
	return false
}
func kotlinTypeName(tokens []dartToken) string {
	var clean []dartToken
	for _, token := range tokens {
		if !kotlinModifier(token.text) {
			clean = append(clean, token)
		}
	}
	return dartJoined(clean)
}
func kotlinParameters(tokens []dartToken) []dartParameter {
	var parameters []dartParameter
	for _, chunk := range dartSplit(tokens) {
		if len(chunk) < 3 || chunk[0].kind != "identifier" || chunk[1].text != ":" {
			continue
		}
		p := dartParameter{name: chunk[0].text}
		end := len(chunk)
		for i, t := range chunk {
			if t.text == "=" {
				p.optional = true
				end = i
				break
			}
		}
		p.typeName = kotlinTypeName(chunk[2:end])
		parameters = append(parameters, p)
	}
	return parameters
}
func parseKotlinSource(file FileRecord, body string) kotlinSource {
	tokens, valid := dartTokens(body)
	pairs, balanced := dartPairs(tokens)
	s := kotlinSource{file: file, tokens: tokens, pairs: pairs, valid: valid && balanced}
	for at, t := range tokens {
		if t.text != "package" && t.text != "import" {
			continue
		}
		end := at + 1
		for end < len(tokens) && tokens[end].line == t.line && tokens[end].text != ";" {
			end++
		}
		value := dartJoined(tokens[at+1 : end])
		if t.text == "package" {
			s.packageName = value
		} else {
			s.imports = append(s.imports, value)
		}
	}
	s.region(0, len(tokens), "", false)
	return s
}
func (s *kotlinSource) region(start, end int, owner string, static bool) {
	t := s.tokens
	for at := start; at < end; at++ {
		if t[at].text == "class" || t[at].text == "interface" || t[at].text == "object" {
			nameAt := at + 1
			companion := at > 0 && t[at-1].text == "companion" && t[at].text == "object"
			name := "Companion"
			if nameAt < end && t[nameAt].kind == "identifier" {
				name = t[nameAt].text
			} else if !companion {
				continue
			}
			kind := t[at].text
			symbol := supplementaryTokenSymbol(s.file, kind, name, t[at])
			symbol.Owner = owner
			s.types = append(s.types, symbol)
			body := nameAt + 1
			if companion && nameAt < end && t[nameAt].text == "{" {
				body = nameAt
			}
			for body < end && t[body].text != "{" && t[body].text != ";" {
				if e, ok := s.pairs[body]; ok && e > body {
					body = e + 1
					continue
				}
				if t[body].line > t[nameAt].line && t[body].text != "(" && t[body].text != ":" {
					break
				}
				body++
			}
			if body >= end || t[body].text != "{" {
				continue
			}
			close, ok := s.pairs[body]
			if !ok {
				continue
			}
			scope := name
			if owner != "" {
				scope = owner + "." + name
			}
			if companion {
				scope = owner
			}
			s.region(body+1, close, scope, companion || kind == "object")
			at = close
			continue
		}
		if t[at].text == "fun" {
			paren := at + 1
			for paren < end && t[paren].text != "(" && t[paren].text != "{" && t[paren].text != ";" {
				paren++
			}
			if paren >= end || t[paren].text != "(" || paren == at+1 {
				continue
			}
			nameAt := paren - 1
			if t[nameAt].kind != "identifier" {
				continue
			}
			close, ok := s.pairs[paren]
			if !ok {
				continue
			}
			body := close + 1
			for body < end && t[body].text != "{" && t[body].text != "=" && t[body].text != ";" {
				if t[body].line > t[close].line && t[body].text == "fun" {
					break
				}
				body++
			}
			if body >= end {
				continue
			}
			bodyEnd := body
			if t[body].text == "{" {
				if e, ok := s.pairs[body]; ok {
					bodyEnd = e
				}
			} else if t[body].text == "=" {
				bodyEnd = body + 1
				for bodyEnd < end && t[bodyEnd].text != ";" && t[bodyEnd].line == t[body].line {
					if e, ok := s.pairs[bodyEnd]; ok && e > bodyEnd {
						bodyEnd = e + 1
						continue
					}
					bodyEnd++
				}
			}
			name := t[nameAt].text
			kind := "function"
			if owner != "" {
				kind = "method"
			}
			if s.hasAnnotation(at, "Test") && s.imported("Test", []string{"org.junit.Test", "org.junit.jupiter.api.Test", "kotlin.test.Test"}) {
				kind = "test"
			}
			symbol := supplementaryTokenSymbol(s.file, kind, name, t[nameAt])
			symbol.Owner = owner
			symbol.Package = s.packageName
			symbol.ExportName = ""
			if !s.modifierBefore(at, "private") {
				symbol.ExportName = name
			}
			s.members = append(s.members, dartMember{symbol: symbol, parameters: kotlinParameters(t[paren+1 : close]), start: body, end: bodyEnd, static: static})
			at = bodyEnd
			continue
		}
		if (t[at].text == "val" || t[at].text == "var") && at+1 < end && t[at+1].kind == "identifier" {
			nameAt := at + 1
			stop := nameAt + 1
			for stop < end && t[stop].line == t[nameAt].line && t[stop].text != ";" {
				if e, ok := s.pairs[stop]; ok && e > stop {
					stop = e + 1
					continue
				}
				stop++
			}
			typeName := ""
			if nameAt+1 < stop && t[nameAt+1].text == ":" {
				typeEnd := nameAt + 2
				for typeEnd < stop && t[typeEnd].text != "=" {
					typeEnd++
				}
				typeName = kotlinTypeName(t[nameAt+2 : typeEnd])
			} else if nameAt+2 < stop && t[nameAt+1].text == "=" {
				typeName = dartConstructorType(t[nameAt+2 : stop])
			}
			symbol := supplementaryTokenSymbol(s.file, "property", t[nameAt].text, t[nameAt])
			symbol.Owner = owner
			symbol.Package = s.packageName
			s.members = append(s.members, dartMember{symbol: symbol, typeName: typeName, start: nameAt, end: stop, static: static})
			at = stop - 1
		}
	}
}
func (s kotlinSource) modifierBefore(at int, name string) bool {
	for i := at - 1; i >= 0 && s.tokens[i].line == s.tokens[at].line; i-- {
		if s.tokens[i].text == name {
			return true
		}
	}
	return false
}
func (s kotlinSource) hasAnnotation(at int, name string) bool {
	for i := at - 1; i >= 0 && s.tokens[at].line-s.tokens[i].line <= 8; i-- {
		if s.tokens[i].text == "}" || s.tokens[i].text == "{" || s.tokens[i].text == ";" {
			break
		}
		if s.tokens[i].text == "@" && i+1 < len(s.tokens) && s.tokens[i+1].text == name {
			return true
		}
	}
	return false
}
func (s kotlinSource) imported(name string, uris []string) bool {
	for _, typ := range s.types {
		if typ.Name == name {
			return false
		}
	}
	for _, uri := range uris {
		for _, imp := range s.imports {
			if imp == uri {
				return true
			}
		}
	}
	return false
}
func assignKotlinModules(sources []kotlinSource, workspace WorkspaceIndex) {
	for i := range sources {
		best := -1
		for _, p := range workspace.GradlePackages {
			root := path.Dir(p.Path)
			if root == "." {
				root = ""
			}
			if (root == "" || strings.HasPrefix(sources[i].file.Path, root+"/")) && len(root) > best {
				sources[i].module = root
				best = len(root)
			}
		}
	}
}
func kotlinAccepts(member dartMember, args []dartToken) bool {
	converted := append([]dartToken(nil), args...)
	for i := range converted {
		if converted[i].text == "=" {
			converted[i].text = ":"
		}
	}
	count, named, valid := dartArgumentShape(converted)
	if !valid {
		return false
	}
	if count > len(member.parameters) {
		return false
	}
	for i, p := range member.parameters {
		if i < count {
			if named[p.name] {
				return false
			}
			continue
		}
		if !p.optional && !named[p.name] {
			return false
		}
		delete(named, p.name)
	}
	return len(named) == 0
}

func analyzeKotlinSources(sources []kotlinSource, result *supplementaryAnalysis) {
	for _, s := range sources {
		result.facts.Declarations = append(result.facts.Declarations, s.types...)
		for _, m := range s.members {
			kotlinRouteEvidence(s, m, result)
			result.facts.Declarations = append(result.facts.Declarations, m.symbol)
			if m.symbol.Kind != "property" && m.end < len(s.tokens) {
				result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: m.symbol.Name, Owner: m.symbol.Owner, Kind: m.symbol.Kind, Language: "kotlin", File: s.file.Path, Line: m.symbol.Line, EndLine: s.tokens[m.end].line})
			}
		}
		for _, imp := range s.imports {
			result.facts.References = append(result.facts.References, supplementaryReference(s.file, imp, "imports_type", 1))
		}
	}
	for _, s := range sources {
		if !s.valid {
			continue
		}
		for _, m := range s.members {
			if m.symbol.Kind == "property" || m.end >= len(s.tokens) {
				continue
			}
			env := map[string]string{}
			for _, field := range s.members {
				if field.symbol.Kind == "property" && field.symbol.Owner == m.symbol.Owner {
					env[field.symbol.Name] = dartBaseType(field.typeName)
				}
			}
			for _, p := range m.parameters {
				env[p.name] = dartBaseType(p.typeName)
			}
			scopes := map[int]map[string]string{}
			for at := m.start + 1; at < m.end; at++ {
				token := s.tokens[at]
				if token.kind == "identifier" && at+1 < m.end && s.tokens[at+1].text == "=" && (at == 0 || s.tokens[at-1].text != "val" && s.tokens[at-1].text != "var") {
					if _, exists := env[token.text]; exists {
						env[token.text] = ""
					}
				}
				if token.text == "{" && at > m.start && at > 0 && (s.tokens[at-1].text == "=" || s.tokens[at-1].kind == "identifier") {
					if close, ok := s.pairs[at]; ok {
						at = close
						continue
					}
				}
				if token.text == "}" {
					if previous, ok := scopes[at]; ok {
						env = previous
						delete(scopes, at)
					}
				}
				if token.text == "{" {
					if close, ok := s.pairs[at]; ok {
						previous := map[string]string{}
						for name, typ := range env {
							previous[name] = typ
						}
						scopes[close] = previous
					}
				}
				if (token.text == "val" || token.text == "var") && at+3 < m.end && s.tokens[at+1].kind == "identifier" {
					name := s.tokens[at+1].text
					typeName := ""
					if s.tokens[at+2].text == "=" {
						typeName = dartConstructorType(s.tokens[at+3:])
					} else if s.tokens[at+2].text == ":" {
						end := at + 3
						for end < m.end && s.tokens[end].text != "=" && s.tokens[end].line == token.line {
							end++
						}
						typeName = kotlinTypeName(s.tokens[at+3 : end])
					}
					env[name] = dartBaseType(typeName)
				}
				if token.text != "(" || at == 0 || s.tokens[at-1].kind != "identifier" {
					continue
				}
				close, ok := s.pairs[at]
				if !ok {
					continue
				}
				nameAt := at - 1
				name := s.tokens[nameAt].text
				if dartControlWord(name) {
					continue
				}
				receiver := ""
				if nameAt >= 2 && (s.tokens[nameAt-1].text == "." || s.tokens[nameAt-1].text == "?.") {
					receiver = s.tokens[nameAt-2].text
				}
				owner := m.symbol.Owner
				static := false
				if receiver != "" && receiver != "this" {
					if typ, ok := env[receiver]; ok {
						owner = typ
						if owner == "" {
							continue
						}
					} else {
						owner = receiver
						static = true
					}
				}
				if receiver == "" {
					if _, shadow := env[name]; shadow {
						continue
					}
				}
				kotlinCallEvidence(s, m, at, close, receiver, name, env, result)
				var candidates []dartMember
				for _, targetSource := range sources {
					if !targetSource.valid || targetSource.module != s.module {
						continue
					}
					visible := targetSource.file.Path == s.file.Path || targetSource.packageName == s.packageName
					for _, imp := range s.imports {
						if imp == targetSource.packageName+"."+owner || imp == targetSource.packageName+"."+name || imp == targetSource.packageName+".*" {
							visible = true
						}
					}
					if !visible {
						continue
					}
					for _, candidate := range targetSource.members {
						if candidate.symbol.Name != name || candidate.symbol.Kind == "property" || candidate.symbol.Owner != owner && !(receiver == "" && candidate.symbol.Owner == "") || static && !candidate.static || candidate.symbol.ExportName == "" && targetSource.file.Path != s.file.Path || !kotlinAccepts(candidate, s.tokens[at+1:close]) {
							continue
						}
						candidates = append(candidates, candidate)
					}
				}
				ref := supplementaryReference(s.file, name, "calls_method_owner", token.line)
				ref.FromSymbolID = m.symbol.ID
				if len(candidates) == 1 {
					target := candidates[0].symbol
					ref.To = target.File
					ref.ToSymbolID = target.ID
					ref.TargetQualifiedName = target.QualifiedName
					ref.Resolution = SymbolResolutionExact
					ref.NonPromotable = false
					ref.preventExact = false
					ref.Internal = true
					ref.Reason = "unique visible Kotlin declaration with matching named/default parameters; runtime dispatch is not evaluated"
					result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("kotlin-call", m.symbol.ID, target.ID, fmt.Sprint(at)), From: MethodRefRecord{Owner: m.symbol.Owner, Method: m.symbol.Name, File: s.file.Path, Line: m.symbol.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, Type: "calls", Line: token.line, SourceFile: s.file.Path, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: m.symbol.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
					if m.symbol.Kind == "test" {
						result.tests = append(result.tests, TestMapRecord{TestFile: s.file.Path, TestClass: m.symbol.Owner, TestMethod: m.symbol.Name, TargetFile: target.File, TargetClass: target.Owner, TargetMethod: target.Name, Type: "unit", Line: token.line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "static Kotlin test call; test execution is not asserted"})
					}
				} else if len(candidates) > 1 {
					ref.Resolution = SymbolResolutionAmbiguous
				}
				result.facts.References = append(result.facts.References, ref)
			}
		}
	}
}
