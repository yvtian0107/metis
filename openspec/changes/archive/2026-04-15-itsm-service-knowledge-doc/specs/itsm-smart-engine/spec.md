## MODIFIED Requirements

### Requirement: Agent 调用机制
SmartEngine SHALL 通过 AI App 的 LLM Client 调用 Agent。使用服务定义绑定的 `agent_id` 获取 Agent 配置（模型、system_prompt、temperature），构建完整的消息序列进行调用。

#### Scenario: 构建 Agent 调用上下文
- **WHEN** 引擎准备调用 Agent
- **THEN** 系统 SHALL 构建消息序列：
  - system message: `[Collaboration Spec 全文]\n\n---\n\n[Agent 自身 system_prompt]\n\n---\n\n[输出格式要求：JSON Schema]`
  - user message: `[TicketCase 快照 JSON]\n\n---\n\n[TicketPolicySnapshot JSON]\n\n请根据以上工单上下文和策略约束，输出下一步决策。`

#### Scenario: 使用服务知识文档补充上下文
- **WHEN** 服务定义关联的 ServiceKnowledgeDocument 中有 `parse_status=completed` 的文档
- **THEN** 系统 SHALL 收集所有已完成解析的文档的 `parsed_text`，拼接后作为 knowledge_context 注入 system message，格式为 `## 服务知识文档\n\n### {file_name}\n{parsed_text}\n\n`

#### Scenario: 无已解析文档
- **WHEN** 服务定义没有已完成解析的知识文档
- **THEN** 系统 SHALL 跳过知识注入，仅使用 Collaboration Spec 和工单上下文

#### Scenario: Agent 配置的模型不可用
- **WHEN** 绑定的 Agent 引用的 LLM Provider/Model 处于禁用状态或配置错误
- **THEN** 系统 SHALL 跳过 AI 决策，ai_failure_count +1，将工单放入人工决策队列并记录原因

#### Scenario: 使用 Agent 的 temperature 配置
- **WHEN** 调用 LLM Client
- **THEN** 系统 SHALL 使用 Agent 配置的 temperature（流程决策 Agent 默认 0.2，低温度确保决策稳定性）

### Requirement: 知识库作为补充上下文
服务定义关联的 ServiceKnowledgeDocument（已解析文档）SHALL 在 Agent 决策时作为补充上下文注入。

#### Scenario: 知识文档注入
- **WHEN** 服务定义有 `parse_status=completed` 的 ServiceKnowledgeDocument
- **THEN** 系统 SHALL 将所有已解析文档的 `parsed_text` 拼接，格式化后作为 system prompt 的补充部分：`## 服务知识文档\n\n### {file_name}\n{parsed_text}`

#### Scenario: 无已解析文档
- **WHEN** 服务定义没有已完成解析的知识文档
- **THEN** 系统 SHALL 跳过知识注入，不影响决策流程

### Requirement: 智能服务配置 UI
管理员在服务定义编辑器中 SHALL 能够配置智能引擎的相关参数。当 `engine_type="smart"` 时显示智能配置面板。

#### Scenario: Collaboration Spec 编辑器
- **WHEN** 管理员编辑智能服务
- **THEN** 系统 SHALL 提供 Markdown 文本编辑器（Textarea）用于编写 Collaboration Spec，支持预览

#### Scenario: Agent 选择器
- **WHEN** 管理员编辑智能服务
- **THEN** 系统 SHALL 提供 Agent 下拉选择器（从 AI App 获取 Agent 列表），选择后保存 `agent_id` 到 ServiceDefinition

#### Scenario: 附件知识文档管理
- **WHEN** 管理员编辑智能服务
- **THEN** 系统 SHALL 提供"附件知识"卡片，支持上传、列出、删除服务专属知识文档，替代原有的全局知识库多选

#### Scenario: 信心阈值设置
- **WHEN** 管理员编辑智能服务
- **THEN** 系统 SHALL 提供信心阈值滑块（0.0-1.0，步长 0.05，默认 0.8），保存到 `agent_config.confidence_threshold`

#### Scenario: 决策超时设置
- **WHEN** 管理员编辑智能服务
- **THEN** 系统 SHALL 提供决策超时输入框（秒，默认 30，范围 10-120），保存到 `agent_config.decision_timeout_seconds`

#### Scenario: AI App 不可用时
- **WHEN** AI App 未安装
- **THEN** 系统 SHALL 禁用 `engine_type="smart"` 选项（灰掉），提示 "需要安装 AI 模块才能使用智能引擎"

## REMOVED Requirements

### Requirement: 知识库绑定
**Reason**: 全局 AI 知识库引用被服务专属知识文档替代。`knowledge_base_ids` 字段和全局知识库多选组件将被移除。
**Migration**: 使用新的"附件知识"功能上传服务相关文档，系统自动解析并在 Agent 决策时注入。
