# Nexdev Setup

## Requirements

- Go 1.26.4, matching `go.mod`; there is no redundant matching `toolchain` directive.
- Git
- C toolchain for sqlite (`gcc`, platform equivalents)
- Node >=22.19.0 for the Pi extension TypeScript check/build.
- Pi coding agent 0.80.3 or newer for the default `nexdev` terminal surface. The checked extension package is tested against Pi 0.80.3.

## Build

```bash
make build
./bin/nexdev version
```

Compile-check the Pi extension:

```bash
make pi-ext-check
```

Prepare distributable extension files under `bin/pi-extension`:

```bash
make pi-ext-build
```

## Initialize Project

From your target repo directory:

```bash
nexdev init
```

Project-local state is stored under `.nexdev/`, including `.nexdev/state.db` and `.nexdev/run/project.lock`.

## Terminal UI

Default interactive launch:

```bash
nexdev
```

In an interactive terminal, `nexdev` launches Pi with the Nexdev extension. Pi opens as the coding assistant, and the extension adds a Nexdev status footer plus a `Ctrl+N` menu. If `Ctrl+N` conflicts with a local Pi binding, use `/nexdev` inside Pi.

Fallback and headless modes:

```bash
nexdev tui
nexdev --no-pi
nexdev run --no-tui --json "implement fake smoke"
```

`nexdev tui` and root `--no-pi` use the Bubbletea fallback. `nexdev run --no-tui` remains the headless/CI path and does not launch Pi.

Pi launcher environment:
- `NEXDEV_CONTROL_URL` points the extension at the local or remote Nexdev control plane.
- `NEXDEV_CONTROL_TOKEN` is passed only when configured through `--token` or the environment.
- `NEXDEV_PROJECT_DIR` identifies the project root.
- `NEXDEV_RUN_ID` is set when a latest run is known.

The launcher does not pass provider API keys to Pi or expose Nexdev providers as Pi custom providers.

Pi extension packaging:
- Source checkouts use `extensions/nexdev/index.ts` directly.
- Installed extension files are copied to a Nexdev-controlled user cache before launch.
- `make pi-ext-clean` removes extension build/cache inputs under the repo build paths.
- `make pi-ext-install-dev PI_EXTENSION_DEV_DIR=<dir>` symlinks the source extension into an explicit Pi development extension directory.

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
- Pi receives only the control-plane bearer token needed for HTTP calls. Extension client errors, rendered details, and notifications redact bearer tokens and auth headers.
- Provider credentials stay in Nexdev's provider layer and are not bridged into Pi custom-provider configuration.
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
make pi-ext-check
```

If provider smoke fails auth, verify the explicit opt-in env gates and provider credential variable. Full real-provider pipeline execution remains deferred; fake-provider E2E is the normal local smoke path.
