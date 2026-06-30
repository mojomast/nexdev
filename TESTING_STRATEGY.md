# Nexdev Testing Strategy

**Status:** M0 bootstrap complete; M1 C1-C9 contract and fixture tests are verified.  
**Canonical requirements:** `SPEC.md` section 24 plus security and acceptance criteria sections.  
**Execution model:** Tests are created alongside implementation by domain workers.

## 1. Current Baseline

Current baseline after M0/M1 first-wave work:
- The repo is a Go module at `github.com/mojomast/nexdev`.
- Imported geoffrussy baseline tests exist and pass.
- M1 contract packages now have package-level tests.
- Shared black-box test fixture contracts live in `internal/testutil`.
- No CI workflows or fake-provider E2E scripts exist yet.

Current valid commands include:
- `go test ./internal/contract`
- `go test ./internal/config ./internal/safety`
- `go test ./internal/git`
- `go test ./internal/safety`
- `go test ./internal/pipeline`
- `go test ./internal/state`
- `go test ./internal/provider`
- `go test ./internal/executor ./internal/detour ./internal/steering`
- `go test ./internal/controlplane`
- `go test ./internal/testutil`
- `go test ./internal/observability`
- `go test ./internal/app ./internal/cli`
- `go test ./internal/tui`
- `go test ./...`
- `go vet ./...`
- `go mod verify`

Current orchestrator-verified M1 commands:
- `go test ./...`
- `go vet ./...`
- `go mod verify`

## 2. Required Final Gates

These are required for full implementation/release readiness; the fake-provider E2E script does not exist yet:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `govulncheck ./...`
- `go mod verify`
- `./scripts/e2e_fake_provider.sh`

Additional recommended commands:
- `make test`
- `make vet`
- `make race`
- `make vuln`
- `make contract`
- `make smoke`
- `make ci`

Recommended commands must not be documented as valid until their files exist.

## 3. Test Layers

### 3.1 Unit Tests

Required package-level tests:
- Config defaults, precedence, unknown-key rejection, profile validation.
- Path sanitizer clean/absolute/root checks.
- Symlink escape rejection.
- Project lock path, acquire/release, contention, reacquire, and symlink escape rejection.
- Deny glob and expected-file enforcement.
- Redaction of API keys, bearer tokens, passwords, private keys, SSH keys, `.env` values, and known secret patterns.
- Auth token hashing, constant-time compare, expiry, revocation, role checks.
- Event envelope creation and event type constants.
- Event log replay query behavior.
- SSE frame formatting.
- Stage prerequisite validation.
- Stage status transitions.
- Plan DAG validation.
- Detour splicing.
- Steering context selection and summarization.
- Provider routing and structured output repair.
- SQLite migrations.

### 3.2 Integration Tests

Required integration tests:
- Fake provider full pre-development pipeline.
- Fake provider invalid JSON repair.
- Pause/resume/skip during develop.
- SSE reconnect with `Last-Event-ID`.
- Auth observer/operator/admin matrix.
- Blocker to detour to resume.
- Review plan edit to plan version increment.
- Verify failure to repair attempt.
- Migration from seeded geoffrussy-like state.
- Control-plane route behavior against OpenAPI schemas.
- CLI local mode and remote `--control-url` mode for implemented commands.

### 3.3 Golden-File Tests

Golden files are for stable contracts only.

Use golden tests for:
- `devplan.md` artifact rendering.
- `phaseNNN.md` artifact rendering.
- `handoff.md` artifact rendering.
- Redacted config output.
- SSE frame examples.
- ErrorResponse examples.
- Prompt context assembly with trusted and untrusted sections.
- Changed-files manifest ordering and shape.

Golden tests must normalize:
- IDs.
- Timestamps.
- Temp paths.
- Provider latency.
- Nondeterministic map ordering.

Golden updates require an explicit environment variable such as `NEXDEV_UPDATE_GOLDEN=1`.

### 3.4 API Contract Tests

`api/openapi.yaml` is authoritative once created.

Current M1 first-wave command:
- `go test ./internal/contract`

Current first-wave coverage:
- Parses `api/openapi.yaml` with the existing YAML dependency.
- Verifies every required `SPEC.md` section 12.2 route exists.
- Verifies `x-nexdev-role` metadata for each route.
- Verifies common schema names required by the first-wave contract task.
- Verifies event contract version, required event names, and event sources.

