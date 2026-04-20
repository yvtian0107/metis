## MODIFIED Requirements

### Requirement: Smart 工单详情页 6 种 UI 状态渲染
Smart 引擎工单详情页 SHALL 在基本信息区域下方渲染一个"当前活动卡片"（SmartCurrentActivityCard），根据以下优先级判定 UI 状态并渲染对应内容：

1. **终态**：`ticket.status ∈ {completed, cancelled}` → 只读回顾卡片，展示最终 AI 推理和处理时长
2. **AI 停用**：`ticket.aiFailureCount >= 3` → 暂停卡片，展示失败原因和覆盖操作按钮
3. **AI 决策待确认**：当前活动 `status = pending_approval` 且当前用户对该活动有确认/拒绝权限 → AI 决策面板 + 确认/拒绝按钮
4. **人工活动等待处理**：当前活动 `status ∈ {pending, in_progress}` 且 `activityType ∈ {approve, form, process}` → 表单数据展示 + 操作按钮（按用户身份分支）
5. **AI 推理中**：无 pending/pending_approval 活动且 ticket 非终态 → loading 状态 + 自动轮询

#### Scenario: 工单处于 AI 推理中
- **WHEN** Smart 工单的 ticket.status = in_progress 且无 pending/pending_approval 活动
- **THEN** 详情页展示"AI 正在分析"loading 卡片，启用 3s 轮询自动刷新 ticket 数据

#### Scenario: 轮询超时
- **WHEN** AI 推理中状态持续超过 60s
- **THEN** 停止轮询并展示"AI 推理超时，请手动刷新"提示

#### Scenario: AI 决策待确认
- **WHEN** 当前活动 status = pending_approval 且 aiDecision 非空
- **AND** 当前用户对该活动有确认/拒绝权限
- **THEN** 展示 AI 决策面板（推荐的下一步、置信度进度条、推理过程）和 [确认 AI 决策] [拒绝 AI 决策] 按钮

#### Scenario: 无权限用户只读查看 AI 待确认
- **WHEN** 当前活动 status = pending_approval 且 aiDecision 非空
- **AND** 当前用户对该活动没有确认/拒绝权限
- **THEN** 页面 SHALL 不展示可操作的 [确认 AI 决策] [拒绝 AI 决策] 按钮

#### Scenario: 人工活动等待处理 - 我是处理人
- **WHEN** 当前活动为人工活动，且 assignment.assigneeId 等于当前用户
- **THEN** 展示表单数据和操作按钮（approve 类型显示 [通过] [驳回]，form/process 类型显示 [提交]）

#### Scenario: 人工活动等待处理 - 我不是处理人
- **WHEN** 当前活动为人工活动，且 assignment.assigneeId 不等于当前用户
- **THEN** 展示表单数据（只读），不显示操作按钮

#### Scenario: AI 停用
- **WHEN** ticket.aiFailureCount >= 3
- **THEN** 展示警告卡片"AI 决策已停用"，显示 [重试 AI] [手动跳转] [重新分配] 按钮

#### Scenario: 终态
- **WHEN** ticket.status = completed 或 cancelled
- **THEN** 展示只读回顾卡片，无操作按钮

### Requirement: AI 决策拒绝后内联暂停引导
当用户拒绝 AI 决策后，详情页 SHALL 自动刷新并渲染暂停态卡片，展示被拒绝的建议信息和 3 个覆盖操作入口（重试 AI / 手动指定下一步 / 指派处理人）。拒绝操作仅对授权用户展示且仅允许授权用户提交。

#### Scenario: 拒绝后展示暂停卡片
- **WHEN** 授权用户点击 [拒绝 AI 决策] 并成功
- **THEN** 页面自动刷新，展示暂停态卡片，包含被拒绝的建议（活动名 + 置信度）、拒绝人和时间、3 个覆盖操作按钮

#### Scenario: 未授权用户无法提交拒绝
- **WHEN** 未授权用户尝试对 AI 待确认活动执行拒绝操作
- **THEN** 页面 SHALL 不提供正常操作入口
- **AND** 即使直接请求接口也 SHALL 失败
