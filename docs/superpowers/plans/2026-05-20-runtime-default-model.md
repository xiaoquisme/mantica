# Runtime default_model Field Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional `default_model` TEXT field to `agent_runtime` that can be set via daemon registration or a new PATCH API, and displayed/edited in the runtime detail UI.

**Architecture:** Add a nullable `default_model` column to the DB, thread it through the sqlc layer (regenerated), the daemon registration and a new PATCH handler, and expose it as an editable field in the frontend runtime detail panel.

**Tech Stack:** Go 1.26, PostgreSQL/pgx, sqlc, Chi router, Next.js (App Router), TanStack Query, Zustand, Tailwind/shadcn

---

## File Map

| File | Action |
|---|---|
| `server/migrations/044_runtime_default_model.up.sql` | Create — add column |
| `server/migrations/044_runtime_default_model.down.sql` | Create — drop column |
| `server/pkg/db/queries/runtime.sql` | Modify — add param to UpsertAgentRuntime; add UpdateAgentRuntimeDefaultModel |
| `server/pkg/db/generated/` | Auto-regenerated (`make sqlc`) — do not hand-edit |
| `server/internal/handler/daemon.go` | Modify — add DefaultModel to runtime struct + upsert params |
| `server/internal/handler/runtime.go` | Modify — add DefaultModel to response; add PatchAgentRuntime handler |
| `server/cmd/server/router.go` | Modify — register PATCH /{runtimeId} |
| `packages/core/types/agent.ts` | Modify — add `default_model: string \| null` to RuntimeDevice |
| `packages/core/api/client.ts` | Modify — add `updateRuntime` method |
| `packages/core/runtimes/mutations.ts` | Modify — add `useUpdateRuntime` hook |
| `packages/views/runtimes/components/runtime-detail.tsx` | Modify — add editable default_model field |

---

### Task 1: Database Migration

**Files:**
- Create: `server/migrations/044_runtime_default_model.up.sql`
- Create: `server/migrations/044_runtime_default_model.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- server/migrations/044_runtime_default_model.up.sql
ALTER TABLE agent_runtime ADD COLUMN default_model TEXT;
```

- [ ] **Step 2: Write the down migration**

```sql
-- server/migrations/044_runtime_default_model.down.sql
ALTER TABLE agent_runtime DROP COLUMN default_model;
```

- [ ] **Step 3: Run the migration**

```bash
make migrate-up
```

Expected: `goose: successfully migrated database to version: 44`

- [ ] **Step 4: Commit**

```bash
git add server/migrations/044_runtime_default_model.up.sql server/migrations/044_runtime_default_model.down.sql
git commit -m "feat(db): add default_model column to agent_runtime"
```

---

### Task 2: Update sqlc Queries

**Files:**
- Modify: `server/pkg/db/queries/runtime.sql`

- [ ] **Step 1: Update UpsertAgentRuntime to include default_model**

Replace the existing `UpsertAgentRuntime` query (lines 14–37 of `runtime.sql`) with:

```sql
-- name: UpsertAgentRuntime :one
INSERT INTO agent_runtime (
    workspace_id,
    daemon_id,
    name,
    runtime_mode,
    provider,
    status,
    device_info,
    metadata,
    owner_id,
    default_model,
    last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
ON CONFLICT (workspace_id, daemon_id, provider)
DO UPDATE SET
    name = EXCLUDED.name,
    runtime_mode = EXCLUDED.runtime_mode,
    status = EXCLUDED.status,
    device_info = EXCLUDED.device_info,
    metadata = EXCLUDED.metadata,
    owner_id = COALESCE(EXCLUDED.owner_id, agent_runtime.owner_id),
    default_model = COALESCE(EXCLUDED.default_model, agent_runtime.default_model),
    last_seen_at = now(),
    updated_at = now()
RETURNING *;
```

- [ ] **Step 2: Add UpdateAgentRuntimeDefaultModel query**

Append after the `DeleteArchivedAgentsByRuntime` query at the bottom of `runtime.sql`:

