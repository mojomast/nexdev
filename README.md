# Nexdev

Nexdev is a next-generation local coding harness that turns a project request into reviewed, tested, auditable code through a staged Go-first pipeline.

It brings together Go execution foundations, a pre-development planning pipeline, and a live control plane into one implementation roadmap.

## Current Status

This repository has completed M0 bootstrap, M1 first-wave contract freeze, the first CLI/app lifecycle wiring for the local control plane, and the first terminal TUI control client. M12 provides `nexdev serve`, project-local control-plane token commands, and control-plane client adapters over the M10 HTTP/SSE server and M11 MCP routes. M13 adds `nexdev tui` as a terminal-only client over the same control-plane/state service boundaries.

Product behavior is still incomplete. Full pipeline execution, verify/handoff, fake-provider E2E, provider-test service wiring, web UI, and release behavior must not be assumed until their later milestones land.

Current verified commands:
- `go test ./...`
- `go vet ./...`
- `go mod verify`

Local control-plane smoke:
- `nexdev auth token create --role operator --ttl 30d`
- `nexdev serve`
- `nexdev status --json`
- `nexdev tui`

Safe defaults:
- `nexdev serve` binds to `127.0.0.1:7432` by default.
- Binding a non-loopback address without auth fails before listening.
- The project lock is held while `serve` is running and released on shutdown.
- Token plaintext is printed only when created; SQLite stores only the token hash and metadata.
- Quitting the TUI exits only the terminal client; cancel/skip actions require explicit confirmation and route through control-plane services.

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
