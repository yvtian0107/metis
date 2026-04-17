## Why

部门管理和人员分配当前是两个独立页面，用户需要在两个页面间跳转才能完成一个连贯的工作流：部门管理页看不到"人"，人员分配页看不到"部门信息"。部门作为组织架构的核心实体，拥有子部门、可用岗位、成员三类子实体，符合 DESIGN.md 的两层模式（List → Detail）判定标准，但当前缺少详情页。

## What Changes

- **新增部门详情页** (`/org/departments/:id`)：整合部门基本信息、可用岗位管理、部门成员管理（含添加/编辑岗位/移除）、子部门导航，形成完整的部门工作区
- **增强部门列表页**：在树形视图中显示 `memberCount`（后端已返回但前端未使用），点击行导航到详情页而非仅展开/折叠
- **删除人员分配独立页面**：`/org/assignments` 的全部功能吸收进部门详情页，移除该页面和对应菜单项
- 菜单从三项（部门管理/岗位管理/人员分配）精简为两项（组织架构/岗位管理）
- 岗位管理页保持不变（全局字典，不受影响）
- 后端 API 无需修改，现有接口已完全满足新 UI 需求

## Capabilities

### New Capabilities

- `org-department-detail-ui`: 部门详情页，包含信息卡、可用岗位管理、成员列表（分页+搜索+添加/编辑/移除）、子部门导航

### Modified Capabilities

- `org-department-ui`: 部门列表页增强——显示 memberCount、行点击导航到详情页、改进搜索体验
- `org-assignment-ui`: 标记为废弃并移除——功能完全吸收到 `org-department-detail-ui`

## Impact

- **前端文件**:
  - 新增: `web/src/apps/org/pages/departments/[id].tsx`（详情页）
  - 修改: `web/src/apps/org/pages/departments/index.tsx`（列表页增强）
  - 修改: `web/src/apps/org/module.ts`（路由调整，移除 assignments 路由，添加 `:id` 路由）
  - 删除: `web/src/apps/org/pages/assignments/` 目录（4 个文件）
  - 复用: `department-sheet.tsx`, `add-member-sheet.tsx`, `change-position-sheet.tsx`, `user-org-sheet.tsx` 移至详情页使用
- **后端**: 无变更，现有 API（`/departments/tree`, `/departments/:id/positions`, `/org/users`）完全足够
- **菜单 Seed**: 需要修改 `internal/app/org/seed.go`，移除人员分配菜单项，重命名部门管理为组织架构
- **i18n**: 需要更新 `web/src/apps/org/locales/` 中的翻译 key
- **权限**: 现有 `org:department:*` 和 `org:assignment:*` 权限保持不变，仅 UI 入口合并
