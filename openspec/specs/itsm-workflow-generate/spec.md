## ADDED Requirements

### Requirement: 协作规范解析 API

系统 SHALL 提供 `POST /api/v1/itsm/workflows/generate` API，接收协作规范和上下文信息，调用 LLM 解析生成 ReactFlow 格式的工作流 JSON。API 受 JWT + Casbin 权限保护。

请求结构：
```json
{
  "service_id": 1,
  "collaboration_spec": "用户提交申请，收集数据库名...",
  "available_actions": [
    {"code": "precheck", "name": "预检查", "description": "检查数据库连接"}
  ]
}
```

响应结构：
```json
{
  "workflow_json": {
    "nodes": [...],
    "edges": [...]
  }
}
```

#### Scenario: 成功解析协作规范
- **WHEN** 管理员调用解析 API 并传入有效的协作规范
- **THEN** 系统 SHALL 读取 `itsm.generator` Agent 配置构建 LLM Client，将协作规范 + 内置约束提示词 + 可用动作信息组合为 prompt，调用 LLM 获取结果，解析为 workflow_json 返回

#### Scenario: 引擎未配置
- **WHEN** 调用解析 API 但 `itsm.generator` Agent 的 model_id 为空
- **THEN** 系统 SHALL 返回 400 错误 "工作流解析引擎未配置，请前往引擎配置页面设置"

#### Scenario: 协作规范为空
- **WHEN** 调用解析 API 但 collaboration_spec 为空字符串
- **THEN** 系统 SHALL 返回 400 错误 "协作规范不能为空"

#### Scenario: LLM 调用失败
- **WHEN** LLM 调用返回错误或超时
- **THEN** 系统 SHALL 按 `itsm.engine.general.max_retries` 配置重试，全部失败后返回 500 错误，包含错误摘要信息

#### Scenario: LLM 返回无效 JSON
- **WHEN** LLM 返回的内容无法解析为合法的 workflow_json 结构
- **THEN** 系统 SHALL 触发重试（计入重试次数），全部失败后返回 500 错误 "工作流解析失败，请检查协作规范描述是否清晰"

### Requirement: 工作流 JSON 结构校验

系统 SHALL 对 LLM 生成的 workflow_json 进行结构校验和拓扑校验，确保工作流合法。

#### Scenario: 结构校验
- **WHEN** LLM 返回 workflow_json
- **THEN** 系统 SHALL 校验：(1) 必须包含 nodes 和 edges 数组 (2) 每个 node 必须有 id、type、position、data 字段 (3) 每个 node.data 必须有 label 和 activity_kind (4) activity_kind 必须为 request/approve/process/action/end 之一 (5) 每个 edge 必须有 source 和 target 且引用有效 node id

#### Scenario: 拓扑校验
- **WHEN** workflow_json 通过结构校验
- **THEN** 系统 SHALL 校验：(1) 有且仅有一个 activity_kind=request 的起始节点 (2) 有且仅有一个 activity_kind=end 的结束节点 (3) 从起始节点到结束节点存在至少一条完整路径 (4) 不存在孤立节点（无入边也无出边，除起始节点无入边、结束节点无出边外）

#### Scenario: 校验失败触发重试
- **WHEN** 校验发现问题
- **THEN** 系统 SHALL 将校验错误附加到下一次 LLM 调用的 prompt 中作为修正提示，重新请求 LLM 生成

### Requirement: 解析引擎内置约束提示词

系统 SHALL 维护内置的工作流解析约束提示词（PathBuilder System Prompt），定义 LLM 生成工作流时必须遵循的规则。formSchema 字段类型 SHALL 与 intake form schema 体系对齐。

#### Scenario: formSchema 字段类型扩展
- **WHEN** PathBuilder prompt 描述 formSchema 可用字段类型
- **THEN** 可选值 SHALL 包含：`text, textarea, select, number, date, checkbox, email, url, radio, datetime, user_picker, dept_picker, rich_text, switch, multi_select, date_range, table`
- **AND** prompt SHALL 说明高级类型（user_picker、dept_picker、rich_text、table 等）仅在协作规范明确需要时使用

#### Scenario: 约束提示词内容保持完整
- **WHEN** 工作流解析引擎调用 LLM
- **THEN** system_prompt SHALL 仍包含所有现有约束（JSON 格式、节点类型枚举、参与人格式、process 节点双出边规则、排他网关格式、布局规则）

### Requirement: 解析结果保存

系统 SHALL 支持将解析生成的 workflow_json 保存到服务定义。

#### Scenario: 保存工作流到服务定义
- **WHEN** 前端获取解析结果后调用 `PUT /api/v1/itsm/services/:id` 保存
- **THEN** 系统 SHALL 将 workflow_json 更新到 ServiceDefinition 的 workflow_json 字段

### Requirement: 验证器 formSchema 引用校验
`ValidateWorkflow` SHALL 对排他网关条件中引用的 `form.xxx` 字段进行 formSchema 存在性校验。

#### Scenario: 条件引用的字段在上游 form 节点 formSchema 中存在
- **WHEN** 排他网关出边条件引用 `form.request_kind`，且网关上游路径中存在 form 节点，其 formSchema.fields 包含 key=`request_kind` 的字段
- **THEN** 验证 SHALL 通过，不产生 warning

#### Scenario: 条件引用的字段在上游 form 节点 formSchema 中不存在
- **WHEN** 排他网关出边条件引用 `form.urgency`，但网关上游路径中所有 form 节点的 formSchema.fields 均不包含 key=`urgency` 的字段
- **THEN** 验证 SHALL 产生 warning（非 error）：`"排他网关 {nodeID} 的条件引用了 form.urgency，但上游 form 节点的 formSchema 中未找到该字段"`

#### Scenario: 无上游 form 节点时跳过
- **WHEN** 排他网关出边条件引用 `form.xxx`，但网关的所有上游路径中不存在 form 类型节点
- **THEN** 验证 SHALL 跳过该检查，不产生 warning（字段可能来自 intake form 或变量映射）

#### Scenario: 多个上游 form 节点取并集
- **WHEN** 排他网关有多条上游路径，各路径经过不同 form 节点
- **THEN** 验证 SHALL 取所有可达 form 节点的 formSchema.fields 的 key 并集，只要引用的字段在并集中即通过

#### Scenario: 条件字段不以 form. 开头时跳过
- **WHEN** 排他网关出边条件引用的 field 不以 `form.` 开头
- **THEN** formSchema 引用校验 SHALL 跳过该条件

#### Scenario: form 节点无 formSchema 时跳过
- **WHEN** 上游 form 节点的 data 中未配置 formSchema
- **THEN** 验证 SHALL 跳过该 form 节点，不视为错误
