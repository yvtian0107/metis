## ADDED Requirements

### Requirement: ITSM App 向 AI App 注册 Builtin Tool
ITSM App SHALL 在 Providers() 阶段通过 IOC 获取 AI App 的 ToolRegistry（如果可用），注册一组 ITSM 专用 Builtin Tool。AI App 不存在时 SHALL 静默跳过注册，不影响 ITSM 经典功能。

#### Scenario: AI App 存在时注册工具
- **WHEN** ITSM App 启动，IOC 容器中存在 AI App 的 ToolRegistry
- **THEN** 系统 SHALL 注册全部 ITSM Builtin Tool（itsm.search_services、itsm.create_ticket、itsm.query_ticket、itsm.list_my_tickets、itsm.cancel_ticket、itsm.add_comment）

#### Scenario: AI App 不存在时静默跳过
- **WHEN** ITSM App 启动，IOC 容器中不存在 AI App 的 ToolRegistry（edition 未包含 AI App）
- **THEN** 系统 SHALL 静默跳过工具注册，不输出错误日志，ITSM 经典功能正常运行

#### Scenario: 工具注册幂等
- **WHEN** ITSM App 重启，ToolRegistry 中已存在同名工具
- **THEN** 系统 SHALL 更新已有工具定义而非创建重复记录

### Requirement: itsm.search_services 工具
系统 SHALL 注册 itsm.search_services 工具，用于搜索可用的 ITSM 服务。该工具 MUST 有 inputSchema（JSON Schema）和 description。

#### Scenario: 按关键词搜索服务
- **WHEN** Agent 调用 itsm.search_services，输入 keyword="网络"
- **THEN** 系统 SHALL 返回名称或描述匹配"网络"的已启用服务列表，每项包含 id、name、description、catalog_name、form_schema 摘要

#### Scenario: 按服务目录筛选
- **WHEN** Agent 调用 itsm.search_services，输入 catalog_id=5
- **THEN** 系统 SHALL 返回该目录下的全部已启用服务列表

#### Scenario: 无匹配结果
- **WHEN** Agent 调用 itsm.search_services，输入 keyword="不存在的服务"
- **THEN** 系统 SHALL 返回空列表和提示信息

### Requirement: itsm.create_ticket 工具
系统 SHALL 注册 itsm.create_ticket 工具，用于创建 ITSM 工单。该工具 MUST 有 inputSchema（JSON Schema）和 description。

#### Scenario: 成功创建工单
- **WHEN** Agent 调用 itsm.create_ticket，输入 service_id、summary、priority、form_data
- **THEN** 系统 SHALL 创建工单并返回 ticket_id 和 ticket_code，工单的 source MUST 设置为 "agent"

#### Scenario: 关联 Agent 会话
- **WHEN** Agent 调用 itsm.create_ticket 创建工单
- **THEN** 系统 MUST 将当前 Agent Session ID 设置为工单的 agent_session_id 字段

#### Scenario: 自动设置提单人
- **WHEN** Agent 调用 itsm.create_ticket，输入包含 requester_id
- **THEN** 系统 SHALL 将 requester_id 设置为工单的提单人；若未提供 requester_id，SHALL 使用 Agent Session 关联的 user_id

#### Scenario: 服务不存在
- **WHEN** Agent 调用 itsm.create_ticket，输入的 service_id 不存在或已禁用
- **THEN** 系统 SHALL 返回错误信息 "指定的服务不存在或已禁用"

#### Scenario: 必填字段缺失
- **WHEN** Agent 调用 itsm.create_ticket，缺少 service_id 或 summary
- **THEN** 系统 SHALL 返回 inputSchema 校验错误

### Requirement: itsm.query_ticket 工具
系统 SHALL 注册 itsm.query_ticket 工具，用于查询工单详细状态。该工具 MUST 有 inputSchema（JSON Schema）和 description。

#### Scenario: 按工单 ID 查询
- **WHEN** Agent 调用 itsm.query_ticket，输入 ticket_id=123
- **THEN** 系统 SHALL 返回工单详情：状态、当前步骤、处理人、优先级、SLA 状态、response_deadline 剩余时间、resolution_deadline 剩余时间

#### Scenario: 按工单编号查询
- **WHEN** Agent 调用 itsm.query_ticket，输入 ticket_code="ITSM-20260414-0001"
- **THEN** 系统 SHALL 按工单编号查找并返回工单详情

#### Scenario: 工单不存在
- **WHEN** Agent 调用 itsm.query_ticket，输入的 ticket_id 或 ticket_code 不存在
- **THEN** 系统 SHALL 返回错误信息 "工单不存在"

#### Scenario: 权限校验
- **WHEN** Agent 调用 itsm.query_ticket，查询的工单提单人非当前会话用户，且当前用户无 itsm_admin 角色
- **THEN** 系统 SHALL 仅返回工单基本信息（编号、状态），不返回处理人和内部评论

### Requirement: itsm.list_my_tickets 工具
系统 SHALL 注册 itsm.list_my_tickets 工具，用于查询当前用户的工单列表。该工具 MUST 有 inputSchema（JSON Schema）和 description。

#### Scenario: 查询全部工单
- **WHEN** Agent 调用 itsm.list_my_tickets，未传入 status 筛选
- **THEN** 系统 SHALL 返回当前会话用户提交的全部工单列表，按创建时间倒序，包含 ticket_code、summary、status、priority、created_at

#### Scenario: 按状态筛选
- **WHEN** Agent 调用 itsm.list_my_tickets，输入 status="in_progress"
- **THEN** 系统 SHALL 返回当前用户处于 "in_progress" 状态的工单列表

