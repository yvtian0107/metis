# itsm-ticket-list-views Specification

## Purpose
定义 ITSM 工单列表相关能力，确保我的工单、我的待办与历史工单在信息展示与查询范围上满足一致、可理解、可操作的产品体验。

## Requirements

### Requirement: 我的工单引擎类型标识
"我的工单"列表 SHALL 在每行展示引擎类型标识，Smart 引擎工单显示 🤖 图标或"智能"标签，Classic 引擎工单显示"经典"标签或不标识。

#### Scenario: Smart 工单显示标识
- **WHEN** 用户查看"我的工单"列表，其中包含 engineType=smart 的工单
- **THEN** 该行展示 Smart 引擎标识

### Requirement: 我的工单关键词搜索
"我的工单"列表 SHALL 支持关键词搜索，搜索范围包括 code、title、description 字段。

#### Scenario: 搜索匹配
- **WHEN** 用户在"我的工单"输入关键词"VPN"
- **THEN** 列表仅显示 code/title/description 包含"VPN"的工单

### Requirement: 我的待办多维参与者查询
"我的待办"后端查询 SHALL 使用多维参与者解析，通过 JOIN TicketAssignment 匹配 `user_id = currentUser OR position_id IN userPositions OR department_id IN userDepts`，替代当前仅按 `assignee_id` 过滤的逻辑。查询 SHALL 限定为活跃状态工单（`status IN {pending, in_progress, waiting_approval}`）。

#### Scenario: 按用户直接匹配
- **WHEN** 用户查看"我的待办"，有一个工单的 assignment.userId 等于当前用户
- **THEN** 该工单出现在待办列表中

#### Scenario: 按岗位匹配
- **WHEN** 用户持有"运维管理员"岗位，有一个工单的 assignment.positionId 匹配该岗位
- **THEN** 该工单出现在待办列表中

#### Scenario: 无关工单不显示
- **WHEN** 工单的 assignment 不匹配当前用户的任何维度
- **THEN** 该工单不出现在待办列表中

### Requirement: 我的待办列表增强
"我的待办"、"我的工单"、"历史工单"和审批列表 SHALL 共享同一状态展示合同（`status`、`statusLabel`、`statusTone`、`outcome`），并提供统一的主动刷新入口。自动刷新 SHALL 仅作为观察兜底，不得作为流程推进机制。

#### Scenario: 列表状态语义一致
- **WHEN** 同一工单同时出现在待办与详情视图
- **THEN** 两处展示的 status/statusLabel/outcome SHALL 一致

#### Scenario: 主动刷新不改变筛选上下文
- **WHEN** 用户在待办列表点击刷新
- **THEN** 系统 SHALL 保持当前筛选、分页与关键词条件
- **AND** 刷新后仅更新数据结果

#### Scenario: 自动刷新仅兜底
- **WHEN** 自动刷新定时触发
- **THEN** 系统 SHALL 仅重取当前视图数据
- **AND** 不得触发流程推进 API 或隐式状态流转

### Requirement: 历史工单用户范围限定
"历史工单"后端查询 SHALL 增加用户范围过滤，仅返回 `requester_id = currentUser OR assignee_id = currentUser` 的终态工单。管理员查看全局历史 SHALL 使用"全部工单"页面。

#### Scenario: 仅展示我参与的历史工单
- **WHEN** 用户查看"历史工单"，数据库中有 100 个已完成工单，其中 5 个由该用户提交，3 个由该用户处理
- **THEN** 列表展示 8 条记录（去重后）

#### Scenario: 其他人的工单不显示
- **WHEN** 历史工单中有一个工单，requester 和 assignee 都不是当前用户
- **THEN** 该工单不出现在历史列表中

### Requirement: 列表统一业务状态展示
工单列表、我的工单、我的待办、审批待办、历史工单和监控列表 SHALL 使用统一状态展示合同渲染状态。列表 SHALL 显示已同意决策中、已驳回决策中、AI 决策中、自动执行中、已通过、已驳回、已撤回、已取消、失败等业务状态，不得把驳回和通过都显示为“已完成”。

#### Scenario: 历史列表区分通过和驳回
- **WHEN** 历史列表中同时存在 outcome=approved 和 outcome=rejected 的终态工单
- **THEN** 通过工单 SHALL 显示“已通过”
- **AND** 驳回工单 SHALL 显示“已驳回”

#### Scenario: 撤回和取消分开展示
- **WHEN** 历史列表中同时存在 status=withdrawn 和 status=cancelled 的工单
- **THEN** 撤回工单 SHALL 显示“已撤回”
- **AND** 取消工单 SHALL 显示“已取消”

### Requirement: 列表刷新入口
工单列表类页面 SHALL 提供刷新按钮，点击后 SHALL 使用当前筛选、分页和搜索条件重新请求数据。刷新按钮 SHALL 显示加载态，且不得重置用户当前筛选条件。

#### Scenario: 刷新不重置筛选
- **WHEN** 用户在历史工单页面选择某个状态筛选并点击刷新
- **THEN** 系统 SHALL 按原状态筛选重新请求数据
- **AND** 当前页码和搜索关键词 SHALL 保持不变

### Requirement: 状态筛选使用业务状态
工单状态筛选 SHALL 使用新的业务状态枚举。系统 SHALL 移除用户可见的泛化 `completed` 筛选文案，改为已通过、已驳回、已撤回、已取消、失败等明确结果。

#### Scenario: 用户筛选已驳回
- **WHEN** 用户选择“已驳回”筛选
- **THEN** 列表 SHALL 请求并展示 status=rejected 或 outcome=rejected 的工单
- **AND** 不应混入 outcome=approved 的工单
