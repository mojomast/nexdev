# Nexdev API Guide

Canonical HTTP contracts live in `api/openapi.yaml`; this guide summarizes current M17 behavior.

## HTTP

Implemented read surfaces:
- `GET /health` is unauthenticated.
- `GET /status`, `/plan`, `/artifacts`, `/events`, `/providers`, `/config`, and `/mcp/tools` are observer reads when auth is enabled.
- `GET /runs/{run_id}/stream` streams persisted SSE events and honors `Last-Event-ID`.

Implemented mutation adapters:
- `POST /pause`, `/resume`, `/skip`, `/steer`, and `/cancel` delegate to injected executor controls when app wiring has a latest run control service.
- `POST /detour` delegates to the M9 detour workflow; the route does not call providers directly or reorder tasks itself.
- `POST /blockers/{blocker_id}/resolve` updates durable blocker state.
- `POST /providers/{name}/test` delegates only to an injected provider tester. M17 app wiring injects one only under explicit real-provider smoke env gates.

Deferred or partial surfaces:
- `POST /runs` requires a full app runner service and may return service-unavailable when unwired.
- Task mutation and config mutation routes are contract-shaped but not fully implemented.
- Generated OpenAPI server types and full response drift checks remain release follow-up.

## Auth

Loopback dev mode may run without auth. Non-loopback bind without auth fails before listening. When auth is enabled, use `Authorization: Bearer <token>`. Token plaintext is returned only by `nexdev auth token create`; SQLite stores token hashes and metadata only.

## SSE

SSE frames use `id`, `event`, `retry`, and `data` with persisted `EventEnvelope` JSON. Heartbeats are comments. Nexdev does not send `data: [DONE]`. Subscriber queues are bounded; unread subscribers are closed on overflow, with package-level coverage in `internal/controlplane`.

## MCP

`POST /mcp/call` uses the same role hierarchy as HTTP and dispatches only static `nexdev_*` tools from `api/mcp_tools.json`. Tool descriptions cannot expand permissions. Legacy stdio MCP registration remains disabled until adapted over this control-plane dispatcher.
