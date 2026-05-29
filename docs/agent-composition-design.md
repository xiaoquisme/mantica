# Agent Composition Design: Three-Layer Architecture

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Kanban Agent (Orchestrator)                        │
├─────────────────────────────────────────────────────────────────────────────┤
│  - Receives high-level tasks (e.g., "Fix bug #123", "Implement feature X")  │
│  - Decomposes into sub-tasks for role agents                                 │
│  - Monitors progress across all role agents                                  │
│  - Aggregates results and reports completion                                 │
└───────────────────────┬─────────────────────────────────────────────────────┘
                        │
                        │ Creates tasks in agent_task_queue
                        │
        ┌───────────────┼───────────────┬───────────────┬───────────────┐
        │               │               │               │               │
        ▼               ▼               ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│    BA Agent   │ │   TL Agent   │ │   DEV Agent  │ │   QA Agent   │ │Reviewer Agent│
│ (Business    │ │ (Tech Lead)  │ │ (Developer)  │ │   (Quality   │ │  (Code       │
│  Analyst)    │ │              │ │              │ │   Assurance) │ │  Reviewer)   │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
        │               │               │               │               │
        └───────────────┼───────────────┼───────────────┼───────────────┘
                        │               │               │
                        ▼               ▼               ▼
                ┌─────────────────────────────────────────────────────┐
                │             Role Agent Composition                   │
                │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
                │  │ Main Agent  │──│Terminal Agent│  │Summary Agent│ │
                │  │  (Reasoning │  │  (Command   │  │  (Result    │ │
                │  │   & Control)│  │   Execution)│  │   Summary)  │ │
                │  └─────────────┘  └─────────────┘  └─────────────┘ │
                │        │                   │               │        │
                │        │                   │               │        │
                │        └───────────────────┼───────────────┘        │
                │                            │                        │
                │                    Parallel Execution               │
                └─────────────────────────────────────────────────────┘
```

## Detailed Sub-Agent Communication Flow

The communication is a **cyclic pattern**, not a one-time linear flow:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Role Agent (e.g., DEV Agent)                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    Main Agent (Orchestrator)                         │   │
│  │                                                                      │   │
│  │  1. Receives task from Kanban Agent                                 │   │
│  │  2. Analyzes and plans execution steps                              │   │
│  │  3. Creates Terminal task(s)                                        │   │
│  │  4. Waits for Terminal results                                      │   │
│  │  5. Creates Summary task                                            │   │
│  │  6. Waits for Summary results                                       │   │
│  │  7. Analyzes summary → decides next action                          │   │
│  │  8. If more work needed → go to step 3 (CYCLE)                      │   │
│  │  9. If done → return final result to Kanban Agent                   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│              ┌─────────────────────┼─────────────────────┐                  │
│              │                     │                     │                  │
│              ▼                     │                     ▼                  │
│  ┌─────────────────────┐          │          ┌─────────────────────┐        │
│  │  Terminal Agent     │          │          │  Summary Agent      │        │
│  │                     │          │          │                     │        │
│  │  - Executes ONE     │          │          │  - Summarizes ONE   │        │
│  │    command          │          │          │    result           │        │
│  │  - Returns raw      │          │          │  - Returns          │        │
│  │    output           │          │          │    structured       │        │
│  │  - Minimal context  │          │          │    summary          │        │
│  └─────────────────────┘          │          └─────────────────────┘        │
│              │                     │                     │                  │
│              │                     │                     │                  │
│              └─────────────────────┴─────────────────────┘                  │
│                                    │                                        │
│                          Cyclic Coordination                                │
│                 (Main → Terminal → Summary → Main → ...)                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Example Cyclic Execution

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Task: "Implement user authentication feature"                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Cycle 1:                                                                   │
│  ├── Main: "First, let's check the existing auth code"                     │
│  ├── Terminal: "grep -r 'auth' src/"                                       │
│  ├── Summary: "Found 3 auth files, no login function exists"               │
│  └── Main: "Need to create login function"                                 │
│                                                                             │
│  Cycle 2:                                                                   │
│  ├── Main: "Create the login handler"                                      │
│  ├── Terminal: "cat src/auth/login.go" (or create it)                      │
│  ├── Summary: "Login function created with email/password params"          │
│  └── Main: "Good, now add tests"                                           │
│                                                                             │
│  Cycle 3:                                                                   │
│  ├── Main: "Run the tests"                                                 │
│  ├── Terminal: "go test ./src/auth/"                                       │
│  ├── Summary: "Tests passed, 3/3 cases successful"                         │
│  └── Main: "Implementation complete, return to Kanban"                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Insight: Cyclic vs Parallel

**Initial (Incorrect) View:**
- Terminal and Summary execute in parallel once
- Linear flow: Main → (Terminal + Summary) → Result

**Correct Cyclic View:**
- Main Agent orchestrates multiple cycles
- Each cycle: Main → Terminal → Summary → Main
- Terminal and Summary are NOT parallel to each other
- They work sequentially within each cycle
- Number of cycles depends on task complexity

## Overview

This design proposes a three-layer agent architecture to improve token utilization and task isolation:

1. **Kanban Agent** (top layer): Task decomposition and coordination
2. **Role Agents** (middle layer): BA, TL, DEV, QA, Reviewer, etc.
3. **Sub-agents** (bottom layer): main, terminal, summary

## Current Architecture

- Single agent per task
- Full context loaded for each agent
- Token waste: 88% due to repeated context loading

## Proposed Architecture

### Layer 1: Kanban Agent (Orchestrator)

**Responsibilities:**
- Receive high-level tasks (e.g., "Fix this bug", "Implement this feature")
- Decompose into sub-tasks for role agents
- Monitor progress of role agents
- Aggregate results and report completion

**Implementation:**
- New agent type in `agent_config.yaml`
- Special instructions for task decomposition
- Can create tasks in `agent_task_queue` for role agents

### Layer 2: Role Agents (BA, TL, DEV, QA, Reviewer)

Each role agent has three internal sub-agents:

#### 2.1 Main Agent
- Receives task from Kanban agent
- Analyzes task requirements
- Decomposes into terminal commands and summary needs
- Coordinates terminal and summary agents
- Makes decisions based on summarized results

#### 2.2 Terminal Agent
- Specialized for command execution
- Receives specific terminal commands from main agent
- Executes commands in isolated environment
- Returns raw output (stdout, stderr, exit code)
- Minimal context - only command and working directory

#### 2.3 Summary Agent
- Specialized for result summarization
- Receives raw output from terminal agent
- Generates human-readable summary
- Extracts key information, errors, warnings
- Provides structured output for main agent

### Layer 3: Sub-agent Coordination

**Execution Flow (Sequential within each cycle):**
```
Main Agent
    │
    ├──► Terminal Agent (executes commands) ──► returns output
    │
    └──► Summary Agent (summarizes output) ──► returns summary
