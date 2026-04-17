"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { labelListOptions } from "@multica/core/issues/queries";
import { useUpdateIssueLabels } from "@multica/core/issues/mutations";
import type { Label } from "@multica/core/types";
import { Check } from "lucide-react";

export function LabelPicker({
  issueId,
  currentLabels = [],
}: {
  issueId: string;
  currentLabels?: Label[];
}) {
  const wsId = useWorkspaceId();
  const { data: allLabels = [] } = useQuery(labelListOptions(wsId));
  const updateLabels = useUpdateIssueLabels();
  const [filter, setFilter] = useState("");

  const selectedIds = new Set(currentLabels.map((l) => l.id));

  const filtered = allLabels.filter((l) =>
    l.name.toLowerCase().includes(filter.toLowerCase()),
  );

  const toggle = (label: Label) => {
    const next = new Set(selectedIds);
    if (next.has(label.id)) {
      next.delete(label.id);
    } else {
      next.add(label.id);
    }
    updateLabels.mutate({ issueId, labelIds: Array.from(next) });
  };

  return (
    <div className="w-52">
      <div className="px-2 py-1.5 border-b">
        <input
          type="text"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter labels..."
          aria-label="Filter labels"
          className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
          autoFocus
        />
      </div>
      <div className="p-1 max-h-60 overflow-y-auto">
        {filtered.map((label) => (
          <button
            key={label.id}
            type="button"
            onClick={() => toggle(label)}
            className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
          >
            <span
              className="h-2.5 w-2.5 rounded-full shrink-0"
              style={{ backgroundColor: label.color }}
            />
            <span className="flex-1 truncate text-left">{label.name}</span>
            {selectedIds.has(label.id) && <Check className="h-3.5 w-3.5 text-muted-foreground" />}
          </button>
        ))}
        {filtered.length === 0 && filter && (
          <div className="px-2 py-3 text-center text-sm text-muted-foreground">
            No results
          </div>
        )}
        {allLabels.length === 0 && (
          <div className="px-2 py-3 text-center text-sm text-muted-foreground">
            No labels yet
          </div>
        )}
      </div>
    </div>
  );
}
