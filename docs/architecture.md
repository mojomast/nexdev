# Nexdev Architecture Plan

**Status:** M0 bootstrap recorded; update as implementation lands.  
**Canonical source:** `SPEC.md`.

## 1. Repository Status

This repository completed M0 bootstrap by importing `mojomast/geoffrussy` source into the existing Nexdev repository while preserving root planning artifacts.

Actual base strategy:
- Source imported from `https://github.com/mojomast/geoffrussy.git` at `e29f8e7649584585a93d8fc8ac9123036fcaf38e`.
- Go module path is `github.com/mojomast/nexdev`.
- Imported geoffrussy packages provide the initial provider, state, navigation, executor, git, CLI/TUI dependency, migration, security, and test foundations.
- Existing planning artifacts remain at the repository root and continue to control implementation: `SPEC.md`, `DEVPLAN.md`, `AGENTS.md`, `WORKER_PROTOCOL.md`, `SPEC_UPDATE_PROTOCOL.md`, `TESTING_STRATEGY.md`, `PROMPT_FOR_DEVELOPMENT_SESSION.md`, `docs/architecture.md`, `docs/contracts.md`, and `README.md`.
- `cmd/nexdev/main.go` is a minimal bootstrap wrapper over the imported CLI wiring; M1/M12 must replace/reshape command behavior through contract-first CLI work.
- The imported upstream `cmd/geoffrussy` remains present for baseline compatibility until an orchestrated CLI/package cleanup task decides how to retire or adapt it.
- Baseline checks after import: `go test ./...` and `go vet ./...` pass.

Next action guidance:
- Begin M1 contract freeze before feature work: OpenAPI skeleton, event envelope/constants, stage/status contracts, state migration skeleton, provider/router contracts, executor/detour interfaces, and test fixtures.
- Keep secondary docs aligned with `SPEC.md`; do not treat imported geoffrussy behavior as Nexdev contract unless M1+ contracts adopt it.

## 2. Runtime Shape

Nexdev is planned as a Go-first single binary with multiple surfaces:
- CLI.
- TUI.
- HTTP control plane.
- SSE event stream.
- MCP-compatible tool surface.

Core state is owned by one process per project in v0.1. Multi-process concurrent mutation is not supported. Mutating operations must acquire `.nexdev/run/project.lock`; the M2 helper lives in `internal/git` and is ready for later app/executor wiring.

## 3. Package Boundaries

Planned package ownership:

| Package | Responsibility | Must Not Own |
|---|---|---|
| `cmd/nexdev` | Binary entrypoint and CLI root | Business logic |
| `internal/app` | Dependency wiring and lifecycle | Domain contracts |
| `internal/config` | Typed config, defaults, loading, validation | Runtime state mutation |
| `internal/controlplane` | HTTP, SSE, auth middleware, MCP adapter | Pipeline state machine |
| `internal/contract` | Inert schemas, constants, validation helpers | Service/business logic |
| `internal/pipeline` | Stage graph contracts, prerequisite validation, runner, stage implementations | Provider implementations or concrete state/control-plane types |
| `internal/executor` | Task execution, updates, steering bridge | HTTP route schemas |
| `internal/detour` | Detour request, task generation, splice | Control-plane routing |
| `internal/provider` | Provider interfaces, registry, router, fake provider | Stage-specific business logic |
| `internal/state` | SQLite store, migrations, repositories | Pipeline/control-plane imports |
| `internal/safety` | Path, redaction, tool policy, prompt-injection guards | Provider calls |
| `internal/git` | Git manager, worktrees, changed files | Task planning |
| `internal/observability` | Logging, OTel, metrics, cost ledger | Domain mutation rules |
| `internal/tui` | TUI client views and controls | Pipeline state ownership |

## 4. Dependency Direction

Preferred dependency direction:
- `cmd/nexdev` wires `internal/app`.
- `internal/app` wires config, state, provider, pipeline, executor, controlplane, TUI.
- Domain packages depend on small interfaces and contracts, not HTTP handlers.
- `internal/state` stays low-level and does not import higher-level domains.
- `internal/provider` is the only boundary for provider implementations.
- `internal/controlplane` adapts HTTP/SSE/MCP to app services.

Avoid import cycles by defining small interfaces at the consumer boundary.

## 5. Stage Flow

