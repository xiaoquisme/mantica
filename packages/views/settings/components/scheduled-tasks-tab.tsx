"use client";

import { useEffect, useState, useCallback } from "react";
import { Clock, Trash2, Play, Plus, Power, PowerOff } from "lucide-react";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import type { ScheduledTask, Agent } from "@multica/core/types";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Label } from "@multica/ui/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@multica/ui/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Badge } from "@multica/ui/components/ui/badge";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { useWorkspaceStore } from "@multica/core/workspace";

const SCHEDULE_PRESETS = [
  { label: "Every 15 minutes", value: "*/15 * * * *" },
  { label: "Every 30 minutes", value: "*/30 * * * *" },
  { label: "Every hour", value: "0 * * * *" },
  { label: "Every 2 hours", value: "0 */2 * * *" },
  { label: "Every 6 hours", value: "0 */6 * * *" },
  { label: "Daily at midnight", value: "0 0 * * *" },
  { label: "Custom", value: "custom" },
];

function formatNextRun(nextRunAt: string | null): string {
  if (!nextRunAt) return "Not scheduled";
  const d = new Date(nextRunAt);
  const now = new Date();
  const diffMs = d.getTime() - now.getTime();
  if (diffMs < 0) return "Overdue";
  if (diffMs < 60_000) return "Less than a minute";
  if (diffMs < 3_600_000) return `${Math.round(diffMs / 60_000)}m`;
  if (diffMs < 86_400_000) return `${Math.round(diffMs / 3_600_000)}h`;
  return d.toLocaleDateString();
}

function formatLastRun(lastRunAt: string | null): string {
  if (!lastRunAt) return "Never";
  return new Date(lastRunAt).toLocaleString();
}

