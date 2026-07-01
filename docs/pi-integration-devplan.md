# Pi Integration Development Plan

**Status:** Planned  
**Canonical source:** `SPEC.md` remains authoritative.  
**Purpose:** Make Pi the default terminal coding interface for Nexdev while keeping Nexdev's Go control-plane, provider router, safety model, and Bubbletea fallback intact.  
**Execution model:** Parallel subagents for research and discovery; sequential implementation subagents for code changes.

## 1. Goal

When a user runs `nexdev` with no subcommand, Nexdev launches Pi as the primary terminal coding assistant. The user types to the coding agent in Pi. From inside Pi, `Ctrl+N` opens a Nexdev control menu for monitoring, pipeline control, provider/config visibility, run history, and steering.

The initial integration intentionally keeps Pi's own assistant loop separate from Nexdev's staged orchestration loop. Nexdev remains the source of truth for runs, control-plane state, events, provider routing, and safety policy. Pi becomes the terminal surface and embedded control client.

## 2. Non-Goals

- Do not route Pi chat prompts into Nexdev's full pipeline in the first milestone.
- Do not expose Nexdev provider credentials as Pi custom providers in the first milestone.
- Do not delete or rewrite the existing Bubbletea TUI.
- Do not edit `SPEC.md` unless a spec-management subagent is explicitly assigned.
- Do not mutate control-plane state through Pi tools that the model can call autonomously.
- Do not log API keys, bearer tokens, auth headers, or unredacted provider errors.

## 3. Architecture Summary

Nexdev launches Pi by shelling out to the `pi` binary with stdio inherited and the Nexdev Pi extension loaded. This is the most reliable path for a Go host because Pi's SDK is TypeScript/Node-oriented and RPC mode is intended for language-agnostic programmatic control, not for embedding the full TUI in a Go process.

Runtime flow:

1. Go CLI loads Nexdev config and project runtime.
2. Go starts or attaches to the local loopback control-plane.
3. Go detects the `pi` binary and resolves the Nexdev extension path.
4. Go passes `NEXDEV_CONTROL_URL`, `NEXDEV_CONTROL_TOKEN`, `NEXDEV_PROJECT_DIR`, and optional `NEXDEV_RUN_ID` to Pi.
5. Go starts `pi --extension <extensions/nexdev/index.ts>` with stdin/stdout/stderr inherited.
6. Pi opens in normal coding assistant mode.
7. The extension handles `session_start`, renders a welcome banner, starts status polling, and registers `Ctrl+N`.
8. `Ctrl+N` opens a full-screen Nexdev overlay menu.
9. Menu actions call Nexdev control-plane HTTP endpoints directly.
10. Closing Pi returns the Pi process exit status to Nexdev.

Data ownership:

- Pi owns the visible terminal coding assistant session.
- Nexdev owns run state, control actions, events, artifacts, config, auth, provider routing, audit, and redaction.
- The Pi extension stores only ephemeral UI state such as the menu stack, whether the hint was dismissed, cached status text, and the latest visible events.
- Durable state is always fetched from or submitted to the Nexdev control-plane.

## 4. Subagent Strategy

Research and discovery should run in parallel. Code-writing subagents should run sequentially because file ownership and contracts overlap across the CLI launcher, extension client, widgets, menu actions, docs, and tests.

### 4.1 Parallel Research Subagents

Use these before implementation or when the Pi API changes.

| Subagent | Role | Inputs | Output |
|---|---|---|---|
| `PI-RESEARCH` | Pi API scout | Pi TUI, extensions, keybindings, SDK, custom provider docs | Exact API names, risks, version constraints |
| `NEXDEV-SOURCE-SCOUT` | Nexdev control-plane/TUI scout | `internal/tui`, `internal/controlplane`, `internal/cli`, provider files | Endpoint map, current keybindings, CLI behavior |
| `CONTRACT-SCOUT` | Contract and safety scout | `SPEC.md`, `docs/contracts.md`, `api/openapi.yaml`, `TESTING_STRATEGY.md` | Contract impacts, auth/redaction/test requirements |
| `PACKAGING-SCOUT` | Distribution scout | Go embedding options, Node/TS extension packaging, release scripts | Recommended extension packaging path |
| `KEYBINDING-SCOUT` | UX conflict scout | Pi keybinding docs and defaults | Ctrl+N conflict report and fallback recommendation |

