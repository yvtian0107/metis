## Context

Org App 是 Metis 的组织管理模块，提供部门树、岗位、人员分配三大功能。代码 review 发现多个层级的质量问题：

**后端现状**：
- `AssignmentService` 的 primary 管理（demote → set）分散在两个独立 DB 操作中，无事务保护
- `demoteCurrentPrimary` 的错误被 `_ =` 丢弃
- `SetPrimary` repo 方法先清空所有 primary 再设目标，但不验证目标存在
- `AddUserPosition` 不验证部门/岗位是否存在或 active
- `CountAssignments` 用 map key 拼接 SQL
- `DepartmentService.Update` 有 8 个 pointer 参数
- `GetUserDepartmentScope` BFS 每层一次查询

**前端现状**：
- `assignments/index.tsx` 866 行，13 个 useState，所有逻辑揉在一起
- 手搓 Popover Combobox（无键盘导航）
- `pageSize=9999` 加载全部岗位
- 用户搜索无 debounce
- Add Member 表单用裸 state 而非 RHF+Zod

## Goals / Non-Goals

**Goals:**
- 消除 primary 管理的并发竞态，确保任一时刻每个用户最多一个 primary assignment
- 所有 DB 错误正确传播，不静默丢弃
- 外键引用的实体存在性在 service 层校验
- 前端 assignments 页面拆分为可维护的子组件
- 用户选择器使用 shadcn Command 组件
- 消除 pageSize=9999 hack
- 搜索加 debounce

**Non-Goals:**
- 不修改数据库 schema（不加新表、不改列）
- 不修改 API 路由结构或响应格式（保持前端兼容）
- 不新增功能（如批量分配、拖拽排序）
- 不处理 Group（用户组）功能（尚未完成，单独处理）
- 不做性能基准测试

## Decisions

### D1: Primary 管理统一收敛到 Repository 事务

**选择**：将所有 primary demote+set 逻辑移到 repo 层的事务中，service 层不再自己做两步操作。

**理由**：当前 `SetPrimary` repo 方法已经正确使用事务，但 `AddUserPosition` 和 `UpdateUserPosition` 在 service 层分散调用 demote + create/update。统一到 repo 事务可以：
1. 保证原子性
2. 避免 service 层忘记处理 demote 错误
3. 减少重复逻辑

**替代方案**：在 service 层加事务 — 但 service 层目前没有持有 `*gorm.DB`，需要改架构引入 UnitOfWork 模式，改动过大。

### D2: AddUserPosition 的外键校验放在 Service 层

**选择**：在 service 层调用 `DepartmentRepo.FindByID` 和 `PositionRepo.FindByID` 验证存在性和 active 状态。

**理由**：依赖 DB FK 约束只能拿到底层错误消息（如 `FOREIGN KEY constraint failed`），无法给用户有意义的提示。Service 层校验可以返回明确的 `ErrDepartmentNotFound` / `ErrPositionInactive`。

**替代方案**：在 handler 层校验 — 但这会让 handler 承担业务逻辑，违反分层原则。

### D3: DepartmentService.Update 改用 struct 入参

**选择**：定义 `UpdateDepartmentInput` struct，用 pointer 字段表示可选更新。

**理由**：8 个 pointer 参数可读性差、容易传错顺序。struct 入参支持 field name 访问，Go 的 named field 初始化天然防止顺序错误。

### D4: GetUserDepartmentScope 改为全量加载 + 内存 BFS

**选择**：一次查询加载所有 active 部门的 `(id, parent_id)`，在内存中构建 parent→children map，再 BFS。

**理由**：部门数量通常 < 1000，一次全量加载的数据量很小（每条约 16 bytes）。避免 N+1 查询，对深层组织树性能提升显著。

**替代方案**：Recursive CTE — SQLite 支持 `WITH RECURSIVE`，但需要写 raw SQL，且 PostgreSQL 语法略有差异，增加维护成本。全量加载更简单且跨数据库兼容。

### D5: 前端 assignments 页面拆分策略

**选择**：拆为 3 个子组件 + 1 个页面容器：

```
pages/assignments/
├── index.tsx              (~120 行，容器 + 状态协调)
├── department-tree.tsx    (~150 行，左侧部门树)
├── member-list.tsx        (~200 行，右侧成员列表)
└── add-member-sheet.tsx   (~180 行，添加成员 Sheet)
```

**理由**：按 UI 区域拆分，每个组件有明确的输入/输出边界。容器组件持有共享状态（selectedDeptId），通过 props 传递给子组件。

### D6: 用户选择器改用 shadcn Command

**选择**：用 `Command` + `CommandInput` + `CommandList` + `CommandItem` 替代手搓的 Popover+Input+button 列表。

**理由**：Command 组件（基于 cmdk）提供：键盘导航（↑↓）、自动聚焦、内置搜索过滤、accessibility 支持。手搓版本没有这些。

### D7: 消除 pageSize=9999

**选择**：岗位数据通过一个不分页的 query（`useQuery` 一次性加载）获取，后端 positions list endpoint 在 `pageSize=0` 时返回全部。

**理由**：岗位是低基数配置数据（通常 < 50 条），不需要分页。用 `pageSize=0` 作为"返回全部"的约定比 9999 语义清晰，且已在其他 endpoint 中使用。

## Risks / Trade-offs

- **[Risk] Primary 管理重构可能遗漏边界场景** → 逐个方法添加测试用例，验证并发场景
- **[Risk] 前端拆分可能导致 props drilling** → 保持一层传递，如果超过 3 层再考虑 context
- **[Trade-off] 全量加载部门 vs Recursive CTE** → 选择简单方案，牺牲理论最优性能（部门数 < 1000 时无差异）
- **[Trade-off] pageSize=0 约定** → 需要在 repo/handler 层约定，所有分页 endpoint 统一行为
