import type {
  ControlRequest,
  DetourRequest,
  DetourResult,
  EventEnvelope,
  NexdevArtifact,
  NexdevConfig,
  NexdevEvents,
  NexdevPlan,
  NexdevProviders,
  NexdevStatus,
  RunSnapshot,
  SkipRequest,
  SteerRequest,
  UnknownRecord,
} from "./types.js";

export interface NexdevEnvironment {
  get(name: string): string | undefined;
}

export interface NexdevClientOptions {
  baseUrl?: string;
  token?: string;
  env?: NexdevEnvironment;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export interface EventQuery {
  runId?: string;
  afterSequence?: number;
  type?: string;
}

export interface StartRunRequest {
  project_dir?: string;
  prompt?: string;
  from_stage?: string;
  stage?: string;
  yes?: boolean;
  cheap?: boolean;
  brrrr?: boolean;
}

export class NexdevClientError extends Error {
  override readonly name = "NexdevClientError";
  readonly status: number | undefined;
  readonly errorCode: string | undefined;
  readonly requestID: string | undefined;
  readonly details: unknown;
  readonly serviceUnavailable: boolean;

  constructor(message: string, options: {
    status?: number | undefined;
    errorCode?: string | undefined;
    requestID?: string | undefined;
    details?: unknown;
    serviceUnavailable?: boolean | undefined;
    cause?: unknown;
  } = {}) {
    super(redactControlSecrets(message), { cause: options.cause });
    if (options.status !== undefined) {
      this.status = options.status;
    }
    if (options.errorCode !== undefined) {
      this.errorCode = options.errorCode;
    }
    if (options.requestID !== undefined) {
      this.requestID = options.requestID;
    }
    this.details = redactUnknown(options.details);
    this.serviceUnavailable = options.serviceUnavailable ?? options.status === 503;
  }
}

const DEFAULT_TIMEOUT_MS = 10_000;
const CONTROL_URL_ENV = "NEXDEV_CONTROL_URL";
const CONTROL_TOKEN_ENV = "NEXDEV_CONTROL_TOKEN";

export const processEnvironment: NexdevEnvironment = {
  get(name: string): string | undefined {
    const maybeProcess = (globalThis as unknown as { process?: { env?: Record<string, string | undefined> } }).process;
    return maybeProcess?.env?.[name];
  },
};

export function redactControlSecrets(value: string): string {
  return value
    .replace(/Authorization\s*:\s*Bearer\s+[^\s,;]+/gi, "Authorization: Bearer [REDACTED]")
    .replace(/Bearer\s+[^\s,;]+/gi, "Bearer [REDACTED]")
    .replace(/(NEXDEV_CONTROL_TOKEN\s*=\s*)[^\s,;]+/gi, "$1[REDACTED]")
    .replace(/(token["'\s:=]+)[^"'\s,;}]+/gi, "$1[REDACTED]");
}

export function redactUnknown(value: unknown): unknown {
  if (typeof value === "string") {
    return redactControlSecrets(value);
  }
  if (Array.isArray(value)) {
    return value.map((item) => redactUnknown(item));
  }
  if (value !== null && typeof value === "object") {
    const redacted: Record<string, unknown> = {};
    for (const [key, nested] of Object.entries(value)) {
      redacted[key] = /authorization|token|secret|password|api[_-]?key/i.test(key)
        ? "[REDACTED]"
        : redactUnknown(nested);
    }
    return redacted;
  }
  return value;
}

export function createNexdevClient(options: NexdevClientOptions = {}): NexdevClient {
  return new NexdevClient(options);
}

export class NexdevClient {
  private readonly baseUrl: string;
  private readonly token: string | undefined;
  private readonly fetchImpl: typeof fetch;
  private readonly timeoutMs: number;

  constructor(options: NexdevClientOptions = {}) {
    const env = options.env ?? processEnvironment;
    const baseUrl = normalizeBaseUrl(options.baseUrl ?? env.get(CONTROL_URL_ENV));
    if (baseUrl === undefined) {
      throw new NexdevClientError("Nexdev control URL is not configured.", { serviceUnavailable: true });
    }

    this.baseUrl = baseUrl;
    this.token = options.token ?? env.get(CONTROL_TOKEN_ENV);
    this.fetchImpl = options.fetchImpl ?? globalThis.fetch;
    this.timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;

    if (typeof this.fetchImpl !== "function") {
      throw new NexdevClientError("Fetch API is not available in this Pi runtime.", { serviceUnavailable: true });
    }
  }

  getStatus(signal?: AbortSignal): Promise<NexdevStatus> {
    return this.request<NexdevStatus>("GET", "/status", { signal });
  }

  getEvents(query: EventQuery = {}, signal?: AbortSignal): Promise<NexdevEvents> {
    const search = new URLSearchParams();
    if (query.runId !== undefined) {
      search.set("run_id", query.runId);
    }
    if (query.afterSequence !== undefined) {
      search.set("after_sequence", String(query.afterSequence));
    }
    if (query.type !== undefined) {
      search.set("type", query.type);
    }
    const suffix = search.size > 0 ? `?${search.toString()}` : "";
    return this.request<NexdevEvents>("GET", `/events${suffix}`, { signal });
  }

  streamEventsUrl(runId: string): string {
    return this.url(`/runs/${encodeURIComponent(runId)}/stream`).toString();
  }

  getPlan(signal?: AbortSignal): Promise<NexdevPlan> {
    return this.request<NexdevPlan>("GET", "/plan", { signal });
  }

  getArtifacts(signal?: AbortSignal): Promise<NexdevArtifact> {
    return this.request<NexdevArtifact>("GET", "/artifacts", { signal });
  }

  getConfig(signal?: AbortSignal): Promise<NexdevConfig> {
    return this.request<NexdevConfig>("GET", "/config", { signal });
  }

  getProviders(signal?: AbortSignal): Promise<NexdevProviders> {
    return this.request<NexdevProviders>("GET", "/providers", { signal });
  }

  pause(request: ControlRequest = {}, signal?: AbortSignal): Promise<NexdevStatus> {
    return this.request<NexdevStatus>("POST", "/pause", { body: request, signal });
  }

  resume(request: ControlRequest = {}, signal?: AbortSignal): Promise<NexdevStatus> {
    return this.request<NexdevStatus>("POST", "/resume", { body: request, signal });
  }

  skip(request: SkipRequest, signal?: AbortSignal): Promise<NexdevStatus> {
    return this.request<NexdevStatus>("POST", "/skip", { body: request, signal });
  }

  cancel(request: ControlRequest = {}, signal?: AbortSignal): Promise<NexdevStatus> {
    return this.request<NexdevStatus>("POST", "/cancel", { body: request, signal });
  }

  steer(request: Omit<SteerRequest, "source"> & { source?: "tui" }, signal?: AbortSignal): Promise<NexdevStatus> {
    return this.request<NexdevStatus>("POST", "/steer", { body: { ...request, source: "tui" }, signal });
  }

  detour(request: Omit<DetourRequest, "source"> & { source?: "operator_manual" }, signal?: AbortSignal): Promise<DetourResult> {
    return this.request<DetourResult>("POST", "/detour", { body: { ...request, source: "operator_manual" }, signal });
  }

  startRun(request: StartRunRequest = {}, signal?: AbortSignal): Promise<RunSnapshot> {
    return this.request<RunSnapshot>("POST", "/runs", { body: request, signal });
  }

  private async request<T>(method: string, path: string, options: { body?: unknown; signal?: AbortSignal | undefined } = {}): Promise<T> {
    const headers = new Headers({ Accept: "application/json" });
    if (options.body !== undefined) {
      headers.set("Content-Type", "application/json");
    }
    if (this.token !== undefined && this.token !== "") {
      headers.set("Authorization", `Bearer ${this.token}`);
    }

    const timeout = new AbortController();
    const timeoutID = globalThis.setTimeout(() => timeout.abort(new Error("request timeout")), this.timeoutMs);
    const signal = composeAbortSignals(timeout.signal, options.signal);

    try {
      const init: RequestInit = {
        method,
        headers,
        signal,
      };
      if (options.body !== undefined) {
        init.body = JSON.stringify(options.body);
      }

      const response = await this.fetchImpl(this.url(path), init);

      const text = await response.text();
      const data = parseJSON(text);
      if (!response.ok) {
        throw errorFromResponse(response.status, data, text);
      }
      return data as T;
    } catch (error) {
      if (error instanceof NexdevClientError) {
        throw error;
      }
      const message = error instanceof Error ? error.message : String(error);
      throw new NexdevClientError(`Nexdev control-plane request failed: ${message}`, {
        serviceUnavailable: true,
        cause: error,
      });
    } finally {
      globalThis.clearTimeout(timeoutID);
    }
  }

  private url(path: string): URL {
    return new URL(path, this.baseUrl);
  }
}

function normalizeBaseUrl(value: string | undefined): string | undefined {
  const trimmed = value?.trim();
  if (trimmed === undefined || trimmed === "") {
    return undefined;
  }
  try {
    const url = new URL(trimmed);
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      return undefined;
    }
    if (!url.pathname.endsWith("/")) {
      url.pathname = `${url.pathname}/`;
    }
    return url.toString();
  } catch {
    return undefined;
  }
}

function parseJSON(text: string): unknown {
  if (text.trim() === "") {
    return {};
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new NexdevClientError("Nexdev control plane returned invalid JSON.", { serviceUnavailable: true });
  }
}

function errorFromResponse(status: number, data: unknown, text: string): NexdevClientError {
  const body = isRecord(data) ? data : {};
  const errorCode = typeof body.error_code === "string" ? body.error_code : `http_${status}`;
  const message = typeof body.message === "string" ? body.message : `Nexdev control plane returned HTTP ${status}.`;
  const requestID = typeof body.request_id === "string" ? body.request_id : undefined;
  const details = body.details ?? (text === "" ? undefined : redactControlSecrets(text));
  return new NexdevClientError(message, { status, errorCode, requestID, details, serviceUnavailable: status === 503 });
}

function isRecord(value: unknown): value is UnknownRecord {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function composeAbortSignals(primary: AbortSignal, secondary?: AbortSignal): AbortSignal {
  if (secondary === undefined) {
    return primary;
  }
  const controller = new AbortController();
  const abort = (event: Event): void => {
    const source = event.target instanceof AbortSignal ? event.target : primary;
    controller.abort(source.reason);
  };
  if (primary.aborted) {
    controller.abort(primary.reason);
    return controller.signal;
  }
  if (secondary.aborted) {
    controller.abort(secondary.reason);
    return controller.signal;
  }
  primary.addEventListener("abort", abort, { once: true });
  secondary.addEventListener("abort", abort, { once: true });
  return controller.signal;
}
