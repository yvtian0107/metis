## ADDED Requirements

### Requirement: 统一 continuation dispatcher
SmartEngine SHALL 提供 `ensureContinuation(tx *gorm.DB, ticket *Ticket, completedActivityID uint)` 方法，作为所有流程推进的唯一入口。该方法 SHALL 按以下顺序执行检查：
1. 检查工单是否已终态（completed/cancelled/failed）→ 是则直接返回
2. 检查熔断状态（ai_failure_count >= MaxAIFailureCount）→ 是则记录日志并返回
3. 如果 completedActivityID 属于并签组，执行收敛检查 → 未全部完成则返回
4. 提交 `itsm-smart-progress` 异步任务

#### Scenario: 正常推进
- **WHEN** activity 完成后调用 ensureContinuation 且工单非终态、未熔断、非并签（或并签已全部完成）
- **THEN** SHALL 提交 `itsm-smart-progress` 异步任务

#### Scenario: 工单已终态
- **WHEN** 调用 ensureContinuation 且工单 status 为 completed
- **THEN** SHALL 直接返回，不提交任何任务

#### Scenario: 熔断状态
- **WHEN** 调用 ensureContinuation 且工单 ai_failure_count >= 3
- **THEN** SHALL 记录 warning 日志并返回，不提交任务

#### Scenario: 并签未全部完成
- **WHEN** 调用 ensureContinuation 且 completedActivityID 属于一个 2 人并签组，组内仅 1 个已完成
- **THEN** SHALL 返回，不提交任务

### Requirement: 所有状态变更路径调用 ensureContinuation
以下操作在完成 activity 状态变更后 SHALL 调用 `ensureContinuation()`：
- `Progress()`（activity 完成后）
- `RejectActivity()`（reject 完成后，completedActivityID 传 0）
- `ConfirmActivity()`（执行 plan 后）
- `HandleActionExecute`（action 完成后）
- `executeParallelPlan`（创建并行 action activity 后需调度执行）

#### Scenario: Reject 后触发新决策循环
- **WHEN** 用户拒绝 AI 决策后 RejectActivity 完成
- **THEN** SHALL 调用 ensureContinuation，触发新的决策循环
- **AND** 新决策循环 SHALL 能通过 decision.ticket_context 工具看到被拒绝的活动及其 reject 原因

#### Scenario: Action 完成后触发下一步
- **WHEN** HandleActionExecute 标记 smart engine 的 action activity 为 completed
- **THEN** SHALL 调用 ensureContinuation，触发下一轮决策

#### Scenario: 并行 action 被正确调度
- **WHEN** executeParallelPlan 创建了 action 类型的并行活动
- **THEN** SHALL 为每个 action activity 提交 `itsm-action-execute` 异步任务
