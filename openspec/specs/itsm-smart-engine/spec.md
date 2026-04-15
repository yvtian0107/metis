### Requirement: SmartEngine 实现 WorkflowEngine 接口
SmartEngine SHALL 实现与 ClassicEngine 相同的 `WorkflowEngine` 接口（Start/Progress/Cancel），通过 AI Agent 驱动的决策循环替代确定性图遍历。SmartEngine 通过 IOC 注入 AI App 的 AgentService 和 LLM Client。

#### Scenario: 启动智能流程
- **WHEN** `SmartEngine.Start()` 被调用且服务的 `engine_type="smart"`
- **THEN** 引擎 SHALL 构建初始 TicketCase 快照，调用 AI Agent 生成第一步决策计划，根据计划创建第一个 TicketActivity，工单状态从 `pending` 转为 `in_progress`

#### Scenario: 推进智能流程
- **WHEN** `SmartEngine.Progress()` 被调用且当前 Activity 已完成（outcome 非空）
- **THEN** 引擎 SHALL 更新 TicketCase 快照（含最新 Activity 结果），重新调用 AI Agent 决策下一步，创建新的 TicketActivity

#### Scenario: 取消智能流程
- **WHEN** `SmartEngine.Cancel()` 被调用
- **THEN** 引擎 SHALL 将当前未完成的 Activity 标记为取消，工单状态更新为 `cancelled`，记录取消原因到 Timeline

#### Scenario: AI App 不可用时禁止启动
- **WHEN** `SmartEngine.Start()` 被调用但 IOC 中无法解析 AI App 服务
- **THEN** 引擎 SHALL 返回错误 "智能引擎不可用：AI 模块未安装"

### Requirement: 决策循环核心流程
SmartEngine 的每一步决策 SHALL 遵循以下循环：TicketCase 快照构建 -> TicketPolicySnapshot 编译 -> Agent Decision -> Validate -> Progress。每一步都会生成新的 TicketActivity 记录。决策循环通过 Scheduler 异步任务 `itsm-smart-progress` 执行。

#### Scenario: 完整决策循环
- **WHEN** 一个 Activity 完成需要决定下一步
- **THEN** 引擎 SHALL 依次执行：(1) 构建 TicketCase 快照 (2) 编译 TicketPolicySnapshot 约束 (3) 调用 Agent 获取 TicketDecisionPlan (4) 校验 DecisionPlan 合法性 (5) 根据信心分数决定自动执行或等待人工确认

#### Scenario: Agent 决定流程结束
- **WHEN** Agent 的 DecisionPlan 中 `next_step_type` 为 `"complete"`
- **THEN** 引擎 SHALL 将工单状态更新为 `completed`，记录 `finished_at`，在 Timeline 添加完结记录

#### Scenario: 决策循环异步执行
- **WHEN** Activity 完成触发 Progress
- **THEN** 系统 SHALL 通过 Scheduler 的 `itsm-smart-progress` 异步任务执行决策循环，避免阻塞 HTTP 请求

### Requirement: TicketCase 快照构建
系统 SHALL 为每次 Agent 决策构建完整的 TicketCase 快照，包含 Agent 做出正确决策所需的全部上下文。

快照字段：
- `ticket`: 工单基本信息（code、summary、description、priority、status、source、form_data）
- `service`: 服务定义信息（name、description、engine_type、collaboration_spec）
- `collaboration_spec`: Collaboration Spec 全文（从 ServiceDefinition 获取）
- `sla_status`: SLA 状态（response_deadline 剩余时间、resolution_deadline 剩余时间；无 SLA 时为空）
- `activity_history`: 已完成的 Activity 列表摘要（type、outcome、operator_name、completed_at、ai_reasoning）
- `current_assignment`: 当前 Assignment（如有：assignee_name、role）
- `form_data`: 工单表单数据的完整 JSON

#### Scenario: 快照包含 SLA 状态
- **WHEN** 构建 TicketCase 快照且工单关联了 SLA 模板
- **THEN** 系统 SHALL 计算当前距 SLA 响应时限和解决时限的剩余时间（秒），包含在快照的 `sla_status` 字段中

#### Scenario: 快照包含历史活动
- **WHEN** 构建 TicketCase 快照且工单已有多个已完成的 Activity
- **THEN** 快照 SHALL 包含所有已完成 Activity 的摘要（type、outcome、operator_name、completed_at），按时间顺序排列

