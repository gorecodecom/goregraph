# GoreGraph 0.9.6 Go And PHP Parity Implementation Plan

**Goal:** Bring Go and PHP to the same evidence-backed end-to-end capability contract as the 0.9.5 reference adapters.

**Architecture:** Reuse `ArchitectureCapabilityFact`, capability coverage, Doctor integrity, Query, and MCP unchanged. Add focused Go and PHP framework recognizers that emit the same language-neutral capabilities and prove them through the shared parity harness.

## Constraints

- Base-language syntax remains separate from framework patterns.
- Facts require deterministic root-relative file/line evidence.
- Dynamic registration, reflection, generated code, and runtime configuration remain explicit static-analysis gaps.
- No project code execution and no new dependencies.
- Existing Schema 1 fields remain compatible; additions are additive.

### Task 1: Go Parity Fixture And Adapter

- Add a realistic static Go fixture covering net/http and common routers, HTTP clients, SQL/GORM, tests, Kafka/AMQP/gRPC, request/response boundaries, middleware, and service calls.
- Extend normalized capability facts for those families.
- Pass the shared capability and evidence assertions.
- Commit the focused Go adapter change.

### Task 2: PHP Parity Fixture And Adapter

- Add a realistic static PHP fixture covering Laravel/Symfony-style routes/controllers, HTTP clients, Eloquent/Doctrine/PDO, PHPUnit/Pest, queues/events/AMQP, gRPC, validation, and response boundaries.
- Extend normalized capability facts for those families.
- Pass the shared capability and evidence assertions.
- Commit the focused PHP adapter change.

### Task 3: Common Consumption And Coverage Gate

- Verify routes, calls, flows, tests, normalized facts, capability evidence, Doctor, Query, and MCP consume Go/PHP facts without language-specific public contracts.
- Mark Go/PHP common capabilities complete only after fixtures pass.
- Add bounded evidence and determinism assertions.

### Task 4: Documentation, Version, And Acceptance

- Document exact Go/PHP framework families and static limits; set 0.9.6.
- Run formatting, full tests, vet, and diff checks.
- Install the local binary; clean and rescan all 44 WEKA projects; inspect outputs, Doctor, Query/MCP, and dashboard payloads without opening a browser.
- Repeat the scan and compare deterministic hashes.
- Do not push, tag, or publish.
