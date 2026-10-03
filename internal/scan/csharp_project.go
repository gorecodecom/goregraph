package scan

import (
	"path"
	"sort"
	"strconv"
	"strings"
)

type csharpAnalysis struct {
	facts        ProjectSymbolFacts
	code         CodeIntelligenceRecord
	graph        CallGraphRecord
	tests        []TestMapRecord
	capabilities []ArchitectureCapabilityFact
}

// analyzeCSharpProject resolves supported static bindings without running a compiler.
// Ambiguous overloads, dynamic receivers and conditional compilation stay unresolved.
func analyzeCSharpProject(sources []csharpSource) csharpAnalysis {
	result := csharpAnalysis{}
	types := map[string][]RichSymbolRecord{}
	members := map[string][]csharpMember{}
	for _, source := range sources {
		for _, typ := range source.types {
			types[typ.symbol.QualifiedName] = append(types[typ.symbol.QualifiedName], typ.symbol)
			result.facts.Declarations = append(result.facts.Declarations, typ.symbol)
		}
		for _, member := range source.members {
			result.facts.Declarations = append(result.facts.Declarations, member.symbol)
			members[member.owner+"."+member.symbol.Name] = append(members[member.owner+"."+member.symbol.Name], member)
		}
	}
	for _, source := range sources {
		result.facts.References = append(result.facts.References, csharpEntityReferences(source, sources, types)...)
		for _, typ := range source.types {
			for _, base := range typ.bases {
				result.facts.References = append(result.facts.References, csharpTypeReference(source, typ.symbol, base, "inherits_type", types))
			}
		}
		for _, member := range source.members {
			for _, parameterType := range member.parameterTypes {
				result.facts.References = append(result.facts.References, csharpTypeReference(source, member.symbol, parameterType, "parameter_type", types))
			}
			if member.typeName != "" {
				result.facts.References = append(result.facts.References, csharpTypeReference(source, member.symbol, member.typeName, "declared_type", types))
			}
			if member.symbol.Kind != "method" && member.symbol.Kind != "constructor" && member.symbol.Kind != "top_level" {
				continue
			}
			function := CodeFunctionRecord{Name: member.symbol.Name, Owner: member.owner, Kind: "method", Language: "csharp", File: source.file, Line: member.symbol.Line}
			if member.end < len(source.tokens) {
				function.EndLine = source.tokens[member.end].line
			}
			for _, attr := range member.attributes {
				switch csharpAttributeName(attr) {
				case "Test", "TestCase", "Fact", "Theory", "UnityTest":
					function.Kind = "test"
				}
			}
			result.code.Functions = append(result.code.Functions, function)
			csharpRoutes(source, member, &result.code)
			variables := map[string]string{}
			fields := map[string]string{}
			var scopes []map[string]string
			for _, field := range source.members {
				if field.owner == member.owner && (field.symbol.Kind == "field" || field.symbol.Kind == "property") {
					variables[field.symbol.Name] = field.typeName
					fields[field.symbol.Name] = field.typeName
				}
			}
			for name, typ := range member.parameters {
				variables[name] = typ
			}

			for i := member.start; i < member.end; i++ {
				if member.symbol.Kind == "top_level" && csharpTypeKeyword(source.tokens[i]) {
					if end := csharpTypeBodyEnd(source, i); end > i {
						i = end
						continue
					}
				}

				if source.tokens[i].text == ")" && i+1 < member.end && source.tokens[i+1].text == "=>" {
					if open, ok := source.pairs[i]; ok {
						for _, token := range source.tokens[open+1 : i] {
							if token.kind == "identifier" {
								variables[token.text] = "unknown"
							}
						}
					}
				}
				if i+1 < member.end && source.tokens[i].kind == "identifier" && source.tokens[i+1].text == "=>" {
					variables[source.tokens[i].text] = "unknown"
				}
				if source.tokens[i].text == "foreach" && i+4 < member.end && source.tokens[i+1].text == "(" {
					variables[source.tokens[i+3].text] = "unknown"
				}

				if source.tokens[i].kind == "punctuation" && source.tokens[i].text == "{" {
					scopes = append(scopes, variables)
					copy := map[string]string{}
					for key, value := range variables {
						copy[key] = value
					}
					variables = copy
				}
				if source.tokens[i].kind == "punctuation" && source.tokens[i].text == "}" && len(scopes) > 0 {
					variables = scopes[len(scopes)-1]
					scopes = scopes[:len(scopes)-1]
				}
				if i+2 < member.end && source.tokens[i].kind == "identifier" && source.tokens[i+1].kind == "identifier" && source.tokens[i+2].text == "=" {
					typ := source.tokens[i].text
					if typ == "var" {
						typ = "unknown"
						if i+3 < member.end {
							if inferred := csharpExpressionType(source.tokens[i+3:i+4], variables); inferred != "" {
								typ = inferred
							}
						}
						if i+4 < member.end && source.tokens[i+3].text == "new" && source.tokens[i+4].kind == "identifier" {
							typ = source.tokens[i+4].text
						}
					}
					if inferred := csharpBuilderLocal(source, i+3, member.end, variables, types); inferred != "" && source.tokens[i].text == "var" {
						typ = inferred
					}
					variables[source.tokens[i+1].text] = typ
				}
				token := source.tokens[i]
				if token.kind != "identifier" || csharpNonCallKeyword(token.text) {
					continue
				}
				open := i + 1
				if open < member.end && source.tokens[open].text == "<" {
					depth := 0
					for open < member.end {
						if source.tokens[open].text == "<" {
							depth++
						}
						if source.tokens[open].text == ">" {
							depth--
							if depth == 0 {
								open++
								break
							}
						}
						open++
					}
				}
				if open >= member.end || source.tokens[open].text != "(" {
					continue
				}
				close, ok := source.pairs[open]
				if !ok || close >= member.end {
					continue
				}
				receiver := ""
				if i >= 2 && (source.tokens[i-1].text == "." || source.tokens[i-1].text == "?.") {
					begin := i - 2
					for begin >= 2 && source.tokens[begin-1].text == "." && source.tokens[begin-2].kind == "identifier" {
						begin -= 2
					}
					receiver = csharpJoined(source.tokens[begin : i-1])
				}
				targetType := member.owner
				if receiver == "" {
					if _, shadowed := variables[token.text]; shadowed {
						targetType = ""
					}
				}
				if i > member.start && source.tokens[i-1].text == "new" {
					targetType = ""
				}
				if receiver != "" && receiver != "this" {
					targetType = receiver
					if value, found := variables[receiver]; found {
						targetType = value
					}
					if strings.HasPrefix(receiver, "this.") {
						targetType = fields[strings.TrimPrefix(receiver, "this.")]
					}
					candidates := csharpFindTypes(source, member.owner, targetType, types)
					if len(candidates) == 1 {
						targetType = candidates[0].QualifiedName
					} else {
						targetType = ""
					}
				}
				candidates := []csharpMember{}
				if targetType != "" {
					for _, candidate := range csharpInheritedMembers(source, targetType, token.text, members, sources, types, map[string]bool{}) {
						if args, valid := csharpBindArguments(candidate, source.tokens[open+1:close]); csharpModuleVisible(source, candidate.symbol.Module) && candidate.symbol.Kind == "method" && valid && csharpMatchingArguments(candidate, args, variables) {
							candidates = append(candidates, candidate)
						}
					}
				}
				if !csharpHas(source.tokens[open+1:close], ":") && len(candidates) > 0 && len(candidates[0].parameterInfo) == csharpParameterCount(source.tokens[open+1:close]) {
					candidates = csharpBetterOverloads(candidates, csharpSplit(source.tokens[open+1:close], ","), variables)
				}
				reference := RichRelationRecord{From: source.file, To: receiver + "." + token.text, TargetQualifiedName: receiver + "." + token.text, Type: "calls_method_owner", Language: "csharp", Analyzer: "csharp-source", Line: token.line, SourceLocation: sourceLocation(token.line), FromSymbolID: member.symbol.ID, Resolution: SymbolResolutionUnresolved, NonPromotable: true, Confidence: "INFERRED", Reason: "static receiver or overload could not be uniquely bound; runtime dispatch is not evaluated"}
				for _, candidate := range candidates {
					reference.CandidateSymbolIDs = append(reference.CandidateSymbolIDs, candidate.symbol.ID)
				}
				sort.Strings(reference.CandidateSymbolIDs)
				if len(candidates) > 1 {
					reference.Resolution = SymbolResolutionAmbiguous
				}
				if len(candidates) == 1 && len(source.limitations) == 0 && len(candidates[0].symbol.Limitations) == 1 {
					provider := candidates[0].symbol
					reference.ToSymbolID = provider.ID
					reference.To = provider.File
					reference.TargetQualifiedName = provider.QualifiedName
					reference.Internal = true
					reference.Resolution = SymbolResolutionExact
					reference.NonPromotable = false
					reference.Confidence = "EXTRACTED"
					reference.ConfidenceScore = 1
					reference.Reason = "unique static declaration with matching receiver type and argument count; runtime dispatch is not evaluated"
					edge := CallGraphEdgeRecord{From: MethodRefRecord{Owner: member.owner, Method: member.symbol.Name, File: source.file, Line: member.symbol.Line}, To: MethodRefRecord{Owner: provider.Owner, Method: provider.Name, File: provider.File, Line: provider.Line}, Type: "calls", SourceFile: source.file, Line: token.line, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: member.symbol.ID, ToSymbolID: provider.ID, TargetQualifiedName: provider.QualifiedName, Resolution: SymbolResolutionExact, Reason: reference.Reason}
					edge.ID = stableID("csharp-call", member.symbol.ID, provider.ID, sourceLocation(token.line), strconv.Itoa(i))
					result.graph.Edges = append(result.graph.Edges, edge)
					if function.Kind == "test" {
						result.tests = append(result.tests, TestMapRecord{TestFile: source.file, TestClass: member.owner, TestMethod: member.symbol.Name, TargetFile: provider.File, TargetClass: provider.Owner, TargetMethod: provider.Name, Type: "static-call", Line: token.line, ConfidenceScore: 1, Reason: "test declaration statically calls target; no test execution is inferred"})
					}
				}
				reference.ID = stableID("csharp-reference", source.file, member.symbol.ID, token.text, receiver, sourceLocation(token.line), strconv.Itoa(i))
				result.facts.References = append(result.facts.References, reference)
				csharpHTTPClient(source, member, receiver, token, open, close, variables, fields, types, &result.code)
				csharpMinimalRoute(source, member, token, receiver, open, close, variables, types, &result.code)
				csharpFrameworkCall(&result, source, member, token, receiver, i, open, close, variables, fields, sources, types)
			}
		}
	}
	result.capabilities = append(result.capabilities, languageCodeCapabilities("csharp", "ASP.NET / .NET / Unity", result.code)...)
	return result
}

