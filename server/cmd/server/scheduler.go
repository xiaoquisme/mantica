package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const schedulerInterval = 30 * time.Second

// runScheduler periodically checks for due scheduled tasks and enqueues
// agent tasks for them. Follows the same pattern as runRuntimeSweeper.
func runScheduler(ctx context.Context, queries *db.Queries, taskService *service.TaskService) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processDueScheduledTasks(ctx, queries, taskService)
		}
	}
}

// processDueScheduledTasks finds all enabled scheduled tasks whose next_run_at
// has passed, enqueues an agent task for each, and advances next_run_at.
func processDueScheduledTasks(ctx context.Context, queries *db.Queries, taskService *service.TaskService) {
	dueTasks, err := queries.ListDueScheduledTasks(ctx)
	if err != nil {
		slog.Warn("scheduler: failed to list due tasks", "error", err)
		return
	}

	if len(dueTasks) == 0 {
		return
	}

	slog.Info("scheduler: processing due scheduled tasks", "count", len(dueTasks))

	for _, st := range dueTasks {
		processOneScheduledTask(ctx, queries, taskService, st)
	}
}

func processOneScheduledTask(ctx context.Context, queries *db.Queries, taskService *service.TaskService, st db.ScheduledTask) {
	stID := util.UUIDToString(st.ID)

	// Enqueue the agent task.
	_, err := taskService.EnqueueScheduledTask(ctx, st)
	if err != nil {
		slog.Warn("scheduler: failed to enqueue task",
			"scheduled_task_id", stID,
			"name", st.Name,
			"error", err,
		)
		// Still advance next_run_at so we don't retry every 30s.
	}

	// Compute next run time.
	nextRun, cronErr := handler.ComputeNextRunFrom(st.Schedule, time.Now())
	var nextRunAt pgtype.Timestamptz
	if cronErr != nil {
		slog.Error("scheduler: invalid cron expression, disabling task",
			"scheduled_task_id", stID,
			"schedule", st.Schedule,
			"error", cronErr,
		)
		// Set next_run_at to NULL so it won't fire again until fixed.
		nextRunAt = pgtype.Timestamptz{Valid: false}
	} else {
		nextRunAt = pgtype.Timestamptz{Time: nextRun, Valid: true}
	}

	// Mark the run and advance.
	if _, err := queries.MarkScheduledTaskRun(ctx, db.MarkScheduledTaskRunParams{
		ID:        st.ID,
		NextRunAt: nextRunAt,
	}); err != nil {
		slog.Warn("scheduler: failed to update last/next run",
			"scheduled_task_id", stID,
			"error", err,
		)
	}
}
