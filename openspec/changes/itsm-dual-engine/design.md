## Context

Metis 是一个 Go + React 单体应用，已有 AI App（Agent/Knowledge/LLM）、Org App（部门/岗位）、Scheduler（定时/异步任务）等成熟子系统。本设计在现有架构上新增 ITSM App，核心挑战是实现"经典 BPMN"与"Agent 全驱动"双引擎共存，且共享统一的工单数据层。

参考项目 bklite-cloud（Django + React）已实现完整的 Agentic ITSM，但过于激进——每步流转都由 LLM 决定、无确定性 fallback、Collaboration Spec 编写门槛高。本设计保留其 AI 能力精华，同时补齐经典确定性流程引擎。

## Goals / Non-Goals

**Goals:**

- 实现服务级别的引擎选择：管理员创建服务时选择"经典"或"智能"，两种模式完全独立运行
- 经典引擎：ReactFlow 可视化编辑器 + 确定性状态机，workflow_json 即执行源
- 智能引擎：Collaboration Spec + Agent 决策循环，移植 bklite-cloud 核心逻辑并增加信心机制和人工覆盖
- 统一工单层：Ticket/Activity/Assignment/Timeline/SLA/Report 不分引擎
- 双入口提单：经典入口（ITSM 服务目录填表）、智能入口（AI Agent 对话提单）
- Agent 绑定：智能服务引用 AI App 的 Agent，不在 ITSM 内部重建 AI 基础设施
- ITSM 向 AI App 注册 Builtin Tool，供 Agent 操作工单

**Non-Goals:**

- CMDB（配置管理数据库）— 后续独立规划
- 问题管理（Problem Management）— P2 阶段
- 变更管理（Change Management）— P2 阶段
- 发布管理 — P3 阶段
- 自定义流程引擎的可扩展接口（不做通用 BPM 平台）
- 移动端适配

## Decisions

### D1: 双引擎通过统一接口隔离，而非混合模式

**选择**: 定义 `WorkflowEngine` 接口，ClassicEngine 和 SmartEngine 各自实现。Service 层根据 `ServiceDefinition.EngineType` 分派。

**替代方案**: 方案 C — 在 BPMN 骨架中嵌入 AI 节点（混合模式）。

**理由**: 混合模式既限制了 AI 能力（不能动态创建路径），又增加了编辑器复杂度。两个纯粹的引擎各自做到极致，代码边界清晰，维护成本反而更低。

```go
type WorkflowEngine interface {
    Start(ctx context.Context, ticket *Ticket) error
    Progress(ctx context.Context, ticket *Ticket, activity *TicketActivity, outcome string) error
    Cancel(ctx context.Context, ticket *Ticket, reason string) error
}
```

### D2: 智能服务的 AI 能力完全委托给 AI App 的 Agent 体系

**选择**: SmartEngine 持有 AI App 的 service 引用（AgentService、LLM Client），通过 IOC 注入。智能服务的 `ServiceDefinition` 包含 `AgentID` 字段引用 AI App 的 Agent。

**替代方案**: 在 ITSM App 内部自建 LLM 调用和 prompt 管理。

**理由**: AI App 已经建好了完整的灵魂体系（模型选择、知识库、工具绑定、MCP Server、Skill、温度、system prompt），重复建设无意义。ITSM 只需组装工单上下文并传给 Agent。

### D3: 智能服务对话提单通过 Agent Tool 实现，而非专用 API

**选择**: ITSM App 向 AI App 的 Tool Registry 注册 `itsm.create_ticket`、`itsm.query_ticket` 等 Builtin Tool。用户在 AI Chat 中与"IT 服务台"Agent 对话时，Agent 通过工具调用创建工单。

**替代方案**: 在 Agent Chat UI 中嵌入特殊的工单创建表单组件。

**理由**: 工具调用是 Agent 的原生能力，不需要前端特殊处理。Agent 可以在对话中灵活决定何时创建工单、如何填充字段。工单创建后 `agent_session_id` 关联对话，处理人可回溯完整上下文。

