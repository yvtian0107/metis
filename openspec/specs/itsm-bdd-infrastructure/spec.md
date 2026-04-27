# itsm-bdd-infrastructure

## Purpose

为 ITSM 模块建立基于 godog 的 BDD 测试基础设施，包括 suite 入口、features 目录结构、共享 context 和 Makefile 集成。

## Requirements

### Requirement: godog 测试依赖引入
项目 SHALL 在 go.mod 中引入 `github.com/cucumber/godog` 作为测试依赖。

#### Scenario: godog 可在测试代码中导入
- **WHEN** 测试文件 import `github.com/cucumber/godog`
- **THEN** `go test` 编译成功

### Requirement: BDD suite 入口文件
系统 SHALL 提供 `bdd_test.go` 作为 godog 测试 suite 的入口，使用 `godog.TestSuite` 配置 features 路径和 scenario initializer。

#### Scenario: godog suite 可运行
- **WHEN** 执行 `go test ./internal/app/itsm/ -run TestBDD -v`
- **THEN** godog suite 启动，扫描 `features/` 目录
- **AND** 无 feature 文件时不报错（0 scenarios, 0 steps）

### Requirement: features 目录结构
系统 SHALL 在 `internal/app/itsm/features/` 下创建目录结构，包含 `.gitkeep` 和一个示例 `.feature` 文件说明格式约定。

#### Scenario: features 目录存在且包含格式说明
- **WHEN** 查看 `internal/app/itsm/features/` 目录
- **THEN** 目录存在，包含 `example.feature`（注释说明格式约定，标记为 @wip 不执行）

### Requirement: 共享 BDD test context
系统 SHALL 提供 `steps_common_test.go`，定义 `bddContext` 结构体作为所有 step definitions 的共享状态容器。

#### Scenario: bddContext 包含核心字段
- **WHEN** 查看 `bddContext` 结构体
- **THEN** 包含以下字段：db (*gorm.DB)、lastErr (error)
- **AND** 提供 `reset()` 方法在每个 Scenario 前重置状态

### Requirement: BDD 测试可通过 Makefile 运行
系统 SHALL 提供 `make test-bdd` target 运行 BDD 测试。

#### Scenario: make test-bdd 运行 godog suite
- **WHEN** 执行 `make test-bdd`
- **THEN** 运行 `go test ./internal/app/itsm/ -run TestBDD -v`

### Requirement: 删除旧 BDD 占位文件
旧的 `workflow_generate_bdd_test.go` 占位文件 SHALL 被删除，其内容并入新的 `bdd_test.go` 注释中。

#### Scenario: 旧占位文件不存在
- **WHEN** 查看 `internal/app/itsm/` 目录
- **THEN** `workflow_generate_bdd_test.go` 不存在

### Requirement: 确定性覆盖全部决策类型的 activity 创建

SmartEngine.ExecuteConfirmedPlan SHALL 正确创建 7 种决策类型（approve / process / action / notify / form / complete / escalate）的 TicketActivity 记录，并在 timeline 记录 `ai_decision_executed` 事件。

#### Scenario: process 类型决策创建处理活动
- **WHEN** 执行 crafted DecisionPlan（type=process, participant_id 指向有效用户）
- **THEN** 创建 status=pending 的 TicketActivity（activity_type=process）
- **AND** 创建 TicketAssignment 指向该用户
- **AND** ticket.assignee_id 更新为该用户

#### Scenario: action 类型决策创建自动动作活动
- **WHEN** 执行 crafted DecisionPlan（type=action, action_id 指向有效 ServiceAction）
- **THEN** 创建 status=in_progress 的 TicketActivity（activity_type=action）
- **AND** 不创建 TicketAssignment（action 无需参与者）

#### Scenario: notify 类型决策创建通知活动
- **WHEN** 执行 crafted DecisionPlan（type=notify）
- **THEN** 创建 status=in_progress 的 TicketActivity（activity_type=notify）

#### Scenario: form 类型决策创建表单填写活动
- **WHEN** 执行 crafted DecisionPlan（type=form, participant_id 指向有效用户）
- **THEN** 创建 status=pending 的 TicketActivity（activity_type=form）
- **AND** 创建 TicketAssignment 指向该用户

#### Scenario: escalate 类型决策创建升级活动
- **WHEN** 执行 crafted DecisionPlan（type=escalate）
- **THEN** 创建 status=in_progress 的 TicketActivity（activity_type=escalate）

#### Scenario: complete 类型决策直接完结工单
- **WHEN** 执行 crafted DecisionPlan（next_step_type=complete）
- **THEN** 工单 status 变为 completed
- **AND** 创建 activity_type=complete 的已完成活动
- **AND** timeline 包含 workflow_completed 事件

