package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestRenderSourceCandidateKeepsCurrentIndexedDeclaration(t *testing.T) {
	lines := numberedSourceLines(12)
	lines[9] = "    public void deleteUser() {"
	lines[10] = "        repository.delete();"
	lines[11] = "    }"
	candidate := sourceCandidate{
		Project: "users", Path: "src/UserService.java", StartLine: 10, EndLine: 12,
		Role: "entrypoint", Kind: "symbol", Name: "deleteUser",
	}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.Project != candidate.Project || section.Path != candidate.Path || section.Role != candidate.Role {
		t.Fatalf("rendered metadata = %#v", section)
	}
	if section.StartLine != 10 || section.EndLine != 12 || section.RenderMode != "body" || section.SourceState != "indexed_range_current" {
		t.Fatalf("rendered range = %#v", section)
	}
	if section.Content != "10\t    public void deleteUser() {\n11\t        repository.delete();\n12\t    }" {
		t.Fatalf("rendered content:\n%s", section.Content)
	}
}

func TestRenderSourceCandidateRelocatesUniqueDeclaration(t *testing.T) {
	lines := []string{
		"package users;",
		"",
		"// newly inserted line",
		"// newly inserted line",
		"public void deleteUser() {",
		"    repository.delete();",
		"}",
	}
	candidate := sourceCandidate{
		Path: "UserService.java", StartLine: 2, EndLine: 4,
		Role: "call_chain", Kind: "symbol", Name: "deleteUser",
	}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 5 || section.EndLine != 7 || section.SourceState != "relocated_current" {
		t.Fatalf("relocated range = %#v", section)
	}
}

func TestRenderSourceCandidateRelocatedEndpointUsesDefaultBodyWindow(t *testing.T) {
	lines := numberedSourceLines(35)
	lines[0] = "@DeleteMapping(\"/users/{id}\")"
	lines[1] = "public void deleteUser() {"
	lines[2] = "    service.deleteUser();"
	lines[3] = "}"
	candidate := sourceCandidate{
		Path: "UserController.java", StartLine: 1, EndLine: 1,
		Kind: "api_endpoint", Name: "DELETE /users/{id}", Qualified: "UserController.deleteUser",
	}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 2 || section.EndLine != 30 {
		t.Fatalf("relocated endpoint range = %d-%d, want 2-30", section.StartLine, section.EndLine)
	}
	if !strings.Contains(section.Content, "2\tpublic void deleteUser() {") ||
		!strings.Contains(section.Content, "3\t    service.deleteUser();") {
		t.Fatalf("relocated endpoint content:\n%s", section.Content)
	}
}

func TestRenderSourceCandidateRejectsAbsentIdentifier(t *testing.T) {
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 1, EndLine: 2, Kind: "symbol", Name: "deleteUser"}
	_, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{
		"public void createUser() {}",
		"public void deleteUsers() {}",
	}}, "body")
	if err == nil || err.Error() != "indexed symbol is absent from current source" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
	}
}

func TestRenderSourceCandidateRejectsAmbiguousDeclaration(t *testing.T) {
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser"}
	_, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{
		"public void unrelated() {}",
		"public void deleteUser() {}",
		"public void deleteUser(boolean force) {}",
	}}, "body")
	if err == nil || err.Error() != "indexed symbol is ambiguous in current source" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
	}
}

func TestRenderSourceCandidateRejectsOldCallAfterDeclarationRename(t *testing.T) {
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 1, EndLine: 3, Kind: "symbol", Name: "deleteUser"}
	_, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{
		"public void removeUser() {",
		"    deleteUser();",
		"}",
	}}, "body")
	if err == nil || err.Error() != "indexed symbol has no unique declaration-like occurrence" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
	}
}

func TestRenderSourceCandidateRejectsAmbiguousCallPrefixes(t *testing.T) {
	candidate := sourceCandidate{Path: "source", StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser"}
	for _, line := range []string{
		"go deleteUser()",
		"defer deleteUser()",
		"echo deleteUser();",
		"void deleteUser()",
		"not deleteUser()",
		"sizeof deleteUser()",
	} {
		t.Run(line, func(t *testing.T) {
			_, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{line}}, "body")
			if err == nil || err.Error() != "indexed symbol has no unique declaration-like occurrence" {
				t.Fatalf("renderSourceCandidate() error = %v", err)
			}
		})
	}
}

func TestRenderSourceCandidateAcceptsConservativeCallableDeclarations(t *testing.T) {
	candidate := sourceCandidate{Path: "source", StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser"}
	for _, line := range []string{
		"public deleteUser() {}",
		"public void deleteUser() {}",
		"static int deleteUser(void) {",
		"protected Task deleteUser() => repository.Delete();",
	} {
		t.Run(line, func(t *testing.T) {
			section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{line}}, "body")
			if err != nil {
				t.Fatal(err)
			}
			if section.Content != "1\t"+line {
				t.Fatalf("rendered content = %q", section.Content)
			}
		})
	}
}