```

**Communication Pattern:**
1. Main agent creates task for terminal agent
2. Terminal agent executes and returns raw output
3. Main agent creates task for summary agent with raw output
4. Summary agent returns structured summary
5. Main agent processes summary, decides next steps (cycle or complete)

## Data Model Changes

### 1. Agent Composition

Create new `agent_composition` table:

```sql
-- Migration 050: Agent Composition
-- File: server/migrations/050_agent_composition.up.sql

-- Agent Composition (defines parent-child relationships between agents)
CREATE TABLE agent_composition (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    child_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('main', 'terminal', 'summary', 'orchestrator')),
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(parent_agent_id, child_agent_id, role)
);

-- Index for faster lookups
CREATE INDEX idx_agent_composition_parent ON agent_composition(parent_agent_id);
CREATE INDEX idx_agent_composition_child ON agent_composition(child_agent_id);
```

### 2. Task Hierarchy

```sql
-- Migration 051: Task Hierarchy
-- File: server/migrations/051_task_hierarchy.up.sql

-- Add parent task reference for task hierarchy
ALTER TABLE agent_task_queue ADD COLUMN parent_task_id UUID REFERENCES agent_task_queue(id) ON DELETE CASCADE;

-- Add subagent role to track which subagent is executing the task
ALTER TABLE agent_task_queue ADD COLUMN subagent_role TEXT CHECK (subagent_role IN ('kanban', 'main', 'terminal', 'summary', 'orchestrator'));

-- Add task depth to prevent infinite recursion
ALTER TABLE agent_task_queue ADD COLUMN task_depth INT NOT NULL DEFAULT 0;

-- Index for parent task queries
CREATE INDEX idx_agent_task_queue_parent ON agent_task_queue(parent_task_id);
CREATE INDEX idx_agent_task_queue_depth ON agent_task_queue(task_depth);
```

### 3. Agent Task Status for Sub-agents

```sql
-- Migration 052: Sub-agent Task Status
-- File: server/migrations/052_subagent_task_status.up.sql

-- Add status for sub-agent coordination
ALTER TABLE agent_task_queue ADD COLUMN waiting_for_subagents BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_task_queue ADD COLUMN completed_subagents INT NOT NULL DEFAULT 0;
ALTER TABLE agent_task_queue ADD COLUMN total_subagents INT NOT NULL DEFAULT 0;
ALTER TABLE agent_task_queue ADD COLUMN failed_subagents INT NOT NULL DEFAULT 0;
```

## Daemon Modifications

### 1. Task Hierarchy Management

The daemon needs to handle parent-child task relationships:

```go
// In daemon.go - New function to create sub-tasks
func (d *Daemon) createSubTask(ctx context.Context, parentTaskID string, subagentRole string, agentID string, issueID string) (string, error) {
    // Generate sub-task with parent_task_id
    // Set task_depth = parent_depth + 1
    // Set subagent_role
    // Return new task ID
}

