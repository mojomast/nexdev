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

## 1.1 Runtime Path Contract

Project-local runtime lock:
- Path: `.nexdev/run/project.lock` under the project root.
- Owner: `internal/git` helper functions.
- Acquisition: create parent directories, then create the lock file atomically with exclusive-create semantics.
- Metadata: best-effort pid and UTC acquisition timestamp text.
- Release: close and remove the lock file.
- Contention: an existing lock file is reported as held; stale lock recovery is deferred to security hardening.

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
- Status: M1-C5 additive skeleton exists as migration version `4` using the imported geoffrussy migration runner. M3 repositories now cover events, runs, stage runs, artifacts, auth tokens, steering events, detour records, navigation events, and plan edit events. The M3 task/blocker follow-up adds migration version `5` for Nexdev-specific task and blocker tables while preserving legacy geoffrussy `tasks` and `blockers`. M14 adds migration version `6` for audit and cost ledger records.

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
- `nexdev_tasks`
- `nexdev_blockers`
- `audit_log`
- `cost_ledger`

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

Task/blocker follow-up indexes:
- `idx_nexdev_tasks_run_order`
- `idx_nexdev_tasks_run_status`
- `idx_nexdev_tasks_phase`
- `idx_nexdev_blockers_run_status`
- `idx_nexdev_blockers_task`
- `idx_audit_log_project_created`
- `idx_audit_log_run_created`
- `idx_audit_log_action`
- `idx_cost_ledger_run_created`
- `idx_cost_ledger_provider_model`

Migration policy:
- Preserve existing geoffrussy tables where possible.
- Additive migrations by default.
- Destructive migration requires explicit spec-management approval.

Current compatibility evidence:
- The migration runner remains the imported custom runner; goose/sqlc are not adopted in M1.
- Existing geoffrussy tables are preserved and Nexdev tables are added in migration version `4`.
- Migration version `5` keeps legacy geoffrussy `tasks` and `blockers` intact and adds `nexdev_tasks` plus `nexdev_blockers` instead of adapting incompatible legacy columns.
- Migration version `6` adds `audit_log` and `cost_ledger` without changing existing state tables.
- `Store.open` enables foreign keys and WAL and now configures `PRAGMA busy_timeout = 5000` before migrations run.

Event repository behavior:
- `Store.PersistEvent` accepts `contract.EventEnvelope`, requires caller-provided `event_id`, `run_id`, `type`, and `source`, validates JSON payload, stores empty payloads as `{}`, recursively redacts JSON string values before insert, and writes to the version `4` `events` table.
- If `Sequence` is zero, the store allocates the next per-run sequence inside a retrying transaction after locking the parent run row. If `Sequence` is provided, it must equal the next sequence for that run; duplicate, stale, or gap sequences return `ErrEventSequenceConflict`.
- Events are loaded by joining `events` to `runs` so returned envelopes include `ProjectID`. Returned envelopes always set `ContractVersion` to `contract.EventContractVersion`.
- Event timestamps are stored as UTC RFC3339Nano text in `created_at` and returned as UTC `time.Time` values.
- `Store.ListEvents` supports replay by `run_id` after a sequence, after `Last-Event-ID` through `AfterEventID`, or both. `Store.EventSequenceForID` provides the explicit `Last-Event-ID` to sequence mapping for later SSE handlers.
- `Limit` is optional and intended for later control-plane replay caps such as `controlplane.sse.replay_max_events`.

M10 SSE follow-ups:
- Broadcast only the envelope returned from `PersistEvent`, not the caller's pre-persist envelope, so subscribers see the durable sequence and UTC timestamp.
- Map HTTP `Last-Event-ID` to `EventListOptions.AfterEventID`, enforce configured replay limits, and handle `ErrEventNotFound` as the control-plane replay policy defines.
- Keep heartbeat, per-client queues, slow-client closure, and SSE frame formatting in `internal/controlplane`; the state repository only owns durable event persistence and replay queries.

