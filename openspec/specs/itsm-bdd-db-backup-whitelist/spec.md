# itsm-bdd-db-backup-whitelist

## Purpose

BDD 测试规约：数据库备份白名单放行工单的智能引擎完整流程，覆盖预检、DBA 审批、放行动作执行、并行工单隔离等场景。

## Requirements

### Requirement: 数据库备份白名单完整流程（预检→审批→放行→完成）

智能引擎 SHALL 按"预检动作→DBA 审批→放行动作→完成"的顺序编排数据库备份白名单放行工单，每阶段通过独立的决策循环推进。

#### Scenario: 完整流程——预检、审批、放行、完成
- **WHEN** 请求人创建数据库备份白名单放行工单
- **AND** 智能引擎执行第一轮决策循环
- **THEN** 引擎 SHALL 决策执行预检动作（action type，action_id 指向 precheck ServiceAction）
- **AND** 预检动作 SHALL 同步执行，TicketActionExecution 记录 status=success
- **AND** LocalActionReceiver 的 /precheck 路径 SHALL 收到包含工单表单数据的 HTTP 请求
- **WHEN** 智能引擎执行第二轮决策循环
- **THEN** 引擎 SHALL 决策创建审批活动（approve type），分配到 db_admin 岗位
- **WHEN** DBA 认领并审批通过
- **AND** 智能引擎执行第三轮决策循环
- **THEN** 引擎 SHALL 决策执行放行动作（action type，action_id 指向 apply ServiceAction）
- **AND** 放行动作 SHALL 同步执行，TicketActionExecution 记录 status=success
- **AND** LocalActionReceiver 的 /apply 路径 SHALL 收到 HTTP 请求
- **WHEN** 智能引擎执行第四轮决策循环
- **THEN** 工单状态 SHALL 变为 completed

### Requirement: Action 执行记录与 Receiver 请求匹配

每次 Action 执行 SHALL 同时在 TicketActionExecution 表和 LocalActionReceiver 中留下可验证的记录。

#### Scenario: precheck 动作记录完整
- **WHEN** 预检动作执行成功
- **THEN** TicketActionExecution 中 SHALL 存在一条 status=success 的记录，service_action_id 指向 precheck action
- **AND** request_payload SHALL 包含工单的 database_name 和 source_ip
- **AND** LocalActionReceiver /precheck 路径的请求记录数 SHALL >= 1

#### Scenario: apply 动作记录完整
- **WHEN** 放行动作执行成功
- **THEN** TicketActionExecution 中 SHALL 存在一条 status=success 的记录，service_action_id 指向 apply action
- **AND** request_payload SHALL 包含工单的 database_name 和 whitelist_window
- **AND** LocalActionReceiver /apply 路径的请求记录数 SHALL >= 1

### Requirement: 审批权限校验

非目标岗位人员 SHALL 无法认领或审批 DBA 审批环节的工单。

#### Scenario: 运维管理员无法认领 DBA 审批工单
- **WHEN** 当前活动为 approve 类型，分配到 db_admin 岗位
- **AND** 运维管理员（ops_admin 岗位）尝试认领
- **THEN** 认领 SHALL 失败

#### Scenario: 预检动作未提前触发放行动作
- **WHEN** 当前活动为 approve 类型（预检已完成，等待 DBA 审批）
- **THEN** apply 类型的 TicketActionExecution 记录 SHALL 不存在
- **AND** LocalActionReceiver /apply 路径的请求记录数 SHALL 为 0

### Requirement: 并行工单 Action 隔离

多张工单各自触发的 Action SHALL 完全独立，不互相干扰。

#### Scenario: 两张工单各自独立触发 precheck 和 apply
- **WHEN** 请求人甲和请求人乙分别创建数据库备份白名单放行工单 A 和 B
- **AND** 两张工单各自经历完整流程（预检→审批→放行→完成）
- **THEN** 工单 A 的 TicketActionExecution 记录 SHALL 仅包含工单 A 的 ticket_id
- **AND** 工单 B 的 TicketActionExecution 记录 SHALL 仅包含工单 B 的 ticket_id
- **AND** LocalActionReceiver 的请求 body 中的 ticket_code SHALL 分别对应各自工单
- **AND** 两张工单均 SHALL 最终达到 completed 状态
