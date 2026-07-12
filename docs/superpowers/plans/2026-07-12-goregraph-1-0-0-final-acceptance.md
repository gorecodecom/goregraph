# GoreGraph 1.0.0 Final Acceptance Plan

**Goal:** Promote the unchanged, accepted Schema 2 release candidate to the local 1.0.0 version after final verification.

## Tasks

1. Confirm the RC produced no required contract or implementation fixes.
2. Change only version and final release documentation to 1.0.0; preserve Schema 2 and all frozen CLI/Query/MCP contracts.
3. Run formatting checks, full tests, vet, and diff checks.
4. Install the local 1.0.0 binary; verify path, version, and schema.
5. Preview and execute generated-output cleanup, scan all 44 WEKA projects, run Doctor, Query, MCP, schema validation, static dashboard checks, and deterministic repeat hashes without opening a browser.
6. Do not push, tag, create a GitHub/GitLab release, or publish packages without separate authorization.
