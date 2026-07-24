package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestApplyContextSourceCoverageRequiresEveryInternalFacet(t *testing.T) {
	base := newContextConcern(
		contextConcernPersistence,
		"services/jobs",
		true,
		[]string{"regular-repository", "change-repository"},
		"requested persistence",
	)
	regular := newContextEvidenceConcern(
		base, "model:regular-job", []string{"regular-repository"}, "regular job persistence",
	)
	change := newContextEvidenceConcern(
		base, "model:change-job", []string{"change-repository"}, "change job persistence",
	)
	pack := ContextPack{
		Concerns: []ContextConcern{{
			Kind: contextConcernPersistence, Project: "services/jobs",
		}},
		SourceSections: []ContextSourceSection{{
			Project: "services/jobs", Path: "RegularRepository.java",
		}},
	}

	applyContextSourceCoverage(
		&pack,
		[]contextConcern{regular, change},
		map[string]bool{regular.key: true},
	)
	if pack.SourceCoverage != "partial" || pack.Concerns[0].Covered {
		t.Fatalf("one of two persistence facets reported complete: %#v", pack)
	}

	applyContextSourceCoverage(
		&pack,
		[]contextConcern{regular, change},
		map[string]bool{regular.key: true, change.key: true},
	)
	if pack.SourceCoverage != "complete" || !pack.Concerns[0].Covered {
		t.Fatalf("all persistence facets did not aggregate: %#v", pack)
	}
}

func TestContextSourceSectionSupportsOnlyMatchingEvidenceFacet(t *testing.T) {
	base := newContextConcern(
		contextConcernSideEffects,
		"services/jobs",
		true,
		[]string{"delete-job"},
		"requested side effects",
	)
	mail := newContextEvidenceConcern(base, "mail", []string{"delete-job"}, "mail")
	audit := newContextEvidenceConcern(base, "audit", []string{"delete-job"}, "audit")
	user := newContextEvidenceConcern(base, "user_information", []string{"delete-job"}, "user")
	section := ContextSourceSection{
		Project:    "services/jobs",
		Role:       "call_chain",
		RenderMode: "declaration_body",
		Content: `void deleteJob() {
  mailService.sendDeletedMail(job);
}`,
	}
	if !contextSourceSectionSupportsEvidence(section, mail) {
		t.Fatal("mail evidence was rejected")
	}
	if contextSourceSectionSupportsEvidence(section, audit) ||
		contextSourceSectionSupportsEvidence(section, user) {
		t.Fatal("mail-only source covered audit or user information")
	}
}

func TestContextSourceEvidenceOmissionsCoalesceFacetsByPath(t *testing.T) {
	base := newContextConcern(
		contextConcernSideEffects,
		"services/jobs",
		true,
		[]string{"service"},
		"requested side effects",
	)
	concerns := []contextConcern{
		newContextEvidenceConcern(base, "mail", []string{"service"}, "mail evidence"),
		newContextEvidenceConcern(base, "audit", []string{"service"}, "audit evidence"),
	}
	candidates := []sourceCandidate{{
		FactID: "service", FactIDs: []string{"service"},
		Project: "services/jobs", Path: "src/JobService.java", Role: "call_chain",
	}}
	got := contextSourceEvidenceOmissions(concerns, candidates, nil, map[string]bool{})
	if len(got) != 1 ||
		got[0].Project != "services/jobs" ||
		got[0].Path != "src/JobService.java" ||
		!strings.Contains(got[0].Reason, "audit evidence") ||
		!strings.Contains(got[0].Reason, "mail evidence") {
		t.Fatalf("coalesced omissions = %#v", got)
	}
}

func TestExpandContextEvidenceConcernsScopesOperationalBoundaries(t *testing.T) {
	pack, index := contextEvidenceExpansionFixture()
	concerns := []contextConcern{
		newContextConcern(
			contextConcernAuth,
			"libraries/job-client",
			true,
			[]string{"job-contract"},
			"client authentication",
		),
		newContextConcern(
			contextConcernAuth,
			"services/jobs",
			true,
			[]string{"jobs-security"},
			"server authentication",
		),
		newContextConcern(
			contextConcernConfiguration,
			"libraries/job-client",
			true,
			[]string{"job-config"},
			"configuration",
		),
		newContextConcern(
			contextConcernResilience,
			"libraries/job-client",
			true,
			[]string{"job-contract"},
			"resilience",
		),
		newContextConcern(
			contextConcernSideEffects,
			"services/jobs",
			true,
			[]string{"jobs-side-effects"},
			"side effects",
		),
	}

	got := expandContextEvidenceConcerns(pack, index, concerns)
	for _, key := range []string{
		contextConcernAuth + ":libraries/job-client#client_transport",
		contextConcernAuth + ":services/jobs#server_policy",
		contextConcernConfiguration + ":libraries/job-client#binding",
		contextConcernConfiguration + ":libraries/job-client#consumer",
		contextConcernResilience + ":libraries/job-client#retry_policy",
		contextConcernResilience + ":libraries/job-client#recovery",
		contextConcernSideEffects + ":services/jobs#mail",
		contextConcernSideEffects + ":services/jobs#audit",
		contextConcernSideEffects + ":services/jobs#user_information",
	} {
		if _, ok := findContextConcern(got, key); !ok {
			t.Errorf("expanded concern %q missing from %#v", key, got)
		}
	}
}

func TestExpandContextEvidenceConcernsTracksPersistencePerRequestedModel(t *testing.T) {
	pack, index := contextEvidenceExpansionFixture()
	base := newContextConcern(
		contextConcernPersistence,
		"services/jobs",
		true,
		[]string{"regular-repository", "change-repository"},
		"persistence",
	)

	got := expandContextEvidenceConcerns(pack, index, []contextConcern{base})
	for key, wantCandidate := range map[string]string{
		contextConcernPersistence + ":services/jobs#model:regular-model": "regular-repository",
		contextConcernPersistence + ":services/jobs#model:change-model":  "change-repository",
	} {
		concern, ok := findContextConcern(got, key)
		if !ok || !reflect.DeepEqual(concern.candidateFactIDs, []string{wantCandidate}) {
			t.Errorf("%q = %#v, want candidate %q", key, concern, wantCandidate)
		}
	}
}

func TestPublicContextConcernsDeduplicatesEvidenceFacets(t *testing.T) {
	base := newContextConcern(
		contextConcernSideEffects,
		"services/jobs",
		true,
		[]string{"jobs-side-effects"},
		"side effects",
	)
	public := publicContextConcerns([]contextConcern{
		newContextEvidenceConcern(base, "mail", base.candidateFactIDs, "mail"),
		newContextEvidenceConcern(base, "audit", base.candidateFactIDs, "audit"),
	})
	if len(public) != 1 ||
		public[0].Kind != contextConcernSideEffects ||
		public[0].Project != "services/jobs" {
		t.Fatalf("public facets were not aggregated: %#v", public)
	}
}

func contextEvidenceExpansionFixture() (ContextPack, scan.AgentContextIndexRecord) {
	query := "delete catalog item change job task types with authentication, configuration, " +
		"retry and error recovery plus mail, audit, and user information"
	pack := ContextPack{
		Query:          query,
		selectionQuery: query,
		Endpoints: []ContextEndpoint{{
			Provider: "services/catalog", HTTPMethod: "DELETE",
			Path: "/catalog/items/{itemId}",
		}},
		Contracts: []ContextLocation{{
			ID: "job-contract", Project: "libraries/job-client", Kind: "api_contract",
		}},
		selectedSourceFactIDs: []string{"regular-model", "change-model"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "catalog-route", Project: "services/catalog", Kind: "route",
			Name: "DELETE /catalog/items/{itemId}", HTTPMethod: "DELETE",
			Path: "/catalog/items/{itemId}", File: "CatalogController.java",
		},
		{
			ID: "job-contract", Project: "libraries/job-client", Kind: "api_contract",
			Name: "GET /jobs", Qualified: "JobClient.getJobs",
			File: "JobClient.java",
		},
		{
			ID: "job-config", Project: "libraries/job-client", Kind: "configuration",
			Name: "JobClientConfig", File: "JobClientConfig.java",
		},
		{
			ID: "jobs-security", Project: "services/jobs", Kind: "endpoint_security",
			Name: "basic", File: "SecurityConfig.java",
		},
		{
			ID: "jobs-side-effects", Project: "services/jobs", Kind: "symbol",
			Name: "deleteJobs", Qualified: "JobService.deleteJobs", File: "JobService.java",
		},
		{
			ID: "regular-model", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogItemJobEntity", Qualified: "example.CatalogItemJobEntity",
			File: "CatalogItemJobEntity.java", Confidence: "EXACT",
		},
		{
			ID: "change-model", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogItemChangeJobEntity", Qualified: "example.CatalogItemChangeJobEntity",
			File: "CatalogItemChangeJobEntity.java", Confidence: "EXACT",
		},
		{
			ID: "regular-repository", Project: "services/jobs", Kind: "persistence",
			Name: "CatalogItemJobRepository", Qualified: "example.CatalogItemJobRepository",
			File: "CatalogItemJobRepository.java", Confidence: "EXACT",
		},
		{
			ID: "change-repository", Project: "services/jobs", Kind: "persistence",
			Name: "CatalogItemChangeJobRepository", Qualified: "example.CatalogItemChangeJobRepository",
			File: "CatalogItemChangeJobRepository.java", Confidence: "EXACT",
		},
	}}
	return pack, index
}

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

