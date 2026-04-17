## ADDED Requirements

### Requirement: 参与者缺失兜底检查

在 `executeDecisionPlan` 执行阶段，对需要参与者的 Activity（`approve`/`process`/`form`），当 `participant_id` 为 nil 或 0 时，系统 SHALL 查询 `EngineConfigProvider.FallbackAssigneeID()`。若返回有效用户 ID，SHALL 替换为该兜底用户创建 assignment 并记录 `participant_fallback` timeline 事件。

#### Scenario: 兜底替换后正常创建 assignment
- **WHEN** Activity 类型为 `approve`，AI 决策的 `participant_id` 为 nil，`FallbackAssigneeID()` 返回用户 ID 5
- **THEN** 系统 SHALL 创建 TicketAssignment（assignee_id=5），更新工单 assignee_id=5，记录 timeline 事件 `participant_fallback`

#### Scenario: 兜底后 Activity 状态正常
- **WHEN** Activity 通过兜底分配了参与者
- **THEN** Activity SHALL 保持原有状态逻辑（approve/process/form → `pending`），不因兜底而改变状态
