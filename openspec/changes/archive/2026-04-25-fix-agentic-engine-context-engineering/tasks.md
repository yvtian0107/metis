## 1. DecisionPlan 结构体与解析

- [x] 1.1 在 `DecisionActivity` 结构体中增加 `NodeID string` 字段（`json:"node_id,omitempty"`）— `internal/app/itsm/engine/smart.go:98-107`
- [x] 1.2 验证 `parseDecisionPlan()` 能正确解析含 `node_id` 的 JSON 输出（检查现有测试并补充 node_id 场景）

## 2. Activity 创建站点写入 NodeID

- [x] 2.1 `executeSinglePlan` 创建 activityModel 时设置 `NodeID: da.NodeID` — `smart.go:500-509`
- [x] 2.2 `executeParallelPlan` 创建 activityModel 时设置 `NodeID: da.NodeID` — `smart.go:424-435`
- [x] 2.3 `pendManualHandlingPlan` 创建 activityModel 时设置 `NodeID`（取 `plan.Activities[0].NodeID`，需判空）— `smart.go:689-698`

## 3. node_id 路径合规验证

- [x] 3.1 在 `validatePlan()` 中增加 node_id 校验逻辑：检查节点是否存在于 workflow_json、节点类型是否与 activity type 兼容，不合法时清空并 log warning — `smart.go`
- [x] 3.2 编写 `validatePlan` node_id 校验的单元测试（存在且匹配、不存在、类型不匹配、无 workflow_json 四种场景）

## 4. Approved 路径上下文注入

- [x] 4.1 在 `buildInitialSeed` 中，当 completed activity outcome 为正向且 NodeID 有效时，查找 approved 出边目标，注入 `seed["approved_next_step"]` — `smart.go:1357-1379`
- [x] 4.2 在 `buildWorkflowContext` 中，当 completed activity outcome 为正向且 NodeID 有效时，给 `related_step` 附加 `approved_edge_target` — `smart_workflow_context.go:67-73`
- [x] 4.3 对称地，当 outcome 为负向时给 `related_step` 附加 `rejected_edge_target` — `smart_workflow_context.go`
- [x] 4.4 补充 `smart_context_test.go` 测试用例：approved 路径注入、rejected 路径注入、NodeID 为空降级

## 5. Activity form_data 透出

- [x] 5.1 在 `activityFactMap` 中增加 `form_data` 返回：当 `a.FormData` 非空时将其作为 `json.RawMessage` 放入返回 map — `smart_tools.go:219-259`
- [x] 5.2 补充 `activityFactMap` 的单元测试（有 form_data、无 form_data 两种场景）

## 6. Seed 去重

- [x] 6.1 将 `buildInitialSeed` 中的 `seed["completed_activity"]` 从 `activityFactMap(completed, assignments)` 改为轻量锚点 `{id, outcome, operator_opinion}` — `smart.go:1361`
- [x] 6.2 确认 `rejected_activity_policy` 和 `approved_next_step` 仍保留在 seed 中
- [x] 6.3 确认 `ticket_context` 工具的 `completed_activity` 仍通过 `activityFactMap` 返回完整事实 — `smart_tools.go:134-138`
- [x] 6.4 更新 `smart_context_test.go` 中断言 seed 结构的测试用例

## 7. formSchema 类型对齐

- [x] 7.1 扩展 `PathBuilderSystemPrompt` 中 formSchema 的字段 type 可选值：增加 `email, url, radio, datetime, user_picker, dept_picker, rich_text, switch, multi_select, date_range, table` — `workflow_generate_prompt.go:112`
- [x] 7.2 在 prompt 中增加说明：高级类型仅在协作规范明确需要时使用

## 8. 验证器 formSchema 引用校验

- [x] 8.1 在 `ValidateWorkflow` 中新增排他网关 condition `form.xxx` 字段的 formSchema 引用校验（BFS 回溯上游 form 节点，取 formSchema.fields key 并集）— `validator.go`
- [x] 8.2 确保 `ParseNodeData` 能解析 formSchema.fields（检查 `NodeData` 结构体是否已有 FormSchema 字段，无则增加）
- [x] 8.3 编写验证器 formSchema 引用校验的单元测试（字段存在、字段不存在、无上游 form、多上游 form 并集、非 form. 前缀跳过）

## 9. 参与人预检路径统一

- [x] 9.1 `tools/operator.go` 的 `Operator` 结构体增加 `resolver *engine.ParticipantResolver` 字段，在构造时注入
- [x] 9.2 重写 `ValidateParticipants`：使用 `engine.ParseWorkflowDef` 解析 workflow_json，遍历 process + form 节点，对每个 participant 调用 `resolver.Resolve()` — `operator.go:241-356`
- [x] 9.3 删除 `ValidateParticipants` 中独立的 `workflowParticipant` 结构体和手工解析逻辑
- [x] 9.4 编写 `ValidateParticipants` 统一路径的单元测试

## 10. Agent prompt node_id 输出说明

- [x] 10.1 在 decision agent 的 system prompt 输出格式说明中增加 `node_id` 字段描述（可选，有 workflow_json 时建议填写）— `tools/provider.go` decisionAgent prompt
- [x] 10.2 验证现有 Agent 不带 node_id 时系统仍正常工作（回归测试）

## 11. 集成验证

- [x] 11.1 运行 `go build -tags dev ./cmd/server` 确认编译通过
- [x] 11.2 运行 `go test ./internal/app/itsm/engine/...` 确认引擎包全部测试通过
- [x] 11.3 运行 `go test ./internal/app/itsm/...` 确认 ITSM 模块全部测试通过