```sql
-- name: UpdateAgentRuntimeDefaultModel :one
UPDATE agent_runtime
SET default_model = $2, updated_at = now()
WHERE id = $1
RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc code**

```bash
make sqlc
```

Expected: no errors, `server/pkg/db/generated/runtime.sql.go` and `models.go` are updated.

Verify the generated code contains:
- `AgentRuntime` struct now has `DefaultModel pgtype.Text`
- `UpsertAgentRuntimeParams` now has `DefaultModel pgtype.Text` as the 10th parameter
- New function `UpdateAgentRuntimeDefaultModel` and struct `UpdateAgentRuntimeDefaultModelParams` exist

- [ ] **Step 4: Commit**

```bash
git add server/pkg/db/queries/runtime.sql server/pkg/db/generated/
git commit -m "feat(db): add default_model to runtime sqlc queries"
```

---

### Task 3: Update Daemon Registration Handler

**Files:**
- Modify: `server/internal/handler/daemon.go`

- [ ] **Step 1: Add DefaultModel to the runtime struct in DaemonRegisterRequest**

In `daemon.go`, find the `DaemonRegisterRequest` struct (lines 23–34). Replace the `Runtimes` field inline struct:

```go
// before:
Runtimes []struct {
    Name    string `json:"name"`
    Type    string `json:"type"`
    Version string `json:"version"`
    Status  string `json:"status"`
} `json:"runtimes"`

// after:
Runtimes []struct {
    Name         string  `json:"name"`
    Type         string  `json:"type"`
    Version      string  `json:"version"`
    Status       string  `json:"status"`
    DefaultModel *string `json:"default_model,omitempty"`
} `json:"runtimes"`
```

- [ ] **Step 2: Pass DefaultModel to UpsertAgentRuntimeParams in DaemonRegister**

In `DaemonRegister`, find the `h.Queries.UpsertAgentRuntime(...)` call (around line 101). Add `DefaultModel` to the params:

```go
registered, err := h.Queries.UpsertAgentRuntime(r.Context(), db.UpsertAgentRuntimeParams{
    WorkspaceID:  parseUUID(req.WorkspaceID),
    DaemonID:     strToText(req.DaemonID),
    Name:         name,
    RuntimeMode:  "local",
    Provider:     provider,
    Status:       status,
    DeviceInfo:   deviceInfo,
    Metadata:     metadata,
    OwnerID:      member.UserID,
    DefaultModel: ptrToText(runtime.DefaultModel),
})
```

- [ ] **Step 3: Verify the Go code compiles**

```bash
cd server && go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/daemon.go
git commit -m "feat(handler): pass default_model through daemon registration"
```

---

### Task 4: Update Runtime Response and Add PATCH Handler

**Files:**
- Modify: `server/internal/handler/runtime.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: Add DefaultModel to AgentRuntimeResponse**

In `runtime.go`, add `DefaultModel` field to the response struct (after `OwnerID`):

```go
type AgentRuntimeResponse struct {
    ID           string  `json:"id"`
    WorkspaceID  string  `json:"workspace_id"`
    DaemonID     *string `json:"daemon_id"`
    Name         string  `json:"name"`
    RuntimeMode  string  `json:"runtime_mode"`
    Provider     string  `json:"provider"`
    Status       string  `json:"status"`
    DeviceInfo   string  `json:"device_info"`
    Metadata     any     `json:"metadata"`
    OwnerID      *string `json:"owner_id"`
    DefaultModel *string `json:"default_model"`
    LastSeenAt   *string `json:"last_seen_at"`
    CreatedAt    string  `json:"created_at"`
    UpdatedAt    string  `json:"updated_at"`
}
```

- [ ] **Step 2: Update runtimeToResponse to include DefaultModel**

In `runtimeToResponse`, add `DefaultModel` to the returned struct:

```go
return AgentRuntimeResponse{
    ID:           uuidToString(rt.ID),
    WorkspaceID:  uuidToString(rt.WorkspaceID),
    DaemonID:     textToPtr(rt.DaemonID),
    Name:         rt.Name,
    RuntimeMode:  rt.RuntimeMode,
    Provider:     rt.Provider,
    Status:       rt.Status,
    DeviceInfo:   rt.DeviceInfo,
    Metadata:     metadata,
    OwnerID:      uuidToPtr(rt.OwnerID),
    DefaultModel: textToPtr(rt.DefaultModel),
    LastSeenAt:   timestampToPtr(rt.LastSeenAt),
    CreatedAt:    timestampToString(rt.CreatedAt),
    UpdatedAt:    timestampToString(rt.UpdatedAt),
}
```

- [ ] **Step 3: Add PatchAgentRuntime handler**

Append this function at the end of `runtime.go` (before the closing of the file):

