package scan

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type dartProject struct {
	sources        []dartSource
	packages       []DartPackageRecord
	files          map[string]int
	librarySources map[string][]int
}

func newDartProject(sources []dartSource, packages []DartPackageRecord) *dartProject {
	p := &dartProject{sources: sources, packages: packages, files: map[string]int{}, librarySources: map[string][]int{}}
	for i, s := range sources {
		p.files[s.file] = i
	}
	// A part shares private declarations only with a unique, reciprocally declared owner.
	owners := map[int][]int{}
	for i, s := range sources {
		for _, d := range s.directives {
			if d.kind != "part" || d.conditional {
				continue
			}
			file := dartResolveURI(s, d.uri, packages, p.files)
			j, ok := p.files[file]
			if !ok || sources[j].partOf == "" {
				continue
			}
			if path.Clean(path.Join(path.Dir(file), sources[j].partOf)) == s.file || s.declaredLibrary != "" && sources[j].partOf == s.declaredLibrary {
				owners[j] = append(owners[j], i)
			}
		}
	}
	for j, indices := range owners {
		if len(indices) == 1 {
			p.sources[j].library = p.sources[indices[0]].file
		} else {
			p.sources[j].limitations = append(p.sources[j].limitations, "ambiguous Dart part ownership")
		}
	}
	for i := range p.sources {
		s := &p.sources[i]
		if s.partOf != "" && s.library == s.file {
			s.limitations = append(s.limitations, "unverified Dart part ownership")
		}
		p.librarySources[s.library] = append(p.librarySources[s.library], i)
		for j := range s.types {
			old := s.types[j].symbol
			s.types[j].symbol = s.symbol(old.Kind, old.Name, old.Owner, old.Line)
			s.types[j].symbol.Package = s.packageName
		}
		for j := range s.members {
			old := s.members[j].symbol
			s.members[j].symbol = s.symbol(old.Kind, old.Name, old.Owner, old.Line)
			s.members[j].symbol.Package = s.packageName
		}
	}
	return p
}

func dartAllows(d dartDirective, name string) bool {
	if strings.HasPrefix(name, "_") {
		return false
	}
	if len(d.show) > 0 {
		found := false
		for _, n := range d.show {
			found = found || n == name
		}
		if !found {
			return false
		}
	}
	for _, n := range d.hide {
		if n == name {
			return false
		}
	}
	return true
}

func (p *dartProject) exported(file, name string, seen map[string]bool) []RichSymbolRecord {
	if seen[file] || len(seen) > 128 || strings.HasPrefix(name, "_") {
		return nil
	}
	seen[file] = true
	index, ok := p.files[file]
	if !ok {
		return nil
	}
	s := p.sources[index]
	var result []RichSymbolRecord
	for _, i := range p.librarySources[s.library] {
		for _, typ := range p.sources[i].types {
			if typ.symbol.Name == name {
				result = append(result, typ.symbol)
			}
		}
		for _, member := range p.sources[i].members {
			if member.symbol.Owner == "" && member.symbol.Name == name {
				result = append(result, member.symbol)
			}
		}
	}
	if len(result) > 0 {
		return result
	}
	for _, i := range p.librarySources[s.library] {
		source := p.sources[i]
		for _, d := range source.directives {
			if d.kind == "export" && !d.conditional && dartAllows(d, name) {
				target := dartResolveURI(source, d.uri, p.packages, p.files)
				branch := map[string]bool{}
				for k, v := range seen {
					branch[k] = v
				}
				result = append(result, p.exported(target, name, branch)...)
			}
		}
	}
	return dartUniqueSymbols(result)
}

