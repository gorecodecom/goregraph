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
does not claim Roslyn-level semantic coverage.

## Swift / SwiftUI / Apple frameworks

The lexer supports nested comments, escaped identifiers, ordinary/raw/multiline
strings and interpolation boundaries. The structural parser records classes,
structs, enums/cases, actors, protocols, extensions, type aliases, associated types,
properties, functions, methods, initializers and deinitializers.

Static method binding distinguishes external argument labels, supported known
literal types and omitted default parameters. Source inheritance and `super`
calls are included. SwiftPM `Sources/<Target>` and `Tests/<Target>` paths keep
distinct module identities; imported module names can select indexed declarations.
Package manifests are not executed, and this is not compiler-verified target
membership or dependency visibility. Xcode project/workspace markers support
project discovery; build configurations and Xcode source-target membership are
not evaluated. Ordinary Xcode sources use the indexed project's source scope.

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

The extractor/agent revisions invalidate old generations independently of the
release version. An existing watcher process retains its original executable
until the user explicitly restarts it. After installing the new binary, restart
the user-enabled watcher to regenerate its project/workspace indexes, and restart
long-running MCP processes. Rebuilding only dashboard HTML cannot introduce new
source facts. Agents do not perform these maintenance actions automatically.
