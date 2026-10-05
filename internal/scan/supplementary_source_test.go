package scan

import "testing"

func TestSupplementaryMarkupStylesAndFormContracts(t *testing.T) {
	files := []FileRecord{{Path: "web/index.html", Language: "html"}, {Path: "web/theme.css", Language: "css"}, {Path: "web/main.js", Language: "javascript"}}
	sources := []supplementarySource{{files[0], `<!-- <form action='/fake'> --><link rel="stylesheet" href="theme.css"><script src="main.js"></script><div id="board" class="board"><label for="email">Mail</label><input id="email"><form action="/api/contact" method="post"></form></div><script>const fake='<form action="/fake">';</script>`}, {files[1], `/* .fake {} */ @import 'base.css'; :root { --brand-color: #fff; } .board { color:var(--brand-color); background:url('logo.svg'); } @media (prefers-color-scheme: dark) { .board { color: black; } }`}}
	result := analyzeSupplementarySources(sources, files)
	if len(result.code.APIContracts) != 1 || result.code.APIContracts[0].Path != "/api/contact" || result.code.APIContracts[0].HTTPMethod != "POST" {
		t.Fatal(result.code.APIContracts)
	}
	kinds := map[string]bool{}
	for _, ref := range result.facts.References {
		kinds[ref.Type] = true
		if ref.Type == "imports_script" && !ref.Internal {
			t.Fatal(ref)
		}
	}
	for _, kind := range []string{"imports_stylesheet", "uses_custom_property", "references_element", "uses_style_class"} {
		if !kinds[kind] {
			t.Error("missing", kind, result.facts.References)
		}
	}
	properties := 0
	for _, symbol := range result.facts.Declarations {
		if symbol.Name == ".fake" {
			t.Fatal(symbol)
		}
		if symbol.Kind == "custom_property" {
			properties++
		}
	}
	if properties != 1 {
		t.Fatal("custom property missing", result.facts.Declarations)
	}
}

func TestSupplementaryBatchLiteralLabelsAndDynamicTargets(t *testing.T) {
	file := FileRecord{Path: "run.cmd", Language: "batch"}
	result := analyzeSupplementarySources([]supplementarySource{{file, "@echo off\nrem call :fake\ncall :build\ncall :%target%\ngoto :eof\n:build\ncall :finish\nexit /b\n:finish\nexit /b\n"}}, []FileRecord{file})
	if len(result.graph.Edges) != 2 {
		t.Fatal(result.graph.Edges)
	}
	for _, ref := range result.facts.References {
		if ref.To == "dynamic batch target" && ref.Resolution == SymbolResolutionExact {
			t.Fatal(ref)
		}
	}
	duplicate := analyzeSupplementarySources([]supplementarySource{{file, "call :work\n:work\n:work\n"}}, []FileRecord{file})
	if len(duplicate.graph.Edges) != 0 {
		t.Fatal("ambiguous label", duplicate.graph)
	}
}

func TestSupplementaryObjectiveCSelectorsAndCBridge(t *testing.T) {
	file := FileRecord{Path: "Bridge.m", Language: "objectivec"}
	body := `#import <Foundation/Foundation.h>
int ready(void) { return 1; }
@interface Service : NSObject
@property(nonatomic) NSString *name;
- (void)load:(int)count with:(NSString *)value;
@end
@implementation Service
- (void)load:(int)count with:(NSString *)value { ready(); }
- (void)run { [self load:1 with:@"example"]; [unknown load:1 with:@"example"]; }
@end`
	result := analyzeSupplementarySources([]supplementarySource{{file, body}}, []FileRecord{file})
	if len(result.graph.Edges) != 2 {
		t.Fatal(result.graph.Edges, result.facts.Declarations)
	}
	for _, edge := range result.graph.Edges {
		if edge.To.Method != "ready" && edge.To.Method != "load:with:" {
			t.Fatal(edge)
		}
	}
}
