"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useCreateSwimlane } from "@multica/core/swimlanes";
import { toast } from "sonner";

export function CreateSwimlaneDialog({ onClose }: { onClose: () => void }) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const createSwimlane = useCreateSwimlane();

  const handleSubmit = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Swimlane name is required.");
      return;
    }
    if (trimmed.length > 255) {
      setError("Name must be 255 characters or fewer.");
      return;
    }
    setError("");
    try {
      await createSwimlane.mutateAsync({ name: trimmed });
      toast.success("Swimlane created");
      onClose();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create swimlane");
    }
  };

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>New Swimlane</DialogTitle>
          <DialogDescription>
            Add a custom swimlane to organize work on the board.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div>
            <Label className="text-xs text-muted-foreground">Name</Label>
            <Input
              autoFocus
              type="text"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                if (error) setError("");
              }}
              placeholder="e.g. Backend, Frontend, Design"
              className="mt-1"
              onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
              maxLength={255}
            />
            {error && (
              <p className="mt-1 text-sm text-destructive">{error}</p>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={createSwimlane.isPending}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={createSwimlane.isPending || !name.trim()}
          >
            {createSwimlane.isPending ? "Creating…" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
