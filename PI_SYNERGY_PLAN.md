# PI_SYNERGY_PLAN.md

> **Orchestrator target document.** Read this before dispatching any subagent.
> Source of truth precedence: `SPEC.md` > contracts > `DEVPLAN.md` > this file > implementation.
> Every subagent MUST read `AGENTS.md` and `SPEC.md` before touching code.

---

## Goal

Achieve **100% synergy** between Nexdev pipeline features and the Pi terminal extension so that:

1. Pi is the **primary interactive surface** for every Nexdev feature — from starting a run through monitoring, steering, and confirming handoff.
2. The **Pi menu system** (`extensions/nexdev/menu.ts`) has full coverage of every current and deferred Nexdev control-plane capability.
3. Pi can act as the **builder** — receiving orchestrator-issued prompts that drive end-to-end Nexdev pipeline execution from inside the Pi chat interface.
4. All currently deferred menu items become fully wired.
5. The extension emits rich, contextual chat messages to Pi so operators get narrated real-time progress without leaving the Pi chat.

---

## Current Gap Analysis

| Area | Current State | Gap |
|---|---|---|
| Menu: Provider Test | `[DEFERRED]` closes overlay | Not wired to `POST /providers/{name}/test` |
| Menu: Config Mutation | `[DEFERRED]` closes overlay | No `PUT /config` admin UX |
| Menu: Task detail drill-down | Plan view shows max 8 tasks, no per-task detail | No `GET /plan/tasks/{id}` view |
| Menu: Artifact open/inspect | Artifact list only, no content viewer | `nexdev artifacts open` deferred |
| Menu: Run history | Only active run shown | No `GET /runs` listing |
| Menu: Cost drill-down | Cost string in overview only | No `GET /cost` or cost history view |
| Chat narration | Pi is silent during pipeline execution | No SSE→chat bridge feeding Pi narrated progress |
| Builder mode | Pi can start runs but not act as autonomous build driver | No `NEXDEV_PI_BUILDER_MODE` flow |
| Stage visibility | Overview shows current stage but no stage history | No stage timeline view |
| Blocker resolution flow | Blockers shown as text | No inline resolve/comment UX |
| Keyboard shortcut coverage | `Ctrl+N` opens menu, `/nexdev` fallback | No `/nexdev run`, `/nexdev status`, `/nexdev steer` slash commands |
| Steer from chat | Steer opens editor then closes | No in-chat steer reply threading |
| Pi as run driver | Pi launches runs via `POST /runs` | Pi cannot orchestrate multi-step build sequences end-to-end |

---

## Parallel Subagent Task Graph

The tasks below are **independent** and MUST be executed in parallel by the orchestrator across separate subagent instances. Dependencies are noted; start dependency-free tasks first.

### TASK-PI-01 — Menu: Provider Test Screen (TS)

**Owner:** Pi Extension Subagent A 
**Files:** `extensions/nexdev/menu.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing 
**Goal:** Replace the `Provider Test (deferred)` menu entry with a real UX that:
- Lists providers from `GET /providers`.
- Allows selecting one and invoking `POST /providers/{name}/test`.
- Renders the test result (pass/fail, latency, error) inside a view panel.
- Falls back gracefully to `[DEFERRED: service not wired]` if the endpoint returns 503.

**Steps:**
1. Add `testProvider(name: string, signal?: AbortSignal): Promise<ProviderTestResult>` to `NexdevClient` in `client.ts`. Map `POST /providers/{name}/test`.
2. Add `ProviderTestResult` type to `types.ts` (fields: `name`, `ok`, `latency_ms?`, `error?`).
3. Add `MenuScreenID` `"provider-test"` and `ViewID` `"provider-test"` to `menu.ts`.
4. Wire `provider-test` screen with a two-step UX: first select provider (submenu of provider names), then confirm and show result view.
5. Render result in `renderProviderTest(result)` — show name, latency, ok/fail, truncated error.
6. Run `make pi-ext-check` and confirm TypeScript clean.

**Acceptance:** `make pi-ext-check` passes. Menu → Providers → Provider Test → select provider → shows test result or deferred message.

---

### TASK-PI-02 — Menu: Config Mutation Screen (TS)

**Owner:** Pi Extension Subagent B 
**Files:** `extensions/nexdev/menu.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing 
**Goal:** Replace the `Config Mutation (deferred)` entry with a real admin UX:
- Fetches current config via `GET /config`.
- Presents a list of mutable keys (non-secret fields returned by the endpoint).
- On select: opens `ctx.ui.editor` pre-filled with current value, lets operator edit, submits `PUT /config` with the changed key/value.
- Requires confirm dialog before submit.
- Shows redacted success or error notification.

