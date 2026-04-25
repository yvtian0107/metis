## MODIFIED Requirements

### Requirement: DecisionPlan 并签字段
DecisionPlan 结构 SHALL 新增 `ExecutionMode string` 字段（JSON key: `execution_mode`）。合法值为 `""` / `"single"` / `"parallel"`。空值或 `"single"` 表示串行（现有行为），`"parallel"` 表示并签。

DecisionActivity 结构体 SHALL 同时包含 `NodeID string` 字段（JSON key: `node_id`，omitempty），由 Agent 声明该活动对应的 workflow_json 节点 ID。

#### Scenario: 解析含 execution_mode 的 DecisionPlan
- **WHEN** Agent 输出的 JSON 包含 `"execution_mode": "parallel"`
- **THEN** `parseDecisionPlan()` SHALL 正确解析该字段到 `DecisionPlan.ExecutionMode`

#### Scenario: execution_mode 缺失向后兼容
- **WHEN** Agent 输出的 JSON 不包含 `execution_mode` 字段
- **THEN** `parseDecisionPlan()` SHALL 将 `ExecutionMode` 默认为空字符串，等同 `"single"`

#### Scenario: 解析含 node_id 的 DecisionActivity
- **WHEN** Agent 输出的 JSON 中 activities 条目包含 `"node_id": "node_4"`
- **THEN** `parseDecisionPlan()` SHALL 正确解析该字段到 `DecisionActivity.NodeID`

#### Scenario: node_id 缺失向后兼容
- **WHEN** Agent 输出的 JSON 中 activities 条目不包含 `node_id` 字段
- **THEN** `DecisionActivity.NodeID` SHALL 默认为空字符串

### Requirement: executeDecisionPlan 并签分支
`executeDecisionPlan()` SHALL 在 `ExecutionMode == "parallel"` 时创建并签活动组，而非逐个覆盖 `current_activity_id`。并行计划中的 action 类型活动 SHALL 被正确调度执行。所有活动创建站点 SHALL 将 `DecisionActivity.NodeID` 写入 `activityModel.NodeID`。

#### Scenario: parallel 模式创建活动组并写入 NodeID
- **WHEN** `executeDecisionPlan()` 处理 `ExecutionMode == "parallel"` 的 DecisionPlan，activities 各有 node_id
- **THEN** SHALL 为每个条目创建独立 TicketActivity，设置相同的 `activity_group_id`
- **AND** 每个 `activityModel.NodeID` SHALL 等于对应 `DecisionActivity.NodeID`

#### Scenario: single 模式只创建第一个 activity 并写入 NodeID
- **WHEN** `executeDecisionPlan()` 处理 single 模式 DecisionPlan，第一个 activity 有 node_id="node_4"
- **THEN** SHALL 只创建第一个 activity 并设为 current
- **AND** `activityModel.NodeID` SHALL 为 `"node_4"`

#### Scenario: pendManualHandlingPlan 写入 NodeID
- **WHEN** `pendManualHandlingPlan` 创建低置信活动且 plan 中第一个 activity 的 node_id 为 "node_3"
- **THEN** `activityModel.NodeID` SHALL 为 `"node_3"`

### Requirement: buildInitialSeed 上下文对称化
`buildInitialSeed` SHALL 在 completed activity outcome 为正向时注入 `approved_next_step`（对称于 `rejected_activity_policy`），并将 `completed_activity` 精简为轻量锚点。

#### Scenario: approved 路径注入
- **WHEN** `buildInitialSeed` 处理通过的 activity（NodeID 有效），workflow_json 有 approved 出边
- **THEN** seed SHALL 包含 `approved_next_step`（target_node_id、target_node_label、target_node_type、instruction）

#### Scenario: rejected 路径注入（现有行为保留）
- **WHEN** `buildInitialSeed` 处理驳回的 activity（NodeID 有效），workflow_json 有 rejected 出边
- **THEN** seed SHALL 包含 `rejected_activity_policy`（workflow_rejected_target、instruction）

#### Scenario: completed_activity 精简为锚点
- **WHEN** `buildInitialSeed` 构建 seed 且有 completedActivityID
- **THEN** `seed["completed_activity"]` SHALL 仅包含 `id`、`outcome`、`operator_opinion`
