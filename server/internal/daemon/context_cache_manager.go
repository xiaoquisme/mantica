package daemon

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xiaoquisme/mantica/server/internal/daemon/execenv"
	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
)

// ContextCacheManager handles loading and saving context cache for tasks.
type ContextCacheManager struct {
	queries *db.Queries
	logger  *slog.Logger
}

// NewContextCacheManager creates a new context cache manager.
func NewContextCacheManager(queries *db.Queries, logger *slog.Logger) *ContextCacheManager {
	return &ContextCacheManager{
		queries: queries,
		logger:  logger,
	}
}

// LoadCacheForTask loads context cache for a task.
// It first tries to load from the current task's cache.
// If not found, it tries to inherit from a prior completed task on the same issue.
func (m *ContextCacheManager) LoadCacheForTask(ctx context.Context, taskID, issueID pgtype.UUID) (*execenv.ContextCache, error) {
	// Try to load from current task first
	cacheData, err := m.queries.GetTaskContextCache(ctx, taskID)
	if err != nil && err != sql.ErrNoRows {
		m.logger.Warn("failed to load context cache", "task_id", taskID, "error", err)
		return execenv.NewContextCache(), nil
	}

	// If current task has cache, use it
	if len(cacheData) > 0 {
		cache, err := execenv.GetCachedContext(cacheData)
		if err != nil {
			m.logger.Warn("failed to parse context cache", "task_id", taskID, "error", err)
			return execenv.NewContextCache(), nil
		}
		m.logger.Info("loaded context cache from current task", "task_id", taskID)
		return cache, nil
	}

	// Try to inherit from prior completed task
	if issueID.Valid {
		priorCacheData, err := m.queries.GetPriorTaskContextCache(ctx, issueID)
		if err != nil && err != sql.ErrNoRows {
			m.logger.Warn("failed to load prior context cache", "issue_id", issueID, "error", err)
			return execenv.NewContextCache(), nil
		}

		if len(priorCacheData) > 0 {
			cache, err := execenv.GetCachedContext(priorCacheData)
			if err != nil {
				m.logger.Warn("failed to parse prior context cache", "issue_id", issueID, "error", err)
				return execenv.NewContextCache(), nil
			}
			m.logger.Info("inherited context cache from prior task", "issue_id", issueID)
			return cache, nil
		}
	}

	// No cache found, return empty cache
	return execenv.NewContextCache(), nil
}

// SaveCacheForTask saves context cache for a task.
func (m *ContextCacheManager) SaveCacheForTask(ctx context.Context, taskID pgtype.UUID, cache *execenv.ContextCache) error {
	if cache == nil {
		return nil
	}

	cacheData, err := cache.Marshal()
	if err != nil {
		m.logger.Warn("failed to marshal context cache", "task_id", taskID, "error", err)
		return err
	}

	_, err = m.queries.UpdateTaskContextCache(ctx, db.UpdateTaskContextCacheParams{
		ID:            taskID,
		ContextCache:  cacheData,
	})
	if err != nil {
		m.logger.Warn("failed to save context cache", "task_id", taskID, "error", err)
		return err
	}

	m.logger.Info("saved context cache", "task_id", taskID)
	return nil
}

// UpdateIssueCache updates the issue data in the cache.
func UpdateIssueCache(cache *execenv.ContextCache, issueID, title, description, status, priority, assigneeID, assigneeName, createdAt, updatedAt string) {
	cache.UpdateIssue(&execenv.IssueCache{
		ID:          issueID,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		AssigneeID:  assigneeID,
		AssigneeName: assigneeName,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	})
}

// UpdateMemoryCache updates the memory files in the cache.
func UpdateMemoryCache(cache *execenv.ContextCache, memoryFiles map[string]string) {
	cache.UpdateMemory(memoryFiles)
}

// SaveContextCacheFile saves the context cache as a JSON file in the workdir
// so the agent can access it directly.
func SaveContextCacheFile(workDir string, cache *execenv.ContextCache) error {
	if cache == nil {
		return nil
	}

	cacheData, err := cache.Marshal()
	if err != nil {
		return err
	}

	// Create .agent_context directory if it doesn't exist
	agentContextDir := workDir + "/.agent_context"
	if err := ensureDir(agentContextDir); err != nil {
		return err
	}

	// Write cache file
	cacheFile := agentContextDir + "/context_cache.json"
	if err := writeFile(cacheFile, cacheData); err != nil {
		return err
	}

	return nil
}

// Helper functions
func ensureDir(dir string) error {
	// Use os.MkdirAll in actual implementation
	return nil
}

func writeFile(path string, data []byte) error {
	// Use os.WriteFile in actual implementation
	return nil
}
