package agent

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestContextSourcePlanningKeepsProvingMethodInSameFile(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "services/jobs/JobService.java", `class JobService {
 void deleteCatalogJob() {
  repository.delete(job);
 }
`+strings.Repeat("\n", 40)+` void deleteCatalogChangeJob() {
  changeRepository.delete(job);
  mailService.sendMail(job);
 }
}
`)
	facts := []scan.AgentContextFactRecord{
		{ID: "regular", Project: "services/jobs", Kind: "symbol", Name: "deleteCatalogJob", Qualified: "JobService.deleteCatalogJob", File: "JobService.java", Line: 2, EndLine: 4, Confidence: "EXACT"},
		{ID: "change", Project: "services/jobs", Kind: "symbol", Name: "deleteCatalogChangeJob", Qualified: "JobService.deleteCatalogChangeJob", File: "JobService.java", Line: 45, EndLine: 48, Confidence: "EXACT"},
	}
	pack := ContextPack{Query: "Delete a job. Analyze services/jobs mail side effects."}
	concern := newContextEvidenceConcern(newContextConcern(
		contextConcernSideEffects, "services/jobs", true, []string{"regular", "change"}, "requested side effects",
	), "mail", []string{"regular", "change"}, "requested mail evidence")
	index := scan.AgentContextIndexRecord{Facts: facts}
	candidates := contextSourceCandidatesForConcerns(pack, index, []contextConcern{concern})
	options, failures, err := contextSourceRenderOptions(pack,
		loadedContextIndex{ScopeRoot: root, Workspace: true, Index: index},
		candidates, []contextConcern{concern}, nil)
	if err != nil || len(failures) != 0 {
		t.Fatalf("render candidates: %v, failures: %#v", err, failures)
	}
	foundMail := false
	for _, option := range options {
		if slices.Contains(option.concernKeys, concern.key) && strings.Contains(option.section.Content, "mailService.sendMail(job)") {
			foundMail = true
		}
	}
	if !foundMail {
		t.Fatalf("mail evidence from the second method was lost before source verification; candidates: %#v", candidates)
	}
	pack.Schema = 1
	pack.Query = "DELETE /catalog/{id}. Analyze services/jobs mail side effects."
	pack.Confidence = "EXACT"
	pack.BudgetTokens = DefaultContextBudgetTokens
	writeSourceFile(t, root, "services/catalog/CatalogController.java", "class CatalogController {\n void deleteItem() { repository.delete(item); }\n}\n")
	index.Facts = append(index.Facts, scan.AgentContextFactRecord{
		ID: "route", Project: "services/catalog", Kind: "route", Name: "deleteItem", Qualified: "CatalogController.deleteItem",
		HTTPMethod: "DELETE", Path: "/catalog/{id}", File: "CatalogController.java", Line: 2, EndLine: 2, Confidence: "EXACT",
	})
	pack.Entrypoints = []ContextLocation{{ID: "route", Project: "services/catalog", File: "CatalogController.java"}}
	pack.selectedSourceFactIDs = []string{"route"}
	pack.Concerns = []ContextConcern{
		{Kind: contextConcernEntrypoint},
		{Kind: contextConcernSideEffects, Project: "services/jobs"},
	}
	file, err := readSourceFile(root + "/services/jobs/JobService.java")
	if err != nil {
		t.Fatal(err)
	}
	index.SourceHashes = map[string]string{"services/jobs/JobService.java": file.Hash}
	loaded := loadedContextIndex{ScopeRoot: root, Workspace: true, Index: index}
	if reason := adaptiveSelectedSourceFallbackReason(pack, loaded); reason != "" {
		t.Fatalf("unchanged concern source rejected: %s", reason)
	}
	writeSourceFile(t, root, "services/jobs/JobService.java", strings.Join(file.Lines, "\n")+"// changed after indexing\n")
	if reason := adaptiveSelectedSourceFallbackReason(pack, loaded); reason != ContextFallbackEvidenceConflict {
		t.Fatalf("changed concern-only source escaped freshness check: %s", reason)
	}
	got, err := attachContextSource(pack,
		loadedContextIndex{ScopeRoot: root, Workspace: true, Index: index},
		ContextRequest{BudgetTokens: DefaultContextBudgetTokens, MaxFiles: DefaultContextMaxFiles})
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range got.SourceSections {
		if strings.Contains(section.Content, "mailService.sendMail(job)") {
			return
		}
	}
	t.Fatalf("verified mail evidence was not delivered in the context pack: %#v", got)
}

func TestContextSourcePlanningBalancesFilesBeforeAdditionalDeclarations(t *testing.T) {
	facts := []scan.AgentContextFactRecord{}
	ids := []string{}
	for i := range 12 {
		id := fmt.Sprintf("method-%02d", i)
		ids = append(ids, id)
		facts = append(facts, scan.AgentContextFactRecord{
			ID: id, Project: "services/jobs", Kind: "side_effects", Name: "deleteJob",
			Qualified: fmt.Sprintf("JobService.deleteJob%d", i), File: "JobService.java",
			Line: 2 + i*4, EndLine: 4 + i*4, Confidence: "EXACT",
		})
	}
	ids = append(ids, "other-file", "duplicate")
	facts = append(facts, scan.AgentContextFactRecord{
		ID: "other-file", Project: "services/jobs", Kind: "side_effects", Name: "deleteJob",
		Qualified: "OtherJobService.deleteJob", File: "OtherJobService.java", Line: 2, EndLine: 4,
	})
	duplicate := facts[0]
	duplicate.ID = "duplicate"
	facts = append(facts, duplicate)
	concerns := []contextConcern{newContextConcern(contextConcernSideEffects, "services/jobs", true, ids, "side effects")}
	pack := ContextPack{Query: "Delete a job. Analyze services/jobs side effects."}
	got := contextSourceCandidatesForConcerns(pack, scan.AgentContextIndexRecord{Facts: facts}, concerns)
	planned := 0
	foundOther := false
	for _, candidate := range got {
		planned += len(candidate.FactIDs)
		foundOther = foundOther || candidate.Path == "OtherJobService.java"
		if slices.Contains(candidate.FactIDs, "duplicate") && slices.Contains(candidate.FactIDs, "method-00") {
			t.Fatalf("duplicate declaration consumed planning capacity: %#v", candidate)
		}
	}
	if planned != maximumContextSourcePlanningCandidates {
		t.Fatalf("planned %d declarations, want bounded available capacity %d", planned, maximumContextSourcePlanningCandidates)
	}
	if !foundOther {
		t.Fatal("methods in one file displaced a different file")
	}
	slices.Reverse(facts)
	slices.Reverse(ids)
	reversed := contextSourceCandidatesForConcerns(pack, scan.AgentContextIndexRecord{Facts: facts}, concerns)
	if !reflect.DeepEqual(got, reversed) {
		t.Fatal("declaration planning depends on input order")
	}
}