func TestRenderSourceCandidateAcceptsPackagePrivateCStyleDeclarations(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		line string
	}{
		{name: "Java void method", path: "UserService.java", line: "void deleteUser() {}"},
		{name: "Java typed method", path: "UserService.java", line: "Task deleteUser() {}"},
		{name: "Java interface method", path: "UserRepository.java", line: "void deleteUser();"},
		{name: "C# typed method", path: "UserService.cs", line: "Task deleteUser() => repository.Delete();"},
		{name: "Java generic return", path: "UserService.java", line: "Result<User> deleteUser() {}"},
		{name: "C++ pointer return", path: "user.cpp", line: "User* deleteUser() {}"},
		{name: "C array return", path: "user.c", line: "user_result[] deleteUser() {}"},
		{name: "C primitive return", path: "user.c", line: "unsigned long deleteUser(void) {"},
		{name: "C tagged pointer return", path: "user.c", line: "struct User * deleteUser(void) {"},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := sourceCandidate{
				Path: test.path, StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser",
			}
			section, err := renderSourceCandidate(
				candidate,
				sourceFile{Path: candidate.Path, Lines: []string{test.line}},
				"body",
			)
			if err != nil {
				t.Fatal(err)
			}
			if section.Content != "1\t"+test.line {
				t.Fatalf("rendered content = %q", section.Content)
			}
		})
	}
}

func TestRenderSourceCandidateRejectsCStyleExpressionPrefixes(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		line string
	}{
		{name: "C# await", path: "UserService.cs", line: "await deleteUser()"},
		{name: "Java new", path: "UserService.java", line: "new deleteUser()"},
		{name: "C# new", path: "UserService.cs", line: "new deleteUser()"},
		{name: "C sizeof", path: "user.c", line: "sizeof deleteUser()"},
		{name: "C++ alignof", path: "user.cpp", line: "alignof deleteUser()"},
		{name: "C++ co_await", path: "user.cpp", line: "co_await deleteUser()"},
		{name: "C++ co_yield", path: "user.cpp", line: "co_yield deleteUser()"},
		{name: "C++ co_return", path: "user.cpp", line: "co_return deleteUser()"},
		{name: "Java comparison", path: "UserService.java", line: "count < limit > deleteUser()"},
		{name: "C# expression", path: "UserService.cs", line: "left * right deleteUser()"},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := sourceCandidate{
				Path: test.path, StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser",
			}
			_, err := renderSourceCandidate(
				candidate,
				sourceFile{Path: candidate.Path, Lines: []string{test.line}},
				"body",
			)
			if err == nil || err.Error() != "indexed symbol has no unique declaration-like occurrence" {
				t.Fatalf("renderSourceCandidate() error = %v", err)
			}
		})
	}
}

func TestRenderSourceCandidateRejectsJavaScriptVoidCall(t *testing.T) {
	candidate := sourceCandidate{
		Path: "module.js", StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser",
	}
	_, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: []string{"void deleteUser()"}},
		"body",
	)
	if err == nil || err.Error() != "indexed symbol has no unique declaration-like occurrence" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
	}
}

func TestRenderSourceCandidateKeepsIndexedConstructorOverClassDeclaration(t *testing.T) {
	lines := []string{
		"public class UserService {",
		"    private final Repository repository;",
		"",
		"    @Inject",
		"    public UserService() {",
		"    }",
		"}",
	}
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 5, EndLine: 6, Kind: "symbol", Name: "UserService"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 5 || section.EndLine != 6 || section.SourceState != "indexed_range_current" {
		t.Fatalf("constructor section = %#v", section)
	}
}

func TestRenderSourceCandidateRejectsIdentifiersOutsideCode(t *testing.T) {
	candidate := sourceCandidate{Path: "source", StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser"}
	for _, line := range []string{
		`print("def deleteUser")`,
		"x(); // class deleteUser",
	} {
		t.Run(line, func(t *testing.T) {
			_, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{line}}, "body")
			if err == nil || err.Error() != "indexed symbol has no unique declaration-like occurrence" {
				t.Fatalf("renderSourceCandidate() error = %v", err)
			}
		})
	}
}

