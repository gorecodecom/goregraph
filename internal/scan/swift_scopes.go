package scan

type swiftLocalScope struct {
	variables, urls, requests, methods map[string]string
}

func swiftCopy(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func swiftClosureShadows(tokens []csharpToken, start, end int, variables map[string]string) {
	stop := start
	for stop < end && stop < start+24 {
		if tokens[stop].text == "in" {
			for _, token := range tokens[start:stop] {
				if token.kind == "identifier" {
					variables[token.text] = ""
				}
			}
			return
		}
		switch tokens[stop].text {
		case "{", "}", "=", ";", "let", "var", "return", "for", "if", "guard", "switch":
			return
		}
		stop++
	}
}