Current codegen status:
- Generated OpenAPI codegen remains deferred; `api/openapi.yaml` is the machine contract until the owning API/codegen task creates generated types and drift checks.

Required checks:
- OpenAPI file validates.
- Generated Go types compile after the M1 integration/codegen path is settled.
- Every implemented route has request/response tests.
- Every JSON error uses `ErrorResponse`.
- Mutating routes require the expected role.
- OpenAPI generation/check has no diff in CI.
- MCP schemas match OpenAPI schemas where possible.

### 3.5 SSE Tests

Required checks:
- Event is persisted before broadcast.
- Event sequence is monotonic per run.
- `Last-Event-ID` replays missed events after the supplied event.
- Heartbeats are sent at configured interval.
- Heartbeats do not corrupt event sequence semantics.
- Slow clients cannot grow unbounded memory.
- Queue overflow emits `sse_client_slow` if possible, then closes.
- SSE frames include `id`, `event`, `retry`, and `data`.
- SSE never sends `data: [DONE]`.

### 3.6 SQLite Migration Tests

Required checks:
- Empty DB migrates to latest.
- Seeded geoffrussy-compatible DB migrates additively.
- Re-running migrations is safe.
- Foreign keys are enabled.
- WAL is enabled.
- Busy timeout is configured.
- Bounded transaction and retry behavior handles write contention.
- Required tables and indexes exist.
- Event `(run_id, sequence)` uniqueness holds under concurrency.

Current M1-C5 coverage:
- `go test ./internal/state` covers empty migration to latest, idempotency through the existing migration tests, seeded geoffrussy-compatible migration from version `3` to Nexdev version `4`, required Nexdev table/index existence, event `(run_id, sequence)` uniqueness, foreign-key enforcement, WAL mode, and configured busy timeout.

Current M3 event repository coverage:
- `go test ./internal/state` covers persisted event load with contract version and UTC RFC3339Nano timestamp behavior, monotonic per-run sequence allocation, independent sequences across runs, replay after sequence, replay after event ID for `Last-Event-ID` mapping, unsafe caller-provided sequence rejection, duplicate event ID failure, and concurrent publishers on one run.
- M10 SSE tests still need to cover persist-before-broadcast at the publisher boundary, HTTP `Last-Event-ID` reconnect behavior, heartbeat frames, bounded client queues, and slow-client overflow handling.

Current M3 run/stage/artifact repository coverage:
- `go test ./internal/state` covers run create/read/status/current-stage/complete/cancel/list behavior, UTC RFC3339Nano timestamp normalization, metadata JSON round-trip, stable run ordering, and run foreign-key enforcement.
- `go test ./internal/state` covers stage-run create/read/status/output/error/attempt/complete/list behavior, default attempt handling, output/error JSON round-trip, stable stage-run ordering, and stage-run foreign-key enforcement.
- `go test ./internal/state` covers artifact upsert/get/list behavior, project/run/kind filters, metadata JSON round-trip, UTC timestamp normalization, stable artifact ordering, and project/run foreign-key enforcement.
- M5/M7 integration tests still need to cover runner status-transition enforcement, artifact file writing, artifact schema validation, `artifact_updated` event emission, and full pipeline artifact indexing.

Current M3 auxiliary repository coverage:
- `go test ./internal/state` covers auth token create/get/get-by-hash/list/revoke/touch behavior, unique token hash enforcement, UTC RFC3339Nano timestamp normalization, and stable token ordering. Auth hashing, constant-time compare, role middleware, expiry rejection, and token plaintext generation remain M10 auth/security coverage.
- `go test ./internal/state` covers steering append/list by run and task, message/summary/source/created-role preservation, UTC timestamp normalization, stable ordering, and project/run foreign-key enforcement. Steering summarization and prompt context selection remain M8 coverage.
- `go test ./internal/state` covers detour create/list by run and trigger, result JSON validation/round-trip, source/depth/reason preservation, UTC timestamp normalization, stable ordering, and project/run foreign-key enforcement. Detour generation, blocker/depth policy, and splice integration remain M9 coverage.
- `go test ./internal/state` covers navigation append/list by project and run, from/to/reason/actor preservation, UTC timestamp normalization, stable ordering, and project/run foreign-key enforcement. Stage prerequisite enforcement remains M5/M10 coverage.
- `go test ./internal/state` covers plan edit create/list by run, plan version before/after, edit type, target, patch JSON validation/round-trip, actor preservation, UTC timestamp normalization, stable ordering, and project/run foreign-key enforcement. Review mutation/version-increment integration and `plan_updated` event emission remain M7/M10 coverage.

