# Architecture

## Layers

1. CLI + TUI (`internal/cli`, `internal/tui`, `internal/executor/monitor.go`)
2. Workflow engines (`interview`, `design`, `devplan`, `reviewer`, `executor`)
3. Integrations (`provider`, `mcp`, `git`, `quota`)
4. Persistence (`state` sqlite store)
5. Configuration (`config`)

## Core Workflow Data Path

- Interview answers -> state DB
- Design generation -> `.geoffrussy/architecture.json` + architecture in DB
- Plan generation -> phases/tasks in DB
- Review -> analysis of existing phases
- Develop -> task execution updates + logs + token/cost tracking

## Persistence Contract

- Project-local DB: `.geoffrussy/state.db`
- Config: `~/.geoffrussy/config.yaml`
- Runtime logs: `.geoffrussy/logs/*.log`

## Provider Contract

Providers implement a common interface for:

- auth
- model discovery/listing
- synchronous and streaming calls
- optional rate/quota info

Bridge-level model listing merges discovery/list results and deduplicates.

## MCP Contract

- Server wraps workflow functionality as MCP tools/resources.
- Uses same local project DB as CLI.
- stderr for logs, stdout for protocol only.

## TUI Contract

- Execution monitor: full live control surface.
- Status: interactive project dashboard (default for `status`).
- Review: interactive report browser (default for `review`).
