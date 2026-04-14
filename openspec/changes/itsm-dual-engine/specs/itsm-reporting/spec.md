## ADDED Requirements

### Requirement: Dashboard 首页数据
系统 SHALL 提供 Dashboard 首页聚合数据 API，返回当前用户视角的工单概览信息。

#### Scenario: 查询 Dashboard 概览
- **WHEN** 用户请求 GET /api/v1/itsm/reports/dashboard
- **THEN** 系统 SHALL 返回以下聚合数据：我的待办工单数量、今日新建工单数、今日完结工单数、SLA 达成率（百分比）、按优先级分布（P0~P4 各自的工单数量）

#### Scenario: 管理员视角 Dashboard
- **WHEN** itsm_admin 角色用户请求 GET /api/v1/itsm/reports/dashboard
- **THEN** 系统 SHALL 返回全局数据（不限于个人），包含全部未完结工单数、全部待办数、今日全部新建/完结数

#### Scenario: 普通用户视角 Dashboard
- **WHEN** 普通用户请求 GET /api/v1/itsm/reports/dashboard
- **THEN** 系统 SHALL 仅返回与该用户相关的数据（我提交的 + 我处理的工单）

### Requirement: 工单吞吐量报表
系统 SHALL 提供工单吞吐量统计 API，按时间维度统计新建和完结工单数，支持折线图展示。

#### Scenario: 按日统计吞吐量
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/throughput，参数 granularity="day"、start_date、end_date
- **THEN** 系统 SHALL 返回每日的新建工单数和完结工单数数组，格式为 [{date, created_count, completed_count}, ...]

#### Scenario: 按周统计吞吐量
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/throughput，参数 granularity="week"
- **THEN** 系统 SHALL 按自然周聚合，返回每周的新建和完结工单数

#### Scenario: 按月统计吞吐量
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/throughput，参数 granularity="month"
- **THEN** 系统 SHALL 按自然月聚合，返回每月的新建和完结工单数

### Requirement: SLA 达成率报表
系统 SHALL 提供 SLA 达成率统计 API，按服务和时间维度统计 SLA 响应达成率和解决达成率。

#### Scenario: 按服务统计 SLA 达成率
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/sla，参数 group_by="service"、start_date、end_date
- **THEN** 系统 SHALL 返回每个服务的响应达成率（未 breach_response 的工单比例）和解决达成率（未 breach_resolution 的工单比例）

#### Scenario: 按时间统计 SLA 达成率
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/sla，参数 group_by="time"、granularity="month"
- **THEN** 系统 SHALL 按月返回整体的 SLA 响应达成率和解决达成率趋势

#### Scenario: 排除无 SLA 工单
- **WHEN** 统计 SLA 达成率时，存在未绑定 SLA 模板的工单
- **THEN** 系统 SHALL 将这些工单排除在 SLA 统计之外

### Requirement: 平均解决时长报表
系统 SHALL 提供平均解决时长统计 API，按服务或优先级维度统计已完结工单的平均解决时长。

#### Scenario: 按服务统计平均解决时长
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/resolution-time，参数 group_by="service"、start_date、end_date
- **THEN** 系统 SHALL 返回每个服务的平均解决时长（分钟），计算方式为完结时间减去创建时间

#### Scenario: 按优先级统计平均解决时长
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/resolution-time，参数 group_by="priority"
- **THEN** 系统 SHALL 返回每个优先级的平均解决时长

#### Scenario: 排除已取消工单
- **WHEN** 统计平均解决时长时，存在状态为 "cancelled" 的工单
- **THEN** 系统 SHALL 将已取消工单排除在平均解决时长统计之外

### Requirement: 分类统计报表
系统 SHALL 提供分类统计 API，按服务目录、优先级、状态等维度统计工单分布，支持饼图和柱状图展示。

