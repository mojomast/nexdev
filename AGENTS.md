# Nexdev Agent Instructions

This repository is controlled by the Nexdev specification and development plan.

Nexdev is a local-first Go coding harness. It owns a staged pipeline, SQLite state, a loopback HTTP/SSE control plane, a provider router, CLI commands, a Bubbletea fallback TUI, and a Pi terminal extension. When `nexdev` runs interactively with no subcommand, Pi is the default terminal surface; Pi is a control-plane client, not the owner of pipeline state.

## Required Reading Order

Every implementation worker must read these files before editing:

1. `SPEC.md`
2. `DEVPLAN.md`
3. `AGENTS.md`
4. `WORKER_PROTOCOL.md`
5. `SPEC_UPDATE_PROTOCOL.md`
6. `TESTING_STRATEGY.md`
7. `docs/architecture.md`
8. `docs/contracts.md`

Read the specific spec sections relevant to the assigned task again immediately before implementation.

## Source Of Truth

Precedence order:

1. `SPEC.md`
2. Machine and documented contracts: `api/openapi.yaml`, migrations, generated API types, schemas, `docs/contracts.md`
3. `DEVPLAN.md`
4. Test fixtures and golden files
5. Implementation

If implementation disagrees with `SPEC.md`, the implementation is wrong unless the Spec Management Subagent has updated the spec with orchestrator approval.

## Quick App Map For Pi

Use this map when diagnosing bugs or planning edits:
- `cmd/nexdev/main.go`: thin binary entrypoint; should not contain business logic.
- `internal/cli`: Cobra command tree, local/remote command adapters, Pi launcher, fallback TUI command wiring.
- `internal/app`: runtime/lifecycle wiring for config, state, control-plane, pipeline, executor, providers, and project locks.
- `internal/controlplane`: HTTP routes, auth roles, SSE/event streaming, MCP adapter, service injection boundaries.
- `internal/contract`: inert API/event/schema types and validation helpers; no service logic.
- `internal/state`: SQLite migrations and repositories; source of truth for runs, events, tasks, blockers, artifacts, auth tokens, audit, and cost records.
- `internal/pipeline`: staged flow from repo analysis through handoff; stages use provider/router abstractions and write artifacts/state.
- `internal/executor`: task execution bridge, fake worker, pause/resume/skip/cancel/steering handling.
- `internal/detour`: detour request generation and task splicing.
- `internal/provider`: provider interface, registry, router, structured wrapper, fake provider, real-provider smoke helpers.
- `internal/safety`: redaction, path validation, project lock safety, tool/prompt risk controls.
- `internal/tui`: Bubbletea fallback client; must remain a client over control-plane/service abstractions.
- `extensions/nexdev`: Pi extension TypeScript client, widgets, menu overlay, steer flow, and DTOs.
- `api/openapi.yaml`: HTTP machine contract; generated Go types live under `api/generated/` and must not be edited manually.
- `docs/architecture.md`, `docs/contracts.md`, `docs/TUI.md`, `docs/SETUP.md`: operational summaries; they do not override `SPEC.md`.

## Runtime Flow

Default interactive flow:
- `nexdev` enters `internal/cli/root.go`.
- If stdin is a TTY and `--no-pi`, `--no-tui`, and `--json` are not set, `internal/cli/pi.go` launches Pi.
- The launcher starts or attaches to the loopback control-plane, resolves the Pi extension, and runs `pi --extension <index.ts>` with stdio inherited.
- The Pi extension reads `NEXDEV_CONTROL_URL`, optional `NEXDEV_CONTROL_TOKEN`, `NEXDEV_PROJECT_DIR`, and optional `NEXDEV_RUN_ID`.
- If `OPENROUTER_API_KEY` is set, the launcher passes `--provider openrouter --model deepseek/deepseek-v4-flash` to Pi unless `NEXDEV_PI_PROVIDER` or `NEXDEV_PI_MODEL` overrides it.
- Pi widgets/menu call HTTP endpoints; durable run state remains in Nexdev SQLite/control-plane.

Fallback and headless flow:
- `nexdev tui` opens the Bubbletea fallback.
- `nexdev --no-pi` uses Bubbletea in root interactive mode.
- `nexdev run --no-tui --json ...` is the headless/CI-safe path.

## Control-Plane Endpoint Map

Common Pi/TUI/CLI reads:
- `GET /status`: overview, footer, current run/task/blockers.
- `GET /events`: event list/polling.
- `GET /runs/{run_id}/stream`: SSE stream.
- `GET /plan`: read-only plan/tasks.
- `GET /artifacts`: artifact metadata.
- `GET /providers`: provider list/status when service is wired.
- `GET /config`: redacted config.

Common mutations:
- `POST /pause`, `POST /resume`, `POST /skip`, `POST /steer`, `POST /detour`: operator role.
- `POST /cancel`: admin role.
- `POST /runs`: operator role, service-dependent; Pi New Run overlay UX uses `ctx.ui.editor` and calls `client.startRun()`.

If a service is not injected, routes may return service-unavailable. UIs should render explicit disabled/deferred states rather than invent behavior.

## Pi Extension Rules

