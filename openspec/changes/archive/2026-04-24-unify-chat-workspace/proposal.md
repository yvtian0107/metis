## Why

AI 管理的智能体对话和 ITSM 服务台对话已经共享同一套 Agent Session/SSE 能力，但前端页面层仍各自维护输入框、消息列表、滚动、停止、错误、会话侧栏和智能体展示逻辑，导致 ITSM 体验粗糙且未来协议、UI、UX 调整需要多处同步。

需要收敛为一个统一的对话工作台系统：AI 管理和 ITSM 使用同一套对话协议、输入能力和视觉语言，ITSM 通过业务 surface 和智能岗位语义保留自己的特色，而不是形成另一套对话产品。

## What Changes

- 新增共享 `ChatWorkspace` 前端能力，统一承载对话页的高度模型、header、智能体切换、会话侧栏、消息列表、输入框、附件/图片、停止、错误、滚动和空态。
- 将现有 AI Chat 的完整输入能力（多行自适应、Enter 发送、Shift+Enter 换行、图片粘贴/预览/上传、发送/停止状态）提升为共享能力，ITSM 服务台默认获得同等级图片输入能力。
- 将消息分组、tool activity、reasoning、plan、data surface 渲染整理为共享消息渲染机制，业务页面只能通过 surface renderer 注册业务 UI。
- 为 ITSM 注册 `itsm.draft_form` 等服务台业务 surface，保留工单草稿确认、表单编辑、提交工单等服务台特色。
- 统一 AI 管理和 ITSM 服务台的智能体展示与切换入口，保留 ITSM “服务受理岗智能体/智能岗位”语义，但不允许出现不同风格的 header 和切换控件。
- 删除 ITSM 服务台页面内重复实现的 composer、消息分组、滚动、停止按钮和粗糙会话壳层，改由共享对话工作台配置驱动。
- 不做向后兼容的双轨 UI；迁移完成后对话协议、输入框、消息渲染和智能体切换只能在共享层调整。

## Capabilities

### New Capabilities
- `chat-workspace-ui`: 跨业务共享的前端对话工作台能力，覆盖 AI 管理和 ITSM 服务台共用的对话布局、输入、消息渲染、surface 注册、智能体切换和会话侧栏。

### Modified Capabilities
- `ai-agent-chat-ui`: AI 智能体对话页改为使用共享 Chat Workspace，而不是维护独立页面级对话实现。
- `ai-chat-sidebar`: 会话侧栏能力改为可复用的 Chat Workspace sidebar，支持不同业务配置相同行为。
- `ai-data-stream-protocol`: 前端 data surface 消费规则明确为共享 surface registry，不允许业务页面直接分叉解析协议。
- `itsm-service-desk-toolkit`: ITSM 服务台工具链生成的交互 surface 必须通过共享 Chat Workspace renderer 展示，并支持图片上下文输入。

## Impact

- 影响前端共享组件：新增 `web/src/components/chat-workspace/` 或等价共享目录。
- 影响 AI 管理对话页：`web/src/apps/ai/pages/chat/index.tsx`、`web/src/apps/ai/pages/chat/[sid].tsx`、`web/src/apps/ai/pages/chat/components/*`。
- 影响 ITSM 服务台页：`web/src/apps/itsm/pages/service-desk/index.tsx`。
- 影响共享 API 使用：继续复用 `sessionApi`、`useAiChat`、Agent Session、图片上传和 SSE 流式协议，不新增后端协议分叉。
- 影响设计文档：需要同步 `DESIGN.md` 的对话工作台设计基线，明确 ITSM 与 AI 管理共享同一套对话风格。
