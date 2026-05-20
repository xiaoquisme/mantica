# Skill 治理系统 — 实现计划

> 灵感来源：Hermes Agent 的 Curator 系统
> 核心理念：自动提取的 skill 需要质量控制、去重、合并、淘汰，否则会变成垃圾堆

## 问题

当前 Evolver 的自动 skill 提取是"只进不出"的：
- 每个符合条件的成功任务都会创建一个新 skill
- 没有质量门槛
- 没有去重（相似 skill 会重复创建）
- 没有淘汰（低质量 skill 永远存在）
- 没有合并（相似 skill 不会自动合并）

结果：skill 表会被大量低质量、重复的 skill 填满，反而降低 agent 的表现。

## 设计原则

```
AgenTank 对应:
  模拟测试    → skill 提取前的质量检查
  排位赛      → skill 在真实任务中的表现
  ELO 排名    → skill 质量评分
  代码迭代    → skill 内容更新/合并
  淘汰赛      → 低质量 skill 的清理
```

## Phase 1: Skill 质量门控

**目标**: 只有高质量的执行模式才能成为 skill

### 1.1 提取条件（已有，需增强）

当前条件：
- ✅ tool_count >= 2
- ✅ error_count <= tool_count / 2

新增条件：
- ❌ communication_quality >= 0.6（输出不能太短）
- ❌ first_attempt_success = true（优先从首次成功的任务提取）
- ❌ task 必须是 completed（不是 cancelled）
- ❌ 同一 agent 在同类任务上至少成功 2 次才提取（避免偶然成功）

### 1.2 Skill 质量评分

每个 skill 有一个 quality_score（0-100），初始值基于提取时的分析数据：

```
quality_score = base_score * success_bonus * efficiency_bonus

base_score = communication_quality * 100
success_bonus = 1.0 + (first_attempt_success ? 0.2 : 0)
efficiency_bonus = 1.0 + (tool_efficiency > 0.5 ? 0.1 : 0)
```

### 1.3 新增字段

```sql
ALTER TABLE skill ADD COLUMN quality_score FLOAT DEFAULT 0;
ALTER TABLE skill ADD COLUMN source_task_id UUID;
ALTER TABLE skill ADD COLUMN usage_count INT DEFAULT 0;
ALTER TABLE skill ADD COLUMN last_used_at TIMESTAMPTZ;
ALTER TABLE skill ADD COLUMN pinned BOOLEAN DEFAULT FALSE;
```

---

## Phase 2: 去重与合并

**目标**: 相似 skill 不重复创建，而是合并增强

### 2.1 相似度检测

在创建新 skill 前，检查 workspace 内是否已有相似 skill：

```
相似度判断:
  1. tool 序列重叠度 > 70% → 相似
  2. skill name 包含相同关键词 → 相似
  3. 内容有 > 50% 相同的步骤 → 相似
```

### 2.2 合并策略

```
如果新 skill 和已有 skill 相似:
  已有 skill 的 quality_score > 新 skill → 跳过，不创建
  新 skill 的 quality_score > 已有 skill → 更新已有 skill 的内容
  两者质量相近 → 合并：保留更详细的步骤，合并 pitfalls
```

### 2.3 合并算法

```go
func mergeSkills(existing, new Skill) Skill {
    // 保留更详细的 workflow steps
    // 合并 pitfalls（去重）
    // 更新 summary（取最新的）
    // quality_score = max(existing, new)
    // usage_count = existing.usage_count
}
```

---

## Phase 3: Skill 生命周期管理

**目标**: skill 会进化、也会淘汰

### 3.1 使用追踪

当 agent 执行任务时，如果 prompt 中注入了某个 skill，记录该 skill 被使用：
- usage_count++
- last_used_at = now()

### 3.2 效果评估

skill 被使用后，观察任务结果：
- 任务成功 → quality_score += 1
- 任务失败 → quality_score -= 2
- 连续 3 次失败 → 标记为 "under_review"

### 3.3 自动淘汰

```
淘汰条件（满足任一）:
  - quality_score < 30
  - 30 天未被使用且 quality_score < 60
  - 连续 5 次使用后任务失败
  - 被标记为 under_review 超过 7 天

淘汰方式:
  - 不直接删除（保留数据用于分析）
  - 设置 archived_at = now()
  - 从 agent_skill 关联中移除
```

### 3.4 Pin 机制

重要的 skill 可以被 pin，防止自动淘汰：
- 手动 pin 的 skill 永不淘汰
- 被使用 10+ 次且 quality_score > 70 的 skill 自动 pin

---

## Phase 4: 治理 API + 前端

### 4.1 API 端点

```
GET    /api/skills/{id}/quality     → skill 质量详情
POST   /api/skills/{id}/pin         → pin/unpin skill
GET    /api/skills/governance       → 治理概览（待清理、待合并、高质量）
POST   /api/skills/merge            → 手动合并两个 skill
DELETE /api/skills/{id}/archive     → 归档低质量 skill
```

### 4.2 前端 — Skill 治理页面

在 Skills 页面新增 "Governance" tab：

```
┌─────────────────────────────────────────────┐
│ Skills    Governance                         │
├─────────────────────────────────────────────┤
│                                              │
│ 📊 Overview                                  │
│   Total: 12  Active: 8  Archived: 4         │
│   Avg Quality: 72  Pinned: 3                │
│                                              │
│ ⚠️ Needs Attention (3)                       │
│   ┌─────────────────────────────────────┐   │
│   │ auto/terminal-workflow  quality: 28  │   │
│   │ Last used: 15 days ago              │   │
│   │ [Archive] [Pin] [Edit]              │   │
│   ├─────────────────────────────────────┤   │
│   │ auto/execute_code-workflow           │   │
│   │ Similar to: auto/python-workflow     │   │
│   │ [Merge] [Archive] [Keep]            │   │
│   └─────────────────────────────────────┘   │
│                                              │
│ ✅ Top Skills (3)                            │
│   auto/terminal-execute_code  q=85 📌       │
│   auto/read_file-patch        q=78 📌       │
│   auto/search-terminal        q=72          │
│                                              │
└─────────────────────────────────────────────┘
```

---

## 实现顺序

| Phase | 内容 | 工作量 | 依赖 |
|-------|------|--------|------|
| 1.1 | 增强提取条件 | 1h | 无 |
| 1.2 | quality_score 计算 | 2h | 1.1 |
| 1.3 | 新增 DB 字段 + migration | 1h | 无 |
| 2.1 | 相似度检测 | 3h | 1.2 |
| 2.2 | 合并策略 | 2h | 2.1 |
| 3.1 | 使用追踪 | 2h | 1.3 |
| 3.2 | 效果评估 | 2h | 3.1 |
| 3.3 | 自动淘汰 | 2h | 3.2 |
| 4.1 | 治理 API | 2h | 3.3 |
| 4.2 | 前端治理页面 | 3h | 4.1 |

总计: ~20h

---

## 与 AgenTank 的对应

```
AgenTank              Skill 治理
──────────────────────────────────
模拟测试              quality_score 门控
排位赛                skill 在真实任务中的使用
ELO 排名              quality_score 迭代
代码迭代              skill 内容合并/更新
淘汰赛                自动归档低质量 skill
submittedBy           source_task_id 追溯来源
replay JSON           skill 使用历史记录
```
