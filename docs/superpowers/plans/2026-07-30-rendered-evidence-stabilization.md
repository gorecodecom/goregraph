# Rendered Evidence Stabilization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GoreGraph prove every requested Context concern from the final published evidence and recover missing high-value evidence through deterministic substitution without increasing output limits.

**Architecture:** Expand requested domain models into project-scoped internal evidence requirements, profile rendered source against those requirements, and recompute coverage from the final sections instead of incremental selection state. Keep discovery and repair bounded with an eight-candidate planning frontier, four proving candidates per requirement, and strictly improving one-for-one substitutions.

**Tech Stack:** Go 1.23, Go standard library, existing `internal/agent` Context planner and renderer, table-driven Go tests, shell benchmark regression fixtures

## Global Constraints

- Maximum estimated Context Pack size remains 4,000 tokens.
- Maximum published file count remains 12.
- Maximum source section count remains 12.
- No private G1 repository, path, class, method, or domain identifier may enter production code or committed fixtures.
- No public CLI command, flag, Context schema, retry, or fallback behavior changes.
- Existing source-read, tool-call, deterministic-output, and omission gates remain strict.
- Tests must be written before each production change.
- Each task changes one causal axis and ends with a focused green test suite.
- Do not run an external private-workspace benchmark until every local gate passes and an active data-sharing authorization has been confirmed.

---

## File Structure

- Create `internal/agent/context_proof.go`: model-scoped evidence requirements, rendered domain proof, final-section coverage audit, and exact evidence-file projection.
- Create `internal/agent/context_substitute.go`: reconstruction of selected source options and strictly improving bounded substitutions.
- Modify `internal/agent/context_select.go`: integrate domain requirement expansion, proof frontier, repair, final audit, and inventory publication into the existing selector.
- Modify `internal/agent/context_source.go`: separate the planning candidate ceiling from the proving-candidate ceiling.
- Modify `internal/agent/context_source_test.go`: focused unit tests for evidence requirements, proof predicates, final audit, frontier bounds, repair invariants, and file projection.
- Modify `internal/agent/context_change_analysis_test.go`: one generic Java/Spring release-shaped regression that reproduces the three quality gaps.
- Use existing `internal/agent/context_size_test.go`: unchanged hard-budget and deterministic-size regression coverage.

### Task 1: Project-scoped domain evidence requirements

**Files:**

- Create: `internal/agent/context_proof.go`
- Modify: `internal/agent/context_select.go:438-690`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Consumes: `contextConcern`, `scan.AgentContextIndexRecord`, requested model IDs from `contextRequestedDomainModelIDsFromConcerns`.
- Produces: `contextDomainModelEvidenceConcerns(base contextConcern, index scan.AgentContextIndexRecord, requestedModels map[string]bool) []contextConcern`.
- Produces: `contextDomainModelEvidenceFactIDs(index scan.AgentContextIndexRecord, modelID string) []string`.
- Later tasks rely on each generated concern using a concrete facet such as
  `model:regular-job`, the requested model's normalized project, and the
  original public concern key.

- [ ] **Step 1: Write the failing model-scope test**

Add this test to `internal/agent/context_source_test.go`:

```go
func TestContextDomainModelEvidenceConcernsScopeModelsAndLinkedBase(t *testing.T) {
	index := scan.AgentContextIndexRecord{
		Facts: []scan.AgentContextFactRecord{
			{
				ID: "consumer-job", Project: "services/catalog", Kind: "symbol",
				Name: "CatalogJobEntity", Qualified: "catalog.CatalogJobEntity",
				File: "CatalogJobEntity.java",
			},
			{
				ID: "regular-job", Project: "services/jobs", Kind: "symbol",
				Name: "CatalogJobEntity", Qualified: "jobs.CatalogJobEntity",
				File: "CatalogJobEntity.java",
			},
			{
				ID: "change-job", Project: "services/jobs", Kind: "symbol",
				Name: "CatalogChangeJobEntity", Qualified: "jobs.CatalogChangeJobEntity",
				File: "CatalogChangeJobEntity.java",
			},
			{
				ID: "base-job", Project: "services/jobs", Kind: "symbol",
				Name: "BaseCatalogJobEntity", Qualified: "jobs.BaseCatalogJobEntity",
				File: "BaseCatalogJobEntity.java",
			},
		},
		Edges: []scan.AgentContextEdgeRecord{
			{
				ID: "regular-extends-base", FromFactID: "regular-job",
				ToFactID: "base-job", Kind: "extends", Confidence: "EXACT",
			},
			{
				ID: "change-extends-base", FromFactID: "change-job",
				ToFactID: "base-job", Kind: "extends", Confidence: "EXACT",
			},
		},
	}
	base := newContextConcern(
		contextConcernDomainModel,
		"",
		true,
		[]string{"consumer-job", "regular-job", "change-job"},
		"requested task types and lookup attributes",
	)

	got := contextDomainModelEvidenceConcerns(
		base,
		index,
		map[string]bool{"regular-job": true, "change-job": true},
	)
	if len(got) != 2 {
		t.Fatalf("domain evidence concerns = %#v, want two requested models", got)
	}
	for _, concern := range got {
		if concern.project != "services/jobs" ||
			concern.publicKey != contextConcernDomainModel ||
			!strings.HasPrefix(concern.facet, "model:") ||
			!slices.Contains(concern.candidateFactIDs, "base-job") ||
			slices.Contains(concern.candidateFactIDs, "consumer-job") {
			t.Fatalf("scoped domain concern = %#v", concern)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and verify the red state**

Run:

```bash
go test ./internal/agent -run TestContextDomainModelEvidenceConcernsScopeModelsAndLinkedBase -count=1
```

Expected: FAIL because `contextDomainModelEvidenceConcerns` is undefined.

- [ ] **Step 3: Implement model-to-base evidence expansion**

Create `internal/agent/context_proof.go` with:

```go
package agent

