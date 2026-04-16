# Capability: itsm-classic-engine

## Purpose
Provides the classic workflow engine for ITSM ticket processing. The engine interprets workflow_json (a directed graph of nodes and edges) to drive ticket lifecycle through form filling, approvals, automated actions, gateways, notifications, and wait states. Includes a ReactFlow visual editor for workflow design and a runtime visualizer for tracking ticket progress.

## Requirements

### Requirement: WorkflowEngine 接口定义
系统 SHALL 定义 `WorkflowEngine` 接口，包含三个方法：`Start(ctx, ticket)`, `Progress(ctx, ticket, activity, outcome)`, `Cancel(ctx, ticket, reason)`。ClassicEngine SHALL 实现此接口。TicketService SHALL 根据 `ServiceDefinition.EngineType` 分派到对应的引擎实现。

#### Scenario: 经典引擎注册
- **WHEN** ITSM App 启动并注册 Providers
- **THEN** ClassicEngine 作为 WorkflowEngine 实现注册到 IOC 容器，可通过 `do.MustInvoke[*ClassicEngine](i)` 获取

#### Scenario: 按引擎类型分派
- **WHEN** 工单的关联服务 `engine_type="classic"` 且调用 TicketService 的流转方法
- **THEN** TicketService 委托给 ClassicEngine 处理

#### Scenario: 根据服务引擎类型分发到智能引擎
- **WHEN** 工单需要流转且关联服务的 engine_type 为 "smart"
- **THEN** 系统 SHALL 调用 SmartEngine 的对应方法

#### Scenario: 未知引擎类型
- **WHEN** 工单的关联服务 `engine_type` 为未注册的值
- **THEN** 系统返回错误，提示不支持的引擎类型

---

### Requirement: ClassicEngine 图遍历 — Start
ClassicEngine.Start() SHALL 解析 `ServiceDefinition.workflow_json`，找到 `start` 节点，**创建 root ExecutionToken**（token_type="main", status="active", scope_id="root"），沿 start 节点唯一出边找到第一个业务节点，调用 processNode 基于 token 推进。工单 SHALL 在创建时保存 workflow_json 的快照副本。

#### Scenario: 正常启动经典流程
- **WHEN** 用户创建一个 engine_type="classic" 的工单，且服务的 workflow_json 合法
- **THEN** 系统创建工单（status=in_progress），保存 workflow_json 快照，创建 root ExecutionToken（main/active），创建 start 节点出边目标的 TicketActivity（绑定 token_id），记录 Timeline 事件"流程启动"

#### Scenario: workflow_json 无效时启动失败
- **WHEN** 用户创建工单但关联服务的 workflow_json 未通过校验（如无 start 节点）
- **THEN** 系统拒绝创建工单，返回错误信息说明 workflow_json 校验失败原因

#### Scenario: Start 遇到自动节点自动步进
- **WHEN** start 节点的出边目标是 exclusive / action / notify 等自动节点
- **THEN** ClassicEngine.Start() 自动递归处理这些自动节点（传递 token），直到到达需要人工干预的节点（form/approve/process/wait）或 end 节点

---

### Requirement: ClassicEngine 图遍历 — Progress
ClassicEngine.Progress() SHALL 接收当前 Activity 和 outcome，**从 Activity 加载关联的 ExecutionToken**，在 workflow_json 中找到当前节点的出边，匹配 outcome 对应的边，基于 token 调用 processNode 推进到目标节点。自动节点（exclusive/action/notify）SHALL 立即递归处理，人工节点（form/approve/process/wait）创建 pending Activity 后停止。到达 end 节点时 SHALL 将 token 标记为 completed，工单状态设为 `completed`。

#### Scenario: 人工节点正常流转
- **WHEN** 处理人对一个 approve 节点的 Activity 提交 outcome="approved"
- **THEN** 系统将当前 Activity 标记为 completed，加载 Activity 关联的 token，找到 outcome="approved" 对应的出边，基于 token 创建目标节点的 Activity，记录 Timeline 事件

#### Scenario: 流转到达 end 节点
- **WHEN** Progress 的目标节点是 end 类型
- **THEN** 系统创建 end 节点的 Activity（status=completed, token_id=token.ID），token.status 设为 "completed"，工单状态设为 `completed`，记录 Timeline 事件"流程完结"

