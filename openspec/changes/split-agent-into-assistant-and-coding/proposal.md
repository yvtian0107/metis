## Why

当前 AI 智能体在产品层只有一个统一入口（`/ai/agents`），创建表单通过 `type` 下拉切换 `assistant` / `coding`，导致用户在同一页面看到两套毫无关联的配置（模型/策略 vs Runtime/工作区），体验混乱。权限也是统一的 `ai:agent:*`，无法对编码型智能体做独立授权——而编码型有更高的安全风险（可执行任意代码）。后端 Gateway 执行链路已经在 `selectExecutor()` 处按类型分叉，但 API、菜单、权限、前端页面还停留在"一个资源"的抽象上。现在需要把这个隐含的分界线显式化。

## What Changes

- 将现有 `/ai/agents` 统一菜单拆分为两个独立菜单：「助手智能体」和「编码智能体」，各自拥有独立的权限集（`ai:assistant-agent:*` / `ai:coding-agent:*`）
- 新增两组 typed API routes（`/api/v1/ai/assistant-agents` / `/api/v1/ai/coding-agents`），每组强制绑定类型，不再由前端传 `type` 字段
- 前端拆分为两套独立页面（列表、创建、详情、编辑），表单不再有 `type` 选择器，由路由决定类型
- 前端表单按类型裁剪字段：助手表单只展示模型/策略/工具/知识库；编码表单只展示 Runtime/执行模式/MCP
- **不拆数据库表**：底层仍保留 `ai_agents` 单表 + 公共绑定表，Gateway/Session/Memory 代码不动
- **BREAKING**: 废弃对外的 `/api/v1/ai/agents` CRUD 路由（仅保留为内部/system agent 使用），前端全部迁移到 typed routes
- **BREAKING**: 废弃 `ai:agent:list/create/update/delete` 权限键，替换为 `ai:assistant-agent:*` 和 `ai:coding-agent:*`
- 模板接口拆到各自 typed route 下，按类型过滤

## Capabilities

### New Capabilities

- `ai-assistant-agent-api`: 助手智能体 typed API routes、typed handler 薄壳、typed 权限与 Casbin policies
- `ai-coding-agent-api`: 编码智能体 typed API routes、typed handler 薄壳、typed 权限与 Casbin policies
- `ai-assistant-agent-ui`: 助手智能体独立前端页面（列表、创建、详情、编辑）、助手专属表单字段
- `ai-coding-agent-ui`: 编码智能体独立前端页面（列表、创建、详情、编辑）、编码专属表单字段

### Modified Capabilities

- `ai-agent`: Agent entity model 增加 `kind` 语义别名（值不变，仍为 `assistant/coding/internal`），service 层增加 `EnsureType()` 校验方法；废弃对外通用 CRUD API
- `ai-management-navigation`: 「智能体」分组下从单一 Agent 菜单改为「助手智能体」+「编码智能体」两个三级菜单项
- `ai-agent-ui`: 废弃统一的 `/ai/agents` 页面及混合表单，由 `ai-assistant-agent-ui` 和 `ai-coding-agent-ui` 替代

## Impact

- 后端：`internal/app/ai/` 下新增 `assistant_agent_handler.go` / `coding_agent_handler.go` 两个薄壳 handler；修改 `agent_service.go`（加 EnsureType）、`app.go`（加 typed routes）、`seed.go`（重写菜单/权限/模板）；Gateway、Session、Memory 代码不受影响
- 前端：`web/src/apps/ai/pages/agents/` 整体重构为 `assistant-agents/` 和 `coding-agents/` 两组页面 + `_shared/` 共享组件；`module.ts` 导航和路由重写；`web/src/lib/api.ts` 新增 typed API、废弃旧 `agentApi`；locale 文件新增双域文案
- 数据库：无 schema 变更，无数据迁移
- 现有已创建的 agent 数据不受影响，通过新 typed routes 可正常访问
- Casbin 权限需要重新 seed 并给已有角色分配新权限键
