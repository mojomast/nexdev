# Nexdev Architecture Plan

**Status:** Final stabilization reflects implemented behavior through TASK-10.
**Canonical source:** `SPEC.md`.

## 1. Repository Status

This repository completed M0 bootstrap by importing `mojomast/geoffrussy` source into the existing Nexdev repository while preserving root planning artifacts.

Actual base strategy:
- Source imported from `https://github.com/mojomast/geoffrussy.git` at `e29f8e7649584585a93d8fc8ac9123036fcaf38e`.
- Go module path is `github.com/mojomast/nexdev`.
- Imported geoffrussy packages provide the initial provider, state, navigation, executor, git, CLI/TUI dependency, migration, security, and test foundations.
- Existing planning artifacts remain at the repository root and continue to control implementation: `SPEC.md`, `DEVPLAN.md`, `AGENTS.md`, `WORKER_PROTOCOL.md`, `SPEC_UPDATE_PROTOCOL.md`, `TESTING_STRATEGY.md`, `PROMPT_FOR_DEVELOPMENT_SESSION.md`, `docs/architecture.md`, `docs/contracts.md`, and `README.md`.
- `cmd/nexdev/main.go` is the Nexdev CLI entrypoint. TASK-10 hides legacy geoffrussy-era root command surfaces from reachable Nexdev help and keeps the root command aligned with the spec command set.
- Baseline and release checks after final stabilization include `go test ./...`, `go test -race ./...`, `go vet ./...`, `go mod verify`, `govulncheck ./...`, generated OpenAPI drift checks, and fake-provider E2E.

Next action guidance:
- Keep secondary docs aligned with `SPEC.md`; do not treat imported geoffrussy behavior as Nexdev contract unless Nexdev contracts adopt it.
- Preserve explicit remaining deferrals: full real-provider pipeline execution, web UI assets, artifact content opening, full OpenAPI response validation/server binding, and the shared changed-file artifact `old_path` extension.

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
- The default test/CI worker path is deterministic `FakeWorker`. It can emit progress, complete tasks, report blockers, and write only task-expected files after `internal/safety.PathSanitizer` validation. General generated-task shell and network execution are not implemented.
- Basic in-process controls exist for pause, resume, cancel, skip, current task, and steering ingestion. Steering is persisted and included in prompt context as last-N messages plus summary, but it cannot override safety policy or acceptance criteria.
- Real LLM code execution remains deferred to full real-provider pipeline work. Shell command execution exists only through explicit policy-gated verification; generated task shell/network execution remains denied unless a future runner is assigned.

Current M16 fake-run behavior:
- `internal/app.RunFakeProvider` wires the existing pipeline runner from `repo_analyze` through `complete` for local `nexdev run --fake-provider --no-tui --json` execution. The fake provider is constructed explicitly with a local registry entry and is not added to the global production provider registry.
- The fake run uses deterministic provider scripts plus `executor.FakeWorker`; the develop task writes only `generated/fake_e2e.txt` through expected-file/path checks. It does not execute shell commands or open network connections.
- `internal/pipeline.VerifyStage` records a verify report and verify events. TASK-01 adds policy-gated command execution with exact allow checks, controlled environment, timeout, output caps, cancellation, and bounded repair attempts; denied commands are reported without execution. `HandoffStage` writes changed-files, run-summary, and handoff artifacts. `CompleteStage` marks the canonical terminal stage.
- The M16 E2E script creates a temp project, runs the full fake pipeline, starts the loopback control plane, validates persisted event/SSE replay, checks artifacts and changed files, and scans artifacts for known fixture secret leaks.

Current M17 real-provider smoke behavior:
- `internal/provider` owns the opt-in real-provider smoke helper and `TestRealProviderSmoke`. The helper requires `NEXDEV_RUN_REAL_PROVIDER_TESTS=1`, provider/model config, credentials through an env var, `NEXDEV_REAL_PROVIDER_MAX_USD <= 0.25`, and a bounded timeout before making any provider call.
- The smoke validates only the router/structured-output path with a tiny fixed JSON prompt. It does not run the full pipeline, read repository files, execute tools, or register fake providers globally.
- `scripts/real_provider_smoke.sh` is safe to run without env gates; it runs skip-path tests and exits without network. With all gates set, it runs only `TestRealProviderSmoke`.

