# C# and Swift static analysis

The adapters extend Schema 3 full symbols, relations, callgraph, test-map,
architecture-capability evidence and agent context. Java/Spring and the script
adapters retain their existing implementations. Scans, watchers and MCP queries
never start a compiler, an IDE, Unity, Blender or project code. No SDK installation
is required to use these adapters.

## C# / .NET / Unity

The lexer separates comments, character literals, ordinary/verbatim/raw strings
and interpolated strings. The structural parser records namespaces, imports and
aliases, classes/interfaces/records/structs/enums, nested types, constructors,
methods, fields, properties and parameter signatures.

Supported static call binding uses typed fields, parameters and simple locals,
declared assembly visibility, inherited source members, argument counts, named
arguments, optional defaults and `ref`/`out`/`in` modes. Known primitive arguments
can distinguish overloads; supported widening conversions retain the more specific
candidate. Unknown conversions, multiple viable overloads, dynamic receivers and
conditional providers remain unresolved or ambiguous. A bound edge identifies a
static declaration; virtual dispatch and actual execution remain unproven.

Additional source evidence includes:

- Literal ASP.NET controller HTTP attributes and typed `WebApplication` minimal
  API registrations. Dynamic routes and unresolved group prefixes are not invented.
- Typed `HttpClient` requests with literal paths; runtime base addresses,
  authentication, middleware and responses are not inferred.
- Parameterless generic `AddScoped`, `AddSingleton` and `AddTransient` registrations
  on a known `IServiceCollection`/`ServiceCollection`, including literal
  `WebApplication.CreateBuilder(...).Services` patterns. Registrations link the
  declared service and implementation types separately from executable call edges.
  Factories, container execution and lifetime correctness are not inferred.
- Typed EF Core `DbContext`/`DbSet` source operations and `DbSet<T>` entity links.
  The analyzer does not translate LINQ, query a database or prove a commit.
- NUnit, xUnit and Unity test declarations plus supported direct target calls.
  Neither discovery by a runner nor successful execution is inferred.
- Literal `.csproj`, `.asmdef` and `.asmref` dependencies. MSBuild conditions,
  generated sources, package assemblies and runtime loading are not evaluated.

Local types shadowing framework names do not become framework evidence merely
because they use names such as `HttpClient` or `DbContext`. Complex receiver chains,
extension methods, generic constraints, delegates/local functions, reflection and
compiler-generated members are outside the supported binding rules. This adapter
does not claim Roslyn-level semantic coverage by itself. The optional compiler
snapshot below can supply verified bindings for these source cases.

## Swift / SwiftUI / Apple frameworks

The lexer supports nested comments, escaped identifiers, ordinary/raw/multiline
strings and interpolation boundaries. The structural parser records classes,
structs, enums/cases, actors, protocols, extensions, type aliases, associated types,
properties, functions, methods, initializers and deinitializers.

Static method binding distinguishes external argument labels, supported known
literal types and omitted default parameters. Source inheritance and `super`
calls are included. Literal SwiftPM targets, dependencies, custom source paths,
source selections and exclusions establish supported source membership. Xcode
source build phases, local target dependencies and synchronized root groups with
membership exceptions are also read directly. Shared, conditional, computed or
unsupported membership remains unassigned; imports do not cross unrelated targets.
Without target metadata, `Sources/<Target>` and `Tests/<Target>` retain separate
module identities. Package manifests and Xcode build settings are never executed;
this is not proof of the effective build configuration.

Additional source evidence includes:

- SwiftUI property-wrapper references, including projected bindings and supported
  `@Query` model links. Local/closure shadows are excluded. Observation delivery,
  view refresh, closure activation and actor scheduling are not execution proof.
- Literal Foundation `URLSession` URLs and tracked local `URLRequest` HTTP methods.
  Dynamic/interpolated URLs and modified unknown requests are excluded. No request
  is sent; completion, authentication and response behavior remain unknown.
- Typed SwiftData `ModelContext`, Core Data `NSManagedObjectContext` and Foundation
  `UserDefaults` operations. This identifies source operations, not durable writes,
  transaction success or callback execution.
- XCTest `test*` methods on a declared `XCTestCase` and Swift Testing `@Test`
  declarations, with supported static target links. No passing result is inferred.

Conditional compilation, generic/conditional-extension constraints, macros,
protocol dispatch, complex receiver expressions and variadic binding are not
evaluated. Ambiguous overloads stay open. No SourceKit, SwiftSyntax dependency or
Swift compiler process is launched by normal GoreGraph work. This adapter does not
provide server-route, messaging or end-to-end data-flow analysis.

## Optional compiler snapshots

These snapshots add compiler symbol identities for selected configurations while
keeping normal scanning and query operations independent of SDKs. The user must
explicitly generate them; neither the watcher nor MCP runs exporters. Existing
static framework facts remain available. Only calls in `covered_files` are replaced
by the verified compiler references, and corresponding declaration-based test links
are rebuilt. A method value or a saved callback never becomes a call merely because
its target is known.

Write a standalone bundle and inspect a read-only input inventory:

```sh
goregraph languages exporter csharp --output /tmp/goregraph-roslyn
goregraph languages inputs csharp /path/to/project > /tmp/csharp-request.json
goregraph languages exporter swift --output /tmp/goregraph-sourcekit
goregraph languages inputs swift /path/to/project > /tmp/swift-request.json
```

