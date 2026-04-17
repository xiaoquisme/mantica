import type { Label } from "@multica/core/types";

export function LabelBadge({ label }: { label: Label }) {
  return (
    <span
      className="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium text-white"
      style={{ backgroundColor: label.color }}
    >
      {label.name}
    </span>
  );
}
