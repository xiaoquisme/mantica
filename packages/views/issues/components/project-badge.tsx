"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@mantica/core/hooks";
import { projectListOptions } from "@mantica/core/projects/queries";
import { FolderKanban } from "lucide-react";

export function ProjectBadge({ projectId }: { projectId: string | null }) {
  const wsId = useWorkspaceId();
  const { data: projects = [] } = useQuery(projectListOptions(wsId));

  if (!projectId) return null;

  const project = projects.find((p) => p.id === projectId);
  if (!project) return null;

  return (
    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
      <FolderKanban className="h-3 w-3 shrink-0 text-muted-foreground" />
      <span className="truncate max-w-[80px]" title={project.title}>
        {project.title}
      </span>
    </span>
  );
}