func TestRenderSourceCandidateIgnoresBlockCommentDeclarationDuringRelocation(t *testing.T) {
	lines := []string{
		"/*",
		"public void deleteUser() {",
		"}",
		"*/",
		"public void deleteUser() {",
	}
	candidate := sourceCandidate{Path: "source", StartLine: 2, EndLine: 2, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 5 || section.EndLine != 5 || section.SourceState != "relocated_current" {
		t.Fatalf("relocated section = %#v", section)
	}
}

func TestRenderSourceCandidateIgnoresMultilineStringDeclarationDuringRelocation(t *testing.T) {
	lines := []string{
		`message = """`,
		"def deleteUser():",
		`"""`,
		"",
		"def deleteUser():",
	}
	candidate := sourceCandidate{Path: "source.py", StartLine: 2, EndLine: 2, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 5 || section.EndLine != 5 || section.SourceState != "relocated_current" {
		t.Fatalf("relocated section = %#v", section)
	}
}

func TestRenderSourceCandidateIgnoresRegexDeclarationDuringRelocation(t *testing.T) {
	lines := []string{
		"/function deleteUser/.test(input)",
		"",
		"export function deleteUser() {}",
	}
	candidate := sourceCandidate{Path: "module.ts", StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 3 || section.EndLine != 3 || section.SourceState != "relocated_current" {
		t.Fatalf("relocated section = %#v", section)
	}
}

func TestRenderSourceCandidateExtractsQualifiedIdentifiers(t *testing.T) {
	tests := []struct {
		name      string
		candidate sourceCandidate
		line      string
	}{
		{
			name:      "Java owner method",
			candidate: sourceCandidate{Kind: "route", Name: "DELETE /users/{id}", Qualified: "Owner.deleteUser"},
			line:      "public void deleteUser() {}",
		},
		{
			name:      "TypeScript module method",
			candidate: sourceCandidate{Kind: "api_endpoint", Name: "DELETE /users/:id", Qualified: "src/module#deleteUser"},
			line:      "export function deleteUser() {}",
		},
		{
			name:      "PHP owner method",
			candidate: sourceCandidate{Kind: "route", Name: "DELETE /users/{id}", Qualified: "Owner::deleteUser"},
			line:      "public function deleteUser() {}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.candidate.Path = "source"
			test.candidate.StartLine = 1
			test.candidate.EndLine = 1
			section, err := renderSourceCandidate(test.candidate, sourceFile{Path: "source", Lines: []string{test.line}}, "body")
			if err != nil {
				t.Fatal(err)
			}
			if section.Content != "1\t"+test.line {
				t.Fatalf("rendered content = %q", section.Content)
			}
		})
	}
}

func TestRenderSourceCandidatePrefersIndexedName(t *testing.T) {
	candidate := sourceCandidate{
		Path: "module.ts", StartLine: 1, EndLine: 1, Kind: "symbol",
		Name: "deleteUser", Qualified: "src/module#differentName",
	}
	if _, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{
		"export function deleteUser() {}",
	}}, "body"); err != nil {
		t.Fatal(err)
	}
}

func TestRenderSourceCandidateDoesNotTreatSameLineCallAsDeclaration(t *testing.T) {
	candidate := sourceCandidate{Path: "module.ts", StartLine: 1, EndLine: 1, Kind: "symbol", Name: "deleteUser"}
	line := "export function deleteUser() { deleteUser(); }"

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: []string{line}}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.Content != "1\t"+line {
		t.Fatalf("rendered content = %q", section.Content)
	}
}

func TestRenderSourceCandidateBodyUnavailableOver120Lines(t *testing.T) {
	lines := numberedSourceLines(121)
	lines[0] = "public void deleteUser() {"
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 1, EndLine: 121, Kind: "symbol", Name: "deleteUser"}

	if _, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body"); err == nil {
		t.Fatal("renderSourceCandidate() accepted a body over 120 lines")
	}
}

func TestRenderSourceCandidateFocusedHasAtMost61NumberedLines(t *testing.T) {
	lines := numberedSourceLines(100)
	lines[49] = "public void deleteUser() {"
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 40, EndLine: 90, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "focused")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 22 || section.EndLine != 82 || section.RenderMode != "focused" {
		t.Fatalf("focused range = %#v", section)
	}
	renderedLines := strings.Split(section.Content, "\n")
	if len(renderedLines) != 61 {
		t.Fatalf("focused line count = %d, want 61", len(renderedLines))
	}
	for index, line := range renderedLines {
		wantPrefix := fmt.Sprintf("%d\t", section.StartLine+index)
		if !strings.HasPrefix(line, wantPrefix) {
			t.Fatalf("rendered line %d = %q, want prefix %q", index, line, wantPrefix)
		}
	}
}