Auxiliary state repository behavior:
- `Store.CreateAuthToken`, `GetAuthToken`, `GetAuthTokenByHash`, `ListAuthTokens`, `RevokeAuthToken`, and `TouchAuthTokenLastUsed` persist only `token_hash` plus role/name/created/expires/revoked/last-used metadata. Token generation, hashing, expiry authorization, and constant-time comparison remain control-plane/auth behavior.
- `Store.AppendSteeringEvent` and `ListSteeringEvents` preserve message, summary, source, created role, task scope, and UTC timestamp. Prompt context selection, summarization, and safety non-override enforcement remain M8 executor/steering work.
- `Store.CreateDetourRecord` and `ListDetourRecords` preserve trigger task, reason, source, depth, result JSON, and UTC timestamp. Provider-backed detour generation, depth policy enforcement, blocker creation, and plan splicing remain M9 detour work.
- `Store.AppendNavigationEvent` and `ListNavigationEvents` preserve project/run scope, from/to stages, reason, actor, and UTC timestamp. Stage prerequisite enforcement and runner navigation decisions remain M5/M10 service behavior.
- `Store.CreatePlanEditEvent` and `ListPlanEditEventsByRun` preserve plan version before/after, edit type, target, patch JSON, actor, and UTC timestamp. Review editing, plan mutation, version increments, and `plan_updated` event emission remain M7/M10 work.
- Repository list methods return deterministic ascending order by persisted UTC timestamp and ID. JSON fields are validated before write and round-tripped as raw JSON bytes.

Task and blocker repository behavior:
- `Store.CreateNexdevTask`, `GetNexdevTask`, `ListNexdevTasks`, and `UpdateNexdevTaskStatus` persist `contract.TaskSpec` fields needed by planning, review, execution, and detour work: ID, phase ID, title, description, expected files, dependencies, acceptance criteria, test commands, risk level, required tools, and notes.
- Task rows add `project_id`, `run_id`, `status`, `plan_version`, `plan_order`, `created_at`, and `updated_at` for stable per-run listing. Lists order by `plan_version`, `plan_order`, then task ID.
- Task creation validates required IDs/title/phase, requires acceptance criteria, stores slice fields as JSON arrays, rejects self-dependencies, and checks dependency IDs already exist in the same run. Full DAG cycle validation remains M7 planning/review behavior.
- `Store.CreateNexdevBlocker`, `ListNexdevBlockers`, and `ResolveNexdevBlocker` persist blocker ID, project/run/task scope, reason, description, status, resolution, metadata JSON, created timestamp, and resolved timestamp. Blocker lists order by `created_at` then ID and can filter by run, task, and status.
- Blocker task scope references `nexdev_tasks`, not legacy geoffrussy `tasks`. Project/run foreign keys enforce valid Nexdev scope while permitting run-level blockers with no task ID.

Audit and cost repository behavior:
- `Store.CreateAuditRecord` and `ListAuditRecords` persist durable audit rows for security/control-plane/operator-sensitive actions. Required fields are ID, project ID, source, action, outcome, and created time. Optional fields include run ID, request ID, actor, actor role, resource type, resource ID, and details JSON.
- `Store.CreateCostRecord` and `ListCostRecords` persist provider usage/cost ledger rows with project/run/stage/task scope, provider/model, prompt/completion/total tokens, optional estimated USD, currency, retry count, latency, metadata JSON, and created time.
- Audit details, cost metadata, and event payloads are decoded as JSON and recursively redacted for string values before persistence. Scalar string columns are also redacted before write.

Auxiliary follow-ups:
- M7 planning writes validated `TaskSpec` rows through `CreateNexdevTask` with pending status after plan validation/versioning and owns DAG cycle checks plus write-task expected-file checks. Review finalization still owns approval semantics, plan edit events, and any version increment after human mutation.
- M8 must load tasks with `ListNexdevTasks`, update execution status through `UpdateNexdevTaskStatus`, and create blockers with `CreateNexdevBlocker` without relying on legacy geoffrussy task/blocker semantics.
- M9 must use Nexdev task/blocker repositories for detour triggers, depth-exceeded blockers, and spliced task persistence, while coordinating plan edits/events outside `internal/state`.
- M8 must consume steering repository rows when building task prompts and must keep steering unable to override safety policy, schemas, or acceptance criteria.
- M9 must write detour records after validated detour creation and coordinate plan edits/events for spliced tasks.
- M10 must use auth token repository lookups from middleware, update `last_used_at` only after successful auth, enforce expiry/revocation/roles, and expose token management without ever returning bearer token plaintext from state.
- M11 must use the same authenticated actor/role model for MCP tool calls and plan/steering mutations.

## 5. Stage Contract

Authoritative Go constants, interfaces, and runner:
- `internal/pipeline`
- Status: M5 runner framework exists. M1-C3 owns canonical stage names, stage order, detour pseudo-stage, status constants, transition checks, prerequisite snapshot validation, and minimal `PipelineStage`/`StageEnv` interfaces. M5 adds durable registration, execution, checkpoint, event, and resume behavior over M3 state repositories.

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

