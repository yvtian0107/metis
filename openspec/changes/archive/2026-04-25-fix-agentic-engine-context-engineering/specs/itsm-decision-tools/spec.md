## MODIFIED Requirements

### Requirement: decision.ticket_context 工具
该工具 SHALL 返回工单的完整上下文信息，包括表单数据、SLA 状态、活动历史和并签组状态。活动历史中每个活动 SHALL 通过 `activityFactMap` 返回，包含活动级 `form_data`。

参数：无（工具执行时从 ReAct 循环上下文获取 ticketID）

返回字段：
- `form_data`: 工单级完整表单 JSON
- `description`: 工单详细描述
- `sla_status`: SLA 剩余时间，无 SLA 时为 null
- `activity_history`: 已完成活动列表（每条通过 activityFactMap 生成，包含 form_data）
- `completed_activity`: 当前完成的活动详细信息（通过 activityFactMap 生成，包含 form_data）
- `workflow_context`: 工作流上下文（通过 buildWorkflowContext 生成，包含 related_step、approved/rejected 出边目标）
- `current_activities`: 当前活跃活动列表
- `parallel_groups`: 活跃并签组状态

#### Scenario: activity_history 包含活动级 form_data
- **WHEN** Agent 调用 `decision.ticket_context` 且活动历史中某活动的 form_data 非空
- **THEN** 该活动在 `activity_history` 中的条目 SHALL 包含 `form_data` 字段

#### Scenario: completed_activity 包含 form_data
- **WHEN** Agent 调用 `decision.ticket_context` 且 completed activity 有 form_data（如表单提交后被驳回的场景）
- **THEN** `completed_activity` SHALL 包含 `form_data` 字段，Agent 可据此了解"上次提交了什么"

#### Scenario: workflow_context 包含 approved 出边目标
- **WHEN** Agent 调用 `decision.ticket_context` 且 completed activity 为通过，NodeID 有效
- **THEN** `workflow_context.related_step` SHALL 包含 `approved_edge_target` 信息

#### Scenario: workflow_context 包含 rejected 出边目标
- **WHEN** Agent 调用 `decision.ticket_context` 且 completed activity 为驳回，NodeID 有效
- **THEN** `workflow_context.related_step` SHALL 包含 `rejected_edge_target` 信息

#### Scenario: 查询含并签组的工单上下文
- **WHEN** Agent 调用 `decision.ticket_context` 且工单有一个活跃的并签组（2 活动，1 已完成）
- **THEN** 返回结果 SHALL 包含 `parallel_groups` 字段，其中 `total=2, completed=1, pending_activities` 列出未完成活动

#### Scenario: seed 与 ticket_context 不重复
- **WHEN** Agent 在 ReAct 循环中先收到 seed 再调用 `ticket_context`
- **THEN** seed 中的 `completed_activity` 仅含轻量锚点（id、outcome、operator_opinion），`ticket_context` 返回完整事实，两者 SHALL NOT 包含相同粒度的重复信息
