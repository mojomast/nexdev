import type { ExtensionContext, Theme } from "@earendil-works/pi-coding-agent";

import { createNexdevClient, NexdevClientError, redactControlSecrets, redactUnknown } from "./client.js";
import { steerNexdev } from "./steer.js";
import type {
  ArtifactItem,
  Blocker,
  EventEnvelope,
  NexdevArtifact,
  NexdevConfig,
  NexdevEvents,
  NexdevPlan,
  NexdevProviders,
  NexdevStatus,
  ProviderStatus,
  TaskSpec,
  UnknownRecord,
} from "./types.js";

type MenuResult = "opened";
type MenuScreenID = "top" | "monitor" | "control" | "providers" | "config";
type ViewID = "overview" | "events" | "plan" | "blockers" | "artifacts" | "providers" | "config";
type ActionID = "pauseResume" | "skip" | "cancel" | "detour" | "steer" | "newRun";

type MenuEntry =
  | { label: string; description: string; action: "submenu"; target: MenuScreenID }
  | { label: string; description: string; action: "view"; view: ViewID }
  | { label: string; description: string; action: "control"; control: ActionID }
  | { label: string; description: string; action: "back" | "close" };

interface MenuScreen {
  title: string;
  message: string;
  entries: MenuEntry[];
}

interface OpenNexdevMenuOptions {
  onFirstSuccessfulOpen?: () => void;
}

interface ViewState {
  id: ViewID;
  title: string;
  status: "loading" | "ready" | "error" | "deferred";
  lines: string[];
  error?: string;
}

interface ConfirmUI {
  confirm(message: string): boolean | Promise<boolean>;
}

interface NotifyUI {
  notify(message: string, level?: "info" | "warning" | "error"): void;
}

const OVERLAY_WIDTH = 86;
const MAX_RENDER_LINES = 32;
const EVENT_BUFFER_LIMIT = 100;
const EVENT_POLL_INTERVAL_MS = 2_500;
const LONG_FIELD_MAX = 72;

let hasOpenedMenu = false;

export async function openNexdevMenu(ctx: ExtensionContext, options: OpenNexdevMenuOptions = {}): Promise<void> {
  if (!canUseTUI(ctx)) {
    return;
  }

  const result = await ctx.ui.custom<MenuResult>(
    (_tui, theme, _keybindings, done) => new NexdevMenuComponent(ctx, theme, done),
    { overlay: true, overlayOptions: { width: OVERLAY_WIDTH } },
  );

  if (result === "opened" && !hasOpenedMenu) {
    hasOpenedMenu = true;
    options.onFirstSuccessfulOpen?.();
  }
}

function canUseTUI(ctx: ExtensionContext): boolean {
  return ctx.mode === "tui" || ctx.hasUI;
}

class NexdevMenuComponent {
  readonly width = OVERLAY_WIDTH;
  focused = false;

  private readonly ctx: ExtensionContext;
  private readonly theme: Theme;
  private readonly done: (result: MenuResult) => void;
  private readonly stack: MenuScreenID[] = ["top"];
  private readonly eventBuffer: EventEnvelope[] = [];
  private selected = 0;
  private closed = false;
  private view: ViewState | undefined;
  private actionBusy = false;
  private eventsAbort: AbortController | undefined;
  private eventsInterval: ReturnType<typeof globalThis.setInterval> | undefined;

  constructor(ctx: ExtensionContext, theme: Theme, done: (result: MenuResult) => void) {
    this.ctx = ctx;
    this.theme = theme;
    this.done = done;
  }

  handleInput(data: string): void {
    if (this.closed) {
      return;
    }
    if (isKey(data, "escape")) {
      this.close();
      return;
    }

    if (this.view !== undefined) {
      if (isKey(data, "left") || isKey(data, "backspace")) {
        this.clearView();
      }
      return;
    }

    const screen = this.currentScreen();
    if (isKey(data, "up")) {
      this.selected = Math.max(0, this.selected - 1);
      return;
    }
    if (isKey(data, "down")) {
      this.selected = Math.min(screen.entries.length - 1, this.selected + 1);
      return;
    }
    if (!isKey(data, "return")) {
      return;
    }

    const entry = screen.entries[this.selected];
    if (entry === undefined) {
      return;
    }
    switch (entry.action) {
      case "close":
        this.close();
        return;
      case "back":
        this.pop();
        return;
      case "submenu":
        this.push(entry.target);
        return;
      case "view":
        this.openView(entry.view);
        return;
      case "control":
        void this.runControl(entry.control);
        return;
    }
  }