#### Scenario: 按服务目录统计
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/distribution，参数 dimension="catalog"、start_date、end_date
- **THEN** 系统 SHALL 返回每个服务目录的工单数量，格式为 [{catalog_name, count}, ...]

#### Scenario: 按优先级统计
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/distribution，参数 dimension="priority"
- **THEN** 系统 SHALL 返回每个优先级的工单数量

#### Scenario: 按状态统计
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/distribution，参数 dimension="status"
- **THEN** 系统 SHALL 返回每个工单状态的数量

### Requirement: 处理人工作量报表
系统 SHALL 提供处理人工作量统计 API，按处理人统计工作量指标。

#### Scenario: 查询处理人工作量
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/workload，参数 start_date、end_date
- **THEN** 系统 SHALL 返回每个处理人的当前待办工单数、指定时间范围内已完结工单数、平均处理时长（分钟）

#### Scenario: 按部门筛选
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/workload，参数包含 department_id
- **THEN** 系统 SHALL 仅返回该部门下处理人的工作量数据（通过 Org App 的部门关系查询）

#### Scenario: Org App 不可用时
- **WHEN** 系统未安装 Org App，管理员请求处理人工作量报表
- **THEN** 系统 SHALL 返回全部处理人的工作量数据，不支持部门筛选

### Requirement: 智能服务 AI 决策统计
系统 SHALL 提供智能服务专属的 AI 决策统计 API，统计自动执行和人工覆盖的比例。

#### Scenario: 查询 AI 决策统计
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/ai-decisions，参数 start_date、end_date
- **THEN** 系统 SHALL 返回：总决策次数、自动执行次数和比例、人工覆盖次数和比例、平均置信度

#### Scenario: 按服务统计 AI 决策
- **WHEN** 管理员请求 GET /api/v1/itsm/reports/ai-decisions，参数包含 service_id
- **THEN** 系统 SHALL 仅返回该智能服务的 AI 决策统计

#### Scenario: 无智能服务数据
- **WHEN** 管理员请求 AI 决策统计，但系统中没有智能服务工单
- **THEN** 系统 SHALL 返回全部字段值为 0 的结果

### Requirement: 报表时间范围筛选
全部报表 API SHALL 支持统一的时间范围筛选参数，提供快捷选项和自定义范围。

#### Scenario: 今日快捷筛选
- **WHEN** 报表请求参数 range="today"
- **THEN** 系统 SHALL 将时间范围设定为今日 00:00:00 到当前时间

#### Scenario: 本周快捷筛选
- **WHEN** 报表请求参数 range="this_week"
- **THEN** 系统 SHALL 将时间范围设定为本周一 00:00:00 到当前时间

#### Scenario: 本月快捷筛选
- **WHEN** 报表请求参数 range="this_month"
- **THEN** 系统 SHALL 将时间范围设定为本月 1 日 00:00:00 到当前时间

#### Scenario: 自定义时间范围
- **WHEN** 报表请求参数包含 start_date 和 end_date
- **THEN** 系统 SHALL 使用指定的时间范围进行统计

#### Scenario: 未指定时间范围
- **WHEN** 报表请求未传入 range、start_date、end_date 参数
- **THEN** 系统 SHALL 默认使用本月（this_month）作为时间范围

### Requirement: 报表权限控制
报表 API SHALL 受 Casbin RBAC 保护，不同角色可查看不同范围的数据。

#### Scenario: 管理员查看全局报表
- **WHEN** itsm_admin 角色用户请求报表 API
- **THEN** 系统 SHALL 返回全局维度的统计数据

#### Scenario: 普通用户查看个人 Dashboard
- **WHEN** 普通用户请求 GET /api/v1/itsm/reports/dashboard
- **THEN** 系统 SHALL 仅返回与该用户相关的概览数据

#### Scenario: 普通用户访问管理报表
- **WHEN** 普通用户请求 GET /api/v1/itsm/reports/throughput 等管理级报表
- **THEN** Casbin SHALL 拒绝并返回 403
