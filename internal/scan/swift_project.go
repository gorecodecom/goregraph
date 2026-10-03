package scan

import (
	"sort"
	"strconv"
	"strings"
)

type swiftAnalysis struct {
	facts        ProjectSymbolFacts
	code         CodeIntelligenceRecord
	graph        CallGraphRecord
	tests        []TestMapRecord
	capabilities []ArchitectureCapabilityFact
}

func analyzeSwiftProject(sources []swiftSource) swiftAnalysis {
	result := swiftAnalysis{}
	types := map[string][]RichSymbolRecord{}
	members := map[string][]swiftMember{}
	for _, s := range sources {
		swiftStateReferences(s, &result)
		for _, t := range s.types {
			types[t.symbol.QualifiedName] = append(types[t.symbol.QualifiedName], t.symbol)
			result.facts.Declarations = append(result.facts.Declarations, t.symbol)
		}
		for _, m := range s.members {
			members[m.owner+"."+m.symbol.Name] = append(members[m.owner+"."+m.symbol.Name], m)
			result.facts.Declarations = append(result.facts.Declarations, m.symbol)
		}
	}
	for _, s := range sources {
		result.facts.References = append(result.facts.References, swiftModelReferences(s, types)...)
		for _, t := range s.types {
			for _, base := range t.bases {
				result.facts.References = append(result.facts.References, swiftTypeReference(s, t.symbol, base, "inherits_type", types))
			}
		}
		for _, m := range s.members {
			if m.typeName != "" {
				result.facts.References = append(result.facts.References, swiftTypeReference(s, m.symbol, m.typeName, "declared_type", types))
			}
			for _, p := range m.parameters {
				result.facts.References = append(result.facts.References, swiftTypeReference(s, m.symbol, p.typeName, "parameter_type", types))
			}
			if m.start <= 0 || m.end <= m.start || m.end > len(s.tokens) {
				continue
			}
			function := CodeFunctionRecord{Name: m.symbol.Name, Owner: m.owner, Kind: "method", Language: "swift", File: s.file, Line: m.symbol.Line, EndLine: s.tokens[m.end].line}
			if swiftTestMember(s, m) {
				function.Kind = "test"
			}
			result.code.Functions = append(result.code.Functions, function)
			variables := map[string]string{}
			fields := map[string]string{}
			for _, field := range s.members {
				if field.owner == m.owner && field.symbol.Kind == "property" {
					variables[field.symbol.Name] = field.typeName
					variables["self."+field.symbol.Name] = field.typeName
					fields[field.symbol.Name] = field.typeName
				}
			}
			for _, p := range m.parameters {
				variables[p.name] = p.typeName
			}
			urls := map[string]string{}
			requests := map[string]string{}
			methods := map[string]string{}
			var scopes []swiftLocalScope
			for i := m.start; i < m.end; i++ {
				if s.tokens[i].text == "{" {
					scopes = append(scopes, swiftLocalScope{variables, urls, requests, methods})
					variables, urls, requests, methods = swiftCopy(variables), swiftCopy(urls), swiftCopy(requests), swiftCopy(methods)
					swiftClosureShadows(s.tokens, i+1, m.end, variables)
				} else if s.tokens[i].text == "}" && len(scopes) > 0 {
					scope := scopes[len(scopes)-1]
					scopes = scopes[:len(scopes)-1]
					variables, urls, requests, methods = scope.variables, scope.urls, scope.requests, scope.methods
				}
				if i+1 < m.end && s.tokens[i].kind == "identifier" && s.tokens[i+1].text == "=" && (i == m.start || (s.tokens[i-1].text != "let" && s.tokens[i-1].text != "var")) {
					delete(urls, s.tokens[i].text)
					delete(requests, s.tokens[i].text)
					delete(methods, s.tokens[i].text)
					for _, scope := range scopes {
						delete(scope.urls, s.tokens[i].text)
						delete(scope.requests, s.tokens[i].text)
						scope.methods[s.tokens[i].text] = "unknown"
					}
				}
				if s.tokens[i].text == "for" && i+2 < m.end && s.tokens[i+2].text == "in" {
					variables[s.tokens[i+1].text] = ""
				}
				if (s.tokens[i].text == "let" || s.tokens[i].text == "var") && i+2 < m.end {
					name := s.tokens[i+1].text
					variables[name] = ""
					delete(urls, name)
					delete(requests, name)
					delete(methods, name)
					j := i + 2
					if s.tokens[j].text == ":" {
						begin := j + 1
						j = begin
						for j < m.end && s.tokens[j].text != "=" && s.tokens[j].line == s.tokens[i].line {
							j++
						}
						variables[name] = csharpJoined(s.tokens[begin:j])
					}
					if j+1 < m.end && s.tokens[j].text == "=" {
						value := j + 1
						if s.tokens[value].kind == "identifier" && value+1 < m.end && s.tokens[value+1].text == "(" {
							variables[name] = s.tokens[value].text
						} else {
							variables[name] = swiftExpressionType(s.tokens[value:value+1], variables)
						}
						if url := swiftURLLiteral(s.tokens, value, m.end, s.pairs, urls); url != "" {
							urls[name] = url
						}
						if s.tokens[value].text == "URLRequest" && value+1 < m.end {
							if close, ok := s.pairs[value+1]; ok {
								if args := csharpSplit(s.tokens[value+2:close], ","); len(args) > 0 {
									if at := swiftArgumentValue(args[0]); at < len(args[0]) {
										requests[name] = swiftURLLiteral(args[0], at, len(args[0]), swiftLocalPairs(args[0]), urls)
									}
								}
							}
						}
					}
				}
				if i+4 < m.end && s.tokens[i+1].text == "." && s.tokens[i+3].text == "=" {
					if s.tokens[i+2].text == "httpMethod" {
						methods[s.tokens[i].text] = "unknown"
						for _, scope := range scopes {
							scope.methods[s.tokens[i].text] = "unknown"
						}
						if s.tokens[i+4].kind == "literal" {
							methods[s.tokens[i].text] = strings.ToUpper(s.tokens[i+4].text)
						}
					}
					if s.tokens[i+2].text == "url" {
						delete(requests, s.tokens[i].text)
						for _, scope := range scopes {
							delete(scope.requests, s.tokens[i].text)
						}
					}
				}
				t := s.tokens[i]
				if t.kind != "identifier" || swiftNonCallKeyword(t.text) || i+1 >= m.end || s.tokens[i+1].text != "(" {
					continue
				}
				close, ok := s.pairs[i+1]
				if !ok || close >= m.end {
					continue
				}
				receiver := ""
				if i >= 2 && (s.tokens[i-1].text == "." || s.tokens[i-1].text == "?.") {
					begin := i - 2
					for begin >= 2 && s.tokens[begin-1].text == "." && s.tokens[begin-2].kind == "identifier" {
						begin -= 2
					}
					receiver = csharpJoined(s.tokens[begin : i-1])
				}
				owner := m.owner
				name := t.text
				if receiver == "super" {
					owner = swiftSuperOwner(s, m.owner, types)
				} else if receiver != "" && receiver != "self" {
					typ := receiver
					bindings := variables
					if strings.HasPrefix(receiver, "self.") {
						bindings = fields
					}
					if value, ok := bindings[strings.TrimPrefix(receiver, "self.")]; ok {
						typ = value
					}
					bound := swiftFindTypes(s, typ, types)
					owner = ""
					if len(bound) == 1 {
						owner = bound[0].QualifiedName
					}
				} else if receiver == "" {
					if bound := swiftFindTypes(s, t.text, types); len(bound) == 1 {
						owner = bound[0].QualifiedName
						name = "init"
					}
					if _, shadowed := variables[t.text]; shadowed {
						owner = ""
					}
				}
				args := csharpSplit(s.tokens[i+2:close], ",")
				var candidates []swiftMember
				for _, candidate := range swiftInheritedMembers(owner, name, members, sources, types, map[string]bool{}) {
					if swiftArgumentsMatch(candidate.parameters, args, variables) {
						candidates = append(candidates, candidate)
					}
				}
				candidates = swiftBetterLiteralCandidates(candidates, args, variables)
				ref := RichRelationRecord{ID: stableID("swift-call", m.symbol.ID, sourceLocation(t.line), strconv.Itoa(i), receiver, t.text), From: s.file, To: receiver + "." + t.text, Type: "calls_method_owner", Language: "swift", Analyzer: "swift-source", FromSymbolID: m.symbol.ID, Line: t.line, SourceLocation: sourceLocation(t.line), Resolution: SymbolResolutionUnresolved, NonPromotable: true, Confidence: "INFERRED", Reason: "static receiver, argument labels or overload could not be uniquely bound"}
				for _, candidate := range candidates {
					ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, candidate.symbol.ID)
				}
				sort.Strings(ref.CandidateSymbolIDs)
				if len(candidates) > 1 {
					ref.Resolution = SymbolResolutionAmbiguous
				}
				if len(candidates) == 1 && len(s.limitations) == 0 && len(candidates[0].symbol.Limitations) == 1 {
					target := candidates[0].symbol
					ref.To, ref.ToSymbolID, ref.TargetQualifiedName = target.File, target.ID, target.QualifiedName
					ref.Internal, ref.NonPromotable, ref.Resolution, ref.Confidence, ref.ConfidenceScore = true, false, SymbolResolutionExact, "EXTRACTED", 1
					ref.Reason = "unique static declaration matching receiver, argument labels and known literal types; callback activation and runtime dispatch are not evaluated"
					result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("swift-edge", ref.ID, target.ID), From: MethodRefRecord{Owner: m.owner, Method: m.symbol.Name, File: s.file, Line: m.symbol.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, Type: "calls", SourceFile: s.file, Line: t.line, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: m.symbol.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
					if function.Kind == "test" {
						result.tests = append(result.tests, TestMapRecord{TestFile: s.file, TestClass: m.owner, TestMethod: m.symbol.Name, TargetFile: target.File, TargetClass: target.Owner, TargetMethod: target.Name, Type: "static-call", Line: t.line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "test declaration statically references target; no test execution is inferred"})
					}
				}
				result.facts.References = append(result.facts.References, ref)
				swiftFrameworkCall(&result, s, m, t, receiver, args, variables, urls, requests, methods, types)
			}
		}
	}
	result.capabilities = append(result.capabilities, languageCodeCapabilities("swift", "Swift / Apple", result.code)...)
	return result
}

