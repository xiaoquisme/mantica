"use client";

import { Cloud, Monitor, TrendingUp, TrendingDown } from "lucide-react";
import type { Agent, AgentScore } from "@mantica/core/types";
import { ActorAvatar } from "../../common/actor-avatar";
import { statusConfig } from "../config";

function TrendIcon({ trend }: { trend?: string }) {
  if (trend === "improving") return <TrendingUp className="h-3 w-3 text-green-500" />;
  if (trend === "declining") return <TrendingDown className="h-3 w-3 text-red-500" />;
  return null;
}

export function AgentListItem({
  agent,
  score,
  isSelected,
  onClick,
}: {
  agent: Agent;
  score?: AgentScore | null;
  isSelected: boolean;
  onClick: () => void;
}) {
  const st = statusConfig[agent.status];
  const isArchived = !!agent.archived_at;

  return (
    <button
      onClick={onClick}
      className={`w-full rounded-lg border p-4 text-left transition-all ${
        isSelected
          ? "border-primary bg-accent shadow-sm"
          : "border-transparent hover:border-border hover:bg-accent/50"
      } ${isArchived ? "opacity-50 grayscale" : ""}`}
    >
      {/* Header: avatar + name + status */}
      <div className="flex items-center gap-3 mb-3">
        <ActorAvatar actorType="agent" actorId={agent.id} size={36} className="rounded-lg" />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium">{agent.name}</span>
            {agent.runtime_mode === "cloud" ? (
              <Cloud className="h-3 w-3 text-muted-foreground" />
            ) : (
              <Monitor className="h-3 w-3 text-muted-foreground" />
            )}
          </div>
          <div className="flex items-center gap-1.5 mt-0.5">
            <span className={`h-1.5 w-1.5 rounded-full ${st.dot}`} />
            <span className={`text-xs ${st.color}`}>{st.label}</span>
          </div>
        </div>
      </div>

      {/* Score row */}
      {score && (
        <div className="flex items-center justify-between">
          <div className="flex items-baseline gap-1.5">
            <span className="text-xl font-bold">{score.overall_score.toFixed(0)}</span>
            <span className="text-[10px] text-muted-foreground">ELO</span>
            <TrendIcon trend={score.score_trend} />
          </div>
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <span>
              <span className="text-green-600 font-medium">{score.successful_tasks}</span>
              <span className="mx-0.5">/</span>
              <span className="text-red-600 font-medium">{score.failed_tasks}</span>
            </span>
            <span>{score.total_tasks} tasks</span>
          </div>
        </div>
      )}

      {/* No score yet */}
      {!score && !isArchived && (
        <div className="text-xs text-muted-foreground">No tasks yet</div>
      )}
    </button>
  );
}
