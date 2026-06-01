import type { IssueStatus } from "../../types";

export const STATUS_ORDER: IssueStatus[] = [
  "backlog",
  "todo",
  "doing",
  "done",
  "blocked",
  "cancelled",
];

export const ALL_STATUSES: IssueStatus[] = [
  "backlog",
  "todo",
  "doing",
  "done",
  "blocked",
  "cancelled",
];

/** Statuses shown as board columns (excludes cancelled). */
export const BOARD_STATUSES: IssueStatus[] = [
  "backlog",
  "todo",
  "doing",
  "done",
  "blocked",
];

export const STATUS_CONFIG: Record<
  IssueStatus,
  {
    label: string;
    iconColor: string;
    hoverBg: string;
    dividerColor: string;
    badgeBg: string;
    badgeText: string;
    columnBg: string;
  }
> = {
  backlog: {
    label: "Backlog",
    iconColor: "text-muted-foreground",
    hoverBg: "hover:bg-accent",
    dividerColor: "bg-muted-foreground/40",
    badgeBg: "bg-muted",
    badgeText: "text-muted-foreground",
    columnBg: "bg-muted/40",
  },
  todo: {
    label: "Todo",
    iconColor: "text-yellow-500",
    hoverBg: "hover:bg-yellow-500/10",
    dividerColor: "bg-yellow-500",
    badgeBg: "bg-yellow-500/20",
    badgeText: "text-yellow-700 dark:text-yellow-300",
    columnBg: "bg-yellow-500/5",
  },
  doing: {
    label: "Doing",
    iconColor: "text-blue-600",
    hoverBg: "hover:bg-blue-600/10",
    dividerColor: "bg-blue-600",
    badgeBg: "bg-blue-600",
    badgeText: "text-white",
    columnBg: "bg-blue-600/5",
  },
  done: {
    label: "Done",
    iconColor: "text-info",
    hoverBg: "hover:bg-info/10",
    dividerColor: "bg-info",
    badgeBg: "bg-info",
    badgeText: "text-white",
    columnBg: "bg-info/5",
  },
  blocked: {
    label: "Blocked",
    iconColor: "text-destructive",
    hoverBg: "hover:bg-destructive/10",
    dividerColor: "bg-destructive",
    badgeBg: "bg-destructive",
    badgeText: "text-white",
    columnBg: "bg-destructive/5",
  },
  cancelled: {
    label: "Cancelled",
    iconColor: "text-muted-foreground",
    hoverBg: "hover:bg-accent",
    dividerColor: "bg-muted-foreground/40",
    badgeBg: "bg-muted",
    badgeText: "text-muted-foreground",
    columnBg: "bg-muted/40",
  },
};
