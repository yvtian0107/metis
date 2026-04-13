## Context

当前 AI 模块有两个独立入口：`/ai/agents`（表格列表管理）和 `/ai/chat`（选择 Agent 后聊天）。`/ai/chat` 的 index 页面本质是一个 Agent 卡片选择器，与管理页功能重叠。Agent 详情页已有 Sessions Tab 可查看历史会话。

现有组件：
- `pages/agents/index.tsx` — DataTable 表格列表
- `pages/chat/index.tsx` — AgentCard 网格选择器（已有卡片组件）
- `pages/chat/[sid].tsx` — 聊天界面（不变）
- `pages/agents/[id].tsx` — Agent 详情页（不变）
- `seed.go` — 菜单种子数据包含"对话"菜单项

## Goals / Non-Goals

**Goals:**
- 将 Agent 列表从表格改为卡片网格，提升视觉表现力
- 在卡片上直接提供"聊天"入口，减少跳转
- 移除冗余的"对话"菜单项和选择页面
- 保持所有现有功能可达（搜索、创建、编辑、删除、查看详情）

**Non-Goals:**
- 不修改聊天界面本身（`[sid].tsx`）
- 不修改 Agent 详情页（`[id].tsx`）
- 不修改后端 API
- 不引入新的组件库或依赖

## Decisions

### D1: 卡片布局复用 chat/index.tsx 的设计模式

**选择**：基于现有 `AgentCard` 组件扩展，不从零设计。

**原因**：`chat/index.tsx` 已有类型图标、渐变背景、名称+描述的卡片布局，经过验证的设计语言。扩展它比重新设计成本更低、一致性更好。

**替代方案**：从零设计新卡片 — 增加设计决策点，可能与现有风格不一致。

### D2: 卡片操作使用 DropdownMenu 而非行内按钮

**选择**：卡片右上角放一个 `MoreHorizontal` 图标按钮，点击展开 DropdownMenu（编辑、查看详情、删除）。

**原因**：卡片空间有限，多个行内按钮会破坏视觉节奏。DropdownMenu 是 shadcn/ui 标准模式，与项目其他地方一致。

**替代方案**：Hover 时显示操作栏 — 移动端不友好；右键菜单 — 发现性差。

### D3: 聊天按钮直接创建新 Session

**选择**：点击"聊天"按钮 → 调用 `sessionApi.create(agentId)` → 跳转 `/ai/chat/:sid`。与现有 `chat/index.tsx` 行为一致。

**原因**：最简单直接，用户期望"点击即聊"。历史会话可通过 Agent 详情页的 Sessions Tab 访问。

**替代方案**：弹出 Popover 显示最近对话 — 增加复杂度，且 Agent 详情页已有此功能。

### D4: 不活跃 Agent 卡片禁用聊天按钮

**选择**：不活跃 Agent 的卡片正常显示但添加视觉区分（降低不透明度），聊天按钮 disabled。编辑/删除仍可操作。

**原因**：管理页需要看到所有 Agent，但不活跃的不应该能开始聊天。

### D5: 后端种子数据 — 软删除对话菜单

**选择**：在 `seed.Sync()` 中查找 `ai:chat` 菜单并执行软删除（`db.Delete`），而不是从 `seed.go` 中删除创建代码。

**原因**：已有数据库中可能已存在该菜单记录。仅删除种子代码不会清理已有数据。Sync 中主动删除确保升级时自动清理。新安装时因为 Install 先跑，菜单会被创建后在 Sync 中又被删除 — 可以直接从 Install 代码中也移除创建逻辑。

### D6: 卡片网格响应式列数

**选择**：`grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4`

**原因**：与 `chat/index.tsx` 类似但最大列数从 5 减为 4，因为管理卡片信息更多（有操作菜单），需要更大的卡片宽度。

## Risks / Trade-offs

- **[搜索和过滤的视觉空间]** 卡片视图下搜索栏仍然需要，但不再有表头排序功能 → 搜索栏保持不变，排序需求不大（当前表格也没有排序功能）
- **[大量 Agent 时卡片性能]** 几十个 Agent 的卡片渲染无压力，保持分页 → 使用已有的 `useListPage` 分页机制
- **[菜单种子回滚]** 如果需要回滚，已删除的菜单需要重新种子 → Sync 逻辑是幂等的，移除删除代码后重启即可恢复

## Migration Plan

1. 前端先改：卡片视图 + 移除 chat index 路由
2. 后端跟进：种子数据清理对话菜单
3. 翻译文件更新
4. 无数据迁移，无 API 变更，可直接部署

## Open Questions

（无）
