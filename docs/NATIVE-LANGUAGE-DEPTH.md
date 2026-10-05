# Objective-C, C++ and Ruby source depth

These adapters record statically verified source declarations and relationships. They do not run Clang, Ruby, a preprocessor, a linker, XCTest, GoogleTest or Minitest. Exact source bindings do not prove runtime dispatch, passing tests or complete application coverage. The language profiles retain `partial` depth.

## Objective-C

- Local classes, interfaces, protocols, properties, complete selectors and C bridge entrypoints retain source identities.
- Literal relative imports follow only indexed headers, including cyclic header chains and canonical include guards.
- Typed method parameters, scoped local declarations, typed properties, class/instance receivers, nested messages with uniquely known return declarations and explicit `super` messages can identify source targets.
- An imported selector declaration can identify a uniquely indexed implementation in another file. Multiple implementations, including colliding categories, remain ambiguous.
- XCTest discovery requires framework import evidence, an XCTestCase inheritance path and a parameterless, void instance method beginning with `test`. Test-to-source mappings describe direct static calls only.
- Unknown `id`/protocol receivers, missing SDK implementations, swizzling, runtime subclass dispatch, arbitrary macros/conditional branches and deferred blocks are not evaluated. C function-pointer shadows do not bind to unrelated C entrypoints.

## C++

- Nested and qualified namespaces, out-of-line declarations, scoped typed parameters/locals and `this` receivers identify visible direct source calls.
- Relative indexed header chains support declarations in one file and definitions in another. Prototype parameter types must match; header cycles terminate.
- Canonical `#ifndef`/`#define` guards and `#pragma once` wrappers are recognized. Comments and raw strings cannot manufacture an include. General preprocessor branches, macro expansion and compiler definitions remain unproven.
- Explicit unknown receivers/qualifiers never fall back to same-named global functions. Local member prototypes hide unrelated global definitions. Anonymous namespaces and static namespace functions retain translation-unit boundaries.
- GoogleTest discovery requires its literal framework header and supported TEST/TEST_F bodies. Disabled-name patterns do not receive active test mappings.
- Overload conversions, templates, virtual dispatch, function pointers, build/linker configuration and deferred lambda execution remain unresolved. Lambda bodies are not assigned to the enclosing immediate caller.

## Ruby

- Nested/qualified classes/modules, explicit and implicit self calls, qualified class receivers, singleton-class methods, predicate/bang names and endless method declarations retain native source evidence.
- Literal unconditional top-level `require_relative` references follow only indexed Ruby files. Dynamic paths, conditional loads and arbitrary gem/load-path resolution do not establish visibility.
- Unique included-module and inherited source methods can identify targets. Reopening conflicts and multiple matching mixins remain ambiguous; cyclic imports and mixin lookup terminate.
- Bare local variables and method parameters are not treated as implicit calls. Reflection, constant reassignment and unsupported method mutation prevent guessed targets.
- Minitest discovery requires an explicit minitest import and a supported Minitest::Test subclass with `test_` methods. Static test calls produce source mappings; tests are not executed.
- Dynamic receiver types, full Ruby runtime lookup/visibility, arbitrary metaprogramming, singleton mutation, deferred block execution and runtime loading remain unproven. RSpec DSL execution is not reconstructed.

## Verification and refresh

`internal/scan/native_language_depth_test.go` exercises positive bindings, ambiguity, typed scopes, header/import cycles, opaque text, prototype mismatch, function-pointer shadows, deferred execution and framework evidence. `internal/agent/context_language_integration_test.go` builds isolated agent indexes and verifies caller/target source sections with read receipts for calls across files in all three languages.

The standard full suite, focused race tests, `go vet` and generated-documentation checks remain required. Productive applications are not executed or changed by these fixtures. Analysis revisions change so an already enabled watcher can regenerate old analysis after it adopts the installed binary. Context queries never trigger that regeneration.

## Language references

- [Clang Objective-C categories](https://clang.llvm.org/doxygen/classclang_1_1ObjCCategoryDecl.html)
- [Apple XCTest method requirements](https://developer.apple.com/documentation/xctest/defining-test-cases-and-test-methods?language=objc)
- [C++ overload resolution](https://www.eel.is/c%2B%2Bdraft/over)
- [GoogleTest primer](https://google.github.io/googletest/primer.html)
- [Ruby modules, classes and singleton classes](https://docs.ruby-lang.org/en/master/syntax/modules_and_classes_rdoc.html)
