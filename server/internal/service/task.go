package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xiaoquisme/mantica/server/internal/events"
	"github.com/xiaoquisme/mantica/server/internal/mention"
	"github.com/xiaoquisme/mantica/server/internal/pipeline"
	"github.com/xiaoquisme/mantica/server/internal/realtime"
	"github.com/xiaoquisme/mantica/server/internal/util"
	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
	"github.com/xiaoquisme/mantica/server/pkg/protocol"
	"github.com/xiaoquisme/mantica/server/pkg/redact"
)

// errorSummaryMaxLen caps the redacted error summary embedded in the
// auto-revert system comment to keep it readable.
const errorSummaryMaxLen = 200

type TaskService struct {
	Queries  *db.Queries
	Hub      *realtime.Hub
	Bus      *events.Bus
	Analyzer *Analyzer
	Scorer   *Scorer
	Evolver  *Evolver
}

func NewTaskService(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *TaskService {
	logger := slog.Default()
	return &TaskService{
		Queries:  q,
		Hub:      hub,
		Bus:      bus,
		Analyzer: NewAnalyzer(q, logger),
		Scorer:   NewScorer(q, logger),
		Evolver:  NewEvolver(q, logger),
	}
}

// EnqueueTaskForIssue creates a queued task for an agent-assigned issue.
// No context snapshot is stored — the agent fetches all data it needs at
// runtime via the mantica CLI.
func (s *TaskService) EnqueueTaskForIssue(ctx context.Context, issue db.Issue, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	if !issue.AssigneeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "issue has no assignee")
		return db.AgentTaskQueue{}, fmt.Errorf("issue has no assignee")
	}

	agent, err := s.Queries.GetAgent(ctx, issue.AssigneeID)
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agent.ID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "agent has no runtime")
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	var commentID pgtype.UUID
	if len(triggerCommentID) > 0 {
		commentID = triggerCommentID[0]
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          issue.AssigneeID,
		RuntimeID:        agent.RuntimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		TriggerCommentID: commentID,
	})
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(issue.AssigneeID))

	// Publish task:queued event for activity tracking
	workspaceID := s.resolveTaskWorkspaceID(ctx, task)
	if workspaceID != "" {
		s.Bus.Publish(events.Event{
			Type:        protocol.EventTaskQueued,
			WorkspaceID: workspaceID,
			ActorType:   "system",
			ActorID:     "",
			Payload: map[string]any{
				"task_id":  util.UUIDToString(task.ID),
				"agent_id": util.UUIDToString(issue.AssigneeID),
				"issue_id": util.UUIDToString(issue.ID),
				"status":   task.Status,
			},
		})
	}

	return task, nil
}

// EnqueueScheduledTask creates a queued task triggered by a scheduled task.
// The task has no issue_id — the agent works from its own instructions.
func (s *TaskService) EnqueueScheduledTask(ctx context.Context, st db.ScheduledTask) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, st.AgentID)
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	task, err := s.Queries.CreateScheduledAgentTask(ctx, db.CreateScheduledAgentTaskParams{
		AgentID:         st.AgentID,
		RuntimeID:       agent.RuntimeID,
		Priority:        1, // low priority for scheduled tasks
		ScheduledTaskID: st.ID,
	})
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("create scheduled task: %w", err)
	}

	slog.Info("scheduled task enqueued",
		"task_id", util.UUIDToString(task.ID),
		"scheduled_task_id", util.UUIDToString(st.ID),
		"agent_id", util.UUIDToString(st.AgentID),
	)

	workspaceID := util.UUIDToString(st.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskQueued,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":           util.UUIDToString(task.ID),
			"agent_id":         util.UUIDToString(st.AgentID),
			"scheduled_task_id": util.UUIDToString(st.ID),
			"status":           task.Status,
		},
	})

	return task, nil
}

