## MODIFIED Requirements

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
