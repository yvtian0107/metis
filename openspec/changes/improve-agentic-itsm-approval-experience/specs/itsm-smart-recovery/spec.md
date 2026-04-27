## ADDED Requirements

### Requirement: 直接决策路径的恢复兜底
Smart recovery SHALL scan smart tickets whose status indicates decisioning (`approved_decisioning`, `rejected_decisioning`, or `decisioning`) but have no active human or action activity and no running decision marker. For those orphaned tickets, recovery SHALL call the same direct decision dispatcher used by the main path instead of relying on `itsm-smart-progress` scheduler worker polling.

#### Scenario: 恢复已同意决策中孤儿工单
- **WHEN** smart recovery finds a ticket with status=`approved_decisioning`
- **AND** the ticket has no pending or in_progress activity
- **THEN** recovery SHALL invoke direct decision dispatch for that ticket
- **AND** triggerReason SHALL be `recovery`

#### Scenario: 恢复已驳回决策中孤儿工单
- **WHEN** smart recovery finds a ticket with status=`rejected_decisioning`
- **AND** the ticket has no pending or in_progress activity
- **THEN** recovery SHALL invoke direct decision dispatch for that ticket
- **AND** decision context SHALL preserve the rejected activity facts

#### Scenario: 最近已提交恢复不重复触发
- **WHEN** recovery has dispatched a ticket within the last 10 minutes
- **THEN** recovery SHALL skip the same ticket until the dedup window expires