// EnqueueTaskForMention creates a queued task for a mentioned agent on an issue.
// Unlike EnqueueTaskForIssue, this takes an explicit agent ID rather than
// deriving it from the issue assignee.
func (s *TaskService) EnqueueTaskForMention(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("mention task enqueue failed: agent not found", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("mention task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("mention task enqueue failed: agent has no runtime", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          agentID,
		RuntimeID:        agent.RuntimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		TriggerCommentID: triggerCommentID,
	})
	if err != nil {
		slog.Error("mention task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("mention task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))

	// Publish task:queued event for activity tracking
	workspaceID := s.resolveTaskWorkspaceID(ctx, task)
	if workspaceID != "" {
		s.Bus.Publish(events.Event{
			Type:        protocol.EventTaskQueued,
			WorkspaceID: workspaceID,
			ActorType:   "system",
			ActorID:     "",
			Payload: map[string]any{
				"task_id":  util.UUIDToString(task.ID),
				"agent_id": util.UUIDToString(agentID),
				"issue_id": util.UUIDToString(issue.ID),
				"status":   task.Status,
			},
		})
	}

	return task, nil
}

// EnqueueChatTask creates a queued task for a chat session.
// Unlike issue tasks, chat tasks have no issue_id.
func (s *TaskService) EnqueueChatTask(ctx context.Context, chatSession db.ChatSession) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, chatSession.AgentID)
	if err != nil {
		slog.Error("chat task enqueue failed", "chat_session_id", util.UUIDToString(chatSession.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	task, err := s.Queries.CreateChatTask(ctx, db.CreateChatTaskParams{
		AgentID:       chatSession.AgentID,
		RuntimeID:     agent.RuntimeID,
		Priority:      2, // medium priority for chat
		ChatSessionID: chatSession.ID,
	})
	if err != nil {
		slog.Error("chat task enqueue failed", "chat_session_id", util.UUIDToString(chatSession.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create chat task: %w", err)
	}

	slog.Info("chat task enqueued", "task_id", util.UUIDToString(task.ID), "chat_session_id", util.UUIDToString(chatSession.ID), "agent_id", util.UUIDToString(chatSession.AgentID))
	return task, nil
}

// CancelTasksForIssue cancels all active tasks for an issue and broadcasts
// task:cancelled for each so frontends can clear live cards immediately.
func (s *TaskService) CancelTasksForIssue(ctx context.Context, issueID pgtype.UUID) error {
	tasks, err := s.Queries.CancelAgentTasksByIssue(ctx, issueID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		slog.Info("task cancelled (batch)", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
		s.ReconcileAgentStatus(ctx, task.AgentID)
		s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)
	}
	return nil
}

// CancelTask cancels a single task by ID. It broadcasts a task:cancelled event
// so frontends can update immediately.
func (s *TaskService) CancelTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.CancelAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("cancel task: %w", err)
	}

	slog.Info("task cancelled", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast cancellation as a task:failed event so frontends clear the live card
	s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)

	// Post-task analysis for cancelled tasks (async, non-blocking)
	// Cancelled tasks count as failures in the scoring system.
	if s.Analyzer != nil {
		go func() {
			bgCtx := context.Background()
			if _, err := s.Analyzer.AnalyzeAndSave(bgCtx, task.ID); err != nil {
				slog.Warn("post-task analysis failed (cancelled)", "task_id", util.UUIDToString(task.ID), "error", err)
			}
			if s.Scorer != nil && task.AgentID.Valid {
				wsID := s.resolveTaskWorkspaceID(bgCtx, task)
				if wsID != "" {
					wsUUID := util.ParseUUID(wsID)
					if err := s.Scorer.ScoreTask(bgCtx, task.ID, task.AgentID, wsUUID); err != nil {
						slog.Warn("agent scoring failed (cancelled)", "task_id", util.UUIDToString(task.ID), "error", err)
					}
				}
			}
		}()
	}

	return &task, nil
}

// ClaimTask atomically claims the next queued task for an agent,
// respecting max_concurrent_tasks.
func (s *TaskService) ClaimTask(ctx context.Context, agentID pgtype.UUID) (*db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("count running tasks: %w", err)
	}
	if running >= int64(agent.MaxConcurrentTasks) {
		slog.Debug("task claim: no capacity", "agent_id", util.UUIDToString(agentID), "running", running, "max", agent.MaxConcurrentTasks)
		return nil, nil // No capacity
	}

	task, err := s.Queries.ClaimAgentTask(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("task claim: no tasks available", "agent_id", util.UUIDToString(agentID))
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("claim task: %w", err)
	}

	slog.Info("task claimed", "task_id", util.UUIDToString(task.ID), "agent_id", util.UUIDToString(agentID))

	// Update agent status to working
	s.updateAgentStatus(ctx, agentID, "working")

	// Broadcast task:dispatch
	s.broadcastTaskDispatch(ctx, task)

	return &task, nil
}

// ClaimTaskForRuntime claims the next runnable task for a runtime while
// still respecting each agent's max_concurrent_tasks limit.
func (s *TaskService) ClaimTaskForRuntime(ctx context.Context, runtimeID pgtype.UUID) (*db.AgentTaskQueue, error) {
	tasks, err := s.Queries.ListPendingTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("list pending tasks: %w", err)
	}

	triedAgents := map[string]struct{}{}
	for _, candidate := range tasks {
		agentKey := util.UUIDToString(candidate.AgentID)
		if _, seen := triedAgents[agentKey]; seen {
			continue
		}
		triedAgents[agentKey] = struct{}{}

		task, err := s.ClaimTask(ctx, candidate.AgentID)
		if err != nil {
			return nil, err
		}
		if task != nil && task.RuntimeID == runtimeID {
			return task, nil
		}
	}

	return nil, nil
}

