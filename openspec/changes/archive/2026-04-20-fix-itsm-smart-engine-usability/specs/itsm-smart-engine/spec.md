## MODIFIED Requirements

### Requirement: ReAct 决策循环

SmartEngine SHALL 通过 ReAct 循环调用 Agent，使用引擎配置中指定的决策智能体的 LLM 配置发起执行。Agent 在循环中可使用决策域工具按需获取信息，最终输出 DecisionPlan JSON。DecisionPlan 解析 SHALL 使用 `llm.ExtractJSON()` 公共函数（含 jsonrepair 修复能力）。当 ChatRequest 的 ResponseFormat 可用时，SHALL 设置 `ResponseFormat{Type: "json_object"}` 以提高结构化输出的可靠性。`decision_mode` 仅用于向 Agent 注入 workflow/collaboration hints，不得将 SmartEngine 降级为规则优先的确定性流程推进器。

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
- **AND** 这些摘要 SHALL 仅作为决策 hints，而非替代 Agent 推理的硬编码路由表
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

### Requirement: executeDecisionPlan 并签分支
`executeDecisionPlan()` SHALL 在 `ExecutionMode == "parallel"` 时创建并签活动组，而非逐个覆盖 `current_activity_id`。当 `ExecutionMode` 为空或 `"single"` 且 Agent 返回多个 activities 时，引擎 SHALL 只创建当前要执行的第一个活动，剩余步骤由后续决策循环基于最新上下文继续决定，不得一次性把 SmartEngine 退化为预排好的规则流程。

#### Scenario: parallel 模式创建活动组
- **WHEN** `executeDecisionPlan()` 处理 `ExecutionMode == "parallel"` 的 DecisionPlan
- **THEN** SHALL 生成一个 UUID 作为 `activity_group_id`
- **AND** SHALL 为 activities 中的每个条目创建独立 TicketActivity，设置相同的 `activity_group_id`
- **AND** SHALL 将工单 `current_activity_id` 设为组内第一个 activity ID

#### Scenario: single 模式只创建当前活动
- **WHEN** `executeDecisionPlan()` 处理 `ExecutionMode` 为空或 `"single"` 的 DecisionPlan 且 activities 包含多个条目
- **THEN** SHALL 只创建第一个 activity 并设为 current
- **AND** SHALL 不预先创建后续 activity 记录
- **AND** 后续步骤 SHALL 由下一轮决策循环根据最新工单上下文重新决定

### Requirement: SmartEngine continuation trigger points
SmartEngine SHALL 在真正完成一个 smart 活动边界时近实时提交 `itsm-smart-progress` 续跑任务，而不是依赖轮询式推进。触发点至少包括：人工审批/处理完成、action 活动完成、AI `pending_approval` 决策被确认、AI `pending_approval` 决策被拒绝。

#### Scenario: 人工审批完成后近实时续跑
- **WHEN** smart 工单的当前人工活动完成并提交结果
- **THEN** 系统 SHALL 在该完成事务成功后提交 `itsm-smart-progress` 任务
- **AND** 下一轮决策 SHALL 无需等待周期性扫描才开始

#### Scenario: action 完成后近实时续跑
- **WHEN** smart 工单的 action 活动执行完成
- **THEN** 系统 SHALL 在 action 完成后提交 `itsm-smart-progress` 任务
- **AND** 引擎 SHALL 基于 action 结果进入下一轮决策

#### Scenario: AI 决策确认后近实时续跑
- **WHEN** status=`pending_approval` 的 AI 活动被授权用户确认
- **THEN** 系统 SHALL 应用该决策并提交 `itsm-smart-progress` 任务

#### Scenario: AI 决策拒绝后近实时续跑
- **WHEN** status=`pending_approval` 的 AI 活动被授权用户拒绝
- **THEN** 系统 SHALL 记录拒绝结果与理由并提交 `itsm-smart-progress` 任务
- **AND** 下一轮决策 SHALL 在包含拒绝上下文的前提下重新运行

#### Scenario: 并发触发续跑不重复推进状态
- **WHEN** 同一 smart 工单因接近同时的完成事件多次提交 `itsm-smart-progress`
- **THEN** 引擎 SHALL 在进入决策前重新检查当前 ticket/activity 状态
- **AND** 重复提交 SHALL 不得导致同一逻辑步骤被重复创建或重复完成
