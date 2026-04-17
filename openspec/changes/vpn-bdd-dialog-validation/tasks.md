## 1. resolved_values 补全

- [x] 1.1 在 `handlers.go` 的 warning struct 中增加 `ResolvedValues []resolvedValue` 字段（`resolvedValue` 含 `Value string` + `Route string`）
- [x] 1.2 在 `draftPrepareHandler` 的 `multivalue_on_single_field` 分支中，当字段匹配 `detail.RoutingFieldHint.FieldKey` 时，split 逗号值、查 `OptionRouteMap`，填充 `resolved_values`
- [x] 1.3 编写单元测试验证：路由字段多值 → warning 含 resolved_values；非路由字段多值 → warning 不含 resolved_values

## 2. BDD 测试基础设施

- [ ] 2.1 在 `steps_vpn_dialog_validation_test.go` 中实现 `memStateStore`（内存 map 实现 `StateStore` 接口）
- [ ] 2.2 实现 `setupDialogTestExecutor` 辅助函数：构造 `ReactExecutor` + ITSM `ToolRegistry`（使用 memStateStore + 测试 DB 的 ServiceDeskOperator）+ LLM client（从 env 读取配置）
- [ ] 2.3 实现 `collectToolCalls(events)` 辅助函数：从 Event Channel 提取所有 `EventTypeToolCall` 事件，返回工具名列表
- [ ] 2.4 实现 `extractFinalContent(events)` 辅助函数：从 Event Channel 提取最终 assistant 回复文本

## 3. Given 步骤

- [ ] 3.1 实现 Given 步骤：发布 VPN 服务（含 exclusive_gateway 路由条件，routing_field_hint 覆盖 network_support/security/remote_maintenance 三个选项），复用 `givenSmartServicePublished` 并扩展 workflow_json
- [ ] 3.2 实现 Given 步骤：构造用户消息（参数化消息内容），初始化 ReactExecutor

## 4. When 步骤

- [ ] 4.1 实现 When 步骤：执行 `ReactExecutor.Execute()`，收集所有 Event 到 bddContext

## 5. Then 步骤（工具调用序列断言）

- [ ] 5.1 实现 Then 步骤：断言工具调用序列包含指定工具（`hasToolCall`）
- [ ] 5.2 实现 Then 步骤：断言双路径——`draft_prepare` 未被调用 OR `draft_prepare` 被调用但 `draft_confirm` 未被调用
- [ ] 5.3 实现 Then 步骤：断言 `draft_prepare` 的 form_data 中路由字段为单值（同路由合并场景）

## 6. Then 步骤（回复内容辅助断言）

- [ ] 6.1 实现 Then 步骤：断言回复内容匹配 regex（用于验证澄清表述 / 追问表述）
- [ ] 6.2 实现 Then 步骤：断言回复内容不匹配 regex（用于验证同路由场景不要求二选一）

## 7. Feature 文件与注册

- [ ] 7.1 创建 `features/vpn_dialog_validation.feature`：Background + 3 个 Scenario（跨路由冲突、同路由多选、必填缺失），标记 `@llm`
- [ ] 7.2 在 `bdd_test.go` 的 `initializeScenario` 中注册 `registerDialogValidationSteps(sc, bc)`
- [ ] 7.3 运行 `go test ./internal/app/itsm/ -run TestBDD -tags llm -v` 验证所有 scenario green
