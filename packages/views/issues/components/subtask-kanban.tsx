"use client";

import * as React from "react";
import { useState } from "react";
import { Columns3, List } from "lucide-react";
import { cn } from "@mantica/ui/lib/utils";
import { StatusIcon } from "./status-icon";

interface Issue {
  id: string;
  title: string;
  status: string;
  assigneeId?: string | null;
  assigneeName?: string;
}

interface SubtaskKanbanProps {
  childIssues: Issue[];
  className?: string;
}

const KANBAN_COLUMNS = [
  { id: "todo", title: "To Do", statuses: ["todo", "backlog"] },
  { id: "in_progress", title: "In Progress", statuses: ["in_dev", "in_arch_design", "in_analyze"] },
  { id: "review", title: "Review", statuses: ["ready_review", "in_review"] },
  { id: "testing", title: "Testing", statuses: ["ready_test", "in_test"] },
  { id: "done", title: "Done", statuses: ["done"] },
  { id: "blocked", title: "Blocked", statuses: ["blocked"] },
];

export function SubtaskKanban({
  childIssues,
  className,
}: SubtaskKanbanProps) {
  const [viewMode, setViewMode] = useState<"list" | "board">("board");

  const getColumnIssues = (columnId: string) => {
    const column = KANBAN_COLUMNS.find((c) => c.id === columnId);
    if (!column) return [];
    return childIssues.filter((issue) =>
      column.statuses.includes(issue.status)
    );
  };

  if (childIssues.length === 0) {
    return null;
  }

  return (
    <div className={cn("space-y-4", className)}>
      {/* Header with view toggle */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-medium">Sub-tasks</h3>
          <span className="text-muted-foreground text-sm">
            ({childIssues.length})
          </span>
        </div>

        <div className="flex items-center gap-1">
          <button
            className={cn(
              "rounded p-1.5 transition-colors",
              viewMode === "list" ? "bg-secondary" : "hover:bg-accent"
            )}
            onClick={() => setViewMode("list")}
          >
            <List className="h-4 w-4" />
          </button>
          <button
            className={cn(
              "rounded p-1.5 transition-colors",
              viewMode === "board" ? "bg-secondary" : "hover:bg-accent"
            )}
            onClick={() => setViewMode("board")}
          >
            <Columns3 className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Board View */}
      {viewMode === "board" && (
        <div className="flex gap-4 overflow-x-auto pb-4">
          {KANBAN_COLUMNS.map((column) => {
            const issues = getColumnIssues(column.id);
            return (
              <div
                key={column.id}
                className="bg-muted/30 flex min-w-[250px] flex-col rounded-lg border p-3"
              >
                <div className="mb-3 flex items-center justify-between">
                  <h4 className="text-sm font-medium">{column.title}</h4>
                  <span className="bg-muted text-muted-foreground rounded-full px-2 py-0.5 text-xs">
                    {issues.length}
                  </span>
                </div>
                <div className="flex-1 space-y-2">
                  {issues.map((issue) => (
                    <a
                      key={issue.id}
                      href={`/issues/${issue.id}`}
                      className="bg-background hover:bg-accent/50 block cursor-pointer rounded-md border p-3 transition-colors"
                    >
                      <div className="flex items-start gap-2">
                        <StatusIcon
                          status={issue.status as any}
                          className="mt-0.5 h-3.5 w-3.5 shrink-0"
                        />
                        <div className="min-w-0 flex-1">
                          <p className="line-clamp-2 text-sm">
                            {issue.title}
                          </p>
                          {issue.assigneeName && (
                            <p className="text-muted-foreground mt-1 text-xs">
                              {issue.assigneeName}
                            </p>
                          )}
                        </div>
                      </div>
                    </a>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* List View */}
      {viewMode === "list" && (
        <div className="space-y-2">
          {childIssues.map((issue) => (
            <a
              key={issue.id}
              href={`/issues/${issue.id}`}
              className="hover:bg-accent/50 flex items-center gap-3 rounded-md border p-2"
            >
              <StatusIcon status={issue.status as any} className="h-3.5 w-3.5" />
              <span className="flex-1 text-sm hover:underline">
                {issue.title}
              </span>
              {issue.assigneeName && (
                <span className="text-muted-foreground text-xs">
                  {issue.assigneeName}
                </span>
              )}
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
