## MODIFIED Requirements

### Requirement: 节点类型 — gateway 网关节点
gateway 节点 SHALL 重命名为 `exclusive` 节点。`exclusive` 节点 SHALL 根据条件自动选择**唯一一条**出边（排他网关语义）。exclusive 是自动节点，不创建需要人工干预的 Activity。节点 `data.conditions` 定义条件列表，每个条件关联一条出边。条件评估基于工单字段值（Ticket 字段、流程变量 `var.*`、表单数据 `form.*`）。

系统 SHALL 同时注册 `parallel` 和 `inclusive` 节点类型常量（执行逻辑在 ④ itsm-gateway-parallel 中实现）。ValidateWorkflow 对这两个类型 SHALL 通过校验但输出提示"节点类型已注册但执行逻辑尚未实现"。

#### Scenario: exclusive 条件匹配到对应出边
- **WHEN** 流程到达 exclusive 节点，第一个条件评估为 true
- **THEN** 系统沿该条件对应的出边继续，跳过后续条件评估

#### Scenario: exclusive 无条件匹配时走默认边
- **WHEN** exclusive 节点的所有条件均评估为 false，但存在 default=true 的出边
- **THEN** 系统沿默认出边继续

#### Scenario: exclusive 无条件匹配且无默认边
- **WHEN** exclusive 节点所有条件均为 false，且没有默认出边
- **THEN** 系统记录错误到 Timeline，将工单标记为异常状态

#### Scenario: parallel 节点校验通过但不可执行
- **WHEN** workflow_json 包含 type="parallel" 的节点
- **THEN** ValidateWorkflow 不报错，但返回 warning 级别提示 "parallel 节点已注册但执行逻辑尚未实现"

#### Scenario: inclusive 节点校验通过但不可执行
- **WHEN** workflow_json 包含 type="inclusive" 的节点
- **THEN** ValidateWorkflow 不报错，但返回 warning 级别提示 "inclusive 节点已注册但执行逻辑尚未实现"

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

### Requirement: Workflow JSON Schema 校验
系统 SHALL 在保存 workflow_json 时进行完整性校验。校验规则包括：有且仅有一个 start 节点；至少一个 end 节点；start 节点有且仅有一条出边；end 节点无出边；所有边的 source 和 target 引用存在的节点 ID；无孤立节点（每个非 start 节点至少有一条入边）；**exclusive** 节点的每条非默认出边 SHALL 配置条件；**exclusive** 节点至少有两条出边；节点类型 SHALL 是合法的已注册类型之一。

对 `parallel`、`inclusive`、`script`、`subprocess`、`timer`、`signal`、`b_timer`、`b_error` 等已注册但未实现执行逻辑的节点类型，ValidateWorkflow SHALL 通过校验但返回 **warning** 级别的 ValidationError（新增 `Level` 字段区分 error/warning）。

#### Scenario: 校验通过
- **WHEN** 管理员保存 workflow_json，内容包含 1 个 start、1 个 end、合法的边关系
- **THEN** 校验通过，workflow_json 保存成功

#### Scenario: exclusive 出边缺少条件
- **WHEN** exclusive 节点的某条非默认出边没有配置 condition
- **THEN** 校验失败，返回错误"排他网关节点 {node_id} 的出边 {edge_id} 缺少条件配置"

#### Scenario: exclusive 出边不足
- **WHEN** exclusive 节点只有一条出边
- **THEN** 校验失败，返回错误"排他网关节点 {node_id} 至少需要两条出边"

#### Scenario: 非法节点类型
- **WHEN** 节点的 type 不在已注册的合法类型中
- **THEN** 校验失败，返回错误"节点 {node_id} 的类型 {type} 不合法"

#### Scenario: 未实现节点类型的 warning
- **WHEN** workflow_json 中包含 type="parallel" 的节点
- **THEN** 校验返回 warning 级别信息"节点 {node_id} 类型 parallel 已注册但执行逻辑尚未实现，当前版本不支持运行"