`internal/pipeline` now owns the Nexdev stage/status contract surface independently from legacy imported `internal/navigation`. The package uses small local interfaces in `StageEnv` so later state, provider, control-plane, git, and safety packages can be wired by `internal/app` without import cycles.

Canonical stage order:

```text
init
  -> repo_analyze
  -> interview
  -> complexity
  -> design
  -> hivemind
  -> validate
  -> plan_sketch
  -> plan_detail
  -> review
  -> develop
  -> verify
  -> handoff
  -> complete
```

`detour` is a pseudo-stage:

```text
develop -> detour -> develop
```

Current M5 runner behavior:
- `internal/pipeline.Runner` registers canonical `PipelineStage` implementations by `Stage`; pseudo-stages are rejected by the runner registry.
- Run execution uses the canonical stage order from persisted run state by default. `RunOptions.From` selects a canonical starting stage and `RunOptions.SingleStage` runs exactly one canonical stage for later CLI/control-plane adapters.
- Resume selection reads `runs.current_stage` and `stage_runs` from SQLite. If the persisted current stage is terminal, resume advances to the next non-terminal canonical stage instead of using in-memory progress.
- Prerequisites are validated before every stage through `ValidatePrerequisites`. Artifact/task-derived facts are supplied by a `PrerequisiteProvider` interface so concrete stages can add evidence without the runner overreaching into artifact semantics.
- Stage status changes are checked through `ValidateStatusTransition` before persistence. Completed, skipped, blocked, and failed stages persist output/error JSON to `stage_runs` as a checkpoint-like record.
- The runner persists `stage_status` events and a final `done` event through the M3 event repository. It does not broadcast SSE; M10 owns replay/broadcast over the persisted log.

Current M6 repo analysis behavior:
- `internal/pipeline.RepoAnalyzeStage` is a deterministic, provider-free stage registered as `repo_analyze` through the existing `PipelineStage` interface.
- The stage is constructed with an explicit project root, walks only bounded repository context, excludes `.git`, `.nexdev`, `node_modules`, `vendor`, `dist`, `build`, secret-like files, and files above the configured size cap, and records repo instructions as untrusted strings.
- It writes `.nexdev/artifacts/repo_analysis.json` and indexes the artifact when `StageEnv.Store` is a `*state.Store` with project/run identifiers available. Shared artifact helper and `artifact_updated` emission remain later artifact/control-plane follow-up work.

Current M6 interview/complexity behavior:
- `internal/pipeline.InterviewStage` is provider-backed through `provider.StructuredClient` and `provider.SlotInterview`. It builds prompts with `SYSTEM POLICY`, `TRUSTED CONFIG`, `UNTRUSTED REPO CONTEXT`, and `TASK` sections, validates `contract.InterviewData`, blocks on unresolved open questions unless yes/CI assumptions are enabled, writes `.nexdev/artifacts/interview.json`, and indexes the artifact when a `*state.Store` plus project/run identifiers are available.
- `internal/pipeline.ComplexityStage` computes a deterministic `contract.ComplexityProfile` from interview and repo-analysis inputs first. Optional provider refinement uses `provider.SlotComplexity`; refinement cannot reduce deterministic score, level, phase count, risk factors, voices, or suggested verification tests. The stage writes `.nexdev/artifacts/complexity_profile.json` and indexes it when state is available.
- Both stages use explicit constructors/config structs rather than expanding `StageEnv`.

Current M6 design behavior:
- `internal/pipeline.DesignStage` is provider-backed through `provider.StructuredClient` and `provider.SlotDesign`. It consumes supplied interview, repo-analysis, and complexity inputs, builds prompts with trusted sections and clearly marked `UNTRUSTED REPO CONTEXT`, and keeps local response structs in `internal/pipeline`.
- The stage runs a bounded self-critique/correction loop with default max iterations of `3`, validates that the resulting markdown includes the ten required `SPEC.md` section 10.4 headings, writes `.nexdev/artifacts/design_draft.md`, and indexes it as artifact kind `design_draft` when state is available. Remaining high/critical actionable findings return `BlockedError` unless `AcceptRisk` is set.