**Tool 注册方式**: ITSM App 在 `Providers()` 中不直接向 AI App 注册（避免硬耦合）。改为在 ITSM App 的 `app.go` 中实现一个 `ToolProvider` 接口，main.go 启动时检测并注册。或更简单地，ITSM Handler 提供 REST API，管理员在 AI App 中手动创建引用这些 API 的 Tool 定义。

**推荐**: 采用代码注册方式 — ITSM App 在 `Providers()` 中通过 IOC 获取 AI App 的 ToolRegistry（如果可用），注册 Builtin Tool。AI App 不存在时静默跳过。

### D4: 经典工作流引擎基于 ReactFlow JSON 的图遍历

**选择**: 经典引擎直接解析 `workflow_json`（ReactFlow 格式），按节点和边的拓扑关系遍历。不引入第三方 BPM 引擎。

**替代方案**: 引入 Go 的 BPM 库（如 bpmn-engine）。

**理由**: ITSM 工作流是有限场景（表单/审批/处理/动作/网关/通知/等待/结束），不需要通用 BPM 的复杂性（子流程、信号事件、补偿等）。自建引擎代码可控、测试方便，且复用老代码已验证的 ReactFlow Schema。

**节点类型枚举**:
- `start` — 开始
- `form` — 表单（提交/补充信息）
- `approve` — 审批（通过/驳回）
- `process` — 处理（填写处理结果）
- `action` — 动作（Webhook 自动执行）
- `gateway` — 网关（条件分支）
- `notify` — 通知
- `wait` — 等待（外部信号/定时）
- `end` — 结束

### D5: 智能引擎保留 bklite-cloud 的决策循环，增加信心机制

**选择**: 移植 bklite-cloud 的 `TicketCase → Policy → Agent Decision → Validate → Progress` 循环。新增：
- `confidence` 字段：Agent 返回置信度
- 信心阈值（服务级配置）：高于阈值自动执行，低于阈值等待人工确认
- 人工覆盖：每个 AI 决策点可被人工替代

**替代方案**: 完全复刻老系统，无信心机制。

**理由**: 中腰部客户需要渐进式信任建立。信心机制让管理员可以从"全部人工确认"逐步调到"大部分自动"。

### D6: SLA 引擎统一，不分经典/智能

**选择**: SLA 定义（SLATemplate）和 SLA 检查（Scheduler 任务）完全统一。工单创建时根据服务绑定的 SLA 模板计算 deadline，定时任务检查超时并触发升级。

**理由**: SLA 是管理侧关心的指标，与工单如何流转无关。

### D7: Ticket.source 字段区分提单入口

**选择**: 工单增加 `source` 字段（`catalog` | `agent`）+ `agent_session_id`（关联 AI 对话）。

**理由**: 处理人需要知道工单来源以判断上下文完整度。Agent 提单的工单天然附带完整对话记录。

### D8: 工作流 JSON Schema 复用 bklite-cloud 的验证逻辑

**选择**: 移植 bklite-cloud 的 `workflow_schema.py` 中的验证规则到 Go，确保：一个 start、至少一个 end、无孤立节点、edge 的 source/target 合法、gateway 边必须有条件。

**理由**: 已验证的 schema，减少设计风险。

## Risks / Trade-offs

- **[风险] 智能引擎对 AI App 的强依赖** → 缓解：SmartEngine 通过 IOC 延迟解析 AI App 服务，AI App 不存在时智能服务创建被禁用（UI 灰掉），经典服务不受影响
- **[风险] Agent 决策延迟影响用户体验** → 缓解：决策异步执行（Scheduler async task），前端通过轮询/SSE 获取进展；设置决策超时（30s），超时转人工
- **[风险] ReactFlow JSON 格式演进** → 缓解：后端仅解析节点/边的 id、type、data、source、target 字段，不依赖 ReactFlow 特有的布局属性
- **[权衡] 两套引擎的维护成本** → 接受：两套引擎共享 Ticket/Activity/Assignment 数据层（占总代码量 60%+），引擎本身的差异化代码可控
- **[权衡] Tool 注册的耦合** → 接受：ITSM 通过 IOC 可选注入 AI App 服务，AI App 不存在时 Tool 不注册，不影响经典功能
- **[风险] Collaboration Spec 编写质量参差** → 缓解：提供模板库 + AI 生成 Spec 功能，降低编写门槛