  render(width: number): string[] {
    const frameWidth = clamp(width - 4, 52, this.width);
    const innerWidth = frameWidth - 2;
    const lines: string[] = [];

    lines.push(this.color("border", `+${"-".repeat(innerWidth)}+`));
    if (this.view !== undefined) {
      this.renderView(lines, innerWidth, this.view);
    } else {
      this.renderMenu(lines, innerWidth);
    }
    lines.push(this.color("border", `+${"-".repeat(innerWidth)}+`));
    return lines.slice(0, MAX_RENDER_LINES);
  }

  invalidate(): void {}

  dispose(): void {
    this.closed = true;
    this.stopEventPolling();
  }

  private renderMenu(lines: string[], innerWidth: number): void {
    const screen = this.currentScreen();
    lines.push(this.row(` ${this.color("accent", screen.title)}`, innerWidth));
    lines.push(this.row("", innerWidth));
    for (const messageLine of wrapLine(sanitize(screen.message), innerWidth - 2)) {
      lines.push(this.row(` ${this.color("dim", messageLine)}`, innerWidth));
    }
    lines.push(this.row("", innerWidth));

    for (let index = 0; index < screen.entries.length; index++) {
      const entry = screen.entries[index];
      if (entry === undefined) {
        continue;
      }
      const selected = index === this.selected;
      const marker = selected ? ">" : " ";
      const label = selected ? this.color("accent", entry.label) : this.color("text", entry.label);
      lines.push(this.row(` ${marker} ${label}`, innerWidth));
      if (selected) {
        lines.push(this.row(`   ${this.color("dim", truncate(sanitize(entry.description), innerWidth - 4))}`, innerWidth));
      }
    }

    lines.push(this.row("", innerWidth));
    const footer = this.actionBusy
      ? "Working..."
      : "Up/Down navigate  Enter select  Back entry pops  Escape closes";
    lines.push(this.row(` ${this.color("dim", footer)}`, innerWidth));
  }

  private renderView(lines: string[], innerWidth: number, view: ViewState): void {
    lines.push(this.row(` ${this.color("accent", view.title)}`, innerWidth));
    lines.push(this.row("", innerWidth));
    const prefix = view.status === "error" ? "[ERROR]" : view.status === "deferred" ? "[DEFERRED]" : view.status === "loading" ? "[LOADING]" : "";
    if (prefix !== "") {
      lines.push(this.row(` ${this.color(view.status === "error" ? "accent" : "dim", prefix)}`, innerWidth));
      lines.push(this.row("", innerWidth));
    }
    const body = view.error !== undefined ? [view.error] : view.lines;
    for (const rawLine of body.length === 0 ? ["No data returned."] : body) {
      for (const wrapped of wrapLine(sanitize(rawLine), innerWidth - 2)) {
        lines.push(this.row(` ${wrapped}`, innerWidth));
      }
    }
    lines.push(this.row("", innerWidth));
    lines.push(this.row(` ${this.color("dim", "Left/Backspace returns to menu  Escape closes")}`, innerWidth));
  }

  private currentScreen(): MenuScreen {
    const id = this.stack[this.stack.length - 1] ?? "top";
    return screens[id];
  }

  private push(id: MenuScreenID): void {
    this.clearView();
    this.stack.push(id);
    this.selected = 0;
  }

  private pop(): void {
    this.clearView();
    if (this.stack.length > 1) {
      this.stack.pop();
    }
    this.selected = 0;
  }

  private close(): void {
    this.closed = true;
    this.stopEventPolling();
    this.done("opened");
  }

  private openView(id: ViewID): void {
    this.stopEventPolling();
    this.view = { id, title: viewTitle(id), status: "loading", lines: ["Fetching Nexdev control-plane data..."] };
    void this.loadView(id);
  }

  private clearView(): void {
    this.stopEventPolling();
    this.view = undefined;
  }

