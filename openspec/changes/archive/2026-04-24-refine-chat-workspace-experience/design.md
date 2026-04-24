## Context

Chat Workspace 已经成为 AI 管理和 ITSM 服务台的共享对话层，但共享组件的视觉能力仍以低层 Tailwind 修补为主。尤其是 Composer 的宽度、欢迎态尺寸、消息密度和 Sidebar 形态没有被建模成稳定接口，导致业务页面容易重新分叉或把共享组件用坏。

## Decisions

### Decision 1: 设计语义替代裸样式修补

`ChatComposer` 和 `ChatWorkspace` 暴露稳定的产品语义，而不是要求业务页传核心布局 `className`。

- Composer: `variant`、`maxWidth`、`minRows`、`showToolbarHint`、`attachmentTone`
- Workspace: `density`、`messageWidth`、`composerPlacement`、`emptyStateTone`
- Sidebar: `variant`

业务页面只能选择这些语义来表达差异。宽度、阴影、最小高度、工具栏提示和消息区密度由共享层映射。

### Decision 2: ITSM 是专业服务台舞台，不是普通聊天空态

ITSM 欢迎态使用 `stage + standard + service-desk` Composer，视觉重心在首屏中部偏上。建议词贴近输入框，表示常见诉求入口，不做独立标签云。图片输入作为基础诉求能力保留。

### Decision 3: Surface 是 AI 工作结果

ITSM 草稿确认 Surface 视觉上表达“服务台已整理出的申请草稿”，四态统一在助手响应区内展示。提交成功后突出工单编号和后续路径，失败在卡片内部给出可修复错误。

### Decision 4: 消息流使用 Timeline 而不是 QAPair

Chat Workspace 按 `UIMessage[]` 时间线渲染，不再把“用户消息 + 助手消息”作为显示前提。用户消息必须能独立显示；助手文本、工具活动、reasoning、plan progress 和业务 Surface 作为后续 timeline item 渲染。空态只能基于可见消息数量判断，不能基于是否形成问答对判断。

### Decision 5: 精修不改变协议

本次只调整前端组件接口和视觉/交互细节，不新增后端字段，不改变 SSE part，不改变图片上传和工单提交 API。
