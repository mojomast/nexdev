# Nexdev Contracts Plan

**Status:** M1 first-wave OpenAPI, event, artifact, and model-output contracts exist.  
**Canonical source:** `SPEC.md`.  
**Rule:** Once created, `api/openapi.yaml`, migrations, generated API types, and schema files are contract artifacts and must stay synchronized with this document.

## 1. Contract Authority

Precedence:

1. `SPEC.md`
2. Machine contracts: `api/openapi.yaml`, migrations, generated types, schemas
3. This document
4. `DEVPLAN.md`
5. Tests and implementation

If this document contradicts `SPEC.md`, `SPEC.md` wins until spec-management updates the spec.

## 2. OpenAPI Contract

Authoritative file:
- `api/openapi.yaml`
- Status: M1 skeleton exists for every required route in `SPEC.md` section 12.2. No HTTP handlers are implemented by this contract task.

Required version:
- `nexdev-api-v1`

Required common error shape:

```json
{
  "error_code": "string",
  "message": "string",
  "details": {},
  "request_id": "string"
}
```

Required routes:
- `GET /health`
- `GET /status`
- `GET /plan`
- `GET /artifacts`
- `GET /events`
- `GET /runs/{run_id}/stream`
- `POST /runs`
- `POST /pause`
- `POST /resume`
- `POST /skip`
- `POST /steer`
- `POST /detour`
- `POST /cancel`
- `PUT /tasks/{task_id}`
- `DELETE /tasks/{task_id}`
- `POST /blockers/{blocker_id}/resolve`
- `GET /config`
- `PUT /config`
- `GET /providers`
- `POST /providers/{name}/test`
- `GET /mcp/tools`
- `POST /mcp/call`

Role mapping:
- No auth: `GET /health`.
- Observer: read-only status, plan, artifacts, events, streams, config, providers, MCP tools.
- Operator: observer plus run start, pause, resume, skip, steer, detour, blocker resolve, provider test, operator MCP tools.
- Admin: operator plus cancel, task mutation, config mutation, token management, destructive operations.

OpenAPI role metadata:
- Every operation includes `x-nexdev-role` with `none`, `observer`, `operator`, `admin`, or `per-tool` for `POST /mcp/call`.

Current codegen status:
- Generated API code is intentionally deferred until the codegen tool path is settled for M1 integration.
- Contract tests currently parse `api/openapi.yaml` with `gopkg.in/yaml.v3` and verify route/role coverage and common schema presence.

## 3. Event Contract

Required event contract version:
- `nexdev-event-v1`

Authoritative Go constants and envelope:
- `internal/contract/events.go`
- Status: event type constants, required source constants, and `EventEnvelope` are defined in an inert contract package.

Required envelope fields:

```go
type EventEnvelope struct {
    EventID         string          `json:"event_id"`
    Sequence        int64           `json:"sequence"`
    ContractVersion string          `json:"contract_version"`
    Type            string          `json:"type"`
    ProjectID       string          `json:"project_id"`
    RunID           string          `json:"run_id"`
    Stage           string          `json:"stage,omitempty"`
    TaskID          string          `json:"task_id,omitempty"`
    Timestamp       time.Time       `json:"ts"`
    Source          string          `json:"source"`
    Payload         json.RawMessage `json:"payload"`
}
```

Required event types:
- `heartbeat`
- `run_started`
- `run_status`
- `stage_transition`
- `stage_status`
- `content_delta`
- `provider_call_started`
- `provider_call_completed`
- `provider_call_failed`
- `artifact_updated`
- `plan_updated`
- `review_required`
- `review_completed`
- `task_started`
- `task_progress`
- `task_completed`
- `task_error`
- `task_blocked`
- `task_paused`
- `task_resumed`
- `task_skipped`
- `steering_added`
- `detour_requested`
- `detour_created`
- `detour_failed`
- `blocker_created`
- `blocker_resolved`
- `verify_started`
- `verify_command_output`
- `verify_completed`
- `git_event`
- `cost_update`
- `security_warning`
- `pipeline_error`
- `done`

