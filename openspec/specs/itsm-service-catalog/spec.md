# itsm-service-catalog

## Purpose

ITSM 服务目录模块，管理服务分类树、服务定义、服务动作、优先级、SLA 模板及升级规则，为工单系统提供基础配置数据。

## Requirements

### Requirement: 服务目录树形分类管理

系统 SHALL 提供服务目录（ServiceCatalog）实体，支持树形分类结构。字段包括：name（名称）、code（唯一编码）、description（描述）、parent_id（父分类 ID，自关联，顶层为 null）、sort_order（排序）、icon（图标）、is_active（是否启用）。内嵌 BaseModel 提供 ID + 时间戳 + 软删除。

创建和编辑分类时，表单 SHALL 包含 code 字段供用户输入。

后端 SHALL 在创建和更新分类时同时执行树形结构校验，拒绝不存在的父分类、超过两层的层级关系以及会导致自引用或祖先/后代环的父子关系变更。创建或更新时若 `code` 与现有分类重复，系统 SHALL 返回 `409 Conflict`。

#### Scenario: 创建顶层分类
- **WHEN** 管理员请求 `POST /api/v1/itsm/catalogs` 并传入 name、code，parent_id 为空
- **THEN** 系统 SHALL 创建一个顶层服务分类并返回完整记录（含 code 字段）

#### Scenario: 创建子分类
- **WHEN** 管理员请求 `POST /api/v1/itsm/catalogs` 并传入有效的 parent_id 和 code
- **THEN** 系统 SHALL 创建子分类，parent_id 指向已有的父分类

#### Scenario: 编码唯一性校验
- **WHEN** 管理员创建或更新分类时使用已存在的 code
- **THEN** 系统 SHALL 返回 409 冲突错误

#### Scenario: 查询分类树
- **WHEN** 用户请求 `GET /api/v1/itsm/catalogs/tree`
- **THEN** 系统 SHALL 返回完整的树形分类结构，每个节点包含 children 数组，Response 包含 code 字段

#### Scenario: 删除含子分类的目录
- **WHEN** 管理员删除一个含有子分类的目录
- **THEN** 系统 SHALL 返回 400 错误，提示需先删除或移动子分类

#### Scenario: 删除含服务定义的目录
- **WHEN** 管理员删除一个已绑定服务定义的目录
- **THEN** 系统 SHALL 返回 400 错误，提示需先解除服务绑定

#### Scenario: 分类排序
- **WHEN** 管理员修改分类的 sort_order
- **THEN** 同层级分类按 sort_order 升序排列

#### Scenario: 创建分类限制两层
- **WHEN** 管理员请求创建分类时传入的 parent_id 指向一个已有 parent 的分类（即尝试创建第三层）
- **THEN** 系统 SHALL 返回 400 错误，提示服务目录最多支持两层

#### Scenario: 更新分类时父分类不存在
- **WHEN** 管理员请求 `PUT /api/v1/itsm/catalogs/:id` 且传入不存在的 parent_id
- **THEN** 系统 SHALL 返回 400 错误，并拒绝保存该更新

#### Scenario: 更新分类时禁止自引用
- **WHEN** 管理员请求 `PUT /api/v1/itsm/catalogs/:id` 并将 parent_id 设置为当前分类自身 ID
- **THEN** 系统 SHALL 返回 400 错误，并拒绝保存该更新

#### Scenario: 更新分类时禁止形成循环层级
- **WHEN** 管理员请求 `PUT /api/v1/itsm/catalogs/:id` 并将 parent_id 设置为该分类后代节点的 ID
- **THEN** 系统 SHALL 返回 400 错误，并拒绝保存该更新

### Requirement: 内置服务目录种子数据

系统 SHALL 在首次安装或 Sync 启动时，通过 `seedCatalogs()` 函数内置 18 条标准服务目录分类（6 个一级域 × 3 个子分类）。Seed 使用 `code` 字段做幂等检查，已存在的记录不覆盖。

一级域及其子分类：

