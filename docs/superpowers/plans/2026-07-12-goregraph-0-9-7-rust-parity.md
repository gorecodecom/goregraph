# GoreGraph 0.9.7 Rust Parity Implementation Plan

**Goal:** Make Rust a full evidence-backed adapter and add cross-language messaging/RPC facts without weakening the shared parity gate.

**Architecture:** Extend the generic code-intelligence pipeline with Rust functions, calls, tests, and common web routes. Reuse the language-neutral architecture capability facts for clients, persistence, messaging/gRPC, and request/response boundaries.

## Tasks

1. Add a failing Rust parity fixture covering Axum/Actix/Rocket-style routes, reqwest, SQLx/Diesel/SeaORM, Rust tests, Kafka/AMQP, tonic gRPC, request/response types, and downstream calls.
2. Implement Rust function, call, route, and test extraction with deterministic evidence and existing common records.
3. Emit normalized Rust capability facts and mark coverage complete only after the shared fixture passes.
4. Verify Doctor, Query, MCP, trace/data-flow compatibility, bounded output, and deterministic records.
5. Document static limits, set 0.9.7, run full tests/vet, install locally, clean/rescan WEKA, inspect outputs without opening a browser, repeat hashes, and do not publish.
