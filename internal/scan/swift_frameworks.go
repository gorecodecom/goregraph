package scan

import (
	"strconv"
	"strings"
)

func swiftURLLiteral(tokens []csharpToken, start, end int, pairs map[int]int, urls map[string]string) string {
	if start < 0 || start >= end {
		return ""
	}
	if value := urls[tokens[start].text]; value != "" {
		return value
	}
	if tokens[start].text != "URL" || start+1 >= end || tokens[start+1].text != "(" {
		return ""
	}
	close, ok := pairs[start+1]
	if !ok || close >= end {
		return ""
	}
	for _, arg := range csharpSplit(tokens[start+2:close], ",") {
		if len(arg) == 3 && arg[0].text == "string" && arg[1].text == ":" && arg[2].kind == "literal" {
			return arg[2].text
		}
	}
	return ""
}

func swiftFrameworkCall(result *swiftAnalysis, s swiftSource, m swiftMember, token csharpToken, receiver string, args [][]csharpToken, variables, urls, requests, methods map[string]string, types map[string][]RichSymbolRecord) {
	if len(s.limitations) > 0 {
		return
	}
	if (hasSwiftImport(s, "Foundation") || hasSwiftImport(s, "FoundationNetworking")) && len(swiftFindTypes(s, "URLSession", types)) == 0 && (receiver == "URLSession.shared" || (variables[receiver] == "URLSession" || variables[receiver] == "Foundation.URLSession" || variables[receiver] == "FoundationNetworking.URLSession")) {
		if token.text == "data" || token.text == "dataTask" || token.text == "download" || token.text == "downloadTask" || token.text == "upload" {
			if len(args) > 0 && len(args[0]) >= 3 {
				url, method := "", "GET"
				label := args[0][0].text
				value := args[0][2:]
				if label == "from" || label == "with" {
					url = swiftURLLiteral(value, 0, len(value), swiftLocalPairs(value), urls)
				}
				if (label == "for" || label == "with") && len(value) == 1 {
					if request := requests[value[0].text]; request != "" {
						url = request
						if configured := methods[value[0].text]; configured != "" {
							method = configured
						}
					}
				}
				if url != "" && isHTTPMethod(method) {
					result.code.APIContracts = append(result.code.APIContracts, APIContractRecord{Language: "swift", HTTPMethod: method, Path: url, RawPath: url, Caller: m.owner + "." + m.symbol.Name, File: s.file, Line: token.line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "literal URLSession source request; scheduling, authentication and response behavior are not evaluated"})
				}
			}
		}
	}
	typ := variables[receiver]
	framework := ""
	if (typ == "ModelContext" || typ == "SwiftData.ModelContext") && hasSwiftImport(s, "SwiftData") && len(swiftFindTypes(s, typ, types)) == 0 {
		framework = "SwiftData"
	}
	if (typ == "NSManagedObjectContext" || typ == "CoreData.NSManagedObjectContext") && hasSwiftImport(s, "CoreData") && len(swiftFindTypes(s, typ, types)) == 0 {
		framework = "Core Data"
	}
	if (receiver == "UserDefaults.standard" || (typ == "UserDefaults" || typ == "Foundation.UserDefaults")) && hasSwiftImport(s, "Foundation") && len(swiftFindTypes(s, "UserDefaults", types)) == 0 {
		framework = "UserDefaults"
	}
	if swiftPersistenceOperation(framework, token.text) {
		qualified := framework + "." + token.text + "@" + s.file + ":" + sourceLocation(token.line)
		fact := s.declaration("persistence", token.text, qualified, m.owner, token.line)
		fact.Limitations = append(fact.Limitations, "source operation only; transaction success, durable writes and callback execution are not inferred")
		result.facts.Declarations = append(result.facts.Declarations, fact)
		result.facts.References = append(result.facts.References, RichRelationRecord{ID: stableID("swift-persistence", m.symbol.ID, fact.ID), From: s.file, To: s.file, Type: "uses_persistence", Language: "swift", Analyzer: "swift-source", FromSymbolID: m.symbol.ID, ToSymbolID: fact.ID, TargetQualifiedName: qualified, Line: token.line, SourceLocation: sourceLocation(token.line), Internal: true, Resolution: SymbolResolutionExact, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "imported framework and typed receiver identify a source operation; execution and commit are not proven"})
		result.capabilities = append(result.capabilities, ArchitectureCapabilityFact{ID: fact.ID, Language: "swift", Capability: CapabilityPersistence, Kind: "persistence", Framework: framework, File: s.file, Line: token.line})
	}
}

