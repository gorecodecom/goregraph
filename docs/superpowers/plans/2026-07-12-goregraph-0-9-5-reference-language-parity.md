# GoreGraph 0.9.5 Reference Language Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax.

**Goal:** Bring Java/Spring and JavaScript/TypeScript/Node/React to the same evidence-backed end-to-end capability contract.

**Architecture:** Extend focused base-language analyzers and separate framework adapters. A shared parity harness asks identical questions of each ecosystem and rejects `COMPLETE` coverage unless representative fixtures prove symbols, calls, routes, clients, tests, persistence, messaging, gRPC, data flow, evidence, diagnostics, Query, MCP, and dashboard consumption.

## Constraints

- Generic core types contain no Spring, React, WEKA, or repository-specific names.
- Framework adapters own conventions; base analyzers own syntax facts.
- Unsupported dynamic behavior produces gaps/coverage warnings, never invented facts.
- No dependency or project code execution; fixtures are static source only.
- Existing outputs remain compatible; new records are additive.

### Task 1: Shared Parity Contract And Fixtures
- [ ] Define `ParityCapability` acceptance questions and a fixture assertion harness.
- [ ] Add Java/Spring and React/Node mixed fixtures with routes, client call, DTO, validation, persistence, test, Kafka/AMQP, and gRPC examples.
- [ ] Verify both fixtures fail the same missing-capability assertions before analyzer changes.
- [ ] Commit `test: add reference language parity contract`.

### Task 2: Java And Spring Adapter Completion
- [ ] Add failing tests for Java records/interfaces/enums/constructors, Gradle modules, MVC/WebFlux, filters/security/validation, DTOs, JPA/JDBC, RestTemplate/WebClient/declarative clients, JUnit/Spring tests, Kafka/RabbitMQ, and gRPC.
- [ ] Implement focused Java syntax and Spring/framework extractors with evidence IDs and normalized common facts.
- [ ] Pass Java parity harness; commit `feat: complete Java Spring reference adapter`.

### Task 3: JavaScript TypeScript Node React Completion
- [ ] Add failing tests for TS types/interfaces/enums/generics, Express/Fastify/Nest/Next, middleware/guards, Fetch/Axios/wrappers, Prisma/TypeORM/Sequelize/SQL, Jest/Vitest/node:test, KafkaJS/AMQP/gRPC, and React components/props/state/events/hooks/context/forms/router/RTL.
- [ ] Implement focused language/framework extractors and evidence-backed common facts.
- [ ] Pass JS/TS/Node/React parity harness; commit `feat: complete JS TS Node React reference adapters`.

### Task 4: Mixed End-To-End Workspace
- [ ] Connect React action through API client, Java/Node route, validation, service, persistence/message, response usage, and tests.
- [ ] Assert Architecture, Endpoints, Directed Trace, Data Flow, Diagnostics, Coverage, Query and MCP consume the same facts.
- [ ] Commit `test: prove mixed reference workspace parity`.

### Task 5: Coverage Gate And Doctor
- [ ] Mark reference capabilities `COMPLETE` only when parity fixtures pass; otherwise retain `PARTIAL`/`UNAVAILABLE` with reason.
- [ ] Doctor validates parity manifest and evidence references.
- [ ] Commit `feat: enforce reference language parity coverage`.

### Task 6: Docs, Version, Acceptance
- [ ] Document exact supported frameworks and static limits; set 0.9.5.
- [ ] Full tests/vet/determinism; install; clean/rescan 44 WEKA projects; inspect real Java and React/Node outputs, Doctor, Query/MCP, dashboard; repeat hashes.
- [ ] No push, tag, or public release.
