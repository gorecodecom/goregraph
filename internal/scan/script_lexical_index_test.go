package scan

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestImportedCallSurvivesUnrelatedShadowScope(t *testing.T) {
	file := FileRecord{Path: "src/view.ts", Language: "typescript"}
	facts := ExtractScriptSymbolFacts(file, "import { run } from './api';\nfunction nested(run: () => void) { run(); }\nrun();\n")
	for _, ref := range facts.References {
		if ref.Type == "calls_export" && ref.TargetExport == "run" && ref.TargetModule == "./api" && ref.Line == 3 {
			return
		}
	}
	t.Fatal("lost imported call outside the shadowing scope")
}

func TestScriptLexicalFactsPreserveSyntaxBoundaries(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "unrelated parameter scope",
			body: "import { run } from './api';\nfunction nested(run: () => void) { run(); }\nrun();\n",
			want: []string{"calls_export|2||run|true|true", "calls_export|3|./api|run|false|false"},
		},
		{
			name: "typed arrow",
			body: "import { run, Input } from './api';\nconst invoke = (run: Input): void => run();\nrun();\n",
			want: []string{"calls_export|2||run|true|false", "calls_export|3|./api|run|false|false", "type_reference|2|./api|Input|false|false"},
		},
		{
			name: "destructured alias and binding",
			body: "import { run } from './api';\n{ const { run: local } = source;\nrun(); }\n{ const { run } = source;\nrun(); }\nrun();\n",
			want: []string{"calls_export|3|./api|run|false|false", "calls_export|5||run|true|false", "calls_export|6|./api|run|false|false"},
		},
		{
			name: "namespace JSX comments and regex",
			body: "import { View, run } from './api';\nimport * as api from './api';\n// run(); api.run(); <View />\nconst pattern = /run\\(\\);/;\napi.run();\nconst view = <View />;\nrun();\n",
			want: []string{"calls_export|5|./api|run|false|false", "calls_export|7|./api|run|false|false", "renders_component|6|./api|View|false|false"},
		},
		{
			name: "unclosed shadow block",
			body: "import { run } from './api';\nfunction broken(run) {\nrun();\n",
			want: []string{"calls_export|3||run|true|false"},
		},
		{
			name: "crossed delimiters",
			body: "import { run } from './api';\n{[(])}\nrun();\n",
			want: []string{"calls_export|3|./api|run|false|false"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/view.tsx", Language: "typescript"}, test.body)
			var got []string
			for _, ref := range facts.References {
				if strings.HasPrefix(ref.Type, "imports_") {
					continue
				}
				got = append(got, fmt.Sprintf("%s|%d|%s|%s|%t|%t", ref.Type, ref.Line, ref.TargetModule, ref.TargetExport, strings.Contains(ref.Reason, "shadow"), ref.FromSymbolID != ""))
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("usage facts = %q, want %q", got, test.want)
			}
		})
	}
}

func TestScriptExtractionCanceledBeforeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	facts, err := ExtractScriptSymbolFactsContext(ctx, FileRecord{Path: "src/app.ts", Language: "typescript"}, "export function run() {}")
	if !errors.Is(err, context.Canceled) || len(facts.Declarations) != 0 || len(facts.References) != 0 {
		t.Fatalf("canceled extraction returned facts=%#v, err=%v", facts, err)
	}
}

type scriptCancelAfterChecks struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (ctx *scriptCancelAfterChecks) Err() error {
	ctx.remaining--
	if ctx.remaining == 0 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestScriptExtractionCancelsDuringBundle(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &scriptCancelAfterChecks{Context: base, cancel: cancel, remaining: 12}
	facts, err := ExtractScriptSymbolFactsContext(ctx, FileRecord{Path: "src/bundle.ts", Language: "typescript"}, scriptLexicalBenchmarkBody(1000))
	if !errors.Is(err, context.Canceled) || len(facts.Declarations) != 0 || len(facts.References) != 0 {
		t.Fatalf("mid-extraction cancellation returned facts=%#v, err=%v", facts, err)
	}
}

func TestScriptLexicalIndexTracksDelimiterAndLineBoundaries(t *testing.T) {
	index, err := newScriptLexicalIndex(context.Background(), "{\n(x)\n}\n}")
	if err != nil {
		t.Fatal(err)
	}
	if index.match(0) != 6 || index.match(2) != 4 || index.match(8) != -1 {
		t.Fatal("delimiter pairing escaped balanced ranges")
	}
	for offset, want := range []int{1, 1, 2, 2, 2, 2, 3, 3, 4, 4} {
		if got := index.lineAt(offset); got != want {
			t.Fatalf("line at %d = %d, want %d", offset, got, want)
		}
	}
}

func TestScriptDeepScopesKeepOutsideImportedCall(t *testing.T) {
	body := "import { run } from './api';\n" + strings.Repeat("{\n", 512) + "const run = replacement;\nrun();\n" + strings.Repeat("}\n", 512) + "run();\n"
	facts := ExtractScriptSymbolFacts(FileRecord{Path: "src/deep.ts", Language: "typescript"}, body)
	for _, ref := range facts.References {
		if ref.Type == "calls_export" && ref.Line == 1028 && ref.TargetModule == "./api" {
			return
		}
	}
	t.Fatal("deep scopes swallowed the module-level imported call")
}

func FuzzScriptLexicalIndexRanges(f *testing.F) {
	for _, body := range []string{"", "{[(])}", "import { run } from './api';\nrun();", "const a = (x): (() => T) => x();", "/* unterminated", "function a(x) { {{{", "0000000{00000}/"} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 8192 {
			t.Skip()
		}
		masked := maskScriptLexical(body)
		index, err := newScriptLexicalIndex(context.Background(), masked)
		if err != nil {
			t.Fatal(err)
		}
		for offset := range masked {
			paired := index.match(offset)
			if paired < -1 || paired >= len(masked) {
				t.Fatalf("pair %d at %d escapes input", paired, offset)
			}
			if paired >= 0 && index.match(paired) != offset {
				t.Fatalf("pair %d at %d is not reciprocal", paired, offset)
			}
		}
		file := FileRecord{Path: "src/fuzz.tsx", Language: "typescript"}
		first := ExtractScriptSymbolFacts(file, body)
		if second := ExtractScriptSymbolFacts(file, body); !reflect.DeepEqual(first, second) {
			t.Fatal("nondeterministic script facts")
		}
	})
}
