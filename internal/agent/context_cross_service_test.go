package agent

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestBuildContextReplacesCrossServiceDiscovery(t *testing.T) {
	root := writeCrossServiceContextFixture(t, ".java")
	query := "When a catalog item is deleted through DELETE /catalogs/{catalogId}/items/{itemId}, analyze services/catalog, libraries/shared-model, and services/jobs. Cover the endpoint, current and required chain, contract, authentication, configuration, persistence, and tests."

	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackRequired || len(pack.Entrypoints) != 1 || pack.Entrypoints[0].ID != "route" {
		t.Fatalf("primary entrypoint = %#v", pack)
	}
	for _, project := range []string{"services/catalog", "libraries/shared-model", "services/jobs"} {
		if !contextPackHasProductionSource(pack, project) {
			t.Fatalf("missing production source for %s: %#v", project, pack.SourceSections)
		}
	}
	if !contextPackHasRelationshipKind(pack, "http_contract") {
		t.Fatalf("cross-service contract path missing: %#v", pack.CallChain)
	}
	if contextPackTestPrecedesProduction(pack, []string{"services/catalog", "libraries/shared-model", "services/jobs"}) {
		t.Fatalf("test source displaced required production: %#v", pack.SourceSections)
	}
	if pack.SourceCoverage != "complete" || pack.EstimatedTokens > 4000 {
		t.Fatalf("source coverage/budget = %q/%d", pack.SourceCoverage, pack.EstimatedTokens)
	}
}

func TestContextSourceProductionBeforeTests(t *testing.T) {
	root := writeCrossServiceContextFixture(t, ".java")
	query := "When a catalog item is deleted through DELETE /catalogs/{catalogId}/items/{itemId}, analyze services/catalog, libraries/shared-model, and services/jobs. Cover the endpoint, current and required chain, contract, authentication, configuration, persistence, and tests."

	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{"services/catalog", "libraries/shared-model", "services/jobs"} {
		if !contextPackHasProductionSource(pack, project) {
			t.Fatalf("missing required production source for %s: %#v", project, pack.SourceSections)
		}
	}
	if contextPackTestPrecedesProduction(pack, []string{"services/catalog", "libraries/shared-model", "services/jobs"}) {
		t.Fatalf("test source precedes required production: %#v", pack.SourceSections)
	}
	hasRequestedTest := false
	for _, section := range pack.SourceSections {
		if section.Role == "test" {
			hasRequestedTest = true
			break
		}
	}
	if !hasRequestedTest {
		t.Fatalf("requested test source was not selected after production: %#v", pack.SourceSections)
	}
	for _, concern := range pack.Concerns {
		if !concern.Covered {
			t.Fatalf("required concern lacks current source: %#v", concern)
		}
	}
	if pack.SourceCoverage != "complete" {
		t.Fatalf("required production pack source coverage = %q: %#v", pack.SourceCoverage, pack.SourceOmissions)
	}
	body, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if pack.EstimatedTokens > DefaultContextBudgetTokens || len(body) > MaxContextBytes ||
		len(pack.SourceSections) > MaxContextSourceSections {
		t.Fatalf("source package bounds: tokens=%d bytes=%d sections=%d", pack.EstimatedTokens, len(body), len(pack.SourceSections))
	}
	t.Logf("source package: tokens=%d bytes=%d sections=%d", pack.EstimatedTokens, len(body), len(pack.SourceSections))
}