func TestRenderSourceCandidateSignatureIncludesAnnotationsWithin12Lines(t *testing.T) {
	lines := make([]string, 0, 12)
	for index := 1; index <= 10; index++ {
		lines = append(lines, fmt.Sprintf("@Annotation%d", index))
	}
	lines = append(lines, "public void deleteUser(", ") {")
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 11, EndLine: 12, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "signature")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 1 || section.EndLine != 12 || section.RenderMode != "signature" {
		t.Fatalf("signature range = %#v", section)
	}
	renderedLines := strings.Split(section.Content, "\n")
	if len(renderedLines) != 12 {
		t.Fatalf("signature line count = %d, want 12", len(renderedLines))
	}
	for index, line := range renderedLines {
		wantPrefix := fmt.Sprintf("%d\t", index+1)
		if !strings.HasPrefix(line, wantPrefix) {
			t.Fatalf("rendered line %d = %q, want prefix %q", index, line, wantPrefix)
		}
	}
}

func TestRenderSourceCandidateSignatureIncludesMultilineAnnotation(t *testing.T) {
	lines := []string{
		"@DeleteMapping(",
		"    path = \"/users/{id}\"",
		")",
		"public void deleteUser() {",
	}
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 4, EndLine: 4, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "signature")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 1 || section.EndLine != 4 {
		t.Fatalf("signature range = %d-%d, want 1-4", section.StartLine, section.EndLine)
	}
}

func TestRenderSourceCandidateSignatureIgnoresNestedParameterTerminators(t *testing.T) {
	lines := []string{
		"def deleteUser(",
		"    user_id: str,",
		`    reason: str = "audit:manual",`,
		"):",
		"    pass",
	}
	candidate := sourceCandidate{Path: "service.py", StartLine: 1, EndLine: 5, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "signature")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 1 || section.EndLine != 4 {
		t.Fatalf("signature range = %d-%d, want 1-4", section.StartLine, section.EndLine)
	}
	if !strings.Contains(section.Content, "4\t):") {
		t.Fatalf("signature content:\n%s", section.Content)
	}
}

func TestRenderSourceCandidateSignatureUnavailableWithoutTerminator(t *testing.T) {
	lines := []string{"public void deleteUser("}
	lines = append(lines, numberedSourceLines(12)...)
	candidate := sourceCandidate{Path: "UserService.java", StartLine: 1, EndLine: len(lines), Kind: "symbol", Name: "deleteUser"}

	if _, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "signature"); err == nil {
		t.Fatal("renderSourceCandidate() accepted a signature without a terminator within 12 lines")
	}
}

func TestRenderSourceCandidateMissingEndLineUsesDeclarationPlus28Lines(t *testing.T) {
	lines := numberedSourceLines(40)
	lines[4] = "func deleteUser() {"
	candidate := sourceCandidate{Path: "service.go", StartLine: 5, Kind: "symbol", Name: "deleteUser"}

	section, err := renderSourceCandidate(candidate, sourceFile{Path: candidate.Path, Lines: lines}, "body")
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 5 || section.EndLine != 33 {
		t.Fatalf("default range = %d-%d, want 5-33", section.StartLine, section.EndLine)
	}
}

func numberedSourceLines(count int) []string {
	lines := make([]string, count)
	for index := range lines {
		lines[index] = fmt.Sprintf("source line %d", index+1)
	}
	return lines
}

func TestContextSourceOptionsMergeNearbyRangesDeterministically(t *testing.T) {
	pack := ContextPack{
		Query:                 "inspect production path",
		Entrypoints:           []ContextLocation{{ID: "entry"}},
		selectedSourceFactIDs: []string{"second", "entry"},
	}
	facts := []scan.AgentContextFactRecord{
		{ID: "entry", Project: "app", Kind: "symbol", Name: "entry", File: "app.go", Line: 2, EndLine: 3},
		{ID: "second", Project: "app", Kind: "symbol", Name: "second", File: "app.go", Line: 11, EndLine: 12},
	}

	forward := contextSourceCandidates(pack, scan.AgentContextIndexRecord{Facts: facts})
	pack.selectedSourceFactIDs[0], pack.selectedSourceFactIDs[1] = pack.selectedSourceFactIDs[1], pack.selectedSourceFactIDs[0]
	facts[0], facts[1] = facts[1], facts[0]
	reversed := contextSourceCandidates(pack, scan.AgentContextIndexRecord{Facts: facts})

	if len(forward) != 1 || len(reversed) != 1 {
		t.Fatalf("nearby candidates were not merged: forward=%#v reversed=%#v", forward, reversed)
	}
	if forward[0].FactID != reversed[0].FactID || forward[0].StartLine != 2 || forward[0].EndLine != 12 ||
		reversed[0].StartLine != forward[0].StartLine || reversed[0].EndLine != forward[0].EndLine {
		t.Fatalf("nearby merge depends on input order: forward=%#v reversed=%#v", forward, reversed)
	}
}