Research subagents must not edit files. Each returns citations, unresolved risks, and recommended implementation constraints.

### 4.2 Sequential Implementation Subagents

Run coding subagents in this order. Each subagent receives one task, owned files, allowed shared contracts, test commands, and docs obligations.

| Order | Task ID | Subagent | Primary ownership | Depends on |
|---|---|---|---|---|
| 1 | `PI-01` | Extension scaffold worker | `extensions/nexdev/*` package scaffold | Research complete |
| 2 | `PI-02` | Control-plane client worker | `extensions/nexdev/client.ts`, `types.ts` | `PI-01` |
| 3 | `PI-03` | Widget worker | `extensions/nexdev/widgets.ts` | `PI-02` |
| 4 | `PI-04` | Menu overlay worker | `extensions/nexdev/menu.ts` | `PI-02`, `PI-03` |
| 5 | `PI-05` | Steer flow worker | `extensions/nexdev/steer.ts` | `PI-04` |
| 6 | `PI-06` | Go launcher worker | `internal/cli/pi.go`, root command wiring | `PI-01` through `PI-05` |
| 7 | `PI-07` | Fallback and CLI compatibility worker | `internal/cli`, Bubbletea fallback tests | `PI-06` |
| 8 | `PI-08` | Packaging worker | extension install/embed/cache, release scripts | `PI-06` |
| 9 | `PI-09` | Docs worker | architecture, contracts, setup, testing docs | `PI-01` through `PI-08` |
| 10 | `PI-10` | Acceptance and release-gate worker | integration tests, smoke tests, final gates | all prior tasks |

Implementation subagents must follow `WORKER_PROTOCOL.md` and update task status evidence in the canonical development tracker when assigned by the orchestrator.

## 5. File Manifest

Owned create/edit:

| File | Purpose |
|---|---|
| `extensions/nexdev/package.json` | Extension dependencies and scripts. |
| `extensions/nexdev/tsconfig.json` | Strict TypeScript compile config. |
| `extensions/nexdev/index.ts` | Pi extension entry point, lifecycle hooks, shortcut registration. |
| `extensions/nexdev/client.ts` | Typed HTTP client for the Nexdev control-plane. |
| `extensions/nexdev/types.ts` | Minimal DTOs for status, events, config, control requests. |
| `extensions/nexdev/widgets.ts` | Welcome banner, menu hint, status footer, event stream component. |
| `extensions/nexdev/menu.ts` | Ctrl+N menu, submenu stack, confirmations, view renderers. |
| `extensions/nexdev/steer.ts` | Multiline steer editor and `/steer` submission. |
| `internal/cli/pi.go` | Pi binary detection, env setup, process launch. |
| `internal/cli/root.go` | Default no-subcommand Pi launch and fallback flag wiring. |
| `internal/cli/*_test.go` | Launcher and fallback behavior tests. |
| `docs/architecture.md` | Runtime boundary updates. |
| `docs/contracts.md` | Env/control-plane client contract updates if needed. |
| `TESTING_STRATEGY.md` | Extension compile and smoke test commands. |
| `README.md` or `docs/SETUP.md` | User-facing install/setup instructions. |

Must not touch unless separately assigned:

| File | Reason |
|---|---|
| `SPEC.md` | Spec-management-only. |
| `internal/provider/provider.go` | Provider boundary is out of scope. |
| `internal/provider/registry.go` | No Pi provider bridge in first milestone. |
| `internal/provider/router.go` | Nexdev staged provider routing remains unchanged. |
| `internal/tui/nexdev.go` | Keep Bubbletea fallback stable. |
| `internal/tui/interview.go` | Legacy flow unaffected. |
| `internal/tui/review.go` | Legacy flow unaffected. |
| `internal/tui/selector.go` | Legacy selector unaffected. |
| `internal/tui/status.go` | Legacy status dashboard unaffected. |
| `api/generated/*` | Generated-code ownership required. |

## 6. Milestones

### M1: Extension Scaffold

