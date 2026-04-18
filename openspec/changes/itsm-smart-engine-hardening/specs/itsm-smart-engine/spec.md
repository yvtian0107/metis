## MODIFIED Requirements

### Requirement: ReAct 决策循环

SmartEngine SHALL 通过 ReAct 循环调用 Agent，使用引擎配置中指定的决策智能体的 LLM 配置发起执行。Agent 在循环中可使用决策域工具按需获取信息，最终输出 DecisionPlan JSON。DecisionPlan 解析 SHALL 使用 `llm.ExtractJSON()` 公共函数（含 jsonrepair 修复能力）。当 ChatRequest 的 ResponseFormat 可用时，SHALL 设置 `ResponseFormat{Type: "json_object"}` 以提高结构化输出的可靠性。

#### Scenario: 构建 Agent 调用上下文
- **WHEN** 引擎准备启动 ReAct 循环
- **THEN** 系统 SHALL 通过 EngineConfigProvider 获取 `DecisionAgentID()`，再通过 AgentProvider 按 agent_id 获取完整 agent 记录，使用其 model_id 和 temperature 作为 LLM 调用参数，构建消息序列：
  - system message: `[Collaboration Spec]\n\n---\n\n[Agent system_prompt]\n\n---\n\n[DecisionMode 提示词注入]\n\n---\n\n[工具使用指引]\n\n---\n\n[最终输出格式要求]`
  - user message: `[精简初始 seed JSON]\n\n[策略约束 JSON]\n\n请通过工具获取所需信息，然后输出决策。`
- **AND** ChatRequest SHALL 携带 `Tools` 字段包含所有决策域工具定义（含 `decision.execute_action`）
- **AND** ChatRequest SHALL 携带 `ResponseFormat: &llm.ResponseFormat{Type: "json_object"}` 以引导 LLM 在最终输出时返回合法 JSON

#### Scenario: 决策 agent 未配置
- **WHEN** 引擎准备启动 ReAct 循环且 `DecisionAgentID()` 返回 0
- **THEN** 系统 SHALL 返回错误 "决策智能体未配置"

#### Scenario: DecisionMode 提示词注入
- **WHEN** 构建 Agent system prompt 且 SystemConfig `itsm.engine.decision.decision_mode` 值为 `direct_first`
- **THEN** 系统 SHALL 从 WorkflowJSON 提取结构化工作流步骤摘要（节点类型、标签、参与人配置、网关条件），作为 `## 工作流参考路径` section 注入 system prompt
- **AND** 提取失败时 SHALL 退化为 ai_only 模式并记录 warning 日志

#### Scenario: DecisionMode ai_only 提示词注入
- **WHEN** 构建 Agent system prompt 且 SystemConfig `itsm.engine.decision.decision_mode` 值为 `ai_only`
- **THEN** 系统 SHALL 在 system prompt 中注入提示词："始终使用 AI 推理决定下一步，不依赖预定义路径"

#### Scenario: Agent 多轮工具调用后输出决策
- **WHEN** Agent 在 ReAct 循环中调用了工具后停止工具调用
- **THEN** 系统 SHALL 使用 `llm.ExtractJSON()` 从 Agent 的最终文本输出中提取并修复 JSON，再解析为 DecisionPlan

#### Scenario: Agent 首轮直接输出决策（简单场景）
- **WHEN** Agent 在 ReAct 循环第 1 轮即不调用任何工具直接输出 DecisionPlan
- **THEN** 系统 SHALL 正常解析 DecisionPlan，不强制要求 Agent 必须使用工具

#### Scenario: 格式纠正在循环内自然处理
- **WHEN** Agent 输出的内容经 `llm.ExtractJSON()` 处理后仍无法解析为 DecisionPlan JSON
- **THEN** 引擎 SHALL 视为决策失败调用 `handleDecisionFailure()`，不再有独立的 `callAgentWithCorrection()` 重试机制
