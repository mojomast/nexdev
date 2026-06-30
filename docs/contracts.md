# Nexdev Contracts Plan

**Status:** M1 first-wave OpenAPI, event, stage/status, state migration, artifact, model-output, provider router, executor, steering, detour, auth role, and test-fixture contracts exist.  
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

Authoritative migration location:
- `internal/state/migrations.go`
- Status: M1-C5 additive skeleton exists as migration version `4` using the imported geoffrussy migration runner. No full Nexdev repositories are implemented yet.

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

Additional M1 skeleton indexes:
- `idx_runs_project_id`
- `idx_stage_runs_run_stage`
- `idx_artifacts_project_kind`
- `idx_artifacts_run_kind`
- `idx_hivemind_results_run_voice`
- `idx_validate_results_run_id`
- `idx_steering_events_run_task`
- `idx_detour_records_run_trigger`
- `idx_navigation_events_project_created`
- `idx_plan_edit_events_run_created`

Migration policy:
- Preserve existing geoffrussy tables where possible.
- Additive migrations by default.
- Destructive migration requires explicit spec-management approval.

Current compatibility evidence:
- The migration runner remains the imported custom runner; goose/sqlc are not adopted in M1.
- Existing geoffrussy tables are preserved and Nexdev tables are added in migration version `4`.
- `Store.open` enables foreign keys and WAL and now configures `PRAGMA busy_timeout = 5000` before migrations run.

## 5. Stage Contract

Authoritative Go constants and interfaces:
- `internal/pipeline`
- Status: M1-C3 owns the canonical stage names, stage order, detour pseudo-stage, status constants, transition checks, prerequisite snapshot validation, and minimal `PipelineStage`/`StageEnv` interfaces. Durable runner/resumption remains M5 work.

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
- Risk: Provider Worker C6 reported `clarification-needed` because the imported geoffrussy `Provider` interface differs materially from `SPEC.md` section 11.1. M1 does not resolve that mismatch; spec-management/M4 must decide whether to add a wrapper/refactor or approve a contract correction.

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

## 8. Executor, Steering, and Detour Contracts

Executor contracts:
- `internal/executor/contracts.go` defines the M1 control boundary only.
- Required controls are `CurrentTask`, `Pause`, `Resume`, `Cancel`, `SkipTask`, and `SetSteeringContext` with `context.Context` where mutations can block or be cancelled.
- `TaskUpdateEventMapping` maps imported geoffrussy `TaskUpdate` values to Nexdev task event types with source `executor` and stage `develop`.
- `TaskReport` is an inert report shape for later M8 state/artifact integration; no execution behavior is implemented by M1-C7.

Steering contracts:
- `internal/steering/contracts.go` defines durable steering `Message`, selected prompt `Context`, source constants, and a minimal store interface.
- Accepted steering sources are `cli`, `api`, `tui`, and `mcp`.
- `SafetyPolicyOverrideAllowed` is false. Steering can add operator context but cannot override safety policy, output schema, or task acceptance criteria unless a later admin plan mutation changes the task contract.

Detour contracts:
- `internal/detour/contracts.go` aliases shared `internal/contract.DetourRequest` and `DetourResult` and uses `contract.TaskSpec` for new tasks.
- `RequestContext` captures current task, neighboring tasks, blocker, phase, design summary, repo context, and depth data for later M9 detour generation.
- `Generator`, `Splicer`, and `DepthPolicy` are compile-safe interfaces only. Provider-backed detour generation and durable plan mutation remain unimplemented.
- Default max depth is `3`; depth exhaustion must surface blocker reason `detour_depth_exceeded` and must not silently skip.

Implemented M1-C7 tests:
- `go test ./internal/executor` checks task update to event mapping and control interface compilation.
- `go test ./internal/steering` checks source constants, safety override prohibition, and store interface compilation.
- `go test ./internal/detour` checks shared detour types, splice/depth contracts, and existing imported detour behavior.

## 9. Config Contract