#### Scenario: 快照包含表单数据
- **WHEN** 构建 TicketCase 快照且工单有 form_data
- **THEN** 快照 SHALL 包含完整的 form_data JSON，供 Agent 理解用户提交的信息

#### Scenario: 无 SLA 的工单
- **WHEN** 构建 TicketCase 快照且工单未关联 SLA 模板
- **THEN** 快照的 `sla_status` 字段 SHALL 为空（null/nil），Agent 不应考虑 SLA 约束

### Requirement: TicketPolicySnapshot 编译
系统 SHALL 为每次 Agent 决策编译 TicketPolicySnapshot，定义 Agent 在当前状态下可采取的合法操作。Policy 约束了 Agent 的行为边界。

Policy 字段：
- `allowed_step_types`: 允许的 activity_type 列表（如 `["approve", "process", "action", "notify", "form", "complete", "escalate"]`）
- `participant_candidates`: 可用的参与人列表（用户 ID + 姓名 + 部门 + 职位，从 Org App 查询）
- `available_actions`: 可用的 ServiceAction 列表（ID、名称、描述、webhook URL）
- `allowed_status_transitions`: 当前工单状态允许的状态转换列表
- `current_status`: 当前工单状态

#### Scenario: 编译可用动作列表
- **WHEN** 编译 Policy 且服务定义关联了 3 个 ServiceAction
- **THEN** Policy SHALL 包含这 3 个动作的 ID、名称、描述供 Agent 选择

#### Scenario: 编译参与人候选列表
- **WHEN** 编译 Policy
- **THEN** Policy SHALL 查询系统中具有 ITSM 相关角色的用户列表，包含 user_id、name、department、position 供 Agent 选择派单目标

#### Scenario: 已完结工单不可操作
- **WHEN** 编译 Policy 且工单状态为 `completed` 或 `cancelled`
- **THEN** Policy SHALL 返回空的 `allowed_step_types` 列表

#### Scenario: Org App 不可用时
- **WHEN** 编译 Policy 且 IOC 中无法解析 Org App 服务
- **THEN** Policy 的 `participant_candidates` SHALL 仅包含系统中的全部活跃用户（无部门/职位信息），不影响决策流程

### Requirement: TicketDecisionPlan 结构
Agent 的决策输出 SHALL 符合 TicketDecisionPlan 结构化 JSON 格式。

```json
{
  "next_step_type": "process",
  "activities": [
    {
      "type": "process",
      "participant_type": "user",
      "participant_id": 42,
      "action_id": null,
      "instructions": "请检查用户的 VPN 配置并重置连接"
    }
  ],
  "reasoning": "根据用户描述的 VPN 连接问题，需要网络运维人员检查配置...",
  "confidence": 0.85
}
```

字段定义：
- `next_step_type`: 下一步类型，枚举值 `"approve" | "process" | "action" | "notify" | "form" | "complete" | "escalate"`
- `activities`: Activity 配置数组，每项含 type、participant_type、participant_id、action_id、instructions
- `reasoning`: 决策推理过程（文本，供人工审核）
- `confidence`: 决策信心分数（0.0-1.0）

#### Scenario: Agent 输出合法 DecisionPlan
- **WHEN** Agent 返回 JSON 格式的 DecisionPlan 且 `next_step_type` 在 Policy 的 `allowed_step_types` 中
- **THEN** 引擎 SHALL 解析该 DecisionPlan，进入信心评估流程

#### Scenario: Agent 输出非法 next_step_type
- **WHEN** Agent 返回的 `next_step_type` 不在 Policy 的 `allowed_step_types` 中
- **THEN** 引擎 SHALL 拒绝该决策，ai_failure_count +1，记录错误到 Activity 的 `ai_reasoning` 字段

#### Scenario: Agent 输出解析失败
- **WHEN** Agent 返回的内容无法解析为合法 JSON
- **THEN** 引擎 SHALL 重试一次（附带格式纠正提示："请严格按照以下 JSON 格式输出..."），仍失败则 ai_failure_count +1 并记录错误

#### Scenario: 参与人不在候选列表中
- **WHEN** Agent 指定的 `participant_id` 不在 Policy 的 `participant_candidates` 中
- **THEN** 引擎 SHALL 拒绝该决策，ai_failure_count +1，记录 "指定的参与人不在候选列表中"

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

### Requirement: 信心机制
SmartEngine SHALL 根据 Agent 返回的 `confidence` 值和服务定义的 `agent_config.confidence_threshold` 对比，决定是否自动执行决策。

