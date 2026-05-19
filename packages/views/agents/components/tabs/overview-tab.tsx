"use client";

import { useEffect, useState } from "react";
import { TrendingUp, TrendingDown, Minus, CheckCircle, XCircle, Activity, Lightbulb } from "lucide-react";
import { api } from "@multica/core/api";
import type { Agent, AgentScore, AgentHint } from "@multica/core/types";

function MiniStat({ label, value, color }: { label: string; value: string | number; color?: string }) {
  return (
    <div className="text-center">
      <div className={`text-lg font-semibold ${color ?? ""}`}>{value}</div>
      <div className="text-[10px] text-muted-foreground uppercase tracking-wider">{label}</div>
    </div>
  );
}

export function OverviewTab({ agent }: { agent: Agent }) {
  const [score, setScore] = useState<AgentScore | null>(null);
  const [hints, setHints] = useState<AgentHint[]>([]);

  useEffect(() => {
    api.getAgentScore(agent.id).then(setScore).catch(() => {});
    api.getAgentHints(agent.id, 7).then((r) => setHints(r.hints ?? [])).catch(() => {});
  }, [agent.id]);

  const trendColor =
    score?.score_trend === "improving"
      ? "text-green-500"
      : score?.score_trend === "declining"
        ? "text-red-500"
        : "text-muted-foreground";

  return (
    <div className="space-y-5 p-4">
      {/* Agent identity */}
      <div className="flex items-center gap-3">
        <div className="text-2xl font-bold">{agent.name}</div>
        <span className="text-xs bg-secondary px-2 py-0.5 rounded-full capitalize">
          {agent.status}
        </span>
      </div>

      {/* Score card */}
      {score && (
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium">Performance</span>
            </div>
            <div className="flex items-center gap-1">
              {score.score_trend === "improving" && <TrendingUp className={`h-4 w-4 ${trendColor}`} />}
              {score.score_trend === "declining" && <TrendingDown className={`h-4 w-4 ${trendColor}`} />}
              {score.score_trend === "stable" && <Minus className="h-4 w-4 text-muted-foreground" />}
            </div>
          </div>

          {/* Big score number */}
          <div className="flex items-baseline gap-2 mb-4">
            <span className="text-3xl font-bold">{score.overall_score.toFixed(0)}</span>
            <span className="text-xs text-muted-foreground">ELO</span>
          </div>

          {/* Stats row */}
          <div className="grid grid-cols-4 gap-2">
            <MiniStat label="Tasks" value={score.total_tasks} />
            <MiniStat label="Wins" value={score.successful_tasks} color="text-green-600" />
            <MiniStat label="Losses" value={score.failed_tasks} color="text-red-600" />
            <MiniStat label="Win Rate" value={`${(score.success_rate * 100).toFixed(0)}%`} />
          </div>
        </div>
      )}

      {/* Quick metrics */}
      {score && (
        <div className="grid grid-cols-3 gap-2">
          <div className="rounded-md border p-3 text-center">
            <div className="text-sm font-mono">{score.avg_tool_count.toFixed(1)}</div>
            <div className="text-[10px] text-muted-foreground">Avg Tools</div>
          </div>
          <div className="rounded-md border p-3 text-center">
            <div className="text-sm font-mono">{(score.avg_error_rate * 100).toFixed(0)}%</div>
            <div className="text-[10px] text-muted-foreground">Error Rate</div>
          </div>
          <div className="rounded-md border p-3 text-center">
            <div className="text-sm font-mono">{score.avg_error_count.toFixed(1)}</div>
            <div className="text-[10px] text-muted-foreground">Avg Errors</div>
          </div>
        </div>
      )}

      {/* Hints */}
      {hints.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Lightbulb className="h-4 w-4 text-yellow-500" />
            Lessons Learned
          </div>
          {hints.map((h, i) => (
            <div key={i} className="rounded-md border border-yellow-200 bg-yellow-50 dark:border-yellow-900 dark:bg-yellow-950 p-3 text-sm">
              <span className="text-xs font-mono bg-yellow-200 dark:bg-yellow-900 px-1.5 py-0.5 rounded mr-2">
                {h.failure_class}
              </span>
              {h.improvement_hint}
            </div>
          ))}
        </div>
      )}

      {/* Instructions preview */}
      {agent.instructions && (
        <div className="space-y-2">
          <div className="text-sm font-medium">Instructions</div>
          <div className="rounded-md border bg-muted/50 p-3 text-xs text-muted-foreground max-h-32 overflow-y-auto whitespace-pre-wrap">
            {agent.instructions.length > 300
              ? agent.instructions.slice(0, 300) + "..."
              : agent.instructions}
          </div>
        </div>
      )}

      {/* No data state */}
      {!score && (
        <div className="text-sm text-muted-foreground text-center py-8">
          No performance data yet. Scores appear after tasks complete.
        </div>
      )}
    </div>
  );
}
