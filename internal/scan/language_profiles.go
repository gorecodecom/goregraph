package scan

import "slices"

// LanguageCapabilityProfile describes the implemented static-analysis depth of
// one source-language adapter.
type LanguageCapabilityProfile struct {
	Language        string
	DisplayName     string
	Level           string
	Scope           string
	Symbols         bool
	Relations       bool
	Calls           bool
	Routes          bool
	Tests           bool
	APIClients      bool
	Persistence     bool
	Messaging       bool
	DataFlow        bool
	ExactSymbols    bool
	DirectUsages    bool
	HTTPProvider    bool
	HTTPConsumer    bool
	PatternFamilies []string
	Limitations     string
	Outputs         []string
}

var sourceLanguageCapabilityProfiles = []LanguageCapabilityProfile{
	{
		Language: "batch", DisplayName: "Windows Batch", Level: "partial", Scope: "language+cmd",
		Symbols: true, Relations: true, Calls: true, ExactSymbols: true, DirectUsages: true,
		PatternFamilies: []string{"literal labels and CALL/GOTO targets"},
		Limitations:     "Variable expansion, command execution and computed targets are not evaluated; duplicate labels remain ambiguous.",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "callgraph.json"},
	},
	{Language: "blender", DisplayName: "Blender asset exports", Level: "partial", Scope: "asset-export", Symbols: true, Relations: true, Limitations: "Explicit source/dependency-hash-verified Blender exports, saved datablock links and bounded optional sampled surfaces only; Blender is never executed by a scan or context query. External libraries, unsampled frames and collision correctness are not proven.", Outputs: []string{"assets.json", "symbols-full.json", "relations-full.json", "graph-full.json"}},
	{
		Language: "c", DisplayName: "C", Level: "partial", Scope: "language",
		Symbols: true, Relations: true, Calls: true, ExactSymbols: true, DirectUsages: true,
		Limitations: "Structured C/C++ declarations and uniquely visible direct calls. Preprocessor branches, macros, function pointers, implicit conversions, templates and linker configuration remain unresolved.",
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "cpp", DisplayName: "C++", Level: "partial", Scope: "language",
		Symbols: true, Relations: true, Calls: true, Tests: true, ExactSymbols: true, DirectUsages: true,
		Limitations: "Structured C/C++ declarations, qualified namespaces, scoped typed receivers, transitive indexed headers and GoogleTest test calls. Preprocessor branches, macros, function pointers, implicit conversions, templates and linker configuration remain unresolved.",
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json", "callgraph.json", "test-map.json"},
	},
	{
		Language: "csharp", DisplayName: "C# / .NET / Unity", Level: "full", Scope: "language+aspnet+efcore+unity",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true, APIClients: true, Persistence: true,
		ExactSymbols: true, DirectUsages: true, HTTPProvider: true, HTTPConsumer: true,
		PatternFamilies: []string{"C# type and member declarations", "typed static calls with inheritance, named, optional and ref arguments", "optional hash-verified Roslyn symbol and call snapshots", "typed EF Core operations", "literal DI registrations", "literal ASP.NET controller routes", "literal HttpClient requests", "NUnit, xUnit and Unity test declarations"},
		Limitations:     "Static source patterns by default; explicitly generated Roslyn snapshots cover only their selected source/configuration inputs. No compiler or MSBuild is run by scans or queries. Runtime dispatch, reflection, SDK freshness and test execution remain unproven; unsupported bindings stay open.",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json", "api-contracts.json", "test-map.json", "graph-full.json"},
	},
	{
		Language: "css", DisplayName: "CSS / Unity USS", Level: "partial", Scope: "styles+assets",
		Symbols: true, Relations: true, ExactSymbols: true, DirectUsages: true,
		PatternFamilies: []string{"selectors, custom properties, imports and resource URLs"},
		Limitations:     "Literal style declarations and references only; browser rendering, selector applicability, cascading, inheritance and Unity visual state are not evaluated.",
		Outputs:         []string{"symbols-full.json", "relations-full.json"},
	},
	{
		Language: "dart", DisplayName: "Dart / Flutter", Level: "full", Scope: "language+pub+flutter",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true, APIClients: true, Persistence: true, Messaging: true, HTTPConsumer: true,
		ExactSymbols: true, DirectUsages: true,
		PatternFamilies: []string{"typed calls / library privacy / parts / export barrels", "Flutter widgets / callbacks / navigation / widget tests", "http / Dio / sqflite / preferences / Riverpod", "library declarations", "classes / mixins / extensions", "constructors / methods / getters", "pub package metadata"},
		Limitations:     "Static source analysis does not execute the Dart VM, Flutter engine, build_runner, or package resolution. Runtime dispatch and conditional or unavailable library bindings remain unresolved.",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "workspace.json", "callgraph.json", "routes.json", "api-contracts.json", "test-map.json", "context-index.json"},
	},
	{
		Language: "gdscript", DisplayName: "GDScript", Level: "partial",
		Scope:   "language+godot",
		Symbols: true, Relations: true, Calls: true, Tests: true, ExactSymbols: true, DirectUsages: true,
		PatternFamilies: []string{"class and member declarations; literal preload/load paths", "local and explicitly constructed script calls; signal callbacks; test-source links"},
		Limitations:     "Static source analysis only; dynamic dispatch, engine APIs, nested classes, escaped or computed paths and runtime signal execution remain unresolved",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "callgraph.json", "test-map.json"},
	},
	{
		Language: "go", DisplayName: "Go", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"net/http and common routers", "net/http clients", "database/sql and GORM",
			"Kafka and AMQP", "gRPC", "JSON request/response boundaries", "go test and httptest",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols.json", "relations.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "godot", DisplayName: "Godot resources", Level: "partial",
		Scope:   "scenes+resources+project",
		Symbols: true, Relations: true, ExactSymbols: true, DirectUsages: true,
		PatternFamilies: []string{"scene nodes; external and embedded resources; attached scripts", "serialized signal callbacks; project main scene and autoloads"},
		Limitations:     "Text formats only; binary resources, inherited scene expansion, UID resolution and engine execution are not analyzed",
		Outputs:         []string{"symbols-full.json", "relations-full.json"},
	},
	{
		Language: "html", DisplayName: "HTML", Level: "partial", Scope: "document+resources+forms",
		Symbols: true, Relations: true, ExactSymbols: true, DirectUsages: true, APIClients: true, HTTPConsumer: true,
		PatternFamilies: []string{"elements, labels, accessibility IDs, resources and form actions"},
		Limitations:     "Literal document and form evidence only; embedded scripts, templates, DOM mutations, browser validation and submission are not executed.",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "api-contracts.json"},
	},
	{
		Language: "java", DisplayName: "Java / Spring", Level: "full", Scope: "language+spring",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		ExactSymbols: true, DirectUsages: true, HTTPProvider: true,
		PatternFamilies: []string{
			"Spring MVC and WebFlux", "Java and Spring HTTP clients", "Spring Data",
			"Spring Messaging", "gRPC", "Jakarta Validation", "JUnit and Spring Test",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "spring.json", "callgraph.json",
			"endpoint-flows.json", "test-map.json",
		},
	},
	{
		Language: "javascript", DisplayName: "JavaScript / Node.js / React", Level: "full", Scope: "language+routes+api",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		ExactSymbols: true, DirectUsages: true, HTTPProvider: true, HTTPConsumer: true,
		PatternFamilies: []string{
			"Express and Fastify", "NestJS", "Next.js", "Web and Node HTTP clients",
			"common Node persistence", "Kafka and AMQP", "gRPC",
			"Node request/response boundaries", "Jest, Vitest, Node Test, and React Testing Library",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "api-contracts.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "kotlin", DisplayName: "Kotlin", Level: "partial", Scope: "language+gradle+spring+ktor+flutter-platform",
		Symbols: true, Relations: true, Calls: true, Tests: true, Routes: true, APIClients: true, Messaging: true, HTTPProvider: true, HTTPConsumer: true, ExactSymbols: true, DirectUsages: true,
		PatternFamilies: []string{"typed visible Kotlin calls with named/default parameters", "JUnit and kotlin.test", "literal Spring mappings, Ktor requests and Flutter MethodChannel"},
		Limitations:     "Structured declarations and conservative direct call bindings. Dynamic dispatch, metaprogramming, unavailable imports and runtime callbacks remain unresolved.",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json", "api-contracts.json", "test-map.json"},
	},
	{
		Language: "objectivec", DisplayName: "Objective-C / Objective-C++", Level: "partial", Scope: "language+objc+c-abi",
		Symbols: true, Relations: true, Calls: true, Tests: true, ExactSymbols: true, DirectUsages: true,
		PatternFamilies: []string{"interfaces, properties, class and instance selectors, C ABI entrypoints, imported selectors, XCTest"},
		Limitations:     "Imported interfaces, typed properties/parameters/locals, nested messages, super and unique source selectors; XCTest test calls. Runtime dispatch, swizzling, category collisions, SDK methods, arbitrary macros and conditional compilation remain unresolved.",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "callgraph.json", "test-map.json"},
	},
	{
		Language: "php", DisplayName: "PHP", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"Laravel and Symfony routes", "PHP HTTP clients", "Eloquent, Doctrine, and PDO",
			"queues and messaging", "gRPC", "PHP request/response boundaries", "PHPUnit and Pest",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "python", DisplayName: "Python", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"FastAPI, Flask, and Django routes", "requests, httpx, and aiohttp",
			"SQLAlchemy, Django ORM, and DB-API", "Kafka, Celery, and AMQP",
			"gRPC", "Python web and validation boundaries", "pytest and unittest",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "ruby", DisplayName: "Ruby", Level: "partial", Scope: "language",
		Symbols: true, Relations: true, Calls: true, Tests: true, ExactSymbols: true, DirectUsages: true,
		Limitations: "Implicit/explicit self calls, qualified class receivers, singleton/endless methods, literal relative imports, unique mixin/inherited methods and Minitest test calls. Dynamic receivers, reopening conflicts, metaprogramming, unavailable imports and deferred callbacks remain unresolved.",
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json", "callgraph.json", "test-map.json"},
	},
	{
		Language: "rust", DisplayName: "Rust", Level: "full", Scope: "language+routes",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		PatternFamilies: []string{
			"Axum, Actix, and Rocket routes", "reqwest", "SQLx, Diesel, and SeaORM",
			"Kafka and AMQP", "tonic gRPC", "Rust web request/response boundaries",
			"Rust and Tokio tests",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "test-map.json", "graph-full.json",
		},
	},
	{
		Language: "scala", DisplayName: "Scala", Level: "index", Scope: "language",
		Symbols: true, Relations: true,
		Limitations: indexAdapterLimitations,
		Outputs:     []string{"symbols-full.json", "relations-full.json", "graph-full.json"},
	},
	{
		Language: "shell", DisplayName: "Shell", Level: "partial", Scope: "language",
		Symbols: true, Relations: true, Calls: true,
		Limitations: "Function declarations, sourcing, and best-effort calls only; no route, API, persistence, messaging, data-flow, or test adapter.",
		Outputs:     []string{"symbols-full.json", "relations-full.json", "callgraph.json", "flows.json", "graph-full.json"},
	},
	{
		Language: "swift", DisplayName: "Swift / SwiftUI / Apple frameworks", Level: "full", Scope: "language+swiftui+apple",
		Symbols: true, Relations: true, Calls: true, Tests: true, APIClients: true, Persistence: true, ExactSymbols: true, DirectUsages: true, HTTPConsumer: true,
		PatternFamilies: []string{"Swift types, extensions, properties and labeled methods", "typed static calls", "literal SwiftPM and Xcode target membership", "optional hash-verified SourceKit symbol and call snapshots", "SwiftUI property wrappers", "literal URLSession requests", "SwiftData, Core Data and UserDefaults", "XCTest and Swift Testing declarations"},
		Limitations:     "Static source patterns and supported literal target metadata by default; explicitly generated SourceKit snapshots cover only their selected source/configuration inputs. No compiler or Xcode is run by scans or queries. Runtime protocol dispatch, closure activation, actor scheduling, SDK freshness and test execution remain unproven.",
		Outputs:         []string{"symbols-full.json", "relations-full.json", "callgraph.json", "api-contracts.json", "test-map.json", "graph-full.json"},
	},
	{
		Language: "typescript", DisplayName: "TypeScript / Node.js / React", Level: "full", Scope: "language+react+routes+api",
		Symbols: true, Relations: true, Calls: true, Routes: true, Tests: true,
		APIClients: true, Persistence: true, Messaging: true, DataFlow: true,
		ExactSymbols: true, DirectUsages: true, HTTPProvider: true, HTTPConsumer: true,
		PatternFamilies: []string{
			"Express and Fastify", "NestJS", "Next.js", "Web and Node HTTP clients",
			"common Node persistence", "Kafka and AMQP", "gRPC",
			"Node request/response boundaries", "Jest, Vitest, Node Test, and React Testing Library",
		},
		Limitations: fullAdapterLimitations,
		Outputs: []string{
			"symbols-full.json", "relations-full.json", "callgraph.json", "routes.json",
			"flows.json", "api-contracts.json", "test-map.json", "graph-full.json",
		},
	},
	{Language: "unity", DisplayName: "Unity serialized assets", Level: "partial", Scope: "serialized-assets", Symbols: true, Relations: true, Limitations: "Static serialized objects, GUID/fileID, prefab correspondence, assembly, script-field and supported persistent event references only; saved wiring does not prove runtime activation. Binary internals and imported geometry require explicit external evidence.", Outputs: []string{"assets.json", "symbols-full.json", "relations-full.json", "graph-full.json"}},
}