Current M6 hivemind/validate behavior:
- `internal/pipeline.HivemindStage` is provider-backed through `provider.StructuredClient`, `provider.SlotHivemindVoice`, and `provider.SlotHivemindSynthesis`. It consumes supplied interview, repo-analysis, complexity, and design markdown inputs, runs configured voices sequentially by default or with bounded concurrency when `Parallel` is set, writes `.nexdev/artifacts/design_review.json`, and indexes it as artifact kind `design_review` when state is available.
- Hivemind prompts preserve explicit trusted/untrusted sections, require the security voice to inspect prompt-injection/tool-poisoning risk, and return `BlockedError` after writing the review artifact when synthesis verdict is `revise` or `block` so later design-correction wiring can consume required changes.
- `internal/pipeline.ValidateStage` is provider-backed through `provider.StructuredClient` and `provider.SlotValidate`. It consumes supplied interview, repo-analysis, complexity, design markdown, and latest hivemind synthesis inputs, writes `.nexdev/artifacts/validation_report.json`, writes `.nexdev/artifacts/validated_design.md` for `pass` and `warn`, and indexes both artifacts when state is available.
- Validation blocks on conflicts, blockers, and `block` verdicts by default. The stage prompt explicitly forbids deleting or weakening requirements to make validation pass.

M6/M7 follow-ups:
- App/runner wiring must pass current stage outputs/artifact content into hivemind and validation constructors; the concrete stages intentionally do not rescan or regenerate upstream artifacts.
- Artifact-writing stages must index files through the artifact repository and emit `artifact_updated` after persistence once the shared artifact/event helper exists.
- App/CLI/control-plane wiring must acquire the project lock before invoking mutating runner paths.
- State follow-up should add typed repositories for `hivemind_results` and `validate_results`; this task only writes/indexes artifacts because state ownership was out of scope.

Current M7 planning behavior:
- `internal/pipeline.PlanSketchStage` is provider-backed through `provider.StructuredClient` and `provider.SlotPlanSketch`. It consumes supplied interview, repo analysis, complexity, validated design, and validation report inputs; validates that validation passed or warned; canonicalizes phase IDs/numbers by provider array order; deduplicates similar phase titles; and writes `.nexdev/artifacts/devplan.json` with phase sketches only.
- `internal/pipeline.PlanDetailStage` is provider-backed through `provider.StructuredClient` and `provider.SlotPlanDetail`. It consumes supplied phase sketches, asks for `contract.TaskSpec` rows, validates task acceptance criteria, write-task expected files, dependency references, and dependency cycles, then writes `.nexdev/artifacts/devplan.json`, `.nexdev/artifacts/devplan.md`, and one `phaseNNN.md` artifact per phase.
- After validation, detailed tasks are persisted to `nexdev_tasks` with stable `plan_version`, `plan_order`, and pending status. `internal/pipeline.ReviewStage` and `ReviewService` now gate those pending tasks before develop: manual mode blocks with `review_required` until an approval service call writes the deterministic `.nexdev/artifacts/review_approval.json` marker, auto mode locally approves the validated pending plan, CI mode rejects high/critical-risk tasks without test commands, and skip mode approves only when explicitly allowed by config/caller.
- Review edits operate over the latest persisted pending plan version. Task updates and deletes validate the resulting plan, increment `plan_version`, and write `plan_edit_events`; non-pending tasks are rejected. Because the state schema has no approved column, develop prerequisite wiring should use the review approval artifact/stage output marker `reviewed_approved_plan` instead of treating pre-review pending tasks as approved.

Current M8 develop/executor behavior:
- `internal/pipeline.DevelopStage` implements the canonical `develop` stage and refuses to run until `.nexdev/artifacts/review_approval.json` contains an approved `reviewed_approved_plan` marker. Pending tasks alone are not sufficient and are left untouched when the marker is absent.
- `internal/executor.NexdevExecutor` is an additive Nexdev bridge over `nexdev_tasks`, `nexdev_blockers`, and the persisted event log. It lists pending/pending-after-detour tasks, updates task status, maps task updates to the required task event family, creates blockers on structured worker blockers, and returns `TaskReport` values.
- The default test/CI worker path is deterministic `FakeWorker`. It can emit progress, complete tasks, report blockers, and write only task-expected files after `internal/safety.PathSanitizer` validation. Shell and network execution are not implemented.
- Basic in-process controls exist for pause, resume, cancel, skip, current task, and steering ingestion. Steering is persisted and included in prompt context as last-N messages plus summary, but it cannot override safety policy or acceptance criteria.
- Real LLM code execution, shell tools, control-plane handlers, project-lock wiring, and full steering prompt context from requirements/design/repo artifacts remain follow-up work.

