package scan

import (
	"fmt"
	"strings"
)

func (p *dartProject) externalImport(s dartSource, uri, name string, env map[string]string) bool {
	if _, shadow := env[name]; shadow {
		return false
	}
	prefix, local := "", name
	if dot := strings.Index(name, "."); dot >= 0 {
		prefix = name[:dot]
		local = name[dot+1:]
	}
	if _, shadow := env[prefix]; prefix != "" && shadow {
		return false
	}
	if len(p.visible(s, name)) > 0 {
		return false
	}
	count := 0
	for _, i := range p.librarySources[s.library] {
		for _, d := range p.sources[i].directives {
			if d.kind == "import" && d.uri == uri && d.prefix == prefix && !d.conditional && !d.deferred && dartAllows(d, local) {
				count++
			}
		}
	}
	return count == 1
}
func (p *dartProject) flutterImport(s dartSource, name string, env map[string]string) bool {
	for _, uri := range []string{"package:flutter/material.dart", "package:flutter/widgets.dart", "package:flutter/cupertino.dart", "package:flutter/foundation.dart"} {
		if p.externalImport(s, uri, name, env) {
			return true
		}
	}
	return false
}

func (p *dartProject) extractDartTests() {
	for i := range p.sources {
		s := &p.sources[i]
		var tests []dartMember
		t := s.tokens
		for at := 0; at+2 < len(t); at++ {
			if t[at].kind != "identifier" || (t[at].text != "test" && t[at].text != "testWidgets") || t[at+1].text != "(" {
				continue
			}
			name := t[at].text
			callName := name
			if at >= 2 && t[at-1].text == "." {
				callName = t[at-2].text + "." + name
			}
			env := map[string]string{}
			for _, m := range s.members {
				if m.symbol.Owner == "" && m.symbol.Name == name {
					env[name] = m.typeName
				}
				if m.start <= at && at < m.end {
					for _, parameter := range m.parameters {
						env[parameter.name] = parameter.typeName
					}
				}
			}
			valid := p.externalImport(*s, "package:test/test.dart", callName, env) || p.externalImport(*s, "package:flutter_test/flutter_test.dart", callName, env)
			if !valid {
				continue
			}
			close, ok := s.pairs[at+1]
			if !ok {
				continue
			}
			args := dartSplit(t[at+2 : close])
			if len(args) < 2 || len(args[0]) != 1 {
				continue
			}
			label, literal := dartLiteral(args[0][0])
			if !literal || len(label) > 240 {
				continue
			}
			callback := args[1]
			if len(callback) == 0 || callback[0].text != "(" {
				continue
			}
			parameterAt := at + 2 + len(args[0]) + 1
			parameterEnd, ok := s.pairs[parameterAt]
			if !ok {
				continue
			}
			body := parameterEnd + 1
			if body < close && t[body].text == "async" {
				body++
			}
			if body >= close || t[body].text != "{" {
				continue
			}
			end, ok := s.pairs[body]
			if !ok {
				continue
			}
			parameters := dartParameters(t[parameterAt+1 : parameterEnd])
			if name == "testWidgets" && len(parameters) == 1 {
				parameters[0].typeName = "WidgetTester"
			}
			symbol := s.symbol("test", label, "", t[at].line)
			symbol.QualifiedName = s.library + "::test@" + fmt.Sprint(t[at].line) + ":" + label
			symbol.ID = StableWorkspaceSymbolID("test", "", "", "dart", symbol.QualifiedName, s.file)
			tests = append(tests, dartMember{symbol: symbol, parameters: parameters, start: body, end: end})
		}
		s.members = append(s.members, tests...)
	}
}