Runner behavior:
- `Runner.Register` accepts only canonical stages. Product stages are registered by later stage wiring; M6 currently implements `repo_analyze`, `interview`, and `complexity` as concrete `PipelineStage` implementations.
- `Runner.Run` loads the persisted run, chooses canonical stages in order, validates prerequisites through `ValidatePrerequisites`, and persists stage status/output/error through the M3 run/stage repository methods.
- `Runner.Resume` is a small wrapper around persisted selection: it reads `runs.current_stage` and `stage_runs`, resumes the current non-terminal stage, or advances to the next canonical non-terminal stage when the current stage is completed or skipped.
- `RunOptions.SingleStage` supports future `nexdev run --stage <stage>` wiring by running exactly one canonical stage.
- `RunOptions.From` supports future `--from` behavior by starting at a canonical stage and continuing in order.
- `PrerequisiteProvider` supplies `PrerequisiteSnapshot` facts. The default provider only derives project existence from `StageEnv.Project` and skipped-stage facts from persisted state; concrete artifact/task prerequisites remain M6/M7/M8 responsibilities.
- `StageOutputter` is an optional stage interface for checkpoint output JSON. Missing output defaults to `{}`.
- `ErrStageSkipped` returned from stage validation persists `pending -> skipped` with output JSON. `BlockedError` from stage execution persists `running -> blocked` with error JSON; other execution errors persist `running -> failed`.
- The runner persists `stage_status` events for status changes and `done` when the run completes, using the M3 event repository. SSE/control-plane broadcast is not part of this contract.

Current deferrals:
- Stage `started_at` updates remain limited by current M3 repository methods; M5 persists status/output/error/completion checkpoints without changing `internal/state`.
- Shared artifact helper extraction and `artifact_updated` events remain M7/M10 work.

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

Repo analysis structured output:
- `internal/pipeline.RepoAnalyzeStage` produces `contract.RepoAnalysis` deterministically without provider calls.
- The output is derived from bounded repository metadata and selected file contents only: repo docs/instructions, package manifests, lockfiles, Go module files, common build/test config, and entrypoint heuristics.
- Repo instructions are captured as untrusted strings and may produce risk notes for prompt-injection patterns; they do not override Nexdev policy.
- Secret-like files including `.env` are excluded from analysis and artifact content.

Interview structured output:
- `internal/pipeline.InterviewStage` calls `provider.StructuredClient.CallStructured` with `provider.SlotInterview` and strict JSON decoding into `contract.InterviewData`.
- The stage requires at least one requirement or open question, non-empty `risk_tolerance`, and non-empty `raw_transcript`.
- If open questions remain and neither yes mode nor CI assumptions are enabled, the stage returns `BlockedError` and does not write the artifact. In yes/CI mode, open questions are converted into `Assumption:` constraints using conservative local-first defaults.
- The durable disk artifact path is `.nexdev/artifacts/interview.json` with artifact kind `interview`.

Complexity structured output:
- `internal/pipeline.ComplexityStage` always computes deterministic complexity before optional model refinement.
- Deterministic output maps score to `trivial`, `small`, `medium`, `large`, or `epic`, recommends phases, collects risk factors and voices, and derives suggested tests from repo-analysis test/lint commands or a manual-review fallback.
- Optional refinement calls `provider.StructuredClient.CallStructured` with `provider.SlotComplexity` and strict JSON decoding into `contract.ComplexityProfile`.
- Provider refinement may add or raise recommendations, but the stage enforces the deterministic floor for score, level, phase count, risk factors, voices, and suggested tests before writing.
- The durable disk artifact path is `.nexdev/artifacts/complexity_profile.json` with artifact kind `complexity_profile`.

Design structured output:
- `internal/pipeline.DesignStage` calls `provider.StructuredClient.CallStructured` with `provider.SlotDesign` and strict JSON decoding into local stage-owned response structs.
- The provider response includes `design_markdown`, `critique.findings`, an `actionable` flag, and metadata. Local semantic validation requires non-empty markdown, valid finding severities, actionable suggestions, and all ten required design headings from `SPEC.md` section 10.4.
- The correction loop defaults to max `3` iterations, feeds actionable findings back into later prompts, stops when no actionable findings remain, and returns `BlockedError` for unresolved high/critical actionable findings unless risk is accepted by stage config.
- The durable disk artifact path is `.nexdev/artifacts/design_draft.md` with artifact kind `design_draft`.

