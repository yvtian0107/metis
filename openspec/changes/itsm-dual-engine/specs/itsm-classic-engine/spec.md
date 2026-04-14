## ADDED Requirements

### Requirement: WorkflowEngine 统一接口
系统 SHALL 定义 WorkflowEngine 接口，作为经典引擎和智能引擎的统一抽象。接口方法包括：`Start(ticket) error`（启动流程）、`Progress(ticket, activity, outcome) error`（推进流程）、`Cancel(ticket, reason) error`（取消流程）。经典引擎和智能引擎 MUST 各自实现此接口。

#### Scenario: 接口注册到 IOC
- **WHEN** ITSM App 启动并注册 Providers
- **THEN** 系统 SHALL 同时注册 ClassicEngine 和 SmartEngine 实例到 IOC 容器

#### Scenario: 根据服务引擎类型分发
- **WHEN** 工单需要流转且关联服务的 engine_type 为 "classic"
- **THEN** 系统 SHALL 调用 ClassicEngine 的对应方法

#### Scenario: 根据服务引擎类型分发到智能引擎
- **WHEN** 工单需要流转且关联服务的 engine_type 为 "smart"
- **THEN** 系统 SHALL 调用 SmartEngine 的对应方法

### Requirement: ClassicEngine 确定性图遍历
ClassicEngine SHALL 基于服务定义的 workflow_json 进行确定性的有向图遍历。workflow_json 包含 nodes（节点数组）和 edges（边数组），引擎按照边的连接关系从 start 节点开始逐步推进到 end 节点。

#### Scenario: 启动经典流程
- **WHEN** ClassicEngine.Start() 被调用
- **THEN** 引擎 SHALL 从 workflow_json 中找到 type 为 "start" 的节点，沿其出边找到第一个目标节点，创建对应的 TicketActivity

#### Scenario: 推进到下一节点
- **WHEN** ClassicEngine.Progress() 被调用且当前 Activity 完成
- **THEN** 引擎 SHALL 根据 Activity 的 outcome 匹配出边的 transition_outcome，找到下一个目标节点并创建新 Activity

#### Scenario: 到达结束节点
- **WHEN** 引擎推进到 type 为 "end" 的节点
- **THEN** 引擎 SHALL 将工单状态更新为 "completed"，记录 finished_at 时间

#### Scenario: 取消经典流程
- **WHEN** ClassicEngine.Cancel() 被调用
- **THEN** 引擎 SHALL 将当前进行中的 Activity 标记为取消，工单状态更新为 "cancelled"

### Requirement: 节点类型定义
workflow_json 中的节点 SHALL 支持以下类型，每种类型有各自的行为语义和配置字段：

- **start**：流程起点，无配置，每个工作流有且仅有一个
- **form**：表单填写节点，配置含 form_schema（JSON Schema）
- **approve**：审批节点，配置含 approval_mode（"single" | "parallel" | "sequential"）、participants（参与人规则列表）
- **process**：人工处理节点，配置含 participants（参与人规则列表）
- **action**：自动动作节点，配置含 service_action_id（引用 ServiceAction）
- **gateway**：条件网关节点，配置含 conditions（条件规则列表，关联到不同出边）
- **notify**：通知节点，配置含 channel（通知渠道）、template（通知模板）、recipients（接收人规则）
- **wait**：等待节点，配置含 wait_type（"signal" | "timer"）、timeout_hours（超时小时数）
- **end**：流程终点，无配置，至少一个

#### Scenario: 执行表单节点
- **WHEN** 引擎推进到 form 节点
- **THEN** 系统 SHALL 创建 activity_type 为 "form" 的 Activity，按 form_schema 渲染表单，等待用户提交

#### Scenario: 执行审批节点（单人审批）
- **WHEN** 引擎推进到 approve 节点且 approval_mode 为 "single"
- **THEN** 系统 SHALL 创建一个 TicketAssignment，等待该审批人通过或驳回

#### Scenario: 执行审批节点（并行审批）
- **WHEN** 引擎推进到 approve 节点且 approval_mode 为 "parallel"
- **THEN** 系统 SHALL 为每个参与人创建 TicketAssignment，所有人独立审批，全部通过后 Activity 完成

#### Scenario: 执行审批节点（串行审批）
- **WHEN** 引擎推进到 approve 节点且 approval_mode 为 "sequential"
- **THEN** 系统 SHALL 按 sequence 顺序创建 TicketAssignment，前一个通过后激活下一个

