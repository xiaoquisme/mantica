"use client";

import type { Issue } from "@multica/core/types";
import type { Swimlane } from "@multica/core/types";
import { useModalStore } from "@multica/core/modals";
import { Button } from "@multica/ui/components/ui/button";
import { Plus } from "lucide-react";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { DraggableBoardCard } from "./board-card";
import type { ChildProgress } from "./list-row";

export function SwimlaneColumn({
  swimlane,
  issues,
  childProgressMap,
}: {
  swimlane: Swimlane;
  issues: Issue[];
  childProgressMap?: Map<string, ChildProgress>;
}) {
  return (
    <div className="flex w-full flex-col rounded-xl bg-muted/30 p-3">
      {/* Header */}
      <div className="mb-2 flex items-center justify-between px-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold">{swimlane.name}</span>
          <span className="text-xs text-muted-foreground">{issues.length}</span>
        </div>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                className="rounded-full text-muted-foreground"
                onClick={() =>
                  useModalStore
                    .getState()
                    .open("create-issue", { swimlane_id: swimlane.id })
                }
              >
                <Plus className="size-3.5" />
              </Button>
            }
          />
          <TooltipContent>Add issue to swimlane</TooltipContent>
        </Tooltip>
      </div>

      {/* Issue list */}
      <div className="flex flex-wrap gap-2">
        {issues.map((issue) => (
          <div key={issue.id} className="w-[280px]">
            <DraggableBoardCard
              issue={issue}
              childProgress={childProgressMap?.get(issue.id)}
            />
          </div>
        ))}
        {issues.length === 0 && (
          <p className="w-full py-6 text-center text-xs text-muted-foreground">
            No issues in this swimlane
          </p>
        )}
      </div>
    </div>
  );
}
