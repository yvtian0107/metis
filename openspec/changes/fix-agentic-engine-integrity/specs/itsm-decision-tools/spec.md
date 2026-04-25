## MODIFIED Requirements

### Requirement: decision.sla_status 工具
返回当前工单的 SLA 紧急度评估。字段：has_sla, response_remaining_seconds, resolution_remaining_seconds, urgency ("normal"/"warning"/"critical"/"breached"), sla_status。紧急度阈值 SHALL 从 EngineConfigProvider 读取：critical_threshold_seconds（默认 1800）和 warning_threshold_seconds（默认 3600）。如果 EngineConfigProvider 为 nil 或未配置，使用默认值。

#### Scenario: SLA 即将违约
- **WHEN** 工单剩余解决时间 < critical_threshold_seconds（默认 1800 秒）
- **THEN** 返回 urgency="critical"

#### Scenario: SLA 已违约
- **WHEN** 工单已超过解决期限
- **THEN** 返回 urgency="breached", 剩余时间为负值

#### Scenario: 无 SLA 的工单
- **WHEN** 工单未关联 SLA
- **THEN** 返回 has_sla=false, urgency="normal"

#### Scenario: 阈值从 EngineConfigProvider 读取
- **WHEN** EngineConfigProvider 配置 critical_threshold_seconds=900, warning_threshold_seconds=1800
- **THEN** 工单剩余 1500 秒时返回 urgency="critical"（而非默认的 "warning"）

#### Scenario: EngineConfigProvider 不可用时使用默认值
- **WHEN** EngineConfigProvider 为 nil
- **THEN** 使用默认阈值 critical=1800s, warning=3600s

### Requirement: decision.similar_history 工具
查询同服务已完成工单的处理模式。返回数组包含 resolution_duration_hours、activity_count、assignee_names，以及聚合统计 avg_resolution_hours、total_count。查询条数上限 SHALL 从 EngineConfigProvider.SimilarHistoryLimit() 读取，默认值为 5。如果 EngineConfigProvider 为 nil 或未配置，使用默认值 5。

#### Scenario: 查询有历史的服务
- **WHEN** 调用 similar_history 工具且该服务有 10 条已完成工单
- **THEN** 返回不超过 SimilarHistoryLimit 条（默认 5）的工单摘要及聚合统计

#### Scenario: 查询无历史的新服务
- **WHEN** 调用 similar_history 工具且该服务无已完成工单
- **THEN** 返回空数组及 total_count=0

#### Scenario: 自定义 limit 从配置读取
- **WHEN** EngineConfigProvider.SimilarHistoryLimit() 返回 10
- **THEN** 最多返回 10 条历史记录而非默认的 5 条
