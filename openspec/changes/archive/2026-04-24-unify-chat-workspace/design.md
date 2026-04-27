## Context

当前 AI 管理智能体对话页和 ITSM 服务台都基于同一套 `AgentSession`、`sessionApi`、`useAiChat` 和 SSE 流式协议，但页面层实现已经分叉：

- AI 管理对话页拥有较完整的输入框、图片上传、消息编辑、停止生成、继续生成、记忆面板、会话侧栏和 tool/reasoning/plan 展示。
- ITSM 服务台复用了 `useAiChat` 和 `QAPair`，但自己实现了 composer、消息分组、滚动、停止按钮、header、会话栏和服务台 surface 注入，导致体验粗糙且和 AI 管理视觉语言不一致。
- ITSM 服务台并不是低配对话框，它同样需要图片输入、精致输入体验、统一消息协议和统一智能体切换入口，只是在业务上表达“服务受理岗智能体”和工单草稿确认 surface。

本设计将前端对话能力收敛为一个共享 Chat Workspace 系统。AI 管理和 ITSM 服务台只负责提供业务配置、surface renderer 和少量业务动作，不再各自维护一套对话 UX。

## Goals / Non-Goals

**Goals:**

- 建立一个共享对话工作台组件层，统一 AI 管理与 ITSM 服务台的布局、输入框、消息列表、滚动、错误、停止、会话侧栏和智能体 header。
- 让 ITSM 服务台获得与 AI 管理对齐的完整输入能力，包括图片粘贴、图片预览、图片上传和发送状态。
- 通过统一 surface registry 处理 reasoning、plan、tool activity、data-ui-surface 和业务 surface，避免页面直接解析协议。
- 保留 ITSM 自己的特色：服务台身份、服务受理岗智能体语义、智能岗位配置入口、工单草稿确认和提交工单 UI。
- 迁移后删除重复实现，不保留双轨对话组件或旧 ITSM 粗糙 composer。
- 同步 `DESIGN.md`，沉淀“一个系统，多业务配置”的对话工作台基线。

**Non-Goals:**

- 不重写 Agent Session 后端模型。
- 不新增独立 ITSM 会话协议；继续使用 AI Agent Session、图片上传和 SSE 流式协议。
- 不改变 ITSM 工具链的业务状态机和工单创建语义。
- 不引入新的前端状态管理库。
- 不为旧 ITSM 对话 UI 做兼容保留。

## Decisions

### Decision 1: 建立 `chat-workspace` 共享前端层

新增共享目录，例如 `web/src/components/chat-workspace/`，包含：

- `chat-workspace.tsx`: 页面级工作台骨架，管理 header、sidebar、message area、composer、stop/error/jump-to-bottom 区域。
- `use-chat-workspace.ts`: 包装 `useAiChat`、session 加载、发送、取消、继续、重试、图片上传、消息分组和滚动状态。
- `composer.tsx`: 统一输入框，支持文本、图片、粘贴、预览、上传、发送和停止状态。
- `message-list.tsx` / `message-pair.tsx`: 统一消息流和 QA pair 渲染。
- `session-sidebar.tsx`: 可配置会话侧栏，支持 AI 管理需要的重命名/删除/折叠，也支持 ITSM 的服务台会话列表。
- `chat-header.tsx` / `agent-switcher.tsx`: 统一智能体展示和切换入口。
- `surface-registry.tsx`: 将 data part、tool、reasoning、plan 和业务 surface 注册为 renderer。

Rationale: 共享层能让协议、UI、UX 的未来调整只落在一个地方。业务页面通过配置表达差异，而不是复制基础对话代码。

Alternative considered: 继续复用 `QAPair`，只把 ITSM composer 改好。这个方案只能局部改善粗糙输入框，不能解决 header、surface、滚动、错误和会话侧栏继续分叉的问题。

### Decision 2: Chat Workspace 以能力配置和 slots 表达业务差异

共享组件不 import `apps/itsm` 或具体 AI 管理页面。业务差异通过 props/slots 注入：

- `identity`: 当前智能体、业务上下文、状态点、切换行为。
- `sidebar`: 会话列表策略、创建会话、选中会话、业务标题。
- `composer`: placeholder、是否允许附件/图片、输入 hint。
- `surfaces`: 业务 surface renderer registry。
- `panels`: AI 管理的 memory panel、ITSM 的智能岗位引导等业务面板。
- `emptyState` / `welcomeStage`: 首屏或空会话业务欢迎态。

Rationale: 共享工作台必须足够稳定，业务页面只能组合，不应把业务 import 带进共享层。

Alternative considered: 在 AI Chat 页面内部加 `mode="itsm"`。这会把 ITSM 业务污染进 AI 管理目录，长期会变成条件分支堆积。

### Decision 3: 图片输入是共享默认能力，不是 AI 管理专属能力

