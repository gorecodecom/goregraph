# GoreGraph 0.9.8 Cross-Language Hardening And Python Plan

**Goal:** Complete Python full-adapter parity, retain honest Shell integration coverage, and harden generic mixed-workspace behavior before the Schema 2 release candidate.

## Tasks

1. Add Python parity fixtures for FastAPI/Flask/Django-style routes, requests/httpx/aiohttp, SQLAlchemy/Django ORM/DB drivers, pytest/unittest, Kafka/Celery/AMQP, gRPC, validation, and response boundaries.
2. Emit the common deterministic capability facts and pass the shared full-adapter gate; keep Shell unsupported capabilities explicitly unavailable.
3. Add generic project-name/layout fixtures that prove no WEKA name or `ms-` prefix is required for workspace discovery and relationships.
4. Verify deterministic output, bounded Query/MCP data, Doctor integrity, no silent analyzer failure, and existing dashboard contracts.
5. Document exact support and limits; set 0.9.8; run full tests/vet; install; clean/rescan WEKA; inspect and repeat hashes without browser use; do not publish.