Current M9 detour behavior:
- `internal/detour.WorkflowManager` is the Nexdev first-class detour workflow beside the legacy imported `detour.Manager`. It captures the trigger task, open blocker ID when available, neighboring tasks, phase, design-summary/artifact placeholder, repo context, source, reason, and current depth into `RequestContext` before generation.
- Detour generation uses `provider.StructuredClient` through `provider.SlotPlanDetail` because no dedicated provider slot exists yet. A later provider/config task can add a detour slot if the orchestrator wants separate model routing.
- Generated detour tasks are validated with local M7-equivalent rules for acceptance criteria, write-task expected files, dependency references, expected-file path shape, and inserted-set cycles. Splicing detects ID conflicts before persistence.
- Max depth defaults to `3`. Depth exhaustion creates an open `nexdev_blockers` row with reason `detour_depth_exceeded`, marks the trigger task blocked, marks the run blocked, and returns an error rather than silently skipping.
- Splicing persists new `nexdev_tasks` with deterministic `D<depth>.<seq>` IDs when the provider omits IDs and stable integer `plan_order` values immediately after the trigger when an order gap exists. Dense persisted plans need a state/review follow-up to shift existing task order values safely.
- Successful detours mark the trigger task `pending_after_detour`, persist `detour_records.result_json`, and persist a `detour_created` event. Control-plane route/UI and automatic develop resume remain M10/M12 integration work.

## 6. State Flow

SQLite is the durable source of truth for:
- Projects.
- Runs.
- Stage runs.
- Events.
- Tasks and blockers.
- Artifacts.
- Hivemind results.
- Validation results.
- Steering events.
- Detour records.
- Navigation events.
- Plan edit events.
- Auth tokens.
- Audit/cost records if implemented.

Disk artifacts under `.nexdev/artifacts/` are human/agent continuation outputs. They are indexed in SQLite and are not the source of truth for run state.

## 7. Event Flow

Every durable event follows this flow:

1. Domain service creates an event payload.
2. Event publisher validates envelope fields.
3. State event repository assigns monotonic per-run sequence and persists the event.
4. Subscriber manager broadcasts the persisted event.
5. SSE clients receive frames.
6. Reconnecting clients use `Last-Event-ID` to replay missed events.

Persist-before-broadcast is mandatory.

## 8. Provider Flow

Stages and executor code must call providers only through `internal/provider` router/wrapper.

Current M1 status:
- `internal/provider` preserves the imported geoffrussy `Provider` interface and registry.
- `internal/provider.Router` resolves Nexdev stage provider slots to provider/model selections and validates provider names against the registry without instantiating providers.
- Empty slot selections inherit the primary provider/model selection.
- `internal/provider.StructuredClient` is the M4 compatibility wrapper for structured calls over the imported `Provider.Call(ctx, model, prompt)` interface. It accepts already-wired provider instances, resolves slot/model through `Router.Resolve`, strict-decodes JSON, rejects unknown fields by default, runs optional semantic validation, and repairs decode/validation failures up to the configured cap.
- The wrapper returns raw response text, attempt count, provider/model metadata, validation errors, and usage metadata exposed by `provider.Response`.
- `internal/provider.FakeProvider` is a deterministic constructor-only provider for CI and later pipeline tests. It scripts responses by model and prompt matcher, supports invalid-then-repair structured flows through `StructuredClient`, retryable and hard errors, usage metadata, deterministic latency records without sleeping, streaming chunks, model list/discovery, and optional auth-required behavior.
- The fake provider is not registered in the global provider registry; M6/M16 wiring must opt in explicitly by constructing it and pairing it with a router selection for provider name `fake`.
- Raw response persistence, provider call event emission, and cost ledger integration remain M14 follow-up work.

Structured output flow:
1. Build prompt with trusted/untrusted sections and schema.
2. Route through configured provider slot.
3. Call the resolved imported provider through `Provider.Call(ctx, model, prompt)`.
4. Strict-decode response and reject unknown fields unless explicitly allowed by the caller.
5. Validate semantic constraints through the caller's callback.
6. Repair decode or validation failures if allowed.
7. Persist raw response and validation errors for audit when state/event wiring owns that integration.
8. Return typed contract object plus wrapper metadata.

## 9. Safety Flow

