## MODIFIED Requirements

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