Current M9 detour behavior:
- `internal/detour.WorkflowManager` is the Nexdev first-class detour workflow beside the legacy imported `detour.Manager`. It captures the trigger task, open blocker ID when available, neighboring tasks, phase, design-summary/artifact placeholder, repo context, source, reason, and current depth into `RequestContext` before generation.
- Detour generation uses `provider.StructuredClient` through `provider.SlotPlanDetail` because no dedicated provider slot exists yet. A later provider/config task can add a detour slot if the orchestrator wants separate model routing.
- Generated detour tasks are validated with local M7-equivalent rules for acceptance criteria, write-task expected files, dependency references, expected-file path shape, and inserted-set cycles. Splicing detects ID conflicts before persistence.
- Max depth defaults to `3`. Depth exhaustion creates an open `nexdev_blockers` row with reason `detour_depth_exceeded`, marks the trigger task blocked, marks the run blocked, and returns an error rather than silently skipping.
- Splicing persists new `nexdev_tasks` with deterministic `D<depth>.<seq>` IDs when the provider omits IDs and stable integer `plan_order` values immediately after the trigger. Dense persisted plans are handled by state-owned transactional insertion, which shifts later tasks within the same run and plan version before inserting detour rows.
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
- Audit log records.
- Cost ledger records.

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
- Provider structured calls can now report redacted usage metadata to an optional recorder; provider call event emission remains follow-up work.

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
Verification command execution exists only through the policy-gated verify runner. General task shell/network tools remain absent unless a later runner is assigned, and any future network-capable tool must call the policy evaluator before execution.

Project lock behavior:
- `internal/git.ProjectLockPath` resolves `.nexdev/run/project.lock` under the project root through `internal/safety.PathSanitizer`.
- `internal/git.AcquireProjectLock` creates `.nexdev/run`, atomically creates the lock file with exclusive create semantics, writes pid and UTC acquisition timestamp metadata, and removes the file on release.
- `internal/git.AcquireProjectLockWithPolicy` handles stale locks with pid-liveness checks: live pids keep the lock held, dead pids allow safe removal and retry, and malformed/unreadable metadata fails safe for manual operator recovery.

## 10. Observability Flow

Current M2 logging baseline:
- `internal/observability` now owns the spec-target structured logging baseline for new Nexdev code. The imported `internal/logging` package remains unchanged for geoffrussy compatibility.
- `observability.NewLogger` constructs standard `log/slog` loggers with configurable level and JSON or text handlers.
- A redacting handler wraps the underlying slog handler and applies `internal/safety.RedactSecrets` to log messages, string attributes, grouped string attributes, and attributes attached through `Logger.With` before write.
- Field helper attributes use the canonical names from `SPEC.md` section 17: `project_id`, `run_id`, `stage`, `task_id`, `provider`, `model`, `event_id`, and `request_id`.
- OpenTelemetry, runtime instrumentation, metrics, audit logs, and the cost ledger were not implemented in M2. M14 added audit and cost ledger behavior; `observability.OpenTelemetryEnabled` remains a false compatibility constant.

Current M14 observability/audit/cost behavior:
- State migration version `6` adds durable `audit_log` and `cost_ledger` tables. Repository writes scrub string fields and JSON string values before persistence.
- `Store.PersistEvent` scrubs event payload JSON before insert by decoding, redacting string values, and re-encoding valid JSON.
- `provider.StructuredClient` accepts an optional `StructuredCallRecorder`; it records redacted provider/model/usage/latency/error metadata and redacts previous raw responses before repair prompts. Prompts are not handed to the recorder.
- `observability.UsageRecorder` implements the provider recorder hook and persists cost ledger entries plus optional audit records when a state store and project scope are supplied. Correlation can come from `observability.ContextWithCorrelation`; app wiring supplies project/latest-run defaults for app-created provider clients.
- `controlplane.Authenticator` writes audit records for failed auth, forbidden authorization, and allowed operator/admin control requests when auth is enabled.
- `observability.ConfigureOTel` is inert when disabled and validates explicit endpoint configuration when enabled. Exporters remain unwired, so normal tests require no network access.

