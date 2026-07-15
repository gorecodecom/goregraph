# Architecture and Exact Code Explorer Design

**Status:** Approved
**Date:** 2026-07-16
**Release target:** Unreleased `1.3.0`
**Issues:** GitHub #23 and #25

## Purpose

GoreGraph 1.3.0 will make the workspace dashboard useful for two related jobs:

1. understanding dense service architecture without losing workspace context;
2. selecting an exact Java or JavaScript/TypeScript symbol and seeing verified direct usages separately from HTTP paths that reach it.

The implementation must remain offline, deterministic, additive to Schema 2, evidence-backed, and honest about incomplete static analysis. It must not execute scanned source code or infer exact usages from names alone.

## Scope

### Architecture map

- Preserve the full architecture and stable service-card positions.
- Render dynamic domain lanes from service-map metadata rather than hardcoded workspace-specific labels or palettes.
- Bundle background relationships into readable domain/service trunks and fan direct relationships out near their cards.
- Highlight all direct incoming and outgoing relationships for the selected service simultaneously.
- Dim unrelated lanes, cards, and edges instead of hiding or re-laying them out.
- Add a sticky relationship summary below the graph controls.
- Support service, domain, direction, and risk focus without changing the underlying layout.
- Keep the inspector as the detailed relationship surface.

### Exact Code Explorer

- Treat Java and JavaScript/TypeScript declarations as selectable canonical workspace symbols.
- Show exact direct code references across indexed projects.
- Show HTTP reachability as a separate relationship category.
- Preserve ambiguous and unresolved candidates instead of selecting by simple name.
- Provide the same operations through the dashboard, Query, Explain, MCP, and generated JSON.
- Validate all canonical references with Doctor.

### Documentation and release target

- Keep the source target at unreleased `1.3.0`.
- Update README usage, generated-output documentation, command reference, release notes, and the README integration-depth table.
- Do not create a tag, GitHub Release, or package-manager publication.

## Non-goals

- Runtime traffic or invocation frequency.
- Bytecode decompilation of arbitrary external dependencies.
- Exact links based only on a simple or qualified name.
- A free-form IDE-scale class graph as the primary interface.
- Executing source, build tools, dependency installers, or scanned applications.
- Full static resolution for every supported language in this release. Java and JavaScript/TypeScript are the exact-symbol targets; other languages remain explicitly partial or unavailable for this capability.

## Architecture

The selected approach is a canonical workspace projection. Existing project outputs remain the source facts. Project scans add provenance-rich symbol and usage fields, and workspace reconciliation converts those facts into two deterministic outputs:

- `.goregraph-workspace/symbol-index.json`
- `.goregraph-workspace/symbol-usages.json`

Dashboard, Query, Explain, MCP, and Doctor consume these projections rather than independently re-resolving symbols. This keeps terminology, ambiguity handling, coverage, and results consistent across interfaces.

The architecture map continues to consume `workspace-service-map.json`. Its new lane, summary, and focus behavior is a presentation layer over the existing stable service and edge identities.

## Canonical Symbol Model

A canonical symbol record contains at least:

- stable symbol ID;
- project and project kind;
- module, package, application, or workspace package when available;
- build artifact identity when available;
- language and symbol kind;
- simple name and qualified/export name;
- declaration file and one-based line;
- evidence IDs;
- analyzer and confidence;
- coverage status and limitations.

Stable identity inputs are:

```text
symbol kind + project + module/artifact/package + language
+ qualified/export name + declaration file
```

A declaration file or name alone is insufficient. IDs must be stable across repeated scans and independent of workspace scan order.

## Java Resolution

Java canonical symbols cover classes, interfaces, enums, and records. Nested declarations retain their owning declaration in their qualified identity when deterministically extractable.

Exact direct usage may be produced from:

- exact imports and explicit fully qualified names;
- field, constructor, parameter, return, generic, array, and annotation types;
- `extends` and `implements`;
- instantiation;
- static imports;
- method calls with a resolved canonical owning class.

Resolution uses package/FQN evidence, declaration files, source module, and Maven/Gradle artifact dependencies. Duplicate qualified names require unique project/module/artifact evidence. Otherwise the reference is `AMBIGUOUS` and excluded from default exact results.

## JavaScript and TypeScript Resolution

JavaScript/TypeScript canonical symbols cover:

- classes;
- interfaces, type aliases, and enums;
- exported functions and arrow functions;
- exported React components;
- other exported declarations when the current extractor can identify their source range and module identity reliably.

Exact direct usage may be produced from:

