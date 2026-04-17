import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { swimlaneKeys, swimlaneListOptions } from "./queries";
import { useWorkspaceId } from "../hooks";
import type { CreateSwimlaneRequest, ListSwimlanesResponse } from "../types";

export function useSwimlanes() {
  const wsId = useWorkspaceId();
  return useQuery(swimlaneListOptions(wsId));
}

export function useCreateSwimlane() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateSwimlaneRequest) => api.createSwimlane(data),
    onSuccess: (newSwimlane) => {
      qc.setQueryData<ListSwimlanesResponse>(swimlaneKeys.list(wsId), (old) =>
        old && !old.swimlanes.some((s) => s.id === newSwimlane.id)
          ? { ...old, swimlanes: [...old.swimlanes, newSwimlane], total: old.total + 1 }
          : old,
      );
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: swimlaneKeys.list(wsId) });
    },
  });
}

export function useDeleteSwimlane() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deleteSwimlane(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: swimlaneKeys.list(wsId) });
      const prevList = qc.getQueryData<ListSwimlanesResponse>(swimlaneKeys.list(wsId));
      qc.setQueryData<ListSwimlanesResponse>(swimlaneKeys.list(wsId), (old) =>
        old ? { ...old, swimlanes: old.swimlanes.filter((s) => s.id !== id), total: old.total - 1 } : old,
      );
      return { prevList };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prevList) qc.setQueryData(swimlaneKeys.list(wsId), ctx.prevList);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: swimlaneKeys.list(wsId) });
    },
  });
}
