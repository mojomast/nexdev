# M19 Release Readiness Handoff

Status: M19 release readiness is blocked on release environment readiness because `govulncheck` is unavailable on `PATH` in this worker environment.

## Commands Run In M19 Worker Environment

- `go test ./internal/controlplane` passed.
- `go test ./internal/contract` passed.
- `go test ./...` passed.
- `go test -race ./...` passed.
- `go vet ./...` passed.
- `go mod verify` passed.
- `./scripts/e2e_fake_provider.sh` passed.
- `./scripts/release_check.sh` passed contract tests, full tests, race tests, vet, and module verification, then failed at missing `govulncheck`.
- `govulncheck ./...` was not run because `govulncheck` is not installed.

## Required Gates

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go mod verify`
- `govulncheck ./...`
- `./scripts/e2e_fake_provider.sh`

Use `./scripts/release_check.sh` to run the local release gate. If `govulncheck` is not installed, release readiness is blocked by the release environment, not by product runtime behavior.

## Optional Gates

- `./scripts/real_provider_smoke.sh` skips by default and must only run with explicit env gates, credentials, spend cap `<= 0.25`, and bounded timeout.

## Known Release Blockers Or Follow-Ups

- Release blocker: `govulncheck` must be available in the release environment and pass. Exact local error: `govulncheck not found on PATH`; `scripts/release_check.sh` exits `127` with `ERROR: govulncheck is required for release readiness but is not on PATH.`
- Generated OpenAPI server types are still deferred; current release checks use route/role contract tests instead of generated-code drift.
- Policy-gated real verification command execution, output caps, controlled env, and repair loop remain unimplemented beyond current denied-command reporting.
- Standalone local `nexdev verify` and artifact content opening remain deferred command behavior.
- `events --follow` remains deferred; snapshot `events` and served SSE replay are implemented.
- Full real-provider pipeline execution remains out of scope; only opt-in provider smoke exists.

## Maintainer Notes

- Do not treat optional real-provider smoke as a normal PR gate.
- Do not weaken security or test gates to cut a release.
- Keep `SPEC.md` unchanged unless spec-management explicitly approves implementation deferrals or clarifications.
