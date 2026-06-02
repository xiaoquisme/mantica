"use client";

import { useState, useEffect, useCallback } from "react";
import { ListTodo, Clock, ChevronDown, ChevronRight, Wrench, Coins, Cpu, Zap, GitBranch, Users } from "lucide-react";
import type { Agent, AgentTask, TaskUsage, TaskMessagePayload } from "@mantica/core/types";
import { Skeleton } from "@mantica/ui/components/ui/skeleton";
import { api } from "@mantica/core/api";
import { useWorkspaceId } from "@mantica/core/hooks";
import { issueListOptions } from "@mantica/core/issues/queries";
import { useQuery } from "@tanstack/react-query";
import { AppLink } from "../../../navigation";
import { taskStatusConfig } from "../../config";

function formatTokenCount(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}K`;
  return String(count);
}

function SubTaskItem({ task }: { task: AgentTask }) {
  const config = taskStatusConfig[task.status] ?? taskStatusConfig.queued!;
  const Icon = config.icon;
  const isRunning = task.status === "running";

  return (
    <div className="flex items-center gap-2 rounded-md bg-background px-3 py-2 border text-xs">
      <Icon className={`h-3 w-3 shrink-0 ${config.color} ${isRunning ? "animate-spin" : ""}`} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          {task.subagent_role && (
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
              {task.subagent_role}
            </span>
          )}
          <span className="truncate">
            {task.issue_id ? `Issue ${task.issue_id.slice(0, 8)}...` : "Subtask"}
          </span>
        </div>
        <div className="text-muted-foreground mt-0.5">
          {task.completed_at
            ? `${task.status === "completed" ? "Completed" : "Failed"} ${new Date(task.completed_at).toLocaleString()}`
            : task.started_at
              ? `Started ${new Date(task.started_at).toLocaleString()}`
              : `Queued ${new Date(task.created_at).toLocaleString()}`}
        </div>
      </div>
      <span className={`shrink-0 text-[10px] font-medium ${config.color}`}>
        {config.label}
      </span>
    </div>
  );
}

function TaskExecutionPanel({ task }: { task: AgentTask }) {
  const [messages, setMessages] = useState<TaskMessagePayload[]>([]);
  const [usage, setUsage] = useState<TaskUsage[]>([]);
  const [subtasks, setSubtasks] = useState<AgentTask[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api.listTaskMessages(task.id).catch(() => []),
      api.getTaskUsage(task.id).catch(() => []),
      task.total_subagents && task.total_subagents > 0
        ? api.getSubTasks(task.id).catch(() => [])
        : Promise.resolve([]),
    ]).then(([msgs, u, subs]) => {
      setMessages(msgs);
      setUsage(u);
      setSubtasks(subs);
    }).finally(() => setLoading(false));
  }, [task.id, task.total_subagents]);

  if (loading) {
    return (
      <div className="px-4 py-3 space-y-2">
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    );
  }

  const toolMessages = messages.filter((m) => m.type === "tool_use");
  const totalInputTokens = usage.reduce((sum, u) => sum + u.input_tokens, 0);
  const totalOutputTokens = usage.reduce((sum, u) => sum + u.output_tokens, 0);
  const totalCacheRead = usage.reduce((sum, u) => sum + u.cache_read_tokens, 0);
  const totalCacheWrite = usage.reduce((sum, u) => sum + u.cache_write_tokens, 0);
  const uniqueModels = [...new Set(usage.map((u) => u.model))];

  return (
    <div className="px-4 py-3 space-y-4 bg-muted/30">
      {/* Token Usage Summary */}
      {(totalInputTokens > 0 || totalOutputTokens > 0) && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <Coins className="h-3.5 w-3.5" />
            Token Usage
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
            <div className="rounded-md bg-background px-3 py-2 border">
              <div className="text-xs text-muted-foreground">Input</div>
              <div className="text-sm font-semibold">{formatTokenCount(totalInputTokens)}</div>
            </div>
            <div className="rounded-md bg-background px-3 py-2 border">
              <div className="text-xs text-muted-foreground">Output</div>
              <div className="text-sm font-semibold">{formatTokenCount(totalOutputTokens)}</div>
            </div>
            <div className="rounded-md bg-background px-3 py-2 border">
              <div className="text-xs text-muted-foreground">Cache Read</div>
              <div className="text-sm font-semibold">{formatTokenCount(totalCacheRead)}</div>
            </div>
            <div className="rounded-md bg-background px-3 py-2 border">
              <div className="text-xs text-muted-foreground">Cache Write</div>
              <div className="text-sm font-semibold">{formatTokenCount(totalCacheWrite)}</div>
            </div>
          </div>
          {uniqueModels.length > 0 && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Cpu className="h-3 w-3" />
              <span>Models: {uniqueModels.join(", ")}</span>
            </div>
          )}
        </div>
      )}

      {/* Tool Calls */}
      {toolMessages.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <Wrench className="h-3.5 w-3.5" />
            Tool Calls ({toolMessages.length})
          </div>
          <div className="space-y-1 max-h-60 overflow-y-auto">
            {toolMessages.map((msg, i) => (
              <div key={i} className="flex items-start gap-2 rounded-md bg-background px-3 py-2 border text-xs">
                <Zap className="h-3 w-3 mt-0.5 shrink-0 text-blue-500" />
                <div className="min-w-0 flex-1">
                  <div className="font-medium">{msg.tool ?? "Unknown Tool"}</div>
                  {msg.input && Object.keys(msg.input).length > 0 && (
                    <div className="mt-1 text-muted-foreground truncate">
                      {Object.entries(msg.input)
                        .filter(([_, v]) => v !== undefined && v !== null && v !== "")
                        .slice(0, 3)
                        .map(([k, v]) => `${k}: ${String(v).slice(0, 50)}`)
                        .join(" · ")}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Subtasks */}
      {subtasks.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <GitBranch className="h-3.5 w-3.5" />
            Subtasks ({subtasks.length})
            {task.waiting_for_subagents && (
              <span className="text-[10px] bg-muted px-1.5 py-0.5 rounded">
                {task.completed_subagents}/{task.total_subagents} completed
              </span>
            )}
          </div>
          <div className="space-y-1 max-h-60 overflow-y-auto">
            {subtasks.map((subtask) => (
              <SubTaskItem key={subtask.id} task={subtask} />
            ))}
          </div>
        </div>
      )}

      {/* Parent Task Info */}
      {task.parent_task_id && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Users className="h-3 w-3" />
          <span>Subtask of {task.parent_task_id.slice(0, 8)}...</span>
          {task.subagent_role && (
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
              {task.subagent_role}
            </span>
          )}
        </div>
      )}

      {toolMessages.length === 0 && totalInputTokens === 0 && totalOutputTokens === 0 && subtasks.length === 0 && !task.parent_task_id && (
        <div className="text-xs text-muted-foreground text-center py-2">
          No execution data available yet.
        </div>
      )}
    </div>
  );
}

export function TasksTab({ agent }: { agent: Agent }) {
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedTaskId, setExpandedTaskId] = useState<string | null>(null);
  const wsId = useWorkspaceId();
  const { data: issues = [] } = useQuery(issueListOptions(wsId));

  useEffect(() => {
    setLoading(true);
    api
      .listAgentTasks(agent.id)
      .then(setTasks)
      .catch(() => setTasks([]))
      .finally(() => setLoading(false));
  }, [agent.id]);

  const toggleTask = useCallback((taskId: string) => {
    setExpandedTaskId((prev) => (prev === taskId ? null : taskId));
  }, []);

  if (loading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3 rounded-lg border px-4 py-3">
            <Skeleton className="h-4 w-4 rounded shrink-0" />
            <div className="flex-1 space-y-1.5">
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-3 w-1/3" />
            </div>
            <Skeleton className="h-4 w-16" />
          </div>
        ))}
      </div>
    );
  }

  // Sort: active tasks (running > dispatched > queued) first, then completed/failed by date
  const activeStatuses = ["running", "dispatched", "queued"];
  const sortedTasks = [...tasks].sort((a, b) => {
    const aActive = activeStatuses.indexOf(a.status);
    const bActive = activeStatuses.indexOf(b.status);
    const aIsActive = aActive !== -1;
    const bIsActive = bActive !== -1;
    if (aIsActive && !bIsActive) return -1;
    if (!aIsActive && bIsActive) return 1;
    if (aIsActive && bIsActive) return aActive - bActive;
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });

  const issueMap = new Map(issues.map((i) => [i.id, i]));

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold">Task Queue</h3>
        <p className="text-xs text-muted-foreground mt-0.5">
          Issues assigned to this agent and their execution status.
        </p>
      </div>

      {tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <ListTodo className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">No tasks in queue</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Assign an issue to this agent to get started.
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {sortedTasks.map((task) => {
            const config = taskStatusConfig[task.status] ?? taskStatusConfig.queued!;
            const Icon = config.icon;
            const issue = task.issue_id ? issueMap.get(task.issue_id) : null;
            const isScheduled = !!task.scheduled_task_id;
            const isActive = task.status === "running" || task.status === "dispatched";
            const isRunning = task.status === "running";
            const isExpanded = expandedTaskId === task.id;
            const hasExecutionData = task.status === "completed" || task.status === "failed";

            return (
              <div key={task.id} className="rounded-lg border overflow-hidden">
                <div
                  className={`flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-muted/50 transition-colors ${
                    isRunning
                      ? "border-success/40 bg-success/5"
                      : task.status === "dispatched"
                        ? "border-info/40 bg-info/5"
                        : ""
                  }`}
                  onClick={() => hasExecutionData && toggleTask(task.id)}
                >
                  {hasExecutionData ? (
                    isExpanded ? (
                      <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                    ) : (
                      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                    )
                  ) : (
                    <Icon
                      className={`h-4 w-4 shrink-0 ${config.color} ${
                        isRunning ? "animate-spin" : ""
                      }`}
                    />
                  )}
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      {isScheduled ? (
                        <>
                          <Clock className="h-3 w-3 shrink-0 text-muted-foreground" />
                          <span className={`text-sm truncate ${isActive ? "font-medium" : ""}`}>
                            Scheduled Task
                          </span>
                        </>
                      ) : (
                        <>
                          {issue && (
                            <AppLink
                              href={`/issues/${issue.identifier}`}
                              className="shrink-0 text-xs font-mono text-muted-foreground hover:text-foreground hover:underline"
                              onClick={(e) => e.stopPropagation()}
                            >
                              {issue.identifier}
                            </AppLink>
                          )}
                          <span className={`text-sm truncate ${isActive ? "font-medium" : ""}`}>
                            {issue?.title ?? (task.issue_id ? `Issue ${task.issue_id.slice(0, 8)}...` : "Task")}
                          </span>
                        </>
                      )}
                    </div>
                    <div className="text-xs text-muted-foreground mt-0.5">
                      {isRunning && task.started_at
                        ? `Started ${new Date(task.started_at).toLocaleString()}`
                        : task.status === "dispatched" && task.dispatched_at
                          ? `Dispatched ${new Date(task.dispatched_at).toLocaleString()}`
                          : task.status === "completed" && task.completed_at
                            ? `Completed ${new Date(task.completed_at).toLocaleString()}`
                            : task.status === "failed" && task.completed_at
                              ? `Failed ${new Date(task.completed_at).toLocaleString()}`
                              : `Queued ${new Date(task.created_at).toLocaleString()}`}
                    </div>
                  </div>
                  <span className={`shrink-0 text-xs font-medium ${config.color}`}>
                    {config.label}
                  </span>
                </div>
                {isExpanded && <TaskExecutionPanel task={task} />}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