**Steps:**
1. Add `updateConfig(patch: Record<string, unknown>, signal?: AbortSignal): Promise<NexdevConfig>` to `NexdevClient`.
2. Add a `"config-mutate"` submenu screen and `"config-mutate"` `ViewID` in `menu.ts`.
3. Render mutable key list (exclude anything matching the existing secret redaction regex).
4. On select → editor → confirm → `PUT /config` → notify result.
5. Handle 403 (not admin) gracefully with a redacted notify.
6. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. Menu → Config → Config Mutation → key list rendered, editor opens, submit path works or returns 503/403 notification.

---

### TASK-PI-03 — Menu: Task Detail Drill-Down (TS)

**Owner:** Pi Extension Subagent C 
**Files:** `extensions/nexdev/menu.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing 
**Goal:** From Plan view, allow drilling into a specific task to see full `TaskSpec` fields:
- All phases/tasks are navigable (not capped at 8).
- Selecting a task opens a detail `ViewState` showing: id, title, description, risk_level, status, assigned_files, expected_outputs, dependencies.

**Steps:**
1. Extend `renderPlan` to render all tasks (remove the `slice(0,8)` cap) and emit task IDs as selectable entries (using a new `"task-detail"` view flow).
2. Add `getPlanTask(taskId: string, signal?: AbortSignal): Promise<TaskSpec>` to `NexdevClient` (hits `GET /plan/tasks/{id}` if it exists, otherwise filters from `GET /plan`).
3. Add `ViewID` `"task-detail"` and a `currentTaskId` field on `NexdevMenuComponent` (private).
4. Render task detail via `renderTaskDetail(task: TaskSpec): string[]`.
5. Navigation: plan view → press Enter on a task row → task detail view → Back returns to plan.
6. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. Can navigate to any task and see full spec fields.

---

### TASK-PI-04 — Menu: Run History Screen (TS)

**Owner:** Pi Extension Subagent D 
**Files:** `extensions/nexdev/menu.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing 
**Goal:** Add a `Run History` entry under Monitor that lists past runs:
- Calls `GET /runs` (paginated if available, otherwise single page).
- Renders run_id, status, started_at, cost summary.
- Selecting a run opens a detail view with stage history if available from `GET /runs/{run_id}`.

**Steps:**
1. Add `RunListItem` and `RunListResponse` types to `types.ts`.
2. Add `listRuns(signal?: AbortSignal): Promise<RunListResponse>` and `getRun(runId: string, signal?: AbortSignal): Promise<RunSnapshot>` to `NexdevClient`.
3. Add `MenuScreenID` `"run-history"` and `ViewID` `"run-history"`, `"run-detail"` to `menu.ts`.
4. Wire `monitor` screen to include `{ label: "Run History", ..., action: "submenu", target: "run-history" }`.
5. Render up to 20 runs with status/cost; select → run detail view.
6. Handle 501/503 (endpoint not yet wired) with a deferred message.
7. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. Monitor → Run History shows list or deferred message.

---

### TASK-PI-05 — Menu: Cost Drill-Down View (TS)

