package scan

import (
	"fmt"
	"strings"
)

func analyzeBatchSource(source supplementarySource, result *supplementaryAnalysis) {
	file := source.file
	labels := map[string][]RichSymbolRecord{}
	lines := strings.Split(source.body, "\n")
	for n, line := range lines {
		text := strings.TrimSpace(line)
		if !strings.HasPrefix(text, ":") || strings.HasPrefix(text, "::") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(text, ":"))
		if name == "" || strings.EqualFold(name, "eof") || strings.ContainsAny(name, " %!&|<>\t") {
			continue
		}
		symbol := supplementarySymbol(file, "label", name, n+1)
		labels[strings.ToLower(name)] = append(labels[strings.ToLower(name)], symbol)
		result.facts.Declarations = append(result.facts.Declarations, symbol)
		result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: name, Kind: "label", Language: "batch", File: file.Path, Line: n + 1})
	}
	caller := supplementarySymbol(file, "script", file.Path, 1)
	result.facts.Declarations = append(result.facts.Declarations, caller)
	for n, line := range lines {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "@"))
		lower := strings.ToLower(text)
		if strings.HasPrefix(lower, "rem ") || lower == "rem" || strings.HasPrefix(lower, "::") || text == "" {
			continue
		}
		if strings.HasPrefix(text, ":") {
			name := strings.TrimSpace(strings.TrimPrefix(text, ":"))
			if records := labels[strings.ToLower(name)]; len(records) == 1 {
				caller = records[0]
			}
			continue
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		command := strings.ToLower(fields[0])
		if command != "call" && command != "goto" {
			continue
		}
		if len(fields) < 2 {
			continue
		}
		target := fields[1]
		rest := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
		if strings.HasPrefix(rest, "\"") {
			end := strings.Index(rest[1:], "\"")
			if end < 0 {
				continue
			}
			target = rest[1 : end+1]
		}
		if strings.ContainsAny(target, "%!&|<>") {
			result.facts.References = append(result.facts.References, supplementaryReference(file, "dynamic batch target", "calls_script", n+1))
			continue
		}
		if command == "goto" && strings.EqualFold(strings.TrimPrefix(target, ":"), "eof") {
			continue
		}
		if command == "goto" || strings.HasPrefix(target, ":") {
			name := strings.TrimPrefix(target, ":")
			records := labels[strings.ToLower(name)]
			ref := supplementaryReference(file, name, "branches_to", n+1)
			ref.FromSymbolID = caller.ID
			if len(records) == 1 {
				to := records[0]
				ref.To = file.Path
				ref.ToSymbolID = to.ID
				ref.TargetQualifiedName = to.QualifiedName
				ref.Resolution = SymbolResolutionExact
				ref.NonPromotable = false
				ref.preventExact = false
				ref.Internal = true
				ref.Reason = "unique literal batch label; command execution is not asserted"
				if command == "call" {
					ref.Type = "calls_method_owner"
					result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("batch-call", caller.ID, to.ID, fmt.Sprint(n+1)), From: MethodRefRecord{Method: caller.Name, File: file.Path, Line: caller.Line}, To: MethodRefRecord{Method: to.Name, File: file.Path, Line: to.Line}, Type: "calls", Line: n + 1, SourceFile: file.Path, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: caller.ID, ToSymbolID: to.ID, TargetQualifiedName: to.QualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
				}
			} else if len(records) > 1 {
				ref.Resolution = SymbolResolutionAmbiguous
			}
			result.facts.References = append(result.facts.References, ref)
			continue
		}
		result.facts.References = append(result.facts.References, supplementaryReference(file, target, "calls_script", n+1))
	}
}
