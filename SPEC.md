# Nexdev v0.1 Implementation Specification

**Status:** Implementation contract draft  
**Target date:** June 2026  
**Primary implementation language:** Go 1.24+  
**Repository strategy:** Fork `mojomast/geoffrussy`, add Nexdev packages, keep backward-compatible state migrations where possible.  
**Product thesis:** Nexdev is a single-binary, local-first, observable, steerable coding harness that turns a project request into reviewed, tested, auditable code by combining geoffrussy's Go execution/state foundation, devussy's pre-development planning pipeline, and nexussy's live control-plane and operator ergonomics.

---

## 0. Executive Summary

Nexdev is not a chat app. It is a software-delivery harness with a staged product-development pipeline and an execution engine. The user gives Nexdev a project request or points Nexdev at an existing repository. Nexdev interviews for missing requirements, analyzes repository context, generates architecture, runs a multi-voice critique, validates the design, decomposes the work into phases and concrete tasks, allows a human review gate, executes tasks through a controlled provider/tool layer, verifies results, writes anchored artifacts, and exposes the run over CLI, TUI, HTTP/SSE, and MCP.

Nexdev must be built as a Go application, not a Python service rewrite. It must fork geoffrussy because geoffrussy already has the strongest foundations: a Go provider registry, SQLite-backed state, navigation primitives, a task executor, path safety, Bubble Tea TUI dependencies, Cobra CLI, and a migration system. Nexdev then ports the useful patterns from devussy and nexussy into this Go foundation.

Major changes from the uploaded draft:

1. Detour is specified as a new first-class package. The current public geoffrussy repository does not contain the standalone `detour.go` described in the draft, so implementation must not assume reusable Detour code exists.
2. The pre-development pipeline is expanded with `repo_analyze`, `complexity`, `verify`, and `handoff` stages. These are required for a next-generation harness because existing-code context, adaptive task sizing, test verification, and continuation artifacts are core to reliable agentic development.
3. SSE is upgraded from a stateless fanout bus to a persisted event log with replay, heartbeat, bounded per-client queues, and `Last-Event-ID` handling. A stateless SSE stream is too lossy for CI, remote dashboards, and restart/reconnect workflows.
4. Auth is changed from homegrown role-in-token strings to a safer default: opaque bearer tokens stored as salted hashes in SQLite with server-side role, expiry, and revocation. Stateless HMAC/JWS tokens are optional, not default.
5. The OpenAPI contract is promoted to a first-class artifact. REST types must be generated from `api/openapi.yaml` using `oapi-codegen` or a comparable tool to avoid drift.
6. Security is expanded around prompt injection, tool poisoning, untrusted MCP/tool descriptions, path traversal, symlink escapes, command execution, secret leakage, supply chain risk, and unbounded spend.
7. Package recommendations are included only where they simplify the build without weakening control: standard `net/http` ServeMux for routes, Cobra for CLI, Bubble Tea/Lipgloss for TUI, sqlc for typed SQL, goose or existing migrations for schema, OpenTelemetry for tracing/metrics/log correlation, govulncheck in CI, and oapi-codegen/kin-openapi for API contracts.

---

## 1. Non-Negotiable Build Rules

1. Nexdev MUST be local-first. By default, all HTTP services bind to `127.0.0.1` only.
2. Nexdev MUST NOT run untrusted generated commands by default. Command execution requires an explicit tool permission policy.
3. Nexdev MUST treat repository files, MCP tool descriptions, issue text, docs, comments, test fixtures, and model responses as untrusted input.
4. Nexdev MUST preserve geoffrussy's provider registry pattern. No stage may call a model provider directly outside `internal/provider` or a wrapper around that interface.
5. Nexdev MUST persist all durable state in SQLite. Artifacts may also be written to disk for human/agent continuation, but SQLite is the source of truth for run status, events, tasks, blockers, steering, and audit.
6. SQLite MUST run with foreign keys, WAL, busy timeout, bounded transactions, and retry handling.
7. Every control-plane mutation MUST be authenticated when bind is not loopback. Binding to `0.0.0.0` without auth MUST fail startup.
8. Every external control request MUST pass role checks before mutation.
9. Every run MUST produce a replayable event stream. SSE events MUST be persisted before broadcast.
10. Every model-generated structured artifact MUST be schema validated and repaired or rejected before use.
11. Every task that modifies files MUST pass path sanitizer checks, symlink escape checks, file lock checks, and policy checks.
12. Every stage MUST be resumable from persisted state.
13. Every provider call MUST record model name, provider name, latency, retries, token usage when available, cost estimate when configured, and redacted error metadata.
14. Logs, events, artifacts, prompts, and error reports MUST scrub provider API keys, bearer tokens, passwords, private keys, SSH keys, `.env` values, and known secret patterns.
15. Nexdev MUST include deterministic fake-provider and fake-worker modes for CI.
16. No implementation agent may use source project internals as implicit contract. This specification is the contract.

---

## 2. Product Definition

### 2.1 What Nexdev Is

Nexdev is a staged coding harness for building or modifying software repositories with a controlled LLM pipeline. It covers:

- ideation and requirement clarification;
- repository and constraint analysis;
- architecture and design generation;
- multi-perspective critique;
- design sanity validation;
- phased planning and task decomposition;
- human review and inline plan edits;
- controlled task execution;
- detours when blockers appear;
- verification through tests/lints/build commands;
- artifact, event, audit, and handoff generation;
- live steering through CLI, TUI, HTTP, SSE, and MCP.

### 2.2 What Nexdev Is Not

Nexdev is not:

- a cloud SaaS;
- an uncontrolled auto-coder that silently runs shell commands;
- a replacement for human code review on high-risk changes;
- a provider-specific wrapper;
- a prompt-only toy;
- a best-effort chat transcript with no durable state.

### 2.3 Target Users

1. Solo developers who want a local coding harness that can plan and execute multi-stage work.
2. Small trusted teams running on LAN/VPN with an operator-controlled executor.
3. CI systems that need a reproducible coding-agent pipeline with a JSON/SSE control plane.
4. External AI agents that need an MCP-compatible surface to start, inspect, steer, and resume builds.

### 2.4 Design Philosophy

Nexdev is allowed to feel fast, weird, ergonomic, and Ussy-coded in user-facing language. Machine contracts must remain typed, boring, versioned, and stable. The system can go brrrr only when it can also explain what it did, replay the event stream, and prove which files changed.

---

## 3. Source Synthesis

### 3.1 geoffrussy: Foundation

Use geoffrussy as the base repository. Preserve and extend the following:

- `internal/provider`: canonical provider abstraction and registry.
- `internal/state`: SQLite store, models, migrations, write retry discipline.
- `internal/navigation`: stage graph and prerequisite validation.
- `internal/executor`: task execution, update stream, pause/resume/skip/blocker behavior.
- `internal/git`: git manager and commit metadata pattern.
- TUI dependencies and CLI dependency stack where already present.

Important implementation note: the uploaded draft says geoffrussy contains a root `detour.go`. Current public geoffrussy did not expose that file in inspection. Nexdev must create `internal/detour` as new code, while preserving the Detour concept from the draft.

### 3.2 devussy: Pre-Development Pipeline

Port the concepts, not the Python code:

- adaptive interview;
- complexity profile;
- architecture generation;
- design correction loop;
- LLM sanity reviewer;
- two-tier devplan generation;
- detailed per-phase planning;
- HiveMind-style parallel generation/arbitration;
- checkpointed stage artifacts;
- anchored `devplan.md`, phase files, and `handoff.md`.

Nexdev's Go implementation should use typed Go structs and JSON schemas instead of regex-first parsing wherever possible. Regex parsing may exist only as a fallback repair path for legacy imports.

### 3.3 nexussy: Control Plane

Port the concepts, not the Python process model:

- HTTP control plane;
- persisted SSE event stream;
- live pause/resume/skip/cancel/steer/inject controls;
- blocker management;
- web/TUI as clients of the core API;
- MCP-compatible tool surface;
- run artifacts and changed-file manifests;
- replayable event/audit model;
- deployment profiles: `dev` and `trusted-lan`.

