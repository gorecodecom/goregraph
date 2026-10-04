package scan

import (
	"strconv"
	"strings"
)

func csharpImported(s csharpSource, namespace string) bool {
	for _, imp := range s.imports {
		if imp == namespace {
			return true
		}
	}
	return false
}

func csharpFrameworkType(s csharpSource, owner, name, namespace, expected string, sources []csharpSource, types map[string][]RichSymbolRecord, visited map[string]bool) bool {
	if name == namespace+"."+expected || name == expected && csharpImported(s, namespace) && len(csharpFindTypes(s, owner, name, types)) == 0 {
		return true
	}
	bound := csharpFindTypes(s, owner, name, types)
	if len(bound) != 1 || visited[bound[0].ID] {
		return false
	}
	visited[bound[0].ID] = true
	for _, provider := range sources {
		for _, typ := range provider.types {
			if typ.symbol.ID != bound[0].ID || len(provider.limitations) > 0 {
				continue
			}
			for _, base := range typ.bases {
				if csharpFrameworkType(provider, typ.symbol.Owner, base, namespace, expected, sources, types, visited) {
					return true
				}
			}
		}
	}
	return false
}

func csharpFrameworkCall(result *csharpAnalysis, s csharpSource, m csharpMember, token csharpToken, receiver string, at, open, close int, variables, fields map[string]string, sources []csharpSource, types map[string][]RichSymbolRecord) {
	if len(s.limitations) > 0 || receiver == "" {
		return
	}
	bindings := variables
	if strings.HasPrefix(receiver, "this.") {
		receiver = strings.TrimPrefix(receiver, "this.")
		bindings = fields
	}
	typ := bindings[receiver]
	if strings.HasSuffix(receiver, ".Services") {
		base := strings.TrimSuffix(receiver, ".Services")
		if bindings[base] == "Microsoft.AspNetCore.Builder.WebApplicationBuilder" && csharpImported(s, "Microsoft.Extensions.DependencyInjection") {
			typ = "Microsoft.Extensions.DependencyInjection.IServiceCollection"
		}
	}
	const ef = "Microsoft.EntityFrameworkCore"
	framework := ""
	if csharpFrameworkType(s, m.owner, typ, ef, "DbContext", sources, types, map[string]bool{}) {
		switch token.text {
		case "SaveChanges", "SaveChangesAsync", "Add", "AddAsync", "Update", "Remove", "Find", "FindAsync":
			framework = "Entity Framework Core"
		}
	}
	if dot := strings.Index(receiver, "."); dot > 0 {
		base := receiver[:dot]
		bound := csharpFindTypes(s, m.owner, bindings[base], types)
		if len(bound) == 1 && csharpFrameworkType(s, m.owner, bindings[base], ef, "DbContext", sources, types, map[string]bool{}) {
			for _, provider := range sources {
				for _, property := range provider.members {
					if property.owner == bound[0].QualifiedName && property.symbol.Module == bound[0].Module && property.symbol.Name == receiver[dot+1:] {
						typ = property.typeName
						s = csharpFrameworkSource(s, provider)
					}
				}
			}
		}
	}
	baseType := typ
	if generic := strings.Index(baseType, "<"); generic >= 0 {
		baseType = baseType[:generic]
	}
	if csharpFrameworkType(s, m.owner, baseType, ef, "DbSet", sources, types, map[string]bool{}) {
		switch token.text {
		case "Add", "AddAsync", "AddRange", "AddRangeAsync", "Update", "UpdateRange", "Remove", "RemoveRange", "Find", "FindAsync", "ToListAsync", "FirstOrDefaultAsync", "SingleOrDefaultAsync", "CountAsync", "AnyAsync":
			framework = "Entity Framework Core"
		}
	}
	if framework != "" {
		qualified := framework + "." + token.text + "@" + s.file + ":" + sourceLocation(token.line) + ":" + strconv.Itoa(at)
		fact := s.declaration("persistence", token.text, qualified, m.owner, token.line)
		fact.Limitations = append(fact.Limitations, "source operation only; query translation, execution and transaction success are not evaluated")
		result.facts.Declarations = append(result.facts.Declarations, fact)
		result.facts.References = append(result.facts.References, RichRelationRecord{ID: stableID("csharp-persistence", m.symbol.ID, fact.ID), From: s.file, To: s.file, Type: "uses_persistence", Language: "csharp", Analyzer: "csharp-source", FromSymbolID: m.symbol.ID, ToSymbolID: fact.ID, TargetQualifiedName: qualified, Line: token.line, SourceLocation: sourceLocation(token.line), Internal: true, Resolution: SymbolResolutionExact, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "typed context or DbSet receiver identifies a source operation; no database execution is inferred"})
		result.capabilities = append(result.capabilities, ArchitectureCapabilityFact{ID: fact.ID, Language: "csharp", Capability: CapabilityPersistence, Kind: "persistence", Framework: framework, File: s.file, Line: token.line})
	}
	const di = "Microsoft.Extensions.DependencyInjection"
	if token.text != "AddScoped" && token.text != "AddSingleton" && token.text != "AddTransient" {
		return
	}
	if !csharpFrameworkType(s, m.owner, typ, di, "IServiceCollection", sources, types, map[string]bool{}) && !csharpFrameworkType(s, m.owner, typ, di, "ServiceCollection", sources, types, map[string]bool{}) {
		return
	}
	if at+1 >= open || s.tokens[at+1].text != "<" || s.tokens[open-1].text != ">" || close != open+1 {
		return
	}
	args := csharpSplit(s.tokens[at+2:open-1], ",")
	if len(args) != 1 && len(args) != 2 {
		return
	}
	qualified := token.text + "<" + csharpJoined(s.tokens[at+2:open-1]) + ">@" + s.file + ":" + strconv.Itoa(at)
	fact := s.declaration("registration", token.text, qualified, m.owner, token.line)
	fact.Limitations = append(fact.Limitations, "registration declaration only; service creation, factory execution, lifetime correctness and runtime resolution are not inferred")
	result.facts.Declarations = append(result.facts.Declarations, fact)
	for index, arg := range args {
		kind := "registers_service"
		if index == 1 {
			kind = "registers_implementation"
		}
		result.facts.References = append(result.facts.References, csharpTypeReference(s, fact, csharpJoined(arg), kind, types))
	}
}

// csharpFrameworkSource uses provider imports while preserving the call site's identity.
func csharpFrameworkSource(caller, provider csharpSource) csharpSource {
	caller.imports, caller.aliases = provider.imports, provider.aliases
	return caller
}