// StartTask transitions a dispatched task to running.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) StartTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.StartAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}

	slog.Info("task started", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	return &task, nil
}

// CompleteTask marks a task as completed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) CompleteTask(ctx context.Context, taskID pgtype.UUID, result []byte, sessionID, workDir string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.CompleteAgentTask(ctx, db.CompleteAgentTaskParams{
		ID:        taskID,
		Result:    result,
		SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
		WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
	})
	if err != nil {
		// Log the current task state to help debug why the update matched no rows.
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			slog.Warn("complete task failed: task not in running or cancelled state",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("complete task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("complete task: %w", err)
	}

	slog.Info("task completed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	// Post agent output as a comment, but only for issue tasks with assignment triggers.
	// Comment-triggered tasks: the agent replies via CLI with --parent, so
	// posting here would create a duplicate.
	// Chat tasks: no comment posting needed.
	if task.IssueID.Valid && !task.TriggerCommentID.Valid {
		var payload protocol.TaskCompletedPayload
		if err := json.Unmarshal(result, &payload); err == nil {
			if payload.Output != "" {
				s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(payload.Output), "comment", task.TriggerCommentID)
			}
		}
	}

	// For chat tasks, save assistant reply, update session, and broadcast chat:done.
	if task.ChatSessionID.Valid {
		var payload protocol.TaskCompletedPayload
		if err := json.Unmarshal(result, &payload); err == nil && payload.Output != "" {
			if _, err := s.Queries.CreateChatMessage(ctx, db.CreateChatMessageParams{
				ChatSessionID: task.ChatSessionID,
				Role:          "assistant",
				Content:       redact.Text(payload.Output),
				TaskID:        task.ID,
			}); err != nil {
				slog.Error("failed to save assistant chat message", "task_id", util.UUIDToString(task.ID), "error", err)
			}
		}
		s.Queries.UpdateChatSessionSession(ctx, db.UpdateChatSessionSessionParams{
			ID:        task.ChatSessionID,
			SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
			WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
		})
		s.broadcastChatDone(ctx, task)
	}

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, task)

	// Post-task analysis (async, non-blocking)
	if s.Analyzer != nil {
		go func() {
			bgCtx := context.Background()
			if _, err := s.Analyzer.AnalyzeAndSave(bgCtx, task.ID); err != nil {
				slog.Warn("post-task analysis failed", "task_id", util.UUIDToString(task.ID), "error", err)
			}
			// Update agent score after analysis
			if s.Scorer != nil && task.AgentID.Valid {
				wsID := s.resolveTaskWorkspaceID(bgCtx, task)
				if wsID != "" {
					wsUUID := util.ParseUUID(wsID)
					if err := s.Scorer.ScoreTask(bgCtx, task.ID, task.AgentID, wsUUID); err != nil {
						slog.Warn("agent scoring failed", "task_id", util.UUIDToString(task.ID), "error", err)
					}
					// Auto-evolve: extract skills or generate improvement hints
					if s.Evolver != nil {
						if result, err := s.Evolver.AnalyzeAndEvolve(bgCtx, task.ID, task.AgentID, wsUUID); err != nil {
							slog.Warn("auto-evolution failed", "task_id", util.UUIDToString(task.ID), "error", err)
						} else if result.SkillExtracted {
							slog.Info("skill auto-extracted", "task_id", util.UUIDToString(task.ID), "skill", result.SkillName)
						} else if result.ImprovementHint != "" {
							slog.Warn("improvement hint generated", "task_id", util.UUIDToString(task.ID), "hint", result.ImprovementHint)
						}
					}
				}
			}
		}()
	}

	return &task, nil
}

