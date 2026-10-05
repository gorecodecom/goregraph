package scan

import (
	"fmt"
	"path"
	"strings"
)

func cloneNativeTypes(env map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range env {
		result[k] = v
	}
	return result
}

func nativeMemberSymbols(members []dartMember) []RichSymbolRecord {
	var symbols []RichSymbolRecord
	for _, m := range members {
		symbols = append(symbols, m.symbol)
	}
	return symbols
}

func addNativeCall(result *supplementaryAnalysis, file FileRecord, from RichSymbolRecord, name string, line, at int, targets []RichSymbolRecord, reason string) {
	ref := supplementaryReference(file, name, "calls_method_owner", line)
	ref.ID = stableID("native-call-reference", file.Path, from.ID, name, fmt.Sprint(at))
	ref.FromSymbolID = from.ID
	if len(targets) == 1 {
		target := targets[0]
		ref.To = target.File
		ref.ToSymbolID = target.ID
		ref.TargetQualifiedName = target.QualifiedName
		ref.Internal = true
		ref.Resolution = SymbolResolutionExact
		ref.NonPromotable = false
		ref.preventExact = false
		ref.Reason = reason
		result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("native-call", from.ID, target.ID, fmt.Sprint(at)), From: MethodRefRecord{Owner: from.Owner, Method: from.Name, File: from.File, Line: from.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, Type: "calls", Line: line, SourceFile: file.Path, Confidence: "EXTRACTED", ConfidenceScore: 1, FromSymbolID: from.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact, Reason: reason})
		if from.Kind == "test" {
			result.tests = append(result.tests, TestMapRecord{TestFile: file.Path, TestClass: from.Owner, TestMethod: from.Name, TargetFile: target.File, TargetClass: target.Owner, TargetMethod: target.Name, Type: "unit", Line: line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "static native test call; test execution and runtime coverage are not asserted"})
		}

	} else if len(targets) > 1 {
		ref.Resolution = SymbolResolutionAmbiguous
	}
	result.facts.References = append(result.facts.References, ref)
}

func nativeRelativeTarget(from, uri string) string {
	if uri == "" || strings.ContainsAny(uri, "<>:$\\\x00\r\n") || path.IsAbs(uri) {
		return ""
	}
	target := path.Clean(path.Join(path.Dir(from), uri))
	if target == ".." || strings.HasPrefix(target, "../") {
		return ""
	}
	return target
}

func cVisibleHeaders(source cFamilySource, files map[string]cFamilySource) map[string]bool {
	seen := map[string]bool{source.file.Path: true}
	queue := []cFamilySource{source}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, include := range current.includes {
			target := nativeRelativeTarget(current.file.Path, include)
			header, ok := files[target]
			if !ok || !header.valid || seen[target] {
				continue
			}
			seen[target] = true
			queue = append(queue, header)
		}
	}
	return seen
}

func cSameSignature(a, b dartMember) bool {
	if a.symbol.Name != b.symbol.Name || a.symbol.Owner != b.symbol.Owner || len(a.parameters) != len(b.parameters) {
		return false
	}
	for i, p := range a.parameters {
		if p.typeName != b.parameters[i].typeName {
			return false
		}
	}
	return true
}

func cMemberVisible(source cFamilySource, candidate dartMember, visible map[string]bool, files map[string]cFamilySource) bool {
	if candidate.symbol.File == source.file.Path {
		return true
	}
	// Static free functions have translation-unit linkage; class static methods do not.
	if candidate.static && !cClassOwner(candidate.symbol.Owner, files) {
		return false
	}
	if visible[candidate.symbol.File] {
		return true
	}
	for file := range visible {
		for _, prototype := range files[file].members {
			if cSameSignature(prototype, candidate) {
				return true
			}
		}
	}
	return false
}

