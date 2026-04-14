## ADDED Requirements

### Requirement: SmartEngine 实现 WorkflowEngine 接口
SmartEngine SHALL 实现与 ClassicEngine 相同的 WorkflowEngine 接口（Start/Progress/Cancel），通过 AI Agent 驱动的决策循环替代确定性图遍历。

#### Scenario: 启动智能流程
- **WHEN** SmartEngine.Start() 被调用
- **THEN** 引擎 SHALL 构建初始 TicketCase 快照，调用 AI Agent 生成第一步决策计划，并根据计划创建第一个 TicketActivity

#### Scenario: 推进智能流程
- **WHEN** SmartEngine.Progress() 被调用且当前 Activity 完成
- **THEN** 引擎 SHALL 更新 TicketCase 快照，重新调用 AI Agent 决策下一步

#### Scenario: 取消智能流程
- **WHEN** SmartEngine.Cancel() 被调用
- **THEN** 引擎 SHALL 将当前 Activity 标记为取消，工单状态更新为 "cancelled"

### Requirement: 决策循环核心流程
SmartEngine 的每一步决策 SHALL 遵循以下循环：TicketCase 快照构建 → TicketPolicySnapshot 编译 → Agent Decision → Validate → Progress。每一步都会生成新的 TicketActivity 记录。

#### Scenario: 完整决策循环
- **WHEN** 一个 Activity 完成需要决定下一步
- **THEN** 引擎 SHALL 依次执行：(1) 构建 TicketCase 快照 (2) 编译 Policy 约束 (3) 调用 Agent 获取 DecisionPlan (4) 校验 DecisionPlan 合法性 (5) 执行 DecisionPlan 创建新 Activity

#### Scenario: Agent 决定流程结束
- **WHEN** Agent 的 DecisionPlan 中 next_step_type 为 "complete"
- **THEN** 引擎 SHALL 将工单状态更新为 "completed"，记录 finished_at

### Requirement: TicketCase 快照构建
系统 SHALL 为每次 Agent 决策构建完整的 TicketCase 快照，包含 Agent 做出正确决策所需的全部上下文。快照内容包括：工单基本信息（code、summary、priority、status、form_data）、服务定义信息（name、description、SLA 配置）、Collaboration Spec（协作规范全文）、SLA 状态（剩余响应时间、剩余解决时间）、历史活动摘要（已完成的 Activity 列表及结果）。

#### Scenario: 快照包含 SLA 状态
- **WHEN** 构建 TicketCase 快照
- **THEN** 系统 SHALL 计算当前距 SLA 响应时限和解决时限的剩余时间，包含在快照中

#### Scenario: 快照包含历史活动
- **WHEN** 构建 TicketCase 快照且工单已有 3 个已完成的 Activity
- **THEN** 快照 SHALL 包含这 3 个 Activity 的摘要（type、outcome、operator、时间）

#### Scenario: 快照包含表单数据
- **WHEN** 构建 TicketCase 快照且工单有 form_data
- **THEN** 快照 SHALL 包含完整的 form_data JSON

### Requirement: TicketPolicySnapshot 编译
系统 SHALL 为每次 Agent 决策编译 TicketPolicySnapshot，定义 Agent 在当前状态下可采取的合法操作。Policy 内容包括：允许的 activity_type 列表（如 approve、process、action、notify、form、complete、escalate）、可用的参与人类型和候选人列表、可用的 ServiceAction 列表、当前工单状态允许的状态转换。

#### Scenario: 编译可用动作列表
- **WHEN** 编译 Policy 且服务定义关联了 3 个 ServiceAction
- **THEN** Policy SHALL 包含这 3 个动作的 ID、名称、描述供 Agent 选择

#### Scenario: 编译参与人候选列表
- **WHEN** 编译 Policy
- **THEN** Policy SHALL 查询可用的用户、职位、部门列表供 Agent 选择派单目标

#### Scenario: 已完成工单不可操作
- **WHEN** 编译 Policy 且工单状态为 "completed" 或 "cancelled"
- **THEN** Policy SHALL 返回空的允许操作列表