| 一级域 | code | 子分类 |
|--------|------|--------|
| 账号与权限 | `account-access` | 账号开通 (`account-access:provisioning`)、权限申请 (`account-access:authorization`)、密码与MFA (`account-access:credential`) |
| 终端与办公支持 | `workplace-support` | 电脑与外设 (`workplace-support:endpoint`)、办公软件支持 (`workplace-support:office-software`)、打印与会议室设备 (`workplace-support:meeting-room`) |
| 基础设施与网络 | `infra-network` | 网络与VPN (`infra-network:network`)、服务器与主机 (`infra-network:compute`)、存储与备份 (`infra-network:storage`) |
| 应用与平台支持 | `application-platform` | 企业应用支持 (`application-platform:business-app`)、发布与变更协助 (`application-platform:release`)、数据库支持 (`application-platform:database`) |
| 安全与合规 | `security-compliance` | 安全事件协助 (`security-compliance:incident`)、漏洞与基线 (`security-compliance:vulnerability`)、审计与合规支持 (`security-compliance:audit`) |
| 监控与告警 | `monitoring-alerting` | 监控接入 (`monitoring-alerting:onboarding`)、告警治理 (`monitoring-alerting:governance`)、值班与通知策略 (`monitoring-alerting:oncall`) |

每条记录 SHALL 包含 name、code、description、icon（Lucide 图标名）、sort_order、is_active=true。子分类通过先查询父分类 ID 建立关联。

#### Scenario: 首次安装种子数据
- **WHEN** 系统首次安装，数据库无服务目录数据
- **THEN** 系统 SHALL 创建全部 18 条分类记录，6 个一级 + 12 个子分类

#### Scenario: 幂等重复执行
- **WHEN** 系统重启执行 Sync，数据库已有 seed 创建的分类
- **THEN** 系统 SHALL 跳过已存在的记录（按 code 匹配），不覆盖用户修改

#### Scenario: 缺失记录补建
- **WHEN** 用户删除了某个 seed 创建的分类后系统重启并再次执行 seed
- **THEN** 系统 SHALL 重新创建缺失的分类记录（包括软删除后不可见的记录），同时不影响其他已存在的 seed 数据

### Requirement: 服务目录管理页面左右分栏布局

服务目录管理页面（`/itsm/catalogs`）SHALL 采用左右分栏布局：左侧固定宽度面板展示一级分类导航列表，右侧展示选中分类的子分类表格。严格两层结构。

#### Scenario: 默认展示
- **WHEN** 用户进入服务目录管理页面
- **THEN** 系统 SHALL 展示左右分栏布局，左侧列出所有一级分类（含图标、名称、子分类数量），默认选中第一个分类，右侧展示其子分类列表

#### Scenario: 切换一级分类
- **WHEN** 用户点击左侧某个一级分类
- **THEN** 右侧 SHALL 切换为该分类的子分类列表，显示子分类的名称、编码、描述、状态和操作按钮

#### Scenario: 新增子分类
- **WHEN** 用户在右侧点击"新增子分类"按钮
- **THEN** 系统 SHALL 打开 Sheet 表单，parentId 自动设为当前选中的一级分类

#### Scenario: 新增一级分类
- **WHEN** 用户点击页面顶部"新增分类"按钮
- **THEN** 系统 SHALL 打开 Sheet 表单，parentId 为空，创建成功后左侧导航列表刷新

#### Scenario: 编辑一级分类
- **WHEN** 用户在左侧分类项上点击编辑按钮
- **THEN** 系统 SHALL 打开 Sheet 表单，允许修改名称、编码、图标、描述、排序

#### Scenario: 空状态
- **WHEN** 当前选中分类无子分类
- **THEN** 右侧 SHALL 展示空状态提示，引导用户创建子分类

#### Scenario: 左侧图标展示
- **WHEN** 一级分类设置了 icon 字段
- **THEN** 左侧导航 SHALL 根据 icon 名称渲染对应的 Lucide 图标

### Requirement: 服务定义管理