Hivemind structured output:
- `internal/pipeline.HivemindStage` calls `provider.StructuredClient.CallStructured` with `provider.SlotHivemindVoice` for each configured voice and `provider.SlotHivemindSynthesis` for synthesis.
- Voice outputs strict-decode into `contract.HivemindCritique`; semantic validation requires matching voice, valid severity, verdict `approve` or `request_changes`, confidence in `[0,1]`, and valid finding fields.
- Synthesis strict-decodes into `contract.HivemindSynthesis`; semantic validation requires final verdict `approve`, `revise`, or `block`, valid findings, and required changes for non-approve verdicts.
- The durable disk artifact path is `.nexdev/artifacts/design_review.json` with artifact kind `design_review`. The artifact contains cycle, critiques, and synthesis. `revise` and `block` verdicts write the artifact, then return `BlockedError`.

Validation structured output:
- `internal/pipeline.ValidateStage` calls `provider.StructuredClient.CallStructured` with `provider.SlotValidate` and strict JSON decoding into `contract.ValidationReport`.
- Semantic validation requires verdict `pass`, `warn`, or `block` and valid finding fields in ambiguities, conflicts, missing prerequisites, blockers, and hallucination risks.
- The durable disk artifact path is `.nexdev/artifacts/validation_report.json` with artifact kind `validation_report`. Verdicts `pass` and `warn` also write `.nexdev/artifacts/validated_design.md` with artifact kind `validated_design`.
- Conflicts, blockers, and `block` verdicts return `BlockedError` by default after report artifact persistence. The stage does not delete or weaken requirements.

Planning structured output:
- `internal/pipeline.PlanSketchStage` calls `provider.StructuredClient.CallStructured` with `provider.SlotPlanSketch` and strict JSON decoding into `[]contract.PhaseSketch`. Semantic validation requires at least one titled phase. The stage canonicalizes returned phases after decode: duplicate normalized titles are removed, IDs become `phase_NNN`, and numbers become canonical one-based order.
- `internal/pipeline.PlanDetailStage` calls `provider.StructuredClient.CallStructured` with `provider.SlotPlanDetail` and strict JSON decoding into `[]contract.TaskSpec`. Semantic validation requires every task to have acceptance criteria, write/edit tasks to list expected project-relative files or globs, dependencies to reference returned task IDs, and the dependency graph to be acyclic.
- Planning artifact paths are `.nexdev/artifacts/devplan.json`, `.nexdev/artifacts/devplan.md`, and `.nexdev/artifacts/phaseNNN.md`. Detailed-plan tasks are persisted to `nexdev_tasks` with stable `plan_version` and `plan_order` after validation. Pending task status means pre-review and must not satisfy develop by itself.
- `internal/pipeline.ReviewStage` supports contract modes `manual`, `auto`, `ci`, and `skip`. Manual returns a `BlockedError` reason `review_required` until approval is performed through `ReviewService`. CI rejects high/critical-risk tasks without `test_commands`. Skip mode requires an explicit allowance flag and writes the same approval marker as other successful modes. Auto is currently a minimal local approval over already validated pending tasks; provider-backed auto review remains a future refinement.
- Review approval writes `.nexdev/artifacts/review_approval.json` with marker `reviewed_approved_plan`, indexes artifact kind `review_approval`, and returns the same marker through stage output. This marker is the deterministic develop prerequisite until a future state migration adds first-class approval columns.
- Review mutations update only latest-version pending tasks. `UpdatePendingTask` can update title, description, expected files, acceptance criteria, test commands, risk level, required tools, and notes. `DeletePendingTask` deletes a pending task only when no remaining task depends on it and at least one task remains. Every successful mutation increments all current task rows to the next `plan_version`, preserves deterministic plan order, and writes a `plan_edit_events` row with before/after versions, edit type, target, patch JSON, and actor.

M4 provider wrapper behavior:
- `internal/provider.StructuredClient.CallStructured` is the compatibility layer for the imported geoffrussy provider boundary, which currently exposes `Provider.Call(ctx, model, prompt)` rather than the illustrative request-shaped interface in `SPEC.md` section 11.1.
- Callers pass a `Slot`, prompt, destination pointer, and `StructuredOptions`; the wrapper resolves provider/model through `Router.Resolve`, calls the resolved provider instance, decodes JSON into the destination type, and only updates the destination after decode and semantic validation succeed.
- Unknown JSON fields are rejected by default with `json.Decoder.DisallowUnknownFields`; callers can set `AllowUnknownFields` only for an explicit compatibility need.
- `StructuredOptions.Validate` provides semantic validation. Decode or validation failures are accumulated as validation errors and feed repair prompts until `MaxRepairAttempts` is exhausted. A negative repair count selects the default of `2`; zero disables repair.
- Provider errors surfaced by the wrapper are passed through `internal/safety.RedactSecrets`.
- `StructuredResult` returns raw response text, total provider-call attempts, provider/model metadata, validation errors, and token/rate/quota usage metadata exposed by `provider.Response`.
- Raw-response persistence and audit/event integration are intentionally not wired in this wrapper task; later state/observability workers should persist or emit the returned metadata without bypassing the wrapper.

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
- Usage/cost persistence and cost ledger integration remain M14 follow-up.