func (p *dartProject) flutterEvidence(result *dartAnalysis, s dartSource, member dartMember, at, close int, receiver, name string, env map[string]string) {
	t := s.tokens
	add := func(capability CapabilityID, kind, framework string) {
		result.capabilities = append(result.capabilities, ArchitectureCapabilityFact{ID: stableID("dart-capability", s.file, fmt.Sprint(at), kind), Language: "dart", Capability: capability, Kind: kind, Framework: framework, File: s.file, Line: t[at].line})
	}
	if receiver == "" {
		for _, typ := range p.visible(s, name) {
			if typ.Kind != "class" {
				continue
			}
			_, decl, ok := p.typeSources(typ)
			if !ok {
				continue
			}
			frameworkWidget := false
			for _, base := range decl.bases {
				frameworkWidget = frameworkWidget || p.flutterImport(s, dartBaseType(base), env)
			}
			kind := "instantiates_type"
			if frameworkWidget {
				kind = "builds_widget"
			}
			result.facts.References = append(result.facts.References, dartReference(s, member.symbol, name, kind, t[at].line, []RichSymbolRecord{typ}))
		}
	}
	if p.externalImport(s, "package:go_router/go_router.dart", strings.TrimPrefix(receiver+"."+name, "."), env) && name == "GoRoute" {
		for _, arg := range dartSplit(t[at+1 : close]) {
			if len(arg) == 3 && arg[0].text == "path" && arg[1].text == ":" {
				route, ok := dartLiteral(arg[2])
				if !ok || !strings.HasPrefix(route, "/") {
					continue
				}
				result.code.Routes = append(result.code.Routes, CodeRouteRecord{Language: "dart", Framework: "go_router", FrameworkBound: true, Kind: "frontend", App: codeFileApp(s.file), Package: s.packageName, RouteID: codeRouteID(codeFileApp(s.file), route), Path: route, File: s.file, Line: t[at].line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "literal Flutter navigation declaration; runtime redirects and navigation are not evaluated"})
				add(CapabilityRoutes, "navigation_route", "go_router")
			}
		}
	}
	if (receiver == "Navigator" || strings.HasSuffix(receiver, ".Navigator")) && p.flutterImport(s, receiver, env) && name == "pushNamed" {
		args := dartSplit(t[at+1 : close])
		if len(args) > 1 && len(args[1]) == 1 {
			if route, ok := dartLiteral(args[1][0]); ok {
				result.facts.References = append(result.facts.References, RichRelationRecord{ID: stableID("dart-navigation", s.file, fmt.Sprint(at)), From: s.file, FromSymbolID: member.symbol.ID, To: route, Type: "navigates_to", Language: "dart", Analyzer: "dart-source", Line: t[at].line, SourceLocation: sourceLocation(t[at].line), Confidence: "EXTRACTED", ConfidenceScore: 1, Resolution: SymbolResolutionUnresolved, NonPromotable: true, preventExact: true, Reason: "literal named navigation target; matching runtime route table is not proven"})
			}
		}
	}
	if (name == "notifyListeners" || name == "setState") && receiver == "" && member.symbol.Owner != "" {
		for _, typ := range s.types {
			if typ.symbol.Name != member.symbol.Owner {
				continue
			}
			for _, base := range typ.bases {
				if p.flutterImport(s, dartBaseType(base), env) {
					add(CapabilityMessaging, "state_notification", "Flutter")
				}
			}
		}
	}
	if member.symbol.Kind == "test" && name == "pumpWidget" && env[receiver] == "WidgetTester" {
		add(CapabilityTests, "widget_test_pump", "flutter_test")
	}
	// Tear-offs are references, not proof that a callback has executed.
	for _, arg := range dartSplit(t[at+1 : close]) {
		if len(arg) < 3 || arg[1].text != ":" || arg[0].text != "onPressed" && arg[0].text != "onTap" && arg[0].text != "builder" && arg[0].text != "listener" {
			continue
		}
		value := arg[2:]
		var candidates []RichSymbolRecord
		if len(value) == 1 && value[0].kind == "identifier" {
			if member.symbol.Owner != "" {
				for _, m := range p.membersFor(s, member.symbol.Owner, value[0].text, false, map[string]bool{}) {
					candidates = append(candidates, m.symbol)
				}
			}
			if len(candidates) == 0 {
				candidates = p.visible(s, value[0].text)
			}
		}
		if len(value) == 3 && value[1].text == "." {
			if typ, ok := env[value[0].text]; ok {
				for _, m := range p.membersFor(s, typ, value[2].text, false, map[string]bool{}) {
					candidates = append(candidates, m.symbol)
				}
			}
		}
		if len(candidates) > 0 {
			result.facts.References = append(result.facts.References, dartReference(s, member.symbol, dartJoined(value), "registers_callback", arg[0].line, candidates))
		}
	}
}
