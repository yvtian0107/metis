## Purpose

ITSM 工具链的执行层实现 — ServiceDeskOperator 具体方法体 + StateStore 持久化。

## Requirements

### Requirement: ServiceDeskOperator.MatchServices
Operator SHALL 查询所有 is_active=true 的 ServiceDefinition，按关键词匹配计算 score，返回 top 3。

#### Scenario: 名称精确包含
- **WHEN** query="VPN"，存在服务名"VPN 接入申请"
- **THEN** 该服务 score SHALL >= 0.8

#### Scenario: 无匹配
- **WHEN** query 与所有服务名称/描述/关键词均不匹配
- **THEN** 返回空列表

### Requirement: ServiceDeskOperator.LoadService
Operator SHALL 根据 serviceID 加载 ServiceDefinition + FormDefinition 字段列表 + ServiceAction 列表 + 从 workflow_json 提取 routing_field_hint。

#### Scenario: 完整加载
- **WHEN** serviceID 对应有效的 ServiceDefinition，且关联了 FormDefinition 和 Actions
- **THEN** 返回 ServiceDetail 包含 collaboration_spec、form_fields、actions、fields_hash

#### Scenario: 提取路由字段提示
- **WHEN** ServiceDefinition.WorkflowJSON 包含 exclusive_gateway 且条件引用表单字段
- **THEN** routing_field_hint SHALL 包含 field_key 和 option_route_map

#### Scenario: 无路由分支
- **WHEN** WorkflowJSON 无 exclusive_gateway 或无条件
- **THEN** routing_field_hint SHALL 为 nil

### Requirement: ServiceDeskOperator.CreateTicket
Operator SHALL 通过 TicketService 创建工单，source 设为 "agent"。

#### Scenario: 创建成功
- **WHEN** 所有参数有效
- **THEN** 返回 TicketResult 包含 ticket_id、ticket_code、status

### Requirement: ServiceDeskOperator.ListMyTickets
Operator SHALL 查询 requester_id=userID 的非终态工单。

#### Scenario: 含撤回判断
- **WHEN** 工单 status=pending 且无已完成 activity
- **THEN** can_withdraw=true

### Requirement: ServiceDeskOperator.WithdrawTicket
Operator SHALL 校验 requester_id 后调用 TicketService.Cancel。

#### Scenario: 非提交人
- **WHEN** 当前用户非工单提交人
- **THEN** 返回错误

### Requirement: ServiceDeskOperator.ValidateParticipants
Operator SHALL 解析 workflow_json 获取将被激活分支的审批节点参与者配置，调用 ParticipantResolver 检查。

#### Scenario: 参与者可达
- **WHEN** 所有审批节点可解析到有效用户
- **THEN** 返回 ok=true

#### Scenario: 参与者不可达
- **WHEN** 某节点的 position+department 无有效用户
- **THEN** 返回 ok=false + failure_reason + node_label + guidance

### Requirement: StateStore 基于 AgentSession.State
StateStore SHALL 读写 ai_agent_sessions 表的 state JSON 字段。

#### Scenario: 首次读取
- **WHEN** session.state 为空
- **THEN** GetState 返回 nil

#### Scenario: 读写往返
- **WHEN** SaveState 写入 state 后 GetState 读取
- **THEN** 返回相同数据
