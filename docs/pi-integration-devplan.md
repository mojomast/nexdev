# Pi Integration Development Plan

**Status:** Active  
**Canonical source:** `SPEC.md` remains authoritative.  
**Purpose:** Make Pi the default terminal coding interface for Nexdev while keeping Nexdev's Go control-plane, provider router, safety model, and Bubbletea fallback intact.  
**Execution model:** Maximum parallel subagents for research; parallel-where-safe implementation lanes converging on sequential Go integration; full Pi TUI surface coverage at every milestone.

---

## 1. Goal

When a user runs `nexdev` with no subcommand, Nexdev launches Pi as the primary terminal coding assistant. The user types to the coding agent in Pi. From inside Pi, `Ctrl+N` opens a Nexdev control menu for monitoring, pipeline control, provider/config visibility, run history, and steering.

The initial integration keeps Pi's own assistant loop separate from Nexdev's staged orchestration loop. Nexdev remains the source of truth for runs, control-plane state, events, provider routing, and safety policy. Pi becomes the terminal surface and embedded control client.

**Pi TUI completeness requirement:** Every control-plane capability visible in the Bubbletea TUI must be reachable from inside Pi before M11 closes. No feature gaps allowed at final acceptance.

---

## 2. Non-Goals

- Do not route Pi chat prompts into Nexdev's full pipeline in the first milestone.
- Do not expose Nexdev provider credentials as Pi custom providers in the first milestone.
- Do not delete or rewrite the existing Bubbletea TUI.
- Do not edit `SPEC.md` unless a spec-management subagent is explicitly assigned.
- Do not mutate control-plane state through Pi tools that the model can call autonomously.
- Do not log API keys, bearer tokens, auth headers, or unredacted provider errors.

---

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
- The Pi extension stores only ephemeral UI state: menu stack, dismissed hint, cached status text, latest visible events.
- Durable state is always fetched from or submitted to the Nexdev control-plane.

---

## 4. Subagent Execution Strategy

### 4.0 Execution DAG

```
Phase 0 (Parallel Research — ALL 5 scouts run simultaneously)
  PI-RESEARCH ──────────────┐
  NEXDEV-SOURCE-SCOUT ──────┤
  CONTRACT-SCOUT ───────────┼──► Phase 1 starts when ALL scouts complete
  PACKAGING-SCOUT ──────────┤
  KEYBINDING-SCOUT ─────────┘

Phase 1 (Parallel Extension Bootstrap — PI-01 + PI-01b run simultaneously)
  PI-01 (scaffold + index.ts) ──────────────────────────┐
  PI-01b (types.ts standalone)  ────────────────────────┤
                                                         ▼
Phase 2 (Parallel Extension Core — PI-02 + PI-02b run simultaneously)
  PI-02  (client.ts — HTTP client) ─────────────────────┐
  PI-02b (types.ts finalization, depends on PI-01b) ─────┤
                                                         ▼
Phase 3 (Parallel Extension UI — PI-03 + PI-04 run simultaneously after PI-02)
  PI-03 (widgets.ts) ───────────────────────────────────┐
  PI-04 (menu.ts scaffold + Ctrl+N) ────────────────────┤
                                                         ▼
Phase 4 (Parallel Extension Features — PI-04b + PI-05 run simultaneously after PI-03/PI-04)
  PI-04b (monitor views + control actions, extends PI-04) ┐
  PI-05  (steer.ts, depends only on PI-04 + PI-02) ───────┤
                                                           ▼
Phase 5 (Sequential Go — must follow all extension work)
  PI-06 (Go launcher) ──► PI-07 (fallback + CLI compat) ──► PI-08 (packaging)

Phase 6 (Parallel Documentation + Acceptance Prep — PI-09 + PI-10 scaffold run simultaneously after PI-08)
  PI-09 (docs) ─────────────────────────────────────────┐
  PI-10 (acceptance gate worker) ───────────────────────┘
```

> **Orchestrator rule:** Never start a phase until every prerequisite task in the previous phase has returned a blocker-free handoff. Stalled scouts must not block other scouts.

---

### 4.1 Parallel Research Subagents (Phase 0)

All five scouts run simultaneously. Research subagents must not edit files. Each returns citations, unresolved risks, and recommended implementation constraints. A scout that cannot complete its research emits a partial result and marks risks as UNRESOLVED — it does not block the other scouts.