- relative and absolute module imports;
- named, default, namespace, type-only, and static dynamic imports when the target is statically known;
- re-exports;
- workspace package exports and package dependencies;
- TypeScript path aliases and base paths;
- explicit type references;
- JSX component references;
- calls with a resolved imported or local owner.

Resolution is based on module path, export name, project/package provenance, and dependency evidence. A matching identifier without module/export evidence is not Exact. Unsupported runtime aliasing, computed imports, dynamic property access, or bundler-only resolution remains unresolved with a coverage explanation.

## Usage Model

Workspace symbol usages use stable IDs and one of these categories:

- `direct_reference`
- `reached_through_api`
- `ambiguous`
- `unresolved`

Every usage includes:

- provider symbol ID;
- consumer project and consumer symbol ID when known;
- language, relation kind, source file, and one-based line;
- confidence, resolution, reason, analyzer, and evidence IDs;
- candidate symbol IDs for ambiguous references;
- dependency or artifact evidence when used for disambiguation.

Direct references and API reachability are never merged into one count or row.

## HTTP Reachability

HTTP reachability reuses existing contracts, workspace matches, endpoint traces, feature flows, and Java implementation steps.

A `reached_through_api` path contains the complete supported chain:

```text
JS/TS route/component/caller
→ API helper and HTTP contract
→ resolved workspace contract
→ Spring route and handler
→ Java implementation step
→ selected canonical Java symbol
```

Selecting a Java symbol shows incoming API consumers whose proven implementation trace reaches that symbol. Selecting a JavaScript/TypeScript symbol shows outbound API paths originating from that symbol when existing route/component/caller evidence supports the association.

HTTP is the implemented transport for 1.3.0. The model retains an explicit transport field so gRPC and messaging can be added later without changing the category contract.

## Workspace Reconciliation

Workspace reconciliation:

1. loads symbol, relation, callgraph, Maven, package, contract, trace, and evidence outputs from every indexed project;
2. namespaces source identities with project and module/artifact provenance;
3. builds language-specific declaration and module indexes;
4. resolves only uniquely evidenced direct references;
5. retains ambiguous and unresolved candidates;
6. derives HTTP reachability from existing canonical workspace flow evidence;
7. sorts all records deterministically;
8. records indexed, missing, partial, unsupported, and failed coverage.

The reconciler rejects duplicate canonical IDs and never depends on project discovery or scan order for output ordering or resolution.

## Architecture Map Experience

### Dynamic domain lanes

Each distinct service-map domain becomes a subtle rounded lane behind its cards. Domain labels come from metadata. Colors come from a deterministic neutral palette keyed by domain identity; service cards remain white and retain existing risk/status semantics.

The domain header and its matching filter chip activate the same state and expose `aria-pressed`. Clicking the active domain again or selecting `All` resets domain focus.

Domain focus defaults to `Outgoing` and supports `Incoming` and `Both`. It keeps every service in the domain, all direct external neighbors in the chosen direction, and matching edges fully visible. Everything else remains in place and is dimmed.

### Selected-service focus

Selecting a service keeps its card at the same coordinates, shows all direct incoming and outgoing edges simultaneously, and dims unrelated context. The viewport may fit the complete direct neighborhood, but node coordinates do not change.

Parallel background relations use bundled trunks. Direct selected relations fan out to separate card ports close to their source and target. Arrowheads communicate direction. There are no redundant Caller/Called labels or opaque OUT badge.

### Relationship badges and summary

Compact `N calls` badges remain outside service-card content. Their tooltip explains that the value is the number of statically detected relationships, not runtime request frequency. Selecting a badge opens its underlying relationship details without moving the graph.

A sticky summary below the graph controls shows:

- selected service;
- incoming relationship and caller-service counts;
- outgoing relationship and target-service counts;
- resolved, unresolved, and mismatch totals;
- `Incoming`, `Outgoing`, and `Risk` toggles;
- reset action.

The summary filters the map but does not replace the inspector.

## Code Explorer Experience

The selected service inspector adds `Explore classes & symbols`. It opens a dedicated semantic HTML workbench and stores the current architecture selection, active domain context, zoom, and pan. `Back to Architecture` restores them exactly.

### Inventory

The inventory is grouped by package/module and supports search by name, qualified/export name, package, module, and file. Each row shows:

- symbol kind and language;
- qualified/export name;
- declaration file and line;
- source action;
- direct-reference count;
- API-reachability count;
- confidence and coverage state.

### Selected symbol

The usage view provides:

- `Direct references`;
- `Reached through API`;
- `All`;
- `Ambiguous / unresolved` when evidence exists.

Filters cover consumer service/project, category, relation kind, language, and confidence. Each row shows the consumer, consuming symbol/method when known, source file and line, reason, confidence, evidence action, and API summary when applicable.

