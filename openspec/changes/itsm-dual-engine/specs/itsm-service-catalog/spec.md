## ADDED Requirements

### Requirement: 服务目录树形分类管理
系统 SHALL 提供服务目录（ServiceCatalog）实体，支持树形分类结构。字段包括：name（名称）、code（唯一编码）、description（描述）、parent_id（父分类 ID，自关联，顶层为 null）、sort_order（排序）、icon（图标）、is_active（是否启用）。内嵌 BaseModel 提供 ID + 时间戳 + 软删除。

#### Scenario: 创建顶层分类
- **WHEN** 管理员请求 `POST /api/v1/itsm/catalogs` 并传入 name、code，parent_id 为空
- **THEN** 系统 SHALL 创建一个顶层服务分类并返回完整记录

#### Scenario: 创建子分类
- **WHEN** 管理员请求 `POST /api/v1/itsm/catalogs` 并传入有效的 parent_id
- **THEN** 系统 SHALL 创建子分类，parent_id 指向已有的父分类

#### Scenario: 编码唯一性校验
- **WHEN** 管理员创建或更新分类时使用已存在的 code
- **THEN** 系统 SHALL 返回 409 冲突错误

#### Scenario: 查询分类树
- **WHEN** 用户请求 `GET /api/v1/itsm/catalogs/tree`
- **THEN** 系统 SHALL 返回完整的树形分类结构，每个节点包含 children 数组

#### Scenario: 删除含子分类的目录
- **WHEN** 管理员删除一个含有子分类的目录
- **THEN** 系统 SHALL 返回 400 错误，提示需先删除或移动子分类

#### Scenario: 删除含服务定义的目录
- **WHEN** 管理员删除一个已绑定服务定义的目录
- **THEN** 系统 SHALL 返回 400 错误，提示需先解除服务绑定

### Requirement: 服务定义管理
系统 SHALL 提供服务定义（ServiceDefinition）实体，代表一个可请求的 IT 服务。字段包括：name（名称）、code（唯一编码）、description（描述）、catalog_id（所属分类 FK）、engine_type（引擎类型："classic" | "smart"）、sla_response_hours（SLA 响应时限，小时）、sla_resolve_hours（SLA 解决时限，小时）、priority_default（默认优先级）、is_active（是否启用）。内嵌 BaseModel。

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

### Requirement: 经典服务引擎配置
engine_type 为 "classic" 的服务定义 SHALL 额外持有以下配置字段：workflow_json（ReactFlow 格式的工作流 JSON）、form_schema（JSON Schema 格式的表单定义）。这些字段存储在 ServiceDefinition 表中（JSON 列）。

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
系统 SHALL 提供动作定义（ServiceAction）实体，代表服务流程中可触发的自动化动作。字段包括：name（名称）、code（唯一编码）、service_id（FK 关联服务定义）、action_type（类型，当前支持 "http"）、config（JSON，HTTP 类型含 url、method、headers、body_template、timeout_seconds）、is_active（是否启用）。内嵌 BaseModel。

#### Scenario: 创建 HTTP 动作
- **WHEN** 管理员请求 `POST /api/v1/itsm/actions` 并传入 action_type 为 "http"、config 含 url 和 method
- **THEN** 系统 SHALL 校验 config 中 url 和 method 必填后创建动作

#### Scenario: 动作列表按服务筛选
- **WHEN** 用户请求 `GET /api/v1/itsm/actions?service_id=xxx`
- **THEN** 系统 SHALL 返回该服务下的所有动作定义

#### Scenario: body_template 变量替换
- **WHEN** 动作的 body_template 中含有 `{{.ticket.code}}`、`{{.form_data.xxx}}` 等模板变量
- **THEN** 系统 SHALL 在执行时使用 Go text/template 引擎将工单数据注入模板

#### Scenario: 删除已被工作流引用的动作
- **WHEN** 管理员删除一个正在被经典工作流 action 节点引用的动作
- **THEN** 系统 SHALL 返回 400 错误，提示动作正在使用中

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