| Subagent | Role | Inputs | Output |
|---|---|---|---|
| `PI-RESEARCH` | Pi API scout | Pi TUI, extensions, keybindings, SDK, custom provider docs | Exact API names (`ctx.ui.*`, shortcut registration, overlay signatures), version constraints, breaking change risks |
| `NEXDEV-SOURCE-SCOUT` | Nexdev control-plane/TUI scout | `internal/tui`, `internal/controlplane`, `internal/cli`, provider files | Endpoint map, current keybindings, CLI flag inventory, Bubbletea view list for parity check |
| `CONTRACT-SCOUT` | Contract and safety scout | `SPEC.md`, `docs/contracts.md`, `api/openapi.yaml`, `TESTING_STRATEGY.md` | Contract impacts, auth/redaction/test requirements, list of endpoints needed for full Pi TUI parity |
| `PACKAGING-SCOUT` | Distribution scout | Go embedding options, Node/TS extension packaging, release scripts, `install.sh`, `Makefile` | Recommended extension packaging path, Makefile target spec, Node version requirements |
| `KEYBINDING-SCOUT` | UX conflict scout | Pi keybinding docs, Pi defaults, existing Nexdev TUI keys | Ctrl+N conflict report, fallback recommendation, full keybinding conflict matrix |

---

### 4.2 Implementation Subagents

#### Phase 1 — Parallel Extension Bootstrap

| Task ID | Subagent | Primary ownership | Parallel with | Depends on |
|---|---|---|---|---|
| `PI-01` | Extension scaffold worker | `extensions/nexdev/package.json`, `tsconfig.json`, `index.ts` (lifecycle + shortcut stubs) | `PI-01b` | Phase 0 complete |
| `PI-01b` | Types bootstrap worker | `extensions/nexdev/types.ts` (initial DTO stubs from CONTRACT-SCOUT output) | `PI-01` | Phase 0 complete |

#### Phase 2 — Parallel Extension Core

| Task ID | Subagent | Primary ownership | Parallel with | Depends on |
|---|---|---|---|---|
| `PI-02` | Control-plane client worker | `extensions/nexdev/client.ts` | `PI-02b` | `PI-01` |
| `PI-02b` | Types finalization worker | `extensions/nexdev/types.ts` (full DTOs, merge with PI-01b stubs) | `PI-02` | `PI-01b`, endpoint map from CONTRACT-SCOUT |

#### Phase 3 — Parallel Extension UI

| Task ID | Subagent | Primary ownership | Parallel with | Depends on |
|---|---|---|---|---|
| `PI-03` | Widget worker | `extensions/nexdev/widgets.ts`, `index.ts` (banner + footer wiring) | `PI-04` | `PI-02`, `PI-02b` |
| `PI-04` | Menu scaffold worker | `extensions/nexdev/menu.ts` (menu skeleton, Ctrl+N, overlay stack) | `PI-03` | `PI-02`, `PI-02b` |

#### Phase 4 — Parallel Extension Features

| Task ID | Subagent | Primary ownership | Parallel with | Depends on |
|---|---|---|---|---|
| `PI-04b` | Monitor + control actions worker | `extensions/nexdev/menu.ts` (all 5 monitor views, pause/resume/skip/cancel/detour) | `PI-05` | `PI-03`, `PI-04` |
| `PI-05` | Steer flow worker | `extensions/nexdev/steer.ts`, `menu.ts` (steer integration) | `PI-04b` | `PI-04`, `PI-02` |

> `PI-04b` and `PI-05` both write `menu.ts`. The orchestrator must assign non-overlapping sections: `PI-04b` owns monitor view renderers and action handlers; `PI-05` owns the steer submenu entry and `steer.ts`. Merge conflict resolution is `PI-05`'s responsibility on completion.

#### Phase 5 — Sequential Go Integration

| Order | Task ID | Subagent | Primary ownership | Depends on |
|---|---|---|---|---|
| 1 | `PI-06` | Go launcher worker | `internal/cli/pi.go`, `internal/cli/root.go`, `*_test.go` | All Phase 4 tasks complete |
| 2 | `PI-07` | Fallback and CLI compat worker | `internal/cli` (fallback flag, headless compat tests) | `PI-06` |
| 3 | `PI-08` | Packaging worker | Extension install/embed/cache, release scripts, `Makefile` targets | `PI-06` |

