# Capability: itsm-smart-recovery

## Purpose

ITSM SmartEngine 启动恢复 -- 在系统重启后自动扫描并恢复中断的智能引擎决策循环。

## Requirements

### Requirement: 智能引擎决策循环恢复任务
系统 SHALL 注册 scheduler 任务 `itsm-smart-recovery`，在 ITSM App 启动时执行一次，扫描并恢复中断的智能引擎决策循环。恢复时 SHALL 利用活动的 NodeID 获取精确的 workflow 路径信息。

#### Scenario: 恢复无活跃活动的 in_progress 票据
- **WHEN** 系统启动且存在 status=in_progress、engine_type=smart 的票据
- **AND** 该票据无任何 pending、in_progress 或 pending_approval 状态的活动
- **THEN** 恢复任务 SHALL 提交 `itsm-smart-progress` 异步任务，payload 包含 ticket_id
- **AND** 恢复触发的下一轮决策 SHALL 通过最后一个已完成活动的 NodeID 获取 `related_step` 和精确的出边目标，不再依赖泛化的 `allowed_recovery_paths`

#### Scenario: 跳过有活跃活动的票据
- **WHEN** 系统启动且存在 status=in_progress、engine_type=smart 的票据
- **AND** 该票据有 pending、in_progress 或 pending_approval 状态的活动
- **THEN** 恢复任务 SHALL 跳过该票据

#### Scenario: 跳过已禁用 AI 的票据
- **WHEN** 系统启动且存在 status=in_progress、engine_type=smart 的票据
- **AND** 该票据的 ai_failure_count >= MaxAIFailureCount
- **THEN** 恢复任务 SHALL 跳过该票据并记录 warning 日志

#### Scenario: 恢复任务幂等性
- **WHEN** 恢复任务被连续执行两次
- **THEN** 第二次执行 SHALL 不产生重复的 itsm-smart-progress 任务（因第一次已触发决策循环，活动状态已改变）

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
