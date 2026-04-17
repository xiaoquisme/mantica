"use client";

import { useCallback, memo } from "react";
import { AppLink } from "../../navigation";
import { useSortable, defaultAnimateLayoutChanges } from "@dnd-kit/sortable";
import type { AnimateLayoutChanges } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { toast } from "sonner";
import type { Issue, UpdateIssueRequest } from "@multica/core/types";
import { CalendarDays, MoreHorizontal, UserMinus, Calendar, FolderKanban, Tag, Check } from "lucide-react";
import { ActorAvatar } from "../../common/actor-avatar";
import { useUpdateIssue } from "@multica/core/issues/mutations";
import { PriorityIcon } from "./priority-icon";
import { StatusIcon } from "./status-icon";
import { PriorityPicker, AssigneePicker, DueDatePicker, LabelPicker } from "./pickers";
import { canAssignAgent } from "./pickers";
import { ALL_STATUSES, STATUS_CONFIG, PRIORITY_ORDER, PRIORITY_CONFIG } from "@multica/core/issues/config";
import { useViewStore } from "@multica/core/issues/stores/view-store-context";
import { ProgressRing } from "./progress-ring";
import type { ChildProgress } from "./list-row";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { memberListOptions, agentListOptions } from "@multica/core/workspace/queries";
import { projectListOptions } from "@multica/core/projects/queries";
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

function formatDate(date: string): string {
  return new Date(date).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}

/** Stops event from bubbling to AppLink/drag handlers */
function PickerWrapper({ children }: { children: React.ReactNode }) {
  const stopAndPrevent = (e: React.SyntheticEvent) => {
    e.stopPropagation();
    e.preventDefault();
  };
  const stopOnly = (e: React.SyntheticEvent) => {
    e.stopPropagation();
  };
  return (
    // onClick: preventDefault blocks native <a> navigation when this element is
    // nested inside AppLink (board card). onPointerDown/onMouseDown: stopPropagation
    // only so @base-ui Menu.Trigger can still open on pointerDown.
    <div onClick={stopAndPrevent} onMouseDown={stopOnly} onPointerDown={stopOnly}>
      {children}
    </div>
  );
}

function BoardCardContextMenu({
  issue,
  onUpdate,
}: {
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
            className="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground"
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
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export const BoardCardContent = memo(function BoardCardContent({
  issue,
  editable = false,
  childProgress,
}: {
  issue: Issue;
  editable?: boolean;
  childProgress?: ChildProgress;
}) {
  const storeProperties = useViewStore((s) => s.cardProperties);
  const priorityCfg = PRIORITY_CONFIG[issue.priority];

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

  const showPriority = storeProperties.priority;
  const showDescription = storeProperties.description && issue.description;
  const showAssignee = storeProperties.assignee && issue.assignee_type && issue.assignee_id;
  const showDueDate = storeProperties.dueDate && issue.due_date;

  return (
    <div className="relative rounded-lg border bg-card p-3.5 shadow-[0_1px_2px_0_rgba(0,0,0,0.03)] transition-shadow group-hover:shadow-sm">
      {/* … context menu button — absolute top-right, visible on card hover */}
      {editable && (
        <PickerWrapper>
          <div className="absolute top-1.5 right-1.5">
            <BoardCardContextMenu issue={issue} onUpdate={handleUpdate} />
          </div>
        </PickerWrapper>
      )}

      {/* Row 1: Identifier */}
      <p className="text-xs text-muted-foreground">{issue.identifier}</p>

      {/* Row 2: Title */}
      <p className="mt-1 text-sm font-medium leading-snug line-clamp-2">
        {issue.title}
      </p>

      {/* Sub-issue progress */}
      {childProgress && (
        <div className="mt-1.5 inline-flex items-center gap-1 rounded-full bg-muted/60 px-1.5 py-0.5">
          <ProgressRing done={childProgress.done} total={childProgress.total} size={14} />
          <span className="text-[11px] text-muted-foreground tabular-nums font-medium">
            {childProgress.done}/{childProgress.total}
          </span>
        </div>
      )}

      {/* Description */}
      {showDescription && (
        <p className="mt-1 text-xs text-muted-foreground line-clamp-1">
          {issue.description}
        </p>
      )}

      {/* Row 3: Assignee, priority badge, due date */}
      {(showAssignee || showPriority || showDueDate) && (
        <div className="mt-3 flex items-center gap-2">
          {showAssignee &&
            (editable ? (
              <PickerWrapper>
                <AssigneePicker
                  assigneeType={issue.assignee_type}
                  assigneeId={issue.assignee_id}
                  onUpdate={handleUpdate}
                  trigger={
                    <ActorAvatar
                      actorType={issue.assignee_type!}
                      actorId={issue.assignee_id!}
                      size={22}
                    />
                  }
                />
              </PickerWrapper>
            ) : (
              <ActorAvatar
                actorType={issue.assignee_type!}
                actorId={issue.assignee_id!}
                size={22}
              />
            ))}
          {showPriority &&
            (editable ? (
              <PickerWrapper>
                <PriorityPicker
                  priority={issue.priority}
                  onUpdate={handleUpdate}
                  trigger={
                    <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${priorityCfg.badgeBg} ${priorityCfg.badgeText}`}>
                      <PriorityIcon priority={issue.priority} className="h-3 w-3" inheritColor />
                      {priorityCfg.label}
                    </span>
                  }
                />
              </PickerWrapper>
            ) : (
              <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${priorityCfg.badgeBg} ${priorityCfg.badgeText}`}>
                <PriorityIcon priority={issue.priority} className="h-3 w-3" inheritColor />
                {priorityCfg.label}
              </span>
            ))}
          {showDueDate && (
            <div className="ml-auto">
              {editable ? (
                <PickerWrapper>
                  <DueDatePicker
                    dueDate={issue.due_date}
                    onUpdate={handleUpdate}
                    trigger={
                      <span
                        className={`flex items-center gap-1 text-xs ${
                          new Date(issue.due_date!) < new Date()
                            ? "text-destructive"
                            : "text-muted-foreground"
                        }`}
                      >
                        <CalendarDays className="size-3" />
                        {formatDate(issue.due_date!)}
                      </span>
                    }
                  />
                </PickerWrapper>
              ) : (
                <span
                  className={`flex items-center gap-1 text-xs ${
                    new Date(issue.due_date!) < new Date()
                      ? "text-destructive"
                      : "text-muted-foreground"
                  }`}
                >
                  <CalendarDays className="size-3" />
                  {formatDate(issue.due_date!)}
                </span>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
});

const animateLayoutChanges: AnimateLayoutChanges = (args) => {
  const { isSorting, wasDragging } = args;
  if (isSorting || wasDragging) return false;
  return defaultAnimateLayoutChanges(args);
};

export const DraggableBoardCard = memo(function DraggableBoardCard({ issue, childProgress }: { issue: Issue; childProgress?: ChildProgress }) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: issue.id,
    data: { status: issue.status },
    animateLayoutChanges,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className={isDragging ? "opacity-30" : ""}
    >
      <AppLink
        href={`/issues/${issue.id}`}
        className={`group block transition-colors ${isDragging ? "pointer-events-none" : ""}`}
      >
        <BoardCardContent issue={issue} editable childProgress={childProgress} />
      </AppLink>
    </div>
  );
});