Do not port:

- Python Starlette service layout;
- Python worker subprocess model as the core executor;
- AgentRouter-specific header tricks or fake user-agent behavior;
- role YAML files on disk;
- separate per-stage Python processes.

---

## 4. Runtime Architecture

### 4.1 High-Level Diagram

```text
                           +---------------------------+
                           |        User / Agent        |
                           +-------------+-------------+
                                         |
                 +-----------------------+-----------------------+
                 |                                               |
                 v                                               v
      +----------------------+                          +----------------------+
      |  CLI / TUI clients   |                          |  MCP clients / CI    |
      +----------+-----------+                          +----------+-----------+
                 |                                                 |
                 | HTTP/SSE or in-process                           | HTTP/SSE/MCP
                 v                                                 v
+--------------------------------------------------------------------------------+
|                              nexdev single binary                               |
|                                                                                |
|  cmd/nexdev                                                                    |
|    |                                                                           |
|    +-- internal/controlplane  HTTP, SSE, OpenAPI, auth, MCP                    |
|    +-- internal/pipeline      repo_analyze -> interview -> ... -> handoff      |
|    +-- internal/executor      geoffrussy executor + steering + verifier hooks  |
|    +-- internal/detour        blocker/manual detour task splice manager        |
|    +-- internal/provider      provider registry and model calls                |
|    +-- internal/state         SQLite store, migrations, event log              |
|    +-- internal/git           commits, worktrees, changed-file manifests       |
|    +-- internal/safety        paths, symlinks, tool policy, redaction          |
|    +-- internal/observability logs, OTel, metrics, cost ledger                 |
|                                                                                |
+--------------------------------------------------------------------------------+
                 |                         |                         |
                 v                         v                         v
          SQLite state.db            Project worktree           Provider APIs
          + artifacts                + git worktrees            + local models
```

### 4.2 Runtime Modes

Nexdev MUST support these modes:

| Mode | Command | Behavior |
|---|---|---|
| Interactive full run | `nexdev run` | Starts pipeline, TUI by default if terminal is interactive, control plane if enabled. |
| Headless full run | `nexdev run --no-tui --json` | Writes JSON logs and events, suitable for CI. |
| Develop only | `nexdev develop` | Enforces prereqs, then runs pending tasks. |
| Daemon/control plane | `nexdev serve` | Starts HTTP/SSE/MCP server without automatically starting a run. |
| Single stage | `nexdev run --stage validate` | Runs exactly one stage and stops. |
| Resume | `nexdev run` | Continues from current persisted stage by default. |
| Doctor | `nexdev doctor` | Verifies providers, config, DB, git, tool policy, sandbox, ports, and deps. |

### 4.3 Single Binary, Multiple Surfaces

The binary may expose multiple surfaces, but core state is owned by a single process per project. Multi-process concurrent mutation is not supported in v0.1. Nexdev MUST acquire a project lock file under `.nexdev/run/project.lock` before mutating project state.

---

## 5. Repository Structure

```text
nexdev/
├── api/
│   ├── openapi.yaml                 # public HTTP contract
│   └── mcp_tools.json               # generated/validated MCP tool manifest
├── cmd/
│   └── nexdev/
│       └── main.go                  # single binary entrypoint
├── internal/
│   ├── app/
│   │   ├── app.go                   # dependency wiring
│   │   └── lifecycle.go             # start/stop project runtime
│   ├── config/
│   │   ├── config.go                # typed config structs
│   │   ├── loader.go                # defaults -> file -> env -> flags
│   │   └── validate.go              # config validation
│   ├── controlplane/
│   │   ├── server.go                # net/http server
│   │   ├── routes.go                # generated or hand-bound routes
│   │   ├── auth.go                  # token verification and role enforcement
│   │   ├── events.go                # event log + subscriber manager
│   │   ├── sse.go                   # SSE stream, replay, heartbeat
│   │   ├── mcp.go                   # MCP JSON-RPC bridge
│   │   └── errors.go                # ErrorResponse helpers
│   ├── contract/
│   │   ├── api_types.go             # generated from OpenAPI
│   │   ├── schemas.go               # JSON schema registry for model outputs
│   │   └── validate.go              # strict decode + repair helpers
│   ├── pipeline/
│   │   ├── stage.go                 # PipelineStage interface
│   │   ├── runner.go                # stage orchestration and resumption
│   │   ├── repo_analyze.go          # existing repository analysis
│   │   ├── interview.go             # requirements extraction
│   │   ├── complexity.go            # adaptive complexity scoring
│   │   ├── design.go                # architecture generation + correction loop
│   │   ├── hivemind.go              # multi-voice critique + synthesis
│   │   ├── validate.go              # sanity reviewer
│   │   ├── plan_sketch.go           # high-level phases
│   │   ├── plan_detail.go           # detailed tasks
│   │   ├── review.go                # human/LLM review gate
│   │   ├── develop.go               # executor bridge
│   │   ├── verify.go                # tests/lints/build validation
│   │   └── handoff.go               # continuation artifacts
│   ├── executor/                    # forked and extended from geoffrussy
│   │   ├── executor.go
│   │   ├── task_executor.go
│   │   ├── monitor.go
│   │   ├── steering.go
│   │   ├── prompts.go
│   │   └── verify_hooks.go
│   ├── detour/
│   │   ├── manager.go
│   │   ├── splice.go
│   │   └── prompts.go
│   ├── provider/                    # forked and extended from geoffrussy
│   │   ├── provider.go
│   │   ├── registry.go
│   │   ├── structured.go            # schema-constrained call helper
│   │   ├── usage.go                 # token/cost metadata
│   │   └── provider implementations
│   ├── state/
│   │   ├── store.go
│   │   ├── models.go
│   │   ├── queries.sql              # sqlc source, if sqlc adopted
│   │   ├── migrations.go            # existing or goose runner
│   │   └── migrations/
│   │       ├── 001_initial.sql
│   │       ├── 002_nexdev_events.sql
│   │       ├── 003_nexdev_pipeline.sql
│   │       └── 004_nexdev_security.sql
│   ├── safety/
│   │   ├── paths.go                 # path/symlink sanitizer
│   │   ├── redaction.go             # secret scrubbing
│   │   ├── policy.go                # tool permission policy
│   │   ├── sandbox.go               # sandbox strategy interface
│   │   └── prompt_injection.go      # untrusted-content guardrails
│   ├── git/
│   │   ├── manager.go
│   │   ├── worktree.go
│   │   └── changed_files.go
│   ├── observability/
│   │   ├── logger.go                # slog setup
│   │   ├── otel.go                  # optional OTel exporters
│   │   ├── metrics.go
│   │   └── cost.go
│   └── tui/
│       ├── app.go
│       ├── plan_editor.go
│       ├── run_view.go
│       └── detour_dialog.go
├── web/
│   └── static/                      # optional embedded dependency-free UI
├── docs/
│   ├── AGENTS.md
│   ├── SECURITY.md
│   ├── OPERATING.md
│   └── API.md
├── scripts/
│   ├── smoke.sh
│   ├── e2e_fake_provider.sh
│   └── release.sh
├── nexdev.yaml
├── go.mod
└── go.sum
```

---

## 6. Package Recommendations

Nexdev should keep dependencies boring and limited. Add packages only where they reduce drift, security risk, or boilerplate.

### 6.1 Required or Recommended

