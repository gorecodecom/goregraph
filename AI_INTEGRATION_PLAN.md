# GoreGraph AI Integration Plan

## Purpose

This document captures future AI and assistant integration ideas for GoreGraph. These features are intentionally not part of the MVP.

The MVP must stay deterministic, local, and AI-free. AI features should be optional, explicit, and separated from the source-of-truth scan outputs.

## Core Principle

GoreGraph should first create stable project intelligence:

```text
goregraph-out/
  graph.json
  files.json
  symbols.json
  relations.json
  report.md
```

AI should consume these files later. AI should not define the canonical index.

## How Assistants Can Use GoreGraph Output

### Manual Prompting

Users can tell an assistant:

```text
Use goregraph-out/report.md and goregraph-out/graph.json first before searching source files.
```

This is simple and safe, but not automatic.

### Project Documentation Hint

Later, projects may optionally include a small committed documentation file:

```text
GOREGRAPH.md
```

Purpose:

- Explain that local GoreGraph output may exist under `goregraph-out/`.
- Tell humans and assistants which files are useful.
- Remind assistants that generated output is orientation only and real source files remain authoritative.

This file must not be a forceful agent-instruction system like `AGENTS.md`. It should be plain project documentation.

### MCP Integration

Later, GoreGraph can expose a stdio MCP server:

```bash
goregraph mcp
```

Potential MCP tools:

- `query_code_map`
- `get_project_summary`
- `get_file_context`
- `get_symbol`
- `get_related_files`
- `get_test_candidates`

Rules:

- MCP reads existing `goregraph-out/`.
- MCP does not scan automatically unless explicitly requested.
- MCP uses stdio first.
- No HTTP listener by default.
- No automatic agent config writes.
- No Git hooks.

Users must explicitly configure their assistant to use the MCP server.

## Optional AI Commands

Potential future commands:

```bash
goregraph ai summarize
goregraph ai flows
goregraph ai onboarding
goregraph ai hotspots
```

Possible outputs:

```text
goregraph-out/
  ai-summary.md
  ai-flows.md
  ai-onboarding.md
```

These files must be clearly marked as AI-generated and non-authoritative.

## Local Vs Cloud AI

Preferred order:

1. No AI by default.
2. Local AI backend if available.
3. Cloud AI only with explicit flags and documentation.

Cloud AI commands must make data movement clear before execution.

Example future command shape:

```bash
goregraph ai summarize --provider local
goregraph ai summarize --provider openai --include src/auth/**
```

## Determinism Boundary

Canonical files must remain deterministic:

- `manifest.json`
- `files.json`
- `symbols.json`
- `relations.json`
- `graph.json`
- deterministic Markdown reports

AI files may vary and must not be used as the source of truth.

## Assistant Behavior Expectations

Without MCP or explicit user instruction, assistants may not automatically notice GoreGraph output.

Expected user guidance for early versions:

```text
Before inspecting source files, check goregraph-out/report.md and use GoreGraph query output as orientation.
```

Long term, MCP is the preferred assistant integration path.
