"use client";

import { useState, useEffect, useMemo } from "react";
import { useDefaultLayout } from "react-resizable-panels";
import {
  Bot, Plus, Archive, AlertTriangle, CheckCircle, Info,
  Lightbulb,
} from "lucide-react";
import type { CreateAgentRequest, UpdateAgentRequest, AgentScore, AgentInsight, SmartSummaryResponse } from "@mantica/core/types";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@mantica/ui/components/ui/resizable";
import { Button } from "@mantica/ui/components/ui/button";
import { toast } from "sonner";
import { Skeleton } from "@mantica/ui/components/ui/skeleton";
import { api } from "@mantica/core/api";
import { useAuthStore } from "@mantica/core/auth";
import { runtimeListOptions } from "@mantica/core/runtimes/queries";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@mantica/core/hooks";
import { agentListOptions, workspaceKeys } from "@mantica/core/workspace/queries";
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
    id: "mantica_agents_layout",
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

      {/* Right: Detail */}
      <ResizablePanel id="detail" minSize="50%">
        {selected ? (
          <AgentDetail
            key={selected.id}
            agent={selected}
            runtimes={runtimes}
            onUpdate={handleUpdate}
            onArchive={handleArchive}
            onRestore={handleRestore}
          />
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
