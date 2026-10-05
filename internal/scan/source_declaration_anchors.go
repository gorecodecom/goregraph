package scan

import "strings"

// SourceDeclarationAnchor describes an independently parsed declaration for read-only navigation.
type SourceDeclarationAnchor struct {
	Name, Kind    string
	Line, EndLine int
}

// StructuredSourceDeclarations shares native adapter rules with current-source verification.
func StructuredSourceDeclarations(file, body string) ([]SourceDeclarationAnchor, bool) {
	language := detectSourceLanguage(file, body)
	record := FileRecord{Path: file, Language: language}
	var anchors []SourceDeclarationAnchor
	add := func(symbol RichSymbolRecord, end int) {
		if end < symbol.Line {
			end = symbol.Line
		}
		anchors = append(anchors, SourceDeclarationAnchor{Name: symbol.Name, Kind: symbol.Kind, Line: symbol.Line, EndLine: end})
	}
	if language == "dart" {
		for _, d := range DartSourceDeclarations(file, body) {
			anchors = append(anchors, SourceDeclarationAnchor(d))
		}
		return anchors, true
	}
	switch language {
	case "html":
		for _, tag := range htmlTags(body) {
			if !tag.closing {
				if id := tag.attrs["id"]; id != "" {
					add(supplementarySymbol(record, "element", id, tag.line), tag.endLine)
				}
				if tag.name == "form" {
					add(supplementarySymbol(record, "element", "form", tag.line), tag.endLine)
				}
			}
		}
	case "css":
		tokens, valid := dartTokens(body)
		pairs, balanced := dartPairs(tokens)
		if !valid || !balanced {
			return nil, true
		}
		for at, t := range tokens {
			if t.text != "{" {
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
			if len(header) == 0 || header[0].text == "@" {
				continue
			}
			for _, selector := range dartSplit(header) {
				if len(selector) > 0 {
					add(supplementaryTokenSymbol(record, "selector", dartJoined(selector), selector[0]), tokens[close].line)
				}
			}
		}
		var facts supplementaryAnalysis
		analyzeCSSSource(supplementarySource{record, body}, &facts)
		for _, symbol := range facts.facts.Declarations {
			if symbol.Kind == "custom_property" {
				add(symbol, symbol.Line)
			}
		}
	case "batch":
		lines := strings.Split(body, "\n")
		for at, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, ":") || strings.HasPrefix(trimmed, "::") {
				continue
			}
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, ":"))
			end := at + 1
			for end < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[end]), ":") {
				end++
			}
			add(supplementarySymbol(record, "label", name, at+1), end)
		}
		add(supplementarySymbol(record, "script", file, 1), len(lines))
	case "objectivec":
		s := parseObjCSource(record, body)
		if !s.valid {
			return nil, true
		}
		for _, symbol := range s.classes {
			add(symbol, symbol.Line)
		}
		for _, member := range s.methods {
			add(member.symbol, s.tokens[member.end].line)
		}
	case "ruby":
		s := parseRubySource(record, body)
		if !s.valid {
			return nil, true
		}
		for _, symbol := range s.symbols {
			add(symbol, symbol.Line)
		}
		for _, m := range s.members {
			if m.end < len(s.tokens) {
				add(m.symbol, s.tokens[m.end].line)
			}
		}
	case "kotlin":
		s := parseKotlinSource(record, body)
		if !s.valid {
			return nil, true
		}
		for _, symbol := range s.types {
			start := kotlinDeclarationAt(s, symbol.Line, "class", symbol.Name)
			end := symbol.Line
			if start >= 0 {
				symbol.Line = kotlinAnnotationStart(s, start)
			}
			add(symbol, end)
		}
		for _, m := range s.members {
			if m.end < len(s.tokens) {
				symbol := m.symbol
				at := kotlinDeclarationAt(s, symbol.Line, "fun", symbol.Name)
				if at >= 0 {
					symbol.Line = kotlinAnnotationStart(s, at)
				}
				add(symbol, s.tokens[m.end].line)
			}
		}
	case "c", "cpp":
		s := parseCFamilySource(record, body)
		for _, symbol := range s.types {
			add(symbol, symbol.Line)
		}
		for _, m := range s.members {
			add(m.symbol, s.tokens[m.end].line)
		}
	default:
		return nil, false
	}
	return anchors, true
}