**Owner:** Pi Extension Subagent E 
**Files:** `extensions/nexdev/menu.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing 
**Goal:** Add a `Cost Summary` view under Monitor:
- Calls `GET /cost` or reads cost records from `GET /status` metadata.
- Renders per-stage cost breakdown: stage name, provider, input_tokens, output_tokens, estimated_usd.
- Shows total at the bottom.

**Steps:**
1. Add `CostSummary` and `CostRecord` types to `types.ts`.
2. Add `getCost(signal?: AbortSignal): Promise<CostSummary>` to `NexdevClient`.
3. Add `ViewID` `"cost"` to `menu.ts` and wire a `Cost Summary` entry in the `monitor` screen.
4. Render with `renderCost(cost: CostSummary): string[]` — per-record rows + total.
5. Fallback: if `GET /cost` is 503, extract from `GET /status` metadata using existing `costFromMetadata` helper.
6. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. Monitor → Cost Summary shows breakdown or fallback.

---

### TASK-PI-06 — Menu: Stage Timeline View (TS)

**Owner:** Pi Extension Subagent F 
**Files:** `extensions/nexdev/menu.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing 
**Goal:** Add a `Stage Timeline` view under Monitor that renders a chronological list of pipeline stages:
- Reads `stages` array from `GET /status`.
- Renders each stage: name, status (pending/running/completed/failed), started_at, duration.
- Active stage is visually marked with `>`.

**Steps:**
1. Confirm `NexdevStatus.stages` type in `types.ts`; add `StageStatus` type if missing.
2. Add `ViewID` `"stages"` in `menu.ts`.
3. Add `{ label: "Stage Timeline", ..., view: "stages" }` entry in the `monitor` screen.
4. Implement `renderStages(status: NexdevStatus): string[]` — render stage rows, mark active.
5. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. Monitor → Stage Timeline renders stages from status.

---

### TASK-PI-07 — Menu: Blocker Resolution UX (TS)

**Owner:** Pi Extension Subagent G 
**Files:** `extensions/nexdev/menu.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing 
**Goal:** Upgrade the Blockers view from read-only text into an interactive resolution flow:
- List unresolved blockers as selectable entries.
- On select: show full blocker detail (id, reason, task_id, hypotheses, constraints).
- Action options: `Resolve` (calls `POST /blockers/{id}/resolve` or triggers a steer with blocker context), `Add Comment` (opens editor and calls `POST /blockers/{id}/comment` or steer), `Back`.

**Steps:**
1. Add `resolveBlocker(id: string, resolution: string, signal?: AbortSignal): Promise<void>` to `NexdevClient`.
2. Add `commentBlocker(id: string, comment: string, signal?: AbortSignal): Promise<void>` to `NexdevClient`.
3. Add `MenuScreenID` `"blocker-detail"` and `ViewID` `"blocker-detail"` in `menu.ts`.
4. Wire `monitor` blockers entry to navigate to `blocker-detail` screen (list of blockers as submenu entries).
5. On resolve/comment → confirm → call endpoint → notify result.
6. Handle 503 gracefully.
7. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. Monitor → Blockers → select blocker → Resolve / Add Comment functional.

---

### TASK-PI-08 — Slash Command Registration: /nexdev (TS + index.ts)

**Owner:** Pi Extension Subagent H 
**Files:** `extensions/nexdev/index.ts`, `extensions/nexdev/menu.ts`, `extensions/nexdev/client.ts` 
**Depends on:** nothing 
**Goal:** Register Pi slash commands for direct Nexdev access without opening the full menu overlay:
- `/nexdev` — opens the full menu (existing behavior via `Ctrl+N`, now also a slash command).
- `/nexdev status` — posts a status summary directly into the Pi chat as a message.
- `/nexdev run <prompt>` — starts a new run with the inline prompt text.
- `/nexdev steer <message>` — posts a steer request inline without opening a modal.
- `/nexdev pause` and `/nexdev resume` — direct control.

**Steps:**
1. In `index.ts`, after registering the existing `Ctrl+N` keybinding and `/nexdev` command, register the subcommand variants using Pi's `ctx.commands.register` or equivalent API.
2. Each subcommand calls the appropriate `NexdevClient` method and posts the result back to Pi chat via `ctx.chat.send` or equivalent.
3. `/nexdev run` should invoke the same `client.startRun()` path as `newRun` control action.
4. `/nexdev steer` should use the same `steerNexdev` flow from `steer.ts`.
5. Format all chat output as readable markdown (status tables, run summaries, etc.).
6. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. All slash commands discoverable and functional.

---

### TASK-PI-09 — SSE→Chat Bridge: Real-Time Narration (TS)

**Owner:** Pi Extension Subagent I 
**Files:** `extensions/nexdev/index.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Depends on:** nothing (can run after TASK-PI-08 lands to avoid index.ts conflict, coordinate with orchestrator) 
**Goal:** When a Nexdev run is active, stream narrated progress messages from SSE into the Pi chat window:
- On extension load, if `NEXDEV_RUN_ID` env is set, attach to `GET /runs/{run_id}/stream` SSE.
- On each significant SSE event (stage_start, stage_complete, task_start, task_complete, blocker_created, run_complete, run_failed), post a formatted markdown summary to Pi chat.
- Use the existing `client.streamEventsUrl()` helper to build the SSE URL.
- The bridge must be abort-safe and must not re-post duplicate events.
- Bridge must be silent when no run is active.

