export type UnknownRecord = Record<string, unknown>;

export type ISODateTime = string;

export type Deferred = {
  state: "deferred";
  reason: string;
  missingEndpoint?: string;
};

export type ViewState<T> = T | Deferred;

export interface ErrorResponse {
  error_code: string;
  message: string;
  details: UnknownRecord;
  request_id: string;
}

export interface HealthResponse {
  ok: boolean;
  api_contract_version: "nexdev-api-v1" | string;
}

export interface RunSnapshot {
  run_id: string;
  project_id: string;
  status: string;
  current_stage?: string;
  started_at?: ISODateTime;
  completed_at?: ISODateTime;
  metadata?: UnknownRecord;
}

export interface StageSnapshot {
  stage: string;
  status: string;
  attempt?: number;
  started_at?: ISODateTime;
  completed_at?: ISODateTime;
}

export interface StatusSnapshot {
  project_id: string;
  active_run: RunSnapshot | null;
  stages?: StageSnapshot[];
  current_task?: TaskSpec | null;
  blockers?: Blocker[];
  updated_at?: ISODateTime;
}

export interface EventEnvelope {
  event_id: string;
  sequence: number;
  contract_version: "nexdev-event-v1" | string;
  type: string;
  project_id: string;
  run_id: string;
  stage?: string;
  task_id?: string;
  ts: ISODateTime;
  source: "core" | "executor" | "worker" | "tui" | "api" | "mcp";
  payload: UnknownRecord;
}

export interface EventsResponse {
  events: EventEnvelope[];
}

export interface Plan {
  project_id: string;
  run_id?: string;
  version: number;
  phases: PhasePlan[];
}

export interface PhasePlan {
  id: string;
  number: number;
  title: string;
  description?: string;
  tasks: TaskSpec[];
}

export interface TaskSpec {
  id: string;
  phase_id: string;
  title: string;
  description: string;
  expected_files: string[];
  dependencies: string[];
  acceptance_criteria: string[];
  test_commands: string[];
  risk_level: string;
  required_tools: string[];
  notes: string[];
}

export interface TaskMutation {
  title?: string;
  description?: string;
  expected_files?: string[];
  dependencies?: string[];
  acceptance_criteria?: string[];
  test_commands?: string[];
  risk_level?: string;
  required_tools?: string[];
  notes?: string[];
  reason?: string;
}

export interface ArtifactManifest {
  project_id: string;
  run_id?: string;
  artifacts: ArtifactItem[];
}

export interface ArtifactItem {
  id: string;
  project_id?: string;
  run_id?: string;
  kind: string;
  path: string;
  sha256?: string;
  version: number;
  metadata?: UnknownRecord;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
}

export interface ProviderStatus {
  name: string;
  authenticated: boolean;
  available: boolean;
  models?: string[];
  last_error?: string;
  checked_at?: ISODateTime;
}

export interface ProvidersResponse {
  providers: ProviderStatus[];
}

export interface RedactedConfig {
  profile?: string;
  redacted?: boolean;
  [key: string]: unknown;
}

export type ConfigUpdateRequest = UnknownRecord;

export interface StartRunRequest {
  project_dir?: string;
  prompt?: string;
  from_stage?: string;
  stage?: string;
  yes?: boolean;
  cheap?: boolean;
  brrrr?: boolean;
}

export interface ControlRequest {
  run_id?: string;
  reason?: string;
}

export interface SkipRequest extends ControlRequest {
  task_id?: string;
}

export interface SteerRequest {
  run_id?: string;
  task_id?: string;
  message: string;
  source?: "cli" | "api" | "tui" | "mcp";
}

export interface DetourRequest {
  project_id: string;
  run_id: string;
  trigger_task_id: string;
  reason: string;
  context: string;
  source: "blocker_auto" | "operator_manual" | "review_replan";
}

export interface DetourResult {
  id: string;
  new_tasks: TaskSpec[];
  spliced_after: string;
  id_conflicts: string[];
  depth: number;
}

export interface Blocker {
  id: string;
  run_id: string;
  task_id?: string;
  reason: string;
  status: string;
  created_at?: ISODateTime;
}

export interface BlockerResolveRequest {
  run_id?: string;
  resolution: string;
  resume?: boolean;
}

export interface ProviderTestRequest {
  model?: string;
}

export interface MCPTool {
  name: string;
  description: string;
  role: "observer" | "operator" | "admin";
  input_schema: UnknownRecord;
}

export interface MCPToolsResponse {
  tools: MCPTool[];
}

export interface MCPCallRequest {
  tool: string;
  arguments: UnknownRecord;
}

export interface MCPCallResult {
  tool: string;
  ok: boolean;
  result?: UnknownRecord;
  error?: ErrorResponse;
}

export type AcceptedStatus = StatusSnapshot;

export type NexdevStatus = StatusSnapshot;
export type NexdevEvent = EventEnvelope;
export type NexdevEvents = EventsResponse;
export type NexdevPlan = Plan;
export type NexdevArtifact = ArtifactManifest;
export type NexdevConfig = RedactedConfig;
export type NexdevProviders = ProvidersResponse;

export interface PiParityViews {
  status: ViewState<NexdevStatus>;
  events: ViewState<NexdevEvents>;
  plan: ViewState<NexdevPlan>;
  blockers: ViewState<Blocker[]>;
  artifacts: ViewState<NexdevArtifact>;
  providers: ViewState<NexdevProviders>;
  config: ViewState<NexdevConfig>;
  newRun: ViewState<RunSnapshot>;
}
