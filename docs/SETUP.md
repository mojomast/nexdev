# Setup

## Requirements

- Go 1.24+
- Git
- C toolchain for sqlite (`gcc`, platform equivalents)

## Build

```bash
make build
./bin/geoffrussy version
```

## Initialize Project

From your target repo directory:

```bash
geoffrussy init
```

State is stored in `.geoffrussy/state.db` in that project.

## Configure Providers

Interactive setup:

```bash
geoffrussy config --set-key
geoffrussy config --set-model
```

Provider docs/help:

```bash
geoffrussy config --provider-help <provider>
geoffrussy config --list-providers
```

## Secure Credential Storage

- Geoffrussy attempts to store API keys in OS keyring first.
- If keyring is unavailable, it falls back to config storage and records source metadata.
- Key source state is visible in config output (`keyring`, `env`, `plaintext`, etc).

## Config File

Default path: `~/.geoffrussy/config.yaml`

Example:

```yaml
api_keys:
  openai: sk-...

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

Supported provider key override pattern:

```bash
export GEOFFRUSSY_OPENAI_API_KEY=...
export GEOFFRUSSY_ANTHROPIC_API_KEY=...
export GEOFFRUSSY_OPENROUTER_API_KEY=...
```

Generic pattern also works:

```bash
export GEOFFRUSSY_API_KEY_OPENAI=...
```

## Validation Checklist

```bash
geoffrussy version
geoffrussy config --list-providers
geoffrussy status --tui=false
```

If providers fail auth, use `--provider-help` for the expected key format and docs links.
