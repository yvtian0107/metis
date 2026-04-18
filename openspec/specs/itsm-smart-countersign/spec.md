## Purpose

ITSM SmartEngine 并签（countersign）支持 -- 允许 SmartEngine 创建并行活动组，多个审批人独立处理后汇聚推进。

## Requirements

### Requirement: Smart Engine 并签活动组
SmartEngine SHALL 支持并签（countersign）模式：当 DecisionPlan 的 `execution_mode` 为 `"parallel"` 时，`executeDecisionPlan()` 创建的所有活动 SHALL 共享同一个 `activity_group_id`（UUID），多个活动并行等待处理。

#### Scenario: AI 输出 parallel execution_mode 创建活动组
- **WHEN** Agent 输出 DecisionPlan 且 `execution_mode` 为 `"parallel"`，activities 包含 2 个审批活动（network_admin 和 security_admin）
- **THEN** `executeDecisionPlan()` SHALL 创建 2 个 TicketActivity，它们共享同一个 `activity_group_id` UUID
- **AND** 每个 activity 的 `execution_mode` SHALL 为 `"parallel"`
- **AND** 工单的 `current_activity_id` SHALL 设为活动组中第一个 activity 的 ID（标记入口）

#### Scenario: 默认 execution_mode 行为不变
- **WHEN** Agent 输出 DecisionPlan 且 `execution_mode` 为空或 `"single"`
- **THEN** `executeDecisionPlan()` SHALL 按现有逻辑创建活动，不设置 `activity_group_id`

#### Scenario: 并签活动各自独立指派
- **WHEN** 并签活动组创建完成
- **THEN** 每个 activity SHALL 独立解析参与人并创建 assignment，各审批人独立处理

### Requirement: Progress 汇聚检查
SmartEngine 的 `Progress()` SHALL 在标记 activity completed 之后、触发下一轮决策之前，检查该 activity 是否属于并签组。若属于并签组，SHALL 检查同组所有 activity 是否全部完成。

#### Scenario: 并签组部分完成不触发下一轮
- **WHEN** 并签组有 2 个活动，第 1 个被 approve 触发 Progress
- **THEN** Progress SHALL 标记该 activity 为 completed
- **AND** Progress SHALL 查询同 `activity_group_id` 的其他活动状态
- **AND** 因同组还有未完成活动，Progress SHALL NOT 提交 `itsm-smart-progress` 触发下一轮决策

#### Scenario: 并签组全部完成触发汇聚
- **WHEN** 并签组有 2 个活动，最后 1 个被 approve 触发 Progress
- **THEN** Progress SHALL 标记该 activity 为 completed
- **AND** Progress SHALL 检测到同组所有活动均已 completed
- **AND** Progress SHALL 提交 `itsm-smart-progress` 触发下一轮决策循环

#### Scenario: 非并签活动 Progress 行为不变
- **WHEN** Progress 处理一个 `activity_group_id` 为空的 activity
- **THEN** Progress SHALL 按现有逻辑直接触发下一轮决策，不执行汇聚检查

### Requirement: TicketActivity 新增 ActivityGroupID 字段
`TicketActivity` 数据模型 SHALL 新增 `ActivityGroupID string` 字段，用于关联并签活动组。

#### Scenario: 字段存储和查询
- **WHEN** 创建并签活动时
- **THEN** `activity_group_id` 字段 SHALL 存储 UUID 字符串
- **AND** 查询同组活动时 SHALL 使用 `WHERE activity_group_id = ?` 条件

#### Scenario: Classic Engine 兼容
- **WHEN** Classic Engine 创建活动时
- **THEN** `activity_group_id` 字段 SHALL 为空字符串，不影响 Classic Engine 行为

### Requirement: BDD 验证并签场景
系统 SHALL 提供 LLM 驱动的 BDD 场景验证 Smart Engine 并签能力。

#### Scenario: 全部审批后汇聚推进
- **WHEN** 工单进入并签节点（2 角色并行审批）
- **THEN** 两个审批人各自 approve 后，系统 SHALL 汇聚推进到下一步（最终审批或完成）
- **AND** 工单最终 SHALL 到达 completed 状态

#### Scenario: 部分审批阻塞汇聚
- **WHEN** 工单进入并签节点且只有 1 个审批人完成
- **THEN** 系统 SHALL NOT 提前创建下一步活动
- **AND** 工单 SHALL 保持在并签节点直到所有审批人完成