// Function to check if all sub-tasks are completed
func (d *Daemon) checkSubTaskCompletion(ctx context.Context, parentTaskID string) (bool, error) {
    // Query: SELECT COUNT(*) FROM agent_task_queue 
    //        WHERE parent_task_id = $1 AND status NOT IN ('completed', 'failed', 'cancelled')
    // Return true if all sub-tasks completed
}
```

### 2. Sub-agent Coordination in handleTask

```go
// Modified handleTask function
func (d *Daemon) handleTask(ctx context.Context, task Task) {
    // ... existing code ...
    
    // Check if this is a parent task with sub-agents
    if task.SubagentRole == "orchestrator" || task.SubagentRole == "main" {
        // Mark task as waiting for sub-agents
        d.client.UpdateTaskStatus(ctx, task.ID, "waiting_subagents")
        
        // Create sub-tasks for terminal and summary agents
        terminalTaskID, err := d.createSubTask(ctx, task.ID, "terminal", terminalAgentID, task.IssueID)
        summaryTaskID, err := d.createSubTask(ctx, task.ID, "summary", summaryAgentID, task.IssueID)
        
        // Mark parent task as waiting
        d.client.SetWaitingForSubagents(ctx, task.ID, 2)
        
        // Parent task will be resumed when sub-tasks complete
        return
    }
    
    // ... existing execution code ...
    
    // After sub-task completion, check if parent needs to be resumed
    if task.ParentTaskID != "" {
        allCompleted, err := d.checkSubTaskCompletion(ctx, task.ParentTaskID)
        if allCompleted {
            // Resume parent task
            d.client.ResumeTask(ctx, task.ParentTaskID)
        }
    }
}
```

### 3. New Poll Loop for Sub-agent Tasks

```go
// Modified pollLoop to prioritize sub-agent tasks
func (d *Daemon) pollLoop(ctx context.Context) error {
    // ... existing code ...
    
    // Prioritize sub-agent tasks over new parent tasks
    // This ensures quick completion of sub-tasks
    task, err := d.client.ClaimTask(ctx, rid)
    if task != nil && task.SubagentRole != "" {
        // This is a sub-agent task - handle with high priority
        d.handleSubAgentTask(ctx, task)
    } else if task != nil {
        // Regular task
        d.handleTask(ctx, task)
    }
}
```

## Sub-agent Communication Protocol

### 1. Cyclic Execution Pattern

The sub-agents follow a cyclic pattern orchestrated by the Main Agent:

```
Main Agent Loop:
    while task_not_complete:
        1. Main analyzes current state
        2. Main creates Terminal task (with specific command)
        3. Wait for Terminal result
        4. Main creates Summary task (with Terminal output)
        5. Wait for Summary result
        6. Main analyzes summary, decides:
           - If more work needed → continue loop
           - If done → break and return result
```

### 2. Task State Machine

```go
type SubAgentState struct {
    CycleCount     int
    CurrentPhase   string // "terminal", "summary", "decision"
    TerminalResult *TerminalResult
    SummaryResult  *SummaryResult
    NextCommand    string
}

func (m *MainAgent) ExecuteCyclic(ctx context.Context, task Task) error {
    state := &SubAgentState{CycleCount: 0}
    
    for {
        // Phase 1: Terminal execution
        state.CurrentPhase = "terminal"
        terminalResult, err := m.executeTerminal(ctx, task, state.NextCommand)
        if err != nil {
            return err
        }
        state.TerminalResult = terminalResult
        
        // Phase 2: Summary
        state.CurrentPhase = "summary"
        summaryResult, err := m.executeSummary(ctx, terminalResult)
        if err != nil {
            return err
        }
        state.SummaryResult = summaryResult
        
        // Phase 3: Decision
        state.CurrentPhase = "decision"
        decision, err := m.analyzeAndDecide(ctx, summaryResult)
        if err != nil {
            return err
        }
        
        if decision.IsComplete {
            // Task done, return final result
            return m.returnResult(ctx, task, decision.FinalResult)
        }
        
        // Prepare for next cycle
        state.NextCommand = decision.NextCommand
        state.CycleCount++
        
        // Safety: prevent infinite loops
        if state.CycleCount > MaxCycles {
            return fmt.Errorf("exceeded maximum cycles (%d)", MaxCycles)
        }
    }
}
```

### 3. Communication Channels

```go
// Each Main Agent maintains channels for its sub-agents
type MainAgentChannels struct {
    TerminalReq  chan TerminalRequest   // Main → Terminal
    TerminalResp chan TerminalResult    // Terminal → Main
    SummaryReq   chan SummaryRequest    // Main → Summary
    SummaryResp  chan SummaryResult     // Summary → Main
}

// Terminal Agent worker loop
func (t *TerminalAgent) WorkerLoop(ctx context.Context, channels *MainAgentChannels) {
    for {
        select {
        case req := <-channels.TerminalReq:
            // Execute command
            result := t.executeCommand(req.Command)
            // Send result back
            channels.TerminalResp <- result
        case <-ctx.Done():
            return
        }
    }
}

// Summary Agent worker loop
func (s *SummaryAgent) WorkerLoop(ctx context.Context, channels *MainAgentChannels) {
    for {
        select {
        case req := <-channels.SummaryReq:
            // Summarize output
            result := s.summarize(req.RawOutput)
            // Send result back
            channels.SummaryResp <- result
        case <-ctx.Done():
            return
        }
    }
}
```

### 4. In-Memory Channels

For sub-agents within the same parent task, use in-memory channels:

```go
type SubAgentChannel struct {
    terminalChan chan TerminalResult
    summaryChan  chan SummaryResult
    mainChan     chan MainDecision
}