Current M15 security hardening behavior:
- `internal/safety.ToolPolicy.ValidateTaskWritePath` combines default write policy, deny globs, active file-lock globs, and task expected-file globs for owned write helpers. Shell and network execution remain absent unless a later policy-gated runner is assigned.
- Stage artifact writes scrub decoded JSON string values before persistence, keeping artifact JSON valid while removing secret-shaped content.
- Hivemind emits persisted `security_warning` events for prompt-injection findings in untrusted repo context when a concrete state store and run are available.
- Control-plane auth uses a deterministic local throttle for authenticated routes; throttled requests return `429` and produce audit records when audit storage is wired.
- `observability.CostGuard` can deny provider launches before execution; Hivemind calls it before voice and synthesis provider launches when configured.

Observability follow-ups:
- Pipeline/executor/app boundaries should pass richer request/event/stage/task correlation context into provider calls.
- `run_summary.json` now aggregates provider usage/cost through `Store.SummarizeCostForRun`.
- OTel exporter setup and metrics remain opt-in follow-up work; do not enable network exporters by default.

## 11. Control Plane Flow

HTTP and MCP calls are adapters over the same services used by local CLI/TUI.

Current M10 HTTP/SSE/auth behavior:
- `internal/controlplane.Server` binds route handlers with standard `net/http` ServeMux patterns and validates startup safety with `RequireAuthForBind`; non-loopback bind without auth fails before a handler is exposed.
- `GET /health` is unauthenticated. When auth is enabled, observer/operator/admin routes use opaque bearer tokens stored as HMAC-SHA256 hashes in the project SQLite `auth_tokens` table. Middleware rejects missing, expired, revoked, or insufficient-role tokens and updates `last_used_at` only after successful auth.
- M14 audit integration records failed authentication, forbidden authorization, and allowed operator/admin control requests to the durable `audit_log` table when auth is enabled and project state is available.
- Read routes load durable state from SQLite: `/status` summarizes the selected project/run, stage runs, current executor task when wired, and open blockers; `/plan` groups persisted `nexdev_tasks`; `/artifacts` lists indexed artifacts; `/events` lists persisted events with replay limits.
- `/runs/{run_id}/stream` replays persisted events, honors `Last-Event-ID`, emits SSE frames with persisted event IDs/types/data, and sends non-durable heartbeat comments rather than invented event envelopes. It also subscribes to the persisted-event publisher and polls durable state so events persisted by domain services such as M9 detour are visible without provider calls from routes.
- `POST /detour` is an authenticated operator route and delegates to the M9 `WorkflowManager.Request`/compatible requester interface. The route does not generate tasks, reorder plans, or call providers directly.
- Pause/resume/skip/steer/cancel routes are thin adapters over an injected `executor.Control` when app wiring supplies one; otherwise they return a contract-shaped service-unavailable error. Task mutation and config mutation remain later worker surfaces.
- Provider-test routes are thin adapters over an injected `ProviderTester`. App wiring injects the M17 real-provider smoke tester only when `NEXDEV_RUN_REAL_PROVIDER_TESTS=1`; otherwise HTTP and MCP provider-test paths return structured service-unavailable/not-wired errors and do not call providers.
- JSON errors use `ErrorResponse` and redact string detail before response.

Current M11 MCP behavior:
- `internal/controlplane/mcp.go` is the M11 MCP-compatible adapter. It exposes static descriptors for the required `nexdev_*` tools and keeps `/mcp/tools` as an authenticated observer read surface.
- `POST /mcp/call` authenticates at observer level, resolves the requested tool's own role, and then applies the same role hierarchy used by HTTP. Tool descriptions and schemas are static code/manifest data and cannot expand permissions at runtime.
- Read-only MCP tools load durable SQLite state through the same store-backed status, plan, artifact, and blocker/event-aware helpers used by HTTP. MCP does not invent non-durable event semantics.
- Workflow tools delegate to existing service boundaries: run start through `RunStarter`, pause/resume/cancel/steer through `executor.Control`, detour through `DetourRequester.Request` or `RequestForBlocker`, and blocker resolution through the durable blocker repository. The MCP layer does not call providers, execute shell commands, or implement task ordering.
- Provider testing is exposed only through an optional injected `ProviderTester`; when it is not wired, the MCP tool returns a structured not-implemented/service error rather than calling providers directly.
- Legacy `internal/mcp` remains an imported geoffrussy-era stdio package and is not the M11 control-plane surface. M11 disables its tool/resource registration so the imported provider/executor handlers are not exposed through CLI stdio MCP; future cleanup should retire or adapt it over the M11 service boundary.

