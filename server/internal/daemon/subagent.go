package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SubAgentRole represents the role of a sub-agent
type SubAgentRole string

const (
	RoleMain     SubAgentRole = "main"
	RoleTerminal SubAgentRole = "terminal"
	RoleSummary  SubAgentRole = "summary"
	RoleKanban   SubAgentRole = "kanban"
)

// TerminalRequest represents a request to the terminal agent
type TerminalRequest struct {
	Command string
	WorkDir string
	TaskID  string
}

// TerminalResult represents the result from the terminal agent
type TerminalResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

// SummaryRequest represents a request to the summary agent
type SummaryRequest struct {
	RawOutput string
	TaskType  string
	TaskID    string
}

// SummaryResult represents the result from the summary agent
type SummaryResult struct {
	Summary    string
	KeyFindings []string
	Errors     []string
	Warnings   []string
	Confidence string // "high", "medium", "low"
}

// MainDecision represents the decision from the main agent
type MainDecision struct {
	IsComplete   bool
	FinalResult  string
	NextCommand  string
	NeedsMoreWork bool
}

// SubAgentState tracks the state of a cyclic sub-agent execution
type SubAgentState struct {
	CycleCount     int
	CurrentPhase   string // "terminal", "summary", "decision"
	TerminalResult *TerminalResult
	SummaryResult  *SummaryResult
	NextCommand    string
	MaxCycles      int
}

// MainAgentChannels maintains channels for sub-agent communication
type MainAgentChannels struct {
	TerminalReq  chan TerminalRequest
	TerminalResp chan TerminalResult
	SummaryReq   chan SummaryRequest
	SummaryResp  chan SummaryResult
	MainDecision chan MainDecision
}

// NewMainAgentChannels creates a new set of channels for a main agent
func NewMainAgentChannels() *MainAgentChannels {
	return &MainAgentChannels{
		TerminalReq:  make(chan TerminalRequest, 10),
		TerminalResp: make(chan TerminalResult, 10),
		SummaryReq:   make(chan SummaryRequest, 10),
		SummaryResp:  make(chan SummaryResult, 10),
		MainDecision: make(chan MainDecision, 10),
	}
}

// SubAgent represents a sub-agent instance
type SubAgent struct {
	ID       string
	Role     SubAgentRole
	AgentID  string
	Provider string
	Channels *MainAgentChannels
}

// SubAgentPool manages sub-agent instances
type SubAgentPool struct {
	mu          sync.RWMutex
	agents      map[string]*SubAgent // agentID -> SubAgent
	logger      *slog.Logger
}

// NewSubAgentPool creates a new sub-agent pool
func NewSubAgentPool(logger *slog.Logger) *SubAgentPool {
	return &SubAgentPool{
		agents: make(map[string]*SubAgent),
		logger: logger,
	}
}

// GetOrCreate gets or creates a sub-agent for the given agent and role
func (p *SubAgentPool) GetOrCreate(agentID string, role SubAgentRole, provider string) *SubAgent {
	key := fmt.Sprintf("%s-%s", agentID, role)
	
	p.mu.RLock()
	if agent, ok := p.agents[key]; ok {
		p.mu.RUnlock()
		return agent
	}
	p.mu.RUnlock()
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Double-check after acquiring write lock
	if agent, ok := p.agents[key]; ok {
		return agent
	}
	
	agent := &SubAgent{
		ID:       fmt.Sprintf("%s-%s-%d", agentID, role, time.Now().UnixNano()),
		Role:     role,
		AgentID:  agentID,
		Provider: provider,
		Channels: NewMainAgentChannels(),
	}
	
	p.agents[key] = agent
	p.logger.Info("created sub-agent", "id", agent.ID, "role", role, "agent_id", agentID)
	
	return agent
}

// Remove removes a sub-agent from the pool
func (p *SubAgentPool) Remove(agentID string, role SubAgentRole) {
	key := fmt.Sprintf("%s-%s", agentID, role)
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	delete(p.agents, key)
	p.logger.Info("removed sub-agent", "agent_id", agentID, "role", role)
}

