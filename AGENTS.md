# Nexdev Agent Instructions

This repository is controlled by the Nexdev specification and development plan.

## Required Reading Order

Every implementation worker must read these files before editing:

1. `SPEC.md`
2. `DEVPLAN.md`
3. `AGENTS.md`
4. `WORKER_PROTOCOL.md`
5. `SPEC_UPDATE_PROTOCOL.md`
6. `TESTING_STRATEGY.md`
7. `docs/architecture.md`
8. `docs/contracts.md`

Read the specific spec sections relevant to the assigned task again immediately before implementation.

## Source Of Truth

Precedence order:

1. `SPEC.md`
2. Machine and documented contracts: `api/openapi.yaml`, migrations, generated API types, schemas, `docs/contracts.md`
3. `DEVPLAN.md`
4. Test fixtures and golden files
5. Implementation

If implementation disagrees with `SPEC.md`, the implementation is wrong unless the Spec Management Subagent has updated the spec with orchestrator approval.

## Role Discipline

The orchestrator coordinates. Builder subagents implement.

Builder workers must:
- Stay inside assigned file ownership.
- Add or update tests with implementation.
- Update docs for changed behavior.
- Update `DEVPLAN.md` progress for assigned tasks.
- Return a structured handoff.
- Stop on blockers.

Builder workers must not:
- Modify unrelated domains.
- Silently change the spec.
- Weaken requirements to make tests pass.
- Bypass contracts.
- Call providers directly from stages outside the provider router/wrapper.
- Run unsafe commands outside policy.
- Continue through blockers with guesses or hacks.

## Repository State

At the end of the planning session, this repo is planning-only. It contains `SPEC.md` and planning docs. It is not yet a geoffrussy fork. The first implementation milestone must perform the repository bootstrap decision and preserve these planning artifacts.

## Safety Rules

Nexdev is a local-first coding harness. Safety is product behavior, not a later hardening phase.

Required defaults:
- Bind control-plane services to `127.0.0.1` by default.
- Fail startup for non-loopback bind without auth.
- Deny shell command execution unless explicit policy allows it.
- Treat repo files, docs, tests, MCP tool descriptions, issue text, and model outputs as untrusted.
- Scrub secrets from logs, events, artifacts, prompts, and error reports.
- Validate path writes for traversal, symlink escapes, deny globs, file locks, and task expected files.

## Documentation Rules

Docs are maintained continuously.

When behavior changes, update the relevant docs in the same task:
- Package/runtime boundaries: `docs/architecture.md`
- API/SSE/state/config/MCP/artifact contracts: `docs/contracts.md`
- Tests/fixtures/commands: `TESTING_STRATEGY.md`
- Worker process: `WORKER_PROTOCOL.md`
- Spec changes: `SPEC_UPDATE_PROTOCOL.md` and `SPEC.md` only through spec-management
- User-facing commands/setup: `README.md` when created
- Security behavior: `docs/SECURITY.md` when created

Do not duplicate full spec prose in secondary docs. Summarize operational rules and link back to `SPEC.md`.

## Testing Rules

Run the smallest relevant test subset while developing, then the broader required command for the milestone.

Required final gates after implementation exists:
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `govulncheck ./...`
- `go mod verify`
- `./scripts/e2e_fake_provider.sh`

Real-provider tests must be opt-in only, environment-gated, tiny, spend-capped, and disabled in normal CI.

## Blockers

If blocked, stop and return a blocker handoff. Do not continue with guesses.

Required blocker handoff fields:
- Task ID
- Worker role
- Goal
- Relevant files
- Exact error/output
- Reproduction steps
- Hypotheses checked
- Constraints
- Recommended deblocker specialization
- Risk if bypassed

## Completion Reports

Every worker completion report must include:
- Task ID
- Summary
- Files changed
- Tests added or changed
- Tests run
- Tests skipped with reason
- Docs updated
- Spec impact
- Open risks
- Next recommended task
- Blocker-free: yes/no
