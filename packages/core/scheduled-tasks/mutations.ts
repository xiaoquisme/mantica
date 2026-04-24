import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type { CreateScheduledTaskRequest, UpdateScheduledTaskRequest } from "../types";
import { scheduledTaskKeys } from "./queries";

export function useCreateScheduledTask(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateScheduledTaskRequest) =>
      api.createScheduledTask(data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: scheduledTaskKeys.all(wsId) });
    },
  });
}

export function useUpdateScheduledTask(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateScheduledTaskRequest }) =>
      api.updateScheduledTask(id, data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: scheduledTaskKeys.all(wsId) });
    },
  });
}

export function useDeleteScheduledTask(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteScheduledTask(id),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: scheduledTaskKeys.all(wsId) });
    },
  });
}

export function useRunScheduledTaskNow(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.runScheduledTaskNow(id),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: scheduledTaskKeys.all(wsId) });
    },
  });
}
