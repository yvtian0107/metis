## Why

AI 供应商管理页当前是标准数据表格 + 行内展开模型的布局。供应商数量通常只有 1-5 个，表格形态信息密度低、视觉辨识度差（OpenAI/Anthropic/Ollama 行看起来几乎一样），且行内展开的模型列表让页面层级混乱。需要将供应商管理升级为卡片化总览 + 独立详情页的两层架构，提升视觉品质和操作效率。

## What Changes

- **列表页重构为卡片网格**：用品牌色条 + 首字母 Avatar 的供应商卡片替代表格行，响应式网格布局（auto-fill, minmax 340px），全量加载不分页
- **新增供应商详情页** (`/ai/providers/:id`)：包含供应商信息区（含编辑、测试连接、同步模型）和完整模型管理表格（分组显示、搜索筛选、CRUD）
- **卡片交互设计**：品牌色顶部条纹（3-4px）、状态指示灯（绿/灰/红 + 动画）、模型类型统计 chips、快捷测试按钮、hover 提升效果
- **操作重分配**：创建保留 Drawer，编辑移至详情页，模型管理全部移至详情页，卡片 ⋯ 菜单保留编辑/删除快捷入口
- **空状态和引导卡片**：无供应商时显示引导空状态，有供应商时网格末尾显示虚线引导添加卡片

## Capabilities

### New Capabilities
- `ai-provider-card-ui`: 供应商卡片化列表布局，包含品牌色系、状态指示灯、模型统计 chips、引导卡片等视觉组件
- `ai-provider-detail-page`: 供应商详情页，包含供应商信息展示/编辑、连接测试、模型分组管理完整功能

### Modified Capabilities

## Impact

- **前端文件**：
  - 重写 `web/src/apps/ai/pages/providers/index.tsx`（表格 → 卡片网格）
  - 新增 `web/src/apps/ai/pages/providers/[id].tsx`（详情页）
  - 新增 `web/src/apps/ai/components/provider-card.tsx`（卡片组件）
  - 调整 `web/src/apps/ai/module.ts`（新增 `:id` 路由）
  - 调整 `web/src/apps/ai/components/provider-sheet.tsx`（仅用于创建）
- **后端**：无需变更，现有 API 完全满足需求
- **i18n**：新增少量翻译 key（详情页标题、返回按钮、信息区标签等）