### Requirement: AI 决策失败递增 failure count

当 AI 决策失败时，SmartEngine SHALL 递增 ticket.ai_failure_count 并记录 `ai_decision_failed` timeline 事件。

#### Scenario: 单次决策失败后 failure count 变为 1
- **WHEN** 智能引擎决策失败（LLM 不可达或返回非法输出）
- **THEN** ticket.ai_failure_count 从 0 变为 1
- **AND** timeline 包含 ai_decision_failed 事件

### Requirement: 连续失败触发 AI 熔断

当 ticket.ai_failure_count 达到 MaxAIFailureCount (3) 时，SmartEngine SHALL 拒绝执行新的决策循环，记录 `ai_disabled` timeline 事件，并返回 ErrAIDisabled。

#### Scenario: ai_failure_count 已达 3 时决策循环直接拒绝
- **WHEN** ticket.ai_failure_count = 3 时执行决策循环
- **THEN** 返回 ErrAIDisabled
- **AND** timeline 包含 ai_disabled 事件
- **AND** 工单状态不变（不会变为 failed）

### Requirement: Cancel 取消智能引擎工单

SmartEngine.Cancel SHALL 取消工单所有活跃活动、取消待处理 assignment、将工单状态设为 cancelled，并记录 timeline。

#### Scenario: 取消有活跃审批活动的智能工单
- **WHEN** 工单有一个 status=pending 的审批活动
- **AND** 执行 SmartEngine.Cancel
- **THEN** 该活动 status 变为 cancelled
- **AND** 关联 assignment status 变为 cancelled
- **AND** 工单 status 变为 cancelled
- **AND** timeline 包含取消事件

### Requirement: 低置信度决策被人工拒绝

当管理员拒绝 pending_approval 的决策时，activity 状态 SHALL 变为 rejected，决策不执行。

#### Scenario: 管理员拒绝低置信度决策
- **WHEN** 存在 status=pending_approval 的活动
- **AND** 管理员将其标记为 rejected
- **THEN** 活动 status 变为 rejected
- **AND** 工单状态不变为 completed
- **AND** timeline 包含决策拒绝事件

### Requirement: 兜底用户无效时记录 warning

当 fallback assignee 配置了但该用户不存在或未激活时，tryFallbackAssignment SHALL 记录 `participant_fallback_warning` timeline 事件，不创建 assignment。

#### Scenario: 兜底用户已停用时记录 warning 而非分配
- **WHEN** 引擎配置兜底处理人为一个 is_active=false 的用户
- **AND** 执行无参与者的审批决策
- **THEN** 不创建 TicketAssignment
- **AND** ticket.assignee_id 不变
- **AND** timeline 包含 participant_fallback_warning 事件

### Requirement: 服务台 Agent 跨路由冲突识别

当用户的诉求涉及映射到不同审批路由分支的多个选项时，服务台 Agent SHALL 识别冲突并向用户澄清，而非替用户选择或直接提交。

#### Scenario: Agent 识别跨路由冲突并向用户澄清
- **WHEN** 用户消息中同时包含映射到不同路由分支的多种需求（如 "network_support" 属网络审批路由，"security" 属安全审批路由）
- **THEN** Agent 的工具调用序列中 SHALL 包含 `itsm.service_match` 和 `itsm.service_load`
- **AND** Agent SHALL 满足以下任一路径：
  - 路径 A：不调用 `itsm.draft_prepare`（在 draft 前根据 routing_field_hint 识别冲突）
  - 路径 B：调用了 `itsm.draft_prepare` 但不调用 `itsm.draft_confirm`（收到 resolved_values 后停止推进）
- **AND** Agent 的回复内容 SHALL 包含关于路由冲突或需要选择的澄清表述

### Requirement: 服务台 Agent 同路由多选合并

当用户提到的多个需求全部映射到同一审批路由分支时，服务台 Agent SHALL 合并处理并继续推进流程，不要求用户二选一。

#### Scenario: Agent 合并同路由多选后正常推进
- **WHEN** 用户消息中包含多种需求，但它们全部映射到同一路由分支（如 "network_support" 和 "remote_maintenance" 均属网络审批路由）
- **THEN** Agent 的工具调用序列中 SHALL 包含 `itsm.service_match`、`itsm.service_load` 和 `itsm.draft_prepare`
- **AND** `itsm.draft_prepare` 的 form_data 中路由字段 SHALL 为单个结构化值（非逗号分隔）
- **AND** Agent 的回复内容中 SHALL 不包含"请选择""二选一""冲突"等要求用户做排他选择的表述

### Requirement: 服务台 Agent 必填缺失追问