func TestContextSelectionIsLanguageNeutral(t *testing.T) {
	query := "When a catalog item is deleted through DELETE /catalogs/{catalogId}/items/{itemId}, analyze services/catalog, libraries/shared-model, and services/jobs. Cover the endpoint, current and required chain, contract, authentication, configuration, persistence, and tests."
	wantFactIDs := []string{
		"change-repository", "client", "contract", "operations", "provider",
		"regular-repository", "route", "service", "test",
	}
	wantConcernKeys := []string{
		"authentication", "entrypoint", "http_contract", "persistence", "primary_path",
		"project:libraries/shared-model", "project:services/catalog", "project:services/jobs",
	}

	var baseline contextSelectionSnapshot
	for _, extension := range []string{".java", ".go", ".ts", ".py"} {
		root := writeCrossServiceContextFixture(t, extension)
		pack, err := BuildContext(ContextRequest{Root: root, Query: query})
		if err != nil {
			t.Fatal(err)
		}
		got := contextSelectionSnapshotForPack(pack)
		if !reflect.DeepEqual(got.FactIDs, wantFactIDs) ||
			!reflect.DeepEqual(got.ConcernKeys, wantConcernKeys) ||
			got.SourceCoverage != "complete" {
			t.Fatalf("extension %s selection = %#v, want fact IDs %#v, concern keys %#v, and complete source coverage", extension, got, wantFactIDs, wantConcernKeys)
		}
		if extension == ".java" {
			baseline = got
			continue
		}
		if !reflect.DeepEqual(got, baseline) {
			t.Fatalf("extension %s selection diverged from .java: %#v != %#v", extension, got, baseline)
		}
	}
}

func TestSelectContextPathsKeepsDisconnectedExplicitProjectAsRelatedProduction(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "DELETE /catalog/items/{id}", HTTPMethod: "DELETE", Path: "/catalog/items/{id}", File: "CatalogController.go", Confidence: "EXACT", Search: "delete catalog item"},
			{ID: "service", Project: "services/catalog", Kind: "symbol", Name: "deleteItem", File: "CatalogService.go", Confidence: "EXACT", Search: "delete catalog item"},
			{ID: "future-client", Project: "libraries/job-client", Kind: "symbol", Name: "deleteRelatedJobs", File: "JobClient.go", Confidence: "EXACT", Search: "delete related jobs client configuration"},
		},
		Edges: []scan.AgentContextEdgeRecord{{
			ID: "call", FromFactID: "route", ToFactID: "service", Kind: "call", Confidence: "EXACT",
		}},
	})

	pack, err := BuildContext(ContextRequest{
		Root:  root,
		Query: "Plan deleting related jobs through a future client configuration in libraries/job-client after DELETE /catalog/items/{id}.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.CallChain) != 1 || pack.CallChain[0].From != "DELETE /catalog/items/{id}" || pack.CallChain[0].To != "deleteItem" {
		t.Fatalf("disconnected project was fabricated into the call chain: %#v", pack.CallChain)
	}
	for _, file := range pack.Files {
		if file.Path != "JobClient.go" {
			continue
		}
		if file.Role != "related_project" || file.Confidence != "RESOLVED" {
			t.Fatalf("disconnected candidate metadata = %#v", file)
		}
		return
	}
	t.Fatalf("disconnected explicit project candidate missing: %#v", pack.Files)
}

