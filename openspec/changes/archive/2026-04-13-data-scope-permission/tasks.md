## 1. 数据模型（Kernel）

- [x] 1.1 `internal/model/role.go` — 新增 `DataScope string` 字段（默认 `"all"`）及 `DataScopeType` 常量枚举（`DataScopeAll / DeptAndSub / Dept / Self / Custom`）
- [x] 1.2 `internal/model/` — 新增 `RoleDeptScope` 模型（`roleID`, `departmentID`），TableName: `role_dept_scopes`
- [x] 1.3 `internal/model/user.go` — 新增 `ManagerID *uint` 自关联字段及 `Manager *User` 预加载关联
- [x] 1.4 `internal/model/user.go` — `UserResponse` 新增 `Manager *ManagerInfo`（`{id, username, avatar}`）字段
- [x] 1.5 `internal/database/database.go` — AutoMigrate 注册 `RoleDeptScope`；确认 `Role`、`User` 表列自动迁移

## 2. OrgScopeResolver 接口（Kernel App 层）

- [x] 2.1 `internal/app/app.go` — 新增 `OrgScopeResolver` 接口定义：`GetUserDeptScope(userID uint) ([]uint, error)`
- [x] 2.2 `internal/app/org/` — 新增 `scope_resolver.go`，实现 `OrgScopeResolver`，内部调用现有 `AssignmentService.GetUserDepartmentScope()`
- [x] 2.3 `internal/app/org/app.go` — 在 `Providers()` 中通过 `do.Provide()` 注册 `OrgScopeResolver` 实现

## 3. DataScopeMiddleware（Kernel）

- [x] 3.1 `internal/middleware/data_scope.go` — 新增 `DataScopeMiddleware`，从 IOC 容器 nil-safe 获取 `OrgScopeResolver`
- [x] 3.2 实现五种 scope 类型的解析逻辑（ALL→nil，SELF→空切片，DEPT→一层，DEPT_AND_SUB→BFS，CUSTOM→查 role_dept_scopes）
- [x] 3.3 `internal/handler/handler.go` — 将 `DataScopeMiddleware` 挂入认证路由链，位置在 `CasbinAuth` 之后
- [x] 3.4 `internal/repository/` — `ListParams` 新增 `DeptScope *[]uint` 字段，添加 scope 过滤的辅助方法

## 4. DataScope 策略 CRUD（后端）

- [x] 4.1 `internal/repository/role.go` — 新增 `GetRoleWithDeptScope(id)` 查询（预加载 RoleDeptScope）；新增 `SetRoleDeptScope(roleID, deptIDs)` 原子替换方法
- [x] 4.2 `internal/service/role.go` — 新增 `UpdateDataScope(roleID, scope, deptIDs)` 方法，含 admin 角色防护和循环检测（ManagerID 场景复用）
- [x] 4.3 `internal/handler/role.go` — 新增 `PUT /api/v1/roles/:id/data-scope` 端点；`GET /api/v1/roles/:id` 响应包含 `dataScope` 和 `deptIds`
- [x] 4.4 `internal/seed/policies.go` — 新增 admin 角色对 `/api/v1/roles/:id/data-scope` PUT 的 Casbin 策略

## 5. User.ManagerID（后端）

- [x] 5.1 `internal/service/user.go` — 新增 `UpdateManager(userID, managerID)` 方法，含循环链检测（BFS，最大深度 10）
- [x] 5.2 `internal/handler/user.go` — `PUT /api/v1/users/:id` 接受 `managerId` 字段；`GET /api/v1/users/:id` 响应预加载 Manager 对象
- [x] 5.3 `internal/handler/user.go` — 新增 `GET /api/v1/users/:id/manager-chain` 端点，返回有序上级链（最多 10 层）

## 6. UserGroup（Org App 后端）

- [x] 6.1 `internal/app/org/model.go` — 新增 `UserGroup`（`name, code, description, isActive`）和 `UserGroupMember`（`groupID, userID`）模型
- [x] 6.2 `internal/app/org/app.go` — `Models()` 返回中加入两个新模型
- [x] 6.3 新增 `group_repository.go` — CRUD + 成员管理方法（`AddMembers`, `RemoveMembers`, `ListMembers`, `GetUserGroups`，含成员计数）
- [x] 6.4 新增 `group_service.go` — 业务逻辑（重复成员幂等处理、空组删除等）
- [x] 6.5 新增 `group_handler.go` — 注册路由：`/org/groups` CRUD + `/org/groups/:id/members` 成员管理 + `/org/users/:id/groups`
- [x] 6.6 `internal/app/org/seed.go` — 新增用户组菜单（"用户组"，路径 `/org/groups`，权限 `org:group:list`）及 admin Casbin 策略

## 7. 前端：角色数据权限配置

- [x] 7.1 `web/src/pages/roles/` — 角色列表表格新增 `dataScope` Badge 列（展示中文标签：全部数据 / 本部门及下级 / 仅本部门 / 仅本人 / 自定义）
- [x] 7.2 权限分配 Sheet（或新建独立 Sheet）新增"数据权限"Tab，含 RadioGroup 选择 scope 类型
- [x] 7.3 "自定义"选项展开部门多选树（复用现有 department tree 组件）
- [x] 7.4 调用 `PUT /api/v1/roles/:id/data-scope` 保存，成功后刷新角色列表；admin 角色该 Tab 只读

## 8. 前端：用户直属上级

- [x] 8.1 `web/src/pages/users/` — 用户列表表格新增"直属上级"列（显示头像+姓名，无则"—"）
- [x] 8.2 用户新建/编辑 Sheet 新增"直属上级"可搜索用户选择器（搜索调用 GET /api/v1/users?keyword=）
- [x] 8.3 前端 API 层新增 `getUserManagerChain(userId)` 方法（供未来 ITSM 使用，暂不展示）

## 9. 前端：用户组管理页面（Org App）

- [x] 9.1 `web/src/apps/org/` — 新增 `pages/groups/index.tsx`，实现用户组列表页（`useListPage` hook，表格含 name/code/成员数/状态/操作列）
- [x] 9.2 新增新建/编辑 Sheet 组件（name、code、description、isActive 字段）
- [x] 9.3 新增"管理成员" Sheet，含当前成员列表（可移除）和用户搜索添加功能
- [x] 9.4 `web/src/apps/org/module.ts` — 注册 `/org/groups` 路由

## 10. 国际化 & 收尾

- [x] 10.1 `internal/locales/` — 新增 zh-CN / en 翻译条目（dataScope 类型标签、用户组相关文案、直属上级）
- [x] 10.2 运行 `go build -tags dev ./cmd/server/` 验证后端编译通过
- [x] 10.3 运行 `cd web && bun run lint` 验证前端无 ESLint 错误
