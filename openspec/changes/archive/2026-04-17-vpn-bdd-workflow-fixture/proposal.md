## Why

VPN 主链路 BDD 测试需要一个预定义的工作流 JSON fixture（ReactFlow 格式），包含：开始 → 提交表单 → 排他网关（网络管理员/安全管理员）→ 审批 → 结束。Phase 1 不调用 LLM 生成，直接注入 fixture。

参考来源：bklite-cloud `tests/bdd/itsm/steps/vpn_service_support.py`（`build_vpn_service_blueprint` + `publish_vpn_service`）

## What Changes

- 创建 `vpn_support_test.go`，包含 VPN 工作流 JSON fixture 常量和服务发布辅助函数
- 定义 `VPN_SAMPLE_FORM_DATA` 测试数据
- 实现 `publishVPNService()` 辅助：创建 ServiceCatalog + SLATemplate + ServiceDefinition

## Capabilities

### New Capabilities
- `vpn-bdd-fixture`: VPN 测试 fixture 和服务发布辅助

## Impact

- `internal/app/itsm/vpn_support_test.go` (new)
- No breaking changes.

## Dependencies

- vpn-bdd-infrastructure (Phase 1)
