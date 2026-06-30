# Nexdev Security Behavior

Canonical requirements live in `SPEC.md`, especially section 16. This document records implemented M15 behavior and operator recovery notes.

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

## Cost Guard

`internal/observability.CostGuard` provides deterministic preflight denial before provider launches. Hivemind checks the guard before launching voice calls and synthesis when configured.
