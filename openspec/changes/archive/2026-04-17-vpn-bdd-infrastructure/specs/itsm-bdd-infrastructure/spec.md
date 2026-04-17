## MODIFIED Requirements

### Requirement: 共享 BDD test context
系统 SHALL 提供 `steps_common_test.go`，定义 `bddContext` 结构体作为所有 step definitions 的共享状态容器。bddContext SHALL 包含以下字段组：

**核心字段（已有）：**
- `db` (*gorm.DB) — 每 Scenario 独立的内存 SQLite
- `lastErr` (error) — 最近一次操作的错误

**引擎字段（新增）：**
- `engine` (*engine.ClassicEngine) — 可工作的经典引擎实例

**参与人字段（新增）：**
- `users` (map[string]*model.User) — key 为身份标签（如"申请人"）
- `usersByName` (map[string]*model.User) — key 为 username
- `positions` (map[string]*org.Position) — key 为 position code
- `departments` (map[string]*org.Department) — key 为 department code

**工单生命周期字段（新增）：**
- `service` (*ServiceDefinition) — 当前场景的服务定义
- `ticket` (*Ticket) — 当前场景的工单
- `tickets` (map[string]*Ticket) — 多工单场景用，key 为别名

#### Scenario: bddContext 包含全部字段组
- **WHEN** 查看 `bddContext` 结构体定义
- **THEN** SHALL 包含核心、引擎、参与人、工单生命周期四组字段

#### Scenario: reset 清理所有字段
- **WHEN** `reset()` 被调用（每 Scenario 之前）
- **THEN** 所有 map 字段 SHALL 被重新初始化为空 map
- **AND** engine SHALL 被重新构建
- **AND** ticket 和 service SHALL 被设为 nil

### Requirement: AutoMigrate 全量模型
`bddContext.reset()` SHALL AutoMigrate 以下全部模型，确保 ClassicEngine 和参与者解析所需的表结构均存在：

- Kernel: `model.User`, `model.Role`
- Org: `org.Department`, `org.Position`, `org.UserPosition`
- AI: `ai.Agent`, `ai.AgentSession`
- ITSM 全部 17 模型: ServiceCatalog, ServiceDefinition, ServiceAction, FormDefinition, Priority, SLATemplate, EscalationRule, Ticket, TicketActivity, TicketAssignment, TicketTimeline, TicketActionExecution, TicketLink, PostMortem, ProcessVariable, ExecutionToken, ServiceKnowledgeDocument

#### Scenario: 所有表在 reset 后存在
- **WHEN** `reset()` 完成
- **THEN** 上述所有模型对应的表 SHALL 存在于内存 SQLite 中
- **AND** `db.AutoMigrate` 不返回错误

### Requirement: testOrgService 实现
系统 SHALL 提供 `testOrgService` struct，实现 `engine.OrgService` 接口，直接查询内存 SQLite 中的 Org 模型表。

- `FindUsersByPositionID(positionID)` SHALL 查询 `user_positions` 表，返回匹配 `position_id` 的所有 `user_id`
- `FindUsersByDepartmentID(departmentID)` SHALL 查询 `user_positions` 表，返回匹配 `department_id` 的所有 `user_id`
- `FindManagerByUserID(userID)` SHALL 查询 `users` 表的 `manager_id` 字段

#### Scenario: FindUsersByPositionID 返回正确用户
- **WHEN** DB 中存在 UserPosition{UserID:1, PositionID:5} 和 UserPosition{UserID:2, PositionID:5}
- **THEN** `FindUsersByPositionID(5)` SHALL 返回 `[1, 2]`

#### Scenario: FindUsersByPositionID 无匹配返回空
- **WHEN** DB 中不存在 PositionID=99 的 UserPosition
- **THEN** `FindUsersByPositionID(99)` SHALL 返回空切片且无错误

### Requirement: noopSubmitter 实现
系统 SHALL 提供 `noopSubmitter` struct，实现 `engine.TaskSubmitter` 接口，`SubmitTask` 方法直接返回 nil。

#### Scenario: noopSubmitter 不执行任何操作
- **WHEN** `noopSubmitter.SubmitTask("any-task", payload)` 被调用
- **THEN** SHALL 返回 nil 且不产生任何副作用

### Requirement: ClassicEngine 在 reset 中实例化
`bddContext.reset()` SHALL 创建可工作的 `ClassicEngine`，使用 `testOrgService`（查 BDD 内存 DB）和 `noopSubmitter`。

#### Scenario: engine 可驱动工单流转
- **WHEN** reset 完成后调用 `bc.engine.Start(ctx, bc.db, params)`
- **THEN** 引擎 SHALL 能解析 workflow JSON 并创建 Activity

## ADDED Requirements

### Requirement: 公共 Given 步骤——系统初始化
系统 SHALL 注册 godog Given 步骤 `^已完成系统初始化$`，该步骤为 no-op（reset 已处理初始化）。

#### Scenario: 系统初始化步骤可匹配
- **WHEN** feature 文件包含 `Given 已完成系统初始化`
- **THEN** godog SHALL 匹配到该步骤且不报 undefined

### Requirement: 公共 Given 步骤——参与人准备
系统 SHALL 注册 godog Given 步骤 `^已准备好以下参与人、岗位与职责$`，从 DataTable 解析并创建参与人。

DataTable 格式：

| 身份 | 用户名 | 部门 | 岗位 |
|------|--------|------|------|
| 申请人 | vpn-requester | - | - |
| 网络管理员审批人 | network-operator | it | network_admin |

处理逻辑：
1. 遍历每行，为每个 username 创建或获取 `model.User`
2. 部门非 `-` 时，创建或获取 `org.Department`（code=部门值, name=部门值）
3. 岗位非 `-` 时，创建或获取 `org.Position`（code=岗位值, name=岗位值），并创建 `org.UserPosition` 关联
4. 存入 `bc.users[身份]`、`bc.usersByName[用户名]`、`bc.positions[岗位code]`、`bc.departments[部门code]`

#### Scenario: DataTable 解析并创建参与人
- **WHEN** feature 包含上述格式的 DataTable
- **THEN** `bc.users["申请人"]` SHALL 是 username 为 `vpn-requester` 的 User
- **AND** `bc.positions["network_admin"]` SHALL 是 code 为 `network_admin` 的 Position
- **AND** `org.UserPosition` 表 SHALL 存在 network-operator 与 it/network_admin 的关联

#### Scenario: 部门和岗位为 - 时跳过
- **WHEN** DataTable 中某行的部门和岗位均为 `-`
- **THEN** SHALL 仅创建 User，不创建 Department/Position/UserPosition
- **AND** `bc.positions` 中不包含该用户的条目

### Requirement: bdd_test.go 注册公共步骤
`initializeScenario` 函数 SHALL 调用公共步骤注册，使 `已完成系统初始化` 和 `已准备好以下参与人、岗位与职责` 在所有 feature 文件中可用。

#### Scenario: 公共步骤在 scenario initializer 中注册
- **WHEN** 查看 `initializeScenario` 函数
- **THEN** SHALL 包含 `sc.Given` 调用注册上述两个公共步骤
