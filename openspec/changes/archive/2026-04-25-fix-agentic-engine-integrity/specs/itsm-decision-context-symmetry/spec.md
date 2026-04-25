## MODIFIED Requirements

### Requirement: Approved 路径上下文注入
当已完成的 DecisionActivity outcome 为正面且 NodeID 有效时，buildInitialSeed SHALL 注入 approved_next_step 字段，包含：target_node_id, target_node_label, target_node_type, instruction。instruction SHALL 包含强约束语句 "应遵循此路径继续推进" 以及语义说明（如 "目标是结束节点，流程即将终结"）。该约束与 rejected_activity_policy 中的 "必须遵循此路径" 对称。approved_next_step 和 rejected_activity_policy SHALL 互斥。

#### Scenario: 通过且有 approved 出边
- **WHEN** 已完成活动 outcome=positive 且 NodeID 有效指向一个有出边的节点
- **THEN** seed 包含 approved_next_step，其 instruction 含 "应遵循此路径继续推进"

#### Scenario: 通过但 NodeID 为空
- **WHEN** 已完成活动 outcome=positive 但 NodeID 为空
- **THEN** seed 不包含 approved_next_step（回退到无路径引导）

#### Scenario: 通过且 approved 出边目标是 end 节点
- **WHEN** 已完成活动 outcome=positive 且出边目标节点 activity_kind=end
- **THEN** approved_next_step.instruction 包含 "目标是结束节点，流程即将终结"

#### Scenario: 与 rejected_activity_policy 互斥
- **WHEN** 已完成活动 outcome=positive
- **THEN** seed 包含 approved_next_step 但不包含 rejected_activity_policy
