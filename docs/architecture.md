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

The pipeline runner owns:
- Stage registry.
- Stage prerequisites.
- Status transitions.
- Resumption from persisted state.
- Checkpointing after each stage.

Stages must be resumable and must persist outputs.

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
- Structured output calls, usage/cost audit hooks, and fake-provider scripting are still M4 follow-up work.

Structured output flow:
1. Build prompt with trusted/untrusted sections and schema.
2. Route through configured provider slot.
3. Strict-decode response.
4. Validate semantic constraints.
5. Repair if allowed.
6. Persist raw response and validation errors for audit when configured.
7. Return typed contract object.

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
