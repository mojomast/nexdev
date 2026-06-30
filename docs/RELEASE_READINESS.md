# M19 Release Readiness Handoff

Status: M19 toolchain vulnerability deblocking updated the module, CI, and release workflow to use the fixed Go toolchain `go1.25.11`. Final release readiness still depends on the full gate passing in the prepared release environment.

## Commands Run In M19 Worker Environment

- `go test ./internal/controlplane` passed.
- `go test ./internal/contract` passed.
- `go test ./...` passed.
- `go test -race ./...` passed.
- `go vet ./...` passed.
- `go mod verify` passed.
- `./scripts/e2e_fake_provider.sh` passed.
- Earlier M19 runs blocked because `govulncheck` was unavailable on `PATH`; the tool is now expected to be installed in the release environment, for example under `$HOME/go/bin` after `go install`.
- The module and CI/release workflows now require Go `1.25.11` through `go.mod` and GitHub Actions setup-go pins.

## Required Gates

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go mod verify`
- `govulncheck ./...`
- `./scripts/e2e_fake_provider.sh`

Use `./scripts/release_check.sh` to run the local release gate. If `govulncheck` is installed under `$HOME/go/bin`, run with `PATH="$HOME/go/bin:$PATH" ./scripts/release_check.sh` or otherwise ensure its install directory is on `PATH`.

## Optional Gates

- `./scripts/real_provider_smoke.sh` skips by default and must only run with explicit env gates, credentials, spend cap `<= 0.25`, and bounded timeout.

## Known Release Blockers Or Follow-Ups

- Release gate: `govulncheck` must remain enabled and pass under the fixed Go toolchain. Do not suppress vulnerabilities or skip the gate.
- Generated OpenAPI server types are still deferred; current release checks use route/role contract tests instead of generated-code drift.
- Policy-gated real verification command execution, output caps, controlled env, and repair loop remain unimplemented beyond current denied-command reporting.
- Standalone local `nexdev verify` and artifact content opening remain deferred command behavior.
- `events --follow` remains deferred; snapshot `events` and served SSE replay are implemented.
- Full real-provider pipeline execution remains out of scope; only opt-in provider smoke exists.

## Maintainer Notes

- Do not treat optional real-provider smoke as a normal PR gate.
- Do not weaken security or test gates to cut a release.
- Keep `SPEC.md` unchanged unless spec-management explicitly approves implementation deferrals or clarifications.
