package scan

import "strings"

func analyzeDartProject(sources []dartSource, packages []DartPackageRecord) dartAnalysis {
	var result dartAnalysis
	project := newDartProject(sources, packages)
	project.extractDartTests()
	project.extractDartClosures()
	for _, s := range project.sources {
		for _, typ := range s.types {
			result.facts.Declarations = append(result.facts.Declarations, typ.symbol)
		}
		for _, member := range s.members {
			result.facts.Declarations = append(result.facts.Declarations, member.symbol)
			switch member.symbol.Kind {
			case "method", "function", "constructor", "getter", "setter", "test", "closure":
				endLine := member.symbol.Line
				if member.end < len(s.tokens) {
					endLine = s.tokens[member.end].line
				}
				result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: member.symbol.Name, Owner: member.symbol.Owner, Kind: member.symbol.Kind, Language: "dart", File: s.file, Line: member.symbol.Line, EndLine: endLine})
			}
		}
		for _, directive := range s.directives {
			result.facts.References = append(result.facts.References, RichRelationRecord{From: s.file, To: directive.uri, Type: directive.kind + "s_library", Language: "dart", Analyzer: "dart-source", Line: directive.line, SourceLocation: sourceLocation(directive.line), Resolution: SymbolResolutionUnresolved, NonPromotable: true, preventExact: true, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "literal Dart library directive; external libraries are not executed"})
		}
	}
	for _, s := range project.sources {
		project.bindSource(&result, s)
	}
	result.capabilities = append(result.capabilities, languageCodeCapabilities("dart", "Dart / Flutter", result.code)...)
	return result
}

func dartBaseType(name string) string {
	name = strings.TrimSuffix(name, "?")
	if i := strings.Index(name, "<"); i >= 0 {
		name = name[:i]
	}
	return name
}