// Shared across sub-agents of the same parent task
var taskChannels = make(map[string]*SubAgentChannel)
```

### 5. Database-based Coordination

For cross-task communication, use database:

```go
// Store sub-task results in parent task's result field
type ParentTaskResult struct {
    TerminalResult *TerminalResult `json:"terminal_result,omitempty"`
    SummaryResult  *SummaryResult  `json:"summary_result,omitempty"`
    MainDecision   *MainDecision   `json:"main_decision,omitempty"`
}
```

## Configuration Changes

### agent_config.yaml Extension

```yaml
agents:
  - name: DEV
    provider: claude
    instructions: |
      You are a Developer agent. Your job is to implement features and fix bugs.
      You will coordinate with sub-agents to execute tasks.
    max_concurrent_tasks: 6
    visibility: workspace
    skills:
      - Code Review
      - Architecture Review
    composition:
      - role: main
        provider: claude
        instructions: |
          You are the Main Agent for the DEV role. Your responsibilities:
          1. Receive tasks from Kanban agent
          2. Analyze requirements and plan implementation
          3. Delegate command execution to Terminal agent
          4. Delegate result summarization to Summary agent
          5. Make decisions based on summarized results
          6. Return final result to Kanban agent
          
          Guidelines:
          - Break down complex tasks into simple commands
          - Only provide essential context to sub-agents
          - Focus on decision-making, not execution
        skills: []
        max_concurrent_tasks: 2
      
      - role: terminal
        provider: claude
        instructions: |
          You are the Terminal Agent for the DEV role. Your responsibilities:
          1. Receive specific terminal commands from Main agent
          2. Execute commands in the working directory
          3. Return raw output (stdout, stderr, exit code)
          4. Do NOT make decisions or analyze output
          
          Rules:
          - Execute ONLY the exact command provided
          - Return complete output, do not truncate
          - Report errors immediately
          - No creativity - just execute
        skills: []
        max_concurrent_tasks: 10
      
      - role: summary
        provider: claude
        instructions: |
          You are the Summary Agent for the DEV role. Your responsibilities:
          1. Receive raw output from Terminal agent
          2. Generate concise, structured summary
          3. Extract key information, errors, warnings
          4. Format for Main agent decision-making
          
          Summary Format:
          - **Key Findings**: List important information
          - **Errors/Warnings**: Highlight issues
          - **Next Steps**: Suggest actions (if any)
          - **Confidence**: High/Medium/Low
          
          Guidelines:
          - Be concise but complete
          - Use bullet points
          - Prioritize actionable information
        skills: []
        max_concurrent_tasks: 10
```

### Complete Agent Composition Example

Here's a complete example showing how a role agent (DEV) interacts with its sub-agents:

```yaml
agents:
  # Kanban Agent - Top Level Coordinator
  - name: KANBAN
    provider: claude
    instructions: |
      You are a Kanban Agent. Your job is to:
      1. Receive high-level tasks from users
      2. Decompose into sub-tasks for role agents (BA, TL, DEV, QA)
      3. Monitor progress across all role agents
      4. Aggregate results and report completion
      
      Task Decomposition Rules:
      - Bug fixes: TL (analyze) → DEV (fix) → QA (test)
      - Features: BA (analyze) → TL (design) → DEV (implement) → QA (test)
      - Refactoring: TL (plan) → DEV (refactor) → Reviewer (review)
      
      Monitoring:
      - Track sub-task status
      - Handle failures with retry logic
      - Provide progress updates to user
    max_concurrent_tasks: 3
    visibility: workspace
  
  # Developer Agent with Sub-agents
  - name: DEV
    provider: claude
    instructions: |
      You are a Developer agent. You coordinate with sub-agents to implement code.
      When you receive a task:
      1. Main agent analyzes requirements
      2. Delegates commands to Terminal agent
      3. Delegates summarization to Summary agent
      4. Makes decisions based on results
      5. Returns final implementation
    max_concurrent_tasks: 6
    visibility: workspace
    skills:
      - Code Review
    composition:
      - role: main
        provider: claude
        instructions: |
          You are the decision-maker for DEV tasks.
          You analyze tasks, delegate to sub-agents, and make implementation decisions.
          Focus on:
          - Understanding requirements
          - Planning implementation steps
          - Delegating execution to Terminal agent
          - Processing results from Summary agent
          - Making final decisions
        skills: []
        max_concurrent_tasks: 2
      
      - role: terminal
        provider: claude
        instructions: |
          You are the executor for DEV tasks.
          Your ONLY job is to run terminal commands and return output.
          No analysis, no decisions, just execution.
          Example input: "git log --oneline -5"
          Example output: "abc1234 feat: add login\ndef5678 fix: auth bug"
        skills: []
        max_concurrent_tasks: 10
      
      - role: summary
        provider: claude
        instructions: |
          You are the summarizer for DEV tasks.
          You take raw terminal output and create concise summaries.
          Format:
          ## Summary
          - Key point 1
          - Key point 2
          
          ## Issues Found
          - Issue 1 (if any)
          
          ## Recommendation
          Next steps or actions needed
        skills: []
        max_concurrent_tasks: 10
```

### Kanban Agent Definition

```yaml
agents:
  - name: KANBAN
    provider: claude
    instructions: |
      You are a Kanban Agent. Your job is to:
      1. Receive high-level tasks
      2. Decompose into sub-tasks for role agents
      3. Monitor progress
      4. Aggregate results
      
      Decomposition Rules:
      - Analyze task type (bug fix, feature, refactor)
      - Assign to appropriate role agents
      - Set clear acceptance criteria
      - Track dependencies between sub-tasks
      
      Example Task Decomposition:
      
      Task: "Fix login bug where users can't authenticate"
      
      Sub-tasks:
      1. TL Agent: Analyze authentication flow
      2. DEV Agent: Fix identified issue
      3. QA Agent: Test fix with multiple scenarios
      
      Monitoring:
      - Poll sub-task status every 30 seconds
      - If sub-task fails, retry up to 2 times
      - Provide progress updates every minute