#### Scenario: 执行处理节点
- **WHEN** 引擎推进到 process 节点
- **THEN** 系统 SHALL 创建 TicketAssignment，等待处理人认领并完成处理

#### Scenario: 执行动作节点
- **WHEN** 引擎推进到 action 节点
- **THEN** 系统 SHALL 自动触发关联的 ServiceAction（HTTP 调用），成功则继续，失败则记录错误

#### Scenario: 执行通知节点
- **WHEN** 引擎推进到 notify 节点
- **THEN** 系统 SHALL 通过 Kernel Channel 发送通知，不阻塞流程，发送后立即沿出边继续

#### Scenario: 执行等待节点（信号等待）
- **WHEN** 引擎推进到 wait 节点且 wait_type 为 "signal"
- **THEN** 系统 SHALL 暂停流程，等待外部 API 调用 `POST /api/v1/itsm/tickets/:id/signal` 发送信号后继续

#### Scenario: 执行等待节点（定时等待）
- **WHEN** 引擎推进到 wait 节点且 wait_type 为 "timer"
- **THEN** 系统 SHALL 创建定时任务，到期后自动推进流程

### Requirement: 边的转换语义
workflow_json 中的边（Edge）SHALL 包含以下字段：source（源节点 ID）、target（目标节点 ID）、transition_outcome（触发条件："submit" | "approve" | "reject" | "success" | "failure" | "timeout" | "default"）、transition_kind（转换类型："forward" | "rework" | "terminal"）。

#### Scenario: 审批通过走正向边
- **WHEN** 审批节点产生 outcome 为 "approve"
- **THEN** 引擎 SHALL 匹配 transition_outcome 为 "approve" 的出边，推进到目标节点

#### Scenario: 审批驳回走退回边
- **WHEN** 审批节点产生 outcome 为 "reject" 且存在 transition_kind 为 "rework" 的出边
- **THEN** 引擎 SHALL 沿该边将流程退回到目标节点（通常是之前的 form 节点）

#### Scenario: 无匹配出边使用默认
- **WHEN** 当前节点产生的 outcome 没有精确匹配的出边
- **THEN** 引擎 SHALL 查找 transition_outcome 为 "default" 的出边，若也不存在则报错

### Requirement: 网关节点条件评估
网关节点 SHALL 根据工单的 form_data 评估条件规则，选择匹配的出边。条件规则格式为 JSON 数组，每条规则包含：field（form_data 中的字段路径）、operator（运算符："equals" | "not_equals" | "contains_any" | "gt" | "lt" | "gte" | "lte"）、value（比较值）、edge_id（匹配时走的边 ID）。

#### Scenario: 条件匹配走对应边
- **WHEN** 引擎推进到 gateway 节点，且 form_data 中 priority 字段值为 "urgent"，存在规则 `{field: "priority", operator: "equals", value: "urgent", edge_id: "e1"}`
- **THEN** 引擎 SHALL 选择 edge_id 为 "e1" 的出边推进

#### Scenario: 多条件顺序评估
- **WHEN** 网关节点有多条规则
- **THEN** 引擎 SHALL 按数组顺序依次评估，选择第一个匹配的规则对应的出边

#### Scenario: 无条件匹配走默认边
- **WHEN** 网关节点的所有条件规则均不匹配
- **THEN** 引擎 SHALL 选择 transition_outcome 为 "default" 的出边

### Requirement: 审批参与人解析
审批节点和处理节点的 participants 配置 SHALL 支持多种参与人类型：user（指定用户 ID）、position（指定职位，解析为该职位下所有用户）、department（指定部门，解析为该部门下所有用户）、requester_manager（申请人的直属上级）。

#### Scenario: 按职位解析参与人
- **WHEN** 审批节点配置 participant_type 为 "position" 且 position_id 为某值
- **THEN** 系统 SHALL 查询 Org App 的 UserPosition 表，解析出该职位下的所有用户作为候选审批人

#### Scenario: 按申请人上级解析
- **WHEN** 审批节点配置 participant_type 为 "requester_manager"
- **THEN** 系统 SHALL 查询工单申请人的部门层级，找到其直属上级作为审批人

#### Scenario: Org App 未安装时的降级
- **WHEN** 参与人类型为 "position" 或 "department" 但 Org App 未安装
- **THEN** 系统 SHALL 返回错误，提示需要安装 Org App 以支持组织架构类参与人