**Steps:**
1. Add `streamEvents(runId: string, onEvent: (e: EventEnvelope) => void, signal: AbortSignal): Promise<void>` to `NexdevClient` using `EventSource` or `fetch`+`ReadableStream` for SSE.
2. In `index.ts`, after extension init, check `NEXDEV_RUN_ID`; if set, start the bridge.
3. Map each significant event type to a human-readable markdown message (e.g. `**Stage started:** repo-analysis`).
4. Post via `ctx.chat.send()` or equivalent Pi API.
5. Deduplicate by event sequence number.
6. Run `make pi-ext-check`.

**Acceptance:** `make pi-ext-check` passes. Running `nexdev run` with Pi open causes live narration in Pi chat.

---

### TASK-PI-10 — Builder Mode: Pi as End-to-End Build Driver (TS + Go)

**Owner:** Pi Extension Subagent J + Go Subagent K (coordinate via orchestrator) 
**Files (TS):** `extensions/nexdev/index.ts`, `extensions/nexdev/client.ts`, `extensions/nexdev/types.ts` 
**Files (Go):** `internal/cli/root.go` or new `internal/cli/builder.go`, `internal/controlplane/` (new `/builder-session` endpoint if needed) 
**Depends on:** TASK-PI-08 slash commands should land first for the chat UX layer 
**Goal:** Enable Pi to act as the autonomous build driver via a structured conversation loop:
- New env var: `NEXDEV_PI_BUILDER_MODE=1`.
- When set, the extension registers a system prompt context block that instructs Pi to:
  1. Ask the operator for the build goal if no `NEXDEV_RUN_ID` is set.
  2. Start a run via `/nexdev run <goal>`.
  3. Monitor SSE events and narrate progress.
  4. When a blocker appears, Pi surfaces it as a chat question and waits for operator input.
  5. Operator replies in chat → Pi issues a `/nexdev steer <reply>` or blocker resolve action.
  6. On `run_complete`, Pi posts a handoff summary with artifact list and cost.
- The Go side adds a `--builder` flag to `nexdev` that sets `NEXDEV_PI_BUILDER_MODE=1` before launching Pi.

**Steps (TS):**
1. In `index.ts`, check `NEXDEV_PI_BUILDER_MODE`; if set, call `ctx.system.addContext()` or equivalent with the builder system prompt.
2. The builder system prompt is a static, non-model-callable string that instructs Pi's LLM behavior (not a tool).
3. Wire the blocker-surfacing logic: on `blocker_created` SSE event, post a formatted chat message with blocker details and wait for Pi to handle operator reply.

**Steps (Go):**
1. In `internal/cli/root.go` (or a new `builder.go` in `internal/cli`), add a `--builder` flag to the root command.
2. When `--builder` is set, set `NEXDEV_PI_BUILDER_MODE=1` in the Pi launcher env.
3. Optionally add a `nexdev builder` subcommand as a convenient alias.
4. Add `go test ./internal/cli` coverage for the new flag.