func csharpFindTypes(source csharpSource, owner, name string, types map[string][]RichSymbolRecord) []RichSymbolRecord {
	name = strings.TrimPrefix(name, "global::")
	if value, ok := source.aliases[name]; ok {
		name = value
	}
	if at := strings.Index(name, "<"); at >= 0 {
		name = name[:at]
	}
	name = strings.TrimSuffix(strings.TrimSuffix(name, "?"), "[]")
	for scope := owner; scope != ""; {
		if records := types[scope+"."+name]; len(records) > 0 {
			return csharpVisibleTypes(source, records)
		}
		at := strings.LastIndex(scope, ".")
		if at < 0 {
			break
		}
		scope = scope[:at]
	}
	if records := types[name]; len(records) > 0 {
		return csharpVisibleTypes(source, records)
	}
	var candidates []RichSymbolRecord
	for _, ns := range source.imports {
		candidates = append(candidates, types[ns+"."+name]...)
	}
	return csharpVisibleTypes(source, candidates)
}

func csharpTypeReference(source csharpSource, from RichSymbolRecord, name, kind string, types map[string][]RichSymbolRecord) RichRelationRecord {
	reference := RichRelationRecord{From: source.file, To: name, TargetQualifiedName: name, Type: kind, Language: "csharp", Analyzer: "csharp-source", Line: from.Line, SourceLocation: sourceLocation(from.Line), FromSymbolID: from.ID, Resolution: SymbolResolutionUnresolved, NonPromotable: true, Confidence: "INFERRED", Reason: "external or unsupported type binding"}
	candidates := csharpFindTypes(source, from.Owner, name, types)
	for _, candidate := range candidates {
		reference.CandidateSymbolIDs = append(reference.CandidateSymbolIDs, candidate.ID)
	}
	sort.Strings(reference.CandidateSymbolIDs)
	if len(candidates) > 1 {
		reference.Resolution = SymbolResolutionAmbiguous
	}
	if len(candidates) == 1 && len(source.limitations) == 0 && len(candidates[0].Limitations) == 1 {
		target := candidates[0]
		reference.ToSymbolID = target.ID
		reference.To = target.File
		reference.TargetQualifiedName = target.QualifiedName
		reference.Resolution = SymbolResolutionExact
		reference.NonPromotable = false
		reference.Internal = true
		reference.Confidence = "EXTRACTED"
		reference.ConfidenceScore = 1
		reference.Reason = "unique source type declaration"
	}
	reference.ID = stableID("csharp-type", from.ID, name, kind)
	return reference
}