Goal: Create a compilable Pi extension package with no runtime Nexdev behavior beyond loading.

Subagent: `PI-01 Extension scaffold worker`

Owned files:

- `extensions/nexdev/package.json`
- `extensions/nexdev/tsconfig.json`
- `extensions/nexdev/index.ts`
- `extensions/nexdev/types.ts`

Implementation details:

- Export the Pi extension factory from `index.ts`.
- Guard TUI-only features with `ctx.mode === "tui"` or `ctx.hasUI` as appropriate.
- Register a lightweight `/nexdev` command only if useful for diagnostics.
- Do not start timers, streams, or background processes in the extension factory.
- Defer runtime work to `session_start` and clean up on `session_shutdown`.

Tests:

- `cd extensions/nexdev && npx tsc --noEmit`

Acceptance:

- Extension compiles.
- Extension can be loaded by Pi in development with `pi -e ./extensions/nexdev/index.ts`.

### M2: Control-Plane HTTP Client

Goal: Add a typed client used by all widgets and menus.

Subagent: `PI-02 Control-plane client worker`

Owned files:

- `extensions/nexdev/client.ts`
- `extensions/nexdev/types.ts`

Implementation details:

- Read `NEXDEV_CONTROL_URL` from env.
- Read optional `NEXDEV_CONTROL_TOKEN` from env.
- Apply bearer auth only when token is present.
- Implement `getStatus`, `pause`, `resume`, `skip`, `cancel`, `steer`, `detour`, and config/provider read methods only when backed by existing endpoints.
- Redact token and auth headers from all thrown/displayed errors.
- Use request timeouts and abort signals.
- Treat all response text as untrusted; parse JSON only into expected DTOs.

Tests:

- TypeScript unit tests if a TS test runner is introduced.
- Minimum compile check: `cd extensions/nexdev && npx tsc --noEmit`.

Acceptance:

- Client rejects missing control URL with a safe UI error.
- Client never includes bearer tokens in error strings.

### M3: Welcome, Hint, and Status Footer

Goal: Make Pi visibly feel like the Nexdev terminal interface without disrupting coding.

Subagent: `PI-03 Widget worker`

Owned files:

- `extensions/nexdev/widgets.ts`
- `extensions/nexdev/index.ts`

Implementation details:

- On `session_start`, render a welcome banner above the editor with `Press Ctrl+N to open the Nexdev menu`.
- Render a one-line below-editor hint until first menu open.
- Start status polling after `session_start`, not in the extension factory.
- Update status through `ctx.ui.setStatus("nexdev.run", text)` or the most stable Pi footer API.
- Poll `/status` every 2 to 5 seconds.
- Show run ID, status, stage, current task, and cost if available.
- Keep footer to one line and truncate long fields.
- Stop timers on `session_shutdown`.

Tests:

- `cd extensions/nexdev && npx tsc --noEmit`
- Manual Pi smoke until Pi UI test harness exists.

Acceptance:

- Pi starts in coding mode.
- Banner is visible.
- Footer updates without stealing focus from the editor.

### M4: Ctrl+N Menu Overlay

Goal: Add the full Nexdev overlay menu with nested navigation.

Subagent: `PI-04 Menu overlay worker`

Owned files:

- `extensions/nexdev/menu.ts`
- `extensions/nexdev/index.ts`
- `extensions/nexdev/widgets.ts`

Implementation details:

- Register `pi.registerShortcut("ctrl+n", { description, handler })`.
- Use `ctx.ui.custom()` with `{ overlay: true }`.
- Render menu options using Pi TUI components such as `SelectList` with a bordered overlay component.
- Implement an explicit menu stack for nested overlays.
- Back pops the stack.
- Escape closes the current overlay.
- Closing the menu returns focus to the Pi editor.
- Hide the below-editor Ctrl+N hint after the first successful open.
- Handle Ctrl+N conflicts gracefully and document fallback if needed.

Menu tree:

```text
Nexdev Menu
├── Monitor Run
│   ├── Overview
│   ├── Events
│   ├── Plan / Tasks
│   ├── Blockers
│   └── Artifacts
├── Control Run
│   ├── Pause / Resume
│   ├── Skip Task
│   ├── Cancel Run
│   ├── Steer
│   └── Request Detour
├── Providers
│   ├── List Providers
│   └── Test Provider deferred
├── New Run
├── Config
└── Close Menu
```

Tests:

- `cd extensions/nexdev && npx tsc --noEmit`
- Manual menu smoke with Pi.

Acceptance:

- Ctrl+N opens the top-level menu.
- Nested menus can go forward, back, and close without corrupting editor focus.

### M5: Monitor Views

Goal: Map the existing Bubbletea monitor views into Pi overlays.

Subagent: `PI-04 Menu overlay worker`, continued or separate monitor worker if ownership is split.

Owned files:

- `extensions/nexdev/menu.ts`
- `extensions/nexdev/widgets.ts`
- `extensions/nexdev/client.ts`

Implementation details:

- Overview maps to Bubbletea `ViewOverview` and `GET /status`.
- Events maps to Bubbletea `ViewEvents` and the live event stream.
- Plan / Tasks maps to Bubbletea `ViewPlan`, read-only.
- Blockers maps to Bubbletea `ViewBlockers`.
- Artifacts maps to Bubbletea `ViewArtifacts`.
- If an endpoint is missing or deferred, render a disabled state with the exact missing endpoint or service.
- Event stream view must not block the Pi agent loop.
- Use bounded buffers for live events, default latest 100.
- Use `AbortController` or Pi handler signals to cancel stream/polling when the overlay closes.

Tests:

- `cd extensions/nexdev && npx tsc --noEmit`
- Control-plane client tests for response parsing if a TS test runner exists.
- Manual event stream smoke against `nexdev serve` or fake-provider E2E control-plane.

Acceptance:

- All five monitor views render from live control-plane data or explicit disabled/deferred state.
- Events auto-refresh without blocking typing in Pi after the overlay closes.

### M6: Control Actions

Goal: Wire pause/resume, skip, cancel, detour, and confirmations.

Subagent: `PI-04 Menu overlay worker`, continued.

Owned files:

- `extensions/nexdev/menu.ts`
- `extensions/nexdev/client.ts`

Implementation details:

- Pause/Resume calls `/pause` or `/resume` based on current status.
- Skip Task requires `ctx.ui.confirm()` and calls `/skip`.
- Cancel Run requires `ctx.ui.confirm()` and calls `/cancel`.
- Request Detour calls `/detour` with project/run/task context and a user-visible reason flow if supported.
- Show `ctx.ui.notify()` success or redacted failure messages.
- Do not expose these actions as model-callable tools in the first implementation.

Tests:

- `cd extensions/nexdev && npx tsc --noEmit`
- Client-level tests with mocked fetch if test runner exists.

Acceptance:

- Mutations are explicit user actions.
- Destructive actions require confirmation.
- Errors are redacted.

### M7: Steer Text Input

Goal: Replace the current hardcoded steer placeholder with real multiline input in Pi.

Subagent: `PI-05 Steer flow worker`

Owned files:

- `extensions/nexdev/steer.ts`
- `extensions/nexdev/menu.ts`
- `extensions/nexdev/client.ts`

Implementation details:

- Use `ctx.ui.editor("Steer Nexdev", "")` for multiline text.
- Reject empty or whitespace-only messages locally.
- POST `{ "message": text, "source": "pi" }` to `/steer`.
- On success, notify `Steering message sent to Nexdev`.
- On failure, notify with a redacted error.
- Do not use `ctx.ui.getEditorText()` because that captures the main Pi coding prompt.
- Do not copy Bubbletea's hardcoded placeholder behavior.

Tests:

- `cd extensions/nexdev && npx tsc --noEmit`
- Client tests for `/steer` payload shape if a TS test runner exists.

Acceptance:

- Steer accepts multiline text.
- `/steer` receives the exact user message.
- Empty messages are not sent.

### M8: Go Pi Launcher

Goal: Make `nexdev` launch Pi by default while preserving existing modes.

Subagent: `PI-06 Go launcher worker`

Owned files:

- `internal/cli/pi.go`
- `internal/cli/root.go`
- `internal/cli/*_test.go`

