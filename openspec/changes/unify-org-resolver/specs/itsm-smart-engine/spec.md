## MODIFIED Requirements

### Requirement: SmartEngine 实现 WorkflowEngine 接口
SmartEngine SHALL 实现与 ClassicEngine 相同的 `WorkflowEngine` 接口（Start/Progress/Cancel），通过 AI Agent 驱动的决策循环替代确定性图遍历。SmartEngine 通过 IOC 注入 AI App 的 AgentService 和 LLM Client。**ParticipantResolver SHALL 消费 `app.OrgResolver` 接口（而非独立的 `engine.OrgService`），在生产环境中正确注入非 nil 实现。**

#### Scenario: 启动智能流程
- **WHEN** `SmartEngine.Start()` 被调用且服务的 `engine_type="smart"`
- **THEN** 引擎 SHALL 构建初始 TicketCase 快照，调用 AI Agent 生成第一步决策计划，根据计划创建第一个 TicketActivity，工单状态从 `pending` 转为 `in_progress`

#### Scenario: AI App 不可用时禁止启动
- **WHEN** `SmartEngine.Start()` 被调用但 IOC 中无法解析 AI App 服务
- **THEN** 引擎 SHALL 返回错误 "智能引擎不可用：AI 模块未安装"

## REMOVED Requirements

### Requirement: engine.OrgService 独立接口
**Reason**: `engine.OrgService` 接口（定义于 `internal/app/itsm/engine/resolver.go`）与 `app.OrgResolver` 功能重复，且在生产环境始终为 nil，导致 position/department/manager 类型的参与人解析无法工作。
**Migration**: `ParticipantResolver` 改为直接消费 `app.OrgResolver`，构造函数签名从 `NewParticipantResolver(orgService OrgService)` 改为 `NewParticipantResolver(orgResolver app.OrgResolver)`。ITSM App 的 IOC 注入通过 `do.InvokeAs[app.OrgResolver](i)` 获取实例。

## ADDED Requirements

### Requirement: ParticipantResolver 使用 app.OrgResolver
`ParticipantResolver` SHALL 使用 `app.OrgResolver` 接口解析所有组织相关的参与人类型。当 `app.OrgResolver` 为 nil（Org App 未安装）时，position/department/position_department/requester_manager 类型 SHALL 返回明确的错误信息。

#### Scenario: position 类型解析通过 OrgResolver
- **WHEN** `Resolve()` 被调用且参与人类型为 `position`，Org App 已安装
- **THEN** 系统 SHALL 调用 `OrgResolver.FindUsersByPositionID(positionID)` 返回用户 ID 列表

#### Scenario: department 类型解析通过 OrgResolver
- **WHEN** `Resolve()` 被调用且参与人类型为 `department`，Org App 已安装
- **THEN** 系统 SHALL 调用 `OrgResolver.FindUsersByDepartmentID(departmentID)` 返回用户 ID 列表

#### Scenario: position_department 类型解析通过 OrgResolver
- **WHEN** `Resolve()` 被调用且参与人类型为 `position_department`，Org App 已安装
- **THEN** 系统 SHALL 调用 `OrgResolver.FindUsersByPositionAndDepartment(positionCode, departmentCode)` 返回用户 ID 列表

#### Scenario: requester_manager 类型解析通过 OrgResolver
- **WHEN** `Resolve()` 被调用且参与人类型为 `requester_manager`，Org App 已安装
- **THEN** 系统 SHALL 调用 `OrgResolver.FindManagerByUserID(requesterID)` 返回上级用户 ID

#### Scenario: Org App 未安装时返回错误
- **WHEN** `Resolve()` 被调用且参与人类型为 `position`，Org App 未安装（OrgResolver 为 nil）
- **THEN** 系统 SHALL 返回错误 "参与人解析失败：position 类型需要安装组织架构模块"

### Requirement: ITSM IOC 正确注入 OrgResolver
ITSM App 的 Providers 方法 SHALL 通过 `do.InvokeAs[app.OrgResolver](i)` 可选解析 OrgResolver，传给 `ParticipantResolver` 和 `Operator`。当 Org App 未安装时，SHALL 传入 nil 并记录日志。

#### Scenario: Org App 已安装时接通
- **WHEN** ITSM App 初始化且 Org App 已安装
- **THEN** `ParticipantResolver` SHALL 接收到非 nil 的 `app.OrgResolver` 实例，日志输出 "ITSM: OrgResolver available"

#### Scenario: Org App 未安装时降级
- **WHEN** ITSM App 初始化且 Org App 未安装
- **THEN** `ParticipantResolver` SHALL 接收 nil OrgResolver，日志输出 "ITSM: OrgResolver not available, org-dependent participant types disabled"