Current M3 task/blocker follow-up coverage:
- `go test ./internal/state` covers additive migration version `5` for `nexdev_tasks` and `nexdev_blockers`, required indexes, migration from seeded geoffrussy-compatible state, and preservation of legacy `tasks` and `blockers` rows.
- `go test ./internal/state` covers Nexdev task create/list/get-status-adjacent behavior through create/list/status update, `contract.TaskSpec` JSON slice round-trips, stable `plan_version`/`plan_order` listing, dependency reference validation, self/missing dependency rejection, acceptance-criteria validation, FK behavior, and unique `(run_id, plan_version, plan_order)` behavior.
- `go test ./internal/state` covers Nexdev blocker create/list/resolve behavior, task/run/status filters, metadata JSON validation/round-trip, UTC timestamp normalization, FK behavior, and missing-row resolve failure.

### 3.7 CLI Smoke Tests

Required as commands become implemented:
- `nexdev doctor`
- `nexdev config validate`
- `nexdev init`
- `nexdev run --fake-provider --no-tui --json`
- `nexdev serve`
- `nexdev status --json`
- `nexdev events --follow`
- `nexdev auth token create`
- `nexdev auth token list`
- `nexdev auth token revoke`

Current M12 CLI/app coverage:
- `go test ./internal/app` covers project-local runtime opening, `.nexdev/run/project.lock` acquire/release for server lifecycle, project row creation, non-loopback/no-auth startup rejection through config/app validation, and hash-only token persistence.
- `go test ./internal/cli` covers M12 command registration, root command identity, token create JSON output, and control mutation commands refusing to bypass the HTTP control-plane service when `--control-url` is absent.
- Remaining CLI coverage: `serve` live listener smoke, remote `events --follow` SSE client, full `run --fake-provider --no-tui --json`, provider-test service execution, verify/handoff commands, and E2E fake-provider script.

### 3.8 Fake Provider and Fake Worker Tests

Fake provider requirements:
- Deterministic responses by stage.
- Scripted valid structured outputs.
- Scripted invalid JSON followed by repairable response.
- Scripted unrecoverable invalid response.
- Provider latency simulation.
- Retryable error simulation.
- Hard failure simulation.
- Usage/cost metadata.
- Streaming chunks for develop simulation.

Fake worker requirements:
- Scripted task updates.
- Safe expected-file writes.
- Unexpected-file write attempts.
- Structured blocker emission.
- Verify failure emission.
- Pause/resume/cancel handling.

### 3.9 Security Tests

Required fixtures:
- Prompt injection in README.
- Malicious `AGENTS.md` attempting to reveal secrets or override safety.
- Symlink escape tree.
- Fake `.env` with secret-looking values.
- Unauthorized role mutation attempts.
- Unbounded command output attempt.
- Slow SSE client.
- MCP tool description poisoning.

Required assertions:
- Repo instructions cannot override safety policy.
- `.env` content is not sent to prompts, logs, events, or API responses.
- Symlink escapes are rejected.
- `.git` writes are rejected.
- Shell commands are denied unless exact policy allows them.
- Remote bind without auth fails startup.
- Wildcard CORS is forbidden outside dev.
- MCP descriptions cannot expand permissions.

### 3.10 Race and Concurrency Tests

Required coverage:
- Concurrent event publishers and SSE subscribers.
- Concurrent auth checks and token revocation.
- Concurrent pause/resume/cancel controls.
- Project lock prevents multiple mutating processes.
- SQLite busy retry behavior under write contention.
- Worker assignment detects file overlap when parallel worktrees are enabled.

Current M2 project lock coverage:
- `go test ./internal/git` covers `.nexdev/run/project.lock` resolution, exclusive acquisition, pid/timestamp metadata, second-acquire failure while held, release/reacquire behavior, and symlink escape rejection where the platform permits symlinks.

Run `go test -race ./...` as a full gate. If a package is excluded, record the reason in `DEVPLAN.md` and spec-management handoff.

