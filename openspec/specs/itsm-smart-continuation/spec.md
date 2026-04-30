# Capability: itsm-smart-continuation

## Purpose

ITSM SmartEngine continuation dispatcher -- 统一所有活动状态变更后的续推进口，避免重复触发、漏触发和熔断后继续推进。
## Requirements
### Requirement: 统一 continuation dispatcher
SmartEngine SHALL 将 `ensureContinuation(tx *gorm.DB, ticket *Ticket, completedActivityID uint)` 作为统一推进入口，并在满足推进条件时直接触发事件驱动决策调度（direct decision dispatch），而非提交 `itsm-smart-progress` 轮询任务。该入口 MUST 执行终态检查、熔断检查、并签收敛检查与幂等防重。

#### Scenario: 正常推进走直接调度
- **WHEN** activity 完成后调用 ensureContinuation 且工单非终态、未熔断、并签已收敛
- **THEN** 系统 SHALL 触发 direct decision dispatch
- **AND** 不提交 itsm-smart-progress 任务作为主路径

#### Scenario: 终态与熔断阻断推进
- **WHEN** 工单已终态或 ai_failure_count 达到熔断阈值
- **THEN** ensureContinuation SHALL 返回且不触发新调度

#### Scenario: 并签未收敛不推进
- **WHEN** completedActivityID 属于并签组且尚未全部完成
- **THEN** ensureContinuation SHALL 返回且不触发新调度

### Requirement: 所有状态变更路径调用 ensureContinuation
以下操作在完成 activity 状态变更后 SHALL 调用统一 continuation dispatcher：
- `Progress()`（人工 activity 同意或驳回后）
- AI 决策确认或拒绝路径
- `HandleActionExecute`（action 完成后）
- 并行动作收敛完成后
- smart recovery 发现孤儿工单后

#### Scenario: 驳回后触发新决策循环
- **WHEN** 用户驳回人工活动后 continuation dispatcher 完成
- **THEN** SHALL 在事务提交后启动新的决策循环
- **AND** 新决策循环 SHALL 能通过 `decision.ticket_context` 工具看到被驳回的活动及其驳回原因

#### Scenario: Action 完成后触发下一步
- **WHEN** `HandleActionExecute` 标记 smart engine 的 action activity 为 completed
- **THEN** SHALL 调用 continuation dispatcher
- **AND** SHALL 在事务提交后启动下一轮决策

#### Scenario: 并行 action 被正确调度
- **WHEN** SmartEngine 创建了 action 类型的并行活动
- **THEN** SHALL 为每个 action activity 提交动作执行任务
- **AND** 所有动作完成或超时收敛后 SHALL 通过 continuation dispatcher 触发决策