// FailTask marks a task as failed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) FailTask(ctx context.Context, taskID pgtype.UUID, errMsg string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.FailAgentTask(ctx, db.FailAgentTaskParams{
		ID:    taskID,
		Error: pgtype.Text{String: errMsg, Valid: true},
	})
	if err != nil {
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			slog.Warn("fail task failed: task not in dispatched/running state",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("fail task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("fail task: %w", err)
	}

	slog.Warn("task failed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID), "error", errMsg)

	if errMsg != "" && task.IssueID.Valid {
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(errMsg), "system", task.TriggerCommentID)
	}
	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Auto-revert issue status if the agent crashed mid-stage so the pipeline
	// can re-dispatch. CAS-guarded — no-op if the issue already moved on.
	s.AutoRevertIssueStatusOnFailure(ctx, task)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task)

	// Post-task analysis for failed tasks (async, non-blocking)
	if s.Analyzer != nil {
		go func() {
			bgCtx := context.Background()
			if _, err := s.Analyzer.AnalyzeAndSave(bgCtx, task.ID); err != nil {
				slog.Warn("post-task analysis failed", "task_id", util.UUIDToString(task.ID), "error", err)
			}
			// Update agent score for failed tasks too
			if s.Scorer != nil && task.AgentID.Valid {
				wsID := s.resolveTaskWorkspaceID(bgCtx, task)
				if wsID != "" {
					wsUUID := util.ParseUUID(wsID)
					if err := s.Scorer.ScoreTask(bgCtx, task.ID, task.AgentID, wsUUID); err != nil {
						slog.Warn("agent scoring failed", "task_id", util.UUIDToString(task.ID), "error", err)
					}
				}
			}
		}()
	}

	return &task, nil
}