### Requirement: 动作节点执行 ServiceAction
动作节点 SHALL 触发关联的 ServiceAction（HTTP webhook），执行过程记录到 TicketActionExecution。body_template 支持 Go text/template 语法，可引用工单数据。

#### Scenario: HTTP 动作成功
- **WHEN** 动作节点触发 HTTP 请求且返回 2xx 状态码
- **THEN** 系统 SHALL 记录 TicketActionExecution（status=success），Activity 产生 "success" outcome 继续推进

#### Scenario: HTTP 动作失败
- **WHEN** 动作节点触发 HTTP 请求且返回非 2xx 或超时
- **THEN** 系统 SHALL 记录 TicketActionExecution（status=failed, failure_reason），Activity 产生 "failure" outcome

#### Scenario: 动作重试
- **WHEN** HTTP 动作失败且 retry_count < 配置的最大重试次数
- **THEN** 系统 SHALL 在指数退避间隔后重试，retry_count 递增

### Requirement: 工作流 JSON Schema 校验
系统 SHALL 在保存 workflow_json 时进行结构校验，确保工作流定义的合法性。

#### Scenario: 校验 start 节点唯一
- **WHEN** 管理员保存 workflow_json 且其中包含多于一个 type 为 "start" 的节点
- **THEN** 系统 SHALL 返回 400 错误，提示 start 节点必须有且仅有一个

#### Scenario: 校验 end 节点存在
- **WHEN** 管理员保存 workflow_json 且其中没有 type 为 "end" 的节点
- **THEN** 系统 SHALL 返回 400 错误，提示至少需要一个 end 节点

#### Scenario: 校验无孤立节点
- **WHEN** 管理员保存 workflow_json 且存在既无入边也无出边的节点（start 和 end 除外）
- **THEN** 系统 SHALL 返回 400 错误，列出孤立节点的 ID

#### Scenario: 校验边的合法性
- **WHEN** 管理员保存 workflow_json 且存在引用不存在节点的边
- **THEN** 系统 SHALL 返回 400 错误，列出非法边

### Requirement: ReactFlow 可视化编辑器
系统 SHALL 提供基于 ReactFlow 的工作流可视化编辑器，用于经典服务的工作流设计。

#### Scenario: 拖拽添加节点
- **WHEN** 管理员从节点面板拖拽一个节点类型到画布
- **THEN** 系统 SHALL 在画布上创建对应类型的节点，并打开属性配置面板

#### Scenario: 连线创建边
- **WHEN** 管理员从一个节点的输出端口拖拽到另一个节点的输入端口
- **THEN** 系统 SHALL 创建一条边，并允许配置 transition_outcome 和 transition_kind

#### Scenario: 节点属性编辑
- **WHEN** 管理员点击画布上的节点
- **THEN** 系统 SHALL 在右侧属性面板中展示该节点类型对应的配置项（参与人/表单/条件/动作等）

#### Scenario: 保存工作流
- **WHEN** 管理员点击保存按钮
- **THEN** 系统 SHALL 将 ReactFlow 的 nodes 和 edges 序列化为 workflow_json 格式，经过 Schema 校验后调用 API 保存

#### Scenario: 加载已有工作流
- **WHEN** 管理员进入已有经典服务的工作流编辑页
- **THEN** 系统 SHALL 从服务定义读取 workflow_json，反序列化到 ReactFlow 画布上还原节点和连线

### Requirement: 流程实例实时可视化
系统 SHALL 在工单详情页中展示经典工作流的实时运行状态，高亮当前步骤。

#### Scenario: 渲染流程图
- **WHEN** 用户查看经典工单的详情页
- **THEN** 系统 SHALL 读取服务定义的 workflow_json 渲染只读流程图

#### Scenario: 高亮当前节点
- **WHEN** 流程图渲染时
- **THEN** 系统 SHALL 将当前 Activity 对应的 node_id 节点高亮显示（不同颜色）

#### Scenario: 标记已完成节点
- **WHEN** 流程图渲染时
- **THEN** 系统 SHALL 将已完成的 Activity 对应的节点标记为已完成状态（如灰色或打钩）

#### Scenario: 标记已走过的边
- **WHEN** 流程图渲染时
- **THEN** 系统 SHALL 将实际走过的边高亮，未走过的边保持默认样式