Implementation details:

- Detect the `pi` binary through `exec.LookPath("pi")`.
- Return a clear install/help error if missing.
- Add `--no-pi` or equivalent fallback flag if accepted by orchestrator.
- Preserve `--no-tui` for headless run behavior.
- Ensure `nexdev run --no-tui` remains headless.
- Ensure `nexdev tui` or fallback command opens Bubbletea.
- Pass extension path and env vars to the Pi process.
- Inherit stdio so Pi owns the terminal.
- Propagate Pi exit code.
- Do not weaken loopback bind/auth validation.

Tests:

- `go test ./internal/cli`
- Test missing Pi binary handling with a controlled `PATH`.
- Test env var construction without launching a real Pi binary.
- Test no-subcommand dispatch chooses Pi launcher when interactive and no fallback flag is set.

Acceptance:

- `nexdev` launches Pi when Pi is installed.
- Missing Pi prints actionable guidance and does not panic.
- Existing headless and Bubbletea fallback paths still work.

### M9: Packaging and Distribution

Goal: Make the extension available outside a development checkout.

Subagent: `PI-08 Packaging worker`

Owned files:

- Go launcher files as needed.
- `extensions/nexdev/package.json`
- Release/check scripts if assigned.
- Setup docs.

Implementation details:

- Start with source-path loading for local development.
- Decide whether release builds embed the extension source and write it to a cache directory before launching Pi.
- Cache path must be under Nexdev-controlled state/cache, not the project root unless explicitly documented.
- Validate path writes for traversal and symlink escape where applicable.
- Pin or document the minimum tested Pi version.
- Keep Node/TypeScript install requirements explicit.

Tests:

- `go test ./internal/cli`
- `cd extensions/nexdev && npx tsc --noEmit`
- Release script dry-run or targeted packaging test if available.

Acceptance:

- Developer checkout works.
- Release packaging path is documented or implemented.
- Failure mode is clear if extension dependencies are missing.

### M10: Documentation and Contracts

Goal: Record the behavior in the right Nexdev docs without duplicating the full spec.

Subagent: `PI-09 Docs worker`

Owned files:

- `docs/architecture.md`
- `docs/contracts.md`
- `TESTING_STRATEGY.md`
- `docs/SETUP.md` or `README.md`
- `docs/TUI.md` or a new Pi-specific docs file if accepted

Implementation details:

- Document Pi as the default terminal surface.
- Document Bubbletea fallback.
- Document required env vars passed to Pi.
- Document extension compile command.
- Document security behavior and token redaction.
- Document unresolved provider bridge deferral.
- Do not edit `SPEC.md` unless assigned through spec-management.

Tests:

- Documentation links checked manually unless a doc linter exists.
- `go test ./internal/cli` if CLI help text changed.

Acceptance:

- User can install Pi and understand how `nexdev` launches it.
- Developer can compile the extension.
- Docs do not contradict `SPEC.md`.

### M11: Final Acceptance and Release Gates

Goal: Verify integration end to end.

Subagent: `PI-10 Acceptance and release-gate worker`

Owned files:

- Tests and scripts assigned by orchestrator.
- No broad implementation edits unless fixing assigned issues.

Required checks:

- `cd extensions/nexdev && npx tsc --noEmit`
- `go test ./internal/cli`
- `go test ./internal/tui`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `govulncheck ./...`
- `go mod verify`
- `./scripts/e2e_fake_provider.sh`

Manual smoke checks:

- `nexdev` opens Pi in coding assistant mode.
- Banner appears.
- Ctrl+N opens menu.
- Monitor views render.
- Pause/resume/skip/cancel/detour behave correctly.
- Steer accepts multiline input.
- Bubbletea fallback still opens.
- Headless fake-provider E2E still passes.

Acceptance:

- All automated required gates pass or are reported with exact environment blockers.
- Manual smoke evidence is recorded.
- No secret leakage is observed in logs, UI errors, or test artifacts.

## 7. Menu and Widget Specification

### 7.1 Persistent UI Elements

