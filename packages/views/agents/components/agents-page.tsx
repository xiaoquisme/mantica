"use client";

import { useState, useEffect, useMemo } from "react";
import { useDefaultLayout } from "react-resizable-panels";
import {
  Bot, Plus, Archive, AlertTriangle, CheckCircle, Info,
  TrendingUp, TrendingDown, Minus, Lightbulb, Send,
} from "lucide-react";
import type { CreateAgentRequest, UpdateAgentRequest, AgentScore, AgentInsight, SmartSummaryResponse } from "@multica/core/types";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@multica/ui/components/ui/resizable";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { toast } from "sonner";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { api } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { runtimeListOptions } from "@multica/core/runtimes/queries";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { agentListOptions, workspaceKeys } from "@multica/core/workspace/queries";
import { CreateAgentDialog } from "./create-agent-dialog";
import { AgentListItem } from "./agent-list-item";
import { AgentDetail } from "./agent-detail";

// ── Insight Card ──

function InsightCard({ insight, onSelectAgent }: {
  insight: AgentInsight;
  onSelectAgent: (id: string) => void;
}) {
  const iconMap = {
    warning: <AlertTriangle className="h-4 w-4 text-yellow-500 shrink-0" />,
    success: <CheckCircle className="h-4 w-4 text-green-500 shrink-0" />,
    info: <Info className="h-4 w-4 text-blue-500 shrink-0" />,
  };
  const borderMap = {
    warning: "border-yellow-200 bg-yellow-50 dark:border-yellow-900 dark:bg-yellow-950",
    success: "border-green-200 bg-green-50 dark:border-green-900 dark:bg-green-950",
    info: "border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950",
  };

  return (
    <div className={`flex items-start gap-3 rounded-lg border p-3 ${borderMap[insight.type]}`}>
      {iconMap[insight.type]}
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium">{insight.title}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{insight.detail}</div>
      </div>
      {insight.action && insight.action_id && (
        <Button
          variant="ghost"
          size="xs"
          className="shrink-0"
          onClick={() => insight.action_id && onSelectAgent(insight.action_id)}
        >
          {insight.action}
        </Button>
      )}
    </div>
  );
}

// ── Smart Summary ──

function SmartSummary({
  summary,
  onSelectAgent,
}: {
  summary: SmartSummaryResponse | null;
  onSelectAgent: (id: string) => void;
}) {
  if (!summary) return null;

  return (
    <div className="space-y-3 mb-6">
      {/* Overall status */}
      <div className="flex items-center gap-2 px-1">
        <Lightbulb className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm text-muted-foreground">{summary.summary}</span>
      </div>

      {/* Insight cards */}
      {summary.insights.length > 0 && (
        <div className="space-y-2">
          {summary.insights.map((insight, i) => (
            <InsightCard key={i} insight={insight} onSelectAgent={onSelectAgent} />
          ))}
        </div>
      )}
    </div>
  );
}

// ── Chat Panel ──

