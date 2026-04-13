## Why

当前系统权限体系仅有 API 访问控制（Casbin：能不能调接口）和菜单可见控制，缺少数据范围控制（能看哪些部门的数据行）。随着 ITSM 和可观测性模块的引入，"运维部经理只看运维部工单"、"一线工程师只看自己负责的告警"这类数据隔离需求无法满足。同时，组织结构缺少直属上级链路和非层级分组，制约了 ITSM 审批流和 on-call 团队管理。

## What Changes

- **Role 新增 dataScope 字段** — 枚举（ALL / DEPT_AND_SUB / DEPT / SELF / CUSTOM），定义该角色的数据可见范围策略
- **新增 RoleDeptScope 表** — CUSTOM 类型时，存储该角色可访问的指定部门集合
- **Kernel 新增 OrgScopeResolver 接口** — 极薄契约，Org App 实现，未装 Org 时为 nil（nil-safe，降级为 ALL）
- **Kernel 新增 DataScopeMiddleware** — 挂在 CasbinAuth 之后，解析当前用户的部门范围并注入请求上下文
- **ListParams 新增 DeptScope 字段** — 各业务 repository 按需消费，渐进式接入
- **User 新增 managerID 字段** — 自关联，指向直属上级，支持 ITSM 多级审批链推导
- **Org App 新增 UserGroup / UserGroupMember 模型** — 扁平分组，支持 on-call 组、变更委员会等非层级团队

## Capabilities

### New Capabilities

- `data-scope-policy`: Role 的数据范围策略配置（dataScope 枚举 + RoleDeptScope 自定义部门集合 + 管理 UI）
- `org-scope-resolver`: Kernel 中的 OrgScopeResolver 接口定义 + DataScopeMiddleware 运行时解析
- `user-manager`: User.managerID 直属上级字段（kernel 模型扩展 + 前端展示）
- `user-group`: Org App 内的用户组模型（UserGroup + UserGroupMember + 管理 UI）

### Modified Capabilities

- `role-management`: Role 模型新增 dataScope 字段；角色管理 UI 新增数据范围配置入口
- `permission-assignment`: 权限分配页面整合数据范围配置（dataScope 选择 + CUSTOM 模式的部门多选）

## Impact

- **后端**
  - `internal/model/role.go` — 新增 `DataScope` 字段
  - `internal/model/user.go` — 新增 `ManagerID` 字段
  - `internal/model/` — 新增 `RoleDeptScope` 模型（kernel）
  - `internal/database/database.go` — 新增 AutoMigrate 注册
  - `internal/app/` — 新增 `OrgScopeResolver` 接口（kernel app 层）
  - `internal/middleware/` — 新增 `DataScopeMiddleware`
  - `internal/repository/` — `ListParams` 新增 `DeptScope []uint`
  - `internal/app/org/` — 实现 `OrgScopeResolver`；新增 UserGroup 模型及 CRUD
  - `internal/handler/role.go` — 角色 CRUD 接入 dataScope 字段
- **前端**
  - `web/src/pages/roles/` — 角色表单新增数据范围选择
  - `web/src/pages/users/` — 用户表单新增直属上级选择
  - `web/src/apps/org/` — 新增用户组管理页面
- **数据库迁移** — roles 表加列、users 表加列、新增 role_dept_scopes 表、user_groups 表、user_group_members 表
- **无 breaking change** — 现有 Edition（lite、license）不安装 Org App，DataScopeMiddleware nil-safe 降级为 ALL，完全向后兼容
