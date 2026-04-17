## ADDED Requirements

### Requirement: Smart engine generates valid decision for network support

智能引擎 SHALL 为 `request_kind=network_support` 的 VPN 申请调用真实 LLM 生成合法的 DecisionPlan。

#### Scenario: 智能引擎为网络支持请求生成合法决策
- **WHEN** 申请人创建 VPN 工单（访问原因 network_support）并触发智能引擎决策循环
- **THEN** 工单状态不为 "failed"
- **THEN** 存在至少一个活动
- **THEN** 活动类型在 AllowedSmartStepTypes 内
- **THEN** 决策置信度 ∈ [0, 1]
- **THEN** 若指定了参与人则 participant_id 在 policy.ParticipantCandidates 内
- **THEN** 时间线包含 AI 决策相关事件

### Requirement: Smart engine generates valid decision for security compliance

智能引擎 SHALL 为 `request_kind=external_collaboration` 的 VPN 申请生成合法的 DecisionPlan。

#### Scenario: 智能引擎为安全合规请求生成合法决策
- **WHEN** 申请人创建 VPN 工单（访问原因 external_collaboration）并触发智能引擎决策循环
- **THEN** 工单状态不为 "failed"
- **THEN** 存在至少一个活动
- **THEN** 活动类型在 AllowedSmartStepTypes 内
- **THEN** 若指定了参与人则 participant_id 在 policy.ParticipantCandidates 内

### Requirement: Low confidence decision enters pending_approval

当 DecisionPlan.Confidence 低于 confidence_threshold 时，智能引擎 SHALL 创建 pending_approval 状态的活动，等待人工确认后再执行。

#### Scenario: 低置信度决策进入待确认状态后人工确认执行
- **WHEN** 申请人创建 VPN 工单（confidence_threshold 设为 0.99）并触发智能引擎决策循环
- **THEN** 工单状态为 "in_progress"
- **THEN** 当前活动状态为 "pending_approval"
- **THEN** 活动记录中包含 AI 推理说明（AIReasoning 非空）
- **WHEN** 管理员确认该待确认决策
- **THEN** 当前活动状态不为 "pending_approval"

### Requirement: Smart engine handles missing participant gracefully

当工作流 approval 节点缺失 participant_type 时，智能引擎 SHALL 安全兜底（生成 escalate/complete 决策），不 SHALL panic 或导致工单 failed。

#### Scenario: 审批节点缺失参与者时智能引擎安全兜底
- **WHEN** 申请人创建使用缺失参与者工作流的 VPN 工单并触发智能引擎决策循环
- **THEN** 工单状态不为 "failed"
- **THEN** 时间线包含 AI 决策相关事件

### Requirement: Smart engine complete e2e cycle

智能引擎 SHALL 支持完整链路：创建工单 → AI 决策生成活动 → 审批通过 → AI 再次决策 → 工单完成。

#### Scenario: 智能引擎完整链路 — 决策 → 审批 → 完成
- **WHEN** 申请人创建 VPN 工单并触发智能引擎决策循环
- **THEN** 工单状态为 "in_progress" 且存在至少一个活动
- **WHEN** 当前活动的被分配人认领并审批通过，然后智能引擎再次执行决策循环
- **THEN** 工单状态为 "completed"