系统 SHALL 提供服务定义（ServiceDefinition）实体，代表一个可请求的 IT 服务。字段包括：name（名称）、code（唯一编码）、description（描述）、catalog_id（所属分类 FK）、engine_type（引擎类型："classic" | "smart"）、sla_id（FK→SLATemplate，可选）、form_schema（JSON，提单表单定义）、workflow_json（JSON，经典模式工作流定义）、collaboration_spec（文本，智能模式协作规范）、agent_id（uint，智能模式关联的 Agent ID）、knowledge_base_ids（JSON 数组，智能模式关联的知识库）、agent_config（JSON，智能模式配置如信心阈值）、is_active（是否启用）、sort_order，嵌入 BaseModel。

后端 SHALL 在创建和更新服务定义时校验 `catalog_id` 引用的服务目录分类存在。创建或更新时若 `code` 与现有服务定义重复，系统 SHALL 返回 `409 Conflict`。`GET /api/v1/itsm/services` SHALL 支持 `catalog_id`、`engine_type`、`is_active`、`keyword` 过滤参数的组合查询。

#### Scenario: 创建经典服务定义
- **WHEN** 管理员请求 `POST /api/v1/itsm/services` 并传入 engine_type 为 "classic"
- **THEN** 系统 SHALL 创建服务定义，并要求后续配置 workflow_json 和 form_schema

#### Scenario: 创建智能服务定义
- **WHEN** 管理员请求 `POST /api/v1/itsm/services` 并传入 engine_type 为 "smart"
- **THEN** 系统 SHALL 创建服务定义，并要求后续配置 collaboration_spec 和 agent_id

#### Scenario: 服务编码唯一性
- **WHEN** 管理员创建服务定义时使用已存在的 code
- **THEN** 系统 SHALL 返回 409 冲突错误

#### Scenario: 服务列表查询
- **WHEN** 用户请求 `GET /api/v1/itsm/services` 并可选传入 catalog_id、engine_type、is_active、keyword 过滤参数
- **THEN** 系统 SHALL 返回分页的服务定义列表

#### Scenario: 服务详情查询
- **WHEN** 用户请求 `GET /api/v1/itsm/services/:id`
- **THEN** 系统 SHALL 返回服务定义完整信息，包括分类信息和引擎配置

#### Scenario: 启用/禁用服务
- **WHEN** 管理员修改服务的 is_active 状态
- **THEN** 禁用的服务不出现在用户提单的服务目录中

#### Scenario: 创建服务时分类不存在
- **WHEN** 管理员请求 `POST /api/v1/itsm/services` 且 `catalog_id` 引用不存在的分类
- **THEN** 系统 SHALL 返回 400 错误，并拒绝创建服务定义

#### Scenario: 列表按引擎类型过滤
- **WHEN** 用户请求 `GET /api/v1/itsm/services?engineType=smart`
- **THEN** 系统 SHALL 仅返回 `engine_type` 为 `smart` 的服务定义

### Requirement: 经典服务引擎配置
engine_type 为 "classic" 的服务定义 SHALL 额外持有以下配置字段：workflow_json（ReactFlow 格式的工作流 JSON）、form_schema（JSON Schema 格式的表单定义）。这些字段存储在 ServiceDefinition 表中（JSON 列）。

后端 SHALL 在创建和更新经典服务时校验 `workflow_json` 的基本结构，并在服务定义为 `smart` 时拒绝写入经典引擎专属字段。

#### Scenario: 保存工作流 JSON
- **WHEN** 管理员请求 `PUT /api/v1/itsm/services/:id` 更新 workflow_json
- **THEN** 系统 SHALL 校验 workflow_json 的基本结构（必须含 nodes 和 edges 数组）后保存

#### Scenario: 保存表单 Schema
- **WHEN** 管理员请求 `PUT /api/v1/itsm/services/:id` 更新 form_schema
- **THEN** 系统 SHALL 校验 form_schema 为合法 JSON 后保存

#### Scenario: 非经典服务设置经典字段
- **WHEN** 管理员尝试对 engine_type 为 "smart" 的服务设置 workflow_json 或 form_schema
- **THEN** 系统 SHALL 返回 400 错误，提示引擎类型不匹配

