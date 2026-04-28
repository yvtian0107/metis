## MODIFIED Requirements

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
