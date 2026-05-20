"use client";

import { useEffect, useState } from "react";
import { TrendingUp, TrendingDown, Minus, AlertTriangle, CheckCircle, Lightbulb, AlertCircle } from "lucide-react";
import { api } from "@mantica/core/api";
import type { AgentScore, AgentScoreHistoryEntry, AgentHint } from "@mantica/core/types";

function TrendIcon({ trend }: { trend: string }) {
  if (trend === "improving") return <TrendingUp className="h-4 w-4 text-green-500" />;
  if (trend === "declining") return <TrendingDown className="h-4 w-4 text-red-500" />;
  return <Minus className="h-4 w-4 text-muted-foreground" />;
}

function ScoreBar({ value, max = 1200, label }: { value: number; max?: number; label: string }) {
  const pct = Math.min(100, Math.max(0, (value / max) * 100));
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-mono">{value.toFixed(0)}</span>
      </div>
      <div className="h-2 bg-muted rounded-full overflow-hidden">
        <div
          className="h-full bg-primary rounded-full transition-all"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

export function PerformanceTab({ agentId }: { agentId: string }) {
  const [score, setScore] = useState<AgentScore | null>(null);
  const [history, setHistory] = useState<AgentScoreHistoryEntry[]>([]);
  const [hints, setHints] = useState<AgentHint[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getAgentScore(agentId).catch(() => null),
      api.getAgentScoreHistory(agentId, 20).catch(() => []),
      api.getAgentHints(agentId, 7).catch(() => ({ agent_id: agentId, hints: [] })),
    ]).then(([s, h, hintResp]) => {
      setScore(s);
      setHistory(h);
      setHints(hintResp?.hints ?? []);
      setLoading(false);
    });
  }, [agentId]);

  if (loading) {
    return <div className="p-4 text-sm text-muted-foreground">Loading performance data...</div>;
  }

  if (!score) {
    return (
      <div className="p-4 text-sm text-muted-foreground">
        No performance data yet. Scores are generated after tasks complete.
      </div>
    );
  }

  return (
    <div className="space-y-6 p-4">
      {/* Score Overview */}
      <div className="flex items-center gap-4">
        <div className="text-4xl font-bold">{score.overall_score.toFixed(0)}</div>
        <div className="flex items-center gap-1">
          <TrendIcon trend={score.score_trend} />
          <span className="text-sm text-muted-foreground capitalize">{score.score_trend}</span>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-4 gap-4">
        <div className="space-y-1">
          <div className="text-2xl font-semibold">{score.total_tasks}</div>
          <div className="text-xs text-muted-foreground">Total Tasks</div>
        </div>
        <div className="space-y-1">
          <div className="text-2xl font-semibold text-green-600">{score.successful_tasks}</div>
          <div className="text-xs text-muted-foreground">Successful</div>
        </div>
        <div className="space-y-1">
          <div className="text-2xl font-semibold text-red-600">{score.failed_tasks}</div>
          <div className="text-xs text-muted-foreground">Failed</div>
        </div>
        <div className="space-y-1">
          <div className="text-2xl font-semibold">{(score.success_rate * 100).toFixed(0)}%</div>
          <div className="text-xs text-muted-foreground">Success Rate</div>
        </div>
      </div>

      {/* Tool Usage */}
      <div className="space-y-3">
        <h4 className="text-sm font-medium">Execution Metrics</h4>
        <ScoreBar value={score.avg_tool_count} max={30} label="Avg Tools per Task" />
        <ScoreBar value={score.avg_error_count} max={30} label="Avg Errors per Task" />
        <ScoreBar value={score.avg_error_rate * 100} max={100} label="Error Rate %" />
      </div>

      {/* Improvement Hints */}
      {hints.length > 0 && (
        <div className="space-y-3">
          <h4 className="text-sm font-medium flex items-center gap-2">
            <Lightbulb className="h-4 w-4 text-yellow-500" />
            Lessons Learned
          </h4>
          <div className="space-y-2">
            {hints.map((hint, i) => (
              <div key={i} className="rounded-md border border-yellow-200 bg-yellow-50 dark:border-yellow-900 dark:bg-yellow-950 p-3">
                <div className="flex items-start gap-2">
                  <AlertCircle className="h-4 w-4 text-yellow-600 mt-0.5 shrink-0" />
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-mono bg-yellow-200 dark:bg-yellow-900 px-1.5 py-0.5 rounded">
                        {hint.failure_class}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {hint.occurrence_count}x
                      </span>
                    </div>
                    <p className="text-sm">{hint.improvement_hint}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recent History */}
      {history.length > 0 && (
        <div className="space-y-3">
          <h4 className="text-sm font-medium">Recent Score Changes</h4>
          <div className="space-y-2">
            {history.slice(0, 10).map((h, i) => (
              <div key={i} className="flex items-center gap-3 text-sm">
                {h.success ? (
                  <CheckCircle className="h-4 w-4 text-green-500 shrink-0" />
                ) : (
                  <AlertTriangle className="h-4 w-4 text-red-500 shrink-0" />
                )}
                <span className="text-muted-foreground w-16">
                  {h.score_delta > 0 ? "+" : ""}{h.score_delta.toFixed(1)}
                </span>
                <span className="flex-1 truncate text-xs text-muted-foreground">
                  {h.tool_count} tools, {h.error_count} errors
                </span>
                <span className="text-xs text-muted-foreground">
                  {new Date(h.created_at).toLocaleDateString()}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