func TestContextSourceOptionsEvaluateEveryFittingRenderMode(t *testing.T) {
	root := t.TempDir()
	lines := make([]string, 110)
	for index := range lines {
		lines[index] = "    total = total + calculateAnotherValue()"
	}
	lines[0] = "func centralOperation() {"
	lines[len(lines)-1] = "}"
	writeSourceFile(t, root, "central.go", strings.Join(lines, "\n")+"\n")
	pack := ContextPack{
		Schema: 1, Query: "central operation", Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
		Concerns:              []ContextConcern{{Kind: contextConcernEntrypoint}},
		Entrypoints:           []ContextLocation{{ID: "central", File: "central.go"}},
		selectedSourceFactIDs: []string{"central"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "central", Kind: "symbol", Name: "centralOperation", File: "central.go", Line: 1, EndLine: 110,
	}}}
	loaded := loadedContextIndex{ScopeRoot: root, Index: index}
	candidates := contextSourceCandidates(pack, index)
	options, _, err := contextSourceRenderOptions(
		pack,
		loaded,
		candidates,
		contextSourceConcerns(pack, index),
		map[string]int{"central": 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	modes := []string{}
	previousCost := int(^uint(0) >> 1)
	for _, option := range options {
		modes = append(modes, option.section.RenderMode)
		if option.estimated <= 0 || option.estimated > previousCost {
			t.Fatalf("option costs are not precomputed from detailed to compact: %#v", options)
		}
		previousCost = option.estimated
	}
	if strings.Join(modes, ",") != "body,focused,signature" {
		t.Fatalf("render option order = %v", modes)
	}

	concerns := contextSourceConcerns(pack, index)
	state := contextSourceSelectionState{
		selectedCandidates: map[string]bool{},
		selectedFactIDs:    map[string]bool{},
		selectedProjects:   map[string]bool{},
		coveredConcerns:    map[string]bool{},
		coveredRoles:       map[string]bool{},
	}
	fitting, err := fittingContextSourceOptions(
		pack,
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens},
		options,
		concerns,
		state,
	)
	if err != nil {
		t.Fatal(err)
	}
	fittingModes := make([]string, 0, len(fitting))
	for _, option := range fitting {
		fittingModes = append(fittingModes, option.section.RenderMode)
	}
	if strings.Join(fittingModes, ",") != "body,focused,signature" {
		t.Fatalf("fitting render modes = %v", fittingModes)
	}
	mandatory, ok, err := smallestFittingContextSourceOption(
		pack,
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens},
		options,
		concerns,
		state,
		contextSourceBoundary{factID: "central"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || mandatory.section.RenderMode != "signature" {
		t.Fatalf("mandatory render option = %#v", mandatory)
	}
	greedy, _, found, err := contextSourceUtilityOption(
		pack,
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens},
		options,
		concerns,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || greedy.section.RenderMode != "signature" {
		t.Fatalf("greedy render option = %#v", greedy)
	}

	got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: DefaultContextBudgetTokens})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 || got.SourceSections[0].RenderMode != "signature" {
		t.Fatalf("large central source selection = %#v", got.SourceSections)
	}
}

func TestContextSourceOptionsUseSpecifiedTieBreakersWithoutPriority(t *testing.T) {
	entrypoint := contextSourceOption{
		candidate: sourceCandidate{FactID: "z", Role: "entrypoint", Priority: 99},
		section:   ContextSourceSection{StartLine: 1, RenderMode: "body"},
	}
	test := contextSourceOption{
		candidate: sourceCandidate{FactID: "a", Role: "test", Priority: 0},
		section:   ContextSourceSection{StartLine: 1, RenderMode: "body"},
	}

	if !contextSourceOptionLess(entrypoint, test) {
		t.Fatalf("priority preceded the specified role tie-breaker: entrypoint=%#v test=%#v", entrypoint, test)
	}
}

