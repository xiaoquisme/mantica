"use client";

import { useEffect, useState } from "react";
import { Save, Plus, Trash2, GitBranch, Lock, FlaskConical, Loader2, CheckCircle2 } from "lucide-react";
import { Input } from "@mantica/ui/components/ui/input";
import { Button } from "@mantica/ui/components/ui/button";
import { Card, CardContent } from "@mantica/ui/components/ui/card";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@mantica/core/auth";
import { useWorkspaceStore } from "@mantica/core/workspace";
import { useWorkspaceId } from "@mantica/core/hooks";
import { memberListOptions } from "@mantica/core/workspace/queries";
import { api } from "@mantica/core/api";
import type { WorkspaceRepo } from "@mantica/core/types";

function ProviderBadge({ url }: { url: string }) {
  if (url.includes("github.com")) {
    return (
      <span className="flex h-5 w-6 shrink-0 items-center justify-center rounded bg-neutral-800 text-[10px] font-bold text-white dark:bg-neutral-200 dark:text-neutral-900">
        GH
      </span>
    );
  }
  if (url.includes("gitlab.com")) {
    return (
      <span className="flex h-5 w-6 shrink-0 items-center justify-center rounded bg-orange-500 text-[10px] font-bold text-white">
        GL
      </span>
    );
  }
  return <GitBranch className="h-4 w-4 shrink-0 text-muted-foreground" />;
}

export function RepositoriesTab() {
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const updateWorkspace = useWorkspaceStore((s) => s.updateWorkspace);

  const [repos, setRepos] = useState<WorkspaceRepo[]>(workspace?.repos ?? []);
  const [saving, setSaving] = useState(false);
  const [testStates, setTestStates] = useState<Record<number, "idle" | "testing" | "ok" | "error">>({});

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";

  useEffect(() => {
    setRepos(workspace?.repos ?? []);
  }, [workspace]);

  const handleSave = async () => {
    if (!workspace) return;
    setSaving(true);
    try {
      const updated = await api.updateWorkspace(workspace.id, { repos });
      updateWorkspace(updated);
      toast.success("Repositories saved");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save repositories");
    } finally {
      setSaving(false);
    }
  };

  const handleAddRepo = () => {
    setRepos([...repos, { url: "", description: "", token: "" }]);
  };

  const handleRemoveRepo = (index: number) => {
    setRepos(repos.filter((_, i) => i !== index));
  };

  const handleRepoChange = (index: number, field: keyof WorkspaceRepo, value: string) => {
    setRepos(repos.map((r, i) => (i === index ? { ...r, [field]: value } : r)));
    if (field === "url" || field === "token") {
      setTestStates((prev) => ({ ...prev, [index]: "idle" }));
    }
  };

  const handleTestRepo = async (index: number) => {
    const repo = repos[index];
    if (!repo?.url) return;
    setTestStates((prev) => ({ ...prev, [index]: "testing" }));
    try {
      const result = await api.testRepo(repo.url, repo.token || undefined);
      if (result.ok) {
        setTestStates((prev) => ({ ...prev, [index]: "ok" }));
        const branchInfo = result.default_branch ? ` (default branch: ${result.default_branch})` : "";
        toast.success(`Connected${branchInfo}`);
      } else {
        setTestStates((prev) => ({ ...prev, [index]: "error" }));
        toast.error(result.error ?? "Connection failed");
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Connection failed";
      setTestStates((prev) => ({ ...prev, [index]: "error" }));
      toast.error(msg);
    }
  };

  if (!workspace) return null;

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Repositories</h2>

        <Card>
          <CardContent className="space-y-3">
            <p className="text-xs text-muted-foreground">
              Git repositories associated with this workspace. Agents use these to clone and work on code. Supports GitHub, GitLab, and any git host.
            </p>

            {repos.map((repo, index) => (
              <div key={index} className="flex gap-2">
                <div className="flex-1 space-y-1.5">
                  {/* URL row */}
                  <div className="flex items-center gap-1.5">
                    <ProviderBadge url={repo.url} />
                    <Input
                      type="url"
                      value={repo.url}
                      onChange={(e) => handleRepoChange(index, "url", e.target.value)}
                      disabled={!canManageWorkspace}
                      placeholder="https://github.com/org/repo or https://gitlab.com/org/repo"
                      className="text-sm"
                    />
                  </div>
                  {/* Description row — spacer keeps inputs left-aligned */}
                  <div className="flex items-center gap-1.5">
                    <span className="w-6 shrink-0" />
                    <Input
                      type="text"
                      value={repo.description}
                      onChange={(e) => handleRepoChange(index, "description", e.target.value)}
                      disabled={!canManageWorkspace}
                      placeholder="Description (e.g. Go backend + Next.js frontend)"
                      className="text-sm"
                    />
                  </div>
                  {/* Token row */}
                  <div className="flex items-center gap-1.5">
                    <Lock className="h-4 w-4 w-6 shrink-0 text-muted-foreground" />
                    <Input
                      type="password"
                      value={repo.token ?? ""}
                      onChange={(e) => handleRepoChange(index, "token", e.target.value)}
                      disabled={!canManageWorkspace}
                      placeholder="Personal access token (for private repos)"
                      className="text-sm"
                    />
                  </div>
                </div>
                <div className="mt-0.5 flex shrink-0 flex-col gap-0.5">
                  <Button
                    variant="outline"
                    size="icon"
                    className={
                      testStates[index] === "error"
                        ? "text-destructive"
                        : testStates[index] === "ok"
                          ? "text-green-500"
                          : "text-muted-foreground"
                    }
                    disabled={!repo.url || testStates[index] === "testing"}
                    onClick={() => handleTestRepo(index)}
                  >
                    {testStates[index] === "testing" ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : testStates[index] === "ok" ? (
                      <CheckCircle2 className="h-3.5 w-3.5" />
                    ) : (
                      <FlaskConical className="h-3.5 w-3.5" />
                    )}
                  </Button>
                  {canManageWorkspace && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-muted-foreground hover:text-destructive"
                      onClick={() => handleRemoveRepo(index)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  )}
                </div>
              </div>
            ))}

            {canManageWorkspace && (
              <div className="flex items-center justify-between pt-1">
                <Button variant="outline" size="sm" onClick={handleAddRepo}>
                  <Plus className="h-3 w-3" />
                  Add repository
                </Button>
                <Button
                  size="sm"
                  onClick={handleSave}
                  disabled={saving}
                >
                  <Save className="h-3 w-3" />
                  {saving ? "Saving..." : "Save"}
                </Button>
              </div>
            )}

            {!canManageWorkspace && (
              <p className="text-xs text-muted-foreground">
                Only admins and owners can manage repositories.
              </p>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