```

## Implementation Plan

### Phase 1: Data Model (2-3 days)

1. Create migration files for:
   - `agent_composition` table
   - `agent_task_queue.parent_task_id`
   - `agent_task_queue.subagent_role`

2. Update sqlc queries for new tables/fields

3. Update Go structs in `pkg/db/generated/`

### Phase 2: Agent Composition (3-4 days)

1. Extend `agent_config.yaml` parser to support `composition`
2. Modify `execenv/runtime_config.go` to generate sub-agent configs
3. Update daemon to handle sub-agent tasks

### Phase 3: Kanban Agent (2-3 days)

1. Implement Kanban agent in `agent_config.yaml`
2. Add task decomposition logic
3. Implement progress monitoring
4. Add result aggregation

### Phase 4: Sub-agent Coordination (4-5 days)

1. Implement main agent task delegation
2. Implement terminal agent command execution
3. Implement summary agent result summarization
4. Add inter-agent communication protocol

### Phase 5: Frontend Updates (2-3 days)

1. Display agent composition in UI
2. Show sub-task hierarchy
3. Visualize parallel execution

## Token Utilization Optimization

**Expected Improvements:**
- Terminal agent: ~80% token reduction (only command + output)
- Summary agent: ~70% token reduction (only raw output + summary)
- Main agent: ~50% token reduction (only summary + decisions)

**Total estimated token savings: 60-70%**

## Risks and Mitigations

### Risk 1: Increased Complexity
- **Mitigation**: Start with simple composition, add features incrementally

### Risk 2: Sub-agent Communication Overhead
- **Mitigation**: Use in-memory channels for sub-agents, database for orchestration

### Risk 3: Debugging Difficulty
- **Mitigation**: Comprehensive logging, task tracing, debug mode

## Next Steps

1. Review and approve design
2. Create database migration
3. Implement agent composition parsing
4. Build Kanban agent
5. Implement sub-agent coordination
6. Test with existing workflows

## Error Handling and Recovery

### 1. Sub-agent Failure Handling

```go
// When a sub-agent task fails
func (d *Daemon) onSubTaskFailure(ctx context.Context, subTask Task, errorMsg string) {
    parentTaskID := subTask.ParentTaskID
    
    // Log the failure
    d.logger.Error("sub-agent task failed", 
        "task_id", subTask.ID,
        "parent_task_id", parentTaskID,
        "subagent_role", subTask.SubagentRole,
        "error", errorMsg)
    
    // Update parent task status
    d.client.UpdateTaskStatus(ctx, parentTaskID, "failed")
    
    // Optionally: Create a retry task
    if subTask.RetryCount < MaxRetries {
        d.createRetryTask(ctx, subTask)
    } else {
        // Mark parent task as failed
        d.client.FailTask(ctx, parentTaskID, fmt.Sprintf("Sub-agent %s failed: %s", subTask.SubagentRole, errorMsg))
    }
}
```

### 2. Timeout Handling

```go
// Sub-agent task timeout
func (d *Daemon) handleSubAgentTimeout(ctx context.Context, task Task) {
    timeout := time.Duration(task.TimeoutSeconds) * time.Second
    
    select {
    case <-time.After(timeout):
        d.logger.Warn("sub-agent task timed out", "task_id", task.ID)
        d.client.FailTask(ctx, task.ID, "timeout")
    case <-ctx.Done():
        // Task completed or cancelled
    }
}
```

### 3. Parent Task Recovery

```go
// If parent task crashes mid-execution
func (d *Daemon) recoverParentTask(ctx context.Context, parentTaskID string) {
    // Check if all sub-tasks are completed
    allCompleted, err := d.checkSubTaskCompletion(ctx, parentTaskID)
    if err != nil {
        d.logger.Error("failed to check sub-task completion", "error", err)
        return
    }
    
    if allCompleted {
        // Resume parent task
        d.client.ResumeTask(ctx, parentTaskID)
    } else {
        // Re-queue incomplete sub-tasks
        d.requeueIncompleteSubTasks(ctx, parentTaskID)
    }
}
```

## Performance Considerations

### 1. Sub-agent Pool Management

```go
// Maintain a pool of sub-agents to avoid repeated creation
type SubAgentPool struct {
    mu          sync.RWMutex
    mainAgents  map[string]*SubAgent
    termAgents  map[string]*SubAgent
    summAgents  map[string]*SubAgent
}

func (p *SubAgentPool) GetOrCreate(role string, agentID string) *SubAgent {
    // Reuse existing sub-agent or create new one
}
```

### 2. Context Caching for Sub-agents

```go
// Cache context for sub-agents to avoid repeated loading
type SubAgentContextCache struct {
    cache map[string]*CachedContext
}

