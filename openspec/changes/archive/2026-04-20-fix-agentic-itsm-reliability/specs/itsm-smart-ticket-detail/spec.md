## MODIFIED Requirements

### Requirement: Smart 工单详情页 6 种 UI 状态渲染
Smart 引擎工单详情页 SHALL 在基本信息区域下方渲染一个"当前活动卡片"（SmartCurrentActivityCard），根据以下优先级判定 UI 状态并渲染对应内容：

1. **终态**：`ticket.status ∈ {completed, cancelled}` → 只读回顾卡片，展示最终 AI 推理和处理时长
2. **AI 停用**：`ticket.aiFailureCount >= 3` → 暂停卡片，展示失败原因和覆盖操作按钮
3. **AI 决策待确认**：当前活动 `status = pending_approval` → AI 决策面板 + 确认/拒绝按钮
4. **人工活动等待处理**：当前活动 `status ∈ {pending, in_progress}` 且 `activityType ∈ {approve, form, process}` → 表单数据展示 + 操作按钮（按用户身份分支）
5. **AI 推理中**：无 pending/pending_approval 活动且 ticket 非终态 → loading 状态 + 自动轮询
6. **空闲过渡**：有活跃活动但不匹配人工类型且非 AI 推理中 → 显示"AI 正在准备下一步"提示

#### Scenario: 工单处于 AI 推理中
- **WHEN** Smart 工单的 ticket.status = in_progress 且无 pending/pending_approval 活动
- **THEN** 详情页展示"AI 正在分析"loading 卡片，启用 3s 轮询自动刷新 ticket 数据

#### Scenario: 轮询超时
- **WHEN** AI 推理中状态持续超过 60s
- **THEN** 停止轮询并展示"AI 推理超时，请手动刷新"提示

#### Scenario: idle 过渡态
- **WHEN** 状态机判定为 idle（有活跃活动但不匹配已知人工类型）
- **THEN** SHALL 显示"AI 正在准备下一步"提示卡片，而非渲染空白

#### Scenario: 人工活动等待处理 - 无 assignee
- **WHEN** 当前活动为人工活动且无 assignee
- **THEN** SHALL 显示"等待分配处理人"提示信息

#### Scenario: AI 决策待确认
- **WHEN** 当前活动 status = pending_approval 且 aiDecision 非空
- **THEN** 展示 AI 决策面板（推荐的下一步、置信度进度条、推理过程）和 [确认 AI 决策] [拒绝 AI 决策] 按钮

#### Scenario: 人工活动等待处理 - 我是处理人
- **WHEN** 当前活动为人工活动，且 assignment.assigneeId 等于当前用户
- **THEN** 展示表单数据和操作按钮（approve 类型显示 [通过] [驳回]，form/process 类型显示 [提交]）
- **AND** form/process 类型的提交 outcome SHALL 使用 `"completed"` 而非 `"submitted"`

#### Scenario: AI 停用
- **WHEN** ticket.aiFailureCount >= 3
- **THEN** 展示警告卡片"AI 决策已停用"，显示 [重试 AI] [手动跳转] [重新分配] 按钮

#### Scenario: 终态
- **WHEN** ticket.status = completed 或 cancelled
- **THEN** 展示只读回顾卡片，无操作按钮

### Requirement: AI 决策拒绝确认弹窗
拒绝 AI 决策 SHALL 弹出确认对话框，要求用户确认操作。

#### Scenario: 用户点击拒绝
- **WHEN** 用户在 AI 决策面板点击 [拒绝 AI 决策]
- **THEN** SHALL 弹出确认对话框，包含被拒绝决策的摘要（活动名、置信度）
- **AND** 对话框 SHALL 提供"确认拒绝"和"取消"两个按钮
- **AND** 仅在用户点击"确认拒绝"后才执行 reject API 调用

### Requirement: Override 操作权限校验
Override 操作（jump、reassign、retry-ai）SHALL 在后端进行权限校验，前端根据权限决定是否展示。

#### Scenario: 无权限用户不可见 override
- **WHEN** 当前用户无 `itsm:ticket:override` 权限
- **THEN** 详情页 SHALL 不渲染 OverrideActions 下拉菜单

#### Scenario: 有权限用户可见 override
- **WHEN** 当前用户有 `itsm:ticket:override` 权限且工单为活跃的 smart 工单
- **THEN** 详情页 SHALL 渲染 OverrideActions 下拉菜单
- **AND** Retry AI 选项 SHALL 仅在 aiFailureCount > 0 时显示

#### Scenario: 后端权限校验
- **WHEN** 无 `itsm:ticket:override` 权限的用户调用 override API
- **THEN** 后端 SHALL 返回 403 Forbidden

### Requirement: Retry AI 确认弹窗
Retry AI 操作 SHALL 弹出确认对话框。

#### Scenario: 用户点击 Retry AI
- **WHEN** 用户点击 Retry AI
- **THEN** SHALL 弹出确认对话框说明"将重置 AI 失败计数并重新触发决策"
- **AND** 仅在确认后执行

### Requirement: OverrideActions 传递 aiFailureCount
详情页头部的 OverrideActions 组件 SHALL 接收并传递 `aiFailureCount` prop。

#### Scenario: Retry AI 在 header 可见
- **WHEN** smart 工单的 aiFailureCount > 0 且用户有 override 权限
- **THEN** header 的 OverrideActions SHALL 显示 Retry AI 选项

### Requirement: Flow visualization 显示用户名
`smart-flow-visualization.tsx` 的 overriddenBy 字段 SHALL 显示用户名而非原始 user ID。

#### Scenario: 活动被 override
- **WHEN** 活动的 overriddenBy 不为空
- **THEN** flow visualization SHALL 显示 override 者的用户名（如"张三"），而非 "#42"

### Requirement: Flow visualization failed 状态颜色
`smart-flow-visualization.tsx` SHALL 为 `failed` 和 `rejected` 状态配置红色（`bg-red-500`）。

#### Scenario: failed 活动显示
- **WHEN** 活动 status 为 failed
- **THEN** 状态圆点 SHALL 显示为红色

#### Scenario: rejected 活动显示
- **WHEN** 活动 status 为 rejected
- **THEN** 状态圆点 SHALL 显示为红色

### Requirement: 处理时长人性化显示
终态卡片的处理时长 SHALL 使用人性化格式（如"2 小时 30 分钟"）而非纯分钟数。

#### Scenario: 超过 60 分钟
- **WHEN** 处理时长为 150 分钟
- **THEN** SHALL 显示"2 小时 30 分钟"

#### Scenario: 不足 60 分钟
- **WHEN** 处理时长为 45 分钟
- **THEN** SHALL 显示"45 分钟"