Current M2 config/path baseline:
- `internal/config` now exposes Nexdev-specific typed defaults and validation alongside the imported geoffrussy config manager for compatibility.
- New Nexdev defaults use project-local `.nexdev` state, loopback control-plane bind `127.0.0.1:7432`, `auth_required: auto`, denied command/network defaults, `.nexdev/tool_policy.yaml`, repo-analyze excludes, and provider stage placeholders.
- `internal/safety` now owns the spec-target path sanitizer baseline for new Nexdev code. The imported `internal/security` sanitizer remains unchanged for geoffrussy compatibility.
- `internal/safety.RedactSecrets` provides deterministic best-effort scrubbing for provider/API key shapes, bearer tokens, password/token assignments, private key blocks, SSH keys, and `.env` style secret assignments before text reaches logs, events, artifacts, prompts, or API responses.
- `internal/safety.DetectPromptInjection` scans untrusted repo/tool text for common instruction override, prompt exfiltration, role override, safety bypass, and secret exfiltration strings and returns warning findings only. It does not change policy decisions by itself.
- `internal/safety.DefaultToolPolicy` is a non-executing policy skeleton: read/write basics are allowed, shell and network are denied by default, writes deny `.git`, `.env`, private-key, PEM, and key-looking paths, and wildcard shell allow rules are invalid in `trusted-lan` and `ci` profiles.

Before a task modifies files:
1. Clean path.
2. Resolve against project root.
3. Evaluate symlinks.
4. Check allowed roots.
5. Check deny globs.
6. Check file locks.
7. Check task expected files.
8. Record hashes where feasible.

Shell/network tools are denied by default and require explicit policy allowance.
Command execution and network tool implementations do not exist in this M2 baseline; later executor/verify work must call the policy evaluator before running any command or network-capable tool.

Project lock baseline:
- `internal/git.ProjectLockPath` resolves `.nexdev/run/project.lock` under the project root through `internal/safety.PathSanitizer`.
- `internal/git.AcquireProjectLock` creates `.nexdev/run`, atomically creates the lock file with exclusive create semantics, writes pid and UTC acquisition timestamp metadata, and removes the file on release.
- Existing lock files are treated as held. Stale lock detection, process liveness checks, and integration with app/executor lifecycle are M15/M21 follow-ups.

## 10. Observability Flow

Current M2 logging baseline:
- `internal/observability` now owns the spec-target structured logging baseline for new Nexdev code. The imported `internal/logging` package remains unchanged for geoffrussy compatibility.
- `observability.NewLogger` constructs standard `log/slog` loggers with configurable level and JSON or text handlers.
- A redacting handler wraps the underlying slog handler and applies `internal/safety.RedactSecrets` to log messages, string attributes, grouped string attributes, and attributes attached through `Logger.With` before write.
- Field helper attributes use the canonical names from `SPEC.md` section 17: `project_id`, `run_id`, `stage`, `task_id`, `provider`, `model`, `event_id`, and `request_id`.
- OpenTelemetry, runtime instrumentation, metrics, audit logs, and the cost ledger are not implemented in M2. `observability.OpenTelemetryEnabled` is a documented false placeholder until M14 owns OTel/cost/audit integration.

M14 follow-ups:
- Wire request IDs, event IDs, provider usage, stage/task timings, and control-plane/executor/provider instrumentation through the logger and event/state layers.
- Add OTel setup only behind explicit config and keep it disabled by default.
- Add persistent cost/audit behavior only after state/provider usage contracts exist.

## 11. Control Plane Flow

HTTP and MCP calls are adapters over the same services used by local CLI/TUI.

Rules:
- `GET /health` requires no auth.
- Observer can read state and streams.
- Operator can pause/resume/skip/steer/detour/resolve/provider-test.
- Admin can cancel, mutate config/tasks/tokens, and perform destructive operations.
- Non-loopback bind without auth fails startup.
- JSON errors use `ErrorResponse`.

## 12. Development Parallelism

Implementation parallelism is achieved by package ownership, not by shared-file contention.

Parallel-safe domains after contracts freeze:
- Config.
- State repositories split by table group.
- Provider fake/wrapper/router.
- Pipeline stages split by stage group.
- Control-plane REST and SSE workers.
- CLI command groups after root command exists.
- Security fixtures.
- Docs updates with a reconciliation pass.

Serialize:
- Bootstrap.
- OpenAPI root contract edits.
- Migration numbering.
- Pipeline runner/navigation core.
- Root CLI wiring.
- Spec edits.
