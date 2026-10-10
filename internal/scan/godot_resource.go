package scan

import (
	"fmt"
	"path"
	"strings"
)

type godotSection struct {
	kind   string
	attrs  map[string]godotToken
	header []godotToken
	body   []godotToken
	line   int
}

type godotResourceBinding struct {
	uri    string
	symbol *RichSymbolRecord
}

func godotSections(tokens []godotToken) []godotSection {
	var sections []godotSection
	bracketEnds := godotBracketEnds(tokens)
	for i := 0; i < len(tokens); i++ {
		end, ok := godotSectionHeaderEnd(tokens, i, bracketEnds)
		if !ok {
			continue
		}
		section := godotSection{kind: tokens[i+1].text, attrs: map[string]godotToken{}, header: tokens[i+2 : end], line: tokens[i].line}
		for j := i + 2; j+2 < end; j++ {
			if tokens[j].kind == "identifier" && tokens[j+1].text == "=" && (tokens[j+2].kind == "string" || tokens[j+2].kind == "number") {
				section.attrs[tokens[j].text] = tokens[j+2]
			}
		}
		bodyStart := end + 1
		bodyEnd := bodyStart
		depth := 0
		for bodyEnd < len(tokens) {
			if depth == 0 && (bodyEnd == bodyStart || tokens[bodyEnd-1].text != "=") {
				if _, sectionStart := godotSectionHeaderEnd(tokens, bodyEnd, bracketEnds); sectionStart {
					break
				}
			}
			if tokens[bodyEnd].kind == "punctuation" {
				switch tokens[bodyEnd].text {
				case "(", "[", "{":
					depth++
				case ")", "]", "}":
					if depth > 0 {
						depth--
					}
				}
			}
			bodyEnd++
		}
		section.body = tokens[bodyStart:bodyEnd]
		sections = append(sections, section)
		i = bodyEnd - 1
	}
	return sections
}

func godotBracketEnds(tokens []godotToken) map[int]int {
	ends := map[int]int{}
	var stack []int
	for i, token := range tokens {
		if token.kind != "punctuation" {
			continue
		}
		switch token.text {
		case "[":
			stack = append(stack, i)
		case "]":
			if len(stack) > 0 {
				opening := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				ends[opening] = i
			}
		}
	}
	return ends
}

func godotSectionHeaderEnd(tokens []godotToken, at int, bracketEnds map[int]int) (int, bool) {
	if tokens[at].kind != "punctuation" || tokens[at].text != "[" || tokens[at].column != 0 || at+1 >= len(tokens) || tokens[at+1].kind != "identifier" {
		return 0, false
	}
	end, ok := bracketEnds[at]
	return end, ok
}