#### Scenario: outcome 无匹配出边时使用默认边
- **WHEN** 当前节点的出边中没有 outcome 完全匹配的边，但存在 `data.default=true` 的默认边
- **THEN** 系统沿默认边流转

#### Scenario: outcome 无匹配出边且无默认边
- **WHEN** 当前节点的出边中没有 outcome 匹配的边，也没有默认边
- **THEN** 系统返回错误，提示"无法找到从节点 X 出发 outcome=Y 的路径"

#### Scenario: 自动步进深度限制
- **WHEN** Progress 过程中自动节点（exclusive→exclusive→...）递归超过 50 层
- **THEN** 系统中止执行，token.status 设为 "cancelled"，工单标记为异常状态，记录 Timeline 错误事件"流程自动步进超过最大深度"

---

### Requirement: ClassicEngine 图遍历 — Cancel
ClassicEngine.Cancel() SHALL 查找工单所有活跃的 ExecutionToken（status IN active, waiting），将它们标记为 `cancelled`，将所有活跃的 Activity（status=pending 或 in_progress）标记为 `cancelled`，将工单状态设为 `cancelled`，记录取消原因到 Timeline。

#### Scenario: 取消正在执行的工单
- **WHEN** 管理员取消一个 in_progress 的工单
- **THEN** 所有活跃 token 状态设为 cancelled，所有活跃 Activity 状态设为 cancelled，工单状态设为 cancelled，Timeline 记录"工单取消：{reason}"

#### Scenario: 取消已完成的工单
- **WHEN** 管理员尝试取消一个已经 completed 的工单
- **THEN** 系统返回错误，提示"已完成的工单不可取消"

---

### Requirement: 节点类型 — start 开始节点
start 节点 SHALL 作为流程的唯一入口点。一个合法的 workflow_json 中 SHALL 有且仅有一个 start 节点。start 节点 SHALL 有且仅有一条出边，不接受入边。ClassicEngine.Start() 自动跳过 start 节点，直接处理其出边目标。

#### Scenario: start 节点执行
- **WHEN** ClassicEngine.Start() 被调用
- **THEN** 系统找到 start 节点，沿唯一出边跳转到目标节点，不为 start 节点本身创建 Activity

---

### Requirement: 节点类型 — form 表单节点
form 节点 SHALL 要求处理人填写表单后提交。节点的 `data.form_schema` 定义表单结构（JSON Schema 格式）。Activity 创建时 status=pending，处理人提交表单后 outcome="submitted"，表单数据记录到 Activity.result。

#### Scenario: 表单节点等待提交
- **WHEN** 流程到达 form 节点
- **THEN** 系统创建 TicketActivity（node_id=当前节点, status=pending），分配参与人，等待处理人操作

#### Scenario: 表单提交成功
- **WHEN** 处理人填写表单并提交
- **THEN** Activity.result 保存表单数据，Activity.status 设为 completed，outcome="submitted"，触发 Progress 继续流转

---

### Requirement: 节点类型 — approve 审批节点
approve 节点 SHALL 支持三种审批模式：`single`（单人审批）、`parallel`（并行会签，所有人须审批）、`sequential`（串行依次审批）。审批结果 outcome 为 "approved" 或 "rejected"。节点 `data` SHALL 包含 `approve_mode` 和 `participants`（参与人列表）字段。

#### Scenario: 单人审批通过
- **WHEN** approve 节点模式为 single，唯一审批人点击"通过"
- **THEN** Activity.status=completed, outcome="approved"，触发 Progress 沿 approved 出边继续

#### Scenario: 单人审批驳回
- **WHEN** approve 节点模式为 single，审批人点击"驳回"
- **THEN** Activity.status=completed, outcome="rejected"，触发 Progress 沿 rejected 出边继续（通常连回之前的 form 节点实现返工）

#### Scenario: 并行会签 — 全部通过
- **WHEN** approve 节点模式为 parallel，所有参与人均点击"通过"
- **THEN** Activity.status=completed, outcome="approved"

#### Scenario: 并行会签 — 任一驳回
- **WHEN** approve 节点模式为 parallel，任一参与人点击"驳回"
- **THEN** Activity.status=completed, outcome="rejected"（无需等待其余人审批）

#### Scenario: 串行依次审批 — 全部通过
- **WHEN** approve 节点模式为 sequential，参与人按顺序依次审批，全部通过
- **THEN** 每个参与人轮到时创建 TicketAssignment，前一个完成后轮到下一个，全部通过后 outcome="approved"

