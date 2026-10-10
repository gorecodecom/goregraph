package scan

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode"
)

type supplementarySource struct {
	file FileRecord
	body string
}
type supplementaryAnalysis struct {
	facts        ProjectSymbolFacts
	code         CodeIntelligenceRecord
	graph        CallGraphRecord
	capabilities []ArchitectureCapabilityFact
	tests        []TestMapRecord
}
type htmlTag struct {
	name          string
	attrs         map[string]string
	line, endLine int
	closing       bool
}

func supplementaryLanguage(language string) bool {
	switch language {
	case "gdscript", "godot", "html", "css", "batch", "objectivec", "c", "cpp", "ruby", "kotlin":
		return true
	}
	return false
}
func supplementarySymbol(file FileRecord, kind, name string, line int) RichSymbolRecord {
	qualified := file.Path + "::" + kind + "@" + fmt.Sprint(line) + ":" + name
	return RichSymbolRecord{ID: StableWorkspaceSymbolID(kind, "", "", file.Language, qualified, file.Path), Name: name, Kind: kind, Language: file.Language, File: file.Path, Line: line, QualifiedName: qualified, SourceLocation: sourceLocation(line), Analyzer: file.Language + "-source", Confidence: ConfidenceExact, Coverage: CoveragePartial, Limitations: []string{"static source evidence; runtime evaluation, cascading and external configuration are not executed"}}
}
func supplementaryReference(file FileRecord, name, kind string, line int) RichRelationRecord {
	return RichRelationRecord{ID: stableID("supplementary-reference", file.Path, kind, name, fmt.Sprint(line)), From: file.Path, To: name, TargetQualifiedName: name, Type: kind, Language: file.Language, Analyzer: file.Language + "-source", Line: line, SourceLocation: sourceLocation(line), Confidence: "EXTRACTED", ConfidenceScore: 1, Resolution: SymbolResolutionUnresolved, NonPromotable: true, preventExact: true, Reason: "literal source reference; runtime applicability is not evaluated"}
}

