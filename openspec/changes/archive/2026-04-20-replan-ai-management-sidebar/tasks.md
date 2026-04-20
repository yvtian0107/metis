## 1. Navigation Foundation

- [x] 1.1 扩展 `web/src/apps/registry.ts` 的 AppModule 导航元数据结构，使其同时支持现有平铺菜单和新的“二级分组 + 三级菜单项”定义。
- [x] 1.2 重构 `web/src/components/layout/sidebar.tsx`，让 sidebar 能渲染分组标题及其下级菜单项，并保持旧的两级 App 兼容。
- [x] 1.3 补齐 active path 解析与高亮逻辑，确保三级菜单项命中时对应 App、分组和叶子项都能正确激活。

## 2. AI Module Restructure

- [x] 2.1 调整 `web/src/apps/ai/module.ts` 的导航定义，按 `智能体 -> 知识 -> 工具 -> 模型接入` 声明 AI 二级分组顺序。
- [x] 2.2 将 `工具` 域拆分为独立三级路由：`/ai/tools/builtin`、`/ai/tools/mcp`、`/ai/tools/skills`，并保留 `/ai/tools` 默认重定向。
- [x] 2.3 拆分或改造 `web/src/apps/ai/pages/tools/index.tsx`，移除依赖 Tab 的主入口组织方式，使 `内建工具 / MCP 服务 / 技能包` 可独立作为页面内容承载。
- [x] 2.4 更新 AI 模块相关国际化文案，补齐分组标题、三级菜单文案和必要的默认跳转文案。

## 3. AI Menu Seed Alignment

- [x] 3.1 重构 `internal/app/ai/seed.go`，将 AI 管理菜单树从平铺页面菜单改为 `AI 管理 -> 二级目录 -> 三级菜单` 结构。
- [x] 3.2 将 Agents、知识库、内建工具、MCP 服务、技能包、供应商分别挂到对应二级目录下，并保持按钮权限继续绑定在叶子菜单节点。
- [x] 3.3 检查并补齐旧菜单结构升级后的兼容逻辑，避免已有实例因 permission 节点迁移导致菜单缺失或重复。

## 4. Verification

- [ ] 4.1 手工验证 AI sidebar 的显示顺序为 `智能体 -> 知识 -> 工具 -> 模型接入`，且 `智能体` 固定置顶。
- [ ] 4.2 手工验证 `/ai/tools` 重定向及 `/ai/tools/builtin`、`/ai/tools/mcp`、`/ai/tools/skills` 的直接访问与高亮状态。
- [x] 4.3 运行前端验证命令 `cd web && bun run lint` 和 `cd web && bun run build`，确认导航与路由重构未破坏构建。
