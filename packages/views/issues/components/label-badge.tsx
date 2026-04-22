import type { Label } from "@multica/core/types";

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