Current M12 CLI/app lifecycle behavior:
- `internal/app` now owns the narrow CLI/server lifecycle used by M12. It resolves project root/config/state paths, opens the project-local SQLite store at `.nexdev/state.db` by default, creates a persistent project ID file, ensures a project row exists, and acquires `.nexdev/run/project.lock` for `nexdev serve`.
- `nexdev serve` constructs the existing M10 `controlplane.Server` with config-derived bind/auth/CORS/SSE settings, project ID, durable state store, and the M11 MCP routes registered by that server. Startup still fails before listening for non-loopback bind without auth.
- The server secret for opaque bearer tokens is project-local under `.nexdev/run/server.secret` with `0600` permissions. `nexdev auth token create|list|revoke` uses the M10 token generation/hash helpers and M3 token repository; plaintext is printed only at creation time.
- If a latest run exists, app wiring supplies M8 `executor.Control` to the server for pause/resume/skip/cancel/steer. App wiring also constructs the M9 `detour.WorkflowManager` with a provider router/structured client from config, so detour requests stay behind the existing detour/provider boundaries. M17 provider-test service wiring is injected only under the explicit real-provider smoke env gate. Full real-provider run-start behavior remains explicitly deferred; fake-provider local run wiring is implemented. CLI control commands use `--control-url` rather than mutating those domains directly.
- Local read commands such as `status --json`, `events`, `provider list`, and `artifacts list` construct the same server handler in-process. Remote mode sends HTTP requests only when `--control-url` is supplied and uses `--token` or `NEXDEV_CONTROL_TOKEN` for bearer auth.
- `nexdev events --follow` follows local in-process events or remote SSE with `Last-Event-ID` reconnect support, JSON-line output in JSON mode, and clean cancellation.

Current M13 terminal TUI behavior:
- `internal/tui` now owns a Nexdev-specific terminal control model separate from the imported legacy interview/review screens. It depends on a narrow `tui.Client` interface and reads status, events, plan/tasks, blockers, artifacts, redacted config, and provider summaries from an HTTP/control-plane client or an in-process control-plane handler client.
- `nexdev tui` launches the terminal client only. With `--control-url`, it talks to the remote control-plane URL with the configured bearer token. Without `--control-url`, it opens the project runtime, acquires the project lock, builds the same M10/M11 in-process server handler, and calls through that handler rather than reading or mutating state directly.
- The TUI exposes overview, event stream, plan/task, blocker/detour, and artifact/config/provider summary views with refresh and navigation keys. Plan editing, richer steering text input, provider testing, and provider/config mutation are shown as deferred/disabled states unless their services are wired.
- Pause/resume, skip, detour, steer, and cancel actions call the injected client/service path. The TUI does not call providers, execute shell commands, implement task ordering, or own pipeline state. Normal quit exits the terminal client without cancelling a run; cancel and skip require explicit confirmation.
- Displayed event/task/blocker/artifact text is treated as untrusted and passed through secret redaction/control-character scrubbing before rendering.
- Embedded web UI files remain unimplemented and are intentionally not created by M13 terminal TUI work.

Final stabilization notes:
- Documentation now treats M0-M19 plus TASK-01 through TASK-10 behavior as implemented at assigned scope.
- TASK-03 added checked-in generated OpenAPI types and drift tests. Current handlers remain manually bound; full OpenAPI response validation/server binding remains deferred.
- TASK-06 added git-diff anchored changed-file detection. Rename `old_path` is parsed internally but is not exposed by shared changed-file artifact JSON until a future contract extension.
- TASK-08 added hostile security fixtures; TASK-09 added HTTP slow-reader SSE stress coverage; TASK-10 completed root CLI cleanup.
- Remaining architecture work is limited to explicit deferrals: full real-provider pipeline execution, web UI assets, artifact content opening, full OpenAPI response validation/server binding, and the changed-file artifact `old_path` extension.

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