func TestRenderSourceCandidateUsesCompactDeclarationBody(t *testing.T) {
	lines := []string{
		"package users;",
		"@Override",
		"public void deleteUser() {",
		"    if (enabled) {",
		`        logger.info("ignored braces {}");`,
		"    }",
		"}",
		"public void unrelated() {}",
	}
	candidate := sourceCandidate{
		Project: "users", Path: "src/UserService.java", StartLine: 3, EndLine: 3,
		Role: "call_chain", Kind: "symbol", Name: "deleteUser",
	}

	section, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: lines},
		"declaration_body",
	)
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 2 || section.EndLine != 7 {
		t.Fatalf("compact declaration range = %d-%d, want 2-7", section.StartLine, section.EndLine)
	}
	if section.RenderMode != "declaration_body" ||
		!strings.Contains(section.Content, "logger.info") ||
		strings.Contains(section.Content, "unrelated") {
		t.Fatalf("compact declaration section = %#v", section)
	}

	cases := []struct {
		name      string
		path      string
		lines     []string
		candidate sourceCandidate
		wantStart int
		wantEnd   int
		want      string
		excluded  string
	}{
		{
			name: "Go nested composite literal", path: "users.go",
			lines: []string{
				"package users",
				"func deleteUser() {",
				"    values := map[string]any{",
				`        "nested": struct{ Enabled bool }{Enabled: true},`,
				"    }",
				"    _ = values",
				"}",
				"func unrelated() {}",
			},
			candidate: sourceCandidate{Path: "users.go", Name: "deleteUser", StartLine: 2, EndLine: 2},
			wantStart: 2, wantEnd: 7, want: "values :=", excluded: "unrelated",
		},
		{
			name: "TypeScript masked braces", path: "users.ts",
			lines: []string{
				"export function deleteUser() {",
				`  const message = "ignored braces {}";`,
				"  // ignored closing brace }",
				"  removeUser();",
				"}",
				"export function unrelated() {}",
			},
			candidate: sourceCandidate{Path: "users.ts", Name: "deleteUser", StartLine: 1, EndLine: 1},
			wantStart: 1, wantEnd: 5, want: "removeUser", excluded: "unrelated",
		},
		{
			name: "Python indented body", path: "users.py",
			lines: []string{
				"@transactional",
				"def delete_user():",
				"    if enabled:",
				"        remove_user()",
				"",
				"def unrelated():",
				"    pass",
			},
			candidate: sourceCandidate{Path: "users.py", Name: "delete_user", StartLine: 2, EndLine: 2},
			wantStart: 1, wantEnd: 5, want: "remove_user", excluded: "unrelated",
		},
		{
			name: "Python one-line suite", path: "users.py",
			lines: []string{
				"def delete_user(): pass",
				"def unrelated(): pass",
			},
			candidate: sourceCandidate{Path: "users.py", Name: "delete_user", StartLine: 1, EndLine: 1},
			wantStart: 1, wantEnd: 1, want: "pass", excluded: "unrelated",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := renderSourceCandidate(
				testCase.candidate,
				sourceFile{Path: testCase.path, Lines: testCase.lines},
				"declaration_body",
			)
			if err != nil {
				t.Fatal(err)
			}
			if got.StartLine != testCase.wantStart || got.EndLine != testCase.wantEnd ||
				!strings.Contains(got.Content, testCase.want) ||
				strings.Contains(got.Content, testCase.excluded) {
				t.Fatalf("compact declaration section = %#v", got)
			}
		})
	}
}

func TestContextSourceUsesVerifiedInheritedOwner(t *testing.T) {
	root := t.TempDir()
	const project = "services/jobs"
	const sourcePath = "src/main/java/example/JobRepository.java"
	writeSourceFile(t, root, filepath.Join(project, sourcePath), `package example;
interface JobRepository extends CrudRepository<JobEntity, Long> {
    List<JobEntity> findByCatalogItem(long catalogId, long itemId);
}
`)
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "owner", Project: project, Kind: "symbol", Name: "JobRepository",
			Qualified: "JobRepository", File: sourcePath, Line: 2, EndLine: 4, Confidence: "EXACT",
		},
		{
			ID: "inherited", Project: project, Kind: "persistence", Name: "findAll",
			Qualified: "JobRepository.findAll", File: sourcePath, Line: 2, EndLine: 2,
			Confidence: "RESOLVED", Summary: "inherited repository method",
		},
	}}
	pack := ContextPack{
		Schema: 3, Query: "inspect services/jobs repository persistence", Confidence: "EXACT",
		BudgetTokens: DefaultContextBudgetTokens,
		Concerns: []ContextConcern{{
			Kind: contextConcernPersistence, Project: project, Covered: true,
		}},
		Persistence:           []ContextLocation{{ID: "inherited", Project: project, File: sourcePath}},
		selectedSourceFactIDs: []string{"inherited"},
	}

	got, err := attachContextSource(
		pack,
		loadedContextIndex{ScopeRoot: root, Workspace: true, Index: index},
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 {
		t.Fatalf("inherited owner source sections = %#v, omissions %#v", got.SourceSections, got.SourceOmissions)
	}
	section := got.SourceSections[0]
	if section.SourceState != "inherited_owner_current" {
		t.Fatalf("source state = %q", section.SourceState)
	}
	if !strings.Contains(section.Content, "interface JobRepository") ||
		strings.Contains(section.Content, "findAll(") {
		t.Fatalf("owner declaration evidence is not honest:\n%s", section.Content)
	}
	if got.SourceCoverage != "complete" || len(got.SourceOmissions) != 0 {
		t.Fatalf("inherited owner coverage = %q / %#v", got.SourceCoverage, got.SourceOmissions)
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

func TestRenderSourceCandidateUsesIndexedTypeDeclarationBeforeConstructor(t *testing.T) {
	candidate := sourceCandidate{
		Path: "CatalogJobEntity.java", StartLine: 2,
		Kind: "symbol", Name: "CatalogJobEntity",
	}
	section, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: []string{
			"@Entity",
			"public class CatalogJobEntity {",
			"  public CatalogJobEntity(long catalogId) {}",
			"}",
		}},
		"declaration_body",
	)
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 1 || section.EndLine != 4 ||
		!strings.Contains(section.Content, "class CatalogJobEntity") {
		t.Fatalf("indexed type declaration = %#v", section)
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

func TestRenderSourceCandidateRelocatesGeneratedAccessorToBackingField(t *testing.T) {
	lines := []string{
		"import lombok.Getter;",
		"@Getter",
		"public class ApplicationConfig {",
		"  private boolean showErrorsInResponse;",
		"  private boolean userLicensesParallelBatching;",
		"}",
	}
	candidate := sourceCandidate{
		Path: "ApplicationConfig.java", StartLine: 3, Kind: "symbol",
		Name:      "isUserLicensesParallelBatching",
		Qualified: "ApplicationConfig.isUserLicensesParallelBatching",
	}

	section, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: lines},
		"signature",
	)
	if err != nil {
		t.Fatal(err)
	}
	if section.StartLine != 5 || section.EndLine != 5 ||
		section.SourceState != "relocated_current" ||
		section.Content != "5\t  private boolean userLicensesParallelBatching;" {
		t.Fatalf("generated accessor section = %#v", section)
	}
}

func TestRenderSourceCandidateRejectsAmbiguousGeneratedAccessorFields(t *testing.T) {
	lines := []string{
		"import lombok.Getter;",
		"@Getter",
		"class FirstConfig {",
		"  private boolean enabled;",
		"  private boolean enabled;",
		"}",
	}
	candidate := sourceCandidate{
		Path: "Config.java", StartLine: 3, Kind: "symbol",
		Name: "isEnabled", Qualified: "FirstConfig.isEnabled",
	}

	_, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: lines},
		"signature",
	)
	if err == nil || err.Error() != "indexed symbol is ambiguous in current source" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
	}
}

func TestRenderSourceCandidateRejectsGeneratedAccessorFieldOutsideOwner(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
	}{
		{
			name: "local variable",
			lines: []string{
				"import lombok.Getter;",
				"@Getter",
				"class FirstConfig {",
				"  void load() {",
				"    boolean enabled;",
				"  }",
				"}",
			},
		},
		{
			name: "sibling class",
			lines: []string{
				"import lombok.Getter;",
				"@Getter",
				"class FirstConfig {",
				"}",
				"class SecondConfig {",
				"  private boolean enabled;",
				"}",
			},
		},
		{
			name: "nested class",
			lines: []string{
				"import lombok.Getter;",
				"@Getter",
				"class FirstConfig {",
				"  class NestedConfig {",
				"    private boolean enabled;",
				"  }",
				"}",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := sourceCandidate{
				Path: "Config.java", StartLine: 3, Kind: "symbol",
				Name: "isEnabled", Qualified: "FirstConfig.isEnabled",
			}
			_, err := renderSourceCandidate(
				candidate,
				sourceFile{Path: candidate.Path, Lines: test.lines},
				"signature",
			)
			if err == nil || err.Error() != "indexed symbol is absent from current source" {
				t.Fatalf("renderSourceCandidate() error = %v", err)
			}
		})
	}
}

func TestRenderSourceCandidateRejectsNonLombokAccessorAnnotations(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
	}{
		{name: "Spring Value", annotation: `@Value("${enabled}")`},
		{name: "suffix match", annotation: "@Target.GetterLike"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lines := []string{
				"class ApplicationConfig {",
				"  " + test.annotation,
				"  private boolean enabled;",
				"}",
			}
			candidate := sourceCandidate{
				Path: "ApplicationConfig.java", StartLine: 1, Kind: "symbol",
				Name: "getEnabled", Qualified: "ApplicationConfig.getEnabled",
			}
			_, err := renderSourceCandidate(
				candidate,
				sourceFile{Path: candidate.Path, Lines: lines},
				"signature",
			)
			if err == nil || err.Error() != "indexed symbol is absent from current source" {
				t.Fatalf("renderSourceCandidate() error = %v", err)
			}
		})
	}
}

func TestRenderSourceCandidateRejectsBooleanIsAccessorForBoxedField(t *testing.T) {
	lines := []string{
		"import lombok.Getter;",
		"@Getter",
		"class ApplicationConfig {",
		"  private Boolean enabled;",
		"}",
	}
	candidate := sourceCandidate{
		Path: "ApplicationConfig.java", StartLine: 3, Kind: "symbol",
		Name: "isEnabled", Qualified: "ApplicationConfig.isEnabled",
	}

	_, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: lines},
		"signature",
	)
	if err == nil || err.Error() != "indexed symbol is absent from current source" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
	}
}

func TestRenderSourceCandidateRequiresGeneratedAccessorAnnotation(t *testing.T) {
	lines := []string{
		"class ApplicationConfig {",
		"  private boolean enabled;",
		"}",
	}
	candidate := sourceCandidate{
		Path: "ApplicationConfig.java", StartLine: 1, Kind: "symbol",
		Name: "isEnabled", Qualified: "ApplicationConfig.isEnabled",
	}

	_, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: lines},
		"signature",
	)
	if err == nil || err.Error() != "indexed symbol is absent from current source" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
	}
}

