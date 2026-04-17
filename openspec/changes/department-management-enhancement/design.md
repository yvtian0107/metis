## Context

部门管理 (`internal/app/org/`) 当前支持树形部门结构、全局岗位库、人员分配三大功能。后端 Department model 已有 `ManagerID *uint` 字段但前端未暴露；Position 与 Department 之间无关联约束，任意职位可分配到任意部门；表单中有一个用户无需关心的 sort 字段。

## Goals / Non-Goals

**Goals:**
- 前端表单移除 sort 字段，简化用户操作
- 前端完整暴露部门负责人能力（选择器 + 列表展示 + Tree API 返回 managerName）
- 新增 DepartmentPosition 多对多关联，实现部门级可用职位管控
- 人员分配时强制校验职位合法性（部门未配置可用职位时不限制，向后兼容）

**Non-Goals:**
- 不改变后端 sort 字段的存储和排序逻辑
- 不实现前端拖拽排序
- 不改变 Position model 本身的结构
- 不修改 DataScope / OrgScopeResolver 逻辑

## Decisions

### 1. DepartmentPosition 用独立关联表而非 JSON 字段

**选择**: 新增 `department_positions` 表 (department_id, position_id) 联合唯一索引

**理由**: 关联表支持 JOIN 查询、外键约束、索引优化，且与现有 UserPosition 模式一致。JSON 字段在 SQLite 下查询不便，且无法做外键约束。

### 2. 可用职位 API 采用 PUT 全量替换而非增删

**选择**: `PUT /api/v1/org/departments/:id/positions` 接收 `positionIds: []uint`，全量替换该部门的可用职位

**理由**: 前端多选组件天然产出完整列表，全量替换实现简单且避免增删接口的并发问题。在事务内先删后插保证一致性。

### 3. 空可用职位列表 = 不限制

**选择**: 如果 department_positions 表中该部门无记录，则人员分配时不校验职位

**理由**: 向后兼容——现有数据没有可用职位配置，升级后不应阻断已有的分配流程。

### 4. Tree API 附带 managerName 通过 JOIN 一次查出

**选择**: Tree 查询时 LEFT JOIN users 表获取 manager 用户名，在 DepartmentTreeNode 中增加 `ManagerName string` 字段

**理由**: 避免 N+1 查询。Tree 一次性返回所有部门，JOIN 一次即可获取所有 manager 信息。

### 5. 负责人从全局用户列表选择

**选择**: 前端用 `/api/v1/users` 获取用户列表作为负责人候选

**理由**: 新建部门时还没有成员，无法从部门成员中选。全局选择更灵活，也符合实际场景（负责人可能先指定再归入部门）。

## Risks / Trade-offs

- **[全量替换可用职位可能误删]** → 前端在 PUT 前始终读取最新列表，UI 展示当前绑定状态
- **[强制校验可能阻断历史分配]** → 空列表 = 不限制，仅在部门显式配置可用职位后才生效
- **[Tree API 增加 managerName 改变返回结构]** → 新增字段，不删除已有字段，前端向后兼容
