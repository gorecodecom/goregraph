# Coherent Release-Plan Evidence Design

**Status:** Approved by the user's instruction to continue with the recommended narrow ranking correction on 2026-08-01

## Purpose

Close the remaining release-benchmark quality gap without increasing the 4,000-token, 12-file, 12-section, or three-omission limits. A natural cross-service change-analysis request must receive one coherent evidence set for production configuration and the production/test file inventory instead of unrelated fragments from each category.

## Root cause

The generic release regression models configuration owners as `configuration` facts and executable tests as `test` facts. The real scanner commonly emits Java configuration owners and test classes as `symbol` facts, repository owners alongside method-level `persistence` facts, and Spring resources as separate configuration facts. Exact-inventory expansion therefore misses the configuration owner and does not reserve both repository siblings. Its eight-path planning cap and category-first balancing can then spend slots on duplicate authentication or weak configuration evidence while the final answer lacks a complete client config, both repositories, or the production/test-profile files.

The Matrix-4 Context Packs show the result: two runs contain the server technical role and credentials resource but not the client configuration class; another contains the configuration class and both repositories but not the server rule. No run receives the complete exact file inventory needed by the binary quality rubric.

## Considered approaches

1. **Increase output budgets.** This would mask the ranking defect and weaken the established efficiency contract.
2. **Add more global score bonuses.** This is small but risks another oscillation because independently scored paths still do not form a coherent set.
3. **Recognize runtime fact shapes and reserve a bounded coherent bundle.** This keeps every public limit unchanged while making exact inventory reflect real scanner output. This is the selected approach.

## Design

Exact inventory accepts generic, source-backed owner shapes in addition to category facts:

- production symbol owners named like `*Config`, `*Configuration`, `*Properties`, or `*Settings` may prove a requested configuration file;
- repository owner symbols and persistence facts may prove requested persistence files;
- existing symbol-shaped executable tests remain eligible;
- endpoint-security facts remain the preferred server-authentication proof.

For a natural production-and-test file plan, exact inventory uses a dedicated bounded candidate cap no larger than the public 12-file limit. It reserves category diversity first, then fills coherent siblings within the already requested projects:

- configuration owner plus production and test-profile resources when available;
- up to two distinct persistence paths for distinct requested domain variants;
- internal management contract and server authentication;
- executable controller/service or retry-pattern tests.

The selector still derives every candidate from indexed facts and normalized query/project identity. It contains no private repository, class, path, or benchmark phrase. Ordinary non-inventory queries retain the existing planning bound and behavior.

## Safety and error behavior

- Configuration values remain redacted; only owner symbols and already safe resource metadata are added.
- Missing siblings remain explicit omissions or uncertainties; the selector never invents a future file.
- Entrypoint and current-path evidence remain mandatory.
- Selection remains deterministic and bounded.
- No prompt, CLI, public schema, dependency, retry, fallback, or release gate changes.

## Testing

TDD starts with a release-shaped generic fixture that mirrors runtime scanner facts: symbol-shaped config owner, endpoint-security server role, symbol-shaped test classes, two repository paths, and production/test-profile resources. Before the fix it must demonstrate the same incomplete bundle seen in Matrix 4.

After the minimal correction, the final pack must represent the config owner, client source, provider security/controller, both repositories, production and test-profile resources, and relevant executable tests while staying within all existing limits and redacting sentinel values. Focused tests cover matching and bounded deterministic sibling reservation. The complete agent, scanner, shell benchmark, docs-sync, vet, and repository test gates remain required before installation or external execution.

