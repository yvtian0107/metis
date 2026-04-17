## Context

Org App 当前有三个独立页面：部门管理（树形表格 CRUD）、岗位管理（分页表格 CRUD）、人员分配（左树右表 Master-Detail）。部门管理页看不到成员数量，人员分配页看不到部门详细信息，用户需要在两个页面间跳转。

后端 API 已完备：`/departments/tree` 返回 `DepartmentTreeNode`（含 `memberCount`、`managerName`、`children`），`/departments/:id/positions` 返回可用岗位，`/org/users?departmentId=X` 返回成员列表。无需后端改动。

## Goals / Non-Goals

**Goals:**
- 以部门为中心建立两层模式（列表 → 详情），让用户在一个上下文中完成部门管理和人员分配
- 删除独立的人员分配页面，减少导航和认知负担
- 在部门列表页展示 memberCount，提供整体概览
- 复用现有组件资产（`add-member-sheet`、`change-position-sheet`、`user-org-sheet`、`member-list`）

**Non-Goals:**
- 不修改后端 API 或数据模型
- 不重构岗位管理页（保持不变）
- 不做组织架构图可视化（拓扑图等高级可视化留待后续）
- 不增加新的权限点，复用现有 `org:department:*` 和 `org:assignment:*`

## Decisions

### 1. L1 保持树形表格，不改为纯树列表

**选择**: 增强现有树形表格，增加 memberCount 列和行导航

**理由**: 树形表格已经在用，改成纯树列表反而是视觉降级——丢失了 code、manager、status 的对齐对比能力。树形表格增加一列 memberCount 成本最低，改动最小。

**替代方案**: 纯树列表（类似 assignments 的 department-tree 组件）——视觉更轻但信息密度低，不适合管理场景。

### 2. 详情页使用单列分区布局，不用 Tab

**选择**: 信息卡 → 可用岗位 → 成员列表 → 子部门，垂直堆叠

**理由**: 部门详情的各区块内容量都不大（岗位通常 3-10 个，成员通常 5-30 人，子部门通常 2-5 个），不需要 Tab 切换来节省空间。垂直堆叠让用户一眼看到全貌。

**替代方案**: Tab 切换（信息/成员/子部门）——增加点击次数，适合大量数据但这里不需要。

### 3. 可用岗位管理：详情页内 inline chips + 管理 popover

**选择**: 在信息卡下方显示岗位 chips，点击"管理"按钮弹出 popover 进行选择

**理由**: 岗位数量少（< 10），chips 展示直观。从 department-sheet 中剥离岗位选择逻辑，让 sheet 更轻量（仅保留基本信息编辑）。

### 4. 成员列表：复用 member-list 组件逻辑

**选择**: 将 `assignments/member-list.tsx` 的表格结构和操作菜单迁移到详情页的成员区，而非直接 import 原组件

**理由**: 原组件的 props 接口是为 Master-Detail 布局设计的（接收 `selectedDept`、空状态提示等），直接复用需要大量 prop 适配。提取核心表格逻辑更干净。

### 5. 行点击导航：chevron 区域展开，行本身导航

**选择**: L1 树表格中，点击 chevron 展开/折叠子部门，点击行其他区域导航到 `/org/departments/:id`

**理由**: 同时保留树的展开导航和详情页导航。用户需要先通过展开看到子部门，然后点击具体部门进入详情。

### 6. 菜单 seed 调整

**选择**: 将"部门管理"菜单重命名为"组织架构"，删除"人员分配"菜单项

**理由**: 部门详情页已包含人员分配功能，独立菜单不再需要。"组织架构"比"部门管理"更准确地描述新页面的职责。

## Risks / Trade-offs

- **[破坏性删除]** 删除 assignments 页面是不可逆的 → 确保详情页覆盖 assignments 的所有功能后再删除，可以先新增详情页，最后一步删除 assignments
- **[菜单 seed 迁移]** 重命名菜单需要 sync seed → `seed.Sync()` 是增量的，不会覆盖已有数据；需要在 seed 中处理旧菜单的清理
- **[权限判断]** 详情页同时需要 `org:department:*` 和 `org:assignment:*` 权限 → 复用现有 `usePermission` hook，不同区域分别判断
- **[URL 书签]** 用户可能收藏了 `/org/assignments` → 可以在 module.ts 中添加重定向到 `/org/departments`
