# Nexdev Security Behavior

Canonical requirements live in `SPEC.md`, especially section 16. This document records implemented behavior through M17 and operator recovery notes. It is not a substitute for the spec.

## Safe Defaults

- Control-plane services bind to loopback by default and non-loopback bind still fails without auth.
- Shell and network tool execution remain denied by default. Nexdev does not implement a general shell/network runner in M15.
- Writes are checked against project-relative path policy, deny globs, active file-lock globs, and task expected files where owned helpers are used.
- Repo, MCP, model, event, artifact, and UI text are treated as untrusted and scrubbed before display or persistence where the current boundary owns output.

## Redaction

`internal/safety.RedactSecrets` is applied at implemented log, event, artifact, prompt, API error, audit, cost, MCP, and TUI rendering boundaries covered by M15 tests. It scrubs bearer tokens, API-key-like values, password/token assignments, private keys, SSH public keys, and `.env`-style assignments.

## Prompt Injection

`internal/safety.DetectPromptInjection` remains warning-only. M15 adds `security_warning` event emission for Hivemind-owned untrusted repo context when a store/run is present. Warnings do not expand permissions or weaken role/tool/path policy.

## Auth Throttling

Authenticated control-plane routes use a deterministic in-process throttle for auth attempts. Exceeded attempts return `429 rate_limited` and create an `auth_throttle` audit record when audit storage is available.

## Project Locks

The project lock remains `.nexdev/run/project.lock`. M15 adds stale metadata detection with a safe failure policy: old locks can be reported as stale, but Nexdev does not probe processes or delete lock files automatically. Recovery is manual: verify no Nexdev process owns the project, then remove `.nexdev/run/project.lock`.

## Token Storage

Opaque bearer tokens are project-local for the current implementation. Plaintext token values are printed only when created. SQLite stores only the token hash plus role, name, expiry, revocation, and last-used metadata. Non-loopback control-plane bind without auth fails before listening.

## Cost Guard

`internal/observability.CostGuard` provides deterministic preflight denial before provider launches. Hivemind checks the guard before launching voice calls and synthesis when configured.

## Real-Provider Smoke

M17 real-provider smoke checks are disabled by default. A provider call is attempted only when `NEXDEV_RUN_REAL_PROVIDER_TESTS=1`, provider/model env vars, provider credentials, and `NEXDEV_REAL_PROVIDER_MAX_USD` are all present. The cap must be `<= 0.25`, the timeout is at most 30 seconds, and the prompt is a tiny fixed JSON request that does not read repository files. Credential-like values in provider errors are redacted before surfacing through tests, HTTP, or MCP provider-test paths.

## No-Secret Expectations

Do not place provider keys, bearer tokens, `.env` values, private keys, or spend-sensitive account details in prompts, model names, docs used as test fixtures, artifact text, or issue/request text. The redactor is a safety boundary, not permission to intentionally route secrets through logs, events, prompts, artifacts, HTTP, MCP, or TUI output.

## Release Gate Environment

`govulncheck` is required for release readiness as a supply-chain gate. Its availability is an environment requirement; Nexdev does not implement vulnerability scanning as product runtime behavior.

Run `./scripts/release_check.sh` in a prepared release environment. The script fails if `govulncheck` is unavailable and never runs real-provider smoke unless that separate opt-in script is invoked with explicit credentials and spend gates.
