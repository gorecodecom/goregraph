package agent

import (
	"strings"
	"testing"
)

func TestDartSourceRenderingVerifiesNamedParametersAndArrowBodies(t *testing.T) {
	body := "class API {\n Future<void> load({required String id}) async {\n final text=\"${render('}')}\";\n }\n int count() => 1;\n}\n"
	file := sourceFile{Path: "lib/api.dart", Lines: strings.Split(body, "\n")}
	for _, test := range []struct {
		name      string
		line, end int
	}{{"load", 2, 4}, {"count", 5, 5}} {
		section, err := renderSourceCandidate(sourceCandidate{Path: "lib/api.dart", Name: test.name, Kind: "symbol", StartLine: test.line}, file, "body")
		if err != nil || section.StartLine != test.line || section.EndLine != test.end {
			t.Fatalf("%s: %#v %v", test.name, section, err)
		}
	}
}
func TestDartSourceRenderingRejectsFakeDeclarationsAndAmbiguity(t *testing.T) {
	for _, body := range []string{`final text = 'void load() {}';`, `/* outer /* nested */ void load() {} */ class Empty {}`, `class Broken { void load() {`, `class A { void load() {} } class B { void load() {} }`} {
		_, err := renderSourceCandidate(sourceCandidate{Path: "lib/api.dart", Name: "load", Kind: "symbol", StartLine: 1}, sourceFile{Path: "lib/api.dart", Lines: strings.Split(body, "\n")}, "body")
		if err == nil {
			t.Fatal("unproven declaration rendered", body)
		}
	}
}
func TestDartWidgetTestSourceIsNavigable(t *testing.T) {
	body := "import 'package:flutter_test/flutter_test.dart';\nvoid main() {\n testWidgets('shows board', (tester) async {\n await tester.pumpWidget(Board());\n });\n}\n"
	section, err := renderSourceCandidate(sourceCandidate{Path: "test/widget_test.dart", Name: "shows board", Kind: "test", StartLine: 3}, sourceFile{Path: "test/widget_test.dart", Lines: strings.Split(body, "\n")}, "body")
	if err != nil || section.StartLine != 3 || section.EndLine != 5 {
		t.Fatal(section, err)
	}
}