#### Phase 6 — Parallel Documentation + Acceptance

| Task ID | Subagent | Primary ownership | Parallel with | Depends on |
|---|---|---|---|---|
| `PI-09` | Docs worker | `docs/architecture.md`, `docs/contracts.md`, `TESTING_STRATEGY.md`, `docs/SETUP.md` | `PI-10` | `PI-08` |
| `PI-10` | Acceptance + release-gate worker | Integration tests, smoke tests, final gates | `PI-09` (docs must be ready for smoke evidence recording) | `PI-08` |

Implementation subagents must follow `WORKER_PROTOCOL.md` and update task status evidence in the canonical development tracker when assigned by the orchestrator.

---

## 5. Pi TUI Parity Checklist

Every item below must be reachable from inside the Pi TUI before M11 closes. The `NEXDEV-SOURCE-SCOUT` and `CONTRACT-SCOUT` are responsible for populating the "Endpoint" column before Phase 1 begins.

| Pi Menu Path | Bubbletea Equivalent | Endpoint | Status |
|---|---|---|---|
| Monitor → Overview | `ViewOverview` | `GET /status` | ✅ implemented |
| Monitor → Events | `ViewEvents` | SSE or `GET /events` | ✅ implemented with bounded `GET /events` polling |
| Monitor → Plan / Tasks | `ViewPlan` | `GET /plan` | ✅ implemented read-only |
| Monitor → Blockers | `ViewBlockers` | `/status` blockers or dedicated | ✅ implemented from `GET /status` blockers |
| Monitor → Artifacts | `ViewArtifacts` | `GET /artifacts` | ✅ implemented read-only |
| Control → Pause / Resume | key `p` | `POST /pause`, `POST /resume` | ✅ implemented |
| Control → Skip Task | key `k` | `POST /skip` | ✅ implemented with confirmation |
| Control → Cancel Run | key `c` | `POST /cancel` | ✅ implemented with confirmation |
| Control → Steer | key `s` | `POST /steer` | ✅ implemented with multiline Pi editor |
| Control → Request Detour | key `d` | `POST /detour` | ✅ implemented with confirmation/context |
| Providers → List Providers | provider list view | `GET /providers` or config | ✅ implemented via `GET /providers` |
| New Run | new run flow | `POST /runs` or equivalent | ☐ deferred: overlay UX for `POST /runs` remains disabled |
| Config | config view | `GET /config` | ✅ implemented read-only/redacted |
| Run status footer | status bar | `GET /status` poll | ✅ implemented |
| Welcome banner | N/A | N/A | ✅ implemented |
| Ctrl+N shortcut | N/A | N/A | ✅ implemented, with `/nexdev` fallback if shortcut registration conflicts |

Deferred items must render an explicit disabled state with the exact missing endpoint, not a blank screen.

PI-09 status note: Provider listing is implemented. Provider-test overlay UX remains intentionally deferred because provider testing is service-gated and real-provider execution remains explicit-env-gated. New Run remains deferred until `POST /runs` overlay UX is assigned.

---

## 6. File Manifest

### Owned create/edit

| File | Owner task(s) | Purpose |
|---|---|---|
| `extensions/nexdev/package.json` | `PI-01` | Extension dependencies and scripts |
| `extensions/nexdev/tsconfig.json` | `PI-01` | Strict TypeScript compile config |
| `extensions/nexdev/index.ts` | `PI-01`, `PI-03` | Pi extension entry, lifecycle hooks, shortcut registration |
| `extensions/nexdev/types.ts` | `PI-01b`, `PI-02b` | Minimal DTOs for status, events, config, control requests |
| `extensions/nexdev/client.ts` | `PI-02` | Typed HTTP client for Nexdev control-plane |
| `extensions/nexdev/widgets.ts` | `PI-03` | Welcome banner, menu hint, status footer, event stream component |
| `extensions/nexdev/menu.ts` | `PI-04`, `PI-04b`, `PI-05` | Ctrl+N menu, submenu stack, monitor views, actions, steer entry |
| `extensions/nexdev/steer.ts` | `PI-05` | Multiline steer editor and `/steer` submission |
| `internal/cli/pi.go` | `PI-06` | Pi binary detection, env setup, process launch |
| `internal/cli/root.go` | `PI-06` | Default no-subcommand Pi launch and fallback flag wiring |
| `internal/cli/*_test.go` | `PI-06`, `PI-07` | Launcher and fallback behavior tests |
| `docs/architecture.md` | `PI-09` | Runtime boundary updates |
| `docs/contracts.md` | `PI-09` | Env/control-plane client contract updates |
| `TESTING_STRATEGY.md` | `PI-09` | Extension compile and smoke test commands |
| `docs/SETUP.md` or `README.md` | `PI-09` | User-facing install/setup instructions |
| `Makefile` | `PI-08` | `make pi-ext-check`, `make pi-ext-build`, `make pi-ext-clean` targets |

