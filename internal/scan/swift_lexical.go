package scan

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// swiftTokens keeps comments and interpolated strings out of structural analysis.
func swiftTokens(body string) []csharpToken {
	var tokens []csharpToken
	line := 1
	for i := 0; i < len(body); {
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
			depth := 1
			i += 2
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
			continue
		}
		start, startLine := i, line
		prefix := i
		for prefix < len(body) && body[prefix] == '#' {
			prefix++
		}
		if prefix < len(body) && body[prefix] == '/' && (prefix > i || swiftRegexPosition(tokens)) {
			savedLine := line
			if stop, ok := swiftRegexLiteral(body, i, prefix, &line); ok {
				tokens = append(tokens, csharpToken{body[i:stop], startLine, "regex_literal"})
				i = stop
				continue
			}
			line = savedLine
		}
		if prefix < len(body) && body[prefix] == '"' {
			hashes := prefix - i
			quotes := 1
			if strings.HasPrefix(body[prefix:], `"""`) {
				quotes = 3
			}
			closing := strings.Repeat(`"`, quotes) + strings.Repeat("#", hashes)
			i = prefix + quotes
			contentStart := i
			for i < len(body) && !strings.HasPrefix(body[i:], closing) {
				marker := "\\" + strings.Repeat("#", hashes) + "("
				if strings.HasPrefix(body[i:], marker) {
					i = skipLiteralExpression(body, i+len(marker), '(', ')', &line)
					continue
				}

				if body[i] == '\n' {
					line++
				}
				if hashes == 0 && quotes == 1 && body[i] == '\\' && i+1 < len(body) {
					i += 2
				} else {
					i++
				}
			}
			value := body[contentStart:i]
			kind := "literal"
			if strings.Contains(value, "\\"+strings.Repeat("#", hashes)+"(") || i == len(body) {
				kind = "dynamic_literal"
			}
			if hashes == 0 && quotes == 1 && kind == "literal" {
				if decoded, err := strconv.Unquote(`"` + value + `"`); err == nil {
					value = decoded
				}
			}
			tokens = append(tokens, csharpToken{value, startLine, kind})
			if i < len(body) {
				i += len(closing)
			}
			continue
		}
		if r == '`' {
			i++
			for i < len(body) && body[i] != '`' && body[i] != '\n' {
				i++
			}
			tokens = append(tokens, csharpToken{body[start+1 : i], startLine, "identifier"})
			if i < len(body) && body[i] == '`' {
				i++
			}
			continue
		}
		if unicode.IsLetter(r) || r == '_' || r == '$' {
			i += size
			for i < len(body) {
				next, n := utf8.DecodeRuneInString(body[i:])
				if !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_' {
					break
				}
				i += n
			}
			tokens = append(tokens, csharpToken{body[start:i], startLine, "identifier"})
			continue
		}
		if unicode.IsDigit(r) {
			i += size
			for i < len(body) && (body[i] >= '0' && body[i] <= '9' || body[i] == '_') {
				i++
			}
			tokens = append(tokens, csharpToken{body[start:i], startLine, "number"})
			continue
		}
		i += size
		value := string(r)
		if i < len(body) && (value == "-" && body[i] == '>' || value == "?" && body[i] == '.') {
			value += body[i : i+1]
			i++
		}
		tokens = append(tokens, csharpToken{value, startLine, "punctuation"})
	}
	return tokens
}