### 3.11 Optional Real-Provider Smoke Tests

Real-provider tests must be:
- Disabled by default.
- Enabled only with explicit env vars.
- Tiny.
- Spend-capped.
- Free of repo secrets.
- Skipped in normal CI.

Recommended env gates:
- `NEXDEV_REAL_PROVIDER=anthropic|openai|local`
- `NEXDEV_REAL_PROVIDER_SMOKE=1`
- Provider-specific API key env var.
- `NEXDEV_REAL_PROVIDER_MAX_USD=0.25`

## 4. Shared Test Fixtures

Use a shared test helper package only for black-box helpers. Production code must not import test fixtures.

Current shared package:
- `internal/testutil`

Implemented M1-C9 helpers:
- `TempProject(t)` creates a minimal temp project tree with `nexdev.yaml`, `README.md`, `.nexdev/artifacts`, `.nexdev/state`, loopback control-plane default, and no `.env` or secret-bearing files.
- `FakeClock` exposes deterministic UTC `Now`, `Set`, and `Advance` methods.
- `FakeIDGenerator` creates stable sortable fixture IDs for project, run, event, token, and caller-supplied prefixes.
- `EventRecorder` records `internal/contract.EventEnvelope` values, returns copies, sorts by sequence, and asserts sequence/type ordering.
- Auth role helpers expose current `internal/controlplane` role fixtures and route role lookup without token storage behavior.

Deferred shared helpers:
- `TempSQLiteStore(t)`.
- `SeedGeoffrussyState(t, db)`.
- `FakeWorker`.
- `SSEClient` with reconnect and `Last-Event-ID`.
- `AuthTokens` helper that creates observer/operator/admin tokens through public APIs after token repositories exist.
- `GoldenPath(t, name)`.
- OpenAPI request/response validation helper.

Provider-owned fake helper:
- `internal/provider.FakeProvider` is the canonical deterministic fake provider for package, pipeline, and E2E tests. It stays out of `internal/testutil` unless a later black-box adapter is needed.

Fixture rules:
- Tests may use fixture helpers.
- Production code may not.
- Shared fixtures must not expose package internals.
- Security fixture repos must be treated as hostile input.

Current fixture test command:
- `go test ./internal/testutil`

## 5. Per-Domain Acceptance Tests

### Config and Paths

- Defaults load without project config.
- Precedence follows spec order.
- Unknown top-level keys reject unless experimental override is enabled.
- `trusted-lan` remote bind without auth fails.
- Traversal and symlink escape writes fail.
- Current M2 baseline coverage: `go test ./internal/config ./internal/safety` validates typed Nexdev defaults/profile/auth-auto/remote-bind/unknown-top-level-key behavior and path traversal, absolute escape, `.git`, symlink escape, and basic deny-glob rejection.
- Current M2 security baseline coverage: `go test ./internal/safety` validates secret redaction, `.env` assignment scrubbing, bearer token scrubbing, private key scrubbing, prompt-injection warning detection, default deny for shell/network, write deny globs, and wildcard shell rejection outside `dev`.
- Remaining config/path/security coverage: full global/project/env/flag precedence wiring, unsafe CORS/profile combinations, app-level logger wiring, redaction integration for events/prompts/API, prompt-injection `security_warning` events, tool policy file loading, executor/verify enforcement, command output caps, controlled env, file locks, task expected-file enforcement, MCP poisoning fixtures, and audit logs after owning integrations exist.

### State

- Latest schema applies from empty DB.
- Existing geoffrussy-compatible state migrates.
- Required tables/indexes exist.
- WAL/FK/busy timeout enabled.
- Event sequence is monotonic per run under serial and concurrent publishers.
- Event replay supports after-sequence and after-event-ID queries for later `/events` and SSE reconnect handlers.
- Run, stage-run, and artifact repositories cover create/read/update/list paths and preserve JSON metadata/output/error fields.
- Auth token, steering, detour, navigation, and plan edit repositories cover repository-level persistence, stable ordering, UTC timestamps, JSON round-trips where applicable, and FK/unique constraints.
- Pipeline runner, artifact file writer, auth middleware, detour manager, review editor, and control-plane snapshot tests remain later milestone coverage.

### Provider