  private async loadView(id: ViewID): Promise<void> {
    const abort = new AbortController();
    if (id === "events") {
      this.eventsAbort = abort;
    }
    try {
      const client = createNexdevClient();
      switch (id) {
        case "overview": {
          const status = await client.getStatus(abort.signal);
          this.setView(id, "ready", renderOverview(status));
          return;
        }
        case "events": {
          const status = await client.getStatus(abort.signal).catch(() => undefined);
          await this.pollEvents(status?.active_run?.run_id, abort);
          this.eventsInterval = globalThis.setInterval(() => {
            void this.pollEvents(status?.active_run?.run_id, abort);
          }, EVENT_POLL_INTERVAL_MS);
          return;
        }
        case "plan": {
          const plan = await client.getPlan(abort.signal);
          this.setView(id, "ready", renderPlan(plan));
          return;
        }
        case "blockers": {
          const status = await client.getStatus(abort.signal);
          this.setView(id, "ready", renderBlockers(status.blockers ?? []));
          return;
        }
        case "artifacts": {
          const artifacts = await client.getArtifacts(abort.signal);
          this.setView(id, "ready", renderArtifacts(artifacts));
          return;
        }
        case "providers": {
          const providers = await client.getProviders(abort.signal);
          this.setView(id, "ready", renderProviders(providers));
          return;
        }
        case "config": {
          const config = await client.getConfig(abort.signal);
          this.setView(id, "ready", renderConfig(config));
          return;
        }

      }
    } catch (error) {
      if (!abort.signal.aborted) {
        this.setView(id, "error", [], formatError(error));
      }
    }
  }

  private async pollEvents(runId: string | undefined, abort: AbortController): Promise<void> {
    try {
      const client = createNexdevClient();
      const afterSequence = this.eventBuffer.at(-1)?.sequence;
      const query: { runId?: string; afterSequence?: number } = {};
      if (runId !== undefined) {
        query.runId = runId;
      }
      if (afterSequence !== undefined) {
        query.afterSequence = afterSequence;
      }
      const response = await client.getEvents(query, abort.signal);
      this.appendEvents(response);
      const lines = renderEvents(this.eventBuffer, runId);
      this.setView("events", "ready", lines);
    } catch (error) {
      if (!abort.signal.aborted) {
        this.setView("events", "error", [], formatError(error));
      }
    }
  }

  private appendEvents(response: NexdevEvents): void {
    const known = new Set(this.eventBuffer.map((event) => event.event_id));
    for (const event of response.events) {
      if (!known.has(event.event_id)) {
        this.eventBuffer.push(event);
      }
    }
    this.eventBuffer.sort((a, b) => a.sequence - b.sequence);
    if (this.eventBuffer.length > EVENT_BUFFER_LIMIT) {
      this.eventBuffer.splice(0, this.eventBuffer.length - EVENT_BUFFER_LIMIT);
    }
  }

  private stopEventPolling(): void {
    this.eventsAbort?.abort();
    this.eventsAbort = undefined;
    if (this.eventsInterval !== undefined) {
      globalThis.clearInterval(this.eventsInterval);
      this.eventsInterval = undefined;
    }
  }

  private setView(id: ViewID, status: ViewState["status"], lines: string[], error?: string): void {
    if (this.closed || this.view?.id !== id) {
      return;
    }
    this.view = error === undefined
      ? { id, title: viewTitle(id), status, lines }
      : { id, title: viewTitle(id), status, lines, error };
  }