### Requirement: TicketDecisionPlan 结构
Agent 的决策输出 SHALL 符合 TicketDecisionPlan 结构化格式，包含：next_step_type（下一步类型："approve" | "process" | "action" | "notify" | "form" | "complete" | "escalate"）、activities（Activity 配置数组，每项含 type、participant 规则、action_id 等）、reasoning（决策推理过程，文本）、confidence（决策信心分数，0.0-1.0）。

#### Scenario: Agent 输出合法 DecisionPlan
- **WHEN** Agent 返回 JSON 格式的 DecisionPlan 且 next_step_type 在允许列表中
- **THEN** 引擎 SHALL 解析并执行该 DecisionPlan

#### Scenario: Agent 输出非法 next_step_type
- **WHEN** Agent 返回的 next_step_type 不在 Policy 允许的列表中
- **THEN** 引擎 SHALL 拒绝该决策，记录错误，将工单放入人工决策队列

#### Scenario: Agent 输出解析失败
- **WHEN** Agent 返回的内容无法解析为合法 JSON
- **THEN** 引擎 SHALL 重试一次（附带格式纠正提示），仍失败则转人工

### Requirement: Agent 调用机制
SmartEngine SHALL 通过 AI App 的 LLM Client 调用 Agent。使用服务定义绑定的 agent_id 对应的 Agent 配置（模型、system_prompt、temperature），将 Collaboration Spec 作为首要上下文，TicketCase 快照和 TicketPolicySnapshot 作为用户消息。

#### Scenario: 构建 Agent 调用上下文
- **WHEN** 引擎准备调用 Agent
- **THEN** 系统 SHALL 构建消息序列：(1) system_prompt 含 Agent 配置的 prompt + Collaboration Spec (2) user message 含 TicketCase 快照 + Policy 约束 + 输出格式要求

#### Scenario: 使用知识库补充上下文
- **WHEN** 服务定义的 knowledge_base_ids 不为空
- **THEN** 系统 SHALL 在调用前从关联知识库中检索与当前工单相关的知识节点，作为补充上下文注入

#### Scenario: Agent 配置的模型不可用
- **WHEN** 绑定的 Agent 引用的 LLM 模型处于禁用状态
- **THEN** 系统 SHALL 跳过 AI 决策，将工单放入人工决策队列并记录原因

### Requirement: 信心机制
SmartEngine SHALL 根据 Agent 返回的 confidence 值和服务定义的 confidence_threshold 配置决定是否自动执行决策。

#### Scenario: 高信心自动执行
- **WHEN** Agent 返回 confidence 为 0.9 且服务的 confidence_threshold 配置为 0.8
- **THEN** 引擎 SHALL 自动执行 DecisionPlan，创建对应的 TicketActivity

#### Scenario: 低信心等待人工确认
- **WHEN** Agent 返回 confidence 为 0.6 且服务的 confidence_threshold 配置为 0.8
- **THEN** 引擎 SHALL 将 DecisionPlan 记录到 Activity 的 ai_decision 字段，工单状态设为 "waiting_approval"，等待人工确认或覆盖

#### Scenario: 人工确认 AI 决策
- **WHEN** 管理员查看低信心决策并点击"确认执行"
- **THEN** 系统 SHALL 按原 DecisionPlan 执行，Activity 记录 overridden_by 为空（表示采纳）

#### Scenario: 人工拒绝 AI 决策
- **WHEN** 管理员查看低信心决策并点击"拒绝并手动决策"
- **THEN** 系统 SHALL 丢弃 AI DecisionPlan，允许管理员手动选择下一步操作，Activity 记录 overridden_by 为管理员 ID

### Requirement: 人工覆盖
SmartEngine 的每个 AI 决策点 SHALL 支持人工覆盖，允许授权用户强制改变流程走向。

#### Scenario: 强制跳转
- **WHEN** 管理员对智能工单执行"强制跳转"操作并指定目标活动类型
- **THEN** 系统 SHALL 取消当前 Activity，创建管理员指定的新 Activity，记录 overridden_by 和覆盖原因

#### Scenario: 改派
- **WHEN** 管理员对智能工单执行"改派"操作并指定新的处理人
- **THEN** 系统 SHALL 更新当前 Assignment 的 assignee_id，记录改派操作到 Timeline

#### Scenario: 驳回
- **WHEN** 管理员对智能工单执行"驳回"操作
- **THEN** 系统 SHALL 取消当前 Activity，工单状态回退到上一个需要用户输入的状态