// TriggerPipeline advances an issue at a ready_* status to the matching in_*
// status, assigns the corresponding pipeline agent, and enqueues a task. This
// is the same logic the HTTP handler uses on a status change to ready_*; it
// also runs on auto-revert so a failed run is automatically re-dispatched.
//
// Returns the updated issue when an advance happened, or nil when no advance
// occurred (issue not at a ready_* status, no agent for the stage, agent has
// no runtime, or the underlying update failed).
func (s *TaskService) TriggerPipeline(ctx context.Context, issue db.Issue) *db.Issue {
	stage, ok := pipeline.Stages[issue.Status]
	if !ok {
		return nil
	}

	agents, err := s.Queries.ListAgents(ctx, issue.WorkspaceID)
	if err != nil {
		slog.Warn("pipeline: failed to list agents", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return nil
	}

	var targetAgent *db.Agent
	for i := range agents {
		if agents[i].Name == stage.AgentName && !agents[i].ArchivedAt.Valid {
			targetAgent = &agents[i]
			break
		}
	}

	if targetAgent == nil {
		slog.Warn("pipeline: no agent found for stage", "issue_id", util.UUIDToString(issue.ID), "status", issue.Status, "agent_name", stage.AgentName)
		return nil
	}

	if !targetAgent.RuntimeID.Valid {
		slog.Warn("pipeline: agent has no runtime", "agent_name", stage.AgentName)
		return nil
	}

	// Cancel any existing tasks for this issue.
	s.CancelTasksForIssue(ctx, issue.ID)

	// Advance status to in_* and assign the agent. Preserve nullable fields
	// (parent, due_date, project) that are directly set by sqlc.narg —
	// passing Go zero values would NULL them out.
	updatedIssue, err := s.Queries.UpdateIssue(ctx, db.UpdateIssueParams{
		ID:            issue.ID,
		Status:        pgtype.Text{String: stage.InProgressStatus, Valid: true},
		AssigneeType:  pgtype.Text{String: "agent", Valid: true},
		AssigneeID:    targetAgent.ID,
		DueDate:       issue.DueDate,
		ParentIssueID: issue.ParentIssueID,
		ProjectID:     issue.ProjectID,
	})
	if err != nil {
		slog.Warn("pipeline: failed to update issue status/assignee", "error", err)
		return nil
	}

	// Broadcast the issue update so frontends see the new status/assignee.
	prefix := s.getIssuePrefix(issue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     issueToMap(updatedIssue, prefix),
	})

	// Enqueue task for the new agent — skip if one is already active.
	hasActive, err := s.Queries.HasActiveTaskForIssueAndAgent(ctx, db.HasActiveTaskForIssueAndAgentParams{
		IssueID: issue.ID,
		AgentID: targetAgent.ID,
	})
	if err == nil && hasActive {
		slog.Debug("pipeline: skipping enqueue, active task already exists",
			"issue_id", util.UUIDToString(issue.ID), "agent", stage.AgentName)
	} else {
		s.EnqueueTaskForIssue(ctx, updatedIssue)
	}

	slog.Info("pipeline: auto-assigned and advanced",
		"issue_id", util.UUIDToString(issue.ID),
		"agent", stage.AgentName,
		"status", stage.InProgressStatus,
	)

	return &updatedIssue
}

