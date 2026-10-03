package agent

import (
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextPrimaryQueryPreservesSourceIdentities(t *testing.T) {
	for _, test := range []struct{ query, want string }{
		{"Explain Service.Run and Extensions.Echo in C#. Include tests.", "Explain Service.Run and Extensions.Echo in C#"},
		{"Why does Sources/Service.swift fail? Include targets.", "Why does Sources/Service.swift fail"},
		{"Catalog items remain linked. Analyze persistence.", "Catalog items remain linked"},
	} {
		if got := contextPrimaryQuery(test.query); got != test.want {
			t.Fatalf("%q: got %q, want %q", test.query, got, test.want)
		}
	}
}

func TestContextSourceQualifiedAnchorsKeepRequestedOrderAndAmbiguity(t *testing.T) {
	facts := []scan.AgentContextFactRecord{
		{ID: "first", Kind: "symbol", Name: "Run", Qualified: "Sample.Service.Run()", File: "Service.cs", Confidence: "EXACT"},
		{ID: "second", Kind: "symbol", Name: "Echo", Qualified: "Sample.Extensions.Echo(T)", File: "Service.cs", Confidence: "EXACT"},
		{ID: "other", Kind: "symbol", Name: "Run", Qualified: "Sample.Other.Run()", File: "Service.cs", Confidence: "EXACT"},
	}
	query := "Explain Service.Run and Extensions.Echo in C#"
	ranked := rankContextFacts(facts, query)
	if ranked[0].fact.ID != "first" || ranked[0].exactClass != 2 {
		t.Fatal(ranked)
	}
	facts = append(facts, scan.AgentContextFactRecord{ID: "overload", Kind: "symbol", Name: "Run", Qualified: "Sample.Service.Run(int)", File: "Service.cs", Confidence: "EXACT"})
	for _, candidate := range rankContextFacts(facts, query) {
		if (candidate.fact.ID == "first" || candidate.fact.ID == "overload") && candidate.exactClass > 0 {
			t.Fatal("short anchor selected an overload", candidate)
		}
	}
	facts = append(facts, scan.AgentContextFactRecord{ID: "module", Kind: "symbol", Name: "Echo", Qualified: "Other.Extensions.Echo(T)", File: "Other.cs", Confidence: "EXACT"})
	if got := contextUniqueLanguageAnchors(facts, contextQueryAnchors(query)); len(got) != 0 {
		t.Fatal("short anchor selected a conflicting module", got)
	}
}

func TestCSharpGenericInlineDeclarationCanBeRendered(t *testing.T) {
	body := "public static class Extensions { public static T Echo<T>(this T value) where T:class => value; }"
	file := sourceFile{Path: "Extensions.cs", Lines: strings.Split(body, "\n")}
	section, err := renderSourceCandidate(sourceCandidate{Path: file.Path, Name: "Echo", Qualified: "Sample.Extensions.Echo(T)", Kind: "symbol", StartLine: 1}, file, "declaration_body")
	if err != nil || !strings.Contains(section.Content, "Echo<T>") {
		t.Fatal(err, section)
	}
	call := "var result = receiver.Echo<string>(value);"
	start := strings.Index(call, "Echo")
	if declarationLikeOccurrence("Service.cs", call, start, start+len("Echo")) {
		t.Fatal("generic invocation was treated as a declaration")
	}
}

func TestTypedSourceBodyDoesNotAttachAFollowingDeclarationOrDefaultClosure(t *testing.T) {
	for _, test := range []struct{ file, body, name, want string }{
		{"Record.cs", "public record Counter(int Count);\npublic class Following {void Run(){}}", "Counter", ""},
		{"Service.swift", "func run(_ body: () -> Void = {\n print(\"default\")\n}) {\n print(\"actual\")\n}", "run", "actual"},
	} {
		file := sourceFile{Path: test.file, Lines: strings.Split(test.body, "\n")}
		section, err := renderSourceCandidate(sourceCandidate{Path: test.file, Name: test.name, Kind: "symbol", StartLine: 1}, file, "declaration_body")
		if test.want == "" {
			if err == nil {
				t.Fatal("record was assigned the following class body", section)
			}
		} else if err != nil || !strings.Contains(section.Content, test.want) {
			t.Fatal("default closure replaced the actual function body", err, section)
		}
	}
}
