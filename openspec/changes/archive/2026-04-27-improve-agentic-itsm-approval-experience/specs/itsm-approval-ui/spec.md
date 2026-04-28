## MODIFIED Requirements

### Requirement: Inline approve/deny actions
The approval tables and ticket detail page SHALL provide action controls for each actionable human activity:
- Workflow human activities: "同意" and "驳回". "驳回" SHALL collect a required reason.
- AI confirmations: "确认" and "拒绝". "拒绝" SHALL collect a required reason.

After workflow human action completion, the row SHALL be removed from the pending list, a success toast SHALL be shown, and all affected ticket list queries SHALL be invalidated. The ticket detail page approval Sheet SHALL close immediately and reset the form on submit. The page SHALL immediately show "已同意，决策中" or "已驳回，决策中" according to the submitted outcome. API failure SHALL refresh actual ticket data and display an error toast, but SHALL NOT reopen the Sheet automatically.

#### Scenario: Approve workflow item from list
- **WHEN** user clicks "同意" button on a workflow approval item
- **THEN** system calls progress API with outcome approved
- **AND** the pending row is removed from list
- **AND** affected approval history and ticket list queries are invalidated
- **AND** success toast is shown

#### Scenario: Deny workflow item with reason from list
- **WHEN** user clicks "驳回" button, enters reason "不符合规范", and confirms
- **THEN** system calls progress API with outcome rejected and the reason
- **AND** the pending row is removed from list
- **AND** affected approval history and ticket list queries are invalidated
- **AND** success toast is shown

#### Scenario: Confirm AI decision from list
- **WHEN** user clicks "确认" button on an AI confirmation item
- **THEN** system calls confirmActivity API
- **AND** removes the row from list
- **AND** shows success toast

#### Scenario: Reject AI decision with reason from list
- **WHEN** user clicks "拒绝" button on an AI confirmation item, enters reason, and confirms
- **THEN** system calls rejectActivity API with reason
- **AND** removes the row from list
- **AND** shows success toast

#### Scenario: Ticket detail approval Sheet optimistic close
- **WHEN** user submits approval opinion in the ticket detail Sheet
- **THEN** Sheet SHALL close immediately
- **AND** page SHALL show "已同意，决策中" for approved outcome or "已驳回，决策中" for rejected outcome
- **AND** API call SHALL proceed in background

#### Scenario: Ticket detail approval API failure after Sheet close
- **WHEN** user submits approval and API returns error
- **THEN** error toast SHALL be displayed
- **AND** ticket data SHALL be refreshed to reflect actual state
- **AND** Sheet SHALL NOT reopen automatically

## ADDED Requirements

### Requirement: 审批页面刷新策略
审批相关列表 SHALL 提供手动刷新按钮。自动刷新 SHALL 仅作为 60 秒兜底观察，不得承担流程推进职责。

#### Scenario: 用户手动刷新审批历史
- **WHEN** 用户点击历史工单列表的刷新按钮
- **THEN** 系统 SHALL 立即重新请求当前筛选条件下的数据

#### Scenario: 60 秒兜底刷新
- **WHEN** 用户停留在审批列表或工单详情页
- **THEN** 页面 SHALL 最多每 60 秒自动刷新一次相关查询
- **AND** 刷新 SHALL NOT 触发任何后端流程推进 API
