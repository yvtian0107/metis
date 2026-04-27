## MODIFIED Requirements

### Requirement: 解析引擎内置约束提示词
系统 SHALL 维护内置的工作流解析约束提示词（PathBuilder System Prompt），定义 LLM 生成工作流时必须遵循的规则。formSchema 字段类型 SHALL 与 intake form schema 体系对齐。

#### Scenario: formSchema 字段类型扩展
- **WHEN** PathBuilder prompt 描述 formSchema 可用字段类型
- **THEN** 可选值 SHALL 包含：`text, textarea, select, number, date, checkbox, email, url, radio, datetime, user_picker, dept_picker, rich_text, switch, multi_select, date_range, table`
- **AND** prompt SHALL 说明高级类型（user_picker、dept_picker、rich_text、table 等）仅在协作规范明确需要时使用

#### Scenario: 约束提示词内容保持完整
- **WHEN** 工作流解析引擎调用 LLM
- **THEN** system_prompt SHALL 仍包含所有现有约束（JSON 格式、节点类型枚举、参与人格式、process 节点双出边规则、排他网关格式、布局规则）

## ADDED Requirements

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
