# Compact Production Plan Evidence Design

**Status:** Approved under the operator's 12-hour project authorization on 2026-08-02.

## Context

The controlled assisted release smoke proved that value-free configuration metadata and dependent-persistence uncertainty can fit within the 4,000-token Context Pack limit, but the final budget pass can displace exact existing provider and primary-persistence file identities. The resulting answer can describe the intended behavior while still omitting the concrete production files needed for a safe implementation plan.

This is a context-selection defect. It must be corrected without increasing the token budget, authorizing additional source reads, adding benchmark-specific identifiers, or weakening uncertainty around future contracts and cascade behavior.

## Goals

- Preserve one exact existing provider contract file and up to two exact primary-persistence files when a query requests an exact production-file inventory for a missing cross-service transition.
- Keep configuration production/test-profile identities, dependent-persistence uncertainty, test-plan identities, and the existing source evidence within the same hard limits.
- Expose only value-free identity metadata. Production-plan metadata never authorizes reading a file.
- Keep selection deterministic and repository-neutral.

## Non-goals

- Do not infer a future route, method, status, lookup implementation, deletion ordering, cascade policy, or compensation strategy.
- Do not increase the 4,000-token budget, source-section limit, file limit, or bounded-omission limit.
- Do not hard-code project, class, repository, path, prompt, or language-specific identifiers.
- Do not change the scanner or the historical benchmark workspace.

## Data model

### Grouped configuration resources

Replace the repeated per-resource configuration metadata shape with deterministic groups that share an exact project and key-group set:

```text
configuration_resources:
  - project
    key_groups
    resources:
      - path
        profile
```

Resources with different key groups remain in different groups. Grouping changes representation only; it must not merge evidence that has different semantics. The total resource cap remains six.

### Production plan files

Add an optional, compact Schema 3 field:

```text
production_plan_files:
  - project
    provider_contract
    primary_persistence
```

`provider_contract` is one exact project-relative path. `primary_persistence` contains at most two exact project-relative paths from distinct primary domain-model families. Empty fields are omitted. The field is additive and backward-compatible for Schema 3 consumers.

## Selection

Production-plan metadata is eligible only when all of these conditions hold:

1. The query requests an exact production-file inventory.
2. The query plans a missing cross-service transition.
3. A single reliable entrypoint project and at least one provider project can be derived from existing selected evidence.
4. The candidate fact is exact, production-scoped, project-relative, and belongs to an explicitly requested or derived provider project.
5. The path is not already represented by files, source sections, bounded omissions, plan files, or other supplemental metadata.

Provider-contract selection prefers an existing internal/management route or handler connected to the selected client/provider area. It remains an existing-file identity only and does not imply that the future operation already exists.

Primary-persistence selection reuses the existing domain-model matching and persistence-family rules. It excludes dependent/comment repositories, generic persistence operations, foreign-project facts, and unrelated domain families. Dependent repositories continue to be reported separately as an uncertainty with unknown ordering and cascade behavior.

Candidates are scored and sorted with existing semantic signals, then by normalized project and path. Input order must not affect output.

## Budget behavior

Configuration grouping funds most of the new compact production identity without exposing values. The final Context Pack loop remains authoritative: if the combined pack still does not fit, normal deterministic budget reduction applies. No metadata may bypass `EstimatedTokens`, byte limits, or file/source ceilings.

The implementation must prove that a 4,000-token runtime-shaped fixture retains:

- existing entrypoint and current path evidence;
- client authentication, configuration, and retry evidence;
- grouped production/test-profile resources;
- one provider contract identity;
- two primary-persistence identities;
- dependent-persistence/cascade uncertainty;
- production/test plan identities;
- bounded source omissions only.

## Rendering and agent behavior

Markdown renders a dedicated heading stating that production-plan identities are metadata only and must not be read. Each group names the project, provider contract path, and primary-persistence paths.

The canonical agent instruction requires separate production- and test-file inventories. It must name every supplied production-plan identity with its role, while continuing to treat future filenames and unproven implementation details as unknown. The benchmark harness must accept exactly the canonical instruction.

## Testing

Follow strict RED-GREEN development:

1. Extend a generic runtime-shaped release fixture so configuration identities become supplemental under a tight budget.
2. Assert that the current candidate loses the provider contract and two primary repositories.
3. Add deterministic unit tests for grouping, eligibility, foreign-project rejection, dependent-repository rejection, represented-path deduplication, caps, and input-order independence.
4. Implement the smallest production change that satisfies those tests.
5. Verify renderer JSON/Markdown, canonical instruction synchronization, benchmark harness behavior, complete agent tests, repository tests, race tests, Go 1.23 compatibility, and release cross-builds.

## Release validation

After local gates pass, commit and push independent documentation and implementation changes, install the exact candidate with commit/build metadata, clean only generated GoreGraph outputs in the historical workspace, rescan all projects, and require unchanged source identity plus all Doctor checks.

The real Context Pack must be deterministic, stay within 4,000 tokens, and contain the newly required production-plan identities alongside the previously corrected configuration and dependent-persistence evidence. A fresh external assisted smoke is required before any three-by-three matrix. No release, tag, merge, or historical source modification is part of this work.
