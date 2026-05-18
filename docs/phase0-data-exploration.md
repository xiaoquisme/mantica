# Multica 自训练系统 — Phase 0 数据探查报告

## 数据概览

| 表 | 记录数 |
|---|---|
| agent_task_queue | 176 |
| task_message | 101 |
| agent | 7 |
| issue | 34 |
| skill | 6 |
| activity_log | ~10 |

## 任务状态分布

| 状态 | 数量 | 占比 |
|---|---|---|
| cancelled | 115 | 65.3% |
| completed | 51 | 29.0% |
| failed | 10 | 5.7% |

## 关键发现

### 🔴 发现 #1：Hermes backend 不发送 tool 调用日志（阻塞性问题）

**现象**：task_message 表中 101 条记录全部是 `type=text`，零条 `tool_use`、`tool_result`、`thinking`、`error`。

**根因**：`server/pkg/agent/hermes.go` 使用 `hermes chat -q -Q`（quiet 模式），只在任务结束时发出一条 `MessageText`（完整输出文本），不流式输出中间的 tool 调用。

对比：
- **Claude Code backend**：使用 `--output-format stream-json`，完整输出 `MessageToolUse`、`MessageToolResult`、`MessageThinking`、`MessageText`
- **Codex backend**：同样有 `MessageToolUse`/`MessageToolResult`
- **Hermes backend**：❌ 只有 `MessageText`

**影响**：这是自训练系统的 #1 阻塞项。没有 tool 调用序列，就无法：
- 分析执行模式（用了哪些工具、什么顺序）
- 识别失败原因（哪一步出错）
- 提取成功模式（成功的任务有什么共同特征）
- 计算执行效率（工具调用次数、重试次数）

### 🟡 发现 #2：65% 的任务被 cancelled

115 个 cancelled 任务全部经过了 dispatched → started 阶段，但没有产出任何 message。

可能原因：
- 用户手动取消
- 系统超时自动取消
- Agent 启动失败

需要进一步确认取消原因（当前 `error` 字段为空）。

### 🟡 发现 #3：所有 failed 任务都是同一个原因

10 个 failed 任务全部来自 **QA agent**，错误均为 `hermes returned empty output`。

这说明 Hermes backend 在某些情况下会返回空输出（可能是 timeout、prompt 问题、或 Hermes 本身的 bug）。

### 🟢 发现 #4：completed 任务时长合理

| 指标 | 值 |
|---|---|
| 最短 | 38 秒 |
| 平均 | 154 秒 (2.5 分钟) |
| 最长 | 736 秒 (12 分钟) |

### 🟢 发现 #5：每个 completed 任务恰好 1 条 message

51 个 completed 任务，每个恰好有 1 条 text message。这是 Hermes backend 只发最终输出的直接结果。

### 🟡 发现 #6：activity_log details 为空

`activity_log` 表的 `details` 字段全部是 `{}`，没有记录任何有用信息（如 agent 名称、issue 标题、耗时等）。

---

## 自训练能力评估

| 能力 | 当前状态 | 说明 |
|---|---|---|
| 执行过程回放 | ❌ | 只有最终文本，没有 tool 调用序列 |
| 失败原因分析 | 🔶 | 只能从 error 字段看，但只有 "empty output" 一种 |
| 执行效率分析 | ❌ | 没有 tool_count、duration 等指标 |
| 跨任务模式识别 | ❌ | 数据不足以做模式分析 |
| Agent 能力评分 | ❌ | 没有质量评分数据 |
| 自动 Skill 提取 | ❌ | 没有执行过程数据 |

---

## 行动计划

### 立即行动（Phase 0.1）：修复 Hermes backend 消息流

**方案 A（推荐）：给 Hermes 加 stream-json 输出**
- 修改 `hermes.go`，用 `hermes chat -q --stream-json` 替代 `-Q`
- 需要 Hermes 支持 `--stream-json` 输出格式
- 最优解：完整的 tool_use/tool_result 流

**方案 B（快速）：解析 Hermes 文本输出**
- 在 daemon 中用正则从 Hermes 的文本输出中提取 tool 调用
- Hermes 的输出格式通常是：`🔧 terminal: command...` + `result`
- 可以从现有 51 条 text message 反向提取
- 优点：不需要改 Hermes，立即可用
- 缺点：解析不稳定，依赖输出格式

**方案 C（最小改动）：在 task 完成时用 LLM 分析文本**
- CompleteTask 时把 text message 送 LLM 做结构化分析
- 提取：用了什么工具、遇到什么错误、最终结果
- 写入 task_analysis 表
- 优点：不依赖 backend 改动
- 缺点：额外 LLM 调用成本

### 短期行动（Phase 0.2）：补全 activity_log

修改 activity_log 写入逻辑，记录：
- agent 名称
- issue 标题
- 任务耗时
- 取消原因（cancelled 任务）

### 中期行动（Phase 1）：结构化日志

在 Phase 0.1 修复后，给 task_message 加字段：
- `duration_ms`
- `success`
- `error_class`

---

## 结论

**当前数据不足以支撑自训练系统。** 核心瓶颈是 Hermes backend 不发送 tool 调用日志。

优先级：
1. **先修 Hermes backend**（方案 A 或 B），让 task_message 有 tool_use 数据
2. 然后才能做 Phase 2 的自动复盘
3. activity_log 补全可以并行

如果短期内无法改 Hermes backend，可以用 **方案 C（LLM 分析文本输出）** 作为过渡方案，先让自训练系统跑起来。