Fake provider behavior:
- `internal/provider.FakeProvider` implements the imported geoffrussy `Provider` interface without changing real provider implementations.
- Construction is explicit through `NewFakeProvider`; `fake` is not added to the global registry and is disabled unless app/test wiring opts in.
- Scripts match by model and/or prompt matcher and consume responses deterministically in order.
- Scripted responses cover valid content, invalid JSON for structured repair tests, unrecoverable invalid content, retryable API errors, hard errors, usage metadata, latency recorded in call history without sleeping, and streaming chunks.
- `ListModels`, `DiscoverModels`, `GetRateLimitInfo`, `GetQuotaInfo`, `SupportsCodingPlan`, and optional auth-required behavior are implemented for provider-test and CI scenarios.
- M6 pipeline tests should use `StructuredClient` with the fake provider for JSON-producing stages. M16 E2E wiring should construct the fake explicitly rather than relying on the default real-provider registry.

## 8. Executor, Steering, and Detour Contracts

Executor contracts:
- `internal/executor/contracts.go` defines the control boundary and report shape. Required controls are `CurrentTask`, `Pause`, `Resume`, `Cancel`, `SkipTask`, and `SetSteeringContext` with `context.Context` where mutations can block or be cancelled.
- `TaskUpdateEventMapping` maps imported geoffrussy `TaskUpdate` values to Nexdev task event types with source `executor` and stage `develop`. The mapping intentionally stores the stage as the stable string `develop` to keep `internal/executor` decoupled from `internal/pipeline` and avoid import cycles.
- `internal/executor.NexdevExecutor` implements the M8 bridge for fake/safe worker execution. It loads `nexdev_tasks`, persists task status changes through `UpdateNexdevTaskStatus`, persists task events through `Store.PersistEvent`, creates `nexdev_blockers` on worker blockers, and returns `TaskReport` values.
- `FakeWorker` is deterministic and non-executing. It can emit progress/completion/blocker updates and write files only when the path matches the task `expected_files` policy and passes `internal/safety.PathSanitizer`. Shell, network, and real LLM code execution are not implemented by this bridge.
- `internal/pipeline.DevelopStage` requires the review approval artifact marker `reviewed_approved_plan` before invoking the executor bridge and must not treat pre-review pending tasks as approved.

Steering contracts:
- `internal/steering/contracts.go` defines durable steering `Message`, selected prompt `Context`, source constants, and a minimal store interface.
- Accepted steering sources are `cli`, `api`, `tui`, and `mcp`.
- `SafetyPolicyOverrideAllowed` is false. Steering can add operator context but cannot override safety policy, output schema, or task acceptance criteria unless a later admin plan mutation changes the task contract.

Detour contracts:
- `internal/detour/contracts.go` aliases shared `internal/contract.DetourRequest` and `DetourResult` and uses `contract.TaskSpec` for new tasks.
- `RequestContext` captures current task, neighboring tasks, blocker, phase, design summary, repo context, and depth data for later M9 detour generation.
- `WorkflowManager` implements provider-backed detour generation and durable plan mutation over `nexdev_tasks`, `nexdev_blockers`, `detour_records`, and persisted events. It uses `provider.SlotPlanDetail` through `provider.StructuredClient` until a dedicated detour provider slot is assigned.
- `SpliceDetourTasks` assigns new tasks immediate post-trigger order metadata and returns ID conflicts without persisting. Dense persisted plans are handled by `Store.InsertNexdevTasksAfter`, which transactionally shifts later tasks in the same run and plan version before inserting detour rows, preserving existing relative order and plan version metadata.
- Default max depth is `3`; depth exhaustion must surface blocker reason `detour_depth_exceeded` and must not silently skip.
- Depth exhaustion creates an open `nexdev_blockers` row, marks the trigger task blocked, and marks the run blocked. Successful detours persist `detour_records.result_json`, mark the trigger task `pending_after_detour`, and persist `detour_created`.

