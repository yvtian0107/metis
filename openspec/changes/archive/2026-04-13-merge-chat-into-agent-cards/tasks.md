## 1. 后端种子数据清理

- [x] 1.1 从 `seed.go` 的 Install 流程中移除"对话"菜单创建代码（第 184-200 行的 chatMenu 块）
- [x] 1.2 在 `seed.go` 的 Sync 流程中添加软删除逻辑：查找 `permission = "ai:chat"` 的菜单记录并 `db.Delete`
- [x] 1.3 移除 Casbin 策略中 `ai:chat` 相关的 `read` 策略种子行

## 2. 前端路由与菜单清理

- [x] 2.1 在 `module.ts` 中移除 `/ai/chat` 的 index 路由（保留 `/ai/chat/:sid` 子路由）
- [x] 2.2 删除 `pages/chat/index.tsx` 文件

## 3. Agent 列表页改为卡片视图

- [x] 3.1 重写 `pages/agents/index.tsx`：将 DataTable 表格替换为响应式卡片网格（grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4）
- [x] 3.2 设计 AgentCard 组件：类型图标+渐变背景、名称、类型 badge、描述（1行截断）、状态指示
- [x] 3.3 卡片底部添加"聊天"按钮：活跃 Agent 可点击（创建 Session → 跳转 `/ai/chat/:sid`），不活跃 Agent 按钮 disabled + 卡片降低不透明度
- [x] 3.4 卡片右上角添加 DropdownMenu 操作菜单：编辑（打开 AgentSheet）、详情（跳转 `/ai/agents/:id`）、删除（AlertDialog 确认）
- [x] 3.5 保留搜索栏和分页组件，适配卡片布局

## 4. 翻译与收尾

- [x] 4.1 更新 `zh-CN.json` 和 `en.json`：添加卡片相关文案（如"查看详情"、"开始聊天"等），清理废弃的 chat 选择页文案
- [x] 4.2 验证：Agent 卡片页搜索、创建、编辑、删除、聊天入口均正常工作
