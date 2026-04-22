"use client";

import { useCallback, memo } from "react";
import { AppLink } from "../../navigation";
import type { Issue, UpdateIssueRequest } from "@multica/core/types";
import { ActorAvatar } from "../../common/actor-avatar";
import { useIssueSelectionStore } from "@multica/core/issues/stores/selection-store";
import { PriorityIcon } from "./priority-icon";
import { StatusIcon } from "./status-icon";
import { ProgressRing } from "./progress-ring";
import { LabelBadge } from "./label-badge";
import { useUpdateIssue } from "@multica/core/issues/mutations";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { memberListOptions, agentListOptions } from "@multica/core/workspace/queries";
import { projectListOptions } from "@multica/core/projects/queries";
import { canAssignAgent, LabelPicker, ParentSubMenuContent } from "./pickers";
import { ALL_STATUSES, STATUS_CONFIG, PRIORITY_ORDER, PRIORITY_CONFIG } from "@multica/core/issues/config";
import {
  MoreHorizontal,
  UserMinus,
  Calendar,
  FolderKanban,
  Tag,
  Check,
  Network,
} from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@multica/ui/components/ui/dropdown-menu";

export interface ChildProgress {
  done: number;
  total: number;
}

function formatDate(date: string): string {
  return new Date(date).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}

/** Stops event from bubbling to AppLink/row handlers */
function PickerWrapper({ children }: { children: React.ReactNode }) {
  const stopAndPrevent = (e: React.SyntheticEvent) => {
    e.stopPropagation();
    e.preventDefault();
  };
  const stopOnly = (e: React.SyntheticEvent) => {
    e.stopPropagation();
  };
  return (
    // onClick: preventDefault blocks native <a> navigation if this element is
    // ever nested inside an AppLink. onPointerDown/onMouseDown: stopPropagation
    // only so @base-ui Menu.Trigger can still open on pointerDown.
    <div onClick={stopAndPrevent} onMouseDown={stopOnly} onPointerDown={stopOnly}>
      {children}
    </div>
  );
}