#### Scenario: 高信心自动执行
- **WHEN** Agent 返回 `confidence` >= 服务的 `confidence_threshold`
- **THEN** 引擎 SHALL 自动执行 DecisionPlan，创建对应的 TicketActivity，Activity 的 `ai_decision` 字段记录 DecisionPlan JSON，`ai_reasoning` 记录推理过程，`confidence` 记录信心分数

#### Scenario: 低信心等待人工确认
- **WHEN** Agent 返回 `confidence` < 服务的 `confidence_threshold`
- **THEN** 引擎 SHALL 将 DecisionPlan 记录到 Activity 的 `ai_decision` 字段，Activity 状态设为 `pending_approval`，在 Timeline 添加记录 "AI 决策信心不足（{confidence}），等待人工确认"

#### Scenario: 人工确认 AI 决策
- **WHEN** 授权用户查看低信心决策并调用确认 API（`POST /api/v1/itsm/tickets/:id/activities/:aid/confirm`）
- **THEN** 系统 SHALL 按原 DecisionPlan 执行，Activity 状态从 `pending_approval` 变为 `in_progress`

#### Scenario: 人工拒绝 AI 决策
- **WHEN** 授权用户查看低信心决策并调用拒绝 API（`POST /api/v1/itsm/tickets/:id/activities/:aid/reject`）并提供拒绝原因
- **THEN** 系统 SHALL 丢弃 AI DecisionPlan，Activity 标记为 `rejected`，记录 `overridden_by` 为操作用户 ID，允许用户手动选择下一步操作

#### Scenario: 默认信心阈值
- **WHEN** 服务定义的 `agent_config` 未设置 `confidence_threshold`
- **THEN** 系统 SHALL 使用默认值 0.8

### Requirement: 人工覆盖
SmartEngine 的每个 AI 决策点 SHALL 支持人工覆盖，允许授权用户强制改变流程走向。所有覆盖操作 MUST 记录 `overridden_by`（用户 ID）和覆盖原因。

#### Scenario: 强制跳转
- **WHEN** 授权用户对智能工单调用强制跳转 API（`POST /api/v1/itsm/tickets/:id/override/jump`），指定目标 activity_type 和参与人
- **THEN** 系统 SHALL 取消当前 Activity（如有），创建用户指定的新 Activity，记录 `overridden_by` 和覆盖原因到 Timeline

#### Scenario: 改派
- **WHEN** 授权用户对智能工单调用改派 API（`POST /api/v1/itsm/tickets/:id/override/reassign`），指定新的处理人
- **THEN** 系统 SHALL 更新当前 Assignment 的 `assignee_id`，记录改派操作到 Timeline（原处理人 -> 新处理人）

#### Scenario: 驳回
- **WHEN** 授权用户对智能工单调用驳回 API（`POST /api/v1/itsm/tickets/:id/override/reject`）
- **THEN** 系统 SHALL 取消当前 Activity，触发新一轮决策循环（重新调用 Agent），在 Timeline 记录驳回原因

#### Scenario: 覆盖权限检查
- **WHEN** 非 itsm_admin 角色的用户尝试人工覆盖操作
- **THEN** 系统 SHALL 返回 403 Forbidden

### Requirement: 决策超时处理
SmartEngine 的 Agent 调用 SHALL 有超时控制，使用 `context.WithTimeout`。超时时间由服务定义的 `agent_config.decision_timeout_seconds` 配置，缺省 30 秒。

#### Scenario: 决策超时
- **WHEN** Agent 调用超过配置的超时时间仍未返回
- **THEN** 引擎 SHALL 取消该调用（context cancel），ai_failure_count +1，将工单放入人工决策队列，Activity 记录 `ai_reasoning` 为 "决策超时（{timeout}s）"

#### Scenario: 自定义超时
- **WHEN** 服务定义的 `agent_config.decision_timeout_seconds` 配置为 60
- **THEN** 引擎 SHALL 使用 60 秒作为该服务所有 Agent 调用的超时时间

#### Scenario: 默认超时
- **WHEN** 服务定义的 `agent_config` 未设置 `decision_timeout_seconds`
- **THEN** 系统 SHALL 使用默认值 30 秒

### Requirement: Fallback 降级策略
当 AI 服务不可用时，SmartEngine SHALL 进行降级处理，确保工单不会卡死。