- Per-stage router chooses configured provider.
- Empty stage provider inherits primary.
- Structured output rejects malformed data.
- Repair attempts are capped.
- Usage/cost metadata is recorded.
- Provider errors are redacted.
- Current M4 provider coverage: `go test ./internal/provider` covers valid decode, unknown-field rejection, repair success, repair cap failure, semantic validation failure without destination mutation, slot/model resolution through `Router.Resolve`, usage metadata capture from `provider.Response`, redacted provider errors, deterministic fake-provider calls by model/prompt, structured repair through `StructuredClient`, unrecoverable invalid fake responses, retryable and hard scripted errors, usage metadata, streaming chunks, model listing/discovery, optional authentication behavior, deterministic latency records without sleeps, and fake-provider disabled-by-default registry behavior.
- Remaining M4/M16 provider coverage: end-to-end fake-provider integration after pipeline stages, CLI run wiring, SSE replay, artifact checks, and fake-provider E2E script coverage.

### Pipeline

- Fake provider completes `repo_analyze` through `handoff`.
- Every stage persists status.
- Every stage writes/indexes required artifacts.
- Invalid navigation is rejected.
- Resume uses persisted state.
- Current M5 runner coverage: `go test ./internal/pipeline` covers canonical fake-stage execution order, prerequisite rejection through `ValidatePrerequisites`, status transition enforcement against persisted stage state, resume selection from `runs.current_stage` and `stage_runs`, completed/skipped checkpoint output JSON, blocked/failed error persistence, and persisted `stage_status`/`done` events through the M3 event repository.
- Current M6 repo_analyze coverage: `go test ./internal/pipeline` covers deterministic language/framework/package-manager/test/lint/entrypoint detection, bounded excludes for `.git`/`.nexdev`/`node_modules`/`vendor`/large files, untrusted instruction capture with prompt-injection risk notes, `.env`/secret file avoidance, `.nexdev/artifacts/repo_analysis.json` JSON writing, artifact repository indexing when `*state.Store` is available, and `PipelineStage`/`StageOutputter` behavior including deterministic `Resume`.
- Current M6 interview/complexity coverage: `go test ./internal/pipeline` covers provider-backed valid interview with trusted/untrusted prompt sections, underspecified interview blocking, yes/CI assumption conversion, invalid structured-output repair through `StructuredClient`, deterministic complexity levels, provider complexity refinement with deterministic verification floor enforcement, `.nexdev/artifacts/interview.json` and `.nexdev/artifacts/complexity_profile.json` JSON writing/indexing, and `PipelineStage`/`StageOutputter` resume/output behavior.
- Current M6 design coverage: `go test ./internal/pipeline` covers provider-backed design generation through `provider.SlotDesign`, trusted/untrusted prompt sections, correction loop prompt feedback, max-iteration `BlockedError` for unresolved high-severity actionable findings, invalid structured-output repair, required section validation, `.nexdev/artifacts/design_draft.md` writing/indexing, and `PipelineStage`/`StageOutputter` resume/output behavior.
- Current M6 hivemind/validate coverage: `go test ./internal/pipeline` covers provider-backed hivemind voice and synthesis calls through `provider.SlotHivemindVoice` and `provider.SlotHivemindSynthesis`, configured sequential and bounded-parallel voice behavior, security-voice prompt focus, revise/block `BlockedError` behavior after artifact persistence, invalid structured-output repair, `.nexdev/artifacts/design_review.json` writing/indexing, provider-backed validation through `provider.SlotValidate`, pass/warn/block behavior, conflict/blocker default blocking, `.nexdev/artifacts/validation_report.json` and `.nexdev/artifacts/validated_design.md` writing/indexing, and `PipelineStage`/`StageOutputter` resume/output behavior.
- Remaining M6/M7 pipeline coverage: shared artifact helper and `artifact_updated` events, state repositories for `hivemind_results` and `validate_results`, schema validation for later stage artifacts, runner/app wiring that passes stage outputs into constructors, and full fake-provider pre-development pipeline through `handoff`.

### Review and Planning

