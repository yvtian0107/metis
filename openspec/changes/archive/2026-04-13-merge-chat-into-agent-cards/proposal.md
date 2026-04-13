## Why

当前 AI 模块的导航有两个独立菜单项：「智能体」（表格列表管理页）和「对话」（先选 Agent 再聊天的入口页）。用户要聊天必须切到"对话"页面，这个页面本质上只是一个 Agent 选择器，功能重复且多了一次跳转。将聊天入口合并到 Agent 卡片上，减少导航层级、提升交互直觉性，同时卡片视图比表格更适合展示 Agent 的个性化信息。

## What Changes

- **移除「对话」菜单项**：删除 `/ai/chat` 路由的独立菜单入口和 `chat/index.tsx`（Agent 选择页面），保留 `/ai/chat/:sid` 聊天界面路由
- **Agent 列表页改为卡片视图**：将 `/ai/agents` 页面从 DataTable 表格改为响应式卡片网格，每张卡片展示图标、名称、类型、描述、状态
- **卡片内嵌聊天入口**：每张活跃 Agent 卡片上增加一个「聊天」小按钮，点击后直接创建新 Session 并跳转到 `/ai/chat/:sid`
- **卡片操作菜单**：卡片右上角提供更多操作（编辑、查看详情、删除），替代原表格行内操作列
- **保留后端菜单种子**：删除"对话"菜单的种子数据（或标记为隐藏），添加/更新"智能体"菜单的权限

## Capabilities

### New Capabilities

（无新增 capability）

### Modified Capabilities

- `ai-agent-ui`: Agent 列表页从表格改为卡片网格布局，卡片内嵌聊天按钮；移除独立的对话选择页面和对应菜单项

## Impact

- **前端路由**：`module.ts` 移除 `/ai/chat` index 路由，保留 `/ai/chat/:sid`
- **前端页面**：删除 `pages/chat/index.tsx`；重写 `pages/agents/index.tsx` 为卡片布局
- **后端种子**：`ai/seed.go` 中移除"对话"菜单条目（或软删除），调整 Casbin 策略
- **翻译文件**：更新 `zh-CN.json` / `en.json` 中 Agent 卡片相关的新文案
- **无 API 变更**：后端 Agent 和 Session API 不变