当用户提供的信息不足以填满服务表单的所有必填字段时，服务台 Agent SHALL 追问缺失字段，而非带着空字段直接提交草稿。

#### Scenario: Agent 追问缺失的必填字段
- **WHEN** 用户消息仅提供了模糊的服务需求（如 "帮我开个VPN"），缺少 vpn_type、access_period 等必填字段
- **THEN** Agent SHALL 满足以下任一路径：
  - 路径 A：不调用 `itsm.draft_prepare`（在 draft 前识别出必填字段缺失）
  - 路径 B：调用了 `itsm.draft_prepare` 但不调用 `itsm.draft_confirm`（收到 missing_required warnings 后停止推进）
- **AND** Agent 的回复内容 SHALL 包含对缺失信息的追问

### Requirement: dialog 测试框架记录 toolResults

`dialogTestState` SHALL 新增 `toolResults []toolResultRecord` 字段，在 `EventTypeToolResult` 事件中记录工具名称、输出内容和是否为错误。用于测试失败时的调试诊断。

#### Scenario: toolResults 在 agent 执行后可用
- **WHEN** 服务台 Agent 处理用户消息，执行过程中产生工具调用和结果
- **THEN** `dialogTestState.toolResults` SHALL 包含每次工具调用的结果记录
- **AND** 每条记录包含 Name (string)、Output (string)、IsError (bool)

### Requirement: 工具调用计数断言 step

系统 SHALL 提供 BDD step `{tool} 被调用至少 {n} 次`，断言指定工具在当前 scenario 的 toolCalls 中出现次数 ≥ n。

#### Scenario: 计数断言通过
- **WHEN** toolCalls 中 "itsm.service_load" 出现 3 次
- **AND** 断言 "service_load 被调用至少 2 次"
- **THEN** 断言 SHALL 通过

#### Scenario: 计数断言失败
- **WHEN** toolCalls 中 "itsm.service_load" 出现 1 次
- **AND** 断言 "service_load 被调用至少 2 次"
- **THEN** 断言 SHALL 失败并报告实际调用次数

### Requirement: 审批岗位分配断言 step

系统 SHALL 提供通用 BDD step `当前审批分配到岗位 "<position_code>"`，断言当前活动的 TicketAssignment 中 position 的 code 匹配期望值。该 step 不绑定具体服务类型，可被所有 BDD 场景复用。

#### Scenario: 岗位分配断言匹配
- **WHEN** 当前活动有 TicketAssignment 且关联的 Position.Code 为 "ops_admin"
- **AND** 执行断言 `当前审批分配到岗位 "ops_admin"`
- **THEN** 断言通过

#### Scenario: 岗位分配断言不匹配
- **WHEN** 当前活动有 TicketAssignment 且关联的 Position.Code 为 "network_admin"
- **AND** 执行断言 `当前审批分配到岗位 "ops_admin"`
- **THEN** 断言失败并报告实际岗位

### Requirement: 审批可见性断言 step

系统 SHALL 提供通用 BDD step `当前审批仅对 "<username>" 可见`，断言当前活动的 TicketAssignment 通过 position_department 解析后，仅指定用户在可处理人列表中。

#### Scenario: 可见性断言通过
- **WHEN** 当前审批分配到 it/ops_admin 岗位，且 ops-operator 是该岗位唯一成员
- **AND** 执行断言 `当前审批仅对 "ops-operator" 可见`
- **THEN** 断言通过（ops-operator 可见，network-operator 和 security-operator 不可见）

### Requirement: 越权认领失败断言 step

系统 SHALL 提供通用 BDD step `"<username>" 认领当前工单应失败`，尝试让指定用户认领当前活动的 assignment，断言操作失败（用户不在该 assignment 的 position_department 可处理人中）。

#### Scenario: 非目标岗位用户认领失败
- **WHEN** 当前审批分配到 it/ops_admin 岗位
- **AND** network-operator（属于 it/network_admin）尝试认领
- **THEN** 认领操作失败

### Requirement: 越权审批失败断言 step

系统 SHALL 提供通用 BDD step `"<username>" 审批当前工单应失败`，尝试让指定用户直接审批当前活动，断言操作失败。

#### Scenario: 非目标岗位用户审批失败
- **WHEN** 当前审批分配到 it/ops_admin 岗位
- **AND** security-operator（属于 it/security_admin）尝试审批
- **THEN** 审批操作失败

### Requirement: syncActionSubmitter 同步执行 Action 任务

系统 SHALL 提供 `syncActionSubmitter` 实现 `engine.TaskSubmitter`，在 BDD 测试中同步执行 `itsm-action-execute` 任务（调用 ActionExecutor + auto-progress），其他任务类型 no-op。

