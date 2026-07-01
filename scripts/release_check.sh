#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

run() {
  printf '\n==> %s\n' "$*"
  "$@"
}

run go test ./internal/contract ./internal/controlplane
run make pi-ext-check
run go test ./...
run go test -race ./...
run go vet ./...
run go mod verify

if ! command -v govulncheck >/dev/null 2>&1; then
  printf '\nERROR: govulncheck is required for release readiness but is not on PATH.\n' >&2
  printf 'Install it in the release environment before rerunning this script.\n' >&2
  exit 127
fi
run govulncheck ./...

run ./scripts/e2e_fake_provider.sh

printf '\nRelease gates passed. Real-provider smoke remains opt-in and is not run by this script.\n'