#### Scenario: 分页查询
- **WHEN** Agent 调用 itsm.list_my_tickets，输入 page=2、page_size=10
- **THEN** 系统 SHALL 返回第二页的 10 条工单记录和总数

### Requirement: itsm.cancel_ticket 工具
系统 SHALL 注册 itsm.cancel_ticket 工具，用于取消工单。该工具 MUST 有 inputSchema（JSON Schema）和 description。

#### Scenario: 成功取消工单
- **WHEN** Agent 调用 itsm.cancel_ticket，输入 ticket_id 和 reason，且当前用户为工单提单人
- **THEN** 系统 SHALL 取消工单，状态变为 "cancelled"，在时间线记录取消原因，返回取消成功

#### Scenario: 无权取消
- **WHEN** Agent 调用 itsm.cancel_ticket，当前会话用户非工单提单人且非 itsm_admin
- **THEN** 系统 SHALL 返回错误信息 "无权取消该工单"

#### Scenario: 工单已完结不可取消
- **WHEN** Agent 调用 itsm.cancel_ticket，目标工单状态为 "completed"
- **THEN** 系统 SHALL 返回错误信息 "已完结的工单不可取消"

### Requirement: itsm.add_comment 工具
系统 SHALL 注册 itsm.add_comment 工具，用于在工单中添加评论。该工具 MUST 有 inputSchema（JSON Schema）和 description。

#### Scenario: 成功添加评论
- **WHEN** Agent 调用 itsm.add_comment，输入 ticket_id 和 content
- **THEN** 系统 SHALL 在工单时间线中添加评论记录，返回评论 ID

#### Scenario: 空评论内容
- **WHEN** Agent 调用 itsm.add_comment，content 为空字符串
- **THEN** 系统 SHALL 返回 inputSchema 校验错误

### Requirement: 工具执行权限验证
每个 ITSM 工具执行时 MUST 验证调用者权限，通过 Agent Session 关联的 user_id 确定当前操作者身份。

#### Scenario: 有效会话用户
- **WHEN** Agent 工具被调用，当前 Agent Session 关联了有效的 user_id
- **THEN** 系统 SHALL 以该 user_id 的身份执行操作，遵守对应的权限约束

#### Scenario: 无会话用户
- **WHEN** Agent 工具被调用，当前 Agent Session 未关联 user_id（系统级 Agent）
- **THEN** 系统 SHALL 以系统身份执行操作，拥有完整的 ITSM 操作权限

### Requirement: 工具 inputSchema 定义
每个 ITSM Builtin Tool MUST 提供符合 JSON Schema 规范的 inputSchema，包含参数名称、类型、描述和必填标记。

#### Scenario: itsm.create_ticket 的 inputSchema
- **WHEN** AI App 读取 itsm.create_ticket 的 inputSchema
- **THEN** SHALL 包含 service_id（required, integer）、summary（required, string）、priority（optional, string）、form_data（optional, object）、requester_id（optional, integer）的完整定义

#### Scenario: Agent 按 Schema 调用
- **WHEN** Agent 根据工具的 inputSchema 构造函数调用参数
- **THEN** 系统 SHALL 按 Schema 校验输入，校验失败返回明确的错误信息

### Requirement: IT 服务台 Agent 预置定义
ITSM App 的 Seed 数据 SHALL 包含一个"IT 服务台"Agent 预置定义（通过 AI App 的 Agent Seed 机制），类型为用户侧 public Agent。

#### Scenario: Seed 创建 IT 服务台 Agent
- **WHEN** ITSM App 执行 Seed，AI App 可用
- **THEN** 系统 SHALL 创建名为 "IT 服务台" 的 Agent，system_prompt 定义服务台行为（引导用户描述问题、搜索匹配服务、确认后创建工单），绑定 itsm.search_services、itsm.create_ticket、itsm.query_ticket、itsm.list_my_tickets、itsm.cancel_ticket、itsm.add_comment 工具

#### Scenario: 用户通过 Agent 对话提单
- **WHEN** 用户在 AI Chat 中与 "IT 服务台" Agent 对话，描述 "我的 VPN 连不上"
- **THEN** Agent SHALL 调用 itsm.search_services 搜索相关服务，确认后调用 itsm.create_ticket 创建工单，返回工单编号

### Requirement: 流程决策 Agent 预置定义
ITSM App 的 Seed 数据 SHALL 包含一个"流程决策"Agent 预置定义，类型为系统侧 private Agent，temperature 设为 0.2。

#### Scenario: Seed 创建流程决策 Agent
- **WHEN** ITSM App 执行 Seed，AI App 可用
- **THEN** 系统 SHALL 创建名为 "流程决策" 的 Agent，temperature=0.2，system_prompt 定义决策行为（分析工单上下文、评估处理方案、输出决策和置信度），绑定组织架构知识

#### Scenario: AI App 不可用时跳过
- **WHEN** ITSM App 执行 Seed，AI App 不可用
- **THEN** 系统 SHALL 跳过 Agent 创建，不输出错误

### Requirement: 处理协助 Agent 预置定义
ITSM App 的 Seed 数据 SHALL 包含一个"处理协助"Agent 预置定义，类型为处理人侧 team Agent。

#### Scenario: Seed 创建处理协助 Agent
- **WHEN** ITSM App 执行 Seed，AI App 可用
- **THEN** 系统 SHALL 创建名为 "处理协助" 的 Agent，system_prompt 定义协助行为（查询知识库、提供诊断建议、辅助编写处理记录），绑定运维知识库和诊断工具

#### Scenario: 处理人调用协助 Agent
- **WHEN** 处理人在工单详情页调用"处理协助"Agent，提问 "这个网络故障可能的原因是什么"
- **THEN** Agent SHALL 结合工单上下文和知识库返回诊断建议
