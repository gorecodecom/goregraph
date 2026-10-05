package scan

import "testing"

func TestCFamilyDeclarationsCallsAndHeaderVisibility(t *testing.T) {
	files := []FileRecord{{Path: "main.c", Language: "c"}, {Path: "value.h", Language: "c"}, {Path: "value.c", Language: "c"}}
	sources := []supplementarySource{{files[0], "#include \"value.h\"\nint run(void) { return value(1); }"}, {files[1], "int value(int count);"}, {files[2], "int value(int count) { return count; }"}}
	result := analyzeSupplementarySources(sources, files)
	if len(result.graph.Edges) != 1 || result.graph.Edges[0].To.File != "value.c" {
		t.Fatal(result.graph, result.facts.Declarations)
	}
}
func TestCFamilyPreprocessingOverloadsAndRawStringsRemainSafe(t *testing.T) {
	for _, body := range []string{
		"#if FEATURE\nint work(int n) {return n;}\nint run(void) {return work(1);}\n#endif",
		`int work(int n) {return n;} int work(double n) {return 1;} int run() {return work(1);}`,
		`const char *text = R"tag(class Fake { int work() {} })tag"; int run() { unknown(); }`,
	} {
		file := FileRecord{Path: "main.cpp", Language: "cpp"}
		result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
		if len(result.graph.Edges) != 0 {
			t.Fatal("unproven C++ call", result.graph.Edges)
		}
		for _, symbol := range result.facts.Declarations {
			if symbol.Name == "Fake" {
				t.Fatal("raw string fabricated a class")
			}
		}
	}
}