Authoritative M2 baseline:
- `internal/config/nexdev.go`
- Status: typed Nexdev defaults and validation exist beside the imported geoffrussy `Config` manager for compatibility. Full file/global/env/flag precedence wiring remains a later CLI/app integration task.

Default config file:
- `nexdev.yaml`

Implemented defaults:
- `profile: dev`.
- `project.state_dir: .nexdev`.
- `controlplane.enabled: true`, `bind: 127.0.0.1`, `port: 7432`, `auth_required: auto`, `token_env: NEXDEV_CONTROL_TOKEN`.
- `security.command_execution_default: deny`, `network_default: deny`, `tool_policy_file: .nexdev/tool_policy.yaml`, `reject_symlink_escape: true`.
- Repo analysis excludes `.git/**`, `node_modules/**`, `vendor/**`, `dist/**`, `build/**`, and `.nexdev/**`.
- Provider primary defaults to the spec placeholder and every required stage slot has an empty placeholder that downstream provider routing can inherit from primary.

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
- `auth_required: auto` resolves to false for loopback `dev`, true for non-loopback bind, `trusted-lan`, or `ci`.
- Unsafe CORS/profile validation remains a follow-up for M2 auth/security wiring.
- Shell command execution default is deny.
- Network access default is deny.

Path safety baseline:
- `internal/safety/paths.go`
- Cleans paths, resolves relative paths against the project root, rejects traversal and absolute paths outside root, rejects writes under `.git`, evaluates existing symlink ancestors for write paths, and supports basic deny globs such as `secrets/**` and `*.pem`.
- File locks and task expected-file enforcement remain later executor/policy integration work.

Security baseline contracts:
- `internal/safety/redaction.go` exposes `RedactSecrets(text string) string`. It deterministically replaces known secret forms with `[REDACTED]`, including provider API key shapes, bearer tokens, password/token/key assignments, private key blocks, SSH keys, and `.env` style secret assignments.
- `internal/safety/prompt_injection.go` exposes `DetectPromptInjection(text string) []PromptInjectionFinding`. Findings include a stable pattern name, message, and severity. Detection is warning-only; enforcement remains the caller's responsibility through events, review notes, or later pipeline policy.
- `internal/safety/policy.go` exposes `DefaultToolPolicy`, `ToolPolicy.Validate(profile)`, `AllowsShellCommand`, `AllowsNetwork`, and `ValidateWritePath`. The default policy allows read/write basics, denies shell and network, denies writes to `.git`, `.env`, private key, PEM, and key-looking files, and rejects wildcard shell allow rules in `trusted-lan` and `ci` profiles.
- No command execution or network tool implementation is provided by the M2 security baseline.

Remaining M2/M15 security gaps:
- Redaction is wired into the new `internal/observability` slog handler baseline, but app startup and provider/event/artifact/prompt/API boundaries do not yet route through it.
- Prompt-injection findings are not yet emitted as `security_warning` events or surfaced by repo analysis/review stages.
- Tool policy loading from `.nexdev/tool_policy.yaml`, executor/verify enforcement, output caps, controlled environments, file locks, task expected-file enforcement, MCP poisoning fixtures, and audit logs remain follow-up work.

Open decision:
- Define exact precedence between duplicate `develop.commit_on_*` and `git.commit_on_*` fields during config implementation.

## 10. Auth Contract

Authoritative skeleton:
- `internal/controlplane/auth.go`
- Status: M1-C8 role and route metadata helpers exist. No HTTP middleware, token repository, token generation, token hashing, or storage behavior is implemented by this skeleton.

Roles:
- `none` for unauthenticated routes such as `GET /health`.
- `observer`
- `operator`
- `admin`
- `per-tool` for `POST /mcp/call`, which delegates to the requested MCP tool's required role.

Role hierarchy:
- `admin` includes `operator`.
- `operator` includes `observer`.
- `observer` includes observer read routes only.
- `none` routes are allowed without auth.

