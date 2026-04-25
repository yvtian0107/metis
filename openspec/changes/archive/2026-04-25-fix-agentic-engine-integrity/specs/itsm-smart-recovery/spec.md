## ADDED Requirements

### Requirement: Recovery submission deduplication
HandleSmartRecovery SHALL maintain an in-memory map of recently submitted ticketIDs with timestamps. If a ticketID was submitted within the last 10 minutes, it SHALL be skipped. The map SHALL be pruned of entries older than 10 minutes on each run.

#### Scenario: Same ticket not resubmitted within 10 minutes
- **WHEN** HandleSmartRecovery runs twice within 10 minutes and finds the same orphaned ticket
- **THEN** the second run skips the ticket because it was already submitted recently

#### Scenario: Ticket resubmitted after 10 minutes
- **WHEN** HandleSmartRecovery runs 11 minutes after a previous submission for the same ticket
- **THEN** the ticket is eligible for resubmission if still orphaned

## MODIFIED Requirements

### Requirement: 恢复任务注册
ITSM App 的 Tasks() 方法 SHALL 返回 itsm-smart-recovery 任务，类型为 scheduled，调度表达式为 `@every 10m`。任务在每次触发时扫描孤儿工单并提交恢复任务。

#### Scenario: 任务在 App 启动时注册
- **WHEN** ITSM App 调用 Tasks()
- **THEN** 返回的任务列表包含 id="itsm-smart-recovery", type="scheduled", schedule="@every 10m"

#### Scenario: 周期性执行恢复扫描
- **WHEN** 10 分钟定时器触发 itsm-smart-recovery
- **THEN** 扫描 status=in_progress AND engine_type=smart 的工单并提交恢复任务
