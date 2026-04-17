## Why

服务台 Agent 在对话中调用 `draft_confirm` 时，若管理员恰好修改了服务表单字段（FieldsHash 变更），当前系统会返回错误，但 Agent 没有恢复策略——无法自愈重试，导致用户对话中断。需要增加 Agent 错误恢复能力并通过 BDD 测试验证。

## What Changes

- 生产 prompt (`tools/provider.go`) 和测试 prompt 中增加 `draft_confirm` 字段变更错误的恢复规则：重新调用 `service_load` → `draft_prepare`
- 新建 BDD feature `vpn_draft_recovery.feature`，验证 Agent 在 `draft_confirm` 返回字段变更错误后能自动重试
- 扩展 dialog 测试框架：增加 `toolResults` 记录（用于调试）、工具调用计数断言、`mutatingStateStore` 装饰器（在 `draft_prepare` 后注入 DB 表单变更）

## Capabilities

### New Capabilities
- `itsm-bdd-draft-recovery`: BDD 测试覆盖服务台 Agent 在 draft_confirm 字段变更错误后的自愈重试行为

### Modified Capabilities
- `itsm-bdd-infrastructure`: 扩展 dialog 测试框架，增加 toolResults 记录和工具调用计数断言
- `itsm-agent-tools`: 生产 prompt 增加 draft_confirm 字段变更错误恢复规则

## Impact

- `internal/app/itsm/tools/provider.go` — 系统提示词增加恢复规则
- `internal/app/itsm/steps_vpn_dialog_validation_test.go` — dialogTestState 增加 toolResults
- `internal/app/itsm/steps_vpn_draft_recovery_test.go` (new) — mutatingStateStore + 新 step definitions
- `internal/app/itsm/features/vpn_draft_recovery.feature` (new) — 1 个 Scenario
