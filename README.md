# Nexdev

Nexdev is a next-generation local coding harness that turns a project request into reviewed, tested, auditable code through a staged Go-first pipeline.

It brings together Go execution foundations, a pre-development planning pipeline, and a live control plane into one implementation roadmap.

## Current Status

This repository has completed final TASK-01 through TASK-10 stabilization and the Pi terminal integration documentation pass. The fake-provider pipeline, control plane, SSE follow, policy-gated verify runner, generated OpenAPI types, cost summary, git-diff changed files, stale-lock handling, hostile security fixtures, SSE stress coverage, CLI cleanup, and Pi default terminal surface are implemented at their assigned scope.

Explicit deferrals remain: full real-provider pipeline execution, web UI assets, artifact content opening, full OpenAPI response validation, and exposing git rename `old_path` in shared changed-file artifact JSON.

Current local verification commands:
- `go test ./...`
- `go vet ./...`
- `go mod verify`
- `./scripts/e2e_fake_provider.sh`
- `make pi-ext-check`
- `PATH="$HOME/go/bin:$PATH" govulncheck ./...`
- `PATH="$HOME/go/bin:$PATH" ./scripts/release_check.sh`

Release-gate commands:
- `go test -race ./...`
- `govulncheck ./...`, after installing `govulncheck` and making its install directory, commonly `$HOME/go/bin`, available on `PATH`; release checks use Go `1.26.4`

Local control-plane smoke:
- `nexdev auth token create --role operator --ttl 30d`
- `nexdev serve`
- `nexdev status --json`
- `nexdev events`
- `nexdev events --follow`
- `nexdev artifacts list`
- `nexdev provider list`
- `nexdev tui`
- `nexdev` to launch Pi in an interactive terminal, with `nexdev tui` or `nexdev --no-pi` for Bubbletea fallback
- `nexdev verify`

Fake-provider E2E smoke:
- `nexdev run --fake-provider --no-tui --json "implement fake smoke"`
- `./scripts/e2e_fake_provider.sh`

Optional real-provider smoke:
- Default command is safe and skips without network: `./scripts/real_provider_smoke.sh`
- To opt in, set `NEXDEV_RUN_REAL_PROVIDER_TESTS=1`, `NEXDEV_REAL_PROVIDER`, `NEXDEV_REAL_PROVIDER_MODEL`, `NEXDEV_REAL_PROVIDER_MAX_USD`, and credentials via the provider key env such as `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`.
- The spend cap must be `> 0` and `<= 0.25`; timeout defaults to 15 seconds and can be set with `NEXDEV_REAL_PROVIDER_TIMEOUT_S` from 1 to 30 seconds.
- The smoke sends only a tiny JSON prompt through the provider structured-output wrapper. Do not put secrets in prompts, model names, or docs used by the smoke.

Safe defaults:
- `nexdev serve` binds to `127.0.0.1:7432` by default.
- Binding a non-loopback address without auth fails before listening.
- The project lock is held while `serve` is running and released on shutdown.
- Token plaintext is printed only when created; SQLite stores only the token hash and metadata.
- Quitting the TUI exits only the terminal client; cancel/skip actions require explicit confirmation and route through control-plane services.
- `nexdev` launches Pi by default only in interactive terminal mode. `--no-pi` uses Bubbletea fallback, and `--no-tui`/`--json` avoid terminal UI launch.
- Pi receives only `NEXDEV_CONTROL_URL`, optional `NEXDEV_CONTROL_TOKEN`, `NEXDEV_PROJECT_DIR`, and optional `NEXDEV_RUN_ID`; provider credentials are not bridged into Pi custom-provider config.
- The fake-provider run path is explicit opt-in only; fake is not registered in the production provider registry.
- Fake E2E uses the safe fake worker and does not execute shell or network commands.
- Real-provider smoke tests are disabled by default and require explicit env gates, provider credentials, a tiny spend cap, and a strict timeout before any provider call is made.

Generated contract code:
- `make generate` regenerates `api/generated/nexdev_api.gen.go` from `api/openapi.yaml`.
- `NEXDEV_CHECK_CODEGEN=1 go test ./internal/contract` checks codegen drift.

Known deferred behavior:
- Local `nexdev run` without `--fake-provider` returns an explicit deferred error until full real-provider run wiring is assigned.
- `nexdev artifacts open`, generated OpenAPI server binding/response validation, and full real-provider pipeline execution are deferred.
- Web UI assets are not implemented; `nexdev tui` is terminal-only.

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

Nexdev is a local-first Go 1.26.4 single binary with:
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

Implementation is intentionally subagent-oriented and milestone-driven. M0-M19 plus TASK-01 through TASK-10 are implemented or verified at their assigned scope; remaining explicit deferrals are tracked in `DEVPLAN.md` and the spec coverage matrix.

Next actions:
- Keep full release gates green after changes: tests, race, vet, mod verify, govulncheck, generated-code drift, and fake-provider E2E.
- Use `docs/OPERATING.md` and `docs/RELEASE_READINESS.md` for release commands and handoff state.
- Keep real-provider smoke out of normal CI unless explicitly configured as a release job with env gates and spend cap.

Use `PROMPT_FOR_DEVELOPMENT_SESSION.md` to start the separate build session.

## Repository Description

Next-generation local coding harness with a Go-first staged pipeline, SQLite state, live HTTP/SSE/MCP control, safe execution, detours, and auditable handoff artifacts.
