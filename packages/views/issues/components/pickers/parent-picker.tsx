"use client";

import { useState } from "react";
import { Unlink, Check } from "lucide-react";
import type { UpdateIssueRequest, Issue } from "@multica/core/types";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueListOptions } from "@multica/core/issues/queries";
import { StatusIcon } from "../status-icon";
import {
  PropertyPicker,
  PickerItem,
  PickerEmpty,
} from "./property-picker";

export function getDescendantIds(issues: Issue[], rootId: string): Set<string> {
  const result = new Set<string>();
  const queue = [rootId];
  while (queue.length > 0) {
    const current = queue.pop()!;
    for (const issue of issues) {
      if (issue.parent_issue_id === current) {
        result.add(issue.id);
        queue.push(issue.id);
      }
    }
  }
  return result;
}

export function ParentSubMenuContent({
  parentIssueId,
  currentIssueId,
  onUpdate,
}: {
  parentIssueId: string | null;
  currentIssueId: string;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
}) {
  const [filter, setFilter] = useState("");
  const wsId = useWorkspaceId();
  const { data: issues = [] } = useQuery(issueListOptions(wsId));

  const descendantIds = getDescendantIds(issues, currentIssueId);
  const query = filter.toLowerCase();
  const filtered = issues.filter(
    (i) =>
      i.id !== currentIssueId &&
      !descendantIds.has(i.id) &&
      (i.title.toLowerCase().includes(query) ||
        i.identifier.toLowerCase().includes(query)),
  );

  return (
    <div className="w-64">
      <div className="px-2 py-1.5 border-b">
        <input
          type="text"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Search issues..."
          aria-label="Search issues"
          className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
          autoFocus
        />
      </div>
      <div className="p-1 max-h-60 overflow-y-auto">
        {filtered.map((issue) => (
          <button
            key={issue.id}
            type="button"
            onClick={() => onUpdate({ parent_issue_id: issue.id })}
            className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
          >
            <StatusIcon status={issue.status} className="h-3.5 w-3.5 shrink-0" />
            <span className="text-muted-foreground shrink-0">{issue.identifier}</span>
            <span className="truncate">{issue.title}</span>
            {issue.id === parentIssueId && (
              <Check className="ml-auto h-3.5 w-3.5 text-muted-foreground shrink-0" />
            )}
          </button>
        ))}
        {filtered.length === 0 && filter && (
          <div className="px-2 py-3 text-center text-sm text-muted-foreground">
            No results
          </div>
        )}
        {issues.length === 0 && !filter && (
          <div className="px-2 py-3 text-center text-sm text-muted-foreground">
            No issues yet
          </div>
        )}
      </div>
    </div>
  );
}

export function ParentPicker({
  parentIssueId,
  currentIssueId,
  onUpdate,
}: {
  parentIssueId: string | null;
  currentIssueId: string;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const wsId = useWorkspaceId();
  const { data: issues = [] } = useQuery(issueListOptions(wsId));

  const descendantIds = getDescendantIds(issues, currentIssueId);
  const query = filter.toLowerCase();
  const filtered = issues.filter(
    (i) =>
      i.id !== currentIssueId &&
      !descendantIds.has(i.id) &&
      (i.title.toLowerCase().includes(query) ||
        i.identifier.toLowerCase().includes(query)),
  );

  const parentIssue = issues.find((i) => i.id === parentIssueId) ?? null;

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) setFilter("");
      }}
      width="w-64"
      align="start"
      searchable
      searchPlaceholder="Search issues..."
      onSearchChange={setFilter}
      trigger={
        parentIssue ? (
          <>
            <StatusIcon status={parentIssue.status} className="h-3.5 w-3.5 shrink-0" />
            <span className="text-muted-foreground shrink-0">{parentIssue.identifier}</span>
            <span className="truncate">{parentIssue.title}</span>
          </>
        ) : (
          <span className="text-muted-foreground">No parent</span>
        )
      }
    >
      <PickerItem
        selected={!parentIssueId}
        onClick={() => {
          onUpdate({ parent_issue_id: null });
          setOpen(false);
        }}
      >
        <Unlink className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-muted-foreground">No parent</span>
      </PickerItem>

      {filtered.map((issue) => (
        <PickerItem
          key={issue.id}
          selected={issue.id === parentIssueId}
          onClick={() => {
            onUpdate({ parent_issue_id: issue.id });
            setOpen(false);
          }}
        >
          <StatusIcon status={issue.status} className="h-3.5 w-3.5 shrink-0" />
          <span className="text-muted-foreground shrink-0">{issue.identifier}</span>
          <span className="truncate">{issue.title}</span>
        </PickerItem>
      ))}

      {filtered.length === 0 && filter && <PickerEmpty />}
    </PropertyPicker>
  );
}