### Requirement: 决策超时处理
SmartEngine 的 Agent 调用 SHALL 有超时控制，默认超时时间由服务定义的 agent_config.decision_timeout_seconds 配置，缺省为 30 秒。

#### Scenario: 决策超时
- **WHEN** Agent 调用超过配置的超时时间仍未返回
- **THEN** 引擎 SHALL 取消该调用，将工单放入人工决策队列，Activity 记录 ai_reasoning 为 "decision timeout"

#### Scenario: 自定义超时
- **WHEN** 服务定义的 agent_config.decision_timeout_seconds 配置为 60
- **THEN** 引擎 SHALL 使用 60 秒作为该服务所有 Agent 调用的超时时间

### Requirement: Fallback 降级策略
当 AI 服务不可用时，SmartEngine SHALL 按照服务定义的 agent_config.fallback_strategy 进行降级处理。

#### Scenario: Fallback 到人工队列
- **WHEN** Agent 调用失败（网络错误/模型不可用）且 fallback_strategy 为 "manual_queue"
- **THEN** 引擎 SHALL 将工单放入人工决策队列，通知管理员有工单需要人工处理

#### Scenario: 连续失败计数
- **WHEN** 同一工单的 Agent 调用连续失败 3 次
- **THEN** 引擎 SHALL 自动将工单转入人工决策队列，不再尝试 AI 决策，直到管理员手动恢复

#### Scenario: AI 服务恢复
- **WHEN** 工单在人工决策队列中且管理员选择"重新尝试 AI 决策"
- **THEN** 引擎 SHALL 重新构建 TicketCase 快照并调用 Agent

### Requirement: 运行时流程可视化
系统 SHALL 在智能工单的详情页中动态展示已走过的流程路径。与经典引擎的预定义流程图不同，智能引擎的路径是实时生成的。

#### Scenario: 渲染动态流程图
- **WHEN** 用户查看智能工单的详情页
- **THEN** 系统 SHALL 根据工单的 TicketActivity 历史记录动态生成流程图，每个已完成的 Activity 作为一个节点，按时间顺序连线

#### Scenario: 展示 AI 决策推理
- **WHEN** 用户点击流程图中某个由 AI 决策创建的节点
- **THEN** 系统 SHALL 展示该决策的 reasoning（推理过程）和 confidence（信心分数）

#### Scenario: 标记人工覆盖节点
- **WHEN** 流程图中某个 Activity 的 overridden_by 不为空
- **THEN** 系统 SHALL 用特殊标识标记该节点为人工覆盖，展示覆盖人和原因

#### Scenario: 当前等待节点
- **WHEN** 工单正在等待人工确认或处理
- **THEN** 系统 SHALL 高亮当前等待中的节点，并展示等待原因（如"低信心待确认"、"等待处理人认领"）

### Requirement: Collaboration Spec 作为首要上下文
Collaboration Spec SHALL 作为 Agent 系统提示的首要上下文，定义服务的处理规范、流程约束和质量要求。Agent 的所有决策 MUST 遵循 Spec 中定义的约束。

#### Scenario: Spec 注入 System Prompt
- **WHEN** 构建 Agent 调用的 system prompt
- **THEN** Collaboration Spec 的全文 SHALL 被注入到 system prompt 的显著位置，优先级高于 Agent 自身的 system_prompt

#### Scenario: 空 Spec 的处理
- **WHEN** 服务定义的 collaboration_spec 为空
- **THEN** Agent SHALL 仅依据自身的 system_prompt 和工单上下文做决策

### Requirement: 知识库作为补充上下文
服务定义的 knowledge_base_ids 关联的知识库 SHALL 在 Agent 决策时作为补充上下文。系统通过 AI App 的 Knowledge 模块检索相关知识。

#### Scenario: 知识检索注入
- **WHEN** 服务定义关联了知识库且知识库有已编译的知识节点
- **THEN** 系统 SHALL 基于工单摘要和当前活动检索相关知识节点，将检索结果作为补充上下文注入 Agent 调用

#### Scenario: 知识库为空或未编译
- **WHEN** 关联的知识库没有已编译的知识节点
- **THEN** 系统 SHALL 跳过知识注入，仅使用 Collaboration Spec 和工单上下文