func cCallOwner(tokens []dartToken, nameAt int, fallback string, env map[string]string) (string, bool, bool) {
	if nameAt >= 2 && (tokens[nameAt-1].text == "." || tokens[nameAt-1].text == ">") {
		receiver := tokens[nameAt-2].text
		if tokens[nameAt-1].text == ">" && nameAt >= 3 && tokens[nameAt-2].text == "-" {
			receiver = tokens[nameAt-3].text
		}
		typ := env[receiver]
		return typ, true, typ == ""
	}
	if nameAt >= 2 && tokens[nameAt-1].text == ":" && tokens[nameAt-2].text == ":" {
		begin := nameAt - 2
		for begin > 0 && tokens[begin-1].kind == "identifier" {
			begin--
			if begin >= 2 && tokens[begin-1].text == ":" && tokens[begin-2].text == ":" {
				begin -= 2
			} else {
				break
			}
		}
		return strings.TrimPrefix(dartJoined(tokens[begin:nameAt-2]), "::"), true, false
	}
	return fallback, false, false
}

func cLocalBinding(s cFamilySource, at, end int, env map[string]string) {
	t := s.tokens
	if at+3 < end && t[at].text == "(" && t[at+1].text == "*" && t[at+3].text == ")" {
		env[t[at+2].text] = ""
		return
	}
	if t[at].kind != "identifier" || at > 0 && (t[at-1].text == "." || t[at-1].text == ":" || t[at-1].text == "->") {
		return
	}
	if at > 0 && t[at-1].text != "{" && t[at-1].text != ";" && t[at-1].text != "}" {
		return
	}
	j := at
	for j < end && (t[j].text == "const" || t[j].text == "volatile") {
		j++
	}
	begin := j
	if j >= end || t[j].kind != "identifier" {
		return
	}
	j++
	for j+2 < end && t[j].text == ":" && t[j+1].text == ":" && t[j+2].kind == "identifier" {
		j += 3
	}
	typ := dartJoined(t[begin:j])
	for j < end && (t[j].text == "*" || t[j].text == "&" || t[j].text == "const") {
		j++
	}
	if j+1 >= end || t[j].kind != "identifier" {
		return
	}
	next := t[j+1].text
	if next != ";" && next != "=" && next != "{" {
		return
	}
	if typ == "auto" {
		typ = ""
	}
	env[t[j].text] = typ
}

func cLambdaEnd(s cFamilySource, at, end int) int {
	close, ok := s.pairs[at]
	if !ok {
		return at
	}
	next := close + 1
	if next < end && s.tokens[next].text == "(" {
		next = s.pairs[next] + 1
	}
	for next < end && (s.tokens[next].text == "mutable" || s.tokens[next].text == "noexcept" || s.tokens[next].text == "static" || s.tokens[next].text == "constexpr" || s.tokens[next].text == "consteval") {
		keyword := s.tokens[next].text
		next++
		if keyword == "noexcept" && next < end && s.tokens[next].text == "(" {
			if close, ok := s.pairs[next]; ok {
				next = close + 1
			}
		}
	}
	if next+1 < end && s.tokens[next].text == "-" && s.tokens[next+1].text == ">" {
		next += 2
		for next < end && s.tokens[next].text != "{" && s.tokens[next].text != ";" {
			next++
		}
	}

	if next < end && s.tokens[next].text == "{" {
		return s.pairs[next]
	}
	return at
}

func cUnqualifiedCandidates(owner, name string, visible map[string]bool, files map[string]cFamilySource, functions map[string][]dartMember) []dartMember {
	for {
		declared := false
		class := false
		for file := range visible {
			for _, m := range files[file].members {
				if m.symbol.Owner == owner && m.symbol.Name == name {
					declared = true
				}
			}
			for _, typ := range files[file].types {
				qualified := typ.Name
				if typ.Owner != "" {
					qualified = typ.Owner + "::" + typ.Name
				}
				if qualified == owner && (typ.Kind == "class" || typ.Kind == "struct") {
					class = true
				}
			}
		}
		candidates := functions[owner+"."+name]
		if declared || len(candidates) > 0 || class || owner == "" {
			return candidates
		}
		if at := strings.LastIndex(owner, "::"); at >= 0 {
			owner = owner[:at]
		} else {
			owner = ""
		}
	}
}

func cClassOwner(owner string, files map[string]cFamilySource) bool {
	for _, file := range files {
		for _, typ := range file.types {
			qualified := typ.Name
			if typ.Owner != "" {
				qualified = typ.Owner + "::" + typ.Name
			}
			if qualified == owner && (typ.Kind == "class" || typ.Kind == "struct") {
				return true
			}
		}
	}
	return false
}
