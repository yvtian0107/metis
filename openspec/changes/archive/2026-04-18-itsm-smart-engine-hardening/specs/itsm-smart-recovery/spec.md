## ADDED Requirements

### Requirement: 智能引擎决策循环恢复任务
系统 SHALL 注册 scheduler 任务 `itsm-smart-recovery`，在 ITSM App 启动时执行一次，扫描并恢复中断的智能引擎决策循环。

#### Scenario: 恢复无活跃活动的 in_progress 票据
- **WHEN** 系统启动且存在 status=in_progress、engine_type=smart 的票据
- **AND** 该票据无任何 pending 或 in_progress 状态的活动
- **THEN** 恢复任务 SHALL 提交 `itsm-smart-progress` 异步任务，payload 包含 ticket_id

#### Scenario: 跳过有活跃活动的票据
- **WHEN** 系统启动且存在 status=in_progress、engine_type=smart 的票据
- **AND** 该票据有 pending 或 in_progress 状态的活动
- **THEN** 恢复任务 SHALL 跳过该票据

#### Scenario: 跳过已禁用 AI 的票据
- **WHEN** 系统启动且存在 status=in_progress、engine_type=smart 的票据
- **AND** 该票据的 ai_failure_count >= MaxAIFailureCount
- **THEN** 恢复任务 SHALL 跳过该票据并记录 warning 日志

#### Scenario: 恢复任务幂等性
- **WHEN** 恢复任务被连续执行两次
- **THEN** 第二次执行 SHALL 不产生重复的 itsm-smart-progress 任务（因第一次已触发决策循环，活动状态已改变）

### Requirement: 恢复任务注册
ITSM App 的 `Tasks()` 方法 SHALL 返回 `itsm-smart-recovery` 任务定义，类型为 scheduled，schedule 为 `@reboot`（仅在启动时执行一次）。

#### Scenario: 任务在 App 启动时注册
- **WHEN** ITSM App 初始化并返回 Tasks 列表
- **THEN** 列表 SHALL 包含 `itsm-smart-recovery` 任务
- **AND** 任务 schedule 为仅启动时执行一次