func swiftFindTypes(s swiftSource, name string, types map[string][]RichSymbolRecord) []RichSymbolRecord {
	name = strings.TrimSuffix(strings.TrimSuffix(name, "?"), "!")
	if at := strings.Index(name, "<"); at >= 0 {
		name = name[:at]
	}
	if records := types[s.module+"."+name]; len(records) > 0 {
		return records
	}
	var result []RichSymbolRecord
	for _, imp := range s.imports {
		for qualified, records := range types {
			for _, record := range records {
				if record.Name == name && (record.Module == imp || strings.HasSuffix(record.Module, "/"+imp)) && strings.HasSuffix(qualified, "."+name) {
					result = append(result, record)
				}
			}
		}
	}
	return result
}

func swiftTypeReference(s swiftSource, from RichSymbolRecord, name, kind string, types map[string][]RichSymbolRecord) RichRelationRecord {
	ref := RichRelationRecord{ID: stableID("swift-type", from.ID, name, kind), From: s.file, To: name, Type: kind, Language: "swift", Analyzer: "swift-source", FromSymbolID: from.ID, TargetQualifiedName: name, Line: from.Line, SourceLocation: from.SourceLocation, Resolution: SymbolResolutionUnresolved, NonPromotable: true, Confidence: "INFERRED", Reason: "external or ambiguous static type; compiler binding is not evaluated"}
	if candidates := swiftFindTypes(s, name, types); len(candidates) == 1 && len(s.limitations) == 0 && len(candidates[0].Limitations) == 1 {
		target := candidates[0]
		ref.To, ref.ToSymbolID, ref.TargetQualifiedName = target.File, target.ID, target.QualifiedName
		ref.Internal, ref.NonPromotable, ref.Resolution, ref.Confidence = true, false, SymbolResolutionExact, "EXTRACTED"
		ref.ConfidenceScore = 1
		ref.Reason = "unique indexed source type; runtime dispatch and compiler target membership are not evaluated"
	}
	return ref
}