```go
// PatchAgentRuntime updates mutable fields on a runtime (currently: default_model).
func (h *Handler) PatchAgentRuntime(w http.ResponseWriter, r *http.Request) {
    runtimeID := chi.URLParam(r, "runtimeId")

    rt, err := h.Queries.GetAgentRuntime(r.Context(), parseUUID(runtimeID))
    if err != nil {
        writeError(w, http.StatusNotFound, "runtime not found")
        return
    }

    wsID := uuidToString(rt.WorkspaceID)
    member, ok := h.requireWorkspaceMember(w, r, wsID, "runtime not found")
    if !ok {
        return
    }

    userID := uuidToString(member.UserID)
    isAdmin := roleAllowed(member.Role, "owner", "admin")
    isOwner := rt.OwnerID.Valid && uuidToString(rt.OwnerID) == userID
    if !isAdmin && !isOwner {
        writeError(w, http.StatusForbidden, "you can only update your own runtimes")
        return
    }

    var req struct {
        DefaultModel *string `json:"default_model"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    updated, err := h.Queries.UpdateAgentRuntimeDefaultModel(r.Context(), db.UpdateAgentRuntimeDefaultModelParams{
        ID:           rt.ID,
        DefaultModel: ptrToText(req.DefaultModel),
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to update runtime")
        return
    }

    h.publish(protocol.EventDaemonRegister, wsID, "member", userID, map[string]any{
        "action": "update",
    })

    writeJSON(w, http.StatusOK, runtimeToResponse(updated))
}
```

- [ ] **Step 4: Register the PATCH route in router.go**

In `router.go`, find the `/api/runtimes/{runtimeId}` sub-router (around line 277). Add the PATCH route:

```go
r.Route("/{runtimeId}", func(r chi.Router) {
    r.Patch("/", h.PatchAgentRuntime)   // add this line
    r.Get("/usage", h.GetRuntimeUsage)
    r.Get("/activity", h.GetRuntimeTaskActivity)
    r.Post("/ping", h.InitiatePing)
    r.Get("/ping/{pingId}", h.GetPing)
    r.Post("/update", h.InitiateUpdate)
    r.Get("/update/{updateId}", h.GetUpdate)
    r.Delete("/", h.DeleteAgentRuntime)
})
```

- [ ] **Step 5: Verify the Go code compiles**

```bash
cd server && go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add server/internal/handler/runtime.go server/cmd/server/router.go
git commit -m "feat(handler): add PatchAgentRuntime endpoint and default_model to response"
```

---

### Task 5: Frontend — Types, API Client, Mutation Hook

**Files:**
- Modify: `packages/core/types/agent.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/runtimes/mutations.ts`

- [ ] **Step 1: Add default_model to RuntimeDevice type**

In `packages/core/types/agent.ts`, add the field to `RuntimeDevice` after `owner_id`:

```typescript
export interface RuntimeDevice {
  id: string;
  workspace_id: string;
  daemon_id: string | null;
  name: string;
  runtime_mode: AgentRuntimeMode;
  provider: string;
  status: "online" | "offline";
  device_info: string;
  metadata: Record<string, unknown>;
  owner_id: string | null;
  default_model: string | null;
  last_seen_at: string | null;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Add updateRuntime API method**

In `packages/core/api/client.ts`, add after `deleteRuntime`:

```typescript
async updateRuntime(runtimeId: string, data: { default_model: string | null }): Promise<AgentRuntime> {
  return this.fetch(`/api/runtimes/${runtimeId}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}
```

- [ ] **Step 3: Add useUpdateRuntime mutation hook**

In `packages/core/runtimes/mutations.ts`, add after `useDeleteRuntime`:

```typescript
export function useUpdateRuntime(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      runtimeId,
      data,
    }: {
      runtimeId: string;
      data: { default_model: string | null };
    }) => api.updateRuntime(runtimeId, data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: runtimeKeys.all(wsId) });
    },
  });
}
```

- [ ] **Step 4: Run TypeScript typecheck to verify**

```bash
pnpm typecheck
```

Expected: no errors in `packages/core/`.

- [ ] **Step 5: Commit**

```bash
git add packages/core/types/agent.ts packages/core/api/client.ts packages/core/runtimes/mutations.ts
git commit -m "feat(core): add default_model to RuntimeDevice type, API client, and mutation hook"
```

---

### Task 6: Frontend — Runtime Detail UI

**Files:**
- Modify: `packages/views/runtimes/components/runtime-detail.tsx`

- [ ] **Step 1: Add the editable default_model section**

In `runtime-detail.tsx`, add the import and state for the edit UI. The full updated file:

```tsx
"use client";