func TestRenderSourceCandidateDoesNotRelocateGeneratedAccessorOutsideJava(t *testing.T) {
	lines := []string{
		"type ApplicationConfig struct {",
		"  userLicensesParallelBatching bool",
		"}",
	}
	candidate := sourceCandidate{
		Path: "config.go", StartLine: 1, Kind: "symbol",
		Name:      "isUserLicensesParallelBatching",
		Qualified: "ApplicationConfig.isUserLicensesParallelBatching",
	}

	_, err := renderSourceCandidate(
		candidate,
		sourceFile{Path: candidate.Path, Lines: lines},
		"signature",
	)
	if err == nil || err.Error() != "indexed symbol is absent from current source" {
		t.Fatalf("renderSourceCandidate() error = %v", err)
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
	for _, option := range options {
		modes = append(modes, option.section.RenderMode)
		if option.estimated <= 0 {
			t.Fatalf("option cost was not precomputed: %#v", options)
		}
	}
	if strings.Join(modes, ",") != "declaration_body,body,focused,signature" {
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
	if strings.Join(fittingModes, ",") != "declaration_body,body,focused,signature" {
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

	got, err := attachContextSource(pack, loaded, ContextRequest{BudgetTokens: MinContextBudgetTokens})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 || got.SourceSections[0].RenderMode != "signature" {
		t.Fatalf("tight-budget central source selection = %#v", got.SourceSections)
	}
}

func TestBuildContextEnrichesEndpointAndFirstLocalServiceSource(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-20T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "endpoint", Project: "services/catalog", Kind: "api_endpoint",
				Name: "DELETE /catalog/items/{id}", Qualified: "CatalogController.deleteItem",
				HTTPMethod: "DELETE", Path: "/catalog/items/{id}", File: "CatalogController.java",
				Line: 2, EndLine: 6, Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "route", Project: "services/catalog", Kind: "route",
				Name: "DELETE /catalog/items/{id}", Qualified: "CatalogController.deleteItem",
				HTTPMethod: "DELETE", Path: "/catalog/items/{id}", File: "CatalogController.java",
				Line: 2, EndLine: 6, Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "controller", Project: "services/catalog", Kind: "symbol",
				Name: "deleteItem", Qualified: "CatalogController.deleteItem",
				File: "CatalogController.java", Line: 2, EndLine: 6,
				Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "service", Project: "services/catalog", Kind: "symbol",
				Name: "deleteItem", Qualified: "CatalogService.deleteItem",
				File: "CatalogService.java", Line: 2, EndLine: 6,
				Confidence: "EXACT", Search: "delete catalog item service",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "route-controller", FromFactID: "route", ToFactID: "controller", Kind: "call", Confidence: "EXACT"},
			{ID: "controller-service", FromFactID: "controller", ToFactID: "service", Kind: "call", Confidence: "EXACT"},
		},
	}
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), index)
	writeSourceFile(t, root, filepath.Join("services/catalog", "CatalogController.java"), `class CatalogController {
  void deleteItem() {
    catalogService.deleteItem();
  }
}
`)
	writeSourceFile(t, root, filepath.Join("services/catalog", "CatalogService.java"), `class CatalogService {
  void deleteItem() {
    repository.deleteItem();
  }
}
`)

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "Delete a catalog item through DELETE /catalog/items/{id}.",
		BudgetTokens: DefaultContextBudgetTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantBodies := map[string]string{
		"CatalogController.java": "catalogService.deleteItem();",
		"CatalogService.java":    "repository.deleteItem();",
	}
	for path, body := range wantBodies {
		found := false
		for _, section := range pack.SourceSections {
			if section.Path != path {
				continue
			}
			found = section.RenderMode != "signature" && strings.Contains(section.Content, body)
			break
		}
		if !found {
			t.Fatalf("enriched source %q missing: %#v", path, pack.SourceSections)
		}
	}
}

func TestContextCoreSourceBoundariesPreferSelectedLocalCallTarget(t *testing.T) {
	pack := ContextPack{
		Entrypoints: []ContextLocation{{ID: "endpoint"}},
		selectedSourceFactIDs: []string{
			"endpoint", "contract", "security", "persistence", "unrelated-symbol", "service",
		},
		selectedEdgeIDs: []string{
			"endpoint-contract", "endpoint-security", "endpoint-persistence", "endpoint-unrelated", "endpoint-service",
		},
	}
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{
			{ID: "endpoint", Project: "catalog", Kind: "api_endpoint", File: "CatalogController.java"},
			{ID: "contract", Project: "catalog", Kind: "api_contract", File: "AContract.java"},
			{ID: "security", Project: "catalog", Kind: "endpoint_security", File: "BSecurity.java"},
			{ID: "persistence", Project: "catalog", Kind: "persistence", File: "CRepository.java"},
			{ID: "unrelated-symbol", Project: "catalog", Kind: "symbol", File: "DHelper.java"},
			{ID: "service", Project: "catalog", Kind: "symbol", File: "ZCatalogService.java"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "endpoint-contract", FromFactID: "endpoint", ToFactID: "contract", Kind: "http_contract"},
			{ID: "endpoint-security", FromFactID: "endpoint", ToFactID: "security", Kind: "auth"},
			{ID: "endpoint-persistence", FromFactID: "endpoint", ToFactID: "persistence", Kind: "persistence"},
			{ID: "endpoint-unrelated", FromFactID: "endpoint", ToFactID: "unrelated-symbol", Kind: "reference"},
			{ID: "endpoint-service", FromFactID: "endpoint", ToFactID: "service", Kind: "call"},
		},
	}
	distances := map[string]int{
		"endpoint": 0, "contract": 1, "security": 1, "persistence": 1, "unrelated-symbol": 1, "service": 1,
	}

	boundaries := contextCoreSourceBoundaries(pack, index, distances)
	if len(boundaries) != 2 || boundaries[0].factID != "endpoint" || boundaries[1].factID != "service" {
		t.Fatalf("core boundaries = %#v, want endpoint followed by selected service call target", boundaries)
	}
}

func TestEnrichContextCoreSourceOptionsFocusesEveryBoundaryBeforeBodies(t *testing.T) {
	endpoint := sourceCandidate{FactID: "endpoint", Project: "catalog", Path: "CatalogController.java"}
	service := sourceCandidate{FactID: "service", Project: "catalog", Path: "CatalogService.java"}
	section := func(candidate sourceCandidate, mode, content string) ContextSourceSection {
		return ContextSourceSection{
			Project: candidate.Project, Path: candidate.Path, StartLine: 1, EndLine: 2,
			Role: "call_chain", RenderMode: mode, SourceState: "indexed_range_current", Content: content,
		}
	}
	endpointSignature := section(endpoint, "signature", "void deleteItem();")
	endpointFocused := section(endpoint, "focused", strings.Repeat("endpoint focused ", 50))
	endpointBody := section(endpoint, "body", strings.Repeat("endpoint body ", 100))
	serviceSignature := section(service, "signature", "void deleteItem();")
	serviceFocused := section(service, "focused", strings.Repeat("service focused ", 50))
	serviceBody := section(service, "body", strings.Repeat("service body ", 100))
	options := []contextSourceOption{
		{candidate: endpoint, section: endpointSignature},
		{candidate: endpoint, section: endpointFocused},
		{candidate: endpoint, section: endpointBody},
		{candidate: service, section: serviceSignature},
		{candidate: service, section: serviceFocused},
		{candidate: service, section: serviceBody},
	}
	base, err := finalizeContextEstimate(ContextPack{SourceSections: []ContextSourceSection{
		endpointSignature, serviceSignature,
	}})
	if err != nil {
		t.Fatal(err)
	}
	withSections := func(sections ...ContextSourceSection) ContextPack {
		candidate := cloneContextPack(base)
		candidate.SourceSections = append([]ContextSourceSection(nil), sections...)
		candidate, finalizeErr := finalizeContextEstimate(candidate)
		if finalizeErr != nil {
			t.Fatal(finalizeErr)
		}
		return candidate
	}
	bothFocused := withSections(endpointFocused, serviceFocused)
	endpointBodyOnly := withSections(endpointBody, serviceSignature)
	endpointBodyAndServiceFocused := withSections(endpointBody, serviceFocused)
	budget := 0
	for candidateBudget := 1; candidateBudget <= DefaultContextBudgetTokens; candidateBudget++ {
		request := ContextRequest{BudgetTokens: candidateBudget}
		baseFits, baseErr := contextSourcePackFits(base, request)
		focusedFits, focusedErr := contextSourcePackFits(bothFocused, request)
		bodyFits, bodyErr := contextSourcePackFits(endpointBodyOnly, request)
		unfairMixFits, unfairMixErr := contextSourcePackFits(endpointBodyAndServiceFocused, request)
		if baseErr != nil || focusedErr != nil || bodyErr != nil || unfairMixErr != nil {
			t.Fatalf("budget check failed: %v %v %v %v", baseErr, focusedErr, bodyErr, unfairMixErr)
		}
		if baseFits && focusedFits && bodyFits && !unfairMixFits {
			budget = candidateBudget
			break
		}
	}
	if budget == 0 {
		t.Fatal("test fixture has no budget that distinguishes fair focused enrichment")
	}

	got, err := enrichContextCoreSourceOptions(
		base,
		ContextRequest{BudgetTokens: budget},
		options,
		contextSourceSelectionState{selectedCandidates: map[string]bool{
			contextSourceCandidateKey(endpoint): true,
			contextSourceCandidateKey(service):  true,
		}},
		[]contextSourceBoundary{{factID: "endpoint"}, {factID: "service"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range got.SourceSections {
		if section.RenderMode == "signature" {
			t.Fatalf("core section remained a signature despite room for both focused sections: %#v", got.SourceSections)
		}
	}
}

func TestBuildContextEnrichesCoreSourcesBeforeOptionalTestSource(t *testing.T) {
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-20T00:00:00Z",
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "endpoint", Project: "services/catalog", Kind: "api_endpoint",
				Name: "DELETE /catalog/items/{id}", Qualified: "CatalogController.deleteItem",
				HTTPMethod: "DELETE", Path: "/catalog/items/{id}", File: "CatalogController.java",
				Line: 2, EndLine: 35, Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "route", Project: "services/catalog", Kind: "route",
				Name: "DELETE /catalog/items/{id}", Qualified: "CatalogController.deleteItem",
				HTTPMethod: "DELETE", Path: "/catalog/items/{id}", File: "CatalogController.java",
				Line: 2, EndLine: 35, Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "controller", Project: "services/catalog", Kind: "symbol",
				Name: "deleteItem", Qualified: "CatalogController.deleteItem",
				File: "CatalogController.java", Line: 2, EndLine: 35,
				Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "service", Project: "services/catalog", Kind: "symbol",
				Name: "deleteItem", Qualified: "CatalogService.deleteItem",
				File: "CatalogService.java", Line: 2, EndLine: 35,
				Confidence: "EXACT", Search: "delete catalog item service",
			},
			{
				ID: "test", Project: "services/catalog", Kind: "test",
				Name: "deletesItem", Qualified: "CatalogControllerTest.deletesItem",
				File: "CatalogControllerTest.java", Line: 2, EndLine: 35,
				Confidence: "EXACT", Search: "delete catalog item test",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "route-controller", FromFactID: "route", ToFactID: "controller", Kind: "call", Confidence: "EXACT"},
			{ID: "controller-service", FromFactID: "controller", ToFactID: "service", Kind: "call", Confidence: "EXACT"},
			{ID: "test-route", FromFactID: "test", ToFactID: "route", Kind: "test_target", Confidence: "EXACT"},
		},
	}
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), index)
	writeSourceFile(t, root, filepath.Join("services/catalog", "CatalogController.java"),
		contextTestMethodSource("CatalogController", "deleteItem", "catalogService.deleteItem();", 30))
	writeSourceFile(t, root, filepath.Join("services/catalog", "CatalogService.java"),
		contextTestMethodSource("CatalogService", "deleteItem", "repository.deleteItem();", 30))
	writeSourceFile(t, root, filepath.Join("services/catalog", "CatalogControllerTest.java"),
		contextTestMethodSource("CatalogControllerTest", "deletesItem", "controller.deleteItem();", 30))

	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "Delete a catalog item through DELETE /catalog/items/{id} and include tests.",
		BudgetTokens: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"CatalogController.java", "CatalogService.java"} {
		found := false
		for _, section := range pack.SourceSections {
			if section.Path == path && section.RenderMode != "signature" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("optional source consumed the enrichment budget for %q: %#v", path, pack.SourceSections)
		}
	}
}