// SubAgentExecution manages the cyclic execution of sub-agents
type SubAgentExecution struct {
	MainAgent    *SubAgent
	TerminalAgent *SubAgent
	SummaryAgent  *SubAgent
	State        *SubAgentState
	logger       *slog.Logger
}

// NewSubAgentExecution creates a new sub-agent execution manager
func NewSubAgentExecution(main, terminal, summary *SubAgent, logger *slog.Logger) *SubAgentExecution {
	return &SubAgentExecution{
		MainAgent:     main,
		TerminalAgent: terminal,
		SummaryAgent:  summary,
		State: &SubAgentState{
			CycleCount: 0,
			MaxCycles:  10, // Safety limit
		},
		logger: logger,
	}
}

// ExecuteCyclic runs the cyclic execution pattern
func (e *SubAgentExecution) ExecuteCyclic(ctx context.Context, taskID string, initialCommand string) (*MainDecision, error) {
	e.State.NextCommand = initialCommand
	
	for {
		e.logger.Info("starting cycle", "task_id", taskID, "cycle", e.State.CycleCount)
		
		// Phase 1: Terminal execution
		e.State.CurrentPhase = "terminal"
		terminalResult, err := e.executeTerminal(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("terminal execution failed: %w", err)
		}
		e.State.TerminalResult = terminalResult
		
		// Phase 2: Summary
		e.State.CurrentPhase = "summary"
		summaryResult, err := e.executeSummary(ctx, taskID, terminalResult)
		if err != nil {
			return nil, fmt.Errorf("summary execution failed: %w", err)
		}
		e.State.SummaryResult = summaryResult
		
		// Phase 3: Decision (handled by main agent externally)
		e.State.CurrentPhase = "decision"
		
		// Return the results for main agent to make decision
		// The caller (main agent) will decide whether to continue or return
		return &MainDecision{
			IsComplete: false, // Caller will determine this
			NextCommand: e.State.NextCommand,
		}, nil
	}
}

// executeTerminal sends a command to the terminal agent and waits for result
func (e *SubAgentExecution) executeTerminal(ctx context.Context, taskID string) (*TerminalResult, error) {
	req := TerminalRequest{
		Command: e.State.NextCommand,
		TaskID:  taskID,
	}
	
	select {
	case e.TerminalAgent.Channels.TerminalReq <- req:
		// Request sent
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout sending terminal request")
	}
	
	select {
	case result := <-e.TerminalAgent.Channels.TerminalResp:
		return &result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(300 * time.Second): // 5 minute timeout for commands
		return nil, fmt.Errorf("timeout waiting for terminal result")
	}
}

// executeSummary sends output to the summary agent and waits for result
func (e *SubAgentExecution) executeSummary(ctx context.Context, taskID string, terminalResult *TerminalResult) (*SummaryResult, error) {
	rawOutput := terminalResult.Stdout
	if terminalResult.Stderr != "" {
		rawOutput += "\nSTDERR:\n" + terminalResult.Stderr
	}
	
	req := SummaryRequest{
		RawOutput: rawOutput,
		TaskID:    taskID,
	}
	
	select {
	case e.SummaryAgent.Channels.SummaryReq <- req:
		// Request sent
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout sending summary request")
	}
	
	select {
	case result := <-e.SummaryAgent.Channels.SummaryResp:
		return &result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(60 * time.Second): // 1 minute timeout for summary
		return nil, fmt.Errorf("timeout waiting for summary result")
	}
}

// SetNextCommand sets the next command for the next cycle
func (e *SubAgentExecution) SetNextCommand(command string) {
	e.State.NextCommand = command
	e.State.CycleCount++
	
	if e.State.CycleCount >= e.State.MaxCycles {
		e.logger.Warn("reached maximum cycles", "max", e.State.MaxCycles)
	}
}