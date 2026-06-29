# Nexdev Worker Protocol

This protocol is mandatory for every later implementation subagent.

## 1. Worker Lifecycle

1. Receive one task from the orchestrator.
2. Confirm the task ID, goal, owned files, allowed shared contracts, dependencies, tests, and docs.
3. Read required context.
4. Inspect only owned files and explicitly allowed contracts.
5. Implement within ownership boundaries.
6. Add or update tests alongside implementation.
7. Run the relevant test subset.
8. Update docs for changed behavior.
9. Update `DEVPLAN.md` task status and evidence.
10. Return a structured handoff.

## 2. Required Context

Every worker must read:
- `SPEC.md`
- Relevant `SPEC.md` sections for the task
- `DEVPLAN.md`
- `AGENTS.md`
- `WORKER_PROTOCOL.md`
- `SPEC_UPDATE_PROTOCOL.md`
- `TESTING_STRATEGY.md` if writing or running tests
- `docs/architecture.md` for package/runtime boundaries
- `docs/contracts.md` for contracts

## 3. File Ownership

Workers may edit:
- Owned files assigned by the orchestrator.
- Tests for their owned package.
- Docs listed in the task.
- Shared contract files only when explicitly authorized.

Workers must not edit:
- Another domain's implementation files.
- `SPEC.md` unless assigned as Spec Management Subagent.
- Generated files unless the task owns generation.
- Migration files owned by another state task.
- Root command wiring owned by the CLI/root worker unless assigned.

If the task requires another domain change, stop and request orchestration.

## 4. Progress Updates

Workers must update `DEVPLAN.md` for assigned work:
- Mark task `in_progress` when starting.
- Mark task `implemented` when code exists but verification/docs are incomplete.
- Mark task `verified` only after required tests and docs are complete.
- Mark task `blocked` with a blocker handoff if stuck.
- Do not mark `verified` based on intent.

## 5. Testing Obligations

Every behavior change needs tests unless the orchestrator explicitly accepts a documented gap.

Minimum expectations:
- Unit tests for package-local logic.
- Integration tests for cross-package behavior.
- Golden tests for stable human/machine artifacts.
- Security tests for auth, path, tool, prompt, and secret behavior.
- Fake-provider tests for model-dependent behavior.

Run commands:
- Run the smallest relevant package command first.
- Run broader milestone commands before handoff.
- Record all commands and outcomes.

If a command cannot be run, report why and whether this blocks acceptance.

## 6. Documentation Obligations

Update docs in the same task as behavior changes.

Doc mapping:
- Runtime/package architecture: `docs/architecture.md`
- API/SSE/state/config/MCP/artifact contracts: `docs/contracts.md`
- Test strategy, fixtures, commands: `TESTING_STRATEGY.md`
- Security behavior: `docs/SECURITY.md` when created
- User-facing CLI/setup: `README.md` when created
- Operations/troubleshooting: `docs/OPERATING.md` when created

Secondary docs must not contradict `SPEC.md`.

## 7. Spec Impact

Workers must classify spec impact in handoff:
- `none`: implementation matches spec.
- `clarification-needed`: spec is ambiguous.
- `approved-change-implemented`: orchestrator approved change and spec-management must update spec.
- `deviation-blocker`: implementation cannot satisfy spec as written.
- `deferral-proposed`: requirement should move out of scope with rationale.

Workers must not edit `SPEC.md` unless acting as Spec Management Subagent.

## 8. Blocker Workflow

When blocked:

1. Stop.
2. Do not guess or hack around the problem.
3. Create a blocker handoff.
4. Return control to the orchestrator.
5. Wait for deblocker findings and reassignment.

Blocker handoff template:

```markdown
## Blocker Handoff

Task ID:
Worker role:
Goal:
Owned files:
Relevant files:
Exact error/output:
Minimal reproduction steps:
Hypotheses checked:
Constraints:
Recommended deblocker specialization:
Risk if bypassed:
```

Deblocker specializations:
- Go compile/test deblocker
- SQLite/migration deblocker
- SSE/control-plane deblocker
- Provider/LLM API deblocker
- Security/auth deblocker
- Concurrency/race deblocker
- Git/worktree deblocker
- Flaky-test deblocker
- Contract mismatch deblocker
- Docs/spec ambiguity deblocker

## 9. Completion Handoff

Use this exact shape:

```markdown
## Worker Handoff

Task ID:
Worker role:
Summary:
Files changed:
Tests added/changed:
Tests run:
Tests skipped:
Docs updated:
Contracts changed:
Spec impact:
Open risks:
Next recommended task:
Blocker-free: yes/no
```

## 10. Merge-Clean Rules

- Keep changes small and domain-local.
- Avoid global refactors unless assigned.
- Do not rename shared types without contract-owner approval.
- Do not modify generated code manually unless generated-code ownership is assigned.
- Do not add dependencies without owning worker and package recommendation alignment.
- Prefer stable boring contracts over clever abstractions.
