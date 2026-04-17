"use client";

import { useState } from "react";
import { Tag } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { labelListOptions } from "@multica/core/issues/queries";
import type { Label } from "@multica/core/types";
import { PropertyPicker, PickerItem, PickerEmpty } from "./property-picker";

export function LabelBadge({ label, size = "sm" }: { label: Label; size?: "sm" | "xs" }) {
  const sizeClass = size === "xs" ? "text-[10px] px-1 py-0" : "text-xs px-1.5 py-0.5";
  return (
    <span
      className={`inline-flex items-center rounded font-medium ${sizeClass}`}
      style={{
        backgroundColor: label.color + "33",
        color: label.color,
        border: `1px solid ${label.color}55`,
      }}
    >
      {label.name}
    </span>
  );
}

export function LabelsPicker({
  labels,
  onUpdate,
  align,
}: {
  labels: Label[];
  onUpdate: (labelIds: string[]) => void;
  align?: "start" | "center" | "end";
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const wsId = useWorkspaceId();
  const { data: allLabels = [] } = useQuery(labelListOptions(wsId));

  const selectedIds = new Set(labels.map((l) => l.id));
  const query = filter.toLowerCase();
  const filtered = allLabels.filter((l) => l.name.toLowerCase().includes(query));

  const toggle = (label: Label) => {
    const next = new Set(selectedIds);
    if (next.has(label.id)) {
      next.delete(label.id);
    } else {
      next.add(label.id);
    }
    onUpdate(Array.from(next));
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) setFilter("");
      }}
      width="w-48"
      align={align}
      searchable
      searchPlaceholder="Filter labels..."
      onSearchChange={setFilter}
      trigger={
        labels.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {labels.map((l) => (
              <LabelBadge key={l.id} label={l} size="xs" />
            ))}
          </div>
        ) : (
          <span className="text-muted-foreground flex items-center gap-1">
            <Tag className="h-3 w-3" />
            No labels
          </span>
        )
      }
    >
      {filtered.map((label) => (
        <PickerItem
          key={label.id}
          selected={selectedIds.has(label.id)}
          onClick={() => toggle(label)}
        >
          <span
            className="h-2.5 w-2.5 rounded-full shrink-0"
            style={{ backgroundColor: label.color }}
          />
          <span className="truncate">{label.name}</span>
        </PickerItem>
      ))}
      {filtered.length === 0 && filter && <PickerEmpty />}
      {allLabels.length === 0 && !filter && (
        <p className="px-3 py-2 text-xs text-muted-foreground">No labels configured</p>
      )}
    </PropertyPicker>
  );
}