#### Scenario: 串行依次审批 — 中途驳回
- **WHEN** approve 节点模式为 sequential，某一位参与人驳回
- **THEN** 后续参与人不再需要审批，outcome="rejected"

---

### Requirement: 节点类型 — process 处理节点
process 节点 SHALL 要求处理人填写处理结果后完成。与 form 节点类似，但语义不同——process 表示实际执行操作（如重启服务器、修复配置）后记录结果。outcome 为 "completed"。节点 `data` 可包含 `form_schema` 定义结果表单。

#### Scenario: 处理节点执行
- **WHEN** 流程到达 process 节点
- **THEN** 系统创建 TicketActivity（status=pending），分配参与人

#### Scenario: 处理完成
- **WHEN** 处理人填写处理结果并提交
- **THEN** Activity.result 保存处理结果，outcome="completed"，触发 Progress 继续

---

### Requirement: 节点类型 — action 动作节点
action 节点 SHALL 自动执行 ServiceAction 定义的 HTTP Webhook。执行 SHALL 通过 Scheduler 异步任务 `itsm-action-execute` 完成。执行结果记录到 TicketActionExecution 模型。成功时 outcome="success"，失败时 outcome="failed"。

#### Scenario: 动作节点触发异步执行
- **WHEN** 流程到达 action 节点
- **THEN** 系统创建 TicketActivity（status=in_progress），提交 `itsm-action-execute` 异步任务（payload 含 ticket_id, activity_id, action_id）

#### Scenario: HTTP 调用成功
- **WHEN** `itsm-action-execute` 任务执行 HTTP 请求返回 2xx
- **THEN** 创建 TicketActionExecution（status=success, response_body=响应体），Activity outcome="success"，自动触发 Progress 继续

#### Scenario: HTTP 调用失败并重试
- **WHEN** HTTP 请求返回非 2xx 或超时
- **THEN** 按 ServiceAction 配置的重试策略重试（默认 3 次，指数退避），每次重试创建 TicketActionExecution 记录

#### Scenario: 重试耗尽后最终失败
- **WHEN** 所有重试均失败
- **THEN** Activity outcome="failed"，触发 Progress 沿 failed 出边继续（如果有），无 failed 出边则工单标记为异常

#### Scenario: 动作节点超时配置
- **WHEN** ServiceAction 配置了自定义 timeout（如 60s）
- **THEN** HTTP 请求使用该 timeout 值，超过时间视为失败

---

### Requirement: 节点类型 — gateway 网关节点
gateway 节点 SHALL 重命名为 `exclusive` 节点。`exclusive` 节点 SHALL 根据条件自动选择**唯一一条**出边（排他网关语义）。exclusive 是自动节点，不创建需要人工干预的 Activity。节点 `data.conditions` 定义条件列表，每个条件关联一条出边。条件评估基于工单字段值（Ticket 字段、流程变量 `var.*`、表单数据 `form.*`）。

系统 SHALL 同时注册 `parallel` 和 `inclusive` 节点类型常量。**parallel 和 inclusive 节点的执行逻辑已实现**：processNode 根据 `NodeData.gateway_direction`（"fork"/"join"）分派到对应的 handler。ValidateWorkflow 对这两个类型 SHALL 执行 fork/join 配对校验。

NodeData SHALL 新增 `GatewayDirection string`（json: `gateway_direction`）字段，值为 `"fork"` 或 `"join"`，仅对 parallel/inclusive 节点有意义。

#### Scenario: exclusive 条件匹配到对应出边
- **WHEN** 流程到达 exclusive 节点，第一个条件评估为 true
- **THEN** 系统沿该条件对应的出边继续，跳过后续条件评估

#### Scenario: exclusive 无条件匹配时走默认边
- **WHEN** exclusive 节点的所有条件均评估为 false，但存在 default=true 的出边
- **THEN** 系统沿默认出边继续

#### Scenario: exclusive 无条件匹配且无默认边
- **WHEN** exclusive 节点所有条件均为 false，且没有默认出边
- **THEN** 系统记录错误到 Timeline，将工单标记为异常状态

#### Scenario: parallel 节点 fork 分派
- **WHEN** processNode 遇到 type="parallel" 且 gateway_direction="fork" 的节点
- **THEN** 系统调用 handleParallelFork 执行并行分裂

#### Scenario: parallel 节点 join 分派
- **WHEN** processNode 遇到 type="parallel" 且 gateway_direction="join" 的节点
- **THEN** 系统调用 handleParallelJoin 执行合并检查

