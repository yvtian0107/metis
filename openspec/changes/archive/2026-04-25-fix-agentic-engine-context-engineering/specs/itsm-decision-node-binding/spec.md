## ADDED Requirements

### Requirement: DecisionActivity node_id 字段
DecisionActivity 结构体 SHALL 新增 `NodeID string` 字段（JSON key: `node_id`，omitempty）。该字段表示 Agent 声明该活动对应的 workflow_json 节点 ID。

#### Scenario: Agent 输出含 node_id 的 DecisionPlan
- **WHEN** Agent 输出的 JSON 中 activities 条目包含 `"node_id": "node_4"`
- **THEN** `parseDecisionPlan()` SHALL 正确解析该字段到 `DecisionActivity.NodeID`

#### Scenario: node_id 缺失向后兼容
- **WHEN** Agent 输出的 JSON 中 activities 条目不包含 `node_id` 字段
- **THEN** `parseDecisionPlan()` SHALL 将 `NodeID` 默认为空字符串，不影响后续执行

### Requirement: Activity 创建时写入 NodeID
`executeSinglePlan`、`executeParallelPlan`、`pendManualHandlingPlan` 三个活动创建站点 SHALL 将 `DecisionActivity.NodeID` 写入 `activityModel.NodeID`。

#### Scenario: executeSinglePlan 写入 NodeID
- **WHEN** `executeSinglePlan` 创建活动且 `da.NodeID` 为 `"node_4"`
- **THEN** 创建的 `activityModel.NodeID` SHALL 为 `"node_4"`

#### Scenario: executeParallelPlan 写入 NodeID
- **WHEN** `executeParallelPlan` 创建并签活动组且各 activity 的 `da.NodeID` 分别为 `"node_5"` 和 `"node_6"`
- **THEN** 创建的各 `activityModel.NodeID` SHALL 分别为 `"node_5"` 和 `"node_6"`

#### Scenario: pendManualHandlingPlan 写入 NodeID
- **WHEN** `pendManualHandlingPlan` 创建低置信活动且 plan 中第一个 activity 的 `da.NodeID` 为 `"node_3"`
- **THEN** 创建的 `activityModel.NodeID` SHALL 为 `"node_3"`

#### Scenario: NodeID 为空时不阻断
- **WHEN** 活动创建时 `da.NodeID` 为空字符串
- **THEN** 创建 SHALL 正常完成，`activityModel.NodeID` 为空（与当前行为一致）

### Requirement: node_id 路径合规验证
`validatePlan()` SHALL 在有 workflow_json 时对 DecisionActivity 的 node_id 做合规检查。

#### Scenario: node_id 在 workflow_json 中存在且类型匹配
- **WHEN** `validatePlan()` 检查一个 node_id="node_4" 的 activity，workflow_json 中 node_4 类型为 process，activity type 为 process
- **THEN** 验证 SHALL 通过，node_id 保留

#### Scenario: node_id 在 workflow_json 中不存在
- **WHEN** `validatePlan()` 检查一个 node_id="node_99" 的 activity，workflow_json 中无此节点
- **THEN** 验证 SHALL 将 node_id 清空为空字符串并记录 warning 日志，不阻断计划执行

#### Scenario: node_id 节点类型不匹配
- **WHEN** `validatePlan()` 检查一个 node_id="node_2" 的 activity（type=process），workflow_json 中 node_2 类型为 form
- **THEN** 验证 SHALL 将 node_id 清空为空字符串并记录 warning 日志，不阻断计划执行

#### Scenario: 无 workflow_json 时跳过检查
- **WHEN** `validatePlan()` 执行时服务定义无 workflow_json
- **THEN** node_id 合规检查 SHALL 被跳过，所有 node_id 值保留原样

### Requirement: buildWorkflowContext 利用 NodeID 输出 related_step
`buildWorkflowContext` SHALL 在 completed activity 有有效 NodeID 时，输出精确的 `related_step`（包含节点详情和出边信息），不再输出 fallback note。

#### Scenario: NodeID 有效时输出 related_step
- **WHEN** `buildWorkflowContext` 接收到 completed activity 且 NodeID="node_4"，workflow_json 中 node_4 存在
- **THEN** 返回的 ctx SHALL 包含 `related_step` 字段，值为 node_4 的完整节点事实（id、type、label、participants、outgoing_edges）

#### Scenario: NodeID 为空时输出 fallback note
- **WHEN** `buildWorkflowContext` 接收到 completed activity 且 NodeID 为空
- **THEN** 返回的 ctx SHALL 包含 `related_step_note` 字段（与当前行为一致）

### Requirement: findRejectedEdgeTarget 利用 NodeID 生效
`findRejectedEdgeTarget` SHALL 在 completed activity 有有效 NodeID 时，从 workflow_json 中查找 rejected 出边的目标节点。

#### Scenario: NodeID 有效且有 rejected 出边
- **WHEN** `buildInitialSeed` 处理一个 rejected activity（NodeID="node_4"），workflow_json 中 node_4 有 outcome=rejected 的出边指向 node_end_rejected
- **THEN** `rejected_activity_policy` SHALL 包含 `workflow_rejected_target: "node_end_rejected"` 和精确的路径指令

#### Scenario: NodeID 为空时使用 fallback 路径
- **WHEN** `buildInitialSeed` 处理一个 rejected activity（NodeID 为空）
- **THEN** `rejected_activity_policy` SHALL 使用 `allowed_recovery_paths` 列表（与当前行为一致）