Implemented M1-C7 tests:
- `go test ./internal/executor` checks task update to event mapping and control interface compilation.
- `go test ./internal/steering` checks source constants, safety override prohibition, and store interface compilation.
- `go test ./internal/detour` checks shared detour types, splice/depth contracts, and existing imported detour behavior.

Current M9 detour tests:
- `go test ./internal/detour` covers blocker-triggered detour request capture, provider-backed structured generation through the fake provider, local task validation, immediate splice ordering after the trigger for gapped and dense plans, ID conflict detection, max-depth blocker creation/no silent skip, structured-output repair, `detour_records` persistence, trigger status updates, and `detour_created` event persistence.
- `go test ./internal/state` covers dense-plan state insertion after a trigger task, multiple inserted detour tasks, shifted later task orders with stable relative order, dependency validation across inserted tasks, and stable plan version metadata.

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
- Cost defaults are enabled with `currency: USD`, `max_run_usd: 25`, `max_stage_usd: 8`, `require_approval_above_usd: 5`, `estimate_before_hivemind: true`, and `stop_on_unknown_price: false`.
- Observability defaults use `log_level: info`, `json_logs: false`, and `otel.enabled: false` with service name `nexdev`.

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
- `cost.currency` is required when cost tracking is enabled.
- `observability.otel.endpoint` is required when OTel is explicitly enabled.

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

Authoritative implementation:
- `internal/controlplane/auth.go`
- Status: M10 adds HTTP auth middleware, opaque token generation/hash helpers, HMAC-SHA256 token hashing with server secret, constant-time auth through hash lookup, role enforcement, expiry/revocation rejection, and `last_used_at` touch after successful authentication. Token command UX remains a CLI follow-up.

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
- Store hash only using `HashBearerToken(server_secret, token)`.
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

Implemented M10 auth/control-plane tests:
- `go test ./internal/controlplane` covers loopback no-auth read routes, non-loopback no-auth startup rejection, bearer-token observer access, operator-only detour access, forbidden observer mutation, and JSON error responses from middleware paths.

## 10.1 HTTP/SSE Control-Plane Runtime Contract

Authoritative implementation:
- `internal/controlplane/server.go`
- `internal/controlplane/routes.go`
- `internal/controlplane/events.go`
- `internal/controlplane/sse.go`
- `internal/controlplane/errors.go`

Implemented routes:
- `GET /health` returns `{ok, api_contract_version}` without auth.
- `GET /status` returns a durable project/run snapshot from `runs`, `stage_runs`, `nexdev_blockers`, and optional injected executor current task.
- `GET /plan` returns grouped persisted `nexdev_tasks` for the selected run.
- `GET /artifacts` returns SQLite-indexed artifact rows.
- `GET /events` returns persisted events, supports `run_id`, `after_sequence`, and `type` filtering, and applies the configured replay cap.
- `GET /runs/{run_id}/stream` replays persisted events after `Last-Event-ID` and streams subsequent persisted events as SSE frames.
- `POST /detour` delegates to an injected M9-compatible detour requester, normally `detour.WorkflowManager`; the route does not call providers directly and does not implement independent task ordering.
- `POST /pause`, `/resume`, `/skip`, `/steer`, and `/cancel` delegate to injected `executor.Control` when wired by app/CLI lifecycle.
- `POST /blockers/{blocker_id}/resolve` updates the durable blocker repository and optionally asks the injected executor to resume.

Current route deferrals:
- `POST /runs` requires an app-level runner service and returns service-unavailable unless `RunStarter` is injected.
- Task mutation routes, config mutation, provider test, and MCP call dispatch remain later worker surfaces and currently return `ErrorResponse` with `not_implemented` or `service_unavailable`.
- Generated OpenAPI server types remain deferred; handlers are manually bound to the existing `api/openapi.yaml` contract.

SSE behavior:
- Frames use `id`, `event`, `retry`, and `data` with the persisted `contract.EventEnvelope` JSON.
- `Last-Event-ID` maps through the state event repository and unknown IDs return `event_not_found`.
- Heartbeats are SSE comments (`: heartbeat`) so the control plane does not invent non-durable event envelopes.
- `Publisher.Publish` persists through `Store.PersistEvent` before broadcasting the stored envelope to bounded subscriber queues.
- Streams also poll durable state after the last sent sequence so events persisted by domain services outside the publisher path are still delivered.

Implemented M10 SSE/control-plane tests:
- `go test ./internal/controlplane` covers persisted replay after `Last-Event-ID`, frame shape, no `[DONE]` sentinel, detour route delegation, and detour-created event persistence through the delegated requester.

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