Selecting a usage opens details for provider and consumer identities, resolution and dependency evidence, API steps, coverage limitations, and safe source/editor actions.

Same-name exclusions are summarized without implying a usage, for example: `3 unrelated symbols share the name UserService and were excluded.`

## Query, Explain, and MCP

The external interfaces expose task-oriented operations for:

- listing symbols declared by a service/project;
- finding exact usages by stable symbol ID;
- finding HTTP consumers that reach a stable symbol ID;
- explaining exact, ambiguous, unresolved, or API-derived classifications;
- resolving human input to candidate symbol IDs when it is not unique.

Query, Explain, MCP, dashboard labels, and generated JSON use the same category and resolution names. Results retain existing continuation/limit behavior where task APIs already provide it.

## Doctor and Integrity

Doctor validates:

- unique canonical symbol and usage IDs;
- every usage provider and known consumer symbol reference;
- candidate IDs on ambiguous usages;
- evidence references;
- project and source references;
- ordered API trace steps and selected implementation symbol;
- Schema 2 and legacy-reader compatibility.

An invalid projection is a Doctor failure with the offending ID and remediation.

## Coverage and Error Handling

Missing results are never presented as proof that a symbol is unused. Coverage explicitly identifies:

- unindexed workspace projects;
- partial or unsupported language capabilities;
- known dependency artifacts without indexed declarations;
- dynamic imports, reflection, dependency injection, proxies, generated code, or runtime loading;
- ambiguous duplicate declarations;
- analyzer or output-read failures.

A single unreadable project remains a structured coverage failure while other indexed projects continue reconciling. Global output-write or integrity failures fail the refresh/scan command rather than silently emitting partial files.

## Accessibility and Responsive Behavior

- Keyboard activation works for service cards, domain headers/chips, summary toggles, relationship badges, inventory rows, tabs, filters, usage rows, and back/reset actions.
- Selected and pressed state uses labels, borders, and ARIA; color is never the only signal.
- Tooltips are available to keyboard focus and pointer hover.
- Focus is visible and logical after view transitions.
- Reduced-motion preference is respected.
- Architecture and Code Explorer are verified at 1280×720, 1440×900, and 1920×1080.
- Large inventories use semantic HTML at normal browser scale, not a compressed SVG.

## Testing Strategy

Every production behavior follows RED, GREEN, and REFACTOR. Required automated coverage includes:

- Java package, nested declaration, class/interface/enum/record, generic/array, import, type, inheritance, annotation, instantiation, and resolved-owner cases;
- Java duplicate simple names and duplicate FQNs separated by artifact provenance;
- JavaScript/TypeScript classes, interfaces, types, enums, functions, components, imports, re-exports, workspace packages, path aliases, JSX, and resolved calls;
- unrelated same-name JS/TS and Java symbols excluded from Exact;
- direct references structurally separate from HTTP reachability;
- full JS/TS-to-Spring-to-Java HTTP chain;
- deterministic repeated output and scan-order independence;
- Query, Explain, MCP, and dashboard parity;
- Doctor duplicate/dangling/evidence/API-chain failures;
- architecture service focus, domain focus, direction/risk filters, summary, badge details, tooltip semantics, reset, and viewport preservation;
- Code Explorer inventory, search, tabs, filters, details, coverage, source actions, back navigation, keyboard, focus, and reduced motion;
- visual verification with a dense Weka-like fixture at all supported viewports.

## Documentation

Update:

- `README.md`, including Quick Start, Code Explorer usage, architecture focus behavior, and the feature/integration-depth table;
- `COMMANDS.md` for new Query/Explain/MCP-facing operations;
- `docs/OUTPUTS.md` for symbol projections and additive project fields;
- `docs/RELEASE.md` for unreleased 1.3.0 scope and pending publication;
- any CLI help or generated-output tests that protect the public contract.

## Acceptance and Delivery

Acceptance requires:

1. all issue #23 and #25 criteria covered by code or tests;
2. focused and full Go tests, format, vet, JavaScript syntax, dashboard interaction, accessibility, and visual checks passing;
3. an independently reviewed full diff with no unresolved Critical or Important finding;
4. current source installed locally as `goregraph 1.3.0`;
5. `main` pushed and verified against `origin/main`;
6. issues #23 and #25 closed only after remote verification;
7. `~/projects/weka/` clean-plan reviewed, generated GoreGraph outputs removed with `workspace clean --execute`, and every discovered project rescanned with `workspace scan-all`;
8. the fresh Weka outputs pass Doctor/integrity checks and contain the new symbol projections and updated offline dashboard;
9. no tag, release, Homebrew, Scoop, or Winget publication.