### Must not touch unless separately assigned

| File | Reason |
|---|---|
| `SPEC.md` | Spec-management-only |
| `internal/provider/provider.go` | Provider boundary is out of scope |
| `internal/provider/registry.go` | No Pi provider bridge in first milestone |
| `internal/provider/router.go` | Nexdev staged provider routing unchanged |
| `internal/tui/nexdev.go` | Keep Bubbletea fallback stable |
| `internal/tui/interview.go` | Legacy flow unaffected |
| `internal/tui/review.go` | Legacy flow unaffected |
| `internal/tui/selector.go` | Legacy selector unaffected |
| `internal/tui/status.go` | Legacy status dashboard unaffected |
| `api/generated/*` | Generated-code ownership required |

---

## 7. Milestones

> Each milestone maps to a phase in the DAG. Milestones within the same phase run in parallel unless stated otherwise.

### M1a + M1b: Extension Scaffold + Types Bootstrap (Parallel, Phase 1)

**M1a — PI-01: Extension Scaffold**

Goal: Compilable Pi extension package with no runtime Nexdev behavior beyond loading.

Owned: `extensions/nexdev/package.json`, `tsconfig.json`, `index.ts`

Implementation:
- Export the Pi extension factory from `index.ts`.
- Guard TUI-only features with `ctx.mode === "tui"` or `ctx.hasUI`.
- Register `/nexdev` diagnostic command stub.
- Do not start timers or streams in the factory — defer to `session_start`.
- Clean up on `session_shutdown`.

Tests: `cd extensions/nexdev && npx tsc --noEmit`

Acceptance: Extension compiles and loads with `pi -e ./extensions/nexdev/index.ts`.

---

**M1b — PI-01b: Types Bootstrap**

Goal: Establish initial DTO stubs from CONTRACT-SCOUT endpoint map so PI-02 and PI-02b can start immediately after M1a/M1b.

Owned: `extensions/nexdev/types.ts` (stubs only; PI-02b finalizes)

Implementation:
- Define stub interfaces for `NexdevStatus`, `NexdevEvent`, `NexdevPlan`, `NexdevArtifact`, `NexdevConfig`, `ControlRequest`, `SteerRequest`.
- Use `// TODO: finalize in PI-02b` markers for uncertain fields.
- Export all types; no runtime logic.

Tests: `cd extensions/nexdev && npx tsc --noEmit`

Acceptance: Types file compiles and is importable by `client.ts`.

---

### M2a + M2b: Control-Plane Client + Types Finalization (Parallel, Phase 2)

**M2a — PI-02: Control-Plane HTTP Client**

Goal: Typed client used by all widgets and menus.

Owned: `extensions/nexdev/client.ts`

Implementation:
- Read `NEXDEV_CONTROL_URL` from env; fail safe with UI error if missing.
- Read optional `NEXDEV_CONTROL_TOKEN`; apply bearer auth only when present.
- Implement: `getStatus`, `pause`, `resume`, `skip`, `cancel`, `steer`, `detour`, config/provider reads — only for existing endpoints.
- Redact token and auth headers from all thrown/displayed errors.
- Use request timeouts and abort signals.
- Parse JSON only into expected DTOs; treat all response text as untrusted.

Tests: `cd extensions/nexdev && npx tsc --noEmit`

Acceptance: Client rejects missing URL with safe UI error. Bearer tokens never appear in error strings.

---

**M2b — PI-02b: Types Finalization**

Goal: Finalize all DTOs based on actual endpoint contracts from CONTRACT-SCOUT.

Owned: `extensions/nexdev/types.ts` (merge and finalize PI-01b stubs)

