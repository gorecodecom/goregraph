package agent

import (
	"strings"
	"testing"
)

func TestRenderScriptTestEvidenceVerifiesRegistrationTitle(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "cartStore.test.ts", "import { addLine } from './cartStore';\n"+
		"test(\"rejects invalid quantity\", () => {\n expect(() => addLine(0)).toThrow();\n});\n"+
		"// test(\"unrelated title\", () => { leak(); });\n")
	file, err := readSourceFile(root + "/cartStore.test.ts")
	if err != nil {
		t.Fatal(err)
	}
	candidate := sourceCandidate{Kind: "test", Name: "rejects invalid quantity", Path: "cartStore.test.ts", StartLine: 2, Role: "test"}
	section, err := renderSourceCandidate(candidate, file, "declaration_body")
	if err != nil || section.StartLine != 2 || section.EndLine != 4 || !strings.Contains(section.Content, "addLine(0)") {
		t.Fatalf("registered test body unavailable: %#v, %v", section, err)
	}
	candidate.Name = "unrelated title"
	if _, err := renderSourceCandidate(candidate, file, "declaration_body"); err == nil {
		t.Fatal("comment was accepted as an executable test")
	}
	candidate.Name = "old test title"
	if _, err := renderSourceCandidate(candidate, file, "declaration_body"); err == nil {
		t.Fatal("stale test title was accepted as current source")
	}
}

func TestRenderScriptTestEvidenceRejectsUncertainBoundaries(t *testing.T) {
	for _, source := range []string{
		"test('quantity', () => {\n const pattern = /[)]/;\n expect(pattern.test(')')).toBe(true);\n});",
		"test('quantity', () => first()); test('quantity', () => second());",
	} {
		candidate := sourceCandidate{Kind: "test", Name: "quantity", Path: "cart.test.ts", StartLine: 1}
		if _, err := renderScriptTestSource(candidate, sourceFile{Lines: strings.Split(source, "\n")}); err == nil {
			t.Fatalf("accepted uncertain registration: %s", source)
		}
	}
}
