# Nexdev Setup

## Requirements

- Go 1.26.4, matching `go.mod`; there is no redundant matching `toolchain` directive.
- Git
- C toolchain for sqlite (`gcc`, platform equivalents)

## Build

```bash
make build
./bin/nexdev version
```

## Initialize Project

From your target repo directory:

```bash
nexdev init
```

Project-local state is stored under `.nexdev/`, including `.nexdev/state.db` and `.nexdev/run/project.lock`.

## Configure Providers

Provider commands:

```bash
nexdev provider list
nexdev provider test <name>
```

Control-plane token setup:

```bash
nexdev auth token create --role operator --ttl 30d
nexdev serve
```

## Secure Credential Storage

- Nexdev control-plane bearer tokens are printed only when created.
- SQLite stores only token hashes and metadata.
- Real-provider smoke tests are disabled by default and require explicit env gates, credentials, spend cap, and timeout.

## Config File

Project-local state defaults to `.nexdev/`. Use `nexdev config print`, `nexdev config validate`, and `nexdev config set` for supported configuration workflows.

Example:

```yaml
api_keys:
  openai: sk-REDACTED

default_models:
  interview.run: gpt-4o
  interview.followup: gpt-4o-mini
  design.generate: claude-3-5-sonnet
  devplan.generate: gpt-4o
  review.phase: claude-3-5-sonnet
  develop.execute: glm-4.7

budget_limit: 100
verbose_logging: false
```

## Environment Variables

Real-provider smoke opt-in uses provider-specific credential env vars such as:

```bash
export OPENAI_API_KEY=...
export ANTHROPIC_API_KEY=...
```

Run the safe default skip path without credentials:

```bash
./scripts/real_provider_smoke.sh
```

## Validation Checklist

```bash
nexdev version
nexdev doctor
nexdev provider list
nexdev status --json
nexdev events --follow
```

If provider smoke fails auth, verify the explicit opt-in env gates and provider credential variable. Full real-provider pipeline execution remains deferred; fake-provider E2E is the normal local smoke path.
