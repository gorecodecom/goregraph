package scan

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type gdscriptMethod struct {
	symbol     RichSymbolRecord
	start, end int
	static     bool
	parameters map[string]bool
}

type gdscriptSource struct {
	file      FileRecord
	tokens    []godotToken
	topLevel  map[int]int
	nested    map[int]bool
	class     RichSymbolRecord
	methods   []gdscriptMethod
	aliases   map[string]string
	fields    map[string]bool
	instances map[string]string
	extends   *godotToken
}

func parseGDScript(source supplementarySource) gdscriptSource {
	s := gdscriptSource{file: source.file, tokens: godotTokens(source.body, false), aliases: map[string]string{}, fields: map[string]bool{}}
	s.topLevel = gdscriptTopLevelStatements(s.tokens)
	s.nested = map[int]bool{}
	for at, end := range s.topLevel {
		if s.tokens[at].kind != "identifier" || s.tokens[at].text != "class" {
			continue
		}
		for j := at; j < end; j++ {
			s.nested[j] = true
		}
	}
	name := strings.TrimSuffix(path.Base(s.file.Path), path.Ext(s.file.Path))
	classLine := 1
	for i, token := range s.tokens {
		if s.topLevel[i] != 0 && token.kind == "identifier" && token.text == "class_name" && i+1 < len(s.tokens) && s.tokens[i+1].kind == "identifier" {
			name, classLine = s.tokens[i+1].text, token.line
		}
	}
	s.class = supplementarySymbol(s.file, "class", name, classLine)
	for i := 0; i < len(s.tokens); i++ {
		t := s.tokens[i]
		inlineExtends := t.text == "extends" && i >= 2 && s.tokens[i-2].text == "class_name" && s.topLevel[i-2] != 0 && s.tokens[i-2].line == t.line && s.tokens[i-1].kind == "identifier"
		if (s.topLevel[i] == 0 && !inlineExtends) || t.kind != "identifier" {
			continue
		}
		if t.text == "extends" && i+1 < len(s.tokens) {
			value := s.tokens[i+1]
			if (value.kind == "identifier" || value.kind == "string") && i+2 < len(s.tokens) && s.tokens[i+2].text == "." {
				value.kind = "qualified"
				for j := i + 2; j < len(s.tokens) && s.tokens[j].line == t.line && (s.tokens[j].kind == "identifier" || s.tokens[j].text == "."); j++ {
					value.text += s.tokens[j].text
				}
			}
			s.extends = &value
		}
		funcAt, static := i, false
		if t.text == "static" && i+1 < len(s.tokens) && s.tokens[i+1].text == "func" {
			funcAt++
			static = true
		}
		if s.tokens[funcAt].text != "func" || funcAt+2 >= len(s.tokens) || s.tokens[funcAt+1].kind != "identifier" || s.tokens[funcAt+2].text != "(" {
			continue
		}
		depth, endHeader := 0, -1
		params := map[string]bool{}
		for j := funcAt + 2; j < len(s.tokens); j++ {
			token := s.tokens[j]
			if depth == 0 && token.line > t.line && token.column == 0 && token.kind == "identifier" && gdscriptDeclarationKeyword(token.text) {
				break
			}
			if token.kind == "punctuation" && (token.text == "(" || token.text == "[" || token.text == "{") {
				depth++
			}
			if token.kind == "punctuation" && (token.text == ")" || token.text == "]" || token.text == "}") {
				depth--
			}
			if depth == 1 && token.kind == "identifier" && (s.tokens[j-1].text == "(" || s.tokens[j-1].text == ",") {
				params[token.text] = true
			}
			if token.kind == "punctuation" && token.text == ":" && depth == 0 {
				endHeader = j
				break
			}
		}
		if endHeader < 0 {
			continue
		}
		end := s.topLevel[i]
		kind := "method"
		if gdscriptTestFile(s.file.Path) {
			kind = "test"
		}
		symbol := supplementarySymbol(s.file, kind, s.tokens[funcAt+1].text, t.line)
		symbol.Owner = s.class.QualifiedName
		s.methods = append(s.methods, gdscriptMethod{symbol, endHeader + 1, end, static, params})
		i = endHeader
	}
	return s
}

func gdscriptTestFile(file string) bool {
	lower := strings.ToLower(file)
	return strings.HasPrefix(lower, "tests/") || strings.Contains(lower, "/tests/") || strings.HasSuffix(lower, "_test.gd")
}