func swiftArgumentValue(tokens []csharpToken) int {
	if len(tokens) >= 2 && tokens[1].text == ":" {
		return 2
	}
	return 0
}
func swiftLocalPairs(tokens []csharpToken) map[int]int { pairs, _ := csharpPairs(tokens); return pairs }
func swiftArgumentsMatch(params []swiftParameter, args [][]csharpToken, variables map[string]string) bool {
	for _, arg := range args {
		if csharpHas(arg, "<") || csharpHas(arg, ">") {
			return false
		}
	}

	if len(args) == 1 && len(args[0]) == 0 {
		args = nil
	}
	at := 0
	for _, param := range params {
		if at >= len(args) {
			if param.optional {
				continue
			}
			return false
		}
		value := swiftArgumentValue(args[at])
		label := "_"
		if value == 2 {
			label = args[at][0].text
		}
		if label != param.label {
			if param.optional {
				continue
			}
			return false
		}
		if param.variadic {
			return false
		}
		arg := args[at][value:]
		actual := swiftExpressionType(arg, variables)
		if actual != "" && param.typeName != actual && param.typeName != actual+"?" && !swiftNumericLiteralCompatible(arg, param.typeName) {
			return false
		}
		at++
	}
	return at == len(args)
}
func swiftExpressionType(tokens []csharpToken, variables map[string]string) string {
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) == 1 {
		switch {
		case tokens[0].kind == "literal":
			return "String"
		case tokens[0].kind == "number":
			return "Int"
		case tokens[0].text == "true" || tokens[0].text == "false":
			return "Bool"
		case tokens[0].kind == "identifier":
			return variables[tokens[0].text]
		}
	}
	if len(tokens) >= 3 && tokens[0].kind == "number" && tokens[1].text == "." && tokens[2].kind == "number" {
		return "Double"
	}
	if len(tokens) > 1 && tokens[0].kind == "identifier" && tokens[1].text == "(" {
		return tokens[0].text
	}
	return ""
}
func swiftTestMember(s swiftSource, m swiftMember) bool {
	if hasSwiftImport(s, "Testing") {
		for _, a := range m.attributes {
			if a == "Test" {
				return true
			}
		}
	}
	if hasSwiftImport(s, "XCTest") && strings.HasPrefix(m.symbol.Name, "test") {
		for _, t := range s.types {
			if t.symbol.QualifiedName == m.owner {
				for _, base := range t.bases {
					if base == "XCTestCase" {
						return true
					}
				}
			}
		}
	}
	return false
}
func hasSwiftImport(s swiftSource, name string) bool {
	for _, imp := range s.imports {
		if imp == name {
			return true
		}
	}
	return false
}
func swiftNonCallKeyword(name string) bool {
	switch name {
	case "if", "while", "for", "switch", "catch", "guard", "return", "throw", "await", "try", "some", "any", "in", "case", "func":
		return true
	}
	return false
}