The generated request is an inventory skeleton, not an evaluated build graph.
Split `modules` into the actual selected compilations before running an exporter.
Every source/configuration input must remain listed in `inputs`. If a request is
stored inside the indexed root, use a `*.goregraph-language-input.json` filename
and include it in `inputs` so changes to its flags invalidate the snapshot.

For C#, modules accept `name`, `files`, explicit assembly `references`, local module
`dependencies` and preprocessor `defines`. The helper uses SDK-bundled Roslyn and
in-memory compilations. It does not evaluate user MSBuild files, emit user binaries,
execute code, or run user analyzers/generators. Compilation errors reject the export.
Generics, extension-method reductions, overloads, local functions, record properties,
primary constructors and supported user operators can contribute compiler identities.
Without explicit references, only the helper runtime's platform assemblies are used;
Unity and framework dependencies require the correct explicitly selected assemblies.

```sh
dotnet build /tmp/goregraph-roslyn/Exporter.csproj
dotnet /tmp/goregraph-roslyn/bin/Debug/net10.0/Exporter.dll \
  /tmp/csharp-request.json /path/to/project/Analysis.goregraph-csharp.json
```

The bundle targets an installed .NET 10 SDK and has no NuGet package sources. An
installed compatible SDK can explicitly override `TargetFramework`; the selected
SDK must provide the matching framework reference pack and Roslyn APIs.

For Swift, provide `sourcekit_library` with the compatible in-process SourceKit
library and each module's explicit `compiler_args`, including SDK/target, module
search paths, compilation flags and a temporary module-cache path. The exporter
uses SourceKit diagnostics and `indexsource` in process; it launches neither Xcode,
SwiftPM nor SourceKit-LSP. Compiler errors reject the report. Explicit plugin-loading
and response-file arguments are rejected. SDK/compiler components may be loaded by
the explicitly invoked exporter; this is not an automatic analysis action.
The default exporter rejects macro-like syntax and arbitrary attributes before
loading SourceKit. This conservative check includes comments and strings, since
interpolations can contain macros. A manually authored request can explicitly set
`allow_macro_expansion: true` when compiler macro execution is intended. Imported
SDK/module macro implementations can then execute during type checking; this is
a separate manual compiler operation. Normal GoreGraph scans, watchers and queries
never expand macros. Static analysis remains available without this opt-in.

```sh
python3 /tmp/goregraph-sourcekit/sourcekit.py \
  /tmp/swift-request.json /path/to/project/Analysis.goregraph-swift.json
```

For a current macOS Xcode toolchain, the library is under
`usr/lib/sourcekitdInProc.framework/Versions/A/sourcekitdInProc`; use the matching
SDK and compiler arguments. Generic/protocol-extension bindings, compiler
constructors and supported trailing-closure calls can then supply exact identities.
Protocol dispatch, actor scheduling and closure execution remain runtime questions.

Both exporters require a new report filename. Reports contain compiler identities,
source coordinates and SHA-256 hashes, not proof of successful application execution.
The scanner validates every required indexed input and source location. Missing,
stale, duplicate or competing snapshots produce diagnostics and retain static
analysis. Queries recheck reports, source/configuration hashes and newly added
inputs before delivering compiler bindings; stale snapshots require an explicit
new export. Tooling audits verify their selected audit sources separately and do
not widen their source scope to unrelated compiler inputs.

Only indexed owned sources are binding targets. External SDK/assembly freshness,
unlisted environment/build inputs, generators, alternate configurations and the
authenticity of a third-party report are not independently verified. Exported
bindings therefore do not upgrade language coverage to exhaustive semantic or
runtime proof.

## Verification and activation

Regression fixtures verify positive bindings alongside overload ambiguity,
framework shadows, local/closure scopes, conditional providers, mutated HTTP
requests, metadata boundaries and repeated same-line calls. Agent-context fixtures
verify source-backed C#/Swift entrypoints, bounded windows and read receipts.
These are also covered by the normal Go test suite.

Compiler fixture checks are optional and explicitly enabled by a developer:

```sh
GOREGRAPH_SWIFTC_SMOKE=/path/to/swiftc \
GOREGRAPH_SWIFT_SDK_SMOKE=/path/to/MacOSX.sdk \
GOREGRAPH_DOTNET_SMOKE=/path/to/dotnet \
go test ./internal/scan -run '^TestRealLanguageCompilerFixtures$' -count=1 -v
```

They compile and execute only disposable language fixtures, with no NuGet sources.
They never build a user repository or launch an application server. SDK fixtures
provide independent checks for representative binding cases, not a claim that all
language/framework semantics are implemented.

The bundled exporters also have optional real-SDK regression tests:

```sh
GOREGRAPH_DOTNET_SMOKE=/path/to/dotnet \
GOREGRAPH_SOURCEKIT_SMOKE=/path/to/sourcekitdInProc \
GOREGRAPH_SWIFT_SDK_SMOKE=/path/to/MacOSX.sdk \
go test ./internal/languageexport -run '^TestReal' -count=1 -v
```

These cover generic/extension/operator/record bindings, trailing closures versus
method values, rejected compiler errors and refusal to overwrite reports. Snapshot
consumer tests separately cover stale/added inputs and overload identity boundaries.

The extractor/agent revisions invalidate old generations independently of the
release version. An existing watcher process retains its original executable
until the user explicitly restarts it. After installing the new binary, restart
the user-enabled watcher to regenerate its project/workspace indexes, and restart
long-running MCP processes. Rebuilding only dashboard HTML cannot introduce new
source facts. Agents do not perform these maintenance actions automatically.
