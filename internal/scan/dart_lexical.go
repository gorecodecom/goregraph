package scan

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Dart tokens deliberately keep strings opaque, including interpolation expressions.
type dartToken struct {
	text       string
	line       int
	kind       string
	start, end int
}

func dartTokens(body string) ([]dartToken, bool) {
	var tokens []dartToken
	line, valid := 1, true
	for i := 0; i < len(body); {
		r, width := utf8.DecodeRuneInString(body[i:])
		if unicode.IsSpace(r) {
			if r == '\n' {
				line++
			}
			i += width
			continue
		}
		if strings.HasPrefix(body[i:], "//") {
			for i < len(body) && body[i] != '\n' {
				i++
			}
			continue
		}
		if strings.HasPrefix(body[i:], "/*") {
			i += 2
			depth := 1
			for i < len(body) && depth > 0 {
				switch {
				case strings.HasPrefix(body[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(body[i:], "*/"):
					depth--
					i += 2
				default:
					if body[i] == '\n' {
						line++
					}
					i++
				}
			}
			valid = valid && depth == 0
			continue
		}
		raw := r == 'r' && i+1 < len(body) && (body[i+1] == '\'' || body[i+1] == '"')
		if raw || r == '\'' || r == '"' {
			start, firstLine := i, line
			end, ok := dartStringEnd(body, i, 0)
			text := body[start:end]
			line += strings.Count(text, "\n")
			i = end
			tokens = append(tokens, dartToken{text: text, line: firstLine, kind: "string", start: start, end: end})
			valid = valid && ok
			continue
		}
		if r == '_' || unicode.IsLetter(r) || r == '$' {
			start := i
			i += width
			for i < len(body) {
				next, w := utf8.DecodeRuneInString(body[i:])
				if next != '_' && next != '$' && !unicode.IsLetter(next) && !unicode.IsDigit(next) {
					break
				}
				i += w
			}
			tokens = append(tokens, dartToken{text: body[start:i], line: line, kind: "identifier", start: start, end: i})
			continue
		}
		if unicode.IsDigit(r) {
			start := i
			i += width
			for i < len(body) && (body[i] >= '0' && body[i] <= '9' || body[i] == '.') {
				i++
			}
			tokens = append(tokens, dartToken{text: body[start:i], line: line, kind: "number", start: start, end: i})
			continue
		}
		punctuation := body[i : i+width]
		for _, op := range []string{"?..", "...", "=>", "?.", "..", "??", "==", "!=", "&&", "||", "<=", ">="} {
			if strings.HasPrefix(body[i:], op) {
				punctuation = op
				break
			}
		}
		tokens = append(tokens, dartToken{text: punctuation, line: line, kind: "punct", start: i, end: i + len(punctuation)})
		i += len(punctuation)
	}
	return tokens, valid
}

func dartStringEnd(body string, start, depth int) (int, bool) {
	if depth > 64 {
		return len(body), false
	}
	i := start
	raw := body[i] == 'r'
	if raw {
		i++
	}
	quote := body[i]
	delimiter := string(quote)
	if strings.HasPrefix(body[i:], strings.Repeat(delimiter, 3)) {
		delimiter = strings.Repeat(delimiter, 3)
	}
	i += len(delimiter)
	for i < len(body) {
		if strings.HasPrefix(body[i:], delimiter) {
			return i + len(delimiter), true
		}
		if !raw && body[i] == '\\' {
			i += 2
			continue
		}
		if !raw && strings.HasPrefix(body[i:], "${") {
			i += 2
			braces := 1
			for i < len(body) && braces > 0 {
				if body[i] == '\'' || body[i] == '"' || body[i] == 'r' && i+1 < len(body) && (body[i+1] == '\'' || body[i+1] == '"') {
					end, ok := dartStringEnd(body, i, depth+1)
					if !ok {
						return end, false
					}
					i = end
					continue
				}
				if strings.HasPrefix(body[i:], "/*") {
					i += 2
					n := 1
					for i < len(body) && n > 0 {
						if strings.HasPrefix(body[i:], "/*") {
							n++
							i += 2
						} else if strings.HasPrefix(body[i:], "*/") {
							n--
							i += 2
						} else {
							i++
						}
					}
					if n != 0 {
						return len(body), false
					}
					continue
				}
				if strings.HasPrefix(body[i:], "//") {
					for i < len(body) && body[i] != '\n' {
						i++
					}
					continue
				}
				if body[i] == '{' {
					braces++
				}
				if body[i] == '}' {
					braces--
				}
				i++
			}
			if braces != 0 {
				return len(body), false
			}
			continue
		}
		if len(delimiter) == 1 && (body[i] == '\n' || body[i] == '\r') {
			return i, false
		}
		i++
	}
	return len(body), false
}

func dartLiteral(token dartToken) (string, bool) {
	if token.kind != "string" {
		return "", false
	}
	text := token.text
	raw := strings.HasPrefix(text, "r")
	if raw {
		text = text[1:]
	}
	if len(text) < 2 {
		return "", false
	}
	delimiter := 1
	if len(text) >= 6 && strings.HasPrefix(text, strings.Repeat(text[:1], 3)) {
		delimiter = 3
	}
	value := text[delimiter : len(text)-delimiter]
	if !raw {
		var out strings.Builder
		for i := 0; i < len(value); i++ {
			if value[i] == '$' {
				return "", false
			}
			if value[i] != '\\' {
				out.WriteByte(value[i])
				continue
			}
			i++
			if i >= len(value) {
				return "", false
			}
			switch value[i] {
			case '\\', '\'', '"', '$':
				out.WriteByte(value[i])
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			default:
				return "", false
			}
		}
		value = out.String()
	}
	return value, true
}

func dartJoined(tokens []dartToken) string {
	var b strings.Builder
	for _, token := range tokens {
		b.WriteString(token.text)
	}
	return b.String()
}

func dartSplit(tokens []dartToken) [][]dartToken {
	var result [][]dartToken
	start, depth, angles := 0, 0, 0
	for i, t := range tokens {
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case "<":
			angles++
		case ">":
			if angles > 0 {
				angles--
			}
		case ",":
			if depth == 0 && angles == 0 {
				result = append(result, tokens[start:i])
				start = i + 1
			}
		}
	}
	if start < len(tokens) {
		result = append(result, tokens[start:])
	}
	return result
}

func dartPairs(tokens []dartToken) (map[int]int, bool) {
	pairs := map[int]int{}
	var stack []int
	valid := true
	for i, t := range tokens {
		if t.kind != "punct" {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			if len(stack) == 0 {
				valid = false
				continue
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			expected := map[string]string{")": "(", "]": "[", "}": "{"}[t.text]
			if tokens[open].text != expected {
				valid = false
				continue
			}
			pairs[open] = i
			pairs[i] = open
		}
	}
	return pairs, valid && len(stack) == 0
}