func TestRelatedProjectSelectionPrefersOperationalEvidence(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "api_endpoint", Name: "DELETE /catalog/items/{id}", Qualified: "CatalogController.deleteItem", HTTPMethod: "DELETE", Path: "/catalog/items/{id}", File: "CatalogController.go", Search: "delete catalog item", Confidence: "EXACT"},
			{ID: "operation", Project: "services/catalog", Kind: "symbol", Name: "deleteItem", File: "CatalogService.go", Search: "delete catalog item", Confidence: "EXACT"},
			{ID: "payload", Project: "libraries/integration", Kind: "symbol", Name: "RelatedJobPayload", File: "RelatedJobPayload.go", Search: "delete catalog item related jobs remain", Confidence: "EXACT"},
			{ID: "client", Project: "libraries/integration", Kind: "api_contract", Name: "DELETE /internal/jobs", Qualified: "JobPort.deleteRelated", HTTPMethod: "DELETE", Path: "/internal/jobs", File: "JobPort.go", Search: "catalog item jobs", Confidence: "EXACT"},
			{ID: "client-security", Project: "libraries/integration", Kind: "authentication", Name: "authenticated", File: "JobPortSecurity.go", Search: "authentication", Confidence: "EXACT"},
			{ID: "unrelated-endpoint", Project: "services/jobs", Kind: "api_endpoint", Name: "GET /users/{id}/jobs", HTTPMethod: "GET", Path: "/users/{id}/jobs", File: "UserJobController.go", Search: "catalog item related jobs remain", Confidence: "EXACT"},
			{ID: "repository", Project: "services/jobs", Kind: "persistence", Name: "findByCatalogItem", Qualified: "JobRepository.findByCatalogItem", File: "JobRepository.go", Search: "catalog item jobs persistence", Confidence: "EXACT"},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "route-operation", FromFactID: "route", ToFactID: "operation", Kind: "call", Confidence: "EXACT"},
			{ID: "client-auth", FromFactID: "client", ToFactID: "client-security", Kind: "requires_auth", Confidence: "EXACT"},
		},
	})
	query := `Problem statement:

Delete a catalog item, related jobs remain.

Analyze services/catalog, libraries/integration, and services/jobs. Cover the internal contract, authentication, configuration, persistence, and tests.`
	writeSourceFile(t, root, "CatalogController.go", "package catalog\nfunc deleteItem() {}\n")
	writeSourceFile(t, root, "CatalogService.go", "package catalog\nfunc deleteItem() {}\n")
	writeSourceFile(t, root, "JobPort.go", "package integration\nfunc deleteRelated() {}\n")
	writeSourceFile(t, root, "JobRepository.go", "package jobs\nfunc findByCatalogItem() {}\n")

	pack, err := BuildContext(ContextRequest{Root: root, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"JobPort.go", "JobRepository.go"} {
		if !contextPackHasFile(pack, path) {
			t.Fatalf("operational support %q missing: %#v", path, pack.Files)
		}
	}
	for _, path := range []string{"RelatedJobPayload.go", "UserJobController.go"} {
		if contextPackHasFile(pack, path) {
			t.Fatalf("lexical distractor %q displaced operational evidence: %#v", path, pack.Files)
		}
	}
	covered := map[string]bool{}
	for _, concern := range pack.Concerns {
		covered[contextPublicConcernKey(concern)] = concern.Covered
	}
	for _, key := range []string{
		contextConcernHTTPContract,
		contextConcernPersistence,
		contextConcernProject + ":libraries/integration",
		contextConcernProject + ":services/jobs",
	} {
		if !covered[key] {
			t.Fatalf("selected support concern %q is not covered: %#v", key, pack.Concerns)
		}
	}
}

func TestRelatedProjectSelectionLeavesWeakEvidenceUncovered(t *testing.T) {
	root := writeContextIndexFixture(t, scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Facts: []scan.AgentContextFactRecord{
			{ID: "route", Project: "services/catalog", Kind: "route", Name: "DELETE /catalog/items/{id}", HTTPMethod: "DELETE", Path: "/catalog/items/{id}", File: "CatalogController.go", Search: "delete catalog item", Confidence: "EXACT"},
			{ID: "payload", Project: "libraries/model", Kind: "symbol", Name: "CatalogPayload", File: "CatalogPayload.go", Search: "catalog payload", Confidence: "EXACT"},
		},
	})

	pack, err := BuildContext(ContextRequest{
		Root:  root,
		Query: "Delete a catalog item. Analyze libraries/model for an internal contract.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if contextPackHasFile(pack, "CatalogPayload.go") {
		t.Fatalf("weak lexical evidence was forced into the pack: %#v", pack.Files)
	}
	if !contextPackHasUncertainty(pack, "libraries/model/project_context", "no relevant production fact selected") {
		t.Fatalf("missing project evidence was not reported honestly: %#v", pack.Uncertainties)
	}
	for _, concern := range pack.Concerns {
		if contextPublicConcernKey(concern) == contextConcernProject+":libraries/model" && concern.Covered {
			t.Fatalf("unselected weak project was reported as covered: %#v", pack.Concerns)
		}
	}
}

