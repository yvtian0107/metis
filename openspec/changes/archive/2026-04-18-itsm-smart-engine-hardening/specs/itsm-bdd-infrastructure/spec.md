## ADDED Requirements

### Requirement: 对话全链路 E2E 场景
系统 SHALL 提供 `vpn_e2e_dialog_flow.feature`，覆盖从用户自然语言对话到工单创建再到引擎触发的完整流程。

#### Scenario: 网络支持完整对话到创单
- **WHEN** 用户通过服务台 Agent 描述 VPN 网络支持需求并提供完整信息
- **THEN** Agent SHALL 完成服务匹配 → 服务装载 → 信息采集 → 草稿整理 → 确认 → 创单的完整流程
- **AND** 创单后智能引擎 SHALL 被触发，生成合法的决策活动

#### Scenario: 安全合规完整对话到创单
- **WHEN** 用户通过服务台 Agent 描述 VPN 安全合规需求并提供完整信息
- **THEN** Agent SHALL 完成完整对话流程并创建工单
- **AND** 智能引擎决策 SHALL 路由到安全相关岗位

### Requirement: 多对话模式覆盖场景
系统 SHALL 提供 `vpn_dialog_coverage.feature`，使用 Scenario Outline 覆盖 6 种对话模式。

#### Scenario: complete_direct 模式 — 用户一次性提供完整信息并确认
- **WHEN** 用户消息包含所有必填字段且语言清晰直接
- **THEN** Agent SHALL 在 1-2 轮交互内完成到 draft_confirm 的流程

#### Scenario: colloquial_complete 模式 — 口语化但信息完整
- **WHEN** 用户用口语化表述提供了所有必要信息
- **THEN** Agent SHALL 正确提取结构化信息并完成草稿

#### Scenario: multi_turn_fill_details 模式 — 多轮补充信息
- **WHEN** 用户首轮信息不完整，经 Agent 追问后补充
- **THEN** Agent SHALL 在追问后收集到完整信息并推进到 draft_prepare

#### Scenario: full_info_hold 模式 — 信息完整但用户不确认
- **WHEN** 用户提供了完整信息但未明确表示确认
- **THEN** Agent SHALL 完成 draft_prepare 但 SHALL NOT 调用 draft_confirm

#### Scenario: ambiguous_incomplete_hold 模式 — 模糊不完整
- **WHEN** 用户表述模糊且信息不完整
- **THEN** Agent SHALL 追问澄清，不强行推进草稿

#### Scenario: multi_turn_hold 模式 — 多轮对话中保持等待
- **WHEN** 用户在多轮对话中始终未提供足够信息或确认意图
- **THEN** Agent SHALL 保持在信息收集阶段，不跳过到 draft_confirm

### Requirement: 会话隔离场景
系统 SHALL 提供 `service_desk_session_isolation.feature`，验证同一会话中连续服务请求之间的状态隔离。

#### Scenario: 连续两次服务请求状态不继承
- **WHEN** 用户在同一会话中先完成一个 VPN 申请，再发起新的服务请求
- **THEN** 第二次请求的 draft_prepare SHALL 不包含第一次请求的表单数据

#### Scenario: new_request 重置后上下文干净
- **WHEN** Agent 调用 itsm.new_request 重置状态后用户描述新需求
- **THEN** service_match 和 service_load SHALL 基于新需求执行，不受前次会话污染

### Requirement: 知识驱动路由场景
系统 SHALL 提供 `service_knowledge_routing.feature`，验证知识库内容对引擎决策路由的影响。

#### Scenario: 知识命中变更窗口期 — 路由到安全管理员
- **WHEN** 工单创建时知识库中包含"变更窗口期需安全审批"的策略
- **THEN** 智能引擎决策 SHALL 将工单路由到 security_admin

#### Scenario: 知识未命中 — 走默认路由
- **WHEN** 工单创建时知识库搜索无匹配结果
- **THEN** 智能引擎决策 SHALL 按 collaboration_spec 的默认规则路由

#### Scenario: 知识库不可用 — 不阻塞决策
- **WHEN** KnowledgeSearcher 不可用（AI 知识模块未安装）
- **THEN** 智能引擎决策 SHALL 正常完成，knowledge_search 返回空结果不影响流程

### Requirement: 智能引擎恢复场景
系统 SHALL 提供 `smart_engine_recovery.feature`，验证 server 重启后决策循环的自动恢复。

#### Scenario: in_progress 无活跃活动的票据被恢复
- **WHEN** 存在 status=in_progress、engine_type=smart 的票据且无 pending/in_progress 活动
- **AND** 执行恢复任务
- **THEN** 系统 SHALL 提交 itsm-smart-progress 异步任务重新触发决策循环

#### Scenario: in_progress 有活跃活动的票据不重复触发
- **WHEN** 存在 status=in_progress、engine_type=smart 的票据且有 pending 活动
- **AND** 执行恢复任务
- **THEN** 系统 SHALL 跳过该票据，不提交额外任务
