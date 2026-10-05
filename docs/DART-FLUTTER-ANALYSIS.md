# Dart, Flutter and authored project languages

GoreGraph indexes static source evidence. It does not start the Dart VM, Flutter engine, package solver, compiler, browser, shell, Unity or Blender during a scan or a context query. A declaration binding describes the source target; it does not prove runtime dispatch or that a test passed.

## Five implementation and verification cycles

1. **Source foundation:** Dart declarations, constructors (including Dart 3.13 primary constructors), fields, accessors, enums, mixins and extensions; nested comments, raw/triple strings and interpolation are opaque to executable-source discovery. Pub manifests identify packages, SDK constraints, dependency sources, workspace members and declared Flutter assets/fonts. `.dart_tool` and `.pub-cache` are excluded by default.
2. **Static bindings:** relative/package imports, prefixes, show/hide, export barrels, reciprocal named/URI parts and library privacy; typed fields, parameters and local constructors; conservative inheritance/mixin lookup and named/default parameter matching. Invalid parts, unavailable imports, ambiguous declarations and runtime assignments do not receive guessed exact calls. Closures retain separate source identities.
3. **Flutter evidence:** import-verified widget creation, source callback references, absolute literal go_router routes, named navigation references, Flutter state notifications and widget/unit test declarations with static target mapping. A tear-off reference is not an execution edge; test discovery does not run a test.
4. **Application boundaries:** literal `http` and Dio requests, locally constructed `Request`/`AbortableRequest` sent through typed clients, local Uri values, sqflite/preferences APIs, typed Riverpod and stream/channel patterns. Unknown base URLs remain unknown. Query values, URL credentials and opaque resource payloads are not exported as API/resource evidence.
5. **Consumer integration:** facts enter symbols, call graphs, test maps, architecture capabilities, the workspace HTTP matcher, the dashboard and the normal agent context index. Current-source renderers reuse the same parsers, honor bounded merged source ranges, and issue ordinary source-hash read receipts. Tests exercise Dart-to-Go workspace matching and generated context packs, not only parser output.

Each cycle includes positive evidence and negative cases for strings/comments, shadowing, ambiguity, unsupported imports or runtime behavior. Existing Java/Go/JS/TS, C#/Swift and asset paths remain in the full regression suite.

## Authored-source inventory used for prioritization

The inspected `/Users/gorecode/projects/gorecode` repositories contain these language families. Generated builds, installed SDKs, caches and vendored dependency trees were excluded from the authored-source comparison.

| Projects | Relevant authored languages/formats |
| --- | --- |
| GoreTransit | Dart/Flutter, Go, Kotlin/Gradle Kotlin, Swift, HTML, Windows launcher |
| Talkloom | Swift, Rust, Python, C/C++, Objective-C bridge, shell |
| crownandrunes | C#, Python, Unity, Blender exports/assets, Unity USS |
| GitHousekeeper | Go, JS/TS/React, HTML/CSS, shell, Windows Batch |
| gorecode.com, stashit, udemy | JS/TS/React, HTML/CSS; PHP in gorecode.com |
| GorePlan | Swift, HTML, shell |
| python_tools | Python, shell, macOS `.command`, Windows Batch |
| homebrew-tap | Ruby |

Existing supported executable languages keep their adapters. Additional native support adds:

- **Kotlin:** packages/imports and Gradle module boundaries, classes/properties/functions, typed calls with named/default parameters, test mapping, import-verified literal Spring routes and Ktor requests, typed Flutter MethodChannel evidence.
- **C/C++:** structured declarations, raw string safety, conservative direct calls and relative indexed header visibility. Macros/conditional preprocessing, function pointers, overload conversions, templates and linking remain unproven.
- **Objective-C:** headers/interfaces/properties, full selectors, distinct class/instance messages, and C ABI bridge entrypoints. Only unique local source targets are bound; external SDK and runtime message dispatch stay open.
- **Ruby:** classes/modules/methods and explicit self/class source references; quoted, percent, heredoc, regex and documentation bodies cannot fabricate declarations. Deferred blocks, dynamic receivers, metaprogramming and monkey-patching are not treated as immediate direct calls.
- **HTML/CSS/USS:** literal element IDs, labels/accessibility links, style classes/selectors/custom properties, relative indexed resources, and safe literal HTML form contracts. No rendering, cascade or inline script execution is asserted.
- **Windows Batch:** literal labels, CALL and GOTO references, ambiguity checks and quoted script paths. No script is executed or variable expansion evaluated.

## Coverage and remaining boundaries

`full` describes the breadth of the registered adapter, not compiler-equivalent completeness. A `complete` capability means complete within its supported static analysis scope. Additional adapter profiles retain `partial` depth and explicit limitations. Runtime-only relationships stay unresolved.

Dart does not resolve arbitrary package caches, dependency overrides, conditional/deferred imports, generated files absent from indexed source, full generic/type inference, extension dispatch or the Pub version solver. Workspace resolution requires declared membership, compatible explicitly supported constraints and uniquely indexed source. Unrecognized constraints or overrides do not establish a target.

Flutter navigation covers absolute literal go_router declarations; nested relative routes, redirects and complete dynamic route tables are not reconstructed. Persistence facts describe API usage rather than stored contents. Kotlin aliases, advanced generic/inheritance resolution and generated compiler symbols are outside this adapter's current scope. Ruby/C/C++/Objective-C profiles deliberately expose less breadth than Java or compiler-assisted C#/Swift.

## Refresh and verification

Extractor, resolver and agent revisions change together so an existing user-enabled watcher recognizes old analyses after the new binary is adopted. Dashboard generation alone cannot create new source facts. A stopped watcher cannot refresh an index; diagnose its coverage and output freshness separately. Context/source queries never initiate a refresh or restart.

Validation commands:

```sh
go test -p 2 ./... -count=1
go vet ./...
go test -race ./internal/scan ./internal/agent -run 'Test(Dart|Supplementary|CFamily|Kotlin|Ruby|ObjectiveC|CSS|StructuredLanguages)' -count=1
go run ./scripts/sync-docs --check
```

Integration tests create temporary fixtures and outputs. They do not modify or execute the inspected applications. Optional native watcher lifecycle tests also use temporary roots, independent of productive watcher/autostart state. Local install and Git push are separate from release publication.