func (p *dartProject) visible(s dartSource, name string) []RichSymbolRecord {
	prefix, local := "", name
	if dot := strings.Index(name, "."); dot >= 0 {
		prefix = name[:dot]
		local = name[dot+1:]
	}
	var result []RichSymbolRecord
	if prefix == "" {
		for _, i := range p.librarySources[s.library] {
			for _, typ := range p.sources[i].types {
				if typ.symbol.Name == local {
					result = append(result, typ.symbol)
				}
			}
			for _, member := range p.sources[i].members {
				if member.symbol.Owner == "" && member.symbol.Name == local {
					result = append(result, member.symbol)
				}
			}
		}
		if len(result) > 0 {
			return dartUniqueSymbols(result)
		}
	}
	for _, i := range p.librarySources[s.library] {
		source := p.sources[i]
		for _, d := range source.directives {
			if d.kind != "import" || d.prefix != prefix || d.prefix == "_" || d.conditional || d.deferred || !dartAllows(d, local) {
				continue
			}
			target := dartResolveURI(source, d.uri, p.packages, p.files)
			result = append(result, p.exported(target, local, map[string]bool{})...)
		}
	}
	return dartUniqueSymbols(result)
}
func dartUniqueSymbols(symbols []RichSymbolRecord) []RichSymbolRecord {
	seen := map[string]bool{}
	var result []RichSymbolRecord
	for _, s := range symbols {
		if !seen[s.ID] {
			seen[s.ID] = true
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
func (p *dartProject) typeSources(symbol RichSymbolRecord) (dartSource, dartType, bool) {
	i, ok := p.files[symbol.File]
	if !ok {
		return dartSource{}, dartType{}, false
	}
	s := p.sources[i]
	for _, typ := range s.types {
		if typ.symbol.ID == symbol.ID {
			return s, typ, true
		}
	}
	return s, dartType{}, false
}
func (p *dartProject) membersFor(s dartSource, typeName, name string, static bool, seen map[string]bool) []dartMember {
	var result []dartMember
	typeCandidates := p.visible(s, dartBaseType(typeName))
	if len(typeCandidates) != 1 {
		return nil
	}
	for _, typ := range typeCandidates {
		source, decl, ok := p.typeSources(typ)
		if !ok || seen[typ.ID] {
			continue
		}
		seen[typ.ID] = true
		for _, i := range p.librarySources[source.library] {
			for _, m := range p.sources[i].members {
				if m.symbol.Owner == typ.Name && m.symbol.Name == name && (!static || m.static || m.symbol.Kind == "constructor") && (!strings.HasPrefix(name, "_") || source.library == s.library) {
					result = append(result, m)
				}
			}
		}
		if len(result) == 0 && !static {
			for i := len(decl.bases) - 1; i >= 0; i-- {
				inherited := p.membersFor(source, decl.bases[i], name, false, seen)
				if len(inherited) > 0 {
					result = append(result, inherited...)
					break
				}
			}
		}
	}
	return result
}
func (p *dartProject) symbolMember(symbol RichSymbolRecord) (dartMember, bool) {
	if i, ok := p.files[symbol.File]; ok {
		for _, m := range p.sources[i].members {
			if m.symbol.ID == symbol.ID {
				return m, true
			}
		}
	}
	return dartMember{}, false
}

func dartArgumentShape(args []dartToken) (int, map[string]bool, bool) {
	positional := 0
	named := map[string]bool{}
	valid := true
	for _, chunk := range dartSplit(args) {
		if len(chunk) == 0 {
			continue
		}
		if chunk[0].text == "..." {
			valid = false
		}
		if len(chunk) > 1 && chunk[0].kind == "identifier" && chunk[1].text == ":" {
			if named[chunk[0].text] {
				valid = false
			}
			named[chunk[0].text] = true
		} else {
			positional++
		}
	}
	return positional, named, valid
}
func dartAccepts(member dartMember, args []dartToken) bool {
	count, named, valid := dartArgumentShape(args)
	if !valid {
		return false
	}
	minimum, maximum := 0, 0
	allowed := map[string]bool{}
	for _, p := range member.parameters {
		if p.named {
			publicName := strings.TrimPrefix(p.name, "_")
			allowed[publicName] = true
			if p.required && !named[publicName] {
				return false
			}
		} else {
			maximum++
			if !p.optional {
				minimum++
			}
		}
	}
	if count < minimum || count > maximum {
		return false
	}
	for name := range named {
		if !allowed[name] {
			return false
		}
	}
	return true
}

func dartReference(source dartSource, from RichSymbolRecord, name, kind string, line int, candidates []RichSymbolRecord) RichRelationRecord {
	ref := RichRelationRecord{ID: stableID("dart-reference", from.ID, kind, name, fmt.Sprint(line)), From: source.file, To: name, TargetQualifiedName: name, Type: kind, Language: "dart", Analyzer: "dart-source", Line: line, SourceLocation: sourceLocation(line), FromSymbolID: from.ID, Resolution: SymbolResolutionUnresolved, NonPromotable: true, preventExact: true, Confidence: "INFERRED", Reason: "external, dynamic, or unsupported Dart binding"}
	candidates = dartUniqueSymbols(candidates)
	for _, c := range candidates {
		ref.CandidateSymbolIDs = append(ref.CandidateSymbolIDs, c.ID)
	}
	if len(candidates) > 1 {
		ref.Resolution = SymbolResolutionAmbiguous
		ref.Reason = "ambiguous Dart declarations"
	}
	if len(candidates) == 1 && len(source.limitations) == 0 && len(candidates[0].Limitations) == 1 {
		c := candidates[0]
		ref.To = c.File
		ref.ToSymbolID = c.ID
		ref.TargetQualifiedName = c.QualifiedName
		ref.Resolution = SymbolResolutionExact
		ref.NonPromotable = false
		ref.preventExact = false
		ref.Internal = true
		ref.Confidence = "EXTRACTED"
		ref.ConfidenceScore = 1
		ref.Reason = "unique static Dart declaration; runtime dispatch is not evaluated"
	}
	return ref
}

func (p *dartProject) bindSource(result *dartAnalysis, s dartSource) {
	for _, typ := range s.types {
		for _, base := range typ.bases {
			result.facts.References = append(result.facts.References, dartReference(s, typ.symbol, base, "extends_type", typ.symbol.Line, p.visible(s, dartBaseType(base))))
		}
	}
	for _, member := range s.members {
		if member.typeName != "" {
			result.facts.References = append(result.facts.References, dartReference(s, member.symbol, member.typeName, "uses_type", member.symbol.Line, p.visible(s, dartBaseType(member.typeName))))
		}
		env := map[string]string{}
		for _, i := range p.librarySources[s.library] {
			for _, field := range p.sources[i].members {
				if field.symbol.Owner == member.symbol.Owner && (field.symbol.Kind == "field" || field.symbol.Kind == "variable") {
					env[field.symbol.Name] = field.typeName
				}
			}
		}
		for _, parameter := range member.parameters {
			env[parameter.name] = parameter.typeName
			if parameter.typeName != "" {
				result.facts.References = append(result.facts.References, dartReference(s, member.symbol, parameter.typeName, "uses_type", member.symbol.Line, p.visible(s, dartBaseType(parameter.typeName))))
			}
		}
		state := dartRequestState{urls: map[string]string{}, requests: map[string]dartRequestDescriptor{}}
		var scopes []map[string]string
		var requestScopes []dartRequestState
		for at := member.start; at < member.end && at < len(s.tokens); at++ {
			token := s.tokens[at]
			nestedTest := false
			{
				for _, test := range s.members {
					if (test.symbol.Kind == "test" || test.symbol.Kind == "closure") && test.symbol.ID != member.symbol.ID && test.start == at {
						at = test.end
						nestedTest = true
						break
					}
				}
			}
			if nestedTest {
				continue
			}
			if token.text == "{" {
				saved := map[string]string{}
				for k, v := range env {
					saved[k] = v
				}
				scopes = append(scopes, saved)
				requestScopes = append(requestScopes, state.clone())
			}
			if token.text == "}" && len(scopes) > 0 {
				env = scopes[len(scopes)-1]
				scopes = scopes[:len(scopes)-1]
				state = requestScopes[len(requestScopes)-1]
				requestScopes = requestScopes[:len(requestScopes)-1]
			}
			dartLocalType(s, at, env)
			p.trackRequestAssignment(s, at, env, &state)
			if token.text != "(" || at == 0 {
				continue
			}
			close, ok := s.pairs[at]
			if !ok || close > member.end {
				continue
			}
			nameAt := at - 1
			if s.tokens[nameAt].text == ">" {
				depth := 1
				nameAt--
				for nameAt >= member.start && depth > 0 {
					if s.tokens[nameAt].text == ">" {
						depth++
					}
					if s.tokens[nameAt].text == "<" {
						depth--
					}
					nameAt--
				}
			}
			if nameAt < member.start || s.tokens[nameAt].kind != "identifier" {
				continue
			}
			name := s.tokens[nameAt].text
			if dartControlWord(name) {
				continue
			}
			receiver := ""
			start := nameAt
			for start-2 >= member.start && (s.tokens[start-1].text == "." || s.tokens[start-1].text == "?.") && s.tokens[start-2].kind == "identifier" {
				start -= 2
			}
			if start < nameAt {
				receiver = dartJoined(s.tokens[start : nameAt-1])
			}
			var candidates []dartMember
			switch {
			case receiver == "":
				if _, shadow := env[name]; shadow {
					break
				}
				if member.symbol.Owner != "" {
					candidates = p.membersFor(s, member.symbol.Owner, name, false, map[string]bool{})
				}
				if len(candidates) == 0 {
					for _, sym := range p.visible(s, name) {
						if m, ok := p.symbolMember(sym); ok {
							candidates = append(candidates, m)
						} else if sym.Kind == "class" || sym.Kind == "enum" || sym.Kind == "extension_type" {
							candidates = append(candidates, p.membersFor(s, name, name, true, map[string]bool{})...)
						}
					}
				}
			case receiver == "this":
				candidates = p.membersFor(s, member.symbol.Owner, name, false, map[string]bool{})
			case receiver == "super":
				for _, typ := range s.types {
					if typ.symbol.Name == member.symbol.Owner {
						for _, base := range typ.bases {
							candidates = append(candidates, p.membersFor(s, base, name, false, map[string]bool{})...)
						}
					}
				}
			default:
				if typeName, exists := env[receiver]; exists {
					if typeName != "" {
						candidates = p.membersFor(s, typeName, name, false, map[string]bool{})
					}
				} else {
					// Import prefixes bind top-level functions and named constructors without global name matching.
					for _, sym := range p.visible(s, receiver+"."+name) {
						if m, ok := p.symbolMember(sym); ok {
							candidates = append(candidates, m)
						} else {
							candidates = append(candidates, p.membersFor(s, receiver+"."+name, sym.Name, true, map[string]bool{})...)
						}
					}
					candidates = append(candidates, p.membersFor(s, receiver, name, true, map[string]bool{})...)
					for _, typ := range p.visible(s, receiver) {
						candidates = append(candidates, p.membersFor(s, receiver, typ.Name+"."+name, true, map[string]bool{})...)
					}
				}
			}
			p.flutterEvidence(result, s, member, at, close, receiver, name, env)
			p.frameworkEvidence(result, s, member, at, close, receiver, name, env, state)
			var symbols []RichSymbolRecord
			for _, candidate := range candidates {
				if dartAccepts(candidate, s.tokens[at+1:close]) {
					symbols = append(symbols, candidate.symbol)
				}
			}
			ref := dartReference(s, member.symbol, strings.TrimPrefix(receiver+"."+name, "."), "calls_method_owner", token.line, symbols)
			result.facts.References = append(result.facts.References, ref)
			if ref.Resolution == SymbolResolutionExact {
				target := symbols[0]
				if member.symbol.Kind == "test" {
					result.tests = append(result.tests, TestMapRecord{TestFile: s.file, TestMethod: member.symbol.Name, TestCase: member.symbol.Name, TargetFile: target.File, TargetClass: target.Owner, TargetMethod: target.Name, Type: "unit", Line: token.line, Confidence: "EXTRACTED", ConfidenceScore: 1, Reason: "static test callback calls this declaration; test execution is not asserted"})
				}
				result.graph.Edges = append(result.graph.Edges, CallGraphEdgeRecord{ID: stableID("dart-call", member.symbol.ID, target.ID, fmt.Sprint(at)), From: MethodRefRecord{Owner: member.symbol.Owner, Method: member.symbol.Name, File: s.file, Line: member.symbol.Line}, To: MethodRefRecord{Owner: target.Owner, Method: target.Name, File: target.File, Line: target.Line}, Type: "calls", Line: token.line, SourceFile: s.file, Confidence: ref.Confidence, ConfidenceScore: 1, FromSymbolID: member.symbol.ID, ToSymbolID: target.ID, TargetQualifiedName: target.QualifiedName, Resolution: SymbolResolutionExact, Reason: ref.Reason})
			}
		}
	}
}

func dartControlWord(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "assert", "return", "throw", "await", "Function":
		return true
	}
	return false
}
func dartLocalType(s dartSource, at int, env map[string]string) {
	t := s.tokens
	if at <= 0 || at+1 >= len(t) || t[at].kind != "identifier" {
		return
	}
	if t[at+1].text == "=" {
		start := at - 1
		for start >= 0 && t[start].text != ";" && t[start].text != "{" && t[start].text != "}" {
			start--
		}
		prefix := t[start+1 : at]
		if len(prefix) == 0 {
			if _, exists := env[t[at].text]; exists {
				env[t[at].text] = ""
			}
			return
		}
		if len(prefix) > 0 && !dartContains(prefix, "return") && !dartContains(prefix, ".") {
			declared := dartDeclaredType(prefix)
			if declared == "" {
				declared = dartConstructorType(t[at+2:])
				if declared == "" {
					declared = ""
				}
			}
			env[t[at].text] = declared
		}
	}
	if t[at+1].text == ";" && at >= 1 && t[at-1].kind == "identifier" && !dartControlWord(t[at-1].text) {
		env[t[at].text] = t[at-1].text
	}
}
