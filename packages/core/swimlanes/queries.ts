import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const swimlaneKeys = {
  all: (wsId: string) => ["swimlanes", wsId] as const,
  list: (wsId: string) => [...swimlaneKeys.all(wsId), "list"] as const,
  detail: (wsId: string, id: string) =>
    [...swimlaneKeys.all(wsId), "detail", id] as const,
};

export function swimlaneListOptions(wsId: string) {
  return queryOptions({
    queryKey: swimlaneKeys.list(wsId),
    queryFn: () => api.listSwimlanes(),
    select: (data) => data.swimlanes,
  });
}
