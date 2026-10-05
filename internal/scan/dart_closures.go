package scan

import "fmt"

func (p *dartProject) extractDartClosures() {
	for i := range p.sources {
		s := &p.sources[i]
		var closures []dartMember
		for at, token := range s.tokens {
			if token.text != "(" || at == 0 {
				continue
			}
			close, ok := s.pairs[at]
			if !ok {
				continue
			}
			previous := s.tokens[at-1]
			if previous.kind == "identifier" || previous.text == ">" || previous.text == "." {
				continue
			}
			body := close + 1
			if body < len(s.tokens) && s.tokens[body].text == "async" {
				body++
			}
			if body >= len(s.tokens) || (s.tokens[body].text != "{" && s.tokens[body].text != "=>") {
				continue
			}
			parentEnd := -1
			owned := false
			for _, m := range s.members {
				if m.start == body && m.symbol.Kind == "test" {
					owned = true
				}
				if m.start < at && m.end > body && (parentEnd < 0 || m.end < parentEnd) {
					parentEnd = m.end
				}
			}
			if owned || parentEnd < 0 {
				continue
			}
			end := body
			if s.tokens[body].text == "{" {
				end, ok = s.pairs[body]
				if !ok {
					continue
				}
			} else {
				body++
				end = body
				for end < parentEnd {
					if s.tokens[end].text == ";" || s.tokens[end].text == "," || s.tokens[end].text == ")" || s.tokens[end].text == "]" || s.tokens[end].text == "}" {
						break
					}
					if e, ok := s.pairs[end]; ok && e > end {
						end = e + 1
						continue
					}
					end++
				}
			}
			name := fmt.Sprintf("<closure@%d:%d>", token.line, at)
			symbol := s.symbol("closure", name, "", token.line)
			closures = append(closures, dartMember{symbol: symbol, parameters: dartParameters(s.tokens[at+1 : close]), start: body, end: end})
		}
		s.members = append(s.members, closures...)
	}
}
