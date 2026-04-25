## ADDED Requirements

### Requirement: 协作规范解析 API

系统 SHALL 提供 `POST /api/v1/itsm/workflows/generate` API，接收协作规范和上下文信息，调用 LLM 解析生成 ReactFlow 格式的工作流 JSON。API 受 JWT + Casbin 权限保护。只要系统最终获得可解析的 workflow_json，API SHALL 返回 200，并在响应体中附带 workflow_json、重试次数、validation issues、保存状态、更新后的服务定义和发布健康检查结果。

请求结构：
```json
{
  "serviceId": 1,
  "collaborationSpec": "用户提交申请，收集数据库名..."
}
```

响应结构：
```json
{
  "workflowJson": {
    "nodes": [],
    "edges": []
  },
  "retries": 1,
  "errors": [
    {"nodeId": "node_security_admin", "level": "blocking", "message": "process 节点缺少 rejected 出边"}
  ],
  "saved": true,
  "service": {},
  "healthCheck": {}
}
```

#### Scenario: 成功解析协作规范
- **WHEN** 管理员调用解析 API 并传入有效的协作规范
- **THEN** 系统 SHALL 读取路径生成引擎配置构建 LLM Client，将协作规范、内置约束提示词和可用动作信息组合为 prompt，调用 LLM 获取结果，解析为 workflow_json 返回

#### Scenario: 生成草图存在 blocking 校验问题
- **WHEN** LLM 返回可解析 workflow_json，但重试后 `ValidateWorkflow` 仍返回 Level="blocking" 的 validation issues
- **THEN** API SHALL 返回 200
- **AND** 响应 SHALL 包含 workflowJson、errors、saved、service 和 healthCheck
- **AND** 系统 SHALL 保存该 workflow_json 作为参考路径草图
- **AND** healthCheck SHALL 将参考路径风险标记为 fail

#### Scenario: 生成草图只有 warning 校验问题
- **WHEN** LLM 返回可解析 workflow_json，且 `ValidateWorkflow` 只返回 Level="warning" 的 validation issues
- **THEN** API SHALL 返回 200
- **AND** 系统 SHALL 保存 workflow_json
- **AND** 响应 SHALL 包含 warning issues

#### Scenario: 引擎未配置
- **WHEN** 调用解析 API 但路径生成引擎模型未配置
- **THEN** 系统 SHALL 返回 400 错误，提示参考路径生成未配置模型

#### Scenario: 协作规范为空
- **WHEN** 调用解析 API 但 collaborationSpec 为空字符串
- **THEN** 系统 SHALL 返回 400 错误 "协作规范不能为空"

#### Scenario: LLM 调用失败
- **WHEN** LLM 调用返回错误或超时
- **THEN** 系统 SHALL 返回上游错误状态，不保存 workflow_json

#### Scenario: LLM 返回无效 JSON
- **WHEN** LLM 返回的内容无法解析为合法 workflow_json
- **THEN** 系统 SHALL 触发重试（计入重试次数）
- **AND** 全部失败后返回错误状态，不保存 workflow_json

### Requirement: 工作流 JSON 结构校验

系统 SHALL 对 LLM 生成的 workflow_json 进行结构校验和拓扑校验。校验结果 SHALL 分为 Level="blocking" 和 Level="warning"。生成阶段 SHALL 将校验问题反馈给 LLM 进行重试；最终仍存在校验问题时，校验结果 SHALL 随 workflow_json 返回并进入发布健康检查，而不是阻止生成响应。运行/发布前仍 SHALL 使用 blocking 校验阻止不可运行流程。

#### Scenario: 结构校验
- **WHEN** LLM 返回 workflow_json
- **THEN** 系统 SHALL 校验 nodes 和 edges 结构、节点类型、边引用、人工节点参与人配置、process 节点 outcome 出边、网关条件和已注册节点运行能力

#### Scenario: 拓扑校验支持多终点
- **WHEN** workflow_json 包含多个 type="end" 的终点节点，例如正常结束和驳回结束
- **THEN** dead-end 检测 SHALL 从所有 end 节点反向遍历
- **AND** 任一节点只要能到达任意一个 end 节点，就 SHALL NOT 被报告为无法到达终点
- **AND** end 节点本身 SHALL NOT 因无法到达另一个 end 节点而被报告为 dead-end

#### Scenario: 校验失败触发重试
- **WHEN** 校验发现 blocking 或 warning issues
- **THEN** 系统 SHALL 将校验错误附加到下一次 LLM 调用的 prompt 中作为修正提示，重新请求 LLM 生成，直到达到配置的最大重试次数

#### Scenario: 运行前 blocking 仍阻断
- **WHEN** 已保存的 workflow_json 在运行、发布或工单创建前存在 Level="blocking" 的 validation issues
- **THEN** 对应运行或发布入口 SHALL 阻止继续执行，并向用户暴露阻断原因

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

系统 SHALL 支持将解析生成的 workflow_json 保存到服务定义。生成阶段保存的是参考路径草图；保存后 SHALL 调用 RefreshPublishHealthCheck，将 blocking/warning validation issues 转换为服务发布健康状态。

#### Scenario: 保存无校验问题的工作流到服务定义
- **WHEN** 生成结果无 validation issues
- **THEN** 系统 SHALL 将 workflow_json 和 collaboration_spec 写入 ServiceDefinition
- **AND** 系统 SHALL 调用 RefreshPublishHealthCheck
- **AND** 响应 SHALL 包含 saved=true、service 和 healthCheck

#### Scenario: 保存存在 blocking issues 的参考路径草图
- **WHEN** 生成结果存在 Level="blocking" 的 validation issues
- **THEN** 系统 SHALL 将 workflow_json 和 collaboration_spec 写入 ServiceDefinition
- **AND** 系统 SHALL 调用 RefreshPublishHealthCheck
- **AND** 响应 SHALL 包含 saved=true、errors、service 和 healthCheck
- **AND** healthCheck SHALL 暴露参考路径阻塞项

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
