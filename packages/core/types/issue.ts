export type IssueStatus =
  | "backlog"
  | "classifying"
  | "ready_analyze"
  | "in_analyze"
  | "ready_arch_design"
  | "in_arch_design"
  | "ready_dev"
  | "in_dev"
  | "ready_review"
  | "in_review"
  | "ready_test"
  | "in_test"
  | "done"
  | "blocked"
  | "cancelled";

export type IssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export type IssueAssigneeType = "member" | "agent";

export interface IssueReaction {
  id: string;
  issue_id: string;
  actor_type: string;
  actor_id: string;
  emoji: string;
  created_at: string;
}

export interface Issue {
  id: string;
  workspace_id: string;
  number: number;
  identifier: string;
  title: string;
  description: string | null;
  status: IssueStatus;
  priority: IssuePriority;
  assignee_type: IssueAssigneeType | null;
  assignee_id: string | null;
  creator_type: IssueAssigneeType;
  creator_id: string;
  parent_issue_id: string | null;
  project_id: string | null;
  position: number;
  due_date: string | null;
  reactions?: IssueReaction[];
  created_at: string;
  updated_at: string;
}