func TestCrossServiceFixtureSourcesAvoidDeclarationCollisions(t *testing.T) {
	facts := crossServiceContextFacts(".go")
	sourceFacts := make(map[string][]scan.AgentContextFactRecord)
	for _, fact := range facts {
		path := filepath.Join(fact.Project, fact.File)
		sourceFacts[path] = append(sourceFacts[path], fact)
	}

	goMethods := make(map[string]string)
	for path, sourceFacts := range sourceFacts {
		fixtureType := crossServiceFixtureType(path)
		goSource := crossServiceSource(".go", path, sourceFacts)
		if _, err := parser.ParseFile(token.NewFileSet(), filepath.Base(path), goSource, 0); err != nil {
			t.Fatalf("Go fixture %q is invalid: %v", path, err)
		}
		for _, fact := range sourceFacts {
			method := fixtureType + "." + crossServiceFactIdentifier(fact)
			projectMethod := fact.Project + "\x00" + method
			if existing, exists := goMethods[projectMethod]; exists {
				t.Fatalf("Go fixture method %q collides for %q and %q", method, existing, path)
			}
			goMethods[projectMethod] = path
		}
	}

	javaTypes := make(map[string]string)
	for _, fact := range crossServiceContextFacts(".java") {
		path := filepath.Join(fact.Project, fact.File)
		fixtureType := crossServiceFixtureType(path)
		if existing, exists := javaTypes[fixtureType]; exists && existing != path {
			t.Fatalf("Java fixture type %q collides for %q and %q", fixtureType, existing, path)
		}
		javaTypes[fixtureType] = path
		if source := crossServiceSource(".java", path, []scan.AgentContextFactRecord{fact}); !strings.Contains(source, "class "+fixtureType+" {") {
			t.Fatalf("Java fixture %q does not declare %q", path, fixtureType)
		}
	}
}

type contextSelectionSnapshot struct {
	FactIDs        []string
	ConcernKeys    []string
	ProjectOrder   []string
	RenderModes    []string
	SourceCoverage string
}

func contextSelectionSnapshotForPack(pack ContextPack) contextSelectionSnapshot {
	factIDs := append([]string(nil), pack.selectedSourceFactIDs...)
	sort.Strings(factIDs)

	projects := make([]string, 0, len(pack.SourceSections))
	seenProjects := make(map[string]bool)
	renderModes := make([]string, 0, len(pack.SourceSections))
	for _, section := range pack.SourceSections {
		if section.Role != "test" && !seenProjects[section.Project] {
			seenProjects[section.Project] = true
			projects = append(projects, section.Project)
		}
		renderModes = append(renderModes, section.RenderMode)
	}

	return contextSelectionSnapshot{
		FactIDs:        factIDs,
		ConcernKeys:    contextPackConcernKeys(pack),
		ProjectOrder:   projects,
		RenderModes:    renderModes,
		SourceCoverage: pack.SourceCoverage,
	}
}