#### Scenario: inclusive 节点 fork 分派
- **WHEN** processNode 遇到 type="inclusive" 且 gateway_direction="fork" 的节点
- **THEN** 系统调用 handleInclusiveFork 执行条件化分裂

#### Scenario: inclusive 节点 join 分派
- **WHEN** processNode 遇到 type="inclusive" 且 gateway_direction="join" 的节点
- **THEN** 系统调用 handleInclusiveJoin 执行动态合并

---

### Requirement: 网关条件评估操作符
网关条件 SHALL 支持以下操作符：`equals`（相等）、`not_equals`（不等）、`contains_any`（包含任一值）、`gt`（大于）、`lt`（小于）、`gte`（大于等于）、`lte`（小于等于）。条件的 `field` 指定评估的字段路径，`value` 指定比较值。字段值来源包括 Ticket 字段（如 `ticket.priority`）和表单数据（如 `form.urgency`）。

#### Scenario: equals 操作符
- **WHEN** 条件为 `{field: "ticket.priority", operator: "equals", value: "P0"}`，且工单优先级为 P0
- **THEN** 条件评估为 true

#### Scenario: not_equals 操作符
- **WHEN** 条件为 `{field: "ticket.priority", operator: "not_equals", value: "P4"}`，且工单优先级为 P0
- **THEN** 条件评估为 true

#### Scenario: contains_any 操作符
- **WHEN** 条件为 `{field: "form.category", operator: "contains_any", value: ["network", "hardware"]}`，且表单 category 值为 "network"
- **THEN** 条件评估为 true

#### Scenario: gt 数值比较
- **WHEN** 条件为 `{field: "form.amount", operator: "gt", value: 1000}`，且表单 amount 值为 1500
- **THEN** 条件评估为 true

#### Scenario: lte 数值比较
- **WHEN** 条件为 `{field: "form.amount", operator: "lte", value: 500}`，且表单 amount 值为 500
- **THEN** 条件评估为 true

#### Scenario: 字段不存在
- **WHEN** 条件引用的字段在工单/表单数据中不存在
- **THEN** 条件评估为 false，不报错

---

### Requirement: 网关条件评估使用流程变量
网关条件评估器 SHALL 从 `itsm_process_variables` 表读取流程变量，而非从最近 Activity 的 form_data JSON 中解析。变量 SHALL 通过 `var.<key>` 前缀在条件字段中访问。为向后兼容，`form.<key>` SHALL 在过渡期内也从流程变量中获取值。

#### Scenario: 网关评估基于变量的条件
- **WHEN** 网关条件为 field="var.urgency", operator="equals", value="high"
- **AND** 工单有流程变量 key="urgency", value="high"
- **THEN** 条件评估为 true

#### Scenario: 向后兼容 form 前缀
- **WHEN** 网关条件为 field="form.urgency", operator="equals", value="high"
- **AND** 工单有流程变量 key="urgency", value="high"
- **THEN** 条件评估为 true（form.* 映射到 var.* 以兼容）

#### Scenario: Ticket 字段仍可访问
- **WHEN** 网关条件为 field="ticket.priority_id"
- **THEN** 条件按工单的 priority_id 字段评估（行为不变）

#### Scenario: Activity outcome 仍可访问
- **WHEN** 网关条件为 field="activity.outcome"
- **THEN** 条件按最近完成 Activity 的 transition_outcome 评估（行为不变）

#### Scenario: 变量不存在
- **WHEN** 网关条件引用 var.nonexistent，且无此流程变量
- **THEN** 条件评估为 false（字段未找到）

#### Scenario: 无变量工单的回退
- **WHEN** 旧工单（此变更之前创建）到达网关，且无流程变量
- **THEN** 评估器 SHALL 回退到从工单和最近 Activity 解析 form_data（保留旧行为）

### Requirement: 边的转换语义
每条边 SHALL 有 `data.outcome` 字段标识触发条件（如 "approved"、"rejected"、"submitted"、"success"、"failed"）。一条边可选设置 `data.default=true` 作为默认出边。从同一节点出发的多条边 SHALL outcome 值不重复（除默认边外）。支持返工边（rework）——边的目标节点可以是流程图中之前的节点，实现驳回后返工。

#### Scenario: outcome 精确匹配
- **WHEN** 当前 Activity outcome="approved"，当前节点有一条 `data.outcome="approved"` 的出边
- **THEN** 系统沿该边流转到目标节点