### Requirement: 智能服务引擎配置
engine_type 为 "smart" 的服务定义 SHALL 额外持有以下配置字段：collaboration_spec（Markdown 格式的协作规范）、agent_id（FK 引用 AI App 的 Agent）、knowledge_base_ids（JSON 数组，引用知识库 ID 列表）、agent_config（JSON，含 confidence_threshold 信心阈值、decision_timeout_seconds 决策超时秒数、fallback_strategy 兜底策略）。

后端 SHALL 在创建和更新智能服务时拒绝写入经典引擎专属字段；当 `agent_id` 被设置时，系统 SHALL 校验该引用有效后再保存。

#### Scenario: 配置智能服务的 Agent
- **WHEN** 管理员请求 `PUT /api/v1/itsm/services/:id` 设置 agent_id
- **THEN** 系统 SHALL 校验 agent_id 引用的 Agent 存在且处于激活状态后保存

#### Scenario: 配置信心阈值
- **WHEN** 管理员设置 agent_config.confidence_threshold 为 0.8
- **THEN** 系统 SHALL 保存该值，后续智能引擎在 AI 信心 >= 0.8 时自动执行决策

#### Scenario: 无效的 Agent 引用
- **WHEN** 管理员设置 agent_id 为不存在或已禁用的 Agent
- **THEN** 系统 SHALL 返回 400 错误

#### Scenario: 非智能服务设置智能字段
- **WHEN** 管理员尝试对 engine_type 为 "classic" 的服务设置 collaboration_spec 或 agent_id
- **THEN** 系统 SHALL 返回 400 错误，提示引擎类型不匹配

### Requirement: 服务目录后端回归测试覆盖

系统 SHALL 为服务目录分类、服务定义和相关种子数据提供自动化后端测试覆盖，以验证业务规则、HTTP 错误契约和 seed 幂等行为在后续变更中保持稳定。

#### Scenario: 分类服务业务规则回归
- **WHEN** 后端测试执行服务目录分类 service 层用例
- **THEN** 系统 SHALL 覆盖创建子分类、更新父分类、删除带引用分类和树形排序等关键业务规则

#### Scenario: 服务定义 HTTP 契约回归
- **WHEN** 后端测试执行服务定义 handler 层用例
- **THEN** 系统 SHALL 覆盖重复 code 返回 409、无效分类返回 400、无效工作流返回 400 和查询过滤行为

#### Scenario: 服务目录种子幂等回归
- **WHEN** 后端测试重复执行 catalog seed 逻辑并模拟软删除后重建
- **THEN** 系统 SHALL 验证 seed 首次创建、幂等跳过和缺失记录补建行为

### Requirement: 智能服务 Spec 模板库
系统 SHALL 提供预置的 Collaboration Spec 模板库（SpecTemplate），包含常见 ITSM 场景的协作规范模板。字段包括：name（模板名称）、category（场景分类，如 "incident"、"change"、"request"）、description（模板描述）、content（Markdown 格式的模板内容）、is_builtin（是否内置）。

#### Scenario: 查询模板列表
- **WHEN** 管理员请求 `GET /api/v1/itsm/services/spec-templates` 并可选传入 category 过滤
- **THEN** 系统 SHALL 返回匹配的模板列表

#### Scenario: 使用模板填充 Spec
- **WHEN** 管理员选择一个模板应用到智能服务
- **THEN** 系统 SHALL 将模板 content 填充到服务定义的 collaboration_spec 字段

#### Scenario: 自定义模板
- **WHEN** 管理员请求 `POST /api/v1/itsm/services/spec-templates` 创建自定义模板
- **THEN** 系统 SHALL 创建 is_builtin 为 false 的模板记录

### Requirement: AI 生成 Collaboration Spec
系统 SHALL 支持通过自然语言描述自动生成 Collaboration Spec。管理员输入服务场景的自然语言描述，系统调用 LLM 生成结构化的 Markdown 格式协作规范。

