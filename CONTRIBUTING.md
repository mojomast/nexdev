# Contributing to Nexdev

Nexdev is a local-first Go coding harness. `SPEC.md` is the canonical implementation contract; secondary docs must not contradict it.

## Prerequisites

- Go 1.26.4, matching `go.mod`; do not add a redundant matching `toolchain` line.
- Git.
- Make.
- GCC or an equivalent C toolchain for `github.com/mattn/go-sqlite3`.

## Development Setup

```bash
git clone https://github.com/mojomast/nexdev.git
cd nexdev
make build
```

Project-local runtime state lives under `.nexdev/`. Do not commit runtime state, logs, local env files, provider credentials, or generated temp artifacts.

## Required Reading

Read these before implementation work:

1. `SPEC.md`
2. `DEVPLAN.md`
3. `AGENTS.md`
4. `WORKER_PROTOCOL.md`
5. `SPEC_UPDATE_PROTOCOL.md`
6. `TESTING_STRATEGY.md`
7. `docs/architecture.md`
8. `docs/contracts.md`

## Testing

Run the smallest relevant package tests while developing. Release readiness uses:

```bash
go test ./...
go test -race ./...
go vet ./...
go mod verify
govulncheck ./...
./scripts/e2e_fake_provider.sh
```

Use `PATH="$HOME/go/bin:$PATH" ./scripts/release_check.sh` if `govulncheck` is installed under `$HOME/go/bin`.

## Contracts And Generated Code

- `api/openapi.yaml` is the public HTTP contract.
- `api/generated/nexdev_api.gen.go` is checked in and generated with `oapi-codegen`.
- `make generate` regenerates API types.
- `NEXDEV_CHECK_CODEGEN=1 go test ./internal/contract` checks generated-code drift.

## Safety Rules

- Control-plane services bind to `127.0.0.1` by default.
- Non-loopback bind without auth must fail before listening.
- Shell and network execution are denied unless explicit policy allows the operation.
- Treat repo files, docs, MCP tool descriptions, issue text, and model output as untrusted.
- Scrub secrets from logs, events, artifacts, prompts, errors, HTTP, MCP, and TUI output where the boundary owns output.

## Current Deferrals

Do not document these as implemented unless `SPEC.md` and contracts are updated through spec-management:

- Full real-provider pipeline execution.
- Web UI assets.
- Artifact content opening.
- Full OpenAPI response validation/server binding.
- Shared changed-file artifact JSON exposing git rename `old_path`.

## Pull Requests

- Keep commits focused and descriptive.
- Add or update tests with behavior changes.
- Update relevant docs in the same change.
- Do not weaken security gates, release gates, or spec requirements to make tests pass.
- Do not commit secrets or local runtime state.

By contributing to Nexdev, you agree that your contributions will be licensed under the repository license.