#### Scenario: itsm-action-execute 任务被同步执行
- **WHEN** smart engine 创建 action 类型活动并提交 `itsm-action-execute` 任务
- **THEN** syncActionSubmitter SHALL 同步调用 `ActionExecutor.Execute()`
- **AND** 执行完成后 SHALL 自动调用 `engine.Progress()` 标记活动完成
- **AND** TicketActionExecution 表中 SHALL 存在对应记录

#### Scenario: 非 action 任务被忽略
- **WHEN** engine 提交 `itsm-smart-progress` 或其他任务
- **THEN** syncActionSubmitter SHALL 静默忽略（no-op）

### Requirement: LocalActionReceiver HTTP 测试接收器

系统 SHALL 提供 `LocalActionReceiver`，基于 `httptest.Server` 在测试进程内启动 HTTP 服务，记录所有收到的请求。

#### Scenario: 记录 HTTP 请求
- **WHEN** ActionExecutor 向 LocalActionReceiver 的 /precheck 路径发送 POST 请求
- **THEN** receiver.Records() SHALL 包含该请求
- **AND** 记录中包含 Path、Method、Body 字段

#### Scenario: 按路径过滤记录
- **WHEN** receiver 收到 /precheck 和 /apply 各 1 个请求
- **THEN** receiver.RecordsByPath("/precheck") SHALL 返回 1 条记录
- **AND** receiver.RecordsByPath("/apply") SHALL 返回 1 条记录

#### Scenario: 清空记录
- **WHEN** 调用 receiver.Clear()
- **THEN** receiver.Records() SHALL 返回空列表

### Requirement: replaceTemplateVars 支持 form_data 和 code 变量

`replaceTemplateVars` SHALL 支持 `{{ticket.form_data.<key>}}` 格式的模板变量（从 ticket 的 FormData JSON 字段中提取一级键值），以及 `{{ticket.code}}` 变量。

#### Scenario: form_data 变量替换
- **WHEN** body 模板为 `{"db":"{{ticket.form_data.database_name}}"}`
- **AND** ticket.FormData 为 `{"database_name":"prod-db-01"}`
- **THEN** 替换结果 SHALL 为 `{"db":"prod-db-01"}`

#### Scenario: code 变量替换
- **WHEN** body 模板为 `{"code":"{{ticket.code}}"}`
- **AND** ticket.Code 为 "DB-001"
- **THEN** 替换结果 SHALL 为 `{"code":"DB-001"}`

#### Scenario: 向后兼容已有变量
- **WHEN** body 模板包含 `{{ticket.id}}` 和 `{{ticket.status}}`
- **THEN** 替换行为 SHALL 与扩展前一致

### Requirement: 多对话模式覆盖场景
系统 SHALL 提供 `vpn_dialog_coverage.feature`，使用 Scenario Outline 覆盖 6 种对话模式。

#### Scenario: complete_direct 模式 -- 用户一次性提供完整信息并确认
- **WHEN** 用户消息包含所有必填字段且语言清晰直接
- **THEN** Agent SHALL 在 1-2 轮交互内完成到 draft_confirm 的流程

#### Scenario: colloquial_complete 模式 -- 口语化但信息完整
- **WHEN** 用户用口语化表述提供了所有必要信息
- **THEN** Agent SHALL 正确提取结构化信息并完成草稿

#### Scenario: multi_turn_fill_details 模式 -- 多轮补充信息
- **WHEN** 用户首轮信息不完整，经 Agent 追问后补充
- **THEN** Agent SHALL 在追问后收集到完整信息并推进到 draft_prepare

#### Scenario: full_info_hold 模式 -- 信息完整但用户不确认
- **WHEN** 用户提供了完整信息但未明确表示确认
- **THEN** Agent SHALL 完成 draft_prepare 但 SHALL NOT 调用 draft_confirm

#### Scenario: ambiguous_incomplete_hold 模式 -- 模糊不完整
- **WHEN** 用户表述模糊且信息不完整
- **THEN** Agent SHALL 追问澄清，不强行推进草稿

#### Scenario: multi_turn_hold 模式 -- 多轮对话中保持等待
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

#### Scenario: 知识命中变更窗口期 -- 路由到安全管理员
- **WHEN** 工单创建时知识库中包含"变更窗口期需安全审批"的策略
- **THEN** 智能引擎决策 SHALL 将工单路由到 security_admin

#### Scenario: 知识未命中 -- 走默认路由
- **WHEN** 工单创建时知识库搜索无匹配结果
- **THEN** 智能引擎决策 SHALL 按 collaboration_spec 的默认规则路由

#### Scenario: 知识库不可用 -- 不阻塞决策
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