func csharpNonCallKeyword(name string) bool {
	switch name {
	case "if", "for", "foreach", "while", "switch", "catch", "using", "lock", "nameof", "typeof", "sizeof", "checked", "unchecked", "default", "delegate", "return", "base", "this":
		return true
	}
	return false
}

func csharpRoutes(source csharpSource, member csharpMember, code *CodeIntelligenceRecord) {
	prefix := ""
	bound := false
	if len(source.limitations) > 0 {
		return
	}
	for _, typ := range source.types {
		if typ.symbol.QualifiedName != member.owner {
			continue
		}
		for _, base := range typ.bases {
			if strings.Contains(base, "Controller") {
				bound = true
			}
		}
		for _, attr := range typ.attributes {
			if csharpAttributeName(attr) == "ApiController" {
				bound = true
			}
			if csharpAttributeName(attr) == "Route" && !attr.literal {
				return
			}
			if csharpAttributeName(attr) == "Route" && attr.literal {
				prefix = attr.value
				bound = true
			}
		}
	}
	if !bound {
		return
	}
	controller := strings.TrimSuffix(path.Base(strings.ReplaceAll(member.owner, ".", "/")), "Controller")
	for _, attr := range member.attributes {
		method := ""
		switch csharpAttributeName(attr) {
		case "HttpGet":
			method = "GET"
		case "HttpPost":
			method = "POST"
		case "HttpPut":
			method = "PUT"
		case "HttpDelete":
			method = "DELETE"
		case "HttpPatch":
			method = "PATCH"
		case "HttpHead":
			method = "HEAD"
		case "HttpOptions":
			method = "OPTIONS"
		}
		if method == "" {
			continue
		}
		if !attr.literal && attr.hasArguments {
			continue
		}
		// An attribute without an argument inherits the literal controller route.
		route := prefix
		if attr.literal {
			route = strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(attr.value, "/")
		}
		route = strings.ReplaceAll(strings.ReplaceAll(route, "[controller]", controller), "[action]", member.symbol.Name)
		code.Routes = append(code.Routes, CodeRouteRecord{Language: "csharp", Framework: "aspnet-core", FrameworkBound: true, Kind: "backend", HTTPMethod: method, Path: "/" + strings.TrimPrefix(route, "/"), Handler: member.owner + "." + member.symbol.Name, File: source.file, Line: attr.line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "literal ASP.NET controller and HTTP method attributes; middleware and runtime routing are not evaluated"})
	}
}

