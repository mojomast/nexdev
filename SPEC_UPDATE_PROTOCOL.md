# Nexdev Spec Update Protocol

`SPEC.md` is the canonical implementation contract. This protocol controls how it changes during implementation.

## 1. Spec Management Role

The Spec Management Subagent is the only worker that may edit `SPEC.md` unless the orchestrator explicitly assigns a focused spec edit.

The Spec Management Subagent runs:
- After every major milestone.
- Before accepting intentional behavior deviations.
- When a contract mismatch blocker indicates the spec is ambiguous or impractical.
- Before final release readiness.

## 2. Allowed Spec Change Classes

| Class | Meaning | Approval Required |
|---|---|---|
| Clarification | Makes existing requirement more precise without changing scope | Orchestrator |
| Additive requirement | Adds requirement discovered as necessary for safety/correctness | Orchestrator |
| Contract correction | Adjusts schema/API/state details to match accepted implementation | Orchestrator |
| Deferral | Moves requirement out of v0.1 scope with explicit rationale | Orchestrator explicit approval |
| Breaking change | Changes public API/event/state behavior | Orchestrator explicit approval and contract update |
| Removal | Deletes requirement | Orchestrator explicit approval; default reject |

## 3. Forbidden Spec Changes

Spec-management must not:
- Weaken requirements to hide incomplete work.
- Mark unimplemented behavior as implemented.
- Remove security requirements for convenience.
- Change source-of-truth hierarchy.
- Delete geoffrussy/devussy/nexussy thesis.
- Make docs or implementation more authoritative than `SPEC.md`.

## 4. Milestone Review Workflow

After a milestone completes:

1. Read the milestone definition in `DEVPLAN.md`.
2. Read worker handoffs.
3. Inspect changed code, tests, contracts, and docs.
4. Compare implemented behavior against `SPEC.md`.
5. Classify each requirement as `planned`, `in_progress`, `implemented`, `verified`, `deferred`, or `blocked`.
6. Update `SPEC.md` only for approved clarifications, accepted behavior, approved deviations, or explicit deferrals.
7. Update the spec coverage matrix.
8. Update this protocol changelog.
9. Return a spec-management handoff.

## 5. Required Cross-Updates

When `SPEC.md` changes, update affected docs and contracts in the same spec-management task or explicitly assign follow-up tasks.

Cross-update map:
- API route/schema change: `api/openapi.yaml`, generated types, `docs/contracts.md`, `docs/API.md`, tests.
- State schema change: migrations, state tests, `docs/contracts.md`.
- Event/SSE change: event contract tests, `docs/contracts.md`, API docs.
- Config change: config tests, `docs/contracts.md`, `docs/configuration.md`.
- Security change: security tests, `docs/SECURITY.md`, `TESTING_STRATEGY.md`.
- CLI change: CLI tests, `README.md`, operations docs.
- Pipeline stage change: pipeline tests, `docs/architecture.md`, `TESTING_STRATEGY.md`.

## 6. Spec Coverage Matrix

Maintain the matrix in `DEVPLAN.md` initially. If it becomes too large, move it to `docs/spec-coverage.md` and link it from `DEVPLAN.md`.

Required columns:
- Requirement ID
- Spec area or section
- Requirement summary
- Milestone
- Owner
- Implementation path
- Test coverage
- Docs
- Status
- Notes/risks

Allowed statuses:
- `planned`
- `in_progress`
- `implemented`
- `verified`
- `deferred`
- `blocked`

## 7. Spec Management Handoff

Use this exact shape:

```markdown
## Spec Management Handoff

Milestone reviewed:
Implementation evidence:
Tests evidence:
Docs evidence:
Spec sections updated:
Contracts updated:
Approved deviations:
Deferred requirements:
Blocked requirements:
Coverage matrix changes:
Remaining unbuilt spec sections:
Next spec-management trigger:
```

## 8. Drift Checks

Before accepting a milestone, verify:
- No secondary doc contradicts `SPEC.md`.
- `api/openapi.yaml` matches implemented routes.
- State migrations match `docs/contracts.md`.
- Event constants match SSE tests and docs.
- CLI help matches README/operations docs.
- Test strategy includes tests for implemented behavior.
- Security docs match actual security defaults.