#### Scenario: AI 生成 Spec
- **WHEN** 管理员请求 `POST /api/v1/itsm/services/:id/generate-spec` 并传入 prompt（自然语言描述）
- **THEN** 系统 SHALL 调用服务绑定的 Agent 的 LLM 模型，生成 Collaboration Spec 并返回（不自动保存）

#### Scenario: 未绑定 Agent 时生成
- **WHEN** 管理员对未绑定 agent_id 的智能服务请求生成 Spec
- **THEN** 系统 SHALL 使用系统默认 LLM 模型生成

#### Scenario: LLM 调用失败
- **WHEN** LLM 服务不可用或返回错误
- **THEN** 系统 SHALL 返回 502 错误并携带失败原因

### Requirement: 动作定义管理

系统 SHALL 提供动作定义（ServiceAction）实体，代表服务流程中可触发的自动化动作。字段包括：name（名称）、code（唯一编码，同一服务内唯一）、description（描述）、prompt（动作说明）、service_id（FK 关联服务定义）、action_type（类型，当前支持 "http"）、config_json（JSON，HTTP 类型含 url、method、headers、body_template、timeout_seconds）、is_active（是否启用）。内嵌 BaseModel。

#### Scenario: 创建 HTTP 动作
- **WHEN** 管理员请求 `POST /api/v1/itsm/actions` 并传入 action_type 为 "http"、config_json 含 url 和 method
- **THEN** 系统 SHALL 校验 config_json 中 url 和 method 必填后创建动作

#### Scenario: 同服务内编码唯一
- **WHEN** 在同一服务下创建 code 重复的动作
- **THEN** 系统 SHALL 返回错误

#### Scenario: 动作列表按服务筛选
- **WHEN** 用户请求 `GET /api/v1/itsm/actions?service_id=xxx`
- **THEN** 系统 SHALL 返回该服务下的所有动作定义

#### Scenario: body_template 变量替换
- **WHEN** 动作的 body_template 中含有 `{{.ticket.code}}`、`{{.form_data.xxx}}` 等模板变量
- **THEN** 系统 SHALL 在执行时使用 Go text/template 引擎将工单数据注入模板

#### Scenario: 删除已被工作流引用的动作
- **WHEN** 管理员删除一个正在被经典工作流 action 节点引用的动作
- **THEN** 系统 SHALL 返回 400 错误，提示动作正在使用中

### Requirement: 优先级管理

系统 SHALL 支持工单优先级定义。Priority 模型包含：name、code（唯一，如 "P0"~"P4"）、value（数字，越小越紧急）、color（十六进制颜色）、description、default_response_minutes、default_resolution_minutes、is_active，嵌入 BaseModel。

#### Scenario: CRUD 优先级
- **WHEN** 管理员创建/编辑/删除优先级
- **THEN** 系统执行相应操作，code 唯一性校验

#### Scenario: 优先级列表排序
- **WHEN** 请求优先级列表
- **THEN** 按 value 升序返回（P0 最紧急排第一）

#### Scenario: Seed 默认优先级
- **WHEN** 系统首次安装或 Sync 启动
- **THEN** 系统创建 P0(紧急)、P1(高)、P2(中)、P3(低)、P4(最低) 五个默认优先级（幂等）

### Requirement: SLA 模板管理

系统 SHALL 支持 SLA 模板定义。SLATemplate 模型包含：name、code（唯一）、description、response_minutes（响应时间）、resolution_minutes（解决时间）、is_active，嵌入 BaseModel。

Seed 阶段 SHALL 内置 5 个 SLA 模板（幂等，按 code 判重）：

| code | 名称 | 响应时间 | 解决时间 |
|------|------|---------|---------|
| `standard` | 标准 | 240 min | 1440 min |
| `urgent` | 紧急 | 30 min | 240 min |
| `rapid-workplace` | 快速办公支持 | 15 min | 120 min |
| `critical-business` | 关键业务 | 10 min | 60 min |
| `infra-change` | 基础设施变更 | 60 min | 480 min |