#### Scenario: 返工边实现审批驳回后返工
- **WHEN** approve 节点 outcome="rejected"，有一条出边目标指向之前的 form 节点
- **THEN** 系统在该 form 节点创建新的 Activity（重新填写），实现返工循环

#### Scenario: 默认边兜底
- **WHEN** 当前节点的出边中没有精确匹配 outcome 的，但有 `data.default=true` 的边
- **THEN** 系统沿默认边流转

---

### Requirement: 参与人解析
系统 SHALL 支持四种参与人类型：`user`（指定用户 ID）、`position`（按岗位查找）、`department`（按部门查找）、`requester_manager`（工单提交人的直属上级）。参与人解析结果为一组用户 ID，用于创建 TicketAssignment 记录。

#### Scenario: user 类型解析
- **WHEN** 节点配置参与人 `{type: "user", value: "123"}`
- **THEN** 直接返回用户 ID 123，创建 TicketAssignment

#### Scenario: position 类型解析（Org App 可用）
- **WHEN** 节点配置参与人 `{type: "position", value: "7"}`，Org App 已安装
- **THEN** 通过 Org App 查询岗位 ID=7 的所有用户，返回用户 ID 列表

#### Scenario: position 类型解析（Org App 未安装）
- **WHEN** 节点配置参与人 `{type: "position", value: "7"}`，Org App 未安装
- **THEN** 返回错误"参与人解析失败：position 类型需要安装组织架构模块"，Activity 记录错误

#### Scenario: department 类型解析
- **WHEN** 节点配置参与人 `{type: "department", value: "5"}`，Org App 已安装
- **THEN** 通过 Org App 查询部门 ID=5 下的所有用户，返回用户 ID 列表

#### Scenario: requester_manager 类型解析
- **WHEN** 节点配置参与人 `{type: "requester_manager"}`，工单提交人 ID=10
- **THEN** 通过 Org App 查询用户 10 的直属上级，返回上级用户 ID

#### Scenario: 解析结果为空
- **WHEN** 参与人解析返回空列表（如岗位下无用户）
- **THEN** 系统记录警告到 Timeline，Activity 保持 pending 状态等待管理员手动指派

---

### Requirement: 节点类型 — notify 通知节点
notify 节点 SHALL 通过 Kernel Channel（消息渠道）发送通知。通知是非阻塞的——发送后立即流转到下一节点，不等待送达确认。节点 `data` SHALL 包含 `channel_id`（渠道 ID）、`template`（消息模板，支持变量替换 `{{ticket.code}}`、`{{ticket.title}}` 等）、`recipients`（收件人列表或参与人规则）。

#### Scenario: 通知发送成功
- **WHEN** 流程到达 notify 节点
- **THEN** 系统通过指定渠道发送通知，不创建需要人工干预的 Activity，立即沿唯一出边继续流转

#### Scenario: 通知发送失败
- **WHEN** 渠道发送失败（如邮件服务不可用）
- **THEN** 系统记录发送失败到 Timeline，但不阻断流程，继续沿出边流转

#### Scenario: 通知模板变量替换
- **WHEN** 通知模板包含 `{{ticket.code}}` 和 `{{ticket.title}}`
- **THEN** 系统将变量替换为实际的工单编号和标题

---

### Requirement: 节点类型 — wait 等待节点
wait 节点 SHALL 支持两种等待模式：`signal`（等待外部信号）和 `timer`（等待指定时间）。signal 模式下，Activity 状态为 pending，外部通过 API 触发继续。timer 模式下，系统通过 Scheduler 异步任务 `itsm-wait-timer` 在指定时间后自动触发继续。

#### Scenario: signal 模式等待
- **WHEN** 流程到达 wait 节点（mode=signal）
- **THEN** 系统创建 TicketActivity（status=pending），等待外部 API 调用

#### Scenario: signal 模式触发继续
- **WHEN** 外部调用 `POST /api/v1/itsm/tickets/:id/signal`，请求体包含 `{activity_id, outcome}`
- **THEN** 系统将对应 Activity 标记为 completed，触发 Progress 继续流转

#### Scenario: timer 模式等待
- **WHEN** 流程到达 wait 节点（mode=timer, duration="2h"）
- **THEN** 系统创建 TicketActivity（status=in_progress），提交 `itsm-wait-timer` 异步任务（execute_after=当前时间+2h）