func TestContextSourceOptionsRecoverFromStaleMergedAnchor(t *testing.T) {
	root := t.TempDir()
	lines := numberedSourceLines(12)
	lines[9] = "func currentNeighbor() {}"
	writeSourceFile(t, root, "shared.go", strings.Join(lines, "\n")+"\n")
	pack := ContextPack{
		Schema: 1, Query: "current neighbor persistence", Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
		Concerns:              []ContextConcern{{Kind: contextConcernPersistence}},
		Persistence:           []ContextLocation{{ID: "current"}},
		selectedSourceFactIDs: []string{"stale", "current"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "stale", Kind: "api_contract", Name: "DELETE /stale", Qualified: "Client.staleAnchor",
			File: "shared.go", Line: 2, EndLine: 3,
		},
		{
			ID: "current", Kind: "persistence", Name: "currentNeighbor", Search: "current neighbor persistence",
			File: "shared.go", Line: 10, EndLine: 10, Confidence: "EXACT",
		},
	}}
	loaded := loadedContextIndex{ScopeRoot: root, Index: index}
	candidates := contextSourceCandidates(pack, index)
	if len(candidates) != 1 || candidates[0].FactID != "stale" {
		t.Fatalf("stale anchor fixture did not merge as expected: %#v", candidates)
	}

	got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: DefaultContextBudgetTokens})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 || !strings.Contains(got.SourceSections[0].Content, "currentNeighbor") ||
		got.SourceCoverage != "complete" || len(got.SourceOmissions) != 0 || !got.Concerns[0].Covered {
		t.Fatalf("current neighbor was suppressed by stale merged anchor: %#v", got)
	}
}

func TestContextSourceOptionsRequireCompleteMergedEvidence(t *testing.T) {
	root := t.TempDir()
	lines := numberedSourceLines(14)
	lines[1] = "func entrypoint() {"
	lines[2] = "}"
	lines[9] = "func firstHop() {"
	lines[10] = "}"
	writeSourceFile(t, root, "flow.go", strings.Join(lines, "\n")+"\n")
	pack := ContextPack{
		Schema: 1, Query: "entrypoint first hop", Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
		Concerns: []ContextConcern{
			{Kind: contextConcernEntrypoint},
			{Kind: contextConcernPrimaryPath},
		},
		Entrypoints:           []ContextLocation{{ID: "entrypoint"}},
		selectedSourceFactIDs: []string{"entrypoint", "first-hop"},
	}
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{
			{ID: "entrypoint", Kind: "symbol", Name: "entrypoint", Search: "entrypoint first hop", File: "flow.go", Line: 2, EndLine: 3, Confidence: "EXACT"},
			{ID: "first-hop", Kind: "symbol", Name: "firstHop", Search: "entrypoint first hop", File: "flow.go", Line: 10, EndLine: 11, Confidence: "EXACT"},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "entrypoint-first-hop", FromFactID: "entrypoint", ToFactID: "first-hop", Kind: "call", Confidence: "EXACT",
		}},
	}
	loaded := loadedContextIndex{ScopeRoot: root, Index: index}
	candidates := contextSourceCandidates(pack, index)
	if len(candidates) != 1 || len(candidates[0].FactIDs) != 2 {
		t.Fatalf("mandatory facts did not form one nearby candidate: %#v", candidates)
	}
	options, _, err := contextSourceRenderOptions(
		pack,
		loaded,
		candidates,
		contextSourceConcerns(pack, index),
		map[string]int{"entrypoint": 0, "first-hop": 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range options {
		if len(option.candidate.FactIDs) != 2 {
			t.Fatalf("partial merged option remained eligible: %#v", option)
		}
	}

	got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: DefaultContextBudgetTokens})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 ||
		!strings.Contains(got.SourceSections[0].Content, "entrypoint") ||
		!strings.Contains(got.SourceSections[0].Content, "firstHop") ||
		got.SourceCoverage != "complete" {
		t.Fatalf("mandatory merged evidence = %#v", got)
	}
}

func TestContextSourceOptionsRespectFileBudget(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "route.go", "func route() {}\n")
	writeSourceFile(t, root, "provider.go", "func provider() {}\n")
	pack := ContextPack{
		Schema: 1, Query: "inspect app and provider", Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
		Concerns: []ContextConcern{
			{Kind: contextConcernEntrypoint},
			{Kind: contextConcernProject, Project: "provider"},
		},
		Entrypoints:           []ContextLocation{{ID: "route", Project: "app", File: "route.go"}},
		selectedSourceFactIDs: []string{"route", "provider"},
	}
	loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "route", Project: "app", Kind: "symbol", Name: "route", File: "route.go", Line: 1, EndLine: 1},
		{ID: "provider", Project: "provider", Kind: "symbol", Name: "provider", File: "provider.go", Line: 1, EndLine: 1},
	}}}

	got, err := attachContextSource(pack, loaded, ContextRequest{
		BudgetTokens: DefaultContextBudgetTokens,
		MaxFiles:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 || got.SourceSections[0].Path != "route.go" || got.SourceCoverage != "partial" {
		t.Fatalf("file budget source selection = %#v", got)
	}
}

func TestContextSourceOptionsRetainFactRolesWithoutMetadataLocations(t *testing.T) {
	pack := ContextPack{
		Query:                 "inspect production path",
		selectedSourceFactIDs: []string{"repository", "contract"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "repository", Kind: "persistence", Name: "deleteRecords", File: "repository.go", Line: 1},
		{ID: "contract", Kind: "api_contract", Name: "DELETE /records", Qualified: "Client.deleteRecords", File: "client.go", Line: 1},
	}}

	candidates := contextSourceCandidates(pack, index)
	if len(candidates) != 2 || candidates[0].FactID != "contract" || candidates[0].Role != "contract" ||
		candidates[1].FactID != "repository" || candidates[1].Role != "persistence" {
		t.Fatalf("source fact roles = %#v", candidates)
	}
}

