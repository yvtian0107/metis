## ADDED Requirements

### Requirement: 确定性覆盖全部决策类型的 activity 创建

SmartEngine.ExecuteConfirmedPlan SHALL 正确创建 7 种决策类型（approve / process / action / notify / form / complete / escalate）的 TicketActivity 记录，并在 timeline 记录 `ai_decision_executed` 事件。

#### Scenario: process 类型决策创建处理活动
- **WHEN** 执行 crafted DecisionPlan（type=process, participant_id 指向有效用户）
- **THEN** 创建 status=pending 的 TicketActivity（activity_type=process）
- **AND** 创建 TicketAssignment 指向该用户
- **AND** ticket.assignee_id 更新为该用户

#### Scenario: action 类型决策创建自动动作活动
- **WHEN** 执行 crafted DecisionPlan（type=action, action_id 指向有效 ServiceAction）
- **THEN** 创建 status=in_progress 的 TicketActivity（activity_type=action）
- **AND** 不创建 TicketAssignment（action 无需参与者）

#### Scenario: notify 类型决策创建通知活动
- **WHEN** 执行 crafted DecisionPlan（type=notify）
- **THEN** 创建 status=in_progress 的 TicketActivity（activity_type=notify）

#### Scenario: form 类型决策创建表单填写活动
- **WHEN** 执行 crafted DecisionPlan（type=form, participant_id 指向有效用户）
- **THEN** 创建 status=pending 的 TicketActivity（activity_type=form）
- **AND** 创建 TicketAssignment 指向该用户

#### Scenario: escalate 类型决策创建升级活动
- **WHEN** 执行 crafted DecisionPlan（type=escalate）
- **THEN** 创建 status=in_progress 的 TicketActivity（activity_type=escalate）

#### Scenario: complete 类型决策直接完结工单
- **WHEN** 执行 crafted DecisionPlan（next_step_type=complete）
- **THEN** 工单 status 变为 completed
- **AND** 创建 activity_type=complete 的已完成活动
- **AND** timeline 包含 workflow_completed 事件

### Requirement: AI 决策失败递增 failure count

当 AI 决策失败时，SmartEngine SHALL 递增 ticket.ai_failure_count 并记录 `ai_decision_failed` timeline 事件。

#### Scenario: 单次决策失败后 failure count 变为 1
- **WHEN** 智能引擎决策失败（LLM 不可达或返回非法输出）
- **THEN** ticket.ai_failure_count 从 0 变为 1
- **AND** timeline 包含 ai_decision_failed 事件

### Requirement: 连续失败触发 AI 熔断

当 ticket.ai_failure_count 达到 MaxAIFailureCount (3) 时，SmartEngine SHALL 拒绝执行新的决策循环，记录 `ai_disabled` timeline 事件，并返回 ErrAIDisabled。

#### Scenario: ai_failure_count 已达 3 时决策循环直接拒绝
- **WHEN** ticket.ai_failure_count = 3 时执行决策循环
- **THEN** 返回 ErrAIDisabled
- **AND** timeline 包含 ai_disabled 事件
- **AND** 工单状态不变（不会变为 failed）

### Requirement: Cancel 取消智能引擎工单

SmartEngine.Cancel SHALL 取消工单所有活跃活动、取消待处理 assignment、将工单状态设为 cancelled，并记录 timeline。

#### Scenario: 取消有活跃审批活动的智能工单
- **WHEN** 工单有一个 status=pending 的审批活动
- **AND** 执行 SmartEngine.Cancel
- **THEN** 该活动 status 变为 cancelled
- **AND** 关联 assignment status 变为 cancelled
- **AND** 工单 status 变为 cancelled
- **AND** timeline 包含取消事件

### Requirement: 低置信度决策被人工拒绝

当管理员拒绝 pending_approval 的决策时，activity 状态 SHALL 变为 rejected，决策不执行。

#### Scenario: 管理员拒绝低置信度决策
- **WHEN** 存在 status=pending_approval 的活动
- **AND** 管理员将其标记为 rejected
- **THEN** 活动 status 变为 rejected
- **AND** 工单状态不变为 completed
- **AND** timeline 包含决策拒绝事件

### Requirement: 兜底用户无效时记录 warning

当 fallback assignee 配置了但该用户不存在或未激活时，tryFallbackAssignment SHALL 记录 `participant_fallback_warning` timeline 事件，不创建 assignment。

#### Scenario: 兜底用户已停用时记录 warning 而非分配
- **WHEN** 引擎配置兜底处理人为一个 is_active=false 的用户
- **AND** 执行无参与者的审批决策
- **THEN** 不创建 TicketAssignment
- **AND** ticket.assignee_id 不变
- **AND** timeline 包含 participant_fallback_warning 事件
