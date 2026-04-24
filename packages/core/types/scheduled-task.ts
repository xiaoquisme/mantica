export interface ScheduledTask {
  id: string;
  workspace_id: string;
  name: string;
  agent_id: string;
  schedule: string;
  prompt: string;
  enabled: boolean;
  last_run_at: string | null;
  next_run_at: string | null;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateScheduledTaskRequest {
  name: string;
  agent_id: string;
  schedule: string;
  prompt: string;
}

export interface UpdateScheduledTaskRequest {
  name?: string;
  agent_id?: string;
  schedule?: string;
  prompt?: string;
  enabled?: boolean;
}
