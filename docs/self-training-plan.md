# Multica 自训练系统实现计划

> 灵感来源：AgenTank 的"回放分析→迭代改进"闭环
> 核心理念：agent 的每一次执行都是训练数据，每一次失败都是进化机会

## 现状分析

### 已有的（回放数据）
- `task_message` 表：记录了 tool_use / tool_result / thinking / text / error 五种事件
- `agent_task_queue` 表：记录了任务生命周期 (queued → dispatched → running → completed/failed)
- `result` (JSONB)：任务完成时的输出
- `error`：任务失败时的错误信息

### 缺失的（复盘能力）
- 没有**失败原因分类** — 只知道"failed"，不知道为什么
- 没有**执行效率指标** — 不知道用了多少工具、花了多久、重试了几次
- 没有**质量评分** — completed 不等于"做得好"
- 没有**跨任务模式识别** — 每个任务是孤岛
- 没有**自动 skill 提取** — 成功经验没有沉淀
- 没有**agent 能力评分** — 无法比较哪个 agent 更适合哪类任务

---

## 实现路径

### Phase 0: 数据探查（零代码改动）
**目标**：用现有数据做一次分析，确认实际缺失什么

**已完成** → 详见 `phase0-data-exploration.md`

核心发现：**Hermes backend 只发 MessageText，不发 tool_use/tool_result**。
这是自训练系统的 #1 阻塞项。

### Phase 0.1: 修复 Hermes backend 消息流（前置条件）

三个方案：
| 方案 | 改动 | 效果 | 优先级 |
|---|---|---|---|
| A: Hermes 加 stream-json | 改 hermes.go | 完整 tool 日志 | 推荐 |
| B: 解析文本输出 | 改 daemon.go | 近似 tool 日志 | 快速 |
| C: LLM 分析文本 | 新增 analyzer | 结构化分析 | 过渡 |

---

### Phase 1: 结构化执行日志（增强 task_message）
**目标**：让每条消息带上下文，而不只是"发生了什么"

#### 1.1 task_message 加字段

```sql
ALTER TABLE task_message ADD COLUMN duration_ms INT;
ALTER TABLE task_message ADD COLUMN success BOOLEAN;
ALTER TABLE task_message ADD COLUMN error_class TEXT;
ALTER TABLE task_message ADD COLUMN tokens_in INT;
ALTER TABLE task_message ADD COLUMN tokens_out INT;
```

- `duration_ms`：tool_use 到 tool_result 的时间差（毫秒）
- `success`：tool_result 是否成功（通过检查 output 是否包含 error/traceback）
- `error_class`：错误分类（syntax / runtime / timeout / permission / network / not_found）
- `tokens_in/out`：该步骤的 token 消耗（如果可获取）

#### 1.2 agent_task_queue 加字段

```sql
ALTER TABLE agent_task_queue ADD COLUMN tool_count INT;
ALTER TABLE agent_task_queue ADD COLUMN error_count INT;
ALTER TABLE agent_task_queue ADD COLUMN duration_ms INT;
ALTER TABLE agent_task_queue ADD COLUMN quality_score FLOAT;
ALTER TABLE agent_task_queue ADD COLUMN failure_class TEXT;
ALTER TABLE agent_task_queue ADD COLUMN retry_count INT DEFAULT 0;
```

#### 1.3 Daemon 端改动

在 `daemon.go` 的消息收集循环中：
- 记录每个 tool_use 的时间戳
- 在收到 tool_result 时计算 duration_ms
- 根据 output 内容自动判断 success 和 error_class
- 在 CompleteTask / FailTask 时汇总统计写入 agent_task_queue

---

### Phase 2: 自动复盘服务
**目标**：每个任务完成后，自动生成一份"复盘报告"

#### 2.1 新增 `task_analysis` 表

```sql
CREATE TABLE task_analysis (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    -- 执行摘要
    summary TEXT,
    -- 失败原因（仅 failed 任务）
    failure_reason TEXT,
    failure_class TEXT, -- test_fail | build_error | timeout | logic_error | dependency
    -- 关键决策点
    decision_points JSONB, -- [{step: 15, tool: "terminal", choice: "方案A", context: "..."}]
    -- 错误模式
    error_pattern TEXT, -- 识别出的重复错误模式
    -- 改进建议
    improvement_hint TEXT, -- "下次遇到类似问题应该..."
    -- 质量评分
    quality_score FLOAT, -- 0-100
    quality_factors JSONB, -- {completeness: 80, efficiency: 60, correctness: 90}
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### 2.2 分析引擎

新增 `server/internal/service/analyzer.go`：

```
任务完成/失败
    → 读取 task_message 序列
    → 提取特征：
        - 工具使用序列 (terminal → read_file → patch → terminal)
        - 错误出现位置和频率
        - 总步数、总耗时
        - 是否有"死循环"（同一 tool 同一参数反复调用）
    → 分类失败原因
    → 生成复盘报告
    → 写入 task_analysis