func analyzeGodotSources(sources []supplementarySource, files []FileRecord) supplementaryAnalysis {
	result := supplementaryAnalysis{}
	hasGodot := false
	for _, source := range sources {
		if source.file.Language == "gdscript" || source.file.Language == "godot" {
			hasGodot = true
			break
		}
	}
	if !hasGodot {
		return result
	}
	scripts := map[string]gdscriptSource{}
	globals := map[string][]RichSymbolRecord{}
	known := map[string]bool{}
	for _, file := range files {
		known[file.Path] = true
	}
	for _, source := range sources {
		if source.file.Language != "gdscript" {
			continue
		}
		script := parseGDScript(source)
		scripts[script.file.Path] = script
		for at, token := range script.tokens {
			if script.topLevel[at] != 0 && token.kind == "identifier" && token.text == "class_name" {
				globals[script.class.Name] = append(globals[script.class.Name], script.class)
				break
			}
		}
	}
	// Sort traversal so aliases, duplicate global classes and output are deterministic.
	var paths []string
	for file := range scripts {
		paths = append(paths, file)
	}
	sort.Strings(paths)
	for _, file := range paths {
		script := scripts[file]
		extractGDScriptDeclarations(&result, &script)
		extractGDScriptResources(&result, &script, scripts, known)
		scripts[file] = script
	}
	for _, file := range paths {
		script := scripts[file]
		if script.extends != nil {
			value := *script.extends
			var targets []RichSymbolRecord
			if value.kind == "identifier" {
				targets = globals[value.text]
			}
			if value.kind == "string" {
				target := godotResourcePath(file, value.text)
				targets = nil
				if parent, ok := scripts[target]; ok {
					targets = []RichSymbolRecord{parent.class}
				}
			}
			ref := godotSymbolReference(script.file, script.class, value.text, "extends", value.line, targets)
			result.facts.References = append(result.facts.References, ref)
		}
		script.instances = gdscriptFieldInstances(script)
		extractGDScriptCalls(&result, script, scripts)
	}
	for _, source := range sources {
		if source.file.Language == "godot" {
			analyzeGodotResource(&result, source, scripts, known)
		}
	}
	return result
}

func extractGDScriptDeclarations(result *supplementaryAnalysis, s *gdscriptSource) {
	result.facts.Declarations = append(result.facts.Declarations, s.class)
	for _, method := range s.methods {
		result.facts.Declarations = append(result.facts.Declarations, method.symbol)
		endLine := method.symbol.Line
		if method.end > method.start {
			endLine = s.tokens[method.end-1].line
		}
		result.code.Functions = append(result.code.Functions, CodeFunctionRecord{Name: method.symbol.Name, Owner: method.symbol.Owner, Kind: method.symbol.Kind, Language: "gdscript", File: s.file.Path, Line: method.symbol.Line, EndLine: endLine})
	}
	for i, t := range s.tokens {
		if t.kind != "identifier" || i+1 >= len(s.tokens) || s.tokens[i+1].kind != "identifier" {
			continue
		}
		kind := map[string]string{"var": "field", "const": "constant", "signal": "signal", "enum": "enum"}[t.text]
		if kind == "" {
			continue
		}
		if s.topLevel[i] == 0 {
			continue
		}
		name := s.tokens[i+1].text
		symbol := supplementarySymbol(s.file, kind, name, t.line)
		symbol.Owner = s.class.QualifiedName
		result.facts.Declarations = append(result.facts.Declarations, symbol)
		s.fields[name] = true
	}
}