#### Scenario: timer 到期自动继续
- **WHEN** `itsm-wait-timer` 任务到达执行时间
- **THEN** 系统自动将 Activity outcome="timeout"，触发 Progress 继续流转

---

### Requirement: 节点类型 — end 结束节点
end 节点 SHALL 标记流程完成。系统 SHALL 根据 token 的层级区分行为：
- **root token**（parent_token_id 为 nil）：到达 end 节点时，将工单状态设为 `completed`，记录流程完结 Timeline 事件
- **child token**（parent_token_id 不为 nil）：到达 end 节点时，仅将当前 token 标记为 `completed`，然后触发 join 合并检查（tryCompleteJoin）

一个合法的 workflow_json 中 SHALL 至少有一个 end 节点。end 节点 SHALL 无出边。

#### Scenario: root token 正常到达 end 节点
- **WHEN** root token（main 类型）流转到达 end 节点
- **THEN** 工单状态设为 completed，记录 Timeline "流程完结"

#### Scenario: child token 到达 end 节点
- **WHEN** 并行分支的子 token 到达 end 节点（而不是 join 节点）
- **THEN** 子 token 标记为 completed，触发 tryCompleteJoin 检查父 token 的所有子 token 是否全部完成

#### Scenario: 多个 end 节点（不同分支）
- **WHEN** workflow_json 包含多个 end 节点（如正常完结和异常完结分支各一个）
- **THEN** 到达任一 end 节点均完结工单（仅 root token 时），end 节点的 `data.label` 记录到 Timeline 区分完结类型

---

### Requirement: Workflow JSON Schema 校验
系统 SHALL 在保存 workflow_json 时进行完整性校验。校验规则包括：有且仅有一个 start 节点；至少一个 end 节点；start 节点有且仅有一条出边；end 节点无出边；所有边的 source 和 target 引用存在的节点 ID；无孤立节点（每个非 start 节点至少有一条入边）；**exclusive** 节点的每条非默认出边 SHALL 配置条件；**exclusive** 节点至少有两条出边；节点类型 SHALL 是合法的已注册类型之一。

**Parallel/Inclusive 校验新增规则：**
- parallel/inclusive 节点 SHALL 移出 UnimplementedNodeTypes（不再输出"未实现"warning）
- parallel 和 inclusive 的 fork 节点 SHALL 至少有两条出边
- parallel 和 inclusive 的 join 节点 SHALL 至少有两条入边且恰好一条出边
- inclusive fork 节点的每条非默认出边 SHALL 配置条件（复用 exclusive 的校验逻辑）
- parallel/inclusive 节点 SHALL 有 gateway_direction 属性（"fork" 或 "join"），缺失时报错

对 `script`、`subprocess`、`timer`、`signal`、`b_timer`、`b_error` 等已注册但未实现执行逻辑的节点类型，ValidateWorkflow SHALL 通过校验但返回 **warning** 级别的 ValidationError。

#### Scenario: 校验通过
- **WHEN** 管理员保存 workflow_json，内容包含 1 个 start、1 个 end、合法的边关系
- **THEN** 校验通过，workflow_json 保存成功

#### Scenario: exclusive 出边缺少条件
- **WHEN** exclusive 节点的某条非默认出边没有配置 condition
- **THEN** 校验失败，返回错误"排他网关节点 {node_id} 的出边 {edge_id} 缺少条件配置"

#### Scenario: exclusive 出边不足
- **WHEN** exclusive 节点只有一条出边
- **THEN** 校验失败，返回错误"排他网关节点 {node_id} 至少需要两条出边"

#### Scenario: parallel fork 出边不足
- **WHEN** parallel fork 节点只有一条出边
- **THEN** 校验失败，返回错误"并行网关 fork 节点 {node_id} 至少需要两条出边"

#### Scenario: parallel join 入边不足
- **WHEN** parallel join 节点只有一条入边
- **THEN** 校验失败，返回错误"并行网关 join 节点 {node_id} 至少需要两条入边"

#### Scenario: parallel join 出边数量不为一
- **WHEN** parallel join 节点有 0 条或 2+ 条出边
- **THEN** 校验失败，返回错误"并行网关 join 节点 {node_id} 必须有且仅有一条出边"

#### Scenario: inclusive fork 出边缺少条件
- **WHEN** inclusive fork 节点的某条非默认出边没有配置 condition
- **THEN** 校验失败，返回错误"包含网关 fork 节点 {node_id} 的出边 {edge_id} 缺少条件配置"

