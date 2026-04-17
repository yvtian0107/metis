## Why

组织管理（Org）的查询能力分散在 4 套接口中（`app.OrgResolver`、`engine.OrgService`、旧 `app.OrgUserResolver`、旧 `ai.OrgResolver`），导致：ITSM `validate_participants` 直接 JOIN 4 张 org 表绕过所有抽象；`ParticipantResolver.orgService` 在生产环境始终为 nil，position/department/manager 类型的参与人解析全部失败；同一领域概念的接口和实现散布在 3 个 App 中。

前一轮重构已将 DataScope + ID 映射 + 富上下文查询合并为 `app.OrgResolver`，并将 `organization.org_context` 工具迁入 Org App。但 "按组织条件找人" 的能力仍然缺失，`engine.OrgService` 接口仍然存在且未接通。

## What Changes

- 扩展 `app.OrgResolver` 接口，新增 5 个 "找人" 方法：`FindUsersByPositionCode`、`FindUsersByPositionAndDepartment`、`FindUsersByPositionID`、`FindUsersByDepartmentID`、`FindManagerByUserID`
- `OrgResolverImpl` 实现这 5 个方法（基于 Org App 已有的 AssignmentRepo/DB 查询）
- **BREAKING**（仅内部）：删除 `engine.OrgService` 接口，`ParticipantResolver` 改为直接消费 `app.OrgResolver`
- ITSM `app.go` 正确注入 `app.OrgResolver` 到 `ParticipantResolver`（不再 nil）
- ITSM `operator.go` 的 `ValidateParticipants` 从 raw SQL 改为调用 `app.OrgResolver` 方法
- ITSM `ticket_service.go` 从旧 `app.OrgUserResolver`（已删除）迁移到 `app.OrgResolver`

## Capabilities

### New Capabilities

（无新能力——本次是既有接口的归一和接通）

### Modified Capabilities

- `org-scope-resolver`: 扩展 OrgResolver 接口，新增 5 个按组织条件查找用户的方法
- `itsm-smart-engine`: ParticipantResolver 从 engine.OrgService 迁移到 app.OrgResolver，接通生产环境
- `itsm-service-desk-toolkit`: validate_participants 工具从 raw SQL 改为通过 OrgResolver 接口调用

## Impact

- **后端 internal/app/app.go**: OrgResolver 接口扩展（5 个新方法）
- **后端 internal/app/org/**: OrgResolverImpl 实现新方法
- **后端 internal/app/itsm/engine/**: 删除 OrgService 接口，ParticipantResolver 改用 app.OrgResolver
- **后端 internal/app/itsm/tools/**: Operator 接收 OrgResolver 依赖，消除 raw SQL
- **后端 internal/app/itsm/app.go**: IOC 注入接通
- **后端 internal/app/itsm/ticket_service.go**: 迁移到新接口
- **测试 internal/app/itsm/steps_common_test.go**: testOrgService 适配新接口
- **无前端改动、无 API 改动、无数据库迁移**