// AutoRevertIssueStatusOnFailure reverts an issue's status from in_* back to
// the matching ready_* when the agent task for that stage failed. Posts a
// system comment recording the run id and error, then re-triggers the
// pipeline so the next agent run is dispatched automatically.
//
// CAS-safe (no-op when the issue status no longer matches the expected
// in_*), idempotent against duplicate failure callbacks for the same issue,
// and skipped entirely for chat tasks (no issue) or unrecognized statuses.
//
// The Classifier stage is special-cased: in_status="classifying" reverts to
// "backlog" AND the Classifier assignee is cleared so the user can re-trigger
// the Classifier by reassigning. Re-dispatch via TriggerPipeline only happens
// for stages whose ready_* maps back into pipeline.Stages.
func (s *TaskService) AutoRevertIssueStatusOnFailure(ctx context.Context, task db.AgentTaskQueue) {
	if !task.IssueID.Valid {
		return
	}

	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		slog.Debug("auto-revert: issue lookup failed", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}

	readyStatus, ok := pipeline.RevertStatusFor(issue.Status)
	if !ok {
		// Issue is not in an in_* status — agent already advanced it (e.g. to
		// ready_review) before crashing, or it was never in a stage status.
		return
	}

	expectedStatus := issue.Status
	isClassifier := expectedStatus == pipeline.ClassifierStage.InProgressStatus

	revertedIssue, err := s.Queries.RevertIssueStatusIfMatching(ctx, db.RevertIssueStatusIfMatchingParams{
		ID:             issue.ID,
		NewStatus:      readyStatus,
		ExpectedStatus: expectedStatus,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// CAS lost: another caller already reverted, or the agent moved
			// status forward before crashing. Either way, do nothing.
			return
		}
		slog.Warn("auto-revert: status revert failed",
			"task_id", util.UUIDToString(task.ID),
			"issue_id", util.UUIDToString(issue.ID),
			"error", err,
		)
		return
	}

	// For the Classifier stage there is no ready_classifying — clear the
	// assignee so the user can re-trigger by reassigning the Classifier agent.
	if isClassifier {
		cleared, err := s.Queries.ClearIssueAssigneeIfStatus(ctx, db.ClearIssueAssigneeIfStatusParams{
			ID:             revertedIssue.ID,
			ExpectedStatus: readyStatus,
		})
		if err == nil {
			revertedIssue = cleared
		} else if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("auto-revert: failed to clear classifier assignee",
				"issue_id", util.UUIDToString(issue.ID),
				"error", err,
			)
		}
	}

	// Build the system comment summarising the failure cause and revert.
	runID := util.UUIDToString(task.ID)
	if len(runID) >= 8 {
		runID = runID[:8]
	}
	errSummary := redact.Text(task.Error.String)
	if errSummary == "" {
		errSummary = "no error message"
	}
	if len(errSummary) > errorSummaryMaxLen {
		errSummary = errSummary[:errorSummaryMaxLen] + "…"
	}
	content := fmt.Sprintf("Run `%s` failed (`%s`); status auto-reverted to `%s` for re-dispatch.", runID, errSummary, readyStatus)
	s.createAgentComment(ctx, revertedIssue.ID, task.AgentID, content, "system", task.TriggerCommentID)

	slog.Info("auto-revert: status reverted",
		"task_id", util.UUIDToString(task.ID),
		"issue_id", util.UUIDToString(issue.ID),
		"prev_status", expectedStatus,
		"new_status", readyStatus,
	)

	// Broadcast the issue update so the frontend reflects the reverted state.
	prefix := s.getIssuePrefix(revertedIssue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(revertedIssue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"issue":          issueToMap(revertedIssue, prefix),
			"status_changed": true,
			"prev_status":    expectedStatus,
		},
	})

	// Re-dispatch via the existing pipeline pickup. Classifier reverts to
	// "backlog" with no assignee — there's no auto-pickup for backlog, so
	// the user must reassign manually. Other stages re-trigger automatically.
	if pipeline.IsReadyStatus(revertedIssue.Status) {
		s.TriggerPipeline(ctx, revertedIssue)
	}
}

// ReportProgress broadcasts a progress update via the event bus.
func (s *TaskService) ReportProgress(ctx context.Context, taskID string, workspaceID string, summary string, step, total int) {
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskProgress,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskProgressPayload{
			TaskID:  taskID,
			Summary: summary,
			Step:    step,
			Total:   total,
		},
	})
}

// RequeueOrphanedTasks requeues tasks that were stuck in dispatched/running or
// marked as failed by the sweeper when a runtime went offline. Called when a daemon
// reconnects to recover in-progress work.
func (s *TaskService) RequeueOrphanedTasks(ctx context.Context, runtimeID pgtype.UUID) (int, error) {
	// Requeue tasks stuck in dispatched/running
	orphanedTasks, err := s.Queries.RequeueOrphanedTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return 0, fmt.Errorf("requeue orphaned tasks: %w", err)
	}

	// Requeue tasks failed by sweeper with "runtime went offline"
	sweeperFailedTasks, err := s.Queries.RequeueSweeperFailedTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return 0, fmt.Errorf("requeue sweeper-failed tasks: %w", err)
	}

	allRequeued := append(orphanedTasks, sweeperFailedTasks...)

	for _, task := range allRequeued {
		slog.Info("task requeued after runtime reconnect",
			"task_id", util.UUIDToString(task.ID),
			"runtime_id", util.UUIDToString(task.RuntimeID),
			"issue_id", util.UUIDToString(task.IssueID),
			"agent_id", util.UUIDToString(task.AgentID),
		)

		// Reconcile agent status
		s.ReconcileAgentStatus(ctx, task.AgentID)

		// Delete old task messages from previous failed attempt
		s.Queries.DeleteTaskMessages(ctx, task.ID)

		// Publish task:queued event
		workspaceID := s.resolveTaskWorkspaceID(ctx, task)
		if workspaceID != "" {
			s.Bus.Publish(events.Event{
				Type:        protocol.EventTaskQueued,
				WorkspaceID: workspaceID,
				ActorType:   "system",
				ActorID:     "",
				Payload: map[string]any{
					"task_id":  util.UUIDToString(task.ID),
					"agent_id": util.UUIDToString(task.AgentID),
					"issue_id": util.UUIDToString(task.IssueID),
					"status":   task.Status,
				},
			})
		}
	}

	return len(allRequeued), nil
}