  private async runControl(id: ActionID): Promise<void> {
    if (id === "steer") {
      this.close();
      await steerNexdev(this.ctx);
      return;
    }

    if (this.actionBusy) {
      return;
    }
    this.actionBusy = true;
    try {
      const client = createNexdevClient();
      const status = await client.getStatus();
      const run = status.active_run;
      switch (id) {
        case "pauseResume": {
          if (run === null) {
            this.notify("No active Nexdev run to pause or resume.", "warning");
            return;
          }
          const request = { run_id: run.run_id, reason: "operator control from Pi overlay" };
          if (isPausedOrBlocked(run.status)) {
            await client.resume(request);
            this.notify("Nexdev run resumed.", "info");
          } else {
            await client.pause(request);
            this.notify("Nexdev run paused.", "info");
          }
          return;
        }
        case "skip": {
          const taskID = status.current_task?.id;
          if (run === null || taskID === undefined || taskID === "") {
            this.notify("Skip disabled: no active run/current task context.", "warning");
            return;
          }
          if (!(await this.confirm(`Skip current task ${taskID}?`))) {
            return;
          }
          await client.skip({ run_id: run.run_id, task_id: taskID, reason: "operator skip from Pi overlay" });
          this.notify(`Skipped task ${taskID}.`, "info");
          return;
        }
        case "cancel": {
          if (run === null) {
            this.notify("Cancel disabled: no active Nexdev run.", "warning");
            return;
          }
          if (!(await this.confirm(`Cancel run ${run.run_id}? Admin role may be required.`))) {
            return;
          }
          await client.cancel({ run_id: run.run_id, reason: "operator cancel from Pi overlay" });
          this.notify("Nexdev run cancel requested.", "info");
          return;
        }
        case "detour": {
          const taskID = status.current_task?.id;
          if (run === null || taskID === undefined || taskID === "") {
            this.notify("Detour disabled: active run and current task are required.", "warning");
            return;
          }
          const reason = "operator requested detour from Pi overlay";
          if (!(await this.confirm(`Request detour for task ${taskID}? Reason: ${reason}`))) {
            return;
          }
          const blockerSummary = (status.blockers ?? [])
            .filter((blocker) => blocker.status !== "resolved")
            .map((blocker) => `${blocker.id}:${blocker.reason}`)
            .join("; ");
          await client.detour({
            project_id: status.project_id,
            run_id: run.run_id,
            trigger_task_id: taskID,
            reason,
            context: blockerSummary === "" ? `Pi operator detour for ${taskID}` : blockerSummary,
          });
          this.notify(`Detour requested for ${taskID}.`, "info");
          return;
        }
        case "newRun": {
          this.close();
          const input = await this.ctx.ui.editor("New Nexdev Run", "Describe what you want Nexdev to build or modify...");
          if (typeof input !== "string") {
            return;
          }
          const prompt = input.trim();
          if (prompt === "") {
            this.notify("Run prompt cannot be empty.", "warning");
            return;
          }
          const snapshot = await client.startRun({ prompt });
          this.notify(`Nexdev run started: ${snapshot.run_id} (${snapshot.status})`, "info");
          return;
        }
      }
    } catch (error) {
      this.notify(formatError(error), "error");
    } finally {
      this.actionBusy = false;
    }
  }

  private async confirm(message: string): Promise<boolean> {
    const ui = this.ctx.ui as unknown;
    if (hasConfirm(ui)) {
      return Boolean(await ui.confirm(sanitize(message)));
    }
    this.notify(`[DEFERRED: ctx.ui.confirm unavailable] ${message}`, "warning");
    return false;
  }

  private notify(message: string, level: "info" | "warning" | "error"): void {
    const ui = this.ctx.ui as unknown;
    const safe = truncate(formatMessage(message), 240);
    if (hasNotify(ui)) {
      ui.notify(safe, level);
    }
  }

  private row(content: string, innerWidth: number): string {
    return `${this.color("border", "|")}${padRight(truncate(content, innerWidth), innerWidth)}${this.color("border", "|")}`;
  }

  private color(name: "accent" | "border" | "dim" | "text", value: string): string {
    const theme = this.theme as unknown as { fg?: (name: string, value: string) => string };
    if (typeof theme.fg !== "function") {
      return value;
    }
    return theme.fg(name, value);
  }
}

