# Nexdev Operating and Release Notes

Canonical requirements live in `SPEC.md`. This document records release-readiness commands for the current implementation.

## Local Release Check

Run the full local gate with:

```bash
./scripts/release_check.sh
```

The script runs OpenAPI/control-plane contract tests, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go mod verify`, `govulncheck ./...`, and `./scripts/e2e_fake_provider.sh`.

Release checks require the module-pinned fixed Go toolchain from `go.mod` (`toolchain go1.25.11`) and `govulncheck` on `PATH`. If `govulncheck` was installed with `go install`, add the Go install directory, commonly `$HOME/go/bin`, to `PATH` before running the gate. The script fails with exit code `127` if `govulncheck` is unavailable. The script does not install tools and does not run real-provider/network tests.

## Optional Real-Provider Smoke

Real-provider smoke is disabled by default:

```bash
./scripts/real_provider_smoke.sh
```

To opt in, set every required gate:

```bash
NEXDEV_RUN_REAL_PROVIDER_TESTS=1
NEXDEV_REAL_PROVIDER=anthropic
NEXDEV_REAL_PROVIDER_MODEL=<tiny-cheap-model>
NEXDEV_REAL_PROVIDER_MAX_USD=0.25
ANTHROPIC_API_KEY=<secret>
```

Do not run real-provider smoke in normal CI. Release jobs may run it only with explicit credentials, spend cap, and timeout controls.

## OpenAPI Drift

Generated OpenAPI server code is not currently checked in. Release checks use the existing contract tests as the drift guard:

```bash
go test ./internal/contract ./internal/controlplane
```

These tests verify `api/openapi.yaml` route/role coverage and parity with the control-plane route role metadata. Full generated-code drift checking remains a release follow-up before a broader public API commitment.