Implementation:
- Remove all `// TODO` stubs with real field definitions.
- Ensure every DTO maps 1:1 to an actual endpoint response.
- Add `Deferred` union type for disabled-state views.

Tests: `cd extensions/nexdev && npx tsc --noEmit`

Acceptance: No stub markers remain. Types align with `api/openapi.yaml` where present.

---

### M3 + M4: Widgets + Menu Scaffold (Parallel, Phase 3)

**M3 — PI-03: Welcome Banner, Hint, and Status Footer**

Goal: Pi visibly feels like the Nexdev terminal interface without disrupting coding.

Owned: `extensions/nexdev/widgets.ts`, `extensions/nexdev/index.ts`

Implementation:
- On `session_start`, render welcome banner above editor: `Press Ctrl+N to open the Nexdev menu`.
- Render one-line below-editor hint until first menu open.
- Start status polling after `session_start`, never in factory.
- Update status via `ctx.ui.setStatus("nexdev.run", text)` or stable Pi footer API.
- Poll `/status` every 2–5 seconds with AbortController on `session_shutdown`.
- Show: run ID, status, stage, current task, cost (if available).
- Keep footer to one line; truncate long fields.

Tests: `cd extensions/nexdev && npx tsc --noEmit` + manual smoke.

Acceptance: Pi starts in coding mode. Banner visible. Footer updates without stealing editor focus.

---

**M4 — PI-04: Ctrl+N Menu Scaffold**

Goal: Ctrl+N opens a working overlay menu with all top-level entries (some disabled stubs OK at this stage).

Owned: `extensions/nexdev/menu.ts`, `extensions/nexdev/index.ts`

Implementation:
- Register `pi.registerShortcut("ctrl+n", { description, handler })`.
- Use `ctx.ui.custom()` with `{ overlay: true }`.
- Render top-level entries using `SelectList` with bordered overlay.
- Implement explicit menu stack; Back pops, Escape closes.
- Closing returns focus to Pi editor.
- Hide below-editor hint after first successful open.
- Document Ctrl+N conflict and fallback per KEYBINDING-SCOUT output.

Menu structure (all entries, stubs allowed):
```
Nexdev Menu
├── Monitor Run
├── Control Run
├── Providers
├── New Run (deferred)
├── Config
└── Close Menu
```

Tests: `cd extensions/nexdev && npx tsc --noEmit` + manual smoke.

Acceptance: Ctrl+N opens menu. Nested navigate, Back, Escape all work without editor focus corruption.

---

### M5 + M6: Monitor Views + Steer (Parallel, Phase 4)

**M5 — PI-04b: Monitor Views + Control Actions**

Goal: Map all Bubbletea monitor views into Pi overlays. Wire pause/resume/skip/cancel/detour.

Owned: `extensions/nexdev/menu.ts` (monitor view renderers + action handlers), `extensions/nexdev/client.ts`

Implementation:

Monitor views:
- Overview → `GET /status`
- Events → live SSE or `GET /events`; bounded buffer ≤100; async, non-blocking; AbortController on close
- Plan/Tasks → `GET /plan` or `/status` task data; read-only
- Blockers → `/status` blocker data or dedicated endpoint
- Artifacts → `GET /artifacts`
- Missing endpoint → render `[DEFERRED: missing endpoint name]` state, not blank

Control actions:
- Pause/Resume: call `/pause` or `/resume` based on current status
- Skip Task: `ctx.ui.confirm()` + `/skip`
- Cancel Run: `ctx.ui.confirm()` + `/cancel`
- Detour: `/detour` with project/run/task context + reason flow
- All mutations: `ctx.ui.notify()` on success or redacted failure
- Do not expose as model-callable tools

Tests: `cd extensions/nexdev && npx tsc --noEmit` + control-plane client tests with mocked fetch + manual smoke against `nexdev serve` or fake-provider E2E.

Acceptance: All 5 monitor views render or show explicit deferred state. Events auto-refresh without blocking typing after overlay closes. Destructive actions require confirmation.

---

**M6 — PI-05: Steer Text Input**

Goal: Real multiline steer input in Pi replacing hardcoded placeholder.

Owned: `extensions/nexdev/steer.ts`, `extensions/nexdev/menu.ts` (steer submenu entry)

