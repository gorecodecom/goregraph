package scan

// csharpTopLevelMember inventories source statements without executing Program.cs.
func csharpTopLevelMember(s csharpSource) (csharpMember, bool) {
	for i := 0; i < len(s.tokens); i++ {
		token := s.tokens[i]
		if csharpTypeKeyword(token) {
			if end := csharpTypeBodyEnd(s, i); end > i {
				i = end
				continue
			}
		}
		if token.text == "namespace" {
			return csharpMember{}, false
		}
		if token.text == "using" {
			for i < len(s.tokens) && s.tokens[i].text != ";" {
				i++
			}
			continue
		}
		if token.text == "var" || token.text == "await" || (token.kind == "identifier" && i+2 < len(s.tokens) && s.tokens[i+1].kind == "identifier" && s.tokens[i+2].text == "=") {
			return csharpMember{symbol: s.declaration("top_level", "Program statements", "<top-level>@"+s.file, "", token.line), owner: "<top-level>", end: len(s.tokens)}, true
		}
	}
	return csharpMember{}, false
}

func csharpTypeBodyEnd(s csharpSource, at int) int {
	for i := at + 1; i < len(s.tokens); i++ {
		if s.tokens[i].text == ";" {
			return i
		}
		if s.tokens[i].text == "{" {
			if close, ok := s.pairs[i]; ok {
				return close
			}
			return at
		}
	}
	return at
}
