import type { ExtensionContext } from "@earendil-works/pi-coding-agent";

import { createNexdevClient, NexdevClientError, redactControlSecrets, redactUnknown } from "./client.js";
import type { NexdevStatus, TaskSpec, UnknownRecord } from "./types.js";

const WELCOME_WIDGET_KEY = "nexdev.welcome";
const HINT_WIDGET_KEY = "nexdev.hint";
const RUN_STATUS_KEY = "nexdev.run";
const POLL_INTERVAL_MS = 3_000;
const FIELD_MAX = 28;
const FOOTER_MAX = 160;

interface WidgetSession {
  pollAbort: AbortController;
  intervalID?: ReturnType<typeof globalThis.setInterval>;
  welcomeInterval?: ReturnType<typeof globalThis.setInterval>;
  welcomeTimeout?: ReturnType<typeof globalThis.setTimeout>;
}

const sessions = new WeakMap<ExtensionContext, WidgetSession>();

export function canUseNexdevWidgets(ctx: ExtensionContext): boolean {
  return ctx.mode === "tui" || ctx.hasUI === true;
}

export function startNexdevWidgets(ctx: ExtensionContext): void {
  if (!canUseNexdevWidgets(ctx)) {
    return;
  }

  stopNexdevWidgets(ctx);
  const pollAbort = new AbortController();
  const session: WidgetSession = { pollAbort };
  const welcomeTimers = startWelcomeAnimation(ctx);
  session.welcomeInterval = welcomeTimers.intervalID;
  session.welcomeTimeout = welcomeTimers.timeoutID;
  sessions.set(ctx, session);

  showNexdevMenuHint(ctx);
  ctx.ui.setStatus(RUN_STATUS_KEY, "Nexdev: status pending");

  let client: ReturnType<typeof createNexdevClient>;
  try {
    client = createNexdevClient();
  } catch (error) {
    ctx.ui.setStatus(RUN_STATUS_KEY, safeStatusError(error));
    return;
  }

  let lastStatusText = "";
  const poll = (): void => {
    void client
      .getStatus(pollAbort.signal)
      .then((status) => {
        if (!pollAbort.signal.aborted) {
          lastStatusText = formatRunStatus(status);
          ctx.ui.setStatus(RUN_STATUS_KEY, lastStatusText);
        }
      })
      .catch((error: unknown) => {
        if (!pollAbort.signal.aborted) {
          if (isTransientControlPlaneError(error)) {
            if (lastStatusText === "") {
              ctx.ui.setStatus(RUN_STATUS_KEY, "Nexdev: connecting to control plane");
            }
            return;
          }
          ctx.ui.setStatus(RUN_STATUS_KEY, safeStatusError(error));
        }
      });
  };

  poll();
  const intervalID = globalThis.setInterval(poll, POLL_INTERVAL_MS);
  session.intervalID = intervalID;
}

export function stopNexdevWidgets(ctx: ExtensionContext): void {
  const session = sessions.get(ctx);
  if (session !== undefined) {
    session.pollAbort.abort();
    if (session.intervalID !== undefined) {
      globalThis.clearInterval(session.intervalID);
    }
    if (session.welcomeInterval !== undefined) {
      globalThis.clearInterval(session.welcomeInterval);
    }
    if (session.welcomeTimeout !== undefined) {
      globalThis.clearTimeout(session.welcomeTimeout);
    }
    sessions.delete(ctx);
  }

  if (!canUseNexdevWidgets(ctx)) {
    return;
  }

  ctx.ui.setStatus(RUN_STATUS_KEY, undefined);
  ctx.ui.setWidget(WELCOME_WIDGET_KEY, undefined, { placement: "aboveEditor" });
  ctx.ui.setWidget(HINT_WIDGET_KEY, undefined, { placement: "belowEditor" });
}

function startWelcomeAnimation(ctx: ExtensionContext): {
  intervalID: ReturnType<typeof globalThis.setInterval>;
  timeoutID: ReturnType<typeof globalThis.setTimeout>;
} {
  const frames = [
    "[boot] warming local-first engines...",
    "[db] sqlite state online...",
    "[http] loopback control plane linked...",
    "[pi] terminal cockpit armed...",
  ];
  let frame = 0;
  const render = (): void => {
    const fallback = frames[frames.length - 1] ?? "ready";
    ctx.ui.setWidget(WELCOME_WIDGET_KEY, welcomeLines(frames[frame] ?? fallback), {
      placement: "aboveEditor",
    });
    frame = Math.min(frame + 1, frames.length - 1);
  };

  render();
  const intervalID = globalThis.setInterval(render, 180);
  const timeoutID = globalThis.setTimeout(() => {
    globalThis.clearInterval(intervalID);
    renderWelcomeBanner(ctx);
  }, 1_100);
  return { intervalID, timeoutID };
}

