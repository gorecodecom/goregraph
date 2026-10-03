package scan

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type csharpToken struct {
	text string
	line int
	kind string
}

// csharpTokens excludes comments and keeps literals opaque to code extraction.
func csharpTokens(body string) []csharpToken {
	var tokens []csharpToken
	line := 1
	for i := 0; i < len(body); {
		start, startLine := i, line
		r, size := utf8.DecodeRuneInString(body[i:])
		if unicode.IsSpace(r) {
			if r == '\n' {
				line++
			}
			i += size
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
			for i < len(body) && !strings.HasPrefix(body[i:], "*/") {
				if body[i] == '\n' {
					line++
				}
				i++
			}
			if i < len(body) {
				i += 2
			}
			continue
		}
		prefix := i
		for prefix < len(body) && (body[prefix] == '$' || body[prefix] == '@') {
			prefix++
		}
		if prefix < len(body) && (body[prefix] == '"' || prefix == i && body[i] == '\'') {
			quote := body[prefix]
			verbatim := strings.Contains(body[i:prefix], "@")
			interpolated := strings.Contains(body[i:prefix], "$")
			raw := 0
			if quote == '"' {
				for j := prefix; j < len(body) && body[j] == '"'; j++ {
					raw++
				}
			}
			if raw < 3 {
				raw = 1
			}
			i = prefix + raw
			contentStart := i
			for i < len(body) {
				if interpolated && raw == 1 && body[i] == '{' {
					if i+1 < len(body) && body[i+1] == '{' {
						i += 2
						continue
					}
					i = skipLiteralExpression(body, i+1, '{', '}', &line)
					continue
				}
				if raw >= 3 && strings.HasPrefix(body[i:], strings.Repeat("\"", raw)) {
					break
				}
				if raw == 1 && body[i] == quote {
					if verbatim && i+1 < len(body) && body[i+1] == quote {
						i += 2
						continue
					}
					break
				}
				if body[i] == '\n' {
					line++
				}
				if raw == 1 && !verbatim && body[i] == '\\' && i+1 < len(body) {
					i += 2
				} else {
					i++
				}
			}
			value := body[contentStart:i]
			if raw == 1 && !verbatim && quote == '"' {
				if decoded, err := strconv.Unquote("\"" + value + "\""); err == nil {
					value = decoded
				}
			}
			if verbatim {
				value = strings.ReplaceAll(value, "\"\"", "\"")
			}
			kind := "literal"
			if quote == '\'' {
				kind = "char_literal"
			}
			if interpolated || i == len(body) {
				kind = "dynamic_literal"
			}
			tokens = append(tokens, csharpToken{value, startLine, kind})
			if i < len(body) {
				i += raw
			}
			continue
		}
		if unicode.IsLetter(r) || r == '_' || r == '@' {
			i += size
			for i < len(body) {
				next, n := utf8.DecodeRuneInString(body[i:])
				if !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_' {
					break
				}
				i += n
			}
			tokens = append(tokens, csharpToken{strings.TrimPrefix(body[start:i], "@"), startLine, "identifier"})
			continue
		}
		if unicode.IsDigit(r) {
			i += size
			for i < len(body) && (body[i] >= '0' && body[i] <= '9' || body[i] == '.') {
				i++
			}
			tokens = append(tokens, csharpToken{body[start:i], startLine, "number"})
			continue
		}
		i += size
		value := string(r)
		if i < len(body) && (value == "=" && body[i] == '>' || value == "?" && body[i] == '.' || value == ":" && body[i] == ':') {
			value += body[i : i+1]
			i++
		}
		tokens = append(tokens, csharpToken{value, startLine, "punctuation"})
	}
	return tokens
}

func csharpPairs(tokens []csharpToken) (map[int]int, bool) {
	pairs := map[int]int{}
	var stack []int
	for i, token := range tokens {
		if token.kind != "punctuation" {
			continue
		}
		switch token.text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			if len(stack) == 0 {
				return pairs, false
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if tokens[open].text+token.text != "()" && tokens[open].text+token.text != "[]" && tokens[open].text+token.text != "{}" {
				return pairs, false
			}
			pairs[open], pairs[i] = i, open
		}
	}
	return pairs, len(stack) == 0
}

func csharpJoined(tokens []csharpToken) string {
	var b strings.Builder
	for _, token := range tokens {
		b.WriteString(token.text)
	}
	return b.String()
}