#### Scenario: parallel/inclusive 缺少 gateway_direction
- **WHEN** parallel 或 inclusive 节点没有配置 gateway_direction 属性
- **THEN** 校验失败，返回错误"节点 {node_id} 类型 {type} 必须配置 gateway_direction（fork 或 join）"

#### Scenario: 非法节点类型
- **WHEN** 节点的 type 不在已注册的合法类型中
- **THEN** 校验失败，返回错误"节点 {node_id} 的类型 {type} 不合法"

#### Scenario: 未实现节点类型的 warning
- **WHEN** workflow_json 中包含 type="script" 的节点
- **THEN** 校验返回 warning 级别信息"节点 {node_id} 类型 script 已注册但执行逻辑尚未实现，当前版本不支持运行"

---

### Requirement: ReactFlow 可视化编辑器
系统 SHALL 在服务定义编辑页中提供基于 @xyflow/react 的工作流可视化编辑器。编辑器 SHALL 包含：左侧节点面板（9 种节点类型可拖拽到画布）、中央画布（支持缩放/平移、节点拖拽布局、连线）、右侧属性面板（选中节点时显示该节点的配置项）。

#### Scenario: 拖拽节点到画布
- **WHEN** 管理员从左侧面板拖拽一个 approve 节点到画布
- **THEN** 画布上出现 approve 节点，带有默认配置，可继续编辑属性

#### Scenario: 连接两个节点
- **WHEN** 管理员从节点 A 的输出连接点拖拽到节点 B 的输入连接点
- **THEN** 创建一条边连接 A→B，边的 data.outcome 默认为空，可在属性面板中配置

#### Scenario: 编辑节点属性
- **WHEN** 管理员点击画布上的某个 approve 节点
- **THEN** 右侧属性面板显示：审批模式选择（single/parallel/sequential）、参与人配置（类型+值）、节点名称编辑

#### Scenario: 编辑边属性
- **WHEN** 管理员点击画布上的某条边
- **THEN** 右侧属性面板显示：outcome 输入框、是否默认边开关、网关条件配置（仅当 source 为 gateway 时）

#### Scenario: 保存工作流
- **WHEN** 管理员点击"保存"按钮
- **THEN** 编辑器将当前 ReactFlow 状态（nodes + edges）序列化为 JSON，调用后端 API 保存到 ServiceDefinition.workflow_json，后端执行 Schema 校验

#### Scenario: 加载已有工作流
- **WHEN** 管理员打开一个已配置 workflow_json 的服务定义编辑器
- **THEN** 编辑器从 JSON 恢复节点位置、连线关系和各节点配置

#### Scenario: 保存时校验失败
- **WHEN** 管理员点击保存但 workflow_json 校验失败
- **THEN** 前端显示校验错误信息，不关闭编辑器，标记有问题的节点/边

---

### Requirement: 流程实例运行时可视化
系统 SHALL 在工单详情页提供只读的流程图可视化组件。该组件 SHALL 基于工单快照的 workflow_json 渲染流程图，并根据工单的 Activity 记录高亮显示：当前活跃节点（高亮色）、已完成节点（绿色标记）、已走过的边（加粗或高亮）、未到达的节点和边（灰色）。

#### Scenario: 查看进行中工单的流程图
- **WHEN** 用户查看一个 in_progress 的经典工单详情
- **THEN** 流程图显示所有节点，当前活跃节点（有 pending/in_progress Activity 的节点）高亮，之前完成的节点标绿，已走过的边加粗

#### Scenario: 查看已完成工单的流程图
- **WHEN** 用户查看一个 completed 的经典工单详情
- **THEN** 流程图显示完整执行路径，所有走过的节点标绿，走过的边加粗，未走的分支灰色

#### Scenario: 流程图点击节点查看详情
- **WHEN** 用户在只读流程图中点击某个已完成的节点
- **THEN** 显示该节点对应的 Activity 详情（处理人、时间、结果、outcome）

---

### Requirement: TicketService 集成 ClassicEngine
当 `ServiceDefinition.engine_type="classic"` 时，TicketService.Create() SHALL 在创建工单后自动调用 ClassicEngine.Start()。TicketService SHALL 新增 Progress() 方法处理工单流转请求，委托给 ClassicEngine.Progress()。TicketService SHALL 新增 Signal() 方法处理等待节点的外部信号。

