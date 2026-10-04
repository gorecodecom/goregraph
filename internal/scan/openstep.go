package scan

import (
	"strconv"
	"strings"
	"unicode"
)

// parseOpenStep reads literal project metadata without invoking plutil or Xcode.
func parseOpenStep(body string) (map[string]any, bool) {
	var tokens []string
	for i := 0; i < len(body); {
		if unicode.IsSpace(rune(body[i])) {
			i++
			continue
		}
		if strings.HasPrefix(body[i:], "//") {
			if at := strings.IndexByte(body[i:], '\n'); at >= 0 {
				i += at + 1
				continue
			}
			break
		}
		if strings.HasPrefix(body[i:], "/*") {
			at := strings.Index(body[i+2:], "*/")
			if at < 0 {
				return nil, false
			}
			i += at + 4
			continue
		}
		if strings.ContainsRune("{}()=;,", rune(body[i])) {
			tokens = append(tokens, body[i:i+1])
			i++
			continue
		}
		begin := i
		if body[i] == '"' {
			i++
			for i < len(body) && body[i] != '"' {
				if body[i] == '\\' {
					i++
				}
				i++
			}
			if i >= len(body) {
				return nil, false
			}
			i++
			value, err := strconv.Unquote(body[begin:i])
			if err != nil {
				return nil, false
			}
			tokens = append(tokens, "\x00"+value)
			continue
		}
		for i < len(body) && !unicode.IsSpace(rune(body[i])) && !strings.ContainsRune("{}()=;,", rune(body[i])) {
			i++
		}
		if i == begin {
			return nil, false
		}
		tokens = append(tokens, body[begin:i])
	}
	at := 0
	var value func(int) (any, bool)
	value = func(depth int) (any, bool) {
		if at >= len(tokens) || depth > 64 {
			return nil, false
		}
		token := tokens[at]
		at++
		if token == "{" {
			result := map[string]any{}
			for at < len(tokens) && tokens[at] != "}" {
				key := strings.TrimPrefix(tokens[at], "\x00")
				at++
				if at >= len(tokens) || tokens[at] != "=" {
					return nil, false
				}
				at++
				item, ok := value(depth + 1)
				if !ok || at >= len(tokens) || tokens[at] != ";" {
					return nil, false
				}
				at++
				if _, exists := result[key]; exists {
					return nil, false
				}
				result[key] = item
			}
			if at >= len(tokens) {
				return nil, false
			}
			at++
			return result, true
		}
		if token == "(" {
			var result []any
			for at < len(tokens) && tokens[at] != ")" {
				item, ok := value(depth + 1)
				if !ok {
					return nil, false
				}
				result = append(result, item)
				if at < len(tokens) && tokens[at] == "," {
					at++
				} else if at >= len(tokens) || tokens[at] != ")" {
					return nil, false
				}
			}
			if at >= len(tokens) {
				return nil, false
			}
			at++
			return result, true
		}
		if strings.ContainsAny(token, "{}()=;,") && !strings.HasPrefix(token, "\x00") {
			return nil, false
		}
		return strings.TrimPrefix(token, "\x00"), true
	}
	parsed, ok := value(0)
	result, isMap := parsed.(map[string]any)
	return result, ok && isMap && at == len(tokens)
}

func plistString(value any) string      { text, _ := value.(string); return text }
func plistMap(value any) map[string]any { result, _ := value.(map[string]any); return result }
func plistList(value any) []any         { result, _ := value.([]any); return result }
