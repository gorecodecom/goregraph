# GoreGraph 1.4.4

GoreGraph 1.4.4 expands local static analysis and improves the reliability of long-running watchers and MCP sessions. Output Schema 3 is retained.

## Language and asset analysis

- Structured C#/.NET/Unity and Swift/SwiftUI adapters, with supported static calls, framework evidence, persistence operations and test links.
- Dart/Flutter analysis and deeper Kotlin, Objective-C, C++, Ruby and authored-language integrations.
- GDScript and Godot text scenes/resources, plus Unity and Blender asset/source references.
- Optional, explicitly generated C#/Swift compiler snapshots with source-hash validation. Normal scans, watchers and MCP queries never run these exporters or application code.
- Scope-aware Go call resolution and explicit diagnostics for unsupported static targets.

Support varies by language and capability. Full static adapters do not imply complete compiler semantics or proven runtime execution. See the README language matrix and linked analysis guides.

## Context and workspace reliability

- More relevant adaptive fallback candidates using the full question and requested directory.
- Project-scoped tooling audits, with broader workspace selection only when requested.
- Coherent workspace publication, readable committed indexes during update preparation and shared-input validation.
- Workspace discovery respects ignore rules.

## Watchers and process updates

- Supervised watchers recover failed workers and adopt a validated executable replacement at the same installation path after the current update finishes.
- Windows scheduled startup uses a hidden Windows Script Host launcher; supervised workers do not open console windows.
- More responsive shutdown during blocked file checks, transient Windows state-sharing retries and clearer operation/timestamp diagnostics.
- macOS/Linux MCP processes with replacement support adopt locally installed updates while preserving their stdio session. Windows MCP clients require reconnection after an update.

Installation does not enable or start a stopped watcher. Older unsupervised watchers require one explicit restart. After upgrading an older Windows task, run `goregraph watch autostart on <workspace>` to register the hidden launcher. Check each workspace with `goregraph watch status <workspace>`. Reconnect MCP clients whose server predates automatic replacement support.

## Dashboard

- Light, Dark and System appearance.
- Clearer capability/diagnostic explanations in Data Quality.
- Swagger 2.0/OpenAPI 3.x contract inventory with source evidence, service matching and explicitly unverified gateway-prefix candidates.

## Installation and verification

GitHub provides checksummed Windows, macOS and Linux archives. Homebrew and Scoop publication accompany the release when configured. Winget 1.4.4 is submitted separately and becomes available after Microsoft approval; use the GitHub ZIP while that review is pending.

The source passed CI on Windows, macOS and Linux, including tests, formatting, vet and generated-documentation checks. Downloaded release assets are checked separately after publication. No new end-to-end token-savings percentage is claimed for this release; earlier measurements remain historical.
