package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
