# TUI Guide

Nexdev has two terminal surfaces:
- Pi is the default interactive coding surface for `nexdev` with no subcommand.
- Bubbletea remains the explicit fallback through `nexdev tui` or root `--no-pi`.

`SPEC.md` remains authoritative for product behavior. This guide summarizes current terminal operation only.

## Pi Default Surface

Run:

```bash
nexdev
```

In an interactive terminal, Nexdev starts or attaches to the local control plane, resolves the Nexdev Pi extension, and launches:

```bash
pi --extension <nexdev-extension-index.ts>
```

The Pi process inherits stdin/stdout/stderr. Closing Pi returns Pi's exit status to Nexdev.

Requirements:
- Pi tested version: 0.80.3.
- Node: >=22.19.0 for extension checks/builds.

Pi extension UI:
- Welcome banner above the editor.
- One-line status footer with run, status, stage, task, and cost when available.
- `Ctrl+N` opens the Nexdev overlay menu.
- `/nexdev` is the fallback opener if `Ctrl+N` conflicts.

Pi menu coverage:
- Monitor Overview: `GET /status`.
- Monitor Events: `GET /events` with a bounded buffer.
- Monitor Plan / Tasks: `GET /plan`.
- Monitor Blockers: blockers from `GET /status`.
- Monitor Artifacts: `GET /artifacts`.
- Providers List: `GET /providers`.
- Config: `GET /config`, redacted/read-only.
- Pause/Resume: `POST /pause` or `POST /resume`.
- Skip/Cancel: confirmation first, then `POST /skip` or `POST /cancel`.
- Steer: multiline editor, then `POST /steer`.
- Detour: confirmation/context, then `POST /detour`.

Deferred Pi menu items render explicit disabled states rather than blank screens. New Run (`POST /runs` overlay UX) and provider-test overlay UX remain deferred.

Security behavior:
- Pi is a control-plane client. It does not own pipeline state.
- The extension receives `NEXDEV_CONTROL_URL`, optional `NEXDEV_CONTROL_TOKEN`, `NEXDEV_PROJECT_DIR`, and optional `NEXDEV_RUN_ID`.
- Provider credentials are not passed to Pi or exposed as Pi custom providers.
- Token/auth strings are redacted from extension errors, rendered details, and notifications.
- Control-plane data is treated as untrusted display text and sanitized before rendering.

## Bubbletea Fallback

Run:

```bash
nexdev tui
nexdev --no-pi
```

The Bubbletea fallback remains a Nexdev control-plane client. It reads status, events, plan/tasks, blockers, artifacts, config, and providers through the same service paths as CLI/control-plane clients. It does not call providers directly, execute shell commands, or own pipeline state.

Use headless mode for CI or scripted runs:

```bash
nexdev run --no-tui --json "implement fake smoke"
```

`--no-tui` prevents Pi and Bubbletea launch.

## Pi Extension Development

Compile-check:

```bash
make pi-ext-check
```

Build distribution files:

```bash
make pi-ext-build
```

Clean extension artifacts:

```bash
make pi-ext-clean
```

Install a development symlink:

```bash
make pi-ext-install-dev PI_EXTENSION_DEV_DIR=<explicit-pi-dev-extension-dir>
```

The launcher prefers source checkout loading from `extensions/nexdev/index.ts`. Installed extension files are copied to a Nexdev-controlled user cache under `nexdev/pi-extension/pi-0.80.3` before launch with manifest and symlink-escape checks.

## Legacy Imported Notes

The imported geoffrussy Bubble Tea screens below remain historical context for legacy commands and fallback behavior. Nexdev root help and default launch now use Nexdev command names.

## Interview TUI (`interview`)

The interview command uses an interactive chat-based TUI by default (`--tui=true`).

Keys:

- `enter` send message
- `ctrl+s` show conversation summary
- `ctrl+d` finish interview early
- `ctrl+b` back to chat (from summary)
- `?` toggle key help
- `ctrl+c`/`esc` quit (saves session)
- `up/down`, `j/k` scroll chat history

The interview TUI shows:

- scrollable conversation history
- user messages (👤) and AI responses (🤖)
- real-time typing input
- conversation summary with topics covered
- session persistence on exit

Use `geoffrussy interview --tui=false` for simple readline mode.

## Execution Monitor (`develop`)

Interactive monitor appears during `geoffrussy develop`.

Keys:

- `p` pause
- `r` resume
- `s` skip current task
- `f` toggle follow-output
- `?` toggle key help
- `q` quit
- `up/down`, `j/k`, `pgup/pgdn`, `home/end`, `g/G` for log navigation

The monitor shows:

- task/phase progress
- completion percent
- elapsed time
- token in/out counters
- live execution log stream

## Status TUI (`status`)

`geoffrussy status` opens TUI by default.

Use `geoffrussy status --tui=false` for plain text output.

Keys:

- `?` key help
- `q` quit
- scroll keys (same as above)

## Review TUI (`review`)

`geoffrussy review` opens a browsable review dashboard by default.

Use `geoffrussy review --tui=false` for plain text output.

Keys:

- `up/down`, `j/k` navigate
- `enter` drill in/back
- `esc` back/quit
- `q` quit

## Design Language

Monitor/status/review share:

- compact header + status row
- bordered content panes
- consistent color palette and controls
- responsive viewport behavior on terminal resize