func extractGDScriptResources(result *supplementaryAnalysis, s *gdscriptSource, scripts map[string]gdscriptSource, known map[string]bool) {
	aliasCounts := map[string]int{}
	methodBindings := map[string][]gdscriptBinding{}
	closures := gdscriptClosures(s.tokens, 0, len(s.tokens), s.topLevel)
	for _, method := range s.methods {
		methodBindings[method.symbol.ID] = gdscriptMethodBindings(*s, method)
	}
	for i, t := range s.tokens {
		if s.topLevel[i] != 0 && t.kind == "identifier" && t.text == "const" && i+1 < len(s.tokens) {
			aliasCounts[s.tokens[i+1].text]++
		}
	}
	for i, t := range s.tokens {
		if s.nested[i] {
			continue
		}
		if t.kind != "identifier" || (t.text != "preload" && t.text != "load") || i+2 >= len(s.tokens) || s.tokens[i+1].text != "(" {
			continue
		}
		if gdscriptNodePathName(s.tokens, i) || i > 0 && (s.tokens[i-1].text == "." || s.tokens[i-1].text == "func") {
			continue
		}
		if s.fields[t.text] || len(gdscriptLookup(*s, t.text, false)) > 0 {
			continue
		}
		shadowed := closures[i]
		for _, method := range s.methods {
			if i < method.start || i >= method.end {
				continue
			}
			shadowed = shadowed || gdscriptLocalBinding(methodBindings[method.symbol.ID], t.text, i) != nil
		}
		if shadowed {
			continue
		}
		value := s.tokens[i+2]
		literal := i+3 < len(s.tokens) && value.kind == "string" && s.tokens[i+3].text == ")"
		uri := "<computed resource>"
		if literal {
			uri = value.text
		}
		ref := supplementaryReference(s.file, uri, "loads_resource", t.line)
		ref.FromSymbolID = s.class.ID
		for _, method := range s.methods {
			if i >= method.start && i < method.end {
				ref.FromSymbolID = method.symbol.ID
				break
			}
		}
		if literal {
			target := godotResourcePath(s.file.Path, uri)
			if t.text == "load" {
				target = godotResourcePath("", uri)
			}
			if known[target] {
				ref.To, ref.Internal = target, true
				ref.Reason = "literal Godot resource path points to an indexed file; runtime loading is not asserted"
				if script, ok := scripts[target]; ok {
					bindGodotReference(&ref, []RichSymbolRecord{script.class})
				}
			}
			// Only immutable top-level aliases provide a proven script class receiver.
			if t.text == "preload" && i >= 3 && s.tokens[i-3].text == "const" && s.topLevel[i-3] != 0 && s.tokens[i-2].kind == "identifier" && s.tokens[i-1].text == "=" && gdscriptExpressionEnd(s.tokens, i, len(s.tokens)) == i+4 {
				if _, ok := scripts[target]; ok {
					alias := s.tokens[i-2].text
					if aliasCounts[alias] == 1 {
						s.aliases[alias] = target
					} else {
						delete(s.aliases, alias)
					}
				}
			}
		}
		result.facts.References = append(result.facts.References, ref)
	}
}

func godotSymbolReference(file FileRecord, from RichSymbolRecord, name, kind string, line int, targets []RichSymbolRecord) RichRelationRecord {
	ref := supplementaryReference(file, name, kind, line)
	ref.FromSymbolID = from.ID
	bindGodotReference(&ref, targets)
	return ref
}

func bindGodotReference(ref *RichRelationRecord, targets []RichSymbolRecord) {
	if len(targets) == 1 {
		target := targets[0]
		ref.To, ref.ToSymbolID, ref.TargetQualifiedName = target.File, target.ID, target.QualifiedName
		ref.Internal, ref.NonPromotable, ref.preventExact = true, false, false
		ref.Resolution = SymbolResolutionExact
		ref.Reason = "unique explicit Godot source declaration; runtime behavior is not asserted"
	} else if len(targets) > 1 {
		ref.Resolution = SymbolResolutionAmbiguous
		for _, target := range targets {
			ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, target.ID)
		}
		sort.Strings(ref.CandidateSymbolIDs)
	}
}

func gdscriptLookup(s gdscriptSource, name string, staticOnly bool) []RichSymbolRecord {
	var targets []RichSymbolRecord
	for _, method := range s.methods {
		if method.symbol.Name == name && (!staticOnly || method.static) {
			targets = append(targets, method.symbol)
		}
	}
	return targets
}