func htmlTags(body string) []htmlTag {
	var tags []htmlTag
	line := 1
	for at := 0; at < len(body); {
		if strings.HasPrefix(body[at:], "<!--") {
			end := strings.Index(body[at+4:], "-->")
			if end < 0 {
				break
			}
			end += at + 7
			line += strings.Count(body[at:end], "\n")
			at = end
			continue
		}
		if body[at] != '<' {
			if body[at] == '\n' {
				line++
			}
			at++
			continue
		}
		start, tagLine := at, line
		at++
		closing := false
		if at < len(body) && body[at] == '/' {
			closing = true
			at++
		}
		nameStart := at
		for at < len(body) && (unicode.IsLetter(rune(body[at])) || body[at] >= '0' && body[at] <= '9' || body[at] == '-' || body[at] == ':') {
			at++
		}
		if at == nameStart {
			continue
		}
		name := strings.ToLower(body[nameStart:at])
		attrs := map[string]string{}
		valid := true
		for at < len(body) && body[at] != '>' {
			for at < len(body) && (body[at] == ' ' || body[at] == '\n' || body[at] == '\r' || body[at] == '\t' || body[at] == '/') {
				at++
			}
			if at >= len(body) || body[at] == '>' {
				break
			}
			keyStart := at
			for at < len(body) && !strings.ContainsRune(" =/>\t\r\n", rune(body[at])) {
				at++
			}
			key := strings.ToLower(body[keyStart:at])
			for at < len(body) && strings.ContainsRune(" \t\r\n", rune(body[at])) {
				at++
			}
			value := ""
			if at < len(body) && body[at] == '=' {
				at++
				for at < len(body) && strings.ContainsRune(" \t\r\n", rune(body[at])) {
					at++
				}
				if at >= len(body) {
					break
				}
				quote := byte(0)
				if body[at] == '\'' || body[at] == '"' {
					quote = body[at]
					at++
				}
				valueStart := at
				if quote != 0 {
					for at < len(body) && body[at] != quote {
						at++
					}
					if at >= len(body) {
						valid = false
						break
					}
					value = body[valueStart:at]
					at++
				} else {
					for at < len(body) && !strings.ContainsRune(" >\t\r\n", rune(body[at])) {
						at++
					}
					value = body[valueStart:at]
				}
			}
			if _, duplicate := attrs[key]; duplicate {
				valid = false
			}
			attrs[key] = html.UnescapeString(value)
			if at == keyStart {
				at++
			}
		}
		if at >= len(body) {
			break
		}
		at++
		line += strings.Count(body[start:at], "\n")
		if valid {
			tags = append(tags, htmlTag{name: name, attrs: attrs, line: tagLine, endLine: line, closing: closing})
		}
		if !closing && (name == "script" || name == "style" || name == "textarea" || name == "title") {
			end := strings.Index(strings.ToLower(body[at:]), "</"+name)
			if end < 0 {
				break
			}
			end += at
			line += strings.Count(body[at:end], "\n")
			at = end
		}
	}
	return tags
}
func analyzeMarkupSource(source supplementarySource, result *supplementaryAnalysis) {
	f := source.file
	if f.Language == "css" {
		analyzeCSSSource(source, result)
		return
	}
	for _, tag := range htmlTags(source.body) {
		if tag.closing {
			continue
		}
		if id := tag.attrs["id"]; id != "" && !strings.ContainsAny(id, "{}\r\n") {
			result.facts.Declarations = append(result.facts.Declarations, supplementarySymbol(f, "element", id, tag.line))
		}
		for _, class := range strings.Fields(tag.attrs["class"]) {
			if !strings.ContainsAny(class, "{}$") {
				result.facts.References = append(result.facts.References, supplementaryReference(f, "."+class, "uses_style_class", tag.line))
			}
		}
		for _, attribute := range []string{"src", "href", "action", "for", "aria-labelledby", "aria-describedby"} {
			value := tag.attrs[attribute]
			if value == "" || strings.ContainsAny(value, "{}\r\n") {
				continue
			}
			kind := "references_resource"
			if attribute == "for" || strings.HasPrefix(attribute, "aria-") {
				for _, id := range strings.Fields(value) {
					result.facts.References = append(result.facts.References, supplementaryReference(f, "#"+id, "references_element", tag.line))
				}
				continue
			}
			if strings.HasPrefix(value, "#") {
				kind = "references_element"
			} else if attribute == "href" && tag.name == "a" {
				kind = "navigates_to"
			} else if attribute == "href" && tag.name == "link" && strings.Contains(tag.attrs["rel"], "stylesheet") {
				kind = "imports_stylesheet"
			} else if attribute == "src" && tag.name == "script" {
				kind = "imports_script"
			}
			// Keep only local or web resources; never export executable URLs or credentials.
			if strings.Contains(value, ":") && !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
				continue
			}
			safe := strings.SplitN(value, "?", 2)[0]
			if strings.Contains(safe, "@") && strings.Contains(safe, "://") {
				continue
			}
			result.facts.References = append(result.facts.References, supplementaryReference(f, safe, kind, tag.line))
		}
		if tag.name == "form" {
			method := strings.ToUpper(tag.attrs["method"])
			if method == "" {
				method = "GET"
			}
			if method != "GET" && method != "POST" {
				continue
			}
			if route, ok := dartSafeRequestPath(tag.attrs["action"]); ok {
				record := literalHTTPContract(f, method, route, "form", nil, nil, tag.line, "literal HTML form action; browser scripts, validation and submission are not executed")
				result.code.APIContracts = append(result.code.APIContracts, record)
			}
		}
	}
}

