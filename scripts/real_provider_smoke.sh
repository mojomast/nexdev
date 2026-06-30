#!/usr/bin/env bash
set -euo pipefail

if [[ "${NEXDEV_RUN_REAL_PROVIDER_TESTS:-}" != "1" ]]; then
  printf '%s\n' "Skipping real-provider smoke. Set NEXDEV_RUN_REAL_PROVIDER_TESTS=1 plus provider/model/credential/spend-cap env vars to opt in."
  go test ./internal/provider -run 'TestRealProviderSmoke(Config|Redacts)|TestRealProviderSmoke$' -count=1
  exit 0
fi

: "${NEXDEV_REAL_PROVIDER:?required, for example anthropic or openai}"
: "${NEXDEV_REAL_PROVIDER_MODEL:?required, for example claude-3-haiku-20240307 or gpt-4o-mini}"
: "${NEXDEV_REAL_PROVIDER_MAX_USD:?required, must be > 0 and <= 0.25}"

go test ./internal/provider -run '^TestRealProviderSmoke$' -count=1 -v