| Concern | Recommendation | Rationale |
|---|---|---|
| CLI | `github.com/spf13/cobra` | Already used by geoffrussy; good for subcommands and help. |
| TUI | `github.com/charmbracelet/bubbletea`, `bubbles`, `lipgloss` | Already used by geoffrussy; appropriate for terminal dashboards and plan editing. |
| YAML config | `gopkg.in/yaml.v3` plus strict custom validation | Already used; avoid global mutable config state. Viper may be used only if wrapped and tested. |
| HTTP routing | Go standard `net/http` ServeMux | Go 1.22+ supports method/path patterns and path values; avoid unnecessary router dependency for v0.1. |
| API generation | `github.com/oapi-codegen/oapi-codegen/v2` | Generate OpenAPI types/server wrappers to prevent HTTP contract drift. |
| OpenAPI validation | `github.com/getkin/kin-openapi` | Optional request/response validation in tests and dev mode. |
| SQLite driver | Keep `github.com/mattn/go-sqlite3` initially | Already in geoffrussy; only move to `modernc.org/sqlite` if CGO-free builds become a release requirement. |
| SQL access | `github.com/sqlc-dev/sqlc` | Optional but recommended for typed queries and fewer runtime SQL scan errors. |
| Migrations | Keep geoffrussy migration runner or adopt `github.com/pressly/goose/v3` | Goose simplifies embedded SQL migrations, status, and validation; custom runner is acceptable if kept small. |
| Observability | OpenTelemetry Go SDK and `slog` | Traces and metrics should use OTel; structured logs should use standard `log/slog`. |
| Token/JWS support | Prefer opaque tokens; if JWT/JWS is required use a vetted JOSE library | Do not hand-roll JWT. Avoid JWS unless stateless tokens are actually needed. |
| Vulnerability scanning | `govulncheck` in CI | Detect reachable vulnerable Go dependencies. |

### 6.2 Avoid by Default

| Package/Class | Reason |
|---|---|
| Heavy web frameworks | Standard `net/http` is enough for v0.1 routes. |
| ORMs | The state model is small and contract-heavy; typed SQL is easier to audit. |
| Homegrown JWT | Easy to get claims, expiry, alg, and key handling wrong. |
| Browser dependencies/CDNs in embedded UI | Nexdev must work offline by default. |
| Unbounded task queue frameworks | Single-project local execution should stay simple until multi-project daemon mode exists. |

---

## 7. Project Files and Runtime Paths

### 7.1 Per-Project Layout

By default, Nexdev operates in the current repository.

```text
<project>/
├── .nexdev/
│   ├── state.db
│   ├── run/
│   │   ├── project.lock
│   │   └── server.pid
│   ├── logs/
│   ├── artifacts/
│   │   ├── interview.json
│   │   ├── repo_analysis.json
│   │   ├── complexity_profile.json
│   │   ├── design_draft.md
│   │   ├── design_review.json
│   │   ├── validated_design.md
│   │   ├── validation_report.json
│   │   ├── devplan.json
│   │   ├── devplan.md
│   │   ├── phase001.md
│   │   ├── handoff.md
│   │   ├── verify_report.json
│   │   ├── changed_files.json
│   │   └── run_summary.json
│   └── workers/
│       └── <worker-id>/
└── nexdev.yaml
```

### 7.2 Global Optional Layout

Nexdev may maintain optional global defaults:

```text
~/.nexdev/
├── config.yaml
├── tokens.db
├── logs/
└── cache/
```

Global state MUST NOT be required for a project to run. Project-local state is authoritative.

---

## 8. IDs, Time, and Versioning

### 8.1 IDs

Use sortable IDs to simplify logs and event replay.

| ID | Format |
|---|---|
| `project_id` | `proj_` + ULID |
| `run_id` | `run_` + ULID |
| `stage_run_id` | `stage_` + ULID |
| `phase_id` | stable slug or `phase_` + ULID |
| `task_id` | `T<phase>.<seq>` for planned tasks, `D<depth>.<seq>` for detours, persisted unique text primary key |
| `event_id` | `evt_` + ULID |
| `blocker_id` | `blk_` + ULID |
| `token_id` | `tok_` + ULID |

### 8.2 Time

All persisted timestamps MUST be UTC ISO-8601/RFC3339 with nanosecond precision where available. SQLite stores as text or integer epoch consistently; do not mix within a table.

### 8.3 Contract Versioning

- API contract version starts at `nexdev-api-v1`.
- Event contract version starts at `nexdev-event-v1`.
- State schema uses integer migration versions.
- Breaking API or event changes require an explicit version bump.

---

## 9. Stage Graph

### 9.1 Canonical Stage Order

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

`detour` is a pseudo-stage reachable only from `develop` and always returns to `develop`.

```text
develop -> detour -> develop
```

### 9.2 Stage Interface

```go
type PipelineStage interface {
    Name() state.Stage
    Run(ctx context.Context, env StageEnv) error
    Validate(ctx context.Context, env StageEnv) error
    Resume(ctx context.Context, env StageEnv) error
}

type StageEnv struct {
    Project     *state.Project
    Run         *state.Run
    Store       *state.Store
    Providers   provider.Router
    Config      config.Config
    Events      controlplane.Publisher
    Git         *git.Manager
    Safety      safety.PolicyEngine
    Logger      *slog.Logger
}
```

### 9.3 Stage Status

Every stage run must transition through:

```text
pending -> running -> completed
pending -> skipped
running -> blocked
running -> failed
blocked -> running
failed -> running  # retry/resume
running -> cancelled
```

### 9.4 Navigation Rules

Navigation MUST be validated with prerequisites:

| Target | Prerequisites |
|---|---|
| `repo_analyze` | project exists |
| `interview` | project exists |
| `complexity` | interview data exists |
| `design` | interview data and repo analysis exist |
| `hivemind` | design draft exists |
| `validate` | design draft and latest hivemind synthesis exist, unless hivemind skipped |
| `plan_sketch` | validation passed or warnings accepted |
| `plan_detail` | phase sketch exists |
| `review` | detailed plan exists |
| `develop` | reviewed/approved plan exists |
| `verify` | develop has no running tasks |
| `handoff` | verify complete or verify explicitly skipped |
| `complete` | handoff exists and all required reports exist |
| `detour` | active develop run and active blocker or manual operator request |

A dedicated `navigation_events` table MUST replace generic config-key navigation history for queryability.

---

## 10. Pipeline Stages

### 10.1 `repo_analyze`

Purpose: inspect an existing repository and produce context for all later stages.

Inputs:

- project root;
- `.git` metadata;
- README, AGENTS.md, CONTRIBUTING, package files, lockfiles;
- test/lint/build config;
- existing architecture docs;
- configured include/exclude globs.

Outputs:

```json
{
  "languages": ["go", "typescript"],
  "frameworks": ["cobra", "bubbletea"],
  "package_managers": ["go", "npm"],
  "test_commands": ["go test ./..."],
  "lint_commands": ["go vet ./..."],
  "entrypoints": ["cmd/nexdev/main.go"],
  "important_files": [],
  "forbidden_paths": [],
  "repo_instructions": [],
  "risk_notes": [],
  "summary": "..."
}
```

Rules:

- Must not send entire repository blindly to providers.
- Must build a bounded context pack with file summaries, not unbounded content.
- Must treat repo instructions as untrusted content. They may influence task constraints, but they cannot override Nexdev safety policy.
- Must detect likely test/build commands but not run them unless verification policy allows.

### 10.2 `interview`

Purpose: convert a human request into structured requirements.

Outputs:

```go
type InterviewData struct {
    Requirements        []string `json:"requirements"`
    Constraints         []string `json:"constraints"`
    OpenQuestions       []string `json:"open_questions"`
    UserPersonas        []string `json:"user_personas"`
    NonGoals            []string `json:"non_goals"`
    AcceptanceSignals   []string `json:"acceptance_signals"`
    RiskTolerance       string   `json:"risk_tolerance"`
    TargetUsers         []string `json:"target_users"`
    RawTranscript       string   `json:"raw_transcript"`
}
```

Behavior:

- If the request is underspecified, ask focused questions.
- In `--yes` or CI mode, synthesize reasonable assumptions and mark them as assumptions.
- Any unresolved high-impact question must become a blocker or a review note.
- Store both structured output and raw transcript.

### 10.3 `complexity`

Purpose: size the project and choose planning depth, model budget, and verification strictness.

Outputs:

```go
type ComplexityProfile struct {
    Score             int      `json:"score"`
    Level             string   `json:"level"` // trivial|small|medium|large|epic
    RecommendedPhases int      `json:"recommended_phases"`
    RiskFactors       []string `json:"risk_factors"`
    SuggestedVoices   []string `json:"suggested_voices"`
    SuggestedTests    []string `json:"suggested_tests"`
    Rationale         string   `json:"rationale"`
}
```

