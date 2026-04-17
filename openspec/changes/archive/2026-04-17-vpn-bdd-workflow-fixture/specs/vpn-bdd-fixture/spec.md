## ADDED Requirements

### Requirement: VPN 工作流 JSON fixture
系统 SHALL 提供 `buildVPNWorkflowJSON(ids vpnWorkflowIDs) json.RawMessage` 函数，返回符合 ReactFlow 格式的工作流 JSON。

工作流拓扑：
- `node_start` (start) → `node_form` (form) → `node_gw` (exclusive) → 两条分支
- 分支 1: `edge_gw_network` (condition: form.request_kind equals "network_support") → `node_approve_network` (approve, participant: position=NetworkAdminPosID) → `node_end_network` (end)
- 分支 2: `edge_gw_security` (default: true) → `node_approve_security` (approve, participant: position=SecurityAdminPosID) → `node_end_security` (end)

`vpnWorkflowIDs` 结构体 SHALL 包含：
- `NetworkAdminPosID uint` — 网络管理员岗位 ID
- `SecurityAdminPosID uint` — 安全管理员岗位 ID

#### Scenario: fixture 通过 ValidateWorkflow 校验
- **WHEN** 调用 `buildVPNWorkflowJSON` 传入有效 ID
- **THEN** 返回的 JSON SHALL 通过 `engine.ValidateWorkflow()` 且无 error 级别结果

#### Scenario: fixture 包含正确的排他网关条件
- **WHEN** 解析返回的 workflow JSON
- **THEN** 排他网关 `node_gw` SHALL 有 2 条出边
- **AND** 一条边的 condition SHALL 为 `{field: "form.request_kind", operator: "equals", value: "network_support"}`
- **AND** 另一条边 SHALL 标记为 `default: true`

#### Scenario: fixture 中 approve 节点使用动态 position ID
- **WHEN** 传入 `NetworkAdminPosID=42, SecurityAdminPosID=99`
- **THEN** `node_approve_network` 的 participant SHALL 为 `{type: "position", value: "42"}`
- **AND** `node_approve_security` 的 participant SHALL 为 `{type: "position", value: "99"}`

### Requirement: VPN 表单测试数据
系统 SHALL 提供 `vpnSampleFormData` 作为 map[string]any 常量/变量，包含 VPN 申请表单的典型字段值。

至少包含：
- `request_kind` (string): "network_support" — 用于排他网关路由
- `vpn_type` (string): "l2tp"
- `reason` (string): 测试用申请原因

#### Scenario: 表单数据包含路由关键字段
- **WHEN** 查看 `vpnSampleFormData`
- **THEN** SHALL 包含 `request_kind` 字段
- **AND** 值 SHALL 为 "network_support"（路由到网络管理员分支）

### Requirement: publishVPNService 辅助函数
系统 SHALL 提供 `publishVPNService(bc *bddContext, ids vpnWorkflowIDs) error` 函数，创建 BDD 测试所需的完整服务配置。

该函数 SHALL 按顺序创建：
1. `ServiceCatalog`（name="VPN服务", code="vpn"）
2. `Priority`（name="普通", code="normal", level=3）
3. `ServiceDefinition`（name="VPN开通申请", code="vpn-activation", engineType="classic", workflowJSON=buildVPNWorkflowJSON(ids)）

创建完成后 SHALL：
- 将 ServiceDefinition 存入 `bc.service`
- 将 Priority 存入 bddContext 可访问位置（新增 `bc.priority` 字段或直接在函数返回值中包含）

#### Scenario: publishVPNService 创建完整依赖链
- **WHEN** 调用 `publishVPNService(bc, ids)`
- **THEN** 数据库 SHALL 包含 ServiceCatalog(code="vpn")
- **AND** 数据库 SHALL 包含 Priority(code="normal")
- **AND** `bc.service` SHALL 非 nil 且 code 为 "vpn-activation"
- **AND** `bc.service.WorkflowJSON` SHALL 非空

#### Scenario: publishVPNService 的 ServiceDefinition 关联正确
- **WHEN** 查询创建的 ServiceDefinition
- **THEN** CatalogID SHALL 指向 code="vpn" 的 ServiceCatalog
- **AND** EngineType SHALL 为 "classic"
- **AND** WorkflowJSON SHALL 通过 engine.ValidateWorkflow 校验

### Requirement: bddContext 扩展 priority 字段
`bddContext` SHALL 新增 `priority *Priority` 字段，用于存储当前场景的默认优先级。`reset()` SHALL 将其设为 nil。

#### Scenario: reset 清空 priority
- **WHEN** `reset()` 被调用
- **THEN** `bc.priority` SHALL 为 nil
