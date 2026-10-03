package scan

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Semantic reports are explicit external evidence, never a request to run a tool.
type languageSemanticReport struct {
	SchemaVersion int                   `json:"schema_version"`
	Language      string                `json:"language"`
	Producer      string                `json:"producer"`
	Inputs        map[string]string     `json:"inputs"`
	CoveredFiles  []string              `json:"covered_files"`
	Declarations  []semanticDeclaration `json:"declarations"`
	References    []semanticReference   `json:"references"`
}
type semanticDeclaration struct {
	USR    string `json:"usr"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Owner  string `json:"owner"`
	Module string `json:"module"`
}
type semanticReference struct {
	USR    string `json:"usr"`
	Caller string `json:"caller"`
	Name   string `json:"name"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Kind   string `json:"kind"`
}

// SemanticInputFile identifies source/configuration inputs required by a snapshot.
func SemanticInputFile(file FileRecord, language string) bool {
	if file.Language == language || language == "csharp" && strings.EqualFold(path.Ext(file.Path), ".cs") || language == "swift" && strings.EqualFold(path.Ext(file.Path), ".swift") {
		return true
	}
	if strings.HasSuffix(file.Path, ".goregraph-language-input.json") || path.Base(file.Path) == "goregraph.yml" {
		return true
	}
	ext := strings.ToLower(path.Ext(file.Path))
	if language == "csharp" {
		switch ext {
		case ".csproj", ".asmdef", ".asmref", ".sln", ".slnx", ".props", ".targets":
			return true
		}
		return strings.HasSuffix(file.Path, ".asmdef.meta") || strings.HasSuffix(file.Path, ".asmref.meta") || path.Base(file.Path) == "global.json" || path.Base(file.Path) == "NuGet.Config" || file.Path == "Packages/manifest.json" || file.Path == "Packages/packages-lock.json" || file.Path == "ProjectSettings/ProjectVersion.txt"
	}
	switch ext {
	case ".pbxproj", ".xcconfig", ".xcscheme":
		return true
	}
	return path.Base(file.Path) == "Package.resolved" || path.Base(file.Path) == "contents.xcworkspacedata"
}

func validateSemanticReport(report languageSemanticReport, files []FileRecord, bodies map[string]string) error {
	if report.SchemaVersion != 1 || (report.Language != "csharp" && report.Language != "swift") || report.Producer == "" {
		return fmt.Errorf("unsupported semantic report metadata")
	}
	if len(report.Declarations) > 200000 || len(report.References) > 500000 {
		return fmt.Errorf("semantic report exceeds supported limits")
	}
	inventory := map[string]FileRecord{}
	for _, file := range files {
		inventory[file.Path] = file
	}
	for name, hash := range report.Inputs {
		file, ok := inventory[name]
		if !ok || path.Clean(name) != name || strings.Contains(name, "\\") || path.IsAbs(name) || strings.HasPrefix(name, "../") || hash == "" || file.Hash != hash {
			return fmt.Errorf("semantic input missing or changed: %s", name)
		}
	}
	for _, file := range files {
		if SemanticInputFile(file, report.Language) && report.Inputs[file.Path] != file.Hash {
			return fmt.Errorf("semantic inventory omitted input: %s", file.Path)
		}
	}
	covered := map[string]bool{}
	for _, name := range report.CoveredFiles {
		file, ok := inventory[name]
		if !ok || covered[name] || file.Language != report.Language || report.Inputs[name] == "" {
			return fmt.Errorf("invalid semantic source coverage")
		}
		covered[name] = true
	}
	sourceLines := map[string][]string{}
	location := func(name string, line, column int, token string) bool {
		if !covered[name] || line < 1 || column < 1 || token == "" {
			return false
		}
		lines, found := sourceLines[name]
		if !found {
			lines = strings.Split(bodies[name], "\n")
			sourceLines[name] = lines
		}
		if line > len(lines) || column > len(lines[line-1])+1 || !utf8.ValidString(token) || !utf8.ValidString(lines[line-1][:column-1]) || !strings.HasPrefix(lines[line-1][column-1:], token) {
			return false
		}
		first, _ := utf8.DecodeRuneInString(token)
		previous, _ := utf8.DecodeLastRuneInString(lines[line-1][:column-1])
		if (unicode.IsLetter(first) || first == '_') && (unicode.IsLetter(previous) || unicode.IsDigit(previous) || unicode.IsMark(previous) || previous == '_') {
			return false
		}
		last, _ := utf8.DecodeLastRuneInString(token)
		if unicode.IsLetter(last) || unicode.IsDigit(last) || last == '_' {
			rest := lines[line-1][column-1+len(token):]
			next, _ := utf8.DecodeRuneInString(rest)
			if unicode.IsLetter(next) || unicode.IsDigit(next) || unicode.IsMark(next) || next == '_' {
				return false
			}
		}
		return true
	}
	seen := map[string]bool{}
	for _, declaration := range report.Declarations {
		key := fmt.Sprintf("%s:%d:%d:%s", declaration.File, declaration.Line, declaration.Column, declaration.USR)
		if declaration.USR == "" || seen[key] || !location(declaration.File, declaration.Line, declaration.Column, declaration.Name) {
			return fmt.Errorf("invalid semantic declaration location or identity")
		}
		switch declaration.Kind {
		case "class", "struct", "interface", "record", "enum", "actor", "protocol", "method", "function", "constructor", "property", "field", "enum_case", "typealias":
		default:
			return fmt.Errorf("unsupported semantic declaration kind")
		}
		seen[key] = true
	}
	for _, reference := range report.References {
		if reference.USR == "" || !location(reference.File, reference.Line, reference.Column, reference.Name) || reference.Kind != "uses_symbol" && reference.Kind != "calls_method_owner" {
			return fmt.Errorf("invalid semantic reference location or kind")
		}
	}
	return nil
}