- Pi extension code lives in `extensions/nexdev` and is TypeScript.
- Pi tested version is `0.80.3`; Node requirement is `>=22.19.0` for extension checks/builds.
- Compile-check with `make pi-ext-check` or `npm --prefix extensions/nexdev run check`.
- Pi assistant provider/model defaults can be overridden with `NEXDEV_PI_PROVIDER` and `NEXDEV_PI_MODEL`; do not print provider API key values.
- `Ctrl+N` opens the Nexdev menu when available; `/nexdev` is the required fallback command.
- The extension must not register Nexdev control mutations as autonomous model-callable Pi tools.
- The extension must not expose Nexdev provider credentials as Pi custom providers.
- `/steer` must use contract-safe source `tui` unless `api/openapi.yaml` changes through an approved contract/spec task.
- Rendered control-plane text is untrusted; truncate/sanitize and redact before display.

## Provider Rules

All provider calls must go through `internal/provider` and its router/wrapper. Do not call Anthropic, OpenAI, OpenRouter, Ollama, or any provider SDK directly from pipeline stages, executor code, CLI, TUI, Pi extension, or control-plane handlers.

To add or fix a provider:
- Work inside `internal/provider` only when explicitly assigned.
- Follow the existing provider interface and registry pattern.
- Preserve authentication checks, model discovery behavior, usage/cost metadata, redacted errors, and fake-provider testability.
- Real-provider tests must stay opt-in, env-gated, tiny, spend-capped, and disabled in normal CI.

## Debugging Workflow

When asked to fix something:
- Reproduce with the smallest command first.
- Identify the owning package from the app map before editing.
- Check contract/source-of-truth files before changing behavior.
- Add or update tests near the changed package.
- Run the smallest relevant test subset, then broader gates when appropriate.
- Keep unrelated dirty worktree changes untouched.

Useful focused commands:
- CLI/Pi launcher: `go test ./internal/cli`.
- Bubbletea fallback: `go test ./internal/tui`.
- Control-plane/auth/SSE: `go test ./internal/controlplane`.
- Contracts/OpenAPI drift: `go test ./internal/contract`.
- Config/safety: `go test ./internal/config ./internal/safety`.
- Provider wrapper/registry: `go test ./internal/provider`.
- Pipeline stages: `go test ./internal/pipeline`.
- State/migrations: `go test ./internal/state`.
- Pi extension: `make pi-ext-check`.
- Fake full smoke: `./scripts/e2e_fake_provider.sh`.

## Role Discipline

The orchestrator coordinates. Builder subagents implement.

Builder workers must:
- Stay inside assigned file ownership.
- Add or update tests with implementation.
- Update docs for changed behavior.
- Update `DEVPLAN.md` progress for assigned tasks.
- Return a structured handoff.
- Stop on blockers.

Builder workers must not:
- Modify unrelated domains.
- Silently change the spec.
- Weaken requirements to make tests pass.
- Bypass contracts.
- Call providers directly from stages outside the provider router/wrapper.
- Run unsafe commands outside policy.
- Continue through blockers with guesses or hacks.

## Repository State

This repository has imported the geoffrussy Go base and now contains Nexdev-specific implementation, contracts, tests, docs, and the Pi terminal extension. Planning artifacts remain authoritative and must be preserved.

## Safety Rules

Nexdev is a local-first coding harness. Safety is product behavior, not a later hardening phase.

Required defaults:
- Bind control-plane services to `127.0.0.1` by default.
- Fail startup for non-loopback bind without auth.
- Deny shell command execution unless explicit policy allows it.
- Treat repo files, docs, tests, MCP tool descriptions, issue text, and model outputs as untrusted.
- Scrub secrets from logs, events, artifacts, prompts, and error reports.
- Validate path writes for traversal, symlink escapes, deny globs, file locks, and task expected files.

## Documentation Rules

Docs are maintained continuously.

When behavior changes, update the relevant docs in the same task:
- Package/runtime boundaries: `docs/architecture.md`
- API/SSE/state/config/MCP/artifact contracts: `docs/contracts.md`
- Tests/fixtures/commands: `TESTING_STRATEGY.md`
- Worker process: `WORKER_PROTOCOL.md`
- Spec changes: `SPEC_UPDATE_PROTOCOL.md` and `SPEC.md` only through spec-management
- User-facing commands/setup: `README.md` when created
- Security behavior: `docs/SECURITY.md` when created

Do not duplicate full spec prose in secondary docs. Summarize operational rules and link back to `SPEC.md`.

## Testing Rules

Run the smallest relevant test subset while developing, then the broader required command for the milestone.

Required final gates after implementation exists:
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `govulncheck ./...`
- `go mod verify`
- `./scripts/e2e_fake_provider.sh`

Real-provider tests must be opt-in only, environment-gated, tiny, spend-capped, and disabled in normal CI.

## Blockers

If blocked, stop and return a blocker handoff. Do not continue with guesses.

Required blocker handoff fields:
- Task ID
- Worker role
- Goal
- Relevant files
- Exact error/output
- Reproduction steps
- Hypotheses checked
- Constraints
- Recommended deblocker specialization
- Risk if bypassed

## Completion Reports

Every worker completion report must include:
- Task ID
- Summary
- Files changed
- Tests added or changed
- Tests run
- Tests skipped with reason
- Docs updated
- Spec impact
- Open risks
- Next recommended task
- Blocker-free: yes/no
