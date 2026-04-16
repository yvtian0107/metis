## 1. bddContext 扩展

- [ ] 1.1 在 `steps_common_test.go` 的 `bddContext` 中新增 `priority *Priority` 字段
- [ ] 1.2 在 `reset()` 中将 `bc.priority` 设为 nil

## 2. 工作流 fixture

- [ ] 2.1 创建 `vpn_support_test.go`，定义 `vpnWorkflowIDs` 结构体（NetworkAdminPosID, SecurityAdminPosID）
- [ ] 2.2 实现 `buildVPNWorkflowJSON(ids vpnWorkflowIDs) json.RawMessage`，生成完整 ReactFlow 格式工作流 JSON（start → form → exclusive_gw → 2×approve → 2×end）
- [ ] 2.3 定义 `vpnSampleFormData` 变量（map[string]any，包含 request_kind, vpn_type, reason）

## 3. 服务发布辅助

- [ ] 3.1 实现 `publishVPNService(bc *bddContext, ids vpnWorkflowIDs) error`：创建 ServiceCatalog + Priority + ServiceDefinition，存入 bc.service 和 bc.priority

## 4. 验证

- [ ] 4.1 运行 `go test ./internal/app/itsm/ -run TestBDD -v` 确认编译通过
- [ ] 4.2 验证 `buildVPNWorkflowJSON` 生成的 JSON 能通过 `engine.ValidateWorkflow` 校验（可在 test 中加一个小断言或手动确认）
