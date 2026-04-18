# itsm-bdd-server-access

## Purpose

生产服务器临时访问申请的智能服务 BDD 测试。覆盖协作规范定义、LLM workflow 生成、智能路由（运维/网络/安全三分支）、边界语义判定和审批责任边界验证。

## Requirements

### Requirement: 生产服务器临时访问协作规范定义

系统 SHALL 提供 `serverAccessCollaborationSpec` 常量，定义生产服务器临时访问申请的协作规范。规范 SHALL 包含 3 条路由分支（运维/网络/安全）的具体归类规则，并明确声明由 AI 运行时判断路由，不使用枚举下拉。

#### Scenario: 协作规范包含完整路由规则
- **WHEN** 查看 `serverAccessCollaborationSpec` 常量
- **THEN** 包含运维类访问（应用排障、主机巡检、日志查看等）→ `ops_admin` 的路由规则
- **AND** 包含网络类访问（网络抓包、链路诊断、ACL 调整等）→ `network_admin` 的路由规则
- **AND** 包含安全类访问（安全审计、取证分析、漏洞修复验证等）→ `security_admin` 的路由规则
- **AND** 所有审批参与者使用 `position_department` 类型，部门编码为 `it`

### Requirement: 4 组 case payload 覆盖全部测试场景

系统 SHALL 提供 `serverAccessCasePayloads` 包含 4 组测试数据：ops、network、security、boundary_security。每组包含 form_data（access_account、target_host、source_ip、access_window、access_purpose）和期望路由到的岗位编码。

#### Scenario: ops payload 路由到 ops_admin
- **WHEN** 使用 ops case payload（access_purpose 描述生产故障排查）
- **THEN** 期望岗位为 `ops_admin`

#### Scenario: network payload 路由到 network_admin
- **WHEN** 使用 network case payload（access_purpose 描述网络链路诊断）
- **THEN** 期望岗位为 `network_admin`

#### Scenario: security payload 路由到 security_admin
- **WHEN** 使用 security case payload（access_purpose 描述安全审计取证）
- **THEN** 期望岗位为 `security_admin`

#### Scenario: boundary_security payload 路由到 security_admin
- **WHEN** 使用 boundary_security case payload（access_purpose 描述异常访问核查+证据保全，语义模糊）
- **THEN** 期望岗位为 `security_admin`

### Requirement: LLM 生成生产服务器访问 workflow

系统 SHALL 提供 `generateServerAccessWorkflow` 函数，将协作规范通过 LLM 生成 workflow JSON。生成的 workflow 结构为 start → request/form → 3 个 approval 节点 → end，边上不带 gateway 条件。

#### Scenario: LLM 生成有效 workflow
- **WHEN** 调用 `generateServerAccessWorkflow` 并传入 LLM 配置
- **THEN** 返回通过 `ValidateWorkflow` 验证的 JSON
- **AND** 包含至少 3 个 approval 类型节点（对应 ops_admin、network_admin、security_admin）

### Requirement: 发布生产服务器临时访问智能服务

系统 SHALL 提供 `publishServerAccessSmartService` 函数，创建 ServiceCatalog + Priority + Agent + ServiceDefinition（engine_type=smart），使用 LLM 生成的 workflow JSON。

#### Scenario: 发布后服务定义可用
- **WHEN** 调用 `publishServerAccessSmartService`
- **THEN** `bddContext.service` 不为 nil
- **AND** `bddContext.service.EngineType` 为 "smart"
- **AND** `bddContext.service.AgentID` 不为 nil

### Requirement: 生产故障排查访问路由到运维管理员

智能引擎 SHALL 将 access_purpose 描述为生产故障排查类的工单路由到 `ops_admin` 岗位审批，审批通过后工单完成。

#### Scenario: ops 场景完整流程
- **WHEN** 申请人创建 ops 场景的生产服务器访问工单
- **AND** 智能引擎执行决策循环
- **THEN** 工单状态为 in_progress
- **AND** 当前活动类型为 approve
- **AND** 当前审批分配到岗位 ops_admin
- **WHEN** 被分配人认领并审批通过
- **AND** 智能引擎再次执行决策循环
- **THEN** 工单状态为 completed

### Requirement: 网络链路诊断访问路由到网络管理员

智能引擎 SHALL 将 access_purpose 描述为网络诊断类的工单路由到 `network_admin` 岗位审批。

#### Scenario: network 场景完整流程
- **WHEN** 申请人创建 network 场景的生产服务器访问工单
- **AND** 智能引擎执行决策循环
- **THEN** 工单状态为 in_progress
- **AND** 当前活动类型为 approve
- **AND** 当前审批分配到岗位 network_admin
- **WHEN** 被分配人认领并审批通过
- **AND** 智能引擎再次执行决策循环
- **THEN** 工单状态为 completed

### Requirement: 安全审计取证访问路由到安全管理员

智能引擎 SHALL 将 access_purpose 描述为安全审计类的工单路由到 `security_admin` 岗位审批。

#### Scenario: security 场景完整流程
- **WHEN** 申请人创建 security 场景的生产服务器访问工单
- **AND** 智能引擎执行决策循环
- **THEN** 工单状态为 in_progress
- **AND** 当前活动类型为 approve
- **AND** 当前审批分配到岗位 security_admin
- **WHEN** 被分配人认领并审批通过
- **AND** 智能引擎再次执行决策循环
- **THEN** 工单状态为 completed

### Requirement: 模糊描述的边界语义判定

智能引擎 SHALL 在访问目的描述模糊时（如"异常访问核查+证据保全"），根据语义权重判定路由到 `security_admin` 而非 `ops_admin`。

#### Scenario: boundary_security 场景判定为安全
- **WHEN** 申请人创建 boundary_security 场景的工单（access_purpose 含"异常访问核查""证据保全"等安全语义）
- **AND** 智能引擎执行决策循环
- **THEN** 当前审批分配到岗位 security_admin（而非 ops_admin）
- **WHEN** 被分配人认领并审批通过
- **AND** 智能引擎再次执行决策循环
- **THEN** 工单状态为 completed

### Requirement: 审批责任边界验证

当工单路由到特定岗位审批时，仅该岗位对应的审批人 SHALL 能认领和审批，其他岗位的审批人认领或审批 SHALL 失败。

#### Scenario: ops 分支审批仅对 ops-operator 可见
- **WHEN** 工单路由到 ops_admin 审批
- **THEN** 当前审批仅对 ops-operator 可见
- **AND** network-operator 认领当前工单 SHALL 失败
- **AND** security-operator 审批当前工单 SHALL 失败

#### Scenario: 正确审批人完成审批后工单结束
- **WHEN** ops-operator 认领并审批通过
- **AND** 智能引擎再次执行决策循环
- **THEN** 工单状态为 completed
