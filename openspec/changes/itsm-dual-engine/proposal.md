## Why

Metis 缺少 ITSM（IT 服务管理）能力，而中腰部客户（IT 团队 10~100 人）急需从 Excel/IM 群组工单模式升级为系统化服务管理。市面上 ServiceNow 成本过高、Jira SM 过于技术化，这一客群存在明显空白。同时，纯 AI 驱动的 ITSM（如老项目 bklite-cloud）因用户信任和可控性不足而接受度低——需要经典确定性流程与 AI Agentic 模式并存，让用户按服务场景自主选择。

## What Changes

- 新增 ITSM App（`internal/app/itsm/`），实现完整的 IT 服务管理能力
- **双引擎架构**：服务定义时选择"经典服务"（BPMN 状态机）或"智能服务"（Agent 全驱动）
  - 经典服务：ReactFlow 可视化流程编辑器 + 确定性状态机引擎
  - 智能服务：Collaboration Spec + AI Agent 决策循环（移植自 bklite-cloud 并改进）
- **双入口提单**：
  - 经典入口：ITSM 模块内浏览服务目录 → 选服务 → 填表单 → 提交
  - 智能入口：AI 模块内通过"IT 服务台"Agent 对话式提单
- **Agent 绑定（灵魂与躯壳）**：智能服务引用 AI App 现有 Agent 体系，不重复造 AI 基础设施
  - IT 服务台 Agent（用户侧，对话提单）
  - 流程决策 Agent（系统侧，工单流转决策）
  - 处理协助 Agent（处理人侧，工单 Copilot）
- **统一层**：Ticket / SLA / Timeline / Assignment / Report / Notification 不分引擎，共用数据模型和 UI
- ITSM App 向 AI App 注册 Builtin Tool（create_ticket、query_ticket 等），供 Agent 操作 ITSM
- 服务目录（树形分类）、优先级定义、SLA 模板、动作系统（Webhook 自动化）
- 工单报表与仪表盘（吞吐量、SLA 达成率、解决时长）
- 故障管理（P0~P4 分级 + 升级链）
- 知识库集成（复用 AI App 的 Knowledge 模块）

## Capabilities

### New Capabilities

- `itsm-service-catalog`: 服务目录树形分类管理，服务定义（含双模式选择、表单 Schema、SLA 绑定）
- `itsm-classic-engine`: 经典工作流引擎——ReactFlow 可视化编辑器 + 确定性状态机执行 + 条件路由
- `itsm-smart-engine`: 智能工作流引擎——Collaboration Spec 配置 + Agent 决策循环 + 信心机制 + 人工覆盖
- `itsm-ticket-lifecycle`: 工单生命周期——创建/流转/派单/处理/完结，统一数据模型（Activity/Assignment/Timeline）
- `itsm-sla`: SLA 引擎——响应时间/解决时间/升级策略/SLA 检查定时任务
- `itsm-agent-tools`: ITSM 向 AI App 注册的工具集（create_ticket、query_ticket 等），支撑对话式提单和 Agent 操作
- `itsm-reporting`: 工单报表与仪表盘——吞吐量、SLA 达成率、解决时长、分类统计
- `itsm-incident`: 故障管理——优先级分级（P0~P4）、升级链、自动通知、故障生命周期

### Modified Capabilities

（无现有 capability 的需求变更）

## Impact

- **新增后端代码**：`internal/app/itsm/` — models、repository、service、handler、engine（classic + smart）
- **新增前端代码**：`web/src/apps/itsm/` — 服务目录、服务定义编辑器（含 ReactFlow）、工单列表/详情、报表
- **AI App 集成**：ITSM 通过 IOC 引用 AI App 的 AgentService / LLM Client / KnowledgeService；注册 Builtin Tool
- **Org App 集成**：派单时引用 Org App 的部门/岗位体系（OrgScopeResolver）
- **Scheduler 集成**：注册 SLA 检查、工单升级、超时检测等定时/异步任务
- **Edition 文件**：`cmd/server/edition_full.go` 增加 `import _ "metis/internal/app/itsm"`
- **前端 Bootstrap**：`web/src/apps/_bootstrap.ts` 增加 `import "./itsm/module"`
- **依赖新增**：前端需引入 `@xyflow/react`（ReactFlow）用于流程编辑器
- **Seed 数据**：初始菜单、Casbin 策略、默认优先级、默认 SLA 模板