## 9. Changelog

- 2026-06-30 M4-SPEC-REVIEW: Clarified `SPEC.md` section 11.1 to record the accepted v0.1 compatibility boundary for the imported geoffrussy `Provider` interface while preserving provider-router, structured-output, audit, usage, and cost requirements. The richer request-shaped provider API remains a future optional capability adapter, not the current concrete interface.
- 2026-06-30 M7-SPEC-REVIEW: Accepted M7 planning/review implementation without `SPEC.md` changes. Classified review approval marker plus indexed artifact and `plan_edit_events` as acceptable interim approval evidence for M8 prerequisites; classified direct SQL inside `ReviewService` task update/delete as a repository-layer follow-up before control-plane/TUI exposure, not a blocker for M8 executor work.
- 2026-06-30 M8-SPEC-REVIEW: Accepted M8 fake/safe develop bridge and basic in-process steering controls without `SPEC.md` changes. Classified executor mapping stage type change from `pipeline.Stage` to stable string `develop` as internal-only because the public event envelope stage field is a string and event values are unchanged. Deferred artifact-backed prompt context, changed-file manifests, project-lock lifecycle wiring, control-plane/CLI adapters, detour splicing, and real policy-gated tool execution to later milestones.
- 2026-06-30 M9-SPEC-REVIEW: Did not update `SPEC.md`. Blocked full M9 acceptance because M7 normal plans persist dense task orders and M9 gap-only detour splicing cannot satisfy the required immediate splice after non-final trigger tasks without safe reorder/version semantics. Classified M9 use of `provider.SlotPlanDetail` for detour structured generation as acceptable interim reuse of the existing task-planning slot, with a dedicated detour provider slot deferred to provider/config follow-up.
- 2026-06-30 M9-SPEC-REVIEW-POST-DEBLOCK: Accepted M9 after dense-order deblocker without `SPEC.md` changes. Dense plans are now handled by state-owned transactional insertion after the trigger task, shifting later tasks in descending order within the same run and plan version before insertion. Kept `provider.SlotPlanDetail` reuse accepted as interim behavior with a dedicated detour slot deferred to provider/config follow-up.
- 2026-06-30 M10-SPEC-REVIEW: Accepted M10 Control Plane HTTP/SSE/Auth without `SPEC.md` or OpenAPI changes. Manual stdlib route binding is acceptable against the existing `api/openapi.yaml` route/schema contract until generated OpenAPI server types are assigned. Classified CLI/app `nexdev serve` wiring, token command UX, MCP dispatch, generated server types, task/config/provider mutation services, auth audit/rate limiting, and slow-client stress coverage as follow-up work rather than M10 blockers.
- 2026-06-30 M11-SPEC-REVIEW: Accepted M11 MCP-compatible tool surface without `SPEC.md` changes after updating `api/mcp_tools.json` to include the same static input schemas exposed by `internal/controlplane.MCPTools()` and adding a manifest parity test. Approved disabling legacy imported stdio MCP registration until a safe adapter over the M11 control-plane dispatcher exists. Deferred artifact file-content reads to a validated artifact reader and provider-test execution to injected `ProviderTester` wiring.
- 2026-06-30 M12-SPEC-REVIEW: Accepted M12 CLI/Application control-plane wiring without `SPEC.md` changes. Classified `nexdev serve`, project-local server secret/token UX, required global flags, project lock lifecycle, local read commands through the same control-plane handler, remote client adapters, and app-owned detour workflow wiring as acceptable M12 scope. Deferred full `run --fake-provider --no-tui --json`, live `events --follow`, verify/handoff, TUI, provider-test execution, legacy geoffrussy command cleanup, generated OpenAPI server types, and app-level pipeline/run starter service to later milestones.
- 2026-06-30 M13-SPEC-REVIEW: Accepted M13 Terminal TUI/Web Control UX scope decision without `SPEC.md` changes. Approved the orchestrator-selected terminal-only scope and recorded embedded web UI as deferred. Classified inline plan edit/review dialogs and rich steering/detour text entry as later UX/service follow-ups because M13 correctly renders disabled/deferred states and does not mutate plan/pipeline state outside injected control-plane/service paths.