func csharpHTTPClient(source csharpSource, member csharpMember, receiver string, token csharpToken, open, close int, variables, fields map[string]string, types map[string][]RichSymbolRecord, code *CodeIntelligenceRecord) {
	if len(source.limitations) > 0 {
		return
	}
	bindings := variables
	if strings.HasPrefix(receiver, "this.") {
		receiver = strings.TrimPrefix(receiver, "this.")
		bindings = fields
	}
	typ := bindings[receiver]
	if typ == "HttpClient" && (!csharpImported(source, "System.Net.Http") || len(csharpFindTypes(source, member.owner, typ, types)) > 0) {
		return
	}
	if typ != "HttpClient" && typ != "System.Net.Http.HttpClient" {
		return
	}
	method := ""
	switch token.text {
	case "GetAsync", "GetStringAsync", "GetByteArrayAsync":
		method = "GET"
	case "PostAsync", "PostAsJsonAsync":
		method = "POST"
	case "PutAsync", "PutAsJsonAsync":
		method = "PUT"
	case "DeleteAsync":
		method = "DELETE"
	case "PatchAsync":
		method = "PATCH"
	}
	args := csharpSplit(source.tokens[open+1:close], ",")
	if method == "" || len(args) == 0 {
		return
	}
	if len(args[0]) != 1 || args[0][0].kind != "literal" {
		return
	}
	code.APIContracts = append(code.APIContracts, APIContractRecord{Language: "csharp", HTTPMethod: method, Path: args[0][0].text, RawPath: args[0][0].text, Caller: member.owner + "." + member.symbol.Name, File: source.file, Line: token.line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "literal HttpClient request; runtime base address is not evaluated"})
}