`ConversationComposer` 统一处理图片粘贴、选择、预览、删除和上传，并通过现有 `sessionApi.uploadMessageImage` 与 `sessionApi.sendMessage(..., images)` 发送。ITSM 服务台默认开启图片输入。

Rationale: IT 服务请求经常需要截图、报错图、设备照片或审批材料，图片上下文是服务台的基础能力。将其做成共享能力可以避免 ITSM 重写上传流程。

Alternative considered: ITSM 单独实现图片上传。该方案会复制 API 调用、预览、错误和发送状态，违背统一对话系统目标。

### Decision 4: Surface registry 是唯一的前端协议消费入口

共享 `surface-registry` 负责识别并渲染：

- text response
- tool activity
- reasoning block
- plan progress
- generic data surface
- ITSM `itsm.draft_form` 等业务 surface

页面不再直接遍历 `UIMessage.parts` 并判断 `data-ui-surface`。业务只注册 renderer：

```ts
registerSurfaceRenderer({
  surfaceType: "itsm.draft_form",
  render: (ctx) => <ITSMDraftFormSurfaceCard {...ctx} />,
  suppressText: true,
})
```

Rationale: 以后调整 SSE part、surface payload、loading/submitted 生命周期，只改 adapter 和 registry。

Alternative considered: 继续在 `QAPair` 暴露 `renderDataPart`。这是一个有用过渡点，但长期仍让每个页面自行解析协议。

### Decision 5: 统一 header 和 AgentSwitcher，但保留 ITSM 岗位语义

AI 管理和 ITSM 服务台都使用同一套 `ChatHeader` 和 `AgentSwitcher` 视觉结构。差异只体现在文案和切换策略：

- AI 管理：切换智能体后进入或创建对应智能体会话。
- ITSM 服务台：展示“服务受理岗智能体”，切换入口指向可选智能体或智能岗位配置；未配置时显示未上岗状态和配置入口。

Rationale: 用户不应该在同一系统内看到多种对话 header 风格。统一组件可以保证未来换智能体设计只改一处。

Alternative considered: ITSM 保留自定义 header。该方案保留了当前粗糙分叉点，不符合本次 change 的目标。

### Decision 6: 迁移采用“抽共享层后替换调用”，不保留旧实现

实施时先抽出无业务依赖的共享能力，再将 AI 管理对话页迁移到共享层，最后迁移 ITSM 服务台并删除重复函数和局部 composer。迁移后不保留旧 ITSM composer、旧消息分组函数或页面内停止/滚动重复实现。

Rationale: 项目要求不做向后兼容设计、不留技术债。保留两套实现会让未来改动继续分叉。

Alternative considered: 新旧组件并存一段时间。对用户体验无收益，只增加维护面。

## Risks / Trade-offs

- 共享组件抽象过宽 → 通过 slots 和明确能力边界控制，禁止共享层 import 业务模块。
- 一次迁移影响 AI 管理和 ITSM 两个入口 → 分阶段提交：先抽共享层并迁移 AI 管理，再迁移 ITSM，最后删除旧实现并回归构建。
- Surface registry 设计不足以覆盖未来业务 surface → renderer 接口保留 `payload`、`message`、`session`、`actions` 上下文，避免只为 ITSM 特化。
- 图片上传失败导致发送状态复杂 → 上传由 workspace hook 串行聚合，失败时保留输入内容和图片预览，不清空 composer。
- ITSM 智能体切换可能涉及岗位配置而非普通 agent list → `AgentSwitcher` 只统一视觉和交互容器，切换策略由业务提供。

## Migration Plan

1. 新增 `chat-workspace` 共享组件层和类型，不改变页面入口。
2. 将 `groupUIMessagesIntoPairs`、tool activity、reasoning/plan renderer、message pair 等从 AI Chat 页面组件迁移到共享层。
3. 将 AI 管理对话页改为 `ChatWorkspace` 配置调用，确保现有图片、编辑、记忆、继续生成、删除会话能力保留。
4. 将 ITSM 服务台改为 `ChatWorkspace` 配置调用，启用共享图片输入和统一 header/AgentSwitcher，注册 `itsm.draft_form` renderer。
5. 删除 ITSM 页面内重复 composer、消息分组、滚动、停止按钮和局部粗糙会话壳。
6. 更新 `DESIGN.md` 中对话工作台设计基线。
7. 运行 `cd web && bun run lint`、`cd web && bun run build` 验证前端。

Rollback 策略：由于不保留双轨实现，如迁移失败应回滚整个 change 分支，而不是在主线保留旧 UI。

## Open Questions

- ITSM 的 AgentSwitcher 第一阶段是否只跳转智能岗位配置，还是允许在服务台内临时选择服务受理岗智能体。实现前需要按当前产品取舍确定。
- 通用附件是否只支持图片，还是同步为未来文件附件预留 UI。第一阶段建议明确为图片，避免后端协议超范围扩张。
