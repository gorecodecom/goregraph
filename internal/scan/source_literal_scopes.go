package scan

import "strings"

// skipLiteralExpression consumes interpolation syntax without extracting calls.
func skipLiteralExpression(body string, start int, open, close byte, line *int) int {
	depth := 1
	for i := start; i < len(body); {
		if body[i] == '\n' {
			*line++
			i++
			continue
		}
		if strings.HasPrefix(body[i:], "//") {
			for i < len(body) && body[i] != '\n' {
				i++
			}
			continue
		}
		if strings.HasPrefix(body[i:], "/*") {
			level := 1
			i += 2
			for i < len(body) && level > 0 {
				if strings.HasPrefix(body[i:], "/*") {
					level++
					i += 2
				} else if strings.HasPrefix(body[i:], "*/") {
					level--
					i += 2
				} else {
					if body[i] == '\n' {
						*line++
					}
					i++
				}
			}
			continue
		}
		prefix := i
		for prefix < len(body) && (body[prefix] == '#' || body[prefix] == '@' || body[prefix] == '$') {
			prefix++
		}
		if prefix < len(body) && (body[prefix] == '"' || body[prefix] == '\'') {
			quote := body[prefix]
			width := 1
			if quote == '"' && strings.HasPrefix(body[prefix:], `"""`) {
				width = 3
			}
			ending := strings.Repeat(string(quote), width)
			hashes := strings.Count(body[i:prefix], "#")
			ending += strings.Repeat("#", hashes)
			verbatim := strings.Contains(body[i:prefix], "@")
			interpolated := strings.Contains(body[i:prefix], "$")
			i = prefix + width
			for i < len(body) {
				if strings.HasPrefix(body[i:], ending) {
					if verbatim && i+1 < len(body) && body[i+1] == quote {
						i += 2
						continue
					}
					i += len(ending)
					break
				}
				if body[i] == '\n' {
					*line++
				}
				marker := "\\" + strings.Repeat("#", hashes) + "("
				if strings.HasPrefix(body[i:], marker) {
					i = skipLiteralExpression(body, i+len(marker), '(', ')', line)
					continue
				}
				if interpolated && body[i] == '{' && !(i+1 < len(body) && body[i+1] == '{') {
					i = skipLiteralExpression(body, i+1, '{', '}', line)
					continue
				}
				if body[i] == '\\' && i+1 < len(body) && !verbatim {
					i += 2
				} else {
					i++
				}
			}
			continue
		}
		if body[i] == open {
			depth++
		}
		if body[i] == close {
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return len(body)
}

func swiftRegexLiteral(body string, start, prefix int, line *int) (int, bool) {
	hashes := prefix - start
	ending := "/" + strings.Repeat("#", hashes)
	for i := prefix + 1; i < len(body); i++ {
		if strings.HasPrefix(body[i:], ending) {
			return i + len(ending), true
		}
		if body[i] == '\n' {
			if hashes == 0 {
				return start, false
			}
			*line++
		}
		if body[i] == '\\' && i+1 < len(body) {
			i++
		}
	}
	return start, false
}

func swiftRegexPosition(tokens []csharpToken) bool {
	if len(tokens) == 0 {
		return true
	}
	switch tokens[len(tokens)-1].text {
	case "=", "(", "[", ",", ":", "return", "case":
		return true
	}
	return false
}