#### Scenario: 创建经典工单自动启动流程
- **WHEN** 用户创建工单，关联的服务 engine_type="classic"
- **THEN** TicketService.Create() 创建工单后自动调用 ClassicEngine.Start()，工单直接进入 in_progress 状态并创建首个 Activity

#### Scenario: 创建手动工单不触发引擎
- **WHEN** 用户创建工单，关联的服务 engine_type="" 或 "manual"
- **THEN** TicketService.Create() 走 Phase 1 的手动模式，不调用任何引擎

#### Scenario: 工单流转 API
- **WHEN** 处理人调用 `POST /api/v1/itsm/tickets/:id/progress`，请求体包含 `{activity_id, outcome, result}`
- **THEN** TicketService.Progress() 验证权限后调用 ClassicEngine.Progress()，完成流转

#### Scenario: 等待信号 API
- **WHEN** 外部系统调用 `POST /api/v1/itsm/tickets/:id/signal`，请求体包含 `{activity_id, outcome, data}`
- **THEN** TicketService.Signal() 验证 Activity 为 wait 节点且 status=pending，然后调用 ClassicEngine.Progress()

---

### Requirement: Scheduler 异步任务注册
ITSM App SHALL 注册两个 Scheduler 异步任务：`itsm-action-execute`（执行动作节点的 HTTP 调用）和 `itsm-wait-timer`（等待节点定时唤醒）。两个任务均为 async 类型，由 ClassicEngine 在遍历到对应节点时提交。

#### Scenario: itsm-action-execute 任务执行
- **WHEN** Scheduler 轮询到 itsm-action-execute 任务
- **THEN** 任务执行器读取 payload（ticket_id, activity_id, action_id），发起 HTTP 请求，根据结果调用 TicketService.Progress()

#### Scenario: itsm-wait-timer 任务执行
- **WHEN** Scheduler 轮询到 itsm-wait-timer 任务，且当前时间 >= payload.execute_after
- **THEN** 任务执行器调用 TicketService.Progress()，outcome="timeout"

#### Scenario: itsm-wait-timer 任务未到时间
- **WHEN** Scheduler 轮询到 itsm-wait-timer 任务，但当前时间 < payload.execute_after
- **THEN** 任务跳过本次执行，等待下次轮询（不报错，不重试）

---

### Requirement: Subprocess node validation
The validator SHALL validate subprocess nodes for structural integrity including SubProcessDef presence, exactly one outgoing edge, and recursive validation of the embedded workflow.

#### Scenario: SubProcessDef missing
- **WHEN** a subprocess node has no SubProcessDef or it is empty
- **THEN** a validation error SHALL be returned: "子流程节点 {nodeID} 必须配置 subprocess_def"

#### Scenario: SubProcessDef parse failure
- **WHEN** a subprocess node's SubProcessDef cannot be parsed as valid workflow JSON
- **THEN** a validation error SHALL be returned indicating parse failure

#### Scenario: Subprocess outgoing edge count
- **WHEN** a subprocess node does not have exactly one outgoing edge
- **THEN** a validation error SHALL be returned: "子流程节点 {nodeID} 必须有且仅有一条出边"

#### Scenario: Recursive validation of SubProcessDef
- **WHEN** a subprocess node has a valid SubProcessDef
- **THEN** the validator SHALL recursively validate the SubProcessDef using the same rules (start/end nodes, edges, gateway constraints, etc.)
- **AND** validation errors from the subprocess SHALL include the subprocess node ID as context prefix

#### Scenario: Nested subprocess rejected in v1
- **WHEN** a SubProcessDef contains a subprocess node (nested subprocess)
- **THEN** a validation error SHALL be returned: "当前版本不支持嵌套子流程"

### Requirement: resolveWorkflowContext helper
The engine SHALL provide a resolveWorkflowContext function that returns the correct WorkflowDef/nodeMap/outEdges for a given token, automatically resolving subprocess context.

#### Scenario: Main flow token
- **WHEN** resolveWorkflowContext is called with a token of type "main" or "parallel"
- **THEN** it SHALL return the def/maps parsed from ticket.WorkflowJSON

#### Scenario: Subprocess token
- **WHEN** resolveWorkflowContext is called with a token of type "subprocess"
- **THEN** it SHALL load the parent token, find the subprocess node in the main workflow, parse its SubProcessDef, and return the subprocess def/maps