Authoritative M11 implementation:
- `internal/controlplane/mcp.go`
- `api/mcp_tools.json`

Implemented M11 behavior:
- `GET /mcp/tools` returns the static checked-in descriptor set with each tool's required role and JSON-compatible input schema.
- `POST /mcp/call` accepts `{ "name": string, "arguments": object }`, rejects unknown arguments and wrong JSON types, resolves the tool role, and enforces the HTTP role hierarchy before dispatch.
- Observer tools read durable state: status snapshots, persisted task plans, and SQLite-indexed artifact metadata.
- Operator/admin tools delegate to already-defined services: `RunStarter`, `executor.Control`, M9-compatible `DetourRequester`, `ResolveNexdevBlocker`, and optional `ProviderTester`.
- `nexdev_detour` calls `RequestForBlocker` when `blocker_id` is supplied or `Request` for manual detours. It does not call providers directly and does not implement independent task ordering.
- Results use `{tool,is_error,result,error}`. Error objects use `error_code`, `message`, and `details`, and string content is redacted through `internal/safety.RedactSecrets`.

M11 deferrals:
- `nexdev_get_artifact` returns durable artifact index metadata only; artifact file content serving remains a later artifact/API path-safety task because the control-plane server is not yet wired with a validated project root reader.
- Provider-test execution requires an injected `ProviderTester`; MCP returns a structured not-implemented/service-unavailable error when that service is absent.
- Stdio MCP mode in legacy `internal/mcp` is not the M11 surface. M11 disables legacy tool/resource registration to avoid exposing imported provider/executor handlers; later CLI/stdin MCP work must adapt to the control-plane service boundary before exposure.

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
- Status: artifact kind constants, manifest/item structs, changed-file manifest, run summary, stage summary, and provider usage structs exist for downstream M1 workers. The M3 state repository indexes artifact rows. M6 now writes `.nexdev/artifacts/repo_analysis.json`, `.nexdev/artifacts/interview.json`, `.nexdev/artifacts/complexity_profile.json`, `.nexdev/artifacts/design_draft.md`, `.nexdev/artifacts/design_review.json`, `.nexdev/artifacts/validation_report.json`, and `.nexdev/artifacts/validated_design.md` from concrete stages and indexes them when a concrete state store and project/run identifiers are available. M7 planning writes and indexes `.nexdev/artifacts/devplan.json`, `.nexdev/artifacts/devplan.md`, and phase markdown artifacts. Shared artifact writing helpers, manifests, and `artifact_updated` event emission remain follow-up work.

## 13. Observability Contract

Authoritative implementation:
- `internal/observability/logger.go`
- `internal/observability/context.go`
- `internal/observability/cost.go`
- `internal/observability/otel.go`
- Status: structured logging construction and redaction wrapper exist. M14 adds correlation context, provider usage/cost recording, audit/cost state integration, and disabled-by-default OTel configuration. Runtime metrics and OTel exporters remain follow-up work.

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

M14 behavior:
- `observability.ContextWithCorrelation` carries project/run/stage/task/request/actor metadata across provider/stage/control boundaries when callers provide it.
- `observability.UsageRecorder` implements `provider.StructuredCallRecorder` and writes redacted `cost_ledger` rows plus optional `audit_log` rows through the state repositories.
- `provider.StructuredClient` invokes the optional recorder with redacted provider/model/usage/latency/error metadata and does not pass prompts to the recorder. Previous raw responses are redacted before repair prompts.
- `observability.ConfigureOTel` is a no-op when disabled. If enabled without an endpoint it fails validation; exporter/network setup remains unwired and is not required by tests.

Current deferrals:
- Runtime metrics and OTel exporters are not wired; future work must keep them disabled by default and network-free in normal tests.
- Run summary cost aggregation waits for verify/handoff artifacts.

## 14. CLI Contract

Required commands are listed in `DEVPLAN.md` and `SPEC.md` section 18.

Rules:
- Local CLI uses in-process services.
- `--control-url` switches to HTTP client mode.
- `nexdev steer` maps to `POST /steer` semantics.
- `nexdev init --import-devussy PATH` is required by migration plan.

