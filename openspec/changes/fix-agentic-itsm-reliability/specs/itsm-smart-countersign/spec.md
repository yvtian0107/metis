## MODIFIED Requirements

### Requirement: Progress 汇聚检查
SmartEngine 的 `Progress()` SHALL 在标记 activity completed 之后、触发下一轮决策之前，检查该 activity 是否属于并签组。若属于并签组，SHALL 使用行锁检查同组所有 activity 是否全部完成，防止并发导致重复触发。

#### Scenario: 并签组部分完成不触发下一轮
- **WHEN** 并签组有 2 个活动，第 1 个被 approve 触发 Progress
- **THEN** Progress SHALL 标记该 activity 为 completed
- **AND** Progress SHALL 查询同 `activity_group_id` 的其他活动状态
- **AND** 因同组还有未完成活动，Progress SHALL NOT 提交 `itsm-smart-progress` 触发下一轮决策

#### Scenario: 并签组全部完成触发汇聚
- **WHEN** 并签组有 2 个活动，最后 1 个被 approve 触发 Progress
- **THEN** Progress SHALL 标记该 activity 为 completed
- **AND** Progress SHALL 检测到同组所有活动均已 completed
- **AND** Progress SHALL 通过 ensureContinuation 触发下一轮决策循环

#### Scenario: 并发安全 — 两个并签活动同时完成
- **WHEN** 并签组有 2 个活动，两个活动几乎同时被 approve
- **THEN** Progress SHALL 使用 `SELECT ... FOR UPDATE` 对 activity_group_id 对应的记录加行锁
- **AND** 仅有一个 goroutine 检测到 incompleteCount=0 并触发下一轮决策
- **AND** 另一个 goroutine SHALL 在获得锁后发现已有 0 个未完成，但因决策已触发而跳过

#### Scenario: 非并签活动 Progress 行为不变
- **WHEN** Progress 处理一个 `activity_group_id` 为空的 activity
- **THEN** Progress SHALL 按现有逻辑直接通过 ensureContinuation 触发下一轮决策，不执行汇聚检查