func csharpModuleVisible(source csharpSource, module string) bool {
	return module == source.module || source.visibleModules[module]
}
func csharpVisibleTypes(source csharpSource, symbols []RichSymbolRecord) []RichSymbolRecord {
	var visible []RichSymbolRecord
	for _, symbol := range symbols {
		if csharpModuleVisible(source, symbol.Module) {
			visible = append(visible, symbol)
		}
	}
	return visible
}

func assignCSharpModules(sources []csharpSource, metadata ProjectSymbolFacts) {
	for i := range sources {
		source := &sources[i]
		closest := ""
		for _, declaration := range metadata.Declarations {
			if declaration.Analyzer != "unity-assembly" || !(strings.HasSuffix(declaration.File, ".asmdef") || strings.HasSuffix(declaration.File, ".asmref")) {
				continue
			}
			directory := path.Dir(declaration.File)
			if directory == "." || strings.HasPrefix(source.file, directory+"/") {
				if len(directory) > len(closest) {
					closest = directory
					source.module = declaration.File
					if strings.HasSuffix(declaration.File, ".asmref") {
						for _, reference := range metadata.References {
							if reference.From == declaration.File && reference.Resolution == SymbolResolutionExact {
								source.module = reference.To
							}
						}
					}
				}
			}
		}
		source.visibleModules = map[string]bool{source.module: true}
		for _, reference := range metadata.References {
			if reference.From == source.module && reference.Resolution == SymbolResolutionExact {
				source.visibleModules[reference.To] = true
			}
		}
		for j := range source.types {
			source.types[j].symbol.Module = source.module
		}
		for j := range source.members {
			source.members[j].symbol.Module = source.module
		}
	}
}