Route role metadata:
- `internal/controlplane.RouteRoles` mirrors `api/openapi.yaml` `x-nexdev-role` metadata for every required route in `SPEC.md` section 12.2.
- `POST /mcp/call` is represented as `per-tool` and must not expand permissions; M11 resolves the requested tool role and passes it through the same role hierarchy.

Default token model:
- Opaque bearer tokens.
- Random 32+ bytes, base64url encoded.
- Store hash only.
- Store role, name, created time, expiry, revocation, last-used time.
- Constant-time compare.
- `TokenRecord` mirrors the migration version `4` `auth_tokens` schema at the contract level only.

Remote bind:
- Startup fails if bind is not loopback and auth is disabled.
- `RequireAuthForBind` provides the compile-safe contract helper for later config/startup integration.

Optional stateless tokens:
- Deferred unless explicitly required.

Implemented M1-C8 tests:
- `go test ./internal/controlplane` checks role hierarchy, required route coverage, mutating route role expectations, MCP per-tool delegation, and remote-bind auth requirement behavior.

## 11. MCP Contract

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

## 12. Artifact Contract

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

## 13. Observability Contract

Authoritative M2 logging baseline:
- `internal/observability/logger.go`
- Status: structured logging construction and redaction wrapper exist. OTel, metrics, runtime instrumentation, audit logs, and cost ledger behavior remain M14 follow-up work.

Logger behavior:
- Uses standard `log/slog`.
- Supports JSON and text handlers.
- Supports caller-provided slog level or defaults to info.
- Redacts messages and string attributes with `internal/safety.RedactSecrets` before write, including grouped attrs and attrs supplied through `Logger.With`.

Required field names:
- `project_id`
- `run_id`
- `stage`
- `task_id`
- `provider`
- `model`
- `event_id`
- `request_id`

Current deferrals:
- OpenTelemetry is disabled/not implemented in M2; `observability.OpenTelemetryEnabled` is false as a documented placeholder.
- Provider usage/cost persistence and audit logs require M3 state and M4 provider usage metadata before M14 implementation.

## 14. CLI Contract

Required commands are listed in `DEVPLAN.md` and `SPEC.md` section 18.

Rules:
- Local CLI uses in-process services.
- `--control-url` switches to HTTP client mode.
- `nexdev steer` maps to `POST /steer` semantics.
- `nexdev init --import-devussy PATH` is required by migration plan.

## 15. Contract Tests

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
- `go test ./internal/executor ./internal/detour ./internal/steering` validates M1-C7 executor, detour, and steering interface contracts while preserving imported package behavior.
- `go test ./internal/pipeline` validates stage/status/prerequisite contracts.
- `go test ./internal/state` validates migration version `4`, required table/index presence, seeded geoffrussy-compatible migration, event sequence uniqueness, foreign-key enforcement, WAL, and busy timeout.
- `go test ./internal/provider` validates router slot resolution, primary inheritance, and unknown slot/provider errors.
- `go test ./internal/controlplane` validates auth role hierarchy, route metadata coverage, MCP per-tool delegation, and remote-bind auth requirement behavior.
- `go test ./internal/testutil` validates M1 black-box fixture contracts for temp projects, deterministic time/IDs, event recording, and auth role fixtures.
- `go test ./internal/observability` validates M2 logging construction, redaction, level filtering, JSON/text modes, and required field helper keys.

## 16. Test Fixture Contract

Authoritative test-only package:
- `internal/testutil`
- Status: M1-C9 provides black-box fixtures for deterministic UTC time, stable fixture IDs, minimal safe temp projects, event recording over `internal/contract.EventEnvelope`, and auth role fixtures from `internal/controlplane` role constants.

Rules:
- Production code must not import `internal/testutil`.
- Fixture helpers should be extended only when an owning feature test needs them.
- Fake provider, fake worker, SSE replay client, golden-path helpers, and security fixture repos remain deferred; no fake-provider E2E script exists yet.
