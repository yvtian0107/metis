## 1. 基础设施与品牌色系

- [ ] 1.1 创建供应商品牌色映射工具 `web/src/apps/ai/lib/provider-brand.ts`：导出 `PROVIDER_BRAND_MAP`（type → { stripe, avatarBg, avatarText, label }）和 `getProviderBrand(type)` 函数，包含 openai/anthropic/ollama 三种映射及 fallback
- [ ] 1.2 新增状态指示灯组件 `web/src/apps/ai/components/status-dot.tsx`：接受 status prop（active/inactive/error），渲染对应颜色圆点 + CSS pulse 动画，支持 loading 状态显示 Spinner

## 2. 供应商卡片组件

- [ ] 2.1 创建供应商卡片组件 `web/src/apps/ai/components/provider-card.tsx`：包含品牌色顶部条纹（3-4px）、首字母 Avatar（品牌背景色）、供应商名称、base URL（truncate）、masked API Key、模型类型统计 chips、状态指示灯 + 相对时间、快捷测试按钮、⋯ 下拉菜单（编辑/删除）
- [ ] 2.2 创建引导添加卡片组件（可内联在列表页）：虚线边框、"+" 图标、点击触发创建 Drawer
- [ ] 2.3 实现卡片 hover 效果：border-primary/20 + shadow-md + translateY(-1px) 过渡动画
- [ ] 2.4 实现卡片点击导航至详情页 `/ai/providers/:id`，排除操作按钮区域的点击穿透

## 3. 列表页重构

- [ ] 3.1 重写 `web/src/apps/ai/pages/providers/index.tsx`：替换 Table 为 CSS Grid 卡片网格（auto-fill, minmax(340px, 1fr)），全量加载 pageSize=100，移除分页组件
- [ ] 3.2 保留搜索栏功能，移除行内展开的 ProviderModels 组件和相关代码
- [ ] 3.3 实现空状态：居中 Server 图标 + 描述文字 + 主操作按钮"添加第一个供应商"
- [ ] 3.4 调整 ProviderSheet 仅用于创建模式（编辑入口移到详情页）

## 4. 供应商详情页

- [ ] 4.1 创建详情页 `web/src/apps/ai/pages/providers/[id].tsx`：使用 `useQuery` 获取单个供应商数据（`GET /api/v1/ai/providers/:id`），包含返回链接 + 页面标题
- [ ] 4.2 实现供应商信息区：description-list 布局展示名称、类型（品牌 Badge）、协议、Base URL、masked API Key、状态指示灯、最近检查时间
- [ ] 4.3 实现信息区操作按钮："编辑"（打开 ProviderSheet 编辑模式）、"测试连接"（调用 test API + 实时状态更新）、"同步模型"（调用 sync API + 刷新模型列表）
- [ ] 4.4 实现模型管理区：从原 `ProviderModels` + `ModelGroupedList` 组件迁移，保留按类型分组的表格、模型 CRUD、设默认等全部功能
- [ ] 4.5 在模型管理区顶部添加关键字搜索（客户端过滤 displayName/modelId）

## 5. 路由与导航

- [ ] 5.1 在 `web/src/apps/ai/module.ts` 中注册新路由 `ai/providers/:id`，lazy import 详情页组件
- [ ] 5.2 确保详情页的"返回供应商列表"链接正确导航到 `/ai/providers`

## 6. 国际化

- [ ] 6.1 在 `web/src/apps/ai/locales/zh-CN.json` 和 `en.json` 中新增翻译 key：详情页标题、返回按钮、信息区字段标签、引导卡片文案、空状态文案等