| Element | Pi API | Position | Update model | Content |
|---|---|---|---|---|
| Welcome banner | `ctx.ui.setWidget` | Above editor | Once on `session_start` | Project/run summary and Ctrl+N hint |
| Run status footer | `ctx.ui.setStatus` or stable footer API | Footer/status area | Poll `/status` every 2 to 5 seconds | Run ID, status, stage, task, cost |
| Ctrl+N hint | `ctx.ui.setWidget` with below-editor placement | Below editor | Until first menu open | `Press Ctrl+N to open the Nexdev menu` |
| Event stream | `ctx.ui.custom` overlay component | Overlay | SSE or polling, cancellable | Latest bounded events |

### 7.2 Overlay Rules

- Use `ctx.ui.custom()` with `{ overlay: true }`.
- Use fresh component instances for submenu transitions.
- Keep explicit stack state for back navigation.
- Close overlays by calling the Pi-provided completion/close handler.
- Do not retain disposed component references.
- Truncate lines to terminal width.
- Keep event buffers bounded.

### 7.3 Control Menu Mapping

| Pi menu action | Existing Bubbletea mapping | Control-plane mapping |
|---|---|---|
| Overview | `ViewOverview` | `GET /status` |
| Events | `ViewEvents` | event stream or `GET /events` |
| Plan / Tasks | `ViewPlan` | `GET /plan` or `/status` task data |
| Blockers | `ViewBlockers` | `/status` blocker data or blockers endpoint |
| Artifacts | `ViewArtifacts` | `GET /artifacts` |
| Pause / Resume | key `p` | `POST /pause`, `POST /resume` |
| Skip Task | key `k` | `POST /skip` with confirm |
| Cancel Run | key `c` | `POST /cancel` with confirm |
| Steer | key `s` | `POST /steer` with Pi editor text |
| Request Detour | key `d` | `POST /detour` |

## 8. Provider Policy

First implementation should not expose Nexdev providers as Pi custom providers.

Rationale:

- Nexdev provider calls are stage-routed, audited, redacted, and cost-tracked through `internal/provider`.
- Pi custom providers are user-facing model-provider definitions for Pi's own assistant loop.
- Bridging them now risks duplicated secret handling and mismatched streaming/tool semantics.
- A provider bridge can be a later opt-in task for simple OpenAI-compatible providers only.

Future provider bridge requirements:

- Spec-management review before exposing credentials.
- Explicit user opt-in.
- No raw API keys in Pi config unless approved.
- Redaction tests.
- Clear separation between Pi assistant provider use and Nexdev pipeline provider routing.

## 9. Testing Plan

Minimum per-task checks:

- Extension-only changes: `cd extensions/nexdev && npx tsc --noEmit`.
- Go launcher changes: `go test ./internal/cli`.
- Bubbletea fallback changes: `go test ./internal/tui`.
- Contract-affecting changes: `go test ./internal/contract` and docs update.
- Security/auth changes: relevant auth/redaction tests plus `go test ./...` before handoff.

Final gates:

- `cd extensions/nexdev && npx tsc --noEmit`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `govulncheck ./...`
- `go mod verify`
- `./scripts/e2e_fake_provider.sh`

Manual smoke matrix:

| Scenario | Expected result |
|---|---|
| `nexdev` with Pi installed | Pi opens in coding assistant mode. |
| `nexdev` without Pi installed | Clear install/fallback message. |
| Ctrl+N | Nexdev menu opens. |
| Escape in menu | Menu closes and editor focus returns. |
| Monitor Overview | Live status renders. |
| Monitor Events | Events update without blocking. |
| Control Pause/Resume | Control-plane updates and confirmation shown. |
| Control Skip/Cancel | Confirmation required before mutation. |
| Steer | Multiline text posts to `/steer`. |
| `nexdev tui` or fallback flag | Bubbletea fallback opens. |
| `nexdev run --no-tui` | Headless mode works. |

## 10. Acceptance Criteria