**Acceptance:** `make pi-ext-check` passes. `go test ./internal/cli` passes. `nexdev --builder` launches Pi with builder mode active.

---

### TASK-PI-11 — Menu Coverage Audit + Footer Help Text (TS)

**Owner:** Pi Extension Subagent L 
**Files:** `extensions/nexdev/menu.ts` 
**Depends on:** Should run AFTER all other TASK-PI-0x tasks are merged (final integration pass) 
**Goal:** Final coverage pass:
- Audit every `[DEFERRED]` label in `menu.ts` — confirm each is now wired or explicitly documented as pending a Go-side endpoint.
- Update all menu `message` strings to reflect current (non-deferred) capability.
- Update footer help text to mention slash commands (`/nexdev status`, `/nexdev run`).
- Ensure every `MenuScreenID` has a `Back` or `Close` entry.
- Ensure `screens` record is fully typed with no implicit `any`.

**Acceptance:** `make pi-ext-check` passes. Zero `[DEFERRED]` labels remain for tasks completed in this plan. Any remaining deferrals reference a specific TASK ID or Go-side blocker.

---

### TASK-PI-12 — Go: POST /providers/{name}/test Endpoint (Go)

**Owner:** Go Subagent M 
**Files:** `internal/controlplane/`, `internal/provider/`, `api/openapi.yaml`, `api/generated/nexdev_api.gen.go` 
**Depends on:** nothing 
**Goal:** Implement `POST /providers/{name}/test` in the control-plane so TASK-PI-01 has a real endpoint to call:
- Route is operator-role gated.
- Calls a `TestProvider(name string) (latency time.Duration, err error)` method on the provider registry/router.
- Returns `{ name, ok, latency_ms, error }` JSON.
- Returns 503 if provider service not injected.
- Add/update `api/openapi.yaml` entry and regenerate types with `make generate`.

**Steps:**
1. Add `TestProvider` method to `internal/provider` interface and fake/real implementations.
2. Add route handler in `internal/controlplane`.
3. Update `api/openapi.yaml` and run `make generate`.
4. Add `go test ./internal/controlplane` and `go test ./internal/provider` coverage.
5. Run full gate: `go test ./...`, `go vet ./...`.

**Acceptance:** `go test ./...` and `go vet ./...` pass. `curl -X POST http://127.0.0.1:7432/providers/fake/test` returns JSON.

---

### TASK-PI-13 — Go: GET /runs + GET /runs/{id} + GET /cost Endpoints (Go)

**Owner:** Go Subagent N 
**Files:** `internal/controlplane/`, `internal/state/`, `api/openapi.yaml`, `api/generated/nexdev_api.gen.go` 
**Depends on:** nothing 
**Goal:** Implement the three endpoints needed by TASK-PI-04 and TASK-PI-05:
- `GET /runs` — returns paginated list of `RunSnapshot` records from SQLite.
- `GET /runs/{run_id}` — returns a single `RunSnapshot` with stage history.
- `GET /cost` — returns `CostSummary` with per-stage cost records from the cost table.

**Steps:**
1. Add `ListRuns(limit, offset int) ([]RunSnapshot, error)` and `GetRunByID(id string) (RunSnapshot, error)` to `internal/state` repositories.
2. Add `GetCostSummary(runID string) (CostSummary, error)` to `internal/state`.
3. Add route handlers in `internal/controlplane` for all three endpoints (operator role).
4. Update `api/openapi.yaml` and run `make generate`.
5. Add tests in `internal/state` and `internal/controlplane`.
6. Run: `go test ./...`, `go vet ./...`.

**Acceptance:** `go test ./...` passes. All three endpoints return JSON.

---

### TASK-PI-14 — Go: PUT /config Admin Endpoint (Go)

