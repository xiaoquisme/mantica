"use client";

import { useState } from "react";
import { Unlink } from "lucide-react";
import type { UpdateIssueRequest } from "@multica/core/types";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueListOptions } from "@multica/core/issues/queries";
import { StatusIcon } from "../status-icon";
import {
  PropertyPicker,
  PickerItem,
  PickerEmpty,
} from "./property-picker";

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

  const query = filter.toLowerCase();
  const filtered = issues.filter(
    (i) =>
      i.id !== currentIssueId &&
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
