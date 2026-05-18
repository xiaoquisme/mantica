# Phase A: Hermes backend 完整执行日志方案

## 核心发现

**Hermes session JSONL 文件里已经有完整的 tool_calls 数据！**

```json
// ~/.hermes/sessions/{session_id}.jsonl
{"role": "assistant", "tool_calls": [{"function": {"name": "terminal", "arguments": "{...}"}}]}
{"role": "tool", "name": "terminal", "content": "..."}
{"role": "assistant", "tool_calls": [{"function": {"name": "read_file", "arguments": "{...}"}}]}
{"role": "tool", "name": "read_file", "content": "..."}
```

每条记录都有：role、tool name、input (arguments)、output (content)、timestamp。

**不需要改 Hermes backend**，只需要在任务完成后从 session 文件中提取数据。

### ✅ 已修复：session_id 丢失问题

**根因**：Hermes CLI 将 `session_id: xxx` 输出到 **STDERR**（`cli.py line 14284`），
而 daemon 的 `hermesBackend.Execute()` 只读取 STDOUT，STDERR 被丢弃到 logger。

**修复**：在 `server/pkg/agent/hermes.go` 中：
1. 新增 `StderrPipe()` 捕获 stderr
2. 用 goroutine 读取 stderr 内容到 buffer
3. 任务完成后先从 stdout 提取 session_id，如果为空则从 stderr 提取
4. 新增 `extractSessionID()` 辅助函数

测试全部通过（15/15）。

### ✅ Step 2 完成：Session JSONL → task_message 提取

**新增文件：**
- `server/internal/daemon/session_extract.go` — Go 实现，集成到 daemon
- `scripts/extract_session.py` — Python 独立脚本，可单独测试

**工作原理：**
1. daemon 完成任务后，用 session_id 查找 `~/.hermes/sessions/{session_id}.jsonl`
2. 解析 JSONL 中的 assistant(tool_calls) + tool(result) 对
3. 转换为 TaskMessageData 格式，通过 ReportTaskMessages 发送到 server
4. 异步执行（`go d.ExtractAndSendSessionMessages(...)`），不阻塞主流程

**集成点：** `daemon.go` handleTask 函数，CompleteTask 成功后触发

## 问题：session_id 没有传到 DB

当前链路：
```
Hermes CLI 输出 "session_id: xxx"
  → hermesBackend.readOutput() 提取 session_id
    → Result.SessionID
      → daemon CompleteTask(result.SessionID)
        → client.CompleteTask(session_id)
          → server CompleteTask handler
            → DB agent_task_queue.session_id  ← 这里是 NULL!
```

**DB 中所有任务的 session_id 都是 NULL。** 需要排查为什么。

可能原因：
1. Hermes 没有输出 `session_id: xxx` 行（import error 导致启动失败）
2. 正则 `(?m)^session_id:\s*(.+)$` 没匹配到
3. daemon 在某处丢失了 session_id

## 实现方案

### Step 1: 修复 session_id 传递

确保 daemon → server → DB 链路通畅。需要：
1. 确认 Hermes 输出 `session_id: xxx` 格式
2. 确认 readOutput 正则匹配
3. 确认 CompleteTask 存入 DB

### Step 2: 任务完成后提取 session 数据

新增 `server/internal/service/session_extractor.go`：

```go
// 在 CompleteTask 末尾异步触发
func (s *TaskService) ExtractSessionMessages(ctx context.Context, taskID pgtype.UUID, sessionID string) {
    // 1. 定位 session 文件: ~/.hermes/sessions/*_{sessionID}*.jsonl
    // 2. 解析 JSONL，提取 assistant(tool_calls) + tool(result) 对
    // 3. 转换为 task_message 格式写入 DB
    // 4. 已有的 text 消息可以跳过（避免重复）
}
```

提取逻辑：
```
对每一对 assistant(tool_calls) + tool(result):
  → 写入 task_message:
    - type: "tool_use"
    - tool: function.name
    - input: function.arguments (JSON)
    - content: assistant 的 reasoning/text（如果有）

  → 写入 task_message:
    - type: "tool_result"
    - tool: function.name
    - output: tool content
```

### Step 3: 增强 task_message 字段

```sql
ALTER TABLE task_message ADD COLUMN duration_ms INT;
ALTER TABLE task_message ADD COLUMN success BOOLEAN;
ALTER TABLE task_message ADD COLUMN error_class TEXT;
```

在提取 session 数据时，通过 timestamp 差值计算 duration_ms，通过 output 内容判断 success 和 error_class。

### Step 4: 自动复盘

有了完整的 tool_use/tool_result 数据后，Phase 2 的分析引擎就可以：
- 统计 tool 使用频率和成功率
- 识别错误模式（哪些 tool 最容易失败）
- 分析执行路径（成功的任务用了什么 tool 组合）
- 计算效率指标（总步数、总耗时、重试次数）

## 文件清单

| 文件 | 改动类型 | 说明 |
|---|---|---|
| `server/internal/daemon/daemon.go` | 调试 | 确认 session_id 传递 |
| `server/internal/service/session_extractor.go` | 新增 | 从 session JSONL 提取 tool 数据 |
| `server/internal/service/task.go` | 修改 | CompleteTask 末尾触发提取 |
| `server/pkg/db/queries/task_message.sql` | 修改 | 新增 upsert 查询 |
| `server/migrations/041_task_message_enhance.up.sql` | 新增 | 加字段 |

## 依赖关系

```
Step 1 (修 session_id) 
  → Step 2 (提取 session 数据) 
    → Step 3 (加字段) 
      → Step 4 (自动复盘)
```

Step 1 和 Step 3 可以并行。
Step 2 依赖 Step 1（需要 session_id 才能找到文件）。
