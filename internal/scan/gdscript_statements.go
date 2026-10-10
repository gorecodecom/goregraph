package scan

// gdscriptTopLevelStatements records declaration starts and block ends from
// logical lines, so delimiters at column zero cannot become declarations.
func gdscriptTopLevelStatements(tokens []godotToken) map[int]int {
	statements := map[int]int{}
	rows := gdscriptScopeRows(tokens, 0, len(tokens))
	for _, row := range rows {
		if row.indent != 0 {
			continue
		}
		at := row.start
		for at < row.sameEnd && tokens[at].kind == "punctuation" && tokens[at].text == "@" {
			if at+1 >= row.sameEnd || tokens[at+1].kind != "identifier" {
				break
			}
			at += 2
			if at < row.sameEnd && tokens[at].kind == "punctuation" && tokens[at].text == "(" {
				depth := 0
				for at < row.sameEnd {
					t := tokens[at]
					at++
					if t.kind == "punctuation" {
						if t.text == "(" {
							depth++
						} else if t.text == ")" {
							depth--
						}
					}
					if depth == 0 {
						break
					}
				}
			}
		}
		if at >= row.sameEnd {
			continue
		}
		statements[at] = row.sameEnd
		if tokens[at].kind == "identifier" && tokens[at].text == "static" && at+1 < row.sameEnd {
			statements[at+1] = row.sameEnd
		}
	}
	return statements
}