Implementation:
- Use `ctx.ui.editor("Steer Nexdev", "")` for multiline input.
- Reject empty or whitespace-only messages locally; show inline error.
- POST `{ "message": text, "source": "pi" }` to `/steer`.
- On success: `ctx.ui.notify("Steering message sent to Nexdev")`.
- On failure: redacted error notification.
- Do not use `ctx.ui.getEditorText()` — that captures the Pi coding prompt.

Tests: `cd extensions/nexdev && npx tsc --noEmit` + client tests for `/steer` payload shape.

Acceptance: Accepts multiline. Empty not sent. `/steer` receives exact message.

---

### M7: Go Pi Launcher (Phase 5, Step 1)

**PI-06: Go Launcher**

Goal: `nexdev` launches Pi by default; all existing modes preserved.

Owned: `internal/cli/pi.go`, `internal/cli/root.go`, `internal/cli/*_test.go`

Implementation:
- Detect `pi` binary via `exec.LookPath("pi")`.
- Return clear install/help error if missing (no panic).
- Add `--no-pi` flag or `nexdev tui` subcommand per Open Decisions §11.
- Preserve `--no-tui` for headless.
- Pass extension path and all env vars to Pi process.
- Inherit stdio; propagate Pi exit code.
- Do not weaken loopback bind/auth validation.

Tests:
- `go test ./internal/cli`
- Test missing Pi binary with controlled `PATH`
- Test env var construction without launching real Pi
- Test no-subcommand dispatch selects Pi launcher when interactive

Acceptance: `nexdev` launches Pi when installed. Missing Pi prints actionable guidance. Existing headless + Bubbletea paths still work.

---

### M8: Fallback + CLI Compatibility (Phase 5, Step 2)

**PI-07**

Goal: Prove Bubbletea fallback is intact after Go launcher changes.

Owned: `internal/cli` (fallback path), `internal/cli/*_test.go`

Tests: `go test ./internal/cli` + `go test ./internal/tui`

Acceptance: `nexdev tui` and `--no-pi` flag open Bubbletea. `nexdev run --no-tui` remains headless.

---

### M9: Packaging + Makefile Targets (Phase 5, Step 3)

**PI-08**

Goal: Extension available outside dev checkout. Makefile targets for extension lifecycle.

Owned: Go launcher files as needed, `extensions/nexdev/package.json`, release/check scripts, `Makefile`

Makefile targets to add:
```makefile
pi-ext-check:       ## Compile-check the Pi extension (npx tsc --noEmit)
pi-ext-build:       ## Build the Pi extension for distribution
pi-ext-clean:       ## Remove extension build artifacts
pi-ext-install-dev: ## Symlink extension to Pi dev extension path
```

Implementation:
- Source-path loading for dev checkout.
- Release builds embed extension source → write to Nexdev-controlled cache dir before launching Pi.
- Cache path under Nexdev state/cache, not project root.
- Validate path writes for traversal and symlink escape.
- Pin or document minimum tested Pi version.
- Keep Node/TS install requirements explicit in docs and `install.sh`.

Tests: `go test ./internal/cli` + `cd extensions/nexdev && npx tsc --noEmit` + release script dry-run.

Acceptance: Dev checkout works. Release packaging documented or implemented. Missing extension deps produce clear error.

---

### M10: Documentation (Phase 6, Parallel with M11 setup)

**PI-09**

Owned: `docs/architecture.md`, `docs/contracts.md`, `TESTING_STRATEGY.md`, `docs/SETUP.md` or `README.md`, `docs/TUI.md` (Pi-specific, new)

Implementation:
- Document Pi as default terminal surface.
- Document Bubbletea fallback and `nexdev tui` command.
- Document all env vars passed to Pi.
- Document extension compile command and Makefile targets.
- Document security behavior and token redaction.
- Document provider bridge deferral rationale.
- Update Pi TUI Parity Checklist (§5) with final status.
- Do not edit `SPEC.md` unless assigned through spec-management.

Acceptance: User can install Pi and understand `nexdev` launch. Developer can compile extension. Docs do not contradict `SPEC.md`.

---

### M11: Final Acceptance + Release Gates (Phase 6)

**PI-10**

Owned: Tests and scripts assigned by orchestrator. No broad implementation edits unless fixing assigned issues.

Required automated gates:
```sh
cd extensions/nexdev && npx tsc --noEmit
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
go mod verify
./scripts/e2e_fake_provider.sh
make pi-ext-check
```

Manual smoke matrix:

