# Workflow

This is the canonical end-to-end workflow for Geoffrussy.

## Stage Order

`init -> interview -> design -> plan -> review -> develop -> complete`

## Commands by Stage

1. `geoffrussy init`
2. `geoffrussy interview`
3. `geoffrussy design`
4. `geoffrussy plan`
5. `geoffrussy review`
6. `geoffrussy develop`

Status and ops:

- `geoffrussy status`
- `geoffrussy stats`
- `geoffrussy quota --refresh`
- `geoffrussy checkpoint`, `geoffrussy rollback`
- `geoffrussy resume`, `geoffrussy navigate`

## Data Persistence

- Project DB: `.geoffrussy/state.db` (project-local)
- Runtime logs: `.geoffrussy/logs/*.log`
- Design JSON export: `.geoffrussy/architecture.json`

## Stage Transition Notes

- Interview uses AI-driven conversation by default (`--mode=chat`). The AI guides users through project essence, technical constraints, integrations, and scope. Use `--mode=guided` for traditional structured questions.
- Interview completion persists session and advances stage to `design`.
- Design generation/regeneration writes architecture to disk and DB, then sets stage `design`.
- Plan generation persists phases/tasks and sets stage `plan`; current phase is set to first phase.
- Review marks stage `review` and can apply improvements.
- Develop sets stage `develop`, tracks current phase, and auto-promotes to `complete` when all tasks are done and unblocked.

## Interview Flow

The interview phase now features an interactive AI-driven conversation:

1. AI greets user and asks about their project idea
2. Natural back-and-forth gathers requirements across 4 areas:
   - Project Essence (problem, users, value proposition)
   - Technical Constraints (stack, performance, scale)
   - Integration Points (APIs, database, authentication)
   - Scope Definition (MVP features, timeline)
3. AI detects when sufficient information is gathered
4. Structured summary is generated for design phase

Sessions are persisted and can be resumed with `--resume`.

## MCP Parity

MCP handlers mirror the same interview/design/plan/execute transitions and use the same project-local state store.

Start server:

```bash
geoffrussy mcp-server --project-path /absolute/path/to/project
```

## Model Selection Resolution

Model lookup is stage-aware and supports granular keys with fallback. Example fallback for `interview.followup`:

1. `interview.followup`
2. `interview`

Alias support includes `plan -> devplan`.