export function renderWelcomeBanner(ctx: ExtensionContext): void {
  if (!canUseNexdevWidgets(ctx)) {
    return;
  }
  ctx.ui.setWidget(
    WELCOME_WIDGET_KEY,
    welcomeLines("ready :: Ctrl+N opens menu :: /nexdev fallback"),
    { placement: "aboveEditor" },
  );
}

function welcomeLines(status: string): string[] {
  return [
    " _   _               _            ",
    "| \\ | | _____  ____| | _____   __",
    "|  \\| |/ _ \\ \\/ / _` |/ _ \\ \\ / /",
    "| |\\  |  __/>  < (_| |  __/\\ V / ",
    "|_| \\_|\\___/_/\\_\\__,_|\\___| \\_/  ",
    `local-first coding harness :: ${status}`,
  ];
}

export function showNexdevMenuHint(ctx: ExtensionContext): void {
  if (!canUseNexdevWidgets(ctx)) {
    return;
  }
  ctx.ui.setWidget(HINT_WIDGET_KEY, ["Press Ctrl+N to open the Nexdev menu (/nexdev fallback)."], {
    placement: "belowEditor",
  });
}

export function hideNexdevMenuHint(ctx: ExtensionContext): void {
  if (!canUseNexdevWidgets(ctx)) {
    return;
  }
  ctx.ui.setWidget(HINT_WIDGET_KEY, undefined, { placement: "belowEditor" });
}

function formatRunStatus(status: NexdevStatus): string {
  const run = status.active_run;
  if (run === null) {
    return "Nexdev: no active run";
  }

  const fields = [
    `run ${shorten(run.run_id, 18)}`,
    sanitizeField(run.status),
    `stage ${shorten(stageName(status), FIELD_MAX)}`,
  ];

  const task = taskName(status.current_task ?? null);
  if (task !== undefined) {
    fields.push(`task ${shorten(task, FIELD_MAX)}`);
  }

  const cost = costFromMetadata(run.metadata);
  if (cost !== undefined) {
    fields.push(`cost ${shorten(cost, 18)}`);
  }

  return shorten(`Nexdev: ${fields.join(" | ")}`, FOOTER_MAX);
}

function stageName(status: NexdevStatus): string {
  const current = status.active_run?.current_stage;
  if (typeof current === "string" && current !== "") {
    return current;
  }
  const running = status.stages?.find((stage) => stage.status === "running");
  if (running !== undefined) {
    return running.stage;
  }
  return "unknown";
}

function taskName(task: TaskSpec | null): string | undefined {
  if (task === null) {
    return undefined;
  }
  if (task.id !== "" && task.title !== "") {
    return `${task.id}:${task.title}`;
  }
  return task.id || task.title || undefined;
}

function costFromMetadata(metadata: UnknownRecord | undefined): string | undefined {
  if (metadata === undefined) {
    return undefined;
  }

  for (const key of ["cost_summary", "cost", "usage", "provider_usage", "metadata"]) {
    const value = metadata[key];
    const found = costFromUnknown(value);
    if (found !== undefined) {
      return found;
    }
  }

  return costFromUnknown(metadata);
}

function costFromUnknown(value: unknown): string | undefined {
  if (typeof value === "number" && Number.isFinite(value)) {
    return `$${value.toFixed(4)}`;
  }
  if (typeof value === "string" && value.trim() !== "") {
    return sanitizeField(value);
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const found = costFromUnknown(item);
      if (found !== undefined) {
        return found;
      }
    }
    return undefined;
  }
  if (value === null || typeof value !== "object") {
    return undefined;
  }

  const record = value as UnknownRecord;
  for (const key of ["estimated_usd", "total_estimated_usd", "estimated_cost_usd", "total_usd", "usd"]) {
    const found = costFromUnknown(record[key]);
    if (found !== undefined) {
      return found;
    }
  }
  return undefined;
}

function safeStatusError(error: unknown): string {
  if (error instanceof NexdevClientError) {
    return shorten(`Nexdev: ${error.message}`, FOOTER_MAX);
  }
  const redacted = redactUnknown(error);
  const message = redacted instanceof Error ? redacted.message : String(redacted);
  return shorten(`Nexdev: ${redactControlSecrets(message)}`, FOOTER_MAX);
}

function isTransientControlPlaneError(error: unknown): boolean {
  return error instanceof NexdevClientError && error.serviceUnavailable && error.status === undefined;
}

function sanitizeField(value: string): string {
  return redactControlSecrets(value).replace(/[\u0000-\u001f\u007f]+/g, " ").trim();
}

function shorten(value: string, max: number): string {
  const sanitized = sanitizeField(value);
  if (sanitized.length <= max) {
    return sanitized;
  }
  if (max <= 3) {
    return sanitized.slice(0, max);
  }
  return `${sanitized.slice(0, max - 3)}...`;
}