const screens: Record<MenuScreenID, MenuScreen> = {
  top: {
    title: "Nexdev Menu",
    message: "Pi control overlay. Reads and mutations call the Nexdev control plane; model-callable tools are not exposed.",
    entries: [
      { label: "Monitor Run", description: "Open status, events, plan, blockers, and artifacts.", action: "submenu", target: "monitor" },
      { label: "Control Run", description: "Pause/resume, skip, cancel, detour, or deferred steer entry.", action: "submenu", target: "control" },
      { label: "Providers", description: "Read provider status; provider testing remains deferred in this overlay.", action: "submenu", target: "providers" },
      { label: "New Run", description: "Start a new Nexdev run. Opens an editor for the run prompt.", action: "control", control: "newRun" },
      { label: "Config", description: "Render redacted config from GET /config.", action: "submenu", target: "config" },
      { label: "Close Menu", description: "Close overlay and return focus to the Pi editor.", action: "close" },
    ],
  },
  monitor: {
    title: "Monitor Run",
    message: "All monitor views are read-only. Events use bounded polling and stop when the view closes.",
    entries: [
      { label: "Overview", description: "Fetch and render GET /status.", action: "view", view: "overview" },
      { label: "Events", description: "Poll GET /events with a bounded <=100 event buffer.", action: "view", view: "events" },
      { label: "Plan / Tasks", description: "Fetch and render read-only GET /plan.", action: "view", view: "plan" },
      { label: "Blockers", description: "Render blockers from GET /status.", action: "view", view: "blockers" },
      { label: "Artifacts", description: "Fetch and render GET /artifacts.", action: "view", view: "artifacts" },
      { label: "Back", description: "Return to the top-level menu.", action: "back" },
    ],
  },
  control: {
    title: "Control Run",
    message: "Mutations call operator/admin control-plane endpoints and notify success or redacted failure.",
    entries: [
      { label: "Pause / Resume", description: "GET /status, then POST /resume if paused/blocked, otherwise POST /pause.", action: "control", control: "pauseResume" },
      { label: "Skip Task", description: "Confirm, then POST /skip for the current task.", action: "control", control: "skip" },
      { label: "Cancel Run", description: "Confirm, then POST /cancel. Admin role errors are redacted.", action: "control", control: "cancel" },
      { label: "Steer", description: "Open multiline editor and POST /steer with source tui.", action: "control", control: "steer" },
      { label: "Request Detour", description: "Confirm with derived context, then POST /detour when run/task context exists.", action: "control", control: "detour" },
      { label: "Back", description: "Return to the top-level menu.", action: "back" },
    ],
  },
  providers: {
    title: "Providers",
    message: "Provider credentials are not exposed to Pi providers by this extension.",
    entries: [
      { label: "List Providers", description: "Fetch and render GET /providers.", action: "view", view: "providers" },
      { label: "Provider Test (deferred)", description: "[DEFERRED: POST /providers/{name}/test overlay UX/service wiring]", action: "close" },
      { label: "Back", description: "Return to the top-level menu.", action: "back" },
    ],
  },

  config: {
    title: "Config",
    message: "Config display is read-only and redacted. Mutation remains unavailable in this overlay.",
    entries: [
      { label: "Redacted Config", description: "Fetch and render GET /config.", action: "view", view: "config" },
      { label: "Config Mutation (deferred)", description: "[DEFERRED: PUT /config admin mutation overlay UX]", action: "close" },
      { label: "Back", description: "Return to the top-level menu.", action: "back" },
    ],
  },
};

function renderOverview(status: NexdevStatus): string[] {
  const run = status.active_run;
  const lines = [`Project: ${status.project_id}`];
  if (run === null) {
    lines.push("Run: none active");
  } else {
    lines.push(`Run: ${run.run_id}`);
    lines.push(`Status: ${run.status}`);
    lines.push(`Stage: ${run.current_stage ?? stageName(status)}`);
    lines.push(`Started: ${run.started_at ?? "unknown"}`);
    const cost = costFromMetadata(run.metadata);
    if (cost !== undefined) {
      lines.push(`Cost: ${cost}`);
    }
  }
  const task = status.current_task;
  lines.push(`Current task: ${task === null || task === undefined ? "none" : taskLabel(task)}`);
  lines.push(`Open blockers: ${(status.blockers ?? []).filter((blocker) => blocker.status !== "resolved").length}`);
  if (status.updated_at !== undefined) {
    lines.push(`Updated: ${status.updated_at}`);
  }
  return lines;
}

function renderEvents(events: EventEnvelope[], runId: string | undefined): string[] {
  const lines = [`Buffer: ${events.length}/${EVENT_BUFFER_LIMIT}`, `Run filter: ${runId ?? "latest/all"}`];
  if (events.length === 0) {
    lines.push("No events returned yet.");
    return lines;
  }
  for (const event of events.slice(-18)) {
    const task = event.task_id === undefined ? "" : ` task=${event.task_id}`;
    lines.push(`#${event.sequence} ${event.type} stage=${event.stage ?? "-"}${task} ${payloadSummary(event.payload)}`);
  }
  return lines;
}

