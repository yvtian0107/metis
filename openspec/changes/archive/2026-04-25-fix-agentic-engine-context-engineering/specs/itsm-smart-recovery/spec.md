## MODIFIED Requirements

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
- **THEN** 第二次执行 SHALL 不产生重复的 itsm-smart-progress 任务
