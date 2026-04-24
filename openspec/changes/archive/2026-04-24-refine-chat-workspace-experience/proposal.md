## Why

`unify-chat-workspace` 已经把 AI 管理和 ITSM 服务台收敛到共享 Chat Workspace，但共享层目前只解决了复用问题，尚未提供足够明确的设计语义。业务页仍可能通过裸 `className` 或 `compact` 修补核心尺寸，导致 ITSM 欢迎态等场景被共享组件拉坏，体验停留在“后台系统里塞聊天框”。

需要在不回滚共享架构、不新增后端协议的前提下，把 Chat Workspace 精修成可持续复用的高完成度对话工作台：AI 管理和 ITSM 服务台共享同一套高级交互能力，ITSM 通过服务受理岗、工单草稿 Surface、图片诉求和智能岗位入口保留业务特色。

## What Changes

- 为 `ChatComposer` 增加明确设计语义：`variant`、`maxWidth`、`minRows`、`showToolbarHint`、`attachmentTone`。
- 为 `ChatWorkspace` 增加布局语义：`density`、`messageWidth`、`composerPlacement`、`emptyStateTone`。
- 精修 ITSM 欢迎态，让内容视觉重心稳定在首屏中部偏上，Composer 不随工作区无限拉伸。
- 精修消息流、流式状态、工具活动、Surface 和错误恢复的视觉节奏。
- 精修 ITSM `itsm.draft_form` Surface，使 loading、editable、submit error、submitted 四态更像 AI 整理出的服务申请草稿，而不是普通后台表单。
- 精修 `SessionSidebar`，AI 管理保留完整会话操作，ITSM 使用更安静的服务台会话列表。
- 更新 `DESIGN.md`，明确业务页面不得用裸样式修核心对话布局，只能选择共享组件提供的设计语义。

## Impact

- 影响共享组件：`web/src/components/chat-workspace/`。
- 影响 AI 管理对话入口：`web/src/apps/ai/pages/chat/[sid].tsx`、`web/src/apps/ai/pages/chat/index.tsx`。
- 影响 ITSM 服务台入口：`web/src/apps/itsm/pages/service-desk/index.tsx`。
- 不改 `AgentSession`、SSE、图片上传 API 或 ITSM 工单提交协议。