function ListRowContextMenu({ issue, onUpdate }: {
  issue: Issue;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
}) {
  const user = useAuthStore((s) => s.user);
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const currentMemberRole = members.find((m) => m.user_id === user?.id)?.role;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            className="opacity-0 group-hover/row:opacity-100 transition-opacity text-muted-foreground"
          >
            <MoreHorizontal className="size-3.5" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-44">
        {/* Status */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <StatusIcon status={issue.status} className="h-3.5 w-3.5" />
            Status
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {ALL_STATUSES.map((s) => (
              <DropdownMenuItem
                key={s}
                onClick={() => onUpdate({ status: s })}
              >
                <StatusIcon status={s} className="h-3.5 w-3.5" />
                {STATUS_CONFIG[s].label}
                {issue.status === s && (
                  <Check className="ml-auto h-3.5 w-3.5 text-muted-foreground" />
                )}
              </DropdownMenuItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Priority */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <PriorityIcon priority={issue.priority} />
            Priority
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {PRIORITY_ORDER.map((p) => (
              <DropdownMenuItem
                key={p}
                onClick={() => onUpdate({ priority: p })}
              >
                <span
                  className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_CONFIG[p].badgeBg} ${PRIORITY_CONFIG[p].badgeText}`}
                >
                  <PriorityIcon priority={p} className="h-3 w-3" inheritColor />
                  {PRIORITY_CONFIG[p].label}
                </span>
                {issue.priority === p && (
                  <Check className="ml-auto h-3.5 w-3.5 text-muted-foreground" />
                )}
              </DropdownMenuItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Assignee */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <UserMinus className="h-3.5 w-3.5" />
            Assignee
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem
              onClick={() =>
                onUpdate({ assignee_type: null, assignee_id: null })
              }
            >
              <UserMinus className="h-3.5 w-3.5 text-muted-foreground" />
              Unassigned
              {!issue.assignee_type && (
                <Check className="ml-auto h-3.5 w-3.5 text-muted-foreground" />
              )}
            </DropdownMenuItem>
            {members.map((m) => (
              <DropdownMenuItem
                key={m.user_id}
                onClick={() =>
                  onUpdate({
                    assignee_type: "member",
                    assignee_id: m.user_id,
                  })
                }
              >
                <ActorAvatar actorType="member" actorId={m.user_id} size={16} />
                {m.name}
                {issue.assignee_type === "member" &&
                  issue.assignee_id === m.user_id && (
                    <Check className="ml-auto h-3.5 w-3.5 text-muted-foreground" />
                  )}
              </DropdownMenuItem>
            ))}
            {agents
              .filter(
                (a) =>
                  !a.archived_at &&
                  canAssignAgent(a, user?.id, currentMemberRole),
              )
              .map((a) => (
                <DropdownMenuItem
                  key={a.id}
                  onClick={() =>
                    onUpdate({
                      assignee_type: "agent",
                      assignee_id: a.id,
                    })
                  }
                >
                  <ActorAvatar actorType="agent" actorId={a.id} size={16} />
                  {a.name}
                  {issue.assignee_type === "agent" &&
                    issue.assignee_id === a.id && (
                      <Check className="ml-auto h-3.5 w-3.5 text-muted-foreground" />
                    )}
                </DropdownMenuItem>
              ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Due Date */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Calendar className="h-3.5 w-3.5" />
            Due date
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem
              onClick={() => onUpdate({ due_date: new Date().toISOString() })}
            >
              Today
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
                const d = new Date();
                d.setDate(d.getDate() + 1);
                onUpdate({ due_date: d.toISOString() });
              }}
            >
              Tomorrow
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
                const d = new Date();
                d.setDate(d.getDate() + 7);
                onUpdate({ due_date: d.toISOString() });
              }}
            >
              Next week
            </DropdownMenuItem>
            {issue.due_date && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => onUpdate({ due_date: null })}
                >
                  Clear date
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Project */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <FolderKanban className="h-3.5 w-3.5" />
            Project
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {projects.map((p) => (
              <DropdownMenuItem
                key={p.id}
                onClick={() => onUpdate({ project_id: p.id })}
              >
                <span>{p.icon || "📁"}</span>
                <span className="truncate">{p.title}</span>
                {issue.project_id === p.id && (
                  <Check className="ml-auto h-3.5 w-3.5 text-muted-foreground" />
                )}
              </DropdownMenuItem>
            ))}
            {projects.length > 0 && issue.project_id && (
              <DropdownMenuSeparator />
            )}
            {issue.project_id && (
              <DropdownMenuItem
                onClick={() => onUpdate({ project_id: null })}
              >
                Remove from project
              </DropdownMenuItem>
            )}
            {projects.length === 0 && (
              <div className="px-2 py-1.5 text-xs text-muted-foreground">
                No projects yet
              </div>
            )}
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Labels */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Tag className="h-3.5 w-3.5" />
            Labels
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <LabelPicker
              issueId={issue.id}
              currentLabels={issue.labels ?? []}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Parent */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Network className="h-3.5 w-3.5" />
            Parent
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <ParentSubMenuContent
              currentIssueId={issue.id}
              parentIssueId={issue.parent_issue_id}
              onUpdate={onUpdate}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export const ListRow = memo(function ListRow({
  issue,
  childProgress,
  parentIssue,
}: {
  issue: Issue;
  childProgress?: ChildProgress;
  parentIssue?: Issue | null;
}) {
  const selected = useIssueSelectionStore((s) => s.selectedIds.has(issue.id));
  const toggle = useIssueSelectionStore((s) => s.toggle);

  const updateIssueMutation = useUpdateIssue();
  const handleUpdate = useCallback(
    (updates: Partial<UpdateIssueRequest>) => {
      updateIssueMutation.mutate(
        { id: issue.id, ...updates },
        { onError: () => toast.error("Failed to update issue") },
      );
    },
    [issue.id, updateIssueMutation],
  );

  return (
    <div
      className={`group/row flex h-9 items-center gap-2 px-4 text-sm transition-colors hover:bg-accent/50 ${
        selected ? "bg-accent/30" : ""
      }`}
    >
      <div className="relative flex shrink-0 items-center justify-center w-4 h-4">
        <PriorityIcon
          priority={issue.priority}
          className={selected ? "hidden" : "group-hover/row:hidden"}
        />
        <input
          type="checkbox"
          checked={selected}
          onChange={() => toggle(issue.id)}
          className={`absolute inset-0 cursor-pointer accent-primary ${
            selected ? "" : "hidden group-hover/row:block"
          }`}
        />
      </div>
      <AppLink
        href={`/issues/${issue.identifier}`}
        className="flex flex-1 items-center gap-2 min-w-0"
      >
        <span className="w-16 shrink-0 text-xs text-muted-foreground">
          {issue.identifier}
        </span>
        {parentIssue && (
          <span className="inline-flex shrink-0 items-center gap-1 rounded bg-muted/60 px-1.5 py-0.5 text-xs text-muted-foreground">
            <Network className="h-3 w-3 shrink-0" />
            <span>{parentIssue.identifier}</span>
          </span>
        )}
        <span className="flex min-w-0 flex-1 items-center gap-1.5">
          <span className="truncate">{issue.title}</span>
          {childProgress && (
            <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-muted/60 px-1.5 py-0.5">
              <ProgressRing done={childProgress.done} total={childProgress.total} size={14} />
              <span className="text-[11px] text-muted-foreground tabular-nums font-medium">
                {childProgress.done}/{childProgress.total}
              </span>
            </span>
          )}
        </span>
        {issue.labels && issue.labels.length > 0 && (
          <div className="flex items-center gap-1 flex-shrink-0">
            {issue.labels.slice(0, 3).map((l) => (
              <LabelBadge key={l.id} label={l} />
            ))}
            {issue.labels.length > 3 && (
              <span className="text-xs text-muted-foreground">
                +{issue.labels.length - 3}
              </span>
            )}
          </div>
        )}
        {issue.due_date && (
          <span className="shrink-0 text-xs text-muted-foreground">
            {formatDate(issue.due_date)}
          </span>
        )}
        {issue.assignee_type && issue.assignee_id && (
          <ActorAvatar
            actorType={issue.assignee_type}
            actorId={issue.assignee_id}
            size={20}
          />
        )}
      </AppLink>
      <PickerWrapper>
        <ListRowContextMenu issue={issue} onUpdate={handleUpdate} />
      </PickerWrapper>
    </div>
  );
});
