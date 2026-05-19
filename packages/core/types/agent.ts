export type AgentStatus = "idle" | "working" | "blocked" | "error" | "offline";

export type AgentRuntimeMode = "local" | "cloud";

export type AgentVisibility = "workspace" | "private";

export interface RuntimeDevice {
  id: string;
  workspace_id: string;
  daemon_id: string | null;
  name: string;
  runtime_mode: AgentRuntimeMode;
  provider: string;
  status: "online" | "offline";
  device_info: string;
  metadata: Record<string, unknown>;
  owner_id: string | null;
  last_seen_at: string | null;
  created_at: string;
  updated_at: string;
}

export type AgentRuntime = RuntimeDevice;

export interface AgentTask {
  id: string;
  agent_id: string;
  runtime_id: string;
  issue_id: string | null;
  status: "queued" | "dispatched" | "running" | "completed" | "failed" | "cancelled";
  priority: number;
  dispatched_at: string | null;
  started_at: string | null;
  completed_at: string | null;
  result: unknown;
  error: string | null;
  created_at: string;
  scheduled_task_id?: string | null;
}

export interface Agent {
  id: string;
  workspace_id: string;
  runtime_id: string;
  name: string;
  description: string;
  instructions: string;
  avatar_url: string | null;
  runtime_mode: AgentRuntimeMode;
  runtime_config: Record<string, unknown>;
  visibility: AgentVisibility;
  status: AgentStatus;
  max_concurrent_tasks: number;
  owner_id: string | null;
  skills: Skill[];
  created_at: string;
  updated_at: string;
  archived_at: string | null;
  archived_by: string | null;
}

export interface CreateAgentRequest {
  name: string;
  description?: string;
  instructions?: string;
  avatar_url?: string;
  runtime_id: string;
  runtime_config?: Record<string, unknown>;
  visibility?: AgentVisibility;
  max_concurrent_tasks?: number;
}

export interface UpdateAgentRequest {
  name?: string;
  description?: string;
  instructions?: string;
  avatar_url?: string;
  runtime_id?: string;
  runtime_config?: Record<string, unknown>;
  visibility?: AgentVisibility;
  status?: AgentStatus;
  max_concurrent_tasks?: number;
}

// Skills

export interface Skill {
  id: string;
  workspace_id: string;
  name: string;
  description: string;
  content: string;
  config: Record<string, unknown>;
  files: SkillFile[];
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface SkillFile {
  id: string;
  skill_id: string;
  path: string;
  content: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSkillRequest {
  name: string;
  description?: string;
  content?: string;
  config?: Record<string, unknown>;
  files?: { path: string; content: string }[];
}

export interface UpdateSkillRequest {
  name?: string;
  description?: string;
  content?: string;
  config?: Record<string, unknown>;
  files?: { path: string; content: string }[];
}

export interface SetAgentSkillsRequest {
  skill_ids: string[];
}

export type RuntimePingStatus = "pending" | "running" | "completed" | "failed" | "timeout";

export interface RuntimePing {
  id: string;
  runtime_id: string;
  status: RuntimePingStatus;
  output?: string;
  error?: string;
  duration_ms?: number;
  created_at: string;
  updated_at: string;
}

export interface IssueUsageSummary {
  total_input_tokens: number;
  total_output_tokens: number;
  total_cache_read_tokens: number;
  total_cache_write_tokens: number;
  task_count: number;
}

export interface RuntimeUsage {
  runtime_id: string;
  date: string;
  provider: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
}

export interface RuntimeHourlyActivity {
  hour: number;
  count: number;
}

export type RuntimeUpdateStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "timeout";

export interface RuntimeUpdate {
  id: string;
  runtime_id: string;
  status: RuntimeUpdateStatus;
  target_version: string;
  output?: string;
  error?: string;
  created_at: string;
  updated_at: string;
}

// Agent Scoring & Analysis types

export interface AgentScore {
  agent_id: string;
  agent_name: string;
  overall_score: number;
  task_type_scores: Record<string, number>;
  total_tasks: number;
  successful_tasks: number;
  failed_tasks: number;
  success_rate: number;
  avg_tool_count: number;
  avg_error_count: number;
  avg_error_rate: number;
  score_trend: "improving" | "stable" | "declining";
  updated_at: string;
}

export interface AgentScoreHistoryEntry {
  task_id: string;
  task_type: string;
  score_before: number;
  score_after: number;
  score_delta: number;
  success: boolean;
  tool_count: number;
  error_count: number;
  created_at: string;
}

export interface TaskAnalysis {
  task_id: string;
  tool_count: number;
  error_count: number;
  unique_tools: number;
  total_duration_ms: number;
  message_count: number;
  failure_class: string;
  failure_detail: string;
  tool_usage: Record<string, { count: number; errors: number }>;
  has_retry_pattern: boolean;
  has_error_recovery: boolean;
  summary: string;
  improvement_hint: string;
  created_at: string;
}

export interface TaskAnalysisWithAgent extends TaskAnalysis {
  agent_id: string;
}

export interface AgentHint {
  failure_class: string;
  improvement_hint: string;
  summary: string;
  occurrence_count: number;
  last_seen: string;
}

export interface AgentHintsResponse {
  agent_id: string;
  hints: AgentHint[];
}

export interface AgentInsight {
  type: "warning" | "success" | "info";
  agent_id: string;
  agent_name: string;
  title: string;
  detail: string;
  action?: string;
  action_id?: string;
}

export interface SmartSummaryResponse {
  generated_at: string;
  insights: AgentInsight[];
  summary: string;
}
