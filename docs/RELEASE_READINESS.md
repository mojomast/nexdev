# M19 Release Readiness Handoff

Status: M19 toolchain vulnerability deblocking updated the module, CI, and release workflow to use Go `1.26.4`. The local release gate now passes with `govulncheck` on `PATH`.

## Commands Run In M19 Worker Environment

- `go test ./internal/controlplane` passed.
- `go test ./internal/contract` passed.
- `go test ./...` passed.
- `go test -race ./...` passed.
- `go vet ./...` passed.
- `go mod verify` passed.
- `./scripts/e2e_fake_provider.sh` passed.
- `PATH="/home/mojo/go/bin:$PATH" govulncheck ./...` passed with zero reachable vulnerabilities.
- `PATH="/home/mojo/go/bin:$PATH" ./scripts/release_check.sh` passed all local release gates.
- The module and CI/release workflows require Go `1.26.4` through the `go.mod` `go` directive and GitHub Actions setup-go pins.

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

## Known Release Follow-Ups

- Release gate: `govulncheck` is passing under the fixed Go toolchain. Do not suppress vulnerabilities or skip the gate.
- Generated OpenAPI types and drift tests are implemented. Full OpenAPI response validation/server binding remains deferred; current handlers are manually bound to the frozen paths.
- Policy-gated verify command execution, output caps, controlled env, and repair attempts are implemented. Full real-provider pipeline execution remains out of scope; only opt-in provider smoke exists.
- `nexdev events --follow` is implemented for local and remote event streams with SSE reconnect.
- Artifact content opening remains deferred command behavior.
- Web UI assets and the shared changed-file artifact `old_path` extension remain deferred.

## Maintainer Notes

- Do not treat optional real-provider smoke as a normal PR gate.
- Do not weaken security or test gates to cut a release.
- Keep `SPEC.md` unchanged unless spec-management explicitly approves implementation deferrals or clarifications.