function renderPlan(plan: NexdevPlan): string[] {
  const lines = [`Project: ${plan.project_id}`, `Run: ${plan.run_id ?? "unknown"}`, `Version: ${plan.version}`];
  if (plan.phases.length === 0) {
    lines.push("No phases returned.");
    return lines;
  }
  for (const phase of plan.phases) {
    lines.push(`Phase ${phase.number}: ${phase.title} (${phase.tasks.length} tasks)`);
    for (const task of phase.tasks.slice(0, 8)) {
      lines.push(`  ${task.id} ${task.title} [${task.risk_level}]`);
    }
    if (phase.tasks.length > 8) {
      lines.push(`  ... ${phase.tasks.length - 8} more tasks`);
    }
  }
  return lines;
}

function renderBlockers(blockers: Blocker[]): string[] {
  if (blockers.length === 0) {
    return ["No blockers returned from /status."];
  }
  return blockers.map((blocker) => `${blocker.id} status=${blocker.status} task=${blocker.task_id ?? "-"} reason=${blocker.reason}`);
}

function renderArtifacts(manifest: NexdevArtifact): string[] {
  const lines = [`Project: ${manifest.project_id}`, `Run: ${manifest.run_id ?? "unknown"}`];
  if (manifest.artifacts.length === 0) {
    lines.push("No artifacts indexed.");
    return lines;
  }
  for (const artifact of manifest.artifacts.slice(0, 20)) {
    lines.push(artifactLine(artifact));
  }
  if (manifest.artifacts.length > 20) {
    lines.push(`... ${manifest.artifacts.length - 20} more artifacts`);
  }
  return lines;
}

function renderProviders(response: NexdevProviders): string[] {
  if (response.providers.length === 0) {
    return ["No providers returned."];
  }
  return response.providers.map((provider) => providerLine(provider));
}

function renderConfig(config: NexdevConfig): string[] {
  const entries = Object.entries(config).filter(([key]) => !/token|secret|password|api[_-]?key|authorization/i.test(key));
  if (entries.length === 0) {
    return ["Redacted config returned no displayable fields."];
  }
  return entries.slice(0, 24).map(([key, value]) => `${key}: ${unknownSummary(value)}`);
}

function viewTitle(id: ViewID): string {
  switch (id) {
    case "overview":
      return "Monitor: Overview";
    case "events":
      return "Monitor: Events";
    case "plan":
      return "Monitor: Plan / Tasks";
    case "blockers":
      return "Monitor: Blockers";
    case "artifacts":
      return "Monitor: Artifacts";
    case "providers":
      return "Providers";
    case "config":
      return "Config";

  }
}

function stageName(status: NexdevStatus): string {
  const running = status.stages?.find((stage) => stage.status === "running");
  return running?.stage ?? "unknown";
}

function taskLabel(task: TaskSpec): string {
  return `${task.id} ${task.title}`;
}

function artifactLine(artifact: ArtifactItem): string {
  return `${artifact.kind} v${artifact.version} ${artifact.path}`;
}

function providerLine(provider: ProviderStatus): string {
  const state = provider.available ? "available" : "unavailable";
  const auth = provider.authenticated ? "auth" : "no-auth";
  const models = provider.models === undefined ? "" : ` models=${provider.models.slice(0, 4).join(",")}`;
  const error = provider.last_error === undefined ? "" : ` error=${provider.last_error}`;
  return `${provider.name} ${state} ${auth}${models}${error}`;
}

function isPausedOrBlocked(status: string): boolean {
  const normalized = status.toLowerCase();
  return normalized === "paused" || normalized === "blocked" || normalized.includes("paused") || normalized.includes("blocked");
}

function payloadSummary(payload: UnknownRecord): string {
  const message = payload.message ?? payload.error ?? payload.reason ?? payload.status ?? payload.summary;
  return message === undefined ? "" : truncate(unknownSummary(message), 44);
}