func swiftPersistenceOperation(framework, method string) bool {
	if framework == "UserDefaults" {
		switch method {
		case "set", "removeObject", "object", "string", "bool", "integer", "double", "data", "array", "dictionary":
			return true
		}
		return false
	}
	if framework == "SwiftData" || framework == "Core Data" {
		switch method {
		case "save", "insert", "delete", "fetch", "rollback":
			return true
		}
	}
	return framework == "Core Data" && (method == "perform" || method == "performAndWait")
}

func isHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}

func swiftStateReferences(s swiftSource, result *swiftAnalysis) {
	if !hasSwiftImport(s, "SwiftUI") || len(s.limitations) > 0 {
		return
	}
	for _, property := range s.members {
		state := false
		for _, attr := range property.attributes {
			switch attr {
			case "State", "StateObject", "ObservedObject", "EnvironmentObject", "Binding", "Environment", "AppStorage", "SceneStorage", "Bindable", "Query":
				state = true
			}
		}
		if !state {
			continue
		}
		for _, member := range s.members {
			if member.owner != property.owner || member.start <= 0 {
				continue
			}
			shadowed := false
			var scopes []bool
			for _, p := range member.parameters {
				if p.name == property.symbol.Name {
					shadowed = true
				}
			}
			for i := member.start; i < member.end; i++ {
				if s.tokens[i].text == "{" {
					scopes = append(scopes, shadowed)
					names := map[string]string{}
					swiftClosureShadows(s.tokens, i+1, member.end, names)
					if _, exists := names[property.symbol.Name]; exists {
						shadowed = true
					}
				}
				if s.tokens[i].text == "}" && len(scopes) > 0 {
					shadowed = scopes[len(scopes)-1]
					scopes = scopes[:len(scopes)-1]
				}

				if s.tokens[i].text == "for" && i+1 < member.end && s.tokens[i+1].text == property.symbol.Name {
					shadowed = true
				}
				if (s.tokens[i].text == "let" || s.tokens[i].text == "var") && i+1 < member.end && s.tokens[i+1].text == property.symbol.Name {
					shadowed = true
				}
				if s.tokens[i].kind != "identifier" || strings.TrimPrefix(s.tokens[i].text, "$") != property.symbol.Name {
					continue
				}
				if i+1 < member.end && s.tokens[i+1].text == ":" {
					continue
				}
				explicitSelf := i >= 2 && s.tokens[i-1].text == "." && s.tokens[i-2].text == "self"
				if shadowed && !explicitSelf {
					continue
				}
				if i > 0 && s.tokens[i-1].text == "." && !explicitSelf {
					continue
				}
				result.facts.References = append(result.facts.References, RichRelationRecord{ID: stableID("swift-state", member.symbol.ID, property.symbol.ID, sourceLocation(s.tokens[i].line), strconv.Itoa(i)), From: s.file, To: s.file, Type: "uses_state", Language: "swift", Analyzer: "swift-source", FromSymbolID: member.symbol.ID, ToSymbolID: property.symbol.ID, TargetQualifiedName: property.symbol.QualifiedName, Line: s.tokens[i].line, SourceLocation: sourceLocation(s.tokens[i].line), Internal: true, Resolution: SymbolResolutionExact, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "lexical reference to a SwiftUI property wrapper; observation delivery, view refresh and actor scheduling are not proven"})
			}
		}
	}
}