**Owner:** Go Subagent O 
**Files:** `internal/controlplane/`, `internal/app/`, `api/openapi.yaml` 
**Depends on:** nothing 
**Goal:** Implement `PUT /config` for admin-role config mutation:
- Accepts a partial config JSON body.
- Validates fields against allowed mutable keys (non-secret, documented in spec).
- Applies changes to the running config and optionally persists.
- Returns the updated redacted config.
- Returns 403 for non-admin callers.

**Steps:**
1. Define `MutableConfigKeys` allowlist in `internal/app` or `internal/config`.
2. Add `UpdateConfig(patch map[string]interface{}) error` to config manager.
3. Add route handler in `internal/controlplane` (admin role gate).
4. Update `api/openapi.yaml` and run `make generate`.
5. Add tests.
6. Run: `go test ./...`, `go vet ./...`.

**Acceptance:** `go test ./...` passes. `PUT /config` returns updated config for admin role, 403 for operator.

---

## Execution Order for Orchestrator

```
Parallel batch 1 (all independent, start simultaneously):
  TASK-PI-01, TASK-PI-02, TASK-PI-03, TASK-PI-04,
  TASK-PI-05, TASK-PI-06, TASK-PI-07, TASK-PI-08,
  TASK-PI-09, TASK-PI-10, TASK-PI-12, TASK-PI-13, TASK-PI-14

Serial after batch 1 completes:
  TASK-PI-11 (audit + cleanup pass)
```

> Note: TASK-PI-09 (SSE narration) and TASK-PI-08 (slash commands) both touch `index.ts`.
> Orchestrator must sequence these two to avoid merge conflicts or assign both to the same subagent.
> All other tasks have non-overlapping file ownership.

---

## Required Gates Per Task

Every TS subagent must run before reporting complete:
```
npm --prefix extensions/nexdev run check
# or
make pi-ext-check
```

Every Go subagent must run before reporting complete:
```
go test ./...
go test -race ./...
go vet ./...
go mod verify
./scripts/e2e_fake_provider.sh
```

---

## Pi Builder Mode: How It Works End-to-End

When `nexdev --builder` is invoked:

1. Go launcher sets `NEXDEV_PI_BUILDER_MODE=1` and starts Pi with the Nexdev extension.
2. Pi extension detects builder mode and injects a system context block into the Pi LLM session.
3. Pi asks the operator in chat: `"What do you want Nexdev to build?"` (if no active run).
4. Operator types their goal in the Pi chat window.
5. Pi LLM (guided by system context) issues `/nexdev run <goal>` slash command → calls `POST /runs`.
6. SSE bridge (TASK-PI-09) attaches and narrates progress into Pi chat.
7. On `blocker_created` event, Pi surfaces the blocker as a chat thread and prompts operator.
8. Operator replies → Pi issues `/nexdev steer <reply>` or resolves the blocker via TASK-PI-07 UX.
9. On `run_complete`, Pi posts a final handoff summary with artifact paths, cost, and next steps.
10. Operator is always in control; Pi is the interface layer, not an autonomous executor.

---

## Completion Criteria (100% Synergy)

- [ ] All `[DEFERRED]` labels in `menu.ts` are resolved or reference a named Go-side blocker task.
- [ ] Every Nexdev control-plane endpoint (`GET/POST/PUT`) has a corresponding Pi menu entry or slash command.
- [ ] `nexdev --builder` launches Pi in builder mode.
- [ ] SSE events are narrated live in Pi chat during runs.
- [ ] All Pi extension TypeScript compiles clean: `make pi-ext-check`.
- [ ] All Go changes pass: `go test ./...`, `go test -race ./...`, `go vet ./...`, `./scripts/e2e_fake_provider.sh`.
- [ ] TASK-PI-11 audit confirms zero unexplained deferrals.

---

## Subagent Completion Report Template

Every subagent must return:

```
Task ID:
Summary:
Files changed:
Tests added/changed:
Tests run:
Tests skipped (with reason):
Docs updated:
Spec impact:
Open risks:
Next recommended task:
Blocker-free: yes/no
```