func contextTestMethodSource(typeName, methodName, statement string, repetitions int) string {
	lines := []string{"class " + typeName + " {", "  void " + methodName + "() {"}
	for range repetitions {
		lines = append(lines, "    "+statement)
	}
	lines = append(lines, "  }", "}", "")
	return strings.Join(lines, "\n")
}

func TestBuildContextLeavesUnrenderedSupportConcernsUncovered(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "route", Project: "services/catalog", Kind: "route",
				Name: "DELETE /catalog/items/{id}", HTTPMethod: "DELETE", Path: "/catalog/items/{id}",
				File: "CatalogController.go", Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "service", Project: "services/catalog", Kind: "symbol",
				Name: "deleteItem", File: "CatalogService.go", Confidence: "EXACT", Search: "delete catalog item",
			},
			{
				ID: "contract", Project: "libraries/integration", Kind: "api_contract",
				Name: "DELETE /internal/jobs", Qualified: "JobClient.deleteRelated",
				HTTPMethod: "DELETE", Path: "/internal/jobs", File: "JobClient.go",
				Confidence: "EXACT", Search: "catalog item internal contract",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "route-service", FromFactID: "route", ToFactID: "service", Kind: "call", Confidence: "EXACT",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root:  root,
		Query: "Delete a catalog item. Analyze libraries/integration for the internal contract.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Contracts) != 1 || pack.Contracts[0].ID != "contract" {
		t.Fatalf("support contract was not selected: %#v", pack.Contracts)
	}
	for _, concern := range pack.Concerns {
		key := contextPublicConcernKey(concern)
		if (key == contextConcernHTTPContract || key == contextConcernProject+":libraries/integration") && concern.Covered {
			t.Fatalf("unrendered support concern %q was marked covered: %#v", key, pack.Concerns)
		}
	}
}

func TestContextSourceConcernsMergeSelectedSupportFacts(t *testing.T) {
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{ID: "route", Project: "services/catalog", Kind: "route", File: "Catalog.go"},
		{ID: "contract", Project: "libraries/integration", Kind: "api_contract", File: "Client.go"},
		{ID: "auth", Project: "libraries/integration", Kind: "authentication", File: "ClientSecurity.go"},
		{ID: "repository", Project: "services/jobs", Kind: "persistence", File: "Repository.go"},
	}}
	pack := ContextPack{
		Query: "Delete a catalog item. Analyze libraries/integration and services/jobs for the contract and persistence.",
		Concerns: []ContextConcern{
			{Kind: contextConcernAuth},
			{Kind: contextConcernHTTPContract},
			{Kind: contextConcernPersistence},
			{Kind: contextConcernProject, Project: "libraries/integration"},
			{Kind: contextConcernProject, Project: "services/jobs"},
		},
		Contracts:             []ContextLocation{{ID: "contract"}},
		Persistence:           []ContextLocation{{ID: "repository"}},
		selectedSourceFactIDs: []string{"route", "contract", "auth", "repository"},
	}

	concerns := contextSourceConcerns(pack, index)
	candidates := map[string]map[string]bool{}
	for _, concern := range concerns {
		publicKey := firstNonEmptyContext(concern.publicKey, concern.key)
		if candidates[publicKey] == nil {
			candidates[publicKey] = map[string]bool{}
		}
		for _, factID := range concern.candidateFactIDs {
			candidates[publicKey][factID] = true
		}
	}
	for key, factID := range map[string]string{
		contextConcernAuth:                               "auth",
		contextConcernHTTPContract:                       "contract",
		contextConcernPersistence:                        "repository",
		contextConcernProject + ":libraries/integration": "contract",
		contextConcernProject + ":services/jobs":         "repository",
	} {
		if !candidates[key][factID] {
			t.Fatalf("source concern %q omitted selected support fact %q: %#v", key, factID, concerns)
		}
	}
}

func TestContextSourceOptionConcernsUseRenderedResilienceEvidence(t *testing.T) {
	candidate := sourceCandidate{
		FactID: "contract", FactIDs: []string{"contract"},
		Project: "libraries/integration", Role: "contract",
	}
	section := ContextSourceSection{
		Project: "libraries/integration",
		Path:    "src/main/java/example/JobClient.java",
		Role:    "contract",
		Content: "73\t  @Retryable(maxAttemptsExpression = \"${jobs.max-retries}\")\n" +
			"74\t  public List<Job> listJobs() {",
	}
	concerns := []contextConcern{
		newContextConcern(
			contextConcernResilience,
			"",
			true,
			nil,
			"requested resilience evidence",
		),
	}

	keys, required := contextSourceOptionConcerns(
		candidate,
		section,
		concerns,
		scan.AgentContextIndexRecord{},
	)
	if !required || len(keys) != 1 || keys[0] != contextConcernResilience {
		t.Fatalf("rendered resilience concerns = %v, required %v", keys, required)
	}
}

func TestContextSourceOptionConcernsKeepRenderedEvidenceProjectScoped(t *testing.T) {
	candidate := sourceCandidate{
		FactID: "contract", FactIDs: []string{"contract"},
		Project: "libraries/integration", Role: "contract",
	}
	section := ContextSourceSection{
		Project: "libraries/integration",
		Path:    "src/main/java/example/JobClient.java",
		Role:    "contract",
		Content: "17\t@ConfigurationProperties(prefix = \"jobs\")\n" +
			"18\tpublic class JobClientConfig {",
	}
	concerns := []contextConcern{
		newContextConcern(
			contextConcernConfiguration,
			"services/jobs",
			true,
			nil,
			"requested configuration evidence",
		),
	}

	keys, required := contextSourceOptionConcerns(
		candidate,
		section,
		concerns,
		scan.AgentContextIndexRecord{},
	)
	if required || len(keys) != 0 {
		t.Fatalf("cross-project rendered concerns = %v, required %v", keys, required)
	}
}

func TestContextSourceOptionConcernsRequireRenderedCrossCuttingEvidence(t *testing.T) {
	concern := newContextConcern(
		contextConcernConfiguration,
		"libraries/jobs",
		true,
		[]string{"config"},
		"requested configuration",
	)
	candidate := sourceCandidate{
		FactID: "config", FactIDs: []string{"config"},
		Project: "libraries/jobs", Role: "call_chain",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "config", Project: "libraries/jobs", Kind: contextConcernConfiguration,
	}}}

	signature := ContextSourceSection{
		Project: "libraries/jobs", RenderMode: "signature",
		Content: "public class JobConfig {",
	}
	if keys, _ := contextSourceOptionConcerns(candidate, signature, []contextConcern{concern}, index); len(keys) != 0 {
		t.Fatalf("type-only exact fact covered configuration: %v", keys)
	}

	body := ContextSourceSection{
		Project: "libraries/jobs", RenderMode: "declaration_body",
		Content: "String path = configuration.getJobsPath();",
	}
	if keys, required := contextSourceOptionConcerns(candidate, body, []contextConcern{concern}, index); !required || !reflect.DeepEqual(keys, []string{concern.key}) {
		t.Fatalf("actionable configuration evidence = %v, required %v", keys, required)
	}
}

func TestContextSourceOptionConcernsRequireRenderedExactPersistenceEvidence(t *testing.T) {
	concern := newContextConcern(
		contextConcernPersistence,
		"services/jobs",
		true,
		[]string{"repository"},
		"requested persistence",
	)
	candidate := sourceCandidate{
		FactID: "repository", FactIDs: []string{"repository"},
		Project: "services/jobs", Role: "persistence",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{{
		ID: "repository", Project: "services/jobs", Kind: contextConcernPersistence,
	}}}

	signature := ContextSourceSection{
		Project: "services/jobs", Role: "persistence", RenderMode: "signature",
		Content: "void deleteRelatedJobs();",
	}
	if keys, _ := contextSourceOptionConcerns(candidate, signature, []contextConcern{concern}, index); len(keys) != 0 {
		t.Fatalf("semantically empty exact persistence fact covered persistence: %v", keys)
	}

	body := ContextSourceSection{
		Project: "services/jobs", Role: "persistence", RenderMode: "declaration_body",
		Content: "repository.delete(job);",
	}
	if keys, required := contextSourceOptionConcerns(candidate, body, []contextConcern{concern}, index); !required || !reflect.DeepEqual(keys, []string{concern.key}) {
		t.Fatalf("actionable exact persistence evidence = %v, required %v", keys, required)
	}
}

func TestContextSourceTestsRequireExecutableRenderedBody(t *testing.T) {
	concern := newContextConcern(contextConcernTests, "services/jobs", true, nil, "")
	signature := ContextSourceSection{
		Project: "services/jobs", Role: "test", RenderMode: "signature",
		Content: "@Test\nvoid deletesJob() {",
	}
	if contextSourceSectionSupportsConcern(signature, concern) {
		t.Fatal("test signature counted as executable evidence")
	}

	body := ContextSourceSection{
		Project: "services/jobs", Role: "test", RenderMode: "declaration_body",
		Content: "@Test\nvoid deletesJob() {\n  mockMvc.perform(delete(\"/jobs/1\")).andExpect(status().isNoContent());\n}",
	}
	if !contextSourceSectionSupportsConcern(body, concern) {
		t.Fatal("executable test body was rejected")
	}
}

func TestContextSourceTestsRecognizeLanguageNeutralExecutableBodies(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Go assignment",
			content: "func TestDeleteItem(t *testing.T) {\n  got := deleteItem()\n}",
		},
		{
			name:    "Go call",
			content: "func TestDeleteItem(t *testing.T) {\n  t.Fatalf(\"delete failed\")\n}",
		},
		{
			name:    "Python assignment",
			content: "def test_delete_item():\n    result = delete_item()",
		},
		{
			name:    "Python assertion",
			content: "def test_delete_item():\n    assert result",
		},
		{
			name:    "TypeScript non-empty inline callback",
			content: `test("deletes item", () => { expect(result).toBeDefined(); });`,
		},
		{
			name:    "TypeScript assertion with nested empty callback",
			content: `expect(fn(() => {})).toThrow()`,
		},
		{
			name:    "TypeScript inline wrapper with nested empty callback",
			content: `test("throws", () => { expect(fn(() => {})).toThrow(); });`,
		},
	}
	concern := newContextConcern(contextConcernTests, "services/catalog", true, nil, "")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			section := ContextSourceSection{
				Project:    "services/catalog",
				Role:       "test",
				RenderMode: "declaration_body",
				Content:    test.content,
			}
			if !contextSourceSectionSupportsConcern(section, concern) {
				t.Fatalf("executable test body was rejected: %q", test.content)
			}
		})
	}
}

