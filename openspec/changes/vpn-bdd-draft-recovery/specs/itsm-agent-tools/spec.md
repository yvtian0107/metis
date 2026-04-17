## ADDED Requirements

### Requirement: draft_confirm 字段变更错误恢复规则

生产 prompt 的严格约束区块 SHALL 新增一条规则：当 `itsm.draft_confirm` 返回含 "字段已变更" 的错误时，Agent MUST 重新调用 `itsm.service_load` 获取最新表单定义，再根据新定义调用 `itsm.draft_prepare` 重新准备草稿；若新增了必填字段，向用户追问后再继续。

#### Scenario: prompt 包含恢复规则
- **WHEN** 查看 IT 服务台智能体的 system_prompt
- **THEN** SHALL 包含对 draft_confirm 字段变更错误的恢复指引
- **AND** 指引 Agent 重新 service_load → draft_prepare

#### Scenario: Agent 遵循恢复规则
- **WHEN** Agent 调用 draft_confirm 收到 "服务表单字段已变更" 错误
- **THEN** Agent SHALL 在 ReAct 循环内重新调用 service_load
- **AND** 随后调用 draft_prepare 重新准备草稿
