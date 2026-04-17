"use client";

import { useMemo, type ReactNode } from "react";
import { EyeOff, GripVertical, MoreHorizontal, Plus } from "lucide-react";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { useDroppable } from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy, useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { Issue, IssueStatus } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@multica/ui/components/ui/dropdown-menu";
import { STATUS_CONFIG } from "@multica/core/issues/config";
import { useModalStore } from "@multica/core/modals";
import { useViewStoreApi } from "@multica/core/issues/stores/view-store-context";
import { StatusIcon } from "./status-icon";
import { DraggableBoardCard } from "./board-card";
import type { ChildProgress } from "./list-row";

export function BoardColumn({
  status,
  issueIds,
  issueMap,
  childProgressMap,
  totalCount,
  footer,
}: {
  status: IssueStatus;
  issueIds: string[];
  issueMap: Map<string, Issue>;
  childProgressMap?: Map<string, ChildProgress>;
  totalCount?: number;
  footer?: ReactNode;
}) {
  const cfg = STATUS_CONFIG[status];
  const { setNodeRef: setDropRef, isOver } = useDroppable({ id: status });
  const viewStoreApi = useViewStoreApi();

  const {
    attributes,
    listeners,
    setNodeRef: setSortableRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: status });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  // Resolve IDs to Issue objects, preserving parent-provided order
  const resolvedIssues = useMemo(
    () =>
      issueIds.flatMap((id) => {
        const issue = issueMap.get(id);
        return issue ? [issue] : [];
      }),
    [issueIds, issueMap],
  );

  return (
    <div
      ref={(node) => {
        setSortableRef(node);
        setDropRef(node);
      }}
      style={style}
      className={`flex w-[280px] shrink-0 flex-col rounded-xl ${cfg.columnBg} p-2 ${isOver ? "ring-2 ring-inset ring-border" : ""} ${isDragging ? "opacity-30" : ""}`}
    >
      <div className="mb-2 flex items-center justify-between px-1.5">
        {/* Left: drag handle + status badge + count */}
        <div className="flex items-center gap-2">
          <button
            {...attributes}
            {...listeners}
            className="cursor-grab touch-none text-muted-foreground hover:text-foreground active:cursor-grabbing"
            aria-label="Drag to reorder column"
            tabIndex={0}
          >
            <GripVertical className="h-3.5 w-3.5" />
          </button>
          <span className={`inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-semibold ${cfg.badgeBg} ${cfg.badgeText}`}>
            <StatusIcon status={status} className="h-3 w-3" inheritColor />
            {cfg.label}
          </span>
          <span className="text-xs text-muted-foreground">
            {totalCount ?? issueIds.length}
          </span>
        </div>

        {/* Right: add + menu */}
        <div className="flex items-center gap-1">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="icon-sm" className="rounded-full text-muted-foreground">
                  <MoreHorizontal className="size-3.5" />
                </Button>
              }
            />
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => viewStoreApi.getState().hideStatus(status)}>
                <EyeOff className="size-3.5" />
                Hide column
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="rounded-full text-muted-foreground"
                  onClick={() => useModalStore.getState().open("create-issue", { status })}
                >
                  <Plus className="size-3.5" />
                </Button>
              }
            />
            <TooltipContent>Add issue</TooltipContent>
          </Tooltip>
        </div>
      </div>
      <div
        className={`min-h-[200px] flex-1 space-y-2 overflow-y-auto rounded-lg p-1`}
      >
        <SortableContext items={issueIds} strategy={verticalListSortingStrategy}>
          {resolvedIssues.map((issue) => (
            <DraggableBoardCard key={issue.id} issue={issue} childProgress={childProgressMap?.get(issue.id)} />
          ))}
        </SortableContext>
        {issueIds.length === 0 && (
          <p className="py-8 text-center text-xs text-muted-foreground">
            No issues
          </p>
        )}
        {footer}
      </div>
    </div>
  );
}