func TestContextSourceConcernCoverage(t *testing.T) {
	t.Run("optional source omission stays complete", func(t *testing.T) {
		root := t.TempDir()
		writeSourceFile(t, root, "route.go", "func route() {}\n")
		pack := ContextPack{
			Schema: 1, Query: "inspect route", Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
			Concerns: []ContextConcern{
				{Kind: contextConcernEntrypoint, Covered: false},
				{Kind: contextConcernPrimaryPath, Covered: false},
			},
			Entrypoints:           []ContextLocation{{ID: "route", File: "route.go"}},
			selectedSourceFactIDs: []string{"route", "optional"},
		}
		loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
			{ID: "route", Kind: "symbol", Name: "route", File: "route.go", Line: 1, EndLine: 1},
			{ID: "optional", Kind: "symbol", Name: "optional", File: "missing.go", Line: 1, EndLine: 1},
		}}}

		got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: DefaultContextBudgetTokens})
		if err != nil {
			t.Fatal(err)
		}
		if got.SourceCoverage != "complete" || len(got.SourceOmissions) != 0 {
			t.Fatalf("optional omission downgraded source coverage: %#v", got)
		}
		for _, concern := range got.Concerns {
			if !concern.Covered {
				t.Fatalf("selected current source did not cover %#v", concern)
			}
		}
	})

	t.Run("missing required project is partial with an exact omission", func(t *testing.T) {
		root := t.TempDir()
		writeSourceFile(t, root, "route.go", "func route() {}\n")
		pack := ContextPack{
			Schema: 1, Query: "inspect app and services/provider", Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
			Concerns: []ContextConcern{
				{Kind: contextConcernEntrypoint, Covered: true},
				{Kind: contextConcernProject, Project: "services/provider", Covered: true},
			},
			Entrypoints:           []ContextLocation{{ID: "route", Project: "app", File: "route.go"}},
			selectedSourceFactIDs: []string{"route", "provider"},
		}
		loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "app", Kind: "symbol", Name: "route", File: "route.go", Line: 1, EndLine: 1},
			{ID: "provider", Project: "services/provider", Kind: "symbol", Name: "provider", File: "Provider.go", Line: 1, EndLine: 1},
		}}}

		got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: DefaultContextBudgetTokens})
		if err != nil {
			t.Fatal(err)
		}
		want := ContextSourceOmission{
			Project: "services/provider", Path: "Provider.go", Role: "call_chain", Reason: "source file is missing",
		}
		if got.SourceCoverage != "partial" || len(got.SourceOmissions) != 1 || got.SourceOmissions[0] != want {
			t.Fatalf("required project omission = %#v, want %#v", got, want)
		}
		if got.Concerns[0].Covered != true || got.Concerns[1].Covered != false {
			t.Fatalf("public concern coverage = %#v", got.Concerns)
		}
	})

	t.Run("no current required source is none", func(t *testing.T) {
		root := t.TempDir()
		pack := ContextPack{
			Schema: 1, Query: "inspect route", Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
			Concerns: []ContextConcern{
				{Kind: contextConcernEntrypoint, Covered: true},
				{Kind: contextConcernPrimaryPath, Covered: true},
			},
			Entrypoints:           []ContextLocation{{ID: "route", File: "route.go"}},
			selectedSourceFactIDs: []string{"route"},
		}
		loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
			{ID: "route", Kind: "symbol", Name: "route", File: "route.go", Line: 1, EndLine: 1},
		}}}

		got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: DefaultContextBudgetTokens})
		if err != nil {
			t.Fatal(err)
		}
		if got.SourceCoverage != "none" || len(got.SourceOmissions) != 1 {
			t.Fatalf("missing current source coverage = %#v", got)
		}
		for _, concern := range got.Concerns {
			if concern.Covered {
				t.Fatalf("unselected concern remained covered: %#v", got.Concerns)
			}
		}
	})
}