SSE frame:

```text
id: <event_id>
event: <type>
retry: 3000
data: {EventEnvelope JSON}

```

Rules:
- Persist before broadcast.
- Sequence is monotonic per run.
- Support `Last-Event-ID`.
- Send heartbeat.
- Bound client queues.
- Close slow clients after emitting `sse_client_slow` if possible.
- Do not use `data: [DONE]`.

## 4. State Contract

Required SQLite behavior:
- Foreign keys on.
- WAL enabled.
- Busy timeout configured.
- Bounded transactions.
- Retry handling for busy writes.
- Consistent UTC timestamp representation.

Required tables:
- `runs`
- `stage_runs`
- `events`
- `artifacts`
- `hivemind_results`
- `validate_results`
- `steering_events`
- `detour_records`
- `navigation_events`
- `plan_edit_events`
- `auth_tokens`

Required event indexes:
- Unique `(run_id, sequence)`.
- Index by `(run_id, sequence)`.
- Index by `(run_id, type)`.

Migration policy:
- Preserve existing geoffrussy tables where possible.
- Additive migrations by default.
- Destructive migration requires explicit spec-management approval.

## 5. Stage Contract

Canonical stage order:

```text
init -> repo_analyze -> interview -> complexity -> design -> hivemind -> validate -> plan_sketch -> plan_detail -> review -> develop -> verify -> handoff -> complete
```

Pseudo-stage:

```text
develop -> detour -> develop
```

Required statuses:
- `pending`
- `running`
- `completed`
- `skipped`
- `blocked`
- `failed`
- `cancelled`

Required transitions:
- `pending -> running`
- `pending -> skipped`
- `running -> completed`
- `running -> blocked`
- `running -> failed`
- `running -> cancelled`
- `blocked -> running`
- `failed -> running`

## 6. Structured Model Output Contracts

Required schema-bearing outputs:
- `RepoAnalysis`
- `InterviewData`
- `ComplexityProfile`
- `Finding`
- `HivemindCritique`
- `HivemindSynthesis`
- `ValidationReport`
- `PhaseSketch`
- `TaskSpec`
- `DetourRequest`
- `DetourResult`
- `VerifyReport`
- `CommandResult`
- `ChangedFile`
- `RunSummary`

Rules:
- Strict decode when configured.
- Reject unknown fields where feasible.
- Validate semantic constraints.
- Repair at most configured attempts.
- Persist raw responses and validation errors when configured.

Authoritative first-wave Go structs:
- `internal/contract/model_outputs.go`
- Status: inert structs exist for required structured outputs and are intentionally free of provider calls or pipeline business logic.

## 7. Provider Contract

Provider boundary:
- Stages must use provider router/wrapper.
- No stage calls concrete providers directly.
- Current M1 router contract is implemented in `internal/provider/router.go`.
- The imported geoffrussy `Provider` interface and registry remain the concrete provider boundary for M1.

Provider slots:
- `interview`
- `complexity`
- `design`
- `hivemind_voice`
- `hivemind_synthesis`
- `validate`
- `plan_sketch`
- `plan_detail`
- `review`
- `develop`
- `verify_repair`
- `handoff`

Empty slot inherits `provider.primary`.

Router behavior:
- `provider.NewRouter` validates primary and slot provider names against the package registry.
- `provider.NewRouterWithRegistry` supports tests and app wiring with an explicit registry.
- `Router.Resolve(slot)` returns the resolved provider/model route for a known slot.
- Unknown slots and unknown provider names return errors.
- Model-only slot overrides inherit the primary provider name.

Optional provider capabilities:
- Structured calls.
- Usage metadata.

Current deferrals:
- Structured output wrapper behavior remains M4 follow-up.
- Usage/cost metadata integration remains M4/M14 follow-up.
- Deterministic fake provider scripting remains M4 follow-up.