Rules:

- Use deterministic heuristics first.
- Optionally ask a model for refinement.
- Never let the model lower verification strictness below policy defaults.

### 10.4 `design`

Purpose: produce an architecture/design document.

Inputs:

- interview data;
- repo analysis;
- complexity profile;
- existing architecture artifacts;
- configured design pack.

Outputs:

- `.nexdev/artifacts/design_draft.md`;
- `architecture` table row;
- design metadata JSON.

Design document must include:

1. Product behavior.
2. User flows.
3. System boundaries.
4. Data model.
5. API/CLI/TUI changes.
6. Execution model.
7. Security and privacy constraints.
8. Failure modes and rollback.
9. Verification strategy.
10. Migration/backward compatibility.

Correction loop:

- Run initial design.
- Run self-critique pass.
- If critique has actionable findings, run correction pass.
- Max iterations default: 3.
- Stop when no actionable findings remain or max iterations reached.
- If high-severity findings remain, mark stage `blocked` unless `--accept-risk` is supplied.

### 10.5 `hivemind`

Purpose: stress-test the design using specialized perspectives.

Default voices:

| Voice | Role |
|---|---|
| `skeptic` | Finds overengineering, weak assumptions, unclear scope. |
| `pragmatist` | Finds operational gaps, maintainability issues, missing error handling. |
| `security` | Finds prompt injection, secret leakage, trust-boundary, tool, auth, and sandbox risks. |
| `ux` | Finds user-facing friction and poor feedback states. |
| `test` | Finds missing test strategy, fixtures, CI gaps, verification holes. |
| `devil` | Argues for a radically simpler design or refuses unjustified complexity. |

Voice output:

```go
type HivemindCritique struct {
    Voice     string           `json:"voice"`
    Findings  []Finding        `json:"findings"`
    Severity  string           `json:"severity"` // low|medium|high|critical
    Verdict   string           `json:"verdict"`  // approve|request_changes
    Confidence float64         `json:"confidence"`
}
```

Synthesis output:

```go
type HivemindSynthesis struct {
    ConsensusFindings []Finding `json:"consensus_findings"`
    RequiredChanges   []string  `json:"required_changes"`
    OptionalChanges   []string  `json:"optional_changes"`
    Disagreements     []string  `json:"disagreements"`
    FinalVerdict      string    `json:"final_verdict"` // approve|revise|block
}
```

Behavior:

- Voices MAY run in parallel.
- Default profile: `parallel: true`, `max_concurrency: 3`, cost guard enabled.
- `--cheap` sets `parallel: false` and fewer voices.
- `--brrrr` sets `parallel: true`, all voices, and larger concurrency, but still honors spend caps.
- If synthesis says `revise`, feed required changes back to design correction.
- Max design/hivemind cycles default: 2.
- If still unresolved, block and surface findings to review.

### 10.6 `validate`

Purpose: sanity-check the complete pre-plan state before planning.

Inputs:

- interview;
- complexity;
- repo analysis;
- design;
- latest hivemind synthesis.

Output:

```go
type ValidationReport struct {
    Ambiguities     []Finding `json:"ambiguities"`
    Conflicts       []Finding `json:"conflicts"`
    MissingPrereqs  []Finding `json:"missing_prereqs"`
    Blockers        []Finding `json:"blockers"`
    HallucinationRisks []Finding `json:"hallucination_risks"`
    Verdict         string    `json:"verdict"` // pass|warn|block
}
```

Rules:

- Block on blockers.
- Warn on ambiguities by default.
- Block on conflicts by default.
- Never silently delete a requirement to make validation pass.

### 10.7 `plan_sketch`

Purpose: create high-level phases only.

Phase output:

```go
type PhaseSketch struct {
    ID                  string   `json:"id"`
    Number              int      `json:"number"`
    Title               string   `json:"title"`
    Description         string   `json:"description"`
    EstimatedComplexity string   `json:"estimated_complexity"`
    Goals               []string `json:"goals"`
    Risks               []string `json:"risks"`
}
```

Rules:

- Number phases canonically by order, not by model text.
- Deduplicate similar phases.
- Phase count should respect complexity profile unless design requires otherwise.

### 10.8 `plan_detail`

Purpose: expand phases into concrete tasks.

Task output:

```go
type TaskSpec struct {
    ID                 string   `json:"id"`
    PhaseID            string   `json:"phase_id"`
    Title              string   `json:"title"`
    Description        string   `json:"description"`
    ExpectedFiles      []string `json:"expected_files"`
    Dependencies       []string `json:"dependencies"`
    AcceptanceCriteria []string `json:"acceptance_criteria"`
    TestCommands       []string `json:"test_commands"`
    RiskLevel          string   `json:"risk_level"`
    RequiredTools      []string `json:"required_tools"`
    Notes              []string `json:"notes"`
}
```

Rules:

- Tasks must be small enough for one execution unit.
- Every task must have acceptance criteria.
- Every write task must list expected files or file globs.
- Dependencies must reference existing task IDs.
- Cycles are invalid.
- If parallel workers are enabled, file-overlap conflicts must be detected before assignment.
- Use structured output first; fallback text parsing only for legacy imports.
- Generate `.nexdev/artifacts/devplan.json` and human `devplan.md`.

### 10.9 `review`

Purpose: gate the plan before execution.

Review modes:

1. `manual`: human approval required.
2. `auto`: LLM reviewer approves or requests changes.
3. `ci`: policy-driven approval; fails if high-risk tasks lack tests.
4. `skip`: allowed only with explicit `--skip-review`.

Review UI must allow:

- rename tasks;
- reorder phases;
- add/delete tasks;
- edit expected files;
- edit acceptance criteria;
- add operator notes;
- mark tasks blocked/skipped;
- approve with risk acknowledgement.

Every manual edit MUST create a `plan_edit_events` row and update plan version.

### 10.10 `develop`

Purpose: execute approved tasks.

Use geoffrussy's executor as the base and extend it.

Required executor additions:

- `SetSteeringContext(taskID string, msg string)` or equivalent steering event ingestion.
- `CurrentTask()` query.
- `Pause(ctx, reason)`, `Resume(ctx)`, `Cancel(ctx, reason)`, `SkipTask(ctx, taskID, reason)` context-aware variants.
- Event publisher bridge from `TaskUpdate` to persisted event log.
- Task-level prompt context builder that includes requirements, design, plan notes, acceptance criteria, expected files, steering summary, and relevant repo context.
- Optional worker/worktree strategy.

Execution strategies:

| Strategy | Default | Description |
|---|---:|---|
| `single_worktree` | yes | One executor modifies main worktree. Safest v0.1 default. |
| `git_worktree_workers` | optional | Parallel workers in git worktrees with serial merge. |
| `container_worker` | optional | Execute tools inside an operator-provided container sandbox. |
| `external_rpc_worker` | optional | Speak a documented JSON-RPC worker protocol. |

Task completion requires:

- files changed only inside policy-allowed paths;
- acceptance criteria addressed in task report;
- task status updated;
- event emitted;
- optional git commit if configured.

### 10.11 `detour` Pseudo-Stage

Purpose: create the minimum plan change needed to unblock work.

Triggers:

1. Task executor emits a structured blocker.
2. Operator calls `POST /detour` or `nexdev detour`.
3. Review gate sends a task back for scoped replanning.

Types:

```go
type DetourRequest struct {
    ProjectID     string       `json:"project_id"`
    RunID         string       `json:"run_id"`
    TriggerTaskID string       `json:"trigger_task_id"`
    Reason        string       `json:"reason"`
    Context       string       `json:"context"`
    Source        DetourSource `json:"source"` // blocker_auto|operator_manual|review_replan
}

type DetourResult struct {
    ID            string       `json:"id"`
    NewTasks      []TaskSpec   `json:"new_tasks"`
    SplicedAfter  string       `json:"spliced_after"`
    IDConflicts   []string     `json:"id_conflicts"`
    Depth         int          `json:"depth"`
}
```

Flow:

1. Capture current task, blocker, phase, neighboring tasks, design summary, and repo context.
2. Ask provider for the minimal task set to unblock.
3. Validate tasks with the same plan_detail schema.
4. Check `detour.max_depth`.
5. Splice tasks immediately after trigger task.
6. Mark trigger task `blocked` or `pending_after_detour` depending on policy.
7. Emit `detour_created` event.
8. Resume develop if policy allows.

Depth exceeded behavior:

- Never silently skip.
- Create a blocker with `reason=detour_depth_exceeded`.
- Pause develop and require operator resolution.

### 10.12 `verify`

Purpose: prove the worktree is coherent after development.

Inputs:

- repo analysis commands;
- task test commands;
- operator-configured verification commands;
- changed files.

Outputs:

```go
type VerifyReport struct {
    Passed       bool              `json:"passed"`
    Commands     []CommandResult   `json:"commands"`
    ChangedFiles []ChangedFile     `json:"changed_files"`
    Failures     []Finding         `json:"failures"`
    Warnings     []Finding         `json:"warnings"`
}
```

Rules:

- Verification commands require policy permission.
- Each command must have timeout, output cap, stripped/controlled env, working directory policy, and cancellable process group.
- If tests fail, Nexdev may run one repair loop if configured, then re-run verify.
- Verification is required before `complete` unless explicitly skipped by admin/operator in dev profile.

### 10.13 `handoff`

Purpose: write continuation artifacts for humans and future agents.

Required artifacts:

- `handoff.md`: what was requested, what was built, what changed, how to run tests, open risks.
- `changed_files.json`: path, status, sha256, byte size, owning tasks.
- `run_summary.json`: stages, timings, costs, provider usage, final status.
- `devplan.md`: anchored, task IDs stable.
- `phaseNNN.md`: per-phase plan and completion status.

---

## 11. Provider Layer

### 11.1 Provider Interface

Keep the geoffrussy provider interface as the canonical boundary, then extend via optional capability interfaces.

```go
type Provider interface {
    Name() string
    Authenticate(ctx context.Context, apiKey string) error
    ListModels(ctx context.Context) ([]Model, error)
    DiscoverModels(ctx context.Context) ([]Model, error)
    Call(ctx context.Context, req Request) (*Response, error)
    Stream(ctx context.Context, req Request) (<-chan StreamChunk, error)
    GetRateLimit() *RateLimitInfo
    GetQuota() *QuotaInfo
    SupportsCodingPlan() bool
}
```

Optional extensions:

```go
type StructuredProvider interface {
    CallStructured(ctx context.Context, req Request, schema JSONSchema) (*Response, error)
}

type UsageProvider interface {
    LastUsage() UsageMetadata
}
```

### 11.2 Provider Router

`provider.Router` chooses the right provider per stage.

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

Empty slot means inherit `provider.primary`.

### 11.3 Structured Output Wrapper

Every stage that expects JSON MUST call through `provider.CallStructured` wrapper:

1. Build prompt with JSON schema and examples.
2. Call provider.
3. Strict decode into Go struct.
4. Reject unknown fields where feasible.
5. Validate semantic constraints.
6. If invalid, run repair prompt with validation errors.
7. Max repair attempts default: 2.
8. Persist raw response and validation errors for audit.

### 11.4 Cost and Rate Control

Config:

```yaml
cost:
  enabled: true
  currency: USD
  max_run_usd: 25.00
  max_stage_usd: 8.00
  require_approval_above_usd: 5.00
  estimate_before_hivemind: true
  stop_on_unknown_price: false
```

Behavior:

- Unknown prices are warnings by default in dev profile.
- CI profile may require known prices.
- Hivemind and parallel workers must check cost guard before launching.

---

## 12. Control Plane

### 12.1 Server Defaults

```yaml
controlplane:
  enabled: true
  bind: 127.0.0.1
  port: 7432
  allow_remote_bind: false
  openapi_enabled: true
  mcp_enabled: true
  sse:
    heartbeat_interval_s: 15
    client_queue_max_events: 1000
    replay_max_events: 10000
    retry_ms: 3000
```

### 12.2 API Contract

`api/openapi.yaml` is authoritative. Go server types SHOULD be generated with `oapi-codegen`.

Core endpoints:

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/health` | none | Health check. |
| GET | `/status` | observer | Full project/run snapshot. |
| GET | `/plan` | observer | Current plan JSON. |
| GET | `/artifacts` | observer | Artifact manifest. |
| GET | `/events` | observer | Event list query, not stream. |
| GET | `/runs/{run_id}/stream` | observer | SSE stream with replay. |
| POST | `/runs` | operator | Start run. |
| POST | `/pause` | operator | Pause active run. |
| POST | `/resume` | operator | Resume active run. |
| POST | `/skip` | operator | Skip current or specified task. |
| POST | `/steer` | operator | Add steering context. |
| POST | `/detour` | operator | Trigger manual detour. |
| POST | `/cancel` | admin | Cancel active run. |
| PUT | `/tasks/{task_id}` | admin | Edit pending task. |
| DELETE | `/tasks/{task_id}` | admin | Delete pending task. |
| POST | `/blockers/{blocker_id}/resolve` | operator | Resolve blocker. |
| GET | `/config` | observer | Redacted resolved config. |
| PUT | `/config` | admin | Update safe config fields. |
| GET | `/providers` | observer | Provider status. |
| POST | `/providers/{name}/test` | operator | Test provider. |
| GET | `/mcp/tools` | observer | List MCP tools. |
| POST | `/mcp/call` | role per tool | Invoke MCP tool. |

### 12.3 ErrorResponse

```go
type ErrorResponse struct {
    ErrorCode string         `json:"error_code"`
    Message   string         `json:"message"`
    Details   map[string]any `json:"details"`
    RequestID string         `json:"request_id"`
}
```

All JSON errors MUST use this shape.

### 12.4 SSE Event Envelope

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
    Source          string          `json:"source"` // core|executor|worker|tui|api|mcp
    Payload         json.RawMessage `json:"payload"`
}
```

SSE frame:

```text
id: <event_id>
event: <type>
retry: 3000
data: {EventEnvelope JSON}

```

Rules:

- Do not use `data: [DONE]`.
- Use `done` event for completion.
- Persist event before broadcast.
- Sequence is monotonic per run.
- Accept `Last-Event-ID` and replay missed events.
- Send heartbeat every configured interval.
- Each client has bounded queue.
- On queue overflow, emit `sse_client_slow` event if possible, then close.

### 12.5 Event Types

Required event types:

```text
heartbeat
run_started
run_status
stage_transition
stage_status
content_delta
provider_call_started
provider_call_completed
provider_call_failed
artifact_updated
plan_updated
review_required
review_completed
task_started
task_progress
task_completed
task_error
task_blocked
task_paused
task_resumed
task_skipped
steering_added
detour_requested
detour_created
detour_failed
blocker_created
blocker_resolved
verify_started
verify_command_output
verify_completed
git_event
cost_update
security_warning
pipeline_error
done
```

Map geoffrussy `TaskUpdate` values to the task event family.

---

## 13. Auth, Roles, and Deployment Profiles

### 13.1 Roles

| Role | Permissions |
|---|---|
| `observer` | GET status, plan, artifacts, events, streams. |
| `operator` | observer plus pause, resume, skip, steer, detour, resolve blockers, provider test. |
| `admin` | operator plus cancel, config mutation, task mutation, token management, destructive operations. |

### 13.2 Default Token Model

Use opaque bearer tokens by default.

Token creation:

```bash
nexdev auth token create --role operator --ttl 30d
```

Server stores:

```sql
CREATE TABLE auth_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL,
    name TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT,
    revoked_at TEXT,
    last_used_at TEXT
);
```

Hashing:

- token value is random 32+ bytes, base64url encoded;
- store `HMAC-SHA256(server_secret, token)` or Argon2id hash;
- compare with constant-time comparison.

Authorization:

```http
Authorization: Bearer <token>
```

### 13.3 Optional Stateless Tokens

Stateless JWS/HMAC tokens MAY be supported only if required by deployment. They MUST include:

- issuer `nexdev`;
- audience `nexdev-controlplane`;
- role;
- issued-at;
- expiry;
- key id;
- signature with vetted library or minimal HMAC envelope.

