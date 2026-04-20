## Context

当前 AI Agent 模块使用单一 `ai_agents` 表、单一 `/api/v1/ai/agents` API、单一前端页面和统一 `ai:agent:*` 权限承载 `assistant`、`coding`、`internal` 三种类型。后端 Gateway 的 `selectExecutor()` 已经按类型分叉（assistant → ReactExecutor/PlanAndExecuteExecutor，coding → LocalCodingExecutor/RemoteCodingExecutor），但 Gateway 主循环（session 加载、prompt 组装、事件流消费、消息持久化）完全统一，不关心 agent 类型。

前端创建表单通过 `type` 下拉切换两套完全不同的配置域（模型/策略 vs Runtime/工作区），用户在同一页面看到不相关的字段。权限无法区分"允许创建助手智能体但不允许创建编码智能体"。

已有 `harden-ai-agent-isolation-and-protocol-abstraction` change 完成了 agent/session/memory 的用户隔离校验和流式协议抽象，本次变更不涉及这些安全边界，只做 API 和 UI 层面的类型拆分。

## Goals / Non-Goals

**Goals:**

- 在产品、API、权限、前端四个层面将 assistant 和 coding 拆分为两个独立子域
- 每个子域拥有独立的菜单、权限集、API routes、页面和表单
- 编码智能体可被独立授权，满足更高安全等级的控制需求
- 保持 Gateway、Session、Memory 等统一核心逻辑不变
- 现有已创建的 agent 数据无需迁移，通过新 typed routes 可正常访问

**Non-Goals:**

- 不拆数据库表——`ai_agents` 单表结构不变，coding 字段对 assistant 行为空，反之亦然
- 不拆后端 service/repo 包结构——核心逻辑仍在 `internal/app/ai/` 下
- 不改 Gateway `Run()` 主循环和 Executor 接口
- 不改 Session/Memory API（仍走 `/api/v1/ai/sessions` 和 `/api/v1/ai/agents/:id/memories`）
- 不改前端 Chat 页面（`/ai/chat/:sid` 保持不变）
- 不做 `internal` 类型 agent 的对外可见化

## Decisions

### Decision: 不拆表，只在 API+权限+UI 层拆分

Gateway 主循环完全统一（load session → build prompt → dispatch executor → consume events → persist），真正按类型分叉的只有 `selectExecutor()` 和 `validateByType()` 两处。Session、Memory、AgentKnowledgeBase 等关联表都通过 `agent_id` FK 关联，如果拆表，这些关联全部需要适配，Gateway 也需要先判断类型再决定查哪张配置表。

收益（字段不混）不值得代价（JOIN 复杂化、Gateway 适配、数据迁移、绑定表分裂）。

不采用的方案：
- 拆成 `ai_agents` + `ai_assistant_agent_configs` + `ai_coding_agent_configs`。原因是 Gateway 不关心类型，拆表后反而需要在 Gateway 路径上增加类型判断逻辑，同时 session/memory 的 FK 关联变复杂。

### Decision: 不拆后端包结构，新增 typed handler 薄壳

真正需要"知道类型"的后端代码只有：handler（强制类型）、service.validateByType、repo.List 默认排除 internal、seed 数据。把这些拆成 `assistant_agent/` 和 `coding_agent/` 两个子包会导致两个包各自 95% 代码是胶水（调用同一个 core service），维护成本大于收益。

采用方案：在现有 `internal/app/ai/` 下新增 `assistant_agent_handler.go` 和 `coding_agent_handler.go` 两个文件，每个约 120 行，作为薄壳包装器调用 `AgentHandler` / `AgentService` 的核心方法，强制注入类型参数。

### Decision: typed API 路由设计

新路由：
- `/api/v1/ai/assistant-agents` — 所有操作强制 `type=assistant`
- `/api/v1/ai/coding-agents` — 所有操作强制 `type=coding`

每组包含：`GET /`、`POST /`、`GET /:id`、`PUT /:id`、`DELETE /:id`、`GET /templates`

行为规则：
- Create 不接受前端传 `type`，由 handler 强制写入
- List 内部自动加 `type` 过滤
- Get/Update/Delete 必须校验实际记录类型匹配，不匹配返回 404（与 harden change 的 not-found 策略一致）
- 模板接口按类型自动过滤