```

#### 2.3 触发时机

在 `TaskService.CompleteTask` 和 `TaskService.FailTask` 末尾，异步触发分析：
- 不阻塞主流程
- 通过 events.Bus 发布 `task:analyze` 事件
- Analyzer 订阅该事件，异步处理

---

### Phase 3: 跨任务模式识别 + Agent 评分
**目标**：从历史数据中提取可操作的洞察

#### 3.1 Agent 能力评分

```sql
CREATE TABLE agent_score (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    -- 评分维度
    overall_score FLOAT,      -- 综合评分 (类似 ELO)
    task_type_scores JSONB,   -- {"bug_fix": 85, "feature": 70, "refactor": 90}
    -- 统计
    total_tasks INT,
    success_rate FLOAT,
    avg_quality FLOAT,
    avg_duration_ms INT,
    -- 趋势
    score_trend TEXT, -- "improving" | "stable" | "declining"
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### 3.2 模式识别查询

```sql
-- 最常见的失败模式
SELECT failure_class, error_pattern, COUNT(*) as cnt
FROM task_analysis
WHERE failure_class IS NOT NULL
GROUP BY failure_class, error_pattern
ORDER BY cnt DESC LIMIT 10;

-- 哪个 agent 在哪类任务上表现最好
SELECT a.name, t.issue_type,
       AVG(ta.quality_score) as avg_quality,
       COUNT(*) as task_count
FROM task_analysis ta
JOIN agent_task_queue atq ON ta.task_id = atq.id
JOIN agent a ON atq.agent_id = a.id
JOIN issue t ON atq.issue_id = t.id
GROUP BY a.name, t.issue_type
ORDER BY avg_quality DESC;

-- 成功任务的共同工具组合
SELECT tool_sequence, COUNT(*) as success_count
FROM (
    SELECT task_id, array_agg(tool ORDER BY seq) as tool_sequence
    FROM task_message
    WHERE type = 'tool_use'
    AND task_id IN (SELECT id FROM agent_task_queue WHERE status = 'completed')
    GROUP BY task_id
) sub
GROUP BY tool_sequence
ORDER BY success_count DESC LIMIT 10;
```

#### 3.3 前端仪表盘

在 agent 详情页新增 "Performance" tab：
- 能力评分雷达图（按任务类型）
- 成功率趋势线
- 最近失败原因分布
- 工具使用热力图

---

### Phase 4: 自动进化
**目标**：让分析结果自动驱动改进

#### 4.1 成功任务 → 自动 Skill 提取

当一个任务 quality_score > 80 且是首次成功完成某类任务时：
1. 提取该任务的 task_message 序列
2. 用 LLM 总结为可复用的 skill 文档
3. 自动创建 skill 记录
4. 关联到执行该任务的 agent

```
触发条件: quality_score > 80 AND 无同类已有 skill
输入: task_message 序列 + issue 描述 + 最终输出
LLM prompt: "根据以下执行记录，总结出一个可复用的 skill 文档..."
输出: skill (name, description, content, files)
```

#### 4.2 失败任务 → 改进指令生成

当一个任务失败时：
1. 读取 task_analysis 的 failure_class 和 improvement_hint
2. 如果同一 agent 连续 2+ 次同类失败，生成改进指令
3. 写入 agent 的 memory 或 skill 中
4. 下次同类任务时自动注入 prompt

```
触发条件: 同一 agent 连续 2+ 次同类 failure_class
输入: 失败的 task_analysis 列表
LLM prompt: "该 agent 在 X 类任务上连续失败，分析原因并生成改进建议..."
输出: 写入 agent memory 的改进指令
```

#### 4.3 任务分配优化

当 agent_score 表有足够数据后：
- 分配任务时参考 agent 的 task_type_scores
- 优先分配评分高的任务类型给对应 agent
- 对评分持续下降的 agent 发出告警

---

## 优先级排序

| Phase | 内容 | ROI | 工作量 | 依赖 |
|-------|------|-----|--------|------|
| 0 | 数据探查 | ★★★ | 1天 | 无 |
| 1 | 结构化日志 | ★★★★ | 3天 | Phase 0 |
| 2 | 自动复盘 | ★★★★★ | 5天 | Phase 1 |
| 3 | 模式识别+评分 | ★★★★ | 5天 | Phase 2 |
| 4 | 自动进化 | ★★★★★ | 7天 | Phase 3 |

**建议：先做 Phase 0 + Phase 2**

理由：
- Phase 0 零成本，立刻能看到数据价值
- Phase 2 的分析引擎可以先基于现有数据做，不需要等 Phase 1 的新字段
- Phase 1 的字段增强可以和 Phase 2 并行，根据分析结果决定哪些字段真正需要

---

## 与 AgenTank 的对应关系

```
AgenTank                    Multica 自训练系统
──────────────────────────────────────────────────
帧级回放                    task_message (已有)
胜负判定 (crashed/star)     failure_class (Phase 1)
回放分析                    task_analysis (Phase 2)
ELO 排名                    agent_score (Phase 3)
代码迭代                    skill 自动提取 (Phase 4)
Simulation                  Phase 0 数据探查
Challenge                   Phase 2 自动复盘
submittedBy                 agent attribution
replay JSON                 task_analysis JSONB
```