const (
	fullAdapterLimitations  = "Static and pattern-backed; runtime-generated behavior, reflection, metaprogramming, dynamic dispatch, and external configuration may remain unresolved."
	indexAdapterLimitations = "Best-effort declarations and imports only; no normalized call, route, test, or architecture adapter."
)

// LanguageCapabilityProfiles returns a deterministic copy of all public
// source-language capability profiles.
func LanguageCapabilityProfiles() []LanguageCapabilityProfile {
	profiles := make([]LanguageCapabilityProfile, len(sourceLanguageCapabilityProfiles))
	for index, profile := range sourceLanguageCapabilityProfiles {
		profile.PatternFamilies = slices.Clone(profile.PatternFamilies)
		profile.Outputs = slices.Clone(profile.Outputs)
		profiles[index] = profile
	}
	return profiles
}

func languageCapabilityProfile(language string) (LanguageCapabilityProfile, bool) {
	for _, profile := range sourceLanguageCapabilityProfiles {
		if profile.Language == language {
			return profile, true
		}
	}
	return LanguageCapabilityProfile{}, false
}

func (profile LanguageCapabilityProfile) analyzerRecord() AnalyzerRecord {
	return AnalyzerRecord{
		Language: profile.Language, Scope: profile.Scope,
		Symbols: profile.Symbols, Relations: profile.Relations,
		Calls: profile.Calls, Endpoints: profile.Routes, Tests: profile.Tests,
		Outputs: slices.Clone(profile.Outputs),
	}
}