func TestContextSourceTestsRejectNonExecutableBodies(t *testing.T) {
	tests := []struct {
		name       string
		renderMode string
		content    string
	}{
		{
			name:       "signature",
			renderMode: "signature",
			content:    "func TestDeleteItem(t *testing.T)",
		},
		{
			name:       "empty Java body",
			renderMode: "declaration_body",
			content:    "@Test\nvoid deletesItem() {\n}",
		},
		{
			name:       "empty TypeScript arrow body",
			renderMode: "declaration_body",
			content:    `test("deletes item", () => {});`,
		},
		{
			name:       "empty TypeScript function body",
			renderMode: "declaration_body",
			content:    `test("deletes item", function deletesItem() {});`,
		},
		{
			name:       "empty Go body",
			renderMode: "declaration_body",
			content:    "func TestDeleteItem(t *testing.T) {\n  // no operation\n}",
		},
		{
			name:       "empty Python body",
			renderMode: "declaration_body",
			content:    "def test_delete_item():\n    # no operation\n    pass",
		},
	}
	concern := newContextConcern(contextConcernTests, "services/catalog", true, nil, "")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			section := ContextSourceSection{
				Project:    "services/catalog",
				Role:       "test",
				RenderMode: test.renderMode,
				Content:    test.content,
			}
			if contextSourceSectionSupportsConcern(section, concern) {
				t.Fatalf("non-executable test body was accepted: %q", test.content)
			}
		})
	}
}

func TestContextSourceOptionConcernsRequireExecutableExactTestEvidence(t *testing.T) {
	concern := newContextConcern(
		contextConcernTests,
		"services/jobs",
		true,
		[]string{"job-delete-test"},
		"requested test evidence",
	)
	candidate := sourceCandidate{
		FactID: "job-delete-test", FactIDs: []string{"job-delete-test"},
		Project: "services/jobs", Role: "test",
	}

	signature := ContextSourceSection{
		Project: "services/jobs", Role: "test", RenderMode: "signature",
		Content: "@Test\n@WithJwtTestUser\nvoid deletesJob() {",
	}
	if keys, _ := contextSourceOptionConcerns(candidate, signature, []contextConcern{concern}, scan.AgentContextIndexRecord{}); len(keys) != 0 {
		t.Fatalf("signature-only exact test fact covered tests: %v", keys)
	}

	body := ContextSourceSection{
		Project: "services/jobs", Role: "test", RenderMode: "declaration_body",
		Content: "@Test\n@WithJwtTestUser\nvoid deletesJob() {\n  mockMvc.perform(delete(\"/jobs/1\")).andExpect(status().isNoContent());\n}",
	}
	if keys, required := contextSourceOptionConcerns(candidate, body, []contextConcern{concern}, scan.AgentContextIndexRecord{}); !required || !reflect.DeepEqual(keys, []string{concern.key}) {
		t.Fatalf("executable exact test fact concerns = %v, required %v", keys, required)
	}
}

func TestContextSourceSectionSupportsOperationalConcerns(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		role       string
		renderMode string
		content    string
	}{
		{name: "authentication", kind: contextConcernAuth, content: "authorize.anyRequest().authenticated();"},
		{name: "configuration", kind: contextConcernConfiguration, content: "@ConfigurationProperties(prefix = \"jobs\")"},
		{name: "resilience", kind: contextConcernResilience, content: "@Retryable(maxAttempts = 3)"},
		{name: "persistence", kind: contextConcernPersistence, content: "taskRepository.delete(task);"},
		{name: "side effects", kind: contextConcernSideEffects, content: "protocolService.writeProtocol(id, text);"},
		{name: "tests", kind: contextConcernTests, role: "test", renderMode: "declaration_body", content: "@Test\nvoid deletesJob() {\n  assert true;\n}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			section := ContextSourceSection{
				Project:    "services/jobs",
				Role:       test.role,
				RenderMode: test.renderMode,
				Content:    test.content,
			}
			concern := newContextConcern(test.kind, "services/jobs", true, nil, "")
			if !contextSourceSectionSupportsConcern(section, concern) {
				t.Fatalf("section %q did not support concern %q", test.content, test.kind)
			}
		})
	}

	typeOnly := ContextSourceSection{
		Project: "services/jobs",
		Role:    "call_chain",
		Content: "public class JobMailService extends BaseService {",
	}
	if contextSourceSectionSupportsConcern(
		typeOnly,
		newContextConcern(contextConcernSideEffects, "services/jobs", true, nil, ""),
	) {
		t.Fatal("type-only mail service signature counted as side-effect evidence")
	}
}

func TestContextSourceSectionIgnoresOperationalMarkersOutsideCode(t *testing.T) {
	section := ContextSourceSection{
		Project: "services/jobs",
		Path:    "JobService.java",
		Role:    "call_chain",
		Content: "10\t// @Retryable(maxAttempts = 3)\n" +
			"11\tString example = \"protocolService.writeProtocol(id, text)\";\n" +
			"12\tvoid deleteJob() {}",
	}
	for _, kind := range []string{contextConcernResilience, contextConcernSideEffects} {
		concern := newContextConcern(kind, "services/jobs", true, nil, "")
		if contextSourceSectionSupportsConcern(section, concern) {
			t.Fatalf("non-code marker counted as %q evidence", kind)
		}
	}
}

func TestContextSourceSectionDoesNotTreatTestRoleAsTestEvidence(t *testing.T) {
	section := ContextSourceSection{
		Project: "services/jobs",
		Path:    "JobTestFixture.java",
		Role:    "test",
		Content: "10\tclass JobTestFixture {",
	}
	concern := newContextConcern(contextConcernTests, "services/jobs", true, nil, "")
	if contextSourceSectionSupportsConcern(section, concern) {
		t.Fatal("test-role helper counted as executable test evidence")
	}
}

func TestContextSourceConcernsKeepNonPublicPlannedDuplicatesOptional(t *testing.T) {
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "route", Project: "services/catalog", Kind: "route",
			Name: "DELETE /catalog/items/{id}", HTTPMethod: "DELETE",
			Path: "/catalog/items/{id}", File: "CatalogController.go",
			Search: "delete catalog item",
		},
		{
			ID: "repo-a", Project: "services/jobs-a", Kind: "persistence",
			Name: "findByCatalogId", File: "JobRepositoryA.go",
			Search: "catalog job persistence",
		},
		{
			ID: "repo-b", Project: "services/jobs-b", Kind: "persistence",
			Name: "findByCatalogId", File: "JobRepositoryB.go",
			Search: "catalog job persistence",
		},
	}}
	pack := ContextPack{
		Query: "Delete a catalog item. Analyze services/catalog, services/jobs-a, and services/jobs-b for job persistence.",
		Concerns: []ContextConcern{
			{Kind: contextConcernEntrypoint},
			{Kind: contextConcernPrimaryPath},
			{Kind: contextConcernProject, Project: "services/catalog"},
			{Kind: contextConcernProject, Project: "services/jobs-a"},
			{Kind: contextConcernProject, Project: "services/jobs-b"},
			{Kind: contextConcernPersistence, Project: "services/jobs-a"},
		},
		Entrypoints:           []ContextLocation{{ID: "route"}},
		selectedSourceFactIDs: []string{"route", "repo-a", "repo-b"},
	}

	concerns := contextSourceConcerns(pack, index)
	found := false
	for _, concern := range concerns {
		if concern.key == contextConcernPersistence+":services/jobs-b" {
			found = true
			if concern.required {
				t.Fatalf("non-public planned concern became a source requirement: %#v", concerns)
			}
		}
	}
	if !found {
		t.Fatalf("non-public planned concern was unavailable for optional evidence: %#v", concerns)
	}
}

func TestContextSourceConcernCandidatesDoNotLetOneFileConsumeTheCap(t *testing.T) {
	facts := []scan.AgentContextFactRecord{}
	candidateIDs := []string{}
	for index := range 4 {
		id := fmt.Sprintf("change-%d", index)
		candidateIDs = append(candidateIDs, id)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Project: "services/jobs", Kind: "persistence",
			Name: "findByCatalogId", Qualified: "CatalogChangeJobRepository.findByCatalogId",
			File: "CatalogChangeJobRepository.java", Confidence: "EXACT",
			Search: "catalog job persistence",
		})
	}
	candidateIDs = append(candidateIDs, "regular", "builder")
	facts = append(
		facts,
		scan.AgentContextFactRecord{
			ID: "regular", Project: "services/jobs", Kind: "persistence",
			Name: "findByCatalogId", Qualified: "CatalogJobRepository.findByCatalogId",
			File: "CatalogJobRepository.java", Confidence: "EXACT",
			Search: "catalog job persistence",
		},
		scan.AgentContextFactRecord{
			ID: "builder", Project: "services/jobs", Kind: "persistence",
			Name: "builder", Qualified: "CatalogJobEntity.builder",
			File: "CatalogJobEntity.java", Confidence: "EXACT",
			Search: "catalog job persistence",
		},
	)
	pack := ContextPack{
		Query: "Analyze services/jobs catalog job persistence and lookup attributes.",
	}
	concerns := []contextConcern{
		newContextConcern(
			contextConcernPersistence,
			"services/jobs",
			true,
			candidateIDs,
			"persistence",
		),
	}

	candidates := contextSourceCandidatesForConcerns(
		pack,
		scan.AgentContextIndexRecord{Facts: facts},
		concerns,
	)
	paths := map[string]bool{}
	for _, candidate := range candidates {
		paths[candidate.Path] = true
	}
	for _, path := range []string{"CatalogChangeJobRepository.java", "CatalogJobRepository.java"} {
		if !paths[path] {
			t.Fatalf("source concern cap omitted %q: %#v", path, candidates)
		}
	}
}