func TestResolveSourcePathUsesSelectedIndexScope(t *testing.T) {
	projectRoot := t.TempDir()
	projectFile := writeSourceFile(t, projectRoot, "src/UserService.java", "class UserService {}\n")
	workspaceRoot := t.TempDir()
	workspaceFile := writeSourceFile(t, workspaceRoot, "services/users/src/UserService.java", "class UserService {}\n")
	configuredIndex := filepath.Join(projectRoot, "build", "generated", "goregraph", "agent", "context-index.json")

	tests := []struct {
		name   string
		loaded loadedContextIndex
		file   sourceCandidate
		want   string
	}{
		{
			name: "project",
			loaded: loadedContextIndex{
				Path: configuredIndex, ScopeRoot: projectRoot,
			},
			file: sourceCandidate{Path: "src/UserService.java"},
			want: projectFile,
		},
		{
			name:   "workspace",
			loaded: loadedContextIndex{ScopeRoot: workspaceRoot, Workspace: true},
			file:   sourceCandidate{Project: "services/users", Path: "src/UserService.java"},
			want:   workspaceFile,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want, err := filepath.EvalSymlinks(test.want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := resolveSourcePath(test.loaded, test.file)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("resolveSourcePath() = %q, want %q", got, want)
			}
		})
	}
}

func TestResolveSourcePathRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	outside := writeSourceFile(t, t.TempDir(), "outside.java", "class Outside {}\n")
	writeSourceFile(t, root, "src/inside.java", "class Inside {}\n")
	if err := os.Symlink(outside, filepath.Join(root, "src", "escape.java")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		loaded loadedContextIndex
		file   sourceCandidate
	}{
		{name: "absolute fact path", loaded: loadedContextIndex{ScopeRoot: root}, file: sourceCandidate{Path: "/etc/passwd"}},
		{name: "fact path traversal", loaded: loadedContextIndex{ScopeRoot: root}, file: sourceCandidate{Path: "../../outside.java"}},
		{name: "workspace project traversal", loaded: loadedContextIndex{ScopeRoot: root, Workspace: true}, file: sourceCandidate{Project: "../../outside", Path: "inside.java"}},
		{name: "escaping symlink", loaded: loadedContextIndex{ScopeRoot: root}, file: sourceCandidate{Path: "src/escape.java"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := resolveSourcePath(test.loaded, test.file)
			if err == nil || !strings.Contains(err.Error(), "source path is unsafe") {
				t.Fatalf("resolveSourcePath() error = %v, want source path is unsafe", err)
			}
		})
	}
}

func TestResolveSourcePathConfinesWorkspaceCandidatesToProjectRoot(t *testing.T) {
	root := t.TempDir()
	secret := writeSourceFile(t, root, "b/secret.java", "class Secret {}\n")
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "a", "link.java")); err != nil {
		t.Fatal(err)
	}

	for _, candidate := range []sourceCandidate{
		{Project: "a", Path: "../b/secret.java"},
		{Project: "a", Path: "link.java"},
	} {
		_, err := resolveSourcePath(
			loadedContextIndex{ScopeRoot: root, Workspace: true},
			candidate,
		)
		if err == nil || err.Error() != "source path is unsafe" {
			t.Fatalf("resolveSourcePath(%#v) error = %v, want source path is unsafe", candidate, err)
		}
	}
}

func TestReadSourceFileRejectsUnsafeContent(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	nonUTF8 := filepath.Join(root, "non-utf8.java")
	if err := os.WriteFile(nonUTF8, []byte{0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	tooLarge := filepath.Join(root, "too-large.java")
	if err := os.WriteFile(tooLarge, make([]byte, MaxContextSourceFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "directory", path: directory, want: "source file is not regular"},
		{name: "non UTF-8", path: nonUTF8, want: "source file is not valid UTF-8"},
		{name: "too large", path: tooLarge, want: "source file exceeds maximum size"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := readSourceFile(test.path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("readSourceFile() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestReadSourceFileRejectsUnreadableRegularFile(t *testing.T) {
	path := writeSourceFile(t, t.TempDir(), "unreadable.java", "class Unreadable {}\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err := readSourceFile(path)
	if err == nil {
		t.Skip("test process can open mode-000 files")
	}
	if !strings.Contains(err.Error(), "source file is unreadable") {
		t.Fatalf("readSourceFile() error = %v, want source file is unreadable", err)
	}
}

func TestReadSourceFileNormalizesCRLFAndPreservesPhysicalLines(t *testing.T) {
	path := writeSourceFile(t, t.TempDir(), "src/UserService.java", "one\r\ntwo\r\n")

	file, err := readSourceFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if file.Path != path {
		t.Fatalf("source path = %q, want %q", file.Path, path)
	}
	if got, want := strings.Join(file.Lines, "|"), "one|two|"; got != want {
		t.Fatalf("source lines = %q, want %q", got, want)
	}
}

func writeSourceFile(t *testing.T, root, relativePath, body string) string {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
