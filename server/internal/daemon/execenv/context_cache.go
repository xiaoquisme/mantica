package execenv

import (
	"encoding/json"
	"time"
)

// ContextCache stores shared context data across agent stages.
// This structure is stored in agent_task_queue.context_cache.
type ContextCache struct {
	// Issue data (fetched once, reused by all stages)
	Issue *IssueCache `json:"issue,omitempty"`
	
	// Comments (incremental - first stage fetches all, later stages fetch since last fetch)
	Comments []CommentCache `json:"comments,omitempty"`
	LastCommentFetch time.Time `json:"last_comment_fetch,omitempty"`
	
	// Memory files (cached, rarely changes during a task)
	Memory map[string]string `json:"memory,omitempty"` // filename -> content
	
	// Code snippets (cached per file)
	CodeSnippets map[string]CodeSnippet `json:"code_snippets,omitempty"`
	
	// Workspace info (static, fetched once)
	Workspace *WorkspaceCache `json:"workspace,omitempty"`
}

// IssueCache stores issue details.
type IssueCache struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	AssigneeID  string `json:"assignee_id,omitempty"`
	AssigneeName string `json:"assignee_name,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CommentCache stores a single comment.
type CommentCache struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id,omitempty"`
	AuthorID  string `json:"author_id"`
	AuthorName string `json:"author_name"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// CodeSnippet stores a cached code file.
type CodeSnippet struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash"` // For cache invalidation
}

// WorkspaceCache stores workspace details.
type WorkspaceCache struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Members []MemberCache `json:"members,omitempty"`
}

// MemberCache stores a workspace member.
type MemberCache struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// NewContextCache creates an empty context cache.
func NewContextCache() *ContextCache {
	return &ContextCache{
		Comments:      make([]CommentCache, 0),
		Memory:        make(map[string]string),
		CodeSnippets:  make(map[string]CodeSnippet),
	}
}

// GetCachedContext deserializes context cache from JSON.
func GetCachedContext(data []byte) (*ContextCache, error) {
	if len(data) == 0 {
		return NewContextCache(), nil
	}
	
	var cache ContextCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return NewContextCache(), err
	}
	
	// Initialize maps if nil
	if cache.Memory == nil {
		cache.Memory = make(map[string]string)
	}
	if cache.CodeSnippets == nil {
		cache.CodeSnippets = make(map[string]CodeSnippet)
	}
	
	return &cache, nil
}

// Marshal serializes context cache to JSON.
func (c *ContextCache) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// HasIssue returns true if issue data is cached.
func (c *ContextCache) HasIssue() bool {
	return c.Issue != nil && c.Issue.ID != ""
}

// HasMemory returns true if memory is cached.
func (c *ContextCache) HasMemory() bool {
	return len(c.Memory) > 0
}

// GetCommentSince returns comments created after the given time.
func (c *ContextCache) GetCommentSince(since time.Time) []CommentCache {
	var result []CommentCache
	for _, comment := range c.Comments {
		t, err := time.Parse(time.RFC3339, comment.CreatedAt)
		if err != nil {
			continue
		}
		if t.After(since) {
			result = append(result, comment)
		}
	}
	return result
}

// UpdateIssue updates the cached issue data.
func (c *ContextCache) UpdateIssue(issue *IssueCache) {
	c.Issue = issue
}

// AddComment adds a comment to the cache if not already present.
func (c *ContextCache) AddComment(comment CommentCache) {
	for _, existing := range c.Comments {
		if existing.ID == comment.ID {
			return // Already cached
		}
	}
	c.Comments = append(c.Comments, comment)
	c.LastCommentFetch = time.Now()
}

// UpdateMemory updates the cached memory files.
func (c *ContextCache) UpdateMemory(files map[string]string) {
	c.Memory = files
}

// UpdateCodeSnippet updates a cached code file.
func (c *ContextCache) UpdateCodeSnippet(path, content, hash string) {
	c.CodeSnippets[path] = CodeSnippet{
		Path:    path,
		Content: content,
		Hash:    hash,
	}
}

// UpdateWorkspace updates the cached workspace data.
func (c *ContextCache) UpdateWorkspace(ws *WorkspaceCache) {
	c.Workspace = ws
}
