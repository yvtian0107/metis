## ADDED Requirements

### Requirement: 智能引擎参与者缺失兜底分配

当智能引擎执行 DecisionPlan 时，若 Activity 类型需要参与者（`approve`/`process`/`form`）但 AI 未指定有效 `participant_id`（nil 或 0），系统 SHALL 检查引擎配置中的 `fallback_assignee` 设置。若已配置且该用户 active，SHALL 自动将 Activity 分配给该兜底处理人，并在 Timeline 记录转派原因。

#### Scenario: 配置兜底人且参与者缺失时自动转派
- **WHEN** 智能引擎执行 DecisionPlan，Activity 类型为 `approve` 且 `participant_id` 为空，且引擎配置 `fallback_assignee` 为有效的活跃用户 ID
- **THEN** 系统 SHALL 创建 TicketAssignment 指向该兜底用户，更新工单 `assignee_id`，并在 Timeline 添加 `participant_fallback` 事件："参与者缺失，已转派兜底处理人（{username}）"

#### Scenario: 未配置兜底人时保持原行为
- **WHEN** 智能引擎执行 DecisionPlan，Activity 类型为 `process` 且 `participant_id` 为空，且引擎配置 `fallback_assignee` 为 0 或未设置
- **THEN** 系统 SHALL 创建 Activity 但不创建 TicketAssignment（与当前行为一致），不记录兜底事件

#### Scenario: 兜底用户不存在或未激活
- **WHEN** 智能引擎执行 DecisionPlan，Activity 需要参与者但缺失，`fallback_assignee` 配置的用户 ID 对应的用户不存在或 `is_active=false`
- **THEN** 系统 SHALL 跳过兜底分配，在 Timeline 记录 warning："兜底处理人无效（ID={id}），请检查引擎配置"

#### Scenario: AI 已指定有效参与者时不触发兜底
- **WHEN** 智能引擎执行 DecisionPlan，Activity 的 `participant_id` 为有效的活跃用户 ID
- **THEN** 系统 SHALL 按正常流程创建 TicketAssignment，不检查 `fallback_assignee` 配置

#### Scenario: 不需要参与者的 Activity 类型不触发兜底
- **WHEN** 智能引擎执行 DecisionPlan，Activity 类型为 `action`/`notify`/`complete`/`escalate` 且 `participant_id` 为空
- **THEN** 系统 SHALL 不触发兜底逻辑，按原有行为处理

### Requirement: EngineConfigProvider 接口

SmartEngine SHALL 通过 `EngineConfigProvider` 接口读取引擎配置，解耦对 service 层 `EngineConfigService` 的直接依赖。

```go
type EngineConfigProvider interface {
    FallbackAssigneeID() uint
}
```

#### Scenario: SmartEngine 通过接口读取兜底配置
- **WHEN** SmartEngine 在 `executeDecisionPlan` 中需要读取 fallback 配置
- **THEN** 系统 SHALL 通过注入的 `EngineConfigProvider` 接口调用 `FallbackAssigneeID()` 获取配置值

#### Scenario: EngineConfigProvider 未注入时返回 0
- **WHEN** SmartEngine 构造时未注入 `EngineConfigProvider`（nil）
- **THEN** 系统 SHALL 将 `FallbackAssigneeID` 视为 0（未配置），不触发兜底逻辑

### Requirement: BDD 场景覆盖

系统 SHALL 提供 BDD 场景验证智能引擎参与者兜底行为。

#### Scenario: 配置兜底人后缺失参与者工单自动转派
- **WHEN** 引擎配置了兜底处理人，且工单使用缺失参与者的工作流
- **THEN** 智能引擎决策循环完成后，工单 SHALL 有 assignment 指向兜底处理人，Timeline 包含 `participant_fallback` 事件

#### Scenario: 参与者完整时正常路由并完成全流程
- **WHEN** 工单使用参与者完整的工作流
- **THEN** 智能引擎 SHALL 正常分配参与者，工单可走完审批→完成的全链路