Revocation is harder with stateless tokens, so opaque tokens remain default.

### 13.4 Deployment Profiles

#### `dev`

- bind: `127.0.0.1`
- auth: optional
- command execution: disabled unless user opts in
- fake provider allowed
- bundled TUI enabled

#### `trusted-lan`

- bind: explicit LAN host or `0.0.0.0`
- auth: required
- CORS: explicit origins only
- command execution: requires sandbox policy
- audit log: required
- secret store: OS keyring or encrypted env file
- wildcard CORS: forbidden

#### `ci`

- bind: disabled unless requested
- auth: required if server enabled
- no TUI
- deterministic logs
- fake provider allowed for tests
- verification required unless explicitly disabled

Startup MUST fail if `bind != 127.0.0.1` and auth is disabled.

---

## 14. State Schema Additions

Keep existing geoffrussy tables where possible. Add these tables with migrations.

### 14.1 Runs and Stages

```sql
CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    status TEXT NOT NULL,
    current_stage TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    cancelled_at TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

CREATE TABLE IF NOT EXISTS stage_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    stage TEXT NOT NULL,
    status TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 1,
    started_at TEXT,
    completed_at TEXT,
    error_json TEXT,
    output_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

### 14.2 Event Log

```sql
CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    type TEXT NOT NULL,
    source TEXT NOT NULL,
    stage TEXT,
    task_id TEXT,
    payload_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(run_id, sequence),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE INDEX IF NOT EXISTS idx_events_run_sequence ON events(run_id, sequence);