function AgentChat({ agents, scores }: { agents: any[]; scores: AgentScore[] }) {
  const [messages, setMessages] = useState<{ role: "user" | "assistant"; content: string }[]>([]);
  const [input, setInput] = useState("");

  const handleSend = () => {
    if (!input.trim()) return;
    const q = input.trim().toLowerCase();
    setMessages((prev) => [...prev, { role: "user", content: input }]);
    setInput("");

    // Simple rule-based responses (can be replaced with LLM later)
    let reply = "";

    if (q.includes("失败") || q.includes("fail")) {
      // Find agents with low win rate
      const lowAgents = scores.filter((s) => s.success_rate < 0.8 && s.total_tasks > 5);
      if (lowAgents.length > 0) {
        reply = "以下 agent 成功率低于 80%:\n" + lowAgents.map((a) =>
          `- ${a.agent_name}: ${(a.success_rate * 100).toFixed(0)}% (${a.total_tasks} tasks)`
        ).join("\n");
      } else {
        reply = "所有 agent 成功率都在 80% 以上。";
      }
    } else if (q.includes("最好") || q.includes("best") || q.includes("最强")) {
      const sorted = [...scores].sort((a, b) => b.overall_score - a.overall_score);
      if (sorted.length > 0) {
        reply = "表现最好的 agent:\n" + sorted.slice(0, 3).map((a, i) =>
          `${i + 1}. ${a.agent_name}: ${a.overall_score.toFixed(0)} ELO, ${(a.success_rate * 100).toFixed(0)}% win`
        ).join("\n");
      }
    } else if (q.includes("趋势") || q.includes("trend")) {
      const declining = scores.filter((s) => s.score_trend === "declining");
      const improving = scores.filter((s) => s.score_trend === "improving");
      reply = `趋势分析:\n- 上升中: ${improving.map((a) => a.agent_name).join(", ") || "无"}\n- 下降中: ${declining.map((a) => a.agent_name).join(", ") || "无"}\n- 稳定: ${scores.filter((s) => s.score_trend === "stable").map((a) => a.agent_name).join(", ")}`;
    } else if (q.includes("多少") || q.includes("统计") || q.includes("how many")) {
      const total = scores.reduce((acc, s) => acc + s.total_tasks, 0);
      const wins = scores.reduce((acc, s) => acc + s.successful_tasks, 0);
      reply = `总览:\n- ${scores.length} 个 agent\n- ${total} 个任务\n- ${wins} 次成功\n- 整体成功率: ${total > 0 ? ((wins / total) * 100).toFixed(0) : 0}%`;
    } else if (q.includes("qa") || q.includes("QA")) {
      const qa = scores.find((s) => s.agent_name.toLowerCase() === "qa");
      if (qa) {
        reply = `QA Agent:\n- 评分: ${qa.overall_score.toFixed(0)} ELO\n- 成功率: ${(qa.success_rate * 100).toFixed(0)}%\n- 任务数: ${qa.total_tasks}\n- 趋势: ${qa.score_trend}\n- 平均错误率: ${(qa.avg_error_rate * 100).toFixed(0)}%`;
      } else {
        reply = "未找到 QA agent。";
      }
    } else {
      reply = "可以问我:\n- 哪个 agent 失败最多？\n- 哪个 agent 表现最好？\n- agent 趋势如何？\n- 总体统计是多少？\n- QA agent 怎么样？";
    }

    setTimeout(() => {
      setMessages((prev) => [...prev, { role: "assistant", content: reply }]);
    }, 300);
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {messages.length === 0 && (
          <div className="text-sm text-muted-foreground text-center py-8">
            <Bot className="h-8 w-8 mx-auto mb-2 text-muted-foreground/30" />
            Ask me about your agents
          </div>
        )}
        {messages.map((msg, i) => (
          <div key={i} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
            <div className={`max-w-[80%] rounded-lg px-3 py-2 text-sm whitespace-pre-wrap ${
              msg.role === "user"
                ? "bg-primary text-primary-foreground"
                : "bg-muted"
            }`}>
              {msg.content}
            </div>
          </div>
        ))}
      </div>
      <div className="border-t p-3 flex gap-2">
        <Input
          placeholder="Ask about your agents..."
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && handleSend()}
          className="text-sm"
        />
        <Button size="icon-sm" onClick={handleSend}>
          <Send className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

// ── Main Page ──

export function AgentsPage() {
  const isLoading = useAuthStore((s) => s.isLoading);
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const [selectedId, setSelectedId] = useState<string>("");
  const [showArchived, setShowArchived] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [scores, setScores] = useState<AgentScore[]>([]);
  const [summary, setSummary] = useState<SmartSummaryResponse | null>(null);
  const { data: runtimes = [], isLoading: runtimesLoading } = useQuery(runtimeListOptions(wsId));
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_agents_layout",
  });

  const filteredAgents = useMemo(
    () => showArchived ? agents.filter((a) => !!a.archived_at) : agents.filter((a) => !a.archived_at),
    [agents, showArchived],
  );

  const archivedCount = useMemo(() => agents.filter((a) => !!a.archived_at).length, [agents]);

  useEffect(() => {
    if (filteredAgents.length > 0 && !filteredAgents.some((a) => a.id === selectedId)) {
      setSelectedId(filteredAgents[0]!.id);
    }
  }, [filteredAgents, selectedId]);

  // Fetch scores and summary
  useEffect(() => {
    api.listAgentScores().then(setScores).catch(() => {});
    api.getSmartSummary().then(setSummary).catch(() => {});
  }, []);

  const handleCreate = async (data: CreateAgentRequest) => {
    const agent = await api.createAgent(data);
    qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
    setSelectedId(agent.id);
  };

  const handleUpdate = async (id: string, data: Record<string, unknown>) => {
    try {
      await api.updateAgent(id, data as UpdateAgentRequest);
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
      toast.success("Agent updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update agent");
      throw e;
    }
  };

  const handleArchive = async (id: string) => {
    try {
      await api.archiveAgent(id);
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
      toast.success("Agent archived");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to archive agent");
    }
  };

  const handleRestore = async (id: string) => {
    try {
      await api.restoreAgent(id);
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
      toast.success("Agent restored");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to restore agent");
    }
  };

  const selected = agents.find((a) => a.id === selectedId) ?? null;

  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0">
        <div className="w-80 border-r p-4 space-y-3">
          {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-24 w-full rounded-lg" />)}
        </div>
        <div className="flex-1 p-6"><Skeleton className="h-64 w-full rounded-lg" /></div>
      </div>
    );
  }

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      {/* Left: Agent list + Smart Summary */}
      <ResizablePanel id="list" defaultSize={320} minSize={280} maxSize={400} groupResizeBehavior="preserve-pixel-size">
        <div className="overflow-y-auto h-full border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <h1 className="text-sm font-semibold">Agents</h1>
            <div className="flex items-center gap-1">
              {archivedCount > 0 && (
                <Button
                  variant={showArchived ? "secondary" : "ghost"}
                  size="icon-xs"
                  onClick={() => setShowArchived(!showArchived)}
                  title={showArchived ? "Show active" : "Show archived"}
                >
                  <Archive className="h-4 w-4 text-muted-foreground" />
                </Button>
              )}
              <Button variant="ghost" size="icon-xs" onClick={() => setShowCreate(true)}>
                <Plus className="h-4 w-4 text-muted-foreground" />
              </Button>
            </div>
          </div>

          <div className="p-3">
            {/* Smart summary */}
            <SmartSummary summary={summary} onSelectAgent={setSelectedId} />

            {/* Agent cards */}
            {filteredAgents.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12">
                <Bot className="h-8 w-8 text-muted-foreground/40" />
                <p className="mt-3 text-sm text-muted-foreground">
                  {showArchived ? "No archived agents" : archivedCount > 0 ? "No active agents" : "No agents yet"}
                </p>
                {!showArchived && (
                  <Button onClick={() => setShowCreate(true)} size="xs" className="mt-3">
                    <Plus className="h-3 w-3" /> Create Agent
                  </Button>
                )}
              </div>
            ) : (
              <div className="space-y-2">
                {filteredAgents.map((agent) => (
                  <AgentListItem
                    key={agent.id}
                    agent={agent}
                    score={scores.find((s) => s.agent_id === agent.id)}
                    isSelected={agent.id === selectedId}
                    onClick={() => setSelectedId(agent.id)}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </ResizablePanel>

      <ResizableHandle />

      {/* Right: Detail + Chat */}
      <ResizablePanel id="detail" minSize="50%">
        {selected ? (
          <ResizablePanelGroup orientation="vertical" className="h-full">
            <ResizablePanel id="detail-content" defaultSize={70} minSize={40}>
              <AgentDetail
                key={selected.id}
                agent={selected}
                runtimes={runtimes}
                onUpdate={handleUpdate}
                onArchive={handleArchive}
                onRestore={handleRestore}
              />
            </ResizablePanel>
            <ResizableHandle />
            <ResizablePanel id="chat" defaultSize={30} minSize={15}>
              <AgentChat agents={agents} scores={scores} />
            </ResizablePanel>
          </ResizablePanelGroup>
        ) : (
          <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
            <Bot className="h-10 w-10 text-muted-foreground/30" />
            <p className="mt-3 text-sm">Select an agent to view details</p>
          </div>
        )}
      </ResizablePanel>

      {showCreate && (
        <CreateAgentDialog
          runtimes={runtimes}
          runtimesLoading={runtimesLoading}
          onClose={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </ResizablePanelGroup>
  );
}