import (
	"sort"
	"strings"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func contextDomainModelEvidenceConcerns(
	base contextConcern,
	index scan.AgentContextIndexRecord,
	requestedModels map[string]bool,
) []contextConcern {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	modelIDs := make([]string, 0, len(requestedModels))
	for modelID := range requestedModels {
		if slicesContainsString(base.candidateFactIDs, modelID) {
			modelIDs = append(modelIDs, modelID)
		}
	}
	sort.Strings(modelIDs)

	result := make([]contextConcern, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		model, ok := factByID[modelID]
		if !ok {
			continue
		}
		concern := newContextEvidenceConcern(
			base,
			"model:"+modelID,
			contextDomainModelEvidenceFactIDs(index, modelID),
			"domain model evidence for requested model "+model.Name,
		)
		concern.project = normalizeContextProject(model.Project)
		result = append(result, concern)
	}
	return result
}

func contextDomainModelEvidenceFactIDs(
	index scan.AgentContextIndexRecord,
	modelID string,
) []string {
	factByID := make(map[string]scan.AgentContextFactRecord, len(index.Facts))
	for _, fact := range index.Facts {
		factByID[fact.ID] = fact
	}
	model, ok := factByID[modelID]
	if !ok {
		return nil
	}
	result := []string{modelID}
	for _, edge := range index.Edges {
		if edge.FromFactID != modelID ||
			strings.ToLower(strings.TrimSpace(edge.Kind)) != "extends" {
			continue
		}
		base, found := factByID[edge.ToFactID]
		if found && normalizeContextProject(base.Project) == normalizeContextProject(model.Project) {
			result = append(result, base.ID)
		}
	}
	return orderedContextConcernIDs(result)
}

func slicesContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
```

If `slices.Contains` is already imported in the final file, use it and omit
`slicesContainsString`; keep exactly one helper.

- [ ] **Step 4: Integrate domain expansion into evidence concerns**

In `expandContextEvidenceConcernsWithProfile`, add this switch branch before
authentication:

```go
case contextConcernDomainModel:
	modelConcerns := contextDomainModelEvidenceConcerns(
		concern,
		index,
		requestedModels,
	)
	if len(modelConcerns) == 0 {
		result = append(result, concern)
	} else {
		result = append(result, modelConcerns...)
	}
```

- [ ] **Step 5: Format and run the focused concern tests**

Run:

```bash
gofmt -w internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextDomainModelEvidenceConcerns|TestApplyContextSourceCoverageRequiresEveryInternalFacet' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the scoped concern change**

```bash
git add internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go
git commit -m "Scope domain evidence by requested model" -m "- Expand requested models into project-specific internal evidence facets
- Include exact same-project base models linked by indexed extends relations
- Keep public domain coverage dependent on every requested model facet"
```

### Task 2: Rendered domain-model proof

**Files:**

- Modify: `internal/agent/context_proof.go`
- Modify: `internal/agent/context_select.go:2056-2325`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Consumes: project-scoped domain concerns from Task 1.
- Produces: `contextSourceSectionSupportsDomainModel(section ContextSourceSection) bool`.
- Changes: `contextSourceOptionConcernsWithAction` requires rendered model structure for `domain_model`.
- Later final audits use the same predicate through each option's `concernKeys`.

- [ ] **Step 1: Write failing rendered-model tests**

Add:

```go
func TestContextSourceOptionConcernsRequireRenderedDomainStructure(t *testing.T) {
	concern := newContextEvidenceConcern(
		newContextConcern(
			contextConcernDomainModel,
			"services/jobs",
			true,
			[]string{"job-model", "base-model"},
			"requested job model",
		),
		"model:job-model",
		[]string{"job-model", "base-model"},
		"requested job model fields",
	)
	index := scan.AgentContextIndexRecord{Facts: []scan.AgentContextFactRecord{
		{
			ID: "job-model", Project: "services/jobs", Kind: "symbol",
			Name: "CatalogJobEntity",
		},
		{
			ID: "base-model", Project: "services/jobs", Kind: "symbol",
			Name: "BaseCatalogJobEntity",
		},
	}}
	candidate := sourceCandidate{
		FactID: "base-model", FactIDs: []string{"base-model"},
		Project: "services/jobs", Role: contextConcernDomainModel,
	}
	tests := []struct {
		name    string
		section ContextSourceSection
		want    bool
	}{
		{
			name: "rejects signature",
			section: ContextSourceSection{
				Project: "services/jobs", Role: contextConcernDomainModel,
				RenderMode: "signature", Content: "@Entity\nclass BaseCatalogJobEntity {",
			},
		},
		{
			name: "rejects wrong project",
			section: ContextSourceSection{
				Project: "services/catalog", Role: contextConcernDomainModel,
				RenderMode: "declaration_body",
				Content: "class CatalogJobEntity {\n  long catalogId;\n}",
			},
		},
		{
			name: "accepts field body",
			section: ContextSourceSection{
				Project: "services/jobs", Role: contextConcernDomainModel,
				RenderMode: "declaration_body",
				Content: "class BaseCatalogJobEntity {\n  long catalogId;\n  long itemId;\n}",
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keys, _ := contextSourceOptionConcerns(
				candidate,
				test.section,
				[]contextConcern{concern},
				index,
			)
			if got := slices.Contains(keys, concern.key); got != test.want {
				t.Fatalf("domain proof = %v, want %v, keys %v", got, test.want, keys)
			}
		})
	}
}
```

Add a table-driven test for language-neutral structure:

```go
func TestContextSourceSectionSupportsLanguageNeutralDomainStructure(t *testing.T) {
	for _, content := range []string{
		"type Job struct {\n  CatalogID int64\n}",
		"interface Job {\n  catalogId: number;\n}",
		"class Job:\n    catalog_id: int",
		"class JobEntity {\n  private long catalogId;\n}",
	} {
		section := ContextSourceSection{
			RenderMode: "declaration_body",
			Content:    content,
		}
		if !contextSourceSectionSupportsDomainModel(section) {
			t.Errorf("domain structure rejected: %q", content)
		}
	}
}
```

- [ ] **Step 2: Verify current signature acceptance fails the new contract**

Run:

```bash
go test ./internal/agent -run 'TestContextSourceOptionConcernsRequireRenderedDomainStructure|TestContextSourceSectionSupportsLanguageNeutralDomainStructure' -count=1
```

Expected: FAIL because domain-model coverage does not currently require
rendered structure.

- [ ] **Step 3: Implement the language-neutral structure predicate**

Add to `context_proof.go`:

```go
func contextSourceSectionSupportsDomainModel(section ContextSourceSection) bool {
	if section.RenderMode == "signature" {
		return false
	}
	for _, line := range strings.Split(contextSourceSemanticContent(section.Content), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if line == "" || strings.HasPrefix(line, "@") ||
			line == "{" || line == "}" ||
			strings.HasPrefix(lower, "class ") ||
			strings.HasPrefix(lower, "interface ") ||
			strings.HasPrefix(lower, "type ") && strings.HasSuffix(line, "{") ||
			strings.Contains(line, "(") {
			continue
		}
		if strings.HasSuffix(line, ";") ||
			strings.Contains(line, ": ") ||
			strings.Contains(line, "\t") ||
			len(strings.Fields(line)) >= 2 {
			return true
		}
	}
	return false
}
```

The test table is the contract. Tighten the implementation if a declaration
line is falsely accepted; do not add language- or private-name special cases.

- [ ] **Step 4: Require rendered proof for domain concerns**

In `contextSourceOptionConcernsWithAction`, evaluate domain concerns before the
generic rendered-evidence branch:

```go
if covered && concern.kind == contextConcernDomainModel {
	covered = contextSourceSectionSupportsDomainModel(section)
} else if covered && contextSourceRequiresRenderedConcernEvidence(concern.kind) {
	covered = contextSourceSectionSupportsConcern(section, concern)
}
```

Add `contextConcernDomainModel` to
`contextSourceRequiresRenderedConcernEvidence`.

- [ ] **Step 5: Run focused and existing language-neutral tests**

```bash
gofmt -w internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextSource.*Domain|TestContextSelectionIsLanguageNeutral|TestContextSourceUtilityPrefersDomainEvidence' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit rendered domain proof**

```bash
git add internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go
git commit -m "Require rendered domain model evidence" -m "- Reject signature-only and wrong-project domain proof
- Accept field-bearing declaration bodies across supported source styles
- Apply rendered proof to every requested model facet"
```

### Task 3: Final-section coverage audit

**Files:**

- Modify: `internal/agent/context_proof.go`
- Modify: `internal/agent/context_select.go:30-190`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `contextSourceCoverageFromFinalSections(pack ContextPack, concerns []contextConcern, options []contextSourceOption) map[string]bool`.
- Consumes: exact final `ContextSourceSection` values and profiled option `concernKeys`.
- Replaces: the last use of incremental `state.coveredConcerns` for public coverage and omission generation.

- [ ] **Step 1: Write a failing stale-proof audit test**

```go
func TestContextSourceCoverageFromFinalSectionsRejectsStaleProof(t *testing.T) {
	concern := newContextEvidenceConcern(
		newContextConcern(
			contextConcernAuth,
			"libraries/job-client",
			true,
			[]string{"client-auth"},
			"selected client authentication",
		),
		"client_transport",
		[]string{"client-auth"},
		"client transport authentication",
	)
	proving := contextSourceOption{
		candidate: sourceCandidate{FactID: "client-auth"},
		section: ContextSourceSection{
			Project: "libraries/job-client", Path: "JobClient.java",
			StartLine: 10, EndLine: 14, RenderMode: "focused",
			Content: "headers.setBasicAuth(user, password);",
		},
		concernKeys: []string{concern.key},
	}
	nonProvingUpgrade := contextSourceOption{
		candidate: proving.candidate,
		section: ContextSourceSection{
			Project: "libraries/job-client", Path: "JobClient.java",
			StartLine: 20, EndLine: 28, RenderMode: "declaration_body",
			Content: "List<Job> listJobs() {\n  return client.get(path);\n}",
		},
	}
	pack := ContextPack{
		SourceSections: []ContextSourceSection{nonProvingUpgrade.section},
	}

	covered := contextSourceCoverageFromFinalSections(
		pack,
		[]contextConcern{concern},
		[]contextSourceOption{proving, nonProvingUpgrade},
	)
	if covered[concern.key] {
		t.Fatalf("stale authentication proof survived final section audit: %#v", covered)
	}
}
```

- [ ] **Step 2: Run the audit test and confirm it is red**

```bash
go test ./internal/agent -run TestContextSourceCoverageFromFinalSectionsRejectsStaleProof -count=1
```

Expected: FAIL because the audit function is undefined.

- [ ] **Step 3: Implement proof reconstruction from final sections**

Add:

```go
func contextSourceCoverageFromFinalSections(
	pack ContextPack,
	concerns []contextConcern,
	options []contextSourceOption,
) map[string]bool {
	known := make(map[string]bool, len(concerns))
	for _, concern := range concerns {
		known[concern.key] = true
	}
	covered := make(map[string]bool, len(concerns))
	for _, section := range pack.SourceSections {
		for _, option := range options {
			if option.section != section {
				continue
			}
			for _, key := range option.concernKeys {
				if known[key] {
					covered[key] = true
				}
			}
		}
	}
	return covered
}
```

- [ ] **Step 4: Make the final audit authoritative**

In `selectContextSourceOptions`, after enrichment and utility selection:

```go
covered := contextSourceCoverageFromFinalSections(pack, concerns, options)
applyContextSourceCoverage(&pack, concerns, covered)
```

Pass `covered`, not `state.coveredConcerns`, to
`contextSourceEvidenceOmissionsWithOptions`. Repeat the final audit after any
later substitution or inventory step before serialization.

- [ ] **Step 5: Add aggregation assertions**

Extend the test to place two internal configuration facets under one public
key, publish only one proving section, call `applyContextSourceCoverage`, and
assert:

```go
if pack.SourceCoverage != "partial" ||
	pack.SourceUnrepresented != 1 ||
	pack.Concerns[0].Covered {
	t.Fatalf("partial final proof was reported complete: %#v", pack)
}
```

- [ ] **Step 6: Run coverage and source-selection tests**

```bash
gofmt -w internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextSourceCoverage|TestApplyContextSourceCoverage|TestContextSourceOptionsRequireCompleteMergedEvidence' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the final audit**

```bash
git add internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go
git commit -m "Audit context coverage from final source" -m "- Rebuild internal proof from the sections that are actually published
- Aggregate public coverage only after every required facet is proven
- Generate omissions from audited coverage instead of stale selection state"
```

### Task 4: Bounded proof-aware candidate frontier

**Files:**

- Modify: `internal/agent/context_source.go:166-300`
- Modify: `internal/agent/context_select.go:1390-1590`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces constants `maximumContextSourcePlanningCandidates = 8` and `maximumContextSourceProvingCandidates = 4`.
- Produces `contextSourceProofFrontier(pack ContextPack, options []contextSourceOption, concerns []contextConcern) []contextSourceOption`.
- Guarantees all render modes for a retained candidate remain together.

- [ ] **Step 1: Write the proof-frontier regression**

```go
func TestContextSourceProofFrontierLooksPastNonProvingCandidates(t *testing.T) {
	concern := newContextConcern(
		contextConcernConfiguration,
		"libraries/job-client",
		true,
		[]string{"weak-1", "weak-2", "weak-3", "weak-4", "config"},
		"requested client configuration",
	)
	options := make([]contextSourceOption, 0, 5)
	for index := 1; index <= 4; index++ {
		id := fmt.Sprintf("weak-%d", index)
		options = append(options, contextSourceOption{
			candidate: sourceCandidate{
				FactID: id, Project: "libraries/job-client",
				Path: id + ".java",
			},
			section: ContextSourceSection{
				Project: "libraries/job-client", Path: id + ".java",
				RenderMode: "signature", Content: "class Candidate {}",
			},
		})
	}
	options = append(options, contextSourceOption{
		candidate: sourceCandidate{
			FactID: "config", Project: "libraries/job-client",
			Path: "JobClientConfig.java",
		},
		section: ContextSourceSection{
			Project: "libraries/job-client", Path: "JobClientConfig.java",
			RenderMode: "declaration_body",
			Content: "@ConfigurationProperties(prefix = \"jobs\")\nclass JobClientConfig {}",
		},
		concernKeys: []string{concern.key},
	})

	got := contextSourceProofFrontier(
		ContextPack{selectedSourceFactIDs: []string{"weak-1"}},
		options,
		[]contextConcern{concern},
	)
	ids := map[string]bool{}
	for _, option := range got {
		ids[option.candidate.FactID] = true
	}
	if !ids["weak-1"] || !ids["config"] {
		t.Fatalf("proof frontier = %#v, want selected core and proving config", ids)
	}
}
```

- [ ] **Step 2: Confirm the fifth proving candidate is currently unavailable**

```bash
go test ./internal/agent -run TestContextSourceProofFrontierLooksPastNonProvingCandidates -count=1
```

Expected: FAIL because the proof-frontier function and planning ceiling do not
exist.

- [ ] **Step 3: Separate planning and proof ceilings**

In `context_source.go`:

```go
const (
	maximumContextSourcePlanningCandidates = 8
	maximumContextSourceProvingCandidates  = 4
)
```

Use `maximumContextSourcePlanningCandidates` where facts are selected for
rendering. Keep the existing same-source deduplication before this limit.

- [ ] **Step 4: Implement proof-aware option retention**

Add to `context_select.go`:

```go
func contextSourceProofFrontier(
	pack ContextPack,
	options []contextSourceOption,
	concerns []contextConcern,
) []contextSourceOption {
	coreFacts := make(map[string]bool, len(pack.selectedSourceFactIDs))
	for _, factID := range pack.selectedSourceFactIDs {
		coreFacts[factID] = true
	}
	keepCandidates := make(map[string]bool)
	for _, option := range options {
		for _, factID := range contextSourceCandidateFactIDs(option.candidate) {
			if coreFacts[factID] {
				keepCandidates[contextSourceCandidateKey(option.candidate)] = true
			}
		}
	}
	for _, concern := range concerns {
		if !concern.required {
			continue
		}
		proving := 0
		firstCandidate := ""
		for _, option := range options {
			key := contextSourceCandidateKey(option.candidate)
			if firstCandidate == "" &&
				contextSourceOptionMatchesConcernFacts(option, concern) {
				firstCandidate = key
			}
			if proving >= maximumContextSourceProvingCandidates ||
				!contextSourceOptionHasConcern(option, concern.key) ||
				keepCandidates[key] {
				continue
			}
			keepCandidates[key] = true
			proving++
		}
		if proving == 0 && firstCandidate != "" {
			keepCandidates[firstCandidate] = true
		}
	}
	result := make([]contextSourceOption, 0, len(options))
	for _, option := range options {
		if keepCandidates[contextSourceCandidateKey(option.candidate)] {
			result = append(result, option)
		}
	}
	return result
}
```

Implement the exact fact matcher used above:

```go
func contextSourceOptionMatchesConcernFacts(
	option contextSourceOption,
	concern contextConcern,
) bool {
	for _, factID := range concern.candidateFactIDs {
		if contextSourceCandidateHasFact(option.candidate, factID) {
			return true
		}
	}
	return false
}
```

Call `contextSourceProofFrontier` immediately after
`contextSourceRenderOptionsWithModels`.

- [ ] **Step 5: Prove determinism and bounded growth**

Extend the frontier test by reversing `options`, sorting both results with the
existing option ordering, and asserting identical candidate keys. Assert no
concern retains more than four distinct proving candidate keys.

Run:

```bash
gofmt -w internal/agent/context_source.go internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestContextSourceProofFrontier|TestContextSourceConcernCandidateGrowthIsSubquadratic|TestContextSourceRenderOptionIndexGrowthStaysBounded' -count=1
```

Expected: PASS without relaxing existing growth assertions.

- [ ] **Step 6: Commit the bounded frontier**

```bash
git add internal/agent/context_source.go internal/agent/context_select.go internal/agent/context_source_test.go
git commit -m "Bound source candidates by rendered proof" -m "- Search a fixed planning frontier beyond non-proving ranked facts
- Retain at most four proving candidates for each required concern
- Preserve core candidates, deterministic ordering, and growth ceilings"
```

### Task 5: Strictly improving source substitution

**Files:**

- Create: `internal/agent/context_substitute.go`
- Modify: `internal/agent/context_select.go:30-190`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `improveContextSourceSelection(base ContextPack, current ContextPack, request ContextRequest, options []contextSourceOption, concerns []contextConcern, boundaries []contextSourceBoundary) (ContextPack, error)`.
- Produces: `rebuildContextSourceSelection(base ContextPack, request ContextRequest, options []contextSourceOption, concerns []contextConcern) (ContextPack, contextSourceSelectionState, error)`.
- Improvement vector is required-proof count, requested-identity quality, negative estimated tokens, and deterministic option key.

- [ ] **Step 1: Write failing substitution invariants**

Add this complete test:

```go
func TestImproveContextSourceSelectionReplacesOnlyUnprotectedEvidence(t *testing.T) {
	entryConcern := newContextConcern(
		contextConcernEntrypoint,
		"",
		true,
		[]string{"entrypoint"},
		"selected entrypoint",
	)
	authConcern := newContextEvidenceConcern(
		newContextConcern(
			contextConcernAuth,
			"libraries/job-client",
			true,
			[]string{"client-auth"},
			"selected client authentication",
		),
		"client_transport",
		[]string{"client-auth"},
		"client transport authentication",
	)
	configConcern := newContextEvidenceConcern(
		newContextConcern(
			contextConcernConfiguration,
			"libraries/job-client",
			true,
			[]string{"client-config"},
			"selected client configuration",
		),
		"binding",
		[]string{"client-config"},
		"client configuration binding",
	)
	option := func(
		id string,
		path string,
		content string,
		keys ...string,
	) contextSourceOption {
		return contextSourceOption{
			candidate: sourceCandidate{
				FactID: id, FactIDs: []string{id},
				Project: "libraries/job-client", Path: path,
			},
			section: ContextSourceSection{
				Project: "libraries/job-client", Path: path,
				StartLine: 1, EndLine: 4,
				RenderMode: "declaration_body", Content: content,
			},
			estimated: 40, concernKeys: keys,
			projectKey: "libraries/job-client", profiled: true,
		}
	}
	entry := option(
		"entrypoint",
		"Entrypoint.java",
		"void deleteItem() { service.delete(); }",
		entryConcern.key,
	)
	entry.candidate.Role = "entrypoint"
	generic := option(
		"generic",
		"GenericHelper.java",
		"void format() { formatter.apply(); }",
	)
	auth := option(
		"client-auth",
		"JobClientAuth.java",
		"void apply() { headers.setBasicAuth(user, password); }",
		authConcern.key,
	)
	base := ContextPack{
		Schema: 1, Query: "delete jobs with authentication",
		Confidence: "EXACT", BudgetTokens: DefaultContextBudgetTokens,
	}
	current := cloneContextPack(base)
	current.SourceSections = []ContextSourceSection{entry.section, generic.section}
	var err error
	current, err = finalizeContextEstimate(current)
	if err != nil {
		t.Fatal(err)
	}
	request := ContextRequest{
		BudgetTokens: DefaultContextBudgetTokens,
		MaxFiles:     2,
	}
	concerns := []contextConcern{entryConcern, authConcern}
	options := []contextSourceOption{entry, generic, auth}

	got, err := improveContextSourceSelection(
		base,
		current,
		request,
		options,
		concerns,
		[]contextSourceBoundary{{factID: "entrypoint"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	paths := contextSourcePathSet(got)
	if !paths["Entrypoint.java"] || !paths["JobClientAuth.java"] ||
		paths["GenericHelper.java"] {
		t.Fatalf("substitution selected %#v", paths)
	}

	reversed := slices.Clone(options)
	slices.Reverse(reversed)
	reversedPack, err := improveContextSourceSelection(
		base,
		current,
		request,
		reversed,
		concerns,
		[]contextSourceBoundary{{factID: "entrypoint"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reversedPack.SourceSections, got.SourceSections) {
		t.Fatalf(
			"reversed options changed substitution:\ngot  %#v\nwant %#v",
			reversedPack.SourceSections,
			got.SourceSections,
		)
	}

	protected := option(
		"client-config",
		"JobClientConfig.java",
		"@ConfigurationProperties(prefix = \"jobs\") class JobClientConfig {}",
		configConcern.key,
	)
	protectedCurrent := cloneContextPack(base)
	protectedCurrent.SourceSections = []ContextSourceSection{
		entry.section,
		protected.section,
	}
	protectedCurrent, err = finalizeContextEstimate(protectedCurrent)
	if err != nil {
		t.Fatal(err)
	}
	protectedPack, err := improveContextSourceSelection(
		base,
		protectedCurrent,
		request,
		[]contextSourceOption{entry, protected, auth},
		[]contextConcern{entryConcern, configConcern, authConcern},
		[]contextSourceBoundary{{factID: "entrypoint"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	protectedPaths := contextSourcePathSet(protectedPack)
	if !protectedPaths["Entrypoint.java"] ||
		!protectedPaths["JobClientConfig.java"] ||
		protectedPaths["JobClientAuth.java"] {
		t.Fatalf("unique required proof was removed: %#v", protectedPaths)
	}
}
```

- [ ] **Step 2: Run the substitution tests and verify they fail**

```bash
go test ./internal/agent -run 'TestImproveContextSourceSelection' -count=1
```

Expected: FAIL because the repair interface is undefined.

- [ ] **Step 3: Implement pack reconstruction**

Create `context_substitute.go` with:

```go
package agent

import (
	"fmt"
	"sort"
)

func rebuildContextSourceSelection(
	base ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
) (ContextPack, contextSourceSelectionState, error) {
	pack := cloneContextPack(base)
	pack.SourceSections = nil
	pack.SourceOmissions = nil
	state := newContextSourceSelectionState(len(options), len(concerns))
	ordered := append([]contextSourceOption(nil), options...)
	sort.Slice(ordered, func(i, j int) bool {
		return contextSourceOptionLess(ordered[i], ordered[j])
	})
	var err error
	for _, option := range ordered {
		pack, state, err = addContextSourceOption(
			pack,
			request,
			option,
			concerns,
			state,
		)
		if err != nil {
			return ContextPack{}, contextSourceSelectionState{}, err
		}
	}
	return pack, state, nil
}
```

Extract the state initialization currently in `selectContextSourceOptions`:

```go
func newContextSourceSelectionState(
	candidateCount int,
	concernCount int,
) contextSourceSelectionState {
	return contextSourceSelectionState{
		selectedCandidates:       make(map[string]bool, candidateCount),
		selectedFactIDs:          make(map[string]bool, candidateCount),
		selectedProjects:         make(map[string]bool),
		coveredConcerns:          make(map[string]bool, concernCount),
		coveredRoles:             make(map[string]bool),
		selectedEvidenceFamilies: make(map[string]int),
	}
}
```

- [ ] **Step 4: Implement the monotone replacement loop**

Represent the comparison explicitly:

```go
type contextSourceSelectionScore struct {
	requiredProofs  int
	identityQuality int
	estimatedTokens int
	key             string
}

func betterContextSourceSelection(
	left contextSourceSelectionScore,
	right contextSourceSelectionScore,
) bool {
	if left.requiredProofs != right.requiredProofs {
		return left.requiredProofs > right.requiredProofs
	}
	if left.identityQuality != right.identityQuality {
		return left.identityQuality > right.identityQuality
	}
	if left.estimatedTokens != right.estimatedTokens {
		return left.estimatedTokens < right.estimatedTokens
	}
	return left.key < right.key
}
```

`improveContextSourceSelection` must:

1. Resolve the currently selected option for each published section.
2. Mark selected candidates containing a mandatory boundary fact as
   non-removable.
3. For each unselected proving option and each removable selected option,
   rebuild `base + kept + replacement`.
4. Recompute proof with `contextSourceCoverageFromFinalSections`.
5. Reject candidates that lose any previously proven required key.
6. Accept only `betterContextSourceSelection`.
7. Repeat no more than `len(options)` times and reject a repeated deterministic
   selection key with an internal error.

Use:

```go
return ContextPack{}, fmt.Errorf(
	"context source substitution repeated selection %q",
	selectionKey,
)
```

for the impossible cycle guard.

- [ ] **Step 5: Integrate substitution before final coverage**

Capture `basePack := cloneContextPack(pack)` before clearing source sections.
After normal utility selection:

```go
pack, err = improveContextSourceSelection(
	basePack,
	pack,
	sectionRequest,
	options,
	concerns,
	coreBoundaries,
)
if err != nil {
	return ContextPack{}, err
}
covered := contextSourceCoverageFromFinalSections(pack, concerns, options)
applyContextSourceCoverage(&pack, concerns, covered)
```

- [ ] **Step 6: Run substitution, budget, and determinism tests**

```bash
gofmt -w internal/agent/context_substitute.go internal/agent/context_select.go internal/agent/context_source_test.go
go test ./internal/agent -run 'TestImproveContextSourceSelection|TestContextSourceOptionsRespectFileBudget|TestContextSourceSelectionKeepsProductionModelsBesideExplicitTestRole' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit deterministic substitution**

```bash
git add internal/agent/context_substitute.go internal/agent/context_select.go internal/agent/context_source_test.go
git commit -m "Substitute stronger context evidence within budget" -m "- Rebuild candidate packs from the source-free base pack
- Replace only removable evidence when required proof strictly improves
- Preserve mandatory boundaries, existing proofs, budgets, and determinism"
```

### Task 6: Bounded evidence-file inventory and release-shaped regression

**Files:**

- Modify: `internal/agent/context_proof.go`
- Modify: `internal/agent/context_select.go:144-190,3460-3545`
- Modify: `internal/agent/context_change_analysis_test.go`
- Test: `internal/agent/context_source_test.go`

**Interfaces:**

- Produces: `appendContextEvidenceInventory(pack ContextPack, request ContextRequest, options []contextSourceOption, concerns []contextConcern) (ContextPack, error)`.
- Keeps metadata inventory independent of source coverage proof.
- Publishes exact indexed paths only; it never creates a future filename.

- [ ] **Step 1: Add the generic release-shaped regression**

Create `releaseQualityMissingContractIndex()` in
`context_change_analysis_test.go` by starting from
`missingContractContextIndex()` and adding these generic facts:

```go
scan.AgentContextFactRecord{
	ID: "consumer-duplicate-model", Project: "services/catalog", Kind: "symbol",
	Name: "CatalogJobEntity", Qualified: "catalog.CatalogJobEntity",
	File: "src/main/java/example/CatalogJobEntity.java",
	Line: 8, EndLine: 9, Confidence: "EXACT",
	Search: "catalog job task model catalogId itemId",
},
scan.AgentContextFactRecord{
	ID: "base-job-model", Project: "services/jobs", Kind: "symbol",
	Name: "BaseCatalogJobEntity", Qualified: "jobs.BaseCatalogJobEntity",
	File: "src/main/java/example/BaseCatalogJobEntity.java",
	Line: 8, EndLine: 13, Confidence: "EXACT",
	Search: "job task base model catalogId itemId",
},
scan.AgentContextFactRecord{
	ID: "job-client-config", Project: "libraries/job-client", Kind: "configuration",
	Name: "JobClientConfig", Qualified: "client.JobClientConfig",
	File: "src/main/java/example/JobClientConfig.java",
	Line: 8, EndLine: 16, Confidence: "EXACT",
	Search: "job client configuration base url credentials timeout retries",
},
scan.AgentContextFactRecord{
	ID: "job-client-auth", Project: "libraries/job-client", Kind: "authentication",
	Name: "applyBasicAuthentication", Qualified: "client.JobClientAuth.apply",
	File: "src/main/java/example/JobClientAuth.java",
	Line: 8, EndLine: 12, Confidence: "EXACT",
	Search: "job client basic authentication credentials",
},
scan.AgentContextFactRecord{
	ID: "job-server-policy", Project: "services/jobs", Kind: "authentication",
	Name: "securityFilterChain", Qualified: "jobs.JobSecurity.securityFilterChain",
	File: "src/main/java/example/JobSecurity.java",
	Line: 8, EndLine: 16, Confidence: "EXACT",
	Search: "job server basic authentication technical role security policy",
},
scan.AgentContextFactRecord{
	ID: "jobs-service-test", Project: "services/jobs", Kind: "test",
	Name: "deletesBothJobVariants", Qualified: "jobs.JobServiceTest.deletesBothJobVariants",
	File: "src/test/java/example/JobServiceTest.java",
	Line: 8, EndLine: 15, Confidence: "EXACT",
	Search: "job deletion persistence side effects test",
},
```

Add exact `extends` edges from both requested provider models to
`base-job-model`. Override the generic fixture files with:

```java
// BaseCatalogJobEntity.java
class BaseCatalogJobEntity {
  long catalogId;
  long itemId;
}

// JobClientConfig.java
@ConfigurationProperties(prefix = "jobs")
class JobClientConfig {
  String baseUrl;
  String username;
  String password;
  Duration connectTimeout;
  Duration readTimeout;
  int maxRetries;
}

// JobClientAuth.java
class JobClientAuth {
  void apply(HttpHeaders headers, JobClientConfig config) {
    headers.setBasicAuth(config.username, config.password);
  }
}

// JobSecurity.java
class JobSecurity {
  SecurityFilterChain securityFilterChain(HttpSecurity http) {
    return http.securityMatcher("/job-management/**")
      .authorizeHttpRequests(auth -> auth.anyRequest().hasRole("TECHNICAL_USER"))
      .httpBasic(Customizer.withDefaults()).build();
  }
}
```

Add `TestBuildContextProvesReleaseQualityWithoutPrivateRules`. It must assert:

```go
for _, want := range []string{
	"BaseCatalogJobEntity.java",
	"JobClientConfig.java",
	"JobClientAuth.java",
	"JobSecurity.java",
	"CatalogJobRepository.java",
	"CatalogChangeJobRepository.java",
	"JobManagementControllerTest.java",
	"JobServiceTest.java",
} {
	if !contextPackContainsFileSuffix(pack, want) {
		t.Errorf("required production/test inventory %q missing", want)
	}
}
for _, want := range []string{
	"catalogId",
	"itemId",
	"setBasicAuth",
	"TECHNICAL_USER",
	"@ConfigurationProperties",
} {
	if !contextSourceContainsStableIdentity(pack, want) {
		t.Errorf("rendered evidence %q missing", want)
	}
}
if contextPackContainsFileSuffix(pack, "services/catalog/src/main/java/example/CatalogJobEntity.java") {
	t.Fatal("wrong-project duplicate model displaced provider evidence")
}
if pack.EstimatedTokens > DefaultContextBudgetTokens ||
	len(pack.Files) > DefaultContextMaxFiles ||
	len(pack.SourceSections) > MaxContextSourceSections {
	t.Fatalf("release-quality pack exceeds limits: %#v", pack)
}
```

Add this test helper next to `contextSourceContainsStableIdentity`:

```go
func contextPackContainsFileSuffix(pack ContextPack, suffix string) bool {
	suffix = filepath.ToSlash(suffix)
	for _, file := range pack.Files {
		if strings.HasSuffix(filepath.ToSlash(file.Path), suffix) {
			return true
		}
	}
	for _, section := range pack.SourceSections {
		if strings.HasSuffix(filepath.ToSlash(section.Path), suffix) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the end-to-end regression and capture the exact missing paths**

```bash
go test ./internal/agent -run TestBuildContextProvesReleaseQualityWithoutPrivateRules -count=1 -v
```

Expected: FAIL because the current pack does not retain the complete required
production/test inventory under the fixed limits.

- [ ] **Step 3: Generalize selected required-evidence file projection**

Replace the two specialized checks in `contextProjectedSourceFile` with one
required-evidence projection:

```go
func contextProjectedRequiredEvidenceFile(
	option contextSourceOption,
	concerns []contextConcern,
) (ContextFile, bool) {
	for _, concern := range concerns {
		if !concern.required ||
			!contextSourceOptionHasConcern(option, concern.key) {
			continue
		}
		return ContextFile{
			Project:   option.section.Project,
			Path:      option.section.Path,
			StartLine: option.section.StartLine,
			EndLine:   option.section.EndLine,
			Role:      contextSourceConcernRole(concern.kind),
			Reason:    "selected required " + strings.ReplaceAll(concern.kind, "_", " ") + " evidence",
		}, true
	}
	return ContextFile{}, false
}
```

Call this first from `contextProjectedSourceFile`. Keep the existing
client-support and side-effect helpers only if focused tests prove they add a
different required file; otherwise remove those now-redundant branches in this
same commit.

- [ ] **Step 4: Publish bounded metadata for remaining required roles**

Add to `context_proof.go`:

```go
func appendContextEvidenceInventory(
	pack ContextPack,
	request ContextRequest,
	options []contextSourceOption,
	concerns []contextConcern,
) (ContextPack, error) {
	ordered := append([]contextSourceOption(nil), options...)
	sort.Slice(ordered, func(i, j int) bool {
		return contextSourceOptionLess(ordered[i], ordered[j])
	})
	represented := make(map[string]bool)
	for _, file := range pack.Files {
		represented[normalizeContextProject(file.Project)+"\x00"+file.Role] = true
	}
	for _, option := range ordered {
		file, publish := contextProjectedRequiredEvidenceFile(option, concerns)
		if !publish {
			continue
		}
		roleKey := normalizeContextProject(file.Project) + "\x00" + file.Role
		if represented[roleKey] {
			continue
		}
		candidate := cloneContextPack(pack)
		if !mergeContextFile(&candidate, file, request.MaxFiles) {
			continue
		}
		candidate, err := finalizeContextEstimate(candidate)
		if err != nil {
			return ContextPack{}, err
		}
		fits, err := contextSourcePackFits(candidate, request)
		if err != nil {
			return ContextPack{}, err
		}
		if fits {
			pack = candidate
			represented[roleKey] = true
		}
	}
	return pack, nil
}
```

This function publishes metadata only. Do not add keys to the coverage map.

- [ ] **Step 5: Integrate inventory and rerun the final audit**

After substitution:

```go
pack, err = appendContextEvidenceInventory(
	pack,
	sectionRequest,
	options,
	concerns,
)
if err != nil {
	return ContextPack{}, err
}
covered := contextSourceCoverageFromFinalSections(pack, concerns, options)
applyContextSourceCoverage(&pack, concerns, covered)
```

- [ ] **Step 6: Prove inventory bounds and no false coverage**

Add this focused unit test:

```go
func TestAppendContextEvidenceInventoryIsBoundedAndDoesNotCreateCoverage(t *testing.T) {
	const optionCount = 20
	concerns := make([]contextConcern, 0, optionCount)
	options := make([]contextSourceOption, 0, optionCount)
	for index := 0; index < optionCount; index++ {
		key := fmt.Sprintf("domain_model:project-%02d", index)
		project := fmt.Sprintf("services/project-%02d", index)
		concerns = append(concerns, contextConcern{
			key:      key,
			kind:     contextConcernDomainModel,
			project:  project,
			required: true,
		})
		options = append(options, contextSourceOption{
			candidate: sourceCandidate{
				FactID:  fmt.Sprintf("model-%02d", index),
				Project: project,
				Path:    fmt.Sprintf("src/Model%02d.java", index),
				Role:    contextConcernDomainModel,
			},
			section: ContextSourceSection{
				Project:    project,
				Path:       fmt.Sprintf("src/Model%02d.java", index),
				StartLine:  1,
				EndLine:    3,
				Role:       contextConcernDomainModel,
				RenderMode: "declaration_body",
				Content:    fmt.Sprintf("class Model%02d {}", index),
			},
			concernKeys: []string{key},
			projectKey: project,
			required:   true,
		})
	}
	request := ContextRequest{
		BudgetTokens: DefaultContextBudgetTokens,
		MaxFiles:     DefaultContextMaxFiles,
	}
	pack, err := finalizeContextEstimate(ContextPack{
		Schema:       1,
		Query:        "compare required domain models",
		BudgetTokens: request.BudgetTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := appendContextEvidenceInventory(pack, request, options, concerns)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != DefaultContextMaxFiles {
		t.Fatalf("inventory files = %d, want %d", len(got.Files), DefaultContextMaxFiles)
	}
	covered := contextSourceCoverageFromFinalSections(got, concerns, options)
	for _, concern := range concerns {
		if covered[concern.key] {
			t.Fatalf("metadata-only inventory became source coverage for %q", concern.key)
		}
	}
}
```

Add `fmt` to the test file's imports. This test intentionally assigns a
different project to every role so project/role deduplication does not hide the
12-file bound.

Run:

```bash
gofmt -w internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go internal/agent/context_change_analysis_test.go
go test ./internal/agent -run 'TestBuildContextProvesReleaseQualityWithoutPrivateRules|TestAppendContextEvidenceInventory|TestBuildContextSupportsMissingContractChangeAnalysis' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the bounded inventory**

```bash
git add internal/agent/context_proof.go internal/agent/context_select.go internal/agent/context_source_test.go internal/agent/context_change_analysis_test.go
git commit -m "Publish bounded required evidence inventory" -m "- Project exact indexed files for requested production and test roles
- Keep metadata inventory separate from rendered source coverage
- Cover the release-shaped quality gaps with a generic cross-service regression"
```

### Task 7: Full local acceptance and private smoke handoff

**Files:**

- Verify only; no production file changes are expected.

**Interfaces:**

- Consumes: the exact clean candidate commit produced by Tasks 1-6.
- Produces: a locally installed candidate binary, a fresh agent index in the authorized historical workspace, and a recorded stop point before external execution.

- [ ] **Step 1: Run the focused package repeatedly**

```bash
go test ./internal/agent -count=10
```

Expected: PASS on all ten executions with no nondeterministic failure.

- [ ] **Step 2: Run the full local verification matrix**

```bash
go test ./... -count=1
go vet ./...
bash scripts/analyze-agent-context-log_test.sh
bash scripts/benchmark-agent-context-regression_test.sh
bash scripts/benchmark-agent-context_test.sh
go run ./scripts/sync-docs --check
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 3: Verify frozen pack limits explicitly**

```bash
go test ./internal/agent -run 'TestDefaultContextPackStaysWithinTokenAndByteBudgets|TestContextHardCeilingsSurviveHighCardinalityInput|TestBuildContextProvesReleaseQualityWithoutPrivateRules' -count=10
```

Expected: PASS; no pack exceeds 4,000 tokens, 12 files, or 12 source sections.

- [ ] **Step 4: Commit any formatting-only residue separately**

Run `git status --short`. If formatting produced tracked changes, inspect them
and commit only those exact files:

```bash
git add internal/agent
git commit -m "Format context evidence stabilization" -m "- Apply gofmt to the completed context evidence changes"
```

If the worktree is already clean, do not create an empty commit.

- [ ] **Step 5: Install the exact candidate**

Record the commit first:

```bash
git rev-parse HEAD
go install ./cmd/goregraph
goregraph version
```

Expected: `goregraph version` reports the local 1.3.0 source build and its
recorded commit metadata.

- [ ] **Step 6: Preview, clean, and rebuild the authorized workspace**

The executor must set `G1_WORKSPACE` to the authorized absolute historical
workspace path outside the repository before running:

```bash
test -n "${G1_WORKSPACE:?set G1_WORKSPACE to the authorized historical workspace}"
goregraph workspace clean "$G1_WORKSPACE" --workspace "$G1_WORKSPACE"
goregraph workspace clean "$G1_WORKSPACE" --workspace "$G1_WORKSPACE" --execute
goregraph workspace scan-all "$G1_WORKSPACE" --workspace "$G1_WORKSPACE"
goregraph doctor "$G1_WORKSPACE"
```

Expected: the preview names only generated GoreGraph outputs; the clean and
scan succeed; Doctor reports no stale or invalid agent index.

- [ ] **Step 7: Inspect the candidate Context Pack without an external model**

Run the frozen focused query once with `goregraph context`, save the JSON or
Markdown output outside the repository, and verify:

- provider task model or linked base fields prove both lookup attributes;
- Basic Authentication client and provider policy are present;
- client configuration properties are present;
- both repositories and relevant tests appear in the bounded file inventory;
- no private-name rule exists in the binary or repository diff;
- the pack stays within all three frozen limits.

Do not change ranking after this inspection unless a generic local regression
also demonstrates the same defect.

- [ ] **Step 8: Stop for active external-run authorization**

Do not start the one-run private G1 smoke automatically. Report:

- candidate commit;
- local test results;
- installed binary identity;
- scan/Doctor result;
- Context ID, estimated tokens, file count, source coverage, and omissions;
- the exact requested authorization: one read-only assisted smoke run against
  the authorized historical workspace.

The subsequent token-metric and release-qualification plan starts only after
this product plan passes locally.
