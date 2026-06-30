# Nexdev

Nexdev is a next-generation local coding harness that turns a project request into reviewed, tested, auditable code through a staged Go-first pipeline.

It brings together Go execution foundations, a pre-development planning pipeline, and a live control plane into one implementation roadmap.

## Current Status

This repository has completed M0 bootstrap and M1 first-wave contract freeze. M0 imported the geoffrussy Go base and set the module path to `github.com/mojomast/nexdev`; M1 C1-C9 froze the initial OpenAPI, event, state migration, stage/status, provider router, executor/steering/detour, auth-role, and test-fixture contracts blocker-free.

Product behavior is still incomplete. CLI, control-plane, MCP, TUI, full pipeline execution, fake-provider E2E, and release behavior must not be assumed until their later milestones land.

Current verified commands:
- `go test ./...`
- `go vet ./...`
- `go mod verify`

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

Implementation is intentionally subagent-oriented and milestone-driven. Completed:
- M0: repository bootstrap and geoffrussy foundation import.
- M1: contract freeze for OpenAPI, events, state, stage interfaces, provider boundary, auth roles, executor/detour interfaces, and test fixtures.

Next actions:
- Continue with M2 config, path, logging, and security baseline work.
- Follow with M3 state repositories, M4 provider behavior, and M5 pipeline framework work.
- Keep later executor, control-plane, MCP, CLI/TUI, E2E, docs, and release milestones aligned with `SPEC.md` and `DEVPLAN.md`.

Use `PROMPT_FOR_DEVELOPMENT_SESSION.md` to start the separate build session.

## Repository Description

Next-generation local coding harness with a Go-first staged pipeline, SQLite state, live HTTP/SSE/MCP control, safe execution, detours, and auditable handoff artifacts.