// ReconcileAgentStatus checks running task count and sets agent status accordingly.
func (s *TaskService) ReconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return
	}
	newStatus := "idle"
	if running > 0 {
		newStatus = "working"
	}
	slog.Debug("agent status reconciled", "agent_id", util.UUIDToString(agentID), "status", newStatus, "running_tasks", running)
	s.updateAgentStatus(ctx, agentID, newStatus)
}

func (s *TaskService) updateAgentStatus(ctx context.Context, agentID pgtype.UUID, status string) {
	agent, err := s.Queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
		ID:     agentID,
		Status: status,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentStatus,
		WorkspaceID: util.UUIDToString(agent.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"agent": agentToMap(agent)},
	})
}

// LoadAgentSkills loads an agent's skills with their files for task execution.
func (s *TaskService) LoadAgentSkills(ctx context.Context, agentID pgtype.UUID) []AgentSkillData {
	skills, err := s.Queries.ListAgentSkills(ctx, agentID)
	if err != nil || len(skills) == 0 {
		return nil
	}

	result := make([]AgentSkillData, 0, len(skills))
	for _, sk := range skills {
		data := AgentSkillData{Name: sk.Name, Content: sk.Content}
		files, _ := s.Queries.ListSkillFiles(ctx, sk.ID)
		for _, f := range files {
			data.Files = append(data.Files, AgentSkillFileData{Path: f.Path, Content: f.Content})
		}
		result = append(result, data)
	}
	return result
}

// AgentSkillData represents a skill for task execution responses.
type AgentSkillData struct {
	Name    string               `json:"name"`
	Content string               `json:"content"`
	Files   []AgentSkillFileData `json:"files,omitempty"`
}