func TestContextSourceRoleRecognizesRepositoryOwner(t *testing.T) {
	fact := scan.AgentContextFactRecord{
		ID: "repository-owner", Project: "services/jobs", Kind: "symbol",
		Name: "CatalogJobRepository", Qualified: "example.CatalogJobRepository",
		File: "CatalogJobRepository.java",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{fact}}
	pack := ContextPack{Query: "Analyze catalog job persistence and lookup attributes."}

	if role := contextSourceRole(pack, index, fact); role != contextConcernPersistence {
		t.Fatalf("repository owner role = %q, want persistence", role)
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

func TestSmallestFittingContextSourceOptionPrefersProjectEvidenceQuality(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types and lookup attributes",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	concerns := []contextConcern{
		newContextConcern(contextConcernProject, "services/jobs", true, []string{"model", "generic"}, "project"),
	}
	state := contextSourceSelectionState{
		selectedCandidates: map[string]bool{},
		selectedFactIDs:    map[string]bool{},
		selectedProjects:   map[string]bool{},
		coveredConcerns:    map[string]bool{},
		coveredRoles:       map[string]bool{},
	}
	model := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "model", Project: "services/jobs", Path: "CatalogJobEntity.java",
			Role: "call_chain", Kind: "symbol", Name: "CatalogJobEntity",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "CatalogJobEntity.java", StartLine: 1, EndLine: 4,
			Role: "call_chain", RenderMode: "declaration_body",
			Content: "class CatalogJobEntity {\nlong catalogId;\nlong itemId;\n}",
		},
		estimated:   100,
		concernKeys: []string{contextConcernProject + ":services/jobs"},
		projectKey:  "services/jobs",
	}
	generic := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "generic", Project: "services/jobs", Path: "MailProperties.java",
			Role: "call_chain", Kind: "configuration", Name: "MailProperties",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "MailProperties.java", StartLine: 1, EndLine: 1,
			Role: "call_chain", RenderMode: "signature", Content: "enum MailProperties",
		},
		estimated:   10,
		concernKeys: []string{contextConcernProject + ":services/jobs"},
		projectKey:  "services/jobs",
	}

	got, ok, err := smallestFittingContextSourceOption(
		pack,
		request,
		[]contextSourceOption{generic, model},
		concerns,
		state,
		contextSourceBoundary{project: "services/jobs"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.candidate.FactID != "model" {
		t.Fatalf("project boundary selected %#v, want informative model", got)
	}

	modelSignature := model
	modelSignature.section.RenderMode = "signature"
	modelSignature.section.Content = "class CatalogJobEntity"
	modelSignature.estimated = 10
	modelSignature.concernKeys = nil
	got, ok, err = smallestFittingContextSourceOption(
		pack,
		request,
		[]contextSourceOption{model, modelSignature},
		concerns,
		state,
		contextSourceBoundary{factID: "model"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.section.RenderMode != "declaration_body" {
		t.Fatalf("exact boundary did not prefer required concern evidence: %#v", got)
	}
}

func TestSmallestFittingContextSourceOptionPrefersActionableFactEvidence(t *testing.T) {
	concern := newContextConcern(
		contextConcernConfiguration,
		"libraries/jobs",
		true,
		[]string{"config"},
		"requested configuration",
	)
	candidate := sourceCandidate{
		FactID: "config", FactIDs: []string{"config"},
		Project: "libraries/jobs", Path: "JobConfig.java",
	}
	signature := contextSourceOption{
		candidate: candidate,
		section:   ContextSourceSection{Project: "libraries/jobs", Path: "JobConfig.java", RenderMode: "signature", Content: "class JobConfig {"},
		estimated: 10, projectKey: "libraries/jobs",
	}
	body := contextSourceOption{
		candidate: candidate,
		section:   ContextSourceSection{Project: "libraries/jobs", Path: "JobConfig.java", RenderMode: "declaration_body", Content: "String path = configuration.getJobsPath();"},
		estimated: 30, projectKey: "libraries/jobs",
		concernKeys: []string{concern.key}, required: true,
	}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{},
		coveredConcerns:          map[string]bool{},
		coveredRoles:             map[string]bool{},
		selectedEvidenceFamilies: map[string]int{},
	}
	got, found, err := smallestFittingContextSourceOption(
		ContextPack{Schema: 1, Query: "jobs configuration", BudgetTokens: DefaultContextBudgetTokens},
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens, MaxFiles: DefaultContextMaxFiles},
		[]contextSourceOption{signature, body},
		[]contextConcern{concern},
		state,
		contextSourceBoundary{factID: "config"},
	)
	if err != nil || !found || got.section.RenderMode != "declaration_body" {
		t.Fatalf("mandatory actionable option = %#v, found %v, err %v", got, found, err)
	}
}

func TestContextSourceUtilityPrefersDomainEvidenceOverGenericSignatures(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types and lookup attributes",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	concerns := []contextConcern{
		newContextConcern(contextConcernDomainModel, "", true, []string{"model"}, "models"),
		newContextConcern(contextConcernProject, "services/jobs", true, []string{"model", "generic"}, "project"),
	}
	state := contextSourceSelectionState{
		selectedCandidates: map[string]bool{},
		selectedFactIDs:    map[string]bool{},
		selectedProjects:   map[string]bool{},
		coveredConcerns:    map[string]bool{},
		coveredRoles:       map[string]bool{},
	}
	model := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "model", Project: "services/jobs", Path: "CatalogJobEntity.java",
			Role: "call_chain", Kind: "symbol", Name: "CatalogJobEntity",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "CatalogJobEntity.java", StartLine: 1, EndLine: 4,
			Role: "call_chain", RenderMode: "declaration_body",
			Content: "class CatalogJobEntity {\nlong catalogId;\nlong itemId;\n}",
		},
		estimated: 120, concernKeys: []string{
			contextConcernDomainModel,
			contextConcernProject + ":services/jobs",
		},
		projectKey: "services/jobs",
	}
	generic := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "generic", Project: "services/jobs", Path: "MailProperties.java",
			Role: "call_chain", Kind: "configuration", Name: "MailProperties",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "MailProperties.java", StartLine: 1, EndLine: 1,
			Role: "call_chain", RenderMode: "signature", Content: "enum MailProperties",
		},
		estimated:   10,
		concernKeys: []string{contextConcernProject + ":services/jobs"},
		projectKey:  "services/jobs",
	}

	got, _, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{generic, model},
		concerns,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.candidate.FactID != "model" {
		t.Fatalf("source utility selected %#v, want domain model", got)
	}
}

func TestContextSourceUtilityRetainsSecondDomainAndPersistenceEvidence(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types lookup attributes persistence",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	state := contextSourceSelectionState{
		selectedCandidates: map[string]bool{},
		selectedFactIDs:    map[string]bool{},
		selectedProjects:   map[string]bool{"services/jobs": true},
		coveredConcerns: map[string]bool{
			contextConcernDomainModel:                true,
			contextConcernProject + ":services/jobs": true,
			contextConcernPersistence:                true,
		},
		coveredRoles: map[string]bool{
			"call_chain":  true,
			"persistence": true,
		},
	}
	option := func(id, path, role, kind, name, mode, content string, estimated int) contextSourceOption {
		return contextSourceOption{
			candidate: sourceCandidate{
				FactID: id, Project: "services/jobs", Path: path,
				Role: role, Kind: kind, Name: name, Qualified: name,
			},
			section: ContextSourceSection{
				Project: "services/jobs", Path: path, StartLine: 1, EndLine: 2,
				Role: role, RenderMode: mode, Content: content,
			},
			estimated: estimated, projectKey: "services/jobs",
		}
	}
	secondModel := option(
		"change-model", "CatalogChangeJobEntity.java", "call_chain", "symbol",
		"CatalogChangeJobEntity", "declaration_body",
		"class CatalogChangeJobEntity { long changeId; }", 100,
	)
	genericConfig := option(
		"generic-config", "MailProperties.java", "call_chain", "configuration",
		"MailProperties", "signature", "enum MailProperties", 10,
	)
	got, _, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{genericConfig, secondModel},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.candidate.FactID != "change-model" {
		t.Fatalf("second domain model lost to generic signature: %#v", got)
	}

	declaredRepository := option(
		"change-repository", "CatalogChangeJobRepository.java", "persistence", "persistence",
		"CatalogChangeJobRepository.findByCatalogIdAndItemId", "declaration_body",
		"findByCatalogIdAndItemId(long catalogId, long itemId)", 60,
	)
	genericRepository := option(
		"generic-repository", "ARepository.java", "persistence", "persistence",
		"findAll", "signature", "findAll()", 10,
	)
	got, _, found, err = contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{genericRepository, declaredRepository},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.candidate.FactID != "change-repository" {
		t.Fatalf("second declared repository lost to generic persistence: %#v", got)
	}
}

func TestAddContextSourceOptionReusesIdenticalRenderedSection(t *testing.T) {
	section := ContextSourceSection{
		Project: "libraries/client", Path: "JobClient.java",
		StartLine: 10, EndLine: 15, Role: "contract",
		RenderMode: "declaration_body", Content: "void listJobs() {}",
	}
	pack := ContextPack{
		Schema: 1, Query: "job client configuration", Confidence: "EXACT",
		BudgetTokens:   DefaultContextBudgetTokens,
		SourceSections: []ContextSourceSection{section},
	}
	option := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "config", Project: "libraries/client", Path: "JobClient.java",
			Role: "call_chain", Kind: "configuration", Name: "listJobs",
		},
		section: ContextSourceSection{
			Project: "libraries/client", Path: "JobClient.java",
			StartLine: 10, EndLine: 15, Role: "call_chain",
			RenderMode: "declaration_body", Content: "void listJobs() {}",
		},
		estimated:   30,
		concernKeys: []string{contextConcernConfiguration + ":libraries/client"},
		projectKey:  "libraries/client",
	}
	concerns := []contextConcern{
		newContextConcern(
			contextConcernConfiguration,
			"libraries/client",
			true,
			[]string{"config"},
			"configuration",
		),
	}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{"libraries/client": true},
		coveredConcerns:          map[string]bool{},
		coveredRoles:             map[string]bool{"contract": true},
		selectedEvidenceFamilies: map[string]int{},
	}

	got, gotState, err := addContextSourceOption(
		pack,
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens},
		option,
		concerns,
		state,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceSections) != 1 {
		t.Fatalf("identical rendered source was duplicated: %#v", got.SourceSections)
	}
	if !gotState.coveredConcerns[contextConcernConfiguration+":libraries/client"] {
		t.Fatalf("reused source did not cover its additional concern: %#v", gotState.coveredConcerns)
	}
}

func TestAddContextSourceOptionCountsOnlySpecializedEvidenceRoles(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "job persistence", Confidence: "EXACT",
		BudgetTokens: DefaultContextBudgetTokens,
	}
	option := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "service", Project: "services/jobs", Path: "JobService.java",
			Role: "call_chain", Kind: "persistence", Name: "loadJobs",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "JobService.java",
			StartLine: 1, EndLine: 2, Role: "call_chain",
			RenderMode: "declaration_body", Content: "void loadJobs() {}",
		},
		estimated: 30, projectKey: "services/jobs",
		evidenceFamily: contextConcernPersistence, profiled: true,
	}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{},
		coveredConcerns:          map[string]bool{},
		coveredRoles:             map[string]bool{},
		selectedEvidenceFamilies: map[string]int{},
	}

	_, gotState, err := addContextSourceOption(
		pack,
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens},
		option,
		nil,
		state,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := gotState.selectedEvidenceFamilies["services/jobs\x00"+contextConcernPersistence]; got != 0 {
		t.Fatalf("call-chain evidence consumed %d persistence slots", got)
	}
}