func extractGDScriptCalls(result *supplementaryAnalysis, s gdscriptSource, scripts map[string]gdscriptSource) {
	for _, method := range s.methods {
		bindings := gdscriptMethodBindings(s, method)
		closures := gdscriptClosures(s.tokens, method.start, method.end, s.topLevel)
		for i := method.start; i+1 < method.end; i++ {
			t := s.tokens[i]
			if t.kind != "identifier" || s.tokens[i+1].text != "(" || gdscriptCallKeyword(t.text) {
				continue
			}
			receiver := ""
			hasReceiver := i >= 2 && s.tokens[i-1].text == "."
			plainReceiver := !hasReceiver || gdscriptPlainReceiver(s.tokens, i)
			if !hasReceiver && gdscriptNodePathName(s.tokens, i) {
				plainReceiver = false
			}
			if hasReceiver {
				receiver = s.tokens[i-2].text
			}
			var targets []RichSymbolRecord
			if !closures[i] && plainReceiver {
				local := gdscriptLocalBinding(bindings, receiver, i)
				switch {
				case !hasReceiver && gdscriptLocalBinding(bindings, t.text, i) == nil && !s.fields[t.text]:
					targets = gdscriptLookup(s, t.text, method.static)
				case receiver == "self":
					targets = gdscriptLookup(s, t.text, method.static)
				case local != nil && local.targetFile != "":
					targets = gdscriptLookup(scripts[local.targetFile], t.text, false)
				case hasReceiver && local == nil && s.aliases[receiver] != "":
					targets = gdscriptLookup(scripts[s.aliases[receiver]], t.text, true)
				case hasReceiver && local == nil && s.instances[receiver] != "":
					targets = gdscriptLookup(scripts[s.instances[receiver]], t.text, false)
				}
			}
			if !closures[i] && plainReceiver && hasReceiver && t.text == "new" && s.aliases[receiver] != "" && gdscriptLocalBinding(bindings, receiver, i) == nil {
				target := scripts[s.aliases[receiver]].class
				result.facts.References = append(result.facts.References, godotSymbolReference(s.file, method.symbol, receiver, "instantiates", t.line, []RichSymbolRecord{target}))
				continue
			}
			addNativeCall(result, s.file, method.symbol, t.text, t.line, i, targets, "unique lexical or literal-preload GDScript method declaration; runtime overrides remain unproven")
			if len(targets) == 0 {
				result.graph.UnresolvedCalls = append(result.graph.UnresolvedCalls, GoCallDiagnosticRecord{File: s.file.Path, Line: t.line, Caller: method.symbol.Name, Method: t.text, Reason: "no unique lexical GDScript target; engine APIs, shadowing and dynamic dispatch are not guessed"})
			}
			if len(targets) > 1 {
				ref := &result.facts.References[len(result.facts.References)-1]
				for _, target := range targets {
					ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, target.ID)
				}
			}
			if !closures[i] {
				extractGDScriptSignal(result, s, method, i, bindings)
			}
		}
	}
}

func gdscriptAssignment(tokens []godotToken, at, end int) bool {
	if at > 0 && (tokens[at-1].text == "=" || tokens[at-1].text == "!" || tokens[at-1].text == "<" || tokens[at-1].text == ">") {
		return false
	}
	j := at + 1
	if j < end && (tokens[j].text == ":" || strings.Contains("+-*/%&|^", tokens[j].text)) {
		if tokens[j].text == "*" && j+1 < end && tokens[j+1].text == "*" {
			j++
		}
		j++
	} else if j+1 < end && ((tokens[j].text == "<" && tokens[j+1].text == "<") || (tokens[j].text == ">" && tokens[j+1].text == ">")) {
		j += 2
	}
	return j < end && tokens[j].text == "=" && (j+1 == end || tokens[j+1].text != "=")
}

func extractGDScriptSignal(result *supplementaryAnalysis, s gdscriptSource, method gdscriptMethod, at int, bindings []gdscriptBinding) {
	tokens := s.tokens
	t := tokens[at]
	name := ""
	if t.text == "emit_signal" && at+3 < method.end && tokens[at+2].kind == "string" && (tokens[at+3].text == ")" || tokens[at+3].text == ",") {
		if gdscriptNodePathName(tokens, at) {
			return
		}
		if at >= 2 && tokens[at-1].text == "." && (!gdscriptPlainReceiver(tokens, at) || tokens[at-2].text != "self") {
			return
		}
		explicitSelf := at >= 2 && tokens[at-1].text == "." && tokens[at-2].text == "self"
		if len(gdscriptLookup(s, "emit_signal", false)) > 0 || s.fields["emit_signal"] || !explicitSelf && gdscriptLocalBinding(bindings, "emit_signal", at) != nil {
			return
		}
		name = tokens[at+2].text
	}
	if t.text == "emit" && at >= 2 && tokens[at-1].text == "." {
		receiver, explicitSelf := gdscriptOwnSignalReceiver(tokens, at)
		if receiver == "" {
			return
		}
		if !explicitSelf && gdscriptLocalBinding(bindings, receiver, at) != nil {
			return
		}
		name = receiver
	}
	if name != "" {
		var targets []RichSymbolRecord
		for _, symbol := range result.facts.Declarations {
			if symbol.File == s.file.Path && symbol.Kind == "signal" && symbol.Name == name {
				targets = append(targets, symbol)
			}
		}
		ref := godotSymbolReference(s.file, method.symbol, name, "emits_signal", t.line, targets)
		ref.ID = stableID("godot-signal", method.symbol.ID, fmt.Sprint(at))
		result.facts.References = append(result.facts.References, ref)
	}
	if t.text != "connect" || at < 2 || tokens[at-1].text != "." || at+3 >= method.end {
		return
	}
	callback := tokens[at+2]
	var targets []RichSymbolRecord
	if callback.kind == "identifier" && tokens[at+3].text == ")" && !s.fields[callback.text] && gdscriptLocalBinding(bindings, callback.text, at) == nil {
		targets = gdscriptLookup(s, callback.text, false)
	}
	if callback.kind == "identifier" && callback.text == "Callable" && at+8 < method.end && tokens[at+3].text == "(" && tokens[at+4].kind == "identifier" && tokens[at+4].text == "self" && tokens[at+5].text == "," && tokens[at+6].kind == "string" && tokens[at+7].text == ")" && tokens[at+8].text == ")" && !s.fields["Callable"] && gdscriptLocalBinding(bindings, "Callable", at) == nil && len(gdscriptLookup(s, "Callable", false)) == 0 {
		callback = tokens[at+6]
		targets = gdscriptLookup(s, callback.text, false)
	}
	ref := godotSymbolReference(s.file, method.symbol, callback.text, "connects_callback", t.line, targets)
	ref.Reason = "literal local signal callback reference; signal type and runtime execution are not asserted"
	ref.ID = stableID("godot-callback", method.symbol.ID, fmt.Sprint(at))
	result.facts.References = append(result.facts.References, ref)
}

