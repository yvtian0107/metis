## Context

Phase 1（vpn-bdd-infrastructure）已完成 bddContext 扩展、AutoMigrate 全量模型、ClassicEngine 实例化、公共 Given 步骤。Phase 2 需要提供 VPN 服务的工作流 JSON fixture 和服务发布辅助函数，使后续 Phase 3+ 的 BDD 场景能直接创建工单并驱动流转。

bklite-cloud 参考实现使用 LLM 生成工作流蓝图（`build_vpn_service_blueprint`），但 Metis BDD 测试需要确定性——直接注入硬编码 ReactFlow JSON 模板。

工作流需要通过 `engine.ValidateWorkflow()` 校验。关键约束：
- 排他网关至少 2 条出边，每条边需要 condition 或 default 标记
- 审批节点需要 participants
- 所有 Participant Value 使用字符串形式的 ID（GORM AutoIncrement 动态生成）

## Goals / Non-Goals

**Goals:**
- 提供可通过 ValidateWorkflow 的 VPN 工作流 JSON fixture
- fixture 使用动态 ID（position ID 参数化），不硬编码
- 提供 `publishVPNService` 辅助函数，创建完整的 ServiceCatalog + Priority + ServiceDefinition
- 提供 `vpnSampleFormData` 常量供后续 When 步骤使用

**Non-Goals:**
- 不创建任何 .feature 文件（Phase 3 再做）
- 不实现 When/Then 步骤（Phase 3 再做）
- 不实现 SmartEngine fixture
- 不覆盖 SLATemplate（简单场景暂不需要 SLA 关联）

## Decisions

### D1: 工作流 fixture 使用函数模板，返回 json.RawMessage

**选择**: `buildVPNWorkflowJSON(ids vpnWorkflowIDs) json.RawMessage` 函数，接收动态 ID 参数
**替代方案**: 硬编码 JSON 字符串常量
**理由**: GORM AutoIncrement 在不同场景下产生不同 ID。函数模板通过 `fmt.Sprintf` 将 position/department ID 注入 workflow JSON，确保引用正确实体。

### D2: 工作流拓扑为最小可验证结构

**选择**: start → form → exclusive_gateway → (2 approve → 2 end)
**替代方案**: 更复杂的拓扑（含 notify、parallel 等）
**理由**: 最小拓扑覆盖 Phase 3-5 的核心场景（提交、路由、审批）。排他网关根据 `form.request_kind` 路由到 network_admin 或 security_admin 审批节点。复杂拓扑可在后续 Phase 按需扩展。

节点拓扑：
```
start → form_submit → exclusive_gw →[form.request_kind=="network_support"]→ approve_network → end_network
                                    →[default]→ approve_security → end_security
```

### D3: Participant 使用 position 类型

**选择**: approve 节点的 participant 类型为 `position`，value 为动态 position ID
**替代方案**: 使用 `position_department` 类型
**理由**: ParticipantResolver 已实现 `position` 类型（查 UserPosition 表），但 `position_department` 类型未实现（resolver.go 中 default 分支返回 error）。BDD 测试中 position 绑定是充分的——通过 Given 步骤创建的 UserPosition 已包含 department 关联。

### D4: publishVPNService 创建完整依赖链

**选择**: `publishVPNService` 一次性创建 ServiceCatalog + Priority + ServiceDefinition，返回 ServiceDefinition
**替代方案**: 分别创建各实体
**理由**: 后续 Phase 的 Given 步骤只需调用一个函数即可获得可用的服务定义。ServiceDefinition 需要 CatalogID（not null），Priority 在创建 Ticket 时需要。一个函数封装整条依赖链最简洁。

### D5: vpnWorkflowIDs 结构体聚合动态 ID

**选择**: 定义 `vpnWorkflowIDs struct{ NetworkAdminPosID, SecurityAdminPosID uint }` 作为 fixture 参数
**替代方案**: 直接传递多个 uint 参数
**理由**: 结构体比多参数更清晰，且便于后续扩展（如增加 department ID）。调用方从 `bc.positions` map 取出 Position.ID 传入。

## Risks / Trade-offs

**[硬编码节点/边 ID]** → fixture 中的节点和边使用固定字符串 ID（如 "node_start", "edge_1"）。这些 ID 只在测试 fixture 内部使用，不影响生产代码。后续 Phase 的 Then 步骤可能需要引用这些 ID 来断言 Activity 的 NodeID。

**[form_submit 节点的 formId]** → ClassicEngine 的 form 节点需要 formId 但当前 BDD 不真正执行表单填写。fixture 中设置一个占位 formId，engine.Start 会创建 form activity 但不会阻塞（startNode 的下一个节点直接处理）。实际上 engine.Start 跳过 start 直接处理第一个目标节点——如果是 form，会创建 form activity 并暂停等待提交。Phase 3 的 When 步骤需要调用 engine.Progress 来推进。

**[Priority 依赖]** → Ticket 创建需要 PriorityID (not null)。publishVPNService 同时创建一个默认 Priority，存入 bddContext 供后续使用。
