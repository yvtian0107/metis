## 1. 生产 Prompt 恢复规则

- [x] 1.1 在 `internal/app/itsm/tools/provider.go` 的 IT 服务台智能体 system_prompt 严格约束区块新增一条规则：当 `itsm.draft_confirm` 返回含"字段已变更"的错误时，重新调用 `itsm.service_load` → `itsm.draft_prepare`；若新增必填字段则向用户追问

## 2. 扩展 Dialog 测试框架

- [x] 2.1 在 `steps_vpn_dialog_validation_test.go` 的 `dialogTestState` 新增 `toolResults []toolResultRecord` 字段（Name, Output, IsError），在 `setupDialogTest` 的 `EventTypeToolResult` 事件中记录
- [x] 2.2 在 `steps_vpn_dialog_validation_test.go` 新增 `toolCallCount(calls []toolCallRecord, name string) int` 辅助函数
- [x] 2.3 注册新 BDD step `{tool} 被调用至少 {n} 次`，调用 `toolCallCount` 断言 ≥ n

## 3. Draft Recovery 测试实现

- [ ] 3.1 创建 `internal/app/itsm/steps_vpn_draft_recovery_test.go`：定义 `mutatingStateStore`（包装 memStateStore，stage 变为 "awaiting_confirmation" 时修改 DB 中 FormDefinition.Schema 为 vpnFormSchemaV2）
- [ ] 3.2 在同文件定义 `vpnFormSchemaV2`（原 4 字段 + optional remark 字段）
- [ ] 3.3 在同文件实现 `setupDialogTestWithMutation` 函数（复用 setupDialogTest 逻辑，但使用 mutatingStateStore）
- [ ] 3.4 注册 BDD step `服务字段将在草稿准备后变更`（设置 mutating 标志）和 `服务台 Agent 处理用户消息（含字段变更）`（使用 mutation 版 setup）
- [ ] 3.5 创建 `internal/app/itsm/features/vpn_draft_recovery.feature`：1 个 Scenario 验证 Agent 自愈
- [ ] 3.6 在 `bdd_test.go` 的 scenario initializer 中注册 draft recovery steps

## 4. 验证

- [ ] 4.1 运行 `make test-bdd` 确认新 scenario 通过，现有 scenario 不受影响