func gdscriptFieldInstances(s gdscriptSource) map[string]string {
	instances := map[string]string{}
	writes := map[string]int{}
	locals := map[string][]gdscriptBinding{}
	for _, method := range s.methods {
		locals[method.symbol.ID] = gdscriptMethodBindings(s, method)
	}
	for i, token := range s.tokens {
		if token.kind != "identifier" || s.nested[i] {
			continue
		}
		if gdscriptAssignment(s.tokens, i, len(s.tokens)) {
			localWrite := false
			if i == 0 || s.tokens[i-1].text != "." {
				for _, method := range s.methods {
					if i >= method.start && i < method.end && gdscriptLocalBinding(locals[method.symbol.ID], token.text, i) != nil {
						localWrite = true
						break
					}
				}
			}
			if !localWrite {
				writes[token.text]++
			}
		}
		if token.text != "var" || s.topLevel[i] == 0 {
			continue
		}
		start, end, ok := gdscriptVariableInitializer(s.tokens, i, len(s.tokens))
		if !ok {
			continue
		}
		name := s.tokens[i+1].text
		if !gdscriptAssignment(s.tokens, i+1, len(s.tokens)) {
			writes[name]++
		}
		if alias := gdscriptConstructorAlias(s.tokens, start, end); alias != "" && s.aliases[alias] != "" {
			instances[name] = s.aliases[alias]
		}
	}
	for name := range instances {
		if writes[name] != 1 {
			delete(instances, name)
		}
	}
	return instances
}

func gdscriptClosures(tokens []godotToken, start, end int, topLevel map[int]int) map[int]bool {
	skipped := map[int]bool{}
	for i := start; i < end; i++ {
		if tokens[i].kind != "identifier" || tokens[i].text != "func" {
			continue
		}
		if topLevel[i] != 0 {
			continue
		}
		lineStart := i
		for lineStart > start && tokens[lineStart-1].line == tokens[i].line {
			lineStart--
		}
		indent := tokens[lineStart].column
		depth, colon := 0, -1
		for j := i + 1; j < end; j++ {
			if tokens[j].kind == "punctuation" && (tokens[j].text == "(" || tokens[j].text == "[" || tokens[j].text == "{") {
				depth++
			}
			if tokens[j].kind == "punctuation" && (tokens[j].text == ")" || tokens[j].text == "]" || tokens[j].text == "}") {
				depth--
			}
			if tokens[j].kind == "punctuation" && tokens[j].text == ":" && depth == 0 {
				colon = j
				break
			}
		}
		if colon < 0 {
			for j := i; j < end; j++ {
				skipped[j] = true
			}
			break
		}
		j := colon + 1
		for j < end && (tokens[j].line == tokens[colon].line || tokens[j].column > indent) {
			j++
		}
		for k := i; k < j; k++ {
			skipped[k] = true
		}
		i = j - 1
	}
	return skipped
}

func gdscriptCallKeyword(name string) bool {
	switch name {
	case "if", "elif", "while", "for", "match", "return", "func", "signal", "class", "extends", "class_name", "await", "not", "and", "or", "in", "is", "as":
		return true
	default:
		return false
	}
}

func gdscriptDeclarationKeyword(name string) bool {
	switch name {
	case "func", "static", "class", "class_name", "extends", "var", "const", "signal", "enum":
		return true
	default:
		return false
	}
}