func analyzeCSSSource(source supplementarySource, result *supplementaryAnalysis) {
	tokens, valid := dartTokens(source.body)
	pairs, balanced := dartPairs(tokens)
	if !valid || !balanced {
		return
	}
	f := source.file
	for at := 0; at < len(tokens); at++ {
		if tokens[at].text == "@" && at+2 < len(tokens) && tokens[at+1].text == "import" {
			end := at + 2
			for end < len(tokens) && tokens[end].text != ";" {
				end++
			}
			value := ""
			for _, t := range tokens[at+2 : end] {
				if literal, ok := dartLiteral(t); ok {
					value = literal
					break
				}
			}
			if value, safe := literalResourceTarget(value); safe {
				result.facts.References = append(result.facts.References, supplementaryReference(f, value, "imports_stylesheet", tokens[at].line))
			}
			at = end
			continue
		}
		if tokens[at].text != "{" {
			continue
		}
		close, ok := pairs[at]
		if !ok {
			continue
		}
		start := at - 1
		for start >= 0 && tokens[start].text != "}" && tokens[start].text != "{" && tokens[start].text != ";" {
			start--
		}
		header := tokens[start+1 : at]
		if len(header) == 0 {
			continue
		}
		if header[0].text == "@" {
			continue
		}
		for _, selector := range dartSplit(header) {
			name := dartJoined(selector)
			if name != "" {
				result.facts.Declarations = append(result.facts.Declarations, supplementaryTokenSymbol(f, "selector", name, selector[0]))
			}
		}
		for i := at + 1; i < close; i++ {
			if tokens[i].text == "{" {
				break
			}
			if tokens[i].text == "-" && i+2 < close && tokens[i+1].text == "-" && tokens[i+2].kind == "identifier" {
				end := i + 2
				for end+1 < close && (tokens[end+1].kind == "identifier" || tokens[end+1].text == "-") {
					end++
				}
				if end+1 < close && tokens[end+1].text == ":" {
					result.facts.Declarations = append(result.facts.Declarations, supplementaryTokenSymbol(f, "custom_property", dartJoined(tokens[i:end+1]), tokens[i]))
				}
			}
			if tokens[i].text == "var" && i+2 < close && tokens[i+1].text == "(" {
				if end, ok := pairs[i+1]; ok {
					args := dartSplit(tokens[i+2 : end])
					if len(args) > 0 {
						result.facts.References = append(result.facts.References, supplementaryReference(f, dartJoined(args[0]), "uses_custom_property", tokens[i].line))
					}
				}
			}
			if tokens[i].text == "url" && i+2 < close && tokens[i+1].text == "(" {
				if end, ok := pairs[i+1]; ok {
					args := tokens[i+2 : end]
					if len(args) == 1 {
						if value, ok := dartLiteral(args[0]); ok {
							if value, safe := literalResourceTarget(value); safe {
								result.facts.References = append(result.facts.References, supplementaryReference(f, value, "references_resource", tokens[i].line))
							}
						}
					}
				}
			}
		}
	}
}

func resolveSupplementaryResources(sources []supplementarySource, files []FileRecord, facts *ProjectSymbolFacts) {
	known := map[string]bool{}
	for _, file := range files {
		known[file.Path] = true
	}
	for i := range facts.References {
		ref := &facts.References[i]
		if ref.Type != "imports_stylesheet" && ref.Type != "imports_script" && ref.Type != "references_resource" {
			continue
		}
		uri := ref.To
		if strings.Contains(uri, ":") || strings.HasPrefix(uri, "/") {
			continue
		}
		target := path.Clean(path.Join(path.Dir(ref.From), uri))
		if strings.HasPrefix(target, "../") || !known[target] {
			continue
		}
		ref.To = target
		ref.Internal = true
		ref.Reason = "literal relative resource points to an indexed file; runtime loading is not asserted"
	}
	sort.Slice(facts.Declarations, func(i, j int) bool { return facts.Declarations[i].ID < facts.Declarations[j].ID })
}

func supplementaryTokenSymbol(file FileRecord, kind, name string, token dartToken) RichSymbolRecord {
	symbol := supplementarySymbol(file, kind, name, token.line)
	symbol.QualifiedName += ":" + fmt.Sprint(token.start)
	symbol.ID = StableWorkspaceSymbolID(kind, "", "", file.Language, symbol.QualifiedName, file.Path)
	return symbol
}

func literalResourceTarget(value string) (string, bool) {
	if value == "" || strings.ContainsAny(value, "{}$\r\n\x00") {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.IsAbs() && parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}