#### Scenario: 单次失败转人工
- **WHEN** Agent 调用失败（网络错误/模型不可用/解析失败/校验不通过）
- **THEN** 引擎 SHALL 将工单放入人工决策队列，通知管理员有工单需要人工处理，在 Timeline 记录失败原因

#### Scenario: 连续 3 次失败自动停用 AI
- **WHEN** 同一工单的 ai_failure_count 达到 3
- **THEN** 引擎 SHALL 自动停用该工单的 AI 决策能力，不再尝试调用 Agent，直到管理员手动恢复。在 Timeline 记录 "AI 决策已停用（连续 3 次失败），请手动处理"

#### Scenario: 手动恢复 AI 决策
- **WHEN** 管理员调用恢复 API（`POST /api/v1/itsm/tickets/:id/override/retry-ai`）
- **THEN** 系统 SHALL 重置 ai_failure_count 为 0，重新构建 TicketCase 快照并调用 Agent

#### Scenario: 首次启动失败
- **WHEN** `SmartEngine.Start()` 调用 Agent 失败
- **THEN** 工单 SHALL 保持 `pending` 状态并放入人工决策队列，不自动转为 `in_progress`

### Requirement: 运行时流程可视化
系统 SHALL 在智能工单的详情页中动态展示已走过的流程路径。与经典引擎的预定义流程图不同，智能引擎的路径是根据 TicketActivity 历史实时生成的。

#### Scenario: 渲染动态流程图
- **WHEN** 用户查看智能工单的详情页
- **THEN** 前端 SHALL 根据工单的 TicketActivity 列表动态生成流程图：每个已完成的 Activity 作为一个节点（显示 type + outcome + operator），按 `sequence_order` 从左到右排列并连线

#### Scenario: 展示 AI 决策推理
- **WHEN** 用户点击流程图中某个由 AI 决策创建的节点
- **THEN** 系统 SHALL 在弹出面板中展示该 Activity 的 `ai_reasoning`（推理过程）和 `confidence`（信心分数）

#### Scenario: 标记人工覆盖节点
- **WHEN** 流程图中某个 Activity 的 `overridden_by` 不为空
- **THEN** 系统 SHALL 用特殊图标（如人形标记）标识该节点为人工覆盖，展示覆盖人姓名和原因

#### Scenario: 高亮当前等待节点
- **WHEN** 工单存在状态为 `pending_approval` 或 `in_progress` 的 Activity
- **THEN** 系统 SHALL 高亮当前活跃节点（脉冲动画或特殊颜色），并展示等待原因（如 "低信心待确认"、"等待处理人操作"）

#### Scenario: 空流程图
- **WHEN** 智能工单刚创建，还没有任何 Activity
- **THEN** 系统 SHALL 显示一个起始节点和 "等待 AI 决策..." 的提示

### Requirement: Collaboration Spec 作为首要上下文
Collaboration Spec（协作规范）SHALL 作为 Agent system prompt 的首要上下文，定义服务的处理规范、流程约束和质量要求。

#### Scenario: Spec 注入 System Prompt
- **WHEN** 构建 Agent 调用的 system prompt
- **THEN** Collaboration Spec 的全文 SHALL 被放在 system prompt 的最前面，格式为 `## 服务处理规范\n\n{spec_content}`

#### Scenario: 空 Spec 的处理
- **WHEN** 服务定义的 `collaboration_spec` 为空
- **THEN** Agent SHALL 仅依据自身的 system_prompt 和工单上下文做决策，system prompt 中跳过规范部分

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

### Requirement: Scheduler 异步任务 itsm-smart-progress
系统 SHALL 注册 `itsm-smart-progress` 异步任务到 Scheduler，用于执行智能决策循环。

#### Scenario: 任务触发
- **WHEN** 工单 Activity 完成且工单的 `engine_type="smart"`
- **THEN** 系统 SHALL 向 Scheduler 提交 `itsm-smart-progress` 异步任务，payload 包含 ticket_id 和 completed_activity_id

#### Scenario: 任务执行
- **WHEN** Scheduler 执行 `itsm-smart-progress` 任务
- **THEN** 系统 SHALL 加载工单和已完成 Activity，执行完整的决策循环（快照 -> Policy -> Agent -> 校验 -> 信心评估 -> 执行/等待）

#### Scenario: 任务超时
- **WHEN** `itsm-smart-progress` 任务执行超过 Scheduler 默认超时（30s）
- **THEN** Scheduler SHALL 按配置重试（默认 3 次），重试仍失败则记录失败日志
