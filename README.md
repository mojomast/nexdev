# Nexdev

Nexdev is a next-generation local coding harness that turns a project request into reviewed, tested, auditable code through a staged Go-first pipeline.

It brings together Go execution foundations, a pre-development planning pipeline, and a live control plane into one implementation plan.

## Current Status

This repository is currently planning-only. It contains the canonical Nexdev specification and implementation-ready development plan. Production implementation has not started yet.

The first implementation session must bootstrap the repository foundation, preserve these planning artifacts, then execute `DEVPLAN.md` with subagent-owned milestones.

## Source Of Truth

Read these files in order before implementation:

1. `SPEC.md`
2. `DEVPLAN.md`
3. `AGENTS.md`
4. `WORKER_PROTOCOL.md`
5. `SPEC_UPDATE_PROTOCOL.md`
6. `TESTING_STRATEGY.md`
7. `docs/architecture.md`
8. `docs/contracts.md`

If implementation disagrees with `SPEC.md`, the implementation is wrong unless the spec-management workflow has approved and recorded a spec update.

## Planned Architecture

Nexdev is planned as a local-first Go 1.24+ single binary with:
- Staged lifecycle from ideation through handoff.
- SQLite-backed durable state.
- Persisted SSE event replay.
- Permissioned HTTP control plane.
- MCP-compatible external tool surface.
- Provider registry and per-stage provider routing.
- Structured model output validation and repair.
- Detour/deblocker workflow for blocked work.
- Safe-by-default local tool execution.
- Deterministic fake-provider and fake-worker modes for CI.

## Development Plan

Implementation is intentionally subagent-oriented and milestone-driven. Start with:
- M0: repository bootstrap and foundation import/fork decision.
- M1: contract freeze for OpenAPI, events, state, stage interfaces, provider boundary, and test fixtures.
- M2-M19: config, state, provider, pipeline, executor, detours, control plane, MCP, CLI, TUI, observability, security, E2E, docs, and release readiness.

Use `PROMPT_FOR_DEVELOPMENT_SESSION.md` to start the separate build session.

## Repository Description

Next-generation local coding harness with a Go-first staged pipeline, SQLite state, live HTTP/SSE/MCP control, safe execution, detours, and auditable handoff artifacts.