| Scenario | Expected result |
|---|---|
| `nexdev` with Pi installed | Pi opens in coding assistant mode |
| `nexdev` without Pi installed | Clear install/fallback message, no panic |
| Ctrl+N | Nexdev menu opens |
| Escape in menu | Menu closes, editor focus returns |
| Monitor Overview | Live status renders |
| Monitor Events | Events update without blocking typing |
| Monitor Plan/Tasks | Plan renders read-only |
| Monitor Blockers | Blockers render or deferred state shown |
| Monitor Artifacts | Artifacts render or deferred state shown |
| Control Pause/Resume | Control-plane updates, confirmation shown |
| Control Skip/Cancel | Confirmation required before mutation |
| Steer | Multiline text posts to `/steer`, empty blocked |
| Request Detour | `/detour` called, result shown |
| Providers → List | Provider list renders or deferred state |
| `nexdev tui` or `--no-pi` | Bubbletea fallback opens |
| `nexdev run --no-tui` | Headless mode works |
| Pi TUI Parity Checklist | All non-deferred items ✅ |

Acceptance: All automated gates pass or report exact environment blockers. Manual smoke evidence recorded. No secret leakage in logs, UI errors, or test artifacts. Pi TUI Parity Checklist has no unchecked non-deferred items.

---

## 8. Menu and Widget Specification

### 8.1 Persistent UI Elements

| Element | Pi API | Position | Update model | Content |
|---|---|---|---|---|
| Welcome banner | `ctx.ui.setWidget` | Above editor | Once on `session_start` | Project/run summary and Ctrl+N hint |
| Run status footer | `ctx.ui.setStatus` or stable footer API | Footer/status area | Poll `/status` every 2–5 seconds | Run ID, status, stage, task, cost |
| Ctrl+N hint | `ctx.ui.setWidget` below-editor | Below editor | Until first menu open | `Press Ctrl+N to open the Nexdev menu` |
| Event stream | `ctx.ui.custom` overlay | Overlay | SSE or polling, cancellable | Latest ≤100 bounded events |

### 8.2 Overlay Rules

- Use `ctx.ui.custom()` with `{ overlay: true }`.
- Use fresh component instances for submenu transitions.
- Keep explicit stack state for back navigation.
- Close overlays via Pi-provided completion/close handler.
- Do not retain disposed component references.
- Truncate lines to terminal width.
- Keep event buffers bounded (default 100).
- All async operations must use `AbortController`; abort on overlay close.

### 8.3 Control Menu Mapping

| Pi menu action | Bubbletea mapping | Control-plane endpoint |
|---|---|---|
| Monitor → Overview | `ViewOverview` | `GET /status` |
| Monitor → Events | `ViewEvents` | SSE or `GET /events` |
| Monitor → Plan/Tasks | `ViewPlan` | `GET /plan` or `/status` task data |
| Monitor → Blockers | `ViewBlockers` | `/status` blockers or dedicated |
| Monitor → Artifacts | `ViewArtifacts` | `GET /artifacts` |
| Control → Pause/Resume | key `p` | `POST /pause`, `POST /resume` |
| Control → Skip Task | key `k` | `POST /skip` + confirm |
| Control → Cancel Run | key `c` | `POST /cancel` + confirm |
| Control → Steer | key `s` | `POST /steer` + Pi editor |
| Control → Detour | key `d` | `POST /detour` |
| Providers → List | provider list | `GET /providers` or config |
| Config | config view | `GET /config` |

---

## 9. Provider Policy

First implementation must not expose Nexdev providers as Pi custom providers.

Rationale: Nexdev provider calls are stage-routed, audited, redacted, and cost-tracked through `internal/provider`. Pi custom providers are user-facing model-provider definitions for Pi's own loop. Bridging now risks duplicated secret handling and mismatched streaming/tool semantics.

Future provider bridge requirements (out of scope for M1):
- Spec-management review before exposing credentials.
- Explicit user opt-in.
- No raw API keys in Pi config unless approved.
- Redaction tests.
- Clear separation between Pi assistant provider use and Nexdev pipeline provider routing.

---

## 10. Testing Plan

### Per-task minimum checks