// AgentSkillFileData represents a supporting file within a skill.
type AgentSkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func priorityToInt(p string) int32 {
	switch p {
	case "urgent":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func (s *TaskService) broadcastTaskDispatch(ctx context.Context, task db.AgentTaskQueue) {
	var payload map[string]any
	if task.Context != nil {
		json.Unmarshal(task.Context, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["task_id"] = util.UUIDToString(task.ID)
	payload["runtime_id"] = util.UUIDToString(task.RuntimeID)

	workspaceID := s.resolveTaskWorkspaceID(ctx, task)
	if workspaceID == "" {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDispatch,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

func (s *TaskService) broadcastTaskEvent(ctx context.Context, eventType string, task db.AgentTaskQueue) {
	workspaceID := s.resolveTaskWorkspaceID(ctx, task)
	if workspaceID == "" {
		return
	}
	payload := map[string]any{
		"task_id":  util.UUIDToString(task.ID),
		"agent_id": util.UUIDToString(task.AgentID),
		"issue_id": util.UUIDToString(task.IssueID),
		"status":   task.Status,
	}
	if task.ChatSessionID.Valid {
		payload["chat_session_id"] = util.UUIDToString(task.ChatSessionID)
	}
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

// resolveTaskWorkspaceID determines the workspace ID for a task.
// For issue tasks, it comes from the issue. For chat tasks, from the chat session.
func (s *TaskService) resolveTaskWorkspaceID(ctx context.Context, task db.AgentTaskQueue) string {
	if task.IssueID.Valid {
		if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
			return util.UUIDToString(issue.WorkspaceID)
		}
	}
	if task.ChatSessionID.Valid {
		if cs, err := s.Queries.GetChatSession(ctx, task.ChatSessionID); err == nil {
			return util.UUIDToString(cs.WorkspaceID)
		}
	}
	if task.ScheduledTaskID.Valid {
		if st, err := s.Queries.GetScheduledTask(ctx, task.ScheduledTaskID); err == nil {
			return util.UUIDToString(st.WorkspaceID)
		}
	}
	return ""
}

func (s *TaskService) broadcastChatDone(ctx context.Context, task db.AgentTaskQueue) {
	workspaceID := s.resolveTaskWorkspaceID(ctx, task)
	if workspaceID == "" {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventChatDone,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.ChatDonePayload{
			ChatSessionID: util.UUIDToString(task.ChatSessionID),
			TaskID:        util.UUIDToString(task.ID),
		},
	})
}

func (s *TaskService) broadcastIssueUpdated(issue db.Issue) {
	prefix := s.getIssuePrefix(issue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"issue": issueToMap(issue, prefix)},
	})
}

func (s *TaskService) getIssuePrefix(workspaceID pgtype.UUID) string {
	ws, err := s.Queries.GetWorkspace(context.Background(), workspaceID)
	if err != nil {
		return ""
	}
	return ws.IssuePrefix
}

func (s *TaskService) createAgentComment(ctx context.Context, issueID, agentID pgtype.UUID, content, commentType string, parentID pgtype.UUID) {
	if content == "" {
		return
	}
	// Look up issue to get workspace ID for mention expansion and broadcasting.
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return
	}
	// Expand bare issue identifiers (e.g. MUL-117) into mention links.
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)
	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issueID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "agent",
		AuthorID:    agentID,
		Content:     content,
		Type:        commentType,
		ParentID:    parentID,
	})
	if err != nil {
		slog.Warn("createAgentComment: insert failed",
			"issue_id", util.UUIDToString(issueID),
			"agent_id", util.UUIDToString(agentID),
			"error", err,
		)
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(agentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
}

func issueToMap(issue db.Issue, issuePrefix string) map[string]any {
	return map[string]any{
		"id":              util.UUIDToString(issue.ID),
		"workspace_id":    util.UUIDToString(issue.WorkspaceID),
		"number":          issue.Number,
		"identifier":      issuePrefix + "-" + strconv.Itoa(int(issue.Number)),
		"title":           issue.Title,
		"description":     util.TextToPtr(issue.Description),
		"status":          issue.Status,
		"priority":        issue.Priority,
		"assignee_type":   util.TextToPtr(issue.AssigneeType),
		"assignee_id":     util.UUIDToPtr(issue.AssigneeID),
		"creator_type":    issue.CreatorType,
		"creator_id":      util.UUIDToString(issue.CreatorID),
		"parent_issue_id": util.UUIDToPtr(issue.ParentIssueID),
		"project_id":      util.UUIDToPtr(issue.ProjectID),
		"position":        issue.Position,
		"due_date":        util.TimestampToPtr(issue.DueDate),
		"created_at":      util.TimestampToString(issue.CreatedAt),
		"updated_at":      util.TimestampToString(issue.UpdatedAt),
	}
}

// agentToMap builds a simple map for broadcasting agent status updates.
func agentToMap(a db.Agent) map[string]any {
	var rc any
	if a.RuntimeConfig != nil {
		json.Unmarshal(a.RuntimeConfig, &rc)
	}
	return map[string]any{
		"id":                   util.UUIDToString(a.ID),
		"workspace_id":         util.UUIDToString(a.WorkspaceID),
		"runtime_id":           util.UUIDToString(a.RuntimeID),
		"name":                 a.Name,
		"description":          a.Description,
		"avatar_url":           util.TextToPtr(a.AvatarUrl),
		"runtime_mode":         a.RuntimeMode,
		"runtime_config":       rc,
		"visibility":           a.Visibility,
		"status":               a.Status,
		"max_concurrent_tasks": a.MaxConcurrentTasks,
		"owner_id":             util.UUIDToPtr(a.OwnerID),
		"skills":               []any{},
		"created_at":           util.TimestampToString(a.CreatedAt),
		"updated_at":           util.TimestampToString(a.UpdatedAt),
		"archived_at":          util.TimestampToPtr(a.ArchivedAt),
		"archived_by":          util.UUIDToPtr(a.ArchivedBy),
	}
}
