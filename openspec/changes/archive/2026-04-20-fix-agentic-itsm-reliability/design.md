## Context

Agentic ITSM 的 smart engine 在代码 review 中发现 25 个问题。最严重的 4 个 P0 问题会导致工单在生产环境中卡死：reject 后无后续、action 完成后不推进、并行 action 不调度、recovery 查询错误列。此外还有并发安全、前端权限、UX 体验等多层问题。

当前 smart engine 的流程推进散布在多个触发点（ticket 创建、activity 完成、人工确认、retry），缺乏统一的 continuation 保障，导致多个路径断裂。

## Goals / Non-Goals

**Goals:**
- 消除所有 P0 流程断裂问题，确保 smart engine 工单不会卡死
- 引入统一的 continuation 机制，确保任何 activity 状态变更都能可靠推进流程
- 修复并行执行的并发安全问题
- 为前端破坏性操作增加权限和确认保护
- 修复已知的 UX 问题（状态显示、outcome 一致性等）

**Non-Goals:**
- 不重构 smart engine 的整体架构（保持 SmartEngine + DecisionPlan 的模式）
- 不修改 AI 模块本身（仅修复 ITSM 侧的集成问题）
- 不新增 API 端点（仅修改现有端点行为）
- 不修改数据库 schema（所有修复在现有字段内完成）

## Decisions

### Decision 1: 统一 continuation dispatcher

**选择**: 在 `SmartEngine` 中引入 `ensureContinuation(ticketID, completedActivityID)` 方法，作为所有流程推进的唯一入口。

**替代方案**:
- A) 在每个触发点手动添加 `itsm-smart-progress` 提交 — 当前方案，已证明容易遗漏
- B) 引入事件总线 — 过度设计，增加架构复杂度

**理由**: 单一方法减少遗漏风险，所有状态变更路径（reject、action完成、progress、confirm）统一调用此方法。该方法内部处理并行收敛检查和熔断判断。

### Decision 2: Reject 后自动触发新决策循环

**选择**: `RejectActivity` 完成后调用 `ensureContinuation()`，将 reject 原因注入下一轮决策上下文。

**替代方案**:
- A) Reject 后自动创建 human process activity — 强制人工介入，不够灵活
- B) Reject 后不做任何事（当前行为）— 导致工单孤儿

**理由**: 让 AI 看到 reject 原因后重新决策，是最灵活的方案。如果 AI 连续失败，熔断机制会自动降级到人工。

### Decision 3: 并行收敛使用 `SELECT FOR UPDATE`

**选择**: 在 `Progress()` 的收敛检查中对 activity_group_id 对应的记录加行锁。

**替代方案**:
- A) 乐观锁（version column）— 需要 schema 变更，违反 non-goal
- B) 应用层分布式锁（Redis）— 增加基础设施依赖

**理由**: `SELECT FOR UPDATE` 是最简单有效的方案，不需要 schema 变更，符合现有 GORM 事务模式。

### Decision 4: Sequential plan 改为逐步创建

**选择**: `executeSequentialPlan` 只创建 DecisionPlan 中的第一个 activity。后续 activity 信息存储在 ticket 的决策上下文中，待当前 activity 完成后由下一轮决策循环处理。

**替代方案**:
- A) 创建所有 activity 但只激活第一个 — 需要额外的排队状态管理
- B) 保持现有行为（全部创建）— 语义错误且 current_activity_id 被覆盖

**理由**: 这符合 smart engine "每一步都由 AI 决策" 的核心设计理念。AI 在下一轮可以根据最新上下文调整后续步骤。

### Decision 5: Agent seed 不覆盖已自定义的 prompt

**选择**: seed 逻辑改为 `FirstOrCreate` 语义 — 仅在 agent 不存在时创建，已存在时只更新 tool bindings 不覆盖 prompt。

**替代方案**:
- A) 保持全量覆盖（当前行为）— 管理员自定义被擦除
- B) 加版本号对比 — 过度复杂

**理由**: 管理员自定义 prompt 是常见需求，seed 覆盖是意外的破坏性行为。Tool binding 需要保持同步（新增 tool），但 prompt 属于用户数据。

### Decision 6: execute_action 幂等保护

**选择**: 在 `execute_action` tool handler 中先查询 `ticket_action_executions` 表，如果该 action 已成功执行过则直接返回缓存结果。

**理由**: LLM 调用不确定性高，重复执行 webhook 可能产生副作用（如重复创建资源）。查询成本极低，防护价值大。

### Decision 7: 前端 override 权限校验

**选择**: 后端在 override 端点（jump/reassign/retry-ai）增加 Casbin 权限检查，前端根据用户权限决定是否渲染 override 按钮。

**理由**: 前端校验可以被绕过，真正的保护必须在后端。前端隐藏按钮是 UX 优化。

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Reject 后自动重新决策可能陷入 reject→决策→低置信度→reject 循环 | 熔断机制限制最多 3 次连续失败，超过后自动降级到人工 |
| `SELECT FOR UPDATE` 在高并发下可能造成锁等待 | 并签组通常只有 2-5 个活动，锁持有时间极短（毫秒级） |
| Sequential plan 只创建第一个 activity 改变了现有行为 | AI agent 本身就是按步决策，实际上 LLM 很少在一次决策中输出多个 sequential activity |
| seed 不覆盖 prompt 可能导致旧 prompt 与新 tool 不兼容 | seed 日志中记录 prompt 是否被跳过，管理员可以手动更新 |
| execute_action 幂等检查基于 action_id 匹配，同一 action 可能需要合法重试 | 只检查 status=success 的执行记录，failed 的不阻止重试 |
