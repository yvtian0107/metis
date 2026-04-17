## ADDED Requirements

### Requirement: 申请人可撤回未认领的工单
系统 SHALL 允许工单申请人在无人认领时撤回自己的工单。撤回后工单状态变为 cancelled，所有活动的 tokens、activities、assignments 被清理。

#### Scenario: 无人认领时成功撤回
- **WHEN** 申请人提交 VPN 工单后，在审批人认领前请求撤回，并提供撤回原因
- **THEN** 工单状态变为 "cancelled"，所有活动的 execution tokens 和 activities 被取消

#### Scenario: 撤回原因记录在时间线
- **WHEN** 申请人成功撤回工单并提供原因 "项目取消"
- **THEN** 工单时间线中存在 event_type 为 "withdrawn" 的记录，message 包含撤回原因文本

### Requirement: 已认领工单不可撤回
系统 SHALL 在任何 assignment 的 claimed_at 不为空时拒绝撤回请求。

#### Scenario: 审批人认领后撤回失败
- **WHEN** 审批人已认领当前工单的活动后，申请人尝试撤回
- **THEN** 操作返回错误，工单状态保持不变（非 cancelled）

### Requirement: 仅申请人可撤回
系统 SHALL 仅允许工单的 requester_id 对应的用户执行撤回操作。其他用户尝试撤回时返回错误。

#### Scenario: 非申请人撤回失败
- **WHEN** 非申请人用户尝试撤回他人提交的工单
- **THEN** 操作返回错误，工单状态不受影响