func analyzeGodotResource(result *supplementaryAnalysis, source supplementarySource, scripts map[string]gdscriptSource, known map[string]bool) {
	tokens := godotTokens(source.body, true)
	sections := godotSections(tokens)
	kind := "resource"
	if strings.EqualFold(path.Ext(source.file.Path), ".tscn") {
		kind = "scene"
	}
	if path.Base(source.file.Path) == "project.godot" {
		kind = "project"
	}
	root := supplementarySymbol(source.file, kind, path.Base(source.file.Path), 1)
	result.facts.Declarations = append(result.facts.Declarations, root)
	external := map[string][]godotResourceBinding{}
	subresources := map[string][]godotResourceBinding{}
	nodes := map[string][]RichSymbolRecord{}
	nodeScripts := map[string][]string{}
	sectionOwners := map[int]RichSymbolRecord{}
	for _, section := range sections {
		switch section.kind {
		case "ext_resource":
			id, uri := section.attrs["id"], section.attrs["path"]
			if id.text == "" || uri.kind != "string" {
				continue
			}
			binding := godotResourceBinding{uri: uri.text}
			target := godotResourcePath(source.file.Path, uri.text)
			if script, ok := scripts[target]; ok {
				symbol := script.class
				binding.symbol = &symbol
			}
			external[id.text] = append(external[id.text], binding)
			ref := godotFileReference(source.file, root, uri.text, "godot_resource", uri.line, scripts, known)
			result.facts.References = append(result.facts.References, ref)
		case "sub_resource":
			id := section.attrs["id"]
			if id.text == "" {
				continue
			}
			symbol := supplementarySymbol(source.file, "resource", id.text, section.line)
			symbol.Owner = root.QualifiedName
			result.facts.Declarations = append(result.facts.Declarations, symbol)
			subresources[id.text] = append(subresources[id.text], godotResourceBinding{symbol: &symbol})
			sectionOwners[section.line] = symbol
		case "node":
			name := section.attrs["name"]
			if name.kind != "string" {
				continue
			}
			parent, hasParent := section.attrs["parent"]
			nodePath := "."
			if hasParent {
				if parent.kind != "string" || strings.Contains(name.text, "/") {
					continue
				}
				nodePath = path.Clean(path.Join(parent.text, name.text))
				if nodePath == ".." || strings.HasPrefix(nodePath, "../") || path.IsAbs(nodePath) {
					continue
				}
			}
			symbol := supplementarySymbol(source.file, "node", name.text, section.line)
			symbol.Owner = root.QualifiedName
			nodes[nodePath] = append(nodes[nodePath], symbol)
			sectionOwners[section.line] = symbol
			result.facts.Declarations = append(result.facts.Declarations, symbol)
		}
	}
	for _, section := range sections {
		owner := root
		if symbol, ok := sectionOwners[section.line]; ok {
			owner = symbol
		}
		uses := append(append([]godotToken(nil), section.header...), section.body...)
		for i, token := range uses {
			if token.kind != "identifier" || (token.text != "ExtResource" && token.text != "SubResource") || i+3 >= len(uses) || uses[i+1].text != "(" || uses[i+3].text != ")" {
				continue
			}
			id := uses[i+2]
			if id.kind != "string" && id.kind != "number" {
				continue
			}
			bindings := external[id.text]
			if token.text == "SubResource" {
				bindings = subresources[id.text]
			}
			relation := "uses_resource"
			isScript := section.kind == "node" && i >= len(section.header)+2 && uses[i-2].text == "script" && uses[i-1].text == "="
			if isScript {
				relation = "attaches_script"
			}
			var targets []RichSymbolRecord
			for _, binding := range bindings {
				if binding.symbol != nil {
					targets = append(targets, *binding.symbol)
				}
			}
			ref := godotSymbolReference(source.file, owner, id.text, relation, token.line, nil)
			ref.ID = stableID("godot-resource-use", source.file.Path, fmt.Sprint(section.line), fmt.Sprint(i))
			if len(bindings) == 1 {
				bindGodotReference(&ref, targets)
				if len(targets) == 0 {
					target := godotResourcePath(source.file.Path, bindings[0].uri)
					if known[target] {
						ref.To, ref.Internal = target, true
						ref.Reason = "serialized resource ID points to an indexed file"
					}
				}
			} else if len(bindings) > 1 {
				ref.Resolution = SymbolResolutionAmbiguous
			}
			result.facts.References = append(result.facts.References, ref)
			if isScript {
				for nodePath, symbols := range nodes {
					for _, symbol := range symbols {
						if symbol.ID == owner.ID {
							for _, binding := range bindings {
								nodeScripts[nodePath] = append(nodeScripts[nodePath], godotResourcePath(source.file.Path, binding.uri))
							}
						}
					}
				}
			}
		}
		if kind == "project" {
			extractGodotSettings(result, source.file, root, section, scripts, known)
		}
	}
	for _, section := range sections {
		if section.kind != "connection" {
			continue
		}
		from, to, method := section.attrs["from"], section.attrs["to"], section.attrs["method"]
		signal := section.attrs["signal"]
		if from.kind != "string" || to.kind != "string" || method.kind != "string" || signal.kind != "string" {
			continue
		}
		owner := root
		if len(nodes[from.text]) == 1 {
			owner = nodes[from.text][0]
		}
		var targets []RichSymbolRecord
		if len(nodes[from.text]) == 1 && len(nodes[to.text]) == 1 && len(nodeScripts[to.text]) == 1 {
			targets = gdscriptLookup(scripts[nodeScripts[to.text][0]], method.text, false)
		}
		ref := godotSymbolReference(source.file, owner, method.text, "connects_signal", section.line, targets)
		ref.LocalName = signal.text
		ref.Reason = "serialized signal connection; inherited scenes, engine dispatch and runtime wiring are not expanded"
		result.facts.References = append(result.facts.References, ref)
	}
}

func godotFileReference(file FileRecord, from RichSymbolRecord, uri, kind string, line int, scripts map[string]gdscriptSource, known map[string]bool) RichRelationRecord {
	ref := supplementaryReference(file, uri, kind, line)
	ref.FromSymbolID = from.ID
	target := godotResourcePath(file.Path, uri)
	if known[target] {
		ref.To, ref.Internal = target, true
		ref.Reason = "literal Godot path points to an indexed file; runtime loading is not asserted"
		if script, ok := scripts[target]; ok {
			bindGodotReference(&ref, []RichSymbolRecord{script.class})
		}
	}
	return ref
}

func extractGodotSettings(result *supplementaryAnalysis, file FileRecord, root RichSymbolRecord, section godotSection, scripts map[string]gdscriptSource, known map[string]bool) {
	for i, token := range section.body {
		if token.kind != "punctuation" || token.text != "=" || i+1 >= len(section.body) || section.body[i+1].kind != "string" {
			continue
		}
		start := i - 1
		for start > 0 && section.body[start-1].line == token.line {
			start--
		}
		var key strings.Builder
		for _, part := range section.body[start:i] {
			key.WriteString(part.text)
		}
		uri, relation := section.body[i+1].text, ""
		if section.kind == "application" && key.String() == "run/main_scene" {
			relation = "starts_scene"
		}
		if section.kind == "autoload" && key.String() != "" {
			relation = "autoloads"
			uri = strings.TrimPrefix(uri, "*")
			symbol := supplementarySymbol(file, "autoload", key.String(), token.line)
			symbol.Owner = root.QualifiedName
			result.facts.Declarations = append(result.facts.Declarations, symbol)
			from := symbol
			result.facts.References = append(result.facts.References, godotFileReference(file, from, uri, relation, token.line, scripts, known))
			continue
		}
		if relation != "" {
			result.facts.References = append(result.facts.References, godotFileReference(file, root, uri, relation, token.line, scripts, known))
		}
	}
}