## 8. Config Contract

Default config file:
- `nexdev.yaml`

Required precedence, lowest to highest:
1. Built-in defaults.
2. Global `~/.nexdev/config.yaml` if present.
3. Project `nexdev.yaml`.
4. `.env` values if configured.
5. Process environment.
6. CLI flags.
7. Safe request-scoped overrides.

Config validation:
- Reject unknown top-level keys unless `experimental.allow_unknown_config` is true.
- Reject non-loopback bind without auth.
- Reject unsafe CORS/profile combinations.
- Shell command execution default is deny.

Open decision:
- Define exact precedence between duplicate `develop.commit_on_*` and `git.commit_on_*` fields during config implementation.

## 9. Auth Contract

Roles:
- `observer`
- `operator`
- `admin`

Default token model:
- Opaque bearer tokens.
- Random 32+ bytes, base64url encoded.
- Store hash only.
- Store role, name, created time, expiry, revocation, last-used time.
- Constant-time compare.

Remote bind:
- Startup fails if bind is not loopback and auth is disabled.

Optional stateless tokens:
- Deferred unless explicitly required.

## 10. MCP Contract

Required tools:
- `nexdev_start_run`
- `nexdev_get_status`
- `nexdev_get_plan`
- `nexdev_list_artifacts`
- `nexdev_get_artifact`
- `nexdev_pause`
- `nexdev_resume`
- `nexdev_cancel`
- `nexdev_steer`
- `nexdev_detour`
- `nexdev_resolve_blocker`
- `nexdev_provider_test`

Rules:
- MCP tools are thin wrappers around control-plane services.
- Tool descriptions are static and generated from checked-in schema.
- Input schemas match OpenAPI where possible.
- Same role checks as HTTP.
- MCP stdio mode must not execute arbitrary shell strings.

## 11. Artifact Contract

Required project-local artifacts:
- `.nexdev/artifacts/interview.json`
- `.nexdev/artifacts/repo_analysis.json`
- `.nexdev/artifacts/complexity_profile.json`
- `.nexdev/artifacts/design_draft.md`
- `.nexdev/artifacts/design_review.json`
- `.nexdev/artifacts/validated_design.md`
- `.nexdev/artifacts/validation_report.json`
- `.nexdev/artifacts/devplan.json`
- `.nexdev/artifacts/devplan.md`
- `.nexdev/artifacts/phase001.md`
- `.nexdev/artifacts/handoff.md`
- `.nexdev/artifacts/verify_report.json`
- `.nexdev/artifacts/changed_files.json`
- `.nexdev/artifacts/run_summary.json`

Artifact index fields:
- ID.
- Project ID.
- Run ID.
- Kind.
- Path.
- SHA256.
- Version.
- Metadata.
- Created time.
- Updated time.

Authoritative first-wave Go structs:
- `internal/contract/artifacts.go`
- Status: artifact kind constants, manifest/item structs, changed-file manifest, run summary, stage summary, and provider usage structs exist for downstream M1 workers.

## 12. CLI Contract

Required commands are listed in `DEVPLAN.md` and `SPEC.md` section 18.

Rules:
- Local CLI uses in-process services.
- `--control-url` switches to HTTP client mode.
- `nexdev steer` maps to `POST /steer` semantics.
- `nexdev init --import-devussy PATH` is required by migration plan.

## 13. Contract Tests

Required contract tests:
- OpenAPI validation and generated code compile.
- ErrorResponse golden tests.
- Event envelope golden tests.
- SSE frame tests.
- Stage graph and status transition tests.
- State migration tests.
- Structured output strictness tests.
- Plan DAG validation tests.
- Auth role matrix tests.
- MCP schema/permission tests.
- Artifact manifest tests.

Implemented first-wave tests:
- `go test ./internal/contract` validates OpenAPI route/role coverage, required schema names, event contract version, event type constants, and event source constants.