function costFromMetadata(metadata: UnknownRecord | undefined): string | undefined {
  if (metadata === undefined) {
    return undefined;
  }
  for (const key of ["cost_summary", "cost", "usage", "provider_usage", "estimated_usd", "total_usd"]) {
    const found = costFromUnknown(metadata[key]);
    if (found !== undefined) {
      return found;
    }
  }
  return undefined;
}

function costFromUnknown(value: unknown): string | undefined {
  if (typeof value === "number" && Number.isFinite(value)) {
    return `$${value.toFixed(4)}`;
  }
  if (typeof value === "string" && value.trim() !== "") {
    return value;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const found = costFromUnknown(item);
      if (found !== undefined) {
        return found;
      }
    }
  }
  if (value !== null && typeof value === "object") {
    const record = value as UnknownRecord;
    for (const key of ["estimated_usd", "total_estimated_usd", "estimated_cost_usd", "total_usd", "usd"]) {
      const found = costFromUnknown(record[key]);
      if (found !== undefined) {
        return found;
      }
    }
  }
  return undefined;
}

function unknownSummary(value: unknown): string {
  const redacted = redactUnknown(value);
  if (typeof redacted === "string") {
    return redacted;
  }
  if (typeof redacted === "number" || typeof redacted === "boolean") {
    return String(redacted);
  }
  if (redacted === null || redacted === undefined) {
    return String(redacted);
  }
  try {
    return JSON.stringify(redacted);
  } catch {
    return "[unrenderable]";
  }
}

function formatError(error: unknown): string {
  if (error instanceof NexdevClientError) {
    const code = error.errorCode === undefined ? "" : ` (${error.errorCode})`;
    return formatMessage(`${error.message}${code}`);
  }
  const redacted = redactUnknown(error);
  const message = redacted instanceof Error ? redacted.message : unknownSummary(redacted);
  return formatMessage(message);
}

function formatMessage(message: string): string {
  return sanitize(redactControlSecrets(message));
}

function sanitize(value: string): string {
  return redactControlSecrets(value).replace(/[\u0000-\u001f\u007f]+/g, " ").trim();
}

function hasConfirm(value: unknown): value is ConfirmUI {
  return value !== null && typeof value === "object" && typeof (value as { confirm?: unknown }).confirm === "function";
}

function hasNotify(value: unknown): value is NotifyUI {
  return value !== null && typeof value === "object" && typeof (value as { notify?: unknown }).notify === "function";
}

function isKey(data: string, key: "escape" | "return" | "up" | "down" | "left" | "backspace"): boolean {
  switch (key) {
    case "escape":
      return data === "\u001b";
    case "return":
      return data === "\r" || data === "\n";
    case "up":
      return data === "\u001b[A" || data === "\u001bOA";
    case "down":
      return data === "\u001b[B" || data === "\u001bOB";
    case "left":
      return data === "\u001b[D" || data === "\u001bOD";
    case "backspace":
      return data === "\u007f" || data === "\b";
  }
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function wrapLine(line: string, maxWidth: number): string[] {
  const safe = truncate(line, LONG_FIELD_MAX * 3);
  const words = safe.split(/\s+/).filter((word) => word.length > 0);
  const lines: string[] = [];
  let current = "";
  for (const word of words) {
    const next = current === "" ? word : `${current} ${word}`;
    if (visibleLength(next) > maxWidth && current !== "") {
      lines.push(current);
      current = word;
    } else {
      current = next;
    }
  }
  if (current !== "") {
    lines.push(current);
  }
  return lines.length === 0 ? [""] : lines;
}

function padRight(value: string, width: number): string {
  const length = visibleLength(value);
  return value + " ".repeat(Math.max(0, width - length));
}

function truncate(value: string, maxWidth: number): string {
  if (maxWidth <= 0 || visibleLength(value) <= maxWidth) {
    return value;
  }
  const suffix = "...";
  const limit = Math.max(0, maxWidth - suffix.length);
  let output = "";
  for (const char of value) {
    if (visibleLength(output + char) > limit) {
      break;
    }
    output += char;
  }
  return output + suffix;
}

function visibleLength(value: string): number {
  return value.replace(/\u001b\[[0-9;]*m/g, "").length;
}