#### Scenario: CRUD SLA 模板
- **WHEN** 管理员创建/编辑/删除 SLA 模板
- **THEN** 系统执行相应操作

#### Scenario: SLA 绑定到服务
- **WHEN** 管理员编辑服务定义，选择 sla_id
- **THEN** 该服务创建的工单使用此 SLA 的时间要求

#### Scenario: Seed 默认 SLA 模板
- **WHEN** 系统首次安装或 Sync 启动
- **THEN** 系统 SHALL 创建全部 5 个 SLA 模板（按 code 幂等），已存在的记录不覆盖

### Requirement: 升级规则管理

系统 SHALL 支持 SLA 升级规则。EscalationRule 模型包含：sla_id（FK→SLATemplate）、trigger_type（"response_timeout"|"resolution_timeout"）、level（升级级别 1/2/3）、wait_minutes（等待分钟数）、action_type（"notify"|"reassign"|"escalate_priority"）、target_config（JSON，通知对象/目标处理人/目标优先级）、is_active，嵌入 BaseModel。

#### Scenario: 创建升级规则
- **WHEN** 管理员为某 SLA 创建升级规则
- **THEN** 系统保存规则，同一 SLA + trigger_type 下 level MUST 唯一

#### Scenario: 查询 SLA 的升级规则链
- **WHEN** 请求 GET /api/v1/itsm/sla/:id/escalations
- **THEN** 系统返回该 SLA 的升级规则列表，按 level 升序

### Requirement: 服务目录前端浏览

系统 SHALL 提供面向用户的服务目录浏览页面（区别于管理页面），用于经典入口提单。

#### Scenario: 用户浏览服务目录
- **WHEN** 用户访问 `/itsm/catalog`
- **THEN** 系统 SHALL 展示分类树形导航 + 服务卡片列表，仅显示 is_active 为 true 的分类和服务

#### Scenario: 按分类筛选
- **WHEN** 用户点击左侧某个分类节点
- **THEN** 系统 SHALL 筛选展示该分类及其子分类下的所有启用服务

#### Scenario: 搜索服务
- **WHEN** 用户在搜索框输入关键词
- **THEN** 系统 SHALL 按服务名称和描述模糊匹配，展示结果列表

#### Scenario: 发起服务请求
- **WHEN** 用户点击某个经典服务的"申请"按钮
- **THEN** 系统 SHALL 根据该服务的 form_schema 渲染动态表单供用户填写

### Requirement: 内置智能服务定义种子数据

系统 SHALL 在 Seed 阶段通过 `seedServiceDefinitions()` 函数内置 5 个智能服务定义。所有服务 engine_type 为 "smart"，workflowJSON 为空（后续处理），通过 catalog_code 和 sla_code 关联分类和 SLA。使用 code 字段做幂等检查。内置服务的 collaborationSpec/participant 配置 SHALL 与 built-in Org position seed 和 install-time admin identity 对齐，使 fresh install 下的 `validate_participants` 结果与真实路由行为一致，不因 seed 失配产生假失败。

| code | 名称 | 分类 code | SLA code | 含 Actions |
|------|------|-----------|----------|-----------|
| `copilot-account-request` | Copilot 账号申请 | `account-access:provisioning` | `rapid-workplace` | 否 |
| `boss-serial-change-request` | 高风险变更协同申请（Boss） | `application-platform:release` | `infra-change` | 否 |
| `db-backup-whitelist-action-flow` | 生产数据库备份白名单临时放行申请 | `application-platform:database` | `infra-change` | 是（2个） |
| `prod-server-temporary-access` | 生产服务器临时访问申请 | `infra-network:compute` | `critical-business` | 否 |
| `vpn-access-request` | VPN 开通申请 | `infra-network:network` | `standard` | 否 |

每个服务的 collaborationSpec 内容从 bklite-cloud 参考实现的 `buildin/init.yml` 直接复制：

