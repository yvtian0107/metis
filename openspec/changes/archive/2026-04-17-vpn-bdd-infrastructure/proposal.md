## Why

VPN 开通申请 BDD 测试需要跨 App 的测试基础设施。现有 `bddContext` 仅 AutoMigrate 了 3 个 ITSM 模型（ServiceCatalog, ServiceDefinition, ServiceAction），不支持 Org App（Department/Position/UserPosition）模型，也缺少通用的 Given 步骤定义和可工作的 ClassicEngine 实例。

参考来源：bklite-cloud `tests/bdd/itsm/steps/common_steps.py`

## What Changes

- 扩展 `bddContext` 结构体，增加 engine、参与人 map、工单生命周期字段
- AutoMigrate 全部 ITSM 17 模型 + Org 模型 + AI 模型 + Kernel User/Role
- 实现 testOrgService（直接查 DB 的 OrgService 实现）供 ParticipantResolver 使用
- 在 reset() 中创建可工作的 ClassicEngine 实例
- 实现公共 Given 步骤：系统初始化、参与人/岗位/部门准备（从 Gherkin DataTable 解析）

## Capabilities

### Modified Capabilities
- `itsm-bdd-infrastructure`: 增加跨 App 模型支持、ClassicEngine 实例化、公共步骤定义

## Impact

- `internal/app/itsm/steps_common_test.go` (modified — bddContext 扩展 + 公共步骤)
- `internal/app/itsm/bdd_test.go` (modified — 注册公共步骤)
- No breaking changes.

## Dependencies

None — this is the foundation for all VPN BDD phases.
