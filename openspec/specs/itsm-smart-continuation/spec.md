# Capability: itsm-smart-continuation

## Purpose

ITSM SmartEngine continuation dispatcher -- 统一所有活动状态变更后的续推进口，避免重复触发、漏触发和熔断后继续推进。
## Requirements
### Requirement: 统一 continuation dispatcher
SmartEngine SHALL 提供统一 continuation dispatcher，作为所有智能流程推进的唯一入口。该入口 SHALL 按以下顺序执行检查：
1. 检查工单是否已终态（completed/rejected/withdrawn/cancelled/failed） -> 是则直接返回
2. 检查熔断状态（`ai_failure_count >= MaxAIFailureCount`） -> 是则记录日志并返回
3. 如果 completedActivityID 属于并签组，执行收敛检查 -> 未全部完成则返回
4. 将工单状态更新为对应决策中状态（approved_decisioning、rejected_decisioning 或 decisioning）
5. 在当前事务提交成功后，通过 direct decision dispatcher 启动 SmartEngine 决策 goroutine

#### Scenario: 正常推进
- **WHEN** activity 完成后调用 continuation dispatcher 且工单非终态、未熔断、非并签（或并签已全部收敛）
- **THEN** SHALL 注册事务后 direct decision dispatch
- **AND** SHALL NOT 依赖 `itsm-smart-progress` scheduler worker 轮询作为主路径

#### Scenario: 工单已终态
- **WHEN** 调用 continuation dispatcher 且工单 status 为 completed
- **THEN** SHALL 直接返回，不启动决策 goroutine

#### Scenario: 熔断状态
- **WHEN** 调用 continuation dispatcher 且工单 `ai_failure_count >= 3`
- **THEN** SHALL 记录 warning 日志并返回，不启动决策 goroutine

#### Scenario: 并签未全部完成
- **WHEN** 调用 continuation dispatcher 且 completedActivityID 属于一个 2 人并签组，组内仅 1 个已完成
- **THEN** SHALL 返回，不启动决策 goroutine

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

