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
The "我的审批" table SHALL provide inline action buttons for each row:
- Workflow approvals: "通过" (approve) and "驳回" (deny). "驳回" SHALL open a popover for entering denial reason.
- AI confirmations: "确认" (confirm) and "拒绝" (reject). "拒绝" SHALL open a popover for entering rejection reason.

After action completion, the row SHALL be removed from the list and a success toast shown.

**工单详情页的审批 Sheet 交互**：在工单详情页点击审批按钮弹出的 Sheet 中，用户点击确认提交后，Sheet SHALL 立即关闭并重置表单（不等待 API 响应）。页面 SHALL 立即进入"决策中"状态（通过 `onMutate` 乐观更新）。API 成功后 SHALL 刷新工单数据并显示成功 toast；API 失败后 SHALL 刷新工单数据并显示错误 toast，但不重新打开 Sheet。

#### Scenario: Approve workflow item from list
- **WHEN** user clicks "通过" button on a workflow approval item
- **THEN** system calls approve API, removes the row from list, shows success toast

#### Scenario: Deny workflow item with reason from list
- **WHEN** user clicks "驳回" button, enters reason "不符合规范", and confirms
- **THEN** system calls deny API with reason, removes the row, shows success toast

#### Scenario: Confirm AI decision from list
- **WHEN** user clicks "确认" button on an AI confirmation item
- **THEN** system calls confirmActivity API, removes the row, shows success toast

#### Scenario: Reject AI decision with reason from list
- **WHEN** user clicks "拒绝" button on an AI confirmation item, enters reason, and confirms
- **THEN** system calls rejectActivity API with reason, removes the row, shows success toast

#### Scenario: Ticket detail approval Sheet optimistic close
- **WHEN** user submits approval opinion in the ticket detail Sheet
- **THEN** Sheet SHALL close immediately
- **AND** page SHALL show "决策中" state
- **AND** API call SHALL proceed in background

#### Scenario: Ticket detail approval API failure after Sheet close
- **WHEN** user submits approval and API returns error
- **THEN** error toast SHALL be displayed
- **AND** ticket data SHALL be refreshed to reflect actual state
- **AND** Sheet SHALL NOT reopen automatically

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
