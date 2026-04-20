"use client";

import { useState } from "react";
import { Tag, Plus, Check } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { labelListOptions } from "@multica/core/issues/queries";
import { useCreateLabel } from "@multica/core/issues/mutations";
import type { Label } from "@multica/core/types";
import { PropertyPicker, PickerItem, PickerEmpty } from "./property-picker";

const LABEL_COLORS = [
  "#ef4444",
  "#f97316",
  "#eab308",
  "#22c55e",
  "#06b6d4",
  "#3b82f6",
  "#8b5cf6",
  "#ec4899",
  "#6b7280",
  "#0f172a",
];

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

function CreateLabelForm({
  onCancel,
  onCreated,
}: {
  onCancel: () => void;
  onCreated: (label: Label) => void;
}) {
  const [name, setName] = useState("");
  const [color, setColor] = useState(LABEL_COLORS[0] ?? "#ef4444");
  const createLabel = useCreateLabel();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    createLabel.mutate(
      { name: name.trim(), color },
      {
        onSuccess: (label) => {
          onCreated(label);
        },
      },
    );
  };

  return (
    <form onSubmit={handleSubmit} className="p-2 flex flex-col gap-2">
      <input
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Label name"
        autoFocus
        className="w-full bg-transparent text-sm border rounded px-2 py-1 outline-none focus:ring-1 focus:ring-ring"
      />
      <div className="flex flex-wrap gap-1">
        {LABEL_COLORS.map((c) => (
          <button
            key={c}
            type="button"
            onClick={() => setColor(c)}
            className="h-5 w-5 rounded-full flex items-center justify-center shrink-0"
            style={{ backgroundColor: c }}
          >
            {color === c && <Check className="h-3 w-3 text-white" />}
          </button>
        ))}
      </div>
      <div className="flex gap-1 justify-end">
        <button
          type="button"
          onClick={onCancel}
          className="text-xs px-2 py-1 rounded hover:bg-accent transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={!name.trim() || createLabel.isPending}
          className="text-xs px-2 py-1 rounded bg-primary text-primary-foreground disabled:opacity-50 hover:bg-primary/90 transition-colors"
        >
          {createLabel.isPending ? "Creating..." : "Create"}
        </button>
      </div>
    </form>
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
  const [showCreate, setShowCreate] = useState(false);
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

  const handleCreated = (label: Label) => {
    setShowCreate(false);
    onUpdate([...Array.from(selectedIds), label.id]);
  };

  const footer = showCreate ? (
    <CreateLabelForm onCancel={() => setShowCreate(false)} onCreated={handleCreated} />
  ) : (
    <button
      type="button"
      onClick={() => setShowCreate(true)}
      className="flex w-full items-center gap-1.5 px-3 py-2 text-xs text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
    >
      <Plus className="h-3 w-3" />
      Create label
    </button>
  );

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) {
          setFilter("");
          setShowCreate(false);
        }
      }}
      width="w-52"
      align={align}
      searchable
      searchPlaceholder="Filter labels..."
      onSearchChange={setFilter}
      footer={footer}
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
        <p className="px-3 py-2 text-xs text-muted-foreground">No labels yet</p>
      )}
    </PropertyPicker>
  );
}