| Change type | Minimum check |
|---|---|
| Extension-only | `cd extensions/nexdev && npx tsc --noEmit` |
| Go launcher | `go test ./internal/cli` |
| Bubbletea fallback | `go test ./internal/tui` |
| Contract-affecting | `go test ./internal/contract` + docs update |
| Security/auth | relevant auth/redaction tests + `go test ./...` before handoff |
| Makefile target | `make pi-ext-check` dry-run |

### Final gates

```sh
cd extensions/nexdev && npx tsc --noEmit
make pi-ext-check
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
go mod verify
./scripts/e2e_fake_provider.sh
```

### Manual smoke matrix (see M11 for full matrix)

---

## 11. Open Decisions

Resolve before `PI-06` or `PI-08` starts.

| Decision | Options | Recommendation |
|---|---|---|
| Fallback flag name | `--no-pi`, `nexdev tui`, both | Support explicit `nexdev tui`; add `--no-pi` only if root launch needs a flag |
| Extension packaging | Source path, embedded copy, npm package | Source path first; embedded copy for release |
| Pi minimum version | Floating latest, pinned tested version | Pin/document tested version (KEYBINDING-SCOUT + PI-RESEARCH output required) |
| Ctrl+N fallback | None, config env, CLI flag | Start with Ctrl+N; add configurable fallback after conflict report |
| New Run menu | Fully enabled, partial, deferred | Partial/deferred until `/runs` UX is stable |
| Provider testing menu | Enabled, deferred | Deferred unless provider-test service is wired |
| TS test runner | None (compile-only), Vitest, Jest | Introduce Vitest for client + steer unit tests (recommended by PACKAGING-SCOUT) |

---

## 12. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Pi binary not installed | High | High | `exec.LookPath`, install guidance, Bubbletea fallback |
| Pi extension API changes | Medium | Medium | Pin/document tested Pi version, minimal API surface, compile in CI |
| TS extension packaging fails in release binary | Medium | High | Dev path first, then embed/copy to controlled cache with tests |
| Control-plane not running when Pi starts | High | Medium | Go launcher starts or attaches before spawning Pi |
| Auth token missing or wrong | Medium | High | Pass token via env, fail mutations safely, document setup |
| Token in error output | Medium | High | Centralize redaction in extension client and Go launcher logs |
| Ctrl+N conflicts with Pi/user keybindings | Medium | Medium | Document conflict, configurable fallback shortcut deferred |
| Nested overlay focus bugs | Medium | Medium | Explicit stack, fresh components, smoke test Back/Escape/focus |
| Event stream blocks Pi loop | Medium | High | Async stream/polling, bounded buffer, AbortController on close |
| Bubbletea fallback regresses | Low | Medium | `internal/tui` mostly untouched, fallback command tested |
| Provider credentials leak through Pi bridge | Medium | High | No bridge in first implementation |
| Non-loopback bind without auth | Low | High | Reuse existing config validation, fail startup |
| `/steer` sends empty placeholder | Medium | Medium | Pi editor input, reject empty, test payload |
| Missing control-plane endpoints for views | Medium | Medium | Explicit deferred state with endpoint name |
| Parallel subagents conflict on `menu.ts` | Medium | Medium | PI-04b owns monitor/action sections; PI-05 owns steer section; merge responsibility on PI-05 |
| Phase 0 scout produces incomplete output | Medium | Low | Partial output + UNRESOLVED markers; does not block other scouts |
| Manual smoke depends on local Pi install | High | Low | Separate compile/unit gates from manual smoke; record environment |

---

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
Pi TUI Parity Checklist items addressed:
Open risks:
Next recommended task:
Blocker-free: yes/no
```

If blocked, use the blocker handoff from `WORKER_PROTOCOL.md` and stop. Do not continue with guesses or hacks.

---

## 14. Recommended First Slice

Prove the architecture with the smallest useful behavior before investing in full nested menus:

1. Run all 5 Phase 0 scouts in parallel.
2. Run PI-01 + PI-01b in parallel to produce a compilable extension with stub types.
3. Make Go (PI-06 preview) launch Pi with the extension in a developer checkout.
4. Show the welcome banner.
5. Show the run status footer from `/status`.
6. Register Ctrl+N and open a top-level menu with disabled placeholder entries.
7. Preserve Bubbletea fallback and headless run behavior.

This slice validates Pi launch, extension loading, UI hooks, control-plane connectivity, and fallback safety before investing in full nested menus, monitor views, and event streaming. Every subsequent phase builds on a proven foundation.
