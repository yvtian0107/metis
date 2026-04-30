# Capability: itsm-smart-recovery

## Purpose

ITSM SmartEngine 启动恢复 -- 在系统重启后自动扫描并恢复中断的智能引擎决策循环。
## Requirements
### Requirement: 智能引擎决策循环恢复任务
系统 SHALL 保留 scheduler 任务 `itsm-smart-recovery` 作为故障恢复兜底，而非主流程推进机制。恢复任务在扫描到孤儿决策工单时 SHALL 调用 direct decision dispatcher 重新进入决策循环，不再提交 `itsm-smart-progress` 任务。

#### Scenario: 恢复孤儿决策工单
- **WHEN** 周期扫描发现 decisioning 状态且无活跃活动的 smart 工单
- **THEN** 恢复任务 SHALL 调用 direct decision dispatcher
- **AND** 后续决策 SHALL 复用当前 workflow 上下文

#### Scenario: 跳过不应恢复的工单
- **WHEN** 工单存在活跃活动或处于 AI 熔断状态
- **THEN** 恢复任务 SHALL 跳过该工单并记录原因

#### Scenario: 恢复任务幂等
- **WHEN** 在 dedup 窗口内重复扫描到同一孤儿工单
- **THEN** 恢复任务 SHALL 不重复触发调度

### Requirement: Recovery submission deduplication
HandleSmartRecovery SHALL maintain an in-memory map of recently submitted ticketIDs with timestamps. If a ticketID was submitted within the last 10 minutes, it SHALL be skipped. The map SHALL be pruned of entries older than 10 minutes on each run.

#### Scenario: Same ticket not resubmitted within 10 minutes
- **WHEN** HandleSmartRecovery runs twice within 10 minutes and finds the same orphaned ticket
- **THEN** the second run skips the ticket because it was already submitted recently

#### Scenario: Ticket resubmitted after 10 minutes
- **WHEN** HandleSmartRecovery runs 11 minutes after a previous submission for the same ticket
- **THEN** the ticket is eligible for resubmission if still orphaned

### Requirement: 恢复任务注册
ITSM App 的 `Tasks()` 方法 SHALL 返回 `itsm-smart-recovery` 任务，类型为 scheduled，调度表达式为 `@every 10m`。任务在每次触发时扫描孤儿工单并提交恢复任务。

#### Scenario: 任务在 App 启动时注册
- **WHEN** ITSM App 初始化并返回 Tasks 列表
- **THEN** 列表 SHALL 包含 `itsm-smart-recovery` 任务
- **AND** 任务 schedule 为 `@every 10m`

#### Scenario: 周期性执行恢复扫描
- **WHEN** 10 分钟定时器触发 itsm-smart-recovery
- **THEN** 扫描 status=in_progress AND engine_type=smart 的工单并提交恢复任务

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