- Tasks require acceptance criteria.
- Write tasks require expected files.
- Missing dependencies fail.
- Cycles fail.
- Manual edit writes `plan_edit_events` and increments plan version.
- Reviewed tasks persist through `nexdev_tasks` after plan validation; repository tests cover dependency references, but M7 remains responsible for cycle detection and plan mutation/version semantics.
- Current M7 planning coverage: `go test ./internal/pipeline` covers provider-backed `plan_sketch` through `provider.SlotPlanSketch`, deterministic phase numbering and deduplication, invalid structured-output repair, provider-backed `plan_detail` through `provider.SlotPlanDetail`, task validation failures for acceptance criteria/expected files/missing dependencies, dependency cycle rejection, deterministic `devplan.json`/`devplan.md`/`phaseNNN.md` rendering, artifact indexing, pending `nexdev_tasks` persistence with stable plan version/order, and `PipelineStage`/`StageOutputter` resume/output behavior.
- Current M7 review coverage: `go test ./internal/pipeline` covers manual `review_required` blocking, approval marker artifact/stage output for develop prerequisites, auto approval, CI rejection for high-risk tasks without tests, skip mode requiring explicit allowance, task update version increment with `plan_edit_events`, delete-pending-task version increment with dependency safety, non-pending edit rejection, and `PipelineStage`/`StageOutputter` resume behavior.
- Remaining review coverage: control-plane and TUI clients must call the same review service path for edits/approval, emit `review_required`, `review_completed`, and `plan_updated` events after persistence, and enforce roles once M10/M12 own those surfaces.

### Executor and Steering

- Fake task emits task events.
- Unexpected writes fail.
- Steering affects next prompt context.
- Steering cannot override safety policy.
- Pause/resume/cancel are context-aware.
- Executor integration loads `nexdev_tasks`, updates task status through the state repository, and writes blockers to `nexdev_blockers` rather than legacy geoffrussy task/blocker tables.
- Current M8 coverage: `go test ./internal/executor ./internal/pipeline` covers fake task completion with persisted status/events, expected-file write allowance, unexpected write rejection, task-blocker event mapping plus `nexdev_blockers` creation, pause/resume/skip/cancel/current-task behavior, steering persistence into prompt context, and develop-stage review approval marker enforcement.
- Remaining executor coverage: richer steering context assembled from requirements/design/repo artifacts, control-plane handler integration, project-lock lifecycle wiring, real worker/worktree strategy tests, shell/network policy enforcement if those tools are later implemented, and changed-file manifest/report population.

### Detour

- Blocker creates detour request.
- Detour tasks validate against `TaskSpec`.
- Tasks splice after trigger task.
- Depth exceeded creates blocker and pauses.
- Detour integration uses `nexdev_blockers` and `nexdev_tasks`; repository tests cover persistence only, while M9 owns task splice/depth policy behavior.
- Current M9 coverage: `go test ./internal/detour` covers blocker-triggered requests from `nexdev_blockers`, structured fake-provider generation and repair, M7-equivalent task validation, immediate splice ordering after the trigger for gapped and dense plans, ID conflict detection, `detour_records` persistence, `detour_created` event persistence, trigger `pending_after_detour` status, and depth-exceeded blocker/no-silent-skip behavior.
- Current dense-order state coverage: `go test ./internal/state` covers transactional insertion after a trigger task in a dense plan, multiple inserted detour tasks, shifted later task orders with stable relative order, dependency validation across inserted tasks, and stable plan version metadata.
- Remaining detour coverage: M10/M12 should cover HTTP/CLI/TUI adapters, role checks, operator approval policy if enabled, and automatic resume if assigned.

### Control Plane and Auth

- `/health` works unauthenticated.
- Observer reads status/plan/artifacts/events/stream.
- Operator can pause/resume/skip/steer/detour/provider-test.
- Admin can cancel/config/task-mutate/token-manage.
- JSON errors use `ErrorResponse`.
- Remote bind without auth fails.
- Current M10 coverage: `go test ./internal/controlplane` covers loopback/no-auth health and status, startup rejection for non-loopback/no-auth, bearer-token observer/operator role behavior, forbidden observer mutation, SSE persisted replay with `Last-Event-ID`, SSE frame shape/no `[DONE]`, and detour route delegation with persisted `detour_created` event evidence.
- Remaining M10/M12/M15 coverage: CLI token create/list/revoke commands, slow-client overflow event behavior under stress, full OpenAPI response validation, app-level `nexdev serve` wiring, task/config/provider mutation route service wiring, MCP per-tool dispatch, auth failure audit/rate limiting, and token rotation UX.
- Current M12 app/CLI coverage adds project-local `nexdev serve` lifecycle wiring, project lock acquire/release, token create/list/revoke command paths, and CLI mutation error handling. Remaining M12/M15 coverage: live listener smoke, remote SSE follow, slow-client overflow stress, provider-test service wiring, and full OpenAPI response validation.

