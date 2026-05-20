"use client";

import { useEffect } from "react";
import { useNavigationStore } from "@mantica/core/navigation";
import { useAuthStore } from "@mantica/core/auth";
import { useWorkspaceStore } from "@mantica/core/workspace";
import { useNavigation } from "../navigation";

export function useDashboardGuard(loginPath = "/") {
  const { pathname, push } = useNavigation();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);

  useEffect(() => {
    if (!isLoading && !user) push(loginPath);
  }, [user, isLoading, push, loginPath]);

  useEffect(() => {
    useNavigationStore.getState().onPathChange(pathname);
  }, [pathname]);

  return { user, isLoading, workspace };
}
