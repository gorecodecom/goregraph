# Balanced Release Evidence Ranking Design

**Status:** Approved through the standing autonomous improvement authorization on 2026-08-01

## Purpose

Correct the release-benchmark quality regression in which a 4,000-token Context Pack proves the core flow, models, persistence, and side effects but leaves requested provider authentication, provider configuration, and exact production/test file inventory unrepresented.

The change must improve general evidence balance and make exact conventional configuration-resource paths available without exposing their values. It must not encode the private benchmark workspace, increase limits, alter the benchmark prompt, or weaken any release gate.

## Evidence and root cause

Release matrix 1 for candidate `dca623e` passed every automatic token, structural, workspace, and skill-isolation gate. Assisted runs were deterministic and used about ten percent of the baseline median effective tokens, but their median manual quality score was 10/12 versus the baseline median of 12/12.

All assisted runs missed the same two report areas:

- complete client and provider authentication/configuration evidence;
- exact internal controller, property, and test file inventory.

The Context Pack used 3,939 of 4,000 tokens. It published multiple internal domain-model and persistence facets while provider authentication and configuration remained uncovered. Its three bounded omissions were assigned to side effects, a mail test, and a second repository. The current selectors count internal facets before the public evidence areas requested by the user, so multi-model concerns can outweigh completion of authentication, configuration, contract, or test evidence.

The project agent index also contains no facts for Spring `application` or `bootstrap` property/YAML resources. Ranking cannot publish an exact configuration-file target that is absent from the index. Raw values in these files may contain credentials, so adding ordinary source facts without value redaction would be unsafe.

## Fixed boundaries

- Keep the 4,000-token Context Pack limit.
- Keep the 12-file and 12-source-section limits.
- Keep the three bounded source-omission limit.
- Preserve mandatory entrypoint and current-path evidence.
- Preserve deterministic output and bounded candidate discovery/substitution.
- Do not add retries, fallbacks, dependencies, prompt instructions, or private identifiers.
- Do not place configuration values in the agent index or rendered Context Pack.
- Do not relax token, source-read, tool-call, workspace, skill-read, or manual quality gates.

## Considered approaches

### Reorder omissions only

Prioritize authentication and configuration in the existing three omissions. This has the smallest footprint, but it leaves the main Context Pack imbalanced and makes the agent spend source reads to recover evidence GoreGraph already indexed.

### Increase output limits

Publish more source or files. This would conceal the ranking defect by spending more tokens and would invalidate the established efficiency contract.

### Balance complete public evidence areas

Score a selection first by the number of fully proven public evidence areas, then by internal proofs and identity quality. Apply the same principle to exact file inventory. Finally, order omissions toward requested public areas that still have no usable representation.

This is the selected approach because it repairs the causal ranking defect within every existing limit.

## Design

### Safe configuration-resource inventory

Project scanning emits bounded internal configuration facts for conventional Spring `application` and `bootstrap` `.properties`, `.yml`, and `.yaml` resources. Facts contain only the portable relative path, profile name, property-key group, and line range. Search text is built from keys and profile metadata; values are never copied into index fields.

The source renderer masks every property or YAML value before a configuration resource can enter a Context Pack. This applies regardless of whether the value appears secret. Exact paths and line numbers remain useful, while credentials and environment-specific values remain local. A file-level fact keeps an existing configuration file discoverable even when the requested future keys are not present yet.

Configuration-resource discovery is capped per file and sorted deterministically. It changes only the internal agent projection; no public schema, CLI, or general language-depth claim changes.

### Public-area completion score

The substitution score gains a leading `requiredPublicProofs` dimension. An internal concern maps to its existing `publicKey`; a public area counts only when every required internal facet under that key is covered by the final rendered source.

Selection comparison becomes:

1. number of completely proven required public areas;
2. number of proven required internal facets;
3. requested identity quality;
4. lower estimated token cost;
5. stable deterministic key.

This keeps multi-model precision without allowing two model or repository facets to automatically outrank completion of a separately requested authentication or configuration area.

### Balanced exact file inventory

Inventory scoring groups exact concern facets by public evidence area as well as retaining internal facet and path counts. It prefers coverage of more requested production areas before adding another path for an already represented area. Executable test evidence retains a bounded place after required production coverage, and distinct requested model repositories remain eligible through the existing internal-facet comparison.

The inventory remains metadata-only where source bodies do not fit. Every path must still come from indexed and profiled evidence.

### Unrepresented-area omission priority

For missing-transition plans, bounded omissions prefer requested public areas that have neither a published source section nor an exact inventory path. Static concern-kind priorities remain tie-breakers. Already represented side-effect, model, or persistence families cannot consume all three omissions while an explicitly requested authentication, configuration, contract, or test family has no usable evidence.

Existing-flow omission ordering remains unchanged.

### Final audit and error handling

The existing final-section proof audit remains authoritative. A public area is complete only when all required internal facets are proved. If no proving source fits, coverage stays partial and the best exact bounded omission is emitted. No missing evidence is converted into an inferred claim.

## Testing

Test-driven implementation starts with a broad, release-shaped Java/Spring query that requests evidence categories and exact production/test files without naming the desired implementation classes. The existing generic fixture supplies competing model, repository, client auth/config, server policy, controller, and executable test evidence.

A scanner regression first proves that production and test-profile Spring resources appear as bounded configuration facts containing keys but no values. A renderer regression proves that `.properties` and YAML values are masked before serialization.

The regression must fail before production changes and then prove that:

- authentication and configuration are no longer displaced by repeated model/persistence evidence;
- the internal controller/security/configuration and relevant test paths are represented by source, inventory, or bounded omissions;
- production and test-profile configuration resources are represented without leaking any value;
- model identity and both repository variants remain honestly represented;
- output stays within 4,000 tokens, 12 files, 12 source sections, and three omissions;
- output is byte-stable across repeated builds;
- generic matrix, growth, shell, documentation, and release benchmark gates remain unchanged.

After local verification and independent review, the exact committed candidate is installed and the historical workspace is freshly scanned. Release matrix 2 uses the identical prompt, model, reasoning level, sandbox, workspace snapshot, and controlled skill configuration. A release candidate passes only if automatic gates pass and the assisted median manual quality score is at least the baseline median.