旧 `/api/v1/ai/agents` 路由保留为内部使用（system agent lookup by code 等），从 seed 中移除其对外权限。

### Decision: 权限模型重建

废弃 `ai:agent:list/create/update/delete`，替换为：
- `ai:assistant-agent:list/create/update/delete`
- `ai:coding-agent:list/create/update/delete`

seed 中需要做兼容处理：先检查旧权限是否存在，如果存在则给已有角色补发新权限，再清理旧权限。这样升级不会导致管理员突然失去访问权限。

### Decision: 菜单拆分策略

当前「智能体」分组下只有一个「Agent」菜单。改为两个三级菜单：
- `助手智能体` → `/ai/assistant-agents`，permission: `ai:assistant-agent:list`
- `编码智能体` → `/ai/coding-agents`，permission: `ai:coding-agent:list`

旧 `ai:agent:list` 菜单在 seed 中 soft-delete。

### Decision: 前端页面组织

目录结构：
```
web/src/apps/ai/pages/
  assistant-agents/
    index.tsx
    create.tsx
    [id].tsx
    [id]/edit.tsx
  coding-agents/
    index.tsx
    create.tsx
    [id].tsx
    [id]/edit.tsx
  _shared/
    agent-list-page.tsx
    agent-detail-page.tsx
    agent-form-common.tsx
    binding-checkbox-list.tsx   (从旧位置移入)
```

`assistant-agents/index.tsx` 和 `coding-agents/index.tsx` 是薄壳页面，传配置给 `_shared/agent-list-page.tsx`。表单不再是一个超级组件按 `type` 切换，而是每个创建/编辑页直接组合公共字段 + 类型专属字段。

### Decision: 前端 API 抽象

新增 `assistantAgentApi` 和 `codingAgentApi`，指向各自 typed route。废弃旧 `agentApi`。

类型定义拆为：
- `AssistantAgentInfo` — 只含 assistant 字段
- `CodingAgentInfo` — 只含 coding 字段
- `AgentBase` — 公共字段（name, description, visibility 等）

### Decision: Session 创建路径不改

`POST /api/v1/ai/sessions` 仍然接受 `agentId`，不按类型拆。原因是 session 是 agent 的下游资源，session 不关心 agent 类型（Gateway 内部处理），拆 session API 没有产品价值。chat 页面也保持 `/ai/chat/:sid` 不变。

## Risks / Trade-offs

- [Risk] 旧角色权限失效 → Mitigation: seed 中检测旧 `ai:agent:*` 权限，自动给持有者补发新 `ai:assistant-agent:*` + `ai:coding-agent:*` 权限，再清理旧权限
- [Risk] 旧 `/api/v1/ai/agents` 可能被外部脚本或 ITSM 模块引用 → Mitigation: 保留旧路由供内部使用，只从 seed 中移除对外 Casbin policy；搜索 codebase 确认无外部直接调用
- [Risk] 前端页面重构量较大 → Mitigation: 抽取 `_shared/` 层复用现有组件逻辑，每个类型页面只是配置不同的薄壳
- [Trade-off] `ai_agents` 表仍有空字段（assistant 行的 runtime 为空，coding 行的 model_id 为空）→ 接受此代价，换取 Gateway/Session/Memory 零改动

## Migration Plan

1. 后端先新增 typed handler + typed routes + EnsureType()，新旧路由并存
2. 后端重写 seed：新增菜单/权限/模板，兼容升级旧权限
3. 前端新建 `assistant-agents/` 和 `coding-agents/` 页面，切换到 typed API
4. 前端删除旧 `pages/agents/` 和旧 `agentApi`
5. 后端从 seed 中移除旧 `ai:agent:*` 的对外 Casbin policy
6. 验证全量功能：创建/编辑/删除/列表/详情/开始对话/模板

回滚策略：由于数据库不变，回滚只需要恢复旧代码。旧 agent 数据完全兼容。

## Open Questions

- Memory API 目前挂在 `/api/v1/ai/agents/:id/memories`，这个路径保持不变还是也分到 typed route 下？初步建议不动，因为 memory 的业务语义不区分 agent 类型。
- 是否需要在两种类型间做更细的绑定限制（比如 coding agent 不允许绑定 Knowledge Base）？初步建议不做硬限制，由前端表单控制展示即可。