1. **copilot-account-request**: "收集提单用户的Github账号信息和申请理由（可选），交给信息部的IT管理员审批，审批通过后结束流程。"
2. **boss-serial-change-request**: 收集申请主题、类别、风险等级、时间、影响范围、回滚要求、影响模块、变更明细表。先交 serial-reviewer 审批，再交 it 部 ops_admin 岗位审批。
3. **db-backup-whitelist-action-flow**: 进入申请节点时执行预检动作，提交后交 it 部与 built-in Org 种子一致的数据库管理员岗位审批，通过后执行白名单放行动作。
4. **prod-server-temporary-access**: 收集访问服务器、时段、目的、原因。按原因路由到 ops_admin / network_admin / security_admin 审批。
5. **vpn-access-request**: 收集 VPN 账号、设备用途、访问原因。按原因路由到 network_admin / security_admin 审批。

#### Scenario: 首次安装种子服务
- **WHEN** 系统首次安装，数据库无服务定义数据
- **THEN** 系统 SHALL 创建全部 5 个服务定义，每个关联正确的 catalog 和 SLA

#### Scenario: Fresh install participant validation succeeds for built-in services
- **WHEN** fresh install 完成后对内置 5 个智能服务执行 `validate_participants`
- **THEN** 使用 built-in Org seed 和 install-time admin 默认身份时，验证结果 SHALL 与服务设计的真实参与人路由一致
- **AND** 不得仅因 seed 中岗位编码不一致而失败

#### Scenario: 幂等重复执行
- **WHEN** 系统重启执行 Sync，数据库已有 seed 创建的服务定义
- **THEN** 系统 SHALL 跳过已存在的记录（按 code 匹配），不覆盖用户修改

#### Scenario: 关联分类不存在
- **WHEN** seed 执行时某服务引用的 catalog_code 在数据库中不存在
- **THEN** 系统 SHALL 记录 slog.Error 日志并跳过该服务，不中断其他服务的 seed

#### Scenario: 关联 SLA 不存在
- **WHEN** seed 执行时某服务引用的 sla_code 在数据库中不存在
- **THEN** 系统 SHALL 记录 slog.Warn 日志，将 sla_id 设为 nil，继续创建服务

### Requirement: 内置服务动作种子数据

系统 SHALL 在 `seedServiceDefinitions()` 中为 `db-backup-whitelist-action-flow` 服务额外创建 2 个 ServiceAction：

| code | 名称 | HTTP Method | 说明 |
|------|------|-------------|------|
| `backup_whitelist_precheck` | 备份白名单预检 | POST | 校验数据库、时间窗与来源 IP 是否齐备 |
| `backup_whitelist_apply` | 执行备份白名单放行 | POST | 审批通过后自动执行白名单放行 |

每个 Action 的 config_json 包含 url（占位 `/precheck` 和 `/apply`）、method（POST）、timeout_seconds（5）。使用 code 字段做幂等检查。

#### Scenario: 首次安装种子动作
- **WHEN** 系统首次安装
- **THEN** 系统 SHALL 为 db-backup-whitelist 服务创建 2 个 ServiceAction

#### Scenario: 幂等重复执行
- **WHEN** 系统重启执行 Sync
- **THEN** 系统 SHALL 跳过已存在的动作记录（按 service_id + code 匹配）

### Requirement: API 路由注册

ITSM App 的服务目录与服务定义 API SHALL 注册在以下前缀下，使用 JWT + Casbin 中间件保护。

#### Scenario: 服务目录 API
- **WHEN** 请求发送到 `/api/v1/itsm/catalogs/*`
- **THEN** 系统 SHALL 路由到 CatalogHandler 处理，支持 CRUD + 树查询

#### Scenario: 服务定义 API
- **WHEN** 请求发送到 `/api/v1/itsm/services/*`
- **THEN** 系统 SHALL 路由到 ServiceHandler 处理，支持 CRUD + 模板 + Spec 生成

#### Scenario: 动作定义 API
- **WHEN** 请求发送到 `/api/v1/itsm/actions/*`
- **THEN** 系统 SHALL 路由到 ActionHandler 处理，支持 CRUD

#### Scenario: 权限控制
- **WHEN** 未授权用户访问 ITSM API
- **THEN** Casbin 中间件返回 403