import { useState } from "react";
import { Trash2, Pencil, Check, X } from "lucide-react";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import type { AgentRuntime } from "@multica/core/types";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useDeleteRuntime, useUpdateRuntime } from "@multica/core/runtimes/mutations";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { ActorAvatar } from "../../common/actor-avatar";
import { formatLastSeen } from "../utils";
import { StatusBadge, InfoField } from "./shared";
import { ProviderLogo } from "./provider-logo";
import { PingSection } from "./ping-section";
import { UpdateSection } from "./update-section";
import { UsageSection } from "./usage-section";

function getCliVersion(metadata: Record<string, unknown>): string | null {
  if (
    metadata &&
    typeof metadata.cli_version === "string" &&
    metadata.cli_version
  ) {
    return metadata.cli_version;
  }
  return null;
}

export function RuntimeDetail({ runtime }: { runtime: AgentRuntime }) {
  const cliVersion =
    runtime.runtime_mode === "local" ? getCliVersion(runtime.metadata) : null;

  const user = useAuthStore((s) => s.user);
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const deleteMutation = useDeleteRuntime(wsId);
  const updateMutation = useUpdateRuntime(wsId);

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [editingModel, setEditingModel] = useState(false);
  const [modelDraft, setModelDraft] = useState(runtime.default_model ?? "");

  // Resolve owner info
  const ownerMember = runtime.owner_id
    ? members.find((m) => m.user_id === runtime.owner_id) ?? null
    : null;

  // Permission check for delete/edit
  const currentMember = user
    ? members.find((m) => m.user_id === user.id)
    : null;
  const isAdmin = currentMember
    ? currentMember.role === "owner" || currentMember.role === "admin"
    : false;
  const isRuntimeOwner = user && runtime.owner_id === user.id;
  const canDelete = isAdmin || isRuntimeOwner;
  const canEdit = isAdmin || isRuntimeOwner;

  const handleDelete = () => {
    deleteMutation.mutate(runtime.id, {
      onSuccess: () => {
        toast.success("Runtime deleted");
        setDeleteOpen(false);
      },
      onError: (e) => {
        toast.error(e instanceof Error ? e.message : "Failed to delete runtime");
      },
    });
  };

  const handleSaveModel = () => {
    const value = modelDraft.trim() || null;
    updateMutation.mutate(
      { runtimeId: runtime.id, data: { default_model: value } },
      {
        onSuccess: () => {
          toast.success("Default model updated");
          setEditingModel(false);
        },
        onError: (e) => {
          toast.error(
            e instanceof Error ? e.message : "Failed to update default model",
          );
        },
      },
    );
  };

  const handleCancelModel = () => {
    setModelDraft(runtime.default_model ?? "");
    setEditingModel(false);
  };

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <div className="flex h-7 w-7 shrink-0 items-center justify-center">
            <ProviderLogo provider={runtime.provider} className="h-5 w-5" />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-semibold truncate">{runtime.name}</h2>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <StatusBadge status={runtime.status} />
          {canDelete && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* Info grid */}
        <div className="grid grid-cols-2 gap-4">
          <InfoField label="Runtime Mode" value={runtime.runtime_mode} />
          <InfoField label="Provider" value={runtime.provider} />
          <InfoField label="Status" value={runtime.status} />
          <InfoField
            label="Last Seen"
            value={formatLastSeen(runtime.last_seen_at)}
          />
          {ownerMember && (
            <div>
              <div className="text-xs text-muted-foreground mb-1">Owner</div>
              <div className="flex items-center gap-2">
                <ActorAvatar
                  actorType="member"
                  actorId={ownerMember.user_id}
                  size={20}
                />
                <span className="text-sm">{ownerMember.name}</span>
              </div>
            </div>
          )}
          {runtime.device_info && (
            <InfoField label="Device" value={runtime.device_info} />
          )}
          {runtime.daemon_id && (
            <InfoField label="Daemon ID" value={runtime.daemon_id} mono />
          )}
        </div>

        {/* Default Model */}
        <div>
          <div className="flex items-center justify-between mb-1">
            <div className="text-xs text-muted-foreground">Default Model</div>
            {canEdit && !editingModel && (
              <Button
                variant="ghost"
                size="icon"
                className="h-5 w-5 text-muted-foreground"
                onClick={() => {
                  setModelDraft(runtime.default_model ?? "");
                  setEditingModel(true);
                }}
              >
                <Pencil className="h-3 w-3" />
              </Button>
            )}
          </div>
          {editingModel ? (
            <div className="flex items-center gap-2">
              <Input
                value={modelDraft}
                onChange={(e) => setModelDraft(e.target.value)}
                placeholder="e.g. claude-sonnet-4-6"
                className="h-7 text-sm"
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleSaveModel();
                  if (e.key === "Escape") handleCancelModel();
                }}
                autoFocus
              />
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 shrink-0"
                onClick={handleSaveModel}
                disabled={updateMutation.isPending}
              >
                <Check className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 shrink-0"
                onClick={handleCancelModel}
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          ) : (
            <div className="mt-0.5 text-sm font-mono text-xs truncate text-foreground">
              {runtime.default_model ?? (
                <span className="text-muted-foreground italic">Not set</span>
              )}
            </div>
          )}
        </div>

        {/* CLI Version & Update */}
        {runtime.runtime_mode === "local" && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              CLI Version
            </h3>
            <UpdateSection
              runtimeId={runtime.id}
              currentVersion={cliVersion}
              isOnline={runtime.status === "online"}
            />
          </div>
        )}

        {/* Connection Test */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Connection Test
          </h3>
          <PingSection runtimeId={runtime.id} />
        </div>

        {/* Usage */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Token Usage
          </h3>
          <UsageSection runtimeId={runtime.id} />
        </div>

        {/* Metadata */}
        {runtime.metadata && Object.keys(runtime.metadata).length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-2">
              Metadata
            </h3>
            <div className="rounded-lg border bg-muted/30 p-3">
              <pre className="text-xs font-mono whitespace-pre-wrap break-all">
                {JSON.stringify(runtime.metadata, null, 2)}
              </pre>
            </div>
          </div>
        )}

        {/* Timestamps */}
        <div className="grid grid-cols-2 gap-4 border-t pt-4">
          <InfoField
            label="Created"
            value={new Date(runtime.created_at).toLocaleString()}
          />
          <InfoField
            label="Updated"
            value={new Date(runtime.updated_at).toLocaleString()}
          />
        </div>
      </div>

      {/* Delete confirmation */}
      <AlertDialog open={deleteOpen} onOpenChange={(v) => { if (!v) setDeleteOpen(false); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Runtime</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete &ldquo;{runtime.name}&rdquo;? This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
```

- [ ] **Step 2: Verify Input component is available**

```bash
ls packages/ui/components/ui/input.tsx
```

Expected: file exists. If not, run `pnpm ui:add input` from project root.

- [ ] **Step 3: Run TypeScript typecheck**

```bash
pnpm typecheck
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add packages/views/runtimes/components/runtime-detail.tsx
git commit -m "feat(ui): add editable default_model field to runtime detail"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|---|---|
| DB migration — add default_model TEXT nullable | Task 1 |
| Update sqlc queries — UpsertAgentRuntime with default_model | Task 2 |
| Update daemon registration handler — handle default_model | Task 3 |
| PATCH /api/runtimes/{id} to update default_model | Task 4 |
| Frontend — display and edit default_model | Task 6 |
| TypeScript types, API client, mutation hook | Task 5 |

All requirements are covered.

### Type Consistency

- `db.UpsertAgentRuntimeParams.DefaultModel` → `pgtype.Text` (set in Task 2, used in Task 3) ✓
- `db.UpdateAgentRuntimeDefaultModelParams` → `{ID pgtype.UUID, DefaultModel pgtype.Text}` (set in Task 2, used in Task 4) ✓
- `AgentRuntimeResponse.DefaultModel` → `*string` (set in Task 4, sourced from `textToPtr(rt.DefaultModel)`) ✓
- `RuntimeDevice.default_model` → `string | null` (set in Task 5, used in Task 6) ✓
- `useUpdateRuntime` mutationFn parameter type matches call site in Task 6 ✓

### Potential Issue: sqlc parameter ordering

When sqlc regenerates `UpsertAgentRuntimeParams`, the `DefaultModel` field will be the 10th positional parameter (`$10`). The `UpsertAgentRuntimeParams` struct field order follows the INSERT column order. Verify the generated struct in `server/pkg/db/generated/runtime.sql.go` after running `make sqlc` to confirm the struct field name matches `DefaultModel` before proceeding to Task 3.