func TestContextSourceUtilitySkipsCoveredCrossCuttingEvidence(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types lookup attributes configuration",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	state := contextSourceSelectionState{
		selectedCandidates: map[string]bool{},
		selectedFactIDs:    map[string]bool{},
		selectedProjects:   map[string]bool{"libraries/client": true},
		coveredConcerns: map[string]bool{
			contextConcernConfiguration + ":libraries/client": true,
			contextConcernProject + ":libraries/client":       true,
		},
		coveredRoles: map[string]bool{"call_chain": true},
		selectedEvidenceFamilies: map[string]int{
			"libraries/client\x00" + contextConcernConfiguration: 1,
		},
	}
	generic := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "generic-config", Project: "libraries/client",
			Path: "MailProperties.java", Role: "call_chain",
			Kind: "configuration", Name: "MailProperties",
		},
		section: ContextSourceSection{
			Project: "libraries/client", Path: "MailProperties.java",
			StartLine: 1, EndLine: 12, Role: "call_chain",
			RenderMode: "declaration_body", Content: "enum MailProperties { TO, CC, BCC }",
		},
		estimated: 80, projectKey: "libraries/client",
		evidenceFamily: contextConcernConfiguration, quality: 220, profiled: true,
	}

	got, utility, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{generic},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("covered cross-cutting option retained with utility %d: %#v", utility, got)
	}
}

func TestContextSourceUtilityPrefersPrimaryModelOverDependentModel(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types and lookup attributes",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{},
		coveredConcerns:          map[string]bool{contextConcernDomainModel: true},
		coveredRoles:             map[string]bool{contextConcernDomainModel: true},
		selectedEvidenceFamilies: map[string]int{},
	}
	option := func(id, project, name string) contextSourceOption {
		return contextSourceOption{
			candidate: sourceCandidate{
				FactID: id, Project: project, Path: name + ".java",
				Role: contextConcernDomainModel, Kind: "symbol",
				Name: name, Qualified: "example." + name,
			},
			section: ContextSourceSection{
				Project: project, Path: name + ".java",
				StartLine: 1, EndLine: 1, Role: contextConcernDomainModel,
				RenderMode: "signature", Content: "class " + name,
			},
			estimated: 20, projectKey: project,
		}
	}
	dependent := option("comment", "services/a", "CatalogJobCommentEntity")
	primary := option("change", "services/b", "CatalogChangeJobEntity")

	got, _, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{dependent, primary},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.candidate.FactID != "change" {
		t.Fatalf("dependent model displaced a primary domain type: %#v", got)
	}
}

func TestContextSourceUtilityPrefersPrimaryRepositoryOverDependency(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job persistence and lookup attributes",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{"services/jobs": true},
		coveredConcerns:          map[string]bool{contextConcernPersistence: true},
		coveredRoles:             map[string]bool{},
		selectedEvidenceFamilies: map[string]int{},
	}
	option := func(id, name string) contextSourceOption {
		return contextSourceOption{
			candidate: sourceCandidate{
				FactID: id, Project: "services/jobs", Path: name + ".java",
				Role: contextConcernPersistence, Kind: contextConcernPersistence,
				Name: "findByCatalogId", Qualified: name + ".findByCatalogId",
			},
			section: ContextSourceSection{
				Project: "services/jobs", Path: name + ".java",
				StartLine: 1, EndLine: 1, Role: contextConcernPersistence,
				RenderMode: "signature", Content: "findByCatalogId(long catalogId)",
			},
			estimated: 20, projectKey: "services/jobs",
		}
	}
	dependent := option("comment", "CatalogJobCommentRepository")
	primary := option("regular", "CatalogJobRepository")
	view := option("view", "UserJobVRepository")
	primaryQuality := contextSourceCandidateQuality(
		pack,
		scan.AgentContextIndexRecord{},
		primary,
	)
	viewQuality := contextSourceCandidateQuality(
		pack,
		scan.AgentContextIndexRecord{},
		view,
	)
	if primaryQuality <= viewQuality {
		t.Fatalf("primary repository quality %d <= derived view %d", primaryQuality, viewQuality)
	}

	got, _, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{dependent, primary, view},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.candidate.FactID != "regular" {
		t.Fatalf("dependent repository displaced a primary owner: %#v", got)
	}
}

func TestContextSourceOptionQualityPenalizesCrossCuttingTypeSignature(t *testing.T) {
	pack := ContextPack{
		Query: "Delete catalog jobs. Cover task types, fields, mail, protocol, and side effects.",
	}
	fact := scan.AgentContextFactRecord{
		ID: "mail-type", Project: "services/jobs", Kind: "symbol",
		Name: "CatalogJobMailService", Qualified: "example.CatalogJobMailService",
		File: "CatalogJobMailService.java", Confidence: "EXACT",
		Search: "catalog job mail side effects",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{fact}}
	option := contextSourceOption{
		candidate: sourceCandidate{
			FactID: fact.ID, FactIDs: []string{fact.ID},
			Project: fact.Project, Path: fact.File, Role: "call_chain",
			Kind: fact.Kind, Name: fact.Name, Qualified: fact.Qualified,
		},
		section: ContextSourceSection{
			Project: fact.Project, Path: fact.File, Role: "call_chain",
			RenderMode: "signature", Content: "public class CatalogJobMailService {",
		},
		concernKeys: []string{contextConcernSideEffects + ":services/jobs"},
	}

	candidateQuality := contextSourceCandidateQuality(pack, index, option)
	optionQuality := contextSourceOptionQuality(pack, index, option)
	if optionQuality >= candidateQuality {
		t.Fatalf(
			"cross-cutting type signature quality = %d, want below candidate quality %d",
			optionQuality,
			candidateQuality,
		)
	}
}

func TestContextSourceUtilitySkipsWeakOptionalPersistence(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job persistence and lookup attributes",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{"libraries/common": true},
		coveredConcerns:          map[string]bool{contextConcernPersistence: true},
		coveredRoles:             map[string]bool{contextConcernPersistence: true},
		selectedEvidenceFamilies: map[string]int{},
	}
	weak := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "protocol", Project: "libraries/common",
			Path: "CatalogProtocolRepository.java",
			Role: contextConcernPersistence, Kind: "symbol",
			Name:      "CatalogProtocolRepository",
			Qualified: "example.CatalogProtocolRepository",
		},
		section: ContextSourceSection{
			Project: "libraries/common", Path: "CatalogProtocolRepository.java",
			StartLine: 1, EndLine: 2, Role: contextConcernPersistence,
			RenderMode: "declaration_body",
			Content:    "interface CatalogProtocolRepository extends Repository<ProtocolEntity> {}",
		},
		estimated: 40, projectKey: "libraries/common",
	}

	got, utility, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{weak},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("weak optional persistence retained with utility %d: %#v", utility, got)
	}
}

func TestContextSourceUtilitySkipsOptionalRepositoryWithoutModelPair(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job persistence and lookup attributes",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{"services/jobs": true},
		coveredConcerns:          map[string]bool{contextConcernPersistence: true},
		coveredRoles:             map[string]bool{},
		selectedEvidenceFamilies: map[string]int{},
	}
	adjacent := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "state", Project: "services/jobs",
			Path: "CatalogJobStateRepository.java",
			Role: contextConcernPersistence, Kind: "symbol",
			Name:      "CatalogJobStateRepository",
			Qualified: "example.CatalogJobStateRepository",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "CatalogJobStateRepository.java",
			StartLine: 1, EndLine: 2, Role: contextConcernPersistence,
			RenderMode: "declaration_body",
			Content:    "interface CatalogJobStateRepository extends Repository<StateEntity> {}",
		},
		estimated: 40, projectKey: "services/jobs",
		evidenceFamily: contextConcernPersistence, stableMatches: 3,
		quality: 700, profiled: true,
	}

	got, utility, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{adjacent},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("unpaired optional persistence retained with utility %d: %#v", utility, got)
	}
}

func TestContextSourceUtilitySkipsUnrequestedDomainModel(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types and lookup attributes",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	request := ContextRequest{BudgetTokens: DefaultContextBudgetTokens}
	state := contextSourceSelectionState{
		selectedCandidates:       map[string]bool{},
		selectedFactIDs:          map[string]bool{},
		selectedProjects:         map[string]bool{"services/jobs": true},
		coveredConcerns:          map[string]bool{contextConcernDomainModel: true},
		coveredRoles:             map[string]bool{},
		selectedEvidenceFamilies: map[string]int{},
	}
	comment := contextSourceOption{
		candidate: sourceCandidate{
			FactID: "comment", Project: "services/jobs",
			Path: "CatalogJobCommentEntity.java",
			Role: contextConcernDomainModel, Kind: "symbol",
			Name: "CatalogJobCommentEntity",
		},
		section: ContextSourceSection{
			Project: "services/jobs", Path: "CatalogJobCommentEntity.java",
			StartLine: 1, EndLine: 2, Role: contextConcernDomainModel,
			RenderMode: "declaration_body",
			Content:    "class CatalogJobCommentEntity {}",
		},
		estimated: 30, projectKey: "services/jobs",
		evidenceFamily: contextConcernDomainModel, stableMatches: 3,
		quality: 700, profiled: true,
	}

	got, utility, found, err := contextSourceUtilityOption(
		pack,
		request,
		[]contextSourceOption{comment},
		nil,
		state,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("unrequested domain model retained with utility %d: %#v", utility, got)
	}
}

func TestContextSourceCandidateQualityPrefersRepositoryMatchingPrimaryModel(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types, lookup attributes, and persistence",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "model", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobEntity", File: "CatalogJobEntity.java", Confidence: "EXACT",
		},
		{
			ID: "primary", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobRepository", File: "CatalogJobRepository.java", Confidence: "EXACT",
		},
		{
			ID: "state", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobStateRepository", File: "CatalogJobStateRepository.java", Confidence: "EXACT",
		},
	}}
	option := func(id, name string) contextSourceOption {
		return contextSourceOption{
			candidate: sourceCandidate{
				FactID: id, FactIDs: []string{id}, Project: "services/jobs",
				Path: name + ".java", Role: contextConcernPersistence,
				Kind: "symbol", Name: name, Qualified: "example." + name,
			},
			section: ContextSourceSection{
				Project: "services/jobs", Path: name + ".java",
				Role: contextConcernPersistence, RenderMode: "signature",
				Content: "interface " + name,
			},
			projectKey: "services/jobs",
		}
	}
	primary := contextSourceCandidateQuality(pack, index, option("primary", "CatalogJobRepository"))
	state := contextSourceCandidateQuality(pack, index, option("state", "CatalogJobStateRepository"))
	if primary <= state {
		t.Fatalf("matching repository quality %d <= adjacent owner %d", primary, state)
	}
}

