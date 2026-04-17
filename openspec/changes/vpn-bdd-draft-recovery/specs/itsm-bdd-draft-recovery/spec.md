## ADDED Requirements

### Requirement: 草稿字段变更后 Agent 自愈重试 BDD 场景

系统 SHALL 提供 `vpn_draft_recovery.feature`，包含 1 个 Scenario 验证服务台 Agent 在 `draft_confirm` 返回字段变更错误后能自动重新 `service_load` → `draft_prepare`。

#### Scenario: Agent 在 draft_confirm 字段变更错误后自动重试
- **WHEN** 用户提供完整 VPN 申请信息（类型、原因、时段），Agent 执行 service_match → service_load → draft_prepare
- **AND** draft_prepare 成功后，服务表单字段被修改（新增 optional 字段，FieldsHash 变更）
- **AND** Agent 调用 draft_confirm 收到 "服务表单字段已变更" 错误
- **THEN** Agent SHALL 重新调用 service_load 获取最新表单定义
- **AND** Agent SHALL 重新调用 draft_prepare 准备草稿
- **AND** service_load 总调用次数 ≥ 2
- **AND** draft_prepare 总调用次数 ≥ 2

### Requirement: mutatingStateStore 测试装饰器

系统 SHALL 提供 `mutatingStateStore` 结构体，包装 `memStateStore`，当 state.Stage 转为 `"awaiting_confirmation"` 时触发一次 DB 表单变更（修改 FormDefinition.Schema 为含新增字段的版本），使后续 `draft_confirm` 调用检测到 FieldsHash 不匹配。

#### Scenario: mutatingStateStore 在 draft_prepare 后触发表单变更
- **WHEN** draft_prepare handler 将 state.Stage 设为 "awaiting_confirmation" 并调用 SaveState
- **THEN** mutatingStateStore SHALL 修改 DB 中 FormDefinition 的 Schema（新增 optional 字段）
- **AND** 仅触发一次，后续 SaveState 调用不再触发

### Requirement: 变更后表单定义

系统 SHALL 提供 `vpnFormSchemaV2`，在原有 4 字段（request_kind, vpn_type, reason, access_period）基础上新增 1 个 optional 字段（remark/备注），使 FieldsHash 与原 Schema 不同，但不影响已收集的 form_data 有效性。

#### Scenario: vpnFormSchemaV2 与原 Schema 的 FieldsHash 不同
- **WHEN** 使用 vpnFormSchemaV2 计算 FieldsHash
- **THEN** 结果 SHALL 与 vpnFormSchema 计算的 FieldsHash 不同
