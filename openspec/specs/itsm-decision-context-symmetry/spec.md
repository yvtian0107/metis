## Purpose

ITSM SmartEngine 决策上下文对称化 -- approved/rejected 路径对称注入、活动级 form_data 透出、seed 去重。

## Requirements

### Requirement: Approved 路径上下文注入
当已完成的 DecisionActivity outcome 为正面且 NodeID 有效时，buildInitialSeed SHALL 注入 `approved_next_step` 字段，包含：target_node_id, target_node_label, target_node_type, instruction。instruction SHALL 包含强约束语句 `应遵循此路径继续推进` 以及语义说明。该约束与 `rejected_activity_policy` 中的“必须遵循此路径”对称。`approved_next_step` 和 `rejected_activity_policy` SHALL 互斥。

#### Scenario: 通过且有 approved 出边
- **WHEN** 已完成活动 outcome=positive 且 NodeID 有效指向一个有出边的节点
- **THEN** seed 包含 `approved_next_step`
- **AND** 其 instruction 含 `应遵循此路径继续推进`

#### Scenario: 通过但 NodeID 为空
- **WHEN** 已完成活动 outcome=positive 但 NodeID 为空
- **THEN** seed 不包含 `approved_next_step`

#### Scenario: 通过且 approved 出边目标是 end 节点
- **WHEN** 已完成活动 outcome=positive 且 approved 出边目标节点 `activity_kind=end`
- **THEN** `approved_next_step.instruction` 包含“目标是结束节点，流程即将终结”

#### Scenario: 与 rejected_activity_policy 互斥
- **WHEN** 已完成活动 outcome=positive
- **THEN** seed 包含 `approved_next_step` 但不包含 `rejected_activity_policy`

### Requirement: buildWorkflowContext approved 出边透出
`buildWorkflowContext` SHALL 在 completed activity outcome 为正向且 NodeID 有效时，在 `related_step` 中附加 `approved_edge_target` 信息。

#### Scenario: related_step 附加 approved 出边目标
- **WHEN** `buildWorkflowContext` 处理 approved activity（NodeID="node_4"），node_4 有 approved 出边指向 node_5
- **THEN** `related_step` SHALL 额外包含 `approved_edge_target` 字段：`{"node_id": "node_5", "label": "IT主管处理", "type": "process"}`

#### Scenario: rejected activity 的 related_step 附加 rejected 出边目标
- **WHEN** `buildWorkflowContext` 处理 rejected activity（NodeID="node_4"），node_4 有 rejected 出边指向 node_end
- **THEN** `related_step` SHALL 额外包含 `rejected_edge_target` 字段：`{"node_id": "node_end", "label": "end", "type": "end"}`

### Requirement: Activity form_data 透出
`activityFactMap` SHALL 返回活动级 form_data，让 Agent 在驳回恢复场景能看到上次提交的内容。

#### Scenario: 活动有 form_data
- **WHEN** `activityFactMap` 处理一个 form_data 非空的 activity
- **THEN** 返回的 map SHALL 包含 `form_data` 字段，值为 JSON 解析后的表单数据

#### Scenario: 活动无 form_data
- **WHEN** `activityFactMap` 处理一个 form_data 为空的 activity
- **THEN** 返回的 map SHALL NOT 包含 `form_data` 字段

### Requirement: Seed 去重
`buildInitialSeed` SHALL 将 `completed_activity` 精简为轻量锚点（id + outcome + operator_opinion），不再通过 `activityFactMap` 生成完整事实。完整事实由 `ticket_context` 工具按需提供。

#### Scenario: seed 中的 completed_activity 为轻量锚点
- **WHEN** `buildInitialSeed` 构建 seed 且有 completedActivityID
- **THEN** `seed["completed_activity"]` SHALL 仅包含 `id`、`outcome`、`operator_opinion` 三个字段
- **AND** SHALL NOT 包含 `type`、`name`、`status`、`participants`、`source_decision`、`ai_reasoning` 等完整事实字段

#### Scenario: rejected_activity_policy 保留在 seed
- **WHEN** completed activity 被驳回
- **THEN** `seed["rejected_activity_policy"]` SHALL 仍包含完整的策略指令（must_explain_rejection、forbidden_path、workflow_rejected_target 或 allowed_recovery_paths）

#### Scenario: approved_next_step 保留在 seed
- **WHEN** completed activity 为通过
- **THEN** `seed["approved_next_step"]` SHALL 仍包含完整的路径引导信息

#### Scenario: ticket_context 工具仍返回完整 completed_activity
- **WHEN** Agent 调用 `ticket_context` 工具
- **THEN** 返回的 `completed_activity` SHALL 仍通过 `activityFactMap` 生成，包含所有详细字段（包括新增的 form_data）
