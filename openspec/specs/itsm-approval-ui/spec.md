# Capability: itsm-approval-ui

## Purpose
Provides the frontend UI for ITSM ticket approval workflows, including the "My Approvals" page, inline approve/deny actions, SLA display enhancements, and menu badge integration.
## Requirements
### Requirement: My Approvals page
The system SHALL provide a "我的审批" page at route `/itsm/tickets/approvals` displaying a table of pending approval items. The page SHALL render two kinds of items with visual distinction:

1. **Workflow approvals** (`approvalKind: "workflow"`): 标准审批行，展示 Ticket Code, Title, Service, Priority, SLA Badge, Activity Name, Created At, 和内联 [通过] [驳回] 按钮
2. **AI decision confirmations** (`approvalKind: "ai_confirm"`): AI 确认行，展示 Ticket Code, Title, Service, Priority, AI 置信度（百分比 + 色彩编码：绿 ≥80% / 黄 50-80% / 红 <50%）, Activity Name, Created At, 和内联 [确认] [拒绝] 按钮

Table SHALL support pagination. AI 确认行 SHALL 有视觉区分（如 🤖 图标或不同背景色）。AI `pending_approval` 行 SHALL 仅展示给当前用户实际有权限确认或拒绝的记录，不得把无权操作的 AI 待确认项混入 "我的审批"。

#### Scenario: View mixed approval list
- **WHEN** user navigates to "我的审批" and has both workflow approvals and AI confirmations
- **THEN** page displays all items with visual distinction between the two types

#### Scenario: AI confirmation inline actions
- **WHEN** user clicks [确认] on an AI confirmation item
- **THEN** system calls confirmActivity API, removes the row, shows success toast

#### Scenario: AI confirmation rejection
- **WHEN** user clicks [拒绝] on an AI confirmation item and enters reason
- **THEN** system calls rejectActivity API with reason, removes the row, shows success toast

#### Scenario: Unauthorized AI confirmation does not appear
- **WHEN** user navigates to "我的审批"
- **AND** there exists a status=`pending_approval` AI activity that the user is not authorized to operate
- **THEN** page SHALL NOT render that AI confirmation row

#### Scenario: Empty approval list
- **WHEN** user has no pending approvals nor actionable AI confirmations
- **THEN** page shows empty state message "暂无待审批工单"

### Requirement: Inline approve/deny actions
"我的审批"与工单详情审批交互 SHALL 统一为即时反馈模型：用户提交同意/驳回后，界面 MUST 立即关闭输入面板并进入对应决策中态展示；后端结果回写 SHALL 仅用于确认或纠正，不得阻塞即时状态反馈。详情页还 MUST 展示本轮决策说明入口与可执行恢复动作（当状态允许时）。

#### Scenario: 审批后立即进入决策中
- **WHEN** 用户在审批界面提交"通过"
- **THEN** 当前界面 SHALL 立即显示"通过后决策中"状态
- **AND** 行项或按钮状态 SHALL 同步更新为不可重复提交

#### Scenario: 驳回后立即进入决策中
- **WHEN** 用户提交"驳回"并附带意见
- **THEN** 当前界面 SHALL 立即显示"驳回后决策中"状态
- **AND** 系统 SHALL 在后台继续处理 API 回写

#### Scenario: 后端失败后的状态纠正
- **WHEN** 提交后 API 返回失败
- **THEN** 系统 SHALL 自动重取真实工单状态并提示错误
- **AND** 不得自动恢复到可重复提交的脏状态

#### Scenario: 决策说明与恢复入口可见
- **WHEN** 工单处于决策中或失败恢复相关状态
- **THEN** 详情页 SHALL 提供决策说明入口
- **AND** 若当前用户有权限 SHALL 显示恢复动作入口

### Requirement: Approval detail navigation
Clicking on a ticket code or title in the approval list SHALL navigate to the ticket detail page (`/itsm/tickets/:id`), where the user can view full ticket context before deciding.

#### Scenario: Navigate to ticket detail
- **WHEN** user clicks on ticket code in approval list
- **THEN** browser navigates to `/itsm/tickets/:id` showing full ticket details

### Requirement: SLA enhancement for existing ticket lists
The existing ticket list pages (mine, todo, history) SHALL display SLA status as a color-coded badge and SLA remaining time when applicable. Breached SLA SHALL show red badge with "已超时" text. On-track SLA SHALL show remaining time as relative duration.

#### Scenario: SLA badge on todo list
- **WHEN** user views todo list and a ticket has `slaStatus="breached_response"`
- **THEN** the ticket row shows a red "已超时" badge in the SLA column

#### Scenario: SLA remaining time on mine list
- **WHEN** user views mine list and a ticket has `slaResolutionDeadline` in the future
- **THEN** the ticket row shows remaining time like "剩余 3小时 20分"

### Requirement: Menu item for My Approvals
The ITSM navigation SHALL include a "我的审批" menu item under the ticket section. The menu item SHALL display a badge with the count of pending approvals (from approvals/count API). Badge SHALL disappear when count is 0. The badge count SHALL stay aligned with the same actionable-item rule used by the list, including AI `pending_approval` ownership.

#### Scenario: Menu badge shows count
- **WHEN** user has 3 pending approvals and views ITSM sidebar
- **THEN** "我的审批" menu item shows badge with "3"

#### Scenario: Menu badge hidden when no approvals
- **WHEN** user has 0 pending approvals
- **THEN** "我的审批" menu item shows no badge

#### Scenario: Unauthorized AI pending approval does not inflate badge
- **WHEN** there exists a status=`pending_approval` AI activity that is not actionable for the current user
- **THEN** the sidebar badge SHALL NOT count that activity

### Requirement: 审批页面刷新策略
审批相关列表 SHALL 提供手动刷新按钮。自动刷新 SHALL 仅作为 60 秒兜底观察，不得承担流程推进职责。

#### Scenario: 用户手动刷新审批历史
- **WHEN** 用户点击历史工单列表的刷新按钮
- **THEN** 系统 SHALL 立即重新请求当前筛选条件下的数据

#### Scenario: 60 秒兜底刷新
- **WHEN** 用户停留在审批列表或工单详情页
- **THEN** 页面 SHALL 最多每 60 秒自动刷新一次相关查询
- **AND** 刷新 SHALL NOT 触发任何后端流程推进 API

