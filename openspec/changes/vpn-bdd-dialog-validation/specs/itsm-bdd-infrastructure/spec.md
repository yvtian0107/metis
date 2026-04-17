## ADDED Requirements

### Requirement: 服务台 Agent 跨路由冲突识别

当用户的诉求涉及映射到不同审批路由分支的多个选项时，服务台 Agent SHALL 识别冲突并向用户澄清，而非替用户选择或直接提交。

#### Scenario: Agent 识别跨路由冲突并向用户澄清
- **WHEN** 用户消息中同时包含映射到不同路由分支的多种需求（如 "network_support" 属网络审批路由，"security" 属安全审批路由）
- **THEN** Agent 的工具调用序列中 SHALL 包含 `itsm.service_match` 和 `itsm.service_load`
- **AND** Agent SHALL 满足以下任一路径：
  - 路径 A：不调用 `itsm.draft_prepare`（在 draft 前根据 routing_field_hint 识别冲突）
  - 路径 B：调用了 `itsm.draft_prepare` 但不调用 `itsm.draft_confirm`（收到 resolved_values 后停止推进）
- **AND** Agent 的回复内容 SHALL 包含关于路由冲突或需要选择的澄清表述

### Requirement: 服务台 Agent 同路由多选合并

当用户提到的多个需求全部映射到同一审批路由分支时，服务台 Agent SHALL 合并处理并继续推进流程，不要求用户二选一。

#### Scenario: Agent 合并同路由多选后正常推进
- **WHEN** 用户消息中包含多种需求，但它们全部映射到同一路由分支（如 "network_support" 和 "remote_maintenance" 均属网络审批路由）
- **THEN** Agent 的工具调用序列中 SHALL 包含 `itsm.service_match`、`itsm.service_load` 和 `itsm.draft_prepare`
- **AND** `itsm.draft_prepare` 的 form_data 中路由字段 SHALL 为单个结构化值（非逗号分隔）
- **AND** Agent 的回复内容中 SHALL 不包含"请选择""二选一""冲突"等要求用户做排他选择的表述

### Requirement: 服务台 Agent 必填缺失追问

当用户提供的信息不足以填满服务表单的所有必填字段时，服务台 Agent SHALL 追问缺失字段，而非带着空字段直接提交草稿。

#### Scenario: Agent 追问缺失的必填字段
- **WHEN** 用户消息仅提供了模糊的服务需求（如 "帮我开个VPN"），缺少 vpn_type、access_period 等必填字段
- **THEN** Agent SHALL 满足以下任一路径：
  - 路径 A：不调用 `itsm.draft_prepare`（在 draft 前识别出必填字段缺失）
  - 路径 B：调用了 `itsm.draft_prepare` 但不调用 `itsm.draft_confirm`（收到 missing_required warnings 后停止推进）
- **AND** Agent 的回复内容 SHALL 包含对缺失信息的追问
