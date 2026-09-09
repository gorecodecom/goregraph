package scan

import (
	"fmt"
	"strings"
	"testing"
)

func scriptLexicalBenchmarkBody(size int) string {
	var body strings.Builder
	body.WriteString("import { run, Input } from './api';\nimport * as api from './api';\n")
	for i := 0; i < size; i++ {
		fmt.Fprintf(&body, "export function task%d(value: Input) { const local = value; run(); api.run(); }\n", i)
		fmt.Fprintf(&body, "const shadow%d = (run: () => void): void => { run(); };\nrun();\n", i)
	}
	return body.String()
}

func BenchmarkExtractScriptSymbolFactsLexical(b *testing.B) {
	for _, size := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("declarations_%d", size), func(b *testing.B) {
			body := scriptLexicalBenchmarkBody(size)
			file := FileRecord{Path: "src/bundle.ts", Language: "typescript"}
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkScriptFacts = ExtractScriptSymbolFacts(file, body)
			}
		})
	}
}