func contextPackConcernKeys(pack ContextPack) []string {
	keys := make([]string, 0, len(pack.Concerns))
	for _, concern := range pack.Concerns {
		key := concern.Kind
		if concern.Project != "" {
			key += ":" + concern.Project
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contextPackHasProductionSource(pack ContextPack, project string) bool {
	for _, section := range pack.SourceSections {
		if section.Project == project && section.Role != "test" {
			return true
		}
	}
	return false
}

func contextPackHasRelationshipKind(pack ContextPack, kind string) bool {
	for _, relationship := range pack.CallChain {
		if relationship.Kind == kind {
			return true
		}
	}
	return false
}

func contextPackTestPrecedesProduction(pack ContextPack, requiredProjects []string) bool {
	required := make(map[string]bool, len(requiredProjects))
	for _, project := range requiredProjects {
		required[project] = true
	}
	firstTest := len(pack.SourceSections)
	lastRequiredProduction := -1
	for index, section := range pack.SourceSections {
		if section.Role == "test" && firstTest == len(pack.SourceSections) {
			firstTest = index
		}
		if section.Role != "test" && required[section.Project] {
			lastRequiredProduction = index
		}
	}
	return firstTest < lastRequiredProduction
}

func writeCrossServiceContextFixture(t *testing.T, extension string) string {
	t.Helper()
	root := t.TempDir()
	index := scan.AgentContextIndexRecord{
		SchemaVersion: scan.SchemaVersion,
		Generated:     "2026-07-19T00:00:00Z",
		Facts:         crossServiceContextFacts(extension),
		Edges: []scan.AgentContextEdgeRecord{
			{ID: "e1", FromFactID: "route", ToFactID: "operations", Kind: "call", Confidence: "EXACT"},
			{ID: "e2", FromFactID: "operations", ToFactID: "client", Kind: "call", Confidence: "RESOLVED"},
			{ID: "e3", FromFactID: "client", ToFactID: "contract", Kind: "call", Confidence: "EXACT"},
			{ID: "e4", FromFactID: "contract", ToFactID: "provider", Kind: "http_contract", Confidence: "RESOLVED"},
			{ID: "e5", FromFactID: "provider", ToFactID: "service", Kind: "call", Confidence: "EXACT"},
			{ID: "e6", FromFactID: "service", ToFactID: "regular-repository", Kind: "persistence", Confidence: "RESOLVED"},
			{ID: "e7", FromFactID: "service", ToFactID: "change-repository", Kind: "persistence", Confidence: "RESOLVED"},
			{ID: "e8", FromFactID: "test", ToFactID: "route", Kind: "test_target", Confidence: "EXACT"},
		},
	}
	writeContextIndexAt(t, filepath.Join(root, ".goregraph-workspace", "agent", "context-index.json"), index)
	sourceFacts := make(map[string][]scan.AgentContextFactRecord)
	for _, fact := range index.Facts {
		if fact.Kind == "test" || fact.Kind == "route" || fact.Kind == "symbol" || fact.Kind == "api_contract" || fact.Kind == "persistence" {
			path := filepath.Join(fact.Project, fact.File)
			sourceFacts[path] = append(sourceFacts[path], fact)
		}
	}
	for path, facts := range sourceFacts {
		writeContextSourceFile(t, root, path, crossServiceSource(extension, path, facts))
	}
	return root
}

func crossServiceContextFacts(extension string) []scan.AgentContextFactRecord {
	facts := []scan.AgentContextFactRecord{
		{ID: "route", Project: "services/catalog", Kind: "route", Name: "DELETE /catalogs/{catalogId}/items/{itemId}", Qualified: "CatalogController.deleteItem", HTTPMethod: "DELETE", Path: "/catalogs/{catalogId}/items/{itemId}", File: "src/main/java/example/CatalogController.java", Line: 10, EndLine: 14, Confidence: "EXACT", Search: "delete catalog item"},
		{ID: "operations", Project: "services/catalog", Kind: "symbol", Name: "deleteItem", Qualified: "CatalogOperations.deleteItem", File: "src/main/java/example/CatalogOperations.java", Line: 8, EndLine: 15, Confidence: "EXACT", Search: "delete catalog item operations"},
		{ID: "client", Project: "libraries/shared-model", Kind: "symbol", Name: "deleteRelatedJobs", Qualified: "JobManagementClient.deleteRelatedJobs", File: "src/main/java/example/JobManagementClient.java", Line: 12, EndLine: 24, Confidence: "EXACT", Search: "delete related jobs basic authentication retry configuration"},
		{ID: "contract", Project: "libraries/shared-model", Kind: "api_contract", Name: "DELETE /job-management/catalogs/{catalogId}/items/{itemId}", Qualified: "JobManagementClient.deleteRelatedJobs", HTTPMethod: "DELETE", Path: "/job-management/catalogs/{catalogId}/items/{itemId}", File: "src/main/java/example/JobManagementClient.java", Line: 18, EndLine: 21, Confidence: "RESOLVED", Search: "delete related jobs internal contract"},
		{ID: "provider", Project: "services/jobs", Kind: "route", Name: "DELETE /job-management/catalogs/{catalogId}/items/{itemId}", Qualified: "JobManagementController.deleteRelatedJobs", HTTPMethod: "DELETE", Path: "/job-management/catalogs/{catalogId}/items/{itemId}", File: "src/main/java/example/JobManagementController.java", Line: 20, EndLine: 25, Confidence: "EXACT", Search: "delete related jobs"},
		{ID: "service", Project: "services/jobs", Kind: "symbol", Name: "deleteRelatedJobs", Qualified: "JobService.deleteRelatedJobs", File: "src/main/java/example/JobService.java", Line: 30, EndLine: 45, Confidence: "EXACT", Search: "delete related jobs persistence"},
		{ID: "regular-repository", Project: "services/jobs", Kind: "persistence", Name: "deleteByCatalogIdAndItemId", Qualified: "JobRepository.deleteByCatalogIdAndItemId", File: "src/main/java/example/JobRepository.java", Line: 8, EndLine: 10, Confidence: "EXACT", Search: "regular job catalog item delete persistence"},
		{ID: "change-repository", Project: "services/jobs", Kind: "persistence", Name: "deleteByCatalogIdAndItemId", Qualified: "ChangeJobRepository.deleteByCatalogIdAndItemId", File: "src/main/java/example/ChangeJobRepository.java", Line: 8, EndLine: 10, Confidence: "EXACT", Search: "change job catalog item delete persistence"},
		{ID: "test", Project: "services/catalog", Kind: "test", Name: "deletes item", File: "src/test/java/example/CatalogControllerTest.java", Line: 15, EndLine: 28, Confidence: "EXACT", Search: "delete catalog item test"},
	}
	if extension == ".java" {
		return facts
	}
	for index := range facts {
		facts[index].File = strings.TrimSuffix(facts[index].File, ".java") + extension
	}
	return facts
}

func crossServiceSource(extension, sourcePath string, facts []scan.AgentContextFactRecord) string {
	lines := make([]string, 50)
	for index := range lines {
		lines[index] = fmt.Sprintf("// source line %d", index+1)
	}
	lines[0] = crossServiceSourceHeader(extension)
	fixtureType := crossServiceFixtureType(sourcePath)
	switch extension {
	case ".java":
		lines[1] = "class " + fixtureType + " {"
		lines[len(lines)-1] = "}"
	case ".go":
		lines[1] = "type " + fixtureType + " struct{}"
	}
	for _, fact := range facts {
		lines[fact.Line-1] = crossServiceSourceDeclaration(extension, fixtureType, crossServiceFactIdentifier(fact))
	}
	return strings.Join(lines, "\n") + "\n"
}

func crossServiceFixtureType(sourcePath string) string {
	name := strings.NewReplacer("/", "_", "\\", "_", ".", "_", "-", "_").Replace(sourcePath)
	return "ContextFixture_" + name
}

func crossServiceSourceHeader(extension string) string {
	switch extension {
	case ".go":
		return "package example"
	case ".ts":
		return "export {}"
	case ".py":
		return "# example source"
	default:
		return "package example;"
	}
}

func crossServiceFactIdentifier(fact scan.AgentContextFactRecord) string {
	identifier := fact.Name
	if fact.Kind == "route" {
		identifier = fact.Qualified
	}
	if index := strings.LastIndex(identifier, "."); index >= 0 {
		identifier = identifier[index+1:]
	}
	return strings.Fields(identifier)[0]
}

func crossServiceSourceDeclaration(extension, fixtureType, identifier string) string {
	switch extension {
	case ".go":
		return "func (" + fixtureType + ") " + identifier + "() {}"
	case ".ts":
		return "export function " + identifier + "() {}"
	case ".py":
		return "def " + identifier + "(): pass"
	default:
		return "    void " + identifier + "() {}"
	}
}
