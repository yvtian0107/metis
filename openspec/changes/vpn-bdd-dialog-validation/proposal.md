## Why

验证服务台 Agent 在对话层的智能识别能力：跨路由冲突识别、同路由多选合并、必填缺失追问。仅覆盖 Agentic（智能引擎）场景，使用真实 LLM 驱动 ReactExecutor + 真实 ITSM 工具。

## What Changes

### 1. 补全 `resolved_values`

`draft_prepare` 返回 `multivalue_on_single_field` warning 时，如果该字段是路由字段，附加 `resolved_values`（每个值对应的路由分支），让 Agent 能可靠判断同路由 vs 跨路由。

### 2. BDD Feature + Steps

- 创建 `features/vpn_dialog_validation.feature`，3 个 Scenario：
  - 跨路由冲突——Agent 识别并向用户澄清
  - 同路由多选——Agent 合并后正常推进
  - 必填缺失——Agent 追问缺失信息而非直接提交
- 创建 `steps_vpn_dialog_validation_test.go`

### 3. 测试架构

直接构造 `llm.Message` + 调用 `ReactExecutor`（真实 LLM），注册真实 ITSM 工具（内存 StateStore）。主断言工具调用序列，辅助断言回复内容。采用双路径断言：路径 A（Agent 在 draft_prepare 前拦截）优先，路径 B（Agent 调了 draft_prepare 但靠 warning 修正）作为可接受 fallback。

## Capabilities

### Modified Capabilities
- `itsm-bdd-infrastructure`: 增加服务台对话校验场景
- `itsm-service-desk-tools`: draft_prepare 补全 resolved_values

## Impact

- `internal/app/itsm/tools/handlers.go` (modified — draft_prepare 补全 resolved_values)
- `internal/app/itsm/features/vpn_dialog_validation.feature` (new)
- `internal/app/itsm/steps_vpn_dialog_validation_test.go` (new)

## Dependencies

- vpn-bdd-dialog-coverage (Phase 6)