export function ScheduledTasksTab() {
  const wsId = useWorkspaceStore((s) => s.workspace?.id);
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  // Create form state
  const [formName, setFormName] = useState("");
  const [formAgentId, setFormAgentId] = useState("");
  const [formSchedulePreset, setFormSchedulePreset] = useState("*/30 * * * *");
  const [formCustomCron, setFormCustomCron] = useState("");
  const [formPrompt, setFormPrompt] = useState("");
  const [creating, setCreating] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const [taskList, agentList] = await Promise.all([
        api.listScheduledTasks(),
        api.listAgents(),
      ]);
      setTasks(taskList);
      setAgents(agentList.filter((a) => !a.archived_at));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to load data");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (wsId) loadData();
  }, [wsId, loadData]);

  const agentName = (agentId: string) =>
    agents.find((a) => a.id === agentId)?.name ?? "Unknown";

  const handleCreate = async () => {
    const schedule = formSchedulePreset === "custom" ? formCustomCron : formSchedulePreset;
    if (!formName.trim() || !formAgentId || !schedule || !formPrompt.trim()) {
      toast.error("All fields are required");
      return;
    }
    setCreating(true);
    try {
      await api.createScheduledTask({
        name: formName.trim(),
        agent_id: formAgentId,
        schedule,
        prompt: formPrompt.trim(),
      });
      toast.success("Scheduled task created");
      setShowCreate(false);
      resetForm();
      await loadData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create");
    } finally {
      setCreating(false);
    }
  };

  const handleToggle = async (st: ScheduledTask) => {
    try {
      await api.updateScheduledTask(st.id, { enabled: !st.enabled });
      toast.success(st.enabled ? "Paused" : "Enabled");
      await loadData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update");
    }
  };

  const handleRunNow = async (id: string) => {
    try {
      await api.runScheduledTaskNow(id);
      toast.success("Task triggered");
      await loadData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to run");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteScheduledTask(id);
      toast.success("Deleted");
      await loadData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete");
    }
  };

  const resetForm = () => {
    setFormName("");
    setFormAgentId("");
    setFormSchedulePreset("*/30 * * * *");
    setFormCustomCron("");
    setFormPrompt("");
  };

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Clock className="h-4 w-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold">Scheduled Tasks</h2>
          </div>
          <Button size="sm" onClick={() => setShowCreate(true)}>
            <Plus className="h-3.5 w-3.5 mr-1" />
            Add
          </Button>
        </div>

        <p className="text-xs text-muted-foreground">
          Scheduled tasks run agents on a cron schedule. Use them for periodic checks, cleanup, or any recurring work.
        </p>

        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: 2 }).map((_, i) => (
              <Card key={i}>
                <CardContent className="flex items-center gap-3">
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-3 w-64" />
                  </div>
                  <Skeleton className="h-8 w-8 rounded" />
                </CardContent>
              </Card>
            ))}
          </div>
        ) : tasks.length === 0 ? (
          <Card>
            <CardContent className="text-center py-8 text-sm text-muted-foreground">
              No scheduled tasks yet. Click &quot;Add&quot; to create one.
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {tasks.map((st) => (
              <Card key={st.id}>
                <CardContent className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium truncate">{st.name}</span>
                      <Badge variant={st.enabled ? "default" : "secondary"} className="text-[10px] px-1.5 py-0">
                        {st.enabled ? "Active" : "Paused"}
                      </Badge>
                    </div>
                    <div className="text-xs text-muted-foreground mt-0.5">
                      {agentName(st.agent_id)} · <code className="text-[11px]">{st.schedule}</code> · Next: {formatNextRun(st.next_run_at)} · Last: {formatLastRun(st.last_run_at)}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button variant="ghost" size="icon-sm" onClick={() => handleToggle(st)}>
                            {st.enabled ? <PowerOff className="h-3.5 w-3.5" /> : <Power className="h-3.5 w-3.5" />}
                          </Button>
                        }
                      />
                      <TooltipContent>{st.enabled ? "Pause" : "Enable"}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button variant="ghost" size="icon-sm" onClick={() => handleRunNow(st.id)} disabled={!st.enabled}>
                            <Play className="h-3.5 w-3.5" />
                          </Button>
                        }
                      />
                      <TooltipContent>Run now</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button variant="ghost" size="icon-sm" onClick={() => setDeleteConfirmId(st.id)}>
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        }
                      />
                      <TooltipContent>Delete</TooltipContent>
                    </Tooltip>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </section>

      {/* Create dialog */}
      <Dialog open={showCreate} onOpenChange={(v) => { if (!v) { setShowCreate(false); resetForm(); } }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Create Scheduled Task</DialogTitle>
            <DialogDescription>
              Run an agent on a recurring schedule.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="st-name">Name</Label>
              <Input id="st-name" value={formName} onChange={(e) => setFormName(e.target.value)} placeholder="e.g. Blocked Issue Patrol" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="st-agent">Agent</Label>
              <Select value={formAgentId} onValueChange={(v) => { if (v) setFormAgentId(v); }}>
                <SelectTrigger id="st-agent">
                  <SelectValue placeholder="Select an agent">
                    {formAgentId ? agents.find((a) => a.id === formAgentId)?.name ?? formAgentId : undefined}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {agents.map((a) => (
                    <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="st-schedule">Schedule</Label>
              <Select value={formSchedulePreset} onValueChange={(v) => { if (v) setFormSchedulePreset(v); }}>
                <SelectTrigger id="st-schedule"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {SCHEDULE_PRESETS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {formSchedulePreset === "custom" && (
                <Input
                  value={formCustomCron}
                  onChange={(e) => setFormCustomCron(e.target.value)}
                  placeholder="*/10 * * * *"
                  className="font-mono text-sm"
                />
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="st-prompt">Prompt</Label>
              <Textarea
                id="st-prompt"
                value={formPrompt}
                onChange={(e) => setFormPrompt(e.target.value)}
                placeholder="Instructions for the agent when this task runs..."
                rows={6}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setShowCreate(false); resetForm(); }}>Cancel</Button>
            <Button onClick={handleCreate} disabled={creating || !formName.trim() || !formAgentId || !formPrompt.trim()}>
              {creating ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={!!deleteConfirmId} onOpenChange={(v) => { if (!v) setDeleteConfirmId(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete scheduled task</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete this scheduled task. Running tasks will not be affected.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={async () => {
                if (deleteConfirmId) await handleDelete(deleteConfirmId);
                setDeleteConfirmId(null);
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
