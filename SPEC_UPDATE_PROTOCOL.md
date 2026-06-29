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

No implementation milestone spec updates have occurred yet. This planning session created the protocol only.