### Observability

- Logger construction supports JSON and text modes.
- Level filtering suppresses lower-severity records.
- Log messages and string attributes are redacted through `internal/safety.RedactSecrets` before write.
- Field helpers emit the canonical `SPEC.md` section 17 keys.
- Current M2 coverage: `go test ./internal/observability` validates redaction for messages, attrs, grouped/with attrs, level filtering, JSON/text construction, level parsing, and required field helper keys.
- Current M14 coverage: `go test ./internal/observability` validates disabled-by-default OTel behavior, endpoint validation when enabled, correlation-aware provider usage recording, cost estimation, cost ledger persistence, audit record persistence, and metadata redaction. `go test ./internal/state` validates audit/cost migrations and repositories plus event-payload redaction before persistence. `go test ./internal/provider` validates structured-call usage recorder hooks and redacted raw-response handoff. `go test ./internal/controlplane` validates auth failure/forbidden/operator control audit records and recursive error-detail redaction coverage through existing error paths.
- Remaining M14/M15 coverage: broader no-secret-leak integration tests across logs/events/artifacts/prompts/API/error responses/audit/cost using hostile fixture repos, cost guard enforcement before parallel launches, run-summary cost aggregation, and optional OTel exporter tests that remain disabled and network-free by default.

### MCP

- Required tool list exists.
- Tool schemas validate.
- MCP calls enforce same role checks as HTTP.
- MCP stdio mode does not shell-execute arbitrary strings.
- Current M11 coverage: `go test ./internal/controlplane` validates the required `nexdev_*` tool descriptor set, static roles, schema rejection of unknown/wrong-typed inputs, observer/operator/admin per-tool role enforcement, durable status/plan/artifact metadata reads, detour `RequestForBlocker` delegation, steering source `mcp`, blocker resolution, optional provider-test redaction, and structured MCP error shape.
- Current compatibility check: `go test ./internal/mcp` still passes for the legacy imported package and verifies that legacy stdio tool/resource registration is disabled until adapted to the M11 service boundary.
- Remaining MCP coverage: app/CLI stdio wiring after legacy cleanup, artifact file content reads through a validated project-root reader, provider-test service integration, and end-to-end MCP over a served control plane.

### Terminal TUI

- The TUI must be tested as a client over fake/control-plane service abstractions, not by owning pipeline state.
- Current M13 coverage: `go test ./internal/tui` covers model refresh/update, required view rendering against fake run state, key navigation, disabled/deferred action status, redaction of secret-like event/blocker text, normal quit not cancelling a run, and explicit confirmation before skip/cancel actions invoke the client.
- Current CLI coverage: `go test ./internal/cli` verifies the `tui` command is registered with the required command tree.
- Remaining TUI coverage: live terminal smoke against `nexdev serve`, richer text input for steering/detour/review edits once those service paths exist, and end-to-end remote TUI auth behavior.
- Web UI remains deferred and has no test surface in M13.

### Verify and Handoff

- Allowed commands run with timeout/output cap.
- Denied commands do not run.
- Failed verify creates report and repair path.
- Handoff includes request, changes, commands, and risks.
- `changed_files.json` includes path, status, sha256, byte size, and owning tasks.

## 6. CI and Release Gates

PR gate once implementation exists:
- `go test ./...`
- `go vet ./...`
- OpenAPI generation/check.
- Unit and integration tests with fake provider.
- Security fixture tests.
- SQLite migration tests.

Nightly/full gate:
- `go test -race ./...`
- `govulncheck ./...`
- Fake-provider E2E.
- CLI smoke from built binary.
- SSE replay/slow-client stress tests.
- Migration tests from seeded legacy state.

Release gate:
- All PR and nightly gates.
- `go mod verify`.
- Reproducible binary build.
- Optional SBOM/checksum/signature if distributing binaries.
- Optional real-provider smoke with env gate and spend cap.
- Docs complete.
- Spec coverage complete.