CREATE INDEX IF NOT EXISTS idx_events_run_type ON events(run_id, type);
```

### 14.3 Pipeline Artifacts

```sql
CREATE TABLE IF NOT EXISTS artifacts (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    run_id TEXT,
    kind TEXT NOT NULL,
    path TEXT NOT NULL,
    sha256 TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

### 14.4 Hivemind

```sql
CREATE TABLE IF NOT EXISTS hivemind_results (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    cycle INTEGER NOT NULL,
    voice TEXT NOT NULL,
    result_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

### 14.5 Validation

```sql
CREATE TABLE IF NOT EXISTS validate_results (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    report_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

### 14.6 Steering

```sql
CREATE TABLE IF NOT EXISTS steering_events (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT,
    message TEXT NOT NULL,
    summary TEXT,
    source TEXT NOT NULL,
    created_by_role TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

### 14.7 Detours

```sql
CREATE TABLE IF NOT EXISTS detour_records (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    trigger_task_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    source TEXT NOT NULL,
    depth INTEGER NOT NULL,
    result_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

### 14.8 Navigation

```sql
CREATE TABLE IF NOT EXISTS navigation_events (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    run_id TEXT,
    from_stage TEXT,
    to_stage TEXT NOT NULL,
    reason TEXT,
    actor TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

### 14.9 Plan Edits

```sql
CREATE TABLE IF NOT EXISTS plan_edit_events (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    plan_version_before INTEGER NOT NULL,
    plan_version_after INTEGER NOT NULL,
    edit_type TEXT NOT NULL,
    target_id TEXT,
    patch_json TEXT NOT NULL,
    actor TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
```

---

## 15. Configuration Schema

Default `nexdev.yaml`:

```yaml
version: "0.1"

project:
  name: ""
  description: ""
  state_dir: ".nexdev"

profile: dev  # dev | trusted-lan | ci

provider:
  primary:
    name: anthropic
    model: claude-sonnet-4-5
    api_key_env: ANTHROPIC_API_KEY
  stages:
    interview: {}
    complexity: {}
    design: {}
    hivemind_voice: {}
    hivemind_synthesis: {}
    validate: {}
    plan_sketch: {}
    plan_detail: {}
    review: {}
    develop: {}
    verify_repair: {}
    handoff: {}
  request_timeout_s: 120
  max_retries: 3
  retry_base_ms: 500
  allow_fallback: false

pipeline:
  stages:
    - repo_analyze
    - interview
    - complexity
    - design
    - hivemind
    - validate
    - plan_sketch
    - plan_detail
    - review
    - develop
    - verify
    - handoff
  skip: []
  resume: true
  checkpoint_after_each_stage: true

repo_analyze:
  enabled: true
  max_file_bytes: 200000
  max_context_bytes: 500000
  include_globs: []
  exclude_globs:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
    - "dist/**"
    - "build/**"
    - ".nexdev/**"

complexity:
  model_refinement: true

structured_outputs:
  strict: true
  max_repair_attempts: 2
  persist_raw_responses: true

design:
  correction_loop:
    enabled: true
    max_iterations: 3

hivemind:
  enabled: true
  voices: [skeptic, pragmatist, security, ux, test, devil]
  parallel: true
  max_concurrency: 3
  max_cycles: 2
  cost_confirmation: required_once

validate:
  block_on_ambiguity: false
  block_on_conflict: true
  block_on_hallucination_risk: true

review:
  mode: manual  # manual | auto | ci | skip
  allow_inline_edits: true
  require_approval_for_high_risk: true

develop:
  strategy: single_worktree  # single_worktree | git_worktree_workers | container_worker | external_rpc_worker
  max_parallel_tasks: 1
  commit_on_task_complete: false
  commit_on_stage_transition: true
  stop_on_first_task_error: true

verify:
  enabled: true
  commands: []
  infer_commands_from_repo: true
  timeout_s: 300
  output_cap_bytes: 200000
  repair_attempts: 1

detour:
  enabled: true
  auto_trigger: true
  max_depth: 3
  require_operator_approval: false

controlplane:
  enabled: true
  bind: 127.0.0.1
  port: 7432
  allow_remote_bind: false
  auth_required: auto
  token_env: NEXDEV_CONTROL_TOKEN
  cors_allow_origins: ["http://127.0.0.1:7432"]
  sse:
    heartbeat_interval_s: 15
    client_queue_max_events: 1000
    replay_max_events: 10000
    retry_ms: 3000
  mcp:
    enabled: true

security:
  reject_symlink_escape: true
  scrub_logs: true
  tool_policy_file: ".nexdev/tool_policy.yaml"
  command_execution_default: deny
  network_default: deny
  secret_env_allowlist: []
  max_prompt_bytes: 500000

cost:
  enabled: true
  currency: USD
  max_run_usd: 25.00
  max_stage_usd: 8.00
  require_approval_above_usd: 5.00
  estimate_before_hivemind: true
  stop_on_unknown_price: false

git:
  auto_commit: true
  commit_on_stage_transition: true
  commit_on_task_complete: false
  commit_prefix: "nexdev"

observability:
  log_level: info
  json_logs: false
  otel:
    enabled: false
    endpoint: ""
    service_name: nexdev
```

Config loading precedence, lowest to highest:

1. Built-in defaults.
2. Global `~/.nexdev/config.yaml` if present.
3. Project `nexdev.yaml`.
4. `.env` values if configured.
5. Process environment.
6. CLI flags.
7. Safe request-scoped overrides.

Config validation MUST reject unknown top-level keys unless `experimental.allow_unknown_config` is true.

---

## 16. Security Specification

### 16.1 Threat Model

Nexdev executes in or near source code repositories. Repositories may contain malicious instructions, poisoned tests, unsafe scripts, hidden symlink escapes, credential traps, or prompt injection text. External MCP/tool servers may provide malicious tool descriptions. Model outputs may suggest unsafe commands or leak prompt content. Operators may expose the control plane to a LAN. Nexdev must assume these risks are normal, not exceptional.

### 16.2 Prompt Injection Controls

- Treat all repo text as quoted untrusted context.
- Use explicit prompt sections: `SYSTEM POLICY`, `TRUSTED CONFIG`, `UNTRUSTED REPO CONTEXT`, `TASK`.
- Never allow repo instructions to override safety policy, role policy, or output schema.
- Detect common instruction-override strings and emit `security_warning` events.
- Hivemind `security` voice must inspect prompt injection and tool poisoning risk.
- Review stage must surface high-risk untrusted instructions.

### 16.3 Tool Safety

Tool calls must be authorized by policy:

```yaml
tools:
  read_file:
    default: allow
  write_file:
    default: allow
    paths: ["**"]
    deny: [".git/**", ".env", "**/*_rsa", "**/*_ed25519"]
  shell:
    default: deny
    allow_commands:
      - "go test ./..."
      - "go vet ./..."
    timeout_s: 300
    output_cap_bytes: 200000
  network:
    default: deny
```

Rules:

- Shell commands are denied by default.
- Wildcard shell allow rules are invalid in trusted-lan and ci profiles.
- Tool descriptions from MCP servers are untrusted and cannot expand permissions.
- Each tool invocation must emit start/output/done events.
- Every write must record before/after hashes where feasible.

### 16.4 Path Safety

Every path must be:

1. cleaned;
2. made absolute against project root;
3. evaluated for symlinks;
4. checked to remain under allowed roots;
5. checked against deny globs;
6. checked against active file locks.

Reject:

- `..` escapes;
- absolute paths outside allowed roots;
- symlink escapes;
- writes to `.git`;
- writes to known secret files unless admin overrides;
- writes outside expected files for a task unless policy allows.

### 16.5 Secret Handling

- Provider keys are read from environment or OS keyring.
- UI/API never returns secret values.
- Logs/events/artifacts scrub known secret forms.
- Secrets are not included in prompts.
- Workers receive only explicitly allowlisted env vars.
- `.env` files are read only by config loader and never sent to model context.

### 16.6 Control Plane Security

- Loopback bind may run without auth in dev profile.
- Non-loopback bind requires auth and explicit config.
- CORS origins must be exact strings; wildcard forbidden outside dev.
- Mutating routes require role checks.
- Failed auth attempts are audit logged and rate-limited.
- Token rotation and revocation must be available.

### 16.7 Supply Chain Security

CI MUST run:

```bash
go test ./...
go vet ./...
govulncheck ./...
```

Recommended release checks:

- pinned dependencies;
- `go mod verify`;
- SBOM generation if distributing binaries;
- checksum/signature release artifacts;
- fake-provider end-to-end smoke tests.

---

## 17. Observability

### 17.1 Logs

Use `log/slog` with structured fields:

- `project_id`
- `run_id`
- `stage`
- `task_id`
- `provider`
- `model`
- `event_id`
- `request_id`

Logs must be redacted before write.

### 17.2 OpenTelemetry

If enabled, emit:

- traces for pipeline stages, provider calls, tool calls, DB writes, git operations;
- metrics for stage duration, provider latency, token usage, task outcomes, event queue drops;
- log correlation fields.

### 17.3 Cost Ledger

Record:

- prompt tokens;
- completion tokens;
- total tokens;
- estimated USD;
- provider/model;
- stage/task;
- retry count.

---

## 18. CLI Surface

```text
nexdev init [--name NAME] [--description DESC]
nexdev run [--from STAGE] [--stage STAGE] [--yes] [--cheap] [--brrrr]
nexdev develop
nexdev verify
nexdev status [--json]
nexdev plan [--json]
nexdev review
nexdev navigate <stage>
nexdev detour [--reason TEXT]
nexdev steer "message"
nexdev pause [--reason TEXT]
nexdev resume
nexdev cancel [--reason TEXT]
nexdev blockers list|resolve
nexdev provider list
nexdev provider test <name>
nexdev events [--follow]
nexdev artifacts list|open
nexdev history
nexdev config print|validate|set
nexdev auth token create|list|revoke
nexdev serve
nexdev doctor
```

Global flags:

```text
--project-dir DIR
--config FILE
--state-dir DIR
--no-tui
--json
--log-level debug|info|warn|error
--profile dev|trusted-lan|ci
--control-url URL
--token TOKEN
```

Command semantics:

- CLI commands should call in-process services when running locally.
- If `--control-url` is supplied, CLI acts as an HTTP client.
- `nexdev steer` maps to `POST /steer` semantics.

---

## 19. TUI Specification

The TUI is a client of the same state/events as the HTTP API. It must not own pipeline state.

Views:

1. Run overview: current stage, tasks, blockers, cost, provider status.
2. Event stream: filtered live events.
3. Plan editor: phases/tasks tree with inline edits.
4. Hivemind view: voices, findings, synthesis.
5. Detour dialog: reason/context/preview tasks/approve.
6. Provider setup/test view.
7. Config view: redacted resolved config.
8. Artifacts view.

Keybindings:

- `p`: pause/resume toggle.
- `s`: steer current task.
- `d`: detour current task.
- `k`: skip current task after confirmation.
- `r`: retry failed stage/task.
- `q`: quit TUI without killing run unless explicitly confirmed.

---

## 20. MCP Tool Surface

MCP tools must be thin wrappers around control-plane operations.

Required tools:

| Tool | Role | Description |
|---|---|---|
| `nexdev_start_run` | operator | Start a pipeline run. |
| `nexdev_get_status` | observer | Get run/project status. |
| `nexdev_get_plan` | observer | Get devplan. |
| `nexdev_list_artifacts` | observer | Get artifact manifest. |
| `nexdev_get_artifact` | observer | Read artifact by kind/path. |
| `nexdev_pause` | operator | Pause run. |
| `nexdev_resume` | operator | Resume run. |
| `nexdev_cancel` | admin | Cancel run. |
| `nexdev_steer` | operator | Add steering message. |
| `nexdev_detour` | operator | Request detour. |
| `nexdev_resolve_blocker` | operator | Resolve blocker. |
| `nexdev_provider_test` | operator | Test provider. |

MCP safety rules:

- Tool descriptions are static and generated from checked-in schema.
- MCP input schemas must match OpenAPI schemas where possible.
- MCP calls must pass the same auth/role checks as HTTP.
- MCP stdio mode must not shell-execute arbitrary strings.

---

## 21. Git and Worktree Strategy

### 21.1 Default Single Worktree

For v0.1, default development should modify the current worktree directly. This is simpler, easier to debug, and matches geoffrussy's existing executor model.

### 21.2 Optional Git Worktree Workers

For parallel execution:

1. Create worker worktree per worker under `.nexdev/workers/<worker-id>`.
2. Assign non-overlapping tasks by file ownership.
3. Workers commit changes to worker branch.
4. Merge serially into main worktree.
5. Conflicts pause run and create blocker.
6. Extract changed files after merge.

### 21.3 Commit Metadata

Stage transition commits:

```text
nexdev: complete stage design

Project: <project_id>
Run: <run_id>
Stage: design
Artifacts: design_draft.md, design_review.json
```

Task commits if enabled:

```text
nexdev: T2.03 implement provider router

Project: <project_id>
Run: <run_id>
Task: T2.03
Phase: phase_002
```

---

## 22. Steering Context Management

Steering messages are durable events, not transient strings.

Rules:

- Store every steering message in `steering_events`.
- Include only relevant steering in prompt context.
- Active context includes last `N` messages plus steering summary.
- Default `N`: 5.
- If steering text exceeds budget, summarize older messages with a model call or deterministic summarizer.
- Steering cannot override safety policy, output schema, or task acceptance criteria unless admin changes the plan.
- Steering event must identify source: `cli`, `api`, `tui`, `mcp`.

TaskExecutor prompt context order:

1. System safety policy.
2. Provider/tool policy.
3. Project requirements.
4. Architecture summary.
5. Current task spec.
6. Relevant repo context.
7. Operator notes.
8. Steering summary.
9. Last steering messages.
10. Output schema.

---

## 23. Verification and Repair Loop

After develop:

1. Detect changed files.
2. Select verification commands.
3. Run allowed commands with timeout and output cap.
4. Parse failures.
5. If failed and `repair_attempts > 0`, create repair task(s) or rerun failed task with failure output.
6. Re-run verification.
7. Persist verify report.

Command result schema:

```go
type CommandResult struct {
    Command      string `json:"command"`
    ExitCode     int    `json:"exit_code"`
    TimedOut     bool   `json:"timed_out"`
    StdoutTail   string `json:"stdout_tail"`
    StderrTail   string `json:"stderr_tail"`
    OutputSHA256 string `json:"output_sha256"`
    StartedAt    string `json:"started_at"`
    CompletedAt  string `json:"completed_at"`
}
```

---

## 24. Testing Strategy

### 24.1 Unit Tests

Required packages:

- config loading/validation;
- path sanitizer;
- redaction;
- role checks;
- event log replay;
- SSE formatting;
- stage prereq validation;
- plan DAG validation;
- detour splicing;
- steering context summarization;
- provider structured output repair;
- SQL migrations.

### 24.2 Integration Tests

- fake provider full pipeline;
- fake provider invalid JSON repair;
- pause/resume/skip during develop;
- SSE reconnect with `Last-Event-ID`;
- auth roles deny/allow matrix;
- blocker -> detour -> resume;
- review plan edit -> plan version increment;
- verify failure -> repair attempt;
- migration from geoffrussy state.

### 24.3 E2E Smoke Tests

`./scripts/e2e_fake_provider.sh` must:

1. create temp repo;
2. run `nexdev init`;
3. run full pipeline with fake provider;
4. connect SSE client;
5. verify event replay;
6. verify artifacts exist;
7. verify changed files manifest exists;
8. verify run reaches `complete`.

### 24.4 Security Tests

- prompt injection fixture in README;
- malicious AGENTS.md attempting to reveal secrets;
- symlink escape write attempt;
- `.env` exfiltration attempt;
- unauthorized token role mutation attempt;
- unbounded command output attempt;
- SSE slow client overflow;
- MCP tool description poisoning fixture.

---

## 25. Migration Plan

### 25.1 From geoffrussy

1. Fork repository.
2. Rename module to target org, e.g. `github.com/<org>/nexdev`.
3. Preserve provider/state/navigation/executor/git packages.
4. Add `runs`, `events`, `stage_runs`, `artifacts`, `hivemind_results`, `steering_events`, `detour_records`, `navigation_events`, and auth tables.
5. Add `internal/pipeline` and `internal/controlplane`.
6. Add event bridge in executor monitor.
7. Add steering support in TaskExecutor.
8. Add stage graph additions.
9. Add OpenAPI and generated API types.
10. Add fake provider/fake worker fixtures.

Existing geoffrussy projects should remain readable because migrations are additive. If a project is already at `develop`, Nexdev should infer skipped earlier stages and require review before executing new tasks.

### 25.2 From devussy

No code migration. Add import command:

```bash
nexdev init --import-devussy PATH
```

Import:

- `interview.json`;
- `complexity_profile.json`;
- `design_draft.md`;
- `validated_design.md`;
- `devplan.md`;
- `phase*.md`;
- `handoff.md`.

Legacy text parsing is allowed only in importer.

### 25.3 From nexussy

No code migration. Concepts only:

- control plane;
- SSE replay;
- MCP tools;
- artifacts;
- steering/inject controls;
- deployment profiles;
- worker isolation concepts.

---

## 26. Implementation Milestones

### Milestone 1: Fork and Contracts

- Fork geoffrussy.
- Rename module.
- Add this spec to `docs/SPEC.md`.
- Add `api/openapi.yaml` skeleton.
- Add config structs and validation.
- Add project lock.

Acceptance:

- `go test ./...` passes.
- `nexdev doctor` works.
- `nexdev config validate` works.

### Milestone 2: State and Event Log

- Add migrations.
- Add event store.
- Add SSE replay manager.
- Add heartbeat and bounded queues.

Acceptance:

- event persistence tests pass;
- reconnect replay test passes;
- slow client test passes.

### Milestone 3: Control Plane and Auth

- Add HTTP routes.
- Add opaque token auth.
- Add role checks.
- Add OpenAPI generated types.
- Add CLI remote mode.

Acceptance:

- auth matrix tests pass;
- OpenAPI server compiles;
- `nexdev serve` exposes health/status/events.

### Milestone 4: Pipeline Stages

- Add stage runner.
- Add repo_analyze/interview/complexity/design/validate/plan stages.
- Add structured output validation.
- Add fake provider fixtures.

Acceptance:

- fake provider full pre-development pipeline passes;
- artifacts are written and indexed.

### Milestone 5: Hivemind and Review

- Add voices and synthesis.
- Add correction cycle.
- Add TUI/CLI plan review.
- Add plan edit events.

Acceptance:

- hivemind revise loop test passes;
- manual edit creates new plan version.

### Milestone 6: Develop, Steering, Detour

- Extend TaskExecutor.
- Bridge TaskUpdate -> event log.
- Add steering event support.
- Add detour manager/splice.

Acceptance:

- steer during fake task changes prompt context;
- blocker triggers detour tasks;
- depth exceeded pauses and creates blocker.

### Milestone 7: Verify and Handoff

- Add verification command runner.
- Add repair loop.
- Add changed files manifest.
- Add handoff artifact.

Acceptance:

- fake repo full run reaches complete;
- verify failures create report;
- handoff includes changed files and commands.

### Milestone 8: Hardening

- govulncheck in CI.
- path/symlink security tests.
- prompt injection fixtures.
- MCP stdio/HTTP tests.
- release scripts.

Acceptance:

- all required tests pass;
- `scripts/e2e_fake_provider.sh` passes;
- `docs/SECURITY.md` complete.

---

## 27. Decisions on Draft Open Questions

1. **Hivemind parallelism default:** Default to parallel with `max_concurrency: 3` and cost guard. Provide `--cheap` for sequential and `--brrrr` for max safe concurrency. Never silently exceed spend thresholds.
2. **Detour max depth:** Keep default `3`. Depth exceeded pauses and surfaces a blocker. It must not silently skip.
3. **Steering context window:** Store append-only event log, inject last N messages plus summary. Summarize old steering once token budget is exceeded.
4. **Provider per stage:** Yes. Every major stage can override provider/model. Empty means inherit primary.
5. **Navigation history table:** Yes. Add `navigation_events` table.

---

## 28. Acceptance Criteria for v0.1

A v0.1 implementation is complete when:

1. `nexdev init` creates config/state.
2. `nexdev run --fake-provider --no-tui` completes the full pipeline in a temp repo.
3. `nexdev serve` exposes health/status/plan/artifacts/events/SSE.
4. SSE replay works after disconnect.
5. Opaque token auth enforces observer/operator/admin roles.
6. Pipeline artifacts are generated and indexed.
7. Structured model output validation and repair are tested.
8. Hivemind runs with at least three voices and synthesis.
9. Review gate supports task edit and plan versioning.
10. Develop stage executes at least fake provider file writes safely.
11. Steering changes the next task prompt context.
12. Detour splices tasks after a blocker.
13. Verify runs configured commands under policy.
14. Handoff and changed files manifest are written.
15. `go test ./...`, `go vet ./...`, and `govulncheck ./...` pass.
16. Security fixtures for prompt injection, symlink escape, and secret redaction pass.

---

## 29. Research Basis

This specification was produced by reading the uploaded Nexdev successor draft and inspecting the public repositories and relevant June 2026-era references:

- `mojomast/geoffrussy` for Go executor, provider registry, state, navigation, TUI/CLI dependencies, and SQLite patterns.
- `mojomast/devussy` for adaptive interview, complexity analysis, design correction, sanity review, HiveMind arbitration, basic/detailed devplan generation, checkpoints, and handoff artifacts.
- `mojomast/nexussy` for control-plane concepts, SSE replay/heartbeat, MCP tools, artifacts, worker controls, deployment profiles, and security patterns.
- Model Context Protocol specification, version 2025-06-18, for MCP architecture, tools/resources/prompts, and security principles.
- OWASP Top 10 for LLM Applications 2025 for prompt injection, supply chain, data/model poisoning, improper output handling, excessive agency, prompt leakage, and unbounded consumption risks.
- OpenTelemetry Go docs for traces, metrics, and logs status.
- Go 1.22 ServeMux routing enhancements for method/path routing in standard `net/http`.
- pkg.go.dev documentation for oapi-codegen, kin-openapi, goose, sqlc, Bubble Tea, Cobra, and related Go tooling.
- Go govulncheck documentation for dependency vulnerability scanning.
