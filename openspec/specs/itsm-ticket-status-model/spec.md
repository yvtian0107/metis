# itsm-ticket-status-model Specification

## Purpose
定义用户可见的 ITSM 工单状态模型和展示合同，确保状态语义与用户决策体验一致，并避免把不同业务结果折叠成泛化状态。

## Requirements
### Requirement: 用户可见工单状态模型
系统 SHALL 使用产品语义优先的 Ticket status 表达用户当前能理解的工单状态。Ticket status SHALL 包含 `submitted`、`waiting_human`、`approved_decisioning`、`rejected_decisioning`、`decisioning`、`executing_action`、`completed`、`rejected`、`withdrawn`、`cancelled`、`failed`。系统 SHALL 使用 Ticket outcome 表达终态结果，包含 `approved`、`rejected`、`fulfilled`、`withdrawn`、`cancelled`、`failed`；非终态工单 outcome SHALL 为空。

#### Scenario: 审批同意后展示已同意决策中
- **WHEN** 用户同意一个智能工单的人工活动
- **THEN** 该工单 status SHALL 更新为 `approved_decisioning`
- **AND** 该工单 outcome SHALL 保持为空
- **AND** API 展示文本 SHALL 表达为“已同意，决策中”

#### Scenario: 审批驳回后展示已驳回决策中
- **WHEN** 用户驳回一个智能工单的人工活动
- **THEN** 该工单 status SHALL 更新为 `rejected_decisioning`
- **AND** 该工单 outcome SHALL 保持为空
- **AND** API 展示文本 SHALL 表达为“已驳回，决策中”

#### Scenario: 终态结果清晰区分
- **WHEN** 工单最终因通过路径结束
- **THEN** 该工单 status SHALL 为 `completed`
- **AND** outcome SHALL 为 `approved` 或 `fulfilled`
- **AND** 列表 SHALL NOT 显示泛化“已完成”来代替具体结果

### Requirement: 人工活动业务结果状态
系统 SHALL 对人工活动直接使用 `approved` 或 `rejected` 作为 activity status。人工活动包括 approve、form、process 中需要人工提交结果的节点。`completed` SHALL 仅用于非人工动作已完成或兼容内部流程节点，不得作为人工同意/驳回的用户可见状态。

#### Scenario: 人工同意活动
- **WHEN** 用户提交 outcome=`approved`
- **THEN** 当前人工 activity status SHALL 更新为 `approved`
- **AND** transition_outcome SHALL 保存为 `approved`
- **AND** assignment status SHALL 更新为 `approved`

#### Scenario: 人工驳回活动
- **WHEN** 用户提交 outcome=`rejected`
- **THEN** 当前人工 activity status SHALL 更新为 `rejected`
- **AND** transition_outcome SHALL 保存为 `rejected`
- **AND** assignment status SHALL 更新为 `rejected`

### Requirement: 状态展示合同
Ticket API 响应 SHALL 返回足够字段让前端无需推导即可展示状态，包括 `status`、`outcome`、`statusLabel`、`statusTone`、`lastHumanOutcome` 和 `decisioningReason`。前端 SHALL 以该状态合同渲染 Badge、筛选项和历史结果。

#### Scenario: 前端无需扫描时间线推导状态
- **WHEN** 前端请求工单列表 API
- **THEN** 每条工单响应 SHALL 包含状态展示合同字段
- **AND** 前端 SHALL NOT 为了区分已通过、已驳回或已撤回而额外扫描 timeline