func applySemanticReport(report languageSemanticReport, facts *ProjectSymbolFacts, graph *CallGraphRecord) {
	byUSR := map[string][]RichSymbolRecord{}
	identity := func(file string, line int, name, kind string) string {
		return fmt.Sprintf("%s:%d:%s:%s", file, line, name, kind)
	}
	counts := map[string]int{}
	for _, declaration := range report.Declarations {
		counts[identity(declaration.File, declaration.Line, declaration.Name, declaration.Kind)]++
	}
	byLocation := map[string][]RichSymbolRecord{}
	positions := map[string]int{}
	for at, existing := range facts.Declarations {
		key := identity(existing.File, existing.Line, existing.Name, existing.Kind)
		byLocation[key] = append(byLocation[key], existing)
		positions[existing.ID] = at
	}
	for _, declaration := range report.Declarations {
		key := identity(declaration.File, declaration.Line, declaration.Name, declaration.Kind)
		matches := byLocation[key]
		symbol := RichSymbolRecord{ID: stableID("semantic-symbol", report.Language, declaration.File, declaration.USR, fmt.Sprint(declaration.Line, ":", declaration.Column)), Name: declaration.Name, Kind: declaration.Kind, Language: report.Language, File: declaration.File, Line: declaration.Line, Owner: declaration.Owner, Module: declaration.Module, QualifiedName: declaration.USR, SourceLocation: sourceLocation(declaration.Line), Analyzer: report.Language + "-semantic", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: []string{"explicit compiler configuration only; external SDK dependencies and runtime dispatch are not verified by this snapshot"}}
		if len(matches) == 1 && counts[key] == 1 {
			limitations := symbol.Limitations
			symbol = matches[0]
			symbol.Analyzer, symbol.Limitations = report.Language+"-semantic", limitations
			symbol.Confidence, symbol.Coverage = ConfidenceExact, CoveragePartial
			facts.Declarations[positions[symbol.ID]] = symbol
		} else {
			facts.Declarations = append(facts.Declarations, symbol)
		}
		byUSR[declaration.USR] = append(byUSR[declaration.USR], symbol)
	}
	covered := map[string]bool{}
	for _, file := range report.CoveredFiles {
		covered[file] = true
	}
	kept := facts.References[:0]
	for _, reference := range facts.References {
		if reference.Type != "calls_method_owner" || !covered[reference.From] {
			kept = append(kept, reference)
		}
	}
	facts.References = kept
	edges := graph.Edges[:0]
	for _, edge := range graph.Edges {
		if !covered[edge.SourceFile] {
			edges = append(edges, edge)
		}
	}
	graph.Edges = edges
	unique := func(usr string) (RichSymbolRecord, bool) {
		items := byUSR[usr]
		if len(items) != 1 {
			return RichSymbolRecord{}, false
		}
		return items[0], true
	}
	for _, item := range report.References {
		ref := RichRelationRecord{ID: stableID("semantic-ref", report.Language, item.File, fmt.Sprint(item.Line, ":", item.Column), item.USR, item.Kind), From: item.File, To: item.USR, Type: item.Kind, Language: report.Language, Analyzer: report.Language + "-semantic", Line: item.Line, SourceLocation: sourceLocation(item.Line), TargetQualifiedName: item.USR, Confidence: "INFERRED", Resolution: SymbolResolutionUnresolved, NonPromotable: true, Reason: "compiler reference has no unique declaration inside verified indexed sources"}
		caller, callerOK := unique(item.Caller)
		if callerOK {
			ref.FromSymbolID = caller.ID
		}
		target, targetOK := unique(item.USR)
		if targetOK {
			ref.To, ref.ToSymbolID, ref.TargetQualifiedName = target.File, target.ID, target.QualifiedName
			ref.Internal, ref.NonPromotable, ref.Resolution, ref.Confidence, ref.ConfidenceScore = true, false, SymbolResolutionExact, "EXTRACTED", 1
			ref.Reason = "compiler symbol identity in source/configuration-hash-verified report; static binding is not runtime execution proof"
		} else if len(byUSR[item.USR]) > 1 {
			ref.Resolution = SymbolResolutionAmbiguous
			for _, candidate := range byUSR[item.USR] {
				ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, candidate.ID)
			}
		}
		facts.References = append(facts.References, ref)
		if item.Kind == "calls_method_owner" && targetOK && callerOK {
			graph.Edges = append(graph.Edges, CallGraphEdgeRecord{ID: stableID("semantic-call", ref.ID), From: MethodRefRecord{Owner: caller.Owner, Method: caller.Name, File: caller.File, Line: caller.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, SourceFile: item.File, Line: item.Line, Type: "calls", Confidence: ref.Confidence, ConfidenceScore: 1, FromSymbolID: caller.ID, ToSymbolID: target.ID, TargetQualifiedName: ref.TargetQualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
		}
	}
}

func applyLanguageSemanticSources(index *Index, sources []assetExportSource, bodies map[string]string) {
	byLanguage := map[string][]assetExportSource{}
	for _, source := range sources {
		language := "swift"
		if strings.HasSuffix(source.file.Path, ".goregraph-csharp.json") {
			language = "csharp"
		}
		byLanguage[language] = append(byLanguage[language], source)
	}
	for _, language := range []string{"csharp", "swift"} {
		reports := byLanguage[language]
		for _, source := range reports {
			var report languageSemanticReport
			err := json.Unmarshal([]byte(source.body), &report)
			if err == nil {
				err = validateSemanticReport(report, index.Files, bodies)
			}
			if err == nil && (report.Language != language || len(reports) != 1) {
				err = fmt.Errorf("mismatched or competing semantic reports")
			}
			if err != nil {
				index.AnalysisIssues = append(index.AnalysisIssues, AnalysisIssue{File: source.file.Path, Reason: err.Error() + "; static source analysis retained"})
				continue
			}
			if language == "csharp" {
				applySemanticReport(report, &index.CSharp.facts, &index.CSharp.graph)
				index.CSharp.tests = semanticTestMaps(report, index.CSharp.tests, index.CSharp.code, index.CSharp.graph)
			} else {
				applySemanticReport(report, &index.Swift.facts, &index.Swift.graph)
				index.Swift.tests = semanticTestMaps(report, index.Swift.tests, index.Swift.code, index.Swift.graph)
			}
			dependencies := map[string]string{}
			for name, hash := range report.Inputs {
				dependencies[name] = hash
			}
			dependencies[source.file.Path] = source.file.Hash
			index.SemanticDependencies = append(index.SemanticDependencies, SemanticDependencyRecord{Language: language, Inputs: dependencies})
		}
	}
}

func semanticTestMaps(report languageSemanticReport, existing []TestMapRecord, code CodeIntelligenceRecord, graph CallGraphRecord) []TestMapRecord {
	covered := map[string]bool{}
	for _, file := range report.CoveredFiles {
		covered[file] = true
	}
	result := make([]TestMapRecord, 0, len(existing))
	for _, test := range existing {
		if !covered[test.TestFile] {
			result = append(result, test)
		}
	}
	identity := func(file string, line int, name string) string { return fmt.Sprintf("%s:%d:%s", file, line, name) }
	functions := map[string]CodeFunctionRecord{}
	for _, function := range code.Functions {
		if function.Kind == "test" {
			functions[identity(function.File, function.Line, function.Name)] = function
		}
	}
	for _, edge := range graph.Edges {
		if !covered[edge.SourceFile] {
			continue
		}
		if function, found := functions[identity(edge.From.File, edge.From.Line, edge.From.Method)]; found {
			result = append(result, TestMapRecord{TestFile: function.File, TestClass: function.Owner, TestMethod: function.Name, TargetFile: edge.To.File, TargetClass: edge.To.Owner, TargetMethod: edge.To.Method, Type: "compiler-call", Line: edge.Line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "test declaration has a verified compiler call binding; no test execution is inferred"})
		}
	}
	return result
}