func TestPersistencePairUsesSelectedDomainModels(t *testing.T) {
	pack := ContextPack{
		Schema: 1, Query: "catalog job task types, lookup attributes, and persistence",
		Confidence:            "EXACT",
		BudgetTokens:          DefaultContextBudgetTokens,
		selectedSourceFactIDs: []string{"requested-model"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "requested-model", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobEntity", File: "CatalogJobEntity.java", Confidence: "EXACT",
		},
		{
			ID: "adjacent-model", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogTopicEntity", File: "CatalogTopicEntity.java", Confidence: "EXACT",
		},
	}}
	requested := scan.AgentContextFactRecord{
		Project: "services/jobs", Kind: "symbol",
		Name: "CatalogJobRepository", File: "CatalogJobRepository.java",
	}
	adjacent := scan.AgentContextFactRecord{
		Project: "services/jobs", Kind: "symbol",
		Name: "CatalogTopicRepository", File: "CatalogTopicRepository.java",
	}

	if !contextPersistenceMatchesSelectedDomainModel(pack, index, requested) {
		t.Fatal("repository matching the selected domain model was rejected")
	}
	if contextPersistenceMatchesSelectedDomainModel(pack, index, adjacent) {
		t.Fatal("repository matching only an unselected adjacent model was retained")
	}
}

func TestContextSourceOptionsRecoverFromStaleMergedAnchor(t *testing.T) {
	root := t.TempDir()
	lines := numberedSourceLines(12)
	lines[9] = "func currentNeighbor() { repository.delete() }"
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

func TestContextInheritedOwnerCandidateDeduplicatesEquivalentOwners(t *testing.T) {
	candidate := sourceCandidate{
		FactID: "find-all", FactIDs: []string{"find-all"},
		Project: "services/jobs", Path: "CatalogJobRepository.java",
		StartLine: 17, Role: contextConcernPersistence,
		Kind: contextConcernPersistence, Name: "findAll",
		Qualified: "CatalogJobRepository.findAll",
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "find-all", Project: "services/jobs", Kind: contextConcernPersistence,
			Name: "findAll", Qualified: "CatalogJobRepository.findAll",
			File: "CatalogJobRepository.java", Line: 17,
		},
		{
			ID: "owner-short", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobRepository", Qualified: "CatalogJobRepository",
			File: "CatalogJobRepository.java", Line: 17,
		},
		{
			ID: "owner-full", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobRepository", Qualified: "example.CatalogJobRepository",
			File: "CatalogJobRepository.java", Line: 17, Confidence: "EXACT",
		},
	}}

	owner, ok := contextInheritedOwnerCandidate(index, candidate)
	if !ok {
		t.Fatal("equivalent indexed owner declarations were treated as ambiguous")
	}
	if owner.Name != "CatalogJobRepository" ||
		owner.Qualified != "CatalogJobRepository" ||
		owner.StartLine != 17 ||
		owner.SourceState != "inherited_owner_current" {
		t.Fatalf("inherited owner = %#v", owner)
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

func TestContextSourceOptionsIncludeUnselectedRequiredConcernEvidence(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "services/catalog/CatalogController.go", "package catalog\n\nfunc listCatalog() {}\n")
	writeSourceFile(
		t,
		root,
		"libraries/client/ClientConfig.go",
		"package client\n\ntype ClientConfig struct {\n\tTimeout int\n}\n",
	)
	pack := ContextPack{
		Schema:       1,
		Query:        "GET /catalog. Analyze libraries/client job client configuration.",
		Confidence:   "EXACT",
		BudgetTokens: DefaultContextBudgetTokens,
		Concerns: []ContextConcern{
			{Kind: contextConcernEntrypoint, Covered: true},
			{Kind: contextConcernConfiguration, Project: "libraries/client", Covered: false},
		},
		Entrypoints: []ContextLocation{{
			ID: "route", Project: "services/catalog", File: "CatalogController.go",
		}},
		selectedSourceFactIDs: []string{"route"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "route", Project: "services/catalog", Kind: "route", Name: "listCatalog",
			Qualified:  "listCatalog",
			HTTPMethod: "GET", Path: "/catalog", File: "CatalogController.go",
			Line: 3, EndLine: 3, Search: "catalog", Confidence: "EXACT",
		},
		{
			ID: "client-config", Project: "libraries/client", Kind: "symbol", Name: "ClientConfig",
			Qualified: "client.ClientConfig", File: "ClientConfig.go",
			Line: 3, EndLine: 3, Search: "job client configuration", Confidence: "EXACT",
		},
	}}

	got, err := attachContextSource(
		pack,
		loadedContextIndex{ScopeRoot: root, Workspace: true, Index: index},
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens, MaxFiles: DefaultContextMaxFiles},
	)
	if err != nil {
		t.Fatal(err)
	}
	foundConfig := false
	for _, section := range got.SourceSections {
		foundConfig = foundConfig ||
			section.Project == "libraries/client" && section.Path == "ClientConfig.go"
	}
	if !foundConfig {
		t.Fatalf("required unselected concern source missing: %#v", got.SourceSections)
	}
	if got.SourceCoverage != "complete" {
		t.Fatalf(
			"source coverage = %q, omissions %#v, sections %#v",
			got.SourceCoverage,
			got.SourceOmissions,
			got.SourceSections,
		)
	}
}

func TestContextCoreSourceBoundariesFollowEndpointHandlerCall(t *testing.T) {
	pack := ContextPack{
		Endpoints: []ContextEndpoint{{
			Provider: "services/catalog", HTTPMethod: "DELETE", Path: "/catalog/{id}",
			Handler: "CatalogController.deleteItem", File: "CatalogController.java", Line: 20,
		}},
		selectedSourceFactIDs: []string{"decoy", "endpoint", "handler", "service"},
	}
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "decoy", Project: "services/jobs", Kind: "route",
				Name: "deleteJob", Qualified: "JobController.deleteJob",
				File: "JobController.java", Line: 15,
			},
			{
				ID: "endpoint", Project: "services/catalog", Kind: "api_endpoint",
				Qualified: "CatalogController.deleteItem", HTTPMethod: "DELETE", Path: "/catalog/{id}",
				File: "CatalogController.java", Line: 20,
			},
			{
				ID: "handler", Project: "services/catalog", Kind: "symbol",
				Name: "deleteItem", Qualified: "CatalogController.deleteItem",
				File: "CatalogController.java", Line: 20,
			},
			{
				ID: "service", Project: "services/catalog", Kind: "symbol",
				Name: "deleteItem", Qualified: "CatalogService.deleteItem",
				File: "CatalogService.java", Line: 40,
			},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "handler-service", FromFactID: "handler", ToFactID: "service", Kind: "call",
		}},
	}

	boundaries := contextCoreSourceBoundaries(pack, index, map[string]int{"decoy": 0})
	foundService := false
	for _, boundary := range boundaries {
		foundService = foundService || boundary.factID == "service"
	}
	if !foundService {
		t.Fatalf("endpoint handler call target missing from core boundaries: %#v", boundaries)
	}
}

func TestContextCoreSourceBoundariesIncludeSelectedRelatedProjectFile(t *testing.T) {
	pack := ContextPack{
		Endpoints: []ContextEndpoint{{
			Provider: "services/catalog", HTTPMethod: "DELETE", Path: "/catalog/{id}",
			Handler: "CatalogController.deleteItem", File: "CatalogController.java", Line: 20,
		}},
		Files: []ContextFile{{
			Project: "services/jobs", Path: "JobManagementController.java",
			StartLine: 30, EndLine: 30, Role: "related_project",
		}},
		selectedSourceFactIDs: []string{"handler", "jobs-controller"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "handler", Project: "services/catalog", Kind: "symbol",
			Name: "deleteItem", Qualified: "CatalogController.deleteItem",
			File: "CatalogController.java", Line: 20,
		},
		{
			ID: "jobs-controller", Project: "services/jobs", Kind: "symbol",
			Name: "listJobs", Qualified: "JobManagementController.listJobs",
			File: "JobManagementController.java", Line: 30,
		},
	}}

	boundaries := contextCoreSourceBoundaries(pack, index, nil)
	foundRelated := false
	for _, boundary := range boundaries {
		foundRelated = foundRelated || boundary.factID == "jobs-controller"
	}
	if !foundRelated {
		t.Fatalf("selected related project file missing from core boundaries: %#v", boundaries)
	}
}

func TestContextCoreSourceBoundariesIncludeSelectedContract(t *testing.T) {
	pack := ContextPack{
		Entrypoints: []ContextLocation{{
			ID: "handler", Project: "services/catalog", File: "CatalogController.java",
		}},
		Contracts: []ContextLocation{{
			ID: "job-client", Project: "libraries/jobs", File: "JobClient.java",
		}},
		selectedSourceFactIDs: []string{"handler", "job-client"},
	}
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "handler", Project: "services/catalog", Kind: "symbol",
			Name: "deleteItem", File: "CatalogController.java", Line: 20,
		},
		{
			ID: "job-client", Project: "libraries/jobs", Kind: "api_contract",
			Name: "GET /jobs", Qualified: "JobClient.listJobs", File: "JobClient.java", Line: 30,
		},
	}}

	boundaries := contextCoreSourceBoundaries(pack, index, nil)
	foundContract := false
	for _, boundary := range boundaries {
		foundContract = foundContract || boundary.factID == "job-client"
	}
	if !foundContract {
		t.Fatalf("selected contract missing from core boundaries: %#v", boundaries)
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
			selectedSourceFactIDs: []string{"route", "provider", "provider-backup"},
		}
		loaded := loadedContextIndex{ScopeRoot: root, Index: scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "app", Kind: "symbol", Name: "route", File: "route.go", Line: 1, EndLine: 1},
			{ID: "provider", Project: "services/provider", Kind: "symbol", Name: "provider", File: "Provider.go", Line: 1, EndLine: 1},
			{ID: "provider-backup", Project: "services/provider", Kind: "symbol", Name: "providerBackup", File: "ProviderBackup.go", Line: 1, EndLine: 1},
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
		if got.SourceUnrepresented != 1 {
			t.Fatalf("source unrepresented = %d, want one uncovered required concern", got.SourceUnrepresented)
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
		t.Skipf("symlink creation is not permitted in this environment: %v", err)
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
		t.Skipf("symlink creation is not permitted in this environment: %v", err)
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