1. `nexdev` with no subcommand launches Pi in the terminal.
2. Pi opens in coding assistant mode, not directly into the Nexdev menu.
3. Welcome banner is visible with `Press Ctrl+N to open Nexdev menu`.
4. Ctrl+N opens the top-level Nexdev menu overlay.
5. All five Monitor views render live data from the control-plane or explicit deferred states for missing endpoints.
6. Pause, Resume, Skip, and Cancel trigger HTTP calls and show confirmations.
7. Steer opens multiline text input and POSTs to `/steer`.
8. Detour calls `/detour` and shows the result.
9. Run status footer updates without interrupting Pi coding.
10. `nexdev tui` or the accepted fallback flag still works.
11. `nexdev run --no-tui` still works headless.
12. No API keys, bearer tokens, auth headers, or known secrets appear in logs or UI errors.
13. Extension compiles with `cd extensions/nexdev && npx tsc --noEmit`.
14. Go final gates pass or report exact environment blockers.

## 11. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Pi binary is not installed | High | High | Detect with `exec.LookPath`, show install guidance, keep Bubbletea fallback. |
| Pi extension API changes | Medium | Medium | Pin/document tested Pi version, keep API usage minimal, compile in CI. |
| TypeScript extension packaging fails in release binary | Medium | High | Start with dev path, then embed/copy extension to controlled cache path with tests. |
| Control-plane is not running when Pi starts | High | Medium | Go launcher starts or attaches before spawning Pi. |
| Auth token missing or wrong | Medium | High | Pass token through env, fail mutation calls safely, document setup. |
| Token appears in error output | Medium | High | Centralize redaction in extension client and Go launcher logs. |
| Ctrl+N conflicts with Pi/user keybindings | Medium | Medium | Document conflict, support configurable fallback shortcut later. |
| Nested overlay focus bugs | Medium | Medium | Use explicit stack and fresh components; smoke test Back/Escape/focus. |
| Event stream blocks Pi loop | Medium | High | Use async stream/polling, bounded buffer, abort on close. |
| Bubbletea fallback regresses | Low | Medium | Keep `internal/tui` mostly untouched and test fallback command. |
| Provider credentials leak through Pi provider bridge | Medium | High | Do not bridge providers in first implementation. |
| Non-loopback bind without auth | Low | High | Reuse existing config validation and fail startup. |
| `/steer` sends empty placeholder | Medium | Medium | Use Pi editor, reject empty text, test payload. |
| Missing control-plane endpoints for views | Medium | Medium | Render explicit disabled/deferred state and assign later endpoint worker. |
| Manual smoke depends on local terminal/Pi install | High | Low | Separate compile/unit gates from manual smoke and record environment. |

## 12. Open Decisions

These should be resolved by the orchestrator before `PI-06` or `PI-08` starts.

| Decision | Options | Recommendation |
|---|---|---|
| Fallback flag name | `--no-pi`, `nexdev tui`, both | Support explicit `nexdev tui`; add `--no-pi` only if root launch needs a flag. |
| Extension packaging | Source path, embedded copy, npm package | Source path first; embedded copy for release. |
| Pi minimum version | Floating latest, pinned tested version | Pin/document tested version. |
| Ctrl+N fallback | None, config env, CLI flag | Start with Ctrl+N; add configurable fallback only after conflict report. |
| New Run menu | Fully enabled, partial, deferred | Partial/deferred until `/runs` UX is stable. |
| Provider testing menu | Enabled, deferred | Deferred unless provider-test service is wired. |

## 13. Worker Handoff Requirements

Every implementation subagent must return:

```markdown
## Worker Handoff

Task ID:
Worker role:
Summary:
Files changed:
Tests added/changed:
Tests run:
Tests skipped:
Docs updated:
Contracts changed:
Spec impact:
Open risks:
Next recommended task:
Blocker-free: yes/no
```

If blocked, use the blocker handoff from `WORKER_PROTOCOL.md` and stop. Do not continue with guesses or hacks.

## 14. Recommended First Slice

The first implementation slice should prove the architecture with the smallest useful behavior:

1. Create a compilable extension.
2. Make Go launch Pi with the extension in a developer checkout.
3. Show the welcome banner.
4. Show the run status footer from `/status`.
5. Register Ctrl+N and open a top-level menu with disabled placeholder entries.
6. Preserve Bubbletea fallback and headless run behavior.

This slice validates Pi launch, extension loading, UI hooks, control-plane connectivity, and fallback safety before investing in full nested menus and event streaming.
