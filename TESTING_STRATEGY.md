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
- `FakeProvider`.
- `FakeWorker`.
- `SSEClient` with reconnect and `Last-Event-ID`.
- `AuthTokens` helper that creates observer/operator/admin tokens through public APIs after token repositories exist.
- `GoldenPath(t, name)`.
- OpenAPI request/response validation helper.

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

### Pipeline

- Fake provider completes `repo_analyze` through `handoff`.
- Every stage persists status.
- Every stage writes/indexes required artifacts.
- Invalid navigation is rejected.
- Resume uses persisted state.

### Review and Planning

- Tasks require acceptance criteria.
- Write tasks require expected files.
- Missing dependencies fail.
- Cycles fail.
- Manual edit writes `plan_edit_events` and increments plan version.

### Executor and Steering

- Fake task emits task events.
- Unexpected writes fail.
- Steering affects next prompt context.
- Steering cannot override safety policy.
- Pause/resume/cancel are context-aware.

### Detour

- Blocker creates detour request.
- Detour tasks validate against `TaskSpec`.
- Tasks splice after trigger task.
- Depth exceeded creates blocker and pauses.

### Control Plane and Auth

- `/health` works unauthenticated.
- Observer reads status/plan/artifacts/events/stream.
- Operator can pause/resume/skip/steer/detour/provider-test.
- Admin can cancel/config/task-mutate/token-manage.
- JSON errors use `ErrorResponse`.
- Remote bind without auth fails.

### Observability

- Logger construction supports JSON and text modes.
- Level filtering suppresses lower-severity records.
- Log messages and string attributes are redacted through `internal/safety.RedactSecrets` before write.
- Field helpers emit the canonical `SPEC.md` section 17 keys.
- Current M2 coverage: `go test ./internal/observability` validates redaction for messages, attrs, grouped/with attrs, level filtering, JSON/text construction, level parsing, and required field helper keys.
- Remaining M14 coverage: OTel disabled-by-default wiring after config integration, request/event correlation, provider usage/cost records, audit logs, and cross-boundary no-secret-leak integration tests for logs/events/artifacts/prompts/API.

### MCP

- Required tool list exists.
- Tool schemas validate.
- MCP calls enforce same role checks as HTTP.
- MCP stdio mode does not shell-execute arbitrary strings.

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