Current M12 implementation:
- Root command identity and global flags are now `nexdev`-oriented and include `--project-dir`, `--config`, `--state-dir`, `--no-tui`, `--json`, `--log-level`, `--profile`, `--control-url`, and `--token`.
- `nexdev serve` opens project-local state, acquires `.nexdev/run/project.lock`, builds the M10/M11 control-plane server, and releases the lock during shutdown.
- `nexdev auth token create|list|revoke` manages project-local opaque bearer tokens. Token hashes are stored in SQLite; plaintext token values are returned only from `create`.
- `nexdev status --json`, `events`, `provider list`, and `artifacts list` read through the same control-plane handler locally or through HTTP when `--control-url` is set.
- `nexdev pause`, `resume --control-url`, `cancel`, `steer`, `detour`, `blockers resolve`, and `provider test` are client adapters over HTTP control-plane routes. Without `--control-url`, mutating control commands fail with a structured CLI error instead of touching state directly.
- `nexdev run`, `verify`, `history`, and `artifacts open` are present in the command tree. `history` reads persisted events; `run`, `verify`, and artifact content opening return explicit deferred errors unless a wired control-plane service is supplied for `run`.
- Full `run --fake-provider --no-tui --json`, verify/handoff commands, and provider-test service execution remain later milestone work because their lower-level services are not yet complete. Detour generation is wired through M9 `WorkflowManager` and the provider router/structured wrapper; it will fail through that service path if provider credentials/configuration are unavailable.

## 14.1 Terminal TUI Contract

Authoritative implementation:
- `internal/tui/nexdev.go`
- `internal/cli/m12_commands.go` for the `nexdev tui` command entry.

Implemented M13 behavior:
- The TUI is terminal-only. No `web/static` files or embedded web UI are part of this milestone.
- `tui.Client` is the only TUI service boundary. It supports `Snapshot`, `Pause`, `Resume`, `Skip`, `Steer`, `RequestDetour`, and `Cancel`. Implementations include an HTTP control-plane client and an in-process handler client over the existing M10 server handler.
- The snapshot model covers run overview/status, event stream data, plan/tasks, blockers/detours, artifacts, redacted config, and provider summary data. Missing provider/test/config mutation services render as deferred or disabled states.
- Views are selected with `1` through `5`: overview, events, plan/tasks, blockers/detours, and artifacts/config/providers. `r` refreshes, `p` pauses/resumes through the client, `s` sends a deferred steering action through the client, `d` requests detour through the client, `k` asks for skip confirmation, `c` asks for cancel confirmation, and `q` quits only the terminal client.
- Quit does not cancel or kill the run. Destructive cancel and task skip require `y` confirmation before invoking the client action.
- Rendered run/task/artifact/event/blocker text is redacted with `internal/safety.RedactSecrets` and control-character scrubbing before display.
- The TUI must not call providers directly, execute shell commands, implement task ordering, or mutate pipeline state outside the injected service/client path.

Implemented M13 tests:
- `go test ./internal/tui` covers snapshot rendering, key navigation, disabled service actions, secret redaction, normal quit behavior, and explicit confirmation for cancel/skip.
- `go test ./internal/cli` covers command registration including `tui`.

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
- `go test ./internal/state` also validates M3 event repository persistence/load, monotonic per-run allocation, independent run sequences, replay after sequence and event ID, unsafe caller sequence rejection, duplicate event ID failure, and concurrent publishers.
- `go test ./internal/provider` validates router slot resolution, primary inheritance, and unknown slot/provider errors.
- `go test ./internal/controlplane` validates auth role hierarchy, route metadata coverage, MCP per-tool delegation, and remote-bind auth requirement behavior.
- `go test ./internal/testutil` validates M1 black-box fixture contracts for temp projects, deterministic time/IDs, event recording, and auth role fixtures.
- `go test ./internal/observability` validates M2 logging construction, redaction, level filtering, JSON/text modes, and required field helper keys.
- `go test ./internal/controlplane` validates M11 MCP descriptors, per-tool role enforcement, input validation, state read surfaces, detour/blocker/control delegation, and redacted structured MCP errors.
- `go test ./internal/app ./internal/cli` validates M12 lifecycle project/lock setup, remote-bind safety via config/app startup, hash-only token storage, command registration, token-create output, and CLI mutation error handling when no control-plane URL is supplied.

## 16. Test Fixture Contract

Authoritative test-only package:
- `internal/testutil`
- Status: M1-C9 provides black-box fixtures for deterministic UTC time, stable fixture IDs, minimal safe temp projects, event recording over `internal/contract.EventEnvelope`, and auth role fixtures from `internal/controlplane` role constants.

Rules:
- Production code must not import `internal/testutil`.
- Fixture helpers should be extended only when an owning feature test needs them.
- Fake provider, fake worker, SSE replay client, golden-path helpers, and security fixture repos remain deferred; no fake-provider E2E script exists yet.
