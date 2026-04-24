import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const scheduledTaskKeys = {
  all: (wsId: string) => ["scheduled-tasks", wsId] as const,
  list: (wsId: string) => [...scheduledTaskKeys.all(wsId), "list"] as const,
};

export function scheduledTaskListOptions(wsId: string) {
  return queryOptions({
    queryKey: scheduledTaskKeys.list(wsId),
    queryFn: () => api.listScheduledTasks(),
  });
}