// Each sub-agent only gets necessary context
func (c *SubAgentContextCache) GetContextForRole(role string, taskID string) *TaskContext {
    switch role {
    case "terminal":
        return &TaskContext{
            Command: c.getCommand(taskID),
            WorkDir: c.getWorkDir(taskID),
        }
    case "summary":
        return &TaskContext{
            RawOutput: c.getRawOutput(taskID),
            TaskType:  c.getTaskType(taskID),
        }
    case "main":
        return &TaskContext{
            IssueDescription: c.getIssueDescription(taskID),
            SubTaskResults:   c.getSubTaskResults(taskID),
        }
    }
}
```

### 3. Batch Sub-agent Execution

```go
// Execute multiple sub-agents in parallel
func (d *Daemon) executeSubAgentsParallel(ctx context.Context, parentTask Task) error {
    var wg sync.WaitGroup
    errChan := make(chan error, 2)
    
    // Execute terminal and summary agents in parallel
    wg.Add(2)
    
    go func() {
        defer wg.Done()
        if err := d.executeTerminalAgent(ctx, parentTask); err != nil {
            errChan <- err
        }
    }()
    
    go func() {
        defer wg.Done()
        if err := d.executeSummaryAgent(ctx, parentTask); err != nil {
            errChan <- err
        }
    }()
    
    wg.Wait()
    close(errChan)
    
    // Check for errors
    for err := range errChan {
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

## Monitoring and Observability

### 1. Metrics Collection

```go
type SubAgentMetrics struct {
    TasksCreated      prometheus.Counter
    TasksCompleted    prometheus.Counter
    TasksFailed       prometheus.Counter
    ExecutionTime     prometheus.Histogram
    TokenUsage        prometheus.CounterVec
}

// Track metrics for each sub-agent type
func (m *SubAgentMetrics) RecordExecution(role string, duration time.Duration, tokens int) {
    m.ExecutionTime.Observe(duration.Seconds())
    m.TokenUsage.WithLabelValues(role).Add(float64(tokens))
}
```

### 2. Tracing

```go
// Add tracing for sub-agent execution
func (d *Daemon) traceSubAgentExecution(ctx context.Context, task Task) func() {
    tracer := otel.Tracer("sub-agent")
    ctx, span := tracer.Start(ctx, fmt.Sprintf("sub-agent-%s", task.SubagentRole))
    
    return func() {
        span.End()
    }
}
```

## Security Considerations

### 1. Isolation

- Each sub-agent runs in isolated context
- Sub-agents cannot access parent task's full context
- Command execution is sandboxed

### 2. Permission Scoping

```go
// Sub-agents have limited permissions
type SubAgentPermissions struct {
    CanExecuteCommands bool
    CanReadFiles       bool
    CanWriteFiles      bool
    CanAccessNetwork   bool
}

// Terminal agent: execute commands, read/write files
// Summary agent: read only
// Main agent: coordinate, no direct execution
```

## Integration with Existing Pipeline

### Current Pipeline Architecture

The existing pipeline (`server/internal/pipeline/pipeline.go`) defines:

```go
var Stages = map[string]StageConfig{
    "ready_analyze":     {AgentName: "BA", InProgressStatus: "in_analyze"},
    "ready_arch_design": {AgentName: "TL", InProgressStatus: "in_arch_design"},
    "ready_dev":         {AgentName: "DEV", InProgressStatus: "in_dev"},
    "ready_review":      {AgentName: "Code Review", InProgressStatus: "in_review"},
    "ready_test":        {AgentName: "QA", InProgressStatus: "in_test"},
}
```

### Selected Approach: Option B - Kanban Agent as Pipeline Orchestrator

**One-step implementation: Replace the entire pipeline with Kanban-led orchestration.**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        NEW ARCHITECTURE (Option B)                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    KANBAN AGENT (Entry Point)                        │   │
│  │                                                                      │   │
│  │  1. Receives issue from user (or Classifier)                        │   │
│  │  2. Analyzes task type (bug fix, feature, refactor)                 │   │
│  │  3. Decomposes into sub-tasks for role agents                       │   │
│  │  4. Creates parallel/sequential tasks in agent_task_queue           │   │
│  │  5. Monitors progress across all role agents                        │   │
│  │  6. Aggregates results and reports completion                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│              ┌─────────────────────┼─────────────────────┬─────────────────┤
│              │                     │                     │                 │
│              ▼                     ▼                     ▼                 │
│  ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐   │
│  │    TL Agent         │ │    DEV Agent        │ │    QA Agent         │   │
│  │ (Tech Lead)         │ │ (Developer)         │ │ (Quality Assurance) │   │
│  │                     │ │                     │ │                     │   │
│  │ ┌─────────────────┐ │ │ ┌─────────────────┐ │ │ ┌─────────────────┐ │   │
│  │ │ Main Agent      │ │ │ │ Main Agent      │ │ │ │ Main Agent      │ │   │
│  │ │ Terminal Agent  │ │ │ │ Terminal Agent  │ │ │ │ Terminal Agent  │ │   │
│  │ │ Summary Agent   │ │ │ │ Summary Agent   │ │ │ │ Summary Agent   │ │   │
│  │ └─────────────────┘ │ │ └─────────────────┘ │ │ └─────────────────┘ │   │
│  └─────────────────────┘ └─────────────────────┘ └─────────────────────┘   │
│              │                     │                     │                 │
│              └─────────────────────┼─────────────────────┘                 │
│                                    ▼                                        │
│                          ┌──────────────────┐                              │
│                          │ Final Result     │                              │
│                          │ (to user)        │                              │
│                          └──────────────────┘                              │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Changes Required

#### 1. Pipeline Replacement

```go
// OLD: Sequential stages
var Stages = map[string]StageConfig{
    "ready_analyze":     {AgentName: "BA", InProgressStatus: "in_analyze"},
    "ready_arch_design": {AgentName: "TL", InProgressStatus: "in_arch_design"},
    "ready_dev":         {AgentName: "DEV", InProgressStatus: "in_dev"},
    // ...
}

// NEW: Single entry point
var Stages = map[string]StageConfig{
    "ready_kanban": {AgentName: "KANBAN", InProgressStatus: "in_kanban"},
}
```

#### 2. Kanban Agent Decomposition Logic

```go
type KanbanAgent struct {
    // Decomposition rules based on task type
    DecompositionRules map[string]DecompositionRule
}

type DecompositionRule struct {
    Name        string
    SubTasks    []SubTaskTemplate
    Dependencies map[string][]string // task dependencies
}

// Example decomposition rules
var BugFixRule = DecompositionRule{
    Name: "Bug Fix",
    SubTasks: []SubTaskTemplate{
        {Agent: "TL", Task: "Analyze root cause"},
        {Agent: "DEV", Task: "Implement fix", DependsOn: []string{"TL"}},
        {Agent: "QA", Task: "Test fix", DependsOn: []string{"DEV"}},
        {Agent: "Reviewer", Task: "Review code", DependsOn: []string{"DEV"}},
    },
}

var FeatureRule = DecompositionRule{
    Name: "Feature Implementation",
    SubTasks: []SubTaskTemplate{
        {Agent: "BA", Task: "Analyze requirements"},
        {Agent: "TL", Task: "Design architecture", DependsOn: []string{"BA"}},
        {Agent: "DEV", Task: "Implement feature", DependsOn: []string{"TL"}},
        {Agent: "QA", Task: "Write and run tests", DependsOn: []string{"DEV"}},
        {Agent: "Reviewer", Task: "Review implementation", DependsOn: []string{"DEV"}},
    },
}
```

#### 3. Parallel Task Execution

```go
func (k *KanbanAgent) ExecuteParallel(ctx context.Context, task Task) error {
    // Get decomposition rule
    rule := k.getDecompositionRule(task.Type)
    
    // Create all sub-tasks
    subTasks := k.createSubTasks(task, rule)
    
    // Execute based on dependencies
    return k.executeWithDependencies(ctx, subTasks, rule.Dependencies)
}

func (k *KanbanAgent) executeWithDependencies(ctx context.Context, tasks []SubTask, deps map[string][]string) error {
    // Track completed tasks
    completed := make(map[string]bool)
    results := make(map[string]*TaskResult)
    
    // Execute tasks respecting dependencies
    for {
        // Find tasks ready to execute (all deps completed)
        ready := k.findReadyTasks(tasks, completed, deps)
        if len(ready) == 0 {
            break
        }
        
        // Execute ready tasks in parallel
        var wg sync.WaitGroup
        var mu sync.Mutex
        
        for _, task := range ready {
            wg.Add(1)
            go func(t SubTask) {
                defer wg.Done()
                result := k.executeSubTask(ctx, t)
                
                mu.Lock()
                results[t.ID] = result
                completed[t.ID] = true
                mu.Unlock()
            }(task)
        }
        
        wg.Wait()
    }
    
    // Aggregate results
    return k.aggregateResults(results)
}
```

#### 4. Database Changes (Simplified)

Since Kanban Agent creates sub-tasks directly:

```sql
-- Add parent_task_id for task hierarchy
ALTER TABLE agent_task_queue ADD COLUMN parent_task_id UUID REFERENCES agent_task_queue(id);
ALTER TABLE agent_task_queue ADD COLUMN subagent_role TEXT; -- 'kanban', 'main', 'terminal', 'summary'
ALTER TABLE agent_task_queue ADD COLUMN task_depth INT NOT NULL DEFAULT 0;
```

#### 5. Agent Configuration

```yaml
agents:
  - name: KANBAN
    provider: claude
    instructions: |
      You are a Kanban Agent, the orchestrator for all tasks.
      
      Your responsibilities:
      1. Receive high-level tasks (bug fixes, features, refactoring)
      2. Decompose into sub-tasks for specialized agents
      3. Monitor progress and handle failures
      4. Aggregate results and report completion
      
      Decomposition Rules:
      
      BUG FIX:
        1. TL Agent: Analyze root cause
        2. DEV Agent: Implement fix (depends on TL)
        3. QA Agent: Test fix (depends on DEV)
        4. Reviewer: Review code (depends on DEV)
      
      FEATURE:
        1. BA Agent: Analyze requirements
        2. TL Agent: Design architecture (depends on BA)
        3. DEV Agent: Implement (depends on TL)
        4. QA Agent: Test (depends on DEV)
        5. Reviewer: Review (depends on DEV)
      
      REFACTORING:
        1. TL Agent: Plan refactoring
        2. DEV Agent: Execute refactoring (depends on TL)
        3. Reviewer: Review changes (depends on DEV)
      
      Monitoring:
      - Poll sub-task status every 30 seconds
      - Retry failed sub-tasks up to 2 times
      - Provide progress updates to user
      
      When all sub-tasks complete:
      - Summarize what was done
      - List any issues or blockers
      - Update issue status to 'done' or 'blocked'
    max_concurrent_tasks: 3
    visibility: workspace
  
  # Role agents with sub-agent composition
  - name: TL
    provider: claude
    instructions: |
      You are a Tech Lead agent. You analyze requirements and design solutions.
    composition:
      - role: main
        provider: claude
        instructions: |
          You are the Main Agent for TL tasks.
          Analyze requirements, plan implementation steps.
          Delegate command execution to Terminal agent.
          Delegate summarization to Summary agent.
          Make decisions based on summarized results.
        max_concurrent_tasks: 2
      
      - role: terminal
        provider: claude
        instructions: |
          You are the Terminal Agent for TL tasks.
          Execute ONLY the exact command provided.
          Return complete output, do not truncate.
        max_concurrent_tasks: 10
      
      - role: summary
        provider: claude
        instructions: |
          You are the Summary Agent for TL tasks.
          Summarize terminal output concisely.
          Extract key information, errors, warnings.
        max_concurrent_tasks: 10
    max_concurrent_tasks: 4
    visibility: workspace
  
  # Similar for DEV, QA, Reviewer, BA agents...
```

### New Flow with Kanban Agent

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Example: "Fix login bug where users can't authenticate"                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Step 1: Kanban Agent receives task                                         │
│  ├── Analyzes: This is a BUG FIX                                           │
│  └── Applies Bug Fix decomposition rule                                     │
│                                                                             │
│  Step 2: Create sub-tasks (parallel where possible)                         │
│  ├── TL-Task: "Analyze authentication flow"                                │
│  │   └── TL Main: Check auth code                                          │
│  │       └── TL Terminal: "grep -r 'auth' src/"                            │
│  │       └── TL Summary: "Found 3 auth files, login function missing"      │
│  │   └── TL returns: "Root cause: no login handler"                        │
│  │                                                                          │
│  ├── DEV-Task: "Implement login handler" (waits for TL)                    │
│  │   └── DEV Main: Plan implementation                                     │
│  │       └── DEV Terminal: "cat > src/auth/login.go"                       │
│  │       └── DEV Summary: "Login function created"                         │
│  │   └── DEV returns: "Implementation complete"                            │
│  │                                                                          │
│  ├── QA-Task: "Test login fix" (waits for DEV)                             │
│  │   └── QA Main: Plan test cases                                          │
│  │       └── QA Terminal: "go test ./src/auth/"                            │
│  │       └── QA Summary: "3/3 tests passed"                                │
│  │   └── QA returns: "All tests passed"                                    │
│  │                                                                          │
│  └── Reviewer-Task: "Review login code" (waits for DEV, parallel with QA)  │
│      └── Reviewer Main: Review code                                        │
│          └── Reviewer Terminal: "git diff src/auth/"                       │
│          └── Reviewer Summary: "Clean code, good error handling"           │
│      └── Reviewer returns: "Approved"                                      │
│                                                                             │
│  Step 3: Kanban Agent aggregates results                                   │
│  ├── Root cause found and fixed                                            │
│  ├── All tests passed                                                      │
│  ├── Code reviewed and approved                                            │
│  └── Final status: DONE                                                    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Implementation Plan for Option B

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Phase 1: Core Infrastructure (1 week)                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│  1. Database migration (parent_task_id, subagent_role, task_depth)          │
│  2. Update sqlc queries for new fields                                      │
│  3. Create SubAgentPool for managing sub-agent instances                    │
│  4. Create MainAgentChannels for in-memory communication                    │
│                                                                             │
│  Deliverable: Database ready, basic sub-agent infrastructure                │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Phase 2: Sub-agent Implementation (1 week)                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│  1. Implement Terminal Agent (command execution only)                       │
│  2. Implement Summary Agent (result summarization only)                     │
│  3. Implement Main Agent (cyclic orchestration)                             │
│  4. Test sub-agent cycle with simple commands                               │
│                                                                             │
│  Deliverable: Sub-agents work correctly in isolation                        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Phase 3: Kanban Agent (1 week)                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  1. Implement Kanban Agent with decomposition rules                         │
│  2. Implement dependency-based parallel execution                           │
│  3. Integrate with existing task queue                                      │
│  4. Test with real issues (bug fixes first)                                 │
│                                                                             │
│  Deliverable: Kanban Agent can orchestrate role agents                      │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Phase 4: Pipeline Replacement (1 week)                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  1. Replace Classifier entry with Kanban Agent                              │
│  2. Update pipeline.go to use Kanban as entry point                         │
│  3. Update AllowedAgentTransitions                                          │
│  4. Handle backward compatibility for existing issues                       │
│                                                                             │
│  Deliverable: New pipeline replaces old one                                 │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Phase 5: Frontend & Testing (1 week)                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│  1. Update UI to show Kanban coordination view                              │
│  2. Show parallel task execution                                            │
│  3. Display sub-task hierarchy                                              │
│  4. End-to-end testing with all issue types                                 │
│  5. Performance testing and optimization                                    │
│                                                                             │
│  Deliverable: Complete, tested implementation                               │
└─────────────────────────────────────────────────────────────────────────────┘

Total: 5 weeks
```

## Open Questions (Updated for Option B)

1. Should old pipeline issues be migrated to Kanban Agent, or continue with old logic?
2. How to handle the Classifier agent (currently the entry point)?
3. Should there be a feature flag to switch between old and new pipeline?
4. How to visualize parallel task execution in the frontend?