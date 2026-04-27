## MODIFIED Requirements

### Requirement: 工单数据模型

系统 SHALL 使用统一的工单数据模型，支持经典和智能两种引擎。

Ticket 模型：code（唯一编号如 TICK-000001）、title、description、service_id（FK→ServiceDefinition）、engine_type（继承自服务）、status（产品语义状态枚举）、outcome（终态结果枚举，可空）、priority_id（FK→Priority）、requester_id（FK→User）、assignee_id（FK→User，可选）、current_activity_id（FK→TicketActivity，可选）、source（"catalog"|"agent"）、agent_session_id（uint，可选）、ai_failure_count、form_data（JSON）、workflow_json（JSON）、sla_response_deadline（时间，可选）、sla_resolution_deadline（时间，可选）、sla_status（"on_track"|"breached_response"|"breached_resolution"）、finished_at（时间，可选），嵌入 BaseModel。

TicketActivity 模型：ticket_id、name、activity_type（"form"|"approve"|"process"|"action"|"end"|"complete"）、status（"pending"|"in_progress"|"approved"|"rejected"|"completed"|"cancelled"|"failed"|"blocked"）、node_id（字符串，引用 workflow_json 节点 ID）、execution_mode（"single"|"parallel"|"serial"）、activity_group_id、form_schema（JSON）、form_data（JSON）、transition_outcome（"approved"|"rejected"|"completed"|"success"|"failed"|"timeout"）、ai_decision（JSON，智能模式）、ai_reasoning（文本）、ai_confidence（float）、overridden_by（uint，被人工覆盖时记录操作人 ID）、decision_reasoning（文本）、started_at、finished_at，嵌入 BaseModel。人工活动同意/驳回后 status SHALL 直接为 `approved` 或 `rejected`。

TicketAssignment 模型：ticket_id、activity_id、participant_type（"user"|"requester"|"requester_manager"|"position"|"department"|"position_department"）、user_id（指定人时）、position_id（指定岗位时）、department_id（指定部门时）、assignee_id（实际认领人）、status（"pending"|"in_progress"|"approved"|"rejected"|"transferred"|"delegated"|"claimed_by_other"|"cancelled"|"failed"）、sequence（并行/串行的顺序）、is_current、claimed_at、finished_at，嵌入 BaseModel。

TicketTimeline 模型：ticket_id、activity_id（可选）、operator_id（FK→User）、event_type（枚举）、message、details（JSON）、reasoning（文本），嵌入 BaseModel。

TicketActionExecution 模型：ticket_id、activity_id、service_action_id、status（"pending"|"success"|"failed"）、request_payload（JSON）、response_payload（JSON）、failure_reason、retry_count，嵌入 BaseModel。

TicketLink 模型：parent_ticket_id、child_ticket_id、link_type（"related"|"caused_by"|"blocked_by"），嵌入 BaseModel。

PostMortem 模型：ticket_id（唯一）、root_cause、impact_summary、action_items（JSON 数组）、lessons_learned、created_by，嵌入 BaseModel。

#### Scenario: 模型自动迁移
- **WHEN** ITSM App 的 Models() 被调用
- **THEN** 返回上述所有模型，main.go 自动 AutoMigrate

#### Scenario: 工单结果字段迁移
- **WHEN** 系统迁移旧工单表
- **THEN** Ticket 模型 SHALL 包含 outcome 字段
- **AND** 旧终态工单 SHALL 根据历史活动和时间线派生 outcome

### Requirement: 工单状态枚举

工单状态 SHALL 包含以下值：submitted（已提交）、waiting_human（待人工处理）、approved_decisioning（已同意，决策中）、rejected_decisioning（已驳回，决策中）、decisioning（AI 决策中）、executing_action（自动执行中）、completed（已通过或已履约）、rejected（已驳回终止）、withdrawn（已撤回）、cancelled（已取消）、failed（失败）。

#### Scenario: 初始状态
- **WHEN** 工单创建
- **THEN** 状态为 submitted

#### Scenario: 终态不可变
- **WHEN** 工单状态为 completed、rejected、withdrawn、failed 或 cancelled
- **THEN** 不允许再变更业务状态（除管理员明确执行恢复操作）

#### Scenario: 同意后进入决策中
- **WHEN** 智能工单人工活动被同意
- **THEN** 工单状态 SHALL 变更为 approved_decisioning

#### Scenario: 驳回后进入决策中
- **WHEN** 智能工单人工活动被驳回
- **THEN** 工单状态 SHALL 变更为 rejected_decisioning
